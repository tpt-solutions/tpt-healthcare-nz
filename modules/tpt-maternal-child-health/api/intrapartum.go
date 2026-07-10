package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// IntrapartumStatus tracks the phase of labour and birth.
type IntrapartumStatus string

const (
	IntrapartumStatusLatentPhase  IntrapartumStatus = "latent-phase"
	IntrapartumStatusActiveLabour IntrapartumStatus = "active-labour"
	IntrapartumStatusSecondStage  IntrapartumStatus = "second-stage"
	IntrapartumStatusThirdStage   IntrapartumStatus = "third-stage"
	IntrapartumStatusCompleted    IntrapartumStatus = "completed"
	IntrapartumStatusTransferred  IntrapartumStatus = "transferred"
	IntrapartumStatusAbandoned    IntrapartumStatus = "abandoned"
)

// LabourOnsetType classifies how labour began.
type LabourOnsetType string

const (
	LabourOnsetSpontaneous LabourOnsetType = "spontaneous"
	LabourOnsetInduced     LabourOnsetType = "induced"
	LabourOnsetElectiveCS  LabourOnsetType = "elective-cs"
	LabourOnsetEmergencyCS LabourOnsetType = "emergency-cs"
)

// DeliveryMethod classifies the mode of birth.
type DeliveryMethod string

const (
	DeliveryMethodSVD                 DeliveryMethod = "SVD"
	DeliveryMethodInstrumentalForceps DeliveryMethod = "instrumental-forceps"
	DeliveryMethodInstrumentalVacuum  DeliveryMethod = "instrumental-vacuum"
	DeliveryMethodLSCS                DeliveryMethod = "LSCS"
	DeliveryMethodEmergencyCS         DeliveryMethod = "emergency-cs"
)

// CTGClassification maps to the RANZCOG/NICE CTG classification categories.
type CTGClassification string

const (
	CTGClassificationNormal       CTGClassification = "normal"
	CTGClassificationSuspicious   CTGClassification = "suspicious"
	CTGClassificationPathological CTGClassification = "pathological"
)

