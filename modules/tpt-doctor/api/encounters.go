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

// EncounterStatus enumerates FHIR R5 Encounter status values used by tpt-doctor.
type EncounterStatus string

const (
	EncounterStatusPlanned    EncounterStatus = "planned"
	EncounterStatusInProgress EncounterStatus = "in-progress"
	EncounterStatusOnHold     EncounterStatus = "on-hold"
	EncounterStatusCompleted  EncounterStatus = "completed"
	EncounterStatusCancelled  EncounterStatus = "cancelled"
)

// Vitals holds a set of clinical measurements recorded during an encounter.
type Vitals struct {
	SystolicBP  *float64 `json:"systolicBp,omitempty"`  // mmHg
	DiastolicBP *float64 `json:"diastolicBp,omitempty"` // mmHg
	HeartRate   *float64 `json:"heartRate,omitempty"`   // bpm
	Temperature *float64 `json:"temperature,omitempty"` // °C
	Weight      *float64 `json:"weight,omitempty"`      // kg
	Height      *float64 `json:"height,omitempty"`      // cm
	BMI         *float64 `json:"bmi,omitempty"`         // kg/m²
	SpO2        *float64 `json:"spo2,omitempty"`        // %
	RespRate    *float64 `json:"respRate,omitempty"`    // breaths/min
}

// SOAPNotes holds the four components of a SOAP clinical note.
type SOAPNotes struct {
	Subjective string `json:"subjective"` // Patient's reported symptoms and history
	Objective  string `json:"objective"`  // Clinician's observations and measurements
	Assessment string `json:"assessment"` // Clinical diagnosis or differential
	Plan       string `json:"plan"`       // Management plan
}

// Encounter represents a clinical consultation.
// WorkflowVariant determines the clinical workflow applied to an encounter.
// Variants change billing paths, required fields, and post-encounter tasks.
type WorkflowVariant string

const (
	// WorkflowStandard is the default GP consultation workflow.
	WorkflowStandard WorkflowVariant = "standard"
	// WorkflowAfterHours is for after-hours and weekend consultations.
	// Triggers after-hours surcharge billing and alternate ACC category codes.
	WorkflowAfterHours WorkflowVariant = "after-hours"
	// WorkflowUrgentCare is for urgent / walk-in presentations.
	// Bypasses appointment requirement; triggers same-day billing rules.
	WorkflowUrgentCare WorkflowVariant = "urgent-care"
	// WorkflowOccHealth is for occupational health assessments.
	// Enables employer billing, return-to-work forms, and workplace injury pathways.
	WorkflowOccHealth WorkflowVariant = "occupational-health"
)

