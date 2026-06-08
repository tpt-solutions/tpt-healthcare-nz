package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// EpisodeStatus tracks the overall maternity episode lifecycle.
type EpisodeStatus string

const (
	EpisodeStatusBooking     EpisodeStatus = "booking"
	EpisodeStatusAntenatal   EpisodeStatus = "antenatal"
	EpisodeStatusIntrapartum EpisodeStatus = "intrapartum"
	EpisodeStatusPostnatal   EpisodeStatus = "postnatal"
	EpisodeStatusCompleted   EpisodeStatus = "completed"
	EpisodeStatusClosed      EpisodeStatus = "closed"
)

// RiskLevel classifies the level of maternity care required.
type RiskLevel string

const (
	RiskLevelStandard  RiskLevel = "standard"
	RiskLevelEnhanced  RiskLevel = "enhanced"
	RiskLevelObstetric RiskLevel = "obstetric"
)

type Episode struct {
	ID                     string    `json:"id"`
	PatientNHI             string    `json:"patientNhi"`
	LMCHpi                 string    `json:"lmcHpi"`
	Status                 string    `json:"status"`
	EDD                    *string   `json:"edd"`
	LMP                    *string   `json:"lmp"`
	GestationAtBookingWeeks *int16   `json:"gestationAtBookingWeeks"`
	Gravida                *int16    `json:"gravida"`
	Parity                 *int16    `json:"parity"`
	RiskLevel              string    `json:"riskLevel"`
	Notes                  *string   `json:"notes"`
	TenantID               string    `json:"tenantId"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type createEpisodeReq struct {
	PatientNHI              string  `json:"patientNhi"`
	LMCHpi                  string  `json:"lmcHpi"`
	EDD                     *string `json:"edd"`
	LMP                     *string `json:"lmp"`
	GestationAtBookingWeeks *int16  `json:"gestationAtBookingWeeks"`
	Gravida                 *int16  `json:"gravida"`
	Parity                  *int16  `json:"parity"`
	RiskLevel               string  `json:"riskLevel"`
	Notes                   *string `json:"notes"`
}

// episodeHandler manages maternity episodes of care.
type episodeHandler struct {
	handlerDeps
}

func (h *episodeHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	statusFilter := r.URL.Query().Get("status")

	var (
		rows pgx.Rows
		err  error
	)
	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(), `
			SELECT id, patient_nhi, lmc_hpi, status, edd, lmp,
			       gestation_at_booking_weeks, gravida, parity, risk_level, notes, tenant_id,
			       created_at, updated_at
			FROM maternity_episodes
			WHERE tenant_id = @tenant_id AND status = @status
			ORDER BY created_at DESC
		`, pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(), `
			SELECT id, patient_nhi, lmc_hpi, status, edd, lmp,
			       gestation_at_booking_weeks, gravida, parity, risk_level, notes, tenant_id,
			       created_at, updated_at
			FROM maternity_episodes
			WHERE tenant_id = @tenant_id
			ORDER BY created_at DESC
		`, pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()

	episodes := make([]Episode, 0)
	for rows.Next() {
		var ep Episode
		if err := rows.Scan(
			&ep.ID, &ep.PatientNHI, &ep.LMCHpi, &ep.Status, &ep.EDD, &ep.LMP,
			&ep.GestationAtBookingWeeks, &ep.Gravida, &ep.Parity, &ep.RiskLevel, &ep.Notes,
			&ep.TenantID, &ep.CreatedAt, &ep.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		episodes = append(episodes, ep)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, episodes)
}

func (h *episodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	var req createEpisodeReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "patientNhi is required"})
		return
	}
	if req.RiskLevel == "" {
		req.RiskLevel = string(RiskLevelStandard)
	}

	var ep Episode
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO maternity_episodes
		    (patient_nhi, lmc_hpi, status, edd, lmp, gestation_at_booking_weeks, gravida, parity, risk_level, notes, tenant_id)
		VALUES
		    (@nhi, @lmc_hpi, 'booking', @edd, @lmp, @gestation, @gravida, @parity, @risk_level, @notes, @tenant_id)
		RETURNING id, patient_nhi, lmc_hpi, status, edd, lmp,
		          gestation_at_booking_weeks, gravida, parity, risk_level, notes, tenant_id,
		          created_at, updated_at
	`, pgx.NamedArgs{
		"nhi":        req.PatientNHI,
		"lmc_hpi":    req.LMCHpi,
		"edd":        req.EDD,
		"lmp":        req.LMP,
		"gestation":  req.GestationAtBookingWeeks,
		"gravida":    req.Gravida,
		"parity":     req.Parity,
		"risk_level": req.RiskLevel,
		"notes":      req.Notes,
		"tenant_id":  tenantID,
	}).Scan(
		&ep.ID, &ep.PatientNHI, &ep.LMCHpi, &ep.Status, &ep.EDD, &ep.LMP,
		&ep.GestationAtBookingWeeks, &ep.Gravida, &ep.Parity, &ep.RiskLevel, &ep.Notes,
		&ep.TenantID, &ep.CreatedAt, &ep.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: fmt.Sprintf("create episode: %s", err)})
		return
	}
	writeJSON(w, http.StatusCreated, ep)
}

func (h *episodeHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	id := r.PathValue("id")
	var ep Episode
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, patient_nhi, lmc_hpi, status, edd, lmp,
		       gestation_at_booking_weeks, gravida, parity, risk_level, notes, tenant_id,
		       created_at, updated_at
		FROM maternity_episodes
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&ep.ID, &ep.PatientNHI, &ep.LMCHpi, &ep.Status, &ep.EDD, &ep.LMP,
		&ep.GestationAtBookingWeeks, &ep.Gravida, &ep.Parity, &ep.RiskLevel, &ep.Notes,
		&ep.TenantID, &ep.CreatedAt, &ep.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ep)
}

func (h *episodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	id := r.PathValue("id")
	var req createEpisodeReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	var ep Episode
	err := h.pool.QueryRow(r.Context(), `
		UPDATE maternity_episodes
		SET lmc_hpi = @lmc_hpi,
		    edd = @edd,
		    lmp = @lmp,
		    gestation_at_booking_weeks = @gestation,
		    gravida = @gravida,
		    parity = @parity,
		    risk_level = @risk_level,
		    notes = @notes,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, patient_nhi, lmc_hpi, status, edd, lmp,
		          gestation_at_booking_weeks, gravida, parity, risk_level, notes, tenant_id,
		          created_at, updated_at
	`, pgx.NamedArgs{
		"lmc_hpi":   req.LMCHpi,
		"edd":       req.EDD,
		"lmp":       req.LMP,
		"gestation": req.GestationAtBookingWeeks,
		"gravida":   req.Gravida,
		"parity":    req.Parity,
		"risk_level": req.RiskLevel,
		"notes":     req.Notes,
		"id":        id,
		"tenant_id": tenantID,
	}).Scan(
		&ep.ID, &ep.PatientNHI, &ep.LMCHpi, &ep.Status, &ep.EDD, &ep.LMP,
		&ep.GestationAtBookingWeeks, &ep.Gravida, &ep.Parity, &ep.RiskLevel, &ep.Notes,
		&ep.TenantID, &ep.CreatedAt, &ep.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ep)
}

func (h *episodeHandler) Close(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}

	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE maternity_episodes
		SET status = 'closed', updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status != 'closed'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found or already closed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "closed"})
}
