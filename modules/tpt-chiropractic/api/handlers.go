package api

import (
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
	chiAcc "github.com/PhillipC05/tpt-healthcare/modules/tpt-chiropractic/internal/acc"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-chiropractic/internal/spine"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-chiropractic/internal/xray"
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

// ---- Spine Handlers ----

func (s *Server) handleGetSpine(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, &spine.SpinalChart{PatientNHI: patientNHI})
}

func (s *Server) handleSaveSpine(w http.ResponseWriter, r *http.Request) {
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var chart spine.SpinalChart
	if err := decodeJSON(r, &chart); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	chart.PatientNHI = patientNHI
	chart.ClinicianID = principal.PractitionerID
	now := time.Now().UnixMilli()
	chart.UpdatedAt = now
	if chart.CreatedAt == 0 {
		chart.CreatedAt = now
	}
	s.recordEvent(r, principal, "update", "SpinalChart", patientNHI, patientNHI)
	s.logger.Info("spinal chart saved", slog.String("patient_nhi", hashNHI(patientNHI)))
	writeJSON(w, http.StatusOK, chart)
}

func (s *Server) handleGetVertebra(w http.ResponseWriter, r *http.Request) {
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
	seg := r.PathValue("segment")
	if seg == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_SEGMENT", Message: "vertebra segment is required"})
		return
	}
	writeJSON(w, http.StatusOK, spine.VertebraEntry{Segment: seg})
}

func (s *Server) handleUpdateVertebra(w http.ResponseWriter, r *http.Request) {
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
	seg := r.PathValue("segment")
	if seg == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_SEGMENT", Message: "vertebra segment is required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var entry spine.VertebraEntry
	if err := decodeJSON(r, &entry); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	entry.Segment = seg
	entry.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "VertebraEntry", seg, patientNHI)
	writeJSON(w, http.StatusOK, entry)
}

// ---- ACC Handlers ----

func (s *Server) handleListClaims(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, []chiAcc.Claim{})
}

