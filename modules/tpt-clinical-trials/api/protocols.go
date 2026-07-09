package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

// --- Protocol handler types (enum constants remain unchanged) ---

// protocolHandler manages the study protocol library including arms, eligibility
// criteria, visit schedules, and protocol amendments.
type protocolHandler struct {
	handlerDeps
}

func (h *protocolHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, tenant_id, title, short_title, anzctr_number, hdec_number, sponsor,
		        principal_investigator_hpi, phase, trial_type, intervention_type,
		        blinding, allocation, icd10_code, condition_name, target_enrolment,
		        status, approved_at, opened_at, closed_at, completed_at, notes,
		        created_at, updated_at
		 FROM ct_protocols
		 ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()

	type protocolRow struct {
		ID, TenantID, Title, ShortTitle, ANZCTR, HDEC, Sponsor, PIHPI string
		Phase, TrialType, InterventionType, Blinding, Allocation      string
		ICD10, Condition                                              string
		TargetEnrolment                                               int
		Status                                                        string
		ApprovedAt, OpenedAt, ClosedAt, CompletedAt                   *time.Time
		Notes                                                         string
		CreatedAt, UpdatedAt                                          time.Time
	}
	var results []protocolRow
	for rows.Next() {
		var p protocolRow
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Title, &p.ShortTitle, &p.ANZCTR,
			&p.HDEC, &p.Sponsor, &p.PIHPI, &p.Phase, &p.TrialType, &p.InterventionType,
			&p.Blinding, &p.Allocation, &p.ICD10, &p.Condition, &p.TargetEnrolment,
			&p.Status, &p.ApprovedAt, &p.OpenedAt, &p.ClosedAt, &p.CompletedAt,
			&p.Notes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			h.logger.Error("scan protocol row", "error", err)
			continue
		}
		results = append(results, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"protocols": results, "total": len(results)})
}

func (h *protocolHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title, ShortTitle, ANZCTR, HDEC, Sponsor, PIHPI string
		Phase, TrialType, InterventionType, Blinding    string
		Allocation, ICD10, Condition, Notes             string
		TargetEnrolment                                 int
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	var id string
	now := time.Now().UTC()
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_protocols (title, short_title, anzctr_number, hdec_number, sponsor,
		        principal_investigator_hpi, phase, trial_type, intervention_type, blinding,
		        allocation, icd10_code, condition_name, target_enrolment, notes, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`,
		req.Title, req.ShortTitle, req.ANZCTR, req.HDEC, req.Sponsor, req.PIHPI,
		req.Phase, req.TrialType, req.InterventionType, req.Blinding,
		req.Allocation, req.ICD10, req.Condition, req.TargetEnrolment, req.Notes, now, now,
	).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "draft", "createdAt": now})
}

func (h *protocolHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var title, status string
	err := h.pool.QueryRow(r.Context(),
		`SELECT title, status FROM ct_protocols WHERE id = $1`, id,
	).Scan(&title, &status)
	if err != nil {
		if db.IsNoRows(err) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "protocol not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "title": title, "status": status})
}

func (h *protocolHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Title, ShortTitle, Condition, Notes string
		TargetEnrolment                     int
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_protocols SET title=$1, short_title=$2, condition_name=$3, notes=$4,
		        target_enrolment=$5, updated_at=now() WHERE id=$6`,
		req.Title, req.ShortTitle, req.Condition, req.Notes, req.TargetEnrolment, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "protocol not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "updated": true})
}

func (h *protocolHandler) Activate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_protocols SET status='active', opened_at=now(), updated_at=now() WHERE id=$1 AND status='approved'`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATE", Message: "protocol must be in 'approved' status to activate"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "active"})
}

func (h *protocolHandler) Close(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_protocols SET status='closed', closed_at=now(), updated_at=now() WHERE id=$1 AND status='active'`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATE", Message: "protocol must be in 'active' status to close"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "closed"})
}

