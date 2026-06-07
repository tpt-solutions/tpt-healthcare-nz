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
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// PHOReportType enumerates the report types submitted to the PHO.
type PHOReportType string

const (
	// PHOReportCapitation is the monthly capitation extract — lists all enrolled
	// patients for the period, used to calculate the capitation payment from the PHO.
	PHOReportCapitation PHOReportType = "capitation"

	// PHOReportFFS is the Fee-for-Service extract — lists individual consultations
	// that attract a per-visit subsidy (e.g. under-25s, CSC holders, maternity).
	PHOReportFFS PHOReportType = "ffs"
)

// PHOReportStatus tracks the submission lifecycle.
type PHOReportStatus string

const (
	PHOReportStatusDraft     PHOReportStatus = "draft"
	PHOReportStatusSubmitted PHOReportStatus = "submitted"
	PHOReportStatusAccepted  PHOReportStatus = "accepted"
	PHOReportStatusRejected  PHOReportStatus = "rejected"
)

// PHOReport is a period extract submitted to the practice's PHO.
type PHOReport struct {
	ID            string          `json:"id"`
	Type          PHOReportType   `json:"type"`
	Period        string          `json:"period"`       // YYYY-MM
	Status        PHOReportStatus `json:"status"`
	RecordCount   int             `json:"recordCount"`
	PHOReference  string          `json:"phoReference,omitempty"` // Reference assigned by PHO on submission
	RejectionNote string          `json:"rejectionNote,omitempty"`
	TenantID      string          `json:"tenantId"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	SubmittedAt   *time.Time      `json:"submittedAt,omitempty"`
}

// PHOCapitationRecord is a single row in a capitation extract.
// PatientNHI is intentionally omitted from the JSON response; NHI is PHI and
// must only be included in the encrypted submission to the PHO, never in the
// API response visible to any authenticated caller (HIPC Rule 11/12).
type PHOCapitationRecord struct {
	PatientID    string `json:"patientId"`    // internal opaque identifier
	EnrolledDate string `json:"enrolledDate"` // YYYY-MM-DD
	FundingCode  string `json:"fundingCode"`
}

// PHOFFSRecord is a single row in a FFS extract.
// PatientNHI is omitted from JSON for the same reason as PHOCapitationRecord.
type PHOFFSRecord struct {
	PatientID     string    `json:"patientId"`     // internal opaque identifier
	VisitDate     time.Time `json:"visitDate"`
	FundingCode   string    `json:"fundingCode"`
	DiagnosisCode string    `json:"diagnosisCode"` // ICD-10-AM
	ProviderHPI   string    `json:"providerHpi"`
}

// phoGenerateRequest is the body for POST /api/v1/pho/reports.
type phoGenerateRequest struct {
	Type   PHOReportType `json:"type"`
	Period string        `json:"period"` // YYYY-MM
}

// PHOHandler handles all /api/v1/pho routes.
type PHOHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListReports handles GET /api/v1/pho/reports.
// Returns all PHO reports for the practice tenant.
func (h *PHOHandler) ListReports(w http.ResponseWriter, r *http.Request) {
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
	typeFilter := q.Get("type")
	statusFilter := q.Get("status")

	reports, err := h.listReports(ctx, tenantID, typeFilter, statusFilter)
	if err != nil {
		h.logger.Error("list PHO reports", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list PHO reports"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "PHOReport",
		ResourceID:   "list",
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"reports": reports,
		"total":   len(reports),
	})
}

// GenerateReport handles POST /api/v1/pho/reports.
// Generates a capitation or FFS extract for the given period, counting the
// relevant records. The generated report starts in "draft" status.
func (h *PHOHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
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

	var req phoGenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if err := validatePHOGenerate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	// Count the records for this period before inserting the report shell.
	recordCount, err := h.countPeriodRecords(ctx, tenantID, req.Type, req.Period)
	if err != nil {
		h.logger.Error("count PHO period records", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "COUNT_ERROR", Message: "failed to count records for period"})
		return
	}

	report, err := h.insertReport(ctx, req, recordCount, tenantID)
	if err != nil {
		h.logger.Error("insert PHO report", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to generate PHO report"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "PHOReport",
		ResourceID:   report.ID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"type": string(req.Type), "period": req.Period},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, report)
}

// GetReport handles GET /api/v1/pho/reports/{id}.
func (h *PHOHandler) GetReport(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "report ID is required"})
		return
	}

	report, err := h.getReportByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "PHO report not found"})
			return
		}
		h.logger.Error("get PHO report", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve PHO report"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "PHOReport",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, report)
}

// SubmitReport handles POST /api/v1/pho/reports/{id}/submit.
// Marks the draft report as submitted, transitioning its status.
// In production this would transmit the extract to the PHO via their API.
func (h *PHOHandler) SubmitReport(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "report ID is required"})
		return
	}

	report, err := h.getReportByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "PHO report not found"})
			return
		}
		h.logger.Error("get PHO report for submit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve PHO report"})
		return
	}

	if report.Status != PHOReportStatusDraft {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "ALREADY_SUBMITTED",
			Message: fmt.Sprintf("report is already in %s status", report.Status),
		})
		return
	}

	// Generate a stub PHO reference. Production calls the PHO's submission API.
	phoRef := fmt.Sprintf("PHO-%s-%s", report.Type, report.Period)
	now := time.Now().UTC()
	submitted, err := h.markReportSubmitted(ctx, id, phoRef, now, tenantID)
	if err != nil {
		h.logger.Error("mark PHO report submitted", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SUBMIT_ERROR", Message: "failed to submit PHO report"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "PHOReport",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "submit", "pho_reference": phoRef},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, submitted)
}

// GetCapitationRecords handles GET /api/v1/pho/reports/{id}/records.
// Returns the individual capitation or FFS records for a generated report.
func (h *PHOHandler) GetCapitationRecords(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "report ID is required"})
		return
	}

	report, err := h.getReportByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "PHO report not found"})
			return
		}
		h.logger.Error("get PHO report for records", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve PHO report"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "PHOReport",
		ResourceID:   id + "/records",
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	switch report.Type {
	case PHOReportCapitation:
		records, err := h.fetchCapitationRecords(ctx, tenantID, report.Period)
		if err != nil {
			h.logger.Error("fetch capitation records", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "FETCH_ERROR", Message: "failed to fetch capitation records"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"records": records, "total": len(records)})

	case PHOReportFFS:
		records, err := h.fetchFFSRecords(ctx, tenantID, report.Period)
		if err != nil {
			h.logger.Error("fetch FFS records", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "FETCH_ERROR", Message: "failed to fetch FFS records"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"records": records, "total": len(records)})

	default:
		writeJSON(w, http.StatusBadRequest, apiError{Code: "UNKNOWN_TYPE", Message: fmt.Sprintf("unknown report type %q", report.Type)})
	}
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func validatePHOGenerate(req *phoGenerateRequest) error {
	if req.Type != PHOReportCapitation && req.Type != PHOReportFFS {
		return fmt.Errorf("type must be %q or %q", PHOReportCapitation, PHOReportFFS)
	}
	if len(req.Period) != 7 || req.Period[4] != '-' {
		return fmt.Errorf("period must be in YYYY-MM format (e.g. 2026-05)")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data access
// ---------------------------------------------------------------------------

func (h *PHOHandler) listReports(
	ctx context.Context,
	tenantID, typeFilter, statusFilter string,
) ([]PHOReport, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, type, period, status, record_count,
		        pho_reference, rejection_note,
		        tenant_id, created_at, updated_at, submitted_at
		 FROM pho_reports
		 WHERE tenant_id = @tenant_id
		   AND (@type_filter   = '' OR type   = @type_filter)
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY created_at DESC
		 LIMIT 100`,
		db.NamedArgs{
			"tenant_id":     tenantID,
			"type_filter":   typeFilter,
			"status_filter": statusFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query PHO reports: %w", err)
	}
	defer rows.Close()

	var results []PHOReport
	for rows.Next() {
		var rep PHOReport
		if err := rows.Scan(
			&rep.ID, &rep.Type, &rep.Period, &rep.Status, &rep.RecordCount,
			&rep.PHOReference, &rep.RejectionNote,
			&rep.TenantID, &rep.CreatedAt, &rep.UpdatedAt, &rep.SubmittedAt,
		); err != nil {
			return nil, fmt.Errorf("scan PHO report: %w", err)
		}
		results = append(results, rep)
	}
	return results, rows.Err()
}

