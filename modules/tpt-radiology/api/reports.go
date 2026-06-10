package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
)

// RadiologyReport is the domain model for a radiology diagnostic report.
type RadiologyReport struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenantId"`
	PatientNHI      string     `json:"patientNhi"`
	ImagingStudyID  *string    `json:"imagingStudyId,omitempty"`
	OrderID         *string    `json:"orderId,omitempty"`
	RadiologistHPI  string     `json:"radiologistHpi"`
	Status          string     `json:"status"`
	Findings        string     `json:"findings,omitempty"`
	Impression      string     `json:"impression,omitempty"`
	SignedAt        *time.Time `json:"signedAt,omitempty"`
	AmendedAt       *time.Time `json:"amendedAt,omitempty"`
	AmendmentReason string     `json:"amendmentReason,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type reportCreateRequest struct {
	PatientNHI     string  `json:"patientNhi"`
	RadiologistHPI string  `json:"radiologistHpi"`
	ImagingStudyID *string `json:"imagingStudyId,omitempty"`
	OrderID        *string `json:"orderId,omitempty"`
	Findings       string  `json:"findings,omitempty"`
	Impression     string  `json:"impression,omitempty"`
}

type reportUpdateRequest struct {
	Findings   *string `json:"findings,omitempty"`
	Impression *string `json:"impression,omitempty"`
}

type reportAmendRequest struct {
	Findings        string `json:"findings"`
	Impression      string `json:"impression"`
	AmendmentReason string `json:"amendmentReason"`
}

// ReportsHandler handles radiology report workflow routes.
type ReportsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/radiology-reports.
func (h *ReportsHandler) List(w http.ResponseWriter, r *http.Request) {
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
	reports, err := h.listReports(ctx, tenantID.String(), q.Get("patientNhi"), q.Get("status"))
	if err != nil {
		h.logger.Error("list radiology reports", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list reports"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "RadiologyReport",
		ResourceID:   "list",
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]any{"reports": reports, "total": len(reports)})
}

// Create handles POST /api/v1/radiology-reports.
func (h *ReportsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req reportCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "patientNhi is required"})
		return
	}
	if req.RadiologistHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_HPI", Message: "radiologistHpi is required"})
		return
	}

	report, err := h.insertReport(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert radiology report", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create report"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "RadiologyReport",
		ResourceID:   report.ID,
		TenantID:     tenantID,
		Details:      map[string]any{"patient_nhi": report.PatientNHI, "radiologist": report.RadiologistHPI},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusCreated, report)
}

// Get handles GET /api/v1/radiology-reports/{id}.
func (h *ReportsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	report, err := h.getReportByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "report not found"})
			return
		}
		h.logger.Error("get radiology report", slog.Any("error", err), slog.String("id", id))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve report"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "RadiologyReport",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, report)
}

// Update handles PUT /api/v1/radiology-reports/{id} — only allowed on drafts.
func (h *ReportsHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var req reportUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getReportByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "report not found"})
			return
		}
		h.logger.Error("get report for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve report"})
		return
	}

	if existing.Status == "final" || existing.Status == "cancelled" {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "cannot update a " + existing.Status + " report — use amend instead"})
		return
	}

	if req.Findings != nil {
		existing.Findings = *req.Findings
	}
	if req.Impression != nil {
		existing.Impression = *req.Impression
	}

	updated, err := h.updateReport(ctx, existing)
	if err != nil {
		h.logger.Error("update radiology report", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update report"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "RadiologyReport",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, updated)
}

// Sign handles POST /api/v1/radiology-reports/{id}/sign — finalises the report.
// The signing radiologist must hold a valid APC (checked by the caller in production).
func (h *ReportsHandler) Sign(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getReportByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "report not found"})
			return
		}
		h.logger.Error("get report for sign", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve report"})
		return
	}

	if existing.Status == "final" {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_SIGNED", Message: "report is already final"})
		return
	}
	if existing.Status == "cancelled" {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "cannot sign a cancelled report"})
		return
	}
	if existing.Impression == "" {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "MISSING_IMPRESSION", Message: "impression is required before signing"})
		return
	}

	now := time.Now().UTC()
	existing.Status = "final"
	existing.SignedAt = &now

	updated, err := h.updateReport(ctx, existing)
	if err != nil {
		h.logger.Error("sign radiology report", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SIGN_ERROR", Message: "failed to sign report"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "RadiologyReport",
		ResourceID:   id,
		TenantID:     tenantID,
		Details:      map[string]any{"action": "sign", "radiologist": existing.RadiologistHPI},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, updated)
}

// Amend handles POST /api/v1/radiology-reports/{id}/amend — creates an
// amended version of a final report. The previous content is replaced and
// status is set to "amended".
func (h *ReportsHandler) Amend(w http.ResponseWriter, r *http.Request) {
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
	var req reportAmendRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AmendmentReason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REASON", Message: "amendmentReason is required"})
		return
	}

	existing, err := h.getReportByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "report not found"})
			return
		}
		h.logger.Error("get report for amend", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve report"})
		return
	}

	if existing.Status != "final" && existing.Status != "amended" {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "only final or amended reports can be amended"})
		return
	}

	now := time.Now().UTC()
	existing.Status = "amended"
	existing.Findings = req.Findings
	existing.Impression = req.Impression
	existing.AmendmentReason = req.AmendmentReason
	existing.AmendedAt = &now

	updated, err := h.updateReport(ctx, existing)
	if err != nil {
		h.logger.Error("amend radiology report", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "AMEND_ERROR", Message: "failed to amend report"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "RadiologyReport",
		ResourceID:   id,
		TenantID:     tenantID,
		Details:      map[string]any{"action": "amend", "reason": req.AmendmentReason},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, updated)
}

// ---------------------------------------------------------------------------
// Database operations
// ---------------------------------------------------------------------------

func (h *ReportsHandler) listReports(ctx context.Context, tenantID, patientNHI, status string) ([]RadiologyReport, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, patient_nhi, imaging_study_id, order_id,
		        radiologist_hpi, status,
		        findings, impression,
		        signed_at, amended_at, amendment_reason,
		        created_at, updated_at
		 FROM radiology_reports
		 WHERE tenant_id = @tenant_id
		   AND (@patient_nhi = '' OR patient_nhi = @patient_nhi)
		   AND (@status      = '' OR status      = @status)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":  tenantID,
			"patient_nhi": patientNHI,
			"status":     status,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query radiology reports: %w", err)
	}
	defer rows.Close()

	var results []RadiologyReport
	for rows.Next() {
		r, err := h.scanReport(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (h *ReportsHandler) insertReport(ctx context.Context, req reportCreateRequest, tenantID string) (RadiologyReport, error) {
	reportID := uuid.New().String()

	// Build FHIR DiagnosticReport JSON for encrypted storage.
	fhirReport := r5.DiagnosticReport{
		ResourceType: "DiagnosticReport",
		ID:           reportID,
		Status:       "registered",
		Category: []r5.CodeableConcept{{
			Coding: []r5.Coding{{
				System:  "http://terminology.hl7.org/CodeSystem/v2-0074",
				Code:    "RAD",
				Display: "Radiology",
			}},
		}},
	}
	if req.ImagingStudyID != nil {
		fhirReport.ImagingStudy = []r5.Reference{{Reference: "ImagingStudy/" + *req.ImagingStudyID}}
	}

	fhirJSON, _ := json.Marshal(fhirReport)
	var encFHIR []byte
	if h.enc != nil {
		var err error
		encFHIR, err = h.enc.Encrypt(fhirJSON)
		if err != nil {
			return RadiologyReport{}, fmt.Errorf("encrypt fhir report: %w", err)
		}
	} else {
		encFHIR = fhirJSON
	}

	// Encrypt findings and impression separately to support partial reads.
	encFindings, err := h.encryptText(req.Findings)
	if err != nil {
		return RadiologyReport{}, fmt.Errorf("encrypt findings: %w", err)
	}
	encImpression, err := h.encryptText(req.Impression)
	if err != nil {
		return RadiologyReport{}, fmt.Errorf("encrypt impression: %w", err)
	}

	var rep RadiologyReport
	var encFindingsDB, encImpressionDB []byte
	err = h.pool.QueryRow(ctx,
		`INSERT INTO radiology_reports
		   (id, tenant_id, patient_nhi, imaging_study_id, order_id,
		    radiologist_hpi, status, findings, impression, fhir_resource)
		 VALUES
		   (@id, @tenant_id, @patient_nhi, @imaging_study_id, @order_id,
		    @radiologist_hpi, 'draft', @findings, @impression, @fhir_resource)
		 RETURNING id, tenant_id, patient_nhi, imaging_study_id, order_id,
		           radiologist_hpi, status,
		           findings, impression,
		           signed_at, amended_at, amendment_reason,
		           created_at, updated_at`,
		db.NamedArgs{
			"id":              reportID,
			"tenant_id":       tenantID,
			"patient_nhi":     req.PatientNHI,
			"imaging_study_id": req.ImagingStudyID,
			"order_id":        req.OrderID,
			"radiologist_hpi": req.RadiologistHPI,
			"findings":        encFindings,
			"impression":      encImpression,
			"fhir_resource":   encFHIR,
		},
	).Scan(
		&rep.ID, &rep.TenantID, &rep.PatientNHI, &rep.ImagingStudyID, &rep.OrderID,
		&rep.RadiologistHPI, &rep.Status,
		&encFindingsDB, &encImpressionDB,
		&rep.SignedAt, &rep.AmendedAt, &rep.AmendmentReason,
		&rep.CreatedAt, &rep.UpdatedAt,
	)
	if err != nil {
		return RadiologyReport{}, fmt.Errorf("insert radiology report: %w", err)
	}

	rep.Findings = h.decryptText(encFindingsDB)
	rep.Impression = h.decryptText(encImpressionDB)
	return rep, nil
}

func (h *ReportsHandler) getReportByID(ctx context.Context, id, tenantID string) (RadiologyReport, error) {
	var rep RadiologyReport
	var encFindings, encImpression []byte

	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, patient_nhi, imaging_study_id, order_id,
		        radiologist_hpi, status,
		        findings, impression,
		        signed_at, amended_at, amendment_reason,
		        created_at, updated_at
		 FROM radiology_reports
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&rep.ID, &rep.TenantID, &rep.PatientNHI, &rep.ImagingStudyID, &rep.OrderID,
		&rep.RadiologistHPI, &rep.Status,
		&encFindings, &encImpression,
		&rep.SignedAt, &rep.AmendedAt, &rep.AmendmentReason,
		&rep.CreatedAt, &rep.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return RadiologyReport{}, errNotFound
		}
		return RadiologyReport{}, fmt.Errorf("get radiology report: %w", err)
	}

	rep.Findings = h.decryptText(encFindings)
	rep.Impression = h.decryptText(encImpression)
	return rep, nil
}

func (h *ReportsHandler) updateReport(ctx context.Context, rep RadiologyReport) (RadiologyReport, error) {
	encFindings, err := h.encryptText(rep.Findings)
	if err != nil {
		return RadiologyReport{}, fmt.Errorf("encrypt findings: %w", err)
	}
	encImpression, err := h.encryptText(rep.Impression)
	if err != nil {
		return RadiologyReport{}, fmt.Errorf("encrypt impression: %w", err)
	}

	var updated RadiologyReport
	var encFindingsDB, encImpressionDB []byte

	err = h.pool.QueryRow(ctx,
		`UPDATE radiology_reports
		 SET status           = @status,
		     findings         = @findings,
		     impression       = @impression,
		     signed_at        = @signed_at,
		     amended_at       = @amended_at,
		     amendment_reason = @amendment_reason,
		     updated_at       = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, patient_nhi, imaging_study_id, order_id,
		           radiologist_hpi, status,
		           findings, impression,
		           signed_at, amended_at, amendment_reason,
		           created_at, updated_at`,
		db.NamedArgs{
			"status":           rep.Status,
			"findings":         encFindings,
			"impression":       encImpression,
			"signed_at":        rep.SignedAt,
			"amended_at":       rep.AmendedAt,
			"amendment_reason": rep.AmendmentReason,
			"id":               rep.ID,
			"tenant_id":        rep.TenantID,
		},
	).Scan(
		&updated.ID, &updated.TenantID, &updated.PatientNHI, &updated.ImagingStudyID, &updated.OrderID,
		&updated.RadiologistHPI, &updated.Status,
		&encFindingsDB, &encImpressionDB,
		&updated.SignedAt, &updated.AmendedAt, &updated.AmendmentReason,
		&updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return RadiologyReport{}, errNotFound
		}
		return RadiologyReport{}, fmt.Errorf("update radiology report: %w", err)
	}

	updated.Findings = h.decryptText(encFindingsDB)
	updated.Impression = h.decryptText(encImpressionDB)
	return updated, nil
}

// scanReport scans a row from a multi-row query into a RadiologyReport.
func (h *ReportsHandler) scanReport(rows interface {
	Scan(...any) error
}) (RadiologyReport, error) {
	var rep RadiologyReport
	var encFindings, encImpression []byte
	if err := rows.Scan(
		&rep.ID, &rep.TenantID, &rep.PatientNHI, &rep.ImagingStudyID, &rep.OrderID,
		&rep.RadiologistHPI, &rep.Status,
		&encFindings, &encImpression,
		&rep.SignedAt, &rep.AmendedAt, &rep.AmendmentReason,
		&rep.CreatedAt, &rep.UpdatedAt,
	); err != nil {
		return RadiologyReport{}, fmt.Errorf("scan radiology report: %w", err)
	}
	rep.Findings = h.decryptText(encFindings)
	rep.Impression = h.decryptText(encImpression)
	return rep, nil
}

// encryptText AES-encrypts a plain string; returns nil for empty strings.
func (h *ReportsHandler) encryptText(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	if h.enc == nil {
		return []byte(s), nil
	}
	return h.enc.Encrypt([]byte(s))
}

// decryptText decrypts a BYTEA column; returns "" for nil/empty values.
func (h *ReportsHandler) decryptText(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if h.enc == nil {
		return string(b)
	}
	plain, err := h.enc.Decrypt(b)
	if err != nil {
		return ""
	}
	return string(plain)
}
