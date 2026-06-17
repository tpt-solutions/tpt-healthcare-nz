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
	episodes, err := h.listEpisodes(ctx, tenantID.String(), statusFilter)
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

	episode, err := h.insertEpisode(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("create HITH episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create HITH episode"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "HITHEpisode",
		ResourceID: episode.ID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
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
	episode, err := h.getEpisodeByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "HITH episode not found"})
			return
		}
		h.logger.Error("get HITH episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve HITH episode"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "HITHEpisode",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
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
	existing, err := h.getEpisodeByID(ctx, id, tenantID.String())
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

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "HITHEpisode",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
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
	ep, err := h.getEpisodeByID(ctx, episodeID, tenantID.String())
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

	visit, err := h.insertVisit(ctx, episodeID, req, tenantID.String())
	if err != nil {
		h.logger.Error("add HITH visit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to add HITH visit"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "HITHVisit",
		ResourceID: visit.ID, TenantID: tenantID,
		Details:    map[string]any{"episode_id": episodeID, "escalated": req.Escalated},
		OccurredAt: time.Now().UTC(),
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
	visits, err := h.listVisits(ctx, episodeID, tenantID.String())
	if err != nil {
		h.logger.Error("list HITH visits", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list HITH visits"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "HITHVisit",
		ResourceID: episodeID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
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

	visit, err := h.updateVisit(ctx, visitID, episodeID, req, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "visit not found"})
			return
		}
		h.logger.Error("update HITH visit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update HITH visit"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "HITHVisit",
		ResourceID: visitID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
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
	existing, err := h.getEpisodeByID(ctx, id, tenantID.String())
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

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "HITHEpisode",
		ResourceID: id, TenantID: tenantID,
		Details:    map[string]any{"action": "discharge"},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, completed)
}
