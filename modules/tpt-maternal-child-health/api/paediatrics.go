package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// PaediatricAdmissionStatus tracks the inpatient lifecycle.
type PaediatricAdmissionStatus string

const (
	PaedAdmissionStatusAdmitted          PaediatricAdmissionStatus = "admitted"
	PaedAdmissionStatusStable            PaediatricAdmissionStatus = "stable"
	PaedAdmissionStatusDischargePlanning PaediatricAdmissionStatus = "discharge-planning"
	PaedAdmissionStatusDischarged        PaediatricAdmissionStatus = "discharged"
	PaedAdmissionStatusTransferred       PaediatricAdmissionStatus = "transferred"
)

// PaediatricAdmissionType classifies how the child came to be admitted.
type PaediatricAdmissionType string

const (
	PaedAdmissionElective PaediatricAdmissionType = "elective"
	PaedAdmissionAcute    PaediatricAdmissionType = "acute"
	PaedAdmissionTransfer PaediatricAdmissionType = "transfer"
)

// PICUStatus tracks the clinical status of a PICU admission.
// PICU covers children >28 days requiring intensive care; neonates are managed
// in the NICU under /api/v1/maternity/nicu.
type PICUStatus string

const (
	PICUStatusAdmitted   PICUStatus = "admitted"
	PICUStatusStable     PICUStatus = "stable"
	PICUStatusCritical   PICUStatus = "critical"
	PICUStatusDischarged PICUStatus = "discharged"
)

// DevelopmentalDomain classifies the developmental area being assessed.
type DevelopmentalDomain string

const (
	DevDomainGrossMotor      DevelopmentalDomain = "gross-motor"
	DevDomainFineMotor       DevelopmentalDomain = "fine-motor"
	DevDomainSpeechLanguage  DevelopmentalDomain = "speech-language"
	DevDomainSocialEmotional DevelopmentalDomain = "social-emotional"
	DevDomainCognitive       DevelopmentalDomain = "cognitive"
)

// ChildProtectionStatus tracks the child protection concern lifecycle.
// Flagging and reporting must comply with the Children's Act 2014 (NZ).
type ChildProtectionStatus string

const (
	ChildProtectionNone               ChildProtectionStatus = "none"
	ChildProtectionConcernRaised      ChildProtectionStatus = "concern-raised"
	ChildProtectionNotified           ChildProtectionStatus = "notified"
	ChildProtectionUnderInvestigation ChildProtectionStatus = "under-investigation"
)