// Aligns with the FHIR R5 Encounter resource.
type Encounter struct {
	ID              string          `json:"id"`
	PatientID       string          `json:"patientId"`
	PatientNHI      string          `json:"patientNhi"`
	PractitionerHPI string          `json:"practitionerHpi"`
	AppointmentID   string          `json:"appointmentId,omitempty"`
	Status          EncounterStatus `json:"status"`
	Workflow        WorkflowVariant `json:"workflow"`
	SOAP            SOAPNotes       `json:"soap"`
	Vitals          Vitals          `json:"vitals"`
	Diagnoses       []string        `json:"diagnoses"`  // ICD-10-AM codes
	Procedures      []string        `json:"procedures"` // SNOMED CT or local codes
	FFSEligible     bool            `json:"ffsEligible"`
	FFSFundingCode  string          `json:"ffsFundingCode,omitempty"`
	EmployerID      string          `json:"employerId,omitempty"` // OccHealth: employer reference
	TenantID        string          `json:"tenantId"`
	StartedAt       time.Time       `json:"startedAt"`
	CompletedAt     *time.Time      `json:"completedAt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// encounterCreateRequest is the body for POST /api/v1/encounters.
type encounterCreateRequest struct {
	PatientID       string          `json:"patientId"`
	PatientNHI      string          `json:"patientNhi"`
	PractitionerHPI string          `json:"practitionerHpi"`
	AppointmentID   string          `json:"appointmentId,omitempty"`
	Workflow        WorkflowVariant `json:"workflow,omitempty"`
	Reason          string          `json:"reason,omitempty"`
	EmployerID      string          `json:"employerId,omitempty"` // OccHealth only
}

// encounterUpdateRequest is the body for PUT /api/v1/encounters/{id}.
// All fields are optional; only non-zero values are applied.
type encounterUpdateRequest struct {
	SOAP       *SOAPNotes `json:"soap,omitempty"`
	Vitals     *Vitals    `json:"vitals,omitempty"`
	Diagnoses  []string   `json:"diagnoses,omitempty"`
	Procedures []string   `json:"procedures,omitempty"`
	Notes      string     `json:"notes,omitempty"`
}

// EncountersHandler handles all /api/v1/encounters routes.
type EncountersHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/encounters.
// Supported query parameters: patient (internal ID), provider (HPI CPN), status.
func (h *EncountersHandler) List(w http.ResponseWriter, r *http.Request) {
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
	providerFilter := q.Get("provider")
	statusFilter := q.Get("status")

	encounters, err := h.listEncounters(ctx, tenantID, patientFilter, providerFilter, statusFilter)
	if err != nil {
		h.logger.Error("list encounters", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list encounters"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Encounter",
		ResourceID:   "list",
		TenantID:     tenantID,
		Metadata: map[string]string{
			"patient":  patientFilter,
			"provider": providerFilter,
			"status":   statusFilter,
		},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"encounters": encounters,
		"total":      len(encounters),
	})
}

// Create handles POST /api/v1/encounters.
// Starts a new clinical encounter and sets its status to "in-progress".
func (h *EncountersHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req encounterCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.PractitionerHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PRACTITIONER", Message: "practitionerHpi is required"})
		return
	}

	// HPCA requirement: validate the practitioner holds a current APC.
	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.PractitionerHPI)
	if err != nil {
		h.logger.Error("HPI APC validation for encounter", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate practitioner APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "INVALID_APC",
			Message: "practitioner does not have a current Annual Practising Certificate",
			Details: apcStatus,
		})
		return
	}

	enc, err := h.insertEncounter(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert encounter", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create encounter"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Encounter",
		ResourceID:   enc.ID,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, enc)
}

// Get handles GET /api/v1/encounters/{id}.
func (h *EncountersHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "encounter ID is required"})
		return
	}

	enc, err := h.getEncounterByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "encounter not found"})
			return
		}
		h.logger.Error("get encounter", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve encounter"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Encounter",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, enc)
}

// Update handles PUT /api/v1/encounters/{id}.
// Allows adding or updating SOAP notes, vitals, diagnoses, and procedures
// on an in-progress encounter.
func (h *EncountersHandler) Update(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "encounter ID is required"})
		return
	}

	var req encounterUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getEncounterByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "encounter not found"})
			return
		}
		h.logger.Error("get encounter for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve encounter"})
		return
	}

	if existing.Status == EncounterStatusCompleted || existing.Status == EncounterStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "TERMINAL_STATUS",
			Message: fmt.Sprintf("cannot update encounter in %s status", existing.Status),
		})
		return
	}

	if req.SOAP != nil {
		existing.SOAP = *req.SOAP
	}
	if req.Vitals != nil {
		existing.Vitals = *req.Vitals
	}
	if len(req.Diagnoses) > 0 {
		existing.Diagnoses = req.Diagnoses
	}
	if len(req.Procedures) > 0 {
		existing.Procedures = req.Procedures
	}

	updated, err := h.updateEncounter(ctx, existing)
	if err != nil {
		h.logger.Error("update encounter", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update encounter"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Encounter",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// Complete handles POST /api/v1/encounters/{id}/complete.
// Transitions the encounter to "completed" and stamps the completion time.
// Requires at least an Assessment and Plan in the SOAP notes.
func (h *EncountersHandler) Complete(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "encounter ID is required"})
		return
	}

	existing, err := h.getEncounterByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "encounter not found"})
			return
		}
		h.logger.Error("get encounter for complete", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve encounter"})
		return
	}

	if existing.Status == EncounterStatusCompleted {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_COMPLETED", Message: "encounter is already completed"})
		return
	}
	if existing.Status == EncounterStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "CANCELLED", Message: "cannot complete a cancelled encounter"})
		return
	}

	// Enforce minimum clinical documentation before completion.
	if existing.SOAP.Assessment == "" {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "MISSING_ASSESSMENT",
			Message: "an Assessment (SOAP 'A') is required before completing an encounter",
		})
		return
	}
	if existing.SOAP.Plan == "" {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "MISSING_PLAN",
			Message: "a Plan (SOAP 'P') is required before completing an encounter",
		})
		return
	}

	now := time.Now().UTC()
	existing.Status = EncounterStatusCompleted
	existing.CompletedAt = &now

	completed, err := h.completeEncounter(ctx, existing)
	if err != nil {
		h.logger.Error("complete encounter", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "COMPLETE_ERROR", Message: "failed to complete encounter"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Encounter",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "complete"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, completed)
}

// listEncounters queries the encounters table with optional filters.
func (h *EncountersHandler) listEncounters(
	ctx context.Context,
	tenantID, patientFilter, providerFilter, statusFilter string,
) ([]Encounter, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, appointment_id,
		        status, soap_subjective, soap_objective, soap_assessment, soap_plan,
		        vitals, diagnoses, procedures,
		        tenant_id, started_at, completed_at, created_at, updated_at
		 FROM encounters
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter  = '' OR patient_id = @patient_filter)
		   AND (@provider_filter = '' OR practitioner_hpi = @provider_filter)
		   AND (@status_filter   = '' OR status = @status_filter)
		 ORDER BY started_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":       tenantID,
			"patient_filter":  patientFilter,
			"provider_filter": providerFilter,
			"status_filter":   statusFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query encounters: %w", err)
	}
	defer rows.Close()

	var results []Encounter
	for rows.Next() {
		enc, err := scanEncounter(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, enc)
	}
	return results, rows.Err()
}

// getEncounterByID retrieves a single encounter with tenant isolation.
func (h *EncountersHandler) getEncounterByID(ctx context.Context, id, tenantID string) (Encounter, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, appointment_id,
		        status, soap_subjective, soap_objective, soap_assessment, soap_plan,
		        vitals, diagnoses, procedures,
		        tenant_id, started_at, completed_at, created_at, updated_at
		 FROM encounters
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	enc, err := scanEncounterRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Encounter{}, errNotFound
		}
		return Encounter{}, fmt.Errorf("get encounter by id: %w", err)
	}
	return enc, nil
}

// insertEncounter persists a new encounter in "in-progress" status.
func (h *EncountersHandler) insertEncounter(ctx context.Context, req encounterCreateRequest, tenantID string) (Encounter, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO encounters
		   (patient_id, patient_nhi, practitioner_hpi, appointment_id,
		    status, tenant_id, started_at)
		 VALUES
		   (@patient_id, @patient_nhi, @practitioner_hpi, @appointment_id,
		    @status, @tenant_id, now())
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, appointment_id,
		           status, soap_subjective, soap_objective, soap_assessment, soap_plan,
		           vitals, diagnoses, procedures,
		           tenant_id, started_at, completed_at, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":       req.PatientID,
			"patient_nhi":      req.PatientNHI,
			"practitioner_hpi": req.PractitionerHPI,
			"appointment_id":   req.AppointmentID,
			"status":           EncounterStatusInProgress,
			"tenant_id":        tenantID,
		},
	)
	enc, err := scanEncounterRow(row)
	if err != nil {
		return Encounter{}, fmt.Errorf("insert encounter: %w", err)
	}
	return enc, nil
}

// updateEncounter writes updated SOAP notes, vitals, diagnoses, and procedures.
func (h *EncountersHandler) updateEncounter(ctx context.Context, e Encounter) (Encounter, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE encounters
		 SET soap_subjective = @soap_subjective,
		     soap_objective  = @soap_objective,
		     soap_assessment = @soap_assessment,
		     soap_plan       = @soap_plan,
		     vitals          = @vitals,
		     diagnoses       = @diagnoses,
		     procedures      = @procedures,
		     updated_at      = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, appointment_id,
		           status, soap_subjective, soap_objective, soap_assessment, soap_plan,
		           vitals, diagnoses, procedures,
		           tenant_id, started_at, completed_at, created_at, updated_at`,
		db.NamedArgs{
			"soap_subjective": e.SOAP.Subjective,
			"soap_objective":  e.SOAP.Objective,
			"soap_assessment": e.SOAP.Assessment,
			"soap_plan":       e.SOAP.Plan,
			"vitals":          e.Vitals,
			"diagnoses":       e.Diagnoses,
			"procedures":      e.Procedures,
			"id":              e.ID,
			"tenant_id":       e.TenantID,
		},
	)
	updated, err := scanEncounterRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Encounter{}, errNotFound
		}
		return Encounter{}, fmt.Errorf("update encounter: %w", err)
	}
	return updated, nil
}

