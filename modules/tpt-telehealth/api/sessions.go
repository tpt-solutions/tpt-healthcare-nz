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
	"github.com/PhillipC05/tpt-healthcare/core/video"
)

// SessionStatus enumerates the lifecycle states of a telehealth session.
type SessionStatus string

const (
	SessionStatusScheduled SessionStatus = "scheduled"
	SessionStatusActive    SessionStatus = "active"
	SessionStatusEnded     SessionStatus = "ended"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// Session is the domain model for a telehealth video consultation.
type Session struct {
	ID              string        `json:"id"`
	AppointmentID   string        `json:"appointmentId,omitempty"`
	PatientNHI      string        `json:"patientNhi"`
	PractitionerHPI string        `json:"practitionerHpi"`
	VideoProvider   string        `json:"videoProvider"`
	ExternalRoomID  string        `json:"externalRoomId"`
	HostURL         string        `json:"hostUrl,omitempty"`
	PatientURL      string        `json:"patientUrl,omitempty"`
	ScheduledAt     time.Time     `json:"scheduledAt"`
	DurationMins    int           `json:"durationMins"`
	Status          SessionStatus `json:"status"`
	RecordingURL    string        `json:"recordingUrl,omitempty"`
	EndedAt         *time.Time    `json:"endedAt,omitempty"`
	TenantID        string        `json:"tenantId"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

// sessionCreateRequest is the body for POST /api/v1/sessions.
type sessionCreateRequest struct {
	AppointmentID   string    `json:"appointmentId,omitempty"`
	PatientNHI      string    `json:"patientNhi"`
	PractitionerHPI string    `json:"practitionerHpi"`
	ScheduledAt     time.Time `json:"scheduledAt"`
	DurationMins    int       `json:"durationMins"`
	Recording       bool      `json:"recording"`
}

// joinRequest is the body for POST /api/v1/sessions/{id}/join.
type joinRequest struct {
	ParticipantName string `json:"participantName"`
	Role            string `json:"role"` // "host" or "guest"
}

// SessionsHandler handles all /api/v1/sessions routes.
type SessionsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	video      video.Provider
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/sessions.
// Query params: status, practitioner (HPI CPN), date (YYYY-MM-DD).
func (h *SessionsHandler) List(w http.ResponseWriter, r *http.Request) {
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
	practitionerFilter := q.Get("practitioner")
	dateFilter := q.Get("date")

	sessions, err := h.listSessions(ctx, tenantID, statusFilter, practitionerFilter, dateFilter)
	if err != nil {
		h.logger.Error("list sessions", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list sessions"})
		return
	}

	// Redact URLs from list responses — callers must use GET /{id} or /join.
	for i := range sessions {
		sessions[i].HostURL = ""
		sessions[i].PatientURL = ""
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "TelehealthSession",
		ResourceID:   "list",
		TenantID:     tenantID,
		Metadata:     map[string]string{"status": statusFilter, "practitioner": practitionerFilter},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// Create handles POST /api/v1/sessions.
// Validates the practitioner APC, creates the video room, and persists the session.
func (h *SessionsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req sessionCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if err := validateSessionCreate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

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

	duration := time.Duration(req.DurationMins) * time.Minute
	room, err := h.video.CreateRoom(ctx, video.RoomOptions{
		AppointmentID: req.AppointmentID,
		HostHPI:       req.PractitionerHPI,
		PatientNHI:    req.PatientNHI,
		MaxDuration:   duration,
		Recording:     req.Recording,
	})
	if err != nil {
		h.logger.Error("create video room", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "VIDEO_ERROR", Message: "failed to create video room"})
		return
	}

	encHostURL, err := h.enc.Encrypt(room.HostURL)
	if err != nil {
		h.logger.Error("encrypt host URL", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to secure session URLs"})
		return
	}
	encPatientURL, err := h.enc.Encrypt(room.PatientURL)
	if err != nil {
		h.logger.Error("encrypt patient URL", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to secure session URLs"})
		return
	}
	encNHI, err := h.enc.Encrypt(req.PatientNHI)
	if err != nil {
		h.logger.Error("encrypt NHI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to secure patient identifier"})
		return
	}

	session, err := h.insertSession(ctx, req, tenantID, room, encHostURL, encPatientURL, encNHI)
	if err != nil {
		h.logger.Error("insert session", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create session"})
		return
	}

	// Return plaintext URLs in the create response only — the host and patient
	// need them to join. After this point they are only accessible via /join.
	session.HostURL = room.HostURL
	session.PatientURL = room.PatientURL

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "TelehealthSession",
		ResourceID:   session.ID,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, session)
}

// Get handles GET /api/v1/sessions/{id}.
// Returns session metadata; URLs are omitted (use /join to obtain a fresh URL).
func (h *SessionsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "session ID is required"})
		return
	}

	session, err := h.getSessionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "session not found"})
			return
		}
		h.logger.Error("get session", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve session"})
		return
	}

	session.HostURL = ""
	session.PatientURL = ""

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "TelehealthSession",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, session)
}

// Join handles POST /api/v1/sessions/{id}/join.
// Returns a fresh provider join URL for the requesting participant.
func (h *SessionsHandler) Join(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "session ID is required"})
		return
	}

	var req joinRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ParticipantName == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "participantName is required"})
		return
	}
	role := video.RoleGuest
	if req.Role == string(video.RoleHost) {
		role = video.RoleHost
	}

	session, err := h.getSessionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "session not found"})
			return
		}
		h.logger.Error("get session for join", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve session"})
		return
	}

	if session.Status == SessionStatusEnded || session.Status == SessionStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "SESSION_CLOSED",
			Message: fmt.Sprintf("session is %s and cannot be joined", session.Status),
		})
		return
	}

	joinURL, err := h.video.GetJoinURL(ctx, session.ExternalRoomID, req.ParticipantName, role)
	if err != nil {
		h.logger.Error("get join URL", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "VIDEO_ERROR", Message: "failed to generate join URL"})
		return
	}

	// Mark as active on first join if still scheduled.
	if session.Status == SessionStatusScheduled {
		if err := h.setSessionStatus(ctx, id, tenantID, SessionStatusActive); err != nil {
			h.logger.Error("activate session", slog.Any("error", err))
		}
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "TelehealthSession",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "join", "role": string(role)},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]string{"joinUrl": joinURL})
}

// End handles POST /api/v1/sessions/{id}/end.
// Terminates the video room via the provider and records the end time.
func (h *SessionsHandler) End(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "session ID is required"})
		return
	}

	session, err := h.getSessionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "session not found"})
			return
		}
		h.logger.Error("get session for end", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve session"})
		return
	}

	if session.Status == SessionStatusEnded {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_ENDED", Message: "session is already ended"})
		return
	}
	if session.Status == SessionStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "SESSION_CANCELLED", Message: "session is cancelled"})
		return
	}

	if err := h.video.EndRoom(ctx, session.ExternalRoomID); err != nil {
		h.logger.Warn("end video room", slog.Any("error", err), slog.String("room", session.ExternalRoomID))
		// Non-fatal: provider room may have already expired; we still close the session.
	}

	recordingURL, err := h.video.GetRecordingURL(ctx, session.ExternalRoomID)
	if err != nil {
		h.logger.Warn("get recording URL", slog.Any("error", err))
	}

	now := time.Now().UTC()
	if err := h.closeSession(ctx, id, tenantID, recordingURL, now); err != nil {
		h.logger.Error("close session", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CLOSE_ERROR", Message: "failed to end session"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "TelehealthSession",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "end"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	w.WriteHeader(http.StatusNoContent)
}

// Recording handles GET /api/v1/sessions/{id}/recording.
func (h *SessionsHandler) Recording(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "session ID is required"})
		return
	}

	session, err := h.getSessionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "session not found"})
			return
		}
		h.logger.Error("get session for recording", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve session"})
		return
	}

	if session.RecordingURL == "" {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NO_RECORDING", Message: "no recording available for this session"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "TelehealthSession",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "recording_access"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]string{"recordingUrl": session.RecordingURL})
}

// validateSessionCreate checks required fields and temporal consistency.
func validateSessionCreate(req *sessionCreateRequest) error {
	if req.PatientNHI == "" {
		return fmt.Errorf("patientNhi is required")
	}
	if req.PractitionerHPI == "" {
		return fmt.Errorf("practitionerHpi is required")
	}
	if req.ScheduledAt.IsZero() {
		return fmt.Errorf("scheduledAt is required")
	}
	if req.DurationMins <= 0 {
		req.DurationMins = 30
	}
	if req.DurationMins > 480 {
		return fmt.Errorf("durationMins must not exceed 480 (8 hours)")
	}
	return nil
}

// listSessions queries sessions for the tenant with optional filters.
func (h *SessionsHandler) listSessions(
	ctx context.Context,
	tenantID, statusFilter, practitionerFilter, dateFilter string,
) ([]Session, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, appointment_id, patient_nhi, practitioner_hpi,
		        video_provider, external_room_id,
		        scheduled_at, duration_mins, status,
		        recording_url, ended_at, tenant_id, created_at, updated_at
		 FROM telehealth_sessions
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter      = '' OR status = @status_filter)
		   AND (@pract_filter       = '' OR practitioner_hpi = @pract_filter)
		   AND (@date_filter        = '' OR scheduled_at::date = @date_filter::date)
		 ORDER BY scheduled_at DESC
		 LIMIT 500`,
		db.NamedArgs{
			"tenant_id":    tenantID,
			"status_filter": statusFilter,
			"pract_filter":  practitionerFilter,
			"date_filter":   dateFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var results []Session
	for rows.Next() {
		s, err := h.scanSession(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// getSessionByID retrieves a single session with tenant isolation.
func (h *SessionsHandler) getSessionByID(ctx context.Context, id, tenantID string) (Session, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, appointment_id, patient_nhi, practitioner_hpi,
		        video_provider, external_room_id,
		        scheduled_at, duration_mins, status,
		        recording_url, ended_at, tenant_id, created_at, updated_at
		 FROM telehealth_sessions
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	s, err := h.scanSession(row.Scan)
	if err != nil {
		if db.IsNoRows(err) {
			return Session{}, errNotFound
		}
		return Session{}, fmt.Errorf("get session by id: %w", err)
	}
	return s, nil
}

// scanSession populates a Session from a row scan function.
func (h *SessionsHandler) scanSession(scan func(...any) error) (Session, error) {
	var s Session
	var encNHI, recURL string
	var endedAt *time.Time
	if err := scan(
		&s.ID, &s.AppointmentID, &encNHI, &s.PractitionerHPI,
		&s.VideoProvider, &s.ExternalRoomID,
		&s.ScheduledAt, &s.DurationMins, &s.Status,
		&recURL, &endedAt, &s.TenantID, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return Session{}, err
	}
	nhi, err := h.enc.Decrypt(encNHI)
	if err != nil {
		return Session{}, fmt.Errorf("decrypt nhi: %w", err)
	}
	s.PatientNHI = nhi
	s.RecordingURL = recURL
	s.EndedAt = endedAt
	return s, nil
}

// insertSession persists a new session with encrypted URLs.
func (h *SessionsHandler) insertSession(
	ctx context.Context,
	req sessionCreateRequest,
	tenantID string,
	room *video.Room,
	encHostURL, encPatientURL, encNHI string,
) (Session, error) {
	var s Session
	var encNHIOut, recURL string
	var endedAt *time.Time
	err := h.pool.QueryRow(ctx,
		`INSERT INTO telehealth_sessions
		   (appointment_id, patient_nhi, practitioner_hpi,
		    video_provider, external_room_id, host_url, patient_url,
		    scheduled_at, duration_mins, status, tenant_id)
		 VALUES
		   (@appointment_id, @patient_nhi, @practitioner_hpi,
		    @video_provider, @external_room_id, @host_url, @patient_url,
		    @scheduled_at, @duration_mins, @status, @tenant_id)
		 RETURNING id, appointment_id, patient_nhi, practitioner_hpi,
		           video_provider, external_room_id,
		           scheduled_at, duration_mins, status,
		           recording_url, ended_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"appointment_id":  req.AppointmentID,
			"patient_nhi":     encNHI,
			"practitioner_hpi": req.PractitionerHPI,
			"video_provider":  room.ExternalID[:0] + getProviderName(room),
			"external_room_id": room.ExternalID,
			"host_url":        encHostURL,
			"patient_url":     encPatientURL,
			"scheduled_at":    req.ScheduledAt,
			"duration_mins":   req.DurationMins,
			"status":          SessionStatusScheduled,
			"tenant_id":       tenantID,
		},
	).Scan(
		&s.ID, &s.AppointmentID, &encNHIOut, &s.PractitionerHPI,
		&s.VideoProvider, &s.ExternalRoomID,
		&s.ScheduledAt, &s.DurationMins, &s.Status,
		&recURL, &endedAt, &s.TenantID, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	nhi, err := h.enc.Decrypt(encNHIOut)
	if err != nil {
		return Session{}, fmt.Errorf("decrypt nhi after insert: %w", err)
	}
	s.PatientNHI = nhi
	s.RecordingURL = recURL
	s.EndedAt = endedAt
	return s, nil
}

// setSessionStatus transitions a session to a new status.
func (h *SessionsHandler) setSessionStatus(ctx context.Context, id, tenantID string, status SessionStatus) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE telehealth_sessions
		 SET status = @status, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"status": status, "id": id, "tenant_id": tenantID},
	)
	return err
}

// closeSession marks a session as ended with an optional recording URL.
func (h *SessionsHandler) closeSession(ctx context.Context, id, tenantID, recordingURL string, endedAt time.Time) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE telehealth_sessions
		 SET status = @status, recording_url = @recording_url,
		     ended_at = @ended_at, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{
			"status":        SessionStatusEnded,
			"recording_url": recordingURL,
			"ended_at":      endedAt,
			"id":            id,
			"tenant_id":     tenantID,
		},
	)
	return err
}

// getProviderName derives a human-readable provider name from the room's external ID prefix.
func getProviderName(room *video.Room) string {
	id := room.ExternalID
	switch {
	case len(id) > 4 && id[:4] == "tpt-":
		return "jitsi"
	default:
		return "unknown"
	}
}