type IntrapartumEpisode struct {
	ID                    string     `json:"id"`
	MaternityEpisodeID    string     `json:"maternityEpisodeId"`
	ClinicianHpi          string     `json:"clinicianHpi"`
	Status                string     `json:"status"`
	OnsetType             string     `json:"onsetType"`
	BirthSetting          string     `json:"birthSetting"`
	LabourOnsetAt         *time.Time `json:"labourOnsetAt"`
	ActiveLabourAt        *time.Time `json:"activeLabourAt"`
	BirthAt               *time.Time `json:"birthAt"`
	DeliveryMethod        *string    `json:"deliveryMethod"`
	PerinealOutcome       *string    `json:"perinealOutcome"`
	BloodLossMl           *int       `json:"bloodLossMl"`
	NeonatalSex           *string    `json:"neonatalSex"`
	BirthWeightGrams      *int       `json:"birthWeightGrams"`
	GestationAtBirthWeeks *int16     `json:"gestationAtBirthWeeks"`
	Apgar1min             *int16     `json:"apgar1min"`
	Apgar5min             *int16     `json:"apgar5min"`
	Apgar10min            *int16     `json:"apgar10min"`
	CordPh                *float64   `json:"cordPh"`
	Notes                 *string    `json:"notes"`
	TenantID              string     `json:"tenantId"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type PartogramEntry struct {
	ID                      string    `json:"id"`
	IntrapartumEpisodeID    string    `json:"intrapartumEpisodeId"`
	ClinicianHpi            string    `json:"clinicianHpi"`
	CervicalDilationCm      *float64  `json:"cervicalDilationCm"`
	FetalStation            *int16    `json:"fetalStation"`
	ContractionsIn10min     *int16    `json:"contractionsIn10min"`
	ContractionDurationSecs *int16    `json:"contractionDurationSecs"`
	FetalHeartRateBpm       *int16    `json:"fetalHeartRateBpm"`
	MaternalBpSystolic      *int16    `json:"maternalBpSystolic"`
	MaternalBpDiastolic     *int16    `json:"maternalBpDiastolic"`
	MaternalPulseBpm        *int16    `json:"maternalPulseBpm"`
	TemperatureCelsius      *float64  `json:"temperatureCelsius"`
	UrineVolumeMl           *int      `json:"urineVolumeMl"`
	LiquorColour            *string   `json:"liquorColour"`
	Caput                   *int16    `json:"caput"`
	Moulding                *int16    `json:"moulding"`
	OxytocinDoseMuPerMin    *float64  `json:"oxytocinDoseMuPerMin"`
	Analgesic               *string   `json:"analgesic"`
	Notes                   *string   `json:"notes"`
	RecordedAt              time.Time `json:"recordedAt"`
	TenantID                string    `json:"tenantId"`
}

type CTGRecord struct {
	ID                   string     `json:"id"`
	IntrapartumEpisodeID string     `json:"intrapartumEpisodeId"`
	ClinicianHpi         string     `json:"clinicianHpi"`
	BaselineBpm          *int16     `json:"baselineBpm"`
	BaselineVariability  *string    `json:"baselineVariability"`
	Accelerations        *bool      `json:"accelerations"`
	Decelerations        *string    `json:"decelerations"`
	UterineActivity      *string    `json:"uterineActivity"`
	CtgClassification    string     `json:"ctgClassification"`
	InterpretationNotes  *string    `json:"interpretationNotes"`
	StartedAt            time.Time  `json:"startedAt"`
	EndedAt              *time.Time `json:"endedAt"`
	TenantID             string     `json:"tenantId"`
	CreatedAt            time.Time  `json:"createdAt"`
}

// intrapartumHandler manages birth episodes, partogram charting, and CTG monitoring.
type intrapartumHandler struct {
	handlerDeps
}

func (h *intrapartumHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, maternity_episode_id, clinician_hpi, status, onset_type, birth_setting,
		       labour_onset_at, active_labour_at, birth_at, delivery_method, perineal_outcome,
		       blood_loss_ml, neonatal_sex, birth_weight_grams, gestation_at_birth_weeks,
		       apgar_1min, apgar_5min, apgar_10min, cord_ph, notes, tenant_id, created_at, updated_at
		FROM intrapartum_episodes
		WHERE maternity_episode_id = @episode_id AND tenant_id = @tenant_id
		ORDER BY created_at DESC
	`, pgx.NamedArgs{"episode_id": episodeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	episodes := make([]IntrapartumEpisode, 0)
	for rows.Next() {
		var ep IntrapartumEpisode
		if err := rows.Scan(
			&ep.ID, &ep.MaternityEpisodeID, &ep.ClinicianHpi, &ep.Status, &ep.OnsetType, &ep.BirthSetting,
			&ep.LabourOnsetAt, &ep.ActiveLabourAt, &ep.BirthAt, &ep.DeliveryMethod, &ep.PerinealOutcome,
			&ep.BloodLossMl, &ep.NeonatalSex, &ep.BirthWeightGrams, &ep.GestationAtBirthWeeks,
			&ep.Apgar1min, &ep.Apgar5min, &ep.Apgar10min, &ep.CordPh, &ep.Notes,
			&ep.TenantID, &ep.CreatedAt, &ep.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		episodes = append(episodes, ep)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, episodes)
}

func (h *intrapartumHandler) Start(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	var req IntrapartumEpisode
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.OnsetType == "" {
		req.OnsetType = "spontaneous"
	}
	if req.BirthSetting == "" {
		req.BirthSetting = "hospital"
	}
	var ep IntrapartumEpisode
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO intrapartum_episodes
		    (maternity_episode_id, clinician_hpi, status, onset_type, birth_setting,
		     labour_onset_at, tenant_id)
		VALUES
		    (@episode_id, @clinician_hpi, 'latent-phase', @onset_type, @birth_setting,
		     @labour_onset_at, @tenant_id)
		RETURNING id, maternity_episode_id, clinician_hpi, status, onset_type, birth_setting,
		          labour_onset_at, active_labour_at, birth_at, delivery_method, perineal_outcome,
		          blood_loss_ml, neonatal_sex, birth_weight_grams, gestation_at_birth_weeks,
		          apgar_1min, apgar_5min, apgar_10min, cord_ph, notes, tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"episode_id":      episodeID,
		"clinician_hpi":   req.ClinicianHpi,
		"onset_type":      req.OnsetType,
		"birth_setting":   req.BirthSetting,
		"labour_onset_at": req.LabourOnsetAt,
		"tenant_id":       tenantID,
	}).Scan(
		&ep.ID, &ep.MaternityEpisodeID, &ep.ClinicianHpi, &ep.Status, &ep.OnsetType, &ep.BirthSetting,
		&ep.LabourOnsetAt, &ep.ActiveLabourAt, &ep.BirthAt, &ep.DeliveryMethod, &ep.PerinealOutcome,
		&ep.BloodLossMl, &ep.NeonatalSex, &ep.BirthWeightGrams, &ep.GestationAtBirthWeeks,
		&ep.Apgar1min, &ep.Apgar5min, &ep.Apgar10min, &ep.CordPh, &ep.Notes,
		&ep.TenantID, &ep.CreatedAt, &ep.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, ep)
}

