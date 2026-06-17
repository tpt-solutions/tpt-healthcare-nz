package api

import (
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

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
