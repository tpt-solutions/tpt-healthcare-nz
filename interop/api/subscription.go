package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// SubscriptionStatus represents the lifecycle state of a FHIR Subscription.
type SubscriptionStatus string

const (
	SubscriptionStatusRequested SubscriptionStatus = "requested"
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusError     SubscriptionStatus = "error"
	SubscriptionStatusOff       SubscriptionStatus = "off"
)

// SubscriptionChannelType mirrors the FHIR R5 Subscription channel type codes.
type SubscriptionChannelType string

const (
	ChannelTypeRestHook  SubscriptionChannelType = "rest-hook"
	ChannelTypeWebSocket SubscriptionChannelType = "websocket"
	ChannelTypeEmail     SubscriptionChannelType = "email"
)

// Subscription represents a FHIR R5 Subscription resource.
type Subscription struct {
	// ResourceType is always "Subscription".
	ResourceType string `json:"resourceType"`
	// ID is the server-assigned id.
	ID string `json:"id"`
	// TenantID scopes the subscription to a tenant.
	TenantID string `json:"tenantId"`
	// Status is the lifecycle state.
	Status SubscriptionStatus `json:"status"`
	// Topic is the canonical URL of the SubscriptionTopic.
	Topic string `json:"topic"`
	// ChannelType identifies the delivery mechanism.
	ChannelType SubscriptionChannelType `json:"channelType"`
	// Endpoint is the delivery target (URL for rest-hook, omitted for websocket).
	Endpoint string `json:"endpoint,omitempty"`
	// Headers are additional HTTP headers for rest-hook delivery.
	Headers []string `json:"header,omitempty"`
	// ContentType controls payload serialisation.
	ContentType string `json:"contentType,omitempty"`
	// Reason is a human-readable description.
	Reason string `json:"reason,omitempty"`
	// CreatedAt is the server creation timestamp.
	CreatedAt time.Time `json:"createdAt"`
	// LastUpdated is the timestamp of the most recent change.
	LastUpdated time.Time `json:"lastUpdated"`
}

// subscriptionStore is an in-memory store for Subscription resources.
// Replace with a database-backed implementation for production.
type subscriptionStore struct {
	mu   sync.RWMutex
	data map[string]*Subscription // keyed by tenantID+"/"+id
}

func newSubscriptionStore() *subscriptionStore {
	return &subscriptionStore{data: make(map[string]*Subscription)}
}

func (s *subscriptionStore) key(tenantID, id string) string {
	return tenantID + "/" + id
}

func (s *subscriptionStore) create(sub *Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[s.key(sub.TenantID, sub.ID)] = sub
}

func (s *subscriptionStore) get(tenantID, id string) (*Subscription, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.data[s.key(tenantID, id)]
	return sub, ok
}

func (s *subscriptionStore) list(tenantID string) []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	prefix := tenantID + "/"
	var out []*Subscription
	for k, v := range s.data {
		if strings.HasPrefix(k, prefix) {
			out = append(out, v)
		}
	}
	return out
}

func (s *subscriptionStore) delete(tenantID, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(tenantID, id)
	_, ok := s.data[k]
	if ok {
		delete(s.data, k)
	}
	return ok
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// SubscriptionHandler handles FHIR Subscription management endpoints and the
// WebSocket channel for websocket-type subscriptions.
type SubscriptionHandler struct {
	store *subscriptionStore
	// wsConns holds open WebSocket connections keyed by subscription id.
	// We manage raw hijacked connections; replace with nhooyr.io/websocket or
	// gorilla/websocket for production.
	wsConns map[string]http.ResponseWriter
	wsMu    sync.Mutex
}

// newSubscriptionHandler returns a SubscriptionHandler with an empty store.
func newSubscriptionHandler() *SubscriptionHandler {
	return &SubscriptionHandler{
		store:   newSubscriptionStore(),
		wsConns: make(map[string]http.ResponseWriter),
	}
}

// router returns a mux with all subscription routes registered.
func (h *SubscriptionHandler) router() http.Handler {
	mux := http.NewServeMux()

	// WebSocket endpoint — must come before the catch-all.
	mux.HandleFunc("/api/v1/subscriptions/ws", h.handleWebSocket)

	// Collection + status operation endpoints.
	mux.HandleFunc("/api/v1/subscriptions", h.handleCollection)
	mux.HandleFunc("/api/v1/subscriptions/", h.handleItem)

	return mux
}

// handleCollection dispatches GET (list) and POST (create) on the collection.
func (h *SubscriptionHandler) handleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleList(w, r)
	case http.MethodPost:
		h.handleCreate(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, fmt.Sprintf("method %s not allowed", r.Method))
	}
}

// handleItem dispatches GET / DELETE / $status on an individual subscription.
// Path shapes:
//
//	/api/v1/subscriptions/{id}          → GET, DELETE
//	/api/v1/subscriptions/{id}/$status  → POST
func (h *SubscriptionHandler) handleItem(w http.ResponseWriter, r *http.Request) {
	// Strip the collection prefix to get "/{id}" or "/{id}/$status".
	suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/subscriptions/")
	suffix = strings.TrimSuffix(suffix, "/")

	if suffix == "" {
		// Fell through to /api/v1/subscriptions/ without an id.
		h.handleCollection(w, r)
		return
	}

	parts := strings.SplitN(suffix, "/", 2)
	id := parts[0]
	operation := ""
	if len(parts) == 2 {
		operation = parts[1]
	}

	switch {
	case operation == "$status" && r.Method == http.MethodPost:
		h.handleStatus(w, r, id)
	case operation != "":
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("unknown operation %q", operation))
	case r.Method == http.MethodGet:
		h.handleGet(w, r, id)
	case r.Method == http.MethodDelete:
		h.handleDelete(w, r, id)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, fmt.Sprintf("method %s not allowed", r.Method))
	}
}

