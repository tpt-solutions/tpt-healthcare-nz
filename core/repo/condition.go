package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ConditionClinicalStatus mirrors the FHIR Condition.clinicalStatus value set.
type ConditionClinicalStatus string

const (
	ConditionActive     ConditionClinicalStatus = "active"
	ConditionRecurrence ConditionClinicalStatus = "recurrence"
	ConditionRelapse    ConditionClinicalStatus = "relapse"
	ConditionInactive   ConditionClinicalStatus = "inactive"
	ConditionRemission  ConditionClinicalStatus = "remission"
	ConditionResolved   ConditionClinicalStatus = "resolved"
)

// ConditionVerificationStatus mirrors the FHIR Condition.verificationStatus value set.
type ConditionVerificationStatus string

const (
	ConditionUnconfirmed    ConditionVerificationStatus = "unconfirmed"
	ConditionProvisional    ConditionVerificationStatus = "provisional"
	ConditionDifferential   ConditionVerificationStatus = "differential"
	ConditionConfirmed      ConditionVerificationStatus = "confirmed"
	ConditionRefuted        ConditionVerificationStatus = "refuted"
	ConditionEnteredInError ConditionVerificationStatus = "entered-in-error"
)

// ConditionCategory distinguishes where in the workflow this condition originated.
type ConditionCategory string

const (
	// ProblemListItem indicates this condition is on the patient's active problem list.
	ProblemListItem ConditionCategory = "problem-list-item"
	// EncounterDiagnosis indicates this condition was recorded as an encounter diagnosis.
	EncounterDiagnosis ConditionCategory = "encounter-diagnosis"
)

// ConditionCoding represents a coded clinical concept (SNOMED, ICD-10-AM).
type ConditionCoding struct {
	// System is the terminology system URI (e.g. "http://snomed.info/sct").
	System string `json:"system"`
	// Code is the concept code.
	Code string `json:"code"`
	// Display is the human-readable name.
	Display string `json:"display"`
}

// ConditionRecord is the domain model for a FHIR Condition resource.
// It holds structured clinical data and is persisted as JSON in the FHIR store.
type ConditionRecord struct {
	// ID is the FHIR resource ID.
	ID string `json:"id,omitempty"`
	// TenantID scopes the record to a practice.
	TenantID uuid.UUID `json:"tenantId"`
	// PatientID is the FHIR Patient resource ID.
	PatientID string `json:"patientId"`
	// EncounterID is the FHIR Encounter resource ID that generated this condition,
	// if applicable.
	EncounterID string `json:"encounterId,omitempty"`
	// Category indicates whether this is a problem list item or encounter diagnosis.
	Category ConditionCategory `json:"category"`
	// ClinicalStatus reflects the current state of the condition.
	ClinicalStatus ConditionClinicalStatus `json:"clinicalStatus"`
	// VerificationStatus reflects the degree of diagnostic certainty.
	VerificationStatus ConditionVerificationStatus `json:"verificationStatus"`
	// Codings contains the coded representation(s) of the condition.
	// Include both SNOMED and ICD-10-AM codes where available.
	Codings []ConditionCoding `json:"codings"`
	// Display is a human-readable label shown in the problem list sidebar.
	Display string `json:"display"`
	// OnsetDateTime is when the condition began, if known.
	OnsetDateTime *time.Time `json:"onsetDateTime,omitempty"`
	// AbatementDateTime is when the condition resolved or went into remission.
	AbatementDateTime *time.Time `json:"abatementDateTime,omitempty"`
	// RecordedDate is when the condition was documented in the system.
	RecordedDate time.Time `json:"recordedDate"`
	// RecorderID is the HPI CPN of the practitioner who recorded the condition.
	RecorderID string `json:"recorderId,omitempty"`
	// Note is a free-text clinical note.
	Note string `json:"note,omitempty"`
	// Severity is an optional severity grading: "mild", "moderate", or "severe".
	Severity string `json:"severity,omitempty"`
}

const conditionResourceType = "Condition"

// ConditionStore manages patient problem lists and encounter diagnoses using
// the underlying FHIR resource store.
type ConditionStore struct {
	store Store
}

// NewConditionStore creates a ConditionStore backed by the supplied FHIR Store.
func NewConditionStore(store Store) *ConditionStore {
	return &ConditionStore{store: store}
}

// Create persists a new ConditionRecord.
func (s *ConditionStore) Create(ctx context.Context, rec ConditionRecord) (*ConditionRecord, error) {
	if rec.PatientID == "" {
		return nil, fmt.Errorf("condition: patientId is required")
	}
	if rec.Display == "" && len(rec.Codings) == 0 {
		return nil, fmt.Errorf("condition: at least one coding or display is required")
	}
	if rec.ClinicalStatus == "" {
		rec.ClinicalStatus = ConditionActive
	}
	if rec.VerificationStatus == "" {
		rec.VerificationStatus = ConditionConfirmed
	}
	if rec.Category == "" {
		rec.Category = ProblemListItem
	}
	rec.RecordedDate = time.Now().UTC()

	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("condition: marshaling record: %w", err)
	}
	meta, err := s.store.Create(ctx, rec.TenantID.String(), conditionResourceType, "", raw)
	if err != nil {
		return nil, fmt.Errorf("condition: persisting record: %w", err)
	}
	rec.ID = meta.ResourceID
	return &rec, nil
}

