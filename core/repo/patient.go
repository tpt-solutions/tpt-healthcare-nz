package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/google/uuid"
)

// PatientStore provides Patient-specific repository operations.
type PatientStore struct {
	store Store
}

// NewPatientStore creates a PatientStore backed by the given Store.
func NewPatientStore(s Store) *PatientStore {
	return &PatientStore{store: s}
}

// Create persists a new Patient resource.
func (ps *PatientStore) Create(ctx context.Context, tenantID uuid.UUID, patient *r5.Patient) (*ResourceMeta, error) {
	data, err := json.Marshal(patient)
	if err != nil {
		return nil, fmt.Errorf("repo: marshal patient: %w", err)
	}
	return ps.store.Create(ctx, tenantID.String(), "Patient", patient.ID, data)
}

// Get retrieves a Patient by resource ID.
func (ps *PatientStore) Get(ctx context.Context, tenantID uuid.UUID, patientID string) (*r5.Patient, *ResourceMeta, error) {
	raw, meta, err := ps.store.Read(ctx, tenantID.String(), "Patient", patientID)
	if err != nil {
		return nil, nil, err
	}
	var patient r5.Patient
	if err := json.Unmarshal(raw, &patient); err != nil {
		return nil, nil, fmt.Errorf("repo: unmarshal patient: %w", err)
	}
	return &patient, meta, nil
}

// GetByNHI retrieves a Patient by NHI identifier.
// This searches the identifier array for the NHI system URL.
func (ps *PatientStore) GetByNHI(ctx context.Context, tenantID uuid.UUID, nhi string) (*r5.Patient, *ResourceMeta, error) {
	result, err := ps.store.Search(ctx, SearchParams{
		ResourceType: "Patient",
		TenantID:     tenantID,
		Params: map[string][]string{
			"identifier": {nhiSystem + "|" + nhi},
		},
		Count:  1,
		Offset: 0,
	})
	if err != nil {
		return nil, nil, err
	}
	if len(result.Resources) == 0 {
		return nil, nil, fmt.Errorf("repo: patient by NHI %s: %w", nhi, ErrNotFound)
	}
	var patient r5.Patient
	if err := json.Unmarshal(result.Resources[0], &patient); err != nil {
		return nil, nil, fmt.Errorf("repo: unmarshal patient: %w", err)
	}
	// Reconstruct meta from the resource (lastUpdated from search isn't returned in ResourceMeta for search results).
	meta := &ResourceMeta{
		ResourceType: "Patient",
		ResourceID:   patient.ID,
		TenantID:     tenantID,
	}
	if patient.Meta != nil {
		meta.VersionID = patient.Meta.VersionID
		meta.LastUpdated = patient.Meta.LastUpdated
	}
	return &patient, meta, nil
}

// Update replaces an existing Patient resource.
func (ps *PatientStore) Update(ctx context.Context, tenantID uuid.UUID, patient *r5.Patient) (*ResourceMeta, error) {
	data, err := json.Marshal(patient)
	if err != nil {
		return nil, fmt.Errorf("repo: marshal patient: %w", err)
	}
	return ps.store.Update(ctx, tenantID.String(), "Patient", patient.ID, data)
}

// Delete soft-deletes a Patient resource.
func (ps *PatientStore) Delete(ctx context.Context, tenantID uuid.UUID, patientID string) error {
	return ps.store.Delete(ctx, tenantID.String(), "Patient", patientID)
}

// SearchPatients performs a Patient search with the given FHIR search parameters.
func (ps *PatientStore) SearchPatients(ctx context.Context, tenantID uuid.UUID, params map[string][]string, count, offset int) (*SearchResult, error) {
	return ps.store.Search(ctx, SearchParams{
		ResourceType: "Patient",
		TenantID:     tenantID,
		Params:       params,
		Count:        count,
		Offset:       offset,
	})
}