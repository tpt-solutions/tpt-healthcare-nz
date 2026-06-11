package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/acc"
	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/worksafe"
)

// ClaimDestination selects which regulatory scheme receives the submitted claim.
type ClaimDestination string

const (
	DestinationACC      ClaimDestination = "acc"
	DestinationWorkSafe ClaimDestination = "worksafe"
)

// ClaimStatus mirrors the ACC/WorkSafe claim lifecycle.
type ClaimStatus string

const (
	ClaimStatusDraft     ClaimStatus = "draft"
	ClaimStatusSubmitted ClaimStatus = "submitted"
	ClaimStatusAccepted  ClaimStatus = "accepted"
	ClaimStatusRejected  ClaimStatus = "rejected"
	ClaimStatusPaid      ClaimStatus = "paid"
	ClaimStatusCancelled ClaimStatus = "cancelled"
	ClaimStatusPending   ClaimStatus = "pending"
)

// ACCFormType enumerates the supported ACC claim form types.
type ACCFormType string

const (
	ACCFormACC45 ACCFormType = "ACC45" // Injury claims
	ACCFormACC6  ACCFormType = "ACC6"  // Treatment injury
)

// Claim is the domain model for a claim generated from a clinical encounter.
// Destination controls whether the claim routes to ACC or WorkSafe NZ.
type Claim struct {
	ID                string           `json:"id"`
	EncounterID       string           `json:"encounterId"`
	PatientID         string           `json:"patientId"`
	PatientNHI        string           `json:"patientNhi"`
	PractitionerHPI   string           `json:"practitionerHpi"`
	FormType          ACCFormType      `json:"formType"`
	FormNumber        string           `json:"formNumber,omitempty"`
	DiagnosisCodes    []string         `json:"diagnosisCodes"`
	InjuryDate        time.Time        `json:"injuryDate"`
	InjuryDescription string           `json:"injuryDescription"`
	Status            ClaimStatus      `json:"status"`
	Destination       ClaimDestination `json:"destination"`
	ACCClaimNumber    string           `json:"accClaimNumber,omitempty"`
	WorkSafeRefNumber string           `json:"workSafeRefNumber,omitempty"`
	EmployerNZBN      string           `json:"employerNzbn,omitempty"`
	InjuryMechanism   string           `json:"injuryMechanism,omitempty"`
	RejectionReason   string           `json:"rejectionReason,omitempty"`
	PaidAmount        *float64         `json:"paidAmount,omitempty"`
	TenantID          string           `json:"tenantId"`
	CreatedAt         time.Time        `json:"createdAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
	SubmittedAt       *time.Time       `json:"submittedAt,omitempty"`
}

// claimCreateRequest is the body for POST /api/v1/claims.
type claimCreateRequest struct {
	EncounterID       string           `json:"encounterId"`
	PatientID         string           `json:"patientId"`
	PatientNHI        string           `json:"patientNhi"`
	PractitionerHPI   string           `json:"practitionerHpi"`
	FormType          ACCFormType      `json:"formType"`
	DiagnosisCodes    []string         `json:"diagnosisCodes"`
	InjuryDate        time.Time        `json:"injuryDate"`
	InjuryDescription string           `json:"injuryDescription"`
	// Destination selects ACC or WorkSafe NZ. Defaults to "acc" when omitted.
	Destination       ClaimDestination `json:"destination"`
	// EmployerNZBN is the NZBN of the employing organisation; only relevant for WorkSafe claims.
	EmployerNZBN      string           `json:"employerNzbn,omitempty"`
	// InjuryMechanism classifies the mechanism of workplace injury; only relevant for WorkSafe claims.
	InjuryMechanism   string           `json:"injuryMechanism,omitempty"`
}

// claimStatusResponse is the response for GET /api/v1/claims/{id}/status.
type claimStatusResponse struct {
	ClaimID           string      `json:"claimId"`
	Status            ClaimStatus `json:"status"`
	ACCClaimNumber    string      `json:"accClaimNumber,omitempty"`
	WorkSafeRefNumber string      `json:"workSafeRefNumber,omitempty"`
	RejectionReason   string      `json:"rejectionReason,omitempty"`
	PaidAmount        *float64    `json:"paidAmount,omitempty"`
	LastCheckedAt     time.Time   `json:"lastCheckedAt"`
}

// ClaimsHandler handles all /api/v1/claims routes.
type ClaimsHandler struct {
	pool           db.Pool
	enc            *encryption.Cipher
	accClient      *acc.Client
	worksafeClient *worksafe.Client
	auditTrail     *audit.Trail
	logger         *slog.Logger
}

// List handles GET /api/v1/claims.
// Returns all claims for the practice tenant, with optional filters:
// patient (internal ID), status, practitioner (HPI CPN), date range.
func (h *ClaimsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	patientFilter := q.Get("patient")
	statusFilter := q.Get("status")
	providerFilter := q.Get("provider")
	fromDate := q.Get("from")
	toDate := q.Get("to")

	claims, err := h.listClaims(ctx, tenantID.String(), patientFilter, statusFilter, providerFilter, fromDate, toDate)
	if err != nil {
		h.logger.Error("list claims", slog.Any("error", err), slog.String("tenant", tenantID.String()))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list claims"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Claim",
		ResourceID:   "list",
		TenantID:     tenantID,
		Details: map[string]any{
			"patient":  patientFilter,
			"status":   statusFilter,
			"provider": providerFilter,
		},
		OccurredAt: time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"claims": claims,
		"total":  len(claims),
	})
}

// Create handles POST /api/v1/claims.
// Generates a claim from a completed encounter.
// Requires a valid encounter ID and at least one ICD-10-AM diagnosis code.
// Set destination to "worksafe" to route the claim to WorkSafe NZ instead of ACC.
func (h *ClaimsHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req claimCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.Destination == "" {
		req.Destination = DestinationACC
	}

	if err := validateClaimCreate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	// Verify the encounter exists and is completed before issuing a claim.
	encounter, err := (&EncountersHandler{pool: h.pool, logger: h.logger}).getEncounterByID(ctx, req.EncounterID, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "ENCOUNTER_NOT_FOUND", Message: "encounter not found"})
			return
		}
		h.logger.Error("get encounter for claim", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCOUNTER_ERROR", Message: "failed to retrieve encounter"})
		return
	}
	if encounter.Status != EncounterStatusCompleted {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "ENCOUNTER_NOT_COMPLETED",
			Message: fmt.Sprintf("encounter must be in 'completed' status to generate a claim (current: %s)", encounter.Status),
		})
		return
	}

	claim, err := h.insertClaim(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert claim", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create claim"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Claim",
		ResourceID:   claim.ID,
		TenantID:     tenantID,
		Metadata: map[string]string{
			"encounter_id": req.EncounterID,
			"form_type":    string(req.FormType),
			"destination":  string(req.Destination),
		},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, claim)
}

// Get handles GET /api/v1/claims/{id}.
func (h *ClaimsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "claim ID is required"})
		return
	}

	claim, err := h.getClaimByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "claim not found"})
			return
		}
		h.logger.Error("get claim", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve claim"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Claim",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, claim)
}

// Submit handles POST /api/v1/claims/{id}/submit.
// Dispatches the claim to ACC or WorkSafe NZ based on the claim's destination field.
// An atomic DB status transition is performed before calling the external API to
// prevent concurrent double-submission.
func (h *ClaimsHandler) Submit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "claim ID is required"})
		return
	}

	// Atomically transition the claim from draft → submitted before touching the external API.
	// This prevents concurrent retries from lodging the same claim twice (TOCTOU prevention).
	// If the claim is not in draft status the UPDATE returns no rows → errNotFound.
	reserved, err := h.reserveClaimForSubmit(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusConflict, apiError{
				Code:    "NOT_SUBMITTABLE",
				Message: "claim is not in draft status or does not exist",
			})
			return
		}
		h.logger.Error("reserve claim for submit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "RESERVE_ERROR", Message: "failed to reserve claim for submission"})
		return
	}

	var refNumber string
	var auditDest string

	if reserved.Destination == DestinationWorkSafe {
		refNumber, auditDest, err = h.lodgeWorkSafe(ctx, id, tenantID.String(), &reserved)
	} else {
		refNumber, auditDest, err = h.lodgeACC(ctx, id, tenantID.String(), &reserved)
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, apiError{Code: "LODGE_ERROR", Message: err.Error()})
		return
	}

	now := time.Now().UTC()
	reserved.SubmittedAt = &now

	submitted, err := h.updateClaimAfterSubmit(ctx, reserved)
	if err != nil {
		h.logger.Error("update claim after submit", slog.Any("error", err))
		// Claim was lodged externally but the local DB update failed.
		// Do NOT roll back the external submission — the ref number is source of truth.
		writeJSON(w, http.StatusAccepted, map[string]any{
			"message":     "claim submitted but local record update failed — contact support",
			"refNumber":   refNumber,
			"destination": auditDest,
		})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "Claim",
		ResourceID:   id,
		TenantID:     tenantID,
		Details: map[string]any{
			"action":      "submit",
			"destination": auditDest,
			"ref_number":  refNumber,
		},
		OccurredAt: time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, submitted)
}

// lodgeACC dispatches a reserved claim to ACC and returns the claim reference number.
// On failure it rolls back the reservation and returns an error describing the failure.
func (h *ClaimsHandler) lodgeACC(ctx context.Context, id, tenantID string, reserved *Claim) (refNumber, dest string, err error) {
	if h.accClient == nil {
		if rbErr := h.resetClaimToDraft(ctx, id, tenantID); rbErr != nil {
			h.logger.Error("rollback claim to draft", slog.Any("error", rbErr))
		}
		return "", "", fmt.Errorf("ACC integration is not configured on this server")
	}

	lodgeReq := acc.LodgeRequest{
		FormType:          string(reserved.FormType),
		PatientNHI:        reserved.PatientNHI,
		PractitionerHPI:   reserved.PractitionerHPI,
		DiagnosisCodes:    reserved.DiagnosisCodes,
		InjuryDate:        reserved.InjuryDate,
		InjuryDescription: reserved.InjuryDescription,
	}

	lodgeResp, err := h.accClient.Lodge(ctx, lodgeReq)
	if err != nil {
		h.logger.Error("ACC lodge claim", slog.Any("error", err), slog.String("claim_id", id))
		if rbErr := h.resetClaimToDraft(ctx, id, tenantID); rbErr != nil {
			h.logger.Error("rollback claim to draft after lodge failure", slog.Any("error", rbErr))
		}
		return "", "", fmt.Errorf("ACC submission failed")
	}

	reserved.ACCClaimNumber = lodgeResp.ClaimNumber
	return lodgeResp.ClaimNumber, "acc", nil
}

// lodgeWorkSafe dispatches a reserved claim to WorkSafe NZ and returns the claim reference number.
// On failure it rolls back the reservation and returns an error describing the failure.
func (h *ClaimsHandler) lodgeWorkSafe(ctx context.Context, id, tenantID string, reserved *Claim) (refNumber, dest string, err error) {
	if h.worksafeClient == nil {
		if rbErr := h.resetClaimToDraft(ctx, id, tenantID); rbErr != nil {
			h.logger.Error("rollback claim to draft", slog.Any("error", rbErr))
		}
		return "", "", fmt.Errorf("WorkSafe integration is not configured on this server")
	}

	wsReq := worksafe.WorkplaceClaim{
		PatientNHI:        reserved.PatientNHI,
		ProviderHPI:       reserved.PractitionerHPI,
		EmployerNZBN:      reserved.EmployerNZBN,
		DateOfInjury:      reserved.InjuryDate,
		InjuryDescription: reserved.InjuryDescription,
		InjuryMechanism:   worksafe.InjuryMechanism(reserved.InjuryMechanism),
		DiagnosisCodes:    reserved.DiagnosisCodes,
	}

	wsResp, err := h.worksafeClient.Lodge(ctx, wsReq)
	if err != nil {
		h.logger.Error("WorkSafe lodge claim", slog.Any("error", err), slog.String("claim_id", id))
		if rbErr := h.resetClaimToDraft(ctx, id, tenantID); rbErr != nil {
			h.logger.Error("rollback claim to draft after lodge failure", slog.Any("error", rbErr))
		}
		return "", "", fmt.Errorf("WorkSafe submission failed")
	}

	reserved.WorkSafeRefNumber = wsResp.ReferenceNumber
	return wsResp.ReferenceNumber, "worksafe", nil
}

// Status handles GET /api/v1/claims/{id}/status.
// Polls the appropriate external system for the latest claim status and syncs it back.
func (h *ClaimsHandler) Status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "claim ID is required"})
		return
	}

	claim, err := h.getClaimByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "claim not found"})
			return
		}
		h.logger.Error("get claim for status", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve claim"})
		return
	}

	if claim.ACCClaimNumber == "" && claim.WorkSafeRefNumber == "" {
		// Claim has not been submitted to any external system yet — return local status.
		writeJSON(w, http.StatusOK, claimStatusResponse{
			ClaimID:       claim.ID,
			Status:        claim.Status,
			LastCheckedAt: time.Now().UTC(),
		})
		return
	}

	var resp claimStatusResponse
	var auditDest string

	if claim.Destination == DestinationWorkSafe {
		resp = h.pollWorkSafeStatus(ctx, &claim)
		auditDest = "worksafe"
	} else {
		resp = h.pollACCStatus(ctx, &claim)
		auditDest = "acc"
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Claim",
		ResourceID:   id,
		TenantID:     tenantID,
		Details:      map[string]any{"action": "status-poll", "destination": auditDest},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, resp)
}

// pollACCStatus fetches the latest ACC status, syncs the local DB, and returns the status response.
func (h *ClaimsHandler) pollACCStatus(ctx context.Context, claim *Claim) claimStatusResponse {
	if h.accClient == nil {
		return claimStatusResponse{
			ClaimID:        claim.ID,
			Status:         claim.Status,
			ACCClaimNumber: claim.ACCClaimNumber,
			LastCheckedAt:  time.Now().UTC(),
		}
	}

	accStatus, err := h.accClient.GetStatus(ctx, claim.ACCClaimNumber)
	if err != nil {
		h.logger.Error("ACC get status", slog.Any("error", err), slog.String("acc_claim_number", claim.ACCClaimNumber))
		return claimStatusResponse{
			ClaimID:        claim.ID,
			Status:         claim.Status,
			ACCClaimNumber: claim.ACCClaimNumber,
			LastCheckedAt:  time.Now().UTC(),
		}
	}

	mappedStatus := mapACCStatus(accStatus.Status)
	if mappedStatus != claim.Status {
		claim.Status = mappedStatus
		claim.RejectionReason = accStatus.RejectionReason
		if accStatus.PaidAmount > 0 {
			claim.PaidAmount = &accStatus.PaidAmount
		}
		if _, err := h.updateClaimStatus(ctx, *claim); err != nil {
			h.logger.Error("sync ACC status to local DB", slog.Any("error", err))
		}
	}

	return claimStatusResponse{
		ClaimID:         claim.ID,
		Status:          claim.Status,
		ACCClaimNumber:  claim.ACCClaimNumber,
		RejectionReason: claim.RejectionReason,
		PaidAmount:      claim.PaidAmount,
		LastCheckedAt:   time.Now().UTC(),
	}
}

// pollWorkSafeStatus fetches the latest WorkSafe status, syncs the local DB, and returns the status response.
func (h *ClaimsHandler) pollWorkSafeStatus(ctx context.Context, claim *Claim) claimStatusResponse {
	if h.worksafeClient == nil {
		return claimStatusResponse{
			ClaimID:           claim.ID,
			Status:            claim.Status,
			WorkSafeRefNumber: claim.WorkSafeRefNumber,
			LastCheckedAt:     time.Now().UTC(),
		}
	}

	wsStatus, err := h.worksafeClient.GetStatus(ctx, claim.WorkSafeRefNumber)
	if err != nil {
		h.logger.Error("WorkSafe get status", slog.Any("error", err), slog.String("worksafe_ref", claim.WorkSafeRefNumber))
		return claimStatusResponse{
			ClaimID:           claim.ID,
			Status:            claim.Status,
			WorkSafeRefNumber: claim.WorkSafeRefNumber,
			LastCheckedAt:     time.Now().UTC(),
		}
	}

	mappedStatus := mapWorkSafeStatus(wsStatus.Status)
	if mappedStatus != claim.Status {
		claim.Status = mappedStatus
		if _, err := h.updateClaimStatus(ctx, *claim); err != nil {
			h.logger.Error("sync WorkSafe status to local DB", slog.Any("error", err))
		}
	}

	return claimStatusResponse{
		ClaimID:           claim.ID,
		Status:            claim.Status,
		WorkSafeRefNumber: claim.WorkSafeRefNumber,
		LastCheckedAt:     time.Now().UTC(),
	}
}

// validateClaimCreate enforces required fields on claim creation.
func validateClaimCreate(req *claimCreateRequest) error {
	if req.EncounterID == "" {
		return fmt.Errorf("encounterId is required")
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		return fmt.Errorf("either patientId or patientNhi is required")
	}
	if req.PractitionerHPI == "" {
		return fmt.Errorf("practitionerHpi is required")
	}
	if req.FormType == "" {
		return fmt.Errorf("formType is required (ACC45 or ACC6)")
	}
	if req.FormType != ACCFormACC45 && req.FormType != ACCFormACC6 {
		return fmt.Errorf("invalid formType %q: must be ACC45 or ACC6", req.FormType)
	}
	if len(req.DiagnosisCodes) == 0 {
		return fmt.Errorf("at least one diagnosis code is required")
	}
	if req.InjuryDate.IsZero() {
		return fmt.Errorf("injuryDate is required")
	}
	if req.InjuryDescription == "" {
		return fmt.Errorf("injuryDescription is required")
	}
	if req.Destination != DestinationACC && req.Destination != DestinationWorkSafe {
		return fmt.Errorf("invalid destination %q: must be 'acc' or 'worksafe'", req.Destination)
	}
	return nil
}

// mapACCStatus translates an ACC API status string to the local ClaimStatus enum.
func mapACCStatus(accStatus string) ClaimStatus {
	switch accStatus {
	case "ACCEPTED":
		return ClaimStatusAccepted
	case "REJECTED":
		return ClaimStatusRejected
	case "PAID":
		return ClaimStatusPaid
	case "PENDING":
		return ClaimStatusPending
	case "SUBMITTED":
		return ClaimStatusSubmitted
	default:
		return ClaimStatusPending
	}
}

// mapWorkSafeStatus translates a WorkSafe API status to the local ClaimStatus enum.
func mapWorkSafeStatus(ws worksafe.ClaimStatus) ClaimStatus {
	switch ws {
	case worksafe.ClaimActive:
		return ClaimStatusAccepted
	case worksafe.ClaimDeclined:
		return ClaimStatusRejected
	case worksafe.ClaimComplete:
		return ClaimStatusPaid
	default:
		return ClaimStatusPending
	}
}

// listClaims queries the claims table with optional filters.
func (h *ClaimsHandler) listClaims(
	ctx context.Context,
	tenantID, patientFilter, statusFilter, providerFilter, fromDate, toDate string,
) ([]Claim, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		        form_type, form_number, diagnosis_codes,
		        injury_date, injury_description, status,
		        acc_claim_number, rejection_reason, paid_amount,
		        tenant_id, created_at, updated_at, submitted_at,
		        claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism
		 FROM acc_claims
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter  = '' OR patient_id        = @patient_filter)
		   AND (@status_filter   = '' OR status             = @status_filter)
		   AND (@provider_filter = '' OR practitioner_hpi  = @provider_filter)
		   AND (@from_date       = '' OR injury_date       >= @from_date::date)
		   AND (@to_date         = '' OR injury_date       <= @to_date::date)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":       tenantID,
			"patient_filter":  patientFilter,
			"status_filter":   statusFilter,
			"provider_filter": providerFilter,
			"from_date":       fromDate,
			"to_date":         toDate,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query claims: %w", err)
	}
	defer rows.Close()

	var results []Claim
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// getClaimByID retrieves a single claim with tenant isolation.
func (h *ClaimsHandler) getClaimByID(ctx context.Context, id, tenantID string) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		        form_type, form_number, diagnosis_codes,
		        injury_date, injury_description, status,
		        acc_claim_number, rejection_reason, paid_amount,
		        tenant_id, created_at, updated_at, submitted_at,
		        claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism
		 FROM acc_claims
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	c, err := scanClaim(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Claim{}, errNotFound
		}
		return Claim{}, fmt.Errorf("get claim by id: %w", err)
	}
	return c, nil
}

// insertClaim persists a new claim in draft status.
func (h *ClaimsHandler) insertClaim(ctx context.Context, req claimCreateRequest, tenantID string) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO acc_claims
		   (encounter_id, patient_id, patient_nhi, practitioner_hpi,
		    form_type, diagnosis_codes, injury_date, injury_description,
		    status, tenant_id, claim_destination, employer_nzbn, injury_mechanism)
		 VALUES
		   (@encounter_id, @patient_id, @patient_nhi, @practitioner_hpi,
		    @form_type, @diagnosis_codes, @injury_date, @injury_description,
		    @status, @tenant_id, @claim_destination, @employer_nzbn, @injury_mechanism)
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at,
		           claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism`,
		db.NamedArgs{
			"encounter_id":       req.EncounterID,
			"patient_id":         req.PatientID,
			"patient_nhi":        req.PatientNHI,
			"practitioner_hpi":   req.PractitionerHPI,
			"form_type":          req.FormType,
			"diagnosis_codes":    req.DiagnosisCodes,
			"injury_date":        req.InjuryDate,
			"injury_description": req.InjuryDescription,
			"status":             ClaimStatusDraft,
			"tenant_id":          tenantID,
			"claim_destination":  req.Destination,
			"employer_nzbn":      req.EmployerNZBN,
			"injury_mechanism":   req.InjuryMechanism,
		},
	)
	c, err := scanClaim(row)
	if err != nil {
		return Claim{}, fmt.Errorf("insert claim: %w", err)
	}
	return c, nil
}

