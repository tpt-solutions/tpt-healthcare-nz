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

// OutpatientHandler handles hospital outpatient clinic routes.
type OutpatientHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListClinics handles GET /api/v1/outpatient/clinics.
func (h *OutpatientHandler) ListClinics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	specialty := r.URL.Query().Get("specialty")
	clinics, err := h.listClinics(ctx, tenantID.String(), specialty)
	if err != nil {
		h.logger.Error("list outpatient clinics", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list clinics"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinics": clinics, "total": len(clinics)})
}

// GetClinic handles GET /api/v1/outpatient/clinics/{id}.
func (h *OutpatientHandler) GetClinic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	id := r.PathValue("id")
	clinic, err := h.getClinicByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "clinic not found"})
			return
		}
		h.logger.Error("get outpatient clinic", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve clinic"})
		return
	}
	writeJSON(w, http.StatusOK, clinic)
}

// ListAppointments handles GET /api/v1/outpatient/clinics/{id}/appointments.
func (h *OutpatientHandler) ListAppointments(w http.ResponseWriter, r *http.Request) {
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

	clinicID := r.PathValue("id")
	statusFilter := r.URL.Query().Get("status")
	appts, err := h.listAppointments(ctx, clinicID, tenantID.String(), statusFilter)
	if err != nil {
		h.logger.Error("list outpatient appointments", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list appointments"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "OutpatientAppointment",
		ResourceID: clinicID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"appointments": appts, "total": len(appts)})
}

// BookAppointment handles POST /api/v1/outpatient/clinics/{id}/appointments.
func (h *OutpatientHandler) BookAppointment(w http.ResponseWriter, r *http.Request) {
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

	clinicID := r.PathValue("id")
	var req opAppointmentCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.Reason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REASON", Message: "reason is required"})
		return
	}

	appt, err := h.insertAppointment(ctx, clinicID, req, tenantID.String())
	if err != nil {
		h.logger.Error("book outpatient appointment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to book appointment"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "OutpatientAppointment",
		ResourceID: appt.ID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, appt)
}

// UpdateAppointment handles PUT /api/v1/outpatient/clinics/{id}/appointments/{apptId}.
func (h *OutpatientHandler) UpdateAppointment(w http.ResponseWriter, r *http.Request) {
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

	clinicID := r.PathValue("id")
	apptID := r.PathValue("apptId")
	existing, err := h.getAppointmentByID(ctx, apptID, clinicID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "appointment not found"})
			return
		}
		h.logger.Error("get appointment for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve appointment"})
		return
	}
	if existing.Status == OPApptAttended || existing.Status == OPApptCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot update an attended or cancelled appointment"})
		return
	}

	var req opAppointmentUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ClinicianHPI != "" {
		existing.ClinicianHPI = req.ClinicianHPI
	}
	if req.Reason != "" {
		existing.Reason = req.Reason
	}
	if req.Notes != "" {
		existing.Notes = req.Notes
	}
	if req.ScheduledAt != nil {
		existing.ScheduledAt = *req.ScheduledAt
	}

	updated, err := h.updateAppointment(ctx, existing)
	if err != nil {
		h.logger.Error("update outpatient appointment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update appointment"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "OutpatientAppointment",
		ResourceID: apptID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// Attend handles POST /api/v1/outpatient/clinics/{id}/appointments/{apptId}/attend.
func (h *OutpatientHandler) Attend(w http.ResponseWriter, r *http.Request) {
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

	clinicID := r.PathValue("id")
	apptID := r.PathValue("apptId")
	existing, err := h.getAppointmentByID(ctx, apptID, clinicID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "appointment not found"})
			return
		}
		h.logger.Error("get appointment for attend", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve appointment"})
		return
	}
	if existing.Status == OPApptAttended {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_ATTENDED", Message: "appointment is already marked as attended"})
		return
	}

	now := time.Now().UTC()
	existing.Status = OPApptAttended
	existing.AttendedAt = &now

	attended, err := h.updateAppointment(ctx, existing)
	if err != nil {
		h.logger.Error("attend outpatient appointment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ATTEND_ERROR", Message: "failed to mark appointment as attended"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "OutpatientAppointment",
		ResourceID: apptID, TenantID: tenantID, Details: map[string]any{"action": "attend"},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, attended)
}

// ListWaitlist handles GET /api/v1/outpatient/waitlist.
func (h *OutpatientHandler) ListWaitlist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	clinicFilter := r.URL.Query().Get("clinic")
	priorityFilter := r.URL.Query().Get("priority")
	entries, err := h.listWaitlist(ctx, tenantID.String(), clinicFilter, priorityFilter)
	if err != nil {
		h.logger.Error("list waitlist", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list waitlist"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": len(entries)})
}

// AddToWaitlist handles POST /api/v1/outpatient/waitlist.
func (h *OutpatientHandler) AddToWaitlist(w http.ResponseWriter, r *http.Request) {
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

	var req waitlistAddRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.ClinicID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLINIC", Message: "clinicId is required"})
		return
	}
	if req.Reason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REASON", Message: "reason is required"})
		return
	}

	entry, err := h.insertWaitlistEntry(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("add to waitlist", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to add to waitlist"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "WaitlistEntry",
		ResourceID: entry.ID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, entry)
}

// UpdateWaitlistEntry handles PUT /api/v1/outpatient/waitlist/{id}.
func (h *OutpatientHandler) UpdateWaitlistEntry(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Priority   WaitlistPriority `json:"priority,omitempty"`
		TargetDate *time.Time       `json:"targetDate,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	entry, err := h.updateWaitlistEntry(ctx, id, req.Priority, req.TargetDate, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "waitlist entry not found"})
			return
		}
		h.logger.Error("update waitlist entry", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update waitlist entry"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "WaitlistEntry",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, entry)
}

// RemoveFromWaitlist handles DELETE /api/v1/outpatient/waitlist/{id}.
func (h *OutpatientHandler) RemoveFromWaitlist(w http.ResponseWriter, r *http.Request) {
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
	if err := h.deleteWaitlistEntry(ctx, id, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "waitlist entry not found"})
			return
		}
		h.logger.Error("remove from waitlist", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DELETE_ERROR", Message: "failed to remove waitlist entry"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "delete", ResourceType: "WaitlistEntry",
		ResourceID: id, TenantID: tenantID, Details: map[string]any{"action": "remove"},
		OccurredAt: time.Now().UTC(),
	})
	w.WriteHeader(http.StatusNoContent)
}
