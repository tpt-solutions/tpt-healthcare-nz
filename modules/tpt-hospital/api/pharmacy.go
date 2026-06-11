// Package api — inpatient pharmacy: medication charts, administration records,
// IV pharmacy, and admission/discharge medication reconciliation.
// NOTE: Community dispensing is handled by tpt-pharmacy. This package covers
// only in-hospital prescribing and administration.
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

// InpatientMedStatus tracks the state of a medication on an inpatient chart.
type InpatientMedStatus string

const (
	InpatientMedStatusActive    InpatientMedStatus = "active"
	InpatientMedStatusOnHold    InpatientMedStatus = "on-hold"
	InpatientMedStatusCeased    InpatientMedStatus = "ceased"
	InpatientMedStatusCompleted InpatientMedStatus = "completed"
)

// RouteOfAdministration enumerates common drug delivery routes.
type RouteOfAdministration string

const (
	RouteOral  RouteOfAdministration = "oral"
	RouteIV    RouteOfAdministration = "intravenous"
	RouteIM    RouteOfAdministration = "intramuscular"
	RouteSC    RouteOfAdministration = "subcutaneous"
	RouteTopical RouteOfAdministration = "topical"
	RouteINH   RouteOfAdministration = "inhaled"
	RouteNasal RouteOfAdministration = "intranasal"
	RouteRectal RouteOfAdministration = "rectal"
	RouteSL    RouteOfAdministration = "sublingual"
)

