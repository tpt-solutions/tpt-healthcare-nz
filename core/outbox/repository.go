package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines persistence operations for outbox messages.
type Repository interface {
	// Enqueue writes a new message atomically. The caller must pass the pgx
	// transaction via context (using pgx.WithTx or similar) so the insert
	// participates in the business transaction.
	Enqueue(ctx context.Context, tenantID uuid.UUID, topic Topic, payload []byte) (*Message, error)

	// Claim atomically marks up to limit pending messages as "processing"
	// and returns them. Used by the River worker to safely claim work.
	Claim(ctx context.Context, topics []Topic, limit int) ([]*Message, error)

	// MarkDone marks a successfully processed message as done.
	MarkDone(ctx context.Context, id uuid.UUID) error

	// MarkFailed increments the attempt counter and schedules the next retry
	// using exponential backoff. After maxAttempts it marks the message dead.
	MarkFailed(ctx context.Context, id uuid.UUID, lastError string, maxAttempts int) error

	// Dead returns messages in the dead state for operator inspection.
	Dead(ctx context.Context, tenantID uuid.UUID, limit int) ([]*Message, error)

	// Retry moves a dead message back to pending so an operator can manually
	// re-queue it after the root cause is resolved.
	Retry(ctx context.Context, id uuid.UUID) error

	// PruneProcessed deletes done messages older than olderThan to prevent
	// unbounded growth. Called by a pg_cron or nightly River job.
	PruneProcessed(ctx context.Context, olderThan time.Duration) (int64, error)
}
