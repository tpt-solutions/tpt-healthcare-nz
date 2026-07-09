// Package ot implements occupational therapy assessment and intervention planning
// for NZ occupational therapy practice, including ACC-funded treatment.
package ot

import (
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// AssessmentType categorises the type of OT assessment.
type AssessmentType string

const (
	AssessmentFunctionalCapacity AssessmentType = "functional_capacity"
	AssessmentADL                AssessmentType = "adl"  // Activities of Daily Living
	AssessmentIADL               AssessmentType = "iadl" // Instrumental ADL
	AssessmentHomeSafety         AssessmentType = "home_safety"
	AssessmentWorksite           AssessmentType = "worksite"
	AssessmentCognitive          AssessmentType = "cognitive"
	AssessmentSensory            AssessmentType = "sensory"
	AssessmentPaediatric         AssessmentType = "paediatric"
	AssessmentDriving            AssessmentType = "driving"
	AssessmentWheelchair         AssessmentType = "wheelchair_seating"
	AssessmentAssistiveTech      AssessmentType = "assistive_technology"
	AssessmentVocational         AssessmentType = "vocational"
)

// InterventionType categorises OT interventions.
type InterventionType string

const (
	InterventionADLRetraining         InterventionType = "adl_retraining"
	InterventionCognitiveRehab        InterventionType = "cognitive_rehab"
	InterventionSensoryIntegration    InterventionType = "sensory_integration"
	InterventionHomeModification      InterventionType = "home_modification"
	InterventionEquipmentPrescription InterventionType = "equipment_prescription"
	InterventionWorkHardening         InterventionType = "work_hardening"
	InterventionErgonomicAssessment   InterventionType = "ergonomic_assessment"
	InterventionSplinting             InterventionType = "splinting"
	InterventionEnergyConservation    InterventionType = "energy_conservation"
	InterventionFallsPrevention       InterventionType = "falls_prevention"
	InterventionDriverRehab           InterventionType = "driver_rehab"
	InterventionPaediatricPlay        InterventionType = "paediatric_play"
)

// Assessment represents an OT assessment.
type Assessment struct {
	ID              string           `json:"id"`
	PatientNHI      string           `json:"patientNhi"`
	ClinicianID     string           `json:"clinicianId"`
	PracticeID      string           `json:"practiceId"`
	ACCNumber       string           `json:"accNumber,omitempty"`
	ReferralSource  string           `json:"referralSource"`
	Type            AssessmentType   `json:"type"`
	Date            int64            `json:"date"`
	Reason          string           `json:"reason"`
	Findings        string           `json:"findings"`
	Recommendations []Recommendation `json:"recommendations"`
	OutcomeMeasures []OutcomeMeasure `json:"outcomeMeasures"`
	Status          AssessmentStatus `json:"status"`
	CreatedAt       int64            `json:"createdAt"`
	UpdatedAt       int64            `json:"updatedAt"`
}

// AssessmentStatus tracks assessment lifecycle.
type AssessmentStatus string

const (
	AssessmentScheduled  AssessmentStatus = "scheduled"
	AssessmentInProgress AssessmentStatus = "in_progress"
	AssessmentCompleted  AssessmentStatus = "completed"
	AssessmentCancelled  AssessmentStatus = "cancelled"
	AssessmentOnHold     AssessmentStatus = "on_hold"
)

// Recommendation represents an OT recommendation.
type Recommendation struct {
	ID            string                 `json:"id"`
	Description   string                 `json:"description"`
	Priority      RecommendationPriority `json:"priority"`
	Type          InterventionType       `json:"type,omitempty"`
	Equipment     string                 `json:"equipment,omitempty"`
	Supplier      string                 `json:"supplier,omitempty"`
	EstimatedCost float64                `json:"estimatedCost,omitempty"`
	FundingSource string                 `json:"fundingSource,omitempty"` // ACC, MOH, private, charity
	Status        RecommendationStatus   `json:"status"`
	CreatedAt     int64                  `json:"createdAt"`
	UpdatedAt     int64                  `json:"updatedAt"`
}

// RecommendationPriority indicates urgency.
type RecommendationPriority string

const (
	PriorityUrgent  RecommendationPriority = "urgent"
	PriorityHigh    RecommendationPriority = "high"
	PriorityMedium  RecommendationPriority = "medium"
	PriorityLow     RecommendationPriority = "low"
	PriorityRoutine RecommendationPriority = "routine"
)

// RecommendationStatus tracks recommendation implementation.
type RecommendationStatus string

const (
	RecommendationPending   RecommendationStatus = "pending"
	RecommendationApproved  RecommendationStatus = "approved"
	RecommendationDeclined  RecommendationStatus = "declined"
	RecommendationOrdered   RecommendationStatus = "ordered"
	RecommendationDelivered RecommendationStatus = "delivered"
	RecommendationCompleted RecommendationStatus = "completed"
)

// OutcomeMeasure represents a standardised OT outcome measure.
type OutcomeMeasure struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`   // e.g., "COPM", "FIM", "AMPS", "MOHOST"
	Domain         string  `json:"domain"` // e.g., "performance", "satisfaction"
	Score          float64 `json:"score"`
	MaxScore       float64 `json:"maxScore"`
	Date           int64   `json:"date"`
	Interpretation string  `json:"interpretation,omitempty"`
	CreatedAt      int64   `json:"createdAt"`
}

// InterventionPlan represents an OT intervention plan.
type InterventionPlan struct {
	ID            string                `json:"id"`
	PatientNHI    string                `json:"patientNhi"`
	ClinicianID   string                `json:"clinicianId"`
	PracticeID    string                `json:"practiceId"`
	AssessmentID  string                `json:"assessmentId,omitempty"`
	ACCNumber     string                `json:"accNumber,omitempty"`
	StartDate     int64                 `json:"startDate"`
	ReviewDate    int64                 `json:"reviewDate"`
	EndDate       int64                 `json:"endDate,omitempty"`
	Status        PlanStatus            `json:"status"`
	Goals         []InterventionGoal    `json:"goals"`
	Interventions []PlannedIntervention `json:"interventions"`
	CreatedAt     int64                 `json:"createdAt"`
	UpdatedAt     int64                 `json:"updatedAt"`
}

// PlanStatus tracks intervention plan lifecycle.
type PlanStatus string

const (
	PlanStatusDraft        PlanStatus = "draft"
	PlanStatusActive       PlanStatus = "active"
	PlanStatusUnderReview  PlanStatus = "under_review"
	PlanStatusCompleted    PlanStatus = "completed"
	PlanStatusDiscontinued PlanStatus = "discontinued"
	PlanStatusOnHold       PlanStatus = "on_hold"
)

// InterventionGoal represents a goal in the intervention plan.
type InterventionGoal struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Domain      string     `json:"domain"` // e.g., "self_care", "productivity", "leisure"
	TargetDate  int64      `json:"targetDate"`
	Status      GoalStatus `json:"status"`
	Outcome     string     `json:"outcome,omitempty"`
	CreatedAt   int64      `json:"createdAt"`
	UpdatedAt   int64      `json:"updatedAt"`
}

// GoalStatus tracks goal progress.
type GoalStatus string

const (
	GoalStatusNotStarted  GoalStatus = "not_started"
	GoalStatusInProgress  GoalStatus = "in_progress"
	GoalStatusAchieved    GoalStatus = "achieved"
	GoalStatusNotAchieved GoalStatus = "not_achieved"
	GoalStatusModified    GoalStatus = "modified"
)

// PlannedIntervention represents a planned intervention.
type PlannedIntervention struct {
	ID              string             `json:"id"`
	Type            InterventionType   `json:"type"`
	Description     string             `json:"description"`
	Frequency       string             `json:"frequency"`
	Duration        string             `json:"duration"`
	Location        string             `json:"location"` // clinic, home, workplace, community
	EquipmentNeeded string             `json:"equipmentNeeded,omitempty"`
	Status          InterventionStatus `json:"status"`
	CreatedAt       int64              `json:"createdAt"`
	UpdatedAt       int64              `json:"updatedAt"`
}

// InterventionStatus tracks intervention status.
type InterventionStatus string

const (
	InterventionPlanned   InterventionStatus = "planned"
	InterventionActive    InterventionStatus = "active"
	InterventionCompleted InterventionStatus = "completed"
	InterventionCancelled InterventionStatus = "cancelled"
	InterventionOnHold    InterventionStatus = "on_hold"
)

// SessionNote represents an OT session note.
type SessionNote struct {
	ID                 string                `json:"id"`
	PatientNHI         string                `json:"patientNhi"`
	ClinicianID        string                `json:"clinicianId"`
	PracticeID         string                `json:"practiceId"`
	InterventionPlanID string                `json:"interventionPlanId"`
	SessionDate        int64                 `json:"sessionDate"`
	SessionNumber      int                   `json:"sessionNumber"`
	Location           string                `json:"location"`
	Subjective         string                `json:"subjective"`
	Objective          string                `json:"objective"`
	Assessment         string                `json:"assessment"`
	Plan               string                `json:"plan"`
	Interventions      []PlannedIntervention `json:"interventions"`
	OutcomeMeasures    []OutcomeMeasure      `json:"outcomeMeasures"`
	DurationMinutes    int                   `json:"durationMinutes"`
	ChargeCode         string                `json:"chargeCode,omitempty"`
	CreatedAt          int64                 `json:"createdAt"`
	UpdatedAt          int64                 `json:"updatedAt"`
}

// NewAssessment creates a new assessment with defaults.
func NewAssessment() *Assessment {
	now := time.Now().UnixMilli()
	return &Assessment{
		Recommendations: []Recommendation{},
		OutcomeMeasures: []OutcomeMeasure{},
		Status:          AssessmentScheduled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// Validate checks required fields for an assessment.
func (a *Assessment) Validate() error {
	if a.PatientNHI == "" {
		return fmt.Errorf("ot: patient NHI is required")
	}
	if !nhi.ValidateNHI(a.PatientNHI) {
		return fmt.Errorf("ot: invalid patient NHI: %s", a.PatientNHI)
	}
	if a.ClinicianID == "" {
		return fmt.Errorf("ot: clinician ID is required")
	}
	if a.Type == "" {
		return fmt.Errorf("ot: assessment type is required")
	}
	if a.Date == 0 {
		return fmt.Errorf("ot: assessment date is required")
	}
	return nil
}

// AddRecommendation adds a recommendation to the assessment.
func (a *Assessment) AddRecommendation(rec Recommendation) {
	rec.ID = fmt.Sprintf("rec-%d", len(a.Recommendations)+1)
	now := time.Now().UnixMilli()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	a.Recommendations = append(a.Recommendations, rec)
	a.UpdatedAt = now
}

// AddOutcomeMeasure adds an outcome measure to the assessment.
func (a *Assessment) AddOutcomeMeasure(measure OutcomeMeasure) {
	measure.ID = fmt.Sprintf("measure-%d", len(a.OutcomeMeasures)+1)
	now := time.Now().UnixMilli()
	measure.CreatedAt = now
	a.OutcomeMeasures = append(a.OutcomeMeasures, measure)
	a.UpdatedAt = now
}

// NewInterventionPlan creates a new intervention plan with defaults.
func NewInterventionPlan() *InterventionPlan {
	now := time.Now().UnixMilli()
	return &InterventionPlan{
		Status:        PlanStatusDraft,
		Goals:         []InterventionGoal{},
		Interventions: []PlannedIntervention{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Validate checks required fields for an intervention plan.
func (p *InterventionPlan) Validate() error {
	if p.PatientNHI == "" {
		return fmt.Errorf("ot: patient NHI is required")
	}
	if !nhi.ValidateNHI(p.PatientNHI) {
		return fmt.Errorf("ot: invalid patient NHI: %s", p.PatientNHI)
	}
	if p.ClinicianID == "" {
		return fmt.Errorf("ot: clinician ID is required")
	}
	if p.StartDate == 0 {
		return fmt.Errorf("ot: start date is required")
	}
	return nil
}

// AddGoal adds a goal to the intervention plan.
func (p *InterventionPlan) AddGoal(goal InterventionGoal) {
	goal.ID = fmt.Sprintf("goal-%d", len(p.Goals)+1)
	now := time.Now().UnixMilli()
	goal.CreatedAt = now
	goal.UpdatedAt = now
	p.Goals = append(p.Goals, goal)
	p.UpdatedAt = now
}

// AddIntervention adds an intervention to the plan.
func (p *InterventionPlan) AddIntervention(intervention PlannedIntervention) {
	intervention.ID = fmt.Sprintf("intervention-%d", len(p.Interventions)+1)
	now := time.Now().UnixMilli()
	intervention.CreatedAt = now
	intervention.UpdatedAt = now
	p.Interventions = append(p.Interventions, intervention)
	p.UpdatedAt = now
}

// NewSessionNote creates a new session note with defaults.
func NewSessionNote() *SessionNote {
	now := time.Now().UnixMilli()
	return &SessionNote{
		Interventions:   []PlannedIntervention{},
		OutcomeMeasures: []OutcomeMeasure{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// Validate checks required fields for a session note.
func (s *SessionNote) Validate() error {
	if s.PatientNHI == "" {
		return fmt.Errorf("ot: patient NHI is required")
	}
	if !nhi.ValidateNHI(s.PatientNHI) {
		return fmt.Errorf("ot: invalid patient NHI: %s", s.PatientNHI)
	}
	if s.ClinicianID == "" {
		return fmt.Errorf("ot: clinician ID is required")
	}
	if s.SessionDate == 0 {
		return fmt.Errorf("ot: session date is required")
	}
	return nil
}
