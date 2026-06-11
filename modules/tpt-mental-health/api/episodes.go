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
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EpisodeType classifies the setting of a mental health episode.
type EpisodeType string

const (
	EpisodeInpatient    EpisodeType = "inpatient"
	EpisodeCommunity    EpisodeType = "community"
	EpisodeCrisis       EpisodeType = "crisis"
	EpisodeDayProgramme EpisodeType = "day-programme"
)

// EpisodeStatus tracks the lifecycle of an episode.
type EpisodeStatus string

const (
	EpisodeActive      EpisodeStatus = "active"
	EpisodeOnHold      EpisodeStatus = "on-hold"
	EpisodeCompleted   EpisodeStatus = "completed"
	EpisodeTransferred EpisodeStatus = "transferred"
	EpisodeDeceased    EpisodeStatus = "deceased"
)

// RiskLevel rates the patient's current risk of harm.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskVeryHigh RiskLevel = "very-high"
)

// Episode represents a mental health episode of care.
type Episode struct {
	ID                 string        `json:"id"`
	PatientID          string        `json:"patientId"`
	PatientNHI         string        `json:"patientNhi"`
	TenantID           string        `json:"tenantId"`
	ResponsibleHPI     string        `json:"responsibleHpi"`
	EpisodeType        EpisodeType   `json:"episodeType"`
	Status             EpisodeStatus `json:"status"`
	AdmissionReason    string        `json:"admissionReason,omitempty"` // decrypted
	PrimaryDiagnosis   string        `json:"primaryDiagnosis"`
	SecondaryDiagnoses []string      `json:"secondaryDiagnoses"`
	WardOrTeam         string        `json:"wardOrTeam"`
	BedNumber          string        `json:"bedNumber,omitempty"`
	AdmittedAt         *time.Time    `json:"admittedAt,omitempty"`
	DischargedAt       *time.Time    `json:"dischargedAt,omitempty"`
	ExtraSensitive     bool          `json:"extraSensitive"`
	CreatedAt          time.Time     `json:"createdAt"`
	UpdatedAt          time.Time     `json:"updatedAt"`
}

