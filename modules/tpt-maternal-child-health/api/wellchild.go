package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// WellChildCheckType identifies the scheduled Well Child Tamariki Ora contact.
// The schedule is defined by Te Whatu Ora under the Well Child Tamariki Ora
// Framework; contacts span birth to ~5 years.
type WellChildCheckType string

const (
	WellChildNeonatal  WellChildCheckType = "neonatal" // LMC handover check (~4–5 days)
	WellChildCheck6wk  WellChildCheckType = "6wk"      // 4–6 week GP check
	WellChildCheck3mo  WellChildCheckType = "3mo"
	WellChildCheck5mo  WellChildCheckType = "5mo"
	WellChildCheck9mo  WellChildCheckType = "9mo"
	WellChildCheck12mo WellChildCheckType = "12mo"
	WellChildCheck15mo WellChildCheckType = "15mo"
	WellChildCheck2yr  WellChildCheckType = "2yr"
	WellChildB4School  WellChildCheckType = "B4SchoolCheck" // ~age 4, before school entry
)

// SDQBand classifies the Strengths and Difficulties Questionnaire total score.
type SDQBand string

const (
	SDQBandNormal     SDQBand = "normal"
	SDQBandBorderline SDQBand = "borderline"
	SDQBandAbnormal   SDQBand = "abnormal"
)

// WellChildCheckStatus tracks whether the check has been completed.
type WellChildCheckStatus string

const (
	WellChildStatusScheduled WellChildCheckStatus = "scheduled"
	WellChildStatusCompleted WellChildCheckStatus = "completed"
	WellChildStatusMissed    WellChildCheckStatus = "missed"
	WellChildStatusDeclined  WellChildCheckStatus = "declined"
)

