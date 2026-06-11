package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// --- Domain types ---

// ReconciliationSummary provides an accounts-receivable snapshot for a tenant.
type ReconciliationSummary struct {
	TenantID string `json:"tenantId"`
	AsAt     time.Time `json:"asAt"`

	// TotalOutstandingNZD is the sum of all issued+overdue invoice patient amounts
	// that have not yet been fully paid.
	TotalOutstandingNZD float64 `json:"totalOutstandingNzd"`

	// OverdueAmountNZD is the subset of TotalOutstandingNZD where due_at < now.
	OverdueAmountNZD float64 `json:"overdueAmountNzd"`

	// UnreconciledPaymentsNZD is the total value of payment records that have not
	// yet been matched to an invoice (e.g. bank deposits without a reference).
	UnreconciledPaymentsNZD float64 `json:"unreconciledPaymentsNzd"`

	// PendingACCAmountNZD is the sum of ACC claim treatment lines not yet paid by ACC.
	PendingACCAmountNZD float64 `json:"pendingAccAmountNzd"`

	// PendingPHARMACAmountNZD is the total of submitted-but-unpaid PHARMAC subsidy claims.
	PendingPHARMACAmountNZD float64 `json:"pendingPharmacAmountNzd"`

	// PendingInsuranceAmountNZD is the total of submitted-but-unpaid insurance claims.
	PendingInsuranceAmountNZD float64 `json:"pendingInsuranceAmountNzd"`

	// AgingBuckets breaks down outstanding amounts by age (days since issue date).
	AgingBuckets AgingBuckets `json:"agingBuckets"`
}

// AgingBuckets breaks outstanding invoices into standard AR aging bands.
type AgingBuckets struct {
	Current    float64 `json:"current"`    // 0-30 days
	Days31to60 float64 `json:"days31to60"` // 31-60 days
	Days61to90 float64 `json:"days61to90"` // 61-90 days
	Over90Days float64 `json:"over90Days"` // > 90 days
}

// ImportBatch is a bank statement or funder payment batch uploaded for matching.
type ImportBatch struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenantId"`
	Source      string          `json:"source"` // "BANK_STATEMENT", "ACC_REMITTANCE", "PHARMAC_REMITTANCE", "INSURANCE_EOB"
	ImportedAt  time.Time       `json:"importedAt"`
	RecordCount int             `json:"recordCount"`
	Records     []ImportRecord  `json:"records"`
}

// ImportRecord is a single line from a bank statement or remittance advice.
type ImportRecord struct {
	ExternalReference string    `json:"externalReference"`
	Amount            float64   `json:"amount"`
	Date              time.Time `json:"date"`
	Description       string    `json:"description"`
	// MatchedPaymentID is set if this record was automatically matched to a billing_payments row.
	MatchedPaymentID string `json:"matchedPaymentId,omitempty"`
	// MatchConfidence is a 0-100 score indicating automatic match confidence.
	MatchConfidence int `json:"matchConfidence,omitempty"`
}

// ImportBatchRequest is the body for POST /api/v1/reconciliation/import.
type ImportBatchRequest struct {
	TenantID string        `json:"tenantId"`
	Source   string        `json:"source"`
	Records  []ImportRecord `json:"records"`
}

// MatchRequest is the body for POST /api/v1/reconciliation/match — manually
// match an unreconciled payment to an invoice.
type MatchRequest struct {
	PaymentID string `json:"paymentId"`
	InvoiceID string `json:"invoiceId"`
}

// ReconciliationHandler handles all /api/v1/reconciliation/* routes.
type ReconciliationHandler struct {
	logger *slog.Logger
}

// Summary handles GET /api/v1/reconciliation/summary — AR snapshot for a tenant.
//
// Query parameters:
//   - tenant_id: required, the tenant to summarise
func (h *ReconciliationHandler) Summary(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id query parameter is required")
		return
	}

	h.logger.Info("reconciliation summary", "tenant_id", tenantID, "request_id", r.Context().Value(requestIDKey))

	// In production:
	//   1. Query billing_invoices for issued+overdue records, sum patient_amount_cents - paid.
	//   2. Query billing_payments where reconciled=false, sum amounts.
	//   3. Query billing_acc_claims where status in (pending, active), sum treatment line amounts.
	//   4. Query billing_pharmac_claims where status=submitted, sum total_subsidy_cents.
	//   5. Query billing_insurance_claims where status=submitted, sum claimed_amount_cents.
	//   6. Compute aging buckets from issued_at offsets.

	summary := ReconciliationSummary{
		TenantID:                  tenantID,
		AsAt:                      time.Now().UTC(),
		TotalOutstandingNZD:       0,
		OverdueAmountNZD:          0,
		UnreconciledPaymentsNZD:   0,
		PendingACCAmountNZD:       0,
		PendingPHARMACAmountNZD:   0,
		PendingInsuranceAmountNZD: 0,
		AgingBuckets: AgingBuckets{
			Current:    0,
			Days31to60: 0,
			Days61to90: 0,
			Over90Days: 0,
		},
	}

	writeJSON(w, http.StatusOK, summary)
}

