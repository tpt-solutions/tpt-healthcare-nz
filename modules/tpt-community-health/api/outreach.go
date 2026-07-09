package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// OutreachProgramme represents a community outreach programme.
type OutreachProgramme struct {
	ID            string `json:"id"`
	ProgrammeName string `json:"programmeName"`
	ProgrammeType string `json:"programmeType"`
	// mobile-clinic | health-promotion | screening | vaccination | wound-clinic | chronic-disease-support
	Description      string `json:"description"`
	TargetPopulation string `json:"targetPopulation"`
	Status           string `json:"status"`
	// active | paused | completed | discontinued
	CoordinatorHpi string     `json:"coordinatorHpi"`
	FundingSource  *string    `json:"fundingSource"`
	Notes          *string    `json:"notes"`
	TenantID       string     `json:"tenantId"`
	StartDate      time.Time  `json:"startDate"`
	EndDate        *time.Time `json:"endDate"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

const opSelectCols = `id, programme_name, programme_type, description, target_population,
       status, coordinator_hpi, funding_source, notes,
       tenant_id, start_date, end_date, created_at, updated_at`

func scanOutreachProgramme(row interface{ Scan(...any) error }, p *OutreachProgramme) error {
	return row.Scan(
		&p.ID, &p.ProgrammeName, &p.ProgrammeType, &p.Description, &p.TargetPopulation,
		&p.Status, &p.CoordinatorHpi, &p.FundingSource, &p.Notes,
		&p.TenantID, &p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt,
	)
}

type outreachProgrammeHandler struct{ handlerDeps }

func (h *outreachProgrammeHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+opSelectCols+` FROM community_outreach_programmes WHERE tenant_id = @tenant_id AND status = @status ORDER BY start_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+opSelectCols+` FROM community_outreach_programmes WHERE tenant_id = @tenant_id ORDER BY start_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	programmes := make([]OutreachProgramme, 0)
	for rows.Next() {
		var p OutreachProgramme
		if err := scanOutreachProgramme(rows, &p); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		programmes = append(programmes, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, programmes)
}

func (h *outreachProgrammeHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req OutreachProgramme
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.CoordinatorHpi) {
		return
	}
	var p OutreachProgramme
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO community_outreach_programmes
		    (programme_name, programme_type, description, target_population, status,
		     coordinator_hpi, funding_source, notes, tenant_id, start_date, end_date)
		VALUES
		    (@programme_name, @programme_type, @description, @target_population, 'active',
		     @coordinator_hpi, @funding_source, @notes, @tenant_id,
		     COALESCE(@start_date, CURRENT_DATE), @end_date)
		RETURNING `+opSelectCols,
		pgx.NamedArgs{
			"programme_name":    req.ProgrammeName,
			"programme_type":    req.ProgrammeType,
			"description":       req.Description,
			"target_population": req.TargetPopulation,
			"coordinator_hpi":   req.CoordinatorHpi,
			"funding_source":    req.FundingSource,
			"notes":             req.Notes,
			"tenant_id":         tenantID,
			"start_date":        req.StartDate,
			"end_date":          req.EndDate,
		}).Scan(
		&p.ID, &p.ProgrammeName, &p.ProgrammeType, &p.Description, &p.TargetPopulation,
		&p.Status, &p.CoordinatorHpi, &p.FundingSource, &p.Notes,
		&p.TenantID, &p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "OutreachProgramme", p.ID, "")
	writeJSON(w, http.StatusCreated, p)
}