// InpatientMedication is a single medication on an inpatient medication chart.
type InpatientMedication struct {
	ID           string                `json:"id"`
	AdmissionID  string                `json:"admissionId"`
	PatientID    string                `json:"patientId"`
	PrescriberHPI string               `json:"prescriberHpi"`
	GenericName  string                `json:"genericName"`
	BrandName    string                `json:"brandName,omitempty"`
	NZMTCode     string                `json:"nzmtCode,omitempty"` // NZMT identifier
	Dose         string                `json:"dose"`               // e.g. "500 mg"
	Route        RouteOfAdministration `json:"route"`
	Frequency    string                `json:"frequency"`           // e.g. "BD", "8-hourly", "PRN"
	MaxDailyDose string                `json:"maxDailyDose,omitempty"`
	Indication   string                `json:"indication,omitempty"`
	StartDate    time.Time             `json:"startDate"`
	EndDate      *time.Time            `json:"endDate,omitempty"`
	Status       InpatientMedStatus    `json:"status"`
	IsIV         bool                  `json:"isIv"`
	IVRate       string                `json:"ivRate,omitempty"`     // e.g. "100 mL/hr"
	AllergiesChecked bool              `json:"allergiesChecked"`
	TenantID     string                `json:"tenantId"`
	CeasedAt     *time.Time            `json:"ceasedAt,omitempty"`
	CeasedReason string                `json:"ceasedReason,omitempty"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
}

// MedAdministrationRecord documents a single administration of a medication.
type MedAdministrationRecord struct {
	ID             string    `json:"id"`
	MedicationID   string    `json:"medicationId"`
	AdmissionID    string    `json:"admissionId"`
	AdministeredBy string    `json:"administeredBy"` // nurse HPI
	ActualDose     string    `json:"actualDose"`
	Route          RouteOfAdministration `json:"route"`
	Notes          string    `json:"notes,omitempty"`
	Withheld       bool      `json:"withheld"`
	WithheldReason string    `json:"withheldReason,omitempty"`
	TenantID       string    `json:"tenantId"`
	AdministeredAt time.Time `json:"administeredAt"`
}

// MedReconciliation is the structured comparison of home medications
// against the inpatient chart, performed on admission and discharge.
type MedReconciliation struct {
	ID           string    `json:"id"`
	AdmissionID  string    `json:"admissionId"`
	ClinicianHPI string    `json:"clinicianHpi"`
	ReconciliationType string `json:"type"` // "admission" or "discharge"
	HomeMedications    []string `json:"homeMedications"`    // from community / NZMT
	ChartMedications   []string `json:"chartMedications"`   // inpatient chart
	Discrepancies      []string `json:"discrepancies,omitempty"`
	ActionsTaken       []string `json:"actionsTaken,omitempty"`
	ClinicalNotes      string   `json:"clinicalNotes,omitempty"`
	TenantID           string   `json:"tenantId"`
	CompletedAt        time.Time `json:"completedAt"`
}

type medPrescribeRequest struct {
	PrescriberHPI string                `json:"prescriberHpi"`
	GenericName   string                `json:"genericName"`
	BrandName     string                `json:"brandName,omitempty"`
	NZMTCode      string                `json:"nzmtCode,omitempty"`
	Dose          string                `json:"dose"`
	Route         RouteOfAdministration `json:"route"`
	Frequency     string                `json:"frequency"`
	MaxDailyDose  string                `json:"maxDailyDose,omitempty"`
	Indication    string                `json:"indication,omitempty"`
	StartDate     time.Time             `json:"startDate"`
	EndDate       *time.Time            `json:"endDate,omitempty"`
	IsIV          bool                  `json:"isIv,omitempty"`
	IVRate        string                `json:"ivRate,omitempty"`
}

type medAdminRequest struct {
	AdministeredBy string                `json:"administeredBy"`
	ActualDose     string                `json:"actualDose"`
	Route          RouteOfAdministration `json:"route,omitempty"`
	Notes          string                `json:"notes,omitempty"`
	Withheld       bool                  `json:"withheld,omitempty"`
	WithheldReason string                `json:"withheldReason,omitempty"`
}

type medCeaseRequest struct {
	Reason string `json:"reason"`
}

type medReconcileRequest struct {
	ClinicianHPI        string   `json:"clinicianHpi"`
	ReconciliationType  string   `json:"type"`
	HomeMedications     []string `json:"homeMedications"`
	Discrepancies       []string `json:"discrepancies,omitempty"`
	ActionsTaken        []string `json:"actionsTaken,omitempty"`
	ClinicalNotes       string   `json:"clinicalNotes,omitempty"`
}

// PharmacyHandler handles inpatient pharmacy routes.
type PharmacyHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/admissions/{admissionId}/medications.
func (h *PharmacyHandler) List(w http.ResponseWriter, r *http.Request) {
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
	statusFilter := r.URL.Query().Get("status")
	meds, err := h.listMedications(ctx, admissionID, tenantID.String(), statusFilter)
	if err != nil {
		h.logger.Error("list inpatient medications", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list medications"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "InpatientMedication",
		ResourceID: admissionID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"medications": meds, "total": len(meds)})
}

// Prescribe handles POST /api/v1/admissions/{admissionId}/medications.
func (h *PharmacyHandler) Prescribe(w http.ResponseWriter, r *http.Request) {
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
	var req medPrescribeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.GenericName == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DRUG", Message: "genericName is required"})
		return
	}
	if req.Dose == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DOSE", Message: "dose is required"})
		return
	}
	if req.PrescriberHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PRESCRIBER", Message: "prescriberHpi is required"})
		return
	}

	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.PrescriberHPI)
	if err != nil {
		h.logger.Error("HPI APC validation for prescribing", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate prescriber APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "INVALID_APC", Message: "prescriber does not hold a current Annual Practising Certificate"})
		return
	}

	med, err := h.insertMedication(ctx, admissionID, req, tenantID.String())
	if err != nil {
		h.logger.Error("prescribe medication", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to prescribe medication"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "InpatientMedication",
		ResourceID: med.ID, TenantID: tenantID,
		Details:    map[string]any{"drug": req.GenericName, "action": "prescribe"},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, med)
}

// Get handles GET /api/v1/admissions/{admissionId}/medications/{medId}.
func (h *PharmacyHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	medID := r.PathValue("medId")
	med, err := h.getMedicationByID(ctx, medID, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "medication not found"})
			return
		}
		h.logger.Error("get inpatient medication", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve medication"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "InpatientMedication",
		ResourceID: medID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, med)
}

// Update handles PUT /api/v1/admissions/{admissionId}/medications/{medId}.
func (h *PharmacyHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	medID := r.PathValue("medId")
	existing, err := h.getMedicationByID(ctx, medID, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "medication not found"})
			return
		}
		h.logger.Error("get medication for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve medication"})
		return
	}
	if existing.Status == InpatientMedStatusCeased || existing.Status == InpatientMedStatusCompleted {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot update a ceased or completed medication"})
		return
	}

	var req medPrescribeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Dose != "" {
		existing.Dose = req.Dose
	}
	if req.Frequency != "" {
		existing.Frequency = req.Frequency
	}
	if req.MaxDailyDose != "" {
		existing.MaxDailyDose = req.MaxDailyDose
	}
	if req.Indication != "" {
		existing.Indication = req.Indication
	}
	if req.IVRate != "" {
		existing.IVRate = req.IVRate
	}
	if req.EndDate != nil {
		existing.EndDate = req.EndDate
	}

	updated, err := h.updateMedication(ctx, existing)
	if err != nil {
		h.logger.Error("update medication", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update medication"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "InpatientMedication",
		ResourceID: medID, TenantID: tenantID, Details: map[string]any{"action": "update"},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// Administer handles POST /api/v1/admissions/{admissionId}/medications/{medId}/administer.
func (h *PharmacyHandler) Administer(w http.ResponseWriter, r *http.Request) {
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
	medID := r.PathValue("medId")
	med, err := h.getMedicationByID(ctx, medID, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "medication not found"})
			return
		}
		h.logger.Error("get medication for administration", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve medication"})
		return
	}
	if med.Status == InpatientMedStatusCeased || med.Status == InpatientMedStatusCompleted {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot administer a ceased or completed medication"})
		return
	}

	var req medAdminRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AdministeredBy == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NURSE", Message: "administeredBy (nurse HPI) is required"})
		return
	}

	record, err := h.insertAdminRecord(ctx, medID, admissionID, med, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert administration record", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to record administration"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "MedAdministration",
		ResourceID: record.ID, TenantID: tenantID,
		Details:    map[string]any{"medication_id": medID, "withheld": req.Withheld},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, record)
}

// Cease handles POST /api/v1/admissions/{admissionId}/medications/{medId}/cease.
func (h *PharmacyHandler) Cease(w http.ResponseWriter, r *http.Request) {
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
	medID := r.PathValue("medId")
	existing, err := h.getMedicationByID(ctx, medID, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "medication not found"})
			return
		}
		h.logger.Error("get medication for cease", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve medication"})
		return
	}
	if existing.Status == InpatientMedStatusCeased {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_CEASED", Message: "medication is already ceased"})
		return
	}

	var req medCeaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	now := time.Now().UTC()
	existing.Status = InpatientMedStatusCeased
	existing.CeasedAt = &now
	existing.CeasedReason = req.Reason

	ceased, err := h.updateMedication(ctx, existing)
	if err != nil {
		h.logger.Error("cease medication", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CEASE_ERROR", Message: "failed to cease medication"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "InpatientMedication",
		ResourceID: medID, TenantID: tenantID,
		Details:    map[string]any{"action": "cease", "reason": req.Reason},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, ceased)
}

// GetReconciliation handles GET /api/v1/admissions/{admissionId}/medications/reconciliation.
func (h *PharmacyHandler) GetReconciliation(w http.ResponseWriter, r *http.Request) {
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
	rec, err := h.getReconciliation(ctx, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "no reconciliation found for this admission"})
			return
		}
		h.logger.Error("get reconciliation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve reconciliation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "MedReconciliation",
		ResourceID: admissionID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, rec)
}

// ReconcileMedications handles POST /api/v1/admissions/{admissionId}/medications/reconciliation.
func (h *PharmacyHandler) ReconcileMedications(w http.ResponseWriter, r *http.Request) {
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
	var req medReconcileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ClinicianHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLINICIAN", Message: "clinicianHpi is required"})
		return
	}
	if req.ReconciliationType != "admission" && req.ReconciliationType != "discharge" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_TYPE", Message: "type must be 'admission' or 'discharge'"})
		return
	}

	// Fetch current chart medications to compare.
	chartMeds, err := h.listMedications(ctx, admissionID, tenantID.String(), "")
	if err != nil {
		h.logger.Error("get chart meds for reconciliation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to retrieve chart medications"})
		return
	}
	chartNames := make([]string, 0, len(chartMeds))
	for _, m := range chartMeds {
		chartNames = append(chartNames, m.GenericName)
	}

	rec, err := h.insertReconciliation(ctx, admissionID, req, chartNames, tenantID.String())
	if err != nil {
		h.logger.Error("insert reconciliation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create reconciliation record"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "MedReconciliation",
		ResourceID: rec.ID, TenantID: tenantID,
		Details:    map[string]any{"type": req.ReconciliationType},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, rec)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *PharmacyHandler) listMedications(ctx context.Context, admissionID, tenantID, statusFilter string) ([]InpatientMedication, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		        nzmt_code, dose, route, frequency, max_daily_dose, indication,
		        start_date, end_date, status, is_iv, iv_rate, allergies_checked,
		        tenant_id, ceased_at, ceased_reason, created_at, updated_at
		 FROM inpatient_medications
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY created_at ASC`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query inpatient medications: %w", err)
	}
	defer rows.Close()

	var results []InpatientMedication
	for rows.Next() {
		m, err := scanMedRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

func (h *PharmacyHandler) getMedicationByID(ctx context.Context, id, admissionID, tenantID string) (InpatientMedication, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		        nzmt_code, dose, route, frequency, max_daily_dose, indication,
		        start_date, end_date, status, is_iv, iv_rate, allergies_checked,
		        tenant_id, ceased_at, ceased_reason, created_at, updated_at
		 FROM inpatient_medications
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "admission_id": admissionID, "tenant_id": tenantID},
	)
	m, err := scanMedRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return InpatientMedication{}, errNotFound
		}
		return InpatientMedication{}, fmt.Errorf("get inpatient medication: %w", err)
	}
	return m, nil
}

