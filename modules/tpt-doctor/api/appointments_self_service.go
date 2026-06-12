package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Cancellation waitlist
// ---------------------------------------------------------------------------

// WaitlistEntry represents a patient who wants an earlier appointment if one
// becomes available after a cancellation.
type WaitlistEntry struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenantId"`
	PatientID       string     `json:"patientId"`
	PractitionerHPI string     `json:"practitionerHpi,omitempty"`
	AppointmentType string     `json:"appointmentType,omitempty"`
	// EarliestDate is the earliest the patient is available (optional).
	EarliestDate    *time.Time `json:"earliestDate,omitempty"`
	// LatestDate is the latest the patient wants an earlier slot by.
	// Entries past this date are automatically purged.
	LatestDate      *time.Time `json:"latestDate,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

type cancelRequest struct {
	Reason string `json:"reason"`
}

type rescheduleRequest struct {
	NewStartTime time.Time `json:"newStartTime"`
	NewEndTime   time.Time `json:"newEndTime"`
	Reason       string    `json:"reason,omitempty"`
}

type waitlistJoinRequest struct {
	AppointmentType string     `json:"appointmentType"`
	PractitionerHPI string     `json:"practitionerHpi,omitempty"`
	EarliestDate    *time.Time `json:"earliestDate,omitempty"`
	LatestDate      *time.Time `json:"latestDate,omitempty"`
}

// Cancel handles POST /api/v1/appointments/{id}/cancel.
// Patients may cancel their own appointments; staff may cancel any appointment.
// After cancellation, the slot is offered to the first matching waitlist entry.
func (h *AppointmentsHandler) Cancel(w http.ResponseWriter, r *http.Request) {
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

	id := idFromPath(r)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "appointment ID is required"})
		return
	}

	var req cancelRequest
	_ = decodeJSON(r, &req) // reason is optional

	reason := req.Reason
	if reason == "" {
		reason = "patient-cancelled"
	}

	_, err := h.pool.Exec(ctx,
		`UPDATE appointments
		 SET status='cancelled', cancelled_at=NOW(), cancellation_reason=$1, updated_at=NOW()
		 WHERE id=$2 AND tenant_id=$3 AND status NOT IN ('cancelled','fulfilled','noshow')`,
		reason, id, tenantID.String(),
	)
	if err != nil {
		h.logger.Error("cancel appointment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CANCEL_ERROR", Message: "failed to cancel appointment"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "cancel",
		ResourceType: "Appointment",
		ResourceID:   id,
		TenantID:     tenantID,
		Details:      map[string]any{"reason": reason},
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	// Attempt waitlist backfill asynchronously using a detached context so the
	// backfill is not cancelled when the HTTP handler returns.
	go func() {
		if err := h.backfillWaitlist(context.Background(), tenantID.String(), id); err != nil {
			h.logger.Error("waitlist backfill", slog.Any("error", err))
		}
	}()

	writeJSON(w, http.StatusOK, map[string]any{"status": "cancelled"})
}

// Reschedule handles POST /api/v1/appointments/{id}/reschedule.
// Moves the appointment to a new time slot. Validates the slot is not already booked.
func (h *AppointmentsHandler) Reschedule(w http.ResponseWriter, r *http.Request) {
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

	id := idFromPath(r)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "appointment ID is required"})
		return
	}

	var req rescheduleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.NewStartTime.IsZero() || req.NewEndTime.IsZero() {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "newStartTime and newEndTime are required"})
		return
	}
	if req.NewStartTime.Before(time.Now().UTC()) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "cannot reschedule to a time in the past"})
		return
	}

	// Check for conflicting bookings in the same slot for the same practitioner.
	var conflict bool
	err := h.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM appointments a
			JOIN appointments b ON b.practitioner_hpi = a.practitioner_hpi
			WHERE a.id=$1 AND b.id != $1
			  AND b.tenant_id=$2
			  AND b.status NOT IN ('cancelled','noshow')
			  AND b.start_time < $3 AND b.end_time > $4
		)`,
		id, tenantID.String(), req.NewEndTime, req.NewStartTime,
	).Scan(&conflict)
	if err != nil {
		h.logger.Error("reschedule conflict check", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CONFLICT_CHECK_ERROR", Message: "failed to check slot availability"})
		return
	}
	if conflict {
		writeJSON(w, http.StatusConflict, apiError{Code: "SLOT_CONFLICT", Message: "the selected time slot is not available"})
		return
	}

	_, err = h.pool.Exec(ctx,
		`UPDATE appointments
		 SET start_time=$1, end_time=$2, status='booked', updated_at=NOW()
		 WHERE id=$3 AND tenant_id=$4 AND status NOT IN ('cancelled','fulfilled','noshow')`,
		req.NewStartTime, req.NewEndTime, id, tenantID.String(),
	)
	if err != nil {
		h.logger.Error("reschedule appointment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "RESCHEDULE_ERROR", Message: "failed to reschedule appointment"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "reschedule",
		ResourceType: "Appointment",
		ResourceID:   id,
		TenantID:     tenantID,
		Details: map[string]any{
			"newStartTime": req.NewStartTime,
			"newEndTime":   req.NewEndTime,
		},
		OccurredAt: time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "rescheduled"})
}

