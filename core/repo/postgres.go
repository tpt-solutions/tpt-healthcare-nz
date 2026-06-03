package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a PostgreSQL JSONB-backed implementation of Store.
// Resources are stored in the fhir_resources table with the following schema:
//
//	CREATE TABLE fhir_resources (
//	    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
//	    tenant_id    UUID        NOT NULL,
//	    resource_type TEXT       NOT NULL,
//	    resource_id  TEXT        NOT NULL,
//	    version_id   BIGINT      NOT NULL DEFAULT 1,
//	    data         JSONB       NOT NULL,
//	    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
//	    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
//	    deleted_at   TIMESTAMPTZ,
//	    UNIQUE (tenant_id, resource_type, resource_id)
//	);
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgresStore backed by the given connection pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Create inserts a new FHIR resource into the fhir_resources table.
// If resourceID is empty, a new UUID is generated.
func (s *PostgresStore) Create(ctx context.Context, tenantID, resourceType, resourceID string, data json.RawMessage) (*ResourceMeta, error) {
	if resourceID == "" {
		resourceID = uuid.NewString()
	}

	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("repo: invalid tenantID %q: %w", tenantID, err)
	}

	const q = `
		INSERT INTO fhir_resources (tenant_id, resource_type, resource_id, version_id, data, created_at, updated_at)
		VALUES ($1, $2, $3, 1, $4, now(), now())
		RETURNING version_id, updated_at`

	var (
		versionID   int64
		lastUpdated time.Time
	)
	err = s.pool.QueryRow(ctx, q, tid, resourceType, resourceID, data).Scan(&versionID, &lastUpdated)
	if err != nil {
		return nil, fmt.Errorf("repo: create %s/%s: %w", resourceType, resourceID, err)
	}

	return &ResourceMeta{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		VersionID:    fmt.Sprintf("%d", versionID),
		TenantID:     tid,
		LastUpdated:  lastUpdated,
	}, nil
}

// Read retrieves a FHIR resource by tenant, type, and logical ID.
// Returns ErrNotFound if the resource does not exist or has been soft-deleted.
func (s *PostgresStore) Read(ctx context.Context, tenantID, resourceType, resourceID string) (json.RawMessage, *ResourceMeta, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("repo: invalid tenantID %q: %w", tenantID, err)
	}

	const q = `
		SELECT data, version_id, updated_at
		FROM fhir_resources
		WHERE tenant_id = $1
		  AND resource_type = $2
		  AND resource_id = $3
		  AND deleted_at IS NULL`

	var (
		raw         json.RawMessage
		versionID   int64
		lastUpdated time.Time
	)
	err = s.pool.QueryRow(ctx, q, tid, resourceType, resourceID).Scan(&raw, &versionID, &lastUpdated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, fmt.Errorf("repo: %s/%s not found: %w", resourceType, resourceID, ErrNotFound)
		}
		return nil, nil, fmt.Errorf("repo: read %s/%s: %w", resourceType, resourceID, err)
	}

	return raw, &ResourceMeta{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		VersionID:    fmt.Sprintf("%d", versionID),
		TenantID:     tid,
		LastUpdated:  lastUpdated,
	}, nil
}

// Update replaces the data for an existing, non-deleted resource and increments version_id.
func (s *PostgresStore) Update(ctx context.Context, tenantID, resourceType, resourceID string, data json.RawMessage) (*ResourceMeta, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("repo: invalid tenantID %q: %w", tenantID, err)
	}

	const q = `
		UPDATE fhir_resources
		SET    data       = $4,
		       version_id = version_id + 1,
		       updated_at = now()
		WHERE  tenant_id     = $1
		  AND  resource_type = $2
		  AND  resource_id   = $3
		  AND  deleted_at IS NULL
		RETURNING version_id, updated_at`

	var (
		versionID   int64
		lastUpdated time.Time
	)
	err = s.pool.QueryRow(ctx, q, tid, resourceType, resourceID, data).Scan(&versionID, &lastUpdated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repo: %s/%s not found: %w", resourceType, resourceID, ErrNotFound)
		}
		return nil, fmt.Errorf("repo: update %s/%s: %w", resourceType, resourceID, err)
	}

	return &ResourceMeta{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		VersionID:    fmt.Sprintf("%d", versionID),
		TenantID:     tid,
		LastUpdated:  lastUpdated,
	}, nil
}

// Delete soft-deletes a resource by setting deleted_at.
func (s *PostgresStore) Delete(ctx context.Context, tenantID, resourceType, resourceID string) error {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return fmt.Errorf("repo: invalid tenantID %q: %w", tenantID, err)
	}

	const q = `
		UPDATE fhir_resources
		SET    deleted_at = now(),
		       updated_at = now()
		WHERE  tenant_id     = $1
		  AND  resource_type = $2
		  AND  resource_id   = $3
		  AND  deleted_at IS NULL`

	tag, err := s.pool.Exec(ctx, q, tid, resourceType, resourceID)
	if err != nil {
		return fmt.Errorf("repo: delete %s/%s: %w", resourceType, resourceID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("repo: %s/%s not found: %w", resourceType, resourceID, ErrNotFound)
	}
	return nil
}

// Search queries for FHIR resources matching params.
// Simple equality FHIR parameters are converted to PostgreSQL JSONB @> containment queries.
// More complex parameter handling (chaining, modifiers) should be delegated to SearchEngine.
func (s *PostgresStore) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	engine := NewSearchEngine()
	whereClause, args, err := engine.Build(params)
	if err != nil {
		return nil, fmt.Errorf("repo: build search query: %w", err)
	}

	// Positional arg index after engine args
	nextArg := len(args) + 1

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM fhir_resources
		WHERE deleted_at IS NULL
		  AND tenant_id = $%d
		  AND resource_type = $%d
		  %s`,
		nextArg, nextArg+1, whereClause)

	countArgs := append(args, params.TenantID, params.ResourceType) //nolint:gocritic

	var total int
	err = s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("repo: search count: %w", err)
	}

	pageSize := params.Count
	if pageSize <= 0 {
		pageSize = 20
	}

	dataQuery := fmt.Sprintf(`
		SELECT data
		FROM fhir_resources
		WHERE deleted_at IS NULL
		  AND tenant_id = $%d
		  AND resource_type = $%d
		  %s
		ORDER BY updated_at DESC
		LIMIT $%d OFFSET $%d`,
		nextArg, nextArg+1, whereClause, nextArg+2, nextArg+3)

	dataArgs := append(countArgs, pageSize, params.Offset) //nolint:gocritic

	rows, err := s.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("repo: search query: %w", err)
	}
	defer rows.Close()

	var resources []json.RawMessage
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("repo: scan search row: %w", err)
		}
		resources = append(resources, raw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: search rows: %w", err)
	}

	nextOffset := -1
	if params.Offset+pageSize < total {
		nextOffset = params.Offset + pageSize
	}

	return &SearchResult{
		Resources:  resources,
		Total:      total,
		NextOffset: nextOffset,
	}, nil
}

// ErrNotFound is returned when a requested resource does not exist or has been deleted.
var ErrNotFound = errors.New("not found")
