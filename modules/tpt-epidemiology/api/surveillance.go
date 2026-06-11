package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

// SurveillanceCase is a notifiable disease case record for EpiSurv/ESR reporting.
// disease_code is a SNOMED CT concept ID or NZ notifiable disease code per the
// Health (Infectious and Notifiable Diseases) Regulations 2016.
// notification_status values: draft | submitted | acknowledged | closed
type SurveillanceCase struct {
	ID                 string     `json:"id"`
	PatientNHI         string     `json:"patientNhi"`
	DiseaseCode        string     `json:"diseaseCode"`
	DiseaseName        string     `json:"diseaseName"`
	DiagnosisDate      string     `json:"diagnosisDate"` // DATE as YYYY-MM-DD
	ReportingHPI       string     `json:"reportingHpi"`
	NotificationStatus string     `json:"notificationStatus"`
	EpisurvReference   *string    `json:"episurvReference"`
	ClinicalNotes      *string    `json:"clinicalNotes"`
	ExposureDetails    *string    `json:"exposureDetails"`
	OutbreakID         *string    `json:"outbreakId"`
	TenantID           string     `json:"tenantId"`
	SubmittedAt        *time.Time `json:"submittedAt"`
	AcknowledgedAt     *time.Time `json:"acknowledgedAt"`
	ClosedAt           *time.Time `json:"closedAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

const caseSelectCols = `id, patient_nhi, disease_code, disease_name,
       diagnosis_date::text, reporting_hpi, notification_status,
       episurv_reference, clinical_notes, exposure_details, outbreak_id,
       tenant_id, submitted_at, acknowledged_at, closed_at,
       created_at, updated_at`

func scanCase(row interface{ Scan(...any) error }, c *SurveillanceCase) error {
	return row.Scan(
		&c.ID, &c.PatientNHI, &c.DiseaseCode, &c.DiseaseName,
		&c.DiagnosisDate, &c.ReportingHPI, &c.NotificationStatus,
		&c.EpisurvReference, &c.ClinicalNotes, &c.ExposureDetails, &c.OutbreakID,
		&c.TenantID, &c.SubmittedAt, &c.AcknowledgedAt, &c.ClosedAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
}

type surveillanceHandler struct{ handlerDeps }

func (h *surveillanceHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	q := r.URL.Query()
	diseaseCode := q.Get("disease_code")
	status := q.Get("status")
	outbreakID := q.Get("outbreak_id")

	var rows pgx.Rows
	var err error
	switch {
	case outbreakID != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+caseSelectCols+` FROM surveillance_cases
			 WHERE tenant_id = @tenant_id AND outbreak_id = @outbreak_id
			 ORDER BY diagnosis_date DESC, created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "outbreak_id": outbreakID})
	case diseaseCode != "" && status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+caseSelectCols+` FROM surveillance_cases
			 WHERE tenant_id = @tenant_id AND disease_code = @disease_code AND notification_status = @status
			 ORDER BY diagnosis_date DESC, created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "disease_code": diseaseCode, "status": status})
	case diseaseCode != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+caseSelectCols+` FROM surveillance_cases
			 WHERE tenant_id = @tenant_id AND disease_code = @disease_code
			 ORDER BY diagnosis_date DESC, created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "disease_code": diseaseCode})
	case status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+caseSelectCols+` FROM surveillance_cases
			 WHERE tenant_id = @tenant_id AND notification_status = @status
			 ORDER BY diagnosis_date DESC, created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": status})
	default:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+caseSelectCols+` FROM surveillance_cases
			 WHERE tenant_id = @tenant_id ORDER BY diagnosis_date DESC, created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	cases := make([]SurveillanceCase, 0)
	for rows.Next() {
		var c SurveillanceCase
		if err := scanCase(rows, &c); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(c.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		c.PatientNHI = nhi
		cases = append(cases, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cases)
}

func (h *surveillanceHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req SurveillanceCase
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "patientNhi is required"})
		return
	}
	if req.DiseaseCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "diseaseCode is required"})
		return
	}
	if req.DiagnosisDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "diagnosisDate is required"})
		return
	}
	if !h.validateHPI(w, r, req.ReportingHPI) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var c SurveillanceCase
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO surveillance_cases
		    (patient_nhi, disease_code, disease_name, diagnosis_date,
		     reporting_hpi, notification_status, clinical_notes,
		     exposure_details, outbreak_id, tenant_id)
		VALUES
		    (@patient_nhi, @disease_code, @disease_name, @diagnosis_date,
		     @reporting_hpi, 'draft', @clinical_notes,
		     @exposure_details, @outbreak_id, @tenant_id)
		RETURNING `+caseSelectCols,
		pgx.NamedArgs{
			"patient_nhi":      nhiEnc,
			"disease_code":     req.DiseaseCode,
			"disease_name":     req.DiseaseName,
			"diagnosis_date":   req.DiagnosisDate,
			"reporting_hpi":    req.ReportingHPI,
			"clinical_notes":   req.ClinicalNotes,
			"exposure_details": req.ExposureDetails,
			"outbreak_id":      req.OutbreakID,
			"tenant_id":        tenantID,
		}).Scan(
		&c.ID, &c.PatientNHI, &c.DiseaseCode, &c.DiseaseName,
		&c.DiagnosisDate, &c.ReportingHPI, &c.NotificationStatus,
		&c.EpisurvReference, &c.ClinicalNotes, &c.ExposureDetails, &c.OutbreakID,
		&c.TenantID, &c.SubmittedAt, &c.AcknowledgedAt, &c.ClosedAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "SurveillanceCase", c.ID, c.PatientNHI)
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, c)
}

func (h *surveillanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var c SurveillanceCase
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+caseSelectCols+` FROM surveillance_cases WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&c.ID, &c.PatientNHI, &c.DiseaseCode, &c.DiseaseName,
		&c.DiagnosisDate, &c.ReportingHPI, &c.NotificationStatus,
		&c.EpisurvReference, &c.ClinicalNotes, &c.ExposureDetails, &c.OutbreakID,
		&c.TenantID, &c.SubmittedAt, &c.AcknowledgedAt, &c.ClosedAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "surveillance case not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "SurveillanceCase", c.ID, c.PatientNHI)
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	writeJSON(w, http.StatusOK, c)
}