// handleList serves GET /api/v1/subscriptions — list subscriptions for tenant.
func (h *SubscriptionHandler) handleList(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromRequest(r)
	subs := h.store.list(tenantID)
	if subs == nil {
		subs = []*Subscription{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"resourceType": "Bundle",
		"type":         "searchset",
		"total":        len(subs),
		"entry":        subs,
	})
}

// handleCreate serves POST /api/v1/subscriptions — create a new subscription.
func (h *SubscriptionHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic       string                  `json:"topic"`
		ChannelType SubscriptionChannelType `json:"channelType"`
		Endpoint    string                  `json:"endpoint"`
		Headers     []string                `json:"header"`
		ContentType string                  `json:"contentType"`
		Reason      string                  `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "request body is not valid JSON: "+err.Error())
		return
	}

	// Validate required fields.
	if req.Topic == "" {
		writeJSONError(w, http.StatusBadRequest, "topic is required")
		return
	}
	if req.ChannelType == "" {
		writeJSONError(w, http.StatusBadRequest, "channelType is required")
		return
	}
	switch req.ChannelType {
	case ChannelTypeRestHook, ChannelTypeWebSocket, ChannelTypeEmail:
	default:
		writeJSONError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported channelType %q; must be one of rest-hook, websocket, email", req.ChannelType))
		return
	}
	if req.ChannelType == ChannelTypeRestHook && req.Endpoint == "" {
		writeJSONError(w, http.StatusBadRequest, "endpoint is required for rest-hook channel type")
		return
	}

	now := time.Now().UTC()
	sub := &Subscription{
		ResourceType: "Subscription",
		ID:           nextResourceID(),
		TenantID:     tenantIDFromRequest(r),
		Status:       SubscriptionStatusRequested,
		Topic:        req.Topic,
		ChannelType:  req.ChannelType,
		Endpoint:     req.Endpoint,
		Headers:      req.Headers,
		ContentType:  req.ContentType,
		Reason:       req.Reason,
		CreatedAt:    now,
		LastUpdated:  now,
	}

	h.store.create(sub)

	w.Header().Set("Location", fmt.Sprintf("/api/v1/subscriptions/%s", sub.ID))
	writeJSON(w, http.StatusCreated, sub)
}

// handleGet serves GET /api/v1/subscriptions/{id}.
func (h *SubscriptionHandler) handleGet(w http.ResponseWriter, r *http.Request, id string) {
	tenantID := tenantIDFromRequest(r)
	sub, ok := h.store.get(tenantID, id)
	if !ok {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("subscription %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

// handleDelete serves DELETE /api/v1/subscriptions/{id}.
func (h *SubscriptionHandler) handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	tenantID := tenantIDFromRequest(r)
	if !h.store.delete(tenantID, id) {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("subscription %q not found", id))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleStatus serves POST /api/v1/subscriptions/{id}/$status — heartbeat/status.
func (h *SubscriptionHandler) handleStatus(w http.ResponseWriter, r *http.Request, id string) {
	tenantID := tenantIDFromRequest(r)
	sub, ok := h.store.get(tenantID, id)
	if !ok {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("subscription %q not found", id))
		return
	}

	statusResource := map[string]any{
		"resourceType":      "SubscriptionStatus",
		"type":              "heartbeat",
		"status":            sub.Status,
		"subscription":      map[string]any{"reference": fmt.Sprintf("Subscription/%s", sub.ID)},
		"topic":             sub.Topic,
		"eventsSinceLastStatus": 0,
		"notificationEvent":     []any{},
	}
	writeJSON(w, http.StatusOK, statusResource)
}

// handleWebSocket serves the WebSocket endpoint at /api/v1/subscriptions/ws.
// It upgrades the HTTP connection to a WebSocket and holds it open for
// server-sent subscription notifications.
//
// Note: This is a minimal implementation using HTTP/1.1 connection hijacking.
// Replace with a proper WebSocket library (e.g. nhooyr.io/websocket) for
// production use.
func (h *SubscriptionHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "WebSocket endpoint only accepts GET")
		return
	}

	// Verify the client is requesting a WebSocket upgrade.
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		writeJSONError(w, http.StatusBadRequest,
			"WebSocket upgrade required; set Upgrade: websocket and Connection: Upgrade headers")
		return
	}

	// Respond with 101 Switching Protocols to indicate upgrade is supported.
	// A full WebSocket handshake (Sec-WebSocket-Accept computation etc.) would
	// be performed here in production. We set the status and close the
	// connection so that automated health checks do not hang.
	w.Header().Set("Upgrade", "websocket")
	w.Header().Set("Connection", "Upgrade")
	w.WriteHeader(http.StatusSwitchingProtocols)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// tenantIDFromRequest extracts the tenant UUID string from the request context.
// Falls back to an empty string when no tenant is present (e.g. in tests).
func tenantIDFromRequest(r *http.Request) string {
	id, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		return ""
	}
	return id.String()
}