// WardRound represents a clinical contact entry within an inpatient episode.
type WardRound struct {
	ID             string            `json:"id"`
	EpisodeID      string            `json:"episodeId"`
	PatientID      string            `json:"patientId"`
	PatientNHI     string            `json:"patientNhi"`
	TenantID       string            `json:"tenantId"`
	ClinicianHPI   string            `json:"clinicianHpi"`
	Notes          string            `json:"notes,omitempty"` // decrypted
	MentalState    map[string]any    `json:"mentalState"`
	RiskLevel      RiskLevel         `json:"riskLevel"`
	Plans          string            `json:"plans,omitempty"` // decrypted
	ExtraSensitive bool              `json:"extraSensitive"`
	OccurredAt     time.Time         `json:"occurredAt"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

// episodeCreateRequest is the body for POST /api/v1/episodes.
type episodeCreateRequest struct {
	PatientID          string      `json:"patientId"`
	PatientNHI         string      `json:"patientNhi"`
	ResponsibleHPI     string      `json:"responsibleHpi"`
	EpisodeType        EpisodeType `json:"episodeType"`
	AdmissionReason    string      `json:"admissionReason"`
	PrimaryDiagnosis   string      `json:"primaryDiagnosis,omitempty"`
	SecondaryDiagnoses []string    `json:"secondaryDiagnoses,omitempty"`
	WardOrTeam         string      `json:"wardOrTeam,omitempty"`
	BedNumber          string      `json:"bedNumber,omitempty"`
	AdmittedAt         *time.Time  `json:"admittedAt,omitempty"`
}

// episodeUpdateRequest is the body for PUT /api/v1/episodes/{id}.
type episodeUpdateRequest struct {
	ResponsibleHPI     string        `json:"responsibleHpi,omitempty"`
	Status             EpisodeStatus `json:"status,omitempty"`
	PrimaryDiagnosis   string        `json:"primaryDiagnosis,omitempty"`
	SecondaryDiagnoses []string      `json:"secondaryDiagnoses,omitempty"`
	WardOrTeam         string        `json:"wardOrTeam,omitempty"`
	BedNumber          string        `json:"bedNumber,omitempty"`
}

// dischargeRequest is the body for POST /api/v1/episodes/{id}/discharge.
type dischargeRequest struct {
	DischargedAt    time.Time `json:"dischargedAt"`
	DischargeSummary string   `json:"dischargeSummary,omitempty"`
	Status          EpisodeStatus `json:"status,omitempty"` // defaults to "completed"
}

// wardRoundCreateRequest is the body for POST /api/v1/episodes/{id}/ward-rounds.
type wardRoundCreateRequest struct {
	ClinicianHPI string         `json:"clinicianHpi"`
	Notes        string         `json:"notes,omitempty"`
	MentalState  map[string]any `json:"mentalState,omitempty"`
	RiskLevel    RiskLevel      `json:"riskLevel,omitempty"`
	Plans        string         `json:"plans,omitempty"`
	OccurredAt   *time.Time     `json:"occurredAt,omitempty"`
}

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

// ---------------------------------------------------------------------------
// Internal record types
// ---------------------------------------------------------------------------

type episodeRecord struct {
	ID                 string
	PatientID          string
	PatientNHI         string
	TenantID           string
	ResponsibleHPI     string
	EpisodeType        string
	Status             string
	AdmissionReasonEnc []byte
	PrimaryDiagnosis   string
	SecondaryDiagnoses []string
	WardOrTeam         string
	BedNumber          string
	AdmittedAt         *time.Time
	DischargedAt       *time.Time
	DischargeSummaryEnc []byte
	ExtraSensitive     bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type wardRoundRecord struct {
	ID             string
	EpisodeID      string
	PatientID      string
	PatientNHI     string
	TenantID       string
	ClinicianHPI   string
	NotesEnc       []byte
	MentalState    []byte
	RiskLevel      string
	PlansEnc       []byte
	ExtraSensitive bool
	OccurredAt     time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ---------------------------------------------------------------------------
// Episode database helpers
// ---------------------------------------------------------------------------

func (h *EpisodesHandler) listEpisodes(
	ctx context.Context,
	tenantID uuid.UUID,
	patientFilter, statusFilter, typeFilter string,
) ([]episodeRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		        episode_type, status, admission_reason,
		        primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		        admitted_at, discharged_at, discharge_summary,
		        extra_sensitive, created_at, updated_at
		 FROM mh_episodes
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR status = $3)
		   AND ($4 = '' OR episode_type = $4)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		tenantID, patientFilter, statusFilter, typeFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("query episodes: %w", err)
	}
	defer rows.Close()

	var results []episodeRecord
	for rows.Next() {
		rec, err := scanEpisode(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (h *EpisodesHandler) getEpisodeByID(ctx context.Context, id string, tenantID uuid.UUID) (episodeRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		        episode_type, status, admission_reason,
		        primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		        admitted_at, discharged_at, discharge_summary,
		        extra_sensitive, created_at, updated_at
		 FROM mh_episodes
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanEpisodeRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return episodeRecord{}, errNotFound
		}
		return episodeRecord{}, fmt.Errorf("get episode by id: %w", err)
	}
	return rec, nil
}

func (h *EpisodesHandler) insertEpisode(ctx context.Context, req episodeCreateRequest, tenantID uuid.UUID) (episodeRecord, error) {
	reasonEnc, err := h.enc.Encrypt([]byte(req.AdmissionReason))
	if err != nil {
		return episodeRecord{}, fmt.Errorf("encrypt admission reason: %w", err)
	}

	admittedAt := req.AdmittedAt
	if admittedAt == nil {
		now := time.Now().UTC()
		admittedAt = &now
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO mh_episodes
		   (patient_id, patient_nhi, tenant_id, responsible_hpi,
		    episode_type, status, admission_reason,
		    primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		    admitted_at, extra_sensitive)
		 VALUES
		   ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, TRUE)
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           episode_type, status, admission_reason,
		           primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		           admitted_at, discharged_at, discharge_summary,
		           extra_sensitive, created_at, updated_at`,
		req.PatientID, req.PatientNHI, tenantID, req.ResponsibleHPI,
		string(req.EpisodeType), string(EpisodeActive), reasonEnc,
		req.PrimaryDiagnosis, req.SecondaryDiagnoses, req.WardOrTeam, req.BedNumber,
		admittedAt,
	)
	return scanEpisodeRow(row)
}

