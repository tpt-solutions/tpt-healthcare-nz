package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// AntenatalVisitType classifies the type of antenatal appointment.
type AntenatalVisitType string

const (
	AntenatalVisitTypeBooking    AntenatalVisitType = "booking"
	AntenatalVisitTypeRoutine    AntenatalVisitType = "routine"
	AntenatalVisitTypeAdditional AntenatalVisitType = "additional"
	AntenatalVisitTypeSpecialist AntenatalVisitType = "specialist"
)

// ScanType classifies the purpose of an obstetric ultrasound.
type ScanType string

const (
	ScanTypeDating                 ScanType = "dating"
	ScanTypeCombinedFirstTrimester ScanType = "combined-first-trimester"
	ScanTypeMorphology             ScanType = "morphology"
	ScanTypeGrowth                 ScanType = "growth"
	ScanTypeWellbeing              ScanType = "wellbeing"
	ScanTypeOther                  ScanType = "other"
)

// ScreenType classifies NZ maternal screening tests.
type ScreenType string

const (
	ScreenTypeCombinedFirstTrimester ScreenType = "combined-first-trimester"
	ScreenTypeNIPT                   ScreenType = "NIPT"
	ScreenTypeGDM50g                 ScreenType = "GDM-50g"
	ScreenTypeGDM75g                 ScreenType = "GDM-75g"
	ScreenTypeGBS                    ScreenType = "GBS"
	ScreenTypeGroupAndHold           ScreenType = "group-and-hold"
	ScreenTypeRhesus                 ScreenType = "rhesus"
	ScreenTypeRubella                ScreenType = "rubella"
	ScreenTypeSyphilis               ScreenType = "syphilis"
	ScreenTypeHIV                    ScreenType = "HIV"
	ScreenTypeHepB                   ScreenType = "hep-b"
	ScreenTypeHepC                   ScreenType = "hep-c"
	ScreenTypeChlamydia              ScreenType = "chlamydia"
	ScreenTypeOther                  ScreenType = "other"
)

type AntenatalVisit struct {
	ID                string    `json:"id"`
	EpisodeID         string    `json:"episodeId"`
	ClinicianHpi      string    `json:"clinicianHpi"`
	VisitType         string    `json:"visitType"`
	GestationWeeks    *int16    `json:"gestationWeeks"`
	BpSystolic        *int16    `json:"bpSystolic"`
	BpDiastolic       *int16    `json:"bpDiastolic"`
	WeightKg          *float64  `json:"weightKg"`
	FundalHeightCm    *float64  `json:"fundalHeightCm"`
	FetalPresentation *string   `json:"fetalPresentation"`
	FetalHeartRateBpm *int16    `json:"fetalHeartRateBpm"`
	UrinalysisProtein *string   `json:"urinalysisProtein"`
	UrinalysisGlucose *string   `json:"urinalysisGlucose"`
	Oedema            string    `json:"oedema"`
	Notes             *string   `json:"notes"`
	TenantID          string    `json:"tenantId"`
	VisitedAt         time.Time `json:"visitedAt"`
	CreatedAt         time.Time `json:"createdAt"`
}

type AntenatalScan struct {
	ID                    string    `json:"id"`
	EpisodeID             string    `json:"episodeId"`
	ScanType              string    `json:"scanType"`
	GestationWeeks        *int16    `json:"gestationWeeks"`
	GestationDays         *int16    `json:"gestationDays"`
	EstimatedFetalWeightG *int      `json:"estimatedFetalWeightG"`
	Presentation          *string   `json:"presentation"`
	Liquor                *string   `json:"liquor"`
	PlacentaPosition      *string   `json:"placentaPosition"`
	CervicalLengthMm      *float64  `json:"cervicalLengthMm"`
	Findings              *string   `json:"findings"`
	SonographerHpi        string    `json:"sonographerHpi"`
	ScannedAt             time.Time `json:"scannedAt"`
	TenantID              string    `json:"tenantId"`
	CreatedAt             time.Time `json:"createdAt"`
}

