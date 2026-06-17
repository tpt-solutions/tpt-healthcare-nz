package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EpisodesHandler handles /api/v1/episodes and /api/v1/episodes/{id}/ward-rounds routes.
type EpisodesHandler struct {
	pool       *pgxpool.Pool
	enc        *encryption.Encryptor
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ---------------------------------------------------------------------------
// Episode handlers
// ---------------------------------------------------------------------------

// List handles GET /api/v1/episodes.
// Query params: patient (internal ID), status, type.
func (h *EpisodesHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	patientFilter := q.Get("patient")
	statusFilter := q.Get("status")
	typeFilter := q.Get("type")

	records, err := h.listEpisodes(ctx, tenantID, patientFilter, statusFilter, typeFilter)
	if err != nil {
		h.logger.Error("list episodes", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list episodes"})
		return
	}

	responses := make([]Episode, 0, len(records))
	for _, rec := range records {
		ep, err := h.decryptEpisode(rec)
		if err != nil {
			h.logger.Error("decrypt episode", slog.Any("error", err), slog.String("id", rec.ID))
			continue
		}
		if accessErr := checkMHAccess(ctx, h.pool, tenantID, ep.PatientNHI, principal); accessErr != nil {
			continue
		}
		responses = append(responses, ep)
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MentalHealthEpisode",
		ResourceID:   "list",
	})

	writeJSON(w, http.StatusOK, map[string]any{"episodes": responses, "total": len(responses)})
}

// Create handles POST /api/v1/episodes.
func (h *EpisodesHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req episodeCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patientId or patientNhi is required"})
		return
	}
	if req.ResponsibleHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_RESPONSIBLE_HPI", Message: "responsibleHpi is required"})
		return
	}
	if !validEpisodeType(req.EpisodeType) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_EPISODE_TYPE", Message: fmt.Sprintf("unknown episode type %q", req.EpisodeType)})
		return
	}

	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.ResponsibleHPI)
	if err != nil {
		h.logger.Error("HPI APC check for RC", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not verify responsible clinician APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusForbidden, apiError{Code: "INVALID_APC", Message: "responsible clinician does not hold a current APC"})
		return
	}

	rec, err := h.insertEpisode(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create episode"})
		return
	}

	ep, err := h.decryptEpisode(rec)
	if err != nil {
		h.logger.Error("decrypt episode after insert", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt episode"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "MentalHealthEpisode",
		ResourceID:   rec.ID,
		PatientNHI:   req.PatientNHI,
		Details:      map[string]any{"type": string(req.EpisodeType)},
	})

	writeJSON(w, http.StatusCreated, ep)
}

// Get handles GET /api/v1/episodes/{id}.
func (h *EpisodesHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "episode ID is required"})
		return
	}

	rec, err := h.getEpisodeByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found"})
			return
		}
		h.logger.Error("get episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve episode"})
		return
	}

	ep, err := h.decryptEpisode(rec)
	if err != nil {
		h.logger.Error("decrypt episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt episode"})
		return
	}

	if accessErr := checkMHAccess(ctx, h.pool, tenantID, ep.PatientNHI, principal); accessErr != nil {
		writeJSON(w, http.StatusForbidden, apiError{Code: "ACCESS_DENIED", Message: accessErr.Error()})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MentalHealthEpisode",
		ResourceID:   id,
		PatientNHI:   ep.PatientNHI,
	})

	writeJSON(w, http.StatusOK, ep)
}

// Update handles PUT /api/v1/episodes/{id}.
func (h *EpisodesHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "episode ID is required"})
		return
	}

	var req episodeUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getEpisodeByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found"})
			return
		}
		h.logger.Error("get episode for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve episode"})
		return
	}

	if existing.Status == string(EpisodeCompleted) || existing.Status == string(EpisodeDeceased) {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot update a completed or deceased episode"})
		return
	}

	if req.ResponsibleHPI != "" {
		existing.ResponsibleHPI = req.ResponsibleHPI
	}
	if req.Status != "" {
		existing.Status = string(req.Status)
	}
	if req.PrimaryDiagnosis != "" {
		existing.PrimaryDiagnosis = req.PrimaryDiagnosis
	}
	if len(req.SecondaryDiagnoses) > 0 {
		existing.SecondaryDiagnoses = req.SecondaryDiagnoses
	}
	if req.WardOrTeam != "" {
		existing.WardOrTeam = req.WardOrTeam
	}
	if req.BedNumber != "" {
		existing.BedNumber = req.BedNumber
	}

	updated, err := h.updateEpisode(ctx, existing, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found"})
			return
		}
		h.logger.Error("update episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update episode"})
		return
	}

	ep, err := h.decryptEpisode(updated)
	if err != nil {
		h.logger.Error("decrypt episode after update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt episode"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "MentalHealthEpisode",
		ResourceID:   id,
	})

	writeJSON(w, http.StatusOK, ep)
}

