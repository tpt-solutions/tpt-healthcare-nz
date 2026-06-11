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
	"github.com/PhillipC05/tpt-healthcare/core/medsafe"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	pharmacygateway "github.com/PhillipC05/tpt-healthcare/core/pharmacy-gateway"
	"github.com/PhillipC05/tpt-healthcare/core/pharmac"
)

// PrescriptionStatus mirrors the FHIR MedicationRequest status value set.
type PrescriptionStatus string

const (
	PrescriptionStatusActive    PrescriptionStatus = "active"
	PrescriptionStatusOnHold    PrescriptionStatus = "on-hold"
	PrescriptionStatusCancelled PrescriptionStatus = "cancelled"
	PrescriptionStatusCompleted PrescriptionStatus = "completed"
	PrescriptionStatusDraft     PrescriptionStatus = "draft"
	PrescriptionStatusStopped   PrescriptionStatus = "stopped"
)

// Dosage represents a medication dosage instruction.
type Dosage struct {
	Text            string  `json:"text"`
	Route           string  `json:"route,omitempty"`           // e.g. "oral", "topical"
	DoseValue       float64 `json:"doseValue"`                 // numeric dose
	DoseUnit        string  `json:"doseUnit"`                  // e.g. "mg", "mL"
	Frequency       string  `json:"frequency"`                 // e.g. "twice daily"
	DurationDays    int     `json:"durationDays,omitempty"`    // 0 = ongoing
	MaxDailyDose    float64 `json:"maxDailyDose,omitempty"`    // safety cap
	MaxDailyDoseUnit string `json:"maxDailyDoseUnit,omitempty"`
}

// Prescription is the domain model for an e-prescription (FHIR MedicationRequest).
type Prescription struct {
	ID                     string             `json:"id"`
	PatientID              string             `json:"patientId"`
	PatientNHI             string             `json:"patientNhi"`
	PractitionerHPI        string             `json:"practitionerHpi"`
	EncounterID            string             `json:"encounterId,omitempty"`
	NZULMCode              string             `json:"nzulmCode"`   // NZMT product code
	MedicationName         string             `json:"medicationName"`
	Status                 PrescriptionStatus `json:"status"`
	Dosage                 Dosage             `json:"dosage"`
	PHARMACSubsidised      bool               `json:"pharmácSubsidised"`
	SubsidyCode            string             `json:"subsidyCode,omitempty"`
	InteractionWarnings    []string           `json:"interactionWarnings,omitempty"`
	// InteractionCheckSkipped is true when the drug interaction check could not
	// be performed (e.g. active-medication lookup failed). The prescriber must
	// manually verify interactions before dispensing.
	InteractionCheckSkipped bool              `json:"interactionCheckSkipped,omitempty"`
	SubsidyCheckSkipped     bool              `json:"subsidyCheckSkipped,omitempty"`
	Repeats                 int               `json:"repeats"`
	RepeatsRemaining        int               `json:"repeatsRemaining"`
	TenantID                string            `json:"tenantId"`
	IssuedAt                time.Time         `json:"issuedAt"`
	ExpiresAt               *time.Time        `json:"expiresAt,omitempty"`
	CreatedAt               time.Time         `json:"createdAt"`
	UpdatedAt               time.Time         `json:"updatedAt"`
}

// prescriptionCreateRequest is the body for POST /api/v1/prescriptions.
type prescriptionCreateRequest struct {
	PatientID       string `json:"patientId"`
	PatientNHI      string `json:"patientNhi"`
	PractitionerHPI string `json:"practitionerHpi"`
	EncounterID     string `json:"encounterId,omitempty"`
	NZULMCode       string `json:"nzulmCode"`
	Dosage          Dosage `json:"dosage"`
	Repeats         int    `json:"repeats"`
}

// prescriptionUpdateRequest is the body for PUT /api/v1/prescriptions/{id}.
type prescriptionUpdateRequest struct {
	Status  *PrescriptionStatus `json:"status,omitempty"`
	Dosage  *Dosage             `json:"dosage,omitempty"`
	Repeats *int                `json:"repeats,omitempty"`
}

