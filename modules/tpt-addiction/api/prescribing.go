package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// OSTPrescription maps ost_prescriptions.
type OSTPrescription struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenantId"`
	PatientNHI    string     `json:"patientNhi"`
	ProgrammeID   *string    `json:"programmeId,omitempty"`
	PrescriberID  string     `json:"prescriberId"`
	PracticeID    string     `json:"practiceId"`
	Drug          string     `json:"drug"`
	DoseMg        float64    `json:"doseMg"`
	Formulation   string     `json:"formulation"`
	Frequency     string     `json:"frequency"`
	Supervised    bool       `json:"supervised"`
	TakeHomeDays  int        `json:"takeHomeDays"`
	StartDate     time.Time  `json:"startDate"`
	EndDate       *time.Time `json:"endDate,omitempty"`
	Status        string     `json:"status"`
	Indication    string     `json:"indication"`
	ClinicalNotes string     `json:"clinicalNotes,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

const prescriptionSelectCols = `id, tenant_id, patient_nhi, programme_id, prescriber_id, practice_id,
       drug, dose_mg, formulation, frequency, supervised, take_home_days,
       start_date, end_date, status, indication, COALESCE(clinical_notes,''),
       created_at, updated_at`

func scanPrescription(row interface{ Scan(...any) error }, p *OSTPrescription) error {
	return row.Scan(
		&p.ID, &p.TenantID, &p.PatientNHI, &p.ProgrammeID, &p.PrescriberID, &p.PracticeID,
		&p.Drug, &p.DoseMg, &p.Formulation, &p.Frequency, &p.Supervised, &p.TakeHomeDays,
		&p.StartDate, &p.EndDate, &p.Status, &p.Indication, &p.ClinicalNotes,
		&p.CreatedAt, &p.UpdatedAt,
	)
}

// DoseAdjustment maps ost_dose_adjustments.
type DoseAdjustment struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenantId"`
	PrescriptionID string    `json:"prescriptionId"`
	AdjustedBy     string    `json:"adjustedBy"`
	AdjustedAt     time.Time `json:"adjustedAt"`
	PreviousDoseMg float64   `json:"previousDoseMg"`
	NewDoseMg      float64   `json:"newDoseMg"`
	Reason         string    `json:"reason"`
	ClinicalNotes  string    `json:"clinicalNotes,omitempty"`
	WitnessedBy    string    `json:"witnessedBy,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

const adjustmentSelectCols = `id, tenant_id, prescription_id, adjusted_by, adjusted_at,
       previous_dose_mg, new_dose_mg, reason,
       COALESCE(clinical_notes,''), COALESCE(witnessed_by,''), created_at`

type prescribingHandler struct{ handlerDeps }

// ListPrescriptions GET /api/v1/ost/prescriptions
func (h *prescribingHandler) ListPrescriptions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	statusFilter := r.URL.Query().Get("status")
	var (
		rows pgx.Rows
		err  error
	)
	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+prescriptionSelectCols+` FROM ost_prescriptions
			 WHERE tenant_id = @tenant_id AND status = @status
			 ORDER BY start_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+prescriptionSelectCols+` FROM ost_prescriptions
			 WHERE tenant_id = @tenant_id
			 ORDER BY start_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	prescriptions := make([]OSTPrescription, 0)
	for rows.Next() {
		var p OSTPrescription
		if err := scanPrescription(rows, &p); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, _ := h.decryptNHI(p.PatientNHI)
		p.PatientNHI = nhi
		prescriptions = append(prescriptions, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, prescriptions)
}

// CreatePrescription POST /api/v1/ost/prescriptions
func (h *prescribingHandler) CreatePrescription(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	var req OSTPrescription
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Frequency == "" {
		req.Frequency = "daily"
	}
	if req.Indication == "" {
		req.Indication = "opioid_dependence"
	}
	if !h.validateHPI(w, r, req.PrescriberID) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt NHI"})
		return
	}
	var p OSTPrescription
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO ost_prescriptions
		    (tenant_id, patient_nhi, programme_id, prescriber_id, practice_id,
		     drug, dose_mg, formulation, frequency, supervised, take_home_days,
		     start_date, indication, clinical_notes)
		VALUES
		    (@tenant_id, @patient_nhi, @programme_id, @prescriber_id, @practice_id,
		     @drug, @dose_mg, @formulation, @frequency, @supervised, @take_home_days,
		     COALESCE(@start_date, now()), @indication, @clinical_notes)
		RETURNING `+prescriptionSelectCols,
		pgx.NamedArgs{
			"tenant_id":      tenantID,
			"patient_nhi":    nhiEnc,
			"programme_id":   req.ProgrammeID,
			"prescriber_id":  req.PrescriberID,
			"practice_id":    req.PracticeID,
			"drug":           req.Drug,
			"dose_mg":        req.DoseMg,
			"formulation":    req.Formulation,
			"frequency":      req.Frequency,
			"supervised":     req.Supervised,
			"take_home_days": req.TakeHomeDays,
			"start_date":     req.StartDate,
			"indication":     req.Indication,
			"clinical_notes": req.ClinicalNotes,
		}).Scan(
		&p.ID, &p.TenantID, &p.PatientNHI, &p.ProgrammeID, &p.PrescriberID, &p.PracticeID,
		&p.Drug, &p.DoseMg, &p.Formulation, &p.Frequency, &p.Supervised, &p.TakeHomeDays,
		&p.StartDate, &p.EndDate, &p.Status, &p.Indication, &p.ClinicalNotes,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "OSTPrescription", p.ID, p.PatientNHI)
	nhi, _ := h.decryptNHI(p.PatientNHI)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, p)
}

