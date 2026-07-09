package r4

// Encounter represents a FHIR R4 Encounter resource for backwards compatibility.
type Encounter struct {
	ResourceType    string              `json:"resourceType"`
	ID              string              `json:"id,omitempty"`
	Meta            *Meta               `json:"meta,omitempty"`
	Identifier      []Identifier        `json:"identifier,omitempty"`
	Status          string              `json:"status"` // planned, arrived, in-progress, onleave, finished, cancelled
	Class           Coding              `json:"class"`   // AMI value set
	Type            []CodeableConcept   `json:"type,omitempty"`
	ServiceType     *CodeableConcept    `json:"serviceType,omitempty"`
	Priority        *CodeableConcept    `json:"priority,omitempty"`
	Subject         *Reference          `json:"subject"`
	EpisodeOfCare   []Reference         `json:"episodeOfCare,omitempty"`
	Participant     []EncounterParticipant `json:"participant,omitempty"`
	Period          *Period             `json:"period,omitempty"`
	ReasonCode      []CodeableConcept   `json:"reasonCode,omitempty"`
	Diagnosis       []EncounterDiagnosis `json:"diagnosis,omitempty"`
	Location        []EncounterLocation  `json:"location,omitempty"`
	ServiceProvider *Reference           `json:"serviceProvider,omitempty"`
}

// EncounterParticipant records a clinician's involvement.
type EncounterParticipant struct {
	Type     []CodeableConcept `json:"type,omitempty"`
	Period   *Period           `json:"period,omitempty"`
	Individual *Reference      `json:"individual,omitempty"`
}

// EncounterDiagnosis links a diagnosis to the encounter.
type EncounterDiagnosis struct {
	Condition *Reference        `json:"condition"`
	Use       []CodeableConcept `json:"use,omitempty"`
	Rank      *int              `json:"rank,omitempty"`
}

// EncounterLocation tracks where the patient is/was.
type EncounterLocation struct {
	Location   *Reference  `json:"location"`
	Status     string      `json:"status,omitempty"` // planned, active, reserved
	Period     *Period     `json:"period,omitempty"`
}

// NewEncounter creates a new FHIR R4 Encounter.
func NewEncounter(id, status string, class Coding, subjectRef string) Encounter {
	return Encounter{
		ResourceType: "Encounter",
		ID:           id,
		Status:       status,
		Class:        class,
		Subject:      &Reference{Reference: subjectRef},
	}
}
