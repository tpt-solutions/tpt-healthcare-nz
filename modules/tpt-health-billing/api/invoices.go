package api

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/payment"
)

// --- Domain types ---

// InvoiceStatus reflects the invoice lifecycle.
type InvoiceStatus string

const (
	InvoiceStatusDraft      InvoiceStatus = "draft"
	InvoiceStatusIssued     InvoiceStatus = "issued"
	InvoiceStatusOverdue    InvoiceStatus = "overdue"
	InvoiceStatusPaid       InvoiceStatus = "paid"
	InvoiceStatusCancelled  InvoiceStatus = "cancelled"
	InvoiceStatusWrittenOff InvoiceStatus = "written_off"
)

// FundingType mirrors core/billing.FundingType to avoid an import dependency
// at the API layer (handlers work with JSON, not the core domain struct).
type FundingType string

const (
	FundingACC     FundingType = "ACC"
	FundingPHO     FundingType = "PHO"
	FundingPrivate FundingType = "PRIVATE"
	FundingDHB     FundingType = "DHB"
	FundingVAC     FundingType = "VAC"
)

// InvoiceLine is a single billable service on an invoice.
type InvoiceLine struct {
	ID               string      `json:"id"`
	ServiceCode      string      `json:"serviceCode"`
	Description      string      `json:"description"`
	FundingType      FundingType `json:"fundingType"`
	Quantity         int         `json:"quantity"`
	UnitFeeNZD       float64     `json:"unitFeeNzd"`
	SubsidyAmountNZD float64     `json:"subsidyAmountNzd"`
	ProviderHPI      string      `json:"providerHpi"`
	ServiceDate      time.Time   `json:"serviceDate"`
	DiagnosisCode    string      `json:"diagnosisCode,omitempty"`
	Notes            string      `json:"notes,omitempty"`
}

// Invoice is the cross-module billing document. Invoices may originate from any
// module (doctor, physio, pharmacy, radiology, etc.) and are consolidated here
// for AR management and downstream claim attachment.
type Invoice struct {
	ID               string        `json:"id"`
	TenantID         string        `json:"tenantId"`
	SourceModule     string        `json:"sourceModule"`
	SourceRefID      string        `json:"sourceRefId,omitempty"` // record ID in the originating module
	PatientNHI       string        `json:"patientNhi"`            // encrypted at rest
	FundingType      FundingType   `json:"fundingType"`
	Status           InvoiceStatus `json:"status"`
	Lines            []InvoiceLine `json:"lines"`
	TotalAmountNZD   float64       `json:"totalAmountNzd"`
	SubsidyAmountNZD float64       `json:"subsidyAmountNzd"`
	PatientAmountNZD float64       `json:"patientAmountNzd"`
	IssuedAt         *time.Time    `json:"issuedAt,omitempty"`
	DueAt            *time.Time    `json:"dueAt,omitempty"`
	PaidAt           *time.Time    `json:"paidAt,omitempty"`
	Notes            string        `json:"notes,omitempty"`
	CreatedAt        time.Time     `json:"createdAt"`
	UpdatedAt        time.Time     `json:"updatedAt"`
}

// CreateInvoiceRequest is the body for POST /api/v1/invoices.
type CreateInvoiceRequest struct {
	TenantID     string        `json:"tenantId"`
	SourceModule string        `json:"sourceModule"`
	SourceRefID  string        `json:"sourceRefId,omitempty"`
	PatientNHI   string        `json:"patientNhi"`
	FundingType  FundingType   `json:"fundingType"`
	Lines        []InvoiceLine `json:"lines"`
	Notes        string        `json:"notes,omitempty"`
}

// PaymentMethod identifies how a payment was made.
type PaymentMethod string

const (
	PaymentEFTPOS          PaymentMethod = "EFTPOS"
	PaymentCash            PaymentMethod = "CASH"
	PaymentInternetBanking PaymentMethod = "INTERNET_BANKING"
	PaymentDirectDebit     PaymentMethod = "DIRECT_DEBIT"
	PaymentInsurance       PaymentMethod = "INSURANCE"
	PaymentACC             PaymentMethod = "ACC"
	PaymentPHO             PaymentMethod = "PHO"
	PaymentDHB             PaymentMethod = "DHB"
	PaymentWriteOff        PaymentMethod = "WRITEOFF"
)

