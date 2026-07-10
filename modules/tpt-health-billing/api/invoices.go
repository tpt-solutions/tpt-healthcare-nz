package api

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/payment"
	"github.com/jackc/pgx/v5/pgxpool"
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
	pool            *pgxpool.Pool
	enc             *encryption.Encryptor
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

	q := newQueryDB(h.pool, h.enc)
	invoices, err := q.listInvoices(r.Context(), tenantID, status, sourceModule, fundingType)
	if err != nil {
		h.logger.Error("list invoices", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list invoices")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"invoices": invoices,
		"total":    len(invoices),
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
	invoiceID := fmt.Sprintf("inv-%d", now.UnixNano())
	lines := make([]InvoiceLine, len(req.Lines))
	for i, l := range req.Lines {
		lines[i] = l
		lines[i].ID = fmt.Sprintf("%s-l%d", invoiceID, i+1)
	}

	inv := Invoice{
		ID:               invoiceID,
		TenantID:         req.TenantID,
		SourceModule:     req.SourceModule,
		SourceRefID:      req.SourceRefID,
		PatientNHI:       req.PatientNHI,
		FundingType:      req.FundingType,
		Status:           InvoiceStatusDraft,
		Lines:            lines,
		TotalAmountNZD:   total,
		SubsidyAmountNZD: subsidy,
		PatientAmountNZD: patientAmount,
		Notes:            req.Notes,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	q := newQueryDB(h.pool, h.enc)
	if err := q.insertInvoice(r.Context(), inv, lines); err != nil {
		h.logger.Error("insert invoice", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create invoice")
		return
	}

	h.logger.Info("invoice created",
		"invoice_id", inv.ID,
		"source_module", inv.SourceModule,
		"total_nzd", inv.TotalAmountNZD,
		"patient_amount_nzd", inv.PatientAmountNZD,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, inv)
}

// Get handles GET /api/v1/invoices/{id}.
func (h *InvoiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	h.logger.Info("get invoice", "invoice_id", id, "request_id", r.Context().Value(requestIDKey))

	q := newQueryDB(h.pool, h.enc)
	inv, err := q.getInvoice(r.Context(), id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "invoice not found")
			return
		}
		h.logger.Error("get invoice", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get invoice")
		return
	}

	writeJSON(w, http.StatusOK, inv)
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

	dueAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	q := newQueryDB(h.pool, h.enc)
	if err := q.issueInvoice(r.Context(), id, dueAt); err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "invoice not found or not in draft status")
			return
		}
		h.logger.Error("issue invoice", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to issue invoice")
		return
	}

	h.logger.Info("invoice issued", "invoice_id", id, "issued_at", time.Now().UTC(), "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"invoiceId": id,
		"status":    string(InvoiceStatusIssued),
		"issuedAt":  time.Now().UTC(),
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

	q := newQueryDB(h.pool, h.enc)
	if err := q.cancelInvoice(r.Context(), id); err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "invoice not found or not cancellable")
			return
		}
		h.logger.Error("cancel invoice", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to cancel invoice")
		return
	}

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

	q := newQueryDB(h.pool, h.enc)

	// Verify invoice exists and is payable
	inv, err := q.getInvoice(r.Context(), id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "invoice not found")
			return
		}
		h.logger.Error("get invoice for payment", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to process payment")
		return
	}
	if inv.Status != InvoiceStatusIssued && inv.Status != InvoiceStatusOverdue {
		writeError(w, http.StatusConflict, "invoice must be issued or overdue to accept payment")
		return
	}

	now := time.Now().UTC()
	pmt := Payment{
		ID:            fmt.Sprintf("pay-%d", now.UnixNano()),
		TenantID:      inv.TenantID,
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

	if err := q.insertPayment(r.Context(), pmt); err != nil {
		h.logger.Error("insert payment", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to record payment")
		return
	}

	// Check if fully paid
	totalPaid, err := q.sumPayments(r.Context(), id)
	if err != nil {
		h.logger.Error("sum payments", "error", err)
	} else if totalPaid >= inv.PatientAmountNZD {
		_ = q.updateInvoiceStatus(r.Context(), id, InvoiceStatusPaid)
	}

	h.logger.Info("payment recorded",
		"payment_id", pmt.ID,
		"invoice_id", id,
		"amount_nzd", pmt.AmountNZD,
		"method", pmt.PaymentMethod,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, pmt)
}

// ListPayments handles GET /api/v1/invoices/{id}/payments — list payments on an invoice.
func (h *InvoiceHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	h.logger.Info("list payments", "invoice_id", id, "request_id", r.Context().Value(requestIDKey))

	q := newQueryDB(h.pool, h.enc)
	payments, err := q.listPayments(r.Context(), id)
	if err != nil {
		h.logger.Error("list payments", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list payments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"payments": payments,
		"total":    len(payments),
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

	q := newQueryDB(h.pool, h.enc)
	inv, err := q.getInvoice(r.Context(), id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "invoice not found")
			return
		}
		h.logger.Error("get invoice for payment", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to initiate payment")
		return
	}

	amountCents := int64(inv.PatientAmountNZD * 100)

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
		amountNZD := math.Round(float64(event.AmountCents)/100*100) / 100
		pmt := Payment{
			ID:            fmt.Sprintf("pay-wh-%d", time.Now().UTC().UnixNano()),
			TenantID:      "",
			InvoiceID:     event.Reference,
			PaymentMethod: PaymentInternetBanking,
			AmountNZD:     amountNZD,
			Reference:     event.ExternalID,
			PaymentDate:   time.Now().UTC(),
			Reconciled:    true,
			CreatedAt:     time.Now().UTC(),
		}
		if event.Reference != "" {
			q := newQueryDB(h.pool, h.enc)
			inv, err := q.getInvoice(r.Context(), event.Reference)
			if err == nil {
				pmt.TenantID = inv.TenantID
			}
			if insertErr := q.insertPayment(r.Context(), pmt); insertErr != nil {
				h.logger.Error("insert webhook payment", "error", insertErr)
			}
		}
		h.logger.Info("payment succeeded",
			"external_id", event.ExternalID,
			"amount_nzd", amountNZD,
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