type PaediatricAdmission struct {
	ID               string     `json:"id"`
	PatientNHI       string     `json:"patientNhi"`
	ProxyGuardianNHI *string    `json:"proxyGuardianNhi"`
	ClinicianHpi     string     `json:"clinicianHpi"`
	Status           string     `json:"status"`
	AdmissionType    string     `json:"admissionType"`
	AdmissionReason  string     `json:"admissionReason"`
	Ward             string     `json:"ward"`
	BedLabel         string     `json:"bedLabel"`
	AgeYears         *int16     `json:"ageYears"`
	AgeMonths        *int16     `json:"ageMonths"`
	WeightKg         *float64   `json:"weightKg"`
	HeightCm         *float64   `json:"heightCm"`
	TenantID         string     `json:"tenantId"`
	AdmittedAt       time.Time  `json:"admittedAt"`
	DischargedAt     *time.Time `json:"dischargedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type PICUAdmission struct {
	ID                   string     `json:"id"`
	PaediatricAdmissionID string    `json:"paediatricAdmissionId"`
	PatientNHI           string     `json:"patientNhi"`
	ClinicianHpi         string     `json:"clinicianHpi"`
	Status               string     `json:"status"`
	AdmissionReason      string     `json:"admissionReason"`
	AdmissionType        string     `json:"admissionType"`
	RespiratorySupport   string     `json:"respiratorySupport"`
	TpnActive            bool       `json:"tpnActive"`
	InotropesActive      bool       `json:"inotropesActive"`
	BedLabel             string     `json:"bedLabel"`
	TenantID             string     `json:"tenantId"`
	AdmittedAt           time.Time  `json:"admittedAt"`
	DischargedAt         *time.Time `json:"dischargedAt"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type PaediatricGrowthRecord struct {
	ID                    string    `json:"id"`
	PaediatricAdmissionID string    `json:"paediatricAdmissionId"`
	PatientNHI            string    `json:"patientNhi"`
	ClinicianHpi          string    `json:"clinicianHpi"`
	WeightKg              *float64  `json:"weightKg"`
	HeightCm              *float64  `json:"heightCm"`
	HeadCircumferenceCm   *float64  `json:"headCircumferenceCm"`
	Bmi                   *float64  `json:"bmi"`
	CentileBand           *string   `json:"centileBand"`
	RecordedAt            time.Time `json:"recordedAt"`
	TenantID              string    `json:"tenantId"`
}

type DevelopmentalMilestone struct {
	ID                    string    `json:"id"`
	PaediatricAdmissionID string    `json:"paediatricAdmissionId"`
	PatientNHI            string    `json:"patientNhi"`
	ClinicianHpi          string    `json:"clinicianHpi"`
	Domain                string    `json:"domain"`
	MilestoneDescription  string    `json:"milestoneDescription"`
	ExpectedAgeMonths     *int16    `json:"expectedAgeMonths"`
	Achieved              bool      `json:"achieved"`
	AchievedAt            *string   `json:"achievedAt"`
	ConcernNoted          bool      `json:"concernNoted"`
	Notes                 *string   `json:"notes"`
	AssessedAt            time.Time `json:"assessedAt"`
	TenantID              string    `json:"tenantId"`
}

type ChildProtectionFlag struct {
	ID                    string     `json:"id"`
	PaediatricAdmissionID string     `json:"paediatricAdmissionId"`
	PatientNHI            string     `json:"patientNhi"`
	RaisedByHpi           string     `json:"raisedByHpi"`
	Status                string     `json:"status"`
	ConcernDescription    string     `json:"concernDescription"`
	NotifiedAt            *time.Time `json:"notifiedAt"`
	NotifiedBody          *string    `json:"notifiedBody"`
	CaseReference         *string    `json:"caseReference"`
	ResolvedAt            *time.Time `json:"resolvedAt"`
	Notes                 *string    `json:"notes"`
	TenantID              string     `json:"tenantId"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

// paediatricHandler manages paediatric inpatient admissions, PICU,
// growth and developmental milestone tracking, and child protection flagging.
type paediatricHandler struct {
	handlerDeps
}

func (h *paediatricHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, patient_nhi, proxy_guardian_nhi, clinician_hpi, status, admission_type,
		       admission_reason, ward, bed_label, age_years, age_months, weight_kg, height_cm,
		       tenant_id, admitted_at, discharged_at, created_at, updated_at
		FROM paediatric_admissions
		WHERE tenant_id = @tenant_id
		ORDER BY admitted_at DESC
		LIMIT 200
	`, pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	admissions := make([]PaediatricAdmission, 0)
	for rows.Next() {
		var a PaediatricAdmission
		if err := rows.Scan(
			&a.ID, &a.PatientNHI, &a.ProxyGuardianNHI, &a.ClinicianHpi, &a.Status, &a.AdmissionType,
			&a.AdmissionReason, &a.Ward, &a.BedLabel, &a.AgeYears, &a.AgeMonths, &a.WeightKg, &a.HeightCm,
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

func (h *paediatricHandler) Admit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req PaediatricAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" || req.ClinicianHpi == "" || req.AdmissionReason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELDS", Message: "patientNhi, clinicianHpi, and admissionReason are required"})
		return
	}
	if req.AdmissionType == "" {
		req.AdmissionType = "acute"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var a PaediatricAdmission
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO paediatric_admissions
		    (patient_nhi, proxy_guardian_nhi, clinician_hpi, status, admission_type,
		     admission_reason, ward, bed_label, age_years, age_months, weight_kg, height_cm, tenant_id)
		VALUES
		    (@patient_nhi, @proxy_guardian_nhi, @clinician_hpi, 'admitted', @admission_type,
		     @admission_reason, @ward, @bed_label, @age_years, @age_months, @weight_kg, @height_cm, @tenant_id)
		RETURNING id, patient_nhi, proxy_guardian_nhi, clinician_hpi, status, admission_type,
		          admission_reason, ward, bed_label, age_years, age_months, weight_kg, height_cm,
		          tenant_id, admitted_at, discharged_at, created_at, updated_at
	`, pgx.NamedArgs{
		"patient_nhi":       nhiEnc,
		"proxy_guardian_nhi": req.ProxyGuardianNHI,
		"clinician_hpi":     req.ClinicianHpi,
		"admission_type":    req.AdmissionType,
		"admission_reason":  req.AdmissionReason,
		"ward":              req.Ward,
		"bed_label":         req.BedLabel,
		"age_years":         req.AgeYears,
		"age_months":        req.AgeMonths,
		"weight_kg":         req.WeightKg,
		"height_cm":         req.HeightCm,
		"tenant_id":         tenantID,
	}).Scan(
		&a.ID, &a.PatientNHI, &a.ProxyGuardianNHI, &a.ClinicianHpi, &a.Status, &a.AdmissionType,
		&a.AdmissionReason, &a.Ward, &a.BedLabel, &a.AgeYears, &a.AgeMonths, &a.WeightKg, &a.HeightCm,
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
	h.recordAudit(r, "create", "PaediatricAdmission", a.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, a)
}

func (h *paediatricHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var a PaediatricAdmission
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, patient_nhi, proxy_guardian_nhi, clinician_hpi, status, admission_type,
		       admission_reason, ward, bed_label, age_years, age_months, weight_kg, height_cm,
		       tenant_id, admitted_at, discharged_at, created_at, updated_at
		FROM paediatric_admissions
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&a.ID, &a.PatientNHI, &a.ProxyGuardianNHI, &a.ClinicianHpi, &a.Status, &a.AdmissionType,
		&a.AdmissionReason, &a.Ward, &a.BedLabel, &a.AgeYears, &a.AgeMonths, &a.WeightKg, &a.HeightCm,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "paediatric admission not found"})
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

func (h *paediatricHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req PaediatricAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var a PaediatricAdmission
	err := h.pool.QueryRow(r.Context(), `
		UPDATE paediatric_admissions
		SET status = @status,
		    ward = @ward,
		    bed_label = @bed_label,
		    weight_kg = @weight_kg,
		    height_cm = @height_cm,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, patient_nhi, proxy_guardian_nhi, clinician_hpi, status, admission_type,
		          admission_reason, ward, bed_label, age_years, age_months, weight_kg, height_cm,
		          tenant_id, admitted_at, discharged_at, created_at, updated_at
	`, pgx.NamedArgs{
		"status":    req.Status,
		"ward":      req.Ward,
		"bed_label": req.BedLabel,
		"weight_kg": req.WeightKg,
		"height_cm": req.HeightCm,
		"id":        id,
		"tenant_id": tenantID,
	}).Scan(
		&a.ID, &a.PatientNHI, &a.ProxyGuardianNHI, &a.ClinicianHpi, &a.Status, &a.AdmissionType,
		&a.AdmissionReason, &a.Ward, &a.BedLabel, &a.AgeYears, &a.AgeMonths, &a.WeightKg, &a.HeightCm,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "paediatric admission not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "PaediatricAdmission", a.ID, a.PatientNHI)
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	writeJSON(w, http.StatusOK, a)
}

func (h *paediatricHandler) Discharge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE paediatric_admissions
		SET status = 'discharged', discharged_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status != 'discharged'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found or already discharged"})
		return
	}
	h.recordAudit(r, "delete", "PaediatricAdmission", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "discharged"})
}

func (h *paediatricHandler) ListGrowth(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, paediatric_admission_id, patient_nhi, clinician_hpi,
		       weight_kg, height_cm, head_circumference_cm, bmi, centile_band, recorded_at, tenant_id
		FROM paediatric_growth_records
		WHERE paediatric_admission_id = @id AND tenant_id = @tenant_id
		ORDER BY recorded_at DESC
		LIMIT 500
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	records := make([]PaediatricGrowthRecord, 0)
	for rows.Next() {
		var g PaediatricGrowthRecord
		if err := rows.Scan(
			&g.ID, &g.PaediatricAdmissionID, &g.PatientNHI, &g.ClinicianHpi,
			&g.WeightKg, &g.HeightCm, &g.HeadCircumferenceCm, &g.Bmi, &g.CentileBand, &g.RecordedAt, &g.TenantID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(g.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		g.PatientNHI = nhi
		records = append(records, g)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *paediatricHandler) RecordGrowth(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req PaediatricGrowthRecord
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var g PaediatricGrowthRecord
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO paediatric_growth_records
		    (paediatric_admission_id, patient_nhi, clinician_hpi,
		     weight_kg, height_cm, head_circumference_cm, bmi, centile_band, tenant_id)
		VALUES
		    (@admission_id, @patient_nhi, @clinician_hpi,
		     @weight_kg, @height_cm, @head_circumference_cm, @bmi, @centile_band, @tenant_id)
		RETURNING id, paediatric_admission_id, patient_nhi, clinician_hpi,
		          weight_kg, height_cm, head_circumference_cm, bmi, centile_band, recorded_at, tenant_id
	`, pgx.NamedArgs{
		"admission_id":         id,
		"patient_nhi":          nhiEnc,
		"clinician_hpi":        req.ClinicianHpi,
		"weight_kg":            req.WeightKg,
		"height_cm":            req.HeightCm,
		"head_circumference_cm": req.HeadCircumferenceCm,
		"bmi":                  req.Bmi,
		"centile_band":         req.CentileBand,
		"tenant_id":            tenantID,
	}).Scan(
		&g.ID, &g.PaediatricAdmissionID, &g.PatientNHI, &g.ClinicianHpi,
		&g.WeightKg, &g.HeightCm, &g.HeadCircumferenceCm, &g.Bmi, &g.CentileBand, &g.RecordedAt, &g.TenantID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(g.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	g.PatientNHI = nhi
	h.recordAudit(r, "create", "PaediatricGrowthRecord", g.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, g)
}

func (h *paediatricHandler) ListMilestones(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, paediatric_admission_id, patient_nhi, clinician_hpi,
		       domain, milestone_description, expected_age_months, achieved, achieved_at::text,
		       concern_noted, notes, assessed_at, tenant_id
		FROM developmental_milestones
		WHERE paediatric_admission_id = @id AND tenant_id = @tenant_id
		ORDER BY assessed_at DESC
		LIMIT 200
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	milestones := make([]DevelopmentalMilestone, 0)
	for rows.Next() {
		var m DevelopmentalMilestone
		if err := rows.Scan(
			&m.ID, &m.PaediatricAdmissionID, &m.PatientNHI, &m.ClinicianHpi,
			&m.Domain, &m.MilestoneDescription, &m.ExpectedAgeMonths, &m.Achieved, &m.AchievedAt,
			&m.ConcernNoted, &m.Notes, &m.AssessedAt, &m.TenantID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(m.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		m.PatientNHI = nhi
		milestones = append(milestones, m)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, milestones)
}

func (h *paediatricHandler) RecordMilestone(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req DevelopmentalMilestone
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var m DevelopmentalMilestone
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO developmental_milestones
		    (paediatric_admission_id, patient_nhi, clinician_hpi,
		     domain, milestone_description, expected_age_months, achieved, achieved_at,
		     concern_noted, notes, tenant_id)
		VALUES
		    (@admission_id, @patient_nhi, @clinician_hpi,
		     @domain, @milestone_description, @expected_age_months, @achieved, @achieved_at,
		     @concern_noted, @notes, @tenant_id)
		RETURNING id, paediatric_admission_id, patient_nhi, clinician_hpi,
		          domain, milestone_description, expected_age_months, achieved, achieved_at::text,
		          concern_noted, notes, assessed_at, tenant_id
	`, pgx.NamedArgs{
		"admission_id":          id,
		"patient_nhi":           nhiEnc,
		"clinician_hpi":         req.ClinicianHpi,
		"domain":                req.Domain,
		"milestone_description": req.MilestoneDescription,
		"expected_age_months":   req.ExpectedAgeMonths,
		"achieved":              req.Achieved,
		"achieved_at":           req.AchievedAt,
		"concern_noted":         req.ConcernNoted,
		"notes":                 req.Notes,
		"tenant_id":             tenantID,
	}).Scan(
		&m.ID, &m.PaediatricAdmissionID, &m.PatientNHI, &m.ClinicianHpi,
		&m.Domain, &m.MilestoneDescription, &m.ExpectedAgeMonths, &m.Achieved, &m.AchievedAt,
		&m.ConcernNoted, &m.Notes, &m.AssessedAt, &m.TenantID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(m.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	m.PatientNHI = nhi
	h.recordAudit(r, "create", "DevelopmentalMilestone", m.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, m)
}

func (h *paediatricHandler) GetChildProtection(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, paediatric_admission_id, patient_nhi, raised_by_hpi, status,
		       concern_description, notified_at, notified_body, case_reference,
		       resolved_at, notes, tenant_id, created_at, updated_at
		FROM child_protection_flags
		WHERE paediatric_admission_id = @id AND tenant_id = @tenant_id
		ORDER BY created_at DESC
		LIMIT 100
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	flags := make([]ChildProtectionFlag, 0)
	for rows.Next() {
		var f ChildProtectionFlag
		if err := rows.Scan(
			&f.ID, &f.PaediatricAdmissionID, &f.PatientNHI, &f.RaisedByHpi, &f.Status,
			&f.ConcernDescription, &f.NotifiedAt, &f.NotifiedBody, &f.CaseReference,
			&f.ResolvedAt, &f.Notes, &f.TenantID, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(f.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		f.PatientNHI = nhi
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, flags)
}

func (h *paediatricHandler) UpdateChildProtection(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req ChildProtectionFlag
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ID == "" {
		// Create new flag — validate required fields first.
		if req.PatientNHI == "" || req.RaisedByHpi == "" || req.ConcernDescription == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELDS", Message: "patientNhi, raisedByHpi, and concernDescription are required"})
			return
		}
		if !h.validateHPI(w, r, req.RaisedByHpi) {
			return
		}
		nhiEnc, err := h.encryptNHI(req.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
			return
		}
		var f ChildProtectionFlag
		err = h.pool.QueryRow(r.Context(), `
			INSERT INTO child_protection_flags
			    (paediatric_admission_id, patient_nhi, raised_by_hpi, status,
			     concern_description, notified_at, notified_body, case_reference,
			     resolved_at, notes, tenant_id)
			VALUES
			    (@admission_id, @patient_nhi, @raised_by_hpi, COALESCE(@status, 'concern-raised'),
			     @concern_description, @notified_at, @notified_body, @case_reference,
			     @resolved_at, @notes, @tenant_id)
			RETURNING id, paediatric_admission_id, patient_nhi, raised_by_hpi, status,
			          concern_description, notified_at, notified_body, case_reference,
			          resolved_at, notes, tenant_id, created_at, updated_at
		`, pgx.NamedArgs{
			"admission_id":        id,
			"patient_nhi":         nhiEnc,
			"raised_by_hpi":       req.RaisedByHpi,
			"status":              req.Status,
			"concern_description": req.ConcernDescription,
			"notified_at":         req.NotifiedAt,
			"notified_body":       req.NotifiedBody,
			"case_reference":      req.CaseReference,
			"resolved_at":         req.ResolvedAt,
			"notes":               req.Notes,
			"tenant_id":           tenantID,
		}).Scan(
			&f.ID, &f.PaediatricAdmissionID, &f.PatientNHI, &f.RaisedByHpi, &f.Status,
			&f.ConcernDescription, &f.NotifiedAt, &f.NotifiedBody, &f.CaseReference,
			&f.ResolvedAt, &f.Notes, &f.TenantID, &f.CreatedAt, &f.UpdatedAt,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(f.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		f.PatientNHI = nhi
		h.recordAudit(r, "create", "ChildProtectionFlag", f.ID, nhiEnc)
		writeJSON(w, http.StatusCreated, f)
		return
	}
	// Update existing flag by its ID
	var f ChildProtectionFlag
	err := h.pool.QueryRow(r.Context(), `
		UPDATE child_protection_flags
		SET status = @status,
		    notified_at = @notified_at,
		    notified_body = @notified_body,
		    case_reference = @case_reference,
		    resolved_at = @resolved_at,
		    notes = @notes,
		    updated_at = now()
		WHERE id = @flag_id AND paediatric_admission_id = @admission_id AND tenant_id = @tenant_id
		RETURNING id, paediatric_admission_id, patient_nhi, raised_by_hpi, status,
		          concern_description, notified_at, notified_body, case_reference,
		          resolved_at, notes, tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"flag_id":        req.ID,
		"admission_id":   id,
		"status":         req.Status,
		"notified_at":    req.NotifiedAt,
		"notified_body":  req.NotifiedBody,
		"case_reference": req.CaseReference,
		"resolved_at":    req.ResolvedAt,
		"notes":          req.Notes,
		"tenant_id":      tenantID,
	}).Scan(
		&f.ID, &f.PaediatricAdmissionID, &f.PatientNHI, &f.RaisedByHpi, &f.Status,
		&f.ConcernDescription, &f.NotifiedAt, &f.NotifiedBody, &f.CaseReference,
		&f.ResolvedAt, &f.Notes, &f.TenantID, &f.CreatedAt, &f.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "child protection flag not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "ChildProtectionFlag", f.ID, f.PatientNHI)
	nhi, err := h.decryptNHI(f.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	f.PatientNHI = nhi
	writeJSON(w, http.StatusOK, f)
}

func (h *paediatricHandler) ListPICU(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, paediatric_admission_id, patient_nhi, clinician_hpi, status,
		       admission_reason, admission_type, respiratory_support, tpn_active, inotropes_active,
		       bed_label, tenant_id, admitted_at, discharged_at, created_at, updated_at
		FROM picu_admissions
		WHERE tenant_id = @tenant_id
		ORDER BY admitted_at DESC
		LIMIT 200
	`, pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	admissions := make([]PICUAdmission, 0)
	for rows.Next() {
		var p PICUAdmission
		if err := rows.Scan(
			&p.ID, &p.PaediatricAdmissionID, &p.PatientNHI, &p.ClinicianHpi, &p.Status,
			&p.AdmissionReason, &p.AdmissionType, &p.RespiratorySupport, &p.TpnActive, &p.InotropesActive,
			&p.BedLabel, &p.TenantID, &p.AdmittedAt, &p.DischargedAt, &p.CreatedAt, &p.UpdatedAt,
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
		admissions = append(admissions, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, admissions)
}

func (h *paediatricHandler) AdmitPICU(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req PICUAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" || req.ClinicianHpi == "" || req.AdmissionReason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELDS", Message: "patientNhi, clinicianHpi, and admissionReason are required"})
		return
	}
	if req.RespiratorySupport == "" {
		req.RespiratorySupport = "none"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var p PICUAdmission
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO picu_admissions
		    (paediatric_admission_id, patient_nhi, clinician_hpi, status,
		     admission_reason, admission_type, respiratory_support, tpn_active, inotropes_active,
		     bed_label, tenant_id)
		VALUES
		    (@paediatric_admission_id, @patient_nhi, @clinician_hpi, 'admitted',
		     @admission_reason, @admission_type, @respiratory_support, @tpn_active, @inotropes_active,
		     @bed_label, @tenant_id)
		RETURNING id, paediatric_admission_id, patient_nhi, clinician_hpi, status,
		          admission_reason, admission_type, respiratory_support, tpn_active, inotropes_active,
		          bed_label, tenant_id, admitted_at, discharged_at, created_at, updated_at
	`, pgx.NamedArgs{
		"paediatric_admission_id": req.PaediatricAdmissionID,
		"patient_nhi":             nhiEnc,
		"clinician_hpi":           req.ClinicianHpi,
		"admission_reason":        req.AdmissionReason,
		"admission_type":          req.AdmissionType,
		"respiratory_support":     req.RespiratorySupport,
		"tpn_active":              req.TpnActive,
		"inotropes_active":        req.InotropesActive,
		"bed_label":               req.BedLabel,
		"tenant_id":               tenantID,
	}).Scan(
		&p.ID, &p.PaediatricAdmissionID, &p.PatientNHI, &p.ClinicianHpi, &p.Status,
		&p.AdmissionReason, &p.AdmissionType, &p.RespiratorySupport, &p.TpnActive, &p.InotropesActive,
		&p.BedLabel, &p.TenantID, &p.AdmittedAt, &p.DischargedAt, &p.CreatedAt, &p.UpdatedAt,
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
	h.recordAudit(r, "create", "PICUAdmission", p.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, p)
}

func (h *paediatricHandler) GetPICU(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var p PICUAdmission
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, paediatric_admission_id, patient_nhi, clinician_hpi, status,
		       admission_reason, admission_type, respiratory_support, tpn_active, inotropes_active,
		       bed_label, tenant_id, admitted_at, discharged_at, created_at, updated_at
		FROM picu_admissions
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.PaediatricAdmissionID, &p.PatientNHI, &p.ClinicianHpi, &p.Status,
		&p.AdmissionReason, &p.AdmissionType, &p.RespiratorySupport, &p.TpnActive, &p.InotropesActive,
		&p.BedLabel, &p.TenantID, &p.AdmittedAt, &p.DischargedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "PICU admission not found"})
		return
	}
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
	writeJSON(w, http.StatusOK, p)
}

func (h *paediatricHandler) UpdatePICU(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req PICUAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var p PICUAdmission
	err := h.pool.QueryRow(r.Context(), `
		UPDATE picu_admissions
		SET status = @status,
		    respiratory_support = @respiratory_support,
		    tpn_active = @tpn_active,
		    inotropes_active = @inotropes_active,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, paediatric_admission_id, patient_nhi, clinician_hpi, status,
		          admission_reason, admission_type, respiratory_support, tpn_active, inotropes_active,
		          bed_label, tenant_id, admitted_at, discharged_at, created_at, updated_at
	`, pgx.NamedArgs{
		"status":              req.Status,
		"respiratory_support": req.RespiratorySupport,
		"tpn_active":          req.TpnActive,
		"inotropes_active":    req.InotropesActive,
		"id":                  id,
		"tenant_id":           tenantID,
	}).Scan(
		&p.ID, &p.PaediatricAdmissionID, &p.PatientNHI, &p.ClinicianHpi, &p.Status,
		&p.AdmissionReason, &p.AdmissionType, &p.RespiratorySupport, &p.TpnActive, &p.InotropesActive,
		&p.BedLabel, &p.TenantID, &p.AdmittedAt, &p.DischargedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "PICU admission not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "PICUAdmission", p.ID, p.PatientNHI)
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

func (h *paediatricHandler) DischargePICU(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE picu_admissions
		SET status = 'discharged', discharged_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status != 'discharged'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "PICU admission not found or already discharged"})
		return
	}
	h.recordAudit(r, "delete", "PICUAdmission", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "discharged"})
}