func (h *PHOHandler) getReportByID(ctx context.Context, id, tenantID string) (PHOReport, error) {
	var rep PHOReport
	err := h.pool.QueryRow(ctx,
		`SELECT id, type, period, status, record_count,
		        pho_reference, rejection_note,
		        tenant_id, created_at, updated_at, submitted_at
		 FROM pho_reports
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&rep.ID, &rep.Type, &rep.Period, &rep.Status, &rep.RecordCount,
		&rep.PHOReference, &rep.RejectionNote,
		&rep.TenantID, &rep.CreatedAt, &rep.UpdatedAt, &rep.SubmittedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return PHOReport{}, errNotFound
		}
		return PHOReport{}, fmt.Errorf("get PHO report: %w", err)
	}
	return rep, nil
}

// countPeriodRecords counts the relevant records for capitation or FFS in a given period.
func (h *PHOHandler) countPeriodRecords(ctx context.Context, tenantID string, reportType PHOReportType, period string) (int, error) {
	switch reportType {
	case PHOReportCapitation:
		// Count distinct enrolled patients active during the period.
		var count int
		err := h.pool.QueryRow(ctx,
			`SELECT COUNT(DISTINCT patient_id)
			 FROM nes_enrolments
			 WHERE tenant_id = @tenant_id
			   AND enrolment_start <= (@period || '-01')::date + interval '1 month - 1 day'
			   AND (enrolment_end IS NULL OR enrolment_end >= (@period || '-01')::date)`,
			db.NamedArgs{"tenant_id": tenantID, "period": period},
		).Scan(&count)
		if err != nil {
			return 0, fmt.Errorf("count capitation records: %w", err)
		}
		return count, nil

	case PHOReportFFS:
		// Count completed encounters in the period that attract a FFS subsidy.
		var count int
		err := h.pool.QueryRow(ctx,
			`SELECT COUNT(*)
			 FROM encounters
			 WHERE tenant_id = @tenant_id
			   AND status = 'completed'
			   AND date_trunc('month', started_at) = (@period || '-01')::date
			   AND ffs_eligible = true`,
			db.NamedArgs{"tenant_id": tenantID, "period": period},
		).Scan(&count)
		if err != nil {
			return 0, fmt.Errorf("count FFS records: %w", err)
		}
		return count, nil
	}
	return 0, fmt.Errorf("unknown report type: %s", reportType)
}

func (h *PHOHandler) insertReport(ctx context.Context, req phoGenerateRequest, recordCount int, tenantID string) (PHOReport, error) {
	var rep PHOReport
	err := h.pool.QueryRow(ctx,
		`INSERT INTO pho_reports (type, period, status, record_count, tenant_id)
		 VALUES (@type, @period, @status, @record_count, @tenant_id)
		 RETURNING id, type, period, status, record_count,
		           pho_reference, rejection_note,
		           tenant_id, created_at, updated_at, submitted_at`,
		db.NamedArgs{
			"type":         req.Type,
			"period":       req.Period,
			"status":       PHOReportStatusDraft,
			"record_count": recordCount,
			"tenant_id":    tenantID,
		},
	).Scan(
		&rep.ID, &rep.Type, &rep.Period, &rep.Status, &rep.RecordCount,
		&rep.PHOReference, &rep.RejectionNote,
		&rep.TenantID, &rep.CreatedAt, &rep.UpdatedAt, &rep.SubmittedAt,
	)
	if err != nil {
		return PHOReport{}, fmt.Errorf("insert PHO report: %w", err)
	}
	return rep, nil
}

func (h *PHOHandler) markReportSubmitted(ctx context.Context, id, phoRef string, submittedAt time.Time, tenantID string) (PHOReport, error) {
	var rep PHOReport
	err := h.pool.QueryRow(ctx,
		`UPDATE pho_reports
		 SET status        = @status,
		     pho_reference = @pho_reference,
		     submitted_at  = @submitted_at,
		     updated_at    = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, type, period, status, record_count,
		           pho_reference, rejection_note,
		           tenant_id, created_at, updated_at, submitted_at`,
		db.NamedArgs{
			"status":        PHOReportStatusSubmitted,
			"pho_reference": phoRef,
			"submitted_at":  submittedAt,
			"id":            id,
			"tenant_id":     tenantID,
		},
	).Scan(
		&rep.ID, &rep.Type, &rep.Period, &rep.Status, &rep.RecordCount,
		&rep.PHOReference, &rep.RejectionNote,
		&rep.TenantID, &rep.CreatedAt, &rep.UpdatedAt, &rep.SubmittedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return PHOReport{}, errNotFound
		}
		return PHOReport{}, fmt.Errorf("mark PHO report submitted: %w", err)
	}
	return rep, nil
}

func (h *PHOHandler) fetchCapitationRecords(ctx context.Context, tenantID, period string) ([]PHOCapitationRecord, error) {
	// Select distinct enrolled patients using patient_id (internal UUID) as the
	// dedup key. NHI is not selected here because it is PHI and must not be
	// exposed in the API response. NHI is only included in the encrypted
	// payload sent to the PHO via the submission API (HIPC Rules 11 and 12).
	rows, err := h.pool.Query(ctx,
		`SELECT DISTINCT ON (e.patient_id)
		        e.patient_id, e.enrolment_start::text, e.funding_code
		 FROM nes_enrolments e
		 WHERE e.tenant_id = @tenant_id
		   AND e.enrolment_start <= (@period || '-01')::date + interval '1 month - 1 day'
		   AND (e.enrolment_end IS NULL OR e.enrolment_end >= (@period || '-01')::date)
		 ORDER BY e.patient_id, e.enrolment_start`,
		db.NamedArgs{"tenant_id": tenantID, "period": period},
	)
	if err != nil {
		return nil, fmt.Errorf("fetch capitation records: %w", err)
	}
	defer rows.Close()

	var results []PHOCapitationRecord
	for rows.Next() {
		var rec PHOCapitationRecord
		if err := rows.Scan(&rec.PatientID, &rec.EnrolledDate, &rec.FundingCode); err != nil {
			return nil, fmt.Errorf("scan capitation record: %w", err)
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (h *PHOHandler) fetchFFSRecords(ctx context.Context, tenantID, period string) ([]PHOFFSRecord, error) {
	// patient_id (internal UUID) is used instead of patient_nhi to avoid
	// exposing NHI in the API response (HIPC Rules 11 and 12).
	rows, err := h.pool.Query(ctx,
		`SELECT enc.patient_id, enc.started_at, enc.ffs_funding_code,
		        enc.primary_diagnosis, enc.practitioner_hpi
		 FROM encounters enc
		 WHERE enc.tenant_id = @tenant_id
		   AND enc.status = 'completed'
		   AND date_trunc('month', enc.started_at) = (@period || '-01')::date
		   AND enc.ffs_eligible = true
		 ORDER BY enc.started_at`,
		db.NamedArgs{"tenant_id": tenantID, "period": period},
	)
	if err != nil {
		return nil, fmt.Errorf("fetch FFS records: %w", err)
	}
	defer rows.Close()

	var results []PHOFFSRecord
	for rows.Next() {
		var rec PHOFFSRecord
		if err := rows.Scan(&rec.PatientID, &rec.VisitDate, &rec.FundingCode, &rec.DiagnosisCode, &rec.ProviderHPI); err != nil {
			return nil, fmt.Errorf("scan FFS record: %w", err)
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}
