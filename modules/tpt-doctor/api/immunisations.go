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
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// Immunisation represents a vaccination event recorded for a patient.
// Aligns with the FHIR R5 Immunization resource and NZ NIR requirements.
type Immunisation struct {
	ID             string     `json:"id"`
	PatientID      string     `json:"patientId"`
	PatientNHI     string     `json:"patientNhi"`
	VaccineCode    string     `json:"vaccineCode"`           // NZMT code
	VaccineName    string     `json:"vaccineName"`
	LotNumber      string     `json:"lotNumber,omitempty"`
	ExpiryDate     string     `json:"expiryDate,omitempty"` // YYYY-MM-DD
	DoseNumber     int        `json:"doseNumber"`
	Series         string     `json:"series,omitempty"`    // e.g. "HPV-3-dose"
	BodySiteCode   string     `json:"bodySiteCode,omitempty"`
	RouteCode      string     `json:"routeCode,omitempty"`  // SNOMED CT
	AdministeredBy string     `json:"administeredBy"`       // HPI CPN
	EncounterID    string     `json:"encounterId,omitempty"`
	OccurrenceDate time.Time  `json:"occurrenceDate"`
	NIRSubmitted   bool       `json:"nirSubmitted"`
	NIRReference   string     `json:"nirReference,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	TenantID       string     `json:"tenantId"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	NIRSubmittedAt *time.Time `json:"nirSubmittedAt,omitempty"`
}

// immunisationCreateRequest is the body for POST /api/v1/immunisations.
type immunisationCreateRequest struct {
	PatientID      string    `json:"patientId"`
	PatientNHI     string    `json:"patientNhi"`
	VaccineCode    string    `json:"vaccineCode"`
	VaccineName    string    `json:"vaccineName"`
	LotNumber      string    `json:"lotNumber,omitempty"`
	ExpiryDate     string    `json:"expiryDate,omitempty"`
	DoseNumber     int       `json:"doseNumber"`
	Series         string    `json:"series,omitempty"`
	BodySiteCode   string    `json:"bodySiteCode,omitempty"`
	RouteCode      string    `json:"routeCode,omitempty"`
	AdministeredBy string    `json:"administeredBy"`
	EncounterID    string    `json:"encounterId,omitempty"`
	OccurrenceDate time.Time `json:"occurrenceDate"`
	Notes          string    `json:"notes,omitempty"`
}

// ImmunisationsHandler handles all /api/v1/immunisations routes.
type ImmunisationsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/immunisations.
// Supports query params: patient (internal ID), vaccine (NZMT code).
func (h *ImmunisationsHandler) List(w http.ResponseWriter, r *http.Request) {
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
	vaccineFilter := q.Get("vaccine")

	records, err := h.listImmunisations(ctx, tenantID.String(), patientFilter, vaccineFilter)
	if err != nil {
		h.logger.Error("list immunisations", slog.Any("error", err), slog.String("tenant", tenantID.String()))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list immunisations"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Immunization",
		ResourceID:   "list",
		TenantID:     tenantID,
		Details:      map[string]any{"patient": patientFilter, "vaccine": vaccineFilter},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"immunisations": records,
		"total":         len(records),
	})
}

// Create handles POST /api/v1/immunisations.
// Records a vaccination event. The NIR submission is triggered separately via
// POST /api/v1/immunisations/{id}/submit-nir.
func (h *ImmunisationsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req immunisationCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if err := validateImmunisationCreate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	// HPCA requirement: validate the administering practitioner holds a current APC.
	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.AdministeredBy)
	if err != nil {
		h.logger.Error("HPI APC validation for immunisation", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate practitioner APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "INVALID_APC",
			Message: "administering practitioner does not have a current Annual Practising Certificate",
			Details: apcStatus,
		})
		return
	}

	record, err := h.insertImmunisation(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert immunisation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to record immunisation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "Immunization",
		ResourceID:   record.ID,
		TenantID:     tenantID,
		Details:      map[string]any{"vaccine": req.VaccineCode, "dose": req.DoseNumber},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusCreated, record)
}

// Get handles GET /api/v1/immunisations/{id}.
func (h *ImmunisationsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "immunisation ID is required"})
		return
	}

	record, err := h.getImmunisationByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "immunisation not found"})
			return
		}
		h.logger.Error("get immunisation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve immunisation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Immunization",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, record)
}

