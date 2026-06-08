package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// PostnatalCheckType identifies the scheduled postnatal check.
type PostnatalCheckType string

const (
	PostnatalCheckImmediate PostnatalCheckType = "immediate"
	PostnatalCheck2hr       PostnatalCheckType = "2hr"
	PostnatalCheck6hr       PostnatalCheckType = "6hr"
	PostnatalCheckDay1      PostnatalCheckType = "day1"
	PostnatalCheckDay2      PostnatalCheckType = "day2"
	PostnatalCheckDay3      PostnatalCheckType = "day3"
	PostnatalCheckDay5      PostnatalCheckType = "day5"
	PostnatalCheckDay7      PostnatalCheckType = "day7"
	PostnatalCheckDay10     PostnatalCheckType = "day10"
	PostnatalCheckDay14     PostnatalCheckType = "day14"
	PostnatalCheck6wk       PostnatalCheckType = "6wk"
)

// CommunityVisitType classifies how the community midwife delivered the visit.
type CommunityVisitType string

const (
	CommunityVisitHome   CommunityVisitType = "home"
	CommunityVisitClinic CommunityVisitType = "clinic"
	CommunityVisitPhone  CommunityVisitType = "phone"
)

type PostnatalCheck struct {
	ID                    string    `json:"id"`
	MaternityEpisodeID    string    `json:"maternityEpisodeId"`
	ClinicianHpi          string    `json:"clinicianHpi"`
	CheckType             string    `json:"checkType"`
	CheckSubject          string    `json:"checkSubject"`
	MaternalBpSystolic    *int16    `json:"maternalBpSystolic"`
	MaternalBpDiastolic   *int16    `json:"maternalBpDiastolic"`
	MaternalPulseBpm      *int16    `json:"maternalPulseBpm"`
	MaternalTemperatureC  *float64  `json:"maternalTemperatureC"`
	MaternalFundalHeightCm *float64 `json:"maternalFundalHeightCm"`
	MaternalLochia        *string   `json:"maternalLochia"`
	MaternalPerineum      *string   `json:"maternalPerineum"`
	MaternalMood          string    `json:"maternalMood"`
	BabyWeightGrams       *int      `json:"babyWeightGrams"`
	BabyTemperatureC      *float64  `json:"babyTemperatureC"`
	BabyBilirubinUmol     *float64  `json:"babyBilirubinUmol"`
	BabyJaundice          bool      `json:"babyJaundice"`
	BabyFeedingMethod     *string   `json:"babyFeedingMethod"`
	BabyFeedingIssues     *string   `json:"babyFeedingIssues"`
	BabyUrineOutput       *string   `json:"babyUrineOutput"`
	BabyStool             *string   `json:"babyStool"`
	Notes                 *string   `json:"notes"`
	CheckedAt             time.Time `json:"checkedAt"`
	TenantID              string    `json:"tenantId"`
	CreatedAt             time.Time `json:"createdAt"`
}

type CommunityMidwifeVisit struct {
	ID                   string    `json:"id"`
	MaternityEpisodeID   string    `json:"maternityEpisodeId"`
	MidwifeHpi           string    `json:"midwifeHpi"`
	VisitNumber          *int16    `json:"visitNumber"`
	VisitType            string    `json:"visitType"`
	DaysPostnatal        *int16    `json:"daysPostnatal"`
	MotherWellbeing      *string   `json:"motherWellbeing"`
	BabyWellbeing        *string   `json:"babyWellbeing"`
	BreastfeedingSupport bool      `json:"breastfeedingSupport"`
	IssuesIdentified     *string   `json:"issuesIdentified"`
	Referrals            *string   `json:"referrals"`
	VisitedAt            time.Time `json:"visitedAt"`
	TenantID             string    `json:"tenantId"`
	CreatedAt            time.Time `json:"createdAt"`
}

// postnatalHandler manages postnatal checks and community midwife visits.
type postnatalHandler struct {
	handlerDeps
}

