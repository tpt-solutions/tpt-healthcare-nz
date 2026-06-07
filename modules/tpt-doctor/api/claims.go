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
)

// ClaimStatus mirrors the ACC claim lifecycle.
type ClaimStatus string

const (
	ClaimStatusDraft      ClaimStatus = "draft"
	ClaimStatusSubmitted  ClaimStatus = "submitted"
	ClaimStatusAccepted   ClaimStatus = "accepted"
	ClaimStatusRejected   ClaimStatus = "rejected"
	ClaimStatusPaid       ClaimStatus = "paid"
	ClaimStatusCancelled  ClaimStatus = "cancelled"
	ClaimStatusPending    ClaimStatus = "pending"
)

// ACCFormType enumerates the supported ACC claim form types.
type ACCFormType string

const (
	ACCFormACC45 ACCFormType = "ACC45" // Injury claims
	ACCFormACC6  ACCFormType = "ACC6"  // Treatment injury
)

// Claim is the domain model for an ACC claim generated from a clinical encounter.
type Claim struct {
	ID              string      `json:"id"`
	EncounterID     string      `json:"encounterId"`
	PatientID       string      `json:"patientId"`
	PatientNHI      string      `json:"patientNhi"`
	PractitionerHPI string      `json:"practitionerHpi"`
	FormType        ACCFormType `json:"formType"`
	FormNumber      string      `json:"formNumber,omitempty"` // Assigned by ACC on submission
	DiagnosisCodes  []string    `json:"diagnosisCodes"`       // ICD-10-AM codes
	InjuryDate      time.Time   `json:"injuryDate"`
	InjuryDescription string    `json:"injuryDescription"`
	Status          ClaimStatus `json:"status"`
	ACCClaimNumber  string      `json:"accClaimNumber,omitempty"`  // ACC reference
	RejectionReason string      `json:"rejectionReason,omitempty"`
	PaidAmount      *float64    `json:"paidAmount,omitempty"`
	TenantID        string      `json:"tenantId"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
	SubmittedAt     *time.Time  `json:"submittedAt,omitempty"`
}

// claimCreateRequest is the body for POST /api/v1/claims.
type claimCreateRequest struct {
	EncounterID       string      `json:"encounterId"`
	PatientID         string      `json:"patientId"`
	PatientNHI        string      `json:"patientNhi"`
	PractitionerHPI   string      `json:"practitionerHpi"`
	FormType          ACCFormType `json:"formType"`
	DiagnosisCodes    []string    `json:"diagnosisCodes"`
	InjuryDate        time.Time   `json:"injuryDate"`
	InjuryDescription string      `json:"injuryDescription"`
}

// claimStatusResponse is the response for GET /api/v1/claims/{id}/status.
type claimStatusResponse struct {
	ClaimID         string      `json:"claimId"`
	Status          ClaimStatus `json:"status"`
	ACCClaimNumber  string      `json:"accClaimNumber,omitempty"`
	RejectionReason string      `json:"rejectionReason,omitempty"`
	PaidAmount      *float64    `json:"paidAmount,omitempty"`
	LastCheckedAt   time.Time   `json:"lastCheckedAt"`
}

// ClaimsHandler handles all /api/v1/claims routes.
type ClaimsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	accClient  *acc.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/claims.
// Returns all ACC claims for the practice tenant, with optional filters:
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

	claims, err := h.listClaims(ctx, tenantID, patientFilter, statusFilter, providerFilter, fromDate, toDate)
	if err != nil {
		h.logger.Error("list claims", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list claims"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Claim",
		ResourceID:   "list",
		TenantID:     tenantID,
		Metadata: map[string]string{
			"patient":  patientFilter,
			"status":   statusFilter,
			"provider": providerFilter,
		},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"claims": claims,
		"total":  len(claims),
	})
}

// Create handles POST /api/v1/claims.
// Generates an ACC claim from a completed encounter.
// Requires a valid encounter ID and at least one ICD-10-AM diagnosis code.
// Validates that the encounter exists and has a completed status before generating.
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
// Submits the claim to ACC via core/acc.Lodge. An atomic DB status transition
// is performed before calling the external API to prevent concurrent double-submission.
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

	if h.accClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "ACC_UNAVAILABLE", Message: "ACC integration is not configured on this server"})
		return
	}

	// Atomically transition the claim from draft → submitted before touching ACC.
	// This prevents concurrent retries from lodging the same claim twice.
	// If the claim is not in draft status the UPDATE returns no rows → errNotFound.
	reserved, err := h.reserveClaimForSubmit(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			// Claim either does not exist, is already submitted, or is cancelled.
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

	// Build the ACC lodge request.
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
		// Roll back the reservation so the claim can be retried.
		if rbErr := h.resetClaimToDraft(ctx, id, tenantID); rbErr != nil {
			h.logger.Error("rollback claim to draft after lodge failure", slog.Any("error", rbErr))
		}
		writeJSON(w, http.StatusBadGateway, apiError{Code: "ACC_LODGE_ERROR", Message: "ACC submission failed"})
		return
	}

	now := time.Now().UTC()
	reserved.ACCClaimNumber = lodgeResp.ClaimNumber
	reserved.SubmittedAt = &now

	submitted, err := h.updateClaimAfterSubmit(ctx, reserved)
	if err != nil {
		h.logger.Error("update claim after submit", slog.Any("error", err))
		// Claim was lodged with ACC but the local DB update failed.
		// Do NOT roll back the ACC submission — the claim number is the source of truth.
		writeJSON(w, http.StatusAccepted, map[string]any{
			"message":        "claim submitted to ACC but local record update failed — contact support",
			"accClaimNumber": lodgeResp.ClaimNumber,
		})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Claim",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata: map[string]string{
			"action":           "submit",
			"acc_claim_number": lodgeResp.ClaimNumber,
		},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, submitted)
}

// Status handles GET /api/v1/claims/{id}/status.
// Polls the ACC API for the latest claim status and updates the local record.
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

	claim, err := h.getClaimByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "claim not found"})
			return
		}
		h.logger.Error("get claim for status", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve claim"})
		return
	}

	if claim.ACCClaimNumber == "" {
		// Claim has not been submitted to ACC yet — return local status.
		writeJSON(w, http.StatusOK, claimStatusResponse{
			ClaimID:       claim.ID,
			Status:        claim.Status,
			LastCheckedAt: time.Now().UTC(),
		})
		return
	}

	if h.accClient == nil {
		// Return the locally-cached status when ACC is not configured.
		writeJSON(w, http.StatusOK, claimStatusResponse{
			ClaimID:        claim.ID,
			Status:         claim.Status,
			ACCClaimNumber: claim.ACCClaimNumber,
			LastCheckedAt:  time.Now().UTC(),
		})
		return
	}

	// Poll ACC for the latest status.
	accStatus, err := h.accClient.GetStatus(ctx, claim.ACCClaimNumber)
	if err != nil {
		h.logger.Error("ACC get status", slog.Any("error", err), slog.String("acc_claim_number", claim.ACCClaimNumber))
		// Return the locally-cached status rather than failing.
		writeJSON(w, http.StatusOK, claimStatusResponse{
			ClaimID:        claim.ID,
			Status:         claim.Status,
			ACCClaimNumber: claim.ACCClaimNumber,
			LastCheckedAt:  time.Now().UTC(),
		})
		return
	}

	// Sync the latest status back to the local database.
	mappedStatus := mapACCStatus(accStatus.Status)
	if mappedStatus != claim.Status {
		claim.Status = mappedStatus
		claim.RejectionReason = accStatus.RejectionReason
		if accStatus.PaidAmount > 0 {
			claim.PaidAmount = &accStatus.PaidAmount
		}
		if _, err := h.updateClaimStatus(ctx, claim); err != nil {
			h.logger.Error("sync ACC status to local DB", slog.Any("error", err))
		}
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Claim",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "status-poll"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, claimStatusResponse{
		ClaimID:         claim.ID,
		Status:          claim.Status,
		ACCClaimNumber:  claim.ACCClaimNumber,
		RejectionReason: claim.RejectionReason,
		PaidAmount:      claim.PaidAmount,
		LastCheckedAt:   time.Now().UTC(),
	})
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
		        tenant_id, created_at, updated_at, submitted_at
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
		        tenant_id, created_at, updated_at, submitted_at
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
		    status, tenant_id)
		 VALUES
		   (@encounter_id, @patient_id, @patient_nhi, @practitioner_hpi,
		    @form_type, @diagnosis_codes, @injury_date, @injury_description,
		    @status, @tenant_id)
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at`,
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
		           tenant_id, created_at, updated_at, submitted_at`,
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
func (h *ClaimsHandler) resetClaimToDraft(ctx context.Context, id, tenantID string) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE acc_claims SET status = 'draft', updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id AND status = 'submitted' AND acc_claim_number = ''`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	if err != nil {
		return fmt.Errorf("reset claim to draft: %w", err)
	}
	return nil
}

// updateClaimAfterSubmit persists ACC's response fields after a successful lodge.
func (h *ClaimsHandler) updateClaimAfterSubmit(ctx context.Context, c Claim) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE acc_claims
		 SET acc_claim_number = @acc_claim_number,
		     submitted_at     = @submitted_at,
		     updated_at       = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at`,
		db.NamedArgs{
			"acc_claim_number": c.ACCClaimNumber,
			"submitted_at":     c.SubmittedAt,
			"id":               c.ID,
			"tenant_id":        c.TenantID,
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

// updateClaimStatus syncs the ACC-polled status back to the local database.
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
		           tenant_id, created_at, updated_at, submitted_at`,
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
	if err := row.Scan(
		&c.ID, &c.EncounterID, &c.PatientID, &c.PatientNHI, &c.PractitionerHPI,
		&c.FormType, &c.FormNumber, &c.DiagnosisCodes,
		&c.InjuryDate, &c.InjuryDescription, &c.Status,
		&c.ACCClaimNumber, &c.RejectionReason, &c.PaidAmount,
		&c.TenantID, &c.CreatedAt, &c.UpdatedAt, &c.SubmittedAt,
	); err != nil {
		return Claim{}, err
	}
	return c, nil
}
