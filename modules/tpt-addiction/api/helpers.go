package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
)

func (h *handlerDeps) encryptNHI(nhi string) (string, error) {
	if nhi == "" {
		return "", nil
	}
	return h.enc.EncryptString(nhi)
}

func (h *handlerDeps) decryptNHI(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	return h.enc.DecryptString(enc)
}

// validateHPI checks the given HPI CPN has a valid APC.
// Writes the error response and returns false on failure.
func (h *handlerDeps) validateHPI(w http.ResponseWriter, r *http.Request, cpn string) bool {
	if cpn == "" {
		return true
	}
	apcStatus, err := h.hpiClient.ValidateAPC(r.Context(), cpn)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "HPI_ERROR", Message: "HPI validation unavailable"})
		return false
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusForbidden, apiError{Code: "APC_INVALID", Message: "practitioner APC is invalid or expired"})
		return false
	}
	return true
}

// recordAudit emits a synchronous audit event per HIPC Rules 10/11.
// patientNHIEnc must be the already-encrypted NHI value (or empty string).
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
		h.logger.Error("audit trail write failed",
			"error", err, "action", action,
			"resource_type", resourceType, "resource_id", resourceID)
	}
}

// tenantFromRequest extracts the tenant UUID or writes a 401 and returns false.
func tenantFromRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
	}
	return id, ok
}