func (h *surveillanceHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req SurveillanceCase
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ReportingHPI) {
		return
	}
	var c SurveillanceCase
	err := h.pool.QueryRow(r.Context(), `
		UPDATE surveillance_cases
		SET disease_name      = @disease_name,
		    diagnosis_date    = @diagnosis_date,
		    reporting_hpi     = @reporting_hpi,
		    clinical_notes    = @clinical_notes,
		    exposure_details  = @exposure_details,
		    outbreak_id       = @outbreak_id,
		    updated_at        = now()
		WHERE id = @id AND tenant_id = @tenant_id AND notification_status = 'draft'
		RETURNING `+caseSelectCols,
		pgx.NamedArgs{
			"disease_name":     req.DiseaseName,
			"diagnosis_date":   req.DiagnosisDate,
			"reporting_hpi":    req.ReportingHPI,
			"clinical_notes":   req.ClinicalNotes,
			"exposure_details": req.ExposureDetails,
			"outbreak_id":      req.OutbreakID,
			"id":               id,
			"tenant_id":        tenantID,
		}).Scan(
		&c.ID, &c.PatientNHI, &c.DiseaseCode, &c.DiseaseName,
		&c.DiagnosisDate, &c.ReportingHPI, &c.NotificationStatus,
		&c.EpisurvReference, &c.ClinicalNotes, &c.ExposureDetails, &c.OutbreakID,
		&c.TenantID, &c.SubmittedAt, &c.AcknowledgedAt, &c.ClosedAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "surveillance case not found or not in draft status"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "SurveillanceCase", c.ID, c.PatientNHI)
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	writeJSON(w, http.StatusOK, c)
}

// Submit transitions the case to submitted and optionally records an EpiSurv reference.
// This represents electronic submission to ESR EpiSurv under the Health Act 1956.
func (h *surveillanceHandler) Submit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var body struct {
		EpisurvReference *string `json:"episurvReference"`
	}
	_ = decodeJSON(r, &body)

	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM surveillance_cases WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "surveillance case not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE surveillance_cases
		SET notification_status = 'submitted',
		    episurv_reference   = COALESCE(@episurv_reference, episurv_reference),
		    submitted_at        = now(),
		    updated_at          = now()
		WHERE id = @id AND tenant_id = @tenant_id AND notification_status = 'draft'
	`, pgx.NamedArgs{"episurv_reference": body.EpisurvReference, "id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "case must be in draft status to submit"})
		return
	}
	h.recordAudit(r, "update", "SurveillanceCase", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

// Acknowledge records ESR acknowledgement of the notification.
func (h *surveillanceHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	h.transitionCase(w, r, "acknowledged", "acknowledged_at = now(),", []string{"submitted"})
}

// Close marks the case as closed after investigation is complete.
func (h *surveillanceHandler) Close(w http.ResponseWriter, r *http.Request) {
	h.transitionCase(w, r, "closed", "closed_at = now(),", []string{"submitted", "acknowledged"})
}

// transitionCase handles the common status-change pattern for acknowledge/close.
func (h *surveillanceHandler) transitionCase(w http.ResponseWriter, r *http.Request, toStatus, extraSet string, fromStatuses []string) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")

	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM surveillance_cases WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "surveillance case not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}

	// Build IN clause — values are hardcoded by caller, not user input.
	inClause := "'"
	for i, s := range fromStatuses {
		if i > 0 {
			inClause += "', '"
		}
		inClause += s
	}
	inClause += "'"

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE surveillance_cases
		 SET notification_status = @status,
		     `+extraSet+`
		     updated_at          = now()
		 WHERE id = @id AND tenant_id = @tenant_id AND notification_status IN (`+inClause+`)`,
		pgx.NamedArgs{"status": toStatus, "id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "case is not in a valid state for this transition"})
		return
	}
	h.recordAudit(r, "update", "SurveillanceCase", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": toStatus})
}
