package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/primhd"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-addiction/internal/methadone"
	"github.com/jackc/pgx/v5"
)

// Programme is the API representation of an OST programme (maps addiction_programmes).
type Programme struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"tenantId"`
	PatientNHI        string     `json:"patientNhi"`
	ClinicianID       string     `json:"clinicianId"`
	PracticeID        string     `json:"practiceId"`
	StartDate         time.Time  `json:"startDate"`
	EndDate           *time.Time `json:"endDate,omitempty"`
	Phase             string     `json:"phase"`
	SubstancePrimary  string     `json:"substancePrimary"`
	SubstanceOther    string     `json:"substanceOther,omitempty"`
	InitialDoseMg     float64    `json:"initialDoseMg"`
	CurrentDoseMg     float64    `json:"currentDoseMg"`
	TargetDoseMg      *float64   `json:"targetDoseMg,omitempty"`
	TakeHomeLevel     int        `json:"takeHomeLevel"`
	TakeHomeMaxDays   int        `json:"takeHomeMaxDays"`
	Pregnancy         bool       `json:"pregnancy"`
	Comorbidities     []string   `json:"comorbidities"`
	LastUrineDate     *time.Time `json:"lastUrineDate,omitempty"`
	NextReviewDate    time.Time  `json:"nextReviewDate"`
	// PRIMHDReferralID is the identifier issued by PRIMHD when the referral was
	// opened for this patient. Required for activity and discharge reporting.
	PRIMHDReferralID  string     `json:"primhdReferralId,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

const progSelectCols = `id, tenant_id, patient_nhi, clinician_id, practice_id,
       start_date, end_date, phase,
       substance_primary, COALESCE(substance_other,''),
       initial_dose_mg, current_dose_mg, target_dose_mg,
       take_home_level, take_home_max_days, pregnancy,
       COALESCE(comorbidities, '{}'), last_urine_date, next_review_date,
       COALESCE(primhd_referral_id,''),
       created_at, updated_at`

func scanProgramme(row interface{ Scan(...any) error }, p *Programme) error {
	return row.Scan(
		&p.ID, &p.TenantID, &p.PatientNHI, &p.ClinicianID, &p.PracticeID,
		&p.StartDate, &p.EndDate, &p.Phase,
		&p.SubstancePrimary, &p.SubstanceOther,
		&p.InitialDoseMg, &p.CurrentDoseMg, &p.TargetDoseMg,
		&p.TakeHomeLevel, &p.TakeHomeMaxDays, &p.Pregnancy,
		&p.Comorbidities, &p.LastUrineDate, &p.NextReviewDate,
		&p.PRIMHDReferralID,
		&p.CreatedAt, &p.UpdatedAt,
	)
}

// DoseRecord is the API representation of a methadone dose (maps methadone_doses).
type DoseRecord struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenantId"`
	ProgrammeID     string    `json:"programmeId"`
	AdministeredAt  time.Time `json:"administeredAt"`
	DoseMg          float64   `json:"doseMg"`
	Formulation     string    `json:"formulation"`
	WitnessedBy     string    `json:"witnessedBy"`
	DispensedBy     string    `json:"dispensedBy"`
	PharmacistCheck bool      `json:"pharmacistCheck"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes,omitempty"`
	TakeHome        bool      `json:"takeHome"`
	CreatedAt       time.Time `json:"createdAt"`
}

const doseSelectCols = `id, tenant_id, programme_id, administered_at, dose_mg,
       formulation, witnessed_by, dispensed_by, pharmacist_check,
       status, COALESCE(notes,''), take_home, created_at`

// TakeHomeApproval is the API representation of methadone_take_home_approvals.
type TakeHomeApproval struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenantId"`
	ProgrammeID   string     `json:"programmeId"`
	ApprovedBy    string     `json:"approvedBy"`
	ApprovedAt    time.Time  `json:"approvedAt"`
	Level         int        `json:"level"`
	MaxDays       int        `json:"maxDays"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	RevokedBy     string     `json:"revokedBy,omitempty"`
	RevokedReason string     `json:"revokedReason,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

