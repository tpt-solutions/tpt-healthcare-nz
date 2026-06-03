package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/google/uuid"
)

// ObservationStore provides Observation-specific repository operations.
type ObservationStore struct {
	store Store
}

// NewObservationStore creates an ObservationStore backed by the given Store.
func NewObservationStore(s Store) *ObservationStore {
	return &ObservationStore{store: s}
}

// Create persists a new Observation resource.
func (os *ObservationStore) Create(ctx context.Context, tenantID uuid.UUID, obs *r5.Observation) (*ResourceMeta, error) {
	data, err := json.Marshal(obs)
	if err != nil {
		return nil, fmt.Errorf("repo: marshal observation: %w", err)
	}
	return os.store.Create(ctx, tenantID.String(), "Observation", obs.ID, data)
}

// Get retrieves an Observation by resource ID.
func (os *ObservationStore) Get(ctx context.Context, tenantID uuid.UUID, obsID string) (*r5.Observation, *ResourceMeta, error) {
	raw, meta, err := os.store.Read(ctx, tenantID.String(), "Observation", obsID)
	if err != nil {
		return nil, nil, err
	}
	var obs r5.Observation
	if err := json.Unmarshal(raw, &obs); err != nil {
		return nil, nil, fmt.Errorf("repo: unmarshal observation: %w", err)
	}
	return &obs, meta, nil
}

// ListByPatient retrieves all observations for a given patient.
func (os *ObservationStore) ListByPatient(ctx context.Context, tenantID uuid.UUID, patientID string, count, offset int) (*SearchResult, error) {
	return os.store.Search(ctx, SearchParams{
		ResourceType: "Observation",
		TenantID:     tenantID,
		Params: map[string][]string{
			"patient": {"Patient/" + patientID},
		},
		Count:  count,
		Offset: offset,
	})
}

// ListByCode retrieves observations matching a specific code (e.g., LOINC).
func (os *ObservationStore) ListByCode(ctx context.Context, tenantID uuid.UUID, code string, count, offset int) (*SearchResult, error) {
	return os.store.Search(ctx, SearchParams{
		ResourceType: "Observation",
		TenantID:     tenantID,
		Params: map[string][]string{
			"code": {code},
		},
		Count:  count,
		Offset: offset,
	})
}

// Update replaces an existing Observation resource.
func (os *ObservationStore) Update(ctx context.Context, tenantID uuid.UUID, obs *r5.Observation) (*ResourceMeta, error) {
	data, err := json.Marshal(obs)
	if err != nil {
		return nil, fmt.Errorf("repo: marshal observation: %w", err)
	}
	return os.store.Update(ctx, tenantID.String(), "Observation", obs.ID, data)
}

// Delete soft-deletes an Observation resource.
func (os *ObservationStore) Delete(ctx context.Context, tenantID uuid.UUID, obsID string) error {
	return os.store.Delete(ctx, tenantID.String(), "Observation", obsID)
}

// SearchObservations performs an Observation search with the given FHIR search parameters.
func (os *ObservationStore) SearchObservations(ctx context.Context, tenantID uuid.UUID, params map[string][]string, count, offset int) (*SearchResult, error) {
	return os.store.Search(ctx, SearchParams{
		ResourceType: "Observation",
		TenantID:     tenantID,
		Params:       params,
		Count:        count,
		Offset:       offset,
	})
}