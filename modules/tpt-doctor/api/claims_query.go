package api

import (
	"context"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

// listClaims queries the claims table with optional filters.
func (h *ClaimsHandler) listClaims(
	ctx context.Context,
	tenantID, patientFilter, statusFilter, providerFilter, fromDate, toDate string,
) ([]Claim, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		        form_type, form_number, diagnosis_codes,
		        injury_date, injury_description, status,
		        acc_claim_number, rejection_reason, paid_amount,
		        tenant_id, created_at, updated_at, submitted_at,
		        claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism
		 FROM acc_claims
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter  = '' OR patient_id        = @patient_filter)
		   AND (@status_filter   = '' OR status             = @status_filter)
		   AND (@provider_filter = '' OR practitioner_hpi  = @provider_filter)
		   AND (@from_date       = '' OR injury_date       >= @from_date::date)
		   AND (@to_date         = '' OR injury_date       <= @to_date::date)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":       tenantID,
			"patient_filter":  patientFilter,
			"status_filter":   statusFilter,
			"provider_filter": providerFilter,
			"from_date":       fromDate,
			"to_date":         toDate,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query claims: %w", err)
	}
	defer rows.Close()

	var results []Claim
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// getClaimByID retrieves a single claim with tenant isolation.
func (h *ClaimsHandler) getClaimByID(ctx context.Context, id, tenantID string) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		        form_type, form_number, diagnosis_codes,
		        injury_date, injury_description, status,
		        acc_claim_number, rejection_reason, paid_amount,
		        tenant_id, created_at, updated_at, submitted_at,
		        claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism
		 FROM acc_claims
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	c, err := scanClaim(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Claim{}, errNotFound
		}
		return Claim{}, fmt.Errorf("get claim by id: %w", err)
	}
	return c, nil
}

// insertClaim persists a new claim in draft status.
func (h *ClaimsHandler) insertClaim(ctx context.Context, req claimCreateRequest, tenantID string) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO acc_claims
		   (encounter_id, patient_id, patient_nhi, practitioner_hpi,
		    form_type, diagnosis_codes, injury_date, injury_description,
		    status, tenant_id, claim_destination, employer_nzbn, injury_mechanism)
		 VALUES
		   (@encounter_id, @patient_id, @patient_nhi, @practitioner_hpi,
		    @form_type, @diagnosis_codes, @injury_date, @injury_description,
		    @status, @tenant_id, @claim_destination, @employer_nzbn, @injury_mechanism)
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at,
		           claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism`,
		db.NamedArgs{
			"encounter_id":       req.EncounterID,
			"patient_id":         req.PatientID,
			"patient_nhi":        req.PatientNHI,
			"practitioner_hpi":   req.PractitionerHPI,
			"form_type":          req.FormType,
			"diagnosis_codes":    req.DiagnosisCodes,
			"injury_date":        req.InjuryDate,
			"injury_description": req.InjuryDescription,
			"status":             ClaimStatusDraft,
			"tenant_id":          tenantID,
			"claim_destination":  req.Destination,
			"employer_nzbn":      req.EmployerNZBN,
			"injury_mechanism":   req.InjuryMechanism,
		},
	)
	c, err := scanClaim(row)
	if err != nil {
		return Claim{}, fmt.Errorf("insert claim: %w", err)
	}
	return c, nil
}

// reserveClaimForSubmit atomically transitions a claim from draft → submitted.
// Returns the reserved claim, or errNotFound if the claim is not in draft status.
// This prevents concurrent requests from lodging the same claim twice (TOCTOU prevention).
func (h *ClaimsHandler) reserveClaimForSubmit(ctx context.Context, id, tenantID string) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE acc_claims
		 SET status     = @status,
		     updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at,
		           claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism`,
		db.NamedArgs{
			"status":    ClaimStatusSubmitted,
			"id":        id,
			"tenant_id": tenantID,
		},
	)
	c, err := scanClaim(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Claim{}, errNotFound
		}
		return Claim{}, fmt.Errorf("reserve claim for submit: %w", err)
	}
	return c, nil
}

// resetClaimToDraft rolls back a reserved claim to draft status after a failed lodge.
// The condition requires both ref number columns to be empty to prevent accidentally
// rolling back a claim that was successfully lodged with the external system.
func (h *ClaimsHandler) resetClaimToDraft(ctx context.Context, id, tenantID string) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE acc_claims SET status = 'draft', updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id AND status = 'submitted'
		   AND acc_claim_number = '' AND worksafe_ref_number = ''`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	if err != nil {
		return fmt.Errorf("reset claim to draft: %w", err)
	}
	return nil
}

// updateClaimAfterSubmit persists the external system's response fields after a successful lodge.
func (h *ClaimsHandler) updateClaimAfterSubmit(ctx context.Context, c Claim) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE acc_claims
		 SET acc_claim_number    = @acc_claim_number,
		     worksafe_ref_number = @worksafe_ref_number,
		     submitted_at        = @submitted_at,
		     updated_at          = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at,
		           claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism`,
		db.NamedArgs{
			"acc_claim_number":    c.ACCClaimNumber,
			"worksafe_ref_number": c.WorkSafeRefNumber,
			"submitted_at":        c.SubmittedAt,
			"id":                  c.ID,
			"tenant_id":           c.TenantID,
		},
	)
	updated, err := scanClaim(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Claim{}, errNotFound
		}
		return Claim{}, fmt.Errorf("update claim after submit: %w", err)
	}
	return updated, nil
}

// updateClaimStatus syncs the externally-polled status back to the local database.
func (h *ClaimsHandler) updateClaimStatus(ctx context.Context, c Claim) (Claim, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE acc_claims
		 SET status           = @status,
		     rejection_reason = @rejection_reason,
		     paid_amount      = @paid_amount,
		     updated_at       = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, encounter_id, patient_id, patient_nhi, practitioner_hpi,
		           form_type, form_number, diagnosis_codes,
		           injury_date, injury_description, status,
		           acc_claim_number, rejection_reason, paid_amount,
		           tenant_id, created_at, updated_at, submitted_at,
		           claim_destination, worksafe_ref_number, employer_nzbn, injury_mechanism`,
		db.NamedArgs{
			"status":           c.Status,
			"rejection_reason": c.RejectionReason,
			"paid_amount":      c.PaidAmount,
			"id":               c.ID,
			"tenant_id":        c.TenantID,
		},
	)
	updated, err := scanClaim(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Claim{}, errNotFound
		}
		return Claim{}, fmt.Errorf("update claim status: %w", err)
	}
	return updated, nil
}

// scanClaim scans a single Claim from a pgx row or rows cursor.
func scanClaim(row dbRow) (Claim, error) {
	var c Claim
	var dest string
	if err := row.Scan(
		&c.ID, &c.EncounterID, &c.PatientID, &c.PatientNHI, &c.PractitionerHPI,
		&c.FormType, &c.FormNumber, &c.DiagnosisCodes,
		&c.InjuryDate, &c.InjuryDescription, &c.Status,
		&c.ACCClaimNumber, &c.RejectionReason, &c.PaidAmount,
		&c.TenantID, &c.CreatedAt, &c.UpdatedAt, &c.SubmittedAt,
		&dest, &c.WorkSafeRefNumber, &c.EmployerNZBN, &c.InjuryMechanism,
	); err != nil {
		return Claim{}, err
	}
	c.Destination = ClaimDestination(dest)
	return c, nil
}
