package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// --- Domain types ---

// PHARMACSubsidyClaimStatus reflects the PHARMAC subsidy claim lifecycle.
type PHARMACSubsidyClaimStatus string

const (
	PHARMACSubsidyDraft     PHARMACSubsidyClaimStatus = "draft"
	PHARMACSubsidySubmitted PHARMACSubsidyClaimStatus = "submitted"
	PHARMACSubsidyAccepted  PHARMACSubsidyClaimStatus = "accepted"
	PHARMACSubsidyRejected  PHARMACSubsidyClaimStatus = "rejected"
	PHARMACSubsidyPaid      PHARMACSubsidyClaimStatus = "paid"
)

// PHARMACSubsidyClaim is a consolidated PHARMAC subsidy claim covering one or more
// MedicationDispense records from tpt-pharmacy over a given billing period.
// Claims are submitted to the PHARMAC ePAD (Electronic Prescription, Administration
// and Dispensing) gateway.
type PHARMACSubsidyClaim struct {
	ID                    string                    `json:"id"`
	TenantID              string                    `json:"tenantId"`
	PharmacyHSPNo         string                    `json:"pharmacyHspNo"` // Health Service Provider number
	Status                PHARMACSubsidyClaimStatus `json:"status"`
	ClaimPeriodStart      time.Time                 `json:"claimPeriodStart"`
	ClaimPeriodEnd        time.Time                 `json:"claimPeriodEnd"`
	SourceDispenseIDs     []string                  `json:"sourceDispenseIds"`
	TotalSubsidyAmountNZD float64                   `json:"totalSubsidyAmountNzd"`
	PHARMACReferenceNo    string                    `json:"pharmacReferenceNo,omitempty"`
	SubmittedAt           *time.Time                `json:"submittedAt,omitempty"`
	PaidAt                *time.Time                `json:"paidAt,omitempty"`
	CreatedAt             time.Time                 `json:"createdAt"`
	UpdatedAt             time.Time                 `json:"updatedAt"`
}

// CreatePHARMACClaimRequest is the body for POST /api/v1/pharmac/claims.
type CreatePHARMACClaimRequest struct {
	TenantID         string    `json:"tenantId"`
	PharmacyHSPNo    string    `json:"pharmacyHspNo"`
	ClaimPeriodStart time.Time `json:"claimPeriodStart"`
	ClaimPeriodEnd   time.Time `json:"claimPeriodEnd"`
	// SourceDispenseIDs lists the MedicationDispense record IDs from tpt-pharmacy
	// that should be included in this subsidy claim batch.
	SourceDispenseIDs []string `json:"sourceDispenseIds"`
}

// PHARMACHandler handles all /api/v1/pharmac/* routes.
type PHARMACHandler struct {
	logger *slog.Logger
}

// List handles GET /api/v1/pharmac/claims — list PHARMAC subsidy claims.
//
// Query parameters:
//   - status: filter by PHARMACSubsidyClaimStatus
//   - pharmacy_hsp_no: filter to a specific pharmacy
//   - tenant_id: filter by tenant
func (h *PHARMACHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	pharmacyHSPNo := r.URL.Query().Get("pharmacy_hsp_no")
	tenantID := r.URL.Query().Get("tenant_id")

	h.logger.Info("list PHARMAC subsidy claims",
		"status", status,
		"pharmacy_hsp_no", pharmacyHSPNo,
		"tenant_id", tenantID,
		"request_id", r.Context().Value(requestIDKey),
	)

	// In production: query billing_pharmac_claims with filters, cursor pagination.
	writeJSON(w, http.StatusOK, map[string]any{
		"claims": []PHARMACSubsidyClaim{},
		"total":  0,
	})
}

