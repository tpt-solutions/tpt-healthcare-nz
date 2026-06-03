package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store backed by the provided pgx connection pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{pool: pool}
}

func (s *pgStore) Submit(ctx context.Context, req SubmitRequest) (*Application, error) {
	addr, err := marshalAddress(req.Address)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO tenant_applications
			(practice_name, hpi_facility_id, contact_name, contact_email, contact_hpi_cpn, address)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, practice_name, hpi_facility_id, contact_name, contact_email,
		          contact_hpi_cpn, address, status, reviewer_notes, tenant_id,
		          submitted_at, reviewed_at, reviewed_by`,
		req.PracticeName, req.HPIFacilityID, req.ContactName,
		req.ContactEmail, req.ContactHPICPN, addr,
	)
	return scanApplication(row)
}

func (s *pgStore) GetApplication(ctx context.Context, id uuid.UUID) (*Application, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, practice_name, hpi_facility_id, contact_name, contact_email,
		       contact_hpi_cpn, address, status, reviewer_notes, tenant_id,
		       submitted_at, reviewed_at, reviewed_by
		FROM tenant_applications WHERE id = $1`, id)
	return scanApplication(row)
}

func (s *pgStore) ListApplications(ctx context.Context, status string) ([]*Application, error) {
	var rows pgx.Rows
	var err error
	if status == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, practice_name, hpi_facility_id, contact_name, contact_email,
			       contact_hpi_cpn, address, status, reviewer_notes, tenant_id,
			       submitted_at, reviewed_at, reviewed_by
			FROM tenant_applications ORDER BY submitted_at DESC`)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, practice_name, hpi_facility_id, contact_name, contact_email,
			       contact_hpi_cpn, address, status, reviewer_notes, tenant_id,
			       submitted_at, reviewed_at, reviewed_by
			FROM tenant_applications WHERE status = $1 ORDER BY submitted_at DESC`, status)
	}
	if err != nil {
		return nil, fmt.Errorf("tenant: list applications: %w", err)
	}
	defer rows.Close()
	return collectApplications(rows)
}

