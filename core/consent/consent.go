// Package consent implements consent management per the NZ Health Information
// Privacy Code (HIPC) Rules 10 (access) and 11 (disclosure).
//
// SQL schema:
//
//	CREATE TABLE IF NOT EXISTS consents (
//	    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
//	    tenant_id     UUID        NOT NULL,
//	    patient_nhi   TEXT        NOT NULL,
//	    consent_type  TEXT        NOT NULL,
//	    granted       BOOLEAN     NOT NULL DEFAULT FALSE,
//	    purpose       TEXT        NOT NULL,
//	    granted_by    TEXT        NOT NULL,
//	    granted_at    TIMESTAMPTZ NOT NULL,
//	    expires_at    TIMESTAMPTZ,
//	    revoked_at    TIMESTAMPTZ,
//	    evidence      TEXT        NOT NULL DEFAULT '',
//	    CONSTRAINT consents_tenant_nhi_type_idx
//	        UNIQUE NULLS NOT DISTINCT (tenant_id, patient_nhi, consent_type, revoked_at)
//	);
//	CREATE INDEX IF NOT EXISTS consents_tenant_nhi_idx ON consents (tenant_id, patient_nhi);
package consent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConsentType identifies the category of consent being recorded.
type ConsentType string

const (
	// ConsentTypeAccess covers HIPC Rule 10 — individual access to their own health information.
	ConsentTypeAccess ConsentType = "access"

	// ConsentTypeDisclosure covers HIPC Rule 11 — disclosure of health information to third parties.
	ConsentTypeDisclosure ConsentType = "disclosure"

	// ConsentTypeResearch covers consent for use of information in research projects.
	ConsentTypeResearch ConsentType = "research"

	// ConsentTypeMarketing covers consent for use of information for marketing purposes.
	ConsentTypeMarketing ConsentType = "marketing"
)

// Consent represents a single consent record for a patient.
type Consent struct {
	// ID is the unique identifier for this consent record.
	ID uuid.UUID

	// TenantID is the healthcare organisation that captured the consent.
	TenantID uuid.UUID

	// PatientNHI is the patient's National Health Index number.
	PatientNHI string

	// ConsentType is the category of consent (access, disclosure, research, marketing).
	ConsentType ConsentType

	// Granted indicates whether consent was given (true) or withheld/withdrawn (false).
	Granted bool

	// Purpose describes the specific reason for which consent is being sought.
	Purpose string

	// GrantedBy is the identity (e.g. staff ID or system) that recorded the consent.
	GrantedBy string

	// GrantedAt is when the consent was recorded.
	GrantedAt time.Time

	// ExpiresAt, if non-nil, is the time after which the consent is no longer valid.
	ExpiresAt *time.Time

	// RevokedAt, if non-nil, is the time at which the patient withdrew consent.
	RevokedAt *time.Time

	// Evidence is a document reference (e.g. S3 key or DMS path) to the signed consent form.
	Evidence string
}

// Store persists and retrieves Consent records backed by a PostgreSQL connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store using the provided pgxpool.Pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Grant persists a new Consent record. If c.ID is the zero UUID a new one is generated.
func (s *Store) Grant(ctx context.Context, c Consent) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	const q = `
		INSERT INTO consents
		    (id, tenant_id, patient_nhi, consent_type, granted, purpose,
		     granted_by, granted_at, expires_at, revoked_at, evidence)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := s.pool.Exec(ctx, q,
		c.ID, c.TenantID, c.PatientNHI, string(c.ConsentType), c.Granted,
		c.Purpose, c.GrantedBy, c.GrantedAt, c.ExpiresAt, c.RevokedAt, c.Evidence,
	)
	if err != nil {
		return fmt.Errorf("consent.Store.Grant: %w", err)
	}
	return nil
}

// Revoke marks an existing consent record as revoked at the current time.
func (s *Store) Revoke(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE consents SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("consent.Store.Revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("consent.Store.Revoke: consent %s not found or already revoked", id)
	}
	return nil
}

// Check returns true if at least one active (granted, non-expired, non-revoked) consent of
// consentType exists for the given tenant and patient.
func (s *Store) Check(ctx context.Context, tenantID uuid.UUID, patientNHI string, consentType ConsentType) (bool, error) {
	const q = `
		SELECT EXISTS (
		    SELECT 1 FROM consents
		    WHERE tenant_id    = $1
		      AND patient_nhi  = $2
		      AND consent_type = $3
		      AND granted      = TRUE
		      AND revoked_at   IS NULL
		      AND (expires_at IS NULL OR expires_at > NOW())
		)`
	var exists bool
	if err := s.pool.QueryRow(ctx, q, tenantID, patientNHI, string(consentType)).Scan(&exists); err != nil {
		return false, fmt.Errorf("consent.Store.Check: %w", err)
	}
	return exists, nil
}

// List returns all consent records for the given tenant and patient, ordered by granted_at descending.
func (s *Store) List(ctx context.Context, tenantID uuid.UUID, patientNHI string) ([]Consent, error) {
	const q = `
		SELECT id, tenant_id, patient_nhi, consent_type, granted, purpose,
		       granted_by, granted_at, expires_at, revoked_at, evidence
		FROM   consents
		WHERE  tenant_id  = $1
		  AND  patient_nhi = $2
		ORDER  BY granted_at DESC`
	rows, err := s.pool.Query(ctx, q, tenantID, patientNHI)
	if err != nil {
		return nil, fmt.Errorf("consent.Store.List: %w", err)
	}
	defer rows.Close()

	var results []Consent
	for rows.Next() {
		var c Consent
		var ct string
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.PatientNHI, &ct, &c.Granted, &c.Purpose,
			&c.GrantedBy, &c.GrantedAt, &c.ExpiresAt, &c.RevokedAt, &c.Evidence,
		); err != nil {
			return nil, fmt.Errorf("consent.Store.List scan: %w", err)
		}
		c.ConsentType = ConsentType(ct)
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("consent.Store.List rows: %w", err)
	}
	return results, nil
}