// completeEncounter transitions the encounter to completed and records the timestamp.
func (h *EncountersHandler) completeEncounter(ctx context.Context, e Encounter) (Encounter, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE encounters
		 SET status       = @status,
		     completed_at = @completed_at,
		     updated_at   = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, appointment_id,
		           status, soap_subjective, soap_objective, soap_assessment, soap_plan,
		           vitals, diagnoses, procedures,
		           tenant_id, started_at, completed_at, created_at, updated_at`,
		db.NamedArgs{
			"status":       e.Status,
			"completed_at": e.CompletedAt,
			"id":           e.ID,
			"tenant_id":    e.TenantID,
		},
	)
	completed, err := scanEncounterRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Encounter{}, errNotFound
		}
		return Encounter{}, fmt.Errorf("complete encounter: %w", err)
	}
	return completed, nil
}

// dbRow is a shared interface for pgx.Row and pgx.Rows to allow a single scan helper.
type dbRow interface {
	Scan(dest ...any) error
}

// scanEncounterRow scans a single encounter from a pgx.Row.
func scanEncounterRow(row dbRow) (Encounter, error) {
	var e Encounter
	if err := row.Scan(
		&e.ID, &e.PatientID, &e.PatientNHI, &e.PractitionerHPI, &e.AppointmentID,
		&e.Status,
		&e.SOAP.Subjective, &e.SOAP.Objective, &e.SOAP.Assessment, &e.SOAP.Plan,
		&e.Vitals, &e.Diagnoses, &e.Procedures,
		&e.TenantID, &e.StartedAt, &e.CompletedAt, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return Encounter{}, err
	}
	return e, nil
}

// scanEncounter scans a single encounter from a pgx.Rows cursor.
func scanEncounter(rows interface{ Scan(dest ...any) error }) (Encounter, error) {
	return scanEncounterRow(rows)
}