// reserveClaimForSubmit atomically transitions a claim from draft → submitted.
// Returns the reserved claim, or errNotFound if the claim is not in draft status.
// This prevents concurrent requests from lodging the same claim twice (TOCTOU prevention).
func (h *ClaimsHandler) reserveClaimForSubmit(ctx context.Context, id, tenantID string) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE acc_claims
		 SET status     = @status,
		     updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at,
		           claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism`,
		db.NamedArgs{
			"status":    ClaimStatusSubmitted,
			"id":        id,
			"tenant_id": tenantID,
		},
	)
	c, err := scanClaim(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Claim{}, errNotFound
		}
		return Claim{}, fmt.Errorf("reserve claim for submit: %w", err)
	}
	return c, nil
}

// resetClaimToDraft rolls back a reserved claim to draft status after a failed lodge.
// The condition requires both ref number columns to be empty to prevent accidentally
// rolling back a claim that was successfully lodged with the external system.
func (h *ClaimsHandler) resetClaimToDraft(ctx context.Context, id, tenantID string) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE acc_claims SET status = 'draft', updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id AND status = 'submitted'
		   AND acc_claim_number = '' AND worksafe_ref_number = ''`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	if err != nil {
		return fmt.Errorf("reset claim to draft: %w", err)
	}
	return nil
}

