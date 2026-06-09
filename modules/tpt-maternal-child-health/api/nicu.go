package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// NICUStatus tracks the clinical status of a NICU admission.
type NICUStatus string

const (
	NICUStatusAdmitted          NICUStatus = "admitted"
	NICUStatusStable            NICUStatus = "stable"
	NICUStatusCritical          NICUStatus = "critical"
	NICUStatusDischargePlanning NICUStatus = "discharge-planning"
	NICUStatusDischarged        NICUStatus = "discharged"
	NICUStatusTransferred       NICUStatus = "transferred"
	NICUStatusDeceased          NICUStatus = "deceased"
)

// NICUAdmissionType classifies whether the neonate was born at this facility.
type NICUAdmissionType string

const (
	NICUAdmissionInborn   NICUAdmissionType = "inborn"
	NICUAdmissionOutborn  NICUAdmissionType = "outborn"
	NICUAdmissionTransfer NICUAdmissionType = "transfer"
)

// RespiratorySupport classifies the level of respiratory assistance.
type RespiratorySupport string

const (
	RespSupportNone             RespiratorySupport = "none"
	RespSupportHFNC             RespiratorySupport = "HFNC"
	RespSupportCPAP             RespiratorySupport = "CPAP"
	RespSupportHFOV             RespiratorySupport = "HFOV"
	RespSupportConventionalVent RespiratorySupport = "conventional-vent"
)

// VentMode classifies the ventilation strategy in use.
type VentMode string

const (
	VentModeNoSupport VentMode = "no-support"
	VentModeHFNC      VentMode = "HFNC"
	VentModeCPAP      VentMode = "CPAP"
	VentModeSIMV      VentMode = "SIMV"
	VentModeACPC      VentMode = "AC-PC"
	VentModeHFOV      VentMode = "HFOV"
)