func (h *PharmacyHandler) insertMedication(ctx context.Context, admissionID string, req medPrescribeRequest, tenantID string) (InpatientMedication, error) {
	var patientID string
	if err := h.pool.QueryRow(ctx,
		`SELECT patient_id FROM hospital_admissions WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": admissionID, "tenant_id": tenantID},
	).Scan(&patientID); err != nil {
		if db.IsNoRows(err) {
			return InpatientMedication{}, errNotFound
		}
		return InpatientMedication{}, fmt.Errorf("get admission for medication: %w", err)
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO inpatient_medications
		   (admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		    nzmt_code, dose, route, frequency, max_daily_dose, indication,
		    start_date, end_date, status, is_iv, iv_rate, tenant_id)
		 VALUES
		   (@admission_id, @patient_id, @prescriber_hpi, @generic_name, @brand_name,
		    @nzmt_code, @dose, @route, @frequency, @max_daily_dose, @indication,
		    @start_date, @end_date, @status, @is_iv, @iv_rate, @tenant_id)
		 RETURNING id, admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		           nzmt_code, dose, route, frequency, max_daily_dose, indication,
		           start_date, end_date, status, is_iv, iv_rate, allergies_checked,
		           tenant_id, ceased_at, ceased_reason, created_at, updated_at`,
		db.NamedArgs{
			"admission_id":   admissionID,
			"patient_id":     patientID,
			"prescriber_hpi": req.PrescriberHPI,
			"generic_name":   req.GenericName,
			"brand_name":     req.BrandName,
			"nzmt_code":      req.NZMTCode,
			"dose":           req.Dose,
			"route":          req.Route,
			"frequency":      req.Frequency,
			"max_daily_dose": req.MaxDailyDose,
			"indication":     req.Indication,
			"start_date":     req.StartDate,
			"end_date":       req.EndDate,
			"status":         InpatientMedStatusActive,
			"is_iv":          req.IsIV,
			"iv_rate":        req.IVRate,
			"tenant_id":      tenantID,
		},
	)
	return scanMedRow(row)
}