// printablePrescription is the response for POST /api/v1/prescriptions/{id}/print.
type printablePrescription struct {
	PrescriptionID  string    `json:"prescriptionId"`
	PatientName     string    `json:"patientName"`
	PatientNHI      string    `json:"patientNhi"`
	PatientDOB      string    `json:"patientDob"`
	PractitionerName string   `json:"practitionerName"`
	PractitionerHPI string    `json:"practitionerHpi"`
	MedicationName  string    `json:"medicationName"`
	NZULMCode       string    `json:"nzulmCode"`
	DosageText      string    `json:"dosageText"`
	Repeats         int       `json:"repeats"`
	SubsidyCode     string    `json:"subsidyCode,omitempty"`
	IssuedAt        time.Time `json:"issuedAt"`
	PrintedAt       time.Time `json:"printedAt"`
}

// PrescriptionsHandler handles all /api/v1/prescriptions routes.
type PrescriptionsHandler struct {
	pool            db.Pool
	enc             *encryption.Cipher
	hpiClient       *hpi.Client
	pharmac         *pharmac.Client
	medsafeClient   *medsafe.Client
	pharmacyGateway *pharmacygateway.Gateway
	auditTrail      *audit.Trail
	logger          *slog.Logger
}

// List handles GET /api/v1/prescriptions.
// Supported query parameters: patient (internal ID), status, provider (HPI CPN).
func (h *PrescriptionsHandler) List(w http.ResponseWriter, r *http.Request) {
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
	providerFilter := q.Get("provider")

	prescriptions, err := h.listPrescriptions(ctx, tenantID.String(), patientFilter, statusFilter, providerFilter)
	if err != nil {
		h.logger.Error("list prescriptions", slog.Any("error", err), slog.String("tenant", tenantID.String()))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list prescriptions"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MedicationRequest",
		ResourceID:   "list",
		TenantID:     tenantID,
		Details: map[string]any{
			"patient":  patientFilter,
			"status":   statusFilter,
			"provider": providerFilter,
		},
		OccurredAt: time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"prescriptions": prescriptions,
		"total":         len(prescriptions),
	})
}

// Create handles POST /api/v1/prescriptions.
// Validates:
//   - prescriber has a valid APC with prescribing scope (HPI)
//   - NZULM product code exists (PHARMAC formulary)
//   - PHARMAC subsidy eligibility
//   - Drug interaction check against the patient's current medications
func (h *PrescriptionsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req prescriptionCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if err := validatePrescriptionCreate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	// 1. Validate prescriber APC — must have prescribing scope.
	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.PractitionerHPI)
	if err != nil {
		h.logger.Error("HPI APC validation for prescription", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate prescriber APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "INVALID_APC",
			Message: "prescriber does not have a current Annual Practising Certificate",
			Details: apcStatus,
		})
		return
	}
	if !apcStatus.HasPrescribingScope {
		writeJSON(w, http.StatusForbidden, apiError{
			Code:    "NO_PRESCRIBING_SCOPE",
			Message: "practitioner's registration does not include prescribing scope of practice",
			Details: map[string]string{"hpi": req.PractitionerHPI, "scope": apcStatus.ScopeOfPractice},
		})
		return
	}

	// 2. Validate NZULM product code and retrieve medication details.
	medication, err := h.pharmac.GetByNZULM(ctx, req.NZULMCode)
	if err != nil {
		h.logger.Error("PHARMAC NZULM lookup", slog.Any("error", err))
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "INVALID_NZULM",
			Message: fmt.Sprintf("NZULM product code %q not found in PHARMAC formulary: %v", req.NZULMCode, err),
		})
		return
	}

	// 3. Check PHARMAC subsidy eligibility (subsidy info is carried on Medicine).
	subsidyCheckSkipped := false
	subsidised := medication.SubsidyType != pharmac.Unsubsidised

	// 4. Drug interaction check against patient's active medications.
	interactionCheckSkipped := false
	activeMedCodes, err := h.getActiveNZULMCodes(ctx, req.PatientID, tenantID.String())
	if err != nil {
		h.logger.Error("get active medications for interaction check", slog.Any("error", err))
		activeMedCodes = nil
		interactionCheckSkipped = true
	}

	var interactionWarnings []string
	if len(activeMedCodes) > 0 {
		nzulms := append([]string{req.NZULMCode}, activeMedCodes...)
		interactions, err := h.pharmac.CheckInteractions(ctx, nzulms)
		if err != nil {
			h.logger.Error("PHARMAC interaction check", slog.Any("error", err))
			interactionCheckSkipped = true
		} else {
			for _, ia := range interactions {
				interactionWarnings = append(interactionWarnings, ia.Description)
			}
		}
	}

	rx, err := h.insertPrescription(ctx, req, medication, subsidised, interactionWarnings, tenantID.String())
	if err != nil {
		h.logger.Error("insert prescription", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to save prescription"})
		return
	}

	// Surface skip flags so the UI can warn the clinician to verify manually.
	rx.InteractionCheckSkipped = interactionCheckSkipped
	rx.SubsidyCheckSkipped = subsidyCheckSkipped

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "MedicationRequest",
		ResourceID:   rx.ID,
		TenantID:     tenantID,
		Details:      map[string]any{"nzulm": req.NZULMCode},
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, rx)
}

