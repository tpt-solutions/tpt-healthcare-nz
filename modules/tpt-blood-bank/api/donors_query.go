package api

import (
	"context"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

func (h *DonorsHandler) listDonors(ctx context.Context, tenantID, statusFilter, bloodGroupFilter, nhiFilter string) ([]Donor, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, nhi, blood_group, rhd, status,
		        deferral_reason, deferral_end_date,
		        total_donations, last_donation_at, haemoglobin_gdl,
		        created_at, updated_at
		 FROM donors
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter      = '' OR status       = @status_filter)
		   AND (@blood_group_filter = '' OR blood_group  = @blood_group_filter)
		   AND (@nhi_filter         = '' OR nhi          = @nhi_filter)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":          tenantID,
			"status_filter":      statusFilter,
			"blood_group_filter": bloodGroupFilter,
			"nhi_filter":         nhiFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query donors: %w", err)
	}
	defer rows.Close()

	var results []Donor
	for rows.Next() {
		var d Donor
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.NHI, &d.BloodGroup, &d.RhD, &d.Status,
			&d.DeferralReason, &d.DeferralEndDate,
			&d.TotalDonations, &d.LastDonationAt, &d.HaemoglobinGDL,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan donor row: %w", err)
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

func (h *DonorsHandler) listEligibleDonors(ctx context.Context, tenantID, bloodGroupFilter string) ([]Donor, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, nhi, blood_group, rhd, status,
		        deferral_reason, deferral_end_date,
		        total_donations, last_donation_at, haemoglobin_gdl,
		        created_at, updated_at
		 FROM donors
		 WHERE tenant_id = @tenant_id
		   AND status = 'active'
		   AND (deferral_end_date IS NULL OR deferral_end_date < now())
		   AND (@blood_group_filter = '' OR blood_group = @blood_group_filter)
		 ORDER BY last_donation_at ASC NULLS FIRST
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":          tenantID,
			"blood_group_filter": bloodGroupFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query eligible donors: %w", err)
	}
	defer rows.Close()

	var results []Donor
	for rows.Next() {
		var d Donor
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.NHI, &d.BloodGroup, &d.RhD, &d.Status,
			&d.DeferralReason, &d.DeferralEndDate,
			&d.TotalDonations, &d.LastDonationAt, &d.HaemoglobinGDL,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan eligible donor row: %w", err)
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

func (h *DonorsHandler) getDonorByID(ctx context.Context, id, tenantID string) (Donor, error) {
	var d Donor
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, nhi, blood_group, rhd, status,
		        deferral_reason, deferral_end_date,
		        total_donations, last_donation_at, haemoglobin_gdl,
		        created_at, updated_at
		 FROM donors
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&d.ID, &d.TenantID, &d.NHI, &d.BloodGroup, &d.RhD, &d.Status,
		&d.DeferralReason, &d.DeferralEndDate,
		&d.TotalDonations, &d.LastDonationAt, &d.HaemoglobinGDL,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Donor{}, errNotFound
		}
		return Donor{}, fmt.Errorf("get donor by id: %w", err)
	}
	return d, nil
}

func (h *DonorsHandler) insertDonor(ctx context.Context, nhi string, bloodGroup BloodGroup, rhd, tenantID string) (Donor, error) {
	var d Donor
	err := h.pool.QueryRow(ctx,
		`INSERT INTO donors (nhi, blood_group, rhd, status, tenant_id)
		 VALUES (@nhi, @blood_group, @rhd, 'active', @tenant_id)
		 RETURNING id, tenant_id, nhi, blood_group, rhd, status,
		           deferral_reason, deferral_end_date,
		           total_donations, last_donation_at, haemoglobin_gdl,
		           created_at, updated_at`,
		db.NamedArgs{
			"nhi":         nhi,
			"blood_group": bloodGroup,
			"rhd":         rhd,
			"tenant_id":   tenantID,
		},
	).Scan(
		&d.ID, &d.TenantID, &d.NHI, &d.BloodGroup, &d.RhD, &d.Status,
		&d.DeferralReason, &d.DeferralEndDate,
		&d.TotalDonations, &d.LastDonationAt, &d.HaemoglobinGDL,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return Donor{}, fmt.Errorf("insert donor: %w", err)
	}
	return d, nil
}

func (h *DonorsHandler) updateDonor(ctx context.Context, d Donor) (Donor, error) {
	var updated Donor
	err := h.pool.QueryRow(ctx,
		`UPDATE donors
		 SET blood_group       = @blood_group,
		     rhd               = @rhd,
		     status            = @status,
		     deferral_reason   = @deferral_reason,
		     deferral_end_date = @deferral_end_date,
		     haemoglobin_gdl   = @haemoglobin_gdl,
		     updated_at        = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, nhi, blood_group, rhd, status,
		           deferral_reason, deferral_end_date,
		           total_donations, last_donation_at, haemoglobin_gdl,
		           created_at, updated_at`,
		db.NamedArgs{
			"blood_group":       d.BloodGroup,
			"rhd":               d.RhD,
			"status":            d.Status,
			"deferral_reason":   d.DeferralReason,
			"deferral_end_date": d.DeferralEndDate,
			"haemoglobin_gdl":   d.HaemoglobinGDL,
			"id":                d.ID,
			"tenant_id":         d.TenantID,
		},
	).Scan(
		&updated.ID, &updated.TenantID, &updated.NHI, &updated.BloodGroup, &updated.RhD, &updated.Status,
		&updated.DeferralReason, &updated.DeferralEndDate,
		&updated.TotalDonations, &updated.LastDonationAt, &updated.HaemoglobinGDL,
		&updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Donor{}, errNotFound
		}
		return Donor{}, fmt.Errorf("update donor: %w", err)
	}
	return updated, nil
}

func (h *DonorsHandler) getDonations(ctx context.Context, donorID, tenantID string) ([]DonationRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, donor_id, product_unit_id, volume_ml, donation_type, collected_at, created_at
		 FROM donations
		 WHERE donor_id = @donor_id
		   AND tenant_id = @tenant_id
		 ORDER BY collected_at DESC
		 LIMIT 100`,
		db.NamedArgs{"donor_id": donorID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query donations: %w", err)
	}
	defer rows.Close()

	var results []DonationRecord
	for rows.Next() {
		var rec DonationRecord
		if err := rows.Scan(&rec.ID, &rec.DonorID, &rec.ProductUnitID, &rec.VolumeML, &rec.DonationType, &rec.CollectedAt, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan donation row: %w", err)
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}
