package api

import (
	"context"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

// dbRow is satisfied by both pgx.Row and pgx.Rows.
type dbRow interface {
	Scan(dest ...any) error
}

// getProductByID retrieves a single blood product for crossmatch compatibility checks.
func (h *CrossmatchHandler) getProductByID(ctx context.Context, id, tenantID string) (BloodProduct, error) {
	var p BloodProduct
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, product_type, abo, rhd, donation_id, donor_id,
		        status, volume_ml, collection_date, expiry_date,
		        test_results, storage_location, created_at, updated_at
		 FROM blood_products
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&p.ID, &p.TenantID, &p.ProductType, &p.ABO, &p.RhD, &p.DonationID, &p.DonorID,
		&p.Status, &p.VolumeML, &p.CollectionDate, &p.ExpiryDate,
		&p.TestResults, &p.StorageLocation, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return BloodProduct{}, errNotFound
		}
		return BloodProduct{}, fmt.Errorf("get product by id: %w", err)
	}
	return p, nil
}

// updateProductStatus updates a single blood product's status.
func (h *CrossmatchHandler) updateProductStatus(ctx context.Context, productID string, status ProductStatus, tenantID string) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE blood_products
		 SET status = @status, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{
			"status":    status,
			"id":        productID,
			"tenant_id": tenantID,
		},
	)
	if err != nil {
		return fmt.Errorf("update product %s status to %s: %w", productID, status, err)
	}
	return nil
}

func (h *CrossmatchHandler) listCrossmatches(ctx context.Context, tenantID, patientFilter, statusFilter string) ([]Crossmatch, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, patient_id, patient_nhi, patient_abo, patient_rhd,
		        antibody_screen, product_unit_ids, status, compatibility,
		        requested_by, issued_by, transfused_by,
		        emergency_reason, notes,
		        requested_at, issued_at, transfused_at, cancelled_at,
		        created_at, updated_at
		 FROM crossmatches
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter = '' OR patient_id = @patient_filter)
		   AND (@status_filter  = '' OR status      = @status_filter)
		 ORDER BY requested_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":      tenantID,
			"patient_filter": patientFilter,
			"status_filter":  statusFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query crossmatches: %w", err)
	}
	defer rows.Close()

	var results []Crossmatch
	for rows.Next() {
		xm, err := scanCrossmatch(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, xm)
	}
	return results, rows.Err()
}

func (h *CrossmatchHandler) getCrossmatchByID(ctx context.Context, id, tenantID string) (Crossmatch, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, patient_id, patient_nhi, patient_abo, patient_rhd,
		        antibody_screen, product_unit_ids, status, compatibility,
		        requested_by, issued_by, transfused_by,
		        emergency_reason, notes,
		        requested_at, issued_at, transfused_at, cancelled_at,
		        created_at, updated_at
		 FROM crossmatches
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	xm, err := scanCrossmatch(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Crossmatch{}, errNotFound
		}
		return Crossmatch{}, fmt.Errorf("get crossmatch by id: %w", err)
	}
	return xm, nil
}

func (h *CrossmatchHandler) insertCrossmatch(ctx context.Context, req crossmatchCreateRequest, compatibility, tenantID string) (Crossmatch, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO crossmatches
		   (patient_id, patient_nhi, patient_abo, patient_rhd,
		    antibody_screen, product_unit_ids, status, compatibility,
		    requested_by, notes, tenant_id, requested_at)
		 VALUES
		   (@patient_id, @patient_nhi, @patient_abo, @patient_rhd,
		    @antibody_screen, @product_unit_ids, 'matched', @compatibility,
		    @requested_by, @notes, @tenant_id, now())
		 RETURNING id, tenant_id, patient_id, patient_nhi, patient_abo, patient_rhd,
		           antibody_screen, product_unit_ids, status, compatibility,
		           requested_by, issued_by, transfused_by,
		           emergency_reason, notes,
		           requested_at, issued_at, transfused_at, cancelled_at,
		           created_at, updated_at`,
		db.NamedArgs{
			"patient_id":       req.PatientID,
			"patient_nhi":      req.PatientNHI,
			"patient_abo":      req.PatientABO,
			"patient_rhd":      req.PatientRhD,
			"antibody_screen":  req.AntibodyScreen,
			"product_unit_ids": req.ProductUnitIDs,
			"compatibility":    compatibility,
			"requested_by":     req.RequestedBy,
			"notes":            req.Notes,
			"tenant_id":        tenantID,
		},
	)
	xm, err := scanCrossmatch(row)
	if err != nil {
		return Crossmatch{}, fmt.Errorf("insert crossmatch: %w", err)
	}
	return xm, nil
}

func (h *CrossmatchHandler) updateCrossmatch(ctx context.Context, xm Crossmatch) (Crossmatch, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE crossmatches
		 SET status           = @status,
		     compatibility    = @compatibility,
		     issued_by        = @issued_by,
		     transfused_by    = @transfused_by,
		     emergency_reason = @emergency_reason,
		     notes            = @notes,
		     issued_at        = @issued_at,
		     transfused_at    = @transfused_at,
		     cancelled_at     = @cancelled_at,
		     updated_at       = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, patient_id, patient_nhi, patient_abo, patient_rhd,
		           antibody_screen, product_unit_ids, status, compatibility,
		           requested_by, issued_by, transfused_by,
		           emergency_reason, notes,
		           requested_at, issued_at, transfused_at, cancelled_at,
		           created_at, updated_at`,
		db.NamedArgs{
			"status":           xm.Status,
			"compatibility":    xm.Compatibility,
			"issued_by":        xm.IssuedBy,
			"transfused_by":    xm.TransfusedBy,
			"emergency_reason": xm.EmergencyReason,
			"notes":            xm.Notes,
			"issued_at":        xm.IssuedAt,
			"transfused_at":    xm.TransfusedAt,
			"cancelled_at":     xm.CancelledAt,
			"id":               xm.ID,
			"tenant_id":        xm.TenantID,
		},
	)
	updated, err := scanCrossmatch(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Crossmatch{}, errNotFound
		}
		return Crossmatch{}, fmt.Errorf("update crossmatch: %w", err)
	}
	return updated, nil
}

// scanCrossmatch scans a single Crossmatch from a row (pgx.Row or pgx.Rows).
func scanCrossmatch(row dbRow) (Crossmatch, error) {
	var xm Crossmatch
	if err := row.Scan(
		&xm.ID, &xm.TenantID, &xm.PatientID, &xm.PatientNHI, &xm.PatientABO, &xm.PatientRhD,
		&xm.AntibodyScreen, &xm.ProductUnitIDs, &xm.Status, &xm.Compatibility,
		&xm.RequestedBy, &xm.IssuedBy, &xm.TransfusedBy,
		&xm.EmergencyReason, &xm.Notes,
		&xm.RequestedAt, &xm.IssuedAt, &xm.TransfusedAt, &xm.CancelledAt,
		&xm.CreatedAt, &xm.UpdatedAt,
	); err != nil {
		return Crossmatch{}, err
	}
	return xm, nil
}