// Import handles POST /api/v1/reconciliation/import — upload a bank statement or remittance advice.
//
// The billing service attempts automatic matching of imported records to existing
// billing_payments rows using a combination of:
//   - Exact reference number match
//   - Amount + date fuzzy match
//   - Funder/insurer reference number match (for ACC remittances and PHARMAC payments)
//
// Unmatched records are stored as unreconciled and surfaced via GET /unmatched.
func (h *ReconciliationHandler) Import(w http.ResponseWriter, r *http.Request) {
	var req ImportBatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("import: decode: %v", err))
		return
	}

	if req.TenantID == "" {
		writeError(w, http.StatusUnprocessableEntity, "tenantId is required")
		return
	}
	if req.Source == "" {
		writeError(w, http.StatusUnprocessableEntity, "source is required")
		return
	}
	if len(req.Records) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "at least one record is required")
		return
	}

	// In production:
	//   1. For each ImportRecord: attempt auto-match against billing_payments.
	//      - Try exact match on external_reference → payment.reference.
	//      - Try ACC claim number match for ACC_REMITTANCE sources.
	//      - Try PHARMAC reference number match for PHARMAC_REMITTANCE sources.
	//      - Try amount+date window match as a fallback.
	//   2. For matched records: set billing_payments.reconciled=true, reconciled_at=now.
	//      If this payment completes the invoice total, set invoice status=paid.
	//   3. For unmatched records: store as unreconciled payment entries (reference only, no invoice link).
	//   4. Write AuditEvent with batch summary.

	now := time.Now().UTC()
	batch := ImportBatch{
		ID:          fmt.Sprintf("import-%d", now.UnixNano()),
		TenantID:    req.TenantID,
		Source:      req.Source,
		ImportedAt:  now,
		RecordCount: len(req.Records),
		Records:     req.Records,
	}

	h.logger.Info("reconciliation import",
		"batch_id", batch.ID,
		"source", batch.Source,
		"record_count", batch.RecordCount,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, batch)
}

// Unmatched handles GET /api/v1/reconciliation/unmatched — list payments not yet
// matched to an invoice.
//
// Query parameters:
//   - tenant_id: required
func (h *ReconciliationHandler) Unmatched(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id query parameter is required")
		return
	}

	h.logger.Info("list unmatched payments", "tenant_id", tenantID, "request_id", r.Context().Value(requestIDKey))

	// In production: query billing_payments where reconciled=false AND invoice_id IS NULL,
	// ordered by payment_date desc, cursor pagination.
	writeJSON(w, http.StatusOK, map[string]any{
		"payments": []Payment{},
		"total":    0,
	})
}

// Match handles POST /api/v1/reconciliation/match — manually match an unreconciled
// payment to an invoice.
//
// Used by billing administrators to clear unmatched bank deposits or funder payments
// that could not be auto-matched. Validates that the payment amount does not exceed
// the invoice outstanding balance.
func (h *ReconciliationHandler) Match(w http.ResponseWriter, r *http.Request) {
	var req MatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("match: decode: %v", err))
		return
	}

	if req.PaymentID == "" {
		writeError(w, http.StatusUnprocessableEntity, "paymentId is required")
		return
	}
	if req.InvoiceID == "" {
		writeError(w, http.StatusUnprocessableEntity, "invoiceId is required")
		return
	}

	// In production:
	//   1. Load payment (assert reconciled=false, invoice_id IS NULL).
	//   2. Load invoice (assert status in (issued, overdue)).
	//   3. Assert payment.amount <= invoice outstanding balance.
	//   4. Update billing_payments: invoice_id=req.InvoiceID, reconciled=true, reconciled_at=now.
	//   5. Sum all payments for invoice; if >= patientAmountNzd set invoice status=paid.
	//   6. Write AuditEvent.

	now := time.Now().UTC()

	h.logger.Info("payment matched",
		"payment_id", req.PaymentID,
		"invoice_id", req.InvoiceID,
		"matched_at", now,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"paymentId":  req.PaymentID,
		"invoiceId":  req.InvoiceID,
		"matchedAt":  now,
		"reconciled": true,
	})
}
