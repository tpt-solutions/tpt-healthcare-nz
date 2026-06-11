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

// OutpatientClinic represents a hospital-based specialist outpatient clinic.
type OutpatientClinic struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Specialty   string    `json:"specialty"` // e.g. "cardiology", "orthopaedics", "general surgery"
	LeadClinicianHPI string `json:"leadClinicianHpi,omitempty"`
	Location    string    `json:"location,omitempty"` // building / room
	Active      bool      `json:"active"`
	TenantID    string    `json:"tenantId"`
	CreatedAt   time.Time `json:"createdAt"`
}

// OutpatientAppointmentStatus tracks an outpatient clinic appointment.
type OutpatientAppointmentStatus string

const (
	OPApptBooked    OutpatientAppointmentStatus = "booked"
	OPApptConfirmed OutpatientAppointmentStatus = "confirmed"
	OPApptAttended  OutpatientAppointmentStatus = "attended"
	OPApptDNAd      OutpatientAppointmentStatus = "did-not-attend"
	OPApptCancelled OutpatientAppointmentStatus = "cancelled"
)

// OutpatientAppointment is a booking in a hospital specialist clinic.
type OutpatientAppointment struct {
	ID           string                      `json:"id"`
	ClinicID     string                      `json:"clinicId"`
	PatientID    string                      `json:"patientId"`
	PatientNHI   string                      `json:"patientNhi"`
	ClinicianHPI string                      `json:"clinicianHpi"`
	Status       OutpatientAppointmentStatus `json:"status"`
	ReferralID   string                      `json:"referralId,omitempty"` // from tpt-doctor
	Reason       string                      `json:"reason"`
	Notes        string                      `json:"notes,omitempty"`
	ScheduledAt  time.Time                   `json:"scheduledAt"`
	AttendedAt   *time.Time                  `json:"attendedAt,omitempty"`
	TenantID     string                      `json:"tenantId"`
	CreatedAt    time.Time                   `json:"createdAt"`
	UpdatedAt    time.Time                   `json:"updatedAt"`
}

// WaitlistPriority classifies clinical urgency on the outpatient waitlist.
type WaitlistPriority string

const (
	WaitlistUrgent   WaitlistPriority = "urgent"   // < 4 weeks
	WaitlistSemUrgent WaitlistPriority = "semi-urgent" // 4–8 weeks
	WaitlistRoutine  WaitlistPriority = "routine"  // > 8 weeks
)

// WaitlistEntry represents a patient waiting for a specialist appointment.
type WaitlistEntry struct {
	ID           string           `json:"id"`
	ClinicID     string           `json:"clinicId"`
	PatientID    string           `json:"patientId"`
	PatientNHI   string           `json:"patientNhi"`
	Priority     WaitlistPriority `json:"priority"`
	Reason       string           `json:"reason"`
	ReferralID   string           `json:"referralId,omitempty"`
	AddedAt      time.Time        `json:"addedAt"`
	TargetDate   *time.Time       `json:"targetDate,omitempty"`
	AppointmentID string          `json:"appointmentId,omitempty"` // set when booked off waitlist
	TenantID     string           `json:"tenantId"`
}

type opAppointmentCreateRequest struct {
	PatientID    string    `json:"patientId"`
	PatientNHI   string    `json:"patientNhi"`
	ClinicianHPI string    `json:"clinicianHpi"`
	ReferralID   string    `json:"referralId,omitempty"`
	Reason       string    `json:"reason"`
	ScheduledAt  time.Time `json:"scheduledAt"`
}

