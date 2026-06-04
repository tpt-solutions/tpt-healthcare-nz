package health

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// CheckArgs is the River job payload for the periodic health poller.
type CheckArgs struct{}

func (CheckArgs) Kind() string { return "health.check_providers" }

// Worker polls all registered providers and upserts results into
// provider_health_status so the /health endpoint can read without blocking.
type Worker struct {
	river.WorkerDefaults[CheckArgs]
	registry *Registry
	pool     *pgxpool.Pool
	logger   *slog.Logger
}

// NewWorker constructs a health check Worker.
func NewWorker(registry *Registry, pool *pgxpool.Pool, logger *slog.Logger) *Worker {
	return &Worker{registry: registry, pool: pool, logger: logger}
}

// Work runs all health checks and persists results.
func (w *Worker) Work(ctx context.Context, _ *river.Job[CheckArgs]) error {
	report := w.registry.CheckAll(ctx)
	for _, s := range report.Providers {
		if err := w.upsert(ctx, s); err != nil {
			w.logger.Error("health: failed to persist status",
				"provider", s.ProviderName,
				"error", err,
			)
		}
	}
	w.logger.Info("health: provider check complete",
		"total", len(report.Providers),
		"ok", countOK(report.Providers),
	)
	return nil
}

func (w *Worker) upsert(ctx context.Context, s Status) error {
	const q = `
		INSERT INTO provider_health_status
			(provider_type, provider_name, ok, last_checked_at, latency_ms, organisation_name, error_text)
		VALUES
			(@provider_type, @provider_name, @ok, @last_checked_at, @latency_ms, @organisation_name, @error_text)
		ON CONFLICT (provider_type, provider_name) DO UPDATE SET
			ok               = EXCLUDED.ok,
			last_checked_at  = EXCLUDED.last_checked_at,
			latency_ms       = EXCLUDED.latency_ms,
			organisation_name = EXCLUDED.organisation_name,
			error_text       = EXCLUDED.error_text`

	_, err := w.pool.Exec(ctx, q, map[string]any{
		"provider_type":     s.ProviderType,
		"provider_name":     s.ProviderName,
		"ok":                s.OK,
		"last_checked_at":   s.LastCheckedAt,
		"latency_ms":        s.LatencyMs,
		"organisation_name": s.OrganisationName,
		"error_text":        s.ErrorText,
	})
	if err != nil {
		return fmt.Errorf("health upsert: %w", err)
	}
	return nil
}

// ReadReport reads the cached health status from the DB.
// Used by the /health HTTP endpoint.
func ReadReport(ctx context.Context, pool *pgxpool.Pool) (*Report, error) {
	const q = `
		SELECT provider_type, provider_name, ok, last_checked_at, latency_ms,
		       COALESCE(organisation_name, ''), COALESCE(error_text, '')
		FROM provider_health_status
		ORDER BY provider_type, provider_name`

	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("health read: %w", err)
	}
	defer rows.Close()

	var statuses []Status
	for rows.Next() {
		var s Status
		if err := rows.Scan(
			&s.ProviderType, &s.ProviderName, &s.OK,
			&s.LastCheckedAt, &s.LatencyMs,
			&s.OrganisationName, &s.ErrorText,
		); err != nil {
			return nil, fmt.Errorf("health scan: %w", err)
		}
		statuses = append(statuses, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("health rows: %w", err)
	}

	allOK := true
	for _, s := range statuses {
		if !s.OK {
			allOK = false
			break
		}
	}
	return &Report{Providers: statuses, AllOK: allOK}, nil
}

func countOK(ss []Status) int {
	n := 0
	for _, s := range ss {
		if s.OK {
			n++
		}
	}
	return n
}
