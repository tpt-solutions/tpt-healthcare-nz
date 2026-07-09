package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

// --- Visit handler types (enum constants remain unchanged) ---

type visitHandler struct {
	handlerDeps
}

func (h *visitHandler) List(w http.ResponseWriter, r *http.Request) {
	participantID := r.PathValue("participantId")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, visit_name, visit_type, sequence_no, status, planned_date, actual_date,
		        completed_by_hpi, within_window, days_from_baseline, notes, created_at, updated_at
		 FROM ct_participant_visits WHERE participant_id=$1 ORDER BY sequence_no`, participantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type visitRow struct {
		ID, VisitName, VisitType, CompletedByHPI, Notes string
		SeqNo                                           int
		Status                                          string
		PlannedDate, ActualDate                         *time.Time
		WithinWindow                                    bool
		DaysFromBaseline                                *int
		CreatedAt, UpdatedAt                            time.Time
	}
	var visits []visitRow
	for rows.Next() {
		var v visitRow
		if err := rows.Scan(&v.ID, &v.VisitName, &v.VisitType, &v.SeqNo, &v.Status,
			&v.PlannedDate, &v.ActualDate, &v.CompletedByHPI, &v.WithinWindow,
			&v.DaysFromBaseline, &v.Notes, &v.CreatedAt, &v.UpdatedAt); err != nil {
			continue
		}
		visits = append(visits, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"visits": visits, "total": len(visits)})
}

func (h *visitHandler) Create(w http.ResponseWriter, r *http.Request) {
	participantID := r.PathValue("participantId")
	var req struct {
		VisitName, VisitType, CompletedByHPI, Notes string
		SeqNo                                       int
		PlannedDate                                 *time.Time
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_participant_visits (participant_id, visit_name, visit_type, sequence_no, status, planned_date, notes)
		 VALUES ($1,$2,$3,$4,'scheduled',$5,$6) RETURNING id`,
		participantID, req.VisitName, req.VisitType, req.SeqNo, req.PlannedDate, req.Notes).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "scheduled"})
}

func (h *visitHandler) Get(w http.ResponseWriter, r *http.Request) {
	visitID := r.PathValue("visitId")
	var visitName, status, notes string
	var plannedDate *time.Time
	err := h.pool.QueryRow(r.Context(),
		`SELECT visit_name, status, planned_date, notes FROM ct_participant_visits WHERE id=$1`, visitID,
	).Scan(&visitName, &status, &plannedDate, &notes)
	if err != nil {
		if db.IsNoRows(err) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "visit not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": visitID, "visitName": visitName,
		"status": status, "plannedDate": plannedDate, "notes": notes})
}

func (h *visitHandler) Update(w http.ResponseWriter, r *http.Request) {
	visitID := r.PathValue("visitId")
	var req struct {
		VisitName, Notes string
		PlannedDate      *time.Time
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_participant_visits SET visit_name=$1, notes=$2, planned_date=$3, updated_at=now()
		 WHERE id=$4`, req.VisitName, req.Notes, req.PlannedDate, visitID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "visit not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": visitID, "updated": true})
}

func (h *visitHandler) Complete(w http.ResponseWriter, r *http.Request) {
	visitID := r.PathValue("visitId")
	var req struct {
		CompletedByHPI string `json:"completedByHpi"`
		ActualDate     string `json:"actualDate"`
	}
	_ = decodeJSON(r, &req)

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_participant_visits SET status='completed', completed_by_hpi=$1,
		        actual_date=COALESCE(NULLIF($2::date, null), now()::date), updated_at=now()
		 WHERE id=$3 AND status='scheduled'`, req.CompletedByHPI, req.ActualDate, visitID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATE", Message: "visit must be scheduled"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": visitID, "status": "completed"})
}

func (h *visitHandler) GetCRF(w http.ResponseWriter, r *http.Request) {
	visitID := r.PathValue("visitId")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, field_key, field_type, unit, normal_range, abnormal, clinically_significant,
		        entered_by_hpi, entered_at, query_flag, query_text, query_resolved
		 FROM ct_crf_entries WHERE visit_id=$1 ORDER BY field_key`, visitID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type crfEntry struct {
		ID, FieldKey, FieldType, Unit, NormalRange, EnteredByHPI, QueryText string
		Abnormal, ClinicallySignificant, QueryFlag, QueryResolved           bool
		EnteredAt                                                           time.Time
	}
	var entries []crfEntry
	for rows.Next() {
		var e crfEntry
		if err := rows.Scan(&e.ID, &e.FieldKey, &e.FieldType, &e.Unit, &e.NormalRange,
			&e.Abnormal, &e.ClinicallySignificant, &e.EnteredByHPI, &e.EnteredAt,
			&e.QueryFlag, &e.QueryText, &e.QueryResolved); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *visitHandler) SaveCRF(w http.ResponseWriter, r *http.Request) {
	visitID := r.PathValue("visitId")
	var req struct {
		Entries []struct {
			FieldKey, FieldType, Unit, NormalRange string
			Abnormal, ClinicallySignificant        bool
			Value                                  string
		}
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}

	// Get participant_id from the visit
	var participantID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT participant_id FROM ct_participant_visits WHERE id=$1`, visitID).Scan(&participantID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "visit not found"})
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer tx.Rollback(r.Context())

	for _, e := range req.Entries {
		_, err := tx.Exec(r.Context(),
			`INSERT INTO ct_crf_entries (visit_id, participant_id, field_key, field_type, unit, normal_range, abnormal, clinically_significant, entered_by_hpi)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'')`,
			visitID, participantID, e.FieldKey, e.FieldType, e.Unit, e.NormalRange,
			e.Abnormal, e.ClinicallySignificant)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": len(req.Entries)})
}

func (h *visitHandler) ListDeviations(w http.ResponseWriter, r *http.Request) {
	participantID := r.PathValue("participantId")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, category, severity, description, corrective_action, reported_by_hpi,
		        occurred_at, reported_at, hdec_reported_at, resolved, created_at
		 FROM ct_protocol_deviations WHERE participant_id=$1 ORDER BY occurred_at DESC`, participantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type deviation struct {
		ID, Category, Severity, Description, CorrectiveAction, ReportedByHPI string
		OccurredAt, ReportedAt, HDECReportedAt                               *time.Time
		Resolved                                                             bool
		CreatedAt                                                            time.Time
	}
	var deviations []deviation
	for rows.Next() {
		var d deviation
		if err := rows.Scan(&d.ID, &d.Category, &d.Severity, &d.Description,
			&d.CorrectiveAction, &d.ReportedByHPI, &d.OccurredAt, &d.ReportedAt,
			&d.HDECReportedAt, &d.Resolved, &d.CreatedAt); err != nil {
			continue
		}
		deviations = append(deviations, d)
	}
	writeJSON(w, http.StatusOK, deviations)
}

func (h *visitHandler) RecordDeviation(w http.ResponseWriter, r *http.Request) {
	participantID := r.PathValue("participantId")
	var req struct {
		Category, Severity, Description, CorrectiveAction, ReportedByHPI string
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_protocol_deviations (participant_id, category, severity, description, corrective_action, reported_by_hpi, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6,now()) RETURNING id`,
		participantID, req.Category, req.Severity, req.Description,
		req.CorrectiveAction, req.ReportedByHPI).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}
