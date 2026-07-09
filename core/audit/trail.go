// Package audit provides a synchronous audit trail writer for NZ Health Information
// Privacy Code compliance.
package audit

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event represents a single auditable action performed within the system.
type Event struct {
	TenantID     uuid.UUID      `json:"tenant_id"`
	PrincipalID  string         `json:"principal_id"`
	Action       string         `json:"action"` // "read", "create", "update", "delete", "export"
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	PatientNHI   string         `json:"patient_nhi,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
	IPAddress    string         `json:"ip_address,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

// Trail is a synchronous audit trail writer backed by a PostgreSQL connection pool.
type Trail struct {
	pool *pgxpool.Pool
}

// New creates a new Trail using the provided connection pool.
func New(pool *pgxpool.Pool) *Trail {
	return &Trail{pool: pool}
}

// NewTrail is a compatibility alias for New, preserved for module compatibility.
func NewTrail(pool *pgxpool.Pool) *Trail { return New(pool) }

// Record synchronously inserts an audit event into the audit_events table.
// It returns an error if the insert fails.
func (t *Trail) Record(ctx context.Context, e Event) error {
	const q = `
		INSERT INTO audit_events (
			tenant_id,
			principal_id,
			action,
			resource_type,
			resource_id,
			patient_nhi,
			details,
			ip_address,
			user_agent,
			occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := t.pool.Exec(ctx, q,
		e.TenantID,
		e.PrincipalID,
		e.Action,
		e.ResourceType,
		e.ResourceID,
		e.PatientNHI,
		e.Details,
		e.IPAddress,
		e.UserAgent,
		e.OccurredAt,
	)
	return err
}

// Execer is the interface satisfied by pgx.Tx, allowing RecordTx to write
// audit events inside an existing transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// RecordTx inserts an audit event using the provided Execer (typically a pgx.Tx),
// allowing the audit write to participate in the caller's transaction.
func (t *Trail) RecordTx(ctx context.Context, e Event, ex Execer) error {
	const q = `
		INSERT INTO audit_events (
			tenant_id,
			principal_id,
			action,
			resource_type,
			resource_id,
			patient_nhi,
			details,
			ip_address,
			user_agent,
			occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := ex.Exec(ctx, q,
		e.TenantID,
		e.PrincipalID,
		e.Action,
		e.ResourceType,
		e.ResourceID,
		e.PatientNHI,
		e.Details,
		e.IPAddress,
		e.UserAgent,
		e.OccurredAt,
	)
	return err
}

// Query retrieves audit events for a tenant within the given time range, capped at limit rows.
// Results are ordered by occurred_at ascending.
func (t *Trail) Query(ctx context.Context, tenantID uuid.UUID, from, to time.Time, limit int) ([]Event, error) {
	const q = `
		SELECT
			tenant_id,
			principal_id,
			action,
			resource_type,
			resource_id,
			patient_nhi,
			details,
			ip_address,
			user_agent,
			occurred_at
		FROM audit_events
		WHERE tenant_id = $1
		  AND occurred_at >= $2
		  AND occurred_at <= $3
		ORDER BY occurred_at ASC
		LIMIT $4`

	rows, err := t.pool.Query(ctx, q, tenantID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.TenantID,
			&e.PrincipalID,
			&e.Action,
			&e.ResourceType,
			&e.ResourceID,
			&e.PatientNHI,
			&e.Details,
			&e.IPAddress,
			&e.UserAgent,
			&e.OccurredAt,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
