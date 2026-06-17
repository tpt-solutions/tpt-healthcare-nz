package api

import (
	"context"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// validateNHIFormat validates the NHI using the checksum-aware validator in core/nhi/.
// This covers both structural format and the Luhn check digit for old-format NHIs.
func validateNHIFormat(nhiValue string) error {
	if !nhi.ValidateNHI(nhiValue) {
		return fmt.Errorf("invalid NHI %q: must be a valid NHI number (AAA9999 or AAA99AA format with correct check digit)", nhiValue)
	}
	return nil
}

// checkDisclosureConsent returns true when an active disclosure consent exists
// for patientNHI within tenantID, per HIPC Rule 11.
func (h *PatientsHandler) checkDisclosureConsent(ctx context.Context, tenantID, patientNHI string) (bool, error) {
	var granted bool
	err := h.pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM consents
		     WHERE tenant_id   = @tenant_id
		       AND patient_nhi = @patient_nhi
		       AND consent_type = 'disclosure'
		       AND granted = TRUE
		       AND revoked_at IS NULL
		       AND (expires_at IS NULL OR expires_at > NOW())
		 )`,
		db.NamedArgs{"tenant_id": tenantID, "patient_nhi": patientNHI},
	).Scan(&granted)
	if err != nil {
		return false, fmt.Errorf("check disclosure consent: %w", err)
	}
	return granted, nil
}

// searchPatients queries the patient table for matching records within the tenant.
// All filter values are applied as ILIKE / exact matches against decrypted index columns.
func (h *PatientsHandler) searchPatients(ctx context.Context, tenantID, name, nhiFilter, dob string) ([]patientRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, nhi_encrypted, tenant_id, fhir_resource, created_at, updated_at
		 FROM patients
		 WHERE tenant_id = @tenant_id
		   AND (@name_filter = '' OR name_search ILIKE '%' || @name_filter || '%')
		   AND (@nhi_filter  = '' OR nhi_index = @nhi_filter)
		   AND (@dob_filter  = '' OR dob_index = @dob_filter)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":   tenantID,
			"name_filter": name,
			"nhi_filter":  nhiFilter,
			"dob_filter":  dob,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query patients: %w", err)
	}
	defer rows.Close()

	var results []patientRecord
	for rows.Next() {
		var rec patientRecord
		if err := rows.Scan(&rec.ID, &rec.NHIEncrypted, &rec.TenantID, &rec.FHIRResource, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan patient row: %w", err)
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

// getPatientByID retrieves a single patient record by internal UUID, enforcing tenant isolation.
func (h *PatientsHandler) getPatientByID(ctx context.Context, id, tenantID string) (patientRecord, error) {
	var rec patientRecord
	err := h.pool.QueryRow(ctx,
		`SELECT id, nhi_encrypted, tenant_id, fhir_resource, created_at, updated_at
		 FROM patients
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(&rec.ID, &rec.NHIEncrypted, &rec.TenantID, &rec.FHIRResource, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if db.IsNoRows(err) {
			return patientRecord{}, errNotFound
		}
		return patientRecord{}, fmt.Errorf("get patient by id: %w", err)
	}
	return rec, nil
}

// persistPatient encrypts PHI and inserts a new patient record.
func (h *PatientsHandler) persistPatient(ctx context.Context, nhiValue string, patient *r5.Patient, tenantID string) (patientRecord, error) {
	nhiEnc, err := h.enc.Encrypt([]byte(nhiValue))
	if err != nil {
		return patientRecord{}, fmt.Errorf("encrypt NHI: %w", err)
	}

	fhirJSON, err := patient.MarshalJSON()
	if err != nil {
		return patientRecord{}, fmt.Errorf("marshal patient FHIR: %w", err)
	}
	fhirEnc, err := h.enc.Encrypt(fhirJSON)
	if err != nil {
		return patientRecord{}, fmt.Errorf("encrypt FHIR resource: %w", err)
	}

	nameSearch := patient.SearchName()
	dobIndex := patient.BirthDate

	var rec patientRecord
	err = h.pool.QueryRow(ctx,
		`INSERT INTO patients (nhi_encrypted, nhi_index, tenant_id, fhir_resource, name_search, dob_index)
		 VALUES (@nhi_encrypted, @nhi_index, @tenant_id, @fhir_resource, @name_search, @dob_index)
		 RETURNING id, nhi_encrypted, tenant_id, fhir_resource, created_at, updated_at`,
		db.NamedArgs{
			"nhi_encrypted": nhiEnc,
			"nhi_index":     nhiValue, // stored encrypted at DB level via column-level encryption policy
			"tenant_id":     tenantID,
			"fhir_resource": fhirEnc,
			"name_search":   nameSearch,
			"dob_index":     dobIndex,
		},
	).Scan(&rec.ID, &rec.NHIEncrypted, &rec.TenantID, &rec.FHIRResource, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return patientRecord{}, fmt.Errorf("insert patient: %w", err)
	}
	return rec, nil
}

// updatePatientFHIR updates only the FHIR resource blob of an existing patient.
func (h *PatientsHandler) updatePatientFHIR(ctx context.Context, id string, patient *r5.Patient, tenantID string) (patientRecord, error) {
	fhirJSON, err := patient.MarshalJSON()
	if err != nil {
		return patientRecord{}, fmt.Errorf("marshal patient FHIR: %w", err)
	}
	fhirEnc, err := h.enc.Encrypt(fhirJSON)
	if err != nil {
		return patientRecord{}, fmt.Errorf("encrypt FHIR resource: %w", err)
	}
	nameSearch := patient.SearchName()

	var rec patientRecord
	err = h.pool.QueryRow(ctx,
		`UPDATE patients
		 SET fhir_resource = @fhir_resource,
		     name_search   = @name_search,
		     updated_at    = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, nhi_encrypted, tenant_id, fhir_resource, created_at, updated_at`,
		db.NamedArgs{
			"fhir_resource": fhirEnc,
			"name_search":   nameSearch,
			"id":            id,
			"tenant_id":     tenantID,
		},
	).Scan(&rec.ID, &rec.NHIEncrypted, &rec.TenantID, &rec.FHIRResource, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if db.IsNoRows(err) {
			return patientRecord{}, errNotFound
		}
		return patientRecord{}, fmt.Errorf("update patient FHIR: %w", err)
	}
	return rec, nil
}

// recordToResponse decrypts a patientRecord and returns an API-safe patientResponse.
func (h *PatientsHandler) recordToResponse(_ context.Context, rec patientRecord) (patientResponse, error) {
	nhiPlain, err := h.enc.Decrypt(rec.NHIEncrypted)
	if err != nil {
		return patientResponse{}, fmt.Errorf("decrypt NHI: %w", err)
	}

	fhirJSON, err := h.enc.Decrypt(rec.FHIRResource)
	if err != nil {
		return patientResponse{}, fmt.Errorf("decrypt FHIR resource: %w", err)
	}

	var patient r5.Patient
	if err := patient.UnmarshalJSON(fhirJSON); err != nil {
		return patientResponse{}, fmt.Errorf("unmarshal FHIR Patient: %w", err)
	}

	return patientResponse{
		ID:        rec.ID,
		NHI:       string(nhiPlain),
		TenantID:  rec.TenantID,
		Patient:   &patient,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}, nil
}
