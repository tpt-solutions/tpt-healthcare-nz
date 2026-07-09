package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

// --- Adverse event handler types (enum constants remain unchanged) ---

type adverseEventHandler struct {
	handlerDeps
}

func (h *adverseEventHandler) List(w http.ResponseWriter, r *http.Request) {
	participantID := r.PathValue("participantId")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, ae_term, meddra_code, ctcae_category, grade, status, causality,
		        is_serious, is_expected, is_related_to_study_drug, onset_date, resolution_date,
		        reported_by_hpi, reported_at, created_at, updated_at
		 FROM ct_adverse_events WHERE participant_id=$1 ORDER BY onset_date DESC`, participantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type aeRow struct {
		ID, AETerm, MEDDRACode, CTCaECat, Status, Causality, ReportedByHPI string
		Grade                                                              int
		IsSerious, IsExpected, IsRelated                                   bool
		OnsetDate, ResolutionDate                                          *time.Time
		ReportedAt, CreatedAt, UpdatedAt                                   time.Time
	}
	var events []aeRow
	for rows.Next() {
		var e aeRow
		if err := rows.Scan(&e.ID, &e.AETerm, &e.MEDDRACode, &e.CTCaECat, &e.Grade,
			&e.Status, &e.Causality, &e.IsSerious, &e.IsExpected, &e.IsRelated,
			&e.OnsetDate, &e.ResolutionDate, &e.ReportedByHPI, &e.ReportedAt,
			&e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		events = append(events, e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"adverseEvents": events, "total": len(events)})
}

func (h *adverseEventHandler) Create(w http.ResponseWriter, r *http.Request) {
	participantID := r.PathValue("participantId")
	var req struct {
		AETerm, MEDDRACode, CTCaECat, Status, Causality, ReportedByHPI string
		Grade                                                          int
		IsSerious, IsExpected, IsRelated                               bool
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_adverse_events (participant_id, ae_term, meddra_code, ctcae_category, grade,
		        status, causality, is_serious, is_expected, is_related_to_study_drug, reported_by_hpi)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		participantID, req.AETerm, req.MEDDRACode, req.CTCaECat, req.Grade,
		req.Status, req.Causality, req.IsSerious, req.IsExpected, req.IsRelated,
		req.ReportedByHPI).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *adverseEventHandler) Get(w http.ResponseWriter, r *http.Request) {
	aeID := r.PathValue("aeId")
	var aeTerm, status string
	var grade int
	err := h.pool.QueryRow(r.Context(),
		`SELECT ae_term, grade, status FROM ct_adverse_events WHERE id=$1`, aeID,
	).Scan(&aeTerm, &grade, &status)
	if err != nil {
		if db.IsNoRows(err) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "adverse event not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": aeID, "aeTerm": aeTerm, "grade": grade, "status": status})
}

func (h *adverseEventHandler) Update(w http.ResponseWriter, r *http.Request) {
	aeID := r.PathValue("aeId")
	var req struct {
		AETerm, Status, Causality string
		Grade                     int
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_adverse_events SET ae_term=$1, grade=$2, status=$3, causality=$4, updated_at=now()
		 WHERE id=$5`, req.AETerm, req.Grade, req.Status, req.Causality, aeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "adverse event not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": aeID, "updated": true})
}

func (h *adverseEventHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	aeID := r.PathValue("aeId")
	var req struct {
		ResolutionDate string `json:"resolutionDate"`
	}
	_ = decodeJSON(r, &req)
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_adverse_events SET status='resolved', resolution_date=COALESCE(NULLIF($1::date, null), now()::date), updated_at=now()
		 WHERE id=$2 AND status != 'resolved'`, req.ResolutionDate, aeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATE", Message: "adverse event already resolved"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": aeID, "status": "resolved"})
}

func (h *adverseEventHandler) GetSAE(w http.ResponseWriter, r *http.Request) {
	aeID := r.PathValue("aeId")
	var saeID, regulatoryStatus, medsafeRef string
	var version int
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, version, regulatory_status, medsafe_report_ref FROM ct_saes WHERE ae_id=$1`, aeID,
	).Scan(&saeID, &version, &regulatoryStatus, &medsafeRef)
	if err != nil {
		if db.IsNoRows(err) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "SAE not found for this adverse event"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": saeID, "aeId": aeID, "version": version,
		"regulatoryStatus": regulatoryStatus, "medsafeReportRef": medsafeRef})
}

func (h *adverseEventHandler) ReportSAE(w http.ResponseWriter, r *http.Request) {
	aeID := r.PathValue("aeId")
	var req struct {
		SAECategories []string `json:"saeCategories"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_saes (ae_id, sae_categories, regulatory_status)
		 VALUES ($1, $2, 'pending') RETURNING id`, aeID, req.SAECategories).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	// Mark the AE as serious
	_, _ = h.pool.Exec(r.Context(),
		`UPDATE ct_adverse_events SET is_serious=true, updated_at=now() WHERE id=$1`, aeID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "aeId": aeID})
}

func (h *adverseEventHandler) UpdateSAE(w http.ResponseWriter, r *http.Request) {
	aeID := r.PathValue("aeId")
	var req struct {
		OutcomeAtReport, RegulatoryStatus string
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_saes SET outcome_at_report=$1, regulatory_status=$2, version=version+1, updated_at=now()
		 WHERE ae_id=$3`, req.OutcomeAtReport, req.RegulatoryStatus, aeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "SAE not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": true})
}

func (h *adverseEventHandler) ReportSUSAR(w http.ResponseWriter, r *http.Request) {
	aeID := r.PathValue("aeId")
	// Find the SAE for this AE
	var saeID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT id FROM ct_saes WHERE ae_id=$1`, aeID).Scan(&saeID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "NO_SAE", Message: "must file SAE before SUSAR"})
		return
	}

	var id string
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_susars (sae_id, ae_id, expectedness, expedited_report, regulatory_status)
		 VALUES ($1, $2, 'unexpected', false, 'pending') RETURNING id`,
		saeID, aeID).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "saeId": saeID, "aeId": aeID})
}

func (h *adverseEventHandler) SafetyReport(w http.ResponseWriter, r *http.Request) {
	type safetySummary struct {
		TotalAEs      int `json:"totalAes"`
		SeriousAEs    int `json:"seriousAes"`
		OngoingAEs    int `json:"ongoingAes"`
		ResolvedAEs   int `json:"resolvedAes"`
		PendingSAEs   int `json:"pendingSaes"`
		PendingSUSARs int `json:"pendingSusars"`
	}
	var s safetySummary
	_ = h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE is_serious), COUNT(*) FILTER (WHERE status='ongoing'),
		        COUNT(*) FILTER (WHERE status='resolved') FROM ct_adverse_events`).Scan(
		&s.TotalAEs, &s.SeriousAEs, &s.OngoingAEs, &s.ResolvedAEs)
	_ = h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM ct_saes WHERE regulatory_status IN ('pending','due')`).Scan(&s.PendingSAEs)
	_ = h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM ct_susars WHERE regulatory_status IN ('pending','due')`).Scan(&s.PendingSUSARs)
	writeJSON(w, http.StatusOK, s)
}
