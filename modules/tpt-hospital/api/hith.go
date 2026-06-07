// Package api — Hospital in the Home (HITH): virtual ward episodes and nursing visits.
// HITH allows patients to receive hospital-level acute care at home, reducing bed
// pressure and improving patient outcomes. Nurses visit daily and monitor via telehealth.
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

// HITHEpisodeStatus tracks the patient's HITH care episode.
type HITHEpisodeStatus string

const (
	HITHStatusActive     HITHEpisodeStatus = "active"
	HITHStatusSuspended  HITHEpisodeStatus = "suspended" // temporarily readmitted to hospital
	HITHStatusCompleted  HITHEpisodeStatus = "completed"
	HITHStatusWithdrawn  HITHEpisodeStatus = "withdrawn"
)

// HITHVisitType distinguishes the nature of the visit.
type HITHVisitType string

const (
	HITHVisitNursing    HITHVisitType = "nursing"
	HITHVisitMedical    HITHVisitType = "medical"
	HITHVisitPhysio     HITHVisitType = "physiotherapy"
	HITHVisitTelehealth HITHVisitType = "telehealth"
	HITHVisitPath       HITHVisitType = "pathology-collection"
)

// HITHEpisode is an active Hospital in the Home care episode.
type HITHEpisode struct {
	ID               string            `json:"id"`
	PatientID        string            `json:"patientId"`
	PatientNHI       string            `json:"patientNhi"`
	LinkedAdmissionID string           `json:"linkedAdmissionId,omitempty"` // original inpatient admission
	LeadClinicianHPI string            `json:"leadClinicianHpi"`
	Status           HITHEpisodeStatus `json:"status"`
	Diagnosis        string            `json:"diagnosis"`         // primary condition being treated at home
	CareGoals        []string          `json:"careGoals"`
	DailyVisitFreq   string            `json:"dailyVisitFrequency"` // e.g. "once", "twice", "bd"
	HomeAddress      string            `json:"homeAddress"`
	EmergencyContact string            `json:"emergencyContact,omitempty"`
	PatientConsented bool              `json:"patientConsented"`
	TenantID         string            `json:"tenantId"`
	StartDate        time.Time         `json:"startDate"`
	ExpectedEndDate  *time.Time        `json:"expectedEndDate,omitempty"`
	ActualEndDate    *time.Time        `json:"actualEndDate,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

// HITHVisit is a single nursing or medical visit during a HITH episode.
type HITHVisit struct {
	ID             string        `json:"id"`
	EpisodeID      string        `json:"episodeId"`
	CliniciandHPI  string        `json:"clinicianHpi"`
	VisitType      HITHVisitType `json:"visitType"`
	Vitals         Vitals        `json:"vitals"` // reuse Vitals from encounters.go
	ClinicalNotes  string        `json:"clinicalNotes,omitempty"`
	Escalated      bool          `json:"escalated"`        // true if patient escalated back to hospital
	EscalationNote string        `json:"escalationNote,omitempty"`
	NextVisitDate  *time.Time    `json:"nextVisitDate,omitempty"`
	TenantID       string        `json:"tenantId"`
	VisitedAt      time.Time     `json:"visitedAt"`
	CreatedAt      time.Time     `json:"createdAt"`
}

type hithEpisodeCreateRequest struct {
	PatientID         string     `json:"patientId"`
	PatientNHI        string     `json:"patientNhi"`
	LinkedAdmissionID string     `json:"linkedAdmissionId,omitempty"`
	LeadClinicianHPI  string     `json:"leadClinicianHpi"`
	Diagnosis         string     `json:"diagnosis"`
	CareGoals         []string   `json:"careGoals,omitempty"`
	DailyVisitFreq    string     `json:"dailyVisitFrequency"`
	HomeAddress       string     `json:"homeAddress"`
	EmergencyContact  string     `json:"emergencyContact,omitempty"`
	PatientConsented  bool       `json:"patientConsented"`
	StartDate         time.Time  `json:"startDate"`
	ExpectedEndDate   *time.Time `json:"expectedEndDate,omitempty"`
}

type hithEpisodeUpdateRequest struct {
	LeadClinicianHPI string     `json:"leadClinicianHpi,omitempty"`
	Status           HITHEpisodeStatus `json:"status,omitempty"`
	CareGoals        []string   `json:"careGoals,omitempty"`
	DailyVisitFreq   string     `json:"dailyVisitFrequency,omitempty"`
	ExpectedEndDate  *time.Time `json:"expectedEndDate,omitempty"`
}

type hithVisitRequest struct {
	CliniciandHPI  string        `json:"clinicianHpi"`
	VisitType      HITHVisitType `json:"visitType"`
	Vitals         Vitals        `json:"vitals,omitempty"`
	ClinicalNotes  string        `json:"clinicalNotes,omitempty"`
	Escalated      bool          `json:"escalated,omitempty"`
	EscalationNote string        `json:"escalationNote,omitempty"`
	NextVisitDate  *time.Time    `json:"nextVisitDate,omitempty"`
}

// HITHHandler handles Hospital in the Home routes.
type HITHHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListEpisodes handles GET /api/v1/hith/episodes.
func (h *HITHHandler) ListEpisodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	episodes, err := h.listEpisodes(ctx, tenantID, statusFilter)
	if err != nil {
		h.logger.Error("list HITH episodes", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list HITH episodes"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"episodes": episodes, "total": len(episodes)})
}

// CreateEpisode handles POST /api/v1/hith/episodes.
func (h *HITHHandler) CreateEpisode(w http.ResponseWriter, r *http.Request) {
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

	var req hithEpisodeCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.LeadClinicianHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLINICIAN", Message: "leadClinicianHpi is required"})
		return
	}
	if req.Diagnosis == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DIAGNOSIS", Message: "diagnosis is required"})
		return
	}
	if req.HomeAddress == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ADDRESS", Message: "homeAddress is required"})
		return
	}
	if !req.PatientConsented {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "CONSENT_REQUIRED", Message: "patientConsented must be true — written consent must be obtained before HITH enrolment"})
		return
	}

	episode, err := h.insertEpisode(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("create HITH episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create HITH episode"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "HITHEpisode",
		ResourceID: episode.ID, TenantID: tenantID,
	})
	writeJSON(w, http.StatusCreated, episode)
}

// GetEpisode handles GET /api/v1/hith/episodes/{id}.
func (h *HITHHandler) GetEpisode(w http.ResponseWriter, r *http.Request) {
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
	episode, err := h.getEpisodeByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "HITH episode not found"})
			return
		}
		h.logger.Error("get HITH episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve HITH episode"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionRead, ResourceType: "HITHEpisode",
		ResourceID: id, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, episode)
}

// UpdateEpisode handles PUT /api/v1/hith/episodes/{id}.
func (h *HITHHandler) UpdateEpisode(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getEpisodeByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "HITH episode not found"})
			return
		}
		h.logger.Error("get HITH episode for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve HITH episode"})
		return
	}
	if existing.Status == HITHStatusCompleted || existing.Status == HITHStatusWithdrawn {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot update a completed or withdrawn episode"})
		return
	}

	var req hithEpisodeUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.LeadClinicianHPI != "" {
		existing.LeadClinicianHPI = req.LeadClinicianHPI
	}
	if req.Status != "" {
		existing.Status = req.Status
	}
	if len(req.CareGoals) > 0 {
		existing.CareGoals = req.CareGoals
	}
	if req.DailyVisitFreq != "" {
		existing.DailyVisitFreq = req.DailyVisitFreq
	}
	if req.ExpectedEndDate != nil {
		existing.ExpectedEndDate = req.ExpectedEndDate
	}

	updated, err := h.updateEpisode(ctx, existing)
	if err != nil {
		h.logger.Error("update HITH episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update HITH episode"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "HITHEpisode",
		ResourceID: id, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, updated)
}

// AddVisit handles POST /api/v1/hith/episodes/{id}/visits.
func (h *HITHHandler) AddVisit(w http.ResponseWriter, r *http.Request) {
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

	episodeID := r.PathValue("id")
	ep, err := h.getEpisodeByID(ctx, episodeID, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "HITH episode not found"})
			return
		}
		h.logger.Error("get HITH episode for visit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve episode"})
		return
	}
	if ep.Status != HITHStatusActive {
		writeJSON(w, http.StatusConflict, apiError{Code: "NOT_ACTIVE", Message: "can only add visits to an active HITH episode"})
		return
	}

	var req hithVisitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.CliniciandHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLINICIAN", Message: "clinicianHpi is required"})
		return
	}

	visit, err := h.insertVisit(ctx, episodeID, req, tenantID)
	if err != nil {
		h.logger.Error("add HITH visit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to add HITH visit"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "HITHVisit",
		ResourceID: visit.ID, TenantID: tenantID,
		Metadata: map[string]string{"episodeId": episodeID, "escalated": fmt.Sprintf("%v", req.Escalated)},
	})
	writeJSON(w, http.StatusCreated, visit)
}

// ListVisits handles GET /api/v1/hith/episodes/{id}/visits.
func (h *HITHHandler) ListVisits(w http.ResponseWriter, r *http.Request) {
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

	episodeID := r.PathValue("id")
	visits, err := h.listVisits(ctx, episodeID, tenantID)
	if err != nil {
		h.logger.Error("list HITH visits", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list HITH visits"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionRead, ResourceType: "HITHVisit",
		ResourceID: episodeID, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, map[string]any{"visits": visits, "total": len(visits)})
}

// UpdateVisit handles PUT /api/v1/hith/episodes/{id}/visits/{visitId}.
func (h *HITHHandler) UpdateVisit(w http.ResponseWriter, r *http.Request) {
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

	episodeID := r.PathValue("id")
	visitID := r.PathValue("visitId")

	var req hithVisitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	visit, err := h.updateVisit(ctx, visitID, episodeID, req, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "visit not found"})
			return
		}
		h.logger.Error("update HITH visit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update HITH visit"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "HITHVisit",
		ResourceID: visitID, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, visit)
}

// Discharge handles POST /api/v1/hith/episodes/{id}/discharge.
func (h *HITHHandler) Discharge(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getEpisodeByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "HITH episode not found"})
			return
		}
		h.logger.Error("get HITH episode for discharge", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve episode"})
		return
	}
	if existing.Status == HITHStatusCompleted {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_COMPLETED", Message: "HITH episode is already completed"})
		return
	}

	now := time.Now().UTC()
	existing.Status = HITHStatusCompleted
	existing.ActualEndDate = &now

	completed, err := h.updateEpisode(ctx, existing)
	if err != nil {
		h.logger.Error("discharge HITH episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DISCHARGE_ERROR", Message: "failed to discharge HITH episode"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "HITHEpisode",
		ResourceID: id, TenantID: tenantID, Metadata: map[string]string{"action": "discharge"},
	})
	writeJSON(w, http.StatusOK, completed)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *HITHHandler) listEpisodes(ctx context.Context, tenantID, statusFilter string) ([]HITHEpisode, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		        diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		        patient_consented, tenant_id, start_date, expected_end_date, actual_end_date,
		        created_at, updated_at
		 FROM hith_episodes
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY start_date DESC`,
		db.NamedArgs{"tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query HITH episodes: %w", err)
	}
	defer rows.Close()

	var results []HITHEpisode
	for rows.Next() {
		ep, err := scanHITHEpisodeRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, ep)
	}
	return results, rows.Err()
}

