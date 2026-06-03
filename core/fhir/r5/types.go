// Package r5 provides FHIR R5 Go structs for NZ healthcare.
// These structs are practical (not exhaustive) and include the most important fields
// for the NZ health system context, including NZ-specific extensions (NZETHNIC, iwi).
package r5

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

// Narrative holds human-readable summary.
type Narrative struct {
	Status string `json:"status"`
	Div    string `json:"div"`
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
	Reference  string `json:"reference,omitempty"`
	Type       string `json:"type,omitempty"`
	Identifier *Identifier `json:"identifier,omitempty"`
	Display    string `json:"display,omitempty"`
}

// Period is a start/end date range.
type Period struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// Quantity is a measured or measurable amount.
type Quantity struct {
	Value      float64 `json:"value,omitempty"`
	Comparator string  `json:"comparator,omitempty"`
	Unit       string  `json:"unit,omitempty"`
	System     string  `json:"system,omitempty"`
	Code       string  `json:"code,omitempty"`
}

// Annotation is a text note with author and time.
type Annotation struct {
	AuthorReference *Reference `json:"authorReference,omitempty"`
	AuthorString    string     `json:"authorString,omitempty"`
	Time            *time.Time `json:"time,omitempty"`
	Text            string     `json:"text"`
}

// Attachment is a document or media attachment.
type Attachment struct {
	ContentType string     `json:"contentType,omitempty"`
	Language    string     `json:"language,omitempty"`
	Data        string     `json:"data,omitempty"` // base64
	URL         string     `json:"url,omitempty"`
	Size        int64      `json:"size,omitempty"`
	Hash        string     `json:"hash,omitempty"`
	Title       string     `json:"title,omitempty"`
	Creation    *time.Time `json:"creation,omitempty"`
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
// NZ-specific extension URLs
// ---------------------------------------------------------------------------

const (
	// NZEthnicityExtURL is the URL for the NZ ethnicity extension (NZETHNIC).
	NZEthnicityExtURL = "http://hl7.org.nz/fhir/StructureDefinition/nz-ethnicity"
	// NZIwiAffiliationExtURL is the URL for NZ iwi affiliation.
	NZIwiAffiliationExtURL = "http://hl7.org.nz/fhir/StructureDefinition/nz-iwi"
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

// Patient is a FHIR R5 Patient resource.
// NZ-specific extensions are carried in Extension:
//   - NZEthnicityExtURL  (one or more, valueCodeableConcept, NZETHNIC coding system)
//   - NZIwiAffiliationExtURL (zero or more, valueCodeableConcept)
type Patient struct {
	ResourceType         string                `json:"resourceType"`
	ID                   string                `json:"id,omitempty"`
	Meta                 *Meta                 `json:"meta,omitempty"`
	Text                 *Narrative            `json:"text,omitempty"`
	Extension            []Extension           `json:"extension,omitempty"`
	Identifier           []Identifier          `json:"identifier,omitempty"` // includes NHI
	Active               *bool                 `json:"active,omitempty"`
	Name                 []HumanName           `json:"name,omitempty"`
	Telecom              []ContactPoint        `json:"telecom,omitempty"`
	Gender               string                `json:"gender,omitempty"`
	BirthDate            string                `json:"birthDate,omitempty"` // YYYY-MM-DD
	DeceasedBoolean      *bool                 `json:"deceasedBoolean,omitempty"`
	DeceasedDateTime     *time.Time            `json:"deceasedDateTime,omitempty"`
	Address              []Address             `json:"address,omitempty"`
	MaritalStatus        *CodeableConcept      `json:"maritalStatus,omitempty"`
	Communication        []PatientCommunication `json:"communication,omitempty"`
	Contact              []PatientContact      `json:"contact,omitempty"`
	ManagingOrganization *Reference            `json:"managingOrganization,omitempty"`
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

// EthnicityExtensions returns all NZ ethnicity extensions on this patient.
func (p *Patient) EthnicityExtensions() []Extension {
	var out []Extension
	for _, ext := range p.Extension {
		if ext.URL == NZEthnicityExtURL {
			out = append(out, ext)
		}
	}
	return out
}

// IwiExtensions returns all NZ iwi affiliation extensions on this patient.
func (p *Patient) IwiExtensions() []Extension {
	var out []Extension
	for _, ext := range p.Extension {
		if ext.URL == NZIwiAffiliationExtURL {
			out = append(out, ext)
		}
	}
	return out
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

// Practitioner is a FHIR R5 Practitioner resource.
type Practitioner struct {
	ResourceType  string                      `json:"resourceType"`
	ID            string                      `json:"id,omitempty"`
	Meta          *Meta                       `json:"meta,omitempty"`
	Text          *Narrative                  `json:"text,omitempty"`
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
// Encounter
// ---------------------------------------------------------------------------

// EncounterParticipant is a participant in an encounter.
type EncounterParticipant struct {
	Type       []CodeableConcept `json:"type,omitempty"`
	Period     *Period           `json:"period,omitempty"`
	Actor      *Reference        `json:"actor,omitempty"`
}

// EncounterDiagnosis is a diagnosis associated with an encounter.
type EncounterDiagnosis struct {
	Condition *Reference        `json:"condition,omitempty"`
	Use       *CodeableConcept  `json:"use,omitempty"`
	Rank      int               `json:"rank,omitempty"`
}

// Encounter is a FHIR R5 Encounter resource.
type Encounter struct {
	ResourceType    string                `json:"resourceType"`
	ID              string                `json:"id,omitempty"`
	Meta            *Meta                 `json:"meta,omitempty"`
	Text            *Narrative            `json:"text,omitempty"`
	Extension       []Extension           `json:"extension,omitempty"`
	Identifier      []Identifier          `json:"identifier,omitempty"`
	Status          string                `json:"status"`
	Class           []CodeableConcept     `json:"class,omitempty"`
	Type            []CodeableConcept     `json:"type,omitempty"`
	Subject         *Reference            `json:"subject,omitempty"` // Patient reference
	Participant     []EncounterParticipant `json:"participant,omitempty"`
	Period          *Period               `json:"period,omitempty"`
	ReasonCode      []CodeableConcept     `json:"reason,omitempty"`
	Diagnosis       []EncounterDiagnosis  `json:"diagnosis,omitempty"`
	ServiceProvider *Reference            `json:"serviceProvider,omitempty"`
}

// ---------------------------------------------------------------------------
// Observation
// ---------------------------------------------------------------------------

// ObservationReferenceRange is a reference range for an observation.
type ObservationReferenceRange struct {
	Low     *Quantity        `json:"low,omitempty"`
	High    *Quantity        `json:"high,omitempty"`
	Type    *CodeableConcept `json:"type,omitempty"`
	Text    string           `json:"text,omitempty"`
}

// Observation is a FHIR R5 Observation resource.
type Observation struct {
	ResourceType          string                      `json:"resourceType"`
	ID                    string                      `json:"id,omitempty"`
	Meta                  *Meta                       `json:"meta,omitempty"`
	Text                  *Narrative                  `json:"text,omitempty"`
	Extension             []Extension                 `json:"extension,omitempty"`
	Identifier            []Identifier                `json:"identifier,omitempty"`
	Status                string                      `json:"status"`
	Category              []CodeableConcept           `json:"category,omitempty"`
	Code                  CodeableConcept             `json:"code"` // LOINC preferred
	Subject               *Reference                  `json:"subject,omitempty"`
	EffectiveDateTime     *time.Time                  `json:"effectiveDateTime,omitempty"`
	ValueQuantity         *Quantity                   `json:"valueQuantity,omitempty"`
	ValueCodeableConcept  *CodeableConcept            `json:"valueCodeableConcept,omitempty"`
	ValueString           string                      `json:"valueString,omitempty"`
	Interpretation        []CodeableConcept           `json:"interpretation,omitempty"`
	ReferenceRange        []ObservationReferenceRange `json:"referenceRange,omitempty"`
	Note                  []Annotation                `json:"note,omitempty"`
}

// ---------------------------------------------------------------------------
// Condition
// ---------------------------------------------------------------------------

// Condition is a FHIR R5 Condition resource.
type Condition struct {
	ResourceType       string          `json:"resourceType"`
	ID                 string          `json:"id,omitempty"`
	Meta               *Meta           `json:"meta,omitempty"`
	Text               *Narrative      `json:"text,omitempty"`
	Extension          []Extension     `json:"extension,omitempty"`
	Identifier         []Identifier    `json:"identifier,omitempty"`
	ClinicalStatus     CodeableConcept `json:"clinicalStatus"`
	VerificationStatus CodeableConcept `json:"verificationStatus,omitempty"`
	Category           []CodeableConcept `json:"category,omitempty"`
	Code               *CodeableConcept  `json:"code,omitempty"` // SNOMED CT or ICD-10
	Subject            *Reference        `json:"subject"`
	OnsetDateTime      *time.Time        `json:"onsetDateTime,omitempty"`
	RecordedDate       *time.Time        `json:"recordedDate,omitempty"`
	Note               []Annotation      `json:"note,omitempty"`
}

// ---------------------------------------------------------------------------
// MedicationRequest
// ---------------------------------------------------------------------------

// Dosage represents dosage instructions.
type Dosage struct {
	Sequence         int              `json:"sequence,omitempty"`
	Text             string           `json:"text,omitempty"`
	AdditionalInstruction []CodeableConcept `json:"additionalInstruction,omitempty"`
	PatientInstruction string          `json:"patientInstruction,omitempty"`
	Route            *CodeableConcept `json:"route,omitempty"`
	Method           *CodeableConcept `json:"method,omitempty"`
	DoseQuantity     *Quantity        `json:"doseAndRate,omitempty"`
}

// MedicationRequestDispenseRequest holds dispensing instructions.
type MedicationRequestDispenseRequest struct {
	NumberOfRepeatsAllowed int        `json:"numberOfRepeatsAllowed,omitempty"`
	Quantity               *Quantity  `json:"quantity,omitempty"`
	ExpectedSupplyDuration *Quantity  `json:"expectedSupplyDuration,omitempty"`
	ValidityPeriod         *Period    `json:"validityPeriod,omitempty"`
}

// MedicationRequestSubstitution holds substitution instructions.
type MedicationRequestSubstitution struct {
	AllowedBoolean          *bool            `json:"allowedBoolean,omitempty"`
	AllowedCodeableConcept  *CodeableConcept `json:"allowedCodeableConcept,omitempty"`
	Reason                  *CodeableConcept `json:"reason,omitempty"`
}

// MedicationRequest is a FHIR R5 MedicationRequest resource.
type MedicationRequest struct {
	ResourceType        string                             `json:"resourceType"`
	ID                  string                             `json:"id,omitempty"`
	Meta                *Meta                              `json:"meta,omitempty"`
	Text                *Narrative                         `json:"text,omitempty"`
	Extension           []Extension                        `json:"extension,omitempty"`
	Identifier          []Identifier                       `json:"identifier,omitempty"`
	Status              string                             `json:"status"`
	Intent              string                             `json:"intent"`
	Medication          CodeableConcept                    `json:"medication"`
	Subject             *Reference                         `json:"subject"`
	AuthoredOn          *time.Time                         `json:"authoredOn,omitempty"`
	Requester           *Reference                         `json:"requester,omitempty"`
	DosageInstruction   []Dosage                           `json:"dosageInstruction,omitempty"`
	DispenseRequest     *MedicationRequestDispenseRequest  `json:"dispenseRequest,omitempty"`
	Substitution        *MedicationRequestSubstitution     `json:"substitution,omitempty"`
	Note                []Annotation                       `json:"note,omitempty"`
}

// ---------------------------------------------------------------------------
// DiagnosticReport
// ---------------------------------------------------------------------------

// DiagnosticReport is a FHIR R5 DiagnosticReport resource.
type DiagnosticReport struct {
	ResourceType      string            `json:"resourceType"`
	ID                string            `json:"id,omitempty"`
	Meta              *Meta             `json:"meta,omitempty"`
	Text              *Narrative        `json:"text,omitempty"`
	Extension         []Extension       `json:"extension,omitempty"`
	Identifier        []Identifier      `json:"identifier,omitempty"`
	Status            string            `json:"status"`
	Category          []CodeableConcept `json:"category,omitempty"`
	Code              CodeableConcept   `json:"code"`
	Subject           *Reference        `json:"subject,omitempty"`
	EffectiveDateTime *time.Time        `json:"effectiveDateTime,omitempty"`
	Issued            *time.Time        `json:"issued,omitempty"`
	Performer         []Reference       `json:"performer,omitempty"`
	Result            []Reference       `json:"result,omitempty"` // Observation references
	Conclusion        string            `json:"conclusion,omitempty"`
	ConclusionCode    []CodeableConcept `json:"conclusionCode,omitempty"`
}

// ---------------------------------------------------------------------------
// ServiceRequest
// ---------------------------------------------------------------------------

// ServiceRequest is a FHIR R5 ServiceRequest resource.
type ServiceRequest struct {
	ResourceType string            `json:"resourceType"`
	ID           string            `json:"id,omitempty"`
	Meta         *Meta             `json:"meta,omitempty"`
	Text         *Narrative        `json:"text,omitempty"`
	Extension    []Extension       `json:"extension,omitempty"`
	Identifier   []Identifier      `json:"identifier,omitempty"`
	Status       string            `json:"status"`
	Intent       string            `json:"intent"`
	Category     []CodeableConcept `json:"category,omitempty"`
	Code         *CodeableConcept  `json:"code,omitempty"`
	Subject      *Reference        `json:"subject"`
	AuthoredOn   *time.Time        `json:"authoredOn,omitempty"`
	Requester    *Reference        `json:"requester,omitempty"`
	Performer    []Reference       `json:"performer,omitempty"`
	ReasonCode   []CodeableConcept `json:"reasonCode,omitempty"`
	Note         []Annotation      `json:"note,omitempty"`
}

// ---------------------------------------------------------------------------
// Immunization
// ---------------------------------------------------------------------------

// Immunization is a FHIR R5 Immunization resource.
type Immunization struct {
	ResourceType       string          `json:"resourceType"`
	ID                 string          `json:"id,omitempty"`
	Meta               *Meta           `json:"meta,omitempty"`
	Text               *Narrative      `json:"text,omitempty"`
	Extension          []Extension     `json:"extension,omitempty"`
	Identifier         []Identifier    `json:"identifier,omitempty"`
	Status             string          `json:"status"`
	VaccineCode        CodeableConcept `json:"vaccineCode"`
	Patient            *Reference      `json:"patient"`
	OccurrenceDateTime *time.Time      `json:"occurrenceDateTime,omitempty"`
	OccurrenceString   string          `json:"occurrenceString,omitempty"`
	LotNumber          string          `json:"lotNumber,omitempty"`
	Site               *CodeableConcept `json:"site,omitempty"`
	Route              *CodeableConcept `json:"route,omitempty"`
	DoseQuantity       *Quantity        `json:"doseQuantity,omitempty"`
	Note               []Annotation     `json:"note,omitempty"`
}

// ---------------------------------------------------------------------------
// Claim
// ---------------------------------------------------------------------------

// ClaimCareTeam represents a care team member on a claim.
type ClaimCareTeam struct {
	Sequence  int        `json:"sequence"`
	Provider  Reference  `json:"provider"`
	Role      *CodeableConcept `json:"role,omitempty"`
}

// ClaimDiagnosis represents a diagnosis on a claim.
type ClaimDiagnosis struct {
	Sequence            int              `json:"sequence"`
	DiagnosisCodeableConcept *CodeableConcept `json:"diagnosisCodeableConcept,omitempty"`
	DiagnosisReference  *Reference       `json:"diagnosisReference,omitempty"`
	Type                []CodeableConcept `json:"type,omitempty"`
}

// ClaimProcedure represents a procedure on a claim.
type ClaimProcedure struct {
	Sequence                int               `json:"sequence"`
	ProcedureCodeableConcept *CodeableConcept `json:"procedureCodeableConcept,omitempty"`
	ProcedureReference      *Reference        `json:"procedureReference,omitempty"`
	Date                    *time.Time         `json:"date,omitempty"`
}

// ClaimItem represents a line item on a claim.
type ClaimItem struct {
	Sequence       int               `json:"sequence"`
	CareTeamSeq    []int             `json:"careTeamSequence,omitempty"`
	DiagnosisSeq   []int             `json:"diagnosisSequence,omitempty"`
	ProductOrService CodeableConcept `json:"productOrService"`
	ServicedDate   string            `json:"servicedDate,omitempty"`
	Quantity       *Quantity         `json:"quantity,omitempty"`
	UnitPrice      *Quantity         `json:"unitPrice,omitempty"`
	Net            *Quantity         `json:"net,omitempty"`
}

// Claim is a FHIR R5 Claim resource.
type Claim struct {
	ResourceType   string            `json:"resourceType"`
	ID             string            `json:"id,omitempty"`
	Meta           *Meta             `json:"meta,omitempty"`
	Text           *Narrative        `json:"text,omitempty"`
	Extension      []Extension       `json:"extension,omitempty"`
	Identifier     []Identifier      `json:"identifier,omitempty"`
	Status         string            `json:"status"`
	Type           CodeableConcept   `json:"type"`
	Use            string            `json:"use"`
	Patient        *Reference        `json:"patient"`
	BillablePeriod *Period           `json:"billablePeriod,omitempty"`
	Created        *time.Time        `json:"created,omitempty"`
	Insurer        *Reference        `json:"insurer,omitempty"`
	Provider       *Reference        `json:"provider,omitempty"`
	Priority       CodeableConcept   `json:"priority"`
	CareTeam       []ClaimCareTeam   `json:"careTeam,omitempty"`
	Diagnosis      []ClaimDiagnosis  `json:"diagnosis,omitempty"`
	Procedure      []ClaimProcedure  `json:"procedure,omitempty"`
	Item           []ClaimItem       `json:"item,omitempty"`
	Total          *Quantity         `json:"total,omitempty"`
}

// ---------------------------------------------------------------------------
// ClaimResponse
// ---------------------------------------------------------------------------

// ClaimResponseItem represents an adjudicated line item.
type ClaimResponseItem struct {
	ItemSequence int                          `json:"itemSequence"`
	Adjudication []ClaimResponseAdjudication  `json:"adjudication,omitempty"`
}

// ClaimResponseAdjudication is an adjudication detail.
type ClaimResponseAdjudication struct {
	Category CodeableConcept `json:"category"`
	Amount   *Quantity       `json:"amount,omitempty"`
	Value    float64         `json:"value,omitempty"`
}

// ClaimResponseAddItem is an added item in a ClaimResponse.
type ClaimResponseAddItem struct {
	ItemSequence []int            `json:"itemSequence,omitempty"`
	ProductOrService CodeableConcept `json:"productOrService"`
	Adjudication []ClaimResponseAdjudication `json:"adjudication,omitempty"`
}

// ClaimResponseTotal is a category total in a ClaimResponse.
type ClaimResponseTotal struct {
	Category CodeableConcept `json:"category"`
	Amount   Quantity        `json:"amount"`
}

// ClaimResponse is a FHIR R5 ClaimResponse resource.
type ClaimResponse struct {
	ResourceType string                 `json:"resourceType"`
	ID           string                 `json:"id,omitempty"`
	Meta         *Meta                  `json:"meta,omitempty"`
	Text         *Narrative             `json:"text,omitempty"`
	Extension    []Extension            `json:"extension,omitempty"`
	Identifier   []Identifier           `json:"identifier,omitempty"`
	Status       string                 `json:"status"`
	Type         CodeableConcept        `json:"type"`
	Use          string                 `json:"use"`
	Patient      *Reference             `json:"patient"`
	Created      *time.Time             `json:"created,omitempty"`
	Insurer      *Reference             `json:"insurer"`
	Outcome      string                 `json:"outcome"`
	Item         []ClaimResponseItem    `json:"item,omitempty"`
	AddItem      []ClaimResponseAddItem `json:"addItem,omitempty"`
	Total        []ClaimResponseTotal   `json:"total,omitempty"`
}

// ---------------------------------------------------------------------------
// Bundle
// ---------------------------------------------------------------------------

// BundleEntry is a single entry in a FHIR Bundle.
type BundleEntry struct {
	FullURL  string          `json:"fullUrl,omitempty"`
	Resource interface{}     `json:"resource,omitempty"`
	Search   *BundleEntrySearch `json:"search,omitempty"`
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

// Bundle is a FHIR R5 Bundle resource.
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

// BundleLink is a link in a Bundle.
type BundleLink struct {
	Relation string `json:"relation"`
	URL      string `json:"url"`
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

// OperationOutcome is a FHIR R5 OperationOutcome resource.
type OperationOutcome struct {
	ResourceType string                  `json:"resourceType"`
	ID           string                  `json:"id,omitempty"`
	Meta         *Meta                   `json:"meta,omitempty"`
	Text         *Narrative              `json:"text,omitempty"`
	Issue        []OperationOutcomeIssue `json:"issue"`
}

// ---------------------------------------------------------------------------
// ImagingStudy
// ---------------------------------------------------------------------------

// ImagingStudySeriesInstance is a single instance (image) in a series.
type ImagingStudySeriesInstance struct {
	UID           string  `json:"uid"`
	SopClass      Coding  `json:"sopClass"`
	Number        int     `json:"number,omitempty"`
	Title         string  `json:"title,omitempty"`
}

// ImagingStudySeriesPerformer is a performer of a series.
type ImagingStudySeriesPerformer struct {
	Function *CodeableConcept `json:"function,omitempty"`
	Actor    *Reference       `json:"actor"`
}

// ImagingStudySeries represents a series of images in an imaging study.
type ImagingStudySeries struct {
	UID                 string                          `json:"uid"`
	Number              int                             `json:"number,omitempty"`
	Modality            Coding                          `json:"modality"`
	Description         string                          `json:"description,omitempty"`
	Started             *time.Time                      `json:"started,omitempty"`
	Performer           []ImagingStudySeriesPerformer   `json:"performer,omitempty"`
	BodySite            *Coding                         `json:"bodySite,omitempty"`
	Laterality          *Coding                         `json:"laterality,omitempty"`
	Specimen            []Reference                     `json:"specimen,omitempty"`
	StartedWithContrast *bool                           `json:"startedWithContrast,omitempty"`
	Instance            []ImagingStudySeriesInstance    `json:"instance,omitempty"`
}

// ImagingStudy is a FHIR R5 ImagingStudy resource.
type ImagingStudy struct {
	ResourceType      string                `json:"resourceType"`
	ID                string                `json:"id,omitempty"`
	Meta              *Meta                 `json:"meta,omitempty"`
	Text              *Narrative            `json:"text,omitempty"`
	Extension         []Extension           `json:"extension,omitempty"`
	Identifier        []Identifier          `json:"identifier,omitempty"`
	Status            string                `json:"status"`
	Modality          []Coding              `json:"modality,omitempty"`
	Subject           *Reference            `json:"subject"`
	Encounter         *Reference            `json:"encounter,omitempty"`
	Started           *time.Time            `json:"started,omitempty"`
	BasedOn           []Reference           `json:"basedOn,omitempty"`
	Referrer          *Reference            `json:"referrer,omitempty"`
	Interpreter       []Reference           `json:"interpreter,omitempty"`
	Endpoint          []Reference           `json:"endpoint,omitempty"`
	NumberOfSeries    int                   `json:"numberOfSeries,omitempty"`
	NumberOfInstances int                   `json:"numberOfInstances,omitempty"`
	Procedure         []CodeableConcept     `json:"procedure,omitempty"`
	Location          *Reference            `json:"location,omitempty"`
	ReasonCode        []CodeableConcept     `json:"reasonCode,omitempty"`
	Note              []Annotation          `json:"note,omitempty"`
	Series            []ImagingStudySeries  `json:"series,omitempty"`
}

// ---------------------------------------------------------------------------
// Subscription
// ---------------------------------------------------------------------------

// SubscriptionFilterBy is a filter criterion for a subscription.
type SubscriptionFilterBy struct {
	ResourceType string  `json:"resourceType,omitempty"`
	FilterParameter string `json:"filterParameter"`
	Value          string `json:"value"`
	Modifier       string `json:"modifier,omitempty"`
}

// SubscriptionChannel is the channel configuration for a subscription.
type SubscriptionChannel struct {
	Type       string `json:"type"`
	Endpoint   string `json:"endpoint,omitempty"`
	Payload    string `json:"payload,omitempty"`
	Header     []string `json:"header,omitempty"`
}

// Subscription is a FHIR R5 Subscription resource.
type Subscription struct {
	ResourceType string                     `json:"resourceType"`
	ID           string                     `json:"id,omitempty"`
	Meta         *Meta                      `json:"meta,omitempty"`
	Text         *Narrative                 `json:"text,omitempty"`
	Extension    []Extension                `json:"extension,omitempty"`
	Identifier   []Identifier               `json:"identifier,omitempty"`
	Name         string                     `json:"name,omitempty"`
	Status       string                     `json:"status"`
	Topic        string                     `json:"topic"` // canonical reference to SubscriptionTopic
	Contact      []ContactPoint             `json:"contact,omitempty"`
	End          *time.Time                 `json:"end,omitempty"`
	ManagingEntity *Reference               `json:"managingEntity,omitempty"`
	Reason       string                     `json:"reason"`
	FilterBy     []SubscriptionFilterBy     `json:"filterBy,omitempty"`
	ChannelType  Coding                     `json:"channelType"`
	Channel      SubscriptionChannel        `json:"channel"`
	MaxCount     int                        `json:"maxCount,omitempty"`
	Timeout      int                        `json:"timeout,omitempty"`
	ContentType  string                     `json:"contentType,omitempty"`
	HeartbeatPeriod int                    `json:"heartbeatPeriod,omitempty"`
}

// ---------------------------------------------------------------------------
// SubscriptionTopic
// ---------------------------------------------------------------------------

// SubscriptionTopicResourceTrigger defines a resource-based trigger.
type SubscriptionTopicResourceTrigger struct {
	ResourceType string            `json:"resourceType"`
	MethodCriteria []string        `json:"methodCriteria,omitempty"`
	QueryCriteria  *SubscriptionTopicQueryCriteria `json:"queryCriteria,omitempty"`
	FhirPathCriteria string        `json:"fhirPathCriteria,omitempty"`
}

// SubscriptionTopicQueryCriteria defines before/after query criteria.
type SubscriptionTopicQueryCriteria struct {
	Previous string `json:"previous,omitempty"`
	ResultForCreate string `json:"resultForCreate,omitempty"`
	ResultForDelete string `json:"resultForDelete,omitempty"`
}

// SubscriptionTopicEventTrigger defines an event-based trigger.
type SubscriptionTopicEventTrigger struct {
	Description string `json:"description"`
	Event       CodeableConcept `json:"event"`
	ResourceType string `json:"resourceType"`
}

// SubscriptionTopicCanFilterBy defines allowable filter parameters.
type SubscriptionTopicCanFilterBy struct {
	ResourceType string   `json:"resourceType,omitempty"`
	FilterParameter string `json:"filterParameter"`
	Modifier      []string `json:"modifier,omitempty"`
	ModifierExtension []Extension `json:"modifierExtension,omitempty"`
}

// SubscriptionTopic is a FHIR R5 SubscriptionTopic resource.
type SubscriptionTopic struct {
	ResourceType     string                              `json:"resourceType"`
	ID               string                              `json:"id,omitempty"`
	Meta             *Meta                               `json:"meta,omitempty"`
	Text             *Narrative                          `json:"text,omitempty"`
	Extension        []Extension                         `json:"extension,omitempty"`
	Identifier       []Identifier                        `json:"identifier,omitempty"`
	URL              string                              `json:"url"`
	Version          string                              `json:"version,omitempty"`
	Title            string                              `json:"title,omitempty"`
	DerivedFrom      []string                            `json:"derivedFrom,omitempty"`
	Status           string                              `json:"status"`
	Experimental     *bool                               `json:"experimental,omitempty"`
	Date             *time.Time                          `json:"date,omitempty"`
	Publisher        string                              `json:"publisher,omitempty"`
	Contact          []ContactPoint                      `json:"contact,omitempty"`
	Description      string                              `json:"description,omitempty"`
	ResourceTrigger  []SubscriptionTopicResourceTrigger  `json:"resourceTrigger,omitempty"`
	EventTrigger     []SubscriptionTopicEventTrigger     `json:"eventTrigger,omitempty"`
	CanFilterBy      []SubscriptionTopicCanFilterBy      `json:"canFilterBy,omitempty"`
	NotifyType       []string                            `json:"notifyType,omitempty"`
}
