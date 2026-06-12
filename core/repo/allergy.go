package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AllergyCategory classifies the type of allergic reaction.
type AllergyCategory string

const (
	AllergyCategoryFood        AllergyCategory = "food"
	AllergyCategoryMedication  AllergyCategory = "medication"
	AllergyCategoryEnvironment AllergyCategory = "environment"
	AllergyCategoryBiologic    AllergyCategory = "biologic"
)

// AllergyClinicalStatus indicates whether the allergy/intolerance is current.
type AllergyClinicalStatus string

const (
	AllergyStatusActive   AllergyClinicalStatus = "active"
	AllergyStatusInactive AllergyClinicalStatus = "inactive"
	AllergyStatusResolved AllergyClinicalStatus = "resolved"
)

// AllergyReaction describes an observed adverse reaction to the substance.
type AllergyReaction struct {
	// Manifestation is a plain-language description or SNOMED code of the reaction
	// (e.g. "urticaria", "anaphylaxis", "nausea").
	Manifestation string `json:"manifestation"`
	// Severity is "mild", "moderate", or "severe".
	Severity string `json:"severity,omitempty"`
	// OnsetDateTime is when this reaction was observed.
	OnsetDateTime *time.Time `json:"onsetDateTime,omitempty"`
}

// AllergyRecord is a structured representation of a patient's allergy or
// intolerance. It serialises to/from the FHIR AllergyIntolerance resource
// stored in the FHIR repo.
type AllergyRecord struct {
	// ID is the FHIR resource ID.
	ID string `json:"id,omitempty"`
	// TenantID scopes the record to a practice/organisation.
	TenantID uuid.UUID `json:"tenantId"`
	// PatientID is the FHIR Patient resource ID.
	PatientID string `json:"patientId"`
	// ClinicalStatus indicates whether the allergy is currently active.
	ClinicalStatus AllergyClinicalStatus `json:"clinicalStatus"`
	// Category classifies the allergen type.
	Category AllergyCategory `json:"category"`
	// Substance is the substance causing the allergy — typically a generic drug
	// name, food item, or environmental trigger.
	Substance string `json:"substance"`
	// SubstanceNZULM is the NZMT NZULM code, if the substance is a medicine.
	SubstanceNZULM string `json:"substanceNzulm,omitempty"`
	// Reactions lists observed adverse reactions with their severity.
	Reactions []AllergyReaction `json:"reactions,omitempty"`
	// RecordedDate is when the allergy was documented.
	RecordedDate time.Time `json:"recordedDate"`
	// RecorderPractitionerID is the HPI CPN of the practitioner who recorded the allergy.
	RecorderPractitionerID string `json:"recorderPractitionerId,omitempty"`
	// Note is a free-text clinical note about the allergy.
	Note string `json:"note,omitempty"`
}

const allergyResourceType = "AllergyIntolerance"

// AllergyStore provides allergy/intolerance record management for a patient,
// backed by the FHIR resource store.
type AllergyStore struct {
	store Store
}

// NewAllergyStore creates an AllergyStore using the supplied FHIR Store.
func NewAllergyStore(store Store) *AllergyStore {
	return &AllergyStore{store: store}
}

// Create persists a new AllergyRecord and returns it with the server-assigned ID.
func (s *AllergyStore) Create(ctx context.Context, rec AllergyRecord) (*AllergyRecord, error) {
	if rec.PatientID == "" {
		return nil, fmt.Errorf("allergy: patientId is required")
	}
	if rec.Substance == "" {
		return nil, fmt.Errorf("allergy: substance is required")
	}
	if rec.ClinicalStatus == "" {
		rec.ClinicalStatus = AllergyStatusActive
	}
	rec.RecordedDate = time.Now().UTC()

	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("allergy: marshaling record: %w", err)
	}

	meta, err := s.store.Create(ctx, rec.TenantID.String(), allergyResourceType, "", raw)
	if err != nil {
		return nil, fmt.Errorf("allergy: persisting record: %w", err)
	}
	rec.ID = meta.ResourceID
	return &rec, nil
}

// Get retrieves a single AllergyRecord by resource ID.
func (s *AllergyStore) Get(ctx context.Context, tenantID uuid.UUID, id string) (*AllergyRecord, error) {
	raw, _, err := s.store.Read(ctx, tenantID.String(), allergyResourceType, id)
	if err != nil {
		return nil, fmt.Errorf("allergy: reading record %s: %w", id, err)
	}
	var rec AllergyRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, fmt.Errorf("allergy: decoding record: %w", err)
	}
	return &rec, nil
}

// Update replaces an existing AllergyRecord.
func (s *AllergyStore) Update(ctx context.Context, rec AllergyRecord) (*AllergyRecord, error) {
	if rec.ID == "" {
		return nil, fmt.Errorf("allergy: id is required for update")
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("allergy: marshaling record: %w", err)
	}
	if _, err := s.store.Update(ctx, rec.TenantID.String(), allergyResourceType, rec.ID, raw); err != nil {
		return nil, fmt.Errorf("allergy: updating record %s: %w", rec.ID, err)
	}
	return &rec, nil
}

// Delete soft-deletes an AllergyRecord.
func (s *AllergyStore) Delete(ctx context.Context, tenantID uuid.UUID, id string) error {
	if err := s.store.Delete(ctx, tenantID.String(), allergyResourceType, id); err != nil {
		return fmt.Errorf("allergy: deleting record %s: %w", id, err)
	}
	return nil
}

// ListForPatient returns all allergy records for the given patient.
// Pass statusFilter to restrict to active/inactive/resolved; pass "" for all.
func (s *AllergyStore) ListForPatient(ctx context.Context, tenantID uuid.UUID, patientID string, statusFilter AllergyClinicalStatus) ([]AllergyRecord, error) {
	params := SearchParams{
		ResourceType: allergyResourceType,
		TenantID:     tenantID,
		Params: map[string][]string{
			"patient": {patientID},
		},
		Count: 200,
	}
	if statusFilter != "" {
		params.Params["clinical-status"] = []string{string(statusFilter)}
	}

	result, err := s.store.Search(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("allergy: searching for patient %s: %w", patientID, err)
	}

	records := make([]AllergyRecord, 0, len(result.Resources))
	for _, raw := range result.Resources {
		var rec AllergyRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}

// ActiveSubstances returns the list of active allergy substance names for
// a patient. This is the fast path used by the DDI checker at prescribing time.
func (s *AllergyStore) ActiveSubstances(ctx context.Context, tenantID uuid.UUID, patientID string) ([]string, error) {
	records, err := s.ListForPatient(ctx, tenantID, patientID, AllergyStatusActive)
	if err != nil {
		return nil, err
	}
	substances := make([]string, 0, len(records))
	for _, rec := range records {
		if rec.Substance != "" {
			substances = append(substances, rec.Substance)
		}
	}
	return substances, nil
}