// SubmitNIR handles POST /api/v1/immunisations/{id}/submit-nir.
// Submits the immunisation event to the National Immunisation Register.
// On success, the NIRReference and NIRSubmittedAt are recorded.
func (h *ImmunisationsHandler) SubmitNIR(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "immunisation ID is required"})
		return
	}

	record, err := h.getImmunisationByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "immunisation not found"})
			return
		}
		h.logger.Error("get immunisation for NIR submit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve immunisation"})
		return
	}

	if record.NIRSubmitted {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "ALREADY_SUBMITTED",
			Message: fmt.Sprintf("immunisation already submitted to NIR (reference: %s)", record.NIRReference),
		})
		return
	}

	// Generate a NIR reference (stub: production calls the MoH NIR FHIR API).
	nirRef := fmt.Sprintf("NIR-%s", record.ID[:8])
	now := time.Now().UTC()

	updated, err := h.markNIRSubmitted(ctx, id, nirRef, now, tenantID.String())
	if err != nil {
		h.logger.Error("mark NIR submitted", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "NIR_ERROR", Message: "failed to record NIR submission"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "Immunization",
		ResourceID:   id,
		TenantID:     tenantID,
		Details:      map[string]any{"action": "nir-submit", "nir_reference": nirRef},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, updated)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func validateImmunisationCreate(req *immunisationCreateRequest) error {
	if req.PatientID == "" && req.PatientNHI == "" {
		return fmt.Errorf("either patientId or patientNhi is required")
	}
	if req.VaccineCode == "" {
		return fmt.Errorf("vaccineCode is required (NZMT code)")
	}
	if req.VaccineName == "" {
		return fmt.Errorf("vaccineName is required")
	}
	if req.AdministeredBy == "" {
		return fmt.Errorf("administeredBy (HPI CPN) is required")
	}
	if req.DoseNumber < 1 {
		return fmt.Errorf("doseNumber must be >= 1")
	}
	if req.OccurrenceDate.IsZero() {
		return fmt.Errorf("occurrenceDate is required")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data access
// ---------------------------------------------------------------------------

func (h *ImmunisationsHandler) listImmunisations(
	ctx context.Context,
	tenantID, patientFilter, vaccineFilter string,
) ([]Immunisation, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, vaccine_code, vaccine_name,
		        lot_number, expiry_date, dose_number, series,
		        body_site_code, route_code, administered_by,
		        encounter_id, occurrence_date,
		        nir_submitted, nir_reference, notes,
		        tenant_id, created_at, updated_at, nir_submitted_at
		 FROM immunisations
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter = '' OR patient_id   = @patient_filter)
		   AND (@vaccine_filter = '' OR vaccine_code = @vaccine_filter)
		 ORDER BY occurrence_date DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":      tenantID,
			"patient_filter": patientFilter,
			"vaccine_filter": vaccineFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query immunisations: %w", err)
	}
	defer rows.Close()

	var results []Immunisation
	for rows.Next() {
		var imm Immunisation
		if err := rows.Scan(
			&imm.ID, &imm.PatientID, &imm.PatientNHI, &imm.VaccineCode, &imm.VaccineName,
			&imm.LotNumber, &imm.ExpiryDate, &imm.DoseNumber, &imm.Series,
			&imm.BodySiteCode, &imm.RouteCode, &imm.AdministeredBy,
			&imm.EncounterID, &imm.OccurrenceDate,
			&imm.NIRSubmitted, &imm.NIRReference, &imm.Notes,
			&imm.TenantID, &imm.CreatedAt, &imm.UpdatedAt, &imm.NIRSubmittedAt,
		); err != nil {
			return nil, fmt.Errorf("scan immunisation: %w", err)
		}
		results = append(results, imm)
	}
	return results, rows.Err()
}

func (h *ImmunisationsHandler) getImmunisationByID(ctx context.Context, id, tenantID string) (Immunisation, error) {
	var imm Immunisation
	err := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, vaccine_code, vaccine_name,
		        lot_number, expiry_date, dose_number, series,
		        body_site_code, route_code, administered_by,
		        encounter_id, occurrence_date,
		        nir_submitted, nir_reference, notes,
		        tenant_id, created_at, updated_at, nir_submitted_at
		 FROM immunisations
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&imm.ID, &imm.PatientID, &imm.PatientNHI, &imm.VaccineCode, &imm.VaccineName,
		&imm.LotNumber, &imm.ExpiryDate, &imm.DoseNumber, &imm.Series,
		&imm.BodySiteCode, &imm.RouteCode, &imm.AdministeredBy,
		&imm.EncounterID, &imm.OccurrenceDate,
		&imm.NIRSubmitted, &imm.NIRReference, &imm.Notes,
		&imm.TenantID, &imm.CreatedAt, &imm.UpdatedAt, &imm.NIRSubmittedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Immunisation{}, errNotFound
		}
		return Immunisation{}, fmt.Errorf("get immunisation: %w", err)
	}
	return imm, nil
}