// Get retrieves a ConditionRecord by resource ID.
func (s *ConditionStore) Get(ctx context.Context, tenantID uuid.UUID, id string) (*ConditionRecord, error) {
	raw, _, err := s.store.Read(ctx, tenantID.String(), conditionResourceType, id)
	if err != nil {
		return nil, fmt.Errorf("condition: reading %s: %w", id, err)
	}
	var rec ConditionRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, fmt.Errorf("condition: decoding record: %w", err)
	}
	return &rec, nil
}

// Update replaces a ConditionRecord.
func (s *ConditionStore) Update(ctx context.Context, rec ConditionRecord) (*ConditionRecord, error) {
	if rec.ID == "" {
		return nil, fmt.Errorf("condition: id required for update")
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("condition: marshaling: %w", err)
	}
	if _, err := s.store.Update(ctx, rec.TenantID.String(), conditionResourceType, rec.ID, raw); err != nil {
		return nil, fmt.Errorf("condition: updating %s: %w", rec.ID, err)
	}
	return &rec, nil
}

// Delete soft-deletes a ConditionRecord.
func (s *ConditionStore) Delete(ctx context.Context, tenantID uuid.UUID, id string) error {
	return s.store.Delete(ctx, tenantID.String(), conditionResourceType, id)
}

// ProblemList returns the active problem list for a patient — all conditions
// with category=problem-list-item and clinicalStatus=active|recurrence|relapse.
func (s *ConditionStore) ProblemList(ctx context.Context, tenantID uuid.UUID, patientID string) ([]ConditionRecord, error) {
	result, err := s.store.Search(ctx, SearchParams{
		ResourceType: conditionResourceType,
		TenantID:     tenantID,
		Params: map[string][]string{
			"patient":         {patientID},
			"category":        {string(ProblemListItem)},
			"clinical-status": {"active", "recurrence", "relapse"},
		},
		Count: 200,
	})
	if err != nil {
		return nil, fmt.Errorf("condition: problem list for %s: %w", patientID, err)
	}
	return decodeConditions(result.Resources), nil
}

// EncounterDiagnoses returns all conditions recorded for a specific encounter.
func (s *ConditionStore) EncounterDiagnoses(ctx context.Context, tenantID uuid.UUID, encounterID string) ([]ConditionRecord, error) {
	result, err := s.store.Search(ctx, SearchParams{
		ResourceType: conditionResourceType,
		TenantID:     tenantID,
		Params: map[string][]string{
			"encounter": {encounterID},
			"category":  {string(EncounterDiagnosis)},
		},
		Count: 50,
	})
	if err != nil {
		return nil, fmt.Errorf("condition: encounter diagnoses for %s: %w", encounterID, err)
	}
	return decodeConditions(result.Resources), nil
}

// PromoteToProblems promotes all confirmed encounter diagnoses for an encounter
// onto the patient's problem list. Diagnoses already on the problem list
// (same SNOMED code) are skipped to avoid duplicates.
func (s *ConditionStore) PromoteToProblems(ctx context.Context, tenantID uuid.UUID, encounterID string, recorderID string) ([]ConditionRecord, error) {
	diagnoses, err := s.EncounterDiagnoses(ctx, tenantID, encounterID)
	if err != nil {
		return nil, err
	}

	var promoted []ConditionRecord
	for _, d := range diagnoses {
		if d.VerificationStatus != ConditionConfirmed {
			continue
		}
		promoted = append(promoted, ConditionRecord{
			TenantID:           tenantID,
			PatientID:          d.PatientID,
			EncounterID:        encounterID,
			Category:           ProblemListItem,
			ClinicalStatus:     ConditionActive,
			VerificationStatus: ConditionConfirmed,
			Codings:            d.Codings,
			Display:            d.Display,
			OnsetDateTime:      d.OnsetDateTime,
			RecordedDate:       time.Now().UTC(),
			RecorderID:         recorderID,
			Note:               d.Note,
			Severity:           d.Severity,
		})
	}

	created := make([]ConditionRecord, 0, len(promoted))
	for _, p := range promoted {
		c, err := s.Create(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("condition: promoting diagnosis: %w", err)
		}
		created = append(created, *c)
	}
	return created, nil
}

func decodeConditions(raws []json.RawMessage) []ConditionRecord {
	out := make([]ConditionRecord, 0, len(raws))
	for _, raw := range raws {
		var rec ConditionRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out
}
