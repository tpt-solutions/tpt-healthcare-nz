// Package recall manages patient recall and care gap management for primary
// care practices. A recall item is a scheduled future contact with a patient
// for a specific clinical purpose — e.g. annual diabetes review, cervical
// screening recall, BP check. The recall worker runs as a River job and
// dispatches notifications via the configured SMS/email/push channels
// (respecting patient comms preferences).
package recall

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// RecallStatus describes the lifecycle state of a recall item.
type RecallStatus string

const (
	RecallPending   RecallStatus = "pending"
	RecallSent      RecallStatus = "sent"
	RecallBooked    RecallStatus = "booked"
	RecallCompleted RecallStatus = "completed"
	RecallDeclined  RecallStatus = "declined"
)

// RecallItem is a single scheduled recall for a patient.
type RecallItem struct {
	ID        uuid.UUID    `json:"id"`
	TenantID  uuid.UUID    `json:"tenantId"`
	PatientID string       `json:"patientId"`
	// DueDate is when the recall should be actioned.
	DueDate   time.Time    `json:"dueDate"`
	// RecallType is a short slug describing the clinical purpose,
	// e.g. "diabetes-annual-review", "cervical-screening", "bp-check".
	RecallType string       `json:"recallType"`
	// Description is the human-readable recall reason shown to staff and
	// included in the patient notification.
	Description string      `json:"description"`
	// EncounterID is the encounter that triggered this recall, if applicable.
	EncounterID string      `json:"encounterId,omitempty"`
	// CreatedByID is the HPI CPN of the practitioner who created the recall.
	CreatedByID string      `json:"createdById,omitempty"`
	Status      RecallStatus `json:"status"`
	// NotificationsSent tracks how many reminders have been dispatched.
	NotificationsSent int  `json:"notificationsSent"`
	// LastNotifiedAt is when the most recent notification was sent.
	LastNotifiedAt *time.Time `json:"lastNotifiedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// Store manages recall items in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create persists a new recall item.
func (s *Store) Create(ctx context.Context, item RecallItem) (*RecallItem, error) {
	item.ID = uuid.New()
	item.Status = RecallPending
	item.CreatedAt = time.Now().UTC()
	item.UpdatedAt = item.CreatedAt

	_, err := s.pool.Exec(ctx,
		`INSERT INTO recall_items
			(id, tenant_id, patient_id, due_date, recall_type, description,
			 encounter_id, created_by_id, status, notifications_sent, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,0,$10,$11)`,
		item.ID, item.TenantID, item.PatientID, item.DueDate, item.RecallType,
		item.Description, nilIfEmpty(item.EncounterID), nilIfEmpty(item.CreatedByID),
		item.Status, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("recall: creating item: %w", err)
	}
	return &item, nil
}

// ListDue returns all pending recall items with a due date on or before cutoff.
func (s *Store) ListDue(ctx context.Context, cutoff time.Time) ([]RecallItem, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, patient_id, due_date, recall_type, description,
			COALESCE(encounter_id,''), COALESCE(created_by_id,''),
			status, notifications_sent, last_notified_at, created_at, updated_at
		 FROM recall_items
		 WHERE status='pending' AND due_date <= $1
		 ORDER BY due_date ASC`,
		cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("recall: listing due items: %w", err)
	}
	defer rows.Close()
	return scanItems(rows)
}

// ListForPatient returns all recall items for a patient, most recent first.
func (s *Store) ListForPatient(ctx context.Context, tenantID uuid.UUID, patientID string) ([]RecallItem, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, patient_id, due_date, recall_type, description,
			COALESCE(encounter_id,''), COALESCE(created_by_id,''),
			status, notifications_sent, last_notified_at, created_at, updated_at
		 FROM recall_items
		 WHERE tenant_id=$1 AND patient_id=$2
		 ORDER BY due_date DESC`,
		tenantID, patientID,
	)
	if err != nil {
		return nil, fmt.Errorf("recall: listing for patient %s: %w", patientID, err)
	}
	defer rows.Close()
	return scanItems(rows)
}

// MarkSent records that a notification was dispatched for the item.
func (s *Store) MarkSent(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE recall_items
		 SET status='sent', notifications_sent = notifications_sent+1,
		     last_notified_at=NOW(), updated_at=NOW()
		 WHERE id=$1`,
		id,
	)
	return err
}

// UpdateStatus sets a new status on the recall item.
func (s *Store) UpdateStatus(ctx context.Context, id uuid.UUID, status RecallStatus) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE recall_items SET status=$1, updated_at=NOW() WHERE id=$2`,
		status, id,
	)
	return err
}

// ---------------------------------------------------------------------------
// River worker
// ---------------------------------------------------------------------------

// WorkerArgs is the River job payload for the recall worker.
type WorkerArgs struct{}

func (WorkerArgs) Kind() string { return "recall.dispatch" }

// Worker is a River job worker that finds overdue recall items and dispatches
// notifications via the NotifyFunc.
type Worker struct {
	river.WorkerDefaults[WorkerArgs]
	store    *Store
	dispatch NotifyFunc
	logger   *slog.Logger
}

// NotifyFunc sends a recall notification for the given item.
// Implementations should check patient comms preferences before dispatching.
type NotifyFunc func(ctx context.Context, item RecallItem) error

// NewWorker creates a recall Worker.
func NewWorker(store *Store, dispatch NotifyFunc, logger *slog.Logger) *Worker {
	return &Worker{store: store, dispatch: dispatch, logger: logger}
}

// Work is called by River to process a single job invocation.
// It fetches all due recalls and dispatches notifications for each.
func (w *Worker) Work(ctx context.Context, job *river.Job[WorkerArgs]) error {
	items, err := w.store.ListDue(ctx, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("recall worker: listing due items: %w", err)
	}

	w.logger.InfoContext(ctx, "recall worker: processing due items", slog.Int("count", len(items)))

	for _, item := range items {
		if err := w.dispatch(ctx, item); err != nil {
			w.logger.ErrorContext(ctx, "recall worker: dispatch failed",
				slog.String("id", item.ID.String()),
				slog.Any("err", err),
			)
			continue
		}
		if err := w.store.MarkSent(ctx, item.ID); err != nil {
			w.logger.ErrorContext(ctx, "recall worker: marking sent",
				slog.String("id", item.ID.String()),
				slog.Any("err", err),
			)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func scanItems(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]RecallItem, error) {
	var items []RecallItem
	for rows.Next() {
		var it RecallItem
		if err := rows.Scan(
			&it.ID, &it.TenantID, &it.PatientID, &it.DueDate, &it.RecallType,
			&it.Description, &it.EncounterID, &it.CreatedByID,
			&it.Status, &it.NotificationsSent, &it.LastNotifiedAt,
			&it.CreatedAt, &it.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("recall: scanning item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
