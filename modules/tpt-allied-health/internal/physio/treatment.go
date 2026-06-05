// Package physio implements physiotherapy treatment planning and documentation
// for NZ physiotherapy practice, including ACC-funded treatment.
package physio

import (
	"fmt"
	"time"
)

// TreatmentType categorises the type of physiotherapy treatment.
type TreatmentType string

const (
	TreatmentManualTherapy     TreatmentType = "manual_therapy"
	TreatmentExerciseTherapy   TreatmentType = "exercise_therapy"
	TreatmentElectrotherapy    TreatmentType = "electrotherapy"
	TreatmentHydrotherapy      TreatmentType = "hydrotherapy"
	TreatmentEducation         TreatmentType = "education"
	TreatmentTaping            TreatmentType = "taping"
	TreatmentDryNeedling       TreatmentType = "dry_needling"
	TreatmentVestibular        TreatmentType = "vestibular"
	TreatmentRespiratory       TreatmentType = "respiratory"
	TreatmentPelvicHealth      TreatmentType = "pelvic_health"
)

// BodyRegion identifies the anatomical region being treated.
type BodyRegion string

const (
	RegionCervicalSpine    BodyRegion = "cervical_spine"
	RegionThoracicSpine    BodyRegion = "thoracic_spine"
	RegionLumbarSpine      BodyRegion = "lumbar_spine"
	RegionShoulder         BodyRegion = "shoulder"
	RegionElbow            BodyRegion = "elbow"
	RegionWristHand        BodyRegion = "wrist_hand"
	RegionHip              BodyRegion = "hip"
	RegionKnee             BodyRegion = "knee"
	RegionAnkleFoot        BodyRegion = "ankle_foot"
	RegionPelvicFloor      BodyRegion = "pelvic_floor"
	RegionTMJ              BodyRegion = "tmj"
	RegionThorax           BodyRegion = "thorax"
	RegionAbdomen          BodyRegion = "abdomen"
	RegionMultiple         BodyRegion = "multiple"
)

// TreatmentPlan represents a physiotherapy treatment plan.
type TreatmentPlan struct {
	ID              string            `json:"id"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	PracticeID      string            `json:"practiceId"`
	ACCNumber       string            `json:"accNumber,omitempty"`
	ReferralSource  string            `json:"referralSource"` // GP, specialist, self, ACC
	Diagnosis       string            `json:"diagnosis"`
	ICD10Code       string            `json:"icd10Code,omitempty"`
	StartDate       int64             `json:"startDate"`
	ReviewDate      int64             `json:"reviewDate"`
	EndDate         int64             `json:"endDate,omitempty"`
	Status          PlanStatus        `json:"status"`
	Goals           []TreatmentGoal   `json:"goals"`
	Interventions   []Intervention    `json:"interventions"`
	OutcomeMeasures []OutcomeMeasure  `json:"outcomeMeasures"`
	Notes           string            `json:"notes,omitempty"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// PlanStatus tracks the lifecycle of a treatment plan.
type PlanStatus string

const (
	PlanStatusDraft       PlanStatus = "draft"
	PlanStatusActive      PlanStatus = "active"
	PlanStatusUnderReview PlanStatus = "under_review"
	PlanStatusCompleted   PlanStatus = "completed"
	PlanStatusDiscontinued PlanStatus = "discontinued"
	PlanStatusOnHold      PlanStatus = "on_hold"
)

// TreatmentGoal represents a specific goal in the treatment plan.
type TreatmentGoal struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	TargetDate  int64     `json:"targetDate"`
	Status      GoalStatus `json:"status"`
	Outcome     string    `json:"outcome,omitempty"`
	CreatedAt   int64     `json:"createdAt"`
	UpdatedAt   int64     `json:"updatedAt"`
}

// GoalStatus tracks goal progress.
type GoalStatus string

const (
	GoalStatusNotStarted GoalStatus = "not_started"
	GoalStatusInProgress GoalStatus = "in_progress"
	GoalStatusAchieved   GoalStatus = "achieved"
	GoalStatusNotAchieved GoalStatus = "not_achieved"
	GoalStatusModified   GoalStatus = "modified"
)