func (h *outreachProgrammeHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var p OutreachProgramme
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+opSelectCols+` FROM community_outreach_programmes WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.ProgrammeName, &p.ProgrammeType, &p.Description, &p.TargetPopulation,
		&p.Status, &p.CoordinatorHpi, &p.FundingSource, &p.Notes,
		&p.TenantID, &p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "outreach programme not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *outreachProgrammeHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req OutreachProgramme
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.CoordinatorHpi) {
		return
	}
	var p OutreachProgramme
	err := h.pool.QueryRow(r.Context(), `
		UPDATE community_outreach_programmes SET
		    programme_name    = @programme_name,
		    programme_type    = @programme_type,
		    description       = @description,
		    target_population = @target_population,
		    status            = @status,
		    coordinator_hpi   = @coordinator_hpi,
		    funding_source    = @funding_source,
		    notes             = @notes,
		    start_date        = @start_date,
		    end_date          = @end_date,
		    updated_at        = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+opSelectCols,
		pgx.NamedArgs{
			"programme_name":    req.ProgrammeName,
			"programme_type":    req.ProgrammeType,
			"description":       req.Description,
			"target_population": req.TargetPopulation,
			"status":            req.Status,
			"coordinator_hpi":   req.CoordinatorHpi,
			"funding_source":    req.FundingSource,
			"notes":             req.Notes,
			"start_date":        req.StartDate,
			"end_date":          req.EndDate,
			"id":                id,
			"tenant_id":         tenantID,
		}).Scan(
		&p.ID, &p.ProgrammeName, &p.ProgrammeType, &p.Description, &p.TargetPopulation,
		&p.Status, &p.CoordinatorHpi, &p.FundingSource, &p.Notes,
		&p.TenantID, &p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "outreach programme not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "OutreachProgramme", p.ID, "")
	writeJSON(w, http.StatusOK, p)
}

// ---------------------------------------------------------------------------
// Outreach Event
// ---------------------------------------------------------------------------

// OutreachEvent represents a single outreach event within a programme.
type OutreachEvent struct {
	ID          string `json:"id"`
	ProgrammeID string `json:"programmeId"`
	EventName   string `json:"eventName"`
	EventType   string `json:"eventType"`
	// clinic | screening | education | vaccination | health-promotion
	Location        string `json:"location"`
	ClinicianHpis   string `json:"clinicianHpis"`
	TargetAttendees *int   `json:"targetAttendees"`
	ActualAttendees int    `json:"actualAttendees"`
	Status          string `json:"status"`
	// planned | confirmed | in-progress | completed | cancelled
	CancellationReason *string    `json:"cancellationReason"`
	Notes              *string    `json:"notes"`
	TenantID           string     `json:"tenantId"`
	ScheduledAt        time.Time  `json:"scheduledAt"`
	StartedAt          *time.Time `json:"startedAt"`
	CompletedAt        *time.Time `json:"completedAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

const oeSelectCols = `id, programme_id, event_name, event_type, location, clinician_hpis,
       target_attendees, actual_attendees, status, cancellation_reason, notes,
       tenant_id, scheduled_at, started_at, completed_at, created_at, updated_at`

func scanOutreachEvent(row interface{ Scan(...any) error }, e *OutreachEvent) error {
	return row.Scan(
		&e.ID, &e.ProgrammeID, &e.EventName, &e.EventType, &e.Location, &e.ClinicianHpis,
		&e.TargetAttendees, &e.ActualAttendees, &e.Status, &e.CancellationReason, &e.Notes,
		&e.TenantID, &e.ScheduledAt, &e.StartedAt, &e.CompletedAt, &e.CreatedAt, &e.UpdatedAt,
	)
}

type outreachEventHandler struct{ handlerDeps }

func (h *outreachEventHandler) ListForProgramme(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	programmeID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+oeSelectCols+` FROM community_outreach_events WHERE programme_id = @programme_id AND tenant_id = @tenant_id ORDER BY scheduled_at DESC`,
		pgx.NamedArgs{"programme_id": programmeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	events := make([]OutreachEvent, 0)
	for rows.Next() {
		var e OutreachEvent
		if err := scanOutreachEvent(rows, &e); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *outreachEventHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	programmeID := r.PathValue("id")
	var req OutreachEvent
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var e OutreachEvent
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO community_outreach_events
		    (programme_id, event_name, event_type, location, clinician_hpis,
		     target_attendees, actual_attendees, status, notes, tenant_id, scheduled_at)
		VALUES
		    (@programme_id, @event_name, @event_type, @location, @clinician_hpis,
		     @target_attendees, 0, 'planned', @notes, @tenant_id, COALESCE(@scheduled_at, now()))
		RETURNING `+oeSelectCols,
		pgx.NamedArgs{
			"programme_id":     programmeID,
			"event_name":       req.EventName,
			"event_type":       req.EventType,
			"location":         req.Location,
			"clinician_hpis":   req.ClinicianHpis,
			"target_attendees": req.TargetAttendees,
			"notes":            req.Notes,
			"tenant_id":        tenantID,
			"scheduled_at":     req.ScheduledAt,
		}).Scan(
		&e.ID, &e.ProgrammeID, &e.EventName, &e.EventType, &e.Location, &e.ClinicianHpis,
		&e.TargetAttendees, &e.ActualAttendees, &e.Status, &e.CancellationReason, &e.Notes,
		&e.TenantID, &e.ScheduledAt, &e.StartedAt, &e.CompletedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "OutreachEvent", e.ID, "")
	writeJSON(w, http.StatusCreated, e)
}

