package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	"github.com/PhillipC05/tpt-healthcare/core/primhd"
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
	apcStatus, err := s.hpiClient.ValidateAPC(r.Context(), principal.PractitionerID)
	if err != nil {
		s.logger.Error("HPI APC validation failed", slog.String("cpn", principal.PractitionerID), slog.Any("error", err))
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "HPI_UNAVAILABLE", Message: "unable to verify practitioner APC"})
		return false
	}
	if !apcStatus.Valid {
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
	claims, err := s.listEAPClaims(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to list EAP claims"})
		return
	}
	writeJSON(w, http.StatusOK, claims)
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
	var claim eap.EAPClaim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(claim.ClientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "clientNhi format is invalid"})
		return
	}
	claim.ID = uuid.New().String()
	claim.ProviderHPI = principal.PractitionerID
	claim.CreatedAt = time.Now().UTC()
	claim.UpdatedAt = claim.CreatedAt
	result, err := s.createEAPClaim(r.Context(), claim)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to create EAP claim"})
		return
	}
	s.recordEvent(r, principal, "create", "EAPClaim", result.ID, result.ClientNHI)
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleEAPGetClaim(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	claimID := r.PathValue("claimId")
	claim, err := s.getEAPClaim(r.Context(), claimID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("EAP claim %s not found", claimID)})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to get EAP claim"})
		return
	}
	writeJSON(w, http.StatusOK, claim)
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
	var claim eap.EAPClaim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	claim.ID = claimID
	claim.ProviderHPI = principal.PractitionerID
	claim.UpdatedAt = time.Now().UTC()
	result, err := s.updateEAPClaim(r.Context(), claim)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("EAP claim %s not found", claimID)})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to update EAP claim"})
		return
	}
	s.recordEvent(r, principal, "update", "EAPClaim", claimID, result.ClientNHI)
	writeJSON(w, http.StatusOK, result)
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
	if !s.checkMentalHealthConsent(w, r, patientNHI) {
		return
	}
	sessions, err := s.listSessions(r.Context(), patientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to list sessions"})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

// sessionCreateRequest wraps session.Session with an optional PRIMHD referral ID.
// If PRIMHDReferralID is provided, a PRIMHD ActivityRecord is submitted to
// report this contact to the PRIMHD outcomes system.
type sessionCreateRequest struct {
	session.Session
	PRIMHDReferralID string `json:"primhdReferralId,omitempty"`
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
	var req sessionCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(req.ClientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "clientNhi format is invalid"})
		return
	}
	req.ID = uuid.New().String()
	req.ClinicianID = principal.PractitionerID
	now := time.Now().UnixMilli()
	req.CreatedAt = now
	req.UpdatedAt = now
	result, err := s.createSession(r.Context(), req.Session)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to create session"})
		return
	}
	s.recordEvent(r, principal, "create", "CounsellingSession", result.ID, result.ClientNHI)

	if s.primhdClient != nil && req.PRIMHDReferralID != "" {
		s.submitPRIMHDActivity(r, result, req.PRIMHDReferralID)
	}

	writeJSON(w, http.StatusCreated, result)
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
	sess, err := s.getSession(r.Context(), sessionID, patientNHI)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("session %s not found", sessionID)})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to get session"})
		return
	}
	s.recordEvent(r, principal, "read", "CounsellingSession", sessionID, patientNHI)
	writeJSON(w, http.StatusOK, sess)
}

// sessionUpdateRequest wraps session.Session with an optional PRIMHD referral ID.
// When PRIMHDReferralID is provided the session contact is reported to PRIMHD.
type sessionUpdateRequest struct {
	session.Session
	PRIMHDReferralID string `json:"primhdReferralId,omitempty"`
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
	var req sessionUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	req.ID = sessionID
	req.ClientNHI = patientNHI
	req.ClinicianID = principal.PractitionerID
	req.UpdatedAt = time.Now().UnixMilli()
	result, err := s.updateSession(r.Context(), req.Session)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("session %s not found", sessionID)})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to update session"})
		return
	}
	s.recordEvent(r, principal, "update", "CounsellingSession", sessionID, patientNHI)

	if s.primhdClient != nil && req.PRIMHDReferralID != "" {
		s.submitPRIMHDActivity(r, result, req.PRIMHDReferralID)
	}

	writeJSON(w, http.StatusOK, result)
}

// submitPRIMHDActivity reports a counselling session as a PRIMHD ActivityRecord.
// Failures are logged but do not fail the HTTP response.
func (s *Server) submitPRIMHDActivity(r *http.Request, sess session.Session, referralID string) {
	act := primhd.ActivityRecord{
		ReferralID:   referralID,
		ActivityType: sessionModality(sess.Modality),
		Duration:     sess.DurationMin,
		ContactDate:  time.UnixMilli(sess.SessionDate).UTC(),
		ClinicianHPI: sess.ClinicianID,
		Setting:      sessionMode(sess.Mode),
	}
	if _, err := s.primhdClient.SubmitActivity(r.Context(), act); err != nil {
		s.logger.Error("PRIMHD activity submission failed",
			slog.String("session", sess.ID),
			slog.String("referral_id", referralID),
			slog.Any("error", err),
		)
	} else {
		s.logger.Info("PRIMHD activity submitted",
			slog.String("session", sess.ID),
			slog.String("referral_id", referralID),
		)
	}
}

// sessionModality maps a counselling modality string to a PRIMHD activity type.
func sessionModality(modality string) string {
	switch strings.ToLower(modality) {
	case "cbt", "act", "dbt", "emdr", "psychodynamic", "person_centred":
		return "face-to-face"
	case "group":
		return "group"
	default:
		return "face-to-face"
	}
}

// sessionMode maps a session delivery mode to a PRIMHD setting value.
func sessionMode(mode string) string {
	switch strings.ToLower(mode) {
	case "video":
		return "telehealth"
	case "phone":
		return "phone"
	default:
		return "outpatient"
	}
}

// ---- Private Practice Handlers ----

func (s *Server) handlePrivateListClients(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	clients, err := s.listPrivateClients(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to list private clients"})
		return
	}
	responses := make([]private.PrivateClientResponse, 0, len(clients))
	for _, c := range clients {
		responses = append(responses, c.ToResponse(s.enc))
	}
	writeJSON(w, http.StatusOK, responses)
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
	result, err := s.createPrivateClient(r.Context(), *client)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to create private client"})
		return
	}
	s.recordEvent(r, principal, "create", "PrivateClient", result.ID, req.NHI)
	writeJSON(w, http.StatusCreated, result.ToResponse(s.enc))
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
	result, err := s.createPrivateInvoice(r.Context(), inv)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to create private invoice"})
		return
	}
	s.recordEvent(r, principal, "create", "PrivateInvoice", result.ID, "")
	writeJSON(w, http.StatusCreated, result)
}