const takeHomeSelectCols = `id, tenant_id, programme_id, approved_by, approved_at,
       level, max_days, expires_at, revoked_at,
       COALESCE(revoked_by,''), COALESCE(revoked_reason,''), created_at`

// UrineScreen is the API representation of a urine drug screen (maps urine_screens).
type UrineScreen struct {
	ID            string                  `json:"id"`
	TenantID      string                  `json:"tenantId"`
	ProgrammeID   string                  `json:"programmeId"`
	CollectedAt   time.Time               `json:"collectedAt"`
	CollectedBy   string                  `json:"collectedBy"`
	LabName       string                  `json:"labName,omitempty"`
	LabReference  string                  `json:"labReference,omitempty"`
	Results       []methadone.DrugResult  `json:"results"`
	MSSAResult    string                  `json:"mssaResult"`
	ClinicalNotes string                  `json:"clinicalNotes,omitempty"`
	CreatedAt     time.Time               `json:"createdAt"`
}

type methadoneHandler struct{ handlerDeps }

// ListProgrammes GET /api/v1/methadone/programmes
func (h *methadoneHandler) ListProgrammes(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	phaseFilter := r.URL.Query().Get("phase")
	var (
		rows pgx.Rows
		err  error
	)
	if phaseFilter != "" {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+progSelectCols+` FROM addiction_programmes
			 WHERE tenant_id = @tenant_id AND phase = @phase
			 ORDER BY start_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "phase": phaseFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+progSelectCols+` FROM addiction_programmes
			 WHERE tenant_id = @tenant_id
			 ORDER BY start_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	programmes := make([]Programme, 0)
	for rows.Next() {
		var p Programme
		if err := scanProgramme(rows, &p); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(p.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt NHI"})
			return
		}
		p.PatientNHI = nhi
		programmes = append(programmes, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, programmes)
}

// CreateProgramme POST /api/v1/methadone/programmes
func (h *methadoneHandler) CreateProgramme(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	var req Programme
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Phase == "" {
		req.Phase = string(methadone.PhaseInduction)
	}
	if req.TakeHomeLevel == 0 {
		req.TakeHomeLevel = 1
	}
	if !h.validateHPI(w, r, req.ClinicianID) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt NHI"})
		return
	}
	var p Programme
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO addiction_programmes
		    (tenant_id, patient_nhi, clinician_id, practice_id,
		     start_date, phase, substance_primary, substance_other,
		     initial_dose_mg, current_dose_mg, target_dose_mg,
		     take_home_level, take_home_max_days,
		     pregnancy, comorbidities, next_review_date)
		VALUES
		    (@tenant_id, @patient_nhi, @clinician_id, @practice_id,
		     COALESCE(@start_date, now()), @phase, @substance_primary, @substance_other,
		     @initial_dose_mg, @initial_dose_mg, @target_dose_mg,
		     @take_home_level, @take_home_max_days,
		     @pregnancy, @comorbidities,
		     COALESCE(@next_review_date, now() + interval '7 days'))
		RETURNING `+progSelectCols,
		pgx.NamedArgs{
			"tenant_id":         tenantID,
			"patient_nhi":       nhiEnc,
			"clinician_id":      req.ClinicianID,
			"practice_id":       req.PracticeID,
			"start_date":        req.StartDate,
			"phase":             req.Phase,
			"substance_primary": req.SubstancePrimary,
			"substance_other":   req.SubstanceOther,
			"initial_dose_mg":   req.InitialDoseMg,
			"target_dose_mg":    req.TargetDoseMg,
			"take_home_level":   req.TakeHomeLevel,
			"take_home_max_days": methadone.TakeHomeDays(req.TakeHomeLevel),
			"pregnancy":         req.Pregnancy,
			"comorbidities":     req.Comorbidities,
			"next_review_date":  req.NextReviewDate,
		}).Scan(
		&p.ID, &p.TenantID, &p.PatientNHI, &p.ClinicianID, &p.PracticeID,
		&p.StartDate, &p.EndDate, &p.Phase,
		&p.SubstancePrimary, &p.SubstanceOther,
		&p.InitialDoseMg, &p.CurrentDoseMg, &p.TargetDoseMg,
		&p.TakeHomeLevel, &p.TakeHomeMaxDays, &p.Pregnancy,
		&p.Comorbidities, &p.LastUrineDate, &p.NextReviewDate,
		&p.PRIMHDReferralID,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "AddictionProgramme", p.ID, p.PatientNHI)
	nhi, _ := h.decryptNHI(p.PatientNHI)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, p)
}

// GetProgramme GET /api/v1/methadone/programmes/{programmeId}
func (h *methadoneHandler) GetProgramme(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	id := r.PathValue("programmeId")
	var p Programme
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+progSelectCols+` FROM addiction_programmes WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.TenantID, &p.PatientNHI, &p.ClinicianID, &p.PracticeID,
		&p.StartDate, &p.EndDate, &p.Phase,
		&p.SubstancePrimary, &p.SubstanceOther,
		&p.InitialDoseMg, &p.CurrentDoseMg, &p.TargetDoseMg,
		&p.TakeHomeLevel, &p.TakeHomeMaxDays, &p.Pregnancy,
		&p.Comorbidities, &p.LastUrineDate, &p.NextReviewDate,
		&p.PRIMHDReferralID,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "programme not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "AddictionProgramme", p.ID, p.PatientNHI)
	nhi, _ := h.decryptNHI(p.PatientNHI)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

// UpdateProgramme PUT /api/v1/methadone/programmes/{programmeId}
func (h *methadoneHandler) UpdateProgramme(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	id := r.PathValue("programmeId")
	var req struct {
		Phase          string     `json:"phase"`
		CurrentDoseMg  float64    `json:"currentDoseMg"`
		TargetDoseMg   *float64   `json:"targetDoseMg"`
		NextReviewDate *time.Time `json:"nextReviewDate"`
		EndDate        *time.Time `json:"endDate"`
		Pregnancy      bool       `json:"pregnancy"`
		Comorbidities  []string   `json:"comorbidities"`
		ClinicianID    string     `json:"clinicianId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianID) {
		return
	}
	var nhiEnc string
	var p Programme
	err := h.pool.QueryRow(r.Context(), `
		UPDATE addiction_programmes
		SET phase            = COALESCE(NULLIF(@phase,''), phase),
		    clinician_id     = COALESCE(NULLIF(@clinician_id,''), clinician_id),
		    current_dose_mg  = CASE WHEN @current_dose_mg > 0 THEN @current_dose_mg ELSE current_dose_mg END,
		    target_dose_mg   = COALESCE(@target_dose_mg, target_dose_mg),
		    next_review_date = COALESCE(@next_review_date, next_review_date),
		    end_date         = @end_date,
		    pregnancy        = @pregnancy,
		    comorbidities    = COALESCE(@comorbidities, comorbidities),
		    updated_at       = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+progSelectCols,
		pgx.NamedArgs{
			"phase":           req.Phase,
			"clinician_id":    req.ClinicianID,
			"current_dose_mg": req.CurrentDoseMg,
			"target_dose_mg":  req.TargetDoseMg,
			"next_review_date": req.NextReviewDate,
			"end_date":        req.EndDate,
			"pregnancy":       req.Pregnancy,
			"comorbidities":   req.Comorbidities,
			"id":              id,
			"tenant_id":       tenantID,
		}).Scan(
		&p.ID, &p.TenantID, &nhiEnc, &p.ClinicianID, &p.PracticeID,
		&p.StartDate, &p.EndDate, &p.Phase,
		&p.SubstancePrimary, &p.SubstanceOther,
		&p.InitialDoseMg, &p.CurrentDoseMg, &p.TargetDoseMg,
		&p.TakeHomeLevel, &p.TakeHomeMaxDays, &p.Pregnancy,
		&p.Comorbidities, &p.LastUrineDate, &p.NextReviewDate,
		&p.PRIMHDReferralID,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "programme not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "AddictionProgramme", p.ID, nhiEnc)

	// PRIMHD reporting: when a programme is discharged, close the referral.
	// Required for all DHB-funded addiction services under PRIMHD obligations.
	if h.primhdClient != nil &&
		p.Phase == string(methadone.PhaseDischarged) &&
		p.EndDate != nil &&
		p.PRIMHDReferralID != "" {
		if _, err := h.primhdClient.CloseReferral(r.Context(), p.PRIMHDReferralID, *p.EndDate); err != nil {
			h.logger.Error("PRIMHD close referral failed",
				slog.String("programme", p.ID),
				slog.String("primhd_referral_id", p.PRIMHDReferralID),
				slog.Any("error", err),
			)
		} else {
			h.logger.Info("PRIMHD referral closed",
				slog.String("programme", p.ID),
				slog.String("primhd_referral_id", p.PRIMHDReferralID),
			)
		}
	}

	nhi, _ := h.decryptNHI(nhiEnc)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

// ListDoses GET /api/v1/methadone/programmes/{programmeId}/doses
func (h *methadoneHandler) ListDoses(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	programmeID := r.PathValue("programmeId")
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+doseSelectCols+` FROM methadone_doses
		 WHERE programme_id = @programme_id AND tenant_id = @tenant_id
		 ORDER BY administered_at DESC`,
		pgx.NamedArgs{"programme_id": programmeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	doses := make([]DoseRecord, 0)
	for rows.Next() {
		var d DoseRecord
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.ProgrammeID, &d.AdministeredAt, &d.DoseMg,
			&d.Formulation, &d.WitnessedBy, &d.DispensedBy, &d.PharmacistCheck,
			&d.Status, &d.Notes, &d.TakeHome, &d.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		doses = append(doses, d)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, doses)
}

// RecordDose POST /api/v1/methadone/programmes/{programmeId}/doses
func (h *methadoneHandler) RecordDose(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	programmeID := r.PathValue("programmeId")
	var req DoseRecord
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	// Look up patient NHI for audit from the programme record.
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM addiction_programmes WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": programmeID, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "programme not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	var d DoseRecord
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO methadone_doses
		    (tenant_id, programme_id, administered_at, dose_mg, formulation,
		     witnessed_by, dispensed_by, pharmacist_check, status, notes, take_home)
		VALUES
		    (@tenant_id, @programme_id, COALESCE(@administered_at, now()), @dose_mg, @formulation,
		     @witnessed_by, @dispensed_by, @pharmacist_check, @status, @notes, @take_home)
		RETURNING `+doseSelectCols,
		pgx.NamedArgs{
			"tenant_id":        tenantID,
			"programme_id":     programmeID,
			"administered_at":  req.AdministeredAt,
			"dose_mg":          req.DoseMg,
			"formulation":      req.Formulation,
			"witnessed_by":     req.WitnessedBy,
			"dispensed_by":     req.DispensedBy,
			"pharmacist_check": req.PharmacistCheck,
			"status":           req.Status,
			"notes":            req.Notes,
			"take_home":        req.TakeHome,
		}).Scan(
		&d.ID, &d.TenantID, &d.ProgrammeID, &d.AdministeredAt, &d.DoseMg,
		&d.Formulation, &d.WitnessedBy, &d.DispensedBy, &d.PharmacistCheck,
		&d.Status, &d.Notes, &d.TakeHome, &d.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "MethadoneDose", d.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, d)
}

// GetDose GET /api/v1/methadone/programmes/{programmeId}/doses/{doseId}
func (h *methadoneHandler) GetDose(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	doseID := r.PathValue("doseId")
	programmeID := r.PathValue("programmeId")
	var d DoseRecord
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+doseSelectCols+` FROM methadone_doses
		 WHERE id = @id AND programme_id = @programme_id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": doseID, "programme_id": programmeID, "tenant_id": tenantID}).Scan(
		&d.ID, &d.TenantID, &d.ProgrammeID, &d.AdministeredAt, &d.DoseMg,
		&d.Formulation, &d.WitnessedBy, &d.DispensedBy, &d.PharmacistCheck,
		&d.Status, &d.Notes, &d.TakeHome, &d.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "dose not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// ApproveTakeHome POST /api/v1/methadone/programmes/{programmeId}/take-home
func (h *methadoneHandler) ApproveTakeHome(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	programmeID := r.PathValue("programmeId")
	var req struct {
		Level     int        `json:"level"`
		ApprovedBy string    `json:"approvedBy"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Level < 1 || req.Level > 5 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_LEVEL", Message: "take-home level must be between 1 and 5"})
		return
	}
	maxDays := methadone.TakeHomeDays(req.Level)
	// Fetch NHI for audit.
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM addiction_programmes WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": programmeID, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "programme not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	// Insert approval and update the programme's level atomically.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	var th TakeHomeApproval
	err = tx.QueryRow(r.Context(), `
		INSERT INTO methadone_take_home_approvals
		    (tenant_id, programme_id, approved_by, level, max_days, expires_at)
		VALUES
		    (@tenant_id, @programme_id, @approved_by, @level, @max_days, @expires_at)
		RETURNING `+takeHomeSelectCols,
		pgx.NamedArgs{
			"tenant_id":    tenantID,
			"programme_id": programmeID,
			"approved_by":  req.ApprovedBy,
			"level":        req.Level,
			"max_days":     maxDays,
			"expires_at":   req.ExpiresAt,
		}).Scan(
		&th.ID, &th.TenantID, &th.ProgrammeID, &th.ApprovedBy, &th.ApprovedAt,
		&th.Level, &th.MaxDays, &th.ExpiresAt, &th.RevokedAt,
		&th.RevokedBy, &th.RevokedReason, &th.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE addiction_programmes
		 SET take_home_level = @level, take_home_max_days = @max_days, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"level": req.Level, "max_days": maxDays, "id": programmeID, "tenant_id": tenantID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "TakeHomeApproval", th.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, th)
}

// ListTakeHome GET /api/v1/methadone/programmes/{programmeId}/take-home
func (h *methadoneHandler) ListTakeHome(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	programmeID := r.PathValue("programmeId")
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+takeHomeSelectCols+` FROM methadone_take_home_approvals
		 WHERE programme_id = @programme_id AND tenant_id = @tenant_id
		 ORDER BY approved_at DESC`,
		pgx.NamedArgs{"programme_id": programmeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	approvals := make([]TakeHomeApproval, 0)
	for rows.Next() {
		var th TakeHomeApproval
		if err := rows.Scan(
			&th.ID, &th.TenantID, &th.ProgrammeID, &th.ApprovedBy, &th.ApprovedAt,
			&th.Level, &th.MaxDays, &th.ExpiresAt, &th.RevokedAt,
			&th.RevokedBy, &th.RevokedReason, &th.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		approvals = append(approvals, th)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, approvals)
}

// ListUrineScreens GET /api/v1/methadone/programmes/{programmeId}/urine-screens
func (h *methadoneHandler) ListUrineScreens(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	programmeID := r.PathValue("programmeId")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, tenant_id, programme_id, collected_at, collected_by,
		        COALESCE(lab_name,''), COALESCE(lab_reference,''),
		        results, mssa_result, COALESCE(clinical_notes,''), created_at
		 FROM urine_screens
		 WHERE programme_id = @programme_id AND tenant_id = @tenant_id
		 ORDER BY collected_at DESC`,
		pgx.NamedArgs{"programme_id": programmeID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	screens := make([]UrineScreen, 0)
	for rows.Next() {
		var us UrineScreen
		var resultsJSON []byte
		if err := rows.Scan(
			&us.ID, &us.TenantID, &us.ProgrammeID, &us.CollectedAt, &us.CollectedBy,
			&us.LabName, &us.LabReference,
			&resultsJSON, &us.MSSAResult, &us.ClinicalNotes, &us.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		_ = json.Unmarshal(resultsJSON, &us.Results)
		screens = append(screens, us)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, screens)
}

// RecordUrineScreen POST /api/v1/methadone/programmes/{programmeId}/urine-screens
func (h *methadoneHandler) RecordUrineScreen(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	programmeID := r.PathValue("programmeId")
	var req UrineScreen
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM addiction_programmes WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": programmeID, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "programme not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	resultsJSON, err := json.Marshal(req.Results)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCODE_ERROR", Message: err.Error()})
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	var us UrineScreen
	var resultsBack []byte
	err = tx.QueryRow(r.Context(), `
		INSERT INTO urine_screens
		    (tenant_id, programme_id, collected_at, collected_by,
		     lab_name, lab_reference, results, mssa_result, clinical_notes)
		VALUES
		    (@tenant_id, @programme_id, COALESCE(@collected_at, now()), @collected_by,
		     @lab_name, @lab_reference, @results, @mssa_result, @clinical_notes)
		RETURNING id, tenant_id, programme_id, collected_at, collected_by,
		          COALESCE(lab_name,''), COALESCE(lab_reference,''),
		          results, mssa_result, COALESCE(clinical_notes,''), created_at`,
		pgx.NamedArgs{
			"tenant_id":      tenantID,
			"programme_id":   programmeID,
			"collected_at":   req.CollectedAt,
			"collected_by":   req.CollectedBy,
			"lab_name":       req.LabName,
			"lab_reference":  req.LabReference,
			"results":        resultsJSON,
			"mssa_result":    req.MSSAResult,
			"clinical_notes": req.ClinicalNotes,
		}).Scan(
		&us.ID, &us.TenantID, &us.ProgrammeID, &us.CollectedAt, &us.CollectedBy,
		&us.LabName, &us.LabReference,
		&resultsBack, &us.MSSAResult, &us.ClinicalNotes, &us.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	_ = json.Unmarshal(resultsBack, &us.Results)
	// Update last_urine_date on the programme.
	if _, err := tx.Exec(r.Context(),
		`UPDATE addiction_programmes SET last_urine_date = now(), updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": programmeID, "tenant_id": tenantID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "UrineScreen", us.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, us)
}

// GetUrineScreen GET /api/v1/methadone/programmes/{programmeId}/urine-screens/{screenId}
func (h *methadoneHandler) GetUrineScreen(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	screenID := r.PathValue("screenId")
	programmeID := r.PathValue("programmeId")
	var us UrineScreen
	var resultsJSON []byte
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, tenant_id, programme_id, collected_at, collected_by,
		        COALESCE(lab_name,''), COALESCE(lab_reference,''),
		        results, mssa_result, COALESCE(clinical_notes,''), created_at
		 FROM urine_screens
		 WHERE id = @id AND programme_id = @programme_id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": screenID, "programme_id": programmeID, "tenant_id": tenantID}).Scan(
		&us.ID, &us.TenantID, &us.ProgrammeID, &us.CollectedAt, &us.CollectedBy,
		&us.LabName, &us.LabReference,
		&resultsJSON, &us.MSSAResult, &us.ClinicalNotes, &us.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "urine screen not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	_ = json.Unmarshal(resultsJSON, &us.Results)
	writeJSON(w, http.StatusOK, us)
}
