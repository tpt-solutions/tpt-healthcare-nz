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
		        barcode, is_controlled_drug, controlled_drug_schedule,
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
		        barcode, is_controlled_drug, controlled_drug_schedule,
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
		    start_date, end_date, status, is_iv, iv_rate,
		    barcode, is_controlled_drug, controlled_drug_schedule, tenant_id)
		 VALUES
		   (@admission_id, @patient_id, @prescriber_hpi, @generic_name, @brand_name,
		    @nzmt_code, @dose, @route, @frequency, @max_daily_dose, @indication,
		    @start_date, @end_date, @status, @is_iv, @iv_rate,
		    @barcode, @is_controlled_drug, @controlled_drug_schedule, @tenant_id)
		 RETURNING id, admission_id, patient_id, prescriber_hpi, generic_name, brand_name,
		           nzmt_code, dose, route, frequency, max_daily_dose, indication,
		           start_date, end_date, status, is_iv, iv_rate, allergies_checked,
		           barcode, is_controlled_drug, controlled_drug_schedule,
		           tenant_id, ceased_at, ceased_reason, created_at, updated_at`,
		db.NamedArgs{
			"admission_id":             admissionID,
			"patient_id":               patientID,
			"prescriber_hpi":           req.PrescriberHPI,
			"generic_name":             req.GenericName,
			"brand_name":               req.BrandName,
			"nzmt_code":                req.NZMTCode,
			"dose":                     req.Dose,
			"route":                    req.Route,
			"frequency":                req.Frequency,
			"max_daily_dose":           req.MaxDailyDose,
			"indication":               req.Indication,
			"start_date":               req.StartDate,
			"end_date":                 req.EndDate,
			"status":                   InpatientMedStatusActive,
			"is_iv":                    req.IsIV,
			"iv_rate":                  req.IVRate,
			"barcode":                  req.Barcode,
			"is_controlled_drug":       req.IsControlledDrug,
			"controlled_drug_schedule": req.ControlledDrugSchedule,
			"tenant_id":                tenantID,
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
		           barcode, is_controlled_drug, controlled_drug_schedule,
		           tenant_id, ceased_at, ceased_reason, created_at, updated_at`,
		db.NamedArgs{
			"dose":           m.Dose,
			"frequency":      m.Frequency,
			"max_daily_dose": m.MaxDailyDose,
			"indication":     m.Indication,
			"end_date":       m.EndDate,
			"status":         m.Status,
			"iv_rate":        m.IVRate,
			"ceased_at":      m.CeasedAt,
			"ceased_reason":  m.CeasedReason,
			"id":             m.ID,
			"admission_id":   m.AdmissionID,
			"tenant_id":      m.TenantID,
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

func (h *PharmacyHandler) insertAdminRecord(ctx context.Context, medID, admissionID string, med InpatientMedication, req medAdminRequest, verification, tenantID string) (MedAdministrationRecord, error) {
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
		    notes, withheld, withheld_reason,
		    patient_barcode_scanned, med_barcode_scanned, verification_method,
		    five_rights_confirmed, witness_hpi, tenant_id, administered_at)
		 VALUES
		   (@medication_id, @admission_id, @administered_by, @actual_dose, @route,
		    @notes, @withheld, @withheld_reason,
		    @patient_barcode, @med_barcode, @verification_method,
		    @five_rights_confirmed, @witness_hpi, @tenant_id, now())
		 RETURNING id, medication_id, admission_id, administered_by, actual_dose, route,
		           notes, withheld, withheld_reason,
		           patient_barcode_scanned, med_barcode_scanned, verification_method,
		           five_rights_confirmed, witness_hpi, tenant_id, administered_at`,
		db.NamedArgs{
			"medication_id":         medID,
			"admission_id":          admissionID,
			"administered_by":       req.AdministeredBy,
			"actual_dose":           actualDose,
			"route":                 route,
			"notes":                 req.Notes,
			"withheld":              req.Withheld,
			"withheld_reason":       req.WithheldReason,
			"patient_barcode":       req.PatientBarcode,
			"med_barcode":           req.MedBarcode,
			"verification_method":   verification,
			"five_rights_confirmed": verification == "barcode",
			"witness_hpi":           req.WitnessHPI,
			"tenant_id":             tenantID,
		},
	)
	var rec MedAdministrationRecord
	if err := row.Scan(
		&rec.ID, &rec.MedicationID, &rec.AdmissionID, &rec.AdministeredBy,
		&rec.ActualDose, &rec.Route, &rec.Notes, &rec.Withheld, &rec.WithheldReason,
		&rec.PatientBarcodeScanned, &rec.MedBarcodeScanned, &rec.VerificationMethod,
		&rec.FiveRightsConfirmed, &rec.WitnessHPI,
		&rec.TenantID, &rec.AdministeredAt,
	); err != nil {
		return MedAdministrationRecord{}, fmt.Errorf("insert administration record: %w", err)
	}
	return rec, nil
}

// controlledDrugBalance returns the running balance for a drug on this
// admission's register (0 if no entries yet).
func (h *PharmacyHandler) controlledDrugBalance(ctx context.Context, admissionID, drugName, tenantID string) (float64, error) {
	var balance float64
	err := h.pool.QueryRow(ctx,
		`SELECT balance_after FROM controlled_drug_register
		 WHERE admission_id = @admission_id AND drug_name = @drug_name AND tenant_id = @tenant_id
		 ORDER BY recorded_at DESC LIMIT 1`,
		db.NamedArgs{"admission_id": admissionID, "drug_name": drugName, "tenant_id": tenantID},
	).Scan(&balance)
	if err != nil {
		if db.IsNoRows(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("get controlled drug balance: %w", err)
	}
	return balance, nil
}

// insertControlledDrugEntry appends a dual-signed entry to the controlled
// drug register and returns the new running balance.
func (h *PharmacyHandler) insertControlledDrugEntry(ctx context.Context, admissionID, medicationID string, entry controlledDrugEntryRequest, tenantID string) (ControlledDrugRegisterEntry, error) {
	priorBalance, err := h.controlledDrugBalance(ctx, admissionID, entry.DrugName, tenantID)
	if err != nil {
		return ControlledDrugRegisterEntry{}, err
	}

	balanceAfter := priorBalance
	switch entry.Action {
	case "administered", "wasted", "returned":
		balanceAfter -= entry.Quantity
	case "stock-received":
		balanceAfter += entry.Quantity
	case "stock-count":
		balanceAfter = entry.Quantity
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO controlled_drug_register
		   (admission_id, medication_id, drug_name, schedule, action, quantity,
		    balance_after, administered_by, witness_hpi, notes, tenant_id, recorded_at)
		 VALUES
		   (@admission_id, @medication_id, @drug_name, @schedule, @action, @quantity,
		    @balance_after, @administered_by, @witness_hpi, @notes, @tenant_id, now())
		 RETURNING id, admission_id, medication_id, drug_name, schedule, action, quantity,
		           balance_after, administered_by, witness_hpi, notes, tenant_id, recorded_at`,
		db.NamedArgs{
			"admission_id":    admissionID,
			"medication_id":   medicationID,
			"drug_name":       entry.DrugName,
			"schedule":        entry.Schedule,
			"action":          entry.Action,
			"quantity":        entry.Quantity,
			"balance_after":   balanceAfter,
			"administered_by": entry.AdministeredBy,
			"witness_hpi":     entry.WitnessHPI,
			"notes":           entry.Notes,
			"tenant_id":       tenantID,
		},
	)
	var e ControlledDrugRegisterEntry
	var medID *string
	if err := row.Scan(
		&e.ID, &e.AdmissionID, &medID, &e.DrugName, &e.Schedule, &e.Action, &e.Quantity,
		&e.BalanceAfter, &e.AdministeredBy, &e.WitnessHPI, &e.Notes, &e.TenantID, &e.RecordedAt,
	); err != nil {
		return ControlledDrugRegisterEntry{}, fmt.Errorf("insert controlled drug register entry: %w", err)
	}
	if medID != nil {
		e.MedicationID = *medID
	}
	return e, nil
}