func (h *outreachEventHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var e OutreachEvent
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+oeSelectCols+` FROM community_outreach_events WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&e.ID, &e.ProgrammeID, &e.EventName, &e.EventType, &e.Location, &e.ClinicianHpis,
		&e.TargetAttendees, &e.ActualAttendees, &e.Status, &e.CancellationReason, &e.Notes,
		&e.TenantID, &e.ScheduledAt, &e.StartedAt, &e.CompletedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "outreach event not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (h *outreachEventHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req OutreachEvent
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var e OutreachEvent
	err := h.pool.QueryRow(r.Context(), `
		UPDATE community_outreach_events SET
		    event_name          = @event_name,
		    event_type          = @event_type,
		    location            = @location,
		    clinician_hpis      = @clinician_hpis,
		    target_attendees    = @target_attendees,
		    actual_attendees    = @actual_attendees,
		    status              = @status,
		    cancellation_reason = @cancellation_reason,
		    notes               = @notes,
		    scheduled_at        = @scheduled_at,
		    started_at          = COALESCE(started_at, CASE WHEN @status = 'in-progress' THEN now() END),
		    updated_at          = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+oeSelectCols,
		pgx.NamedArgs{
			"event_name":          req.EventName,
			"event_type":          req.EventType,
			"location":            req.Location,
			"clinician_hpis":      req.ClinicianHpis,
			"target_attendees":    req.TargetAttendees,
			"actual_attendees":    req.ActualAttendees,
			"status":              req.Status,
			"cancellation_reason": req.CancellationReason,
			"notes":               req.Notes,
			"scheduled_at":        req.ScheduledAt,
			"id":                  id,
			"tenant_id":           tenantID,
		}).Scan(
		&e.ID, &e.ProgrammeID, &e.EventName, &e.EventType, &e.Location, &e.ClinicianHpis,
		&e.TargetAttendees, &e.ActualAttendees, &e.Status, &e.CancellationReason, &e.Notes,
		&e.TenantID, &e.ScheduledAt, &e.StartedAt, &e.CompletedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "outreach event not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "OutreachEvent", e.ID, "")
	writeJSON(w, http.StatusOK, e)
}

