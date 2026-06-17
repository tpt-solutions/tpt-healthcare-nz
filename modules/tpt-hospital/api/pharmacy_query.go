package api

import (
	"context"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

func (h *PharmacyHandler) listMedications(ctx context.Context, admissionID, tenantID, statusFilter string) ([]InpatientMedication, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		        nzmt_code, dose, route, frequency, max_daily_dose, indication,
		        start_date, end_date, status, is_iv, iv_rate, allergies_checked,
		        tenant_id, ceased_at, ceased_reason, created_at, updated_at
		 FROM inpatient_medications
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY created_at ASC`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query inpatient medications: %w", err)
	}
	defer rows.Close()

	var results []InpatientMedication
	for rows.Next() {
		m, err := scanMedRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

func (h *PharmacyHandler) getMedicationByID(ctx context.Context, id, admissionID, tenantID string) (InpatientMedication, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		        nzmt_code, dose, route, frequency, max_daily_dose, indication,
		        start_date, end_date, status, is_iv, iv_rate, allergies_checked,
		        tenant_id, ceased_at, ceased_reason, created_at, updated_at
		 FROM inpatient_medications
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "admission_id": admissionID, "tenant_id": tenantID},
	)
	m, err := scanMedRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return InpatientMedication{}, errNotFound
		}
		return InpatientMedication{}, fmt.Errorf("get inpatient medication: %w", err)
	}
	return m, nil
}

func (h *PharmacyHandler) insertMedication(ctx context.Context, admissionID string, req medPrescribeRequest, tenantID string) (InpatientMedication, error) {
	var patientID string
	if err := h.pool.QueryRow(ctx,
		`SELECT patient_id FROM hospital_admissions WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": admissionID, "tenant_id": tenantID},
	).Scan(&patientID); err != nil {
		if db.IsNoRows(err) {
			return InpatientMedication{}, errNotFound
		}
		return InpatientMedication{}, fmt.Errorf("get admission for medication: %w", err)
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO inpatient_medications
		   (admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		    nzmt_code, dose, route, frequency, max_daily_dose, indication,
		    start_date, end_date, status, is_iv, iv_rate, tenant_id)
		 VALUES
		   (@admission_id, @patient_id, @prescriber_hpi, @generic_name, @brand_name,
		    @nzmt_code, @dose, @route, @frequency, @max_daily_dose, @indication,
		    @start_date, @end_date, @status, @is_iv, @iv_rate, @tenant_id)
		 RETURNING id, admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		           nzmt_code, dose, route, frequency, max_daily_dose, indication,
		           start_date, end_date, status, is_iv, iv_rate, allergies_checked,
		           tenant_id, ceased_at, ceased_reason, created_at, updated_at`,
		db.NamedArgs{
			"admission_id":   admissionID,
			"patient_id":     patientID,
			"prescriber_hpi": req.PrescriberHPI,
			"generic_name":   req.GenericName,
			"brand_name":     req.BrandName,
			"nzmt_code":      req.NZMTCode,
			"dose":           req.Dose,
			"route":          req.Route,
			"frequency":      req.Frequency,
			"max_daily_dose": req.MaxDailyDose,
			"indication":     req.Indication,
			"start_date":     req.StartDate,
			"end_date":       req.EndDate,
			"status":         InpatientMedStatusActive,
			"is_iv":          req.IsIV,
			"iv_rate":        req.IVRate,
			"tenant_id":      tenantID,
		},
	)
	return scanMedRow(row)
}

func (h *PharmacyHandler) updateMedication(ctx context.Context, m InpatientMedication) (InpatientMedication, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE inpatient_medications
		 SET dose = @dose, frequency = @frequency, max_daily_dose = @max_daily_dose,
		     indication = @indication, end_date = @end_date, status = @status,
		     iv_rate = @iv_rate, ceased_at = @ceased_at, ceased_reason = @ceased_reason,
		     updated_at = now()
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id
		 RETURNING id, admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		           nzmt_code, dose, route, frequency, max_daily_dose, indication,
		           start_date, end_date, status, is_iv, iv_rate, allergies_checked,
		           tenant_id, ceased_at, ceased_reason, created_at, updated_at`,
		db.NamedArgs{
			"dose":          m.Dose,
			"frequency":     m.Frequency,
			"max_daily_dose": m.MaxDailyDose,
			"indication":    m.Indication,
			"end_date":      m.EndDate,
			"status":        m.Status,
			"iv_rate":       m.IVRate,
			"ceased_at":     m.CeasedAt,
			"ceased_reason": m.CeasedReason,
			"id":            m.ID,
			"admission_id":  m.AdmissionID,
			"tenant_id":     m.TenantID,
		},
	)
	updated, err := scanMedRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return InpatientMedication{}, errNotFound
		}
		return InpatientMedication{}, fmt.Errorf("update inpatient medication: %w", err)
	}
	return updated, nil
}