func (h *EpisodesHandler) updateEpisode(ctx context.Context, rec episodeRecord, tenantID uuid.UUID) (episodeRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE mh_episodes
		 SET responsible_hpi     = $1,
		     status              = $2,
		     primary_diagnosis   = $3,
		     secondary_diagnoses = $4,
		     ward_or_team        = $5,
		     bed_number          = $6,
		     updated_at          = now()
		 WHERE id = $7 AND tenant_id = $8
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           episode_type, status, admission_reason,
		           primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		           admitted_at, discharged_at, discharge_summary,
		           extra_sensitive, created_at, updated_at`,
		rec.ResponsibleHPI, rec.Status, rec.PrimaryDiagnosis,
		rec.SecondaryDiagnoses, rec.WardOrTeam, rec.BedNumber,
		rec.ID, tenantID,
	)
	updated, err := scanEpisodeRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return episodeRecord{}, errNotFound
		}
		return episodeRecord{}, fmt.Errorf("update episode: %w", err)
	}
	return updated, nil
}

func (h *EpisodesHandler) dischargeEpisode(
	ctx context.Context,
	id string,
	dischargedAt time.Time,
	status string,
	summaryEnc []byte,
	tenantID uuid.UUID,
) (episodeRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE mh_episodes
		 SET status           = $1,
		     discharged_at    = $2,
		     discharge_summary = $3,
		     updated_at       = now()
		 WHERE id = $4 AND tenant_id = $5
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           episode_type, status, admission_reason,
		           primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		           admitted_at, discharged_at, discharge_summary,
		           extra_sensitive, created_at, updated_at`,
		status, dischargedAt, summaryEnc, id, tenantID,
	)
	updated, err := scanEpisodeRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return episodeRecord{}, errNotFound
		}
		return episodeRecord{}, fmt.Errorf("discharge episode: %w", err)
	}
	return updated, nil
}

func (h *EpisodesHandler) decryptEpisode(rec episodeRecord) (Episode, error) {
	var reason string
	if len(rec.AdmissionReasonEnc) > 0 {
		plain, err := h.enc.Decrypt(rec.AdmissionReasonEnc)
		if err != nil {
			return Episode{}, fmt.Errorf("decrypt admission reason: %w", err)
		}
		reason = string(plain)
	}
	return Episode{
		ID:                 rec.ID,
		PatientID:          rec.PatientID,
		PatientNHI:         rec.PatientNHI,
		TenantID:           rec.TenantID,
		ResponsibleHPI:     rec.ResponsibleHPI,
		EpisodeType:        EpisodeType(rec.EpisodeType),
		Status:             EpisodeStatus(rec.Status),
		AdmissionReason:    reason,
		PrimaryDiagnosis:   rec.PrimaryDiagnosis,
		SecondaryDiagnoses: rec.SecondaryDiagnoses,
		WardOrTeam:         rec.WardOrTeam,
		BedNumber:          rec.BedNumber,
		AdmittedAt:         rec.AdmittedAt,
		DischargedAt:       rec.DischargedAt,
		ExtraSensitive:     rec.ExtraSensitive,
		CreatedAt:          rec.CreatedAt,
		UpdatedAt:          rec.UpdatedAt,
	}, nil
}

func scanEpisode(s rowScanner) (episodeRecord, error) {
	return scanEpisodeRow(s)
}

func scanEpisodeRow(s rowScanner) (episodeRecord, error) {
	var rec episodeRecord
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID, &rec.ResponsibleHPI,
		&rec.EpisodeType, &rec.Status, &rec.AdmissionReasonEnc,
		&rec.PrimaryDiagnosis, &rec.SecondaryDiagnoses, &rec.WardOrTeam, &rec.BedNumber,
		&rec.AdmittedAt, &rec.DischargedAt, &rec.DischargeSummaryEnc,
		&rec.ExtraSensitive, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return episodeRecord{}, err
	}
	return rec, nil
}

// ---------------------------------------------------------------------------
// Ward round database helpers
// ---------------------------------------------------------------------------

func (h *EpisodesHandler) listWardRounds(ctx context.Context, episodeID string, tenantID uuid.UUID) ([]WardRound, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, episode_id, patient_id, patient_nhi, tenant_id,
		        clinician_hpi, notes, mental_state, risk_level, plans,
		        extra_sensitive, occurred_at, created_at, updated_at
		 FROM mh_ward_rounds
		 WHERE episode_id = $1 AND tenant_id = $2
		 ORDER BY occurred_at DESC
		 LIMIT 200`,
		episodeID, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("query ward rounds: %w", err)
	}
	defer rows.Close()

	var results []WardRound
	for rows.Next() {
		rec, err := scanWardRound(rows)
		if err != nil {
			return nil, err
		}
		wr, err := h.decryptWardRound(rec)
		if err != nil {
			return nil, err
		}
		results = append(results, wr)
	}
	return results, rows.Err()
}

func (h *EpisodesHandler) getWardRoundByID(ctx context.Context, id, episodeID string, tenantID uuid.UUID) (WardRound, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, episode_id, patient_id, patient_nhi, tenant_id,
		        clinician_hpi, notes, mental_state, risk_level, plans,
		        extra_sensitive, occurred_at, created_at, updated_at
		 FROM mh_ward_rounds
		 WHERE id = $1 AND episode_id = $2 AND tenant_id = $3`,
		id, episodeID, tenantID,
	)
	rec, err := scanWardRoundRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WardRound{}, errNotFound
		}
		return WardRound{}, fmt.Errorf("get ward round by id: %w", err)
	}
	return h.decryptWardRound(rec)
}