type WellChildCheck struct {
	ID                    string    `json:"id"`
	MaternityEpisodeID    *string   `json:"maternityEpisodeId"`
	PatientNHI            string    `json:"patientNhi"`
	ProviderHpi           string    `json:"providerHpi"`
	CheckType             string    `json:"checkType"`
	Status                string    `json:"status"`
	AgeAtCheckWeeks       *int16    `json:"ageAtCheckWeeks"`
	WeightKg              *float64  `json:"weightKg"`
	HeightCm              *float64  `json:"heightCm"`
	HeadCircumferenceCm   *float64  `json:"headCircumferenceCm"`
	DevelopmentalConcerns bool      `json:"developmentalConcerns"`
	VisionConcerns        bool      `json:"visionConcerns"`
	HearingConcerns       bool      `json:"hearingConcerns"`
	SdqScore              *int16    `json:"sdqScore"`
	SdqBand               *string   `json:"sdqBand"`
	ImmunisationsUpToDate bool      `json:"immunisationsUpToDate"`
	Referrals             *string   `json:"referrals"`
	Notes                 *string   `json:"notes"`
	TenantID              string    `json:"tenantId"`
	CheckedAt             time.Time `json:"checkedAt"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type WellChildGrowthPoint struct {
	ID                  string    `json:"id"`
	WellChildCheckID    string    `json:"wellChildCheckId"`
	PatientNHI          string    `json:"patientNhi"`
	WeightKg            *float64  `json:"weightKg"`
	HeightCm            *float64  `json:"heightCm"`
	HeadCircumferenceCm *float64  `json:"headCircumferenceCm"`
	CentileBand         *string   `json:"centileBand"`
	RecordedAt          time.Time `json:"recordedAt"`
	TenantID            string    `json:"tenantId"`
}

// wellChildHandler manages Well Child Tamariki Ora checks and growth monitoring.
// Growth points store raw measurements (weight, height, head circumference);
// centile band calculation uses WHO growth standards and is performed client-side.
type wellChildHandler struct {
	handlerDeps
}

func (h *wellChildHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, maternity_episode_id, patient_nhi, provider_hpi, check_type, status,
		       age_at_check_weeks, weight_kg, height_cm, head_circumference_cm,
		       developmental_concerns, vision_concerns, hearing_concerns,
		       sdq_score, sdq_band, immunisations_up_to_date, referrals, notes,
		       tenant_id, checked_at, created_at, updated_at
		FROM well_child_checks
		WHERE tenant_id = @tenant_id
		ORDER BY checked_at DESC
	`, pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	checks := make([]WellChildCheck, 0)
	for rows.Next() {
		var c WellChildCheck
		if err := rows.Scan(
			&c.ID, &c.MaternityEpisodeID, &c.PatientNHI, &c.ProviderHpi, &c.CheckType, &c.Status,
			&c.AgeAtCheckWeeks, &c.WeightKg, &c.HeightCm, &c.HeadCircumferenceCm,
			&c.DevelopmentalConcerns, &c.VisionConcerns, &c.HearingConcerns,
			&c.SdqScore, &c.SdqBand, &c.ImmunisationsUpToDate, &c.Referrals, &c.Notes,
			&c.TenantID, &c.CheckedAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(c.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		c.PatientNHI = nhi
		checks = append(checks, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

func (h *wellChildHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req WellChildCheck
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" || req.ProviderHpi == "" || req.CheckType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELDS", Message: "patientNhi, providerHpi, and checkType are required"})
		return
	}
	if req.Status == "" {
		req.Status = "completed"
	}
	if !h.validateHPI(w, r, req.ProviderHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var c WellChildCheck
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO well_child_checks
		    (maternity_episode_id, patient_nhi, provider_hpi, check_type, status,
		     age_at_check_weeks, weight_kg, height_cm, head_circumference_cm,
		     developmental_concerns, vision_concerns, hearing_concerns,
		     sdq_score, sdq_band, immunisations_up_to_date, referrals, notes, tenant_id)
		VALUES
		    (@maternity_episode_id, @patient_nhi, @provider_hpi, @check_type, @status,
		     @age_at_check_weeks, @weight_kg, @height_cm, @head_circumference_cm,
		     @developmental_concerns, @vision_concerns, @hearing_concerns,
		     @sdq_score, @sdq_band, @immunisations_up_to_date, @referrals, @notes, @tenant_id)
		RETURNING id, maternity_episode_id, patient_nhi, provider_hpi, check_type, status,
		          age_at_check_weeks, weight_kg, height_cm, head_circumference_cm,
		          developmental_concerns, vision_concerns, hearing_concerns,
		          sdq_score, sdq_band, immunisations_up_to_date, referrals, notes,
		          tenant_id, checked_at, created_at, updated_at
	`, pgx.NamedArgs{
		"maternity_episode_id":     req.MaternityEpisodeID,
		"patient_nhi":              nhiEnc,
		"provider_hpi":             req.ProviderHpi,
		"check_type":               req.CheckType,
		"status":                   req.Status,
		"age_at_check_weeks":       req.AgeAtCheckWeeks,
		"weight_kg":                req.WeightKg,
		"height_cm":                req.HeightCm,
		"head_circumference_cm":    req.HeadCircumferenceCm,
		"developmental_concerns":   req.DevelopmentalConcerns,
		"vision_concerns":          req.VisionConcerns,
		"hearing_concerns":         req.HearingConcerns,
		"sdq_score":                req.SdqScore,
		"sdq_band":                 req.SdqBand,
		"immunisations_up_to_date": req.ImmunisationsUpToDate,
		"referrals":                req.Referrals,
		"notes":                    req.Notes,
		"tenant_id":                tenantID,
	}).Scan(
		&c.ID, &c.MaternityEpisodeID, &c.PatientNHI, &c.ProviderHpi, &c.CheckType, &c.Status,
		&c.AgeAtCheckWeeks, &c.WeightKg, &c.HeightCm, &c.HeadCircumferenceCm,
		&c.DevelopmentalConcerns, &c.VisionConcerns, &c.HearingConcerns,
		&c.SdqScore, &c.SdqBand, &c.ImmunisationsUpToDate, &c.Referrals, &c.Notes,
		&c.TenantID, &c.CheckedAt, &c.CreatedAt, &c.UpdatedAt,
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
	h.recordAudit(r, "create", "WellChildCheck", c.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, c)
}

func (h *wellChildHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var c WellChildCheck
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, maternity_episode_id, patient_nhi, provider_hpi, check_type, status,
		       age_at_check_weeks, weight_kg, height_cm, head_circumference_cm,
		       developmental_concerns, vision_concerns, hearing_concerns,
		       sdq_score, sdq_band, immunisations_up_to_date, referrals, notes,
		       tenant_id, checked_at, created_at, updated_at
		FROM well_child_checks
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&c.ID, &c.MaternityEpisodeID, &c.PatientNHI, &c.ProviderHpi, &c.CheckType, &c.Status,
		&c.AgeAtCheckWeeks, &c.WeightKg, &c.HeightCm, &c.HeadCircumferenceCm,
		&c.DevelopmentalConcerns, &c.VisionConcerns, &c.HearingConcerns,
		&c.SdqScore, &c.SdqBand, &c.ImmunisationsUpToDate, &c.Referrals, &c.Notes,
		&c.TenantID, &c.CheckedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "well child check not found"})
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

func (h *wellChildHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req WellChildCheck
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var c WellChildCheck
	err := h.pool.QueryRow(r.Context(), `
		UPDATE well_child_checks
		SET status = @status,
		    weight_kg = @weight_kg,
		    height_cm = @height_cm,
		    head_circumference_cm = @head_circumference_cm,
		    developmental_concerns = @developmental_concerns,
		    vision_concerns = @vision_concerns,
		    hearing_concerns = @hearing_concerns,
		    sdq_score = @sdq_score,
		    sdq_band = @sdq_band,
		    immunisations_up_to_date = @immunisations_up_to_date,
		    referrals = @referrals,
		    notes = @notes,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, maternity_episode_id, patient_nhi, provider_hpi, check_type, status,
		          age_at_check_weeks, weight_kg, height_cm, head_circumference_cm,
		          developmental_concerns, vision_concerns, hearing_concerns,
		          sdq_score, sdq_band, immunisations_up_to_date, referrals, notes,
		          tenant_id, checked_at, created_at, updated_at
	`, pgx.NamedArgs{
		"status":                   req.Status,
		"weight_kg":                req.WeightKg,
		"height_cm":                req.HeightCm,
		"head_circumference_cm":    req.HeadCircumferenceCm,
		"developmental_concerns":   req.DevelopmentalConcerns,
		"vision_concerns":          req.VisionConcerns,
		"hearing_concerns":         req.HearingConcerns,
		"sdq_score":                req.SdqScore,
		"sdq_band":                 req.SdqBand,
		"immunisations_up_to_date": req.ImmunisationsUpToDate,
		"referrals":                req.Referrals,
		"notes":                    req.Notes,
		"id":                       id,
		"tenant_id":                tenantID,
	}).Scan(
		&c.ID, &c.MaternityEpisodeID, &c.PatientNHI, &c.ProviderHpi, &c.CheckType, &c.Status,
		&c.AgeAtCheckWeeks, &c.WeightKg, &c.HeightCm, &c.HeadCircumferenceCm,
		&c.DevelopmentalConcerns, &c.VisionConcerns, &c.HearingConcerns,
		&c.SdqScore, &c.SdqBand, &c.ImmunisationsUpToDate, &c.Referrals, &c.Notes,
		&c.TenantID, &c.CheckedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "well child check not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "WellChildCheck", c.ID, c.PatientNHI)
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	writeJSON(w, http.StatusOK, c)
}

func (h *wellChildHandler) ListGrowthPoints(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, well_child_check_id, patient_nhi,
		       weight_kg, height_cm, head_circumference_cm, centile_band, recorded_at, tenant_id
		FROM well_child_growth_points
		WHERE well_child_check_id = @id AND tenant_id = @tenant_id
		ORDER BY recorded_at DESC
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	points := make([]WellChildGrowthPoint, 0)
	for rows.Next() {
		var p WellChildGrowthPoint
		if err := rows.Scan(
			&p.ID, &p.WellChildCheckID, &p.PatientNHI,
			&p.WeightKg, &p.HeightCm, &p.HeadCircumferenceCm, &p.CentileBand, &p.RecordedAt, &p.TenantID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(p.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		p.PatientNHI = nhi
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (h *wellChildHandler) RecordGrowthPoint(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req WellChildGrowthPoint
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var p WellChildGrowthPoint
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO well_child_growth_points
		    (well_child_check_id, patient_nhi,
		     weight_kg, height_cm, head_circumference_cm, centile_band, tenant_id)
		VALUES
		    (@check_id, @patient_nhi,
		     @weight_kg, @height_cm, @head_circumference_cm, @centile_band, @tenant_id)
		RETURNING id, well_child_check_id, patient_nhi,
		          weight_kg, height_cm, head_circumference_cm, centile_band, recorded_at, tenant_id
	`, pgx.NamedArgs{
		"check_id":              id,
		"patient_nhi":           nhiEnc,
		"weight_kg":             req.WeightKg,
		"height_cm":             req.HeightCm,
		"head_circumference_cm": req.HeadCircumferenceCm,
		"centile_band":          req.CentileBand,
		"tenant_id":             tenantID,
	}).Scan(
		&p.ID, &p.WellChildCheckID, &p.PatientNHI,
		&p.WeightKg, &p.HeightCm, &p.HeadCircumferenceCm, &p.CentileBand, &p.RecordedAt, &p.TenantID,
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
	h.recordAudit(r, "create", "WellChildGrowthPoint", p.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, p)
}
