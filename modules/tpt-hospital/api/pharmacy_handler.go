package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

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
