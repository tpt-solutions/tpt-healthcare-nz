package api

import (
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

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
		"patient_nhi":        nhiEnc,
		"proxy_guardian_nhi": req.ProxyGuardianNHI,
		"clinician_hpi":      req.ClinicianHpi,
		"admission_type":     req.AdmissionType,
		"admission_reason":   req.AdmissionReason,
		"ward":               req.Ward,
		"bed_label":          req.BedLabel,
		"age_years":          req.AgeYears,
		"age_months":         req.AgeMonths,
		"weight_kg":          req.WeightKg,
		"height_cm":          req.HeightCm,
		"tenant_id":          tenantID,
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