type NICUAdmission struct {
	ID                    string     `json:"id"`
	MaternityEpisodeID    *string    `json:"maternityEpisodeId"`
	PatientNHI            string     `json:"patientNhi"`
	NeonatologistHpi      string     `json:"neonatologistHpi"`
	Status                string     `json:"status"`
	AdmissionReason       string     `json:"admissionReason"`
	AdmissionType         string     `json:"admissionType"`
	GestationAtBirthWeeks *int16     `json:"gestationAtBirthWeeks"`
	BirthWeightGrams      *int       `json:"birthWeightGrams"`
	CurrentWeightGrams    *int       `json:"currentWeightGrams"`
	CorrectedAgeWeeks     *int16     `json:"correctedAgeWeeks"`
	BedLabel              string     `json:"bedLabel"`
	Apgar1min             *int16     `json:"apgar1min"`
	Apgar5min             *int16     `json:"apgar5min"`
	RespiratorySupport    string     `json:"respiratorySupport"`
	SurfactantGiven       bool       `json:"surfactantGiven"`
	TpnActive             bool       `json:"tpnActive"`
	PhototherapyActive    bool       `json:"phototherapyActive"`
	AntibioticsActive     bool       `json:"antibioticsActive"`
	TenantID              string     `json:"tenantId"`
	AdmittedAt            time.Time  `json:"admittedAt"`
	DischargedAt          *time.Time `json:"dischargedAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type NICUVentilationChart struct {
	ID              string    `json:"id"`
	NicuAdmissionID string    `json:"nicuAdmissionId"`
	ClinicianHpi    string    `json:"clinicianHpi"`
	Mode            string    `json:"mode"`
	Fio2            *float64  `json:"fio2"`
	PeepCmh2o       *float64  `json:"peepCmh2o"`
	PipCmh2o        *float64  `json:"pipCmh2o"`
	MapCmh2o        *float64  `json:"mapCmh2o"`
	AmplitudeCmh2o  *float64  `json:"amplitudeCmh2o"`
	FrequencyHz     *float64  `json:"frequencyHz"`
	TidalVolumeMl   *float64  `json:"tidalVolumeMl"`
	RatePerMin      *int16    `json:"ratePerMin"`
	Spo2Percent     *int16    `json:"spo2Percent"`
	Ph              *float64  `json:"ph"`
	Pco2Mmhg        *float64  `json:"pco2Mmhg"`
	Po2Mmhg         *float64  `json:"po2Mmhg"`
	BaseExcess      *float64  `json:"baseExcess"`
	Hco3Meql        *float64  `json:"hco3Meql"`
	Lactate         *float64  `json:"lactate"`
	BloodGasType    string    `json:"bloodGasType"`
	Notes           *string   `json:"notes"`
	RecordedAt      time.Time `json:"recordedAt"`
	TenantID        string    `json:"tenantId"`
}

type NICUDischargePlan struct {
	ID                        string    `json:"id"`
	NicuAdmissionID           string    `json:"nicuAdmissionId"`
	ClinicianHpi              string    `json:"clinicianHpi"`
	PlannedDischargeDate      *string   `json:"plannedDischargeDate"`
	DischargeDestination      *string   `json:"dischargeDestination"`
	DischargeWeightTargetGrams *int     `json:"dischargeWeightTargetGrams"`
	FeedingPlan               *string   `json:"feedingPlan"`
	Medications               *string   `json:"medications"`
	FollowUpAppointments      *string   `json:"followUpAppointments"`
	CarSeatOrganised          bool      `json:"carSeatOrganised"`
	HomeOxygenRequired        bool      `json:"homeOxygenRequired"`
	ApnoeaMonitorRequired     bool      `json:"apnoeaMonitorRequired"`
	CommunityNurseReferral    bool      `json:"communityNurseReferral"`
	Notes                     *string   `json:"notes"`
	TenantID                  string    `json:"tenantId"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// nicuHandler manages NICU admissions, ventilation charting, and discharge planning.
type nicuHandler struct {
	handlerDeps
}

func (h *nicuHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(), `
			SELECT id, maternity_episode_id, patient_nhi, neonatologist_hpi, status,
			       admission_reason, admission_type, gestation_at_birth_weeks, birth_weight_grams,
			       current_weight_grams, corrected_age_weeks, bed_label, apgar_1min, apgar_5min,
			       respiratory_support, surfactant_given, tpn_active, phototherapy_active, antibiotics_active,
			       tenant_id, admitted_at, discharged_at, created_at, updated_at
			FROM nicu_admissions
			WHERE tenant_id = @tenant_id AND status = @status
			ORDER BY admitted_at DESC
			LIMIT 200
		`, pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(), `
			SELECT id, maternity_episode_id, patient_nhi, neonatologist_hpi, status,
			       admission_reason, admission_type, gestation_at_birth_weeks, birth_weight_grams,
			       current_weight_grams, corrected_age_weeks, bed_label, apgar_1min, apgar_5min,
			       respiratory_support, surfactant_given, tpn_active, phototherapy_active, antibiotics_active,
			       tenant_id, admitted_at, discharged_at, created_at, updated_at
			FROM nicu_admissions
			WHERE tenant_id = @tenant_id
			ORDER BY admitted_at DESC
			LIMIT 200
		`, pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	admissions := make([]NICUAdmission, 0)
	for rows.Next() {
		var a NICUAdmission
		if err := rows.Scan(
			&a.ID, &a.MaternityEpisodeID, &a.PatientNHI, &a.NeonatologistHpi, &a.Status,
			&a.AdmissionReason, &a.AdmissionType, &a.GestationAtBirthWeeks, &a.BirthWeightGrams,
			&a.CurrentWeightGrams, &a.CorrectedAgeWeeks, &a.BedLabel, &a.Apgar1min, &a.Apgar5min,
			&a.RespiratorySupport, &a.SurfactantGiven, &a.TpnActive, &a.PhototherapyActive, &a.AntibioticsActive,
			&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(a.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		a.PatientNHI = nhi
		admissions = append(admissions, a)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, admissions)
}

func (h *nicuHandler) Admit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req NICUAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" || req.NeonatologistHpi == "" || req.AdmissionReason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELDS", Message: "patientNhi, neonatologistHpi, and admissionReason are required"})
		return
	}
	if req.AdmissionType == "" {
		req.AdmissionType = "inborn"
	}
	if req.RespiratorySupport == "" {
		req.RespiratorySupport = "none"
	}
	if !h.validateHPI(w, r, req.NeonatologistHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var a NICUAdmission
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO nicu_admissions
		    (maternity_episode_id, patient_nhi, neonatologist_hpi, status,
		     admission_reason, admission_type, gestation_at_birth_weeks, birth_weight_grams,
		     current_weight_grams, bed_label, apgar_1min, apgar_5min,
		     respiratory_support, surfactant_given, tpn_active, phototherapy_active, antibiotics_active,
		     tenant_id)
		VALUES
		    (@maternity_episode_id, @patient_nhi, @neonatologist_hpi, 'admitted',
		     @admission_reason, @admission_type, @gestation_at_birth_weeks, @birth_weight_grams,
		     @current_weight_grams, @bed_label, @apgar_1min, @apgar_5min,
		     @respiratory_support, @surfactant_given, @tpn_active, @phototherapy_active, @antibiotics_active,
		     @tenant_id)
		RETURNING id, maternity_episode_id, patient_nhi, neonatologist_hpi, status,
		          admission_reason, admission_type, gestation_at_birth_weeks, birth_weight_grams,
		          current_weight_grams, corrected_age_weeks, bed_label, apgar_1min, apgar_5min,
		          respiratory_support, surfactant_given, tpn_active, phototherapy_active, antibiotics_active,
		          tenant_id, admitted_at, discharged_at, created_at, updated_at
	`, pgx.NamedArgs{
		"maternity_episode_id":    req.MaternityEpisodeID,
		"patient_nhi":             nhiEnc,
		"neonatologist_hpi":       req.NeonatologistHpi,
		"admission_reason":        req.AdmissionReason,
		"admission_type":          req.AdmissionType,
		"gestation_at_birth_weeks": req.GestationAtBirthWeeks,
		"birth_weight_grams":      req.BirthWeightGrams,
		"current_weight_grams":    req.CurrentWeightGrams,
		"bed_label":               req.BedLabel,
		"apgar_1min":              req.Apgar1min,
		"apgar_5min":              req.Apgar5min,
		"respiratory_support":     req.RespiratorySupport,
		"surfactant_given":        req.SurfactantGiven,
		"tpn_active":              req.TpnActive,
		"phototherapy_active":     req.PhototherapyActive,
		"antibiotics_active":      req.AntibioticsActive,
		"tenant_id":               tenantID,
	}).Scan(
		&a.ID, &a.MaternityEpisodeID, &a.PatientNHI, &a.NeonatologistHpi, &a.Status,
		&a.AdmissionReason, &a.AdmissionType, &a.GestationAtBirthWeeks, &a.BirthWeightGrams,
		&a.CurrentWeightGrams, &a.CorrectedAgeWeeks, &a.BedLabel, &a.Apgar1min, &a.Apgar5min,
		&a.RespiratorySupport, &a.SurfactantGiven, &a.TpnActive, &a.PhototherapyActive, &a.AntibioticsActive,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	h.recordAudit(r, "create", "NICUAdmission", a.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, a)
}

func (h *nicuHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var a NICUAdmission
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, maternity_episode_id, patient_nhi, neonatologist_hpi, status,
		       admission_reason, admission_type, gestation_at_birth_weeks, birth_weight_grams,
		       current_weight_grams, corrected_age_weeks, bed_label, apgar_1min, apgar_5min,
		       respiratory_support, surfactant_given, tpn_active, phototherapy_active, antibiotics_active,
		       tenant_id, admitted_at, discharged_at, created_at, updated_at
		FROM nicu_admissions
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&a.ID, &a.MaternityEpisodeID, &a.PatientNHI, &a.NeonatologistHpi, &a.Status,
		&a.AdmissionReason, &a.AdmissionType, &a.GestationAtBirthWeeks, &a.BirthWeightGrams,
		&a.CurrentWeightGrams, &a.CorrectedAgeWeeks, &a.BedLabel, &a.Apgar1min, &a.Apgar5min,
		&a.RespiratorySupport, &a.SurfactantGiven, &a.TpnActive, &a.PhototherapyActive, &a.AntibioticsActive,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NICU admission not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	writeJSON(w, http.StatusOK, a)
}

func (h *nicuHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req NICUAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var a NICUAdmission
	err := h.pool.QueryRow(r.Context(), `
		UPDATE nicu_admissions
		SET status = @status,
		    current_weight_grams = @current_weight_grams,
		    corrected_age_weeks = @corrected_age_weeks,
		    respiratory_support = @respiratory_support,
		    surfactant_given = @surfactant_given,
		    tpn_active = @tpn_active,
		    phototherapy_active = @phototherapy_active,
		    antibiotics_active = @antibiotics_active,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, maternity_episode_id, patient_nhi, neonatologist_hpi, status,
		          admission_reason, admission_type, gestation_at_birth_weeks, birth_weight_grams,
		          current_weight_grams, corrected_age_weeks, bed_label, apgar_1min, apgar_5min,
		          respiratory_support, surfactant_given, tpn_active, phototherapy_active, antibiotics_active,
		          tenant_id, admitted_at, discharged_at, created_at, updated_at
	`, pgx.NamedArgs{
		"status":               req.Status,
		"current_weight_grams": req.CurrentWeightGrams,
		"corrected_age_weeks":  req.CorrectedAgeWeeks,
		"respiratory_support":  req.RespiratorySupport,
		"surfactant_given":     req.SurfactantGiven,
		"tpn_active":           req.TpnActive,
		"phototherapy_active":  req.PhototherapyActive,
		"antibiotics_active":   req.AntibioticsActive,
		"id":                   id,
		"tenant_id":            tenantID,
	}).Scan(
		&a.ID, &a.MaternityEpisodeID, &a.PatientNHI, &a.NeonatologistHpi, &a.Status,
		&a.AdmissionReason, &a.AdmissionType, &a.GestationAtBirthWeeks, &a.BirthWeightGrams,
		&a.CurrentWeightGrams, &a.CorrectedAgeWeeks, &a.BedLabel, &a.Apgar1min, &a.Apgar5min,
		&a.RespiratorySupport, &a.SurfactantGiven, &a.TpnActive, &a.PhototherapyActive, &a.AntibioticsActive,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NICU admission not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "NICUAdmission", a.ID, a.PatientNHI)
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	writeJSON(w, http.StatusOK, a)
}

func (h *nicuHandler) Discharge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE nicu_admissions
		SET status = 'discharged', discharged_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status != 'discharged'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NICU admission not found or already discharged"})
		return
	}
	h.recordAudit(r, "delete", "NICUAdmission", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "discharged"})
}

func (h *nicuHandler) ListVentilation(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, nicu_admission_id, clinician_hpi, mode,
		       fio2, peep_cmh2o, pip_cmh2o, map_cmh2o, amplitude_cmh2o, frequency_hz,
		       tidal_volume_ml, rate_per_min, spo2_percent,
		       ph, pco2_mmhg, po2_mmhg, base_excess, hco3_meql, lactate,
		       blood_gas_type, notes, recorded_at, tenant_id
		FROM nicu_ventilation_chart
		WHERE nicu_admission_id = @id AND tenant_id = @tenant_id
		ORDER BY recorded_at DESC
		LIMIT 500
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	charts := make([]NICUVentilationChart, 0)
	for rows.Next() {
		var c NICUVentilationChart
		if err := rows.Scan(
			&c.ID, &c.NicuAdmissionID, &c.ClinicianHpi, &c.Mode,
			&c.Fio2, &c.PeepCmh2o, &c.PipCmh2o, &c.MapCmh2o, &c.AmplitudeCmh2o, &c.FrequencyHz,
			&c.TidalVolumeMl, &c.RatePerMin, &c.Spo2Percent,
			&c.Ph, &c.Pco2Mmhg, &c.Po2Mmhg, &c.BaseExcess, &c.Hco3Meql, &c.Lactate,
			&c.BloodGasType, &c.Notes, &c.RecordedAt, &c.TenantID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		charts = append(charts, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, charts)
}

func (h *nicuHandler) RecordVentilation(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req NICUVentilationChart
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Mode == "" {
		req.Mode = "CPAP"
	}
	if req.BloodGasType == "" {
		req.BloodGasType = "none"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var c NICUVentilationChart
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO nicu_ventilation_chart
		    (nicu_admission_id, clinician_hpi, mode,
		     fio2, peep_cmh2o, pip_cmh2o, map_cmh2o, amplitude_cmh2o, frequency_hz,
		     tidal_volume_ml, rate_per_min, spo2_percent,
		     ph, pco2_mmhg, po2_mmhg, base_excess, hco3_meql, lactate,
		     blood_gas_type, notes, tenant_id)
		VALUES
		    (@nicu_admission_id, @clinician_hpi, @mode,
		     @fio2, @peep_cmh2o, @pip_cmh2o, @map_cmh2o, @amplitude_cmh2o, @frequency_hz,
		     @tidal_volume_ml, @rate_per_min, @spo2_percent,
		     @ph, @pco2_mmhg, @po2_mmhg, @base_excess, @hco3_meql, @lactate,
		     @blood_gas_type, @notes, @tenant_id)
		RETURNING id, nicu_admission_id, clinician_hpi, mode,
		          fio2, peep_cmh2o, pip_cmh2o, map_cmh2o, amplitude_cmh2o, frequency_hz,
		          tidal_volume_ml, rate_per_min, spo2_percent,
		          ph, pco2_mmhg, po2_mmhg, base_excess, hco3_meql, lactate,
		          blood_gas_type, notes, recorded_at, tenant_id
	`, pgx.NamedArgs{
		"nicu_admission_id": id,
		"clinician_hpi":     req.ClinicianHpi,
		"mode":              req.Mode,
		"fio2":              req.Fio2,
		"peep_cmh2o":        req.PeepCmh2o,
		"pip_cmh2o":         req.PipCmh2o,
		"map_cmh2o":         req.MapCmh2o,
		"amplitude_cmh2o":   req.AmplitudeCmh2o,
		"frequency_hz":      req.FrequencyHz,
		"tidal_volume_ml":   req.TidalVolumeMl,
		"rate_per_min":      req.RatePerMin,
		"spo2_percent":      req.Spo2Percent,
		"ph":                req.Ph,
		"pco2_mmhg":         req.Pco2Mmhg,
		"po2_mmhg":          req.Po2Mmhg,
		"base_excess":       req.BaseExcess,
		"hco3_meql":         req.Hco3Meql,
		"lactate":           req.Lactate,
		"blood_gas_type":    req.BloodGasType,
		"notes":             req.Notes,
		"tenant_id":         tenantID,
	}).Scan(
		&c.ID, &c.NicuAdmissionID, &c.ClinicianHpi, &c.Mode,
		&c.Fio2, &c.PeepCmh2o, &c.PipCmh2o, &c.MapCmh2o, &c.AmplitudeCmh2o, &c.FrequencyHz,
		&c.TidalVolumeMl, &c.RatePerMin, &c.Spo2Percent,
		&c.Ph, &c.Pco2Mmhg, &c.Po2Mmhg, &c.BaseExcess, &c.Hco3Meql, &c.Lactate,
		&c.BloodGasType, &c.Notes, &c.RecordedAt, &c.TenantID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "NICUVentilationChart", c.ID, "")
	writeJSON(w, http.StatusCreated, c)
}

