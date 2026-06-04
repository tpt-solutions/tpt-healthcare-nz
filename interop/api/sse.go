package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// sseEvent is a single Server-Sent Event frame.
type sseEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// sseSubscriber is one connected SSE client.
type sseSubscriber struct {
	ch     chan sseEvent
	closed chan struct{}
}

// SSEHub routes domain events to connected SSE clients.
// Staff subscribers receive all events for a queue (including location).
// Patient subscribers receive only their own entry events (no location of others).
type SSEHub struct {
	mu      sync.RWMutex
	// staffSubs holds all staff subscribers for a queueID.
	staffSubs map[uuid.UUID][]*sseSubscriber
	// patientSubs holds patient-specific subscribers keyed by entryID.
	patientSubs map[uuid.UUID][]*sseSubscriber
	logger      *slog.Logger
}

// NewSSEHub creates an empty SSEHub.
func NewSSEHub(logger *slog.Logger) *SSEHub {
	return &SSEHub{
		staffSubs:   make(map[uuid.UUID][]*sseSubscriber),
		patientSubs: make(map[uuid.UUID][]*sseSubscriber),
		logger:      logger,
	}
}

// subscribeStaff registers a new staff SSE subscriber for queueID.
func (h *SSEHub) subscribeStaff(queueID uuid.UUID) *sseSubscriber {
	sub := &sseSubscriber{ch: make(chan sseEvent, 32), closed: make(chan struct{})}
	h.mu.Lock()
	h.staffSubs[queueID] = append(h.staffSubs[queueID], sub)
	h.mu.Unlock()
	return sub
}

// unsubscribeStaff removes a staff subscriber.
func (h *SSEHub) unsubscribeStaff(queueID uuid.UUID, sub *sseSubscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.staffSubs[queueID]
	for i, s := range subs {
		if s == sub {
			h.staffSubs[queueID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	close(sub.closed)
}

// subscribePatient registers a patient SSE subscriber for their entryID.
func (h *SSEHub) subscribePatient(entryID uuid.UUID) *sseSubscriber {
	sub := &sseSubscriber{ch: make(chan sseEvent, 16), closed: make(chan struct{})}
	h.mu.Lock()
	h.patientSubs[entryID] = append(h.patientSubs[entryID], sub)
	h.mu.Unlock()
	return sub
}

// unsubscribePatient removes a patient subscriber.
func (h *SSEHub) unsubscribePatient(entryID uuid.UUID, sub *sseSubscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.patientSubs[entryID]
	for i, s := range subs {
		if s == sub {
			h.patientSubs[entryID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	close(sub.closed)
}

// BroadcastStaff sends an event to all staff subscribers watching queueID.
func (h *SSEHub) BroadcastStaff(queueID uuid.UUID, ev sseEvent) {
	h.mu.RLock()
	subs := make([]*sseSubscriber, len(h.staffSubs[queueID]))
	copy(subs, h.staffSubs[queueID])
	h.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- ev:
		default:
			h.logger.Warn("SSE staff subscriber buffer full, dropping event",
				slog.String("queue", queueID.String()),
				slog.String("event", ev.Event),
			)
		}
	}
}

// BroadcastPatient sends an event to all subscribers for a specific entryID.
func (h *SSEHub) BroadcastPatient(entryID uuid.UUID, ev sseEvent) {
	h.mu.RLock()
	subs := make([]*sseSubscriber, len(h.patientSubs[entryID]))
	copy(subs, h.patientSubs[entryID])
	h.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- ev:
		default:
			h.logger.Warn("SSE patient subscriber buffer full, dropping event",
				slog.String("entry", entryID.String()),
			)
		}
	}
}

// ---------------------------------------------------------------------------
// HTTP handlers
// ---------------------------------------------------------------------------

// handleStaffStream serves GET /api/v1/queue/{queueID}/stream for staff clients.
func (s *Server) handleStaffStream(w http.ResponseWriter, r *http.Request) {
	queueIDStr := r.PathValue("queueID")
	queueID, err := uuid.Parse(queueIDStr)
	if err != nil {
		http.Error(w, "invalid queueID", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := s.sseHub.subscribeStaff(queueID)
	defer s.sseHub.unsubscribeStaff(queueID, sub)

	// Send an immediate heartbeat so the client knows the stream is live.
	writeSSE(w, "heartbeat", map[string]string{"ts": time.Now().UTC().Format(time.RFC3339)})
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			writeSSE(w, "heartbeat", map[string]string{"ts": time.Now().UTC().Format(time.RFC3339)})
			flusher.Flush()
		case ev, ok := <-sub.ch:
			if !ok {
				return
			}
			writeSSE(w, ev.Event, ev.Data)
			flusher.Flush()
		}
	}
}

// handlePatientStream serves GET /api/v1/queue/{queueID}/entries/{entryID}/stream for patients.
func (s *Server) handlePatientStream(w http.ResponseWriter, r *http.Request) {
	entryIDStr := r.PathValue("entryID")
	entryID, err := uuid.Parse(entryIDStr)
	if err != nil {
		http.Error(w, "invalid entryID", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := s.sseHub.subscribePatient(entryID)
	defer s.sseHub.unsubscribePatient(entryID, sub)

	writeSSE(w, "heartbeat", map[string]string{"ts": time.Now().UTC().Format(time.RFC3339)})
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			writeSSE(w, "heartbeat", map[string]string{"ts": time.Now().UTC().Format(time.RFC3339)})
			flusher.Flush()
		case ev, ok := <-sub.ch:
			if !ok {
				return
			}
			writeSSE(w, ev.Event, ev.Data)
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}