func (h *postnatalHandler) ListChecks(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, maternity_episode_id, clinician_hpi, check_type, check_subject,
		       maternal_bp_systolic, maternal_bp_diastolic, maternal_pulse_bpm,
		       maternal_temperature_c, maternal_fundal_height_cm, maternal_lochia,
		       maternal_perineum, maternal_mood, baby_weight_grams, baby_temperature_c,
		       baby_bilirubin_umol, baby_jaundice, baby_feeding_method, baby_feeding_issues,
		       baby_urine_output, baby_stool, notes, checked_at, tenant_id, created_at
		FROM postnatal_checks
		WHERE maternity_episode_id = @episode_id AND tenant_id = @tenant_id
		ORDER BY checked_at DESC
	`, pgx.NamedArgs{"episode_id": episodeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	checks := make([]PostnatalCheck, 0)
	for rows.Next() {
		var c PostnatalCheck
		if err := rows.Scan(
			&c.ID, &c.MaternityEpisodeID, &c.ClinicianHpi, &c.CheckType, &c.CheckSubject,
			&c.MaternalBpSystolic, &c.MaternalBpDiastolic, &c.MaternalPulseBpm,
			&c.MaternalTemperatureC, &c.MaternalFundalHeightCm, &c.MaternalLochia,
			&c.MaternalPerineum, &c.MaternalMood, &c.BabyWeightGrams, &c.BabyTemperatureC,
			&c.BabyBilirubinUmol, &c.BabyJaundice, &c.BabyFeedingMethod, &c.BabyFeedingIssues,
			&c.BabyUrineOutput, &c.BabyStool, &c.Notes, &c.CheckedAt, &c.TenantID, &c.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		checks = append(checks, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

func (h *postnatalHandler) CreateCheck(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	var req PostnatalCheck
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.CheckType == "" {
		req.CheckType = "routine"
	}
	if req.CheckSubject == "" {
		req.CheckSubject = "both"
	}
	if req.MaternalMood == "" {
		req.MaternalMood = "normal"
	}
	var c PostnatalCheck
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO postnatal_checks
		    (maternity_episode_id, clinician_hpi, check_type, check_subject,
		     maternal_bp_systolic, maternal_bp_diastolic, maternal_pulse_bpm,
		     maternal_temperature_c, maternal_fundal_height_cm, maternal_lochia,
		     maternal_perineum, maternal_mood, baby_weight_grams, baby_temperature_c,
		     baby_bilirubin_umol, baby_jaundice, baby_feeding_method, baby_feeding_issues,
		     baby_urine_output, baby_stool, notes, tenant_id)
		VALUES
		    (@episode_id, @clinician_hpi, @check_type, @check_subject,
		     @maternal_bp_systolic, @maternal_bp_diastolic, @maternal_pulse_bpm,
		     @maternal_temperature_c, @maternal_fundal_height_cm, @maternal_lochia,
		     @maternal_perineum, @maternal_mood, @baby_weight_grams, @baby_temperature_c,
		     @baby_bilirubin_umol, @baby_jaundice, @baby_feeding_method, @baby_feeding_issues,
		     @baby_urine_output, @baby_stool, @notes, @tenant_id)
		RETURNING id, maternity_episode_id, clinician_hpi, check_type, check_subject,
		          maternal_bp_systolic, maternal_bp_diastolic, maternal_pulse_bpm,
		          maternal_temperature_c, maternal_fundal_height_cm, maternal_lochia,
		          maternal_perineum, maternal_mood, baby_weight_grams, baby_temperature_c,
		          baby_bilirubin_umol, baby_jaundice, baby_feeding_method, baby_feeding_issues,
		          baby_urine_output, baby_stool, notes, checked_at, tenant_id, created_at
	`, pgx.NamedArgs{
		"episode_id":               episodeID,
		"clinician_hpi":            req.ClinicianHpi,
		"check_type":               req.CheckType,
		"check_subject":            req.CheckSubject,
		"maternal_bp_systolic":     req.MaternalBpSystolic,
		"maternal_bp_diastolic":    req.MaternalBpDiastolic,
		"maternal_pulse_bpm":       req.MaternalPulseBpm,
		"maternal_temperature_c":   req.MaternalTemperatureC,
		"maternal_fundal_height_cm": req.MaternalFundalHeightCm,
		"maternal_lochia":          req.MaternalLochia,
		"maternal_perineum":        req.MaternalPerineum,
		"maternal_mood":            req.MaternalMood,
		"baby_weight_grams":        req.BabyWeightGrams,
		"baby_temperature_c":       req.BabyTemperatureC,
		"baby_bilirubin_umol":      req.BabyBilirubinUmol,
		"baby_jaundice":            req.BabyJaundice,
		"baby_feeding_method":      req.BabyFeedingMethod,
		"baby_feeding_issues":      req.BabyFeedingIssues,
		"baby_urine_output":        req.BabyUrineOutput,
		"baby_stool":               req.BabyStool,
		"notes":                    req.Notes,
		"tenant_id":                tenantID,
	}).Scan(
		&c.ID, &c.MaternityEpisodeID, &c.ClinicianHpi, &c.CheckType, &c.CheckSubject,
		&c.MaternalBpSystolic, &c.MaternalBpDiastolic, &c.MaternalPulseBpm,
		&c.MaternalTemperatureC, &c.MaternalFundalHeightCm, &c.MaternalLochia,
		&c.MaternalPerineum, &c.MaternalMood, &c.BabyWeightGrams, &c.BabyTemperatureC,
		&c.BabyBilirubinUmol, &c.BabyJaundice, &c.BabyFeedingMethod, &c.BabyFeedingIssues,
		&c.BabyUrineOutput, &c.BabyStool, &c.Notes, &c.CheckedAt, &c.TenantID, &c.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *postnatalHandler) ListCommunityVisits(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, maternity_episode_id, midwife_hpi, visit_number, visit_type,
		       days_postnatal, mother_wellbeing, baby_wellbeing, breastfeeding_support,
		       issues_identified, referrals, visited_at, tenant_id, created_at
		FROM community_midwife_visits
		WHERE maternity_episode_id = @episode_id AND tenant_id = @tenant_id
		ORDER BY visited_at DESC
	`, pgx.NamedArgs{"episode_id": episodeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	visits := make([]CommunityMidwifeVisit, 0)
	for rows.Next() {
		var v CommunityMidwifeVisit
		if err := rows.Scan(
			&v.ID, &v.MaternityEpisodeID, &v.MidwifeHpi, &v.VisitNumber, &v.VisitType,
			&v.DaysPostnatal, &v.MotherWellbeing, &v.BabyWellbeing, &v.BreastfeedingSupport,
			&v.IssuesIdentified, &v.Referrals, &v.VisitedAt, &v.TenantID, &v.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		visits = append(visits, v)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, visits)
}

func (h *postnatalHandler) CreateCommunityVisit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	var req CommunityMidwifeVisit
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.VisitType == "" {
		req.VisitType = "home"
	}
	var v CommunityMidwifeVisit
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO community_midwife_visits
		    (maternity_episode_id, midwife_hpi, visit_number, visit_type,
		     days_postnatal, mother_wellbeing, baby_wellbeing, breastfeeding_support,
		     issues_identified, referrals, tenant_id)
		VALUES
		    (@episode_id, @midwife_hpi, @visit_number, @visit_type,
		     @days_postnatal, @mother_wellbeing, @baby_wellbeing, @breastfeeding_support,
		     @issues_identified, @referrals, @tenant_id)
		RETURNING id, maternity_episode_id, midwife_hpi, visit_number, visit_type,
		          days_postnatal, mother_wellbeing, baby_wellbeing, breastfeeding_support,
		          issues_identified, referrals, visited_at, tenant_id, created_at
	`, pgx.NamedArgs{
		"episode_id":           episodeID,
		"midwife_hpi":          req.MidwifeHpi,
		"visit_number":         req.VisitNumber,
		"visit_type":           req.VisitType,
		"days_postnatal":       req.DaysPostnatal,
		"mother_wellbeing":     req.MotherWellbeing,
		"baby_wellbeing":       req.BabyWellbeing,
		"breastfeeding_support": req.BreastfeedingSupport,
		"issues_identified":    req.IssuesIdentified,
		"referrals":            req.Referrals,
		"tenant_id":            tenantID,
	}).Scan(
		&v.ID, &v.MaternityEpisodeID, &v.MidwifeHpi, &v.VisitNumber, &v.VisitType,
		&v.DaysPostnatal, &v.MotherWellbeing, &v.BabyWellbeing, &v.BreastfeedingSupport,
		&v.IssuesIdentified, &v.Referrals, &v.VisitedAt, &v.TenantID, &v.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

// Discharge transitions the maternity episode from postnatal to completed.
func (h *postnatalHandler) Discharge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE maternity_episodes
		SET status = 'completed', updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'postnatal'
	`, pgx.NamedArgs{"id": episodeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "episode not found or not in postnatal status"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}
