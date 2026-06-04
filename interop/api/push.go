package api

import (
	"encoding/json"
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/push"
	"github.com/google/uuid"
)

// vapidPublicKey is set from Config at server startup.
var vapidPublicKeyGlobal string

// handleVAPIDKey returns the VAPID public key so browser clients can subscribe.
//
//	GET /api/v1/push/vapid-key
func (s *Server) handleVAPIDKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"publicKey": s.vapidPublicKey})
}

// handlePushSubscribe saves a browser push subscription for the authenticated patient.
//
//	POST /api/v1/push/subscribe
func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
		http.Error(w, "endpoint, keys.p256dh, and keys.auth are required", http.StatusBadRequest)
		return
	}

	patientID, err := uuid.Parse(principal.ID)
	if err != nil {
		http.Error(w, "invalid subject claim", http.StatusUnauthorized)
		return
	}

	if err := s.pushStore.Upsert(r.Context(), push.Subscription{
		PatientID: patientID,
		TenantID:  principal.TenantID,
		Endpoint:  body.Endpoint,
		P256dh:    body.Keys.P256dh,
		Auth:      body.Keys.Auth,
	}); err != nil {
		http.Error(w, "failed to save subscription", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handlePushUnsubscribe removes a push subscription for the authenticated patient.
//
//	DELETE /api/v1/push/subscribe
func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	patientID, err := uuid.Parse(principal.ID)
	if err != nil {
		http.Error(w, "invalid subject claim", http.StatusUnauthorized)
		return
	}

	if err := s.pushStore.Delete(r.Context(), patientID, body.Endpoint); err != nil {
		http.Error(w, "failed to remove subscription", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
