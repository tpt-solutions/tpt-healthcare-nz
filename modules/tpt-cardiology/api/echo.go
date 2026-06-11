package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

type EchoStudy struct {
	ID                       string     `json:"id"`
	PatientNHI               string     `json:"patientNhi"`
	OrderingClinicianHpi     string     `json:"orderingClinicianHpi"`
	ReportingClinicianHpi    string     `json:"reportingClinicianHpi"`
	StudyType                string     `json:"studyType"`
	Status                   string     `json:"status"`
	Indication               string     `json:"indication"`
	LvefPercent              *int16     `json:"lvefPercent"`
	LvEdvMl                  *float64   `json:"lvEdvMl"`
	LvEsvMl                  *float64   `json:"lvEsvMl"`
	LvDiastolicDiameterMm    *float64   `json:"lvDiastolicDiameterMm"`
	LvSystolicDiameterMm     *float64   `json:"lvSystolicDiameterMm"`
	LvPosteriorWallMm        *float64   `json:"lvPosteriorWallMm"`
	InterventricularSeptumMm *float64   `json:"interventricularSeptumMm"`
	LvMassG                  *float64   `json:"lvMassG"`
	WallMotionAbnormality    bool       `json:"wallMotionAbnormality"`
	WallMotionSegments       *string    `json:"wallMotionSegments"`
	DiastolicFunction        string     `json:"diastolicFunction"`
	AorticValveFinding       string     `json:"aorticValveFinding"`
	AorticGradientMmhg       *float64   `json:"aorticGradientMmhg"`
	AorticValveAreaCm2       *float64   `json:"aorticValveAreaCm2"`
	MitralValveFinding       string     `json:"mitralValveFinding"`
	MitralEVelocity          *float64   `json:"mitralEVelocity"`
	MitralAVelocity          *float64   `json:"mitralAVelocity"`
	EARatio                  *float64   `json:"eaRatio"`
	MitralEPrime             *float64   `json:"mitralEPrime"`
	EEPrimeRatio             *float64   `json:"eePrimeRatio"`
	TricuspidValveFinding    string     `json:"tricuspidValveFinding"`
	TvRegurgVelocity         *float64   `json:"tvRegurgVelocity"`
	RvspMmhg                 *float64   `json:"rvspMmhg"`
	PulmonaryValveFinding    string     `json:"pulmonaryValveFinding"`
	RvFunction               string     `json:"rvFunction"`
	PericardialEffusion      string     `json:"pericardialEffusion"`
	IvcDiameterMm            *float64   `json:"ivcDiameterMm"`
	IvcCollapsibility        string     `json:"ivcCollapsibility"`
	LaVolumeMl               *float64   `json:"laVolumeMl"`
	RaAreaCm2                *float64   `json:"raAreaCm2"`
	Interpretation           string     `json:"interpretation"`
	Notes                    *string    `json:"notes"`
	TenantID                 string     `json:"tenantId"`
	OrderedAt                time.Time  `json:"orderedAt"`
	PerformedAt              *time.Time `json:"performedAt"`
	ReportedAt               *time.Time `json:"reportedAt"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
}

const echoSelectCols = `id, patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, study_type, status, indication,
       lvef_percent, lv_edv_ml, lv_esv_ml, lv_diastolic_diameter_mm, lv_systolic_diameter_mm,
       lv_posterior_wall_mm, interventricular_septum_mm, lv_mass_g, wall_motion_abnormality,
       wall_motion_segments, diastolic_function,
       aortic_valve_finding, aortic_gradient_mmhg, aortic_valve_area_cm2,
       mitral_valve_finding, mitral_e_velocity, mitral_a_velocity, e_a_ratio, mitral_e_prime, e_e_prime_ratio,
       tricuspid_valve_finding, tv_regurg_velocity, rvsp_mmhg,
       pulmonary_valve_finding, rv_function, pericardial_effusion,
       ivc_diameter_mm, ivc_collapsibility, la_volume_ml, ra_area_cm2,
       interpretation, notes, tenant_id, ordered_at, performed_at, reported_at, created_at, updated_at`

func scanEcho(row interface{ Scan(...any) error }, e *EchoStudy) error {
	return row.Scan(
		&e.ID, &e.PatientNHI, &e.OrderingClinicianHpi, &e.ReportingClinicianHpi, &e.StudyType, &e.Status, &e.Indication,
		&e.LvefPercent, &e.LvEdvMl, &e.LvEsvMl, &e.LvDiastolicDiameterMm, &e.LvSystolicDiameterMm,
		&e.LvPosteriorWallMm, &e.InterventricularSeptumMm, &e.LvMassG, &e.WallMotionAbnormality,
		&e.WallMotionSegments, &e.DiastolicFunction,
		&e.AorticValveFinding, &e.AorticGradientMmhg, &e.AorticValveAreaCm2,
		&e.MitralValveFinding, &e.MitralEVelocity, &e.MitralAVelocity, &e.EARatio, &e.MitralEPrime, &e.EEPrimeRatio,
		&e.TricuspidValveFinding, &e.TvRegurgVelocity, &e.RvspMmhg,
		&e.PulmonaryValveFinding, &e.RvFunction, &e.PericardialEffusion,
		&e.IvcDiameterMm, &e.IvcCollapsibility, &e.LaVolumeMl, &e.RaAreaCm2,
		&e.Interpretation, &e.Notes, &e.TenantID, &e.OrderedAt, &e.PerformedAt, &e.ReportedAt, &e.CreatedAt, &e.UpdatedAt,
	)
}

type echoHandler struct{ handlerDeps }

func (h *echoHandler) List(w http.ResponseWriter, r *http.Request) {
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
			`SELECT `+echoSelectCols+` FROM echo_studies WHERE tenant_id = @tenant_id AND status = @status ORDER BY ordered_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+echoSelectCols+` FROM echo_studies WHERE tenant_id = @tenant_id ORDER BY ordered_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	studies := make([]EchoStudy, 0)
	for rows.Next() {
		var e EchoStudy
		if err := scanEcho(rows, &e); err != nil {
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

func (h *echoHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req EchoStudy
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.StudyType == "" {
		req.StudyType = "TTE"
	}
	if req.DiastolicFunction == "" {
		req.DiastolicFunction = "normal"
	}
	if req.PericardialEffusion == "" {
		req.PericardialEffusion = "none"
	}
	if req.RvFunction == "" {
		req.RvFunction = "normal"
	}
	if !h.validateHPI(w, r, req.OrderingClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var e EchoStudy
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO echo_studies
		    (patient_nhi, ordering_clinician_hpi, reporting_clinician_hpi, study_type, status, indication,
		     lvef_percent, lv_edv_ml, lv_esv_ml, lv_diastolic_diameter_mm, lv_systolic_diameter_mm,
		     lv_posterior_wall_mm, interventricular_septum_mm, lv_mass_g, wall_motion_abnormality,
		     wall_motion_segments, diastolic_function,
		     aortic_valve_finding, aortic_gradient_mmhg, aortic_valve_area_cm2,
		     mitral_valve_finding, mitral_e_velocity, mitral_a_velocity, e_a_ratio, mitral_e_prime, e_e_prime_ratio,
		     tricuspid_valve_finding, tv_regurg_velocity, rvsp_mmhg,
		     pulmonary_valve_finding, rv_function, pericardial_effusion,
		     ivc_diameter_mm, ivc_collapsibility, la_volume_ml, ra_area_cm2,
		     interpretation, notes, tenant_id)
		VALUES
		    (@patient_nhi, @ordering_clinician_hpi, @reporting_clinician_hpi, @study_type, 'ordered', @indication,
		     @lvef_percent, @lv_edv_ml, @lv_esv_ml, @lv_diastolic_diameter_mm, @lv_systolic_diameter_mm,
		     @lv_posterior_wall_mm, @interventricular_septum_mm, @lv_mass_g, @wall_motion_abnormality,
		     @wall_motion_segments, @diastolic_function,
		     @aortic_valve_finding, @aortic_gradient_mmhg, @aortic_valve_area_cm2,
		     @mitral_valve_finding, @mitral_e_velocity, @mitral_a_velocity, @e_a_ratio, @mitral_e_prime, @e_e_prime_ratio,
		     @tricuspid_valve_finding, @tv_regurg_velocity, @rvsp_mmhg,
		     @pulmonary_valve_finding, @rv_function, @pericardial_effusion,
		     @ivc_diameter_mm, @ivc_collapsibility, @la_volume_ml, @ra_area_cm2,
		     @interpretation, @notes, @tenant_id)
		RETURNING `+echoSelectCols,
		pgx.NamedArgs{
			"patient_nhi":                nhiEnc,
			"ordering_clinician_hpi":     req.OrderingClinicianHpi,
			"reporting_clinician_hpi":    req.ReportingClinicianHpi,
			"study_type":                 req.StudyType,
			"indication":                 req.Indication,
			"lvef_percent":               req.LvefPercent,
			"lv_edv_ml":                  req.LvEdvMl,
			"lv_esv_ml":                  req.LvEsvMl,
			"lv_diastolic_diameter_mm":   req.LvDiastolicDiameterMm,
			"lv_systolic_diameter_mm":    req.LvSystolicDiameterMm,
			"lv_posterior_wall_mm":       req.LvPosteriorWallMm,
			"interventricular_septum_mm": req.InterventricularSeptumMm,
			"lv_mass_g":                  req.LvMassG,
			"wall_motion_abnormality":    req.WallMotionAbnormality,
			"wall_motion_segments":       req.WallMotionSegments,
			"diastolic_function":         req.DiastolicFunction,
			"aortic_valve_finding":       req.AorticValveFinding,
			"aortic_gradient_mmhg":       req.AorticGradientMmhg,
			"aortic_valve_area_cm2":      req.AorticValveAreaCm2,
			"mitral_valve_finding":       req.MitralValveFinding,
			"mitral_e_velocity":          req.MitralEVelocity,
			"mitral_a_velocity":          req.MitralAVelocity,
			"e_a_ratio":                  req.EARatio,
			"mitral_e_prime":             req.MitralEPrime,
			"e_e_prime_ratio":            req.EEPrimeRatio,
			"tricuspid_valve_finding":    req.TricuspidValveFinding,
			"tv_regurg_velocity":         req.TvRegurgVelocity,
			"rvsp_mmhg":                  req.RvspMmhg,
			"pulmonary_valve_finding":    req.PulmonaryValveFinding,
			"rv_function":                req.RvFunction,
			"pericardial_effusion":       req.PericardialEffusion,
			"ivc_diameter_mm":            req.IvcDiameterMm,
			"ivc_collapsibility":         req.IvcCollapsibility,
			"la_volume_ml":               req.LaVolumeMl,
			"ra_area_cm2":                req.RaAreaCm2,
			"interpretation":             req.Interpretation,
			"notes":                      req.Notes,
			"tenant_id":                  tenantID,
		}).Scan(
		&e.ID, &e.PatientNHI, &e.OrderingClinicianHpi, &e.ReportingClinicianHpi, &e.StudyType, &e.Status, &e.Indication,
		&e.LvefPercent, &e.LvEdvMl, &e.LvEsvMl, &e.LvDiastolicDiameterMm, &e.LvSystolicDiameterMm,
		&e.LvPosteriorWallMm, &e.InterventricularSeptumMm, &e.LvMassG, &e.WallMotionAbnormality,
		&e.WallMotionSegments, &e.DiastolicFunction,
		&e.AorticValveFinding, &e.AorticGradientMmhg, &e.AorticValveAreaCm2,
		&e.MitralValveFinding, &e.MitralEVelocity, &e.MitralAVelocity, &e.EARatio, &e.MitralEPrime, &e.EEPrimeRatio,
		&e.TricuspidValveFinding, &e.TvRegurgVelocity, &e.RvspMmhg,
		&e.PulmonaryValveFinding, &e.RvFunction, &e.PericardialEffusion,
		&e.IvcDiameterMm, &e.IvcCollapsibility, &e.LaVolumeMl, &e.RaAreaCm2,
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
	h.recordAudit(r, "create", "EchoStudy", e.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, e)
}

