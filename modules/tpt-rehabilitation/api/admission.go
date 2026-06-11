package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// RehabAdmission represents an inpatient rehabilitation admission episode.
type RehabAdmission struct {
	ID                  string     `json:"id"`
	PatientNHI          string     `json:"patientNhi"`
	ClinicianHpi        string     `json:"clinicianHpi"`
	Ward                string     `json:"ward"`
	AdmissionType       string     `json:"admissionType"`
	AdmissionSource     string     `json:"admissionSource"`
	PrimaryDiagnosis    string     `json:"primaryDiagnosis"`
	SecondaryDiagnoses  string     `json:"secondaryDiagnoses"`
	Status              string     `json:"status"`
	MobilityOnAdmission string     `json:"mobilityOnAdmission"`
	CognitiveStatus     string     `json:"cognitiveStatus"`
	GoalsSetAt          *time.Time `json:"goalsSetAt"`
	Notes               *string    `json:"notes"`
	TenantID            string     `json:"tenantId"`
	AdmittedAt          time.Time  `json:"admittedAt"`
	DischargedAt        *time.Time `json:"dischargedAt"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

const admissionSelectCols = `id, patient_nhi, clinician_hpi, ward, admission_type, admission_source,
       primary_diagnosis, secondary_diagnoses, status,
       mobility_on_admission, cognitive_status, goals_set_at, notes,
       tenant_id, admitted_at, discharged_at, created_at, updated_at`

func scanAdmission(row interface{ Scan(...any) error }, a *RehabAdmission) error {
	return row.Scan(
		&a.ID, &a.PatientNHI, &a.ClinicianHpi, &a.Ward, &a.AdmissionType, &a.AdmissionSource,
		&a.PrimaryDiagnosis, &a.SecondaryDiagnoses, &a.Status,
		&a.MobilityOnAdmission, &a.CognitiveStatus, &a.GoalsSetAt, &a.Notes,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
}

type admissionHandler struct{ handlerDeps }

func (h *admissionHandler) List(w http.ResponseWriter, r *http.Request) {
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
			`SELECT `+admissionSelectCols+` FROM rehab_admissions WHERE tenant_id = @tenant_id AND status = @status ORDER BY admitted_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+admissionSelectCols+` FROM rehab_admissions WHERE tenant_id = @tenant_id ORDER BY admitted_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	admissions := make([]RehabAdmission, 0)
	for rows.Next() {
		var a RehabAdmission
		if err := scanAdmission(rows, &a); err != nil {
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

func (h *admissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req RehabAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AdmissionType == "" {
		req.AdmissionType = "inpatient"
	}
	if req.AdmissionSource == "" {
		req.AdmissionSource = "hospital"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var a RehabAdmission
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO rehab_admissions
		    (patient_nhi, clinician_hpi, ward, admission_type, admission_source,
		     primary_diagnosis, secondary_diagnoses, status,
		     mobility_on_admission, cognitive_status, notes, tenant_id, admitted_at)
		VALUES
		    (@patient_nhi, @clinician_hpi, @ward, @admission_type, @admission_source,
		     @primary_diagnosis, @secondary_diagnoses, 'admitted',
		     @mobility_on_admission, @cognitive_status, @notes, @tenant_id,
		     COALESCE(@admitted_at, now()))
		RETURNING `+admissionSelectCols,
		pgx.NamedArgs{
			"patient_nhi":           nhiEnc,
			"clinician_hpi":         req.ClinicianHpi,
			"ward":                  req.Ward,
			"admission_type":        req.AdmissionType,
			"admission_source":      req.AdmissionSource,
			"primary_diagnosis":     req.PrimaryDiagnosis,
			"secondary_diagnoses":   req.SecondaryDiagnoses,
			"mobility_on_admission": req.MobilityOnAdmission,
			"cognitive_status":      req.CognitiveStatus,
			"notes":                 req.Notes,
			"tenant_id":             tenantID,
			"admitted_at":           req.AdmittedAt,
		}).Scan(
		&a.ID, &a.PatientNHI, &a.ClinicianHpi, &a.Ward, &a.AdmissionType, &a.AdmissionSource,
		&a.PrimaryDiagnosis, &a.SecondaryDiagnoses, &a.Status,
		&a.MobilityOnAdmission, &a.CognitiveStatus, &a.GoalsSetAt, &a.Notes,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "RehabAdmission", a.ID, a.PatientNHI)
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, a)
}

func (h *admissionHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var a RehabAdmission
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+admissionSelectCols+` FROM rehab_admissions WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&a.ID, &a.PatientNHI, &a.ClinicianHpi, &a.Ward, &a.AdmissionType, &a.AdmissionSource,
		&a.PrimaryDiagnosis, &a.SecondaryDiagnoses, &a.Status,
		&a.MobilityOnAdmission, &a.CognitiveStatus, &a.GoalsSetAt, &a.Notes,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
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

func (h *admissionHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req RehabAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var a RehabAdmission
	err := h.pool.QueryRow(r.Context(), `
		UPDATE rehab_admissions
		SET clinician_hpi          = @clinician_hpi,
		    ward                   = @ward,
		    status                 = @status,
		    primary_diagnosis      = @primary_diagnosis,
		    secondary_diagnoses    = @secondary_diagnoses,
		    mobility_on_admission  = @mobility_on_admission,
		    cognitive_status       = @cognitive_status,
		    goals_set_at           = COALESCE(goals_set_at, @goals_set_at),
		    notes                  = @notes,
		    updated_at             = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+admissionSelectCols,
		pgx.NamedArgs{
			"clinician_hpi":         req.ClinicianHpi,
			"ward":                  req.Ward,
			"status":                req.Status,
			"primary_diagnosis":     req.PrimaryDiagnosis,
			"secondary_diagnoses":   req.SecondaryDiagnoses,
			"mobility_on_admission": req.MobilityOnAdmission,
			"cognitive_status":      req.CognitiveStatus,
			"goals_set_at":          req.GoalsSetAt,
			"notes":                 req.Notes,
			"id":                    id,
			"tenant_id":             tenantID,
		}).Scan(
		&a.ID, &a.PatientNHI, &a.ClinicianHpi, &a.Ward, &a.AdmissionType, &a.AdmissionSource,
		&a.PrimaryDiagnosis, &a.SecondaryDiagnoses, &a.Status,
		&a.MobilityOnAdmission, &a.CognitiveStatus, &a.GoalsSetAt, &a.Notes,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "RehabAdmission", a.ID, a.PatientNHI)
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	writeJSON(w, http.StatusOK, a)
}

func (h *admissionHandler) Discharge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM rehab_admissions WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found or already discharged"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE rehab_admissions
		SET status = 'discharged', discharged_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('discharged', 'transferred')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found or already discharged"})
		return
	}
	h.recordAudit(r, "discharge", "RehabAdmission", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "discharged"})
}
