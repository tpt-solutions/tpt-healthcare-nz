package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

type ECGStudy struct {
	ID                    string     `json:"id"`
	PatientNHI            string     `json:"patientNhi"`
	OrderingClinicianHpi  string     `json:"orderingClinicianHpi"`
	ReportingClinicianHpi string     `json:"reportingClinicianHpi"`
	StudyType             string     `json:"studyType"`
	Status                string     `json:"status"`
	Indication            string     `json:"indication"`
	HeartRateBpm          *int16     `json:"heartRateBpm"`
	Rhythm                string     `json:"rhythm"`
	PrIntervalMs          *int16     `json:"prIntervalMs"`
	QrsDurationMs         *int16     `json:"qrsDurationMs"`
	QtIntervalMs          *int16     `json:"qtIntervalMs"`
	QtcMs                 *int16     `json:"qtcMs"`
	QrsAxisDegrees        *int16     `json:"qrsAxisDegrees"`
	PAxisDegrees          *int16     `json:"pAxisDegrees"`
	StChanges             string     `json:"stChanges"`
	TWaveChanges          string     `json:"tWaveChanges"`
	Lbbb                  bool       `json:"lbbb"`
	Rbbb                  bool       `json:"rbbb"`
	WolffParkinsonWhite   bool       `json:"wolffParkinsonWhite"`
	Interpretation        string     `json:"interpretation"`
	Notes                 *string    `json:"notes"`
	TenantID              string     `json:"tenantId"`
	OrderedAt             time.Time  `json:"orderedAt"`
	PerformedAt           *time.Time `json:"performedAt"`
	ReportedAt            *time.Time `json:"reportedAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

const ecgSelectCols = `id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, study_type, status,
       indication, heart_rate_bpm, rhythm, pr_interval_ms, qrs_duration_ms, qt_interval_ms, qtc_ms,
       qrs_axis_degrees, p_axis_degrees, st_changes, t_wave_changes, lbbb, rbbb, wolff_parkinson_white,
       interpretation, notes, tenant_id, ordered_at, performed_at, reported_at, created_at, updated_at`

func scanECG(row interface{ Scan(...any) error }, e *ECGStudy) error {
	return row.Scan(
		&e.ID, &e.PatientNHI, &e.OrderingClinicianHpi, &e.ReportingClinicianHpi, &e.StudyType, &e.Status,
		&e.Indication, &e.HeartRateBpm, &e.Rhythm, &e.PrIntervalMs, &e.QrsDurationMs, &e.QtIntervalMs, &e.QtcMs,
		&e.QrsAxisDegrees, &e.PAxisDegrees, &e.StChanges, &e.TWaveChanges, &e.Lbbb, &e.Rbbb, &e.WolffParkinsonWhite,
		&e.Interpretation, &e.Notes, &e.TenantID, &e.OrderedAt, &e.PerformedAt, &e.ReportedAt, &e.CreatedAt, &e.UpdatedAt,
	)
}

type ecgHandler struct{ handlerDeps }

func (h *ecgHandler) List(w http.ResponseWriter, r *http.Request) {
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
			`SELECT `+ecgSelectCols+` FROM ecg_studies WHERE tenant_id = @tenant_id AND status = @status ORDER BY ordered_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+ecgSelectCols+` FROM ecg_studies WHERE tenant_id = @tenant_id ORDER BY ordered_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	studies := make([]ECGStudy, 0)
	for rows.Next() {
		var e ECGStudy
		if err := scanECG(rows, &e); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(e.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		e.PatientNHI = nhi
		studies = append(studies, e)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, studies)
}

func (h *ecgHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req ECGStudy
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.StudyType == "" {
		req.StudyType = "resting"
	}
	if req.StChanges == "" {
		req.StChanges = "none"
	}
	if req.TWaveChanges == "" {
		req.TWaveChanges = "none"
	}
	if !h.validateHPI(w, r, req.OrderingClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var e ECGStudy
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO ecg_studies
		    (patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, study_type, status,
		     indication, heart_rate_bpm, rhythm, pr_interval_ms, qrs_duration_ms, qt_interval_ms, qtc_ms,
		     qrs_axis_degrees, p_axis_degrees, st_changes, t_wave_changes,
		     lbbb, rbbb, wolff_parkinson_white, interpretation, notes, tenant_id)
		VALUES
		    (@patient_nhi, @ordering_clinician_hpi, @reporting_clinician_hpi, @study_type, 'ordered',
		     @indication, @heart_rate_bpm, @rhythm, @pr_interval_ms, @qrs_duration_ms, @qt_interval_ms, @qtc_ms,
		     @qrs_axis_degrees, @p_axis_degrees, @st_changes, @t_wave_changes,
		     @lbbb, @rbbb, @wolff_parkinson_white, @interpretation, @notes, @tenant_id)
		RETURNING `+ecgSelectCols,
		pgx.NamedArgs{
			"patient_nhi":            nhiEnc,
			"ordering_clinician_hpi": req.OrderingClinicianHpi,
			"reporting_clinician_hpi": req.ReportingClinicianHpi,
			"study_type":             req.StudyType,
			"indication":             req.Indication,
			"heart_rate_bpm":         req.HeartRateBpm,
			"rhythm":                 req.Rhythm,
			"pr_interval_ms":         req.PrIntervalMs,
			"qrs_duration_ms":        req.QrsDurationMs,
			"qt_interval_ms":         req.QtIntervalMs,
			"qtc_ms":                 req.QtcMs,
			"qrs_axis_degrees":       req.QrsAxisDegrees,
			"p_axis_degrees":         req.PAxisDegrees,
			"st_changes":             req.StChanges,
			"t_wave_changes":         req.TWaveChanges,
			"lbbb":                   req.Lbbb,
			"rbbb":                   req.Rbbb,
			"wolff_parkinson_white":  req.WolffParkinsonWhite,
			"interpretation":         req.Interpretation,
			"notes":                  req.Notes,
			"tenant_id":              tenantID,
		}).Scan(
		&e.ID, &e.PatientNHI, &e.OrderingClinicianHpi, &e.ReportingClinicianHpi, &e.StudyType, &e.Status,
		&e.Indication, &e.HeartRateBpm, &e.Rhythm, &e.PrIntervalMs, &e.QrsDurationMs, &e.QtIntervalMs, &e.QtcMs,
		&e.QrsAxisDegrees, &e.PAxisDegrees, &e.StChanges, &e.TWaveChanges, &e.Lbbb, &e.Rbbb, &e.WolffParkinsonWhite,
		&e.Interpretation, &e.Notes, &e.TenantID, &e.OrderedAt, &e.PerformedAt, &e.ReportedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(e.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	e.PatientNHI = nhi
	h.recordAudit(r, "create", "ECGStudy", e.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, e)
}

func (h *ecgHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var e ECGStudy
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+ecgSelectCols+` FROM ecg_studies WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&e.ID, &e.PatientNHI, &e.OrderingClinicianHpi, &e.ReportingClinicianHpi, &e.StudyType, &e.Status,
		&e.Indication, &e.HeartRateBpm, &e.Rhythm, &e.PrIntervalMs, &e.QrsDurationMs, &e.QtIntervalMs, &e.QtcMs,
		&e.QrsAxisDegrees, &e.PAxisDegrees, &e.StChanges, &e.TWaveChanges, &e.Lbbb, &e.Rbbb, &e.WolffParkinsonWhite,
		&e.Interpretation, &e.Notes, &e.TenantID, &e.OrderedAt, &e.PerformedAt, &e.ReportedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ECG study not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(e.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	e.PatientNHI = nhi
	writeJSON(w, http.StatusOK, e)
}

func (h *ecgHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req ECGStudy
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ReportingClinicianHpi) {
		return
	}
	var e ECGStudy
	err := h.pool.QueryRow(r.Context(), `
		UPDATE ecg_studies
		SET reporting_clinician_hpi = @reporting_clinician_hpi,
		    status          = @status,
		    heart_rate_bpm  = @heart_rate_bpm,
		    rhythm          = @rhythm,
		    pr_interval_ms  = @pr_interval_ms,
		    qrs_duration_ms = @qrs_duration_ms,
		    qt_interval_ms  = @qt_interval_ms,
		    qtc_ms          = @qtc_ms,
		    qrs_axis_degrees = @qrs_axis_degrees,
		    p_axis_degrees  = @p_axis_degrees,
		    st_changes      = @st_changes,
		    t_wave_changes  = @t_wave_changes,
		    lbbb            = @lbbb,
		    rbbb            = @rbbb,
		    wolff_parkinson_white = @wolff_parkinson_white,
		    interpretation  = @interpretation,
		    notes           = @notes,
		    performed_at    = COALESCE(performed_at, CASE WHEN @status IN ('performed','reported') THEN now() END),
		    reported_at     = CASE WHEN @status = 'reported' THEN now() ELSE reported_at END,
		    updated_at      = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+ecgSelectCols,
		pgx.NamedArgs{
			"reporting_clinician_hpi": req.ReportingClinicianHpi,
			"status":                  req.Status,
			"heart_rate_bpm":          req.HeartRateBpm,
			"rhythm":                  req.Rhythm,
			"pr_interval_ms":          req.PrIntervalMs,
			"qrs_duration_ms":         req.QrsDurationMs,
			"qt_interval_ms":          req.QtIntervalMs,
			"qtc_ms":                  req.QtcMs,
			"qrs_axis_degrees":        req.QrsAxisDegrees,
			"p_axis_degrees":          req.PAxisDegrees,
			"st_changes":              req.StChanges,
			"t_wave_changes":          req.TWaveChanges,
			"lbbb":                    req.Lbbb,
			"rbbb":                    req.Rbbb,
			"wolff_parkinson_white":   req.WolffParkinsonWhite,
			"interpretation":          req.Interpretation,
			"notes":                   req.Notes,
			"id":                      id,
			"tenant_id":               tenantID,
		}).Scan(
		&e.ID, &e.PatientNHI, &e.OrderingClinicianHpi, &e.ReportingClinicianHpi, &e.StudyType, &e.Status,
		&e.Indication, &e.HeartRateBpm, &e.Rhythm, &e.PrIntervalMs, &e.QrsDurationMs, &e.QtIntervalMs, &e.QtcMs,
		&e.QrsAxisDegrees, &e.PAxisDegrees, &e.StChanges, &e.TWaveChanges, &e.Lbbb, &e.Rbbb, &e.WolffParkinsonWhite,
		&e.Interpretation, &e.Notes, &e.TenantID, &e.OrderedAt, &e.PerformedAt, &e.ReportedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ECG study not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhiEnc := e.PatientNHI
	nhi, err := h.decryptNHI(e.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	e.PatientNHI = nhi
	h.recordAudit(r, "update", "ECGStudy", e.ID, nhiEnc)
	writeJSON(w, http.StatusOK, e)
}