func (h *nicuHandler) GetDischargePlan(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var p NICUDischargePlan
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, nicu_admission_id, clinician_hpi, planned_discharge_date::text,
		       discharge_destination, discharge_weight_target_grams, feeding_plan, medications,
		       follow_up_appointments, car_seat_organised, home_oxygen_required,
		       apnoea_monitor_required, community_nurse_referral, notes, tenant_id, created_at, updated_at
		FROM nicu_discharge_plans
		WHERE nicu_admission_id = @id AND tenant_id = @tenant_id
		ORDER BY created_at DESC
		LIMIT 1
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.NicuAdmissionID, &p.ClinicianHpi, &p.PlannedDischargeDate,
		&p.DischargeDestination, &p.DischargeWeightTargetGrams, &p.FeedingPlan, &p.Medications,
		&p.FollowUpAppointments, &p.CarSeatOrganised, &p.HomeOxygenRequired,
		&p.ApnoeaMonitorRequired, &p.CommunityNurseReferral, &p.Notes, &p.TenantID, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "no discharge plan found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *nicuHandler) UpdateDischargePlan(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req NICUDischargePlan
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var p NICUDischargePlan
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO nicu_discharge_plans
		    (nicu_admission_id, clinician_hpi, planned_discharge_date,
		     discharge_destination, discharge_weight_target_grams, feeding_plan, medications,
		     follow_up_appointments, car_seat_organised, home_oxygen_required,
		     apnoea_monitor_required, community_nurse_referral, notes, tenant_id)
		VALUES
		    (@nicu_admission_id, @clinician_hpi, @planned_discharge_date,
		     @discharge_destination, @discharge_weight_target_grams, @feeding_plan, @medications,
		     @follow_up_appointments, @car_seat_organised, @home_oxygen_required,
		     @apnoea_monitor_required, @community_nurse_referral, @notes, @tenant_id)
		ON CONFLICT (nicu_admission_id) DO UPDATE
		SET clinician_hpi = EXCLUDED.clinician_hpi,
		    planned_discharge_date = EXCLUDED.planned_discharge_date,
		    discharge_destination = EXCLUDED.discharge_destination,
		    discharge_weight_target_grams = EXCLUDED.discharge_weight_target_grams,
		    feeding_plan = EXCLUDED.feeding_plan,
		    medications = EXCLUDED.medications,
		    follow_up_appointments = EXCLUDED.follow_up_appointments,
		    car_seat_organised = EXCLUDED.car_seat_organised,
		    home_oxygen_required = EXCLUDED.home_oxygen_required,
		    apnoea_monitor_required = EXCLUDED.apnoea_monitor_required,
		    community_nurse_referral = EXCLUDED.community_nurse_referral,
		    notes = EXCLUDED.notes,
		    updated_at = now()
		RETURNING id, nicu_admission_id, clinician_hpi, planned_discharge_date::text,
		          discharge_destination, discharge_weight_target_grams, feeding_plan, medications,
		          follow_up_appointments, car_seat_organised, home_oxygen_required,
		          apnoea_monitor_required, community_nurse_referral, notes, tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"nicu_admission_id":             id,
		"clinician_hpi":                 req.ClinicianHpi,
		"planned_discharge_date":        req.PlannedDischargeDate,
		"discharge_destination":         req.DischargeDestination,
		"discharge_weight_target_grams": req.DischargeWeightTargetGrams,
		"feeding_plan":                  req.FeedingPlan,
		"medications":                   req.Medications,
		"follow_up_appointments":        req.FollowUpAppointments,
		"car_seat_organised":            req.CarSeatOrganised,
		"home_oxygen_required":          req.HomeOxygenRequired,
		"apnoea_monitor_required":       req.ApnoeaMonitorRequired,
		"community_nurse_referral":      req.CommunityNurseReferral,
		"notes":                         req.Notes,
		"tenant_id":                     tenantID,
	}).Scan(
		&p.ID, &p.NicuAdmissionID, &p.ClinicianHpi, &p.PlannedDischargeDate,
		&p.DischargeDestination, &p.DischargeWeightTargetGrams, &p.FeedingPlan, &p.Medications,
		&p.FollowUpAppointments, &p.CarSeatOrganised, &p.HomeOxygenRequired,
		&p.ApnoeaMonitorRequired, &p.CommunityNurseReferral, &p.Notes, &p.TenantID, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "NICUDischargePlan", p.ID, "")
	writeJSON(w, http.StatusOK, p)
}