func (s *Server) handleCreateClaim(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var claim chiAcc.Claim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(claim.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	claim.ID = uuid.New().String()
	claim.ProviderHPI = principal.PractitionerID
	claim.Status = chiAcc.StatusDraft
	claim.CreatedAt = time.Now().UTC()
	claim.UpdatedAt = claim.CreatedAt
	s.recordEvent(r, principal, "create", "ACCClaim", claim.ID, claim.PatientNHI)
	writeJSON(w, http.StatusCreated, claim)
}

func (s *Server) handleGetClaim(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleSubmitClaim(w http.ResponseWriter, r *http.Request) {
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
	var internalClaim chiAcc.Claim
	if err := decodeJSON(r, &internalClaim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: "expected claim body for submission"})
		return
	}
	if !nhi.ValidateNHI(internalClaim.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	if s.accClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "ACC_NOT_CONFIGURED", Message: "ACC integration is not configured"})
		return
	}
	coreClaim := coreAcc.Claim{
		PatientNHI:        strings.ToUpper(internalClaim.PatientNHI),
		ProviderHPI:       principal.PractitionerID,
		DateOfAccident:    internalClaim.AccidentDate,
		InjuryDescription: internalClaim.AccidentDesc,
		DiagnosisCodes:    []string{internalClaim.Diagnosis},
	}
	lodged, err := s.accClient.Lodge(r.Context(), coreClaim)
	if err != nil {
		s.logger.Error("ACC claim lodge failed", slog.String("claim_id", claimID), slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "ACC_LODGE_FAILED", Message: fmt.Sprintf("ACC lodgement failed: %v", err)})
		return
	}
	s.recordEvent(r, principal, "create", "ACCSubmission", claimID, internalClaim.PatientNHI)

	// Request the initial funded PO allocation for chiropractic sessions.
	resp := map[string]any{
		"status":      "submitted",
		"claimId":     claimID,
		"accClaimRef": lodged.ClaimNumber,
		"accStatus":   string(lodged.Status),
	}
	cap, capErr := coreAcc.SessionCapFor(coreAcc.DisciplineChiropractic)
	if capErr == nil {
		sessionsToRequest := cap.InitialGranted
		if sessionsToRequest == 0 {
			sessionsToRequest = cap.MaxWithExtension
		}
		po, poErr := s.accClient.RequestPO(r.Context(), coreAcc.PORequest{
			ClaimNumber:       lodged.ClaimNumber,
			PatientNHI:        strings.ToUpper(internalClaim.PatientNHI),
			ProviderHPI:       principal.PractitionerID,
			Discipline:        coreAcc.DisciplineChiropractic,
			SessionsRequested: sessionsToRequest,
		})
		if poErr != nil {
			s.logger.Warn("ACC PO request failed after claim lodge", slog.String("claim_id", claimID), slog.Any("error", poErr))
		} else {
			resp["poNumber"] = po.PONumber
			resp["poStatus"] = string(po.Status)
			resp["sessionsApproved"] = po.SessionsApproved
			if po.Status == coreAcc.POApproved {
				consumed, consumeErr := s.accClient.ConsumeSession(r.Context(), po)
				if consumeErr != nil {
					s.logger.Warn("ACC session consume failed", slog.String("po_number", po.PONumber), slog.Any("error", consumeErr))
				} else {
					resp["sessionsRemaining"] = consumed.SessionsRemaining()
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetClaimPO(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	claimID := r.PathValue("claimId")
	if claimID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLAIM_ID", Message: "claim ID is required"})
		return
	}
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("no purchase order found for claim %s", claimID)})
}

func (s *Server) handleRequestPOExtension(w http.ResponseWriter, r *http.Request) {
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
	if s.accClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "ACC_NOT_CONFIGURED", Message: "ACC integration is not configured"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		AccClaimRef           string `json:"accClaimRef"`
		PatientNHI            string `json:"patientNhi"`
		SessionsRequested     int    `json:"sessionsRequested"`
		ClinicalJustification string `json:"clinicalJustification"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(req.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	cap, _ := coreAcc.SessionCapFor(coreAcc.DisciplineChiropractic)
	if req.SessionsRequested <= 0 {
		req.SessionsRequested = cap.MaxWithExtension
	}
	po, err := s.accClient.RequestPO(r.Context(), coreAcc.PORequest{
		ClaimNumber:           req.AccClaimRef,
		PatientNHI:            strings.ToUpper(req.PatientNHI),
		ProviderHPI:           principal.PractitionerID,
		Discipline:            coreAcc.DisciplineChiropractic,
		SessionsRequested:     req.SessionsRequested,
		ClinicalJustification: req.ClinicalJustification,
	})
	if err != nil {
		s.logger.Error("ACC PO extension request failed", slog.String("claim_id", claimID), slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "PO_REQUEST_FAILED", Message: fmt.Sprintf("PO extension request failed: %v", err)})
		return
	}
	s.recordEvent(r, principal, "create", "ACCPurchaseOrder", po.ID.String(), req.PatientNHI)
	writeJSON(w, http.StatusCreated, po)
}

// ---- X-Ray Referral Handlers ----

func (s *Server) handleListXRayReferrals(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, []xray.Referral{})
}

func (s *Server) handleCreateXRayReferral(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var ref xray.Referral
	if err := decodeJSON(r, &ref); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(ref.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	ref.ID = uuid.New().String()
	ref.ClinicianID = principal.PractitionerID
	ref.CreatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "create", "XRayReferral", ref.ID, ref.PatientNHI)
	writeJSON(w, http.StatusCreated, ref)
}

func (s *Server) handleGetXRayReferral(w http.ResponseWriter, r *http.Request) {
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
	refID := r.PathValue("referralId")
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("referral %s not found", refID)})
}
