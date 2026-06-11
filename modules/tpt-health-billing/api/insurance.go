package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// --- Domain types ---

// Insurer identifies a NZ private health insurer.
type Insurer string

const (
	// InsurerSouthernCross is Southern Cross Health Society, the largest NZ private health insurer.
	InsurerSouthernCross Insurer = "SOUTHERN_CROSS"
	// InsurerNIB is nib nz limited.
	InsurerNIB Insurer = "NIB"
	// InsurerAIA is AIA New Zealand Limited.
	InsurerAIA Insurer = "AIA"
	// InsurerPartnersLife is Partners Life.
	InsurerPartnersLife Insurer = "PARTNERS_LIFE"
	// InsurerAccuro is Accuro Health Insurance.
	InsurerAccuro Insurer = "ACCURO"
	// InsurerOther covers any insurer not listed above.
	InsurerOther Insurer = "OTHER"
)

// InsuranceClaimStatus reflects the health insurance claim lifecycle.
type InsuranceClaimStatus string

const (
	InsuranceClaimDraft     InsuranceClaimStatus = "draft"
	InsuranceClaimSubmitted InsuranceClaimStatus = "submitted"
	InsuranceClaimApproved  InsuranceClaimStatus = "approved"
	InsuranceClaimDeclined  InsuranceClaimStatus = "declined"
	InsuranceClaimPaid      InsuranceClaimStatus = "paid"
	InsuranceClaimAppealed  InsuranceClaimStatus = "appealed"
)