func (h *echoHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var e EchoStudy
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+echoSelectCols+` FROM echo_studies WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&e.ID, &e.PatientNHI, &e.OrderingClinicianHpi, &e.ReportingClinicianHpi, &e.StudyType, &e.Status, &e.Indication,
		&e.LvefPercent, &e.LvEdvMl, &e.LvEsvMl, &e.LvDiastolicDiameterMm, &e.LvSystolicDiameterMm,
		&e.LvPosteriorWallMm, &e.InterventricularSeptumMm, &e.LvMassG, &e.WallMotionAbnormality,
		&e.WallMotionSegments, &e.DiastolicFunction,
		&e.AorticValveFinding, &e.AorticGradientMmhg, &e.AorticValveAreaCm2,
		&e.MitralValveFinding, &e.MitralEVelocity, &e.MitralAVelocity, &e.EARatio, &e.MitralEPrime, &e.EEPrimeRatio,
		&e.TricuspidValveFinding, &e.TvRegurgVelocity, &e.RvspMmhg,
		&e.PulmonaryValveFinding, &e.RvFunction, &e.PericardialEffusion,
		&e.IvcDiameterMm, &e.IvcCollapsibility, &e.LaVolumeMl, &e.RaAreaCm2,
		&e.Interpretation, &e.Notes, &e.TenantID, &e.OrderedAt, &e.PerformedAt, &e.ReportedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "echo study not found"})
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

func (h *echoHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req EchoStudy
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ReportingClinicianHpi) {
		return
	}
	var e EchoStudy
	err := h.pool.QueryRow(r.Context(), `
		UPDATE echo_studies
		SET reporting_clinician_hpi  = @reporting_clinician_hpi,
		    status                   = @status,
		    lvef_percent             = @lvef_percent,
		    lv_edv_ml                = @lv_edv_ml,
		    lv_esv_ml                = @lv_esv_ml,
		    lv_diastolic_diameter_mm = @lv_diastolic_diameter_mm,
		    lv_systolic_diameter_mm  = @lv_systolic_diameter_mm,
		    lv_posterior_wall_mm     = @lv_posterior_wall_mm,
		    interventricular_septum_mm = @interventricular_septum_mm,
		    lv_mass_g                = @lv_mass_g,
		    wall_motion_abnormality  = @wall_motion_abnormality,
		    wall_motion_segments     = @wall_motion_segments,
		    diastolic_function       = @diastolic_function,
		    aortic_valve_finding     = @aortic_valve_finding,
		    aortic_gradient_mmhg     = @aortic_gradient_mmhg,
		    aortic_valve_area_cm2    = @aortic_valve_area_cm2,
		    mitral_valve_finding     = @mitral_valve_finding,
		    mitral_e_velocity        = @mitral_e_velocity,
		    mitral_a_velocity        = @mitral_a_velocity,
		    e_a_ratio                = @e_a_ratio,
		    mitral_e_prime           = @mitral_e_prime,
		    e_e_prime_ratio          = @e_e_prime_ratio,
		    tricuspid_valve_finding  = @tricuspid_valve_finding,
		    tv_regurg_velocity       = @tv_regurg_velocity,
		    rvsp_mmhg                = @rvsp_mmhg,
		    pulmonary_valve_finding  = @pulmonary_valve_finding,
		    rv_function              = @rv_function,
		    pericardial_effusion     = @pericardial_effusion,
		    ivc_diameter_mm         = @ivc_diameter_mm,
		    ivc_collapsibility      = @ivc_collapsibility,
		    la_volume_ml            = @la_volume_ml,
		    ra_area_cm2             = @ra_area_cm2,
		    interpretation          = @interpretation,
		    notes                   = @notes,
		    performed_at            = COALESCE(performed_at, CASE WHEN @status IN ('performed','reported') THEN now() END),
		    reported_at             = CASE WHEN @status = 'reported' THEN now() ELSE reported_at END,
		    updated_at              = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+echoSelectCols,
		pgx.NamedArgs{
			"reporting_clinician_hpi":     req.ReportingClinicianHpi,
			"status":                      req.Status,
			"lvef_percent":                req.LvefPercent,
			"lv_edv_ml":                   req.LvEdvMl,
			"lv_esv_ml":                   req.LvEsvMl,
			"lv_diastolic_diameter_mm":    req.LvDiastolicDiameterMm,
			"lv_systolic_diameter_mm":     req.LvSystolicDiameterMm,
			"lv_posterior_wall_mm":        req.LvPosteriorWallMm,
			"interventricular_septum_mm":  req.InterventricularSeptumMm,
			"lv_mass_g":                   req.LvMassG,
			"wall_motion_abnormality":     req.WallMotionAbnormality,
			"wall_motion_segments":        req.WallMotionSegments,
			"diastolic_function":          req.DiastolicFunction,
			"aortic_valve_finding":        req.AorticValveFinding,
			"aortic_gradient_mmhg":        req.AorticGradientMmhg,
			"aortic_valve_area_cm2":       req.AorticValveAreaCm2,
			"mitral_valve_finding":        req.MitralValveFinding,
			"mitral_e_velocity":           req.MitralEVelocity,
			"mitral_a_velocity":           req.MitralAVelocity,
			"e_a_ratio":                   req.EARatio,
			"mitral_e_prime":              req.MitralEPrime,
			"e_e_prime_ratio":             req.EEPrimeRatio,
			"tricuspid_valve_finding":     req.TricuspidValveFinding,
			"tv_regurg_velocity":          req.TvRegurgVelocity,
			"rvsp_mmhg":                   req.RvspMmhg,
			"pulmonary_valve_finding":     req.PulmonaryValveFinding,
			"rv_function":                 req.RvFunction,
			"pericardial_effusion":        req.PericardialEffusion,
			"ivc_diameter_mm":             req.IvcDiameterMm,
			"ivc_collapsibility":          req.IvcCollapsibility,
			"la_volume_ml":                req.LaVolumeMl,
			"ra_area_cm2":                 req.RaAreaCm2,
			"interpretation":              req.Interpretation,
			"notes":                       req.Notes,
			"id":                          id,
			"tenant_id":                   tenantID,
		}).Scan(
		&e.ID, &e.PatientNHI, &e.OrderingClinicianHpi, &e.ReportingClinicianHpi, &e.StudyType, &e.Status, &e.Indication,
		&e.LvefPercent, &e.LvEdvMl, &e.LvEsvMl, &e.LvDiastolicDiameterMm, &e.LvSystolicDiameterMm,
		&e.LvPosteriorWallMm, &e.InterventricularSeptumMm, &e.LvMassG, &e.WallMotionAbnormality,
		&e.WallMotionSegments, &e.DiastolicFunction,
		&e.AorticValveFinding, &e.AorticGradientMmhg, &e.AorticValveAreaCm2,
		&e.MitralValveFinding, &e.MitralEVelocity, &e.MitralAVelocity, &e.EARatio, &e.MitralEPrime, &e.EEPrimeRatio,
		&e.TricuspidValveFinding, &e.TvRegurgVelocity, &e.RvspMmhg,
		&e.PulmonaryValveFinding, &e.RvFunction, &e.PericardialEffusion,
		&e.IvcDiameterMm, &e.IvcCollapsibility, &e.LaVolumeMl, &e.RaAreaCm2,
		&e.Interpretation, &e.Notes, &e.TenantID, &e.OrderedAt, &e.PerformedAt, &e.ReportedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "echo study not found"})
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
	h.recordAudit(r, "update", "EchoStudy", e.ID, nhiEnc)
	writeJSON(w, http.StatusOK, e)
}