// GetPrescription GET /api/v1/ost/prescriptions/{prescriptionId}
func (h *prescribingHandler) GetPrescription(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	id := r.PathValue("prescriptionId")
	var p OSTPrescription
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+prescriptionSelectCols+` FROM ost_prescriptions
		 WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.TenantID, &p.PatientNHI, &p.ProgrammeID, &p.PrescriberID, &p.PracticeID,
		&p.Drug, &p.DoseMg, &p.Formulation, &p.Frequency, &p.Supervised, &p.TakeHomeDays,
		&p.StartDate, &p.EndDate, &p.Status, &p.Indication, &p.ClinicalNotes,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "prescription not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "OSTPrescription", p.ID, p.PatientNHI)
	nhi, _ := h.decryptNHI(p.PatientNHI)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

// UpdatePrescription PUT /api/v1/ost/prescriptions/{prescriptionId}
func (h *prescribingHandler) UpdatePrescription(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	id := r.PathValue("prescriptionId")
	var req struct {
		Status        string     `json:"status"`
		EndDate       *time.Time `json:"endDate"`
		ClinicalNotes string     `json:"clinicalNotes"`
		Supervised    *bool      `json:"supervised"`
		TakeHomeDays  *int       `json:"takeHomeDays"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var nhiEnc string
	var p OSTPrescription
	err := h.pool.QueryRow(r.Context(), `
		UPDATE ost_prescriptions
		SET status         = COALESCE(NULLIF(@status,''), status),
		    end_date       = COALESCE(@end_date, end_date),
		    clinical_notes = COALESCE(NULLIF(@clinical_notes,''), clinical_notes),
		    supervised     = COALESCE(@supervised, supervised),
		    take_home_days = COALESCE(@take_home_days, take_home_days),
		    updated_at     = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+prescriptionSelectCols,
		pgx.NamedArgs{
			"status":         req.Status,
			"end_date":       req.EndDate,
			"clinical_notes": req.ClinicalNotes,
			"supervised":     req.Supervised,
			"take_home_days": req.TakeHomeDays,
			"id":             id,
			"tenant_id":      tenantID,
		}).Scan(
		&p.ID, &p.TenantID, &nhiEnc, &p.ProgrammeID, &p.PrescriberID, &p.PracticeID,
		&p.Drug, &p.DoseMg, &p.Formulation, &p.Frequency, &p.Supervised, &p.TakeHomeDays,
		&p.StartDate, &p.EndDate, &p.Status, &p.Indication, &p.ClinicalNotes,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "prescription not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "OSTPrescription", p.ID, nhiEnc)
	nhi, _ := h.decryptNHI(nhiEnc)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

// AdjustDose POST /api/v1/ost/prescriptions/{prescriptionId}/adjust
func (h *prescribingHandler) AdjustDose(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	prescriptionID := r.PathValue("prescriptionId")
	var req DoseAdjustment
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.AdjustedBy) {
		return
	}
	// Fetch NHI for audit and verify prescription exists.
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM ost_prescriptions WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": prescriptionID, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "prescription not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	var a DoseAdjustment
	err = tx.QueryRow(r.Context(), `
		INSERT INTO ost_dose_adjustments
		    (tenant_id, prescription_id, adjusted_by, previous_dose_mg,
		     new_dose_mg, reason, clinical_notes, witnessed_by)
		VALUES
		    (@tenant_id, @prescription_id, @adjusted_by, @previous_dose_mg,
		     @new_dose_mg, @reason, @clinical_notes, @witnessed_by)
		RETURNING `+adjustmentSelectCols,
		pgx.NamedArgs{
			"tenant_id":        tenantID,
			"prescription_id":  prescriptionID,
			"adjusted_by":      req.AdjustedBy,
			"previous_dose_mg": req.PreviousDoseMg,
			"new_dose_mg":      req.NewDoseMg,
			"reason":           req.Reason,
			"clinical_notes":   req.ClinicalNotes,
			"witnessed_by":     req.WitnessedBy,
		}).Scan(
		&a.ID, &a.TenantID, &a.PrescriptionID, &a.AdjustedBy, &a.AdjustedAt,
		&a.PreviousDoseMg, &a.NewDoseMg, &a.Reason,
		&a.ClinicalNotes, &a.WitnessedBy, &a.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	// Apply the new dose to the prescription itself.
	if _, err := tx.Exec(r.Context(),
		`UPDATE ost_prescriptions SET dose_mg = @dose_mg, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"dose_mg": req.NewDoseMg, "id": prescriptionID, "tenant_id": tenantID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	// Mirror current_dose_mg on the linked programme if present.
	if _, err := tx.Exec(r.Context(), `
		UPDATE addiction_programmes
		SET current_dose_mg = @dose_mg, updated_at = now()
		WHERE id = (SELECT programme_id FROM ost_prescriptions WHERE id = @rx_id)
		  AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"dose_mg": req.NewDoseMg, "rx_id": prescriptionID, "tenant_id": tenantID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "OSTPrescription", prescriptionID, nhiEnc)
	writeJSON(w, http.StatusCreated, a)
}

// ListAdjustments GET /api/v1/ost/prescriptions/{prescriptionId}/adjustments
func (h *prescribingHandler) ListAdjustments(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(w, r)
	if !ok {
		return
	}
	prescriptionID := r.PathValue("prescriptionId")
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+adjustmentSelectCols+` FROM ost_dose_adjustments
		 WHERE prescription_id = @prescription_id AND tenant_id = @tenant_id
		 ORDER BY adjusted_at DESC`,
		pgx.NamedArgs{"prescription_id": prescriptionID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	adjustments := make([]DoseAdjustment, 0)
	for rows.Next() {
		var a DoseAdjustment
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.PrescriptionID, &a.AdjustedBy, &a.AdjustedAt,
			&a.PreviousDoseMg, &a.NewDoseMg, &a.Reason,
			&a.ClinicalNotes, &a.WitnessedBy, &a.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		adjustments = append(adjustments, a)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, adjustments)
}