func (s *pgStore) Approve(ctx context.Context, applicationID uuid.UUID, reviewerID, notes string) (*Tenant, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("tenant: approve: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Load the application inside the transaction.
	row := tx.QueryRow(ctx, `
		SELECT id, practice_name, hpi_facility_id, contact_name, contact_email, address
		FROM tenant_applications WHERE id = $1 AND status = 'pending'
		FOR UPDATE`, applicationID)

	var (
		appID         uuid.UUID
		practiceName  string
		hpiFacilityID string
		contactName   string
		contactEmail  string
		addrRaw       []byte
	)
	if err := row.Scan(&appID, &practiceName, &hpiFacilityID, &contactName, &contactEmail, &addrRaw); err != nil {
		return nil, fmt.Errorf("tenant: approve: load application: %w", err)
	}

	// Create the tenant record.
	now := time.Now().UTC()
	tenantRow := tx.QueryRow(ctx, `
		INSERT INTO tenants (name, hpi_facility_id, contact_email, contact_name, address)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, hpi_facility_id, status, contact_email, contact_name,
		          address, created_at, updated_at`,
		practiceName, hpiFacilityID, contactEmail, contactName, addrRaw,
	)
	t, err := scanTenant(tenantRow)
	if err != nil {
		return nil, fmt.Errorf("tenant: approve: create tenant: %w", err)
	}

	// Mark the application approved.
	if _, err := tx.Exec(ctx, `
		UPDATE tenant_applications
		SET status = 'approved', reviewer_notes = $1, tenant_id = $2,
		    reviewed_at = $3, reviewed_by = $4
		WHERE id = $5`,
		notes, t.ID, now, reviewerID, applicationID,
	); err != nil {
		return nil, fmt.Errorf("tenant: approve: update application: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("tenant: approve: commit: %w", err)
	}
	return t, nil
}

func (s *pgStore) Reject(ctx context.Context, applicationID uuid.UUID, reviewerID, notes string) error {
	now := time.Now().UTC()
	tag, err := s.pool.Exec(ctx, `
		UPDATE tenant_applications
		SET status = 'rejected', reviewer_notes = $1, reviewed_at = $2, reviewed_by = $3
		WHERE id = $4 AND status = 'pending'`,
		notes, now, reviewerID, applicationID,
	)
	if err != nil {
		return fmt.Errorf("tenant: reject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("tenant: reject: application %s not found or not pending", applicationID)
	}
	return nil
}

func (s *pgStore) ListTenants(ctx context.Context) ([]*Tenant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, hpi_facility_id, status, contact_email, contact_name,
		       address, created_at, updated_at
		FROM tenants ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("tenant: list tenants: %w", err)
	}
	defer rows.Close()
	return collectTenants(rows)
}

func (s *pgStore) GetTenant(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, hpi_facility_id, status, contact_email, contact_name,
		       address, created_at, updated_at
		FROM tenants WHERE id = $1`, id)
	return scanTenant(row)
}

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

func scanApplication(row pgx.Row) (*Application, error) {
	var (
		a       Application
		addrRaw []byte
		tenID   *uuid.UUID
		revAt   *time.Time
		revBy   string
	)
	err := row.Scan(
		&a.ID, &a.PracticeName, &a.HPIFacilityID, &a.ContactName, &a.ContactEmail,
		&a.ContactHPICPN, &addrRaw, &a.Status, &a.ReviewerNotes,
		&tenID, &a.SubmittedAt, &revAt, &revBy,
	)
	if err != nil {
		return nil, fmt.Errorf("tenant: scan application: %w", err)
	}
	a.TenantID = tenID
	a.ReviewedAt = revAt
	a.ReviewedBy = revBy
	if err := json.Unmarshal(addrRaw, &a.Address); err != nil {
		return nil, fmt.Errorf("tenant: unmarshal address: %w", err)
	}
	return &a, nil
}

func collectApplications(rows pgx.Rows) ([]*Application, error) {
	var out []*Application
	for rows.Next() {
		var (
			a       Application
			addrRaw []byte
			tenID   *uuid.UUID
			revAt   *time.Time
			revBy   string
		)
		if err := rows.Scan(
			&a.ID, &a.PracticeName, &a.HPIFacilityID, &a.ContactName, &a.ContactEmail,
			&a.ContactHPICPN, &addrRaw, &a.Status, &a.ReviewerNotes,
			&tenID, &a.SubmittedAt, &revAt, &revBy,
		); err != nil {
			return nil, fmt.Errorf("tenant: scan application row: %w", err)
		}
		a.TenantID = tenID
		a.ReviewedAt = revAt
		a.ReviewedBy = revBy
		if err := json.Unmarshal(addrRaw, &a.Address); err != nil {
			return nil, fmt.Errorf("tenant: unmarshal address row: %w", err)
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

func scanTenant(row pgx.Row) (*Tenant, error) {
	var (
		t       Tenant
		addrRaw []byte
	)
	err := row.Scan(
		&t.ID, &t.Name, &t.HPIFacilityID, &t.Status,
		&t.ContactEmail, &t.ContactName, &addrRaw,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("tenant: scan tenant: %w", err)
	}
	if err := json.Unmarshal(addrRaw, &t.Address); err != nil {
		return nil, fmt.Errorf("tenant: unmarshal tenant address: %w", err)
	}
	return &t, nil
}

func collectTenants(rows pgx.Rows) ([]*Tenant, error) {
	var out []*Tenant
	for rows.Next() {
		var (
			t       Tenant
			addrRaw []byte
		)
		if err := rows.Scan(
			&t.ID, &t.Name, &t.HPIFacilityID, &t.Status,
			&t.ContactEmail, &t.ContactName, &addrRaw,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("tenant: scan tenant row: %w", err)
		}
		if err := json.Unmarshal(addrRaw, &t.Address); err != nil {
			return nil, fmt.Errorf("tenant: unmarshal tenant address row: %w", err)
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

func marshalAddress(addr map[string]any) ([]byte, error) {
	if addr == nil {
		return []byte("{}"), nil
	}
	b, err := json.Marshal(addr)
	if err != nil {
		return nil, fmt.Errorf("tenant: marshal address: %w", err)
	}
	return b, nil
}
