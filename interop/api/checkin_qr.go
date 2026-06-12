package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// QRPayload is the data encoded in a clinic check-in QR code.
// It contains only non-sensitive identifiers; the patient proves their identity
// via their patient portal session when they scan the code.
type QRPayload struct {
	TenantID  string    `json:"t"`
	QueueID   string    `json:"q"`
	ExpiresAt time.Time `json:"e"`
}

// GenerateCheckInQR handles GET /api/v1/queues/{queueId}/checkin-qr.
// Returns a base64-encoded JSON payload suitable for rendering as a QR code on
// the practice's reception screen or on appointment confirmation emails.
// The payload is valid for 24 hours and encodes only the tenant + queue IDs —
// no patient information is included.
//
// Frontend: pass the returned `payload` string to a QR library such as qrcode.js
// or react-qr-code. The patient portal deep-link is:
//   tpt-portal://checkin?data=<base64payload>
// or equivalently the web URL:
//   https://<portal>/checkin?data=<base64payload>
func (s *Server) GenerateCheckInQR(w http.ResponseWriter, r *http.Request) {
	queueID := idFromPath(r, "queueId")
	if queueID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_QUEUE_ID", Message: "queueId is required"})
		return
	}

	tenantID, ok := tenantFromCtx(r.Context())
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	payload := QRPayload{
		TenantID:  tenantID,
		QueueID:   queueID,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "QR_ERROR", Message: "failed to encode QR payload"})
		return
	}

	encoded := base64.URLEncoding.EncodeToString(raw)
	portalURL := fmt.Sprintf("/checkin?data=%s", encoded)

	writeJSON(w, http.StatusOK, map[string]any{
		"payload":   encoded,
		"portalUrl": portalURL,
		"expiresAt": payload.ExpiresAt,
		"instructions": "Display this as a QR code at reception or embed in appointment confirmation emails. " +
			"Patients scan with their phone camera to check in via the portal.",
	})
}

// RedeemCheckInQR handles POST /api/v1/queues/checkin-qr/redeem.
// Called by the patient portal after the patient scans a QR code.
// Validates the payload and performs check-in using the patient's authenticated identity.
func (s *Server) RedeemCheckInQR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Data      string `json:"data"`      // base64-encoded QRPayload
		PatientNHI string `json:"patientNhi"` // encrypted NHI from session
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	raw, err := base64.URLEncoding.DecodeString(req.Data)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_QR", Message: "invalid QR payload"})
		return
	}

	var payload QRPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_QR", Message: "malformed QR payload"})
		return
	}

	if time.Now().UTC().After(payload.ExpiresAt) {
		writeJSON(w, http.StatusGone, apiError{Code: "QR_EXPIRED", Message: "this QR code has expired; please ask staff for a new one"})
		return
	}

	queueID, err := uuid.Parse(payload.QueueID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_QUEUE_ID", Message: "invalid queue ID in QR payload"})
		return
	}

	// Delegate to the existing queue check-in handler logic.
	s.checkInPatient(w, r, queueID, req.PatientNHI, "portal-qr")
}
