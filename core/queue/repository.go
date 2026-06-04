package queue

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository is the persistence interface for the queue domain.
type Repository interface {
	// CreateQueue opens a new queue for a clinic and day. Returns the created Queue.
	CreateQueue(ctx context.Context, tenantID uuid.UUID, name string, date time.Time) (*Queue, error)

	// GetQueue retrieves a queue by ID.
	GetQueue(ctx context.Context, id uuid.UUID) (*Queue, error)

	// GetOrCreateTodayQueue returns the open queue for today, creating it if needed.
	GetOrCreateTodayQueue(ctx context.Context, tenantID uuid.UUID, name string) (*Queue, error)

	// ListEntries returns all entries for a queue ordered by position.
	ListEntries(ctx context.Context, queueID uuid.UUID) ([]EntryWithLocation, error)

	// CheckIn adds a patient to the queue. Returns the new entry with its assigned position.
	CheckIn(ctx context.Context, entry QueueEntry) (*QueueEntry, error)

	// GetEntry fetches a single entry by ID.
	GetEntry(ctx context.Context, id uuid.UUID) (*QueueEntry, error)

	// NextPosition returns the next available position number for a queue.
	NextPosition(ctx context.Context, queueID uuid.UUID) (int, error)

	// CallNext marks the next waiting entry as "called" and returns it.
	CallNext(ctx context.Context, queueID uuid.UUID, roomHint string) (*QueueEntry, error)

	// UpdateEntryStatus transitions an entry to a new status.
	// For terminal statuses (done/skipped/left) the location row is deleted in the same transaction.
	UpdateEntryStatus(ctx context.Context, entryID uuid.UUID, status EntryStatus) (*QueueEntry, error)

	// UpsertLocation saves (or replaces) a patient's current GPS fix.
	// Returns ErrEntryNotActive if the entry is in a terminal status.
	UpsertLocation(ctx context.Context, loc Location) error

	// AverageWaitMinutes estimates current average wait in the queue based on recent completions.
	AverageWaitMinutes(ctx context.Context, queueID uuid.UUID) (int, error)
}

// ErrEntryNotFound is returned when a requested queue entry does not exist.
var ErrEntryNotFound = &queueError{"entry not found"}

// ErrEntryNotActive is returned when a location update is attempted on a terminal entry.
var ErrEntryNotActive = &queueError{"entry is no longer active"}

// ErrQueueNotFound is returned when a requested queue does not exist.
var ErrQueueNotFound = &queueError{"queue not found"}

// ErrNoWaitingEntries is returned by CallNext when the queue is empty.
var ErrNoWaitingEntries = &queueError{"no waiting entries"}

type queueError struct{ msg string }

func (e *queueError) Error() string { return "queue: " + e.msg }
