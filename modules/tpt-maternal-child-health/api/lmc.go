package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// LMCRegistrationType classifies the nature of the case-loading.
type LMCRegistrationType string

const (
	LMCRegistrationTypePrimary   LMCRegistrationType = "primary"
	LMCRegistrationTypeSecondary LMCRegistrationType = "secondary"
	LMCRegistrationTypeHandover  LMCRegistrationType = "handover"
)

type LMCRegistration struct {
	ID               string     `json:"id"`
	EpisodeID        string     `json:"episodeId"`
	LMCHpi           string     `json:"lmcHpi"`
	LMCOrganisation  string     `json:"lmcOrganisation"`
	RegistrationType string     `json:"registrationType"`
	AcceptedAt       time.Time  `json:"acceptedAt"`
	HandoverAt       *time.Time `json:"handoverAt"`
	HandoverToHpi    *string    `json:"handoverToHpi"`
	HandoverReason   *string    `json:"handoverReason"`
	TenantID         string     `json:"tenantId"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type registerLMCReq struct {
	EpisodeID        string `json:"episodeId"`
	LMCHpi           string `json:"lmcHpi"`
	LMCOrganisation  string `json:"lmcOrganisation"`
	RegistrationType string `json:"registrationType"`
}

type handoverReq struct {
	HandoverToHpi  string `json:"handoverToHpi"`
	HandoverReason string `json:"handoverReason"`
}

// lmcHandler manages LMC registrations and case-loading for maternity episodes.
type lmcHandler struct {
	handlerDeps
}

func (h *lmcHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, episode_id, lmc_hpi, lmc_organisation, registration_type,
		       accepted_at, handover_at, handover_to_hpi, handover_reason,
		       tenant_id, created_at, updated_at
		FROM lmc_registrations
		WHERE tenant_id = @tenant_id
		ORDER BY created_at DESC
	`, pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()

	registrations := make([]LMCRegistration, 0)
	for rows.Next() {
		var reg LMCRegistration
		if err := rows.Scan(
			&reg.ID, &reg.EpisodeID, &reg.LMCHpi, &reg.LMCOrganisation, &reg.RegistrationType,
			&reg.AcceptedAt, &reg.HandoverAt, &reg.HandoverToHpi, &reg.HandoverReason,
			&reg.TenantID, &reg.CreatedAt, &reg.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		registrations = append(registrations, reg)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, registrations)
}

func (h *lmcHandler) Register(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	var req registerLMCReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.LMCHpi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_HPI", Message: "lmcHpi is required"})
		return
	}
	if req.RegistrationType == "" {
		req.RegistrationType = string(LMCRegistrationTypePrimary)
	}

	var reg LMCRegistration
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO lmc_registrations
		    (episode_id, lmc_hpi, lmc_organisation, registration_type, tenant_id)
		VALUES
		    (@episode_id, @lmc_hpi, @lmc_org, @reg_type, @tenant_id)
		RETURNING id, episode_id, lmc_hpi, lmc_organisation, registration_type,
		          accepted_at, handover_at, handover_to_hpi, handover_reason,
		          tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"episode_id": req.EpisodeID,
		"lmc_hpi":    req.LMCHpi,
		"lmc_org":    req.LMCOrganisation,
		"reg_type":   req.RegistrationType,
		"tenant_id":  tenantID,
	}).Scan(
		&reg.ID, &reg.EpisodeID, &reg.LMCHpi, &reg.LMCOrganisation, &reg.RegistrationType,
		&reg.AcceptedAt, &reg.HandoverAt, &reg.HandoverToHpi, &reg.HandoverReason,
		&reg.TenantID, &reg.CreatedAt, &reg.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, reg)
}

func (h *lmcHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	id := r.PathValue("id")
	var reg LMCRegistration
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, episode_id, lmc_hpi, lmc_organisation, registration_type,
		       accepted_at, handover_at, handover_to_hpi, handover_reason,
		       tenant_id, created_at, updated_at
		FROM lmc_registrations
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&reg.ID, &reg.EpisodeID, &reg.LMCHpi, &reg.LMCOrganisation, &reg.RegistrationType,
		&reg.AcceptedAt, &reg.HandoverAt, &reg.HandoverToHpi, &reg.HandoverReason,
		&reg.TenantID, &reg.CreatedAt, &reg.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "registration not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, reg)
}

func (h *lmcHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	id := r.PathValue("id")
	var req registerLMCReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	var reg LMCRegistration
	err := h.pool.QueryRow(r.Context(), `
		UPDATE lmc_registrations
		SET lmc_hpi = @lmc_hpi,
		    lmc_organisation = @lmc_org,
		    registration_type = @reg_type,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, episode_id, lmc_hpi, lmc_organisation, registration_type,
		          accepted_at, handover_at, handover_to_hpi, handover_reason,
		          tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"lmc_hpi":   req.LMCHpi,
		"lmc_org":   req.LMCOrganisation,
		"reg_type":  req.RegistrationType,
		"id":        id,
		"tenant_id": tenantID,
	}).Scan(
		&reg.ID, &reg.EpisodeID, &reg.LMCHpi, &reg.LMCOrganisation, &reg.RegistrationType,
		&reg.AcceptedAt, &reg.HandoverAt, &reg.HandoverToHpi, &reg.HandoverReason,
		&reg.TenantID, &reg.CreatedAt, &reg.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "registration not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, reg)
}

// Handover closes the current LMC registration and records the handover details.
func (h *lmcHandler) Handover(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	id := r.PathValue("id")
	var req handoverReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.HandoverToHpi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_HPI", Message: "handoverToHpi is required"})
		return
	}

	tag, err := h.pool.Exec(r.Context(), `
		UPDATE lmc_registrations
		SET handover_at = now(),
		    handover_to_hpi = @handover_hpi,
		    handover_reason = @handover_reason,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND handover_at IS NULL
	`, pgx.NamedArgs{
		"handover_hpi":    req.HandoverToHpi,
		"handover_reason": req.HandoverReason,
		"id":              id,
		"tenant_id":       tenantID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "registration not found or already handed over"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "handed-over", "handoverToHpi": req.HandoverToHpi})
}
