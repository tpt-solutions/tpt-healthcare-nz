// Package commsprefs manages per-patient communication channel preferences.
// Before dispatching any outbound notification (push, SMS, email), callers
// should check whether the patient has opted in to that channel for that
// notification purpose.
//
// Privacy Act 2020 and HIPC Rule 10/11: marketing-category notifications
// require explicit opt-in. Clinical notifications (appointment reminders,
// test results) may be sent on the basis of the implied consent from the
// treatment relationship, but patients retain the right to opt out of any
// individual channel.
package commsprefs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Channel identifies a communication channel.
type Channel string

const (
	ChannelPush  Channel = "push"
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
)

// Purpose identifies why a notification is being sent.
type Purpose string

const (
	PurposeAppointmentReminder Purpose = "appointment_reminder"
	PurposeQueueCalled         Purpose = "queue_called"
	PurposeTestResult          Purpose = "test_result"
	PurposeRecall              Purpose = "recall"
	PurposeSecureMessage       Purpose = "secure_message"
	PurposeBilling             Purpose = "billing"
	PurposeMarketing           Purpose = "marketing"
)

// Preference holds a single opt-in/opt-out decision for a patient.
type Preference struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenantId"`
	PatientID string    `json:"patientId"`
	Channel   Channel   `json:"channel"`
	Purpose   Purpose   `json:"purpose"`
	OptedIn   bool      `json:"optedIn"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store manages patient communication preferences in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// IsAllowed returns true when the patient has not explicitly opted out of the
// given channel+purpose combination. When no preference record exists, the
// default is true for clinical purposes and false for marketing.
func (s *Store) IsAllowed(ctx context.Context, tenantID uuid.UUID, patientID string, ch Channel, purpose Purpose) (bool, error) {
	var optedIn bool
	err := s.pool.QueryRow(ctx,
		`SELECT opted_in FROM patient_comms_prefs
		 WHERE tenant_id=$1 AND patient_id=$2 AND channel=$3 AND purpose=$4`,
		tenantID, patientID, ch, purpose,
	).Scan(&optedIn)

	if err != nil {
		// No record found → apply default by purpose category.
		if isMarketingPurpose(purpose) {
			return false, nil // marketing requires explicit opt-in
		}
		return true, nil // clinical: default allow
	}
	return optedIn, nil
}

// Set creates or updates a preference record for a patient.
func (s *Store) Set(ctx context.Context, tenantID uuid.UUID, patientID string, ch Channel, purpose Purpose, optedIn bool) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO patient_comms_prefs
			(id, tenant_id, patient_id, channel, purpose, opted_in, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,NOW())
		 ON CONFLICT (tenant_id, patient_id, channel, purpose)
		 DO UPDATE SET opted_in=$6, updated_at=NOW()`,
		uuid.New(), tenantID, patientID, ch, purpose, optedIn,
	)
	if err != nil {
		return fmt.Errorf("comms-prefs: setting preference: %w", err)
	}
	return nil
}

// ListForPatient returns all preference records for a patient.
func (s *Store) ListForPatient(ctx context.Context, tenantID uuid.UUID, patientID string) ([]Preference, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, patient_id, channel, purpose, opted_in, updated_at
		 FROM patient_comms_prefs
		 WHERE tenant_id=$1 AND patient_id=$2
		 ORDER BY channel, purpose`,
		tenantID, patientID,
	)
	if err != nil {
		return nil, fmt.Errorf("comms-prefs: listing for patient %s: %w", patientID, err)
	}
	defer rows.Close()

	var prefs []Preference
	for rows.Next() {
		var p Preference
		if err := rows.Scan(&p.ID, &p.TenantID, &p.PatientID, &p.Channel, &p.Purpose, &p.OptedIn, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("comms-prefs: scanning row: %w", err)
		}
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}

// OptOutAll marks a patient as opted out of all channels for a specific purpose.
func (s *Store) OptOutAll(ctx context.Context, tenantID uuid.UUID, patientID string, purpose Purpose) error {
	for _, ch := range []Channel{ChannelPush, ChannelSMS, ChannelEmail} {
		if err := s.Set(ctx, tenantID, patientID, ch, purpose, false); err != nil {
			return err
		}
	}
	return nil
}

func isMarketingPurpose(p Purpose) bool {
	return p == PurposeMarketing
}
