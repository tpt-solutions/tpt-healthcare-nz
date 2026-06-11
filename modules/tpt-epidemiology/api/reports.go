package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

// PublicHealthReport is an aggregate surveillance report submitted to ESR or the Ministry of Health.
// report_type values: weekly_surveillance | monthly_communicable | outbreak_summary
// report_period: ISO week "YYYY-Www", month "YYYY-MM", or an outbreak UUID for outbreak_summary
// submitted_to values: esr | moh
// status values: draft | submitted | acknowledged
type PublicHealthReport struct {
	ID             string     `json:"id"`
	ReportType     string     `json:"reportType"`
	ReportPeriod   string     `json:"reportPeriod"`
	Status         string     `json:"status"`
	DataSummary    any        `json:"dataSummary"` // JSONB: map[diseaseCode]caseCount + metadata
	SubmittedTo    *string    `json:"submittedTo"`
	Reference      *string    `json:"reference"` // acknowledgement reference from ESR/MoH
	TenantID       string     `json:"tenantId"`
	SubmittedAt    *time.Time `json:"submittedAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

const reportSelectCols = `id, report_type, report_period, status,
       data_summary, submitted_to, reference, tenant_id,
       submitted_at, acknowledged_at, created_at, updated_at`

func scanReport(row interface{ Scan(...any) error }, rp *PublicHealthReport) error {
	return row.Scan(
		&rp.ID, &rp.ReportType, &rp.ReportPeriod, &rp.Status,
		&rp.DataSummary, &rp.SubmittedTo, &rp.Reference, &rp.TenantID,
		&rp.SubmittedAt, &rp.AcknowledgedAt, &rp.CreatedAt, &rp.UpdatedAt,
	)
}

type reportHandler struct{ handlerDeps }

func (h *reportHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	q := r.URL.Query()
	reportType := q.Get("report_type")
	status := q.Get("status")

	var rows pgx.Rows
	var err error
	switch {
	case reportType != "" && status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+reportSelectCols+` FROM public_health_reports
			 WHERE tenant_id = @tenant_id AND report_type = @report_type AND status = @status
			 ORDER BY report_period DESC, created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "report_type": reportType, "status": status})
	case reportType != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+reportSelectCols+` FROM public_health_reports
			 WHERE tenant_id = @tenant_id AND report_type = @report_type
			 ORDER BY report_period DESC, created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "report_type": reportType})
	case status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+reportSelectCols+` FROM public_health_reports
			 WHERE tenant_id = @tenant_id AND status = @status
			 ORDER BY report_period DESC, created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": status})
	default:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+reportSelectCols+` FROM public_health_reports
			 WHERE tenant_id = @tenant_id ORDER BY report_period DESC, created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	reports := make([]PublicHealthReport, 0)
	for rows.Next() {
		var rp PublicHealthReport
		if err := scanReport(rows, &rp); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		reports = append(reports, rp)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, reports)
}

func (h *reportHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req PublicHealthReport
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ReportType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "reportType is required"})
		return
	}
	if req.ReportPeriod == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "reportPeriod is required"})
		return
	}
	if req.DataSummary == nil {
		req.DataSummary = map[string]any{}
	}
	var rp PublicHealthReport
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO public_health_reports
		    (report_type, report_period, status, data_summary, submitted_to, tenant_id)
		VALUES
		    (@report_type, @report_period, 'draft', @data_summary, @submitted_to, @tenant_id)
		RETURNING `+reportSelectCols,
		pgx.NamedArgs{
			"report_type":   req.ReportType,
			"report_period": req.ReportPeriod,
			"data_summary":  req.DataSummary,
			"submitted_to":  req.SubmittedTo,
			"tenant_id":     tenantID,
		}).Scan(
		&rp.ID, &rp.ReportType, &rp.ReportPeriod, &rp.Status,
		&rp.DataSummary, &rp.SubmittedTo, &rp.Reference, &rp.TenantID,
		&rp.SubmittedAt, &rp.AcknowledgedAt, &rp.CreatedAt, &rp.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "PublicHealthReport", rp.ID, "")
	writeJSON(w, http.StatusCreated, rp)
}

func (h *reportHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var rp PublicHealthReport
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+reportSelectCols+` FROM public_health_reports WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&rp.ID, &rp.ReportType, &rp.ReportPeriod, &rp.Status,
		&rp.DataSummary, &rp.SubmittedTo, &rp.Reference, &rp.TenantID,
		&rp.SubmittedAt, &rp.AcknowledgedAt, &rp.CreatedAt, &rp.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "public health report not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "PublicHealthReport", rp.ID, "")
	writeJSON(w, http.StatusOK, rp)
}

func (h *reportHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req PublicHealthReport
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var rp PublicHealthReport
	err := h.pool.QueryRow(r.Context(), `
		UPDATE public_health_reports
		SET data_summary = @data_summary,
		    submitted_to = @submitted_to,
		    updated_at   = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
		RETURNING `+reportSelectCols,
		pgx.NamedArgs{
			"data_summary": req.DataSummary,
			"submitted_to": req.SubmittedTo,
			"id":           id,
			"tenant_id":    tenantID,
		}).Scan(
		&rp.ID, &rp.ReportType, &rp.ReportPeriod, &rp.Status,
		&rp.DataSummary, &rp.SubmittedTo, &rp.Reference, &rp.TenantID,
		&rp.SubmittedAt, &rp.AcknowledgedAt, &rp.CreatedAt, &rp.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "public health report not found or not in draft status"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "PublicHealthReport", rp.ID, "")
	writeJSON(w, http.StatusOK, rp)
}

// Submit transitions the report to submitted and optionally records a submission reference.
func (h *reportHandler) Submit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var body struct {
		Reference   *string `json:"reference"`
		SubmittedTo *string `json:"submittedTo"`
	}
	_ = decodeJSON(r, &body)

	tag, err := h.pool.Exec(r.Context(), `
		UPDATE public_health_reports
		SET status       = 'submitted',
		    reference    = COALESCE(@reference, reference),
		    submitted_to = COALESCE(@submitted_to, submitted_to),
		    submitted_at = now(),
		    updated_at   = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
	`, pgx.NamedArgs{
		"reference":    body.Reference,
		"submitted_to": body.SubmittedTo,
		"id":           id,
		"tenant_id":    tenantID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "report must be in draft status to submit"})
		return
	}
	h.recordAudit(r, "update", "PublicHealthReport", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

// Acknowledge records that ESR or MoH has acknowledged receipt of the report.
func (h *reportHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var body struct {
		Reference *string `json:"reference"`
	}
	_ = decodeJSON(r, &body)

	tag, err := h.pool.Exec(r.Context(), `
		UPDATE public_health_reports
		SET status          = 'acknowledged',
		    reference       = COALESCE(@reference, reference),
		    acknowledged_at = now(),
		    updated_at      = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'submitted'
	`, pgx.NamedArgs{"reference": body.Reference, "id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "report must be in submitted status to acknowledge"})
		return
	}
	h.recordAudit(r, "update", "PublicHealthReport", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}