func (h *intrapartumHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	birthID := r.PathValue("birthId")
	var ep IntrapartumEpisode
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, maternity_episode_id, clinician_hpi, status, onset_type, birth_setting,
		       labour_onset_at, active_labour_at, birth_at, delivery_method, perineal_outcome,
		       blood_loss_ml, neonatal_sex, birth_weight_grams, gestation_at_birth_weeks,
		       apgar_1min, apgar_5min, apgar_10min, cord_ph, notes, tenant_id, created_at, updated_at
		FROM intrapartum_episodes
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": birthID, "tenant_id": tenantID}).Scan(
		&ep.ID, &ep.MaternityEpisodeID, &ep.ClinicianHpi, &ep.Status, &ep.OnsetType, &ep.BirthSetting,
		&ep.LabourOnsetAt, &ep.ActiveLabourAt, &ep.BirthAt, &ep.DeliveryMethod, &ep.PerinealOutcome,
		&ep.BloodLossMl, &ep.NeonatalSex, &ep.BirthWeightGrams, &ep.GestationAtBirthWeeks,
		&ep.Apgar1min, &ep.Apgar5min, &ep.Apgar10min, &ep.CordPh, &ep.Notes,
		&ep.TenantID, &ep.CreatedAt, &ep.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "birth episode not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ep)
}

