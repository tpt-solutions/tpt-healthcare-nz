package api

import (
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

// AdmissionsHandler handles all /api/v1/admissions routes.
type AdmissionsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/admissions.
func (h *AdmissionsHandler) List(w http.ResponseWriter, r *http.Request) {
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
	statusFilter := q.Get("status")
	wardFilter := q.Get("ward")
	typeFilter := q.Get("type")

	admissions, err := h.listAdmissions(ctx, tenantID.String(), statusFilter, wardFilter, typeFilter)
	if err != nil {
		h.logger.Error("list admissions", slog.Any("error", err), slog.String("tenant", tenantID.String()))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list admissions"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "Admission",
		ResourceID: "list", TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"admissions": admissions, "total": len(admissions)})
}

// Create handles POST /api/v1/admissions.
func (h *AdmissionsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req admissionCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.AdmittingClinicianHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLINICIAN", Message: "admittingClinicianHpi is required"})
		return
	}

	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.AdmittingClinicianHPI)
	if err != nil {
		h.logger.Error("HPI APC validation", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate clinician APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "INVALID_APC", Message: "clinician does not hold a current Annual Practising Certificate", Details: apcStatus})
		return
	}

	adm, err := h.insertAdmission(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create admission"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "Admission",
		ResourceID: adm.ID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, adm)
}

// Get handles GET /api/v1/admissions/{id}.
func (h *AdmissionsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	adm, err := h.getAdmissionByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "Admission",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, adm)
}

// Update handles PUT /api/v1/admissions/{id}.
func (h *AdmissionsHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAdmissionByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}
	if existing.Status == AdmissionStatusDischarged || existing.Status == AdmissionStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: fmt.Sprintf("cannot update admission in %s status", existing.Status)})
		return
	}

	var req admissionUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.ResponsibleClinicianHPI != "" {
		existing.ResponsibleClinicianHPI = req.ResponsibleClinicianHPI
	}
	if req.WardID != "" {
		existing.WardID = req.WardID
	}
	if req.BedID != "" {
		existing.BedID = req.BedID
	}
	if req.PrimaryDiagnosis != "" {
		existing.PrimaryDiagnosis = req.PrimaryDiagnosis
	}
	if req.AdmissionReason != "" {
		existing.AdmissionReason = req.AdmissionReason
	}

	updated, err := h.updateAdmission(ctx, existing)
	if err != nil {
		h.logger.Error("update admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update admission"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "Admission",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// Discharge handles POST /api/v1/admissions/{id}/discharge.
func (h *AdmissionsHandler) Discharge(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAdmissionByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission for discharge", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}
	if existing.Status == AdmissionStatusDischarged {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_DISCHARGED", Message: "admission is already discharged"})
		return
	}

	var req dischargeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Destination == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DESTINATION", Message: "discharge destination is required"})
		return
	}

	now := time.Now().UTC()
	existing.Status = AdmissionStatusDischarged
	existing.DischargedAt = &now
	existing.DischargeDestination = req.Destination
	existing.DischargeNotes = req.DischargeNotes

	discharged, err := h.dischargeAdmission(ctx, existing)
	if err != nil {
		h.logger.Error("discharge admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DISCHARGE_ERROR", Message: "failed to discharge admission"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "Admission",
		ResourceID: id, TenantID: tenantID,
		Details:    map[string]any{"action": "discharge", "destination": string(req.Destination)},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, discharged)
}

// Transfer handles POST /api/v1/admissions/{id}/transfer.
func (h *AdmissionsHandler) Transfer(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAdmissionByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission for transfer", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}
	if existing.Status == AdmissionStatusDischarged || existing.Status == AdmissionStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot transfer a discharged or cancelled admission"})
		return
	}

	var req transferRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ToWardID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_WARD", Message: "toWardId is required"})
		return
	}

	existing.WardID = req.ToWardID
	existing.BedID = req.ToBedID
	existing.Status = AdmissionStatusTransferred

	transferred, err := h.updateAdmission(ctx, existing)
	if err != nil {
		h.logger.Error("transfer admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "TRANSFER_ERROR", Message: "failed to transfer admission"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "Admission",
		ResourceID: id, TenantID: tenantID,
		Details:    map[string]any{"action": "transfer", "to_ward": req.ToWardID},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, transferred)
}

// GetDischargeSummary handles GET /api/v1/admissions/{admissionId}/discharge-summary.
func (h *AdmissionsHandler) GetDischargeSummary(w http.ResponseWriter, r *http.Request) {
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
	summary, err := h.getDischargeSummary(ctx, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "discharge summary not found"})
			return
		}
		h.logger.Error("get discharge summary", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve discharge summary"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "DischargeSummary",
		ResourceID: admissionID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, summary)
}

// CreateDischargeSummary handles POST /api/v1/admissions/{admissionId}/discharge-summary.
func (h *AdmissionsHandler) CreateDischargeSummary(w http.ResponseWriter, r *http.Request) {
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
	adm, err := h.getAdmissionByID(ctx, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission for discharge summary", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}
	if adm.Status != AdmissionStatusDischarged {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "NOT_DISCHARGED", Message: "discharge summary can only be created for a discharged admission"})
		return
	}

	var req dischargeSummaryCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ClinicalSummary == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_SUMMARY", Message: "clinicalSummary is required"})
		return
	}

	summary, err := h.insertDischargeSummary(ctx, admissionID, adm, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert discharge summary", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create discharge summary"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "DischargeSummary",
		ResourceID: summary.ID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, summary)
}
