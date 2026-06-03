// Package repo provides the FHIR resource repository abstraction and implementations.
package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Store is the FHIR resource repository interface.
// All operations are scoped to a tenant identified by TenantID.
type Store interface {
	// Create persists a new FHIR resource and returns its metadata.
	// resourceID may be empty, in which case the implementation generates one.
	Create(ctx context.Context, tenantID, resourceType, resourceID string, data json.RawMessage) (*ResourceMeta, error)

	// Read retrieves a FHIR resource by type and ID.
	// Returns the raw JSON and metadata, or an error if not found or deleted.
	Read(ctx context.Context, tenantID, resourceType, resourceID string) (json.RawMessage, *ResourceMeta, error)

	// Update replaces an existing FHIR resource and increments the version.
	// Returns the updated metadata.
	Update(ctx context.Context, tenantID, resourceType, resourceID string, data json.RawMessage) (*ResourceMeta, error)

	// Delete soft-deletes a FHIR resource (sets deleted_at).
	Delete(ctx context.Context, tenantID, resourceType, resourceID string) error

	// Search queries resources matching the given parameters.
	Search(ctx context.Context, params SearchParams) (*SearchResult, error)
}

// ResourceMeta holds server-assigned metadata for a stored FHIR resource.
type ResourceMeta struct {
	// ResourceType is the FHIR resource type (e.g. "Patient", "Observation").
	ResourceType string

	// ResourceID is the logical resource ID.
	ResourceID string

	// VersionID is the current version identifier (monotonically increasing integer, as string).
	VersionID string

	// TenantID is the tenant this resource belongs to.
	TenantID uuid.UUID

	// LastUpdated is when the resource was last written.
	LastUpdated time.Time
}

// SearchParams encapsulates the parameters for a FHIR search operation.
type SearchParams struct {
	// ResourceType is the FHIR resource type to search (e.g. "Patient").
	ResourceType string

	// Params holds raw FHIR search parameters as key → values.
	// Multiple values for a single key are treated as OR within that parameter
	// and AND across different parameters (standard FHIR search semantics).
	Params map[string][]string

	// TenantID scopes the search to a specific tenant.
	TenantID uuid.UUID

	// Count is the maximum number of results to return (page size).
	// A value of 0 uses the implementation default.
	Count int

	// Offset is the zero-based index of the first result to return.
	Offset int
}

// SearchResult holds the results of a FHIR search operation.
type SearchResult struct {
	// Resources contains the matched resources as raw JSON objects.
	Resources []json.RawMessage

	// Total is the total number of matching resources (before paging).
	Total int

	// NextOffset is the offset to use for the next page, or -1 if this is the last page.
	NextOffset int
}
