package api

import (
	"context"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

func (h *AdmissionsHandler) listAdmissions(ctx context.Context, tenantID, statusFilter, wardFilter, typeFilter string) ([]Admission, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		        admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		        acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		        tenant_id, admitted_at, discharged_at, created_at, updated_at
		 FROM hospital_admissions
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		   AND (@ward_filter   = '' OR ward_id = @ward_filter)
		   AND (@type_filter   = '' OR admission_type = @type_filter)
		 ORDER BY admitted_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":     tenantID,
			"status_filter": statusFilter,
			"ward_filter":   wardFilter,
			"type_filter":   typeFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query admissions: %w", err)
	}
	defer rows.Close()

	var results []Admission
	for rows.Next() {
		adm, err := scanAdmissionRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, adm)
	}
	return results, rows.Err()
}

func (h *AdmissionsHandler) getAdmissionByID(ctx context.Context, id, tenantID string) (Admission, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		        admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		        acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		        tenant_id, admitted_at, discharged_at, created_at, updated_at
		 FROM hospital_admissions
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	adm, err := scanAdmissionRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Admission{}, errNotFound
		}
		return Admission{}, fmt.Errorf("get admission: %w", err)
	}
	return adm, nil
}

func (h *AdmissionsHandler) insertAdmission(ctx context.Context, req admissionCreateRequest, tenantID string) (Admission, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO hospital_admissions
		   (patient_id, patient_nhi, admitting_clinician_hpi, admission_type, status,
		    ward_id, bed_id, admission_reason, acc_claim_number, referring_facility_hpi,
		    tenant_id, admitted_at)
		 VALUES
		   (@patient_id, @patient_nhi, @admitting_clinician_hpi, @admission_type, @status,
		    @ward_id, @bed_id, @admission_reason, @acc_claim_number, @referring_facility_hpi,
		    @tenant_id, now())
		 RETURNING id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		           admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		           acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		           tenant_id, admitted_at, discharged_at, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":              req.PatientID,
			"patient_nhi":             req.PatientNHI,
			"admitting_clinician_hpi": req.AdmittingClinicianHPI,
			"admission_type":          req.AdmissionType,
			"status":                  AdmissionStatusAdmitted,
			"ward_id":                 req.WardID,
			"bed_id":                  req.BedID,
			"admission_reason":        req.AdmissionReason,
			"acc_claim_number":        req.ACCClaimNumber,
			"referring_facility_hpi":  req.ReferringFacilityHPI,
			"tenant_id":               tenantID,
		},
	)
	adm, err := scanAdmissionRow(row)
	if err != nil {
		return Admission{}, fmt.Errorf("insert admission: %w", err)
	}
	return adm, nil
}

func (h *AdmissionsHandler) updateAdmission(ctx context.Context, a Admission) (Admission, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hospital_admissions
		 SET responsible_clinician_hpi = @responsible_clinician_hpi,
		     status                    = @status,
		     ward_id                   = @ward_id,
		     bed_id                    = @bed_id,
		     admission_reason          = @admission_reason,
		     primary_diagnosis         = @primary_diagnosis,
		     updated_at                = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		           admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		           acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		           tenant_id, admitted_at, discharged_at, created_at, updated_at`,
		db.NamedArgs{
			"responsible_clinician_hpi": a.ResponsibleClinicianHPI,
			"status":                    a.Status,
			"ward_id":                   a.WardID,
			"bed_id":                    a.BedID,
			"admission_reason":          a.AdmissionReason,
			"primary_diagnosis":         a.PrimaryDiagnosis,
			"id":                        a.ID,
			"tenant_id":                 a.TenantID,
		},
	)
	updated, err := scanAdmissionRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Admission{}, errNotFound
		}
		return Admission{}, fmt.Errorf("update admission: %w", err)
	}
	return updated, nil
}

func (h *AdmissionsHandler) dischargeAdmission(ctx context.Context, a Admission) (Admission, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hospital_admissions
		 SET status                = @status,
		     discharged_at         = @discharged_at,
		     discharge_destination = @discharge_destination,
		     discharge_notes       = @discharge_notes,
		     updated_at            = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		           admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		           acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		           tenant_id, admitted_at, discharged_at, created_at, updated_at`,
		db.NamedArgs{
			"status":                a.Status,
			"discharged_at":         a.DischargedAt,
			"discharge_destination": a.DischargeDestination,
			"discharge_notes":       a.DischargeNotes,
			"id":                    a.ID,
			"tenant_id":             a.TenantID,
		},
	)
	discharged, err := scanAdmissionRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Admission{}, errNotFound
		}
		return Admission{}, fmt.Errorf("discharge admission: %w", err)
	}
	return discharged, nil
}

