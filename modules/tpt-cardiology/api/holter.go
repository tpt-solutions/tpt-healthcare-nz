package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

type HolterMonitor struct {
	ID                    string     `json:"id"`
	PatientNHI            string     `json:"patientNhi"`
	OrderingClinicianHpi  string     `json:"orderingClinicianHpi"`
	ReportingClinicianHpi string     `json:"reportingClinicianHpi"`
	MonitorType           string     `json:"monitorType"`
	Status                string     `json:"status"`
	Indication            string     `json:"indication"`
	DurationHours         *int16     `json:"durationHours"`
	TotalBeats            *int       `json:"totalBeats"`
	MinHrBpm              *int16     `json:"minHrBpm"`
	MaxHrBpm              *int16     `json:"maxHrBpm"`
	MeanHrBpm             *int16     `json:"meanHrBpm"`
	AfBurdenPercent       *float64   `json:"afBurdenPercent"`
	PauseCount            *int       `json:"pauseCount"`
	LongestPauseSeconds   *float64   `json:"longestPauseSeconds"`
	SvtEpisodes           *int       `json:"svtEpisodes"`
	VtEpisodes            *int       `json:"vtEpisodes"`
	VfEpisodes            *int       `json:"vfEpisodes"`
	PvcBurdenPercent      *float64   `json:"pvcBurdenPercent"`
	Interpretation        string     `json:"interpretation"`
	Notes                 *string    `json:"notes"`
	TenantID              string     `json:"tenantId"`
	OrderedAt             time.Time  `json:"orderedAt"`
	MonitorOnAt           *time.Time `json:"monitorOnAt"`
	MonitorOffAt          *time.Time `json:"monitorOffAt"`
	ReportedAt            *time.Time `json:"reportedAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type ABPMStudy struct {
	ID                    string     `json:"id"`
	PatientNHI            string     `json:"patientNhi"`
	OrderingClinicianHpi  string     `json:"orderingClinicianHpi"`
	ReportingClinicianHpi string     `json:"reportingClinicianHpi"`
	Status                string     `json:"status"`
	Indication            string     `json:"indication"`
	DurationHours         *int16     `json:"durationHours"`
	ReadingsCount         *int16     `json:"readingsCount"`
	AwakeSystolicMean     *int16     `json:"awakeSystolicMean"`
	AwakeDiastolicMean    *int16     `json:"awakeDiastolicMean"`
	AwakeHrMean           *int16     `json:"awakeHrMean"`
	SleepSystolicMean     *int16     `json:"sleepSystolicMean"`
	SleepDiastolicMean    *int16     `json:"sleepDiastolicMean"`
	SleepHrMean           *int16     `json:"sleepHrMean"`
	OverallSystolicMean   *int16     `json:"overallSystolicMean"`
	OverallDiastolicMean  *int16     `json:"overallDiastolicMean"`
	DippingStatus         string     `json:"dippingStatus"`
	WchPattern            bool       `json:"wchPattern"`
	MaskedHypertension    bool       `json:"maskedHypertension"`
	Interpretation        string     `json:"interpretation"`
	Notes                 *string    `json:"notes"`
	TenantID              string     `json:"tenantId"`
	OrderedAt             time.Time  `json:"orderedAt"`
	MonitorOnAt           *time.Time `json:"monitorOnAt"`
	MonitorOffAt          *time.Time `json:"monitorOffAt"`
	ReportedAt            *time.Time `json:"reportedAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type holterHandler struct{ handlerDeps }

func (h *holterHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, monitor_type, status, indication,
		       duration_hours, total_beats, min_hr_bpm, max_hr_bpm, mean_hr_bpm,
		       af_burden_percent, pause_count, longest_pause_seconds,
		       svt_episodes, vt_episodes, vf_episodes, pvc_burden_percent,
		       interpretation, notes, tenant_id, ordered_at, monitor_on_at, monitor_off_at, reported_at, created_at, updated_at
		FROM holter_monitors WHERE tenant_id = @tenant_id ORDER BY ordered_at DESC
	`, pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	monitors := make([]HolterMonitor, 0)
	for rows.Next() {
		var m HolterMonitor
		if err := rows.Scan(
			&m.ID, &m.PatientNHI, &m.OrderingClinicianHpi, &m.ReportingClinicianHpi, &m.MonitorType, &m.Status, &m.Indication,
			&m.DurationHours, &m.TotalBeats, &m.MinHrBpm, &m.MaxHrBpm, &m.MeanHrBpm,
			&m.AfBurdenPercent, &m.PauseCount, &m.LongestPauseSeconds,
			&m.SvtEpisodes, &m.VtEpisodes, &m.VfEpisodes, &m.PvcBurdenPercent,
			&m.Interpretation, &m.Notes, &m.TenantID, &m.OrderedAt, &m.MonitorOnAt, &m.MonitorOffAt, &m.ReportedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(m.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		m.PatientNHI = nhi
		monitors = append(monitors, m)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, monitors)
}

func (h *holterHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req HolterMonitor
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.MonitorType == "" {
		req.MonitorType = "24h-holter"
	}
	if !h.validateHPI(w, r, req.OrderingClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var m HolterMonitor
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO holter_monitors
		    (patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, monitor_type, status, indication,
		     duration_hours, total_beats, min_hr_bpm, max_hr_bpm, mean_hr_bpm,
		     af_burden_percent, pause_count, longest_pause_seconds,
		     svt_episodes, vt_episodes, vf_episodes, pvc_burden_percent,
		     interpretation, notes, tenant_id)
		VALUES
		    (@patient_nhi, @ordering_clinician_hpi, @reporting_clinician_hpi, @monitor_type, 'ordered', @indication,
		     @duration_hours, @total_beats, @min_hr_bpm, @max_hr_bpm, @mean_hr_bpm,
		     @af_burden_percent, @pause_count, @longest_pause_seconds,
		     @svt_episodes, @vt_episodes, @vf_episodes, @pvc_burden_percent,
		     @interpretation, @notes, @tenant_id)
		RETURNING id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, monitor_type, status, indication,
		          duration_hours, total_beats, min_hr_bpm, max_hr_bpm, mean_hr_bpm,
		          af_burden_percent, pause_count, longest_pause_seconds,
		          svt_episodes, vt_episodes, vf_episodes, pvc_burden_percent,
		          interpretation, notes, tenant_id, ordered_at, monitor_on_at, monitor_off_at, reported_at, created_at, updated_at
	`, pgx.NamedArgs{
		"patient_nhi":            nhiEnc,
		"ordering_clinician_hpi": req.OrderingClinicianHpi,
		"reporting_clinician_hpi": req.ReportingClinicianHpi,
		"monitor_type":           req.MonitorType,
		"indication":             req.Indication,
		"duration_hours":         req.DurationHours,
		"total_beats":            req.TotalBeats,
		"min_hr_bpm":             req.MinHrBpm,
		"max_hr_bpm":             req.MaxHrBpm,
		"mean_hr_bpm":            req.MeanHrBpm,
		"af_burden_percent":      req.AfBurdenPercent,
		"pause_count":            req.PauseCount,
		"longest_pause_seconds":  req.LongestPauseSeconds,
		"svt_episodes":           req.SvtEpisodes,
		"vt_episodes":            req.VtEpisodes,
		"vf_episodes":            req.VfEpisodes,
		"pvc_burden_percent":     req.PvcBurdenPercent,
		"interpretation":         req.Interpretation,
		"notes":                  req.Notes,
		"tenant_id":              tenantID,
	}).Scan(
		&m.ID, &m.PatientNHI, &m.OrderingClinicianHpi, &m.ReportingClinicianHpi, &m.MonitorType, &m.Status, &m.Indication,
		&m.DurationHours, &m.TotalBeats, &m.MinHrBpm, &m.MaxHrBpm, &m.MeanHrBpm,
		&m.AfBurdenPercent, &m.PauseCount, &m.LongestPauseSeconds,
		&m.SvtEpisodes, &m.VtEpisodes, &m.VfEpisodes, &m.PvcBurdenPercent,
		&m.Interpretation, &m.Notes, &m.TenantID, &m.OrderedAt, &m.MonitorOnAt, &m.MonitorOffAt, &m.ReportedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(m.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	m.PatientNHI = nhi
	h.recordAudit(r, "create", "HolterMonitor", m.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, m)
}

func (h *holterHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var m HolterMonitor
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, monitor_type, status, indication,
		       duration_hours, total_beats, min_hr_bpm, max_hr_bpm, mean_hr_bpm,
		       af_burden_percent, pause_count, longest_pause_seconds,
		       svt_episodes, vt_episodes, vf_episodes, pvc_burden_percent,
		       interpretation, notes, tenant_id, ordered_at, monitor_on_at, monitor_off_at, reported_at, created_at, updated_at
		FROM holter_monitors WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&m.ID, &m.PatientNHI, &m.OrderingClinicianHpi, &m.ReportingClinicianHpi, &m.MonitorType, &m.Status, &m.Indication,
		&m.DurationHours, &m.TotalBeats, &m.MinHrBpm, &m.MaxHrBpm, &m.MeanHrBpm,
		&m.AfBurdenPercent, &m.PauseCount, &m.LongestPauseSeconds,
		&m.SvtEpisodes, &m.VtEpisodes, &m.VfEpisodes, &m.PvcBurdenPercent,
		&m.Interpretation, &m.Notes, &m.TenantID, &m.OrderedAt, &m.MonitorOnAt, &m.MonitorOffAt, &m.ReportedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "holter monitor not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(m.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	m.PatientNHI = nhi
	writeJSON(w, http.StatusOK, m)
}

func (h *holterHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req HolterMonitor
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ReportingClinicianHpi) {
		return
	}
	var m HolterMonitor
	err := h.pool.QueryRow(r.Context(), `
		UPDATE holter_monitors
		SET reporting_clinician_hpi = @reporting_clinician_hpi,
		    status                  = @status,
		    duration_hours          = @duration_hours,
		    total_beats             = @total_beats,
		    min_hr_bpm              = @min_hr_bpm,
		    max_hr_bpm              = @max_hr_bpm,
		    mean_hr_bpm             = @mean_hr_bpm,
		    af_burden_percent       = @af_burden_percent,
		    pause_count             = @pause_count,
		    longest_pause_seconds   = @longest_pause_seconds,
		    svt_episodes            = @svt_episodes,
		    vt_episodes             = @vt_episodes,
		    vf_episodes             = @vf_episodes,
		    pvc_burden_percent      = @pvc_burden_percent,
		    interpretation          = @interpretation,
		    notes                   = @notes,
		    monitor_on_at           = COALESCE(monitor_on_at, @monitor_on_at),
		    monitor_off_at          = COALESCE(monitor_off_at, @monitor_off_at),
		    reported_at             = CASE WHEN @status = 'reported' THEN now() ELSE reported_at END,
		    updated_at              = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, monitor_type, status, indication,
		          duration_hours, total_beats, min_hr_bpm, max_hr_bpm, mean_hr_bpm,
		          af_burden_percent, pause_count, longest_pause_seconds,
		          svt_episodes, vt_episodes, vf_episodes, pvc_burden_percent,
		          interpretation, notes, tenant_id, ordered_at, monitor_on_at, monitor_off_at, reported_at, created_at, updated_at
	`, pgx.NamedArgs{
		"reporting_clinician_hpi": req.ReportingClinicianHpi,
		"status":                  req.Status,
		"duration_hours":          req.DurationHours,
		"total_beats":             req.TotalBeats,
		"min_hr_bpm":              req.MinHrBpm,
		"max_hr_bpm":              req.MaxHrBpm,
		"mean_hr_bpm":             req.MeanHrBpm,
		"af_burden_percent":       req.AfBurdenPercent,
		"pause_count":             req.PauseCount,
		"longest_pause_seconds":   req.LongestPauseSeconds,
		"svt_episodes":            req.SvtEpisodes,
		"vt_episodes":             req.VtEpisodes,
		"vf_episodes":             req.VfEpisodes,
		"pvc_burden_percent":      req.PvcBurdenPercent,
		"interpretation":          req.Interpretation,
		"notes":                   req.Notes,
		"monitor_on_at":           req.MonitorOnAt,
		"monitor_off_at":          req.MonitorOffAt,
		"id":                      id,
		"tenant_id":               tenantID,
	}).Scan(
		&m.ID, &m.PatientNHI, &m.OrderingClinicianHpi, &m.ReportingClinicianHpi, &m.MonitorType, &m.Status, &m.Indication,
		&m.DurationHours, &m.TotalBeats, &m.MinHrBpm, &m.MaxHrBpm, &m.MeanHrBpm,
		&m.AfBurdenPercent, &m.PauseCount, &m.LongestPauseSeconds,
		&m.SvtEpisodes, &m.VtEpisodes, &m.VfEpisodes, &m.PvcBurdenPercent,
		&m.Interpretation, &m.Notes, &m.TenantID, &m.OrderedAt, &m.MonitorOnAt, &m.MonitorOffAt, &m.ReportedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "holter monitor not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhiEnc := m.PatientNHI
	nhi, err := h.decryptNHI(m.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	m.PatientNHI = nhi
	h.recordAudit(r, "update", "HolterMonitor", m.ID, nhiEnc)
	writeJSON(w, http.StatusOK, m)
}

// ABPM handlers

type abpmHandler struct{ handlerDeps }

func (h *abpmHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, status, indication,
		       duration_hours, readings_count,
		       awake_systolic_mean, awake_diastolic_mean, awake_hr_mean,
		       sleep_systolic_mean, sleep_diastolic_mean, sleep_hr_mean,
		       overall_systolic_mean, overall_diastolic_mean,
		       dipping_status, wch_pattern, masked_hypertension,
		       interpretation, notes, tenant_id, ordered_at, monitor_on_at, monitor_off_at, reported_at, created_at, updated_at
		FROM abpm_studies WHERE tenant_id = @tenant_id ORDER BY ordered_at DESC
	`, pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	studies := make([]ABPMStudy, 0)
	for rows.Next() {
		var s ABPMStudy
		if err := rows.Scan(
			&s.ID, &s.PatientNHI, &s.OrderingClinicianHpi, &s.ReportingClinicianHpi, &s.Status, &s.Indication,
			&s.DurationHours, &s.ReadingsCount,
			&s.AwakeSystolicMean, &s.AwakeDiastolicMean, &s.AwakeHrMean,
			&s.SleepSystolicMean, &s.SleepDiastolicMean, &s.SleepHrMean,
			&s.OverallSystolicMean, &s.OverallDiastolicMean,
			&s.DippingStatus, &s.WchPattern, &s.MaskedHypertension,
			&s.Interpretation, &s.Notes, &s.TenantID, &s.OrderedAt, &s.MonitorOnAt, &s.MonitorOffAt, &s.ReportedAt, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(s.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		s.PatientNHI = nhi
		studies = append(studies, s)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, studies)
}

func (h *abpmHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req ABPMStudy
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.OrderingClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var s ABPMStudy
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO abpm_studies
		    (patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, status, indication,
		     duration_hours, readings_count,
		     awake_systolic_mean, awake_diastolic_mean, awake_hr_mean,
		     sleep_systolic_mean, sleep_diastolic_mean, sleep_hr_mean,
		     overall_systolic_mean, overall_diastolic_mean,
		     dipping_status, wch_pattern, masked_hypertension,
		     interpretation, notes, tenant_id)
		VALUES
		    (@patient_nhi, @ordering_clinician_hpi, @reporting_clinician_hpi, 'ordered', @indication,
		     @duration_hours, @readings_count,
		     @awake_systolic_mean, @awake_diastolic_mean, @awake_hr_mean,
		     @sleep_systolic_mean, @sleep_diastolic_mean, @sleep_hr_mean,
		     @overall_systolic_mean, @overall_diastolic_mean,
		     @dipping_status, @wch_pattern, @masked_hypertension,
		     @interpretation, @notes, @tenant_id)
		RETURNING id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, status, indication,
		          duration_hours, readings_count,
		          awake_systolic_mean, awake_diastolic_mean, awake_hr_mean,
		          sleep_systolic_mean, sleep_diastolic_mean, sleep_hr_mean,
		          overall_systolic_mean, overall_diastolic_mean,
		          dipping_status, wch_pattern, masked_hypertension,
		          interpretation, notes, tenant_id, ordered_at, monitor_on_at, monitor_off_at, reported_at, created_at, updated_at
	`, pgx.NamedArgs{
		"patient_nhi":            nhiEnc,
		"ordering_clinician_hpi": req.OrderingClinicianHpi,
		"reporting_clinician_hpi": req.ReportingClinicianHpi,
		"indication":             req.Indication,
		"duration_hours":         req.DurationHours,
		"readings_count":         req.ReadingsCount,
		"awake_systolic_mean":    req.AwakeSystolicMean,
		"awake_diastolic_mean":   req.AwakeDiastolicMean,
		"awake_hr_mean":          req.AwakeHrMean,
		"sleep_systolic_mean":    req.SleepSystolicMean,
		"sleep_diastolic_mean":   req.SleepDiastolicMean,
		"sleep_hr_mean":          req.SleepHrMean,
		"overall_systolic_mean":  req.OverallSystolicMean,
		"overall_diastolic_mean": req.OverallDiastolicMean,
		"dipping_status":         req.DippingStatus,
		"wch_pattern":            req.WchPattern,
		"masked_hypertension":    req.MaskedHypertension,
		"interpretation":         req.Interpretation,
		"notes":                  req.Notes,
		"tenant_id":              tenantID,
	}).Scan(
		&s.ID, &s.PatientNHI, &s.OrderingClinicianHpi, &s.ReportingClinicianHpi, &s.Status, &s.Indication,
		&s.DurationHours, &s.ReadingsCount,
		&s.AwakeSystolicMean, &s.AwakeDiastolicMean, &s.AwakeHrMean,
		&s.SleepSystolicMean, &s.SleepDiastolicMean, &s.SleepHrMean,
		&s.OverallSystolicMean, &s.OverallDiastolicMean,
		&s.DippingStatus, &s.WchPattern, &s.MaskedHypertension,
		&s.Interpretation, &s.Notes, &s.TenantID, &s.OrderedAt, &s.MonitorOnAt, &s.MonitorOffAt, &s.ReportedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(s.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	s.PatientNHI = nhi
	h.recordAudit(r, "create", "HolterMonitor", s.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, s)
}

func (h *abpmHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var s ABPMStudy
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, status, indication,
		       duration_hours, readings_count,
		       awake_systolic_mean, awake_diastolic_mean, awake_hr_mean,
		       sleep_systolic_mean, sleep_diastolic_mean, sleep_hr_mean,
		       overall_systolic_mean, overall_diastolic_mean,
		       dipping_status, wch_pattern, masked_hypertension,
		       interpretation, notes, tenant_id, ordered_at, monitor_on_at, monitor_off_at, reported_at, created_at, updated_at
		FROM abpm_studies WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&s.ID, &s.PatientNHI, &s.OrderingClinicianHpi, &s.ReportingClinicianHpi, &s.Status, &s.Indication,
		&s.DurationHours, &s.ReadingsCount,
		&s.AwakeSystolicMean, &s.AwakeDiastolicMean, &s.AwakeHrMean,
		&s.SleepSystolicMean, &s.SleepDiastolicMean, &s.SleepHrMean,
		&s.OverallSystolicMean, &s.OverallDiastolicMean,
		&s.DippingStatus, &s.WchPattern, &s.MaskedHypertension,
		&s.Interpretation, &s.Notes, &s.TenantID, &s.OrderedAt, &s.MonitorOnAt, &s.MonitorOffAt, &s.ReportedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ABPM study not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(s.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	s.PatientNHI = nhi
	writeJSON(w, http.StatusOK, s)
}

func (h *abpmHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req ABPMStudy
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ReportingClinicianHpi) {
		return
	}
	var s ABPMStudy
	err := h.pool.QueryRow(r.Context(), `
		UPDATE abpm_studies
		SET reporting_clinician_hpi = @reporting_clinician_hpi,
		    status                  = @status,
		    duration_hours          = @duration_hours,
		    readings_count          = @readings_count,
		    awake_systolic_mean     = @awake_systolic_mean,
		    awake_diastolic_mean    = @awake_diastolic_mean,
		    awake_hr_mean           = @awake_hr_mean,
		    sleep_systolic_mean     = @sleep_systolic_mean,
		    sleep_diastolic_mean    = @sleep_diastolic_mean,
		    sleep_hr_mean           = @sleep_hr_mean,
		    overall_systolic_mean   = @overall_systolic_mean,
		    overall_diastolic_mean  = @overall_diastolic_mean,
		    dipping_status          = @dipping_status,
		    wch_pattern             = @wch_pattern,
		    masked_hypertension     = @masked_hypertension,
		    interpretation          = @interpretation,
		    notes                   = @notes,
		    monitor_on_at           = COALESCE(monitor_on_at, @monitor_on_at),
		    monitor_off_at          = COALESCE(monitor_off_at, @monitor_off_at),
		    reported_at             = CASE WHEN @status = 'reported' THEN now() ELSE reported_at END,
		    updated_at              = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, status, indication,
		          duration_hours, readings_count,
		          awake_systolic_mean, awake_diastolic_mean, awake_hr_mean,
		          sleep_systolic_mean, sleep_diastolic_mean, sleep_hr_mean,
		          overall_systolic_mean, overall_diastolic_mean,
		          dipping_status, wch_pattern, masked_hypertension,
		          interpretation, notes, tenant_id, ordered_at, monitor_on_at, monitor_off_at, reported_at, created_at, updated_at
	`, pgx.NamedArgs{
		"reporting_clinician_hpi": req.ReportingClinicianHpi,
		"status":                  req.Status,
		"duration_hours":          req.DurationHours,
		"readings_count":          req.ReadingsCount,
		"awake_systolic_mean":     req.AwakeSystolicMean,
		"awake_diastolic_mean":    req.AwakeDiastolicMean,
		"awake_hr_mean":           req.AwakeHrMean,
		"sleep_systolic_mean":     req.SleepSystolicMean,
		"sleep_diastolic_mean":    req.SleepDiastolicMean,
		"sleep_hr_mean":           req.SleepHrMean,
		"overall_systolic_mean":   req.OverallSystolicMean,
		"overall_diastolic_mean":  req.OverallDiastolicMean,
		"dipping_status":          req.DippingStatus,
		"wch_pattern":             req.WchPattern,
		"masked_hypertension":     req.MaskedHypertension,
		"interpretation":          req.Interpretation,
		"notes":                   req.Notes,
		"monitor_on_at":           req.MonitorOnAt,
		"monitor_off_at":          req.MonitorOffAt,
		"id":                      id,
		"tenant_id":               tenantID,
	}).Scan(
		&s.ID, &s.PatientNHI, &s.OrderingClinicianHpi, &s.ReportingClinicianHpi, &s.Status, &s.Indication,
		&s.DurationHours, &s.ReadingsCount,
		&s.AwakeSystolicMean, &s.AwakeDiastolicMean, &s.AwakeHrMean,
		&s.SleepSystolicMean, &s.SleepDiastolicMean, &s.SleepHrMean,
		&s.OverallSystolicMean, &s.OverallDiastolicMean,
		&s.DippingStatus, &s.WchPattern, &s.MaskedHypertension,
		&s.Interpretation, &s.Notes, &s.TenantID, &s.OrderedAt, &s.MonitorOnAt, &s.MonitorOffAt, &s.ReportedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ABPM study not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhiEnc := s.PatientNHI
	nhi, err := h.decryptNHI(s.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	s.PatientNHI = nhi
	h.recordAudit(r, "update", "HolterMonitor", s.ID, nhiEnc)
	writeJSON(w, http.StatusOK, s)
}
