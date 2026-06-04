package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	// MaxAttempts is the maximum number of delivery attempts before a message
	// is moved to the dead-letter partition.
	MaxAttempts = 10

	// ClaimBatch is the number of messages claimed per River job execution.
	ClaimBatch = 50

	// PruneAfter is how long done messages are retained before being pruned.
	PruneAfter = 7 * 24 * time.Hour
)

// WorkerArgs is the River job payload for the outbox processor.
// Topics controls which message types this worker instance handles,
// allowing different worker pools to process different integration types.
type WorkerArgs struct {
	Topics []Topic `json:"topics"`
}

func (WorkerArgs) Kind() string { return "outbox.process" }

// Handler is a function that processes a single outbox message.
// Implementations should be idempotent — River guarantees at-least-once
// delivery, so a message may be processed more than once in failure scenarios.
type Handler func(ctx context.Context, msg *Message) error

// Dispatcher routes messages to the correct Handler by topic.
type Dispatcher struct {
	handlers map[Topic]Handler
}

// NewDispatcher creates an empty Dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[Topic]Handler)}
}

// Register binds a handler to a topic. Panics on duplicate registration.
func (d *Dispatcher) Register(topic Topic, h Handler) {
	if _, dup := d.handlers[topic]; dup {
		panic(fmt.Sprintf("outbox: handler for topic %q already registered", topic))
	}
	d.handlers[topic] = h
}

// Dispatch calls the handler registered for msg.Topic.
func (d *Dispatcher) Dispatch(ctx context.Context, msg *Message) error {
	h, ok := d.handlers[msg.Topic]
	if !ok {
		return fmt.Errorf("outbox: no handler for topic %q", msg.Topic)
	}
	return h(ctx, msg)
}

// Worker is a River worker that claims and processes outbox messages.
type Worker struct {
	river.WorkerDefaults[WorkerArgs]
	repo       Repository
	dispatcher *Dispatcher
	logger     *slog.Logger
}

// NewWorker constructs a Worker.
func NewWorker(repo Repository, dispatcher *Dispatcher, logger *slog.Logger) *Worker {
	return &Worker{repo: repo, dispatcher: dispatcher, logger: logger}
}

// Work claims a batch of pending messages and dispatches each one.
func (w *Worker) Work(ctx context.Context, job *river.Job[WorkerArgs]) error {
	msgs, err := w.repo.Claim(ctx, job.Args.Topics, ClaimBatch)
	if err != nil {
		return fmt.Errorf("outbox worker claim: %w", err)
	}

	for _, msg := range msgs {
		if err := w.process(ctx, msg); err != nil {
			w.logger.Error("outbox message failed",
				"id", msg.ID,
				"topic", msg.Topic,
				"attempt", msg.Attempts+1,
				"error", err,
			)
		}
	}
	return nil
}

func (w *Worker) process(ctx context.Context, msg *Message) error {
	err := w.dispatcher.Dispatch(ctx, msg)
	if err == nil {
		return w.repo.MarkDone(ctx, msg.ID)
	}
	return w.repo.MarkFailed(ctx, msg.ID, err.Error(), MaxAttempts)
}

// PruneArgs is the River job payload for the outbox pruner.
type PruneArgs struct{}

func (PruneArgs) Kind() string { return "outbox.prune" }

// PruneWorker deletes processed messages older than PruneAfter.
type PruneWorker struct {
	river.WorkerDefaults[PruneArgs]
	repo   Repository
	logger *slog.Logger
}

// NewPruneWorker constructs a PruneWorker.
func NewPruneWorker(repo Repository, logger *slog.Logger) *PruneWorker {
	return &PruneWorker{repo: repo, logger: logger}
}

// Work deletes old done messages.
func (w *PruneWorker) Work(ctx context.Context, _ *river.Job[PruneArgs]) error {
	n, err := w.repo.PruneProcessed(ctx, PruneAfter)
	if err != nil {
		return fmt.Errorf("outbox prune: %w", err)
	}
	w.logger.Info("outbox pruned", "deleted", n)
	return nil
}

// Enqueue is a convenience helper to JSON-encode a payload and write it to the
// outbox. It must be called within the same pgx transaction as the business
// write to guarantee atomicity.
func Enqueue[T any](ctx context.Context, repo Repository, tenantID uuid.UUID, topic Topic, payload T) (*Message, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("outbox enqueue marshal: %w", err)
	}
	return repo.Enqueue(ctx, tenantID, topic, b)
}

// _ suppresses the unused import warning when time is used only in constants.
var _ = time.Hour