// Payment records a payment against an invoice.
type Payment struct {
	ID            string        `json:"id"`
	TenantID      string        `json:"tenantId"`
	InvoiceID     string        `json:"invoiceId"`
	PaymentMethod PaymentMethod `json:"paymentMethod"`
	AmountNZD     float64       `json:"amountNzd"`
	Reference     string        `json:"reference,omitempty"`
	Payer         string        `json:"payer,omitempty"`
	PaymentDate   time.Time     `json:"paymentDate"`
	Reconciled    bool          `json:"reconciled"`
	ReconciledAt  *time.Time    `json:"reconciledAt,omitempty"`
	Notes         string        `json:"notes,omitempty"`
	CreatedAt     time.Time     `json:"createdAt"`
}

// RecordPaymentRequest is the body for POST /api/v1/invoices/{id}/payments.
type RecordPaymentRequest struct {
	PaymentMethod PaymentMethod `json:"paymentMethod"`
	AmountNZD     float64       `json:"amountNzd"`
	Reference     string        `json:"reference,omitempty"`
	Payer         string        `json:"payer,omitempty"`
	PaymentDate   time.Time     `json:"paymentDate"`
	Notes         string        `json:"notes,omitempty"`
}

// InvoiceHandler handles all /api/v1/invoices/* routes.
type InvoiceHandler struct {
	logger          *slog.Logger
	paymentProvider payment.Provider
}

// WithPayment attaches a payment provider for online invoice payments (patient
// portal redirect) and EFTPOS terminal transactions.
func (h *InvoiceHandler) WithPayment(provider payment.Provider) *InvoiceHandler {
	h.paymentProvider = provider
	return h
}

// List handles GET /api/v1/invoices — list invoices with optional filters.
//
// Query parameters:
//   - status: filter by InvoiceStatus
//   - source_module: filter by originating module
//   - funding_type: filter by FundingType
//   - tenant_id: filter by tenant
func (h *InvoiceHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	sourceModule := r.URL.Query().Get("source_module")
	fundingType := r.URL.Query().Get("funding_type")
	tenantID := r.URL.Query().Get("tenant_id")

	h.logger.Info("list invoices",
		"status", status,
		"source_module", sourceModule,
		"funding_type", fundingType,
		"tenant_id", tenantID,
		"request_id", r.Context().Value(requestIDKey),
	)

	// In production: query billing_invoices with filters, cursor pagination.
	writeJSON(w, http.StatusOK, map[string]any{
		"invoices": []Invoice{},
		"total":    0,
	})
}

// Create handles POST /api/v1/invoices — create a draft invoice.
//
// Called by clinical modules when a billable encounter or service is finalised.
// The invoice is created in "draft" status; no charge is presented to the patient
// until Issue is called. Totals (TotalAmountNZD, SubsidyAmountNZD, PatientAmountNZD)
// are calculated server-side from the line items.
func (h *InvoiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("create invoice: decode: %v", err))
		return
	}

	if req.PatientNHI == "" {
		writeError(w, http.StatusUnprocessableEntity, "patientNhi is required")
		return
	}
	if req.SourceModule == "" {
		writeError(w, http.StatusUnprocessableEntity, "sourceModule is required")
		return
	}
	if len(req.Lines) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "at least one line item is required")
		return
	}

	// In production:
	//   1. Validate NHI checksum via core/nhi.
	//   2. Encrypt patientNhi with core/encryption before persisting.
	//   3. Calculate totals using core/billing.CalculateTotals.
	//   4. Persist invoice + lines (billing_invoices, billing_invoice_lines).
	//   5. Write AuditEvent.

	var total, subsidy float64
	for _, l := range req.Lines {
		lineTotal := l.UnitFeeNZD * float64(l.Quantity)
		lineSub := l.SubsidyAmountNZD * float64(l.Quantity)
		total += lineTotal
		subsidy += lineSub
	}
	patientAmount := total - subsidy
	if patientAmount < 0 {
		patientAmount = 0
	}

	now := time.Now().UTC()
	invoice := Invoice{
		ID:               fmt.Sprintf("inv-%d", now.UnixNano()),
		TenantID:         req.TenantID,
		SourceModule:     req.SourceModule,
		SourceRefID:      req.SourceRefID,
		PatientNHI:       req.PatientNHI,
		FundingType:      req.FundingType,
		Status:           InvoiceStatusDraft,
		Lines:            req.Lines,
		TotalAmountNZD:   total,
		SubsidyAmountNZD: subsidy,
		PatientAmountNZD: patientAmount,
		Notes:            req.Notes,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	h.logger.Info("invoice created",
		"invoice_id", invoice.ID,
		"source_module", invoice.SourceModule,
		"total_nzd", invoice.TotalAmountNZD,
		"patient_amount_nzd", invoice.PatientAmountNZD,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, invoice)
}