func (h *ImmunisationsHandler) insertImmunisation(ctx context.Context, req immunisationCreateRequest, tenantID string) (Immunisation, error) {
	var imm Immunisation
	err := h.pool.QueryRow(ctx,
		`INSERT INTO immunisations
		   (patient_id, patient_nhi, vaccine_code, vaccine_name, lot_number, expiry_date,
		    dose_number, series, body_site_code, route_code, administered_by,
		    encounter_id, occurrence_date, notes, tenant_id)
		 VALUES
		   (@patient_id, @patient_nhi, @vaccine_code, @vaccine_name, @lot_number, @expiry_date,
		    @dose_number, @series, @body_site_code, @route_code, @administered_by,
		    @encounter_id, @occurrence_date, @notes, @tenant_id)
		 RETURNING id, patient_id, patient_nhi, vaccine_code, vaccine_name,
		           lot_number, expiry_date, dose_number, series,
		           body_site_code, route_code, administered_by,
		           encounter_id, occurrence_date,
		           nir_submitted, nir_reference, notes,
		           tenant_id, created_at, updated_at, nir_submitted_at`,
		db.NamedArgs{
			"patient_id":      req.PatientID,
			"patient_nhi":     req.PatientNHI,
			"vaccine_code":    req.VaccineCode,
			"vaccine_name":    req.VaccineName,
			"lot_number":      req.LotNumber,
			"expiry_date":     req.ExpiryDate,
			"dose_number":     req.DoseNumber,
			"series":          req.Series,
			"body_site_code":  req.BodySiteCode,
			"route_code":      req.RouteCode,
			"administered_by": req.AdministeredBy,
			"encounter_id":    req.EncounterID,
			"occurrence_date": req.OccurrenceDate,
			"notes":           req.Notes,
			"tenant_id":       tenantID,
		},
	).Scan(
		&imm.ID, &imm.PatientID, &imm.PatientNHI, &imm.VaccineCode, &imm.VaccineName,
		&imm.LotNumber, &imm.ExpiryDate, &imm.DoseNumber, &imm.Series,
		&imm.BodySiteCode, &imm.RouteCode, &imm.AdministeredBy,
		&imm.EncounterID, &imm.OccurrenceDate,
		&imm.NIRSubmitted, &imm.NIRReference, &imm.Notes,
		&imm.TenantID, &imm.CreatedAt, &imm.UpdatedAt, &imm.NIRSubmittedAt,
	)
	if err != nil {
		return Immunisation{}, fmt.Errorf("insert immunisation: %w", err)
	}
	return imm, nil
}

func (h *ImmunisationsHandler) markNIRSubmitted(ctx context.Context, id, nirRef string, submittedAt time.Time, tenantID string) (Immunisation, error) {
	var imm Immunisation
	err := h.pool.QueryRow(ctx,
		`UPDATE immunisations
		 SET nir_submitted    = true,
		     nir_reference    = @nir_reference,
		     nir_submitted_at = @nir_submitted_at,
		     updated_at       = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, vaccine_code, vaccine_name,
		           lot_number, expiry_date, dose_number, series,
		           body_site_code, route_code, administered_by,
		           encounter_id, occurrence_date,
		           nir_submitted, nir_reference, notes,
		           tenant_id, created_at, updated_at, nir_submitted_at`,
		db.NamedArgs{
			"nir_reference":    nirRef,
			"nir_submitted_at": submittedAt,
			"id":               id,
			"tenant_id":        tenantID,
		},
	).Scan(
		&imm.ID, &imm.PatientID, &imm.PatientNHI, &imm.VaccineCode, &imm.VaccineName,
		&imm.LotNumber, &imm.ExpiryDate, &imm.DoseNumber, &imm.Series,
		&imm.BodySiteCode, &imm.RouteCode, &imm.AdministeredBy,
		&imm.EncounterID, &imm.OccurrenceDate,
		&imm.NIRSubmitted, &imm.NIRReference, &imm.Notes,
		&imm.TenantID, &imm.CreatedAt, &imm.UpdatedAt, &imm.NIRSubmittedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Immunisation{}, errNotFound
		}
		return Immunisation{}, fmt.Errorf("mark NIR submitted: %w", err)
	}
	return imm, nil
}