func (h *EpisodesHandler) insertWardRound(
	ctx context.Context,
	episodeID string,
	ep episodeRecord,
	req wardRoundCreateRequest,
	tenantID uuid.UUID,
) (WardRound, error) {
	notesEnc, err := h.enc.Encrypt([]byte(req.Notes))
	if err != nil {
		return WardRound{}, fmt.Errorf("encrypt notes: %w", err)
	}
	plansEnc, err := h.enc.Encrypt([]byte(req.Plans))
	if err != nil {
		return WardRound{}, fmt.Errorf("encrypt plans: %w", err)
	}

	mentalStateJSON, err := json.Marshal(req.MentalState)
	if err != nil {
		return WardRound{}, fmt.Errorf("marshal mental state: %w", err)
	}

	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil {
		occurredAt = *req.OccurredAt
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO mh_ward_rounds
		   (episode_id, patient_id, patient_nhi, tenant_id,
		    clinician_hpi, notes, mental_state, risk_level, plans,
		    extra_sensitive, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, $10)
		 RETURNING id, episode_id, patient_id, patient_nhi, tenant_id,
		           clinician_hpi, notes, mental_state, risk_level, plans,
		           extra_sensitive, occurred_at, created_at, updated_at`,
		episodeID, ep.PatientID, ep.PatientNHI, tenantID,
		req.ClinicianHPI, notesEnc, mentalStateJSON, string(req.RiskLevel), plansEnc,
		occurredAt,
	)
	rec, err := scanWardRoundRow(row)
	if err != nil {
		return WardRound{}, fmt.Errorf("insert ward round: %w", err)
	}
	return h.decryptWardRound(rec)
}

func (h *EpisodesHandler) decryptWardRound(rec wardRoundRecord) (WardRound, error) {
	var notes, plans string
	if len(rec.NotesEnc) > 0 {
		plain, err := h.enc.Decrypt(rec.NotesEnc)
		if err != nil {
			return WardRound{}, fmt.Errorf("decrypt notes: %w", err)
		}
		notes = string(plain)
	}
	if len(rec.PlansEnc) > 0 {
		plain, err := h.enc.Decrypt(rec.PlansEnc)
		if err != nil {
			return WardRound{}, fmt.Errorf("decrypt plans: %w", err)
		}
		plans = string(plain)
	}

	var mentalState map[string]any
	if len(rec.MentalState) > 0 {
		if err := json.Unmarshal(rec.MentalState, &mentalState); err != nil {
			mentalState = map[string]any{}
		}
	}

	return WardRound{
		ID:             rec.ID,
		EpisodeID:      rec.EpisodeID,
		PatientID:      rec.PatientID,
		PatientNHI:     rec.PatientNHI,
		TenantID:       rec.TenantID,
		ClinicianHPI:   rec.ClinicianHPI,
		Notes:          notes,
		MentalState:    mentalState,
		RiskLevel:      RiskLevel(rec.RiskLevel),
		Plans:          plans,
		ExtraSensitive: rec.ExtraSensitive,
		OccurredAt:     rec.OccurredAt,
		CreatedAt:      rec.CreatedAt,
		UpdatedAt:      rec.UpdatedAt,
	}, nil
}

func scanWardRound(s rowScanner) (wardRoundRecord, error) {
	return scanWardRoundRow(s)
}

func scanWardRoundRow(s rowScanner) (wardRoundRecord, error) {
	var rec wardRoundRecord
	if err := s.Scan(
		&rec.ID, &rec.EpisodeID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID,
		&rec.ClinicianHPI, &rec.NotesEnc, &rec.MentalState, &rec.RiskLevel, &rec.PlansEnc,
		&rec.ExtraSensitive, &rec.OccurredAt, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return wardRoundRecord{}, err
	}
	return rec, nil
}

func validEpisodeType(t EpisodeType) bool {
	switch t {
	case EpisodeInpatient, EpisodeCommunity, EpisodeCrisis, EpisodeDayProgramme:
		return true
	}
	return false
}