// Get handles GET /api/v1/invoices/{id}.
func (h *InvoiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	h.logger.Info("get invoice", "invoice_id", id, "request_id", r.Context().Value(requestIDKey))

	writeError(w, http.StatusNotFound, "invoice not found")
}

// Issue handles POST /api/v1/invoices/{id}/issue — finalise and send invoice to patient.
//
// Transitions the invoice from "draft" to "issued" and sets the due date.
// In production this triggers the notification pathway (email, SMS, patient portal)
// configured for the tenant.
func (h *InvoiceHandler) Issue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	// In production:
	//   1. Load invoice, assert status == draft.
	//   2. Set status=issued, issued_at=now, due_at=now+30days (configurable per tenant).
	//   3. Dispatch patient notification via core/email or core/sms outbox.
	//   4. Write AuditEvent.

	now := time.Now().UTC()

	h.logger.Info("invoice issued", "invoice_id", id, "issued_at", now, "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"invoiceId": id,
		"status":    string(InvoiceStatusIssued),
		"issuedAt":  now,
	})
}

// Cancel handles POST /api/v1/invoices/{id}/cancel — cancel an issued invoice.
//
// Only invoices in "draft" or "issued" status can be cancelled. Paid invoices
// require a credit note workflow instead.
func (h *InvoiceHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	// In production:
	//   1. Load invoice, assert status in (draft, issued).
	//   2. Set status=cancelled.
	//   3. If any insurance or ACC claims reference this invoice: reject the cancellation
	//      until claims are also cancelled.
	//   4. Write AuditEvent.

	h.logger.Info("invoice cancelled", "invoice_id", id, "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"invoiceId": id,
		"status":    string(InvoiceStatusCancelled),
	})
}

// RecordPayment handles POST /api/v1/invoices/{id}/payments — record a payment against an invoice.
//
// Multiple partial payments are supported (e.g., patient pays $20 cash, insurer pays
// the remainder). Once total payments >= PatientAmountNZD, the invoice status is
// automatically transitioned to "paid".
func (h *InvoiceHandler) RecordPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	var req RecordPaymentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("record payment: decode: %v", err))
		return
	}

	if req.AmountNZD <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "amountNzd must be greater than zero")
		return
	}
	if req.PaymentMethod == "" {
		writeError(w, http.StatusUnprocessableEntity, "paymentMethod is required")
		return
	}

	// In production:
	//   1. Load invoice, assert status in (issued, overdue).
	//   2. Insert payment record into billing_payments.
	//   3. Sum all payments for this invoice; if >= patientAmountNzd set invoice status=paid.
	//   4. Write AuditEvent.

	now := time.Now().UTC()
	payment := Payment{
		ID:            fmt.Sprintf("pay-%d", now.UnixNano()),
		InvoiceID:     id,
		PaymentMethod: req.PaymentMethod,
		AmountNZD:     req.AmountNZD,
		Reference:     req.Reference,
		Payer:         req.Payer,
		PaymentDate:   req.PaymentDate,
		Reconciled:    false,
		Notes:         req.Notes,
		CreatedAt:     now,
	}

	h.logger.Info("payment recorded",
		"payment_id", payment.ID,
		"invoice_id", id,
		"amount_nzd", payment.AmountNZD,
		"method", payment.PaymentMethod,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, payment)
}

