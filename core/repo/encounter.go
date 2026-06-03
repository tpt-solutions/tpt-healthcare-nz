package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/google/uuid"
)

// EncounterStore provides Encounter-specific repository operations.
type EncounterStore struct {
	store Store
}

// NewEncounterStore creates an EncounterStore backed by the given Store.
func NewEncounterStore(s Store) *EncounterStore {
	return &EncounterStore{store: s}
}

// Create persists a new Encounter resource.
func (es *EncounterStore) Create(ctx context.Context, tenantID uuid.UUID, enc *r5.Encounter) (*ResourceMeta, error) {
	data, err := json.Marshal(enc)
	if err != nil {
		return nil, fmt.Errorf("repo: marshal encounter: %w", err)
	}
	return es.store.Create(ctx, tenantID.String(), "Encounter", enc.ID, data)
}

// Get retrieves an Encounter by resource ID.
func (es *EncounterStore) Get(ctx context.Context, tenantID uuid.UUID, encID string) (*r5.Encounter, *ResourceMeta, error) {
	raw, meta, err := es.store.Read(ctx, tenantID.String(), "Encounter", encID)
	if err != nil {
		return nil, nil, err
	}
	var enc r5.Encounter
	if err := json.Unmarshal(raw, &enc); err != nil {
		return nil, nil, fmt.Errorf("repo: unmarshal encounter: %w", err)
	}
	return &enc, meta, nil
}

// ListByPatient retrieves all encounters for a given patient.
func (es *EncounterStore) ListByPatient(ctx context.Context, tenantID uuid.UUID, patientID string, count, offset int) (*SearchResult, error) {
	return es.store.Search(ctx, SearchParams{
		ResourceType: "Encounter",
		TenantID:     tenantID,
		Params: map[string][]string{
			"patient": {"Patient/" + patientID},
		},
		Count:  count,
		Offset: offset,
	})
}

// ListByStatus retrieves encounters matching a specific status.
func (es *EncounterStore) ListByStatus(ctx context.Context, tenantID uuid.UUID, status string, count, offset int) (*SearchResult, error) {
	return es.store.Search(ctx, SearchParams{
		ResourceType: "Encounter",
		TenantID:     tenantID,
		Params: map[string][]string{
			"status": {status},
		},
		Count:  count,
		Offset: offset,
	})
}

// ListByPeriod retrieves encounters within a date range.
// date is a FHIR date search parameter value, e.g. "ge2024-01-01" or "2024-01-01".
func (es *EncounterStore) ListByPeriod(ctx context.Context, tenantID uuid.UUID, date string, count, offset int) (*SearchResult, error) {
	return es.store.Search(ctx, SearchParams{
		ResourceType: "Encounter",
		TenantID:     tenantID,
		Params: map[string][]string{
			"date": {date},
		},
		Count:  count,
		Offset: offset,
	})
}

// Update replaces an existing Encounter resource.
func (es *EncounterStore) Update(ctx context.Context, tenantID uuid.UUID, enc *r5.Encounter) (*ResourceMeta, error) {
	data, err := json.Marshal(enc)
	if err != nil {
		return nil, fmt.Errorf("repo: marshal encounter: %w", err)
	}
	return es.store.Update(ctx, tenantID.String(), "Encounter", enc.ID, data)
}

// Delete soft-deletes an Encounter resource.
func (es *EncounterStore) Delete(ctx context.Context, tenantID uuid.UUID, encID string) error {
	return es.store.Delete(ctx, tenantID.String(), "Encounter", encID)
}

// SearchEncounters performs an Encounter search with the given FHIR search parameters.
func (es *EncounterStore) SearchEncounters(ctx context.Context, tenantID uuid.UUID, params map[string][]string, count, offset int) (*SearchResult, error) {
	return es.store.Search(ctx, SearchParams{
		ResourceType: "Encounter",
		TenantID:     tenantID,
		Params:       params,
		Count:        count,
		Offset:       offset,
	})
}