func (h *PharmacyHandler) updateMedication(ctx context.Context, m InpatientMedication) (InpatientMedication, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE inpatient_medications
		 SET dose = @dose, frequency = @frequency, max_daily_dose = @max_daily_dose,
		     indication = @indication, end_date = @end_date, status = @status,
		     iv_rate = @iv_rate, ceased_at = @ceased_at, ceased_reason = @ceased_reason,
		     updated_at = now()
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id
		 RETURNING id, admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		           nzmt_code, dose, route, frequency, max_daily_dose, indication,
		           start_date, end_date, status, is_iv, iv_rate, allergies_checked,
		           tenant_id, ceased_at, ceased_reason, created_at, updated_at`,
		db.NamedArgs{
			"dose":          m.Dose,
			"frequency":     m.Frequency,
			"max_daily_dose": m.MaxDailyDose,
			"indication":    m.Indication,
			"end_date":      m.EndDate,
			"status":        m.Status,
			"iv_rate":       m.IVRate,
			"ceased_at":     m.CeasedAt,
			"ceased_reason": m.CeasedReason,
			"id":            m.ID,
			"admission_id":  m.AdmissionID,
			"tenant_id":     m.TenantID,
		},
	)
	updated, err := scanMedRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return InpatientMedication{}, errNotFound
		}
		return InpatientMedication{}, fmt.Errorf("update inpatient medication: %w", err)
	}
	return updated, nil
}

func (h *PharmacyHandler) insertAdminRecord(ctx context.Context, medID, admissionID string, med InpatientMedication, req medAdminRequest, tenantID string) (MedAdministrationRecord, error) {
	route := req.Route
	if route == "" {
		route = med.Route
	}
	actualDose := req.ActualDose
	if actualDose == "" {
		actualDose = med.Dose
	}
	row := h.pool.QueryRow(ctx,
		`INSERT INTO med_administration_records
		   (medication_id, admission_id, administered_by, actual_dose, route,
		    notes, withheld, withheld_reason, tenant_id, administered_at)
		 VALUES
		   (@medication_id, @admission_id, @administered_by, @actual_dose, @route,
		    @notes, @withheld, @withheld_reason, @tenant_id, now())
		 RETURNING id, medication_id, admission_id, administered_by, actual_dose, route,
		           notes, withheld, withheld_reason, tenant_id, administered_at`,
		db.NamedArgs{
			"medication_id":   medID,
			"admission_id":    admissionID,
			"administered_by": req.AdministeredBy,
			"actual_dose":     actualDose,
			"route":           route,
			"notes":           req.Notes,
			"withheld":        req.Withheld,
			"withheld_reason": req.WithheldReason,
			"tenant_id":       tenantID,
		},
	)
	var rec MedAdministrationRecord
	if err := row.Scan(
		&rec.ID, &rec.MedicationID, &rec.AdmissionID, &rec.AdministeredBy,
		&rec.ActualDose, &rec.Route, &rec.Notes, &rec.Withheld, &rec.WithheldReason,
		&rec.TenantID, &rec.AdministeredAt,
	); err != nil {
		return MedAdministrationRecord{}, fmt.Errorf("insert administration record: %w", err)
	}
	return rec, nil
}

func (h *PharmacyHandler) getReconciliation(ctx context.Context, admissionID, tenantID string) (MedReconciliation, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, admission_id, clinician_hpi, reconciliation_type,
		        home_medications, chart_medications, discrepancies, actions_taken, clinical_notes,
		        tenant_id, completed_at
		 FROM med_reconciliations
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		 ORDER BY completed_at DESC LIMIT 1`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID},
	)
	var rec MedReconciliation
	if err := row.Scan(
		&rec.ID, &rec.AdmissionID, &rec.ClinicianHPI, &rec.ReconciliationType,
		&rec.HomeMedications, &rec.ChartMedications, &rec.Discrepancies, &rec.ActionsTaken, &rec.ClinicalNotes,
		&rec.TenantID, &rec.CompletedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return MedReconciliation{}, errNotFound
		}
		return MedReconciliation{}, fmt.Errorf("get reconciliation: %w", err)
	}
	return rec, nil
}

