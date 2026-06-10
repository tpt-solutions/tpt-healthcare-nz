package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// FundingSource identifies how the admission is funded.
type FundingSource string

const (
	FundingSourceDHB     FundingSource = "dhb"      // District Health Board / Te Whatu Ora
	FundingSourceACC     FundingSource = "acc"
	FundingSourcePrivate FundingSource = "private"   // health insurance or self-pay
	FundingSourceMixed   FundingSource = "mixed"
)

// InvoiceStatus tracks the hospital invoice lifecycle.
type InvoiceStatus string

const (
	InvoiceStatusDraft     InvoiceStatus = "draft"
	InvoiceStatusSubmitted InvoiceStatus = "submitted"
	InvoiceStatusPaid      InvoiceStatus = "paid"
	InvoiceStatusRejected  InvoiceStatus = "rejected"
)

// DRGAssignment is the result of DRG (Diagnosis Related Group) grouping for an admission.
// In NZ this maps to the AR-DRG (Australian Refined DRG) classification used by MoH.
type DRGAssignment struct {
	AdmissionID string    `json:"admissionId"`
	MDC         string    `json:"mdc"`          // Major Diagnostic Category e.g. "05"
	ARDRG       string    `json:"arDrg"`        // AR-DRG code e.g. "F74A"
	Description string    `json:"description"`  // e.g. "Chest Pain"
	Complexity  string    `json:"complexity"`   // A (most), B, C, Z (no CC)
	BaseDays    int       `json:"baseDays"`     // trim points used for outlier calculation
	BasePrice   float64   `json:"basePrice"`    // NZD, from published NZ casemix price
	LengthOfStay int      `json:"lengthOfStay"` // days
	Outlier     bool      `json:"outlier"`      // true if stay > high trim threshold
	TenantID    string    `json:"tenantId"`
	GeneratedAt time.Time `json:"generatedAt"`
}

// HospitalInvoiceLine is a single billable item on a hospital invoice.
type HospitalInvoiceLine struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
	TotalPrice  float64 `json:"totalPrice"`
}

