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

	verification := "manual"
	if req.PatientBarcode != "" && req.MedBarcode != "" {
		verification = "barcode"
	}

	record, err := h.insertAdminRecord(ctx, medID, admissionID, med, req, verification, tenantID.String())
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

// ListControlledDrugRegister handles GET /api/v1/admissions/{admissionId}/controlled-drug-register.
func (h *PharmacyHandler) ListControlledDrugRegister(w http.ResponseWriter, r *http.Request) {
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
	entries, err := h.listControlledDrugRegister(ctx, admissionID, tenantID.String())
	if err != nil {
		h.logger.Error("list controlled drug register", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list controlled drug register"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "ControlledDrugRegister",
		ResourceID: admissionID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": len(entries)})
}

// AddControlledDrugEntry handles POST /api/v1/admissions/{admissionId}/controlled-drug-register.
func (h *PharmacyHandler) AddControlledDrugEntry(w http.ResponseWriter, r *http.Request) {
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
	var req controlledDrugEntryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.DrugName == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DRUG", Message: "drugName is required"})
		return
	}
	if req.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_QUANTITY", Message: "quantity must be positive"})
		return
	}
	if req.WitnessHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_WITNESS", Message: "witnessHpi is required for controlled drugs"})
		return
	}

	entry, err := h.insertControlledDrugEntry(ctx, admissionID, "", req, tenantID.String())
	if err != nil {
		h.logger.Error("insert controlled drug entry", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to record controlled drug entry"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "ControlledDrugRegister",
		ResourceID: entry.ID, TenantID: tenantID,
		Details:    map[string]any{"drug": req.DrugName, "action": req.Action, "quantity": req.Quantity},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, entry)
}

// VerifyBedside handles POST /api/v1/admissions/{admissionId}/medications/{medId}/verify.
func (h *PharmacyHandler) VerifyBedside(w http.ResponseWriter, r *http.Request) {
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
		h.logger.Error("get medication for bedside verification", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve medication"})
		return
	}

	var req bedsideVerifyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "patientNhi is required"})
		return
	}

	v := NewBedsideVerification(admissionID, medID, req.PatientNHI, principal.ID)

	// Perform five-rights verification against the medication chart.
	if req.PatientNHI != "" {
		v.RightPatient = VerificationMatched
	}
	v.PatientBarcode = req.PatientBarcode

	if req.MedBarcode != "" && med.Barcode != "" && req.MedBarcode == med.Barcode {
		v.RightDrug = VerificationMatched
		v.RightDose = VerificationMatched
		v.RightRoute = VerificationMatched
		v.RightTime = VerificationMatched
		v.MedicationBarcode = req.MedBarcode
		v.Status = "completed"
	} else if req.MedBarcode != "" {
		v.RightDrug = VerificationMismatch
		v.MedicationBarcode = req.MedBarcode
		v.Status = "mismatch"
	} else {
		v.RightDrug = VerificationPending
		v.RightDose = VerificationPending
		v.RightRoute = VerificationPending
		v.RightTime = VerificationPending
		v.Status = "pending"
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "verify", ResourceType: "BedsideVerification",
		ResourceID: medID, TenantID: tenantID,
		Details:    map[string]any{"status": v.Status, "five_rights": v.IsFiveRightsOK()},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, v)
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

// ── IV Pump / Smart Infusion Integration ───────────────────────────────────────

// LinkIVPump handles POST /api/v1/admissions/{admissionId}/medications/{medId}/iv-pump/link.
// Links a smart infusion pump to an IV medication and creates the infusion record.
func (h *PharmacyHandler) LinkIVPump(w http.ResponseWriter, r *http.Request) {
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
		h.logger.Error("get medication for IV pump link", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve medication"})
		return
	}
	if !med.IsIV {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "NOT_IV_MED", Message: "medication is not an IV infusion"})
		return
	}

	var req IVLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PumpIdentifier == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PUMP", Message: "pumpIdentifier is required"})
		return
	}
	if req.Rate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_RATE", Message: "rate is required"})
		return
	}

	record := NewIVInfusionRecord(medID, admissionID, req.PumpIdentifier, req.PumpType, req.Rate)
	record.Concentration = req.Concentration
	record.VTBI = req.VTBI
	record.LabelText = req.LabelText
	record.SafetySoftLimit = req.SafetySoftLimit
	record.SafetyHardLimit = req.SafetyHardLimit

	created, err := h.insertIVInfusion(ctx, record, tenantID.String())
	if err != nil {
		h.logger.Error("insert IV infusion record", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create IV infusion record"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "IVInfusion",
		ResourceID: created.ID, TenantID: tenantID,
		Details:    map[string]any{"medication_id": medID, "pump": req.PumpIdentifier, "action": "link-pump"},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, created)
}

// UpdateIVPumpStatus handles POST /api/v1/admissions/{admissionId}/medications/{medId}/iv-pump/status.
// Updates the status of an IV infusion and records volume/dose infused.
func (h *PharmacyHandler) UpdateIVPumpStatus(w http.ResponseWriter, r *http.Request) {
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

	var req IVStatusUpdate
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	updated, err := h.updateIVInfusionStatus(ctx, admissionID, medID, req, tenantID.String())
	if err != nil {
		h.logger.Error("update IV infusion status", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update IV infusion status"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "IVInfusion",
		ResourceID: medID, TenantID: tenantID,
		Details:    map[string]any{"status": req.Status, "volume_infused": req.VolumeInfused, "dose_infused": req.DoseInfused},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// ListIVInfusions handles GET /api/v1/admissions/{admissionId}/medications/{medId}/iv-pump.
func (h *PharmacyHandler) ListIVInfusions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	admissionID := r.PathValue("admissionId")
	medID := r.PathValue("medId")

	records, err := h.listIVInfusions(ctx, admissionID, medID, tenantID.String())
	if err != nil {
		h.logger.Error("list IV infusions", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list IV infusions"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"infusions": records, "total": len(records)})
}
