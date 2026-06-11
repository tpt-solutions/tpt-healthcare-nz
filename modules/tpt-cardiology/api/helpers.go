package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// encryptNHI encrypts a plaintext NHI for storage. Empty string is returned unchanged.
func (h *handlerDeps) encryptNHI(nhi string) (string, error) {
	if nhi == "" {
		return "", nil
	}
	return h.enc.EncryptString(nhi)
}

// decryptNHI decrypts a stored NHI value. Empty string is returned unchanged.
func (h *handlerDeps) decryptNHI(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	return h.enc.DecryptString(enc)
}

// validateHPI checks that the given HPI CPN holds a valid APC.
// Writes the error response and returns false when validation fails.
func (h *handlerDeps) validateHPI(w http.ResponseWriter, r *http.Request, cpn string) bool {
	if cpn == "" {
		return true
	}
	valid, err := h.hpiClient.ValidateAPC(r.Context(), cpn)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "HPI_ERROR", Message: "HPI validation unavailable"})
		return false
	}
	if !valid {
		writeJSON(w, http.StatusForbidden, apiError{Code: "APC_INVALID", Message: "practitioner APC is invalid or expired"})
		return false
	}
	return true
}

// recordAudit emits a synchronous audit event per HIPC Rules 10/11.
// patientNHIEnc must be the already-encrypted NHI value (or empty string).
// TODO: wrap inside the DB transaction that performs the clinical write for atomicity.
func (h *handlerDeps) recordAudit(r *http.Request, action, resourceType, resourceID, patientNHIEnc string) {
	tenantID, _ := middleware.TenantFromContext(r.Context())
	principal, _ := auth.PrincipalFromContext(r.Context())
	ev := audit.Event{
		TenantID:     tenantID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		PatientNHI:   patientNHIEnc,
		IPAddress:    r.RemoteAddr,
		UserAgent:    r.Header.Get("User-Agent"),
		OccurredAt:   time.Now().UTC(),
	}
	if principal != nil {
		ev.PrincipalID = principal.ID
	}
	if err := h.auditTrail.Record(r.Context(), ev); err != nil {
		h.logger.Error("audit trail write failed", "error", err, "action", action, "resource_type", resourceType, "resource_id", resourceID)
	}
}
