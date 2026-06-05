package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/nhi"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-counselling/internal/eap"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-counselling/internal/private"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-counselling/internal/session"
	"github.com/google/uuid"
)

func hashNHI(n string) string {
	h := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(n))))
	return "sha256:" + hex.EncodeToString(h[:8])
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (*auth.Principal, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHORIZED", Message: "no authenticated principal"})
		return nil, false
	}
	return p, true
}

func validateNHIParam(w http.ResponseWriter, r *http.Request, param string) (string, bool) {
	v := r.PathValue(param)
	if v == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "patient NHI is required"})
		return "", false
	}
	if !nhi.ValidateNHI(v) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "NHI format is invalid"})
		return "", false
	}
	return strings.ToUpper(strings.TrimSpace(v)), true
}

func (s *Server) checkAPC(w http.ResponseWriter, r *http.Request, principal *auth.Principal) bool {
	if !principal.Practitioner || principal.PractitionerID == "" {
		writeJSON(w, http.StatusForbidden, apiError{Code: "NOT_PRACTITIONER", Message: "clinical actions require a registered practitioner identity"})
		return false
	}
	valid, err := s.hpiClient.ValidateAPC(r.Context(), principal.PractitionerID)
	if err != nil {
		s.logger.Error("HPI APC validation failed", slog.String("cpn", principal.PractitionerID), slog.Any("error", err))
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "HPI_UNAVAILABLE", Message: "unable to verify practitioner APC"})
		return false
	}
	if !valid {
		writeJSON(w, http.StatusForbidden, apiError{Code: "APC_INVALID", Message: "practitioner does not hold a current Annual Practising Certificate"})
		return false
	}
	return true
}

// checkMentalHealthConsent enforces the elevated consent check for mental health (counselling) records.
// Mental health session notes are extra-sensitive under HIPC and require explicit separate consent.
func (s *Server) checkMentalHealthConsent(w http.ResponseWriter, r *http.Request, patientNHI string) bool {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant context required"})
		return false
	}
	// Counselling session notes are mental health records — use disclosure consent type
	// which captures the heightened sensitivity under HIPC Rule 11.
	granted, err := s.consentStore.Check(r.Context(), tenantID, patientNHI, consent.ConsentTypeDisclosure)
	if err != nil {
		s.logger.Error("mental health consent check failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CONSENT_CHECK_ERROR", Message: "unable to verify patient consent"})
		return false
	}
	if !granted {
		writeJSON(w, http.StatusForbidden, apiError{Code: "MENTAL_HEALTH_CONSENT_REQUIRED", Message: "patient has not granted disclosure consent for mental health records"})
		return false
	}
	return true
}

func (s *Server) checkConsent(w http.ResponseWriter, r *http.Request, patientNHI string) bool {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant context required"})
		return false
	}
	granted, err := s.consentStore.Check(r.Context(), tenantID, patientNHI, consent.ConsentTypeAccess)
	if err != nil {
		s.logger.Error("consent check failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CONSENT_CHECK_ERROR", Message: "unable to verify patient consent"})
		return false
	}
	if !granted {
		writeJSON(w, http.StatusForbidden, apiError{Code: "CONSENT_REQUIRED", Message: "patient has not granted access consent"})
		return false
	}
	return true
}

func (s *Server) recordEvent(r *http.Request, principal *auth.Principal, action, resourceType, resourceID, patientNHI string) {
	tenantID, _ := middleware.TenantFromContext(r.Context())
	if err := s.auditTrail.Record(r.Context(), audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		PatientNHI:   hashNHI(patientNHI),
		IPAddress:    r.RemoteAddr,
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		s.logger.Error("audit record failed", slog.String("resource_type", resourceType), slog.Any("error", err))
	}
}

// ---- EAP Handlers ----

func (s *Server) handleEAPListClaims(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, []eap.Claim{})
}

func (s *Server) handleEAPCreateClaim(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var claim eap.Claim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(claim.ClientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "clientNhi format is invalid"})
		return
	}
	claim.ID = uuid.New().String()
	claim.CounsellorID = principal.PractitionerID
	now := time.Now().UnixMilli()
	claim.CreatedAt = now
	claim.UpdatedAt = now
	s.recordEvent(r, principal, "create", "EAPClaim", claim.ID, claim.ClientNHI)
	writeJSON(w, http.StatusCreated, claim)
}

func (s *Server) handleEAPGetClaim(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	claimID := r.PathValue("claimId")
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("EAP claim %s not found", claimID)})
}

func (s *Server) handleEAPUpdateClaim(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	claimID := r.PathValue("claimId")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var claim eap.Claim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	claim.ID = claimID
	claim.CounsellorID = principal.PractitionerID
	claim.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "EAPClaim", claimID, claim.ClientNHI)
	writeJSON(w, http.StatusOK, claim)
}

// ---- Session Note Handlers — mental health records, elevated consent required ----

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	// Mental health records require the elevated disclosure consent check.
	if !s.checkMentalHealthConsent(w, r, patientNHI) {
		return
	}
	writeJSON(w, http.StatusOK, []session.Session{})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var sess session.Session
	if err := decodeJSON(r, &sess); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(sess.ClientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "clientNhi format is invalid"})
		return
	}
	sess.ID = uuid.New().String()
	sess.ClinicianID = principal.PractitionerID
	now := time.Now().UnixMilli()
	sess.CreatedAt = now
	sess.UpdatedAt = now
	s.recordEvent(r, principal, "create", "CounsellingSession", sess.ID, sess.ClientNHI)
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	if !s.checkMentalHealthConsent(w, r, patientNHI) {
		return
	}
	sessionID := r.PathValue("sessionId")
	s.recordEvent(r, principal, "read", "CounsellingSession", sessionID, patientNHI)
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("session %s not found", sessionID)})
}

func (s *Server) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	sessionID := r.PathValue("sessionId")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var sess session.Session
	if err := decodeJSON(r, &sess); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	sess.ID = sessionID
	sess.ClientNHI = patientNHI
	sess.ClinicianID = principal.PractitionerID
	sess.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "CounsellingSession", sessionID, patientNHI)
	writeJSON(w, http.StatusOK, sess)
}

// ---- Private Practice Handlers ----

func (s *Server) handlePrivateListClients(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, []private.PrivateClientResponse{})
}

func (s *Server) handlePrivateCreateClient(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req private.PrivateClientRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if req.NHI != "" && !nhi.ValidateNHI(req.NHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "NHI format is invalid"})
		return
	}
	// PHI fields (Name, Email, Phone, NHI) are encrypted before persistence.
	client, err := private.NewEncryptedClient(req, s.enc)
	if err != nil {
		s.logger.Error("PHI encryption failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPTION_ERROR", Message: "failed to encrypt client record"})
		return
	}
	client.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	client.CreatedAt = now
	client.UpdatedAt = now
	s.recordEvent(r, principal, "create", "PrivateClient", client.ID, req.NHI)
	writeJSON(w, http.StatusCreated, client.ToResponse(s.enc))
}

func (s *Server) handlePrivateCreateInvoice(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var inv private.Invoice
	if err := decodeJSON(r, &inv); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	inv.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	inv.CreatedAt = now
	inv.UpdatedAt = now
	s.recordEvent(r, principal, "create", "PrivateInvoice", inv.ID, "")
	writeJSON(w, http.StatusCreated, inv)
}
