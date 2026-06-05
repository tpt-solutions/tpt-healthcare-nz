package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	coreAcc "github.com/PhillipC05/tpt-healthcare/core/acc"
	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/nhi"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-acupuncture/internal/acc"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-acupuncture/internal/needle"
	"github.com/google/uuid"
)

// hashNHI returns a short SHA-256 prefix safe for logging (not reversible to plaintext).
func hashNHI(n string) string {
	h := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(n))))
	return "sha256:" + hex.EncodeToString(h[:8])
}

// requirePrincipal extracts the authenticated principal from ctx; returns false + writes 401 if missing.
func requirePrincipal(w http.ResponseWriter, r *http.Request) (*auth.Principal, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHORIZED", Message: "no authenticated principal"})
		return nil, false
	}
	return p, true
}

// validateNHIParam extracts and validates an NHI from a path value; writes 400 if invalid.
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

// checkAPC validates the principal's practitioner APC via the HPI. Returns false + writes 403 if invalid.
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

// checkConsent verifies patient consent before returning health data. Returns false + writes 403 if not consented.
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

// recordEvent writes a per-resource audit event. Errors are logged but do not abort the response.
func (s *Server) recordEvent(r *http.Request, principal *auth.Principal, action, resourceType, resourceID, patientNHI string) {
	tenantID, _ := middleware.TenantFromContext(r.Context())
	err := s.auditTrail.Record(r.Context(), audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		PatientNHI:   hashNHI(patientNHI),
		IPAddress:    r.RemoteAddr,
		OccurredAt:   time.Now().UTC(),
	})
	if err != nil {
		s.logger.Error("audit record failed", slog.String("resource_type", resourceType), slog.Any("error", err))
	}
}

// ---- ACC Handlers ----

func (s *Server) handleAccListClaims(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, []acc.Claim{})
}

func (s *Server) handleAccCreateClaim(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var claim acc.Claim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	if !nhi.ValidateNHI(claim.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}

	claim.ID = uuid.New().String()
	claim.ProviderHPI = principal.PractitionerID
	claim.Status = acc.ClaimDraft
	claim.CreatedAt = time.Now().UTC()
	claim.UpdatedAt = claim.CreatedAt

	result := claim.Validate()
	if !result.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "VALIDATION_FAILED", Message: "claim failed validation", Details: result.Errors})
		return
	}

	s.recordEvent(r, principal, "create", "ACCClaim", claim.ID, claim.PatientNHI)
	s.logger.Info("ACC acupuncture claim created", slog.String("claim_id", claim.ID), slog.String("patient_nhi", hashNHI(claim.PatientNHI)))
	writeJSON(w, http.StatusCreated, claim)
}

func (s *Server) handleAccGetClaim(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	claimID := r.PathValue("claimId")
	if claimID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLAIM_ID", Message: "claim ID is required"})
		return
	}
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("claim %s not found", claimID)})
}

func (s *Server) handleAccUpdateClaim(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	claimID := r.PathValue("claimId")
	if claimID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLAIM_ID", Message: "claim ID is required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var claim acc.Claim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("invalid request body: %v", err)})
		return
	}
	claim.ID = claimID
	claim.ProviderHPI = principal.PractitionerID
	claim.UpdatedAt = time.Now().UTC()
	s.recordEvent(r, principal, "update", "ACCClaim", claimID, claim.PatientNHI)
	writeJSON(w, http.StatusOK, claim)
}

