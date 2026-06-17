package api

import (
	"context"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/pharmac"
)

// getActiveNZULMCodes returns the NZULM codes for the patient's current active medications.
func (h *PrescriptionsHandler) getActiveNZULMCodes(ctx context.Context, patientID, tenantID string) ([]string, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT nzulm_code FROM prescriptions
		 WHERE patient_id = @patient_id
		   AND tenant_id  = @tenant_id
		   AND status     = 'active'`,
		db.NamedArgs{"patient_id": patientID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query active NZULM codes: %w", err)
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan NZULM code: %w", err)
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

// listPrescriptions queries the prescriptions table with optional filters.
func (h *PrescriptionsHandler) listPrescriptions(
	ctx context.Context,
	tenantID, patientFilter, statusFilter, providerFilter string,
) ([]Prescription, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, encounter_id,
		        nzulm_code, medication_name, status,
		        dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		        repeats, repeats_remaining, tenant_id,
		        issued_at, expires_at, created_at, updated_at
		 FROM prescriptions
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter  = '' OR patient_id        = @patient_filter)
		   AND (@status_filter   = '' OR status             = @status_filter)
		   AND (@provider_filter = '' OR practitioner_hpi  = @provider_filter)
		 ORDER BY issued_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":       tenantID,
			"patient_filter":  patientFilter,
			"status_filter":   statusFilter,
			"provider_filter": providerFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query prescriptions: %w", err)
	}
	defer rows.Close()

	var results []Prescription
	for rows.Next() {
		rx, err := scanPrescription(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rx)
	}
	return results, rows.Err()
}

// getPrescriptionByID retrieves a single prescription with tenant isolation.
func (h *PrescriptionsHandler) getPrescriptionByID(ctx context.Context, id, tenantID string) (Prescription, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, encounter_id,
		        nzulm_code, medication_name, status,
		        dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		        repeats, repeats_remaining, tenant_id,
		        issued_at, expires_at, created_at, updated_at
		 FROM prescriptions
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	rx, err := scanPrescription(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Prescription{}, errNotFound
		}
		return Prescription{}, fmt.Errorf("get prescription by id: %w", err)
	}
	return rx, nil
}

// insertPrescription persists a validated prescription.
func (h *PrescriptionsHandler) insertPrescription(
	ctx context.Context,
	req prescriptionCreateRequest,
	medication *pharmac.Medicine,
	subsidised bool,
	warnings []string,
	tenantID string,
) (Prescription, error) {
	subsidyCode := ""

	row := h.pool.QueryRow(ctx,
		`INSERT INTO prescriptions
		   (patient_id, patient_nhi, practitioner_hpi, encounter_id,
		    nzulm_code, medication_name, status,
		    dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		    repeats, repeats_remaining, tenant_id, issued_at)
		 VALUES
		   (@patient_id, @patient_nhi, @practitioner_hpi, @encounter_id,
		    @nzulm_code, @medication_name, @status,
		    @dosage, @pharmac_subsidised, @subsidy_code, @interaction_warnings,
		    @repeats, @repeats_remaining, @tenant_id, now())
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, encounter_id,
		           nzulm_code, medication_name, status,
		           dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		           repeats, repeats_remaining, tenant_id,
		           issued_at, expires_at, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":           req.PatientID,
			"patient_nhi":          req.PatientNHI,
			"practitioner_hpi":     req.PractitionerHPI,
			"encounter_id":         req.EncounterID,
			"nzulm_code":           req.NZULMCode,
			"medication_name":      medication.GenericName,
			"status":               PrescriptionStatusActive,
			"dosage":               req.Dosage,
			"pharmac_subsidised":   subsidised,
			"subsidy_code":         subsidyCode,
			"interaction_warnings": warnings,
			"repeats":              req.Repeats,
			"repeats_remaining":    req.Repeats,
			"tenant_id":            tenantID,
		},
	)
	rx, err := scanPrescription(row)
	if err != nil {
		return Prescription{}, fmt.Errorf("insert prescription: %w", err)
	}
	return rx, nil
}

// updatePrescription writes status/dosage/repeat changes back to the database.
func (h *PrescriptionsHandler) updatePrescription(ctx context.Context, rx Prescription) (Prescription, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE prescriptions
		 SET status     = @status,
		     dosage     = @dosage,
		     repeats    = @repeats,
		     updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, encounter_id,
		           nzulm_code, medication_name, status,
		           dosage, pharmac_subsidised, subsidy_code, interaction_warnings,
		           repeats, repeats_remaining, tenant_id,
		           issued_at, expires_at, created_at, updated_at`,
		db.NamedArgs{
			"status":    rx.Status,
			"dosage":    rx.Dosage,
			"repeats":   rx.Repeats,
			"id":        rx.ID,
			"tenant_id": rx.TenantID,
		},
	)
	updated, err := scanPrescription(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Prescription{}, errNotFound
		}
		return Prescription{}, fmt.Errorf("update prescription: %w", err)
	}
	return updated, nil
}

// scanPrescription scans a single Prescription from a row (pgx.Row or pgx.Rows).
func scanPrescription(row dbRow) (Prescription, error) {
	var rx Prescription
	if err := row.Scan(
		&rx.ID, &rx.PatientID, &rx.PatientNHI, &rx.PractitionerHPI, &rx.EncounterID,
		&rx.NZULMCode, &rx.MedicationName, &rx.Status,
		&rx.Dosage, &rx.PHARMACSubsidised, &rx.SubsidyCode, &rx.InteractionWarnings,
		&rx.Repeats, &rx.RepeatsRemaining, &rx.TenantID,
		&rx.IssuedAt, &rx.ExpiresAt, &rx.CreatedAt, &rx.UpdatedAt,
	); err != nil {
		return Prescription{}, err
	}
	return rx, nil
}
