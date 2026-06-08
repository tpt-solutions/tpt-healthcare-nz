// Package outreach implements community outreach program management
// for NZ community health practice.
package outreach

import (
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// ProgramType categorises outreach programs.
type ProgramType string

const (
	ProgramMobileClinic        ProgramType = "mobile_clinic"
	ProgramVaccination         ProgramType = "vaccination"
	ProgramScreening           ProgramType = "screening"
	ProgramHealthPromotion     ProgramType = "health_promotion"
	ProgramChronicDisease      ProgramType = "chronic_disease_support"
)

// ProgramStatus tracks program lifecycle.
type ProgramStatus string

const (
	ProgramActive       ProgramStatus = "active"
	ProgramPaused       ProgramStatus = "paused"
	ProgramCompleted    ProgramStatus = "completed"
	ProgramDiscontinued ProgramStatus = "discontinued"
)

// EventType categorises outreach events.
type EventType string

const (
	EventClinic       EventType = "clinic"
	EventScreening    EventType = "screening"
	EventEducation    EventType = "education"
	EventVaccination  EventType = "vaccination"
)

// EventStatus tracks event lifecycle.
type EventStatus string

const (
	EventPlanned     EventStatus = "planned"
	EventConfirmed   EventStatus = "confirmed"
	EventInProgress  EventStatus = "in_progress"
	EventCompleted   EventStatus = "completed"
	EventCancelled   EventStatus = "cancelled"
)

// AttendeeType categorises outreach attendees.
type AttendeeType string

const (
	AttendeePatient         AttendeeType = "patient"
	AttendeeCarer           AttendeeType = "carer"
	AttendeeCommunityMember AttendeeType = "community_member"
	AttendeeStaff           AttendeeType = "staff"
)

// ReferralType categorises outreach referrals.
type ReferralType string

const (
	ReferralGP           ReferralType = "gp"
	ReferralSpecialist   ReferralType = "specialist"
	ReferralMentalHealth ReferralType = "mental_health"
	ReferralSocial       ReferralType = "social_services"
	ReferralHousing      ReferralType = "housing"
	ReferralOther        ReferralType = "other"
)

// ScreeningType categorises health screenings offered at outreach events.
type ScreeningType string

const (
	ScreeningBloodPressure ScreeningType = "blood_pressure"
	ScreeningDiabetes      ScreeningType = "diabetes"
	ScreeningCervical      ScreeningType = "cervical"
	ScreeningBowel         ScreeningType = "bowel"
	ScreeningHearing       ScreeningType = "hearing"
	ScreeningVision        ScreeningType = "vision"
)

// ResultCategory categorises screening results.
type ResultCategory string

const (
	ResultNormal        ResultCategory = "normal"
	ResultAbnormal      ResultCategory = "abnormal"
	ResultBorderline    ResultCategory = "borderline"
	ResultInconclusive  ResultCategory = "inconclusive"
)

// Program represents an ongoing community outreach program.
type Program struct {
	ID               string        `json:"id"`
	PracticeID       string        `json:"practiceId"`
	ProgramName      string        `json:"programName"`
	ProgramType      ProgramType   `json:"programType"`
	Description      string        `json:"description,omitempty"`
	TargetPopulation string        `json:"targetPopulation,omitempty"`
	Status           ProgramStatus `json:"status"`
	StartDate        int64         `json:"startDate"`
	EndDate          int64         `json:"endDate,omitempty"`
	FundingSource    string        `json:"fundingSource,omitempty"`
	FundingCode      string        `json:"fundingCode,omitempty"`
	Budget           float64       `json:"budget,omitempty"`
	Spent            float64       `json:"spent,omitempty"`
	CreatedAt        int64         `json:"createdAt"`
	UpdatedAt        int64         `json:"updatedAt"`
}

// Event represents a single outreach event (clinic, screening session).
type Event struct {
	ID                   string     `json:"id"`
	ProgramID            string     `json:"programId"`
	EventName            string     `json:"eventName"`
	EventType            EventType  `json:"eventType"`
	ScheduledDate        int64      `json:"scheduledDate"`
	EstimatedDuration    int        `json:"estimatedDurationMinutes"`
	LocationAddress      string     `json:"locationAddress"`
	Latitude             float64    `json:"latitude,omitempty"`
	Longitude            float64    `json:"longitude,omitempty"`
	VenueName            string     `json:"venueName,omitempty"`
	VenueContact         string     `json:"venueContact,omitempty"`
	TargetAttendees      int        `json:"targetAttendees,omitempty"`
	ActualAttendees      int        `json:"actualAttendees,omitempty"`
	Clinicians           []string   `json:"clinicians"`
	EquipmentList        []string   `json:"equipmentList,omitempty"`
	Status               EventStatus `json:"status"`
	CancellationReason   string     `json:"cancellationReason,omitempty"`
	Notes                string     `json:"notes,omitempty"`
	CreatedAt            int64      `json:"createdAt"`
	UpdatedAt            int64      `json:"updatedAt"`
}

// Attendee represents an individual who attended an outreach event.
type Attendee struct {
	ID                 string     `json:"id"`
	EventID            string     `json:"eventId"`
	PatientNHI         string     `json:"patientNhi,omitempty"`
	AttendeeName       string     `json:"attendeeName"`
	AttendeeType       AttendeeType `json:"attendeeType"`
	ContactPhone       string     `json:"contactPhone,omitempty"`
	ContactEmail       string     `json:"contactEmail,omitempty"`
	Demographics       map[string]string `json:"demographics,omitempty"`
	NHIProvided        bool       `json:"nhiProvided"`
	RegistrationMethod string     `json:"registrationMethod,omitempty"`
	AttendedAt         int64      `json:"attendedAt,omitempty"`
	ServicesReceived   []string   `json:"servicesReceived,omitempty"`
	ConsentGiven       bool       `json:"consentGiven"`
	FollowUpRequired   bool       `json:"followUpRequired"`
	FollowUpDetails    string     `json:"followUpDetails,omitempty"`
	CreatedAt          int64      `json:"createdAt"`
	UpdatedAt          int64      `json:"updatedAt"`
}

// Referral represents a referral made during an outreach event.
type Referral struct {
	ID              string       `json:"id"`
	EventID         string       `json:"eventId"`
	PatientNHI      string       `json:"patientNhi"`
	ReferredBy      string       `json:"referredBy"`
	ReferralDate    int64        `json:"referralDate"`
	ReferralType    ReferralType `json:"referralType"`
	ReferralReason  string       `json:"referralReason"`
	Urgency         string       `json:"urgency"`
	Status          string       `json:"status"`
	Outcome         string       `json:"outcome,omitempty"`
	OutcomeDate     int64        `json:"outcomeDate,omitempty"`
	CreatedAt       int64        `json:"createdAt"`
	UpdatedAt       int64        `json:"updatedAt"`
}

// Screening represents a health screening conducted at an outreach event.
type Screening struct {
	ID                 string         `json:"id"`
	EventID            string         `json:"eventId"`
	PatientNHI         string         `json:"patientNhi"`
	ClinicianID        string         `json:"clinicianId"`
	ScreeningType      ScreeningType  `json:"screeningType"`
	ScreeningDate      int64          `json:"screeningDate"`
	ResultCategory     ResultCategory `json:"resultCategory"`
	ResultValue        string         `json:"resultValue,omitempty"`
	Interpretation     string         `json:"interpretation,omitempty"`
	ConsentGiven       bool           `json:"consentGiven"`
	FollowUpRequired   bool           `json:"followUpRequired"`
	FollowUpDetails    string         `json:"followUpDetails,omitempty"`
	ReferralID         string         `json:"referralId,omitempty"`
	CreatedAt          int64          `json:"createdAt"`
	UpdatedAt          int64          `json:"updatedAt"`
}

// NewProgram creates a new outreach program with defaults.
func NewProgram() *Program {
	now := time.Now().UnixMilli()
	return &Program{
		Status:    ProgramActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks required fields for a program.
func (p *Program) Validate() error {
	if p.PracticeID == "" {
		return fmt.Errorf("outreach: practice ID is required")
	}
	if p.ProgramName == "" {
		return fmt.Errorf("outreach: program name is required")
	}
	if p.ProgramType == "" {
		return fmt.Errorf("outreach: program type is required")
	}
	if p.StartDate == 0 {
		return fmt.Errorf("outreach: start date is required")
	}
	return nil
}

// NewEvent creates a new outreach event with defaults.
func NewEvent() *Event {
	now := time.Now().UnixMilli()
	return &Event{
		Status:               EventPlanned,
		EstimatedDuration:    120,
		Clinicians:           []string{},
		EquipmentList:        []string{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

// Validate checks required fields for an event.
func (e *Event) Validate() error {
	if e.ProgramID == "" {
		return fmt.Errorf("outreach: program ID is required")
	}
	if e.EventName == "" {
		return fmt.Errorf("outreach: event name is required")
	}
	if e.EventType == "" {
		return fmt.Errorf("outreach: event type is required")
	}
	if e.ScheduledDate == 0 {
		return fmt.Errorf("outreach: scheduled date is required")
	}
	if e.LocationAddress == "" {
		return fmt.Errorf("outreach: location address is required")
	}
	if len(e.Clinicians) == 0 {
		return fmt.Errorf("outreach: at least one clinician is required")
	}
	return nil
}

// NewAttendee creates a new attendee with defaults.
func NewAttendee() *Attendee {
	now := time.Now().UnixMilli()
	return &Attendee{
		AttendeeType:   AttendeePatient,
		ConsentGiven:   false,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Validate checks required fields for an attendee.
func (a *Attendee) Validate() error {
	if a.EventID == "" {
		return fmt.Errorf("outreach: event ID is required")
	}
	if a.AttendeeName == "" {
		return fmt.Errorf("outreach: attendee name is required")
	}
	if a.PatientNHI != "" && !nhi.ValidateNHI(a.PatientNHI) {
		return fmt.Errorf("outreach: invalid patient NHI: %s", a.PatientNHI)
	}
	return nil
}

// NewReferral creates a new referral with defaults.
func NewReferral() *Referral {
	now := time.Now().UnixMilli()
	return &Referral{
		ReferralDate: now,
		Urgency:      "routine",
		Status:       "pending",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Validate checks required fields for a referral.
func (r *Referral) Validate() error {
	if r.EventID == "" {
		return fmt.Errorf("outreach: event ID is required")
	}
	if r.PatientNHI == "" {
		return fmt.Errorf("outreach: patient NHI is required")
	}
	if !nhi.ValidateNHI(r.PatientNHI) {
		return fmt.Errorf("outreach: invalid patient NHI: %s", r.PatientNHI)
	}
	if r.ReferredBy == "" {
		return fmt.Errorf("outreach: referred by is required")
	}
	if r.ReferralType == "" {
		return fmt.Errorf("outreach: referral type is required")
	}
	if r.ReferralReason == "" {
		return fmt.Errorf("outreach: referral reason is required")
	}
	return nil
}

// NewScreening creates a new screening with defaults.
func NewScreening() *Screening {
	now := time.Now().UnixMilli()
	return &Screening{
		ScreeningDate: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Validate checks required fields for a screening.
func (s *Screening) Validate() error {
	if s.EventID == "" {
		return fmt.Errorf("outreach: event ID is required")
	}
	if s.PatientNHI == "" {
		return fmt.Errorf("outreach: patient NHI is required")
	}
	if !nhi.ValidateNHI(s.PatientNHI) {
		return fmt.Errorf("outreach: invalid patient NHI: %s", s.PatientNHI)
	}
	if s.ClinicianID == "" {
		return fmt.Errorf("outreach: clinician ID is required")
	}
	if s.ScreeningType == "" {
		return fmt.Errorf("outreach: screening type is required")
	}
	return nil
}
