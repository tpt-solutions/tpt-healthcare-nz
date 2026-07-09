// Package homevisit implements home visit scheduling and documentation
// for NZ community health practice.
package homevisit

import (
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// VisitType categorises the type of home visit.
type VisitType string

const (
	VisitWoundCare        VisitType = "wound_care"
	VisitMedicationReview VisitType = "medication_review"
	VisitPostAcute        VisitType = "post_acute"
	VisitPalliative       VisitType = "palliative"
	VisitAssessment       VisitType = "assessment"
	VisitFollowUp         VisitType = "follow_up"
	VisitDiabetesCare     VisitType = "diabetes_care"
	VisitRespiratory      VisitType = "respiratory"
	VisitRehabilitation   VisitType = "rehabilitation"
	VisitPostnatal        VisitType = "postnatal"
)

// VisitStatus tracks the lifecycle of a home visit.
type VisitStatus string

const (
	VisitScheduled   VisitStatus = "scheduled"
	VisitInTransit   VisitStatus = "in_transit"
	VisitArrived     VisitStatus = "arrived"
	VisitInProgress  VisitStatus = "in_progress"
	VisitCompleted   VisitStatus = "completed"
	VisitCancelled   VisitStatus = "cancelled"
	VisitRescheduled VisitStatus = "rescheduled"
	VisitNoShow      VisitStatus = "no_show"
)

// Priority indicates urgency level of a visit.
type Priority string

const (
	PriorityUrgent  Priority = "urgent"
	PriorityHigh    Priority = "high"
	PriorityRoutine Priority = "routine"
	PriorityLow     Priority = "low"
)

// TransportMode indicates how the clinician travels.
type TransportMode string

const (
	TransportCar            TransportMode = "car"
	TransportBike           TransportMode = "bike"
	TransportPublic         TransportMode = "public_transport"
	TransportWalking        TransportMode = "walking"
	TransportCompanyVehicle TransportMode = "company_vehicle"
)

// HomeVisit represents a scheduled home visit.
type HomeVisit struct {
	ID                 string        `json:"id"`
	PatientNHI         string        `json:"patientNhi"`
	ClinicianID        string        `json:"clinicianId"`
	PracticeID         string        `json:"practiceId"`
	ScheduledDate      int64         `json:"scheduledDate"`
	EstimatedDuration  int           `json:"estimatedDurationMinutes"`
	ActualStartTime    int64         `json:"actualStartTime,omitempty"`
	ActualEndTime      int64         `json:"actualEndTime,omitempty"`
	VisitType          VisitType     `json:"visitType"`
	Priority           Priority      `json:"priority"`
	Status             VisitStatus   `json:"status"`
	Address            string        `json:"address"`
	Latitude           float64       `json:"latitude,omitempty"`
	Longitude          float64       `json:"longitude,omitempty"`
	ContactPhone       string        `json:"contactPhone,omitempty"`
	ContactName        string        `json:"contactName,omitempty"`
	AccessInstructions string        `json:"accessInstructions,omitempty"`
	SafetyNotes        string        `json:"safetyNotes,omitempty"`
	TransportMode      TransportMode `json:"transportMode,omitempty"`
	RouteOrder         int           `json:"routeOrder,omitempty"`
	PreviousVisitID    string        `json:"previousVisitId,omitempty"`
	CancellationReason string        `json:"cancellationReason,omitempty"`
	CancellationNotes  string        `json:"cancellationNotes,omitempty"`
	CreatedAt          int64         `json:"createdAt"`
	UpdatedAt          int64         `json:"updatedAt"`
}

// HomeVisitNote represents a clinical note recorded during a home visit.
type HomeVisitNote struct {
	ID               string   `json:"id"`
	HomeVisitID      string   `json:"homeVisitId"`
	PatientNHI       string   `json:"patientNhi"`
	ClinicianID      string   `json:"clinicianId"`
	NoteType         NoteType `json:"noteType"`
	Narrative        string   `json:"narrative"`
	Concerns         []string `json:"concerns,omitempty"`
	Actions          []string `json:"actions,omitempty"`
	FollowUpRequired bool     `json:"followUpRequired"`
	FollowUpDetails  string   `json:"followUpDetails,omitempty"`
	CreatedAt        int64    `json:"createdAt"`
	UpdatedAt        int64    `json:"updatedAt"`
}

// NoteType categorises the type of clinical note.
type NoteType string

const (
	NoteSubjective    NoteType = "subjective"
	NoteObjective     NoteType = "objective"
	NoteAssessment    NoteType = "assessment"
	NotePlan          NoteType = "plan"
	NoteSupplementary NoteType = "supplementary"
)

// VisitOutcome represents an outcome recorded for a visit.
type VisitOutcome struct {
	ID                 string          `json:"id"`
	HomeVisitID        string          `json:"homeVisitId"`
	PatientNHI         string          `json:"patientNhi"`
	OutcomeCategory    OutcomeCategory `json:"outcomeCategory"`
	OutcomeDescription string          `json:"outcomeDescription"`
	OutcomeScore       *int            `json:"outcomeScore,omitempty"`
	Achieved           *bool           `json:"achieved,omitempty"`
	ReviewDate         int64           `json:"reviewDate,omitempty"`
	CreatedAt          int64           `json:"createdAt"`
}

// OutcomeCategory groups visit outcomes.
type OutcomeCategory string

const (
	OutcomeWoundHealing          OutcomeCategory = "wound_healing"
	OutcomeMedicationAdherence   OutcomeCategory = "medication_adherence"
	OutcomeFunctionalImprovement OutcomeCategory = "functional_improvement"
	OutcomeSafety                OutcomeCategory = "safety"
	OutcomeReferral              OutcomeCategory = "referral"
	OutcomeEducation             OutcomeCategory = "education"
)

// SafetyCheck represents a home safety assessment during a visit.
type SafetyCheck struct {
	ID                  string   `json:"id"`
	HomeVisitID         string   `json:"homeVisitId"`
	PatientNHI          string   `json:"patientNhi"`
	CheckedBy           string   `json:"checkedBy"`
	CheckDate           int64    `json:"checkDate"`
	FallRisk            int      `json:"fallRisk"`
	PressureInjuryRisk  int      `json:"pressureInjuryRisk"`
	FireSafetyOK        bool     `json:"fireSafetyOk"`
	SmokeAlarmsOK       bool     `json:"smokeAlarmsOk"`
	MedicationStorageOK bool     `json:"medicationStorageOk"`
	TrippingHazards     []string `json:"trippingHazards,omitempty"`
	Recommendations     []string `json:"recommendations,omitempty"`
	ActionsTaken        []string `json:"actionsTaken,omitempty"`
	NextCheckDate       int64    `json:"nextCheckDate,omitempty"`
	CreatedAt           int64    `json:"createdAt"`
}

// EquipmentCheck represents equipment verification during a home visit.
type EquipmentCheck struct {
	ID            string          `json:"id"`
	HomeVisitID   string          `json:"homeVisitId"`
	EquipmentName string          `json:"equipmentName"`
	SerialNumber  string          `json:"serialNumber,omitempty"`
	CheckedAt     int64           `json:"checkedAt"`
	Status        EquipmentStatus `json:"status"`
	Notes         string          `json:"notes,omitempty"`
	CreatedAt     int64           `json:"createdAt"`
	UpdatedAt     int64           `json:"updatedAt"`
}

// EquipmentStatus tracks equipment condition.
type EquipmentStatus string

const (
	EquipmentFunctioning  EquipmentStatus = "functioning"
	EquipmentNeedsService EquipmentStatus = "needs_service"
	EquipmentBroken       EquipmentStatus = "broken"
	EquipmentMissing      EquipmentStatus = "missing"
)

// NewHomeVisit creates a new home visit with defaults.
func NewHomeVisit() *HomeVisit {
	now := time.Now().UnixMilli()
	return &HomeVisit{
		Status:            VisitScheduled,
		Priority:          PriorityRoutine,
		EstimatedDuration: 30,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// Validate checks required fields for a home visit.
func (v *HomeVisit) Validate() error {
	if v.PatientNHI == "" {
		return fmt.Errorf("homevisit: patient NHI is required")
	}
	if !nhi.ValidateNHI(v.PatientNHI) {
		return fmt.Errorf("homevisit: invalid patient NHI: %s", v.PatientNHI)
	}
	if v.ClinicianID == "" {
		return fmt.Errorf("homevisit: clinician ID is required")
	}
	if v.ScheduledDate == 0 {
		return fmt.Errorf("homevisit: scheduled date is required")
	}
	if v.Address == "" {
		return fmt.Errorf("homevisit: address is required")
	}
	if v.PracticeID == "" {
		return fmt.Errorf("homevisit: practice ID is required")
	}
	return nil
}

// NewHomeVisitNote creates a new note with defaults.
func NewHomeVisitNote() *HomeVisitNote {
	now := time.Now().UnixMilli()
	return &HomeVisitNote{
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks required fields for a home visit note.
func (n *HomeVisitNote) Validate() error {
	if n.HomeVisitID == "" {
		return fmt.Errorf("homevisit: home visit ID is required")
	}
	if n.PatientNHI == "" {
		return fmt.Errorf("homevisit: patient NHI is required")
	}
	if !nhi.ValidateNHI(n.PatientNHI) {
		return fmt.Errorf("homevisit: invalid patient NHI: %s", n.PatientNHI)
	}
	if n.Narrative == "" {
		return fmt.Errorf("homevisit: narrative is required")
	}
	return nil
}

// NewSafetyCheck creates a new safety check with defaults.
func NewSafetyCheck() *SafetyCheck {
	now := time.Now().UnixMilli()
	return &SafetyCheck{
		FallRisk:            0,
		PressureInjuryRisk:  0,
		FireSafetyOK:        true,
		SmokeAlarmsOK:       true,
		MedicationStorageOK: true,
		CheckDate:           now,
		CreatedAt:           now,
	}
}

// Validate checks required fields for a safety check.
func (s *SafetyCheck) Validate() error {
	if s.HomeVisitID == "" {
		return fmt.Errorf("homevisit: home visit ID is required")
	}
	if s.PatientNHI == "" {
		return fmt.Errorf("homevisit: patient NHI is required")
	}
	if !nhi.ValidateNHI(s.PatientNHI) {
		return fmt.Errorf("homevisit: invalid patient NHI: %s", s.PatientNHI)
	}
	if s.CheckedBy == "" {
		return fmt.Errorf("homevisit: checked by is required")
	}
	return nil
}

// NewEquipmentCheck creates a new equipment check with defaults.
func NewEquipmentCheck() *EquipmentCheck {
	now := time.Now().UnixMilli()
	return &EquipmentCheck{
		Status:    EquipmentFunctioning,
		CheckedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks required fields.
func (e *EquipmentCheck) Validate() error {
	if e.HomeVisitID == "" {
		return fmt.Errorf("homevisit: home visit ID is required")
	}
	if e.EquipmentName == "" {
		return fmt.Errorf("homevisit: equipment name is required")
	}
	return nil
}
