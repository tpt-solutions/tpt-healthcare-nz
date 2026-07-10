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

// TheatreBookingStatus tracks a surgical booking through its lifecycle.
type TheatreBookingStatus string

const (
	TheatreStatusPlanned    TheatreBookingStatus = "planned"
	TheatreStatusConfirmed  TheatreBookingStatus = "confirmed"
	TheatreStatusInProgress TheatreBookingStatus = "in-progress"
	TheatreStatusCompleted  TheatreBookingStatus = "completed"
	TheatreStatusCancelled  TheatreBookingStatus = "cancelled"
	TheatreStatusPostponed  TheatreBookingStatus = "postponed"
)

// AnaesthesiaType enumerates the planned anaesthetic technique.
type AnaesthesiaType string

const (
	AnaesthesiaGA       AnaesthesiaType = "general"
	AnaesthesiaRegional AnaesthesiaType = "regional"
	AnaesthesiaSpinal   AnaesthesiaType = "spinal"
	AnaesthesiaEpidural AnaesthesiaType = "epidural"
	AnaesthesiaLocal    AnaesthesiaType = "local"
	AnaesthesiaMonitor  AnaesthesiaType = "monitored-sedation"
)

// TheatreBooking represents a surgical theatre booking, aligned to FHIR R5 Appointment.
type TheatreBooking struct {
	ID                 string               `json:"id"`
	PatientID          string               `json:"patientId"`
	PatientNHI         string               `json:"patientNhi"`
	AdmissionID        string               `json:"admissionId,omitempty"` // set when booked against an admission
	SurgeonHPI         string               `json:"surgeonHpi"`
	AssistantHPI       string               `json:"assistantHpi,omitempty"`
	AnaesthetistHPI    string               `json:"anaesthetistHpi,omitempty"`
	ScrubNurseHPI      string               `json:"scrubNurseHpi,omitempty"`
	TheatreID          string               `json:"theatreId"` // maps to a ward with type=theatre
	Status             TheatreBookingStatus `json:"status"`
	ProcedureName      string               `json:"procedureName"`
	ProcedureCodes     []string             `json:"procedureCodes"` // ACHI codes
	AnaesthesiaType    AnaesthesiaType      `json:"anaesthesiaType"`
	PlannedDurationMin int                  `json:"plannedDurationMins"`
	ActualDurationMin  *int                 `json:"actualDurationMins,omitempty"`
	ScheduledAt        time.Time            `json:"scheduledAt"`
	StartedAt          *time.Time           `json:"startedAt,omitempty"`
	CompletedAt        *time.Time           `json:"completedAt,omitempty"`
	OperativeNotes     string               `json:"operativeNotes,omitempty"`
	PostOpNotes        string               `json:"postOpNotes,omitempty"`
	Complications      []string             `json:"complications,omitempty"` // free-text or SNOMED
	TenantID           string               `json:"tenantId"`
	CreatedAt          time.Time            `json:"createdAt"`
	UpdatedAt          time.Time            `json:"updatedAt"`
}

type theatreCreateRequest struct {
	PatientID          string          `json:"patientId"`
	PatientNHI         string          `json:"patientNhi"`
	AdmissionID        string          `json:"admissionId,omitempty"`
	SurgeonHPI         string          `json:"surgeonHpi"`
	AssistantHPI       string          `json:"assistantHpi,omitempty"`
	AnaesthetistHPI    string          `json:"anaesthetistHpi,omitempty"`
	TheatreID          string          `json:"theatreId"`
	ProcedureName      string          `json:"procedureName"`
	ProcedureCodes     []string        `json:"procedureCodes,omitempty"`
	AnaesthesiaType    AnaesthesiaType `json:"anaesthesiaType"`
	PlannedDurationMin int             `json:"plannedDurationMins"`
	ScheduledAt        time.Time       `json:"scheduledAt"`
}

