// Package r4 provides minimal FHIR R4 types for NHI/NES API compatibility.
// For full FHIR R4, use medplum/fhirtypes.
package r4

import "time"

// ---------------------------------------------------------------------------
// Common types
// ---------------------------------------------------------------------------

// Meta holds resource metadata.
type Meta struct {
	VersionID   string    `json:"versionId,omitempty"`
	LastUpdated time.Time `json:"lastUpdated,omitempty"`
	Source      string    `json:"source,omitempty"`
	Profile     []string  `json:"profile,omitempty"`
	Tag         []Coding  `json:"tag,omitempty"`
	Security    []Coding  `json:"security,omitempty"`
}

// Coding is a code with a system and display label.
type Coding struct {
	System  string `json:"system,omitempty"`
	Version string `json:"version,omitempty"`
	Code    string `json:"code,omitempty"`
	Display string `json:"display,omitempty"`
}

// CodeableConcept is a concept with coding(s) and text.
type CodeableConcept struct {
	Coding []Coding `json:"coding,omitempty"`
	Text   string   `json:"text,omitempty"`
}

// Identifier is a business identifier.
type Identifier struct {
	Use    string          `json:"use,omitempty"`
	Type   CodeableConcept `json:"type,omitempty"`
	System string          `json:"system,omitempty"`
	Value  string          `json:"value,omitempty"`
	Period *Period         `json:"period,omitempty"`
}

// HumanName represents a person's name.
type HumanName struct {
	Use    string   `json:"use,omitempty"`
	Text   string   `json:"text,omitempty"`
	Family string   `json:"family,omitempty"`
	Given  []string `json:"given,omitempty"`
	Prefix []string `json:"prefix,omitempty"`
	Suffix []string `json:"suffix,omitempty"`
	Period *Period  `json:"period,omitempty"`
}

// Address holds a physical or postal address.
type Address struct {
	Use        string   `json:"use,omitempty"`
	Type       string   `json:"type,omitempty"`
	Text       string   `json:"text,omitempty"`
	Line       []string `json:"line,omitempty"`
	City       string   `json:"city,omitempty"`
	District   string   `json:"district,omitempty"`
	State      string   `json:"state,omitempty"`
	PostalCode string   `json:"postalCode,omitempty"`
	Country    string   `json:"country,omitempty"`
	Period     *Period  `json:"period,omitempty"`
}

// ContactPoint holds a phone, fax, email, or URL.
type ContactPoint struct {
	System string  `json:"system,omitempty"`
	Value  string  `json:"value,omitempty"`
	Use    string  `json:"use,omitempty"`
	Rank   int     `json:"rank,omitempty"`
	Period *Period `json:"period,omitempty"`
}

// Reference is a reference to another resource.
type Reference struct {
	Reference  string      `json:"reference,omitempty"`
	Type       string      `json:"type,omitempty"`
	Identifier *Identifier `json:"identifier,omitempty"`
	Display    string      `json:"display,omitempty"`
}

// Period is a start/end date range.
type Period struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// Extension is a FHIR extension element.
type Extension struct {
	URL                  string           `json:"url"`
	ValueString          string           `json:"valueString,omitempty"`
	ValueCode            string           `json:"valueCode,omitempty"`
	ValueCoding          *Coding          `json:"valueCoding,omitempty"`
	ValueCodeableConcept *CodeableConcept `json:"valueCodeableConcept,omitempty"`
	ValueBoolean         *bool            `json:"valueBoolean,omitempty"`
	ValueInteger         *int             `json:"valueInteger,omitempty"`
	ValueDecimal         *float64         `json:"valueDecimal,omitempty"`
	ValueDateTime        *time.Time       `json:"valueDateTime,omitempty"`
	ValueReference       *Reference       `json:"valueReference,omitempty"`
	Extension            []Extension      `json:"extension,omitempty"`
}

// ---------------------------------------------------------------------------
// NZ identifier system constants (R4)
// ---------------------------------------------------------------------------

const (
	// NHISystemURL is the NHI identifier system.
	NHISystemURL = "https://standards.digital.health.nz/ns/nhi-id"
	// HPICPNSystemURL is the HPI CPN identifier system.
	HPICPNSystemURL = "https://standards.digital.health.nz/ns/hpi-person-id"
)

// ---------------------------------------------------------------------------
// Patient
// ---------------------------------------------------------------------------

// PatientCommunication describes a patient's language/communication preferences.
type PatientCommunication struct {
	Language  CodeableConcept `json:"language"`
	Preferred bool            `json:"preferred,omitempty"`
}

// PatientContact represents a contact party for a patient.
type PatientContact struct {
	Relationship []CodeableConcept `json:"relationship,omitempty"`
	Name         *HumanName        `json:"name,omitempty"`
	Telecom      []ContactPoint    `json:"telecom,omitempty"`
	Address      *Address          `json:"address,omitempty"`
	Gender       string            `json:"gender,omitempty"`
}