func (h *protocolHandler) ListArms(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name, arm_type, description, intervention, dose, route, frequency, duration
		 FROM ct_protocol_arms WHERE protocol_id = $1 ORDER BY created_at`, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type arm struct {
		ID, Name, ArmType, Description, Intervention, Dose, Route, Frequency, Duration string
	}
	var arms []arm
	for rows.Next() {
		var a arm
		if err := rows.Scan(&a.ID, &a.Name, &a.ArmType, &a.Description, &a.Intervention,
			&a.Dose, &a.Route, &a.Frequency, &a.Duration); err != nil {
			continue
		}
		arms = append(arms, a)
	}
	writeJSON(w, http.StatusOK, arms)
}

func (h *protocolHandler) CreateArm(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	var req struct {
		Name, ArmType, Description, Intervention, Dose, Route, Frequency, Duration string
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_protocol_arms (protocol_id, name, arm_type, description, intervention, dose, route, frequency, duration)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		protocolID, req.Name, req.ArmType, req.Description, req.Intervention,
		req.Dose, req.Route, req.Frequency, req.Duration).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *protocolHandler) GetEligibility(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, criterion_type, sequence_no, text FROM ct_eligibility_criteria
		 WHERE protocol_id = $1 ORDER BY criterion_type, sequence_no`, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type criterion struct {
		ID, CriterionType, Text string
		SeqNo                   int
	}
	var criteria []criterion
	for rows.Next() {
		var c criterion
		if err := rows.Scan(&c.ID, &c.CriterionType, &c.SeqNo, &c.Text); err != nil {
			continue
		}
		criteria = append(criteria, c)
	}
	writeJSON(w, http.StatusOK, criteria)
}

func (h *protocolHandler) UpdateEligibility(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	var req struct {
		Criteria []struct {
			CriterionType, Text string
			SeqNo               int
		}
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer tx.Rollback(r.Context())
	_, _ = tx.Exec(r.Context(), `DELETE FROM ct_eligibility_criteria WHERE protocol_id = $1`, protocolID)
	for _, c := range req.Criteria {
		_, err := tx.Exec(r.Context(),
			`INSERT INTO ct_eligibility_criteria (protocol_id, criterion_type, sequence_no, text) VALUES ($1,$2,$3,$4)`,
			protocolID, c.CriterionType, c.SeqNo, c.Text)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": true})
}

func (h *protocolHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name, visit_type, sequence_no, day_from_baseline, window_before, window_after, assessments_required
		 FROM ct_scheduled_visits WHERE protocol_id = $1 ORDER BY sequence_no`, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type schedVisit struct {
		ID, Name, VisitType       string
		SeqNo, DayFromBaseline    int
		WindowBefore, WindowAfter int
		Assessments               json.RawMessage
	}
	var visits []schedVisit
	for rows.Next() {
		var v schedVisit
		if err := rows.Scan(&v.ID, &v.Name, &v.VisitType, &v.SeqNo, &v.DayFromBaseline,
			&v.WindowBefore, &v.WindowAfter, &v.Assessments); err != nil {
			continue
		}
		visits = append(visits, v)
	}
	writeJSON(w, http.StatusOK, visits)
}

func (h *protocolHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	var req struct {
		Visits []struct {
			Name, VisitType           string
			SeqNo, DayFromBL          int
			WindowBefore, WindowAfter int
			Assessments               json.RawMessage
		}
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer tx.Rollback(r.Context())
	_, _ = tx.Exec(r.Context(), `DELETE FROM ct_scheduled_visits WHERE protocol_id = $1`, protocolID)
	for _, v := range req.Visits {
		_, err := tx.Exec(r.Context(),
			`INSERT INTO ct_scheduled_visits (protocol_id, name, visit_type, sequence_no, day_from_baseline, window_before, window_after, assessments_required)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			protocolID, v.Name, v.VisitType, v.SeqNo, v.DayFromBL, v.WindowBefore, v.WindowAfter, v.Assessments)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": true})
}

func (h *protocolHandler) ListAmendments(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, amendment_number, hdec_amendment_ref, summary, effective_date, approved_at, created_at
		 FROM ct_protocol_amendments WHERE protocol_id = $1 ORDER BY amendment_number`, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type amendment struct {
		ID, HDECRef, Summary string
		AmendmentNo          int
		EffectiveDate        *time.Time
		ApprovedAt           *time.Time
		CreatedAt            time.Time
	}
	var amendments []amendment
	for rows.Next() {
		var a amendment
		if err := rows.Scan(&a.ID, &a.AmendmentNo, &a.HDECRef, &a.Summary,
			&a.EffectiveDate, &a.ApprovedAt, &a.CreatedAt); err != nil {
			continue
		}
		amendments = append(amendments, a)
	}
	writeJSON(w, http.StatusOK, amendments)
}

func (h *protocolHandler) CreateAmendment(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	var req struct {
		HDECRef, Summary string
		EffectiveDate    *time.Time
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	var amendNo int
	_ = h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(amendment_number), 0) FROM ct_protocol_amendments WHERE protocol_id = $1`,
		protocolID).Scan(&amendNo)
	amendNo++

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_protocol_amendments (protocol_id, amendment_number, hdec_amendment_ref, summary, effective_date)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		protocolID, amendNo, req.HDECRef, req.Summary, req.EffectiveDate).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "amendmentNumber": amendNo})
}