type theatreUpdateRequest struct {
	SurgeonHPI         string          `json:"surgeonHpi,omitempty"`
	AnaesthetistHPI    string          `json:"anaesthetistHpi,omitempty"`
	ScrubNurseHPI      string          `json:"scrubNurseHpi,omitempty"`
	TheatreID          string          `json:"theatreId,omitempty"`
	ProcedureName      string          `json:"procedureName,omitempty"`
	ProcedureCodes     []string        `json:"procedureCodes,omitempty"`
	AnaesthesiaType    AnaesthesiaType `json:"anaesthesiaType,omitempty"`
	PlannedDurationMin int             `json:"plannedDurationMins,omitempty"`
	ScheduledAt        *time.Time      `json:"scheduledAt,omitempty"`
}

type theatreCompleteRequest struct {
	OperativeNotes    string   `json:"operativeNotes,omitempty"`
	PostOpNotes       string   `json:"postOpNotes,omitempty"`
	Complications     []string `json:"complications,omitempty"`
	ActualDurationMin int      `json:"actualDurationMins,omitempty"`
}

// TheatreHandler handles all /api/v1/theatre routes.
type TheatreHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/theatre/bookings.
func (h *TheatreHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	surgeonFilter := r.URL.Query().Get("surgeon")
	theatreFilter := r.URL.Query().Get("theatre")

	bookings, err := h.listBookings(ctx, tenantID.String(), statusFilter, surgeonFilter, theatreFilter)
	if err != nil {
		h.logger.Error("list theatre bookings", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list theatre bookings"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bookings": bookings, "total": len(bookings)})
}

// Create handles POST /api/v1/theatre/bookings.
func (h *TheatreHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req theatreCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.SurgeonHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_SURGEON", Message: "surgeonHpi is required"})
		return
	}
	if req.TheatreID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_THEATRE", Message: "theatreId is required"})
		return
	}
	if req.ProcedureName == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PROCEDURE", Message: "procedureName is required"})
		return
	}

	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.SurgeonHPI)
	if err != nil {
		h.logger.Error("HPI APC validation for theatre", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate surgeon APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "INVALID_APC", Message: "surgeon does not hold a current Annual Practising Certificate", Details: apcStatus})
		return
	}

	booking, err := h.insertBooking(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert theatre booking", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create theatre booking"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "TheatreBooking",
		ResourceID: booking.ID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, booking)
}

// Get handles GET /api/v1/theatre/bookings/{id}.
func (h *TheatreHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	booking, err := h.getBookingByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "theatre booking not found"})
			return
		}
		h.logger.Error("get theatre booking", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve theatre booking"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "TheatreBooking",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, booking)
}