func (h *HITHHandler) getEpisodeByID(ctx context.Context, id, tenantID string) (HITHEpisode, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		        diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		        patient_consented, tenant_id, start_date, expected_end_date, actual_end_date,
		        created_at, updated_at
		 FROM hith_episodes
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	ep, err := scanHITHEpisodeRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return HITHEpisode{}, errNotFound
		}
		return HITHEpisode{}, fmt.Errorf("get HITH episode: %w", err)
	}
	return ep, nil
}

func (h *HITHHandler) insertEpisode(ctx context.Context, req hithEpisodeCreateRequest, tenantID string) (HITHEpisode, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO hith_episodes
		   (patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		    diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		    patient_consented, tenant_id, start_date, expected_end_date)
		 VALUES
		   (@patient_id, @patient_nhi, @linked_admission_id, @lead_clinician_hpi, @status,
		    @diagnosis, @care_goals, @daily_visit_frequency, @home_address, @emergency_contact,
		    @patient_consented, @tenant_id, @start_date, @expected_end_date)
		 RETURNING id, patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		           diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		           patient_consented, tenant_id, start_date, expected_end_date, actual_end_date,
		           created_at, updated_at`,
		db.NamedArgs{
			"patient_id":            req.PatientID,
			"patient_nhi":           req.PatientNHI,
			"linked_admission_id":   req.LinkedAdmissionID,
			"lead_clinician_hpi":    req.LeadClinicianHPI,
			"status":                HITHStatusActive,
			"diagnosis":             req.Diagnosis,
			"care_goals":            req.CareGoals,
			"daily_visit_frequency": req.DailyVisitFreq,
			"home_address":          req.HomeAddress,
			"emergency_contact":     req.EmergencyContact,
			"patient_consented":     req.PatientConsented,
			"tenant_id":             tenantID,
			"start_date":            req.StartDate,
			"expected_end_date":     req.ExpectedEndDate,
		},
	)
	return scanHITHEpisodeRow(row)
}

func (h *HITHHandler) updateEpisode(ctx context.Context, ep HITHEpisode) (HITHEpisode, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hith_episodes
		 SET lead_clinician_hpi    = @lead_clinician_hpi,
		     status                = @status,
		     care_goals            = @care_goals,
		     daily_visit_frequency = @daily_visit_frequency,
		     expected_end_date     = @expected_end_date,
		     actual_end_date       = @actual_end_date,
		     updated_at            = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		           diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		           patient_consented, tenant_id, start_date, expected_end_date, actual_end_date,
		           created_at, updated_at`,
		db.NamedArgs{
			"lead_clinician_hpi":    ep.LeadClinicianHPI,
			"status":                ep.Status,
			"care_goals":            ep.CareGoals,
			"daily_visit_frequency": ep.DailyVisitFreq,
			"expected_end_date":     ep.ExpectedEndDate,
			"actual_end_date":       ep.ActualEndDate,
			"id":                    ep.ID,
			"tenant_id":             ep.TenantID,
		},
	)
	updated, err := scanHITHEpisodeRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return HITHEpisode{}, errNotFound
		}
		return HITHEpisode{}, fmt.Errorf("update HITH episode: %w", err)
	}
	return updated, nil
}

