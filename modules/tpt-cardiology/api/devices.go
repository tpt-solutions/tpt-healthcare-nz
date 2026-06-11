package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

type ImplantableDevice struct {
	ID                       string     `json:"id"`
	PatientNHI               string     `json:"patientNhi"`
	ImplantingClinicianHpi   string     `json:"implantingClinicianHpi"`
	FollowUpClinicianHpi     string     `json:"followUpClinicianHpi"`
	DeviceType               string     `json:"deviceType"`
	DeviceBrand              string     `json:"deviceBrand"`
	ModelName                string     `json:"modelName"`
	SerialNumber             string     `json:"serialNumber"`
	Status                   string     `json:"status"`
	Indication               string     `json:"indication"`
	RvLeadImpedanceOhm       *int       `json:"rvLeadImpedanceOhm"`
	RvPacingThresholdV       *float64   `json:"rvPacingThresholdV"`
	RvSensedAmplitudeMv      *float64   `json:"rvSensedAmplitudeMv"`
	LvLeadImpedanceOhm       *int       `json:"lvLeadImpedanceOhm"`
	LvPacingThresholdV       *float64   `json:"lvPacingThresholdV"`
	LvSensedAmplitudeMv      *float64   `json:"lvSensedAmplitudeMv"`
	RaLeadImpedanceOhm       *int       `json:"raLeadImpedanceOhm"`
	RaPacingThresholdV       *float64   `json:"raPacingThresholdV"`
	RaSensedAmplitudeMv      *float64   `json:"raSensedAmplitudeMv"`
	BatteryVoltage           *float64   `json:"batteryVoltage"`
	EstimatedLongevityMonths *int16     `json:"estimatedLongevityMonths"`
	Notes                    *string    `json:"notes"`
	TenantID                 string     `json:"tenantId"`
	ImplantedAt              *time.Time `json:"implantedAt"`
	NextFollowUpAt           *time.Time `json:"nextFollowUpAt"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
}

type DeviceInterrogation struct {
	ID                       string     `json:"id"`
	DeviceID                 string     `json:"deviceId"`
	InterrogatingClinicianHpi string    `json:"interrogatingClinicianHpi"`
	BatteryStatus            string     `json:"batteryStatus"`
	BatteryVoltage           *float64   `json:"batteryVoltage"`
	EstimatedLongevityMonths *int16     `json:"estimatedLongevityMonths"`
	PercentVPaced            *float64   `json:"percentVPaced"`
	PercentAPaced            *float64   `json:"percentAPaced"`
	AfBurdenPercent          *float64   `json:"afBurdenPercent"`
	VtEpisodes               *int       `json:"vtEpisodes"`
	VfEpisodes               *int       `json:"vfEpisodes"`
	ShockTherapyDelivered    bool       `json:"shockTherapyDelivered"`
	ShockCount               *int16     `json:"shockCount"`
	AtpEpisodes              *int       `json:"atpEpisodes"`
	ProgrammeChanges         string     `json:"programmeChanges"`
	ClinicalNotes            string     `json:"clinicalNotes"`
	TenantID                 string     `json:"tenantId"`
	InterrogatedAt           time.Time  `json:"interrogatedAt"`
	NextInterrogationAt      *time.Time `json:"nextInterrogationAt"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
}

const deviceSelectCols = `id, patient_nhi, implanting_clinician_hpi, follow_up_clinician_hpi,
       device_type, device_brand, model_name, serial_number, status, indication,
       rv_lead_impedance_ohm, rv_pacing_threshold_v, rv_sensed_amplitude_mv,
       lv_lead_impedance_ohm, lv_pacing_threshold_v, lv_sensed_amplitude_mv,
       ra_lead_impedance_ohm, ra_pacing_threshold_v, ra_sensed_amplitude_mv,
       battery_voltage, estimated_longevity_months,
       notes, tenant_id, implanted_at, next_follow_up_at, created_at, updated_at`