func (s *Server) handleAccSubmitClaim(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	claimID := r.PathValue("claimId")
	if claimID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLAIM_ID", Message: "claim ID is required"})
		return
	}

	// Build a core acc.Claim to lodge with ACC FHIR API.
	// In full implementation this would be fetched from DB by claimID.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var internalClaim acc.Claim
	if err := decodeJSON(r, &internalClaim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: "expected claim body for submission"})
		return
	}
	if !nhi.ValidateNHI(internalClaim.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}

	coreClaim := coreAcc.Claim{
		PatientNHI:        strings.ToUpper(internalClaim.PatientNHI),
		ProviderHPI:       principal.PractitionerID,
		DateOfAccident:    internalClaim.AccidentDate,
		InjuryDescription: internalClaim.AccidentDesc,
		DiagnosisCodes:    []string{internalClaim.Diagnosis},
	}

	if s.cfg.ACCBaseURL == "" {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "ACC_NOT_CONFIGURED", Message: "ACC integration is not configured"})
		return
	}

	// tokenFunc returns a SMART on FHIR bearer token for the ACC API.
	// A full implementation would perform a client_credentials OAuth2 flow here.
	accTokenFunc := func(_ context.Context) (string, error) {
		return "", fmt.Errorf("ACC SMART on FHIR token acquisition not yet implemented")
	}
	accClient := coreAcc.New(s.cfg.ACCBaseURL, accTokenFunc)
	lodged, err := accClient.Lodge(r.Context(), coreClaim)
	if err != nil {
		s.logger.Error("ACC claim lodge failed", slog.String("claim_id", claimID), slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "ACC_LODGE_FAILED", Message: fmt.Sprintf("ACC lodgement failed: %v", err)})
		return
	}

	s.recordEvent(r, principal, "create", "ACCSubmission", claimID, internalClaim.PatientNHI)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "submitted",
		"claimId":     claimID,
		"accClaimRef": lodged.ClaimNumber,
		"accStatus":   string(lodged.Status),
	})
}

// ---- Needle Site Handlers ----

func (s *Server) handleListNeedleSites(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	if !s.checkConsent(w, r, patientNHI) {
		return
	}
	writeJSON(w, http.StatusOK, []needle.NeedleSession{})
}

func (s *Server) handleCreateNeedleSite(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var session needle.NeedleSession
	if err := decodeJSON(r, &session); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("invalid request body: %v", err)})
		return
	}
	if !nhi.ValidateNHI(session.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	session.ID = uuid.New().String()
	session.ClinicianID = principal.PractitionerID
	session.CreatedAt = time.Now().UnixMilli()
	session.UpdatedAt = session.CreatedAt

	s.recordEvent(r, principal, "create", "NeedleSession", session.ID, session.PatientNHI)
	s.logger.Info("needle session created", slog.String("session_id", session.ID), slog.String("patient_nhi", hashNHI(session.PatientNHI)))
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) handleGetNeedleSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	if !s.checkConsent(w, r, patientNHI) {
		return
	}
	sessionID := r.PathValue("sessionId")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_SESSION_ID", Message: "session ID is required"})
		return
	}
	s.recordEvent(r, principal, "read", "NeedleSession", sessionID, patientNHI)
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "needle session not found"})
}

func (s *Server) handleUpdateNeedleSession(w http.ResponseWriter, r *http.Request) {
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
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_SESSION_ID", Message: "session ID is required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var session needle.NeedleSession
	if err := decodeJSON(r, &session); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("invalid request body: %v", err)})
		return
	}
	session.ID = sessionID
	session.PatientNHI = patientNHI
	session.ClinicianID = principal.PractitionerID
	session.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "NeedleSession", sessionID, patientNHI)
	writeJSON(w, http.StatusOK, session)
}

// ---- Treatment Handlers ----

func (s *Server) handleListTreatments(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	if !s.checkConsent(w, r, patientNHI) {
		return
	}
	writeJSON(w, http.StatusOK, []needle.TreatmentRecord{})
}

func (s *Server) handleCreateTreatment(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var rec needle.TreatmentRecord
	if err := decodeJSON(r, &rec); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("invalid request body: %v", err)})
		return
	}
	if !nhi.ValidateNHI(rec.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	rec.ID = uuid.New().String()
	rec.ClinicianID = principal.PractitionerID
	now := time.Now().UnixMilli()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	s.recordEvent(r, principal, "create", "TreatmentRecord", rec.ID, rec.PatientNHI)
	s.logger.Info("treatment record created", slog.String("patient_nhi", hashNHI(rec.PatientNHI)))
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleGetTreatment(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	if !s.checkConsent(w, r, patientNHI) {
		return
	}
	treatmentID := r.PathValue("treatmentId")
	s.recordEvent(r, principal, "read", "TreatmentRecord", treatmentID, patientNHI)
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "treatment not found"})
}

func (s *Server) handleUpdateTreatment(w http.ResponseWriter, r *http.Request) {
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
	treatmentID := r.PathValue("treatmentId")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var rec needle.TreatmentRecord
	if err := decodeJSON(r, &rec); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("invalid request body: %v", err)})
		return
	}
	rec.ID = treatmentID
	rec.PatientNHI = patientNHI
	rec.ClinicianID = principal.PractitionerID
	rec.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "TreatmentRecord", treatmentID, patientNHI)
	writeJSON(w, http.StatusOK, rec)
}