// Intervention represents a specific treatment intervention.
type Intervention struct {
	ID              string        `json:"id"`
	Type            TreatmentType `json:"type"`
	BodyRegion      BodyRegion    `json:"bodyRegion"`
	Description     string        `json:"description"`
	Frequency       string        `json:"frequency"`        // e.g., "2x/week"
	Duration        string        `json:"duration"`         // e.g., "30 min"
	Intensity       string        `json:"intensity,omitempty"`
	Parameters      string        `json:"parameters,omitempty"` // JSON for specific params
	StartDate       int64         `json:"startDate"`
	EndDate         int64         `json:"endDate,omitempty"`
	Status          InterventionStatus `json:"status"`
	CreatedAt       int64         `json:"createdAt"`
	UpdatedAt       int64         `json:"updatedAt"`
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

// OutcomeMeasure represents a standardised outcome measure.
type OutcomeMeasure struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`           // e.g., "NDI", "ODI", "DASH", "LEFS"
	Score       float64 `json:"score"`
	MaxScore    float64 `json:"maxScore"`
	Date        int64   `json:"date"`
	Interpretation string `json:"interpretation,omitempty"`
	CreatedAt   int64   `json:"createdAt"`
}

// SessionNote represents a single treatment session note (SOAP format).
type SessionNote struct {
	ID              string            `json:"id"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	PracticeID      string            `json:"practiceId"`
	TreatmentPlanID string            `json:"treatmentPlanId"`
	SessionDate     int64             `json:"sessionDate"`
	SessionNumber   int               `json:"sessionNumber"`
	Subjective      string            `json:"subjective"`
	Objective       string            `json:"objective"`
	Assessment      string            `json:"assessment"`
	Plan            string            `json:"plan"`
	Interventions   []Intervention    `json:"interventions"`
	OutcomeMeasures []OutcomeMeasure  `json:"outcomeMeasures"`
	DurationMinutes int               `json:"durationMinutes"`
	ChargeCode      string            `json:"chargeCode,omitempty"` // ACC charge code
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// NewTreatmentPlan creates a new treatment plan with defaults.
func NewTreatmentPlan() *TreatmentPlan {
	now := time.Now().UnixMilli()
	return &TreatmentPlan{
		Status:      PlanStatusDraft,
		Goals:       []TreatmentGoal{},
		Interventions: []Intervention{},
		OutcomeMeasures: []OutcomeMeasure{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Validate checks required fields for a treatment plan.
func (p *TreatmentPlan) Validate() error {
	if p.PatientNHI == "" {
		return fmt.Errorf("physio: patient NHI is required")
	}
	if p.ClinicianID == "" {
		return fmt.Errorf("physio: clinician ID is required")
	}
	if p.Diagnosis == "" {
		return fmt.Errorf("physio: diagnosis is required")
	}
	if p.StartDate == 0 {
		return fmt.Errorf("physio: start date is required")
	}
	return nil
}

// AddGoal adds a goal to the treatment plan.
func (p *TreatmentPlan) AddGoal(goal TreatmentGoal) {
	goal.ID = fmt.Sprintf("goal-%d", len(p.Goals)+1)
	now := time.Now().UnixMilli()
	goal.CreatedAt = now
	goal.UpdatedAt = now
	p.Goals = append(p.Goals, goal)
	p.UpdatedAt = now
}

// AddIntervention adds an intervention to the treatment plan.
func (p *TreatmentPlan) AddIntervention(intervention Intervention) {
	intervention.ID = fmt.Sprintf("intervention-%d", len(p.Interventions)+1)
	now := time.Now().UnixMilli()
	intervention.CreatedAt = now
	intervention.UpdatedAt = now
	p.Interventions = append(p.Interventions, intervention)
	p.UpdatedAt = now
}

// AddOutcomeMeasure adds an outcome measure to the treatment plan.
func (p *TreatmentPlan) AddOutcomeMeasure(measure OutcomeMeasure) {
	measure.ID = fmt.Sprintf("measure-%d", len(p.OutcomeMeasures)+1)
	now := time.Now().UnixMilli()
	measure.CreatedAt = now
	p.OutcomeMeasures = append(p.OutcomeMeasures, measure)
	p.UpdatedAt = now
}

// NewSessionNote creates a new session note with defaults.
func NewSessionNote() *SessionNote {
	now := time.Now().UnixMilli()
	return &SessionNote{
		Interventions:   []Intervention{},
		OutcomeMeasures: []OutcomeMeasure{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// Validate checks required fields for a session note.
func (s *SessionNote) Validate() error {
	if s.PatientNHI == "" {
		return fmt.Errorf("physio: patient NHI is required")
	}
	if s.ClinicianID == "" {
		return fmt.Errorf("physio: clinician ID is required")
	}
	if s.SessionDate == 0 {
		return fmt.Errorf("physio: session date is required")
	}
	return nil
}