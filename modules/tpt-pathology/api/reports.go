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
)

// DiagnosticReport is the domain model for a diagnostic report row.
type DiagnosticReport struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenantId"`
	PatientNHI       string     `json:"patientNhi,omitempty"`
	SpecimenID       string     `json:"specimenId,omitempty"`
	AccessionNumber  string     `json:"accessionNumber"`
	OrderingHPI      string     `json:"orderingHpi"`
	PerformingLab    string     `json:"performingLab"`
	Status           string     `json:"status"`
	Category         string     `json:"category"`
	LOINCCode        string     `json:"loincCode"`
	LOINCDisplay     string     `json:"loincDisplay"`
	FHIRReport       string     `json:"fhirReport,omitempty"` // decrypted FHIR JSON (Get only)
	IssuedAt         *time.Time `json:"issuedAt,omitempty"`
	EffectiveAt      *time.Time `json:"effectiveAt,omitempty"`
	NotificationSent bool       `json:"notificationSent"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// ReportsHandler handles /api/v1/reports routes.
type ReportsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/reports.
// Supports query params: patient (NHI), status, ordering_hpi, accession.
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
	patientFilter := q.Get("patient")
	statusFilter := q.Get("status")
	hpiFilter := q.Get("ordering_hpi")
	accessionFilter := q.Get("accession")

	reports, err := h.listReports(ctx, tenantID, patientFilter, statusFilter, hpiFilter, accessionFilter)
	if err != nil {
		h.logger.Error("list diagnostic reports", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list reports"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "DiagnosticReport",
		ResourceID:   "list",
		TenantID:     tenantID,
		Metadata:     map[string]string{"patient": patientFilter, "status": statusFilter},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"reports": reports,
		"total":   len(reports),
	})
}

// Get handles GET /api/v1/reports/{id}.
// Decrypts and returns the full FHIR DiagnosticReport JSON.
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
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "report ID is required"})
		return
	}

	report, err := h.getReport(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "report not found"})
			return
		}
		h.logger.Error("get diagnostic report", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve report"})
		return
	}

	// Decrypt FHIR report blob.
	if len(report.FHIRReport) > 0 && h.enc != nil {
		plain, err := h.enc.Decrypt([]byte(report.FHIRReport))
		if err != nil {
			h.logger.Error("decrypt FHIR report", slog.Any("error", err))
		} else {
			report.FHIRReport = string(plain)
		}
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "DiagnosticReport",
		ResourceID:   id,
		TenantID:     tenantID,
	})

	writeJSON(w, http.StatusOK, report)
}

// GetObservations handles GET /api/v1/reports/{id}/observations.
// Decrypts the FHIR DiagnosticReport and returns its embedded Observation list.
func (h *ReportsHandler) GetObservations(w http.ResponseWriter, r *http.Request) {
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

	report, err := h.getReport(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "report not found"})
			return
		}
		h.logger.Error("get report for observations", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve report"})
		return
	}

	// Decrypt the FHIR blob and extract Observations.
	fhirJSON := []byte(report.FHIRReport)
	if h.enc != nil && len(fhirJSON) > 0 {
		plain, err := h.enc.Decrypt(fhirJSON)
		if err != nil {
			h.logger.Error("decrypt FHIR report", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt report"})
			return
		}
		fhirJSON = plain
	}

	var dr r5.DiagnosticReport
	if err := json.Unmarshal(fhirJSON, &dr); err != nil {
		h.logger.Error("unmarshal FHIR DiagnosticReport", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "PARSE_ERROR", Message: "failed to parse report"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Observation",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"report_id": id},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"reportId":     id,
		"resultRefs":   dr.Result,
		"observations": extractObservationsFromReport(fhirJSON),
	})
}

// ---------------------------------------------------------------------------
// Data access
// ---------------------------------------------------------------------------

func (h *ReportsHandler) listReports(
	ctx context.Context,
	tenantID, patientFilter, statusFilter, hpiFilter, accessionFilter string,
) ([]DiagnosticReport, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, patient_nhi, specimen_id,
		        accession_number, ordering_hpi, performing_lab, status, category,
		        loinc_code, loinc_display, issued_at, effective_at,
		        notification_sent, created_at, updated_at
		 FROM diagnostic_reports
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter   = '' OR patient_nhi      = @patient_filter)
		   AND (@status_filter    = '' OR status           = @status_filter)
		   AND (@hpi_filter       = '' OR ordering_hpi     = @hpi_filter)
		   AND (@accession_filter = '' OR accession_number = @accession_filter)
		 ORDER BY issued_at DESC NULLS LAST, created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":        tenantID,
			"patient_filter":   patientFilter,
			"status_filter":    statusFilter,
			"hpi_filter":       hpiFilter,
			"accession_filter": accessionFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query diagnostic reports: %w", err)
	}
	defer rows.Close()

	var results []DiagnosticReport
	for rows.Next() {
		var dr DiagnosticReport
		if err := rows.Scan(
			&dr.ID, &dr.TenantID, &dr.PatientNHI, &dr.SpecimenID,
			&dr.AccessionNumber, &dr.OrderingHPI, &dr.PerformingLab,
			&dr.Status, &dr.Category,
			&dr.LOINCCode, &dr.LOINCDisplay,
			&dr.IssuedAt, &dr.EffectiveAt,
			&dr.NotificationSent, &dr.CreatedAt, &dr.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan diagnostic report: %w", err)
		}
		results = append(results, dr)
	}
	return results, rows.Err()
}

func (h *ReportsHandler) getReport(ctx context.Context, id, tenantID string) (DiagnosticReport, error) {
	var dr DiagnosticReport
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, patient_nhi, specimen_id,
		        accession_number, ordering_hpi, performing_lab, status, category,
		        loinc_code, loinc_display, fhir_report, issued_at, effective_at,
		        notification_sent, created_at, updated_at
		 FROM diagnostic_reports
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&dr.ID, &dr.TenantID, &dr.PatientNHI, &dr.SpecimenID,
		&dr.AccessionNumber, &dr.OrderingHPI, &dr.PerformingLab,
		&dr.Status, &dr.Category,
		&dr.LOINCCode, &dr.LOINCDisplay, &dr.FHIRReport,
		&dr.IssuedAt, &dr.EffectiveAt,
		&dr.NotificationSent, &dr.CreatedAt, &dr.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return DiagnosticReport{}, errNotFound
		}
		return DiagnosticReport{}, fmt.Errorf("get diagnostic report: %w", err)
	}
	return dr, nil
}

// extractObservationsFromReport attempts to parse inline Observation resources
// from a raw FHIR DiagnosticReport JSON blob (as a convenience for callers
// that stored observations embedded rather than by reference).
// Returns nil if parsing fails or no inline observations are present.
func extractObservationsFromReport(raw []byte) []r5.Observation {
	var wrapper struct {
		Contained []json.RawMessage `json:"contained"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil
	}
	var out []r5.Observation
	for _, entry := range wrapper.Contained {
		var obs r5.Observation
		if err := json.Unmarshal(entry, &obs); err == nil && obs.ResourceType == "Observation" {
			out = append(out, obs)
		}
	}
	return out
}