// Update handles PUT /api/v1/theatre/bookings/{id}.
func (h *TheatreHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getBookingByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "theatre booking not found"})
			return
		}
		h.logger.Error("get theatre booking for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve theatre booking"})
		return
	}
	if existing.Status == TheatreStatusCompleted || existing.Status == TheatreStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: fmt.Sprintf("cannot update booking in %s status", existing.Status)})
		return
	}

	var req theatreUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.SurgeonHPI != "" {
		existing.SurgeonHPI = req.SurgeonHPI
	}
	if req.AnaesthetistHPI != "" {
		existing.AnaesthetistHPI = req.AnaesthetistHPI
	}
	if req.ScrubNurseHPI != "" {
		existing.ScrubNurseHPI = req.ScrubNurseHPI
	}
	if req.TheatreID != "" {
		existing.TheatreID = req.TheatreID
	}
	if req.ProcedureName != "" {
		existing.ProcedureName = req.ProcedureName
	}
	if len(req.ProcedureCodes) > 0 {
		existing.ProcedureCodes = req.ProcedureCodes
	}
	if req.AnaesthesiaType != "" {
		existing.AnaesthesiaType = req.AnaesthesiaType
	}
	if req.PlannedDurationMin > 0 {
		existing.PlannedDurationMin = req.PlannedDurationMin
	}
	if req.ScheduledAt != nil {
		existing.ScheduledAt = *req.ScheduledAt
	}

	updated, err := h.updateBooking(ctx, existing)
	if err != nil {
		h.logger.Error("update theatre booking", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update theatre booking"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "TheatreBooking",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// Confirm handles POST /api/v1/theatre/bookings/{id}/confirm.
func (h *TheatreHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	h.transitionStatus(w, r, TheatreStatusPlanned, TheatreStatusConfirmed, "confirm")
}

// Cancel handles POST /api/v1/theatre/bookings/{id}/cancel.
func (h *TheatreHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	h.transitionStatus(w, r, "", TheatreStatusCancelled, "cancel")
}

// Start handles POST /api/v1/theatre/bookings/{id}/start.
func (h *TheatreHandler) Start(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getBookingByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "theatre booking not found"})
			return
		}
		h.logger.Error("get theatre booking for start", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve theatre booking"})
		return
	}
	if existing.Status != TheatreStatusConfirmed && existing.Status != TheatreStatusPlanned {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "can only start a planned or confirmed booking"})
		return
	}

	now := time.Now().UTC()
	existing.Status = TheatreStatusInProgress
	existing.StartedAt = &now

	updated, err := h.updateBooking(ctx, existing)
	if err != nil {
		h.logger.Error("start theatre booking", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "START_ERROR", Message: "failed to start theatre booking"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "TheatreBooking",
		ResourceID: id, TenantID: tenantID, Details: map[string]any{"action": "start"},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// Complete handles POST /api/v1/theatre/bookings/{id}/complete.
func (h *TheatreHandler) Complete(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getBookingByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "theatre booking not found"})
			return
		}
		h.logger.Error("get theatre booking for complete", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve theatre booking"})
		return
	}
	if existing.Status != TheatreStatusInProgress {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "can only complete an in-progress booking"})
		return
	}

	var req theatreCompleteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	now := time.Now().UTC()
	existing.Status = TheatreStatusCompleted
	existing.CompletedAt = &now
	existing.OperativeNotes = req.OperativeNotes
	existing.PostOpNotes = req.PostOpNotes
	existing.Complications = req.Complications
	if req.ActualDurationMin > 0 {
		existing.ActualDurationMin = &req.ActualDurationMin
	}

	completed, err := h.updateBooking(ctx, existing)
	if err != nil {
		h.logger.Error("complete theatre booking", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "COMPLETE_ERROR", Message: "failed to complete theatre booking"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "TheatreBooking",
		ResourceID: id, TenantID: tenantID, Details: map[string]any{"action": "complete"},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, completed)
}