func (h *PharmacyHandler) insertReconciliation(ctx context.Context, admissionID string, req medReconcileRequest, chartNames []string, tenantID string) (MedReconciliation, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO med_reconciliations
		   (admission_id, clinician_hpi, reconciliation_type,
		    home_medications, chart_medications, discrepancies, actions_taken, clinical_notes,
		    tenant_id, completed_at)
		 VALUES
		   (@admission_id, @clinician_hpi, @reconciliation_type,
		    @home_medications, @chart_medications, @discrepancies, @actions_taken, @clinical_notes,
		    @tenant_id, now())
		 RETURNING id, admission_id, clinician_hpi, reconciliation_type,
		           home_medications, chart_medications, discrepancies, actions_taken, clinical_notes,
		           tenant_id, completed_at`,
		db.NamedArgs{
			"admission_id":       admissionID,
			"clinician_hpi":      req.ClinicianHPI,
			"reconciliation_type": req.ReconciliationType,
			"home_medications":   req.HomeMedications,
			"chart_medications":  chartNames,
			"discrepancies":      req.Discrepancies,
			"actions_taken":      req.ActionsTaken,
			"clinical_notes":     req.ClinicalNotes,
			"tenant_id":          tenantID,
		},
	)
	var rec MedReconciliation
	if err := row.Scan(
		&rec.ID, &rec.AdmissionID, &rec.ClinicianHPI, &rec.ReconciliationType,
		&rec.HomeMedications, &rec.ChartMedications, &rec.Discrepancies, &rec.ActionsTaken, &rec.ClinicalNotes,
		&rec.TenantID, &rec.CompletedAt,
	); err != nil {
		return MedReconciliation{}, fmt.Errorf("insert reconciliation: %w", err)
	}
	return rec, nil
}

func scanMedRow(row dbRow) (InpatientMedication, error) {
	var m InpatientMedication
	if err := row.Scan(
		&m.ID, &m.AdmissionID, &m.PatientID, &m.PrescriberHPI,
		&m.GenericName, &m.BrandName, &m.NZMTCode,
		&m.Dose, &m.Route, &m.Frequency, &m.MaxDailyDose, &m.Indication,
		&m.StartDate, &m.EndDate, &m.Status,
		&m.IsIV, &m.IVRate, &m.AllergiesChecked,
		&m.TenantID, &m.CeasedAt, &m.CeasedReason, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return InpatientMedication{}, err
	}
	return m, nil
}
