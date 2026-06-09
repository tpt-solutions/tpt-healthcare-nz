package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

type CardiacRehabProgramme struct {
	ID                     string     `json:"id"`
	PatientNHI             string     `json:"patientNhi"`
	ClinicianHpi           string     `json:"clinicianHpi"`
	Indication             string     `json:"indication"`
	Phase                  string     `json:"phase"`
	Status                 string     `json:"status"`
	RiskLevel              string     `json:"riskLevel"`
	TargetHrMin            *int16     `json:"targetHrMin"`
	TargetHrMax            *int16     `json:"targetHrMax"`
	BaselineMets           *float64   `json:"baselineMets"`
	GoalMets               *float64   `json:"goalMets"`
	SessionsPlanned        *int16     `json:"sessionsPlanned"`
	SessionsCompleted      *int16     `json:"sessionsCompleted"`
	Notes                  *string    `json:"notes"`
	TenantID               string     `json:"tenantId"`
	ReferredAt             time.Time  `json:"referredAt"`
	StartedAt              *time.Time `json:"startedAt"`
	CompletedAt            *time.Time `json:"completedAt"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type CardiacRehabSession struct {
	ID                string     `json:"id"`
	ProgrammeID       string     `json:"programmeId"`
	ClinicianHpi      string     `json:"clinicianHpi"`
	SessionType       string     `json:"sessionType"`
	SessionNumber     *int16     `json:"sessionNumber"`
	PeakHrBpm         *int16     `json:"peakHrBpm"`
	AchievedMets      *float64   `json:"achievedMets"`
	BorgRpe           *int16     `json:"borgRpe"`
	PreSystolicBp     *int16     `json:"preSystolicBp"`
	PreDiastolicBp    *int16     `json:"preDiastolicBp"`
	PostSystolicBp    *int16     `json:"postSystolicBp"`
	PostDiastolicBp   *int16     `json:"postDiastolicBp"`
	SymptomsDuring    string     `json:"symptomsDuring"`
	EcgChangesNoted   bool       `json:"ecgChangesNoted"`
	SessionNotes      *string    `json:"sessionNotes"`
	TenantID          string     `json:"tenantId"`
	SessionDate       time.Time  `json:"sessionDate"`
	DurationMinutes   *int16     `json:"durationMinutes"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

const rehabProgrammeSelectCols = `id, patient_nhi, clinician_hpi, indication, phase, status, risk_level,
       target_hr_min, target_hr_max, baseline_mets, goal_mets,
       sessions_planned, sessions_completed, notes, tenant_id,
       referred_at, started_at, completed_at, created_at, updated_at`

func scanRehabProgramme(row interface{ Scan(...any) error }, p *CardiacRehabProgramme) error {
	return row.Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.Indication, &p.Phase, &p.Status, &p.RiskLevel,
		&p.TargetHrMin, &p.TargetHrMax, &p.BaselineMets, &p.GoalMets,
		&p.SessionsPlanned, &p.SessionsCompleted, &p.Notes, &p.TenantID,
		&p.ReferredAt, &p.StartedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
}

type rehabHandler struct{ handlerDeps }

func (h *rehabHandler) ListProgrammes(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+rehabProgrammeSelectCols+` FROM cardiac_rehab_programmes WHERE tenant_id = @tenant_id AND status = @status ORDER BY referred_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+rehabProgrammeSelectCols+` FROM cardiac_rehab_programmes WHERE tenant_id = @tenant_id ORDER BY referred_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	programmes := make([]CardiacRehabProgramme, 0)
	for rows.Next() {
		var p CardiacRehabProgramme
		if err := scanRehabProgramme(rows, &p); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(p.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		p.PatientNHI = nhi
		programmes = append(programmes, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, programmes)
}

func (h *rehabHandler) CreateProgramme(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req CardiacRehabProgramme
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Phase == "" {
		req.Phase = "2"
	}
	if req.RiskLevel == "" {
		req.RiskLevel = "moderate"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var p CardiacRehabProgramme
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO cardiac_rehab_programmes
		    (patient_nhi, clinician_hpi, indication, phase, status, risk_level,
		     target_hr_min, target_hr_max, baseline_mets, goal_mets,
		     sessions_planned, sessions_completed, notes, tenant_id, referred_at)
		VALUES
		    (@patient_nhi, @clinician_hpi, @indication, @phase, 'referred', @risk_level,
		     @target_hr_min, @target_hr_max, @baseline_mets, @goal_mets,
		     @sessions_planned, 0, @notes, @tenant_id, COALESCE(@referred_at, now()))
		RETURNING `+rehabProgrammeSelectCols,
		pgx.NamedArgs{
			"patient_nhi":      nhiEnc,
			"clinician_hpi":    req.ClinicianHpi,
			"indication":       req.Indication,
			"phase":            req.Phase,
			"risk_level":       req.RiskLevel,
			"target_hr_min":    req.TargetHrMin,
			"target_hr_max":    req.TargetHrMax,
			"baseline_mets":    req.BaselineMets,
			"goal_mets":        req.GoalMets,
			"sessions_planned": req.SessionsPlanned,
			"notes":            req.Notes,
			"tenant_id":        tenantID,
			"referred_at":      req.ReferredAt,
		}).Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.Indication, &p.Phase, &p.Status, &p.RiskLevel,
		&p.TargetHrMin, &p.TargetHrMax, &p.BaselineMets, &p.GoalMets,
		&p.SessionsPlanned, &p.SessionsCompleted, &p.Notes, &p.TenantID,
		&p.ReferredAt, &p.StartedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	h.recordAudit(r, "create", "CardiacRehab", p.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, p)
}

func (h *rehabHandler) GetProgramme(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var p CardiacRehabProgramme
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+rehabProgrammeSelectCols+` FROM cardiac_rehab_programmes WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.Indication, &p.Phase, &p.Status, &p.RiskLevel,
		&p.TargetHrMin, &p.TargetHrMax, &p.BaselineMets, &p.GoalMets,
		&p.SessionsPlanned, &p.SessionsCompleted, &p.Notes, &p.TenantID,
		&p.ReferredAt, &p.StartedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "rehab programme not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

func (h *rehabHandler) UpdateProgramme(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req CardiacRehabProgramme
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var p CardiacRehabProgramme
	err := h.pool.QueryRow(r.Context(), `
		UPDATE cardiac_rehab_programmes
		SET clinician_hpi        = @clinician_hpi,
		    phase                = @phase,
		    status               = @status,
		    risk_level           = @risk_level,
		    target_hr_min        = @target_hr_min,
		    target_hr_max        = @target_hr_max,
		    baseline_mets        = @baseline_mets,
		    goal_mets            = @goal_mets,
		    sessions_planned     = @sessions_planned,
		    sessions_completed   = @sessions_completed,
		    notes                = @notes,
		    started_at           = COALESCE(started_at, CASE WHEN @status = 'active' THEN now() END),
		    completed_at         = CASE WHEN @status IN ('completed','withdrawn') THEN now() ELSE completed_at END,
		    updated_at           = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+rehabProgrammeSelectCols,
		pgx.NamedArgs{
			"clinician_hpi":      req.ClinicianHpi,
			"phase":              req.Phase,
			"status":             req.Status,
			"risk_level":         req.RiskLevel,
			"target_hr_min":      req.TargetHrMin,
			"target_hr_max":      req.TargetHrMax,
			"baseline_mets":      req.BaselineMets,
			"goal_mets":          req.GoalMets,
			"sessions_planned":   req.SessionsPlanned,
			"sessions_completed": req.SessionsCompleted,
			"notes":              req.Notes,
			"id":                 id,
			"tenant_id":          tenantID,
		}).Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.Indication, &p.Phase, &p.Status, &p.RiskLevel,
		&p.TargetHrMin, &p.TargetHrMax, &p.BaselineMets, &p.GoalMets,
		&p.SessionsPlanned, &p.SessionsCompleted, &p.Notes, &p.TenantID,
		&p.ReferredAt, &p.StartedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "rehab programme not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhiEnc := p.PatientNHI
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	h.recordAudit(r, "update", "CardiacRehab", p.ID, nhiEnc)
	writeJSON(w, http.StatusOK, p)
}

func (h *rehabHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	programmeID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, programme_id, clinician_hpi, session_type, session_number,
		       peak_hr_bpm, achieved_mets, borg_rpe,
		       pre_systolic_bp, pre_diastolic_bp, post_systolic_bp, post_diastolic_bp,
		       symptoms_during, ecg_changes_noted, session_notes,
		       tenant_id, session_date, duration_minutes, created_at, updated_at
		FROM cardiac_rehab_sessions
		WHERE programme_id = @programme_id AND tenant_id = @tenant_id
		ORDER BY session_date DESC
	`, pgx.NamedArgs{"programme_id": programmeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	sessions := make([]CardiacRehabSession, 0)
	for rows.Next() {
		var s CardiacRehabSession
		if err := rows.Scan(
			&s.ID, &s.ProgrammeID, &s.ClinicianHpi, &s.SessionType, &s.SessionNumber,
			&s.PeakHrBpm, &s.AchievedMets, &s.BorgRpe,
			&s.PreSystolicBp, &s.PreDiastolicBp, &s.PostSystolicBp, &s.PostDiastolicBp,
			&s.SymptomsDuring, &s.EcgChangesNoted, &s.SessionNotes,
			&s.TenantID, &s.SessionDate, &s.DurationMinutes, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *rehabHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	programmeID := r.PathValue("id")
	var req CardiacRehabSession
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.SessionType == "" {
		req.SessionType = "group"
	}
	if req.SymptomsDuring == "" {
		req.SymptomsDuring = "none"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var s CardiacRehabSession
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO cardiac_rehab_sessions
		    (programme_id, clinician_hpi, session_type, session_number,
		     peak_hr_bpm, achieved_mets, borg_rpe,
		     pre_systolic_bp, pre_diastolic_bp, post_systolic_bp, post_diastolic_bp,
		     symptoms_during, ecg_changes_noted, session_notes,
		     tenant_id, session_date, duration_minutes)
		VALUES
		    (@programme_id, @clinician_hpi, @session_type, @session_number,
		     @peak_hr_bpm, @achieved_mets, @borg_rpe,
		     @pre_systolic_bp, @pre_diastolic_bp, @post_systolic_bp, @post_diastolic_bp,
		     @symptoms_during, @ecg_changes_noted, @session_notes,
		     @tenant_id, COALESCE(@session_date, now()), @duration_minutes)
		RETURNING id, programme_id, clinician_hpi, session_type, session_number,
		          peak_hr_bpm, achieved_mets, borg_rpe,
		          pre_systolic_bp, pre_diastolic_bp, post_systolic_bp, post_diastolic_bp,
		          symptoms_during, ecg_changes_noted, session_notes,
		          tenant_id, session_date, duration_minutes, created_at, updated_at
	`, pgx.NamedArgs{
		"programme_id":     programmeID,
		"clinician_hpi":    req.ClinicianHpi,
		"session_type":     req.SessionType,
		"session_number":   req.SessionNumber,
		"peak_hr_bpm":      req.PeakHrBpm,
		"achieved_mets":    req.AchievedMets,
		"borg_rpe":         req.BorgRpe,
		"pre_systolic_bp":  req.PreSystolicBp,
		"pre_diastolic_bp": req.PreDiastolicBp,
		"post_systolic_bp": req.PostSystolicBp,
		"post_diastolic_bp": req.PostDiastolicBp,
		"symptoms_during":  req.SymptomsDuring,
		"ecg_changes_noted": req.EcgChangesNoted,
		"session_notes":    req.SessionNotes,
		"tenant_id":        tenantID,
		"session_date":     req.SessionDate,
		"duration_minutes": req.DurationMinutes,
	}).Scan(
		&s.ID, &s.ProgrammeID, &s.ClinicianHpi, &s.SessionType, &s.SessionNumber,
		&s.PeakHrBpm, &s.AchievedMets, &s.BorgRpe,
		&s.PreSystolicBp, &s.PreDiastolicBp, &s.PostSystolicBp, &s.PostDiastolicBp,
		&s.SymptomsDuring, &s.EcgChangesNoted, &s.SessionNotes,
		&s.TenantID, &s.SessionDate, &s.DurationMinutes, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "CardiacRehab", s.ID, "")
	writeJSON(w, http.StatusCreated, s)
}
