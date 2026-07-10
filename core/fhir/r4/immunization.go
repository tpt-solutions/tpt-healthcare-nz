// Package r4 provides minimal FHIR R4 types for NHI/NES API compatibility.
// For full FHIR R4, use medplum/fhirtypes.
package r4

import "time"

// ---------------------------------------------------------------------------
// Immunization
// ---------------------------------------------------------------------------

// Immunization is a FHIR R4 Immunization resource.
// This type is used for NIR submission (NIR uses FHIR R4).
type Immunization struct {
	ResourceType       string        `json:"resourceType"`
	ID                 string        `json:"id,omitempty"`
	Meta               *Meta         `json:"meta,omitempty"`
	Extension          []Extension   `json:"extension,omitempty"`
	Identifier         []Identifier  `json:"identifier,omitempty"`
	Status             string        `json:"status"`
	VaccineCode        CodeableConcept `json:"vaccineCode"`
	Patient            *Reference    `json:"patient"`
	OccurrenceDateTime *time.Time    `json:"occurrenceDateTime,omitempty"`
	OccurrenceString   string        `json:"occurrenceString,omitempty"`
	LotNumber          string        `json:"lotNumber,omitempty"`
	ExpirationDate     string        `json:"expirationDate,omitempty"` // YYYY-MM-DD
	Site               *CodeableConcept `json:"site,omitempty"`
	Route              *CodeableConcept `json:"route,omitempty"`
	DoseQuantity       *Quantity     `json:"doseQuantity,omitempty"`
	Note               []Annotation  `json:"note,omitempty"`
}

// Annotation is a text note with author and time.
type Annotation struct {
	AuthorReference *Reference `json:"authorReference,omitempty"`
	AuthorString    string     `json:"authorString,omitempty"`
	Time            *time.Time  `json:"time,omitempty"`
	Text            string     `json:"text"`
}

// Quantity is a measured or measurable amount.
type Quantity struct {
	Value      float64 `json:"value,omitempty"`
	Comparator string  `json:"comparator,omitempty"`
	Unit       string  `json:"unit,omitempty"`
	System     string  `json:"system,omitempty"`
	Code       string  `json:"code,omitempty"`
}