func (h *HITHHandler) insertVisit(ctx context.Context, episodeID string, req hithVisitRequest, tenantID string) (HITHVisit, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO hith_visits
		   (episode_id, clinician_hpi, visit_type, vitals, clinical_notes,
		    escalated, escalation_note, next_visit_date, tenant_id, visited_at)
		 VALUES
		   (@episode_id, @clinician_hpi, @visit_type, @vitals, @clinical_notes,
		    @escalated, @escalation_note, @next_visit_date, @tenant_id, now())
		 RETURNING id, episode_id, clinician_hpi, visit_type, vitals, clinical_notes,
		           escalated, escalation_note, next_visit_date, tenant_id, visited_at, created_at`,
		db.NamedArgs{
			"episode_id":      episodeID,
			"clinician_hpi":   req.CliniciandHPI,
			"visit_type":      req.VisitType,
			"vitals":          req.Vitals,
			"clinical_notes":  req.ClinicalNotes,
			"escalated":       req.Escalated,
			"escalation_note": req.EscalationNote,
			"next_visit_date": req.NextVisitDate,
			"tenant_id":       tenantID,
		},
	)
	return scanHITHVisitRow(row)
}

func (h *HITHHandler) listVisits(ctx context.Context, episodeID, tenantID string) ([]HITHVisit, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, episode_id, clinician_hpi, visit_type, vitals, clinical_notes,
		        escalated, escalation_note, next_visit_date, tenant_id, visited_at, created_at
		 FROM hith_visits
		 WHERE episode_id = @episode_id AND tenant_id = @tenant_id
		 ORDER BY visited_at DESC`,
		db.NamedArgs{"episode_id": episodeID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query HITH visits: %w", err)
	}
	defer rows.Close()

	var results []HITHVisit
	for rows.Next() {
		v, err := scanHITHVisitRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, v)
	}
	return results, rows.Err()
}