// DaySchedule handles GET /api/v1/theatre/schedule — bookings for a given date.
func (h *TheatreHandler) DaySchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	date := r.URL.Query().Get("date") // YYYY-MM-DD; defaults to today
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	theatreFilter := r.URL.Query().Get("theatre")

	bookings, err := h.listByDate(ctx, tenantID.String(), date, theatreFilter)
	if err != nil {
		h.logger.Error("theatre day schedule", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCHEDULE_ERROR", Message: "failed to retrieve theatre schedule"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "bookings": bookings, "total": len(bookings)})
}

// transitionStatus is a helper for simple status transitions.
func (h *TheatreHandler) transitionStatus(w http.ResponseWriter, r *http.Request, fromStatus TheatreBookingStatus, toStatus TheatreBookingStatus, action string) {
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
	existing, err := h.getBookingByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "theatre booking not found"})
			return
		}
		h.logger.Error("get theatre booking for "+action, slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve theatre booking"})
		return
	}
	if fromStatus != "" && existing.Status != fromStatus {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: fmt.Sprintf("booking must be in %s status to %s", fromStatus, action)})
		return
	}
	if existing.Status == TheatreStatusCompleted || existing.Status == TheatreStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot modify a completed or cancelled booking"})
		return
	}

	existing.Status = toStatus
	updated, err := h.updateBooking(ctx, existing)
	if err != nil {
		h.logger.Error(action+" theatre booking", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to " + action + " booking"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "TheatreBooking",
		ResourceID: id, TenantID: tenantID, Details: map[string]any{"action": action},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *TheatreHandler) listBookings(ctx context.Context, tenantID, statusFilter, surgeonFilter, theatreFilter string) ([]TheatreBooking, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, admission_id, surgeon_hpi, assistant_hpi,
		        anaesthetist_hpi, scrub_nurse_hpi, theatre_id, status, procedure_name,
		        procedure_codes, anaesthesia_type, planned_duration_mins, actual_duration_mins,
		        scheduled_at, started_at, completed_at, operative_notes, post_op_notes, complications,
		        tenant_id, created_at, updated_at
		 FROM theatre_bookings
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter  = '' OR status = @status_filter)
		   AND (@surgeon_filter = '' OR surgeon_hpi = @surgeon_filter)
		   AND (@theatre_filter = '' OR theatre_id = @theatre_filter)
		 ORDER BY scheduled_at ASC`,
		db.NamedArgs{
			"tenant_id":      tenantID,
			"status_filter":  statusFilter,
			"surgeon_filter": surgeonFilter,
			"theatre_filter": theatreFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query theatre bookings: %w", err)
	}
	defer rows.Close()

	var results []TheatreBooking
	for rows.Next() {
		b, err := scanTheatreRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, b)
	}
	return results, rows.Err()
}

func (h *TheatreHandler) listByDate(ctx context.Context, tenantID, date, theatreFilter string) ([]TheatreBooking, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, admission_id, surgeon_hpi, assistant_hpi,
		        anaesthetist_hpi, scrub_nurse_hpi, theatre_id, status, procedure_name,
		        procedure_codes, anaesthesia_type, planned_duration_mins, actual_duration_mins,
		        scheduled_at, started_at, completed_at, operative_notes, post_op_notes, complications,
		        tenant_id, created_at, updated_at
		 FROM theatre_bookings
		 WHERE tenant_id = @tenant_id
		   AND scheduled_at::date = @date::date
		   AND (@theatre_filter = '' OR theatre_id = @theatre_filter)
		   AND status != 'cancelled'
		 ORDER BY scheduled_at ASC`,
		db.NamedArgs{"tenant_id": tenantID, "date": date, "theatre_filter": theatreFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query theatre day schedule: %w", err)
	}
	defer rows.Close()

	var results []TheatreBooking
	for rows.Next() {
		b, err := scanTheatreRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, b)
	}
	return results, rows.Err()
}

func (h *TheatreHandler) getBookingByID(ctx context.Context, id, tenantID string) (TheatreBooking, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, admission_id, surgeon_hpi, assistant_hpi,
		        anaesthetist_hpi, scrub_nurse_hpi, theatre_id, status, procedure_name,
		        procedure_codes, anaesthesia_type, planned_duration_mins, actual_duration_mins,
		        scheduled_at, started_at, completed_at, operative_notes, post_op_notes, complications,
		        tenant_id, created_at, updated_at
		 FROM theatre_bookings
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	b, err := scanTheatreRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return TheatreBooking{}, errNotFound
		}
		return TheatreBooking{}, fmt.Errorf("get theatre booking: %w", err)
	}
	return b, nil
}

func (h *TheatreHandler) insertBooking(ctx context.Context, req theatreCreateRequest, tenantID string) (TheatreBooking, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO theatre_bookings
		   (patient_id, patient_nhi, admission_id, surgeon_hpi, assistant_hpi, anaesthetist_hpi,
		    theatre_id, status, procedure_name, procedure_codes, anaesthesia_type,
		    planned_duration_mins, scheduled_at, tenant_id)
		 VALUES
		   (@patient_id, @patient_nhi, @admission_id, @surgeon_hpi, @assistant_hpi, @anaesthetist_hpi,
		    @theatre_id, @status, @procedure_name, @procedure_codes, @anaesthesia_type,
		    @planned_duration_mins, @scheduled_at, @tenant_id)
		 RETURNING id, patient_id, patient_nhi, admission_id, surgeon_hpi, assistant_hpi,
		           anaesthetist_hpi, scrub_nurse_hpi, theatre_id, status, procedure_name,
		           procedure_codes, anaesthesia_type, planned_duration_mins, actual_duration_mins,
		           scheduled_at, started_at, completed_at, operative_notes, post_op_notes, complications,
		           tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":            req.PatientID,
			"patient_nhi":           req.PatientNHI,
			"admission_id":          req.AdmissionID,
			"surgeon_hpi":           req.SurgeonHPI,
			"assistant_hpi":         req.AssistantHPI,
			"anaesthetist_hpi":      req.AnaesthetistHPI,
			"theatre_id":            req.TheatreID,
			"status":                TheatreStatusPlanned,
			"procedure_name":        req.ProcedureName,
			"procedure_codes":       req.ProcedureCodes,
			"anaesthesia_type":      req.AnaesthesiaType,
			"planned_duration_mins": req.PlannedDurationMin,
			"scheduled_at":          req.ScheduledAt,
			"tenant_id":             tenantID,
		},
	)
	return scanTheatreRow(row)
}