func scanDevice(row interface{ Scan(...any) error }, d *ImplantableDevice) error {
	return row.Scan(
		&d.ID, &d.PatientNHI, &d.ImplantingClinicianHpi, &d.FollowUpClinicianHpi,
		&d.DeviceType, &d.DeviceBrand, &d.ModelName, &d.SerialNumber, &d.Status, &d.Indication,
		&d.RvLeadImpedanceOhm, &d.RvPacingThresholdV, &d.RvSensedAmplitudeMv,
		&d.LvLeadImpedanceOhm, &d.LvPacingThresholdV, &d.LvSensedAmplitudeMv,
		&d.RaLeadImpedanceOhm, &d.RaPacingThresholdV, &d.RaSensedAmplitudeMv,
		&d.BatteryVoltage, &d.EstimatedLongevityMonths,
		&d.Notes, &d.TenantID, &d.ImplantedAt, &d.NextFollowUpAt, &d.CreatedAt, &d.UpdatedAt,
	)
}

type deviceHandler struct{ handlerDeps }

func (h *deviceHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+deviceSelectCols+` FROM implantable_devices WHERE tenant_id = @tenant_id ORDER BY created_at DESC`,
		pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	devices := make([]ImplantableDevice, 0)
	for rows.Next() {
		var d ImplantableDevice
		if err := scanDevice(rows, &d); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(d.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		d.PatientNHI = nhi
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (h *deviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req ImplantableDevice
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ImplantingClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var d ImplantableDevice
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO implantable_devices
		    (patient_nhi, implanting_clinician_hpi, follow_up_clinician_hpi,
		     device_type, device_brand, model_name, serial_number, status, indication,
		     rv_lead_impedance_ohm, rv_pacing_threshold_v, rv_sensed_amplitude_mv,
		     lv_lead_impedance_ohm, lv_pacing_threshold_v, lv_sensed_amplitude_mv,
		     ra_lead_impedance_ohm, ra_pacing_threshold_v, ra_sensed_amplitude_mv,
		     battery_voltage, estimated_longevity_months,
		     notes, tenant_id, implanted_at, next_follow_up_at)
		VALUES
		    (@patient_nhi, @implanting_clinician_hpi, @follow_up_clinician_hpi,
		     @device_type, @device_brand, @model_name, @serial_number, 'active', @indication,
		     @rv_lead_impedance_ohm, @rv_pacing_threshold_v, @rv_sensed_amplitude_mv,
		     @lv_lead_impedance_ohm, @lv_pacing_threshold_v, @lv_sensed_amplitude_mv,
		     @ra_lead_impedance_ohm, @ra_pacing_threshold_v, @ra_sensed_amplitude_mv,
		     @battery_voltage, @estimated_longevity_months,
		     @notes, @tenant_id, @implanted_at, @next_follow_up_at)
		RETURNING `+deviceSelectCols,
		pgx.NamedArgs{
			"patient_nhi":                nhiEnc,
			"implanting_clinician_hpi":   req.ImplantingClinicianHpi,
			"follow_up_clinician_hpi":    req.FollowUpClinicianHpi,
			"device_type":                req.DeviceType,
			"device_brand":               req.DeviceBrand,
			"model_name":                 req.ModelName,
			"serial_number":              req.SerialNumber,
			"indication":                 req.Indication,
			"rv_lead_impedance_ohm":      req.RvLeadImpedanceOhm,
			"rv_pacing_threshold_v":      req.RvPacingThresholdV,
			"rv_sensed_amplitude_mv":     req.RvSensedAmplitudeMv,
			"lv_lead_impedance_ohm":      req.LvLeadImpedanceOhm,
			"lv_pacing_threshold_v":      req.LvPacingThresholdV,
			"lv_sensed_amplitude_mv":     req.LvSensedAmplitudeMv,
			"ra_lead_impedance_ohm":      req.RaLeadImpedanceOhm,
			"ra_pacing_threshold_v":      req.RaPacingThresholdV,
			"ra_sensed_amplitude_mv":     req.RaSensedAmplitudeMv,
			"battery_voltage":            req.BatteryVoltage,
			"estimated_longevity_months": req.EstimatedLongevityMonths,
			"notes":                      req.Notes,
			"tenant_id":                  tenantID,
			"implanted_at":               req.ImplantedAt,
			"next_follow_up_at":          req.NextFollowUpAt,
		}).Scan(
		&d.ID, &d.PatientNHI, &d.ImplantingClinicianHpi, &d.FollowUpClinicianHpi,
		&d.DeviceType, &d.DeviceBrand, &d.ModelName, &d.SerialNumber, &d.Status, &d.Indication,
		&d.RvLeadImpedanceOhm, &d.RvPacingThresholdV, &d.RvSensedAmplitudeMv,
		&d.LvLeadImpedanceOhm, &d.LvPacingThresholdV, &d.LvSensedAmplitudeMv,
		&d.RaLeadImpedanceOhm, &d.RaPacingThresholdV, &d.RaSensedAmplitudeMv,
		&d.BatteryVoltage, &d.EstimatedLongevityMonths,
		&d.Notes, &d.TenantID, &d.ImplantedAt, &d.NextFollowUpAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(d.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	d.PatientNHI = nhi
	h.recordAudit(r, "create", "ImplantableDevice", d.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, d)
}

func (h *deviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var d ImplantableDevice
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+deviceSelectCols+` FROM implantable_devices WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&d.ID, &d.PatientNHI, &d.ImplantingClinicianHpi, &d.FollowUpClinicianHpi,
		&d.DeviceType, &d.DeviceBrand, &d.ModelName, &d.SerialNumber, &d.Status, &d.Indication,
		&d.RvLeadImpedanceOhm, &d.RvPacingThresholdV, &d.RvSensedAmplitudeMv,
		&d.LvLeadImpedanceOhm, &d.LvPacingThresholdV, &d.LvSensedAmplitudeMv,
		&d.RaLeadImpedanceOhm, &d.RaPacingThresholdV, &d.RaSensedAmplitudeMv,
		&d.BatteryVoltage, &d.EstimatedLongevityMonths,
		&d.Notes, &d.TenantID, &d.ImplantedAt, &d.NextFollowUpAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "device not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(d.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	d.PatientNHI = nhi
	writeJSON(w, http.StatusOK, d)
}

func (h *deviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req ImplantableDevice
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.FollowUpClinicianHpi) {
		return
	}
	var d ImplantableDevice
	err := h.pool.QueryRow(r.Context(), `
		UPDATE implantable_devices
		SET follow_up_clinician_hpi    = @follow_up_clinician_hpi,
		    status                     = @status,
		    rv_lead_impedance_ohm      = @rv_lead_impedance_ohm,
		    rv_pacing_threshold_v      = @rv_pacing_threshold_v,
		    rv_sensed_amplitude_mv     = @rv_sensed_amplitude_mv,
		    lv_lead_impedance_ohm      = @lv_lead_impedance_ohm,
		    lv_pacing_threshold_v      = @lv_pacing_threshold_v,
		    lv_sensed_amplitude_mv     = @lv_sensed_amplitude_mv,
		    ra_lead_impedance_ohm      = @ra_lead_impedance_ohm,
		    ra_pacing_threshold_v      = @ra_pacing_threshold_v,
		    ra_sensed_amplitude_mv     = @ra_sensed_amplitude_mv,
		    battery_voltage            = @battery_voltage,
		    estimated_longevity_months = @estimated_longevity_months,
		    next_follow_up_at         = @next_follow_up_at,
		    notes                     = @notes,
		    updated_at                = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+deviceSelectCols,
		pgx.NamedArgs{
			"follow_up_clinician_hpi":    req.FollowUpClinicianHpi,
			"status":                     req.Status,
			"rv_lead_impedance_ohm":      req.RvLeadImpedanceOhm,
			"rv_pacing_threshold_v":      req.RvPacingThresholdV,
			"rv_sensed_amplitude_mv":     req.RvSensedAmplitudeMv,
			"lv_lead_impedance_ohm":      req.LvLeadImpedanceOhm,
			"lv_pacing_threshold_v":      req.LvPacingThresholdV,
			"lv_sensed_amplitude_mv":     req.LvSensedAmplitudeMv,
			"ra_lead_impedance_ohm":      req.RaLeadImpedanceOhm,
			"ra_pacing_threshold_v":      req.RaPacingThresholdV,
			"ra_sensed_amplitude_mv":     req.RaSensedAmplitudeMv,
			"battery_voltage":            req.BatteryVoltage,
			"estimated_longevity_months": req.EstimatedLongevityMonths,
			"next_follow_up_at":          req.NextFollowUpAt,
			"notes":                      req.Notes,
			"id":                         id,
			"tenant_id":                  tenantID,
		}).Scan(
		&d.ID, &d.PatientNHI, &d.ImplantingClinicianHpi, &d.FollowUpClinicianHpi,
		&d.DeviceType, &d.DeviceBrand, &d.ModelName, &d.SerialNumber, &d.Status, &d.Indication,
		&d.RvLeadImpedanceOhm, &d.RvPacingThresholdV, &d.RvSensedAmplitudeMv,
		&d.LvLeadImpedanceOhm, &d.LvPacingThresholdV, &d.LvSensedAmplitudeMv,
		&d.RaLeadImpedanceOhm, &d.RaPacingThresholdV, &d.RaSensedAmplitudeMv,
		&d.BatteryVoltage, &d.EstimatedLongevityMonths,
		&d.Notes, &d.TenantID, &d.ImplantedAt, &d.NextFollowUpAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "device not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhiEnc := d.PatientNHI
	nhi, err := h.decryptNHI(d.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	d.PatientNHI = nhi
	h.recordAudit(r, "update", "ImplantableDevice", d.ID, nhiEnc)
	writeJSON(w, http.StatusOK, d)
}

func (h *deviceHandler) ListInterrogations(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	deviceID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, device_id, interrogating_clinician_hpi, battery_status,
		       battery_voltage, estimated_longevity_months,
		       percent_v_paced, percent_a_paced, af_burden_percent,
		       vt_episodes, vf_episodes, shock_therapy_delivered, shock_count, atp_episodes,
		       programme_changes, clinical_notes,
		       tenant_id, interrogated_at, next_interrogation_at, created_at, updated_at
		FROM device_interrogations
		WHERE device_id = @device_id AND tenant_id = @tenant_id
		ORDER BY interrogated_at DESC
	`, pgx.NamedArgs{"device_id": deviceID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	interrogations := make([]DeviceInterrogation, 0)
	for rows.Next() {
		var i DeviceInterrogation
		if err := rows.Scan(
			&i.ID, &i.DeviceID, &i.InterrogatingClinicianHpi, &i.BatteryStatus,
			&i.BatteryVoltage, &i.EstimatedLongevityMonths,
			&i.PercentVPaced, &i.PercentAPaced, &i.AfBurdenPercent,
			&i.VtEpisodes, &i.VfEpisodes, &i.ShockTherapyDelivered, &i.ShockCount, &i.AtpEpisodes,
			&i.ProgrammeChanges, &i.ClinicalNotes,
			&i.TenantID, &i.InterrogatedAt, &i.NextInterrogationAt, &i.CreatedAt, &i.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		interrogations = append(interrogations, i)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, interrogations)
}

func (h *deviceHandler) CreateInterrogation(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	deviceID := r.PathValue("id")
	var req DeviceInterrogation
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.InterrogatingClinicianHpi) {
		return
	}
	if req.BatteryStatus == "" {
		req.BatteryStatus = "adequate"
	}
	var i DeviceInterrogation
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO device_interrogations
		    (device_id, interrogating_clinician_hpi, battery_status,
		     battery_voltage, estimated_longevity_months,
		     percent_v_paced, percent_a_paced, af_burden_percent,
		     vt_episodes, vf_episodes, shock_therapy_delivered, shock_count, atp_episodes,
		     programme_changes, clinical_notes,
		     tenant_id, interrogated_at, next_interrogation_at)
		VALUES
		    (@device_id, @interrogating_clinician_hpi, @battery_status,
		     @battery_voltage, @estimated_longevity_months,
		     @percent_v_paced, @percent_a_paced, @af_burden_percent,
		     @vt_episodes, @vf_episodes, @shock_therapy_delivered, @shock_count, @atp_episodes,
		     @programme_changes, @clinical_notes,
		     @tenant_id, COALESCE(@interrogated_at, now()), @next_interrogation_at)
		RETURNING id, device_id, interrogating_clinician_hpi, battery_status,
		          battery_voltage, estimated_longevity_months,
		          percent_v_paced, percent_a_paced, af_burden_percent,
		          vt_episodes, vf_episodes, shock_therapy_delivered, shock_count, atp_episodes,
		          programme_changes, clinical_notes,
		          tenant_id, interrogated_at, next_interrogation_at, created_at, updated_at
	`, pgx.NamedArgs{
		"device_id":                  deviceID,
		"interrogating_clinician_hpi": req.InterrogatingClinicianHpi,
		"battery_status":             req.BatteryStatus,
		"battery_voltage":            req.BatteryVoltage,
		"estimated_longevity_months": req.EstimatedLongevityMonths,
		"percent_v_paced":            req.PercentVPaced,
		"percent_a_paced":            req.PercentAPaced,
		"af_burden_percent":          req.AfBurdenPercent,
		"vt_episodes":                req.VtEpisodes,
		"vf_episodes":                req.VfEpisodes,
		"shock_therapy_delivered":    req.ShockTherapyDelivered,
		"shock_count":                req.ShockCount,
		"atp_episodes":               req.AtpEpisodes,
		"programme_changes":          req.ProgrammeChanges,
		"clinical_notes":             req.ClinicalNotes,
		"tenant_id":                  tenantID,
		"interrogated_at":            req.InterrogatedAt,
		"next_interrogation_at":      req.NextInterrogationAt,
	}).Scan(
		&i.ID, &i.DeviceID, &i.InterrogatingClinicianHpi, &i.BatteryStatus,
		&i.BatteryVoltage, &i.EstimatedLongevityMonths,
		&i.PercentVPaced, &i.PercentAPaced, &i.AfBurdenPercent,
		&i.VtEpisodes, &i.VfEpisodes, &i.ShockTherapyDelivered, &i.ShockCount, &i.AtpEpisodes,
		&i.ProgrammeChanges, &i.ClinicalNotes,
		&i.TenantID, &i.InterrogatedAt, &i.NextInterrogationAt, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "ImplantableDevice", i.ID, "")
	writeJSON(w, http.StatusCreated, i)
}

func (h *deviceHandler) GetInterrogation(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	interrogationID := r.PathValue("interrogationId")
	var i DeviceInterrogation
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, device_id, interrogating_clinician_hpi, battery_status,
		       battery_voltage, estimated_longevity_months,
		       percent_v_paced, percent_a_paced, af_burden_percent,
		       vt_episodes, vf_episodes, shock_therapy_delivered, shock_count, atp_episodes,
		       programme_changes, clinical_notes,
		       tenant_id, interrogated_at, next_interrogation_at, created_at, updated_at
		FROM device_interrogations WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": interrogationID, "tenant_id": tenantID}).Scan(
		&i.ID, &i.DeviceID, &i.InterrogatingClinicianHpi, &i.BatteryStatus,
		&i.BatteryVoltage, &i.EstimatedLongevityMonths,
		&i.PercentVPaced, &i.PercentAPaced, &i.AfBurdenPercent,
		&i.VtEpisodes, &i.VfEpisodes, &i.ShockTherapyDelivered, &i.ShockCount, &i.AtpEpisodes,
		&i.ProgrammeChanges, &i.ClinicalNotes,
		&i.TenantID, &i.InterrogatedAt, &i.NextInterrogationAt, &i.CreatedAt, &i.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "interrogation not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, i)
}
