package api

import (
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

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
