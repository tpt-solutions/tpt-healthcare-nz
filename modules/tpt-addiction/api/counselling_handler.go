package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type counsellingHandler struct{ handlerDeps }

// ListSessions GET /api/v1/counselling/sessions
func (h *counsellingHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	nhiFilter := r.URL.Query().Get("patientNhi")
	var (
		rows pgx.Rows
		err  error
	)
	if nhiFilter != "" {
		nhiEnc, encErr := h.encryptNHI(nhiFilter)
		if encErr != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt NHI"})
			return
		}
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+sessionSelectCols+` FROM addiction_counselling_sessions
			 WHERE tenant_id = @tenant_id AND patient_nhi = @patient_nhi
			 ORDER BY session_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "patient_nhi": nhiEnc})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+sessionSelectCols+` FROM addiction_counselling_sessions
			 WHERE tenant_id = @tenant_id
			 ORDER BY session_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	sessions := make([]CounsellingSession, 0)
	for rows.Next() {
		var s CounsellingSession
		if err := scanSession(rows, &s); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, _ := h.decryptNHI(s.PatientNHI)
		s.PatientNHI = nhi
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

// CreateSession POST /api/v1/counselling/sessions
func (h *counsellingHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	var req CounsellingSession
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.BillingType == "" {
		req.BillingType = "dhb_funded"
	}
	if !h.validateHPI(w, r, req.ClinicianID) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt NHI"})
		return
	}
	var s CounsellingSession
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO addiction_counselling_sessions
		    (tenant_id, patient_nhi, clinician_id, practice_id,
		     session_type, group_session_id, session_date, duration_min,
		     modality, presenting_issue, clinical_notes, risk_assessment,
		     readiness_score, homework_given, next_session_date,
		     billing_type, fee_in_cents)
		VALUES
		    (@tenant_id, @patient_nhi, @clinician_id, @practice_id,
		     @session_type, @group_session_id, COALESCE(@session_date, now()), @duration_min,
		     @modality, @presenting_issue, @clinical_notes, @risk_assessment,
		     @readiness_score, @homework_given, @next_session_date,
		     @billing_type, @fee_in_cents)
		RETURNING `+sessionSelectCols,
		pgx.NamedArgs{
			"tenant_id":         tenantID,
			"patient_nhi":       nhiEnc,
			"clinician_id":      req.ClinicianID,
			"practice_id":       req.PracticeID,
			"session_type":      req.SessionType,
			"group_session_id":  req.GroupSessionID,
			"session_date":      req.SessionDate,
			"duration_min":      req.DurationMin,
			"modality":          req.Modality,
			"presenting_issue":  req.PresentingIssue,
			"clinical_notes":    req.ClinicalNotes,
			"risk_assessment":   req.RiskAssessment,
			"readiness_score":   req.ReadinessScore,
			"homework_given":    req.HomeworkGiven,
			"next_session_date": req.NextSessionDate,
			"billing_type":      req.BillingType,
			"fee_in_cents":      req.FeeInCents,
		}).Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.ClinicianID, &s.PracticeID,
		&s.SessionType, &s.GroupSessionID, &s.SessionDate, &s.DurationMin, &s.Modality,
		&s.PresentingIssue, &s.ClinicalNotes, &s.RiskAssessment,
		&s.ReadinessScore, &s.HomeworkGiven, &s.NextSessionDate,
		&s.BillingType, &s.FeeInCents, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "CounsellingSession", s.ID, s.PatientNHI)
	nhi, _ := h.decryptNHI(s.PatientNHI)
	s.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, s)
}

// GetSession GET /api/v1/counselling/sessions/{sessionId}
func (h *counsellingHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	id := r.PathValue("sessionId")
	var s CounsellingSession
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+sessionSelectCols+` FROM addiction_counselling_sessions
		 WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.ClinicianID, &s.PracticeID,
		&s.SessionType, &s.GroupSessionID, &s.SessionDate, &s.DurationMin, &s.Modality,
		&s.PresentingIssue, &s.ClinicalNotes, &s.RiskAssessment,
		&s.ReadinessScore, &s.HomeworkGiven, &s.NextSessionDate,
		&s.BillingType, &s.FeeInCents, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "session not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "CounsellingSession", s.ID, s.PatientNHI)
	nhi, _ := h.decryptNHI(s.PatientNHI)
	s.PatientNHI = nhi
	writeJSON(w, http.StatusOK, s)
}

// UpdateSession PUT /api/v1/counselling/sessions/{sessionId}
func (h *counsellingHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	id := r.PathValue("sessionId")
	var req struct {
		ClinicalNotes   string     `json:"clinicalNotes"`
		RiskAssessment  string     `json:"riskAssessment"`
		ReadinessScore  *int       `json:"readinessScore"`
		HomeworkGiven   string     `json:"homeworkGiven"`
		NextSessionDate *time.Time `json:"nextSessionDate"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var nhiEnc string
	var s CounsellingSession
	err := h.pool.QueryRow(r.Context(), `
		UPDATE addiction_counselling_sessions
		SET clinical_notes    = COALESCE(NULLIF(@clinical_notes,''), clinical_notes),
		    risk_assessment   = COALESCE(NULLIF(@risk_assessment,''), risk_assessment),
		    readiness_score   = COALESCE(@readiness_score, readiness_score),
		    homework_given    = COALESCE(NULLIF(@homework_given,''), homework_given),
		    next_session_date = COALESCE(@next_session_date, next_session_date),
		    updated_at        = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+sessionSelectCols,
		pgx.NamedArgs{
			"clinical_notes":    req.ClinicalNotes,
			"risk_assessment":   req.RiskAssessment,
			"readiness_score":   req.ReadinessScore,
			"homework_given":    req.HomeworkGiven,
			"next_session_date": req.NextSessionDate,
			"id":                id,
			"tenant_id":         tenantID,
		}).Scan(
		&s.ID, &s.TenantID, &nhiEnc, &s.ClinicianID, &s.PracticeID,
		&s.SessionType, &s.GroupSessionID, &s.SessionDate, &s.DurationMin, &s.Modality,
		&s.PresentingIssue, &s.ClinicalNotes, &s.RiskAssessment,
		&s.ReadinessScore, &s.HomeworkGiven, &s.NextSessionDate,
		&s.BillingType, &s.FeeInCents, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "session not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "CounsellingSession", s.ID, nhiEnc)
	nhi, _ := h.decryptNHI(nhiEnc)
	s.PatientNHI = nhi
	writeJSON(w, http.StatusOK, s)
}

// ListGroupSessions GET /api/v1/counselling/group-sessions
func (h *counsellingHandler) ListGroupSessions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+groupSelectCols+` FROM addiction_group_sessions
		 WHERE tenant_id = @tenant_id ORDER BY scheduled_at DESC`,
		pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	groups := make([]GroupSession, 0)
	for rows.Next() {
		var g GroupSession
		if err := rows.Scan(
			&g.ID, &g.TenantID, &g.Name, &g.ClinicianID, &g.PracticeID,
			&g.ScheduledAt, &g.DurationMin, &g.Topic, &g.MaxAttendees,
			&g.Attendees, &g.Notes, &g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

// CreateGroupSession POST /api/v1/counselling/group-sessions
func (h *counsellingHandler) CreateGroupSession(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	var req GroupSession
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.MaxAttendees == 0 {
		req.MaxAttendees = 12
	}
	if req.Attendees == nil {
		req.Attendees = []string{}
	}
	var g GroupSession
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO addiction_group_sessions
		    (tenant_id, name, clinician_id, practice_id,
		     scheduled_at, duration_min, topic, max_attendees, attendees, notes)
		VALUES
		    (@tenant_id, @name, @clinician_id, @practice_id,
		     @scheduled_at, @duration_min, @topic, @max_attendees, @attendees, @notes)
		RETURNING `+groupSelectCols,
		pgx.NamedArgs{
			"tenant_id":     tenantID,
			"name":          req.Name,
			"clinician_id":  req.ClinicianID,
			"practice_id":   req.PracticeID,
			"scheduled_at":  req.ScheduledAt,
			"duration_min":  req.DurationMin,
			"topic":         req.Topic,
			"max_attendees": req.MaxAttendees,
			"attendees":     req.Attendees,
			"notes":         req.Notes,
		}).Scan(
		&g.ID, &g.TenantID, &g.Name, &g.ClinicianID, &g.PracticeID,
		&g.ScheduledAt, &g.DurationMin, &g.Topic, &g.MaxAttendees,
		&g.Attendees, &g.Notes, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "GroupSession", g.ID, "")
	writeJSON(w, http.StatusCreated, g)
}

// GetGroupSession GET /api/v1/counselling/group-sessions/{groupId}
func (h *counsellingHandler) GetGroupSession(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	id := r.PathValue("groupId")
	var g GroupSession
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+groupSelectCols+` FROM addiction_group_sessions
		 WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&g.ID, &g.TenantID, &g.Name, &g.ClinicianID, &g.PracticeID,
		&g.ScheduledAt, &g.DurationMin, &g.Topic, &g.MaxAttendees,
		&g.Attendees, &g.Notes, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "group session not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// ListTreatmentPlans GET /api/v1/counselling/treatment-plans
func (h *counsellingHandler) ListTreatmentPlans(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+planSelectCols+` FROM addiction_treatment_plans
		 WHERE tenant_id = @tenant_id AND status = 'active'
		 ORDER BY start_date DESC`,
		pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	plans := make([]TreatmentPlan, 0)
	for rows.Next() {
		var p TreatmentPlan
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.PatientNHI, &p.ProgrammeID, &p.ClinicianID, &p.PracticeID,
			&p.StartDate, &p.ReviewDate, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, _ := h.decryptNHI(p.PatientNHI)
		p.PatientNHI = nhi
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, plans)
}

// CreateTreatmentPlan POST /api/v1/counselling/treatment-plans
func (h *counsellingHandler) CreateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	var req TreatmentPlan
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianID) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt NHI"})
		return
	}
	var p TreatmentPlan
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO addiction_treatment_plans
		    (tenant_id, patient_nhi, programme_id, clinician_id, practice_id,
		     start_date, review_date)
		VALUES
		    (@tenant_id, @patient_nhi, @programme_id, @clinician_id, @practice_id,
		     COALESCE(@start_date, now()), COALESCE(@review_date, now() + interval '90 days'))
		RETURNING `+planSelectCols,
		pgx.NamedArgs{
			"tenant_id":    tenantID,
			"patient_nhi":  nhiEnc,
			"programme_id": req.ProgrammeID,
			"clinician_id": req.ClinicianID,
			"practice_id":  req.PracticeID,
			"start_date":   req.StartDate,
			"review_date":  req.ReviewDate,
		}).Scan(
		&p.ID, &p.TenantID, &p.PatientNHI, &p.ProgrammeID, &p.ClinicianID, &p.PracticeID,
		&p.StartDate, &p.ReviewDate, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "TreatmentPlan", p.ID, p.PatientNHI)
	nhi, _ := h.decryptNHI(p.PatientNHI)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, p)
}

// GetTreatmentPlan GET /api/v1/counselling/treatment-plans/{planId}
func (h *counsellingHandler) GetTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	id := r.PathValue("planId")
	var p TreatmentPlan
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+planSelectCols+` FROM addiction_treatment_plans
		 WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.TenantID, &p.PatientNHI, &p.ProgrammeID, &p.ClinicianID, &p.PracticeID,
		&p.StartDate, &p.ReviewDate, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "treatment plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "TreatmentPlan", p.ID, p.PatientNHI)
	nhi, _ := h.decryptNHI(p.PatientNHI)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

// UpdateTreatmentPlan PUT /api/v1/counselling/treatment-plans/{planId}
func (h *counsellingHandler) UpdateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	id := r.PathValue("planId")
	var req struct {
		ReviewDate *time.Time `json:"reviewDate"`
		Status     string     `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var nhiEnc string
	var p TreatmentPlan
	err := h.pool.QueryRow(r.Context(), `
		UPDATE addiction_treatment_plans
		SET review_date = COALESCE(@review_date, review_date),
		    status      = COALESCE(NULLIF(@status,''), status),
		    updated_at  = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+planSelectCols,
		pgx.NamedArgs{
			"review_date": req.ReviewDate,
			"status":      req.Status,
			"id":          id,
			"tenant_id":   tenantID,
		}).Scan(
		&p.ID, &p.TenantID, &nhiEnc, &p.ProgrammeID, &p.ClinicianID, &p.PracticeID,
		&p.StartDate, &p.ReviewDate, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "treatment plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "TreatmentPlan", p.ID, nhiEnc)
	nhi, _ := h.decryptNHI(nhiEnc)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

// AddGoal POST /api/v1/counselling/treatment-plans/{planId}/goals
func (h *counsellingHandler) AddGoal(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	planID := r.PathValue("planId")
	var req Goal
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Status == "" {
		req.Status = "not_started"
	}
	// Verify plan exists and belongs to tenant.
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM addiction_treatment_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": planID, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "treatment plan not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	var g Goal
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO addiction_goals (tenant_id, plan_id, description, target_date, status, evidence)
		VALUES (@tenant_id, @plan_id, @description, @target_date, @status, @evidence)
		RETURNING id, tenant_id, plan_id, description, target_date, status,
		          COALESCE(evidence,''), created_at`,
		pgx.NamedArgs{
			"tenant_id":   tenantID,
			"plan_id":     planID,
			"description": req.Description,
			"target_date": req.TargetDate,
			"status":      req.Status,
			"evidence":    req.Evidence,
		}).Scan(
		&g.ID, &g.TenantID, &g.PlanID, &g.Description, &g.TargetDate, &g.Status,
		&g.Evidence, &g.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "TreatmentGoal", g.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, g)
}

// RecordRelapse POST /api/v1/counselling/treatment-plans/{planId}/relapses
func (h *counsellingHandler) RecordRelapse(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	planID := r.PathValue("planId")
	var req RelapseEvent
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM addiction_treatment_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": planID, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "treatment plan not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	var e RelapseEvent
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO addiction_relapses
		    (tenant_id, plan_id, occurred_at, substance_used,
		     trigger_notes, severity, intervention, plan_modified)
		VALUES
		    (@tenant_id, @plan_id, COALESCE(@occurred_at, now()), @substance_used,
		     @trigger_notes, @severity, @intervention, @plan_modified)
		RETURNING id, tenant_id, plan_id, occurred_at, substance_used,
		          COALESCE(trigger_notes,''), severity,
		          COALESCE(intervention,''), plan_modified, created_at`,
		pgx.NamedArgs{
			"tenant_id":      tenantID,
			"plan_id":        planID,
			"occurred_at":    req.OccurredAt,
			"substance_used": req.SubstanceUsed,
			"trigger_notes":  req.TriggerNotes,
			"severity":       req.Severity,
			"intervention":   req.Intervention,
			"plan_modified":  req.PlanModified,
		}).Scan(
		&e.ID, &e.TenantID, &e.PlanID, &e.OccurredAt, &e.SubstanceUsed,
		&e.TriggerNotes, &e.Severity, &e.Intervention, &e.PlanModified, &e.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "RelapseEvent", e.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, e)
}