type opAppointmentUpdateRequest struct {
	ClinicianHPI string     `json:"clinicianHpi,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	ScheduledAt  *time.Time `json:"scheduledAt,omitempty"`
}

type waitlistAddRequest struct {
	ClinicID   string           `json:"clinicId"`
	PatientID  string           `json:"patientId"`
	PatientNHI string           `json:"patientNhi"`
	Priority   WaitlistPriority `json:"priority"`
	Reason     string           `json:"reason"`
	ReferralID string           `json:"referralId,omitempty"`
	TargetDate *time.Time       `json:"targetDate,omitempty"`
}

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

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *OutpatientHandler) listClinics(ctx context.Context, tenantID, specialty string) ([]OutpatientClinic, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, name, specialty, lead_clinician_hpi, location, active, tenant_id, created_at
		 FROM outpatient_clinics
		 WHERE tenant_id = @tenant_id AND active = true
		   AND (@specialty = '' OR specialty = @specialty)
		 ORDER BY name`,
		db.NamedArgs{"tenant_id": tenantID, "specialty": specialty},
	)
	if err != nil {
		return nil, fmt.Errorf("query outpatient clinics: %w", err)
	}
	defer rows.Close()

	var results []OutpatientClinic
	for rows.Next() {
		var c OutpatientClinic
		if err := rows.Scan(&c.ID, &c.Name, &c.Specialty, &c.LeadClinicianHPI, &c.Location, &c.Active, &c.TenantID, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan clinic: %w", err)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

func (h *OutpatientHandler) getClinicByID(ctx context.Context, id, tenantID string) (OutpatientClinic, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, name, specialty, lead_clinician_hpi, location, active, tenant_id, created_at
		 FROM outpatient_clinics WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	var c OutpatientClinic
	if err := row.Scan(&c.ID, &c.Name, &c.Specialty, &c.LeadClinicianHPI, &c.Location, &c.Active, &c.TenantID, &c.CreatedAt); err != nil {
		if db.IsNoRows(err) {
			return OutpatientClinic{}, errNotFound
		}
		return OutpatientClinic{}, fmt.Errorf("get clinic: %w", err)
	}
	return c, nil
}

func (h *OutpatientHandler) listAppointments(ctx context.Context, clinicID, tenantID, statusFilter string) ([]OutpatientAppointment, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id,
		        reason, notes, scheduled_at, attended_at, tenant_id, created_at, updated_at
		 FROM outpatient_appointments
		 WHERE clinic_id = @clinic_id AND tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY scheduled_at ASC`,
		db.NamedArgs{"clinic_id": clinicID, "tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query outpatient appointments: %w", err)
	}
	defer rows.Close()

	var results []OutpatientAppointment
	for rows.Next() {
		a, err := scanOPApptRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

func (h *OutpatientHandler) getAppointmentByID(ctx context.Context, id, clinicID, tenantID string) (OutpatientAppointment, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id,
		        reason, notes, scheduled_at, attended_at, tenant_id, created_at, updated_at
		 FROM outpatient_appointments
		 WHERE id = @id AND clinic_id = @clinic_id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "clinic_id": clinicID, "tenant_id": tenantID},
	)
	a, err := scanOPApptRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return OutpatientAppointment{}, errNotFound
		}
		return OutpatientAppointment{}, fmt.Errorf("get outpatient appointment: %w", err)
	}
	return a, nil
}

func (h *OutpatientHandler) insertAppointment(ctx context.Context, clinicID string, req opAppointmentCreateRequest, tenantID string) (OutpatientAppointment, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO outpatient_appointments
		   (clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id, reason, scheduled_at, tenant_id)
		 VALUES
		   (@clinic_id, @patient_id, @patient_nhi, @clinician_hpi, @status, @referral_id, @reason, @scheduled_at, @tenant_id)
		 RETURNING id, clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id,
		           reason, notes, scheduled_at, attended_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"clinic_id":    clinicID,
			"patient_id":   req.PatientID,
			"patient_nhi":  req.PatientNHI,
			"clinician_hpi": req.ClinicianHPI,
			"status":       OPApptBooked,
			"referral_id":  req.ReferralID,
			"reason":       req.Reason,
			"scheduled_at": req.ScheduledAt,
			"tenant_id":    tenantID,
		},
	)
	return scanOPApptRow(row)
}

func (h *OutpatientHandler) updateAppointment(ctx context.Context, a OutpatientAppointment) (OutpatientAppointment, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE outpatient_appointments
		 SET clinician_hpi = @clinician_hpi, status = @status, reason = @reason, notes = @notes,
		     scheduled_at = @scheduled_at, attended_at = @attended_at, updated_at = now()
		 WHERE id = @id AND clinic_id = @clinic_id AND tenant_id = @tenant_id
		 RETURNING id, clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id,
		           reason, notes, scheduled_at, attended_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"clinician_hpi": a.ClinicianHPI,
			"status":       a.Status,
			"reason":       a.Reason,
			"notes":        a.Notes,
			"scheduled_at": a.ScheduledAt,
			"attended_at":  a.AttendedAt,
			"id":           a.ID,
			"clinic_id":    a.ClinicID,
			"tenant_id":    a.TenantID,
		},
	)
	updated, err := scanOPApptRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return OutpatientAppointment{}, errNotFound
		}
		return OutpatientAppointment{}, fmt.Errorf("update outpatient appointment: %w", err)
	}
	return updated, nil
}

func (h *OutpatientHandler) listWaitlist(ctx context.Context, tenantID, clinicFilter, priorityFilter string) ([]WaitlistEntry, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, clinic_id, patient_id, patient_nhi, priority, reason, referral_id,
		        added_at, target_date, appointment_id, tenant_id
		 FROM outpatient_waitlist
		 WHERE tenant_id = @tenant_id AND appointment_id IS NULL
		   AND (@clinic_filter    = '' OR clinic_id = @clinic_filter)
		   AND (@priority_filter  = '' OR priority = @priority_filter)
		 ORDER BY CASE priority WHEN 'urgent' THEN 1 WHEN 'semi-urgent' THEN 2 ELSE 3 END, added_at ASC`,
		db.NamedArgs{"tenant_id": tenantID, "clinic_filter": clinicFilter, "priority_filter": priorityFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query waitlist: %w", err)
	}
	defer rows.Close()

	var results []WaitlistEntry
	for rows.Next() {
		var e WaitlistEntry
		if err := rows.Scan(
			&e.ID, &e.ClinicID, &e.PatientID, &e.PatientNHI, &e.Priority, &e.Reason, &e.ReferralID,
			&e.AddedAt, &e.TargetDate, &e.AppointmentID, &e.TenantID,
		); err != nil {
			return nil, fmt.Errorf("scan waitlist entry: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (h *OutpatientHandler) insertWaitlistEntry(ctx context.Context, req waitlistAddRequest, tenantID string) (WaitlistEntry, error) {
	priority := req.Priority
	if priority == "" {
		priority = WaitlistRoutine
	}
	row := h.pool.QueryRow(ctx,
		`INSERT INTO outpatient_waitlist
		   (clinic_id, patient_id, patient_nhi, priority, reason, referral_id, target_date, tenant_id, added_at)
		 VALUES
		   (@clinic_id, @patient_id, @patient_nhi, @priority, @reason, @referral_id, @target_date, @tenant_id, now())
		 RETURNING id, clinic_id, patient_id, patient_nhi, priority, reason, referral_id,
		           added_at, target_date, appointment_id, tenant_id`,
		db.NamedArgs{
			"clinic_id":   req.ClinicID,
			"patient_id":  req.PatientID,
			"patient_nhi": req.PatientNHI,
			"priority":    priority,
			"reason":      req.Reason,
			"referral_id": req.ReferralID,
			"target_date": req.TargetDate,
			"tenant_id":   tenantID,
		},
	)
	var e WaitlistEntry
	if err := row.Scan(
		&e.ID, &e.ClinicID, &e.PatientID, &e.PatientNHI, &e.Priority, &e.Reason, &e.ReferralID,
		&e.AddedAt, &e.TargetDate, &e.AppointmentID, &e.TenantID,
	); err != nil {
		return WaitlistEntry{}, fmt.Errorf("insert waitlist entry: %w", err)
	}
	return e, nil
}

func (h *OutpatientHandler) updateWaitlistEntry(ctx context.Context, id string, priority WaitlistPriority, targetDate *time.Time, tenantID string) (WaitlistEntry, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE outpatient_waitlist
		 SET priority = COALESCE(NULLIF(@priority, ''), priority),
		     target_date = COALESCE(@target_date, target_date)
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, clinic_id, patient_id, patient_nhi, priority, reason, referral_id,
		           added_at, target_date, appointment_id, tenant_id`,
		db.NamedArgs{"priority": priority, "target_date": targetDate, "id": id, "tenant_id": tenantID},
	)
	var e WaitlistEntry
	if err := row.Scan(
		&e.ID, &e.ClinicID, &e.PatientID, &e.PatientNHI, &e.Priority, &e.Reason, &e.ReferralID,
		&e.AddedAt, &e.TargetDate, &e.AppointmentID, &e.TenantID,
	); err != nil {
		if db.IsNoRows(err) {
			return WaitlistEntry{}, errNotFound
		}
		return WaitlistEntry{}, fmt.Errorf("update waitlist entry: %w", err)
	}
	return e, nil
}

func (h *OutpatientHandler) deleteWaitlistEntry(ctx context.Context, id, tenantID string) error {
	tag, err := h.pool.Exec(ctx,
		`DELETE FROM outpatient_waitlist WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	if err != nil {
		return fmt.Errorf("delete waitlist entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

func scanOPApptRow(row dbRow) (OutpatientAppointment, error) {
	var a OutpatientAppointment
	if err := row.Scan(
		&a.ID, &a.ClinicID, &a.PatientID, &a.PatientNHI, &a.ClinicianHPI, &a.Status, &a.ReferralID,
		&a.Reason, &a.Notes, &a.ScheduledAt, &a.AttendedAt, &a.TenantID, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return OutpatientAppointment{}, err
	}
	return a, nil
}