func (h *outreachEventHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE community_outreach_events
		SET status = 'completed', completed_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('completed', 'cancelled')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "outreach event not found or already completed"})
		return
	}
	h.recordAudit(r, "complete", "OutreachEvent", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// ---------------------------------------------------------------------------
// Outreach Encounter
// ---------------------------------------------------------------------------

// OutreachEncounter records a patient (or community member) contact at an outreach event.
type OutreachEncounter struct {
	ID           string `json:"id"`
	EventID      string `json:"eventId"`
	PatientNHI   string `json:"patientNhi"`
	ClinicianHpi string `json:"clinicianHpi"`
	AttendeeType string `json:"attendeeType"`
	// patient | carer | community-member
	ServicesProvided string  `json:"servicesProvided"`
	ScreeningType    *string `json:"screeningType"`
	// blood-pressure | diabetes | cervical | bowel | hearing | vision
	ScreeningResult *string `json:"screeningResult"`
	ReferralType    *string `json:"referralType"`
	// gp | specialist | mental-health | social-services | housing
	ReferralReason   *string   `json:"referralReason"`
	FollowUpRequired bool      `json:"followUpRequired"`
	FollowUpDetails  *string   `json:"followUpDetails"`
	ConsentGiven     bool      `json:"consentGiven"`
	Notes            *string   `json:"notes"`
	TenantID         string    `json:"tenantId"`
	EncounteredAt    time.Time `json:"encounteredAt"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

const encSelectCols = `id, event_id, patient_nhi, clinician_hpi, attendee_type,
       services_provided, screening_type, screening_result,
       referral_type, referral_reason,
       follow_up_required, follow_up_details, consent_given, notes,
       tenant_id, encountered_at, created_at, updated_at`

func scanOutreachEncounter(row interface{ Scan(...any) error }, e *OutreachEncounter) error {
	return row.Scan(
		&e.ID, &e.EventID, &e.PatientNHI, &e.ClinicianHpi, &e.AttendeeType,
		&e.ServicesProvided, &e.ScreeningType, &e.ScreeningResult,
		&e.ReferralType, &e.ReferralReason,
		&e.FollowUpRequired, &e.FollowUpDetails, &e.ConsentGiven, &e.Notes,
		&e.TenantID, &e.EncounteredAt, &e.CreatedAt, &e.UpdatedAt,
	)
}

type outreachEncounterHandler struct{ handlerDeps }

func (h *outreachEncounterHandler) ListForEvent(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	eventID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+encSelectCols+` FROM community_outreach_encounters WHERE event_id = @event_id AND tenant_id = @tenant_id ORDER BY encountered_at DESC`,
		pgx.NamedArgs{"event_id": eventID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	encounters := make([]OutreachEncounter, 0)
	for rows.Next() {
		var e OutreachEncounter
		if err := scanOutreachEncounter(rows, &e); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, _ := h.decryptNHI(e.PatientNHI)
		e.PatientNHI = nhi
		encounters = append(encounters, e)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, encounters)
}

func (h *outreachEncounterHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	eventID := r.PathValue("id")
	var req OutreachEncounter
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AttendeeType == "" {
		req.AttendeeType = "patient"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var e OutreachEncounter
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO community_outreach_encounters
		    (event_id, patient_nhi, clinician_hpi, attendee_type,
		     services_provided, screening_type, screening_result,
		     referral_type, referral_reason,
		     follow_up_required, follow_up_details, consent_given, notes,
		     tenant_id, encountered_at)
		VALUES
		    (@event_id, @patient_nhi, @clinician_hpi, @attendee_type,
		     @services_provided, @screening_type, @screening_result,
		     @referral_type, @referral_reason,
		     @follow_up_required, @follow_up_details, @consent_given, @notes,
		     @tenant_id, COALESCE(@encountered_at, now()))
		RETURNING `+encSelectCols,
		pgx.NamedArgs{
			"event_id":           eventID,
			"patient_nhi":        nhiEnc,
			"clinician_hpi":      req.ClinicianHpi,
			"attendee_type":      req.AttendeeType,
			"services_provided":  req.ServicesProvided,
			"screening_type":     req.ScreeningType,
			"screening_result":   req.ScreeningResult,
			"referral_type":      req.ReferralType,
			"referral_reason":    req.ReferralReason,
			"follow_up_required": req.FollowUpRequired,
			"follow_up_details":  req.FollowUpDetails,
			"consent_given":      req.ConsentGiven,
			"notes":              req.Notes,
			"tenant_id":          tenantID,
			"encountered_at":     req.EncounteredAt,
		}).Scan(
		&e.ID, &e.EventID, &e.PatientNHI, &e.ClinicianHpi, &e.AttendeeType,
		&e.ServicesProvided, &e.ScreeningType, &e.ScreeningResult,
		&e.ReferralType, &e.ReferralReason,
		&e.FollowUpRequired, &e.FollowUpDetails, &e.ConsentGiven, &e.Notes,
		&e.TenantID, &e.EncounteredAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	// Increment actual_attendees count on the event
	_, _ = h.pool.Exec(r.Context(),
		`UPDATE community_outreach_events SET actual_attendees = actual_attendees + 1 WHERE id = @id`,
		pgx.NamedArgs{"id": eventID})
	h.recordAudit(r, "create", "OutreachEncounter", e.ID, e.PatientNHI)
	nhi, _ := h.decryptNHI(e.PatientNHI)
	e.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, e)
}

func (h *outreachEncounterHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var e OutreachEncounter
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+encSelectCols+` FROM community_outreach_encounters WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&e.ID, &e.EventID, &e.PatientNHI, &e.ClinicianHpi, &e.AttendeeType,
		&e.ServicesProvided, &e.ScreeningType, &e.ScreeningResult,
		&e.ReferralType, &e.ReferralReason,
		&e.FollowUpRequired, &e.FollowUpDetails, &e.ConsentGiven, &e.Notes,
		&e.TenantID, &e.EncounteredAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "encounter not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, _ := h.decryptNHI(e.PatientNHI)
	e.PatientNHI = nhi
	writeJSON(w, http.StatusOK, e)
}