// Get handles GET /api/v1/prescriptions/{id}.
func (h *PrescriptionsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "prescription ID is required"})
		return
	}

	rx, err := h.getPrescriptionByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "prescription not found"})
			return
		}
		h.logger.Error("get prescription", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve prescription"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MedicationRequest",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, rx)
}

// Update handles PUT /api/v1/prescriptions/{id}.
// Supports status changes (e.g. cancel, stop) and dosage amendments.
func (h *PrescriptionsHandler) Update(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "prescription ID is required"})
		return
	}

	var req prescriptionUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getPrescriptionByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "prescription not found"})
			return
		}
		h.logger.Error("get prescription for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve prescription"})
		return
	}

	if existing.Status == PrescriptionStatusCancelled || existing.Status == PrescriptionStatusCompleted {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "TERMINAL_STATUS",
			Message: fmt.Sprintf("cannot update prescription in %s status", existing.Status),
		})
		return
	}

	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.Dosage != nil {
		existing.Dosage = *req.Dosage
	}
	if req.Repeats != nil {
		existing.Repeats = *req.Repeats
	}

	updated, err := h.updatePrescription(ctx, existing)
	if err != nil {
		h.logger.Error("update prescription", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update prescription"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "MedicationRequest",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// Print handles POST /api/v1/prescriptions/{id}/print.
// Returns a structured printable prescription data object. The caller is
// responsible for rendering this into a PDF or paper prescription.
func (h *PrescriptionsHandler) Print(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "prescription ID is required"})
		return
	}

	rx, err := h.getPrescriptionByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "prescription not found"})
			return
		}
		h.logger.Error("get prescription for print", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve prescription"})
		return
	}

	if rx.Status == PrescriptionStatusCancelled || rx.Status == PrescriptionStatusStopped {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "CANNOT_PRINT",
			Message: fmt.Sprintf("prescription is %s and cannot be printed", rx.Status),
		})
		return
	}

	printData := printablePrescription{
		PrescriptionID:  rx.ID,
		PatientNHI:      rx.PatientNHI,
		PractitionerHPI: rx.PractitionerHPI,
		MedicationName:  rx.MedicationName,
		NZULMCode:       rx.NZULMCode,
		DosageText:      rx.Dosage.Text,
		Repeats:         rx.RepeatsRemaining,
		SubsidyCode:     rx.SubsidyCode,
		IssuedAt:        rx.IssuedAt,
		PrintedAt:       time.Now().UTC(),
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MedicationRequest",
		ResourceID:   id,
		TenantID:     tenantID,
		Details:      map[string]any{"action": "print"},
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, printData)
}