type AntenatalScreening struct {
	ID          string     `json:"id"`
	EpisodeID   string     `json:"episodeId"`
	ScreenType  string     `json:"screenType"`
	Result      *string    `json:"result"`
	ResultValue *float64   `json:"resultValue"`
	ResultUnit  *string    `json:"resultUnit"`
	RiskScore   *string    `json:"riskScore"`
	HighRisk    bool       `json:"highRisk"`
	CollectedAt *time.Time `json:"collectedAt"`
	ReportedAt  *time.Time `json:"reportedAt"`
	Notes       *string    `json:"notes"`
	TenantID    string     `json:"tenantId"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// antenatalHandler manages antenatal visits, ultrasound scans, and maternal screening.
type antenatalHandler struct {
	handlerDeps
}

func (h *antenatalHandler) ListVisits(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, episode_id, clinician_hpi, visit_type, gestation_weeks,
		       bp_systolic, bp_diastolic, weight_kg, fundal_height_cm, fetal_presentation,
		       fetal_heart_rate_bpm, urinalysis_protein, urinalysis_glucose, oedema, notes,
		       tenant_id, visited_at, created_at
		FROM antenatal_visits
		WHERE episode_id = @episode_id AND tenant_id = @tenant_id
		ORDER BY visited_at DESC
	`, pgx.NamedArgs{"episode_id": episodeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	visits := make([]AntenatalVisit, 0)
	for rows.Next() {
		var v AntenatalVisit
		if err := rows.Scan(
			&v.ID, &v.EpisodeID, &v.ClinicianHpi, &v.VisitType, &v.GestationWeeks,
			&v.BpSystolic, &v.BpDiastolic, &v.WeightKg, &v.FundalHeightCm, &v.FetalPresentation,
			&v.FetalHeartRateBpm, &v.UrinalysisProtein, &v.UrinalysisGlucose, &v.Oedema, &v.Notes,
			&v.TenantID, &v.VisitedAt, &v.CreatedAt,
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

func (h *antenatalHandler) CreateVisit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	var req AntenatalVisit
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Oedema == "" {
		req.Oedema = "none"
	}
	if req.VisitType == "" {
		req.VisitType = "routine"
	}
	var v AntenatalVisit
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO antenatal_visits
		    (episode_id, clinician_hpi, visit_type, gestation_weeks,
		     bp_systolic, bp_diastolic, weight_kg, fundal_height_cm, fetal_presentation,
		     fetal_heart_rate_bpm, urinalysis_protein, urinalysis_glucose, oedema, notes, tenant_id)
		VALUES
		    (@episode_id, @clinician_hpi, @visit_type, @gestation_weeks,
		     @bp_systolic, @bp_diastolic, @weight_kg, @fundal_height_cm, @fetal_presentation,
		     @fetal_heart_rate_bpm, @urinalysis_protein, @urinalysis_glucose, @oedema, @notes, @tenant_id)
		RETURNING id, episode_id, clinician_hpi, visit_type, gestation_weeks,
		          bp_systolic, bp_diastolic, weight_kg, fundal_height_cm, fetal_presentation,
		          fetal_heart_rate_bpm, urinalysis_protein, urinalysis_glucose, oedema, notes,
		          tenant_id, visited_at, created_at
	`, pgx.NamedArgs{
		"episode_id":           episodeID,
		"clinician_hpi":        req.ClinicianHpi,
		"visit_type":           req.VisitType,
		"gestation_weeks":      req.GestationWeeks,
		"bp_systolic":          req.BpSystolic,
		"bp_diastolic":         req.BpDiastolic,
		"weight_kg":            req.WeightKg,
		"fundal_height_cm":     req.FundalHeightCm,
		"fetal_presentation":   req.FetalPresentation,
		"fetal_heart_rate_bpm": req.FetalHeartRateBpm,
		"urinalysis_protein":   req.UrinalysisProtein,
		"urinalysis_glucose":   req.UrinalysisGlucose,
		"oedema":               req.Oedema,
		"notes":                req.Notes,
		"tenant_id":            tenantID,
	}).Scan(
		&v.ID, &v.EpisodeID, &v.ClinicianHpi, &v.VisitType, &v.GestationWeeks,
		&v.BpSystolic, &v.BpDiastolic, &v.WeightKg, &v.FundalHeightCm, &v.FetalPresentation,
		&v.FetalHeartRateBpm, &v.UrinalysisProtein, &v.UrinalysisGlucose, &v.Oedema, &v.Notes,
		&v.TenantID, &v.VisitedAt, &v.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h *antenatalHandler) GetVisit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	visitID := r.PathValue("visitId")
	var v AntenatalVisit
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, episode_id, clinician_hpi, visit_type, gestation_weeks,
		       bp_systolic, bp_diastolic, weight_kg, fundal_height_cm, fetal_presentation,
		       fetal_heart_rate_bpm, urinalysis_protein, urinalysis_glucose, oedema, notes,
		       tenant_id, visited_at, created_at
		FROM antenatal_visits
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": visitID, "tenant_id": tenantID}).Scan(
		&v.ID, &v.EpisodeID, &v.ClinicianHpi, &v.VisitType, &v.GestationWeeks,
		&v.BpSystolic, &v.BpDiastolic, &v.WeightKg, &v.FundalHeightCm, &v.FetalPresentation,
		&v.FetalHeartRateBpm, &v.UrinalysisProtein, &v.UrinalysisGlucose, &v.Oedema, &v.Notes,
		&v.TenantID, &v.VisitedAt, &v.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "visit not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *antenatalHandler) ListScans(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, episode_id, scan_type, gestation_weeks, gestation_days,
		       estimated_fetal_weight_g, presentation, liquor, placenta_position,
		       cervical_length_mm, findings, sonographer_hpi, scanned_at, tenant_id, created_at
		FROM antenatal_scans
		WHERE episode_id = @episode_id AND tenant_id = @tenant_id
		ORDER BY scanned_at DESC
	`, pgx.NamedArgs{"episode_id": episodeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	scans := make([]AntenatalScan, 0)
	for rows.Next() {
		var s AntenatalScan
		if err := rows.Scan(
			&s.ID, &s.EpisodeID, &s.ScanType, &s.GestationWeeks, &s.GestationDays,
			&s.EstimatedFetalWeightG, &s.Presentation, &s.Liquor, &s.PlacentaPosition,
			&s.CervicalLengthMm, &s.Findings, &s.SonographerHpi, &s.ScannedAt, &s.TenantID, &s.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		scans = append(scans, s)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, scans)
}

func (h *antenatalHandler) CreateScan(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	var req AntenatalScan
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var s AntenatalScan
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO antenatal_scans
		    (episode_id, scan_type, gestation_weeks, gestation_days,
		     estimated_fetal_weight_g, presentation, liquor, placenta_position,
		     cervical_length_mm, findings, sonographer_hpi, tenant_id)
		VALUES
		    (@episode_id, @scan_type, @gestation_weeks, @gestation_days,
		     @efw, @presentation, @liquor, @placenta, @cervical_length, @findings, @sonographer_hpi, @tenant_id)
		RETURNING id, episode_id, scan_type, gestation_weeks, gestation_days,
		          estimated_fetal_weight_g, presentation, liquor, placenta_position,
		          cervical_length_mm, findings, sonographer_hpi, scanned_at, tenant_id, created_at
	`, pgx.NamedArgs{
		"episode_id":      episodeID,
		"scan_type":       req.ScanType,
		"gestation_weeks": req.GestationWeeks,
		"gestation_days":  req.GestationDays,
		"efw":             req.EstimatedFetalWeightG,
		"presentation":    req.Presentation,
		"liquor":          req.Liquor,
		"placenta":        req.PlacentaPosition,
		"cervical_length": req.CervicalLengthMm,
		"findings":        req.Findings,
		"sonographer_hpi": req.SonographerHpi,
		"tenant_id":       tenantID,
	}).Scan(
		&s.ID, &s.EpisodeID, &s.ScanType, &s.GestationWeeks, &s.GestationDays,
		&s.EstimatedFetalWeightG, &s.Presentation, &s.Liquor, &s.PlacentaPosition,
		&s.CervicalLengthMm, &s.Findings, &s.SonographerHpi, &s.ScannedAt, &s.TenantID, &s.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *antenatalHandler) ListScreening(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, episode_id, screen_type, result, result_value, result_unit,
		       risk_score, high_risk, collected_at, reported_at, notes, tenant_id, created_at
		FROM antenatal_screening
		WHERE episode_id = @episode_id AND tenant_id = @tenant_id
		ORDER BY created_at DESC
	`, pgx.NamedArgs{"episode_id": episodeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	screens := make([]AntenatalScreening, 0)
	for rows.Next() {
		var s AntenatalScreening
		if err := rows.Scan(
			&s.ID, &s.EpisodeID, &s.ScreenType, &s.Result, &s.ResultValue, &s.ResultUnit,
			&s.RiskScore, &s.HighRisk, &s.CollectedAt, &s.ReportedAt, &s.Notes, &s.TenantID, &s.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		screens = append(screens, s)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, screens)
}

func (h *antenatalHandler) CreateScreening(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	var req AntenatalScreening
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ScreenType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "screenType is required"})
		return
	}
	var s AntenatalScreening
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO antenatal_screening
		    (episode_id, screen_type, result, result_value, result_unit,
		     risk_score, high_risk, collected_at, reported_at, notes, tenant_id)
		VALUES
		    (@episode_id, @screen_type, @result, @result_value, @result_unit,
		     @risk_score, @high_risk, @collected_at, @reported_at, @notes, @tenant_id)
		RETURNING id, episode_id, screen_type, result, result_value, result_unit,
		          risk_score, high_risk, collected_at, reported_at, notes, tenant_id, created_at
	`, pgx.NamedArgs{
		"episode_id":   episodeID,
		"screen_type":  req.ScreenType,
		"result":       req.Result,
		"result_value": req.ResultValue,
		"result_unit":  req.ResultUnit,
		"risk_score":   req.RiskScore,
		"high_risk":    req.HighRisk,
		"collected_at": req.CollectedAt,
		"reported_at":  req.ReportedAt,
		"notes":        req.Notes,
		"tenant_id":    tenantID,
	}).Scan(
		&s.ID, &s.EpisodeID, &s.ScreenType, &s.Result, &s.ResultValue, &s.ResultUnit,
		&s.RiskScore, &s.HighRisk, &s.CollectedAt, &s.ReportedAt, &s.Notes, &s.TenantID, &s.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, s)
}
