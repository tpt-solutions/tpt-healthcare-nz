package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/email"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/video"
)

// AppointmentStatus enumerates the valid states for an appointment.
type AppointmentStatus string

const (
	AppointmentStatusProposed  AppointmentStatus = "proposed"
	AppointmentStatusPending   AppointmentStatus = "pending"
	AppointmentStatusBooked    AppointmentStatus = "booked"
	AppointmentStatusArrived   AppointmentStatus = "arrived"
	AppointmentStatusFulfilled AppointmentStatus = "fulfilled"
	AppointmentStatusCancelled AppointmentStatus = "cancelled"
	AppointmentStatusNoShow    AppointmentStatus = "noshow"
)

// Appointment is the domain model for a scheduled appointment.
// Aligns with the FHIR R5 Appointment resource.
type Appointment struct {
	ID              string            `json:"id"`
	PatientNHI      string            `json:"patientNhi"`
	PatientID       string            `json:"patientId"`
	PractitionerHPI string            `json:"practitionerHpi"`
	StartTime       time.Time         `json:"startTime"`
	EndTime         time.Time         `json:"endTime"`
	AppointmentType string            `json:"appointmentType"`
	Status          AppointmentStatus `json:"status"`
	Reason          string            `json:"reason,omitempty"`
	Notes           string            `json:"notes,omitempty"`
	TenantID        string            `json:"tenantId"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

// appointmentCreateRequest is the body for POST /api/v1/appointments.
type appointmentCreateRequest struct {
	PatientNHI      string    `json:"patientNhi"`
	PatientID       string    `json:"patientId"`
	PractitionerHPI string    `json:"practitionerHpi"`
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	AppointmentType string    `json:"appointmentType"`
	Reason          string    `json:"reason,omitempty"`
	Notes           string    `json:"notes,omitempty"`
}

// appointmentUpdateRequest is the body for PUT /api/v1/appointments/{id}.
type appointmentUpdateRequest struct {
	StartTime       *time.Time         `json:"startTime,omitempty"`
	EndTime         *time.Time         `json:"endTime,omitempty"`
	PractitionerHPI string             `json:"practitionerHpi,omitempty"`
	Status          *AppointmentStatus `json:"status,omitempty"`
	Reason          string             `json:"reason,omitempty"`
	Notes           string             `json:"notes,omitempty"`
}

// AppointmentsHandler handles all /api/v1/appointments routes.
type AppointmentsHandler struct {
	pool          db.Pool
	enc           *encryption.Cipher
	hpiClient     *hpi.Client
	auditTrail    *audit.Trail
	emailProvider email.Provider
	fromEmail     string
	videoProvider video.Provider
	logger        *slog.Logger
}

// WithEmail attaches an email provider for sending appointment confirmation emails
// to patients. fromEmail is the "From:" address for outbound messages.
func (h *AppointmentsHandler) WithEmail(provider email.Provider, fromEmail string) *AppointmentsHandler {
	h.emailProvider = provider
	h.fromEmail = fromEmail
	return h
}

// WithVideo attaches a video provider for creating telehealth rooms when
// an appointment of type "telehealth" is confirmed.
func (h *AppointmentsHandler) WithVideo(provider video.Provider) *AppointmentsHandler {
	h.videoProvider = provider
	return h
}

// List handles GET /api/v1/appointments.
// Supported query parameters: date (YYYY-MM-DD), provider (HPI CPN), status, patient (internal ID).
func (h *AppointmentsHandler) List(w http.ResponseWriter, r *http.Request) {
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
	dateFilter := q.Get("date")
	providerFilter := q.Get("provider")
	statusFilter := q.Get("status")
	patientFilter := q.Get("patient")

	appointments, err := h.listAppointments(ctx, tenantID.String(), dateFilter, providerFilter, statusFilter, patientFilter)
	if err != nil {
		h.logger.Error("list appointments", slog.Any("error", err), slog.String("tenant", tenantID.String()))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list appointments"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Appointment",
		ResourceID:   "list",
		TenantID:     tenantID,
		Details: map[string]any{
			"date":     dateFilter,
			"provider": providerFilter,
			"status":   statusFilter,
			"patient":  patientFilter,
		},
		OccurredAt: time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"appointments": appointments,
		"total":        len(appointments),
	})
}

// Create handles POST /api/v1/appointments.
// Validates the practitioner's APC via the HPI client before booking.
func (h *AppointmentsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req appointmentCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if err := validateAppointmentCreate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	// Validate the practitioner has a current APC before booking.
	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.PractitionerHPI)
	if err != nil {
		h.logger.Error("HPI APC validation", slog.Any("error", err), slog.String("hpi", req.PractitionerHPI))
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

	appt, err := h.insertAppointment(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert appointment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create appointment"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "Appointment",
		ResourceID:   appt.ID,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	// Create a video room for telehealth appointments when a provider is wired.
	if req.AppointmentType == "telehealth" && h.videoProvider != nil {
		go func() {
			room, roomErr := h.videoProvider.CreateRoom(context.Background(), video.RoomOptions{
				AppointmentID: appt.ID,
				HostHPI:       req.PractitionerHPI,
				PatientNHI:    req.PatientNHI,
				MaxDuration:   60 * time.Minute,
			})
			if roomErr != nil {
				h.logger.WarnContext(ctx, "telehealth room creation failed",
					slog.String("apptID", appt.ID),
					slog.String("error", roomErr.Error()),
				)
				return
			}
			// Persist the room URLs into the appointment notes field.
			updatedNotes := fmt.Sprintf("Telehealth room: host=%s patient=%s", room.HostURL, room.PatientURL)
			_, _ = h.pool.Exec(context.Background(),
				`UPDATE appointments SET notes = @notes, updated_at = now() WHERE id = @id`,
				db.NamedArgs{"notes": updatedNotes, "id": appt.ID},
			)
		}()
	}

	// Send confirmation email to the patient when an email provider is wired.
	if appt.PatientID != "" && h.emailProvider != nil {
		go func() {
			patientEmail, lookupErr := h.fetchPatientEmail(context.Background(), appt.PatientID)
			if lookupErr != nil {
				return // patient has no email on record — silently skip
			}
			local := appt.StartTime.In(nzLocationAppt())
			subject := fmt.Sprintf("Appointment confirmed — %s", local.Format("Monday 2 January 2006 at 3:04 PM"))
			body := fmt.Sprintf(
				"Your appointment has been confirmed.\n\n"+
					"Date: %s\n"+
					"Type: %s\n"+
					"Reason: %s\n\n"+
					"If you need to cancel or reschedule, please contact the practice.",
				local.Format("Monday 2 January 2006 at 3:04 PM"),
				appt.AppointmentType,
				appt.Reason,
			)
			if _, emailErr := h.emailProvider.Send(context.Background(), email.Message{
				To:       []string{patientEmail},
				From:     h.fromEmail,
				Subject:  subject,
				TextBody: body,
				Tags:     []string{"appointment-confirmation"},
			}); emailErr != nil {
				h.logger.WarnContext(ctx, "appointment confirmation email failed",
					slog.String("apptID", appt.ID),
					slog.String("error", emailErr.Error()),
				)
			}
		}()
	}

	writeJSON(w, http.StatusCreated, appt)
}

// Get handles GET /api/v1/appointments/{id}.
func (h *AppointmentsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "appointment ID is required"})
		return
	}

	appt, err := h.getAppointmentByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "appointment not found"})
			return
		}
		h.logger.Error("get appointment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve appointment"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Appointment",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, appt)
}

// Update handles PUT /api/v1/appointments/{id}.
// Supports rescheduling (new start/end times) and status transitions.
// Re-validates practitioner APC if the practitioner is being changed.
func (h *AppointmentsHandler) Update(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "appointment ID is required"})
		return
	}

	var req appointmentUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getAppointmentByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "appointment not found"})
			return
		}
		h.logger.Error("get appointment for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve appointment"})
		return
	}

	if existing.Status == AppointmentStatusCancelled || existing.Status == AppointmentStatusFulfilled {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "TERMINAL_STATUS",
			Message: fmt.Sprintf("cannot update appointment in %s status", existing.Status),
		})
		return
	}

	// If the practitioner is being reassigned, re-validate APC.
	if req.PractitionerHPI != "" && req.PractitionerHPI != existing.PractitionerHPI {
		apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.PractitionerHPI)
		if err != nil {
			h.logger.Error("HPI APC validation on update", slog.Any("error", err))
			writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate practitioner APC"})
			return
		}
		if !apcStatus.Valid {
			writeJSON(w, http.StatusUnprocessableEntity, apiError{
				Code:    "INVALID_APC",
				Message: "replacement practitioner does not have a current Annual Practising Certificate",
				Details: apcStatus,
			})
			return
		}
		existing.PractitionerHPI = req.PractitionerHPI
	}

	if req.StartTime != nil {
		existing.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		existing.EndTime = *req.EndTime
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.Reason != "" {
		existing.Reason = req.Reason
	}
	if req.Notes != "" {
		existing.Notes = req.Notes
	}

	updated, err := h.updateAppointment(ctx, existing)
	if err != nil {
		h.logger.Error("update appointment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update appointment"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "Appointment",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/appointments/{id}.
// Cancels the appointment (soft delete — status set to "cancelled").
func (h *AppointmentsHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "appointment ID is required"})
		return
	}

	existing, err := h.getAppointmentByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "appointment not found"})
			return
		}
		h.logger.Error("get appointment for cancel", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve appointment"})
		return
	}

	if existing.Status == AppointmentStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_CANCELLED", Message: "appointment is already cancelled"})
		return
	}

	existing.Status = AppointmentStatusCancelled
	if _, err := h.updateAppointment(ctx, existing); err != nil {
		h.logger.Error("cancel appointment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CANCEL_ERROR", Message: "failed to cancel appointment"})
		return
	}

	if err := h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "delete",
		ResourceType: "Appointment",
		ResourceID:   id,
		TenantID:     tenantID,
		Details:      map[string]any{"action": "cancel"},
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	w.WriteHeader(http.StatusNoContent)
}

// fetchPatientEmail decrypts the patient's FHIR resource and returns the first
// email address found in Telecom, or an error if none exists.
func (h *AppointmentsHandler) fetchPatientEmail(ctx context.Context, patientID string) (string, error) {
	var fhirEnc []byte
	if err := h.pool.QueryRow(ctx,
		`SELECT fhir_resource FROM patients WHERE id = $1`,
		patientID,
	).Scan(&fhirEnc); err != nil {
		return "", fmt.Errorf("patient email lookup: %w", err)
	}
	plain, err := h.enc.Decrypt(fhirEnc)
	if err != nil {
		return "", fmt.Errorf("patient email decrypt: %w", err)
	}
	var p r5.Patient
	if err := json.Unmarshal(plain, &p); err != nil {
		return "", fmt.Errorf("patient email unmarshal: %w", err)
	}
	for _, cp := range p.Telecom {
		if cp.System == "email" && cp.Value != "" {
			return cp.Value, nil
		}
	}
	return "", fmt.Errorf("patient %s has no email address on record", patientID)
}

// nzLocationAppt returns the New Zealand/Auckland timezone for formatting
// appointment times. Falls back to UTC if the timezone data is unavailable.
func nzLocationAppt() *time.Location {
	loc, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		return time.UTC
	}
	return loc
}

// validateAppointmentCreate checks required fields and temporal consistency.
func validateAppointmentCreate(req *appointmentCreateRequest) error {
	if req.PatientID == "" && req.PatientNHI == "" {
		return fmt.Errorf("either patientId or patientNhi is required")
	}
	if req.PractitionerHPI == "" {
		return fmt.Errorf("practitionerHpi is required")
	}
	if req.AppointmentType == "" {
		return fmt.Errorf("appointmentType is required")
	}
	if req.StartTime.IsZero() {
		return fmt.Errorf("startTime is required")
	}
	if req.EndTime.IsZero() {
		return fmt.Errorf("endTime is required")
	}
	if !req.EndTime.After(req.StartTime) {
		return fmt.Errorf("endTime must be after startTime")
	}
	return nil
}

// listAppointments queries appointments for the tenant with optional filters.
func (h *AppointmentsHandler) listAppointments(
	ctx context.Context,
	tenantID, dateFilter, providerFilter, statusFilter, patientFilter string,
) ([]Appointment, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_nhi, patient_id, practitioner_hpi,
		        start_time, end_time, appointment_type, status, reason, notes,
		        tenant_id, created_at, updated_at
		 FROM appointments
		 WHERE tenant_id = @tenant_id
		   AND (@date_filter     = '' OR start_time::date = @date_filter::date)
		   AND (@provider_filter = '' OR practitioner_hpi = @provider_filter)
		   AND (@status_filter   = '' OR status = @status_filter)
		   AND (@patient_filter  = '' OR patient_id = @patient_filter)
		 ORDER BY start_time ASC
		 LIMIT 500`,
		db.NamedArgs{
			"tenant_id":       tenantID,
			"date_filter":     dateFilter,
			"provider_filter": providerFilter,
			"status_filter":   statusFilter,
			"patient_filter":  patientFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query appointments: %w", err)
	}
	defer rows.Close()

	var results []Appointment
	for rows.Next() {
		var a Appointment
		if err := rows.Scan(
			&a.ID, &a.PatientNHI, &a.PatientID, &a.PractitionerHPI,
			&a.StartTime, &a.EndTime, &a.AppointmentType, &a.Status,
			&a.Reason, &a.Notes, &a.TenantID, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan appointment: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// getAppointmentByID retrieves a single appointment with tenant isolation.
func (h *AppointmentsHandler) getAppointmentByID(ctx context.Context, id, tenantID string) (Appointment, error) {
	var a Appointment
	err := h.pool.QueryRow(ctx,
		`SELECT id, patient_nhi, patient_id, practitioner_hpi,
		        start_time, end_time, appointment_type, status, reason, notes,
		        tenant_id, created_at, updated_at
		 FROM appointments
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&a.ID, &a.PatientNHI, &a.PatientID, &a.PractitionerHPI,
		&a.StartTime, &a.EndTime, &a.AppointmentType, &a.Status,
		&a.Reason, &a.Notes, &a.TenantID, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Appointment{}, errNotFound
		}
		return Appointment{}, fmt.Errorf("get appointment by id: %w", err)
	}
	return a, nil
}

// insertAppointment persists a new appointment record.
func (h *AppointmentsHandler) insertAppointment(ctx context.Context, req appointmentCreateRequest, tenantID string) (Appointment, error) {
	var a Appointment
	err := h.pool.QueryRow(ctx,
		`INSERT INTO appointments
		   (patient_nhi, patient_id, practitioner_hpi, start_time, end_time,
		    appointment_type, status, reason, notes, tenant_id)
		 VALUES
		   (@patient_nhi, @patient_id, @practitioner_hpi, @start_time, @end_time,
		    @appointment_type, @status, @reason, @notes, @tenant_id)
		 RETURNING id, patient_nhi, patient_id, practitioner_hpi,
		           start_time, end_time, appointment_type, status, reason, notes,
		           tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"patient_nhi":      req.PatientNHI,
			"patient_id":       req.PatientID,
			"practitioner_hpi": req.PractitionerHPI,
			"start_time":       req.StartTime,
			"end_time":         req.EndTime,
			"appointment_type": req.AppointmentType,
			"status":           AppointmentStatusBooked,
			"reason":           req.Reason,
			"notes":            req.Notes,
			"tenant_id":        tenantID,
		},
	).Scan(
		&a.ID, &a.PatientNHI, &a.PatientID, &a.PractitionerHPI,
		&a.StartTime, &a.EndTime, &a.AppointmentType, &a.Status,
		&a.Reason, &a.Notes, &a.TenantID, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return Appointment{}, fmt.Errorf("insert appointment: %w", err)
	}
	return a, nil
}

// updateAppointment writes the mutated appointment back to the database.
func (h *AppointmentsHandler) updateAppointment(ctx context.Context, a Appointment) (Appointment, error) {
	var updated Appointment
	err := h.pool.QueryRow(ctx,
		`UPDATE appointments
		 SET practitioner_hpi = @practitioner_hpi,
		     start_time       = @start_time,
		     end_time         = @end_time,
		     status           = @status,
		     reason           = @reason,
		     notes            = @notes,
		     updated_at       = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_nhi, patient_id, practitioner_hpi,
		           start_time, end_time, appointment_type, status, reason, notes,
		           tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"practitioner_hpi": a.PractitionerHPI,
			"start_time":       a.StartTime,
			"end_time":         a.EndTime,
			"status":           a.Status,
			"reason":           a.Reason,
			"notes":            a.Notes,
			"id":               a.ID,
			"tenant_id":        a.TenantID,
		},
	).Scan(
		&updated.ID, &updated.PatientNHI, &updated.PatientID, &updated.PractitionerHPI,
		&updated.StartTime, &updated.EndTime, &updated.AppointmentType, &updated.Status,
		&updated.Reason, &updated.Notes, &updated.TenantID, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Appointment{}, errNotFound
		}
		return Appointment{}, fmt.Errorf("update appointment: %w", err)
	}
	return updated, nil
}
