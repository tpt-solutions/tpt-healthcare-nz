package api

import (
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

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
		"admission_id":          id,
		"patient_nhi":           nhiEnc,
		"clinician_hpi":         req.ClinicianHpi,
		"weight_kg":             req.WeightKg,
		"height_cm":             req.HeightCm,
		"head_circumference_cm": req.HeadCircumferenceCm,
		"bmi":                   req.Bmi,
		"centile_band":          req.CentileBand,
		"tenant_id":             tenantID,
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