// validatePrescriptionCreate enforces required fields.
func validatePrescriptionCreate(req *prescriptionCreateRequest) error {
	if req.PatientID == "" && req.PatientNHI == "" {
		return fmt.Errorf("either patientId or patientNhi is required")
	}
	if req.PractitionerHPI == "" {
		return fmt.Errorf("practitionerHpi is required")
	}
	if req.NZULMCode == "" {
		return fmt.Errorf("nzulmCode is required")
	}
	if req.Dosage.Text == "" {
		return fmt.Errorf("dosage.text is required")
	}
	if req.Dosage.DoseValue <= 0 {
		return fmt.Errorf("dosage.doseValue must be positive")
	}
	if req.Dosage.DoseUnit == "" {
		return fmt.Errorf("dosage.doseUnit is required")
	}
	if req.Repeats < 0 {
		return fmt.Errorf("repeats cannot be negative")
	}
	return nil
}

// getActiveNZULMCodes returns the NZULM codes for the patient's current active medications.
func (h *PrescriptionsHandler) getActiveNZULMCodes(ctx context.Context, patientID, tenantID string) ([]string, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT nzulm_code FROM prescriptions
		 WHERE patient_id = @patient_id
		   AND tenant_id  = @tenant_id
		   AND status     = 'active'`,
		db.NamedArgs{"patient_id": patientID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query active NZULM codes: %w", err)
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan NZULM code: %w", err)
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

// listPrescriptions queries the prescriptions table with optional filters.
func (h *PrescriptionsHandler) listPrescriptions(
	ctx context.Context,
	tenantID, patientFilter, statusFilter, providerFilter string,
) ([]Prescription, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, encounter_id,
		        nzulm_code, medication_name, status,
		        dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		        repeats, repeats_remaining, tenant_id,
		        issued_at, expires_at, created_at, updated_at
		 FROM prescriptions
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter  = '' OR patient_id        = @patient_filter)
		   AND (@status_filter   = '' OR status             = @status_filter)
		   AND (@provider_filter = '' OR practitioner_hpi  = @provider_filter)
		 ORDER BY issued_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":       tenantID,
			"patient_filter":  patientFilter,
			"status_filter":   statusFilter,
			"provider_filter": providerFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query prescriptions: %w", err)
	}
	defer rows.Close()

	var results []Prescription
	for rows.Next() {
		rx, err := scanPrescription(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rx)
	}
	return results, rows.Err()
}

// getPrescriptionByID retrieves a single prescription with tenant isolation.
func (h *PrescriptionsHandler) getPrescriptionByID(ctx context.Context, id, tenantID string) (Prescription, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, encounter_id,
		        nzulm_code, medication_name, status,
		        dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		        repeats, repeats_remaining, tenant_id,
		        issued_at, expires_at, created_at, updated_at
		 FROM prescriptions
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	rx, err := scanPrescription(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Prescription{}, errNotFound
		}
		return Prescription{}, fmt.Errorf("get prescription by id: %w", err)
	}
	return rx, nil
}

// insertPrescription persists a validated prescription.
func (h *PrescriptionsHandler) insertPrescription(
	ctx context.Context,
	req prescriptionCreateRequest,
	medication *pharmac.Medicine,
	subsidised bool,
	warnings []string,
	tenantID string,
) (Prescription, error) {
	subsidyCode := ""

	row := h.pool.QueryRow(ctx,
		`INSERT INTO prescriptions
		   (patient_id, patient_nhi, practitioner_hpi, encounter_id,
		    nzulm_code, medication_name, status,
		    dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		    repeats, repeats_remaining, tenant_id, issued_at)
		 VALUES
		   (@patient_id, @patient_nhi, @practitioner_hpi, @encounter_id,
		    @nzulm_code, @medication_name, @status,
		    @dosage, @pharmac_subsidised, @subsidy_code, @interaction_warnings,
		    @repeats, @repeats_remaining, @tenant_id, now())
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, encounter_id,
		           nzulm_code, medication_name, status,
		           dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		           repeats, repeats_remaining, tenant_id,
		           issued_at, expires_at, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":           req.PatientID,
			"patient_nhi":          req.PatientNHI,
			"practitioner_hpi":     req.PractitionerHPI,
			"encounter_id":         req.EncounterID,
			"nzulm_code":           req.NZULMCode,
			"medication_name":      medication.GenericName,
			"status":               PrescriptionStatusActive,
			"dosage":               req.Dosage,
			"pharmac_subsidised":   subsidised,
			"subsidy_code":         subsidyCode,
			"interaction_warnings": warnings,
			"repeats":              req.Repeats,
			"repeats_remaining":    req.Repeats,
			"tenant_id":            tenantID,
		},
	)
	rx, err := scanPrescription(row)
	if err != nil {
		return Prescription{}, fmt.Errorf("insert prescription: %w", err)
	}
	return rx, nil
}