// InsuranceClaim represents a private health insurance claim lodged on behalf of a patient.
//
// NZ insurers typically require:
//   - Policy number and member ID (from patient-held insurance card)
//   - Itemised invoice from the provider
//   - Diagnosis codes (ICD-10-AM) and procedure codes
//   - Referral from GP where required by policy (specialist services)
//
// Submission pathways vary by insurer:
//   - Southern Cross: HealthPoint Provider Portal / direct EFTPOS integration
//   - nib, AIA: Online provider portals
//   - Most others: PDF/fax or patient self-submission
type InsuranceClaim struct {
	ID                    string               `json:"id"`
	TenantID              string               `json:"tenantId"`
	InvoiceID             string               `json:"invoiceId,omitempty"`
	PatientNHI            string               `json:"patientNhi"`   // encrypted at rest
	Insurer               Insurer              `json:"insurer"`
	PolicyNumber          string               `json:"policyNumber"`
	MemberID              string               `json:"memberId"`
	Status                InsuranceClaimStatus `json:"status"`
	ClaimedAmountNZD      float64              `json:"claimedAmountNzd"`
	ApprovedAmountNZD     float64              `json:"approvedAmountNzd,omitempty"`
	InsurerReference      string               `json:"insurerReference,omitempty"`
	SubmittedAt           *time.Time           `json:"submittedAt,omitempty"`
	DecisionAt            *time.Time           `json:"decisionAt,omitempty"`
	PaidAt                *time.Time           `json:"paidAt,omitempty"`
	DeclineReason         string               `json:"declineReason,omitempty"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
}

// CreateInsuranceClaimRequest is the body for POST /api/v1/insurance/claims.
type CreateInsuranceClaimRequest struct {
	TenantID         string  `json:"tenantId"`
	InvoiceID        string  `json:"invoiceId,omitempty"`
	PatientNHI       string  `json:"patientNhi"`
	Insurer          Insurer `json:"insurer"`
	PolicyNumber     string  `json:"policyNumber"`
	MemberID         string  `json:"memberId"`
	ClaimedAmountNZD float64 `json:"claimedAmountNzd"`
}

// InsuranceHandler handles all /api/v1/insurance/* routes.
type InsuranceHandler struct {
	logger *slog.Logger
}

// List handles GET /api/v1/insurance/claims — list health insurance claims.
//
// Query parameters:
//   - status: filter by InsuranceClaimStatus
//   - insurer: filter by Insurer
//   - tenant_id: filter by tenant
func (h *InsuranceHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	insurer := r.URL.Query().Get("insurer")
	tenantID := r.URL.Query().Get("tenant_id")

	h.logger.Info("list insurance claims",
		"status", status,
		"insurer", insurer,
		"tenant_id", tenantID,
		"request_id", r.Context().Value(requestIDKey),
	)

	// In production: query billing_insurance_claims with filters, cursor pagination.
	writeJSON(w, http.StatusOK, map[string]any{
		"claims": []InsuranceClaim{},
		"total":  0,
	})
}

// Create handles POST /api/v1/insurance/claims — register a new insurance claim.
//
// The claim is created in "draft" status. An itemised invoice must be attached
// (via invoiceId) before submission. For Southern Cross, the billing service can
// submit electronically via the HealthPoint provider portal integration. For other
// insurers, a PDF claim form is generated for patient or provider submission.
func (h *InsuranceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateInsuranceClaimRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("create insurance claim: decode: %v", err))
		return
	}

	if req.PatientNHI == "" {
		writeError(w, http.StatusUnprocessableEntity, "patientNhi is required")
		return
	}
	if req.Insurer == "" {
		writeError(w, http.StatusUnprocessableEntity, "insurer is required")
		return
	}
	if req.PolicyNumber == "" {
		writeError(w, http.StatusUnprocessableEntity, "policyNumber is required")
		return
	}
	if req.MemberID == "" {
		writeError(w, http.StatusUnprocessableEntity, "memberId is required")
		return
	}
	if req.ClaimedAmountNZD <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "claimedAmountNzd must be greater than zero")
		return
	}

	// In production:
	//   1. Validate NHI checksum via core/nhi.
	//   2. Encrypt patientNhi with core/encryption before persisting.
	//   3. If invoiceId provided: load invoice and verify claimedAmountNzd <= invoice.PatientAmount.
	//   4. Persist to billing_insurance_claims with status=draft.
	//   5. Write AuditEvent.

	now := time.Now().UTC()
	claim := InsuranceClaim{
		ID:               fmt.Sprintf("ins-%d", now.UnixNano()),
		TenantID:         req.TenantID,
		InvoiceID:        req.InvoiceID,
		PatientNHI:       req.PatientNHI,
		Insurer:          req.Insurer,
		PolicyNumber:     req.PolicyNumber,
		MemberID:         req.MemberID,
		Status:           InsuranceClaimDraft,
		ClaimedAmountNZD: req.ClaimedAmountNZD,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	h.logger.Info("insurance claim created",
		"claim_id", claim.ID,
		"insurer", claim.Insurer,
		"claimed_amount", claim.ClaimedAmountNZD,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, claim)
}

// Get handles GET /api/v1/insurance/claims/{id}.
func (h *InsuranceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	h.logger.Info("get insurance claim", "claim_id", id, "request_id", r.Context().Value(requestIDKey))

	writeError(w, http.StatusNotFound, "claim not found")
}

// Submit handles POST /api/v1/insurance/claims/{id}/submit — submit the claim to the insurer.
//
// Submission is insurer-specific:
//   - SOUTHERN_CROSS: submits via HealthPoint Provider Portal API (JSON REST).
//   - NIB, AIA, ACCURO: submits via insurer-specific provider portals (varies).
//   - PARTNERS_LIFE, OTHER: generates a PDF claim form for manual submission.
//
// The Insurer field on the claim determines which submission path is taken.
func (h *InsuranceHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	// In production:
	//   1. Load InsuranceClaim, assert status == draft.
	//   2. Assert an invoiceId is linked (itemised invoice required by all NZ insurers).
	//   3. Dispatch to the appropriate insurer submission adapter based on claim.Insurer.
	//   4. On success: update status=submitted, store insurerReference.
	//   5. For SOUTHERN_CROSS: expect synchronous approval decision (update status accordingly).
	//   6. Write AuditEvent with action="INSURANCE-CLAIM-SUBMITTED" and claim metadata.
	//      Note: Do NOT log full policy details to avoid unnecessary PHI exposure in audit.

	now := time.Now().UTC()

	h.logger.Info("insurance claim submitted",
		"claim_id", id,
		"submitted_at", now,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"claimId":     id,
		"status":      string(InsuranceClaimSubmitted),
		"submittedAt": now,
	})
}

// Status handles GET /api/v1/insurance/claims/{id}/status — check insurer decision status.
func (h *InsuranceHandler) Status(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	// In production:
	//   1. Load InsuranceClaim.
	//   2. For insurers with polling APIs (Southern Cross, nib): query insurer for updated status.
	//   3. If status changed to approved: update approvedAmountNzd and create billing_payments entry.
	//   4. If declined: store declineReason.
	//   5. Write AuditEvent if status changed.

	h.logger.Info("insurance claim status check", "claim_id", id, "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"claimId":   id,
		"status":    string(InsuranceClaimSubmitted),
		"checkedAt": time.Now().UTC(),
	})
}