// updateClaimAfterSubmit persists the external system's response fields after a successful lodge.
func (h *ClaimsHandler) updateClaimAfterSubmit(ctx context.Context, c Claim) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE acc_claims
		 SET acc_claim_number    = @acc_claim_number,
		     worksafe_ref_number = @worksafe_ref_number,
		     submitted_at        = @submitted_at,
		     updated_at          = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at,
		           claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism`,
		db.NamedArgs{
			"acc_claim_number":    c.ACCClaimNumber,
			"worksafe_ref_number": c.WorkSafeRefNumber,
			"submitted_at":        c.SubmittedAt,
			"id":                  c.ID,
			"tenant_id":           c.TenantID,
		},
	)
	updated, err := scanClaim(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Claim{}, errNotFound
		}
		return Claim{}, fmt.Errorf("update claim after submit: %w", err)
	}
	return updated, nil
}

// updateClaimStatus syncs the externally-polled status back to the local database.
func (h *ClaimsHandler) updateClaimStatus(ctx context.Context, c Claim) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE acc_claims
		 SET status           = @status,
		     rejection_reason = @rejection_reason,
		     paid_amount      = @paid_amount,
		     updated_at       = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at,
		           claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism`,
		db.NamedArgs{
			"status":           c.Status,
			"rejection_reason": c.RejectionReason,
			"paid_amount":      c.PaidAmount,
			"id":               c.ID,
			"tenant_id":        c.TenantID,
		},
	)
	updated, err := scanClaim(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Claim{}, errNotFound
		}
		return Claim{}, fmt.Errorf("update claim status: %w", err)
	}
	return updated, nil
}

// scanClaim scans a single Claim from a pgx row or rows cursor.
func scanClaim(row dbRow) (Claim, error) {
	var c Claim
	var dest string
	if err := row.Scan(
		&c.ID, &c.EncounterID, &c.PatientID, &c.PatientNHI, &c.PractitionerHPI,
		&c.FormType, &c.FormNumber, &c.DiagnosisCodes,
		&c.InjuryDate, &c.InjuryDescription, &c.Status,
		&c.ACCClaimNumber, &c.RejectionReason, &c.PaidAmount,
		&c.TenantID, &c.CreatedAt, &c.UpdatedAt, &c.SubmittedAt,
		&dest, &c.WorkSafeRefNumber, &c.EmployerNZBN, &c.InjuryMechanism,
	); err != nil {
		return Claim{}, err
	}
	c.Destination = ClaimDestination(dest)
	return c, nil
}