func (h *intrapartumHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	birthID := r.PathValue("birthId")
	var req IntrapartumEpisode
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var ep IntrapartumEpisode
	err := h.pool.QueryRow(r.Context(), `
		UPDATE intrapartum_episodes
		SET status = @status,
		    active_labour_at = @active_labour_at,
		    birth_at = @birth_at,
		    delivery_method = @delivery_method,
		    perineal_outcome = @perineal_outcome,
		    blood_loss_ml = @blood_loss_ml,
		    neonatal_sex = @neonatal_sex,
		    birth_weight_grams = @birth_weight_grams,
		    gestation_at_birth_weeks = @gestation_at_birth_weeks,
		    apgar_1min = @apgar_1min,
		    apgar_5min = @apgar_5min,
		    apgar_10min = @apgar_10min,
		    cord_ph = @cord_ph,
		    notes = @notes,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, maternity_episode_id, clinician_hpi, status, onset_type, birth_setting,
		          labour_onset_at, active_labour_at, birth_at, delivery_method, perineal_outcome,
		          blood_loss_ml, neonatal_sex, birth_weight_grams, gestation_at_birth_weeks,
		          apgar_1min, apgar_5min, apgar_10min, cord_ph, notes, tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"status":                   req.Status,
		"active_labour_at":         req.ActiveLabourAt,
		"birth_at":                 req.BirthAt,
		"delivery_method":          req.DeliveryMethod,
		"perineal_outcome":         req.PerinealOutcome,
		"blood_loss_ml":            req.BloodLossMl,
		"neonatal_sex":             req.NeonatalSex,
		"birth_weight_grams":       req.BirthWeightGrams,
		"gestation_at_birth_weeks": req.GestationAtBirthWeeks,
		"apgar_1min":               req.Apgar1min,
		"apgar_5min":               req.Apgar5min,
		"apgar_10min":              req.Apgar10min,
		"cord_ph":                  req.CordPh,
		"notes":                    req.Notes,
		"id":                       birthID,
		"tenant_id":                tenantID,
	}).Scan(
		&ep.ID, &ep.MaternityEpisodeID, &ep.ClinicianHpi, &ep.Status, &ep.OnsetType, &ep.BirthSetting,
		&ep.LabourOnsetAt, &ep.ActiveLabourAt, &ep.BirthAt, &ep.DeliveryMethod, &ep.PerinealOutcome,
		&ep.BloodLossMl, &ep.NeonatalSex, &ep.BirthWeightGrams, &ep.GestationAtBirthWeeks,
		&ep.Apgar1min, &ep.Apgar5min, &ep.Apgar10min, &ep.CordPh, &ep.Notes,
		&ep.TenantID, &ep.CreatedAt, &ep.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "birth episode not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ep)
}

func (h *intrapartumHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	birthID := r.PathValue("birthId")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE intrapartum_episodes
		SET status = 'completed', birth_at = COALESCE(birth_at, now()), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status != 'completed'
	`, pgx.NamedArgs{"id": birthID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "birth episode not found or already completed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (h *intrapartumHandler) ListPartogram(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	birthID := r.PathValue("birthId")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, intrapartum_episode_id, clinician_hpi,
		       cervical_dilation_cm, fetal_station, contractions_in_10min, contraction_duration_secs,
		       fetal_heart_rate_bpm, maternal_bp_systolic, maternal_bp_diastolic, maternal_pulse_bpm,
		       temperature_celsius, urine_volume_ml, liquor_colour, caput, moulding,
		       oxytocin_dose_mu_per_min, analgesic, notes, recorded_at, tenant_id
		FROM partogram_entries
		WHERE intrapartum_episode_id = @birth_id AND tenant_id = @tenant_id
		ORDER BY recorded_at ASC
	`, pgx.NamedArgs{"birth_id": birthID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	entries := make([]PartogramEntry, 0)
	for rows.Next() {
		var e PartogramEntry
		if err := rows.Scan(
			&e.ID, &e.IntrapartumEpisodeID, &e.ClinicianHpi,
			&e.CervicalDilationCm, &e.FetalStation, &e.ContractionsIn10min, &e.ContractionDurationSecs,
			&e.FetalHeartRateBpm, &e.MaternalBpSystolic, &e.MaternalBpDiastolic, &e.MaternalPulseBpm,
			&e.TemperatureCelsius, &e.UrineVolumeMl, &e.LiquorColour, &e.Caput, &e.Moulding,
			&e.OxytocinDoseMuPerMin, &e.Analgesic, &e.Notes, &e.RecordedAt, &e.TenantID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *intrapartumHandler) AddPartogramEntry(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	birthID := r.PathValue("birthId")
	var req PartogramEntry
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var e PartogramEntry
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO partogram_entries
		    (intrapartum_episode_id, clinician_hpi,
		     cervical_dilation_cm, fetal_station, contractions_in_10min, contraction_duration_secs,
		     fetal_heart_rate_bpm, maternal_bp_systolic, maternal_bp_diastolic, maternal_pulse_bpm,
		     temperature_celsius, urine_volume_ml, liquor_colour, caput, moulding,
		     oxytocin_dose_mu_per_min, analgesic, notes, tenant_id)
		VALUES
		    (@birth_id, @clinician_hpi,
		     @cervical_dilation_cm, @fetal_station, @contractions_in_10min, @contraction_duration_secs,
		     @fetal_heart_rate_bpm, @maternal_bp_systolic, @maternal_bp_diastolic, @maternal_pulse_bpm,
		     @temperature_celsius, @urine_volume_ml, @liquor_colour, @caput, @moulding,
		     @oxytocin_dose_mu_per_min, @analgesic, @notes, @tenant_id)
		RETURNING id, intrapartum_episode_id, clinician_hpi,
		          cervical_dilation_cm, fetal_station, contractions_in_10min, contraction_duration_secs,
		          fetal_heart_rate_bpm, maternal_bp_systolic, maternal_bp_diastolic, maternal_pulse_bpm,
		          temperature_celsius, urine_volume_ml, liquor_colour, caput, moulding,
		          oxytocin_dose_mu_per_min, analgesic, notes, recorded_at, tenant_id
	`, pgx.NamedArgs{
		"birth_id":                  birthID,
		"clinician_hpi":             req.ClinicianHpi,
		"cervical_dilation_cm":      req.CervicalDilationCm,
		"fetal_station":             req.FetalStation,
		"contractions_in_10min":     req.ContractionsIn10min,
		"contraction_duration_secs": req.ContractionDurationSecs,
		"fetal_heart_rate_bpm":      req.FetalHeartRateBpm,
		"maternal_bp_systolic":      req.MaternalBpSystolic,
		"maternal_bp_diastolic":     req.MaternalBpDiastolic,
		"maternal_pulse_bpm":        req.MaternalPulseBpm,
		"temperature_celsius":       req.TemperatureCelsius,
		"urine_volume_ml":           req.UrineVolumeMl,
		"liquor_colour":             req.LiquorColour,
		"caput":                     req.Caput,
		"moulding":                  req.Moulding,
		"oxytocin_dose_mu_per_min":  req.OxytocinDoseMuPerMin,
		"analgesic":                 req.Analgesic,
		"notes":                     req.Notes,
		"tenant_id":                 tenantID,
	}).Scan(
		&e.ID, &e.IntrapartumEpisodeID, &e.ClinicianHpi,
		&e.CervicalDilationCm, &e.FetalStation, &e.ContractionsIn10min, &e.ContractionDurationSecs,
		&e.FetalHeartRateBpm, &e.MaternalBpSystolic, &e.MaternalBpDiastolic, &e.MaternalPulseBpm,
		&e.TemperatureCelsius, &e.UrineVolumeMl, &e.LiquorColour, &e.Caput, &e.Moulding,
		&e.OxytocinDoseMuPerMin, &e.Analgesic, &e.Notes, &e.RecordedAt, &e.TenantID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (h *intrapartumHandler) ListCTG(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	birthID := r.PathValue("birthId")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, intrapartum_episode_id, clinician_hpi, baseline_bpm, baseline_variability,
		       accelerations, decelerations, uterine_activity, ctg_classification,
		       interpretation_notes, started_at, ended_at, tenant_id, created_at
		FROM ctg_records
		WHERE intrapartum_episode_id = @birth_id AND tenant_id = @tenant_id
		ORDER BY started_at DESC
	`, pgx.NamedArgs{"birth_id": birthID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	records := make([]CTGRecord, 0)
	for rows.Next() {
		var c CTGRecord
		if err := rows.Scan(
			&c.ID, &c.IntrapartumEpisodeID, &c.ClinicianHpi, &c.BaselineBpm, &c.BaselineVariability,
			&c.Accelerations, &c.Decelerations, &c.UterineActivity, &c.CtgClassification,
			&c.InterpretationNotes, &c.StartedAt, &c.EndedAt, &c.TenantID, &c.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		records = append(records, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *intrapartumHandler) AddCTG(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	birthID := r.PathValue("birthId")
	var req CTGRecord
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.CtgClassification == "" {
		req.CtgClassification = "normal"
	}
	var c CTGRecord
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO ctg_records
		    (intrapartum_episode_id, clinician_hpi, baseline_bpm, baseline_variability,
		     accelerations, decelerations, uterine_activity, ctg_classification,
		     interpretation_notes, ended_at, tenant_id)
		VALUES
		    (@birth_id, @clinician_hpi, @baseline_bpm, @baseline_variability,
		     @accelerations, @decelerations, @uterine_activity, @ctg_classification,
		     @interpretation_notes, @ended_at, @tenant_id)
		RETURNING id, intrapartum_episode_id, clinician_hpi, baseline_bpm, baseline_variability,
		          accelerations, decelerations, uterine_activity, ctg_classification,
		          interpretation_notes, started_at, ended_at, tenant_id, created_at
	`, pgx.NamedArgs{
		"birth_id":             birthID,
		"clinician_hpi":        req.ClinicianHpi,
		"baseline_bpm":         req.BaselineBpm,
		"baseline_variability": req.BaselineVariability,
		"accelerations":        req.Accelerations,
		"decelerations":        req.Decelerations,
		"uterine_activity":     req.UterineActivity,
		"ctg_classification":   req.CtgClassification,
		"interpretation_notes": req.InterpretationNotes,
		"ended_at":             req.EndedAt,
		"tenant_id":            tenantID,
	}).Scan(
		&c.ID, &c.IntrapartumEpisodeID, &c.ClinicianHpi, &c.BaselineBpm, &c.BaselineVariability,
		&c.Accelerations, &c.Decelerations, &c.UterineActivity, &c.CtgClassification,
		&c.InterpretationNotes, &c.StartedAt, &c.EndedAt, &c.TenantID, &c.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, c)
}