func (h *PharmacyHandler) insertAdminRecord(ctx context.Context, medID, admissionID string, med InpatientMedication, req medAdminRequest, tenantID string) (MedAdministrationRecord, error) {
	route := req.Route
	if route == "" {
		route = med.Route
	}
	actualDose := req.ActualDose
	if actualDose == "" {
		actualDose = med.Dose
	}
	row := h.pool.QueryRow(ctx,
		`INSERT INTO med_administration_records
		   (medication_id, admission_id, administered_by, actual_dose, route,
		    notes, withheld, withheld_reason, tenant_id, administered_at)
		 VALUES
		   (@medication_id, @admission_id, @administered_by, @actual_dose, @route,
		    @notes, @withheld, @withheld_reason, @tenant_id, now())
		 RETURNING id, medication_id, admission_id, administered_by, actual_dose, route,
		           notes, withheld, withheld_reason, tenant_id, administered_at`,
		db.NamedArgs{
			"medication_id":   medID,
			"admission_id":    admissionID,
			"administered_by": req.AdministeredBy,
			"actual_dose":     actualDose,
			"route":           route,
			"notes":           req.Notes,
			"withheld":        req.Withheld,
			"withheld_reason": req.WithheldReason,
			"tenant_id":       tenantID,
		},
	)
	var rec MedAdministrationRecord
	if err := row.Scan(
		&rec.ID, &rec.MedicationID, &rec.AdmissionID, &rec.AdministeredBy,
		&rec.ActualDose, &rec.Route, &rec.Notes, &rec.Withheld, &rec.WithheldReason,
		&rec.TenantID, &rec.AdministeredAt,
	); err != nil {
		return MedAdministrationRecord{}, fmt.Errorf("insert administration record: %w", err)
	}
	return rec, nil
}

func (h *PharmacyHandler) getReconciliation(ctx context.Context, admissionID, tenantID string) (MedReconciliation, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, admission_id, clinician_hpi, reconciliation_type,
		        home_medications, chart_medications, discrepancies, actions_taken, clinical_notes,
		        tenant_id, completed_at
		 FROM med_reconciliations
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		 ORDER BY completed_at DESC LIMIT 1`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID},
	)
	var rec MedReconciliation
	if err := row.Scan(
		&rec.ID, &rec.AdmissionID, &rec.ClinicianHPI, &rec.ReconciliationType,
		&rec.HomeMedications, &rec.ChartMedications, &rec.Discrepancies, &rec.ActionsTaken, &rec.ClinicalNotes,
		&rec.TenantID, &rec.CompletedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return MedReconciliation{}, errNotFound
		}
		return MedReconciliation{}, fmt.Errorf("get reconciliation: %w", err)
	}
	return rec, nil
}

func (h *PharmacyHandler) insertReconciliation(ctx context.Context, admissionID string, req medReconcileRequest, chartNames []string, tenantID string) (MedReconciliation, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO med_reconciliations
		   (admission_id, clinician_hpi, reconciliation_type,
		    home_medications, chart_medications, discrepancies, actions_taken, clinical_notes,
		    tenant_id, completed_at)
		 VALUES
		   (@admission_id, @clinician_hpi, @reconciliation_type,
		    @home_medications, @chart_medications, @discrepancies, @actions_taken, @clinical_notes,
		    @tenant_id, now())
		 RETURNING id, admission_id, clinician_hpi, reconciliation_type,
		           home_medications, chart_medications, discrepancies, actions_taken, clinical_notes,
		           tenant_id, completed_at`,
		db.NamedArgs{
			"admission_id":        admissionID,
			"clinician_hpi":       req.ClinicianHPI,
			"reconciliation_type": req.ReconciliationType,
			"home_medications":    req.HomeMedications,
			"chart_medications":   chartNames,
			"discrepancies":       req.Discrepancies,
			"actions_taken":       req.ActionsTaken,
			"clinical_notes":      req.ClinicalNotes,
			"tenant_id":           tenantID,
		},
	)
	var rec MedReconciliation
	if err := row.Scan(
		&rec.ID, &rec.AdmissionID, &rec.ClinicianHPI, &rec.ReconciliationType,
		&rec.HomeMedications, &rec.ChartMedications, &rec.Discrepancies, &rec.ActionsTaken, &rec.ClinicalNotes,
		&rec.TenantID, &rec.CompletedAt,
	); err != nil {
		return MedReconciliation{}, fmt.Errorf("insert reconciliation: %w", err)
	}
	return rec, nil
}

func scanMedRow(row dbRow) (InpatientMedication, error) {
	var m InpatientMedication
	if err := row.Scan(
		&m.ID, &m.AdmissionID, &m.PatientID, &m.PrescriberHPI,
		&m.GenericName, &m.BrandName, &m.NZMTCode,
		&m.Dose, &m.Route, &m.Frequency, &m.MaxDailyDose, &m.Indication,
		&m.StartDate, &m.EndDate, &m.Status,
		&m.IsIV, &m.IVRate, &m.AllergiesChecked,
		&m.TenantID, &m.CeasedAt, &m.CeasedReason, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return InpatientMedication{}, err
	}
	return m, nil
}
