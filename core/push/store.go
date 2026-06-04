package push

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Subscription is a browser Web Push subscription for a patient.
type Subscription struct {
	ID        uuid.UUID
	PatientID uuid.UUID
	TenantID  uuid.UUID
	Endpoint  string
	P256dh    string
	Auth      string
}

// Store persists and retrieves VAPID push subscriptions.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a Store backed by the provided connection pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Upsert saves a push subscription for a patient, replacing any existing row
// for the same (patient_id, endpoint) pair.
func (s *Store) Upsert(ctx context.Context, sub Subscription) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (patient_id, tenant_id, endpoint, p256dh, auth)
		VALUES (@patient_id, @tenant_id, @endpoint, @p256dh, @auth)
		ON CONFLICT (patient_id, endpoint)
		DO UPDATE SET p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth
	`,
		pgNamedArgs(map[string]any{
			"patient_id": sub.PatientID,
			"tenant_id":  sub.TenantID,
			"endpoint":   sub.Endpoint,
			"p256dh":     sub.P256dh,
			"auth":       sub.Auth,
		}),
	)
	if err != nil {
		return fmt.Errorf("push store upsert: %w", err)
	}
	return nil
}

// ListByPatient returns all active push subscriptions for a patient.
func (s *Store) ListByPatient(ctx context.Context, patientID uuid.UUID) ([]Subscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, patient_id, tenant_id, endpoint, p256dh, auth
		FROM   push_subscriptions
		WHERE  patient_id = @patient_id
	`, pgNamedArgs(map[string]any{"patient_id": patientID}))
	if err != nil {
		return nil, fmt.Errorf("push store list: %w", err)
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var sub Subscription
		if err := rows.Scan(&sub.ID, &sub.PatientID, &sub.TenantID, &sub.Endpoint, &sub.P256dh, &sub.Auth); err != nil {
			return nil, fmt.Errorf("push store scan: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// Delete removes a specific subscription by patient + endpoint.
func (s *Store) Delete(ctx context.Context, patientID uuid.UUID, endpoint string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM push_subscriptions
		WHERE  patient_id = @patient_id AND endpoint = @endpoint
	`, pgNamedArgs(map[string]any{"patient_id": patientID, "endpoint": endpoint}))
	if err != nil {
		return fmt.Errorf("push store delete: %w", err)
	}
	return nil
}