// updatePrescription writes status/dosage/repeat changes back to the database.
func (h *PrescriptionsHandler) updatePrescription(ctx context.Context, rx Prescription) (Prescription, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE prescriptions
		 SET status     = @status,
		     dosage     = @dosage,
		     repeats    = @repeats,
		     updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, encounter_id,
		           nzulm_code, medication_name, status,
		           dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		           repeats, repeats_remaining, tenant_id,
		           issued_at, expires_at, created_at, updated_at`,
		db.NamedArgs{
			"status":    rx.Status,
			"dosage":    rx.Dosage,
			"repeats":   rx.Repeats,
			"id":        rx.ID,
			"tenant_id": rx.TenantID,
		},
	)
	updated, err := scanPrescription(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Prescription{}, errNotFound
		}
		return Prescription{}, fmt.Errorf("update prescription: %w", err)
	}
	return updated, nil
}

// prescriptionADERequest is the body for POST /api/v1/prescriptions/{id}/ade.
// The prescriber fills this in when they suspect the prescribed medicine caused
// an adverse event in the patient. Medicines Act 1981 s45 obliges reporters
// to notify Medsafe via the CARM system.
type prescriptionADERequest struct {
	EventDate        time.Time            `json:"eventDate"`
	EventDescription string               `json:"eventDescription"`
	Seriousness      medsafe.Seriousness  `json:"seriousness"`
	Outcome          string               `json:"outcome,omitempty"`
	PatientAge       int                  `json:"patientAge,omitempty"`
	PatientSex       string               `json:"patientSex,omitempty"`
	RelevantHistory  string               `json:"relevantHistory,omitempty"`
}

// ReportADE handles POST /api/v1/prescriptions/{id}/ade.
// Allows a prescriber to report a suspected adverse drug event to Medsafe/CARM
// for a medicine they have prescribed.
func (h *PrescriptionsHandler) ReportADE(w http.ResponseWriter, r *http.Request) {
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

	if h.medsafeClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "MEDSAFE_DISABLED", Message: "Medsafe ADE reporting is not configured"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "prescription ID is required"})
		return
	}

	var req prescriptionADERequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.EventDescription == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DESCRIPTION", Message: "eventDescription is required"})
		return
	}
	if req.Seriousness == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_SERIOUSNESS", Message: "seriousness is required"})
		return
	}

	rx, err := h.getPrescriptionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "prescription not found"})
			return
		}
		h.logger.Error("get prescription for ADE", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve prescription"})
		return
	}

	report := medsafe.ADEReport{
		PatientNHI:       rx.PatientNHI,
		PatientAge:       req.PatientAge,
		PatientSex:       req.PatientSex,
		ReporterHPI:      rx.PractitionerHPI,
		ReporterType:     "prescriber",
		EventDate:        req.EventDate,
		EventDescription: req.EventDescription,
		Seriousness:      req.Seriousness,
		Outcome:          req.Outcome,
		RelevantHistory:  req.RelevantHistory,
		SuspectDrugs: []medsafe.SuspectDrug{
			{
				NZULM:       rx.NZULMCode,
				GenericName: rx.MedicationName,
				Dose:        fmt.Sprintf("%.4g %s", rx.Dosage.DoseValue, rx.Dosage.DoseUnit),
				Route:       rx.Dosage.Route,
				StartDate:   &rx.IssuedAt,
				Causality:   medsafe.CausalityPossible,
			},
		},
	}

	submitted, err := h.medsafeClient.SubmitADE(ctx, report)
	if err != nil {
		h.logger.Error("medsafe ADE submission failed",
			slog.String("prescription", id),
			slog.Any("error", err),
		)
		writeJSON(w, http.StatusBadGateway, apiError{Code: "MEDSAFE_ERROR", Message: "ADE report submission failed"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ADEReport",
		ResourceID:   submitted.ID.String(),
		TenantID:     tenantID,
		Metadata:     map[string]string{"prescription": id, "nzulm": rx.NZULMCode},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, submitted)
}

// prescriptionDispatchRequest is the body for POST /api/v1/prescriptions/{id}/dispatch.
type prescriptionDispatchRequest struct {
	PharmacyHPI string `json:"pharmacyHpi"`
	Quantity    int    `json:"quantity,omitempty"` // defaults to 1 when zero or absent
	IsUrgent    bool   `json:"isUrgent,omitempty"`
}

// Dispatch handles POST /api/v1/prescriptions/{id}/dispatch.
// Electronically routes the prescription to the named community pharmacy via the
// pharmacy gateway, replacing print/fax for in-network pharmacies. Returns 503
// when the gateway is not configured.
func (h *PrescriptionsHandler) Dispatch(w http.ResponseWriter, r *http.Request) {
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

	if h.pharmacyGateway == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "GATEWAY_DISABLED", Message: "pharmacy gateway is not configured"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "prescription ID is required"})
		return
	}

	var req prescriptionDispatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PharmacyHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PHARMACY_HPI", Message: "pharmacyHpi is required"})
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	rx, err := h.getPrescriptionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "prescription not found"})
			return
		}
		h.logger.Error("get prescription for dispatch", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve prescription"})
		return
	}

	if rx.Status == PrescriptionStatusCancelled || rx.Status == PrescriptionStatusStopped {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "CANNOT_DISPATCH",
			Message: fmt.Sprintf("prescription is %s and cannot be dispatched", rx.Status),
		})
		return
	}

	dispatchReq := pharmacygateway.DispatchRequest{
		MedicationRequestID: rx.ID,
		PatientNHI:          rx.PatientNHI,
		PrescriberHPI:       rx.PractitionerHPI,
		PharmacyHPI:         req.PharmacyHPI,
		NZULM:               rx.NZULMCode,
		BrandName:           rx.MedicationName,
		Dose:                fmt.Sprintf("%.4g %s", rx.Dosage.DoseValue, rx.Dosage.DoseUnit),
		Route:               rx.Dosage.Route,
		Frequency:           rx.Dosage.Frequency,
		Quantity:            req.Quantity,
		Repeats:             rx.RepeatsRemaining,
		Instructions:        rx.Dosage.Text,
		IsUrgent:            req.IsUrgent,
	}

	result, err := h.pharmacyGateway.Dispatch(ctx, dispatchReq)
	if err != nil {
		h.logger.Error("pharmacy gateway dispatch failed",
			slog.String("prescription", id),
			slog.String("pharmacyHpi", req.PharmacyHPI),
			slog.Any("error", err),
		)
		writeJSON(w, http.StatusBadGateway, apiError{Code: "DISPATCH_ERROR", Message: "electronic prescription dispatch failed"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "MedicationRequest",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata: map[string]string{
			"action":      "dispatch",
			"pharmacyHpi": req.PharmacyHPI,
			"connector":   string(result.Connector),
			"externalId":  result.ExternalID,
		},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, result)
}

// scanPrescription scans a single Prescription from a row (pgx.Row or pgx.Rows).
func scanPrescription(row dbRow) (Prescription, error) {
	var rx Prescription
	if err := row.Scan(
		&rx.ID, &rx.PatientID, &rx.PatientNHI, &rx.PractitionerHPI, &rx.EncounterID,
		&rx.NZULMCode, &rx.MedicationName, &rx.Status,
		&rx.Dosage, &rx.PHARMACSubsidised, &rx.SubsidyCode, &rx.InteractionWarnings,
		&rx.Repeats, &rx.RepeatsRemaining, &rx.TenantID,
		&rx.IssuedAt, &rx.ExpiresAt, &rx.CreatedAt, &rx.UpdatedAt,
	); err != nil {
		return Prescription{}, err
	}
	return rx, nil
}
