package outbox

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements Repository against a PostgreSQL pool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Enqueue(ctx context.Context, tenantID uuid.UUID, topic Topic, payload []byte) (*Message, error) {
	const q = `
		INSERT INTO outbox_messages (id, tenant_id, topic, payload, status, attempts, next_attempt_at)
		VALUES (@id, @tenant_id, @topic, @payload, 'pending', 0, NOW())
		RETURNING id, tenant_id, topic, payload, status, attempts, next_attempt_at, dead_at, created_at, processed_at, last_error`

	row := r.pool.QueryRow(ctx, q, named(map[string]any{
		"id":        uuid.New(),
		"tenant_id": tenantID,
		"topic":     topic,
		"payload":   payload,
	}))
	return scanMessage(row)
}

func (r *PostgresRepository) Claim(ctx context.Context, topics []Topic, limit int) ([]*Message, error) {
	const q = `
		UPDATE outbox_messages
		SET status = 'processing'
		WHERE id IN (
			SELECT id FROM outbox_messages
			WHERE status = 'pending'
			  AND topic = ANY(@topics)
			  AND next_attempt_at <= NOW()
			ORDER BY next_attempt_at
			LIMIT @limit
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, tenant_id, topic, payload, status, attempts, next_attempt_at, dead_at, created_at, processed_at, last_error`

	topicStrs := make([]string, len(topics))
	for i, t := range topics {
		topicStrs[i] = string(t)
	}
	rows, err := r.pool.Query(ctx, q, named(map[string]any{
		"topics": topicStrs,
		"limit":  limit,
	}))
	if err != nil {
		return nil, fmt.Errorf("outbox claim: %w", err)
	}
	defer rows.Close()

	var msgs []*Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *PostgresRepository) MarkDone(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE outbox_messages
		SET status = 'done', processed_at = NOW()
		WHERE id = @id`
	_, err := r.pool.Exec(ctx, q, named(map[string]any{"id": id}))
	return err
}

func (r *PostgresRepository) MarkFailed(ctx context.Context, id uuid.UUID, lastError string, maxAttempts int) error {
	// Exponential backoff: 2^attempts minutes, capped at 24h.
	const q = `
		UPDATE outbox_messages
		SET attempts    = attempts + 1,
		    last_error  = @last_error,
		    status      = CASE WHEN attempts + 1 >= @max_attempts THEN 'dead' ELSE 'pending' END,
		    dead_at     = CASE WHEN attempts + 1 >= @max_attempts THEN NOW() ELSE NULL END,
		    next_attempt_at = CASE
		        WHEN attempts + 1 >= @max_attempts THEN next_attempt_at
		        ELSE NOW() + (LEAST(POWER(2, attempts + 1), 1440) * INTERVAL '1 minute')
		    END
		WHERE id = @id`
	_, err := r.pool.Exec(ctx, q, named(map[string]any{
		"id":           id,
		"last_error":   lastError,
		"max_attempts": maxAttempts,
	}))
	return err
}

func (r *PostgresRepository) Dead(ctx context.Context, tenantID uuid.UUID, limit int) ([]*Message, error) {
	const q = `
		SELECT id, tenant_id, topic, payload, status, attempts, next_attempt_at, dead_at, created_at, processed_at, last_error
		FROM outbox_messages
		WHERE tenant_id = @tenant_id AND status = 'dead'
		ORDER BY dead_at DESC
		LIMIT @limit`
	rows, err := r.pool.Query(ctx, q, named(map[string]any{
		"tenant_id": tenantID,
		"limit":     limit,
	}))
	if err != nil {
		return nil, fmt.Errorf("outbox dead: %w", err)
	}
	defer rows.Close()
	var msgs []*Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *PostgresRepository) Retry(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE outbox_messages
		SET status = 'pending', dead_at = NULL, next_attempt_at = NOW()
		WHERE id = @id AND status = 'dead'`
	_, err := r.pool.Exec(ctx, q, named(map[string]any{"id": id}))
	return err
}

func (r *PostgresRepository) PruneProcessed(ctx context.Context, olderThan time.Duration) (int64, error) {
	const q = `
		DELETE FROM outbox_messages
		WHERE status = 'done' AND processed_at < NOW() - @older_than::interval`
	tag, err := r.pool.Exec(ctx, q, named(map[string]any{
		"older_than": olderThan.String(),
	}))
	if err != nil {
		return 0, fmt.Errorf("outbox prune: %w", err)
	}
	return tag.RowsAffected(), nil
}

// backoffMinutes returns 2^attempt minutes, capped at 1440 (24h).
func backoffMinutes(attempt int) time.Duration {
	mins := math.Pow(2, float64(attempt))
	if mins > 1440 {
		mins = 1440
	}
	return time.Duration(mins) * time.Minute
}

// scanner is satisfied by both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanMessage(s scanner) (*Message, error) {
	var m Message
	err := s.Scan(
		&m.ID, &m.TenantID, &m.Topic, &m.Payload,
		&m.Status, &m.Attempts, &m.NextAttemptAt,
		&m.DeadAt, &m.CreatedAt, &m.ProcessedAt, &m.LastError,
	)
	if err != nil {
		return nil, fmt.Errorf("outbox scan: %w", err)
	}
	return &m, nil
}

// named converts a plain map to pgx named args.
func named(m map[string]any) pgxNamedArgs {
	return pgxNamedArgs(m)
}

// pgxNamedArgs is a type alias accepted by pgx QueryRow/Exec as named parameters.
type pgxNamedArgs map[string]any