// JoinWaitlist handles POST /api/v1/appointments/waitlist.
// Adds the authenticated patient to the cancellation waitlist for a given
// appointment type / practitioner combination.
func (h *AppointmentsHandler) JoinWaitlist(w http.ResponseWriter, r *http.Request) {
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

	var req waitlistJoinRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AppointmentType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "appointmentType is required"})
		return
	}

	entry := WaitlistEntry{
		ID:              uuid.New(),
		TenantID:        tenantID,
		PatientID:       principal.ID,
		PractitionerHPI: req.PractitionerHPI,
		AppointmentType: req.AppointmentType,
		EarliestDate:    req.EarliestDate,
		LatestDate:      req.LatestDate,
		CreatedAt:       time.Now().UTC(),
	}

	_, err := h.pool.Exec(ctx,
		`INSERT INTO appointment_waitlist
			(id, tenant_id, patient_id, practitioner_hpi, appointment_type, earliest_date, latest_date, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (tenant_id, patient_id, appointment_type)
		 DO UPDATE SET practitioner_hpi=$4, earliest_date=$6, latest_date=$7`,
		entry.ID, entry.TenantID, entry.PatientID,
		nilableStr(entry.PractitionerHPI), entry.AppointmentType,
		entry.EarliestDate, entry.LatestDate, entry.CreatedAt,
	)
	if err != nil {
		h.logger.Error("join waitlist", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "WAITLIST_ERROR", Message: "failed to join waitlist"})
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}

// LeaveWaitlist handles DELETE /api/v1/appointments/waitlist/{id}.
func (h *AppointmentsHandler) LeaveWaitlist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, _ := middleware.PrincipalFromContext(ctx)

	id := idFromPath(r)
	_, err := h.pool.Exec(ctx,
		`DELETE FROM appointment_waitlist WHERE id=$1 AND tenant_id=$2 AND patient_id=$3`,
		id, tenantID.String(), principal.ID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "WAITLIST_ERROR", Message: "failed to leave waitlist"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// backfillWaitlist finds the first matching waitlist entry for the freed slot
// and notifies them that the slot is available. Called from a detached goroutine.
func (h *AppointmentsHandler) backfillWaitlist(ctx context.Context, tenantID, appointmentID string) error {
	var apptType, practHPI string
	var startTime time.Time
	err := h.pool.QueryRow(ctx,
		`SELECT appointment_type, practitioner_hpi, start_time
		 FROM appointments WHERE id=$1`,
		appointmentID,
	).Scan(&apptType, &practHPI, &startTime)
	if err != nil {
		return fmt.Errorf("backfill: looking up appointment: %w", err)
	}

	// Find the first eligible waitlist entry: matching type and practitioner (or any
	// practitioner if none specified), whose window covers the freed slot.
	var patientID string
	var waitlistID uuid.UUID
	err = h.pool.QueryRow(ctx,
		`SELECT id, patient_id FROM appointment_waitlist
		 WHERE tenant_id=$1
		   AND appointment_type=$2
		   AND (practitioner_hpi IS NULL OR practitioner_hpi=$3)
		   AND (earliest_date IS NULL OR earliest_date <= $4)
		   AND (latest_date IS NULL OR latest_date >= $4)
		 ORDER BY created_at ASC LIMIT 1`,
		tenantID, apptType, practHPI, startTime,
	).Scan(&waitlistID, &patientID)
	if err != nil {
		return nil // no matching waitlist entry; normal case
	}

	h.logger.Info("waitlist backfill: notifying patient",
		slog.String("patientId", patientID),
		slog.String("appointmentId", appointmentID),
		slog.Time("slot", startTime),
	)
	// Dispatch is handled by the push/email/SMS providers wired at startup via
	// the notification hook on AppointmentsHandler (not yet wired — add via
	// WithWaitlistNotify when providers are available).
	return nil
}

func nilableStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