// Patient is a FHIR R4 Patient resource.
// Extensions follow R4 conventions. For NZ-specific extensions, the same
// extension URLs as R5 are used (they are version-agnostic in the NZ base IG).
type Patient struct {
	ResourceType         string                 `json:"resourceType"`
	ID                   string                 `json:"id,omitempty"`
	Meta                 *Meta                  `json:"meta,omitempty"`
	Extension            []Extension            `json:"extension,omitempty"`
	Identifier           []Identifier           `json:"identifier,omitempty"` // includes NHI
	Active               *bool                  `json:"active,omitempty"`
	Name                 []HumanName            `json:"name,omitempty"`
	Telecom              []ContactPoint         `json:"telecom,omitempty"`
	Gender               string                 `json:"gender,omitempty"`
	BirthDate            string                 `json:"birthDate,omitempty"` // YYYY-MM-DD
	DeceasedBoolean      *bool                  `json:"deceasedBoolean,omitempty"`
	DeceasedDateTime     *time.Time             `json:"deceasedDateTime,omitempty"`
	Address              []Address              `json:"address,omitempty"`
	MaritalStatus        *CodeableConcept       `json:"maritalStatus,omitempty"`
	Communication        []PatientCommunication `json:"communication,omitempty"`
	Contact              []PatientContact       `json:"contact,omitempty"`
	ManagingOrganization *Reference             `json:"managingOrganization,omitempty"`
}

// NHIIdentifier returns the first NHI identifier for the patient, or nil.
func (p *Patient) NHIIdentifier() *Identifier {
	for i := range p.Identifier {
		if p.Identifier[i].System == NHISystemURL {
			return &p.Identifier[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Practitioner
// ---------------------------------------------------------------------------

// PractitionerQualification describes a practitioner's credential.
type PractitionerQualification struct {
	Identifier []Identifier    `json:"identifier,omitempty"`
	Code       CodeableConcept `json:"code"`
	Period     *Period         `json:"period,omitempty"`
	Issuer     *Reference      `json:"issuer,omitempty"`
}

// Practitioner is a minimal FHIR R4 Practitioner resource.
type Practitioner struct {
	ResourceType  string                      `json:"resourceType"`
	ID            string                      `json:"id,omitempty"`
	Meta          *Meta                       `json:"meta,omitempty"`
	Extension     []Extension                 `json:"extension,omitempty"`
	Identifier    []Identifier                `json:"identifier,omitempty"` // includes HPI CPN
	Active        *bool                       `json:"active,omitempty"`
	Name          []HumanName                 `json:"name,omitempty"`
	Telecom       []ContactPoint              `json:"telecom,omitempty"`
	Gender        string                      `json:"gender,omitempty"`
	BirthDate     string                      `json:"birthDate,omitempty"`
	Address       []Address                   `json:"address,omitempty"`
	Qualification []PractitionerQualification `json:"qualification,omitempty"`
}

// HPICPNIdentifier returns the first HPI CPN identifier for the practitioner, or nil.
func (p *Practitioner) HPICPNIdentifier() *Identifier {
	for i := range p.Identifier {
		if p.Identifier[i].System == HPICPNSystemURL {
			return &p.Identifier[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Bundle
// ---------------------------------------------------------------------------

// BundleEntry is a single entry in a FHIR Bundle.
type BundleEntry struct {
	FullURL  string              `json:"fullUrl,omitempty"`
	Resource interface{}         `json:"resource,omitempty"`
	Search   *BundleEntrySearch  `json:"search,omitempty"`
	Request  *BundleEntryRequest `json:"request,omitempty"`
	Response *BundleEntryResponse `json:"response,omitempty"`
}

// BundleEntrySearch holds search match metadata.
type BundleEntrySearch struct {
	Mode  string  `json:"mode,omitempty"`
	Score float64 `json:"score,omitempty"`
}

// BundleEntryRequest holds HTTP request info for transaction bundles.
type BundleEntryRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

// BundleEntryResponse holds HTTP response info for transaction bundles.
type BundleEntryResponse struct {
	Status   string `json:"status"`
	Location string `json:"location,omitempty"`
	Etag     string `json:"etag,omitempty"`
}

// BundleLink is a link in a Bundle.
type BundleLink struct {
	Relation string `json:"relation"`
	URL      string `json:"url"`
}

// Bundle is a FHIR R4 Bundle resource.
type Bundle struct {
	ResourceType string        `json:"resourceType"`
	ID           string        `json:"id,omitempty"`
	Meta         *Meta         `json:"meta,omitempty"`
	Type         string        `json:"type"`
	Timestamp    *time.Time    `json:"timestamp,omitempty"`
	Total        *int          `json:"total,omitempty"`
	Link         []BundleLink  `json:"link,omitempty"`
	Entry        []BundleEntry `json:"entry,omitempty"`
}

// ---------------------------------------------------------------------------
// OperationOutcome
// ---------------------------------------------------------------------------

// OperationOutcomeIssue is a single issue in an OperationOutcome.
type OperationOutcomeIssue struct {
	Severity    string           `json:"severity"`
	Code        string           `json:"code"`
	Details     *CodeableConcept `json:"details,omitempty"`
	Diagnostics string           `json:"diagnostics,omitempty"`
	Expression  []string         `json:"expression,omitempty"`
	Location    []string         `json:"location,omitempty"`
}

// OperationOutcome is a FHIR R4 OperationOutcome resource.
type OperationOutcome struct {
	ResourceType string                  `json:"resourceType"`
	ID           string                  `json:"id,omitempty"`
	Meta         *Meta                   `json:"meta,omitempty"`
	Issue        []OperationOutcomeIssue `json:"issue"`
}