// ListPayments handles GET /api/v1/invoices/{id}/payments — list payments on an invoice.
func (h *InvoiceHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	h.logger.Info("list payments", "invoice_id", id, "request_id", r.Context().Value(requestIDKey))

	// In production: query billing_payments where invoice_id = id.
	writeJSON(w, http.StatusOK, map[string]any{
		"payments": []Payment{},
		"total":    0,
	})
}

// InitiatePayment handles POST /api/v1/invoices/{id}/initiate-payment.
// Creates a payment intent via the configured payment provider (e.g. Windcave,
// Stripe). For redirect-based providers (Windcave, Paymark), the response
// includes a RedirectURL which the patient portal opens in the browser.
// For server-side providers (Stripe PI), only ExternalID is returned.
//
// Query parameter: return_url — the URL the provider should redirect to after
// the patient completes payment.
func (h *InvoiceHandler) InitiatePayment(w http.ResponseWriter, r *http.Request) {
	if h.paymentProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "payment provider not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	returnURL := r.URL.Query().Get("return_url")

	// In production: load the invoice from the database to get the real amount.
	// Using a placeholder 1000 cents (NZD $10.00) for the scaffold.
	amountCents := int64(1000)

	intent, err := h.paymentProvider.CreatePaymentRequest(r.Context(), payment.PaymentRequest{
		AmountCents: amountCents,
		Currency:    "NZD",
		InvoiceID:   id,
		Description: fmt.Sprintf("Invoice %s — TPT Health", id),
		ReturnURL:   returnURL,
	})
	if err != nil {
		h.logger.Error("initiate payment: create request",
			"invoice_id", id,
			"error", err,
			"request_id", r.Context().Value(requestIDKey),
		)
		writeError(w, http.StatusBadGateway, "payment provider error: "+err.Error())
		return
	}

	h.logger.Info("payment intent created",
		"invoice_id", id,
		"external_id", intent.ExternalID,
		"has_redirect", intent.RedirectURL != "",
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, map[string]any{
		"invoiceId":   id,
		"externalId":  intent.ExternalID,
		"redirectUrl": intent.RedirectURL,
	})
}

// HandlePaymentWebhook handles POST /api/v1/billing/webhooks/payment.
// Receives and verifies an inbound webhook event from the payment provider.
// On success it records the payment and transitions the invoice to "paid".
func (h *InvoiceHandler) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request) {
	if h.paymentProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "payment provider not configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read webhook body")
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Payment-Signature")

	event, err := h.paymentProvider.HandleWebhook(r.Context(), body, signature)
	if err != nil {
		h.logger.Warn("payment webhook: invalid signature or payload",
			"error", err,
			"request_id", r.Context().Value(requestIDKey),
		)
		writeError(w, http.StatusBadRequest, "invalid webhook: "+err.Error())
		return
	}

	h.logger.Info("payment webhook received",
		"type", event.Type,
		"external_id", event.ExternalID,
		"amount_cents", event.AmountCents,
		"reference", event.Reference,
		"request_id", r.Context().Value(requestIDKey),
	)

	switch event.Type {
	case payment.WebhookPaymentSucceeded:
		// In production:
		//   1. Look up billing_payment_intents by external_id to get invoice_id.
		//   2. Insert billing_payments record.
		//   3. Transition invoice to "paid" if fully settled.
		//   4. Write AuditEvent.
		h.logger.Info("payment succeeded",
			"external_id", event.ExternalID,
			"amount_nzd", math.Round(float64(event.AmountCents)/100*100)/100,
		)

	case payment.WebhookPaymentFailed:
		h.logger.Warn("payment failed",
			"external_id", event.ExternalID,
			"reference", event.Reference,
		)

	case payment.WebhookRefundCompleted:
		h.logger.Info("refund completed",
			"external_id", event.ExternalID,
			"amount_cents", event.AmountCents,
		)
	}

	w.WriteHeader(http.StatusOK)
}