// Create handles POST /api/v1/pharmac/claims — assemble a new subsidy claim batch.
//
// The claim is created in "draft" status. The billing administrator must review the
// calculated subsidy totals before calling /submit to lodge with PHARMAC. Each
// dispense ID must reference a completed MedicationDispense in tpt-pharmacy with
// subsidy-eligible PHARMAC schedule items.
func (h *PHARMACHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreatePHARMACClaimRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("create PHARMAC claim: decode: %v", err))
		return
	}

	if req.PharmacyHSPNo == "" {
		writeError(w, http.StatusUnprocessableEntity, "pharmacyHspNo is required")
		return
	}
	if len(req.SourceDispenseIDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "at least one sourceDispenseId is required")
		return
	}
	if req.ClaimPeriodEnd.Before(req.ClaimPeriodStart) {
		writeError(w, http.StatusUnprocessableEntity, "claimPeriodEnd must be after claimPeriodStart")
		return
	}

	// In production:
	//   1. Load each MedicationDispense from tpt-pharmacy (status must be "completed").
	//   2. For each dispense: look up PHARMAC schedule subsidy amount via core/pharmac.
	//   3. Reject any dispenses whose NZMT code is unsubsidised or on restricted schedule
	//      without a Special Authority approval.
	//   4. Sum total subsidy. Build claim in draft status.
	//   5. Persist to billing_pharmac_claims.
	//   6. Write AuditEvent.

	now := time.Now().UTC()
	claim := PHARMACSubsidyClaim{
		ID:                    fmt.Sprintf("pharmac-%d", now.UnixNano()),
		TenantID:              req.TenantID,
		PharmacyHSPNo:         req.PharmacyHSPNo,
		Status:                PHARMACSubsidyDraft,
		ClaimPeriodStart:      req.ClaimPeriodStart,
		ClaimPeriodEnd:        req.ClaimPeriodEnd,
		SourceDispenseIDs:     req.SourceDispenseIDs,
		TotalSubsidyAmountNZD: 0,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	h.logger.Info("PHARMAC subsidy claim created",
		"claim_id", claim.ID,
		"pharmacy_hsp_no", claim.PharmacyHSPNo,
		"dispense_count", len(claim.SourceDispenseIDs),
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, claim)
}

// Get handles GET /api/v1/pharmac/claims/{id}.
func (h *PHARMACHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	h.logger.Info("get PHARMAC claim", "claim_id", id, "request_id", r.Context().Value(requestIDKey))

	writeError(w, http.StatusNotFound, "claim not found")
}

// Submit handles POST /api/v1/pharmac/claims/{id}/submit — lodge claim with PHARMAC ePAD.
//
// PHARMAC claims are submitted via the PHARMAC ePAD API using a proprietary
// submission format (currently HL7 v2.5.1 or PHARMAC-specific XML, depending
// on the pharmacy PMS version). The billing service normalises this from the
// internal representation and handles the submission handshake.
func (h *PHARMACHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	// In production:
	//   1. Load PHARMACSubsidyClaim, assert status == draft.
	//   2. Re-validate subsidy amounts against current PHARMAC schedule (prices may have changed).
	//   3. Build the PHARMAC ePAD submission payload.
	//   4. POST to PHARMAC ePAD endpoint (cfg.PHARMACBaseURL) with mTLS client certificate.
	//   5. On success: update status=submitted, store PHARMACReferenceNo.
	//   6. Enqueue a background polling job (River) to check status at 24h intervals.
	//   7. Write AuditEvent.

	now := time.Now().UTC()

	h.logger.Info("PHARMAC subsidy claim submitted",
		"claim_id", id,
		"submitted_at", now,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"claimId":     id,
		"status":      string(PHARMACSubsidySubmitted),
		"submittedAt": now,
	})
}

// Status handles GET /api/v1/pharmac/claims/{id}/status — poll PHARMAC processing status.
func (h *PHARMACHandler) Status(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	// In production:
	//   1. Load PHARMACSubsidyClaim.
	//   2. If status is "submitted": query PHARMAC ePAD status endpoint and update local record.
	//   3. If status changed to "accepted": create billing_payments entry for the subsidy amount.
	//   4. Return current status.

	h.logger.Info("PHARMAC claim status check", "claim_id", id, "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"claimId":            id,
		"status":             string(PHARMACSubsidySubmitted),
		"pharmacReferenceNo": "",
		"checkedAt":          time.Now().UTC(),
	})
}
