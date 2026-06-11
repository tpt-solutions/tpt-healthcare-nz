package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

type CathProcedure struct {
	ID                       string     `json:"id"`
	PatientNHI               string     `json:"patientNhi"`
	OperatorClinicianHpi     string     `json:"operatorClinicianHpi"`
	ProcedureType            string     `json:"procedureType"`
	Status                   string     `json:"status"`
	Indication               string     `json:"indication"`
	AccessSite               string     `json:"accessSite"`
	AnaesthesiaType          string     `json:"anaesthesiaType"`
	ContrastVolumeMl         *float64   `json:"contrastVolumeMl"`
	RadiationDoseGy          *float64   `json:"radiationDoseGy"`
	FluoroscopyTimeMinutes   *float64   `json:"fluoroscopyTimeMinutes"`
	LesionsTreated           *string    `json:"lesionsTreated"`
	StentsPlaced             *string    `json:"stentsPlaced"`
	TimiFlowPost             *int16     `json:"timiFlowPost"`
	Complications            string     `json:"complications"`
	Notes                    *string    `json:"notes"`
	TenantID                 string     `json:"tenantId"`
	ScheduledAt              *time.Time `json:"scheduledAt"`
	StartedAt                *time.Time `json:"startedAt"`
	CompletedAt              *time.Time `json:"completedAt"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
}

type CathPostCare struct {
	ID                       string     `json:"id"`
	ProcedureID              string     `json:"procedureId"`
	NurseHpi                 string     `json:"nurseHpi"`
	Haematoma                string     `json:"haematoma"`
	NeurovascularStatus      string     `json:"neurovascularStatus"`
	SystolicBp               *int16     `json:"systolicBp"`
	DiastolicBp              *int16     `json:"diastolicBp"`
	HeartRateBpm             *int16     `json:"heartRateBpm"`
	SpO2Percent              *int16     `json:"spO2Percent"`
	EcgChanges               bool       `json:"ecgChanges"`
	AnticoagulationReversed  bool       `json:"anticoagulationReversed"`
	SheathRemoved            bool       `json:"sheathRemoved"`
	SheathRemovedAt          *time.Time `json:"sheathRemovedAt"`
	Notes                    *string    `json:"notes"`
	TenantID                 string     `json:"tenantId"`
	AssessedAt               time.Time  `json:"assessedAt"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
}

const cathSelectCols = `id, patient_nhi, operator_clinician_hpi, procedure_type, status, indication,
       access_site, anaesthesia_type, contrast_volume_ml, radiation_dose_gy, fluoroscopy_time_minutes,
       lesions_treated, stents_placed, timi_flow_post, complications, notes,
       tenant_id, scheduled_at, started_at, completed_at, created_at, updated_at`