// HospitalInvoice is the billing document for an inpatient admission.
type HospitalInvoice struct {
	ID            string                `json:"id"`
	AdmissionID   string                `json:"admissionId"`
	PatientID     string                `json:"patientId"`
	ARDRG         string                `json:"arDrg,omitempty"`
	FundingSource FundingSource         `json:"fundingSource"`
	Status        InvoiceStatus         `json:"status"`
	Lines         []HospitalInvoiceLine `json:"lines"`
	SubtotalNZD   float64               `json:"subtotalNzd"`
	GSTAmountNZD  float64               `json:"gstAmountNzd"`
	TotalNZD      float64               `json:"totalNzd"`
	Notes         string                `json:"notes,omitempty"`
	TenantID      string                `json:"tenantId"`
	SubmittedAt   *time.Time            `json:"submittedAt,omitempty"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
}

type invoiceCreateRequest struct {
	FundingSource FundingSource         `json:"fundingSource"`
	Lines         []HospitalInvoiceLine `json:"lines"`
	Notes         string                `json:"notes,omitempty"`
}

// BillingHandler handles hospital billing and DRG grouping routes.
type BillingHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// GetDRG handles GET /api/v1/admissions/{admissionId}/drg.
// Derives the AR-DRG from the coded diagnoses and procedures on the admission.
func (h *BillingHandler) GetDRG(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")

	drg, err := h.deriveDRG(ctx, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "no coded diagnoses found for this admission"})
			return
		}
		h.logger.Error("derive DRG", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DRG_ERROR", Message: "failed to derive DRG"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "DRGAssignment",
		ResourceID: admissionID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, drg)
}

// GetInvoice handles GET /api/v1/admissions/{admissionId}/invoice.
func (h *BillingHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	invoice, err := h.getInvoiceByAdmission(ctx, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "no invoice found for this admission"})
			return
		}
		h.logger.Error("get hospital invoice", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve invoice"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "HospitalInvoice",
		ResourceID: admissionID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, invoice)
}

// CreateInvoice handles POST /api/v1/admissions/{admissionId}/invoice.
func (h *BillingHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")

	var req invoiceCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.FundingSource == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FUNDING", Message: "fundingSource is required"})
		return
	}
	if len(req.Lines) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_LINES", Message: "at least one invoice line is required"})
		return
	}

	invoice, err := h.insertInvoice(ctx, admissionID, req, tenantID)
	if err != nil {
		h.logger.Error("create hospital invoice", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create invoice"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "HospitalInvoice",
		ResourceID: invoice.ID, TenantID: tenantID,
	})
	writeJSON(w, http.StatusCreated, invoice)
}

// SubmitInvoice handles POST /api/v1/admissions/{admissionId}/invoice/submit.
func (h *BillingHandler) SubmitInvoice(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	existing, err := h.getInvoiceByAdmission(ctx, admissionID, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "no invoice found for this admission"})
			return
		}
		h.logger.Error("get invoice for submit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve invoice"})
		return
	}
	if existing.Status != InvoiceStatusDraft {
		writeJSON(w, http.StatusConflict, apiError{Code: "NOT_DRAFT", Message: "only a draft invoice can be submitted"})
		return
	}

	now := time.Now().UTC()
	submitted, err := h.submitInvoice(ctx, existing.ID, now, tenantID)
	if err != nil {
		h.logger.Error("submit hospital invoice", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SUBMIT_ERROR", Message: "failed to submit invoice"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "HospitalInvoice",
		ResourceID: existing.ID, TenantID: tenantID, Metadata: map[string]string{"action": "submit"},
	})
	writeJSON(w, http.StatusOK, submitted)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

// deriveDRG computes the AR-DRG grouping from the principal diagnosis on the
// admission. Full ARDRG grouping logic would call the core/terminology/ DRG
// grouper; this implementation derives from stored codes and MoH price weights.
func (h *BillingHandler) deriveDRG(ctx context.Context, admissionID, tenantID string) (DRGAssignment, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT ha.id, ha.patient_id, ha.admitted_at, ha.discharged_at,
		        cc.code AS principal_diagnosis
		 FROM hospital_admissions ha
		 LEFT JOIN clinical_codes cc ON cc.admission_id = ha.id
		     AND cc.code_type = 'principal-diagnosis'
		     AND cc.tenant_id = ha.tenant_id
		 WHERE ha.id = @admission_id AND ha.tenant_id = @tenant_id
		 ORDER BY cc.coded_at ASC
		 LIMIT 1`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID},
	)

	var (
		admID, patientID string
		admittedAt       time.Time
		dischargedAt     *time.Time
		principalDx      *string
	)
	if err := row.Scan(&admID, &patientID, &admittedAt, &dischargedAt, &principalDx); err != nil {
		if db.IsNoRows(err) {
			return DRGAssignment{}, errNotFound
		}
		return DRGAssignment{}, fmt.Errorf("derive DRG query: %w", err)
	}

	var los int
	if dischargedAt != nil {
		los = int(dischargedAt.Sub(admittedAt).Hours() / 24)
	}

	drg := DRGAssignment{
		AdmissionID: admissionID,
		LengthOfStay: los,
		GeneratedAt: time.Now().UTC(),
		TenantID:    tenantID,
	}

	// Placeholder DRG grouping from principal diagnosis first character (MDC mapping).
	// A production implementation would call core/terminology/drg_grouper.go.
	if principalDx != nil && len(*principalDx) > 0 {
		switch (*principalDx)[0] {
		case 'I':
			drg.MDC = "05"
			drg.ARDRG = "F62B"
			drg.Description = "Heart Failure & Shock"
			drg.Complexity = "B"
			drg.BaseDays = 4
			drg.BasePrice = 4250.00
		case 'J':
			drg.MDC = "04"
			drg.ARDRG = "E65B"
			drg.Description = "Respiratory Signs & Symptoms"
			drg.Complexity = "B"
			drg.BaseDays = 3
			drg.BasePrice = 2800.00
		case 'S', 'T':
			drg.MDC = "21"
			drg.ARDRG = "W60B"
			drg.Description = "Other Injury"
			drg.Complexity = "B"
			drg.BaseDays = 2
			drg.BasePrice = 2100.00
		default:
			drg.MDC = "99"
			drg.ARDRG = "Z64B"
			drg.Description = "Other Factors Influencing Health Status"
			drg.Complexity = "B"
			drg.BaseDays = 2
			drg.BasePrice = 1800.00
		}
	} else {
		return DRGAssignment{}, errNotFound
	}

	drg.Outlier = los > drg.BaseDays*3
	return drg, nil
}