func (h *AdmissionsHandler) getDischargeSummary(ctx context.Context, admissionID, tenantID string) (DischargeSummary, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, admission_id, patient_id, author_hpi, admission_date, discharge_date,
		        primary_diagnosis, secondary_diagnoses, procedures_performed,
		        clinical_summary, discharge_condition, follow_up_plan, medications,
		        gp_notified, gp_notified_at, tenant_id, created_at
		 FROM hospital_discharge_summaries
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID},
	)
	var s DischargeSummary
	if err := row.Scan(
		&s.ID, &s.AdmissionID, &s.PatientID, &s.AuthorHPI,
		&s.AdmissionDate, &s.DischargeDate,
		&s.PrimaryDiagnosis, &s.SecondaryDiagnoses, &s.ProceduresPerformed,
		&s.ClinicalSummary, &s.DischargeCondition, &s.FollowUpPlan, &s.Medications,
		&s.GPNotified, &s.GPNotifiedAt, &s.TenantID, &s.CreatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return DischargeSummary{}, errNotFound
		}
		return DischargeSummary{}, fmt.Errorf("get discharge summary: %w", err)
	}
	return s, nil
}

func (h *AdmissionsHandler) insertDischargeSummary(ctx context.Context, admissionID string, adm Admission, req dischargeSummaryCreateRequest, tenantID string) (DischargeSummary, error) {
	var dischargeDate time.Time
	if adm.DischargedAt != nil {
		dischargeDate = *adm.DischargedAt
	}
	row := h.pool.QueryRow(ctx,
		`INSERT INTO hospital_discharge_summaries
		   (admission_id, patient_id, author_hpi, admission_date, discharge_date,
		    primary_diagnosis, secondary_diagnoses, procedures_performed,
		    clinical_summary, discharge_condition, follow_up_plan, medications, tenant_id)
		 VALUES
		   (@admission_id, @patient_id, @author_hpi, @admission_date, @discharge_date,
		    @primary_diagnosis, @secondary_diagnoses, @procedures_performed,
		    @clinical_summary, @discharge_condition, @follow_up_plan, @medications, @tenant_id)
		 RETURNING id, admission_id, patient_id, author_hpi, admission_date, discharge_date,
		           primary_diagnosis, secondary_diagnoses, procedures_performed,
		           clinical_summary, discharge_condition, follow_up_plan, medications,
		           gp_notified, gp_notified_at, tenant_id, created_at`,
		db.NamedArgs{
			"admission_id":         admissionID,
			"patient_id":           adm.PatientID,
			"author_hpi":           req.AuthorHPI,
			"admission_date":       adm.AdmittedAt,
			"discharge_date":       dischargeDate,
			"primary_diagnosis":    req.PrimaryDiagnosis,
			"secondary_diagnoses":  req.SecondaryDiagnoses,
			"procedures_performed": req.ProceduresPerformed,
			"clinical_summary":     req.ClinicalSummary,
			"discharge_condition":  req.DischargeCondition,
			"follow_up_plan":       req.FollowUpPlan,
			"medications":          req.Medications,
			"tenant_id":            tenantID,
		},
	)
	var s DischargeSummary
	if err := row.Scan(
		&s.ID, &s.AdmissionID, &s.PatientID, &s.AuthorHPI,
		&s.AdmissionDate, &s.DischargeDate,
		&s.PrimaryDiagnosis, &s.SecondaryDiagnoses, &s.ProceduresPerformed,
		&s.ClinicalSummary, &s.DischargeCondition, &s.FollowUpPlan, &s.Medications,
		&s.GPNotified, &s.GPNotifiedAt, &s.TenantID, &s.CreatedAt,
	); err != nil {
		return DischargeSummary{}, fmt.Errorf("insert discharge summary: %w", err)
	}
	return s, nil
}

// dbRow is the scanner interface satisfied by both pgx.Row and pgx.Rows.
type dbRow interface {
	Scan(dest ...any) error
}

func scanAdmissionRow(row dbRow) (Admission, error) {
	var a Admission
	if err := row.Scan(
		&a.ID, &a.PatientID, &a.PatientNHI, &a.AdmittingClinicianHPI, &a.ResponsibleClinicianHPI,
		&a.AdmissionType, &a.Status, &a.WardID, &a.BedID, &a.AdmissionReason, &a.PrimaryDiagnosis,
		&a.ACCClaimNumber, &a.ReferringFacilityHPI, &a.DischargeDestination, &a.DischargeNotes,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return Admission{}, err
	}
	return a, nil
}