func scanCath(row interface{ Scan(...any) error }, c *CathProcedure) error {
	return row.Scan(
		&c.ID, &c.PatientNHI, &c.OperatorClinicianHpi, &c.ProcedureType, &c.Status, &c.Indication,
		&c.AccessSite, &c.AnaesthesiaType, &c.ContrastVolumeMl, &c.RadiationDoseGy, &c.FluoroscopyTimeMinutes,
		&c.LesionsTreated, &c.StentsPlaced, &c.TimiFlowPost, &c.Complications, &c.Notes,
		&c.TenantID, &c.ScheduledAt, &c.StartedAt, &c.CompletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
}

type cathHandler struct{ handlerDeps }

func (h *cathHandler) List(w http.ResponseWriter, r *http.Request) {
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
			`SELECT `+cathSelectCols+` FROM cath_procedures WHERE tenant_id = @tenant_id AND status = @status ORDER BY scheduled_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+cathSelectCols+` FROM cath_procedures WHERE tenant_id = @tenant_id ORDER BY scheduled_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	procs := make([]CathProcedure, 0)
	for rows.Next() {
		var c CathProcedure
		if err := scanCath(rows, &c); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(c.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		c.PatientNHI = nhi
		procs = append(procs, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, procs)
}

func (h *cathHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req CathProcedure
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ProcedureType == "" {
		req.ProcedureType = "coronary-angiogram"
	}
	if req.AccessSite == "" {
		req.AccessSite = "radial-arterial"
	}
	if req.AnaesthesiaType == "" {
		req.AnaesthesiaType = "local"
	}
	if req.Complications == "" {
		req.Complications = "none"
	}
	if !h.validateHPI(w, r, req.OperatorClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var c CathProcedure
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO cath_procedures
		    (patient_nhi, operator_clinician_hpi, procedure_type, status, indication,
		     access_site, anaesthesia_type, contrast_volume_ml, radiation_dose_gy, fluoroscopy_time_minutes,
		     lesions_treated, stents_placed, timi_flow_post, complications, notes,
		     tenant_id, scheduled_at)
		VALUES
		    (@patient_nhi, @operator_clinician_hpi, @procedure_type, 'booked', @indication,
		     @access_site, @anaesthesia_type, @contrast_volume_ml, @radiation_dose_gy, @fluoroscopy_time_minutes,
		     @lesions_treated, @stents_placed, @timi_flow_post, @complications, @notes,
		     @tenant_id, @scheduled_at)
		RETURNING `+cathSelectCols,
		pgx.NamedArgs{
			"patient_nhi":              nhiEnc,
			"operator_clinician_hpi":   req.OperatorClinicianHpi,
			"procedure_type":           req.ProcedureType,
			"indication":               req.Indication,
			"access_site":              req.AccessSite,
			"anaesthesia_type":         req.AnaesthesiaType,
			"contrast_volume_ml":       req.ContrastVolumeMl,
			"radiation_dose_gy":        req.RadiationDoseGy,
			"fluoroscopy_time_minutes": req.FluoroscopyTimeMinutes,
			"lesions_treated":          req.LesionsTreated,
			"stents_placed":            req.StentsPlaced,
			"timi_flow_post":           req.TimiFlowPost,
			"complications":            req.Complications,
			"notes":                    req.Notes,
			"tenant_id":                tenantID,
			"scheduled_at":             req.ScheduledAt,
		}).Scan(
		&c.ID, &c.PatientNHI, &c.OperatorClinicianHpi, &c.ProcedureType, &c.Status, &c.Indication,
		&c.AccessSite, &c.AnaesthesiaType, &c.ContrastVolumeMl, &c.RadiationDoseGy, &c.FluoroscopyTimeMinutes,
		&c.LesionsTreated, &c.StentsPlaced, &c.TimiFlowPost, &c.Complications, &c.Notes,
		&c.TenantID, &c.ScheduledAt, &c.StartedAt, &c.CompletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	h.recordAudit(r, "create", "CathLabBooking", c.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, c)
}

func (h *cathHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var c CathProcedure
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+cathSelectCols+` FROM cath_procedures WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&c.ID, &c.PatientNHI, &c.OperatorClinicianHpi, &c.ProcedureType, &c.Status, &c.Indication,
		&c.AccessSite, &c.AnaesthesiaType, &c.ContrastVolumeMl, &c.RadiationDoseGy, &c.FluoroscopyTimeMinutes,
		&c.LesionsTreated, &c.StentsPlaced, &c.TimiFlowPost, &c.Complications, &c.Notes,
		&c.TenantID, &c.ScheduledAt, &c.StartedAt, &c.CompletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "cath procedure not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	writeJSON(w, http.StatusOK, c)
}

func (h *cathHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req CathProcedure
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.OperatorClinicianHpi) {
		return
	}
	var c CathProcedure
	err := h.pool.QueryRow(r.Context(), `
		UPDATE cath_procedures
		SET operator_clinician_hpi    = @operator_clinician_hpi,
		    status                    = @status,
		    access_site               = @access_site,
		    anaesthesia_type          = @anaesthesia_type,
		    contrast_volume_ml        = @contrast_volume_ml,
		    radiation_dose_gy         = @radiation_dose_gy,
		    fluoroscopy_time_minutes  = @fluoroscopy_time_minutes,
		    lesions_treated           = @lesions_treated,
		    stents_placed             = @stents_placed,
		    timi_flow_post            = @timi_flow_post,
		    complications             = @complications,
		    notes                     = @notes,
		    updated_at                = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+cathSelectCols,
		pgx.NamedArgs{
			"operator_clinician_hpi":   req.OperatorClinicianHpi,
			"status":                   req.Status,
			"access_site":              req.AccessSite,
			"anaesthesia_type":         req.AnaesthesiaType,
			"contrast_volume_ml":       req.ContrastVolumeMl,
			"radiation_dose_gy":        req.RadiationDoseGy,
			"fluoroscopy_time_minutes": req.FluoroscopyTimeMinutes,
			"lesions_treated":          req.LesionsTreated,
			"stents_placed":            req.StentsPlaced,
			"timi_flow_post":           req.TimiFlowPost,
			"complications":            req.Complications,
			"notes":                    req.Notes,
			"id":                       id,
			"tenant_id":                tenantID,
		}).Scan(
		&c.ID, &c.PatientNHI, &c.OperatorClinicianHpi, &c.ProcedureType, &c.Status, &c.Indication,
		&c.AccessSite, &c.AnaesthesiaType, &c.ContrastVolumeMl, &c.RadiationDoseGy, &c.FluoroscopyTimeMinutes,
		&c.LesionsTreated, &c.StentsPlaced, &c.TimiFlowPost, &c.Complications, &c.Notes,
		&c.TenantID, &c.ScheduledAt, &c.StartedAt, &c.CompletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "cath procedure not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhiEnc := c.PatientNHI
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	h.recordAudit(r, "update", "CathLabBooking", c.ID, nhiEnc)
	writeJSON(w, http.StatusOK, c)
}

func (h *cathHandler) Start(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE cath_procedures
		SET status = 'in-progress', started_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'booked'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "procedure not found or not in booked state"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "in-progress"})
}

func (h *cathHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE cath_procedures
		SET status = 'completed', completed_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'in-progress'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "procedure not found or not in-progress"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (h *cathHandler) GetPostCare(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	procedureID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, procedure_id, nurse_hpi, haematoma, neurovascular_status,
		       systolic_bp, diastolic_bp, heart_rate_bpm, sp_o2_percent,
		       ecg_changes, anticoagulation_reversed, sheath_removed, sheath_removed_at,
		       notes, tenant_id, assessed_at, created_at, updated_at
		FROM cath_post_care WHERE procedure_id = @procedure_id AND tenant_id = @tenant_id
		ORDER BY assessed_at DESC
	`, pgx.NamedArgs{"procedure_id": procedureID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	records := make([]CathPostCare, 0)
	for rows.Next() {
		var pc CathPostCare
		if err := rows.Scan(
			&pc.ID, &pc.ProcedureID, &pc.NurseHpi, &pc.Haematoma, &pc.NeurovascularStatus,
			&pc.SystolicBp, &pc.DiastolicBp, &pc.HeartRateBpm, &pc.SpO2Percent,
			&pc.EcgChanges, &pc.AnticoagulationReversed, &pc.SheathRemoved, &pc.SheathRemovedAt,
			&pc.Notes, &pc.TenantID, &pc.AssessedAt, &pc.CreatedAt, &pc.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		records = append(records, pc)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *cathHandler) AddPostCare(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	procedureID := r.PathValue("id")
	var req CathPostCare
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Haematoma == "" {
		req.Haematoma = "none"
	}
	if req.NeurovascularStatus == "" {
		req.NeurovascularStatus = "normal"
	}
	if !h.validateHPI(w, r, req.NurseHpi) {
		return
	}
	var pc CathPostCare
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO cath_post_care
		    (procedure_id, nurse_hpi, haematoma, neurovascular_status,
		     systolic_bp, diastolic_bp, heart_rate_bpm, sp_o2_percent,
		     ecg_changes, anticoagulation_reversed, sheath_removed, sheath_removed_at,
		     notes, tenant_id, assessed_at)
		VALUES
		    (@procedure_id, @nurse_hpi, @haematoma, @neurovascular_status,
		     @systolic_bp, @diastolic_bp, @heart_rate_bpm, @sp_o2_percent,
		     @ecg_changes, @anticoagulation_reversed, @sheath_removed, @sheath_removed_at,
		     @notes, @tenant_id, COALESCE(@assessed_at, now()))
		RETURNING id, procedure_id, nurse_hpi, haematoma, neurovascular_status,
		          systolic_bp, diastolic_bp, heart_rate_bpm, sp_o2_percent,
		          ecg_changes, anticoagulation_reversed, sheath_removed, sheath_removed_at,
		          notes, tenant_id, assessed_at, created_at, updated_at
	`, pgx.NamedArgs{
		"procedure_id":            procedureID,
		"nurse_hpi":               req.NurseHpi,
		"haematoma":               req.Haematoma,
		"neurovascular_status":    req.NeurovascularStatus,
		"systolic_bp":             req.SystolicBp,
		"diastolic_bp":            req.DiastolicBp,
		"heart_rate_bpm":          req.HeartRateBpm,
		"sp_o2_percent":           req.SpO2Percent,
		"ecg_changes":             req.EcgChanges,
		"anticoagulation_reversed": req.AnticoagulationReversed,
		"sheath_removed":          req.SheathRemoved,
		"sheath_removed_at":       req.SheathRemovedAt,
		"notes":                   req.Notes,
		"tenant_id":               tenantID,
		"assessed_at":             req.AssessedAt,
	}).Scan(
		&pc.ID, &pc.ProcedureID, &pc.NurseHpi, &pc.Haematoma, &pc.NeurovascularStatus,
		&pc.SystolicBp, &pc.DiastolicBp, &pc.HeartRateBpm, &pc.SpO2Percent,
		&pc.EcgChanges, &pc.AnticoagulationReversed, &pc.SheathRemoved, &pc.SheathRemovedAt,
		&pc.Notes, &pc.TenantID, &pc.AssessedAt, &pc.CreatedAt, &pc.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "CathPostCare", pc.ID, "")
	writeJSON(w, http.StatusCreated, pc)
}