func (h *BillingHandler) getInvoiceByAdmission(ctx context.Context, admissionID, tenantID string) (HospitalInvoice, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, admission_id, patient_id, ar_drg, funding_source, status,
		        lines, subtotal_nzd, gst_amount_nzd, total_nzd, notes,
		        tenant_id, submitted_at, created_at, updated_at
		 FROM hospital_invoices
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		 ORDER BY created_at DESC
		 LIMIT 1`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID},
	)
	var inv HospitalInvoice
	if err := row.Scan(
		&inv.ID, &inv.AdmissionID, &inv.PatientID, &inv.ARDRG,
		&inv.FundingSource, &inv.Status, &inv.Lines,
		&inv.SubtotalNZD, &inv.GSTAmountNZD, &inv.TotalNZD, &inv.Notes,
		&inv.TenantID, &inv.SubmittedAt, &inv.CreatedAt, &inv.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return HospitalInvoice{}, errNotFound
		}
		return HospitalInvoice{}, fmt.Errorf("get hospital invoice: %w", err)
	}
	return inv, nil
}

func (h *BillingHandler) insertInvoice(ctx context.Context, admissionID string, req invoiceCreateRequest, tenantID string) (HospitalInvoice, error) {
	var subtotal float64
	for i := range req.Lines {
		req.Lines[i].TotalPrice = req.Lines[i].Quantity * req.Lines[i].UnitPrice
		subtotal += req.Lines[i].TotalPrice
	}
	gst := subtotal * 0.15 // NZ GST 15%
	total := subtotal + gst

	// Fetch patient_id from the admission.
	var patientID string
	if err := h.pool.QueryRow(ctx,
		`SELECT patient_id FROM hospital_admissions WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": admissionID, "tenant_id": tenantID},
	).Scan(&patientID); err != nil {
		if db.IsNoRows(err) {
			return HospitalInvoice{}, errNotFound
		}
		return HospitalInvoice{}, fmt.Errorf("get admission for invoice: %w", err)
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO hospital_invoices
		   (admission_id, patient_id, funding_source, status, lines,
		    subtotal_nzd, gst_amount_nzd, total_nzd, notes, tenant_id)
		 VALUES
		   (@admission_id, @patient_id, @funding_source, @status, @lines,
		    @subtotal_nzd, @gst_amount_nzd, @total_nzd, @notes, @tenant_id)
		 RETURNING id, admission_id, patient_id, ar_drg, funding_source, status,
		           lines, subtotal_nzd, gst_amount_nzd, total_nzd, notes,
		           tenant_id, submitted_at, created_at, updated_at`,
		db.NamedArgs{
			"admission_id":   admissionID,
			"patient_id":     patientID,
			"funding_source": req.FundingSource,
			"status":         InvoiceStatusDraft,
			"lines":          req.Lines,
			"subtotal_nzd":   subtotal,
			"gst_amount_nzd": gst,
			"total_nzd":      total,
			"notes":          req.Notes,
			"tenant_id":      tenantID,
		},
	)
	var inv HospitalInvoice
	if err := row.Scan(
		&inv.ID, &inv.AdmissionID, &inv.PatientID, &inv.ARDRG,
		&inv.FundingSource, &inv.Status, &inv.Lines,
		&inv.SubtotalNZD, &inv.GSTAmountNZD, &inv.TotalNZD, &inv.Notes,
		&inv.TenantID, &inv.SubmittedAt, &inv.CreatedAt, &inv.UpdatedAt,
	); err != nil {
		return HospitalInvoice{}, fmt.Errorf("insert hospital invoice: %w", err)
	}
	return inv, nil
}

func (h *BillingHandler) submitInvoice(ctx context.Context, invoiceID string, submittedAt time.Time, tenantID string) (HospitalInvoice, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hospital_invoices
		 SET status = @status, submitted_at = @submitted_at, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, admission_id, patient_id, ar_drg, funding_source, status,
		           lines, subtotal_nzd, gst_amount_nzd, total_nzd, notes,
		           tenant_id, submitted_at, created_at, updated_at`,
		db.NamedArgs{
			"status":       InvoiceStatusSubmitted,
			"submitted_at": submittedAt,
			"id":           invoiceID,
			"tenant_id":    tenantID,
		},
	)
	var inv HospitalInvoice
	if err := row.Scan(
		&inv.ID, &inv.AdmissionID, &inv.PatientID, &inv.ARDRG,
		&inv.FundingSource, &inv.Status, &inv.Lines,
		&inv.SubtotalNZD, &inv.GSTAmountNZD, &inv.TotalNZD, &inv.Notes,
		&inv.TenantID, &inv.SubmittedAt, &inv.CreatedAt, &inv.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return HospitalInvoice{}, errNotFound
		}
		return HospitalInvoice{}, fmt.Errorf("submit hospital invoice: %w", err)
	}
	return inv, nil
}