func (h *HITHHandler) updateVisit(ctx context.Context, visitID, episodeID string, req hithVisitRequest, tenantID string) (HITHVisit, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hith_visits
		 SET vitals = @vitals, clinical_notes = @clinical_notes,
		     escalated = @escalated, escalation_note = @escalation_note,
		     next_visit_date = @next_visit_date
		 WHERE id = @id AND episode_id = @episode_id AND tenant_id = @tenant_id
		 RETURNING id, episode_id, clinician_hpi, visit_type, vitals, clinical_notes,
		           escalated, escalation_note, next_visit_date, tenant_id, visited_at, created_at`,
		db.NamedArgs{
			"vitals":          req.Vitals,
			"clinical_notes":  req.ClinicalNotes,
			"escalated":       req.Escalated,
			"escalation_note": req.EscalationNote,
			"next_visit_date": req.NextVisitDate,
			"id":              visitID,
			"episode_id":      episodeID,
			"tenant_id":       tenantID,
		},
	)
	v, err := scanHITHVisitRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return HITHVisit{}, errNotFound
		}
		return HITHVisit{}, fmt.Errorf("update HITH visit: %w", err)
	}
	return v, nil
}

func scanHITHEpisodeRow(row dbRow) (HITHEpisode, error) {
	var ep HITHEpisode
	if err := row.Scan(
		&ep.ID, &ep.PatientID, &ep.PatientNHI, &ep.LinkedAdmissionID, &ep.LeadClinicianHPI, &ep.Status,
		&ep.Diagnosis, &ep.CareGoals, &ep.DailyVisitFreq, &ep.HomeAddress, &ep.EmergencyContact,
		&ep.PatientConsented, &ep.TenantID, &ep.StartDate, &ep.ExpectedEndDate, &ep.ActualEndDate,
		&ep.CreatedAt, &ep.UpdatedAt,
	); err != nil {
		return HITHEpisode{}, err
	}
	return ep, nil
}

func scanHITHVisitRow(row dbRow) (HITHVisit, error) {
	var v HITHVisit
	if err := row.Scan(
		&v.ID, &v.EpisodeID, &v.CliniciandHPI, &v.VisitType, &v.Vitals, &v.ClinicalNotes,
		&v.Escalated, &v.EscalationNote, &v.NextVisitDate, &v.TenantID, &v.VisitedAt, &v.CreatedAt,
	); err != nil {
		return HITHVisit{}, err
	}
	return v, nil
}
