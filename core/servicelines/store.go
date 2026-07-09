package servicelines

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store backed by the provided pgx connection pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{pool: pool}
}

func (s *pgStore) GetProfile(ctx context.Context, tenantID uuid.UUID) (*Profile, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT service_line_id, enabled_at
		FROM tenant_service_lines
		WHERE tenant_id = $1
		ORDER BY service_line_id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("servicelines: get profile: %w", err)
	}
	defer rows.Close()

	p := &Profile{TenantID: tenantID, ServiceLines: []string{}}
	for rows.Next() {
		var id string
		var enabledAt time.Time
		if err := rows.Scan(&id, &enabledAt); err != nil {
			return nil, fmt.Errorf("servicelines: scan profile row: %w", err)
		}
		p.ServiceLines = append(p.ServiceLines, id)
		if enabledAt.After(p.UpdatedAt) {
			p.UpdatedAt = enabledAt
		}
	}
	return p, rows.Err()
}

func (s *pgStore) SetProfile(ctx context.Context, tenantID uuid.UUID, serviceLineIDs []string) (*Profile, error) {
	ids := dedupeSorted(serviceLineIDs)
	if err := ValidateIDs(ids); err != nil {
		return nil, fmt.Errorf("servicelines: set profile: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("servicelines: set profile: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `DELETE FROM tenant_service_lines WHERE tenant_id = $1`, tenantID); err != nil {
		return nil, fmt.Errorf("servicelines: set profile: clear existing: %w", err)
	}

	now := time.Now().UTC()
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `
			INSERT INTO tenant_service_lines (tenant_id, service_line_id, enabled_at)
			VALUES ($1, $2, $3)`, tenantID, id, now,
		); err != nil {
			return nil, fmt.Errorf("servicelines: set profile: insert %s: %w", id, err)
		}
	}

	if err := mergeActiveModules(ctx, tx, tenantID, ResolveModules(ids)); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("servicelines: set profile: commit: %w", err)
	}

	return &Profile{TenantID: tenantID, ServiceLines: ids, UpdatedAt: now}, nil
}

// mergeActiveModules additively unions the modules implied by a tenant's
// newly selected service lines into tenants.settings.activeModules
// (see modules/tpt-practice/api/settings.go's TenantSettings). Modules
// manually enabled by a practice admin are preserved even if the
// corresponding service line is later deselected.
func mergeActiveModules(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, resolvedModules []string) error {
	var raw []byte
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(settings, '{}') FROM tenants WHERE id = $1 FOR UPDATE`, tenantID,
	).Scan(&raw); err != nil {
		return fmt.Errorf("servicelines: load tenant settings: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil || settings == nil {
		settings = map[string]any{}
	}

	var existing []string
	if am, ok := settings["activeModules"].([]any); ok {
		for _, v := range am {
			if s, ok := v.(string); ok {
				existing = append(existing, s)
			}
		}
	}

	settings["activeModules"] = unionStrings(existing, resolvedModules)

	encoded, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("servicelines: marshal tenant settings: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE tenants SET settings = $1, updated_at = now() WHERE id = $2`, encoded, tenantID,
	); err != nil {
		return fmt.Errorf("servicelines: update tenant settings: %w", err)
	}
	return nil
}