func (h *TheatreHandler) updateBooking(ctx context.Context, b TheatreBooking) (TheatreBooking, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE theatre_bookings
		 SET surgeon_hpi           = @surgeon_hpi,
		     assistant_hpi         = @assistant_hpi,
		     anaesthetist_hpi      = @anaesthetist_hpi,
		     scrub_nurse_hpi       = @scrub_nurse_hpi,
		     theatre_id            = @theatre_id,
		     status                = @status,
		     procedure_name        = @procedure_name,
		     procedure_codes       = @procedure_codes,
		     anaesthesia_type      = @anaesthesia_type,
		     planned_duration_mins = @planned_duration_mins,
		     actual_duration_mins  = @actual_duration_mins,
		     scheduled_at          = @scheduled_at,
		     started_at            = @started_at,
		     completed_at          = @completed_at,
		     operative_notes       = @operative_notes,
		     post_op_notes         = @post_op_notes,
		     complications         = @complications,
		     updated_at            = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, admission_id, surgeon_hpi, assistant_hpi,
		           anaesthetist_hpi, scrub_nurse_hpi, theatre_id, status, procedure_name,
		           procedure_codes, anaesthesia_type, planned_duration_mins, actual_duration_mins,
		           scheduled_at, started_at, completed_at, operative_notes, post_op_notes, complications,
		           tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"surgeon_hpi":           b.SurgeonHPI,
			"assistant_hpi":         b.AssistantHPI,
			"anaesthetist_hpi":      b.AnaesthetistHPI,
			"scrub_nurse_hpi":       b.ScrubNurseHPI,
			"theatre_id":            b.TheatreID,
			"status":                b.Status,
			"procedure_name":        b.ProcedureName,
			"procedure_codes":       b.ProcedureCodes,
			"anaesthesia_type":      b.AnaesthesiaType,
			"planned_duration_mins": b.PlannedDurationMin,
			"actual_duration_mins":  b.ActualDurationMin,
			"scheduled_at":          b.ScheduledAt,
			"started_at":            b.StartedAt,
			"completed_at":          b.CompletedAt,
			"operative_notes":       b.OperativeNotes,
			"post_op_notes":         b.PostOpNotes,
			"complications":         b.Complications,
			"id":                    b.ID,
			"tenant_id":             b.TenantID,
		},
	)
	updated, err := scanTheatreRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return TheatreBooking{}, errNotFound
		}
		return TheatreBooking{}, fmt.Errorf("update theatre booking: %w", err)
	}
	return updated, nil
}

func scanTheatreRow(row dbRow) (TheatreBooking, error) {
	var b TheatreBooking
	if err := row.Scan(
		&b.ID, &b.PatientID, &b.PatientNHI, &b.AdmissionID,
		&b.SurgeonHPI, &b.AssistantHPI, &b.AnaesthetistHPI, &b.ScrubNurseHPI,
		&b.TheatreID, &b.Status, &b.ProcedureName,
		&b.ProcedureCodes, &b.AnaesthesiaType, &b.PlannedDurationMin, &b.ActualDurationMin,
		&b.ScheduledAt, &b.StartedAt, &b.CompletedAt,
		&b.OperativeNotes, &b.PostOpNotes, &b.Complications,
		&b.TenantID, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return TheatreBooking{}, err
	}
	return b, nil
}