// Discharge handles POST /api/v1/episodes/{id}/discharge.
// Records the discharge time and encrypted summary, then closes the episode.
func (h *EpisodesHandler) Discharge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "episode ID is required"})
		return
	}

	var req dischargeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.DischargedAt.IsZero() {
		req.DischargedAt = time.Now().UTC()
	}
	if req.Status == "" {
		req.Status = EpisodeCompleted
	}

	existing, err := h.getEpisodeByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found"})
			return
		}
		h.logger.Error("get episode for discharge", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve episode"})
		return
	}

	if existing.Status == string(EpisodeCompleted) {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_DISCHARGED", Message: "episode is already discharged"})
		return
	}

	var summaryEnc []byte
	if req.DischargeSummary != "" {
		var err error
		summaryEnc, err = h.enc.Encrypt([]byte(req.DischargeSummary))
		if err != nil {
			h.logger.Error("encrypt discharge summary", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt discharge summary"})
			return
		}
	}

	updated, err := h.dischargeEpisode(ctx, id, req.DischargedAt, string(req.Status), summaryEnc, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found"})
			return
		}
		h.logger.Error("discharge episode", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DISCHARGE_ERROR", Message: "failed to discharge episode"})
		return
	}

	ep, err := h.decryptEpisode(updated)
	if err != nil {
		h.logger.Error("decrypt episode after discharge", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt episode"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "MentalHealthEpisode",
		ResourceID:   id,
		Details:      map[string]any{"action": "discharge", "status": string(req.Status)},
	})

	writeJSON(w, http.StatusOK, ep)
}

// ---------------------------------------------------------------------------
// Ward round handlers
// ---------------------------------------------------------------------------

// ListWardRounds handles GET /api/v1/episodes/{id}/ward-rounds.
func (h *EpisodesHandler) ListWardRounds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	episodeID := r.PathValue("id")
	if episodeID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "episode ID is required"})
		return
	}

	// Verify the episode exists and the caller has access.
	epRec, err := h.getEpisodeByID(ctx, episodeID, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found"})
			return
		}
		h.logger.Error("get episode for ward rounds", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve episode"})
		return
	}
	if accessErr := checkMHAccess(ctx, h.pool, tenantID, epRec.PatientNHI, principal); accessErr != nil {
		writeJSON(w, http.StatusForbidden, apiError{Code: "ACCESS_DENIED", Message: accessErr.Error()})
		return
	}

	rounds, err := h.listWardRounds(ctx, episodeID, tenantID)
	if err != nil {
		h.logger.Error("list ward rounds", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list ward rounds"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MentalHealthWardRound",
		ResourceID:   "list:" + episodeID,
		PatientNHI:   epRec.PatientNHI,
	})

	writeJSON(w, http.StatusOK, map[string]any{"wardRounds": rounds, "total": len(rounds)})
}

// CreateWardRound handles POST /api/v1/episodes/{id}/ward-rounds.
func (h *EpisodesHandler) CreateWardRound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	episodeID := r.PathValue("id")
	if episodeID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "episode ID is required"})
		return
	}

	var req wardRoundCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ClinicianHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLINICIAN", Message: "clinicianHpi is required"})
		return
	}
	if req.RiskLevel == "" {
		req.RiskLevel = RiskLow
	}

	epRec, err := h.getEpisodeByID(ctx, episodeID, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found"})
			return
		}
		h.logger.Error("get episode for ward round create", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve episode"})
		return
	}

	if epRec.Status != string(EpisodeActive) && epRec.Status != string(EpisodeOnHold) {
		writeJSON(w, http.StatusConflict, apiError{Code: "EPISODE_NOT_ACTIVE", Message: "ward rounds can only be added to active or on-hold episodes"})
		return
	}

	round, err := h.insertWardRound(ctx, episodeID, epRec, req, tenantID)
	if err != nil {
		h.logger.Error("insert ward round", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create ward round"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "MentalHealthWardRound",
		ResourceID:   round.ID,
		PatientNHI:   epRec.PatientNHI,
		Details:      map[string]any{"episodeId": episodeID, "riskLevel": string(req.RiskLevel)},
	})

	writeJSON(w, http.StatusCreated, round)
}

// GetWardRound handles GET /api/v1/episodes/{id}/ward-rounds/{roundId}.
func (h *EpisodesHandler) GetWardRound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	episodeID := r.PathValue("id")
	roundID := r.PathValue("roundId")
	if episodeID == "" || roundID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "episode ID and round ID are required"})
		return
	}

	round, err := h.getWardRoundByID(ctx, roundID, episodeID, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ward round not found"})
			return
		}
		h.logger.Error("get ward round", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve ward round"})
		return
	}

	if accessErr := checkMHAccess(ctx, h.pool, tenantID, round.PatientNHI, principal); accessErr != nil {
		writeJSON(w, http.StatusForbidden, apiError{Code: "ACCESS_DENIED", Message: accessErr.Error()})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MentalHealthWardRound",
		ResourceID:   roundID,
		PatientNHI:   round.PatientNHI,
	})

	writeJSON(w, http.StatusOK, round)
}
