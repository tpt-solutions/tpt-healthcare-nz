package subscription

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// wsWriteTimeout is the per-write deadline for WebSocket messages.
	wsWriteTimeout = 10 * time.Second

	// wsPingInterval is how often a ping frame is sent to each client to keep
	// the connection alive.
	wsPingInterval = 30 * time.Second

	// wsMaxMessageSize is the maximum inbound WebSocket message size (bytes).
	wsMaxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// CheckOrigin validates the Origin header. Callers should replace this with
	// a production-hardened check (allowlist of trusted Origins).
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// wsClient represents a single WebSocket connection registered to receive
// FHIR subscription notifications.
type wsClient struct {
	id           uuid.UUID
	conn         *websocket.Conn
	send         chan []byte
	subscriptionIDs []uuid.UUID // subscriptions this connection is listening to
}

// Hub manages all active WebSocket connections for the subscription engine.
// It is safe for concurrent use.
type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]*wsClient

	// register and unregister are used by ServeWS to add/remove clients safely.
	register   chan *wsClient
	unregister chan *wsClient

	// broadcast routes a notification payload to the clients whose
	// subscriptions match.
	broadcast chan hubMessage
}

// hubMessage pairs a notification bundle with the subscription IDs it targets.
type hubMessage struct {
	subscriptionIDs []uuid.UUID
	payload         []byte
}

// NewHub creates an idle Hub. Call Run() in a goroutine to start it.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]*wsClient),
		register:   make(chan *wsClient, 8),
		unregister: make(chan *wsClient, 8),
		broadcast:  make(chan hubMessage, 64),
	}
}

// Run processes register, unregister, and broadcast events. Blocks until the
// channel is closed (typically when the application shuts down).
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.id] = client
			h.mu.Unlock()
			log.Printf("subscription/ws: client %s connected (%d total)", client.id, h.clientCount())

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.id]; ok {
				delete(h.clients, client.id)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("subscription/ws: client %s disconnected (%d total)", client.id, h.clientCount())

		case msg := <-h.broadcast:
			// Build a lookup set for O(1) subscription matching.
			idSet := make(map[uuid.UUID]struct{}, len(msg.subscriptionIDs))
			for _, id := range msg.subscriptionIDs {
				idSet[id] = struct{}{}
			}
			h.mu.RLock()
			for _, client := range h.clients {
				for _, subID := range client.subscriptionIDs {
					if _, ok := idSet[subID]; ok {
						select {
						case client.send <- msg.payload:
						default:
							// Slow consumer — drop rather than block.
							log.Printf("subscription/ws: dropped message for slow client %s", client.id)
						}
						break
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// ServeWS upgrades an HTTP request to a WebSocket connection and registers the
// client with the Hub. subscriptionIDs lists which FHIR subscriptions this
// connection wants to receive notifications for.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, subscriptionIDs []uuid.UUID) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("subscription/ws: upgrade: %w", err)
	}

	client := &wsClient{
		id:              uuid.New(),
		conn:            conn,
		send:            make(chan []byte, 32),
		subscriptionIDs: subscriptionIDs,
	}

	h.register <- client

	go client.writePump()
	go client.readPump(h.unregister)

	return nil
}

// Dispatch sends a FHIR notification bundle to all WebSocket clients that are
// listening to any of the given subscription IDs.
func (h *Hub) Dispatch(subscriptionIDs []uuid.UUID, bundle json.RawMessage) {
	h.broadcast <- hubMessage{
		subscriptionIDs: subscriptionIDs,
		payload:         bundle,
	}
}

func (h *Hub) clientCount() int {
	h.mu.RLock()
	n := len(h.clients)
	h.mu.RUnlock()
	return n
}

// writePump pumps outbound messages from client.send to the WebSocket connection.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(wsPingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("subscription/ws: write to %s: %v", c.id, err)
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump drains inbound frames (the WebSocket spec requires reading to
// process control frames like pong and close). FHIR WebSocket subscriptions
// are server-push only; client messages are discarded.
func (c *wsClient) readPump(unregister chan<- *wsClient) {
	defer func() {
		unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(wsMaxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPingInterval * 3))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(wsPingInterval * 3))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("subscription/ws: read from %s: %v", c.id, err)
			}
			return
		}
	}
}