func (h *PharmacyHandler) listControlledDrugRegister(ctx context.Context, admissionID, tenantID string) ([]ControlledDrugRegisterEntry, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, admission_id, medication_id, drug_name, schedule, action, quantity,
		        balance_after, administered_by, witness_hpi, notes, tenant_id, recorded_at
		 FROM controlled_drug_register
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		 ORDER BY recorded_at ASC`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query controlled drug register: %w", err)
	}
	defer rows.Close()

	var results []ControlledDrugRegisterEntry
	for rows.Next() {
		var e ControlledDrugRegisterEntry
		var medID *string
		if err := rows.Scan(
			&e.ID, &e.AdmissionID, &medID, &e.DrugName, &e.Schedule, &e.Action, &e.Quantity,
			&e.BalanceAfter, &e.AdministeredBy, &e.WitnessHPI, &e.Notes, &e.TenantID, &e.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan controlled drug register entry: %w", err)
		}
		if medID != nil {
			e.MedicationID = *medID
		}
		results = append(results, e)
	}
	return results, rows.Err()
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
		&m.Barcode, &m.IsControlledDrug, &m.ControlledDrugSchedule,
		&m.TenantID, &m.CeasedAt, &m.CeasedReason, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return InpatientMedication{}, err
	}
	return m, nil
}

// ── IV Infusion Database Helpers ──────────────────────────────────────────────

func (h *PharmacyHandler) insertIVInfusion(ctx context.Context, r *IVInfusionRecord, tenantID string) (IVInfusionRecord, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO iv_infusion_records
		   (medication_id, admission_id, pump_identifier, pump_type, rate,
		    concentration, vtbi, label_text, safety_soft_limit, safety_hard_limit,
		    status, tenant_id)
		 VALUES
		   (@medication_id, @admission_id, @pump_identifier, @pump_type, @rate,
		    @concentration, @vtbi, @label_text, @safety_soft_limit, @safety_hard_limit,
		    @status, @tenant_id)
		 RETURNING id, medication_id, admission_id, pump_identifier, pump_type, rate,
		           concentration, vtbi, volume_infused, dose_infused,
		           started_at, stopped_at, status, paused_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"medication_id":     r.MedicationID,
			"admission_id":      r.AdmissionID,
			"pump_identifier":   r.PumpIdentifier,
			"pump_type":         r.PumpType,
			"rate":              r.Rate,
			"concentration":     r.Concentration,
			"vtbi":              r.VTBI,
			"label_text":        r.LabelText,
			"safety_soft_limit": r.SafetySoftLimit,
			"safety_hard_limit": r.SafetyHardLimit,
			"status":            r.Status,
			"tenant_id":         tenantID,
		},
	)
	var created IVInfusionRecord
	if err := row.Scan(
		&created.ID, &created.MedicationID, &created.AdmissionID, &created.PumpIdentifier,
		&created.PumpType, &created.Rate, &created.Concentration, &created.VTBI,
		&created.VolumeInfused, &created.DoseInfused,
		&created.StartedAt, &created.StoppedAt, &created.Status, &created.PausedAt,
		&created.TenantID, &created.CreatedAt, &created.UpdatedAt,
	); err != nil {
		return IVInfusionRecord{}, fmt.Errorf("insert IV infusion: %w", err)
	}
	return created, nil
}

func (h *PharmacyHandler) updateIVInfusionStatus(ctx context.Context, admissionID, medID string, req IVStatusUpdate, tenantID string) (IVInfusionRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE iv_infusion_records
		 SET status = @status,
		     volume_infused = @volume_infused,
		     dose_infused = @dose_infused,
		     stopped_at = CASE WHEN @status IN ('stopped', 'completed', 'error') THEN now() ELSE stopped_at END,
		     updated_at = now()
		 WHERE admission_id = @admission_id AND medication_id = @medication_id AND tenant_id = @tenant_id
		   AND status = 'running'
		 RETURNING id, medication_id, admission_id, pump_identifier, pump_type, rate,
		           concentration, vtbi, volume_infused, dose_infused,
		           started_at, stopped_at, status, paused_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"status":         req.Status,
			"volume_infused": req.VolumeInfused,
			"dose_infused":   req.DoseInfused,
			"admission_id":   admissionID,
			"medication_id":  medID,
			"tenant_id":      tenantID,
		},
	)
	var updated IVInfusionRecord
	if err := row.Scan(
		&updated.ID, &updated.MedicationID, &updated.AdmissionID, &updated.PumpIdentifier,
		&updated.PumpType, &updated.Rate, &updated.Concentration, &updated.VTBI,
		&updated.VolumeInfused, &updated.DoseInfused,
		&updated.StartedAt, &updated.StoppedAt, &updated.Status, &updated.PausedAt,
		&updated.TenantID, &updated.CreatedAt, &updated.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return IVInfusionRecord{}, errNotFound
		}
		return IVInfusionRecord{}, fmt.Errorf("update IV infusion status: %w", err)
	}
	return updated, nil
}

func (h *PharmacyHandler) listIVInfusions(ctx context.Context, admissionID, medID, tenantID string) ([]IVInfusionRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, medication_id, admission_id, pump_identifier, pump_type, rate,
		        concentration, vtbi, volume_infused, dose_infused,
		        started_at, stopped_at, status, paused_at, tenant_id, created_at, updated_at
		 FROM iv_infusion_records
		 WHERE admission_id = @admission_id AND medication_id = @medication_id AND tenant_id = @tenant_id
		 ORDER BY created_at DESC`,
		db.NamedArgs{
			"admission_id":  admissionID,
			"medication_id": medID,
			"tenant_id":     tenantID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query IV infusions: %w", err)
	}
	defer rows.Close()

	var results []IVInfusionRecord
	for rows.Next() {
		var r IVInfusionRecord
		if err := rows.Scan(
			&r.ID, &r.MedicationID, &r.AdmissionID, &r.PumpIdentifier,
			&r.PumpType, &r.Rate, &r.Concentration, &r.VTBI,
			&r.VolumeInfused, &r.DoseInfused,
			&r.StartedAt, &r.StoppedAt, &r.Status, &r.PausedAt,
			&r.TenantID, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan IV infusion: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
