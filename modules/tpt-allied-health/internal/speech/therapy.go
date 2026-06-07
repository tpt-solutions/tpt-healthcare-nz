// Package speech implements speech-language therapy assessment and intervention
// for NZ speech-language therapy practice, including ACC-funded treatment.
package speech

import (
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// AssessmentType categorises the type of speech-language assessment.
type AssessmentType string

const (
	AssessmentSpeechSound      AssessmentType = "speech_sound"
	AssessmentLanguage         AssessmentType = "language"
	AssessmentFluency          AssessmentType = "fluency"
	AssessmentVoice            AssessmentType = "voice"
	AssessmentSwallowing       AssessmentType = "swallowing"
	AssessmentCognitiveComm    AssessmentType = "cognitive_communication"
	AssessmentAAC              AssessmentType = "aac" // Augmentative and Alternative Communication
	AssessmentPaediatric       AssessmentType = "paediatric"
	AssessmentAdultNeuro       AssessmentType = "adult_neurological"
	AssessmentProgress         AssessmentType = "progress_review"
)

// InterventionType categorises speech-language interventions.
type InterventionType string

const (
	InterventionArticulationTherapy InterventionType = "articulation_therapy"
	InterventionPhonologyTherapy    InterventionType = "phonology_therapy"
	InterventionLanguageTherapy     InterventionType = "language_therapy"
	InterventionFluencyTherapy      InterventionType = "fluency_therapy"
	InterventionVoiceTherapy        InterventionType = "voice_therapy"
	InterventionSwallowingTherapy   InterventionType = "swallowing_therapy"
	InterventionCognitiveCommTherapy InterventionType = "cognitive_communication_therapy"
	InterventionAACImplementation   InterventionType = "aac_implementation"
	InterventionParentTraining      InterventionType = "parent_training"
	InterventionTeacherTraining     InterventionType = "teacher_training"
	InterventionLSVT                InterventionType = "lsvt" // Lee Silverman Voice Treatment
)

// Assessment represents a speech-language assessment.
type Assessment struct {
	ID              string            `json:"id"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	PracticeID      string            `json:"practiceId"`
	ACCNumber       string            `json:"accNumber,omitempty"`
	ReferralSource  string            `json:"referralSource"`
	Type            AssessmentType    `json:"type"`
	Date            int64             `json:"date"`
	Reason          string            `json:"reason"`
	Findings        string            `json:"findings"`
	Diagnosis       string            `json:"diagnosis,omitempty"`
	ICD10Code       string            `json:"icd10Code,omitempty"`
	Recommendations []Recommendation  `json:"recommendations"`
	OutcomeMeasures []OutcomeMeasure  `json:"outcomeMeasures"`
	Status          AssessmentStatus  `json:"status"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// AssessmentStatus tracks assessment lifecycle.
type AssessmentStatus string

const (
	AssessmentScheduled AssessmentStatus = "scheduled"
	AssessmentInProgress AssessmentStatus = "in_progress"
	AssessmentCompleted AssessmentStatus = "completed"
	AssessmentCancelled AssessmentStatus = "cancelled"
	AssessmentOnHold    AssessmentStatus = "on_hold"
)

// Recommendation represents a speech-language recommendation.
type Recommendation struct {
	ID              string            `json:"id"`
	Description     string            `json:"description"`
	Priority        RecommendationPriority `json:"priority"`
	Type            InterventionType  `json:"type,omitempty"`
	Frequency       string            `json:"frequency,omitempty"` // e.g., "weekly", "fortnightly"
	Duration        string            `json:"duration,omitempty"`  // e.g., "45 min"
	Setting         string            `json:"setting,omitempty"`   // clinic, home, school, telehealth
	Equipment       string            `json:"equipment,omitempty"`
	FundingSource   string            `json:"fundingSource,omitempty"` // ACC, MOH, MOE, private
	Status          RecommendationStatus `json:"status"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// RecommendationPriority indicates urgency.
type RecommendationPriority string

const (
	PriorityUrgent   RecommendationPriority = "urgent"
	PriorityHigh     RecommendationPriority = "high"
	PriorityMedium   RecommendationPriority = "medium"
	PriorityLow      RecommendationPriority = "low"
	PriorityRoutine  RecommendationPriority = "routine"
)

// RecommendationStatus tracks recommendation implementation.
type RecommendationStatus string

const (
	RecommendationPending   RecommendationStatus = "pending"
	RecommendationApproved  RecommendationStatus = "approved"
	RecommendationDeclined  RecommendationStatus = "declined"
	RecommendationInProgress RecommendationStatus = "in_progress"
	RecommendationCompleted RecommendationStatus = "completed"
)

// OutcomeMeasure represents a standardised speech-language outcome measure.
type OutcomeMeasure struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`           // e.g., "CELF-5", "PPVT-5", "GFTA-3", "SSI-4", "VHI-10"
	Domain        string  `json:"domain"`         // e.g., "receptive_language", "expressive_language", "articulation", "fluency", "voice"
	Score         float64 `json:"score"`
	MaxScore      float64 `json:"maxScore,omitempty"`
	Percentile    float64 `json:"percentile,omitempty"`
	AgeEquivalent string  `json:"ageEquivalent,omitempty"`
	Date          int64   `json:"date"`
	Interpretation string `json:"interpretation,omitempty"`
	CreatedAt     int64   `json:"createdAt"`
}

// TherapyPlan represents a speech-language therapy plan.
type TherapyPlan struct {
	ID              string            `json:"id"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	PracticeID      string            `json:"practiceId"`
	AssessmentID    string            `json:"assessmentId,omitempty"`
	ACCNumber       string            `json:"accNumber,omitempty"`
	StartDate       int64             `json:"startDate"`
	ReviewDate      int64             `json:"reviewDate"`
	EndDate         int64             `json:"endDate,omitempty"`
	Status          PlanStatus        `json:"status"`
	Goals           []TherapyGoal     `json:"goals"`
	Interventions   []PlannedIntervention `json:"interventions"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// PlanStatus tracks therapy plan lifecycle.
type PlanStatus string

const (
	PlanStatusDraft       PlanStatus = "draft"
	PlanStatusActive      PlanStatus = "active"
	PlanStatusUnderReview PlanStatus = "under_review"
	PlanStatusCompleted   PlanStatus = "completed"
	PlanStatusDiscontinued PlanStatus = "discontinued"
	PlanStatusOnHold      PlanStatus = "on_hold"
)

// TherapyGoal represents a goal in the therapy plan.
type TherapyGoal struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Domain      string    `json:"domain"` // e.g., "articulation", "language", "fluency", "voice", "swallowing"
	TargetDate  int64     `json:"targetDate"`
	Criteria    string    `json:"criteria"` // Measurable criteria for goal achievement
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

// PlannedIntervention represents a planned intervention.
type PlannedIntervention struct {
	ID              string            `json:"id"`
	Type            InterventionType  `json:"type"`
	Description     string            `json:"description"`
	Frequency       string            `json:"frequency"`
	Duration        string            `json:"duration"`
	Setting         string            `json:"setting"` // clinic, home, school, telehealth
	Techniques      string            `json:"techniques,omitempty"`
	Materials       string            `json:"materials,omitempty"`
	Status          InterventionStatus `json:"status"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
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

// SessionNote represents a speech-language therapy session note.
type SessionNote struct {
	ID              string            `json:"id"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	PracticeID      string            `json:"practiceId"`
	TherapyPlanID   string            `json:"therapyPlanId"`
	SessionDate     int64             `json:"sessionDate"`
	SessionNumber   int               `json:"sessionNumber"`
	Setting         string            `json:"setting"`
	Subjective      string            `json:"subjective"`
	Objective       string            `json:"objective"`
	Assessment      string            `json:"assessment"`
	Plan            string            `json:"plan"`
	Interventions   []PlannedIntervention `json:"interventions"`
	OutcomeMeasures []OutcomeMeasure  `json:"outcomeMeasures"`
	DurationMinutes int               `json:"durationMinutes"`
	ChargeCode      string            `json:"chargeCode,omitempty"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// SwallowingAssessment represents a specialised swallowing assessment.
type SwallowingAssessment struct {
	ID              string            `json:"id"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	PracticeID      string            `json:"practiceId"`
	Date            int64             `json:"date"`
	Reason          string            `json:"reason"`
	OralMechanism   string            `json:"oralMechanism"`
	ClinicalFindings string           `json:"clinicalFindings"`
	InstrumentalExam string            `json:"instrumentalExam,omitempty"` // VFSS, FEES
	DietRecommendations string        `json:"dietRecommendations"` // IDDSI levels
	Strategies      string            `json:"strategies"`
	Referrals       string            `json:"referrals,omitempty"`
	Status          AssessmentStatus  `json:"status"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
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
		return fmt.Errorf("speech: patient NHI is required")
	}
	if !nhi.ValidateNHI(a.PatientNHI) {
		return fmt.Errorf("speech: invalid patient NHI: %s", a.PatientNHI)
	}
	if a.ClinicianID == "" {
		return fmt.Errorf("speech: clinician ID is required")
	}
	if a.Type == "" {
		return fmt.Errorf("speech: assessment type is required")
	}
	if a.Date == 0 {
		return fmt.Errorf("speech: assessment date is required")
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

// NewTherapyPlan creates a new therapy plan with defaults.
func NewTherapyPlan() *TherapyPlan {
	now := time.Now().UnixMilli()
	return &TherapyPlan{
		Status:        PlanStatusDraft,
		Goals:         []TherapyGoal{},
		Interventions: []PlannedIntervention{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Validate checks required fields for a therapy plan.
func (p *TherapyPlan) Validate() error {
	if p.PatientNHI == "" {
		return fmt.Errorf("speech: patient NHI is required")
	}
	if !nhi.ValidateNHI(p.PatientNHI) {
		return fmt.Errorf("speech: invalid patient NHI: %s", p.PatientNHI)
	}
	if p.ClinicianID == "" {
		return fmt.Errorf("speech: clinician ID is required")
	}
	if p.StartDate == 0 {
		return fmt.Errorf("speech: start date is required")
	}
	return nil
}

// AddGoal adds a goal to the therapy plan.
func (p *TherapyPlan) AddGoal(goal TherapyGoal) {
	goal.ID = fmt.Sprintf("goal-%d", len(p.Goals)+1)
	now := time.Now().UnixMilli()
	goal.CreatedAt = now
	goal.UpdatedAt = now
	p.Goals = append(p.Goals, goal)
	p.UpdatedAt = now
}

// AddIntervention adds an intervention to the plan.
func (p *TherapyPlan) AddIntervention(intervention PlannedIntervention) {
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
		return fmt.Errorf("speech: patient NHI is required")
	}
	if !nhi.ValidateNHI(s.PatientNHI) {
		return fmt.Errorf("speech: invalid patient NHI: %s", s.PatientNHI)
	}
	if s.ClinicianID == "" {
		return fmt.Errorf("speech: clinician ID is required")
	}
	if s.SessionDate == 0 {
		return fmt.Errorf("speech: session date is required")
	}
	return nil
}

// NewSwallowingAssessment creates a new swallowing assessment with defaults.
func NewSwallowingAssessment() *SwallowingAssessment {
	now := time.Now().UnixMilli()
	return &SwallowingAssessment{
		Status:    AssessmentScheduled,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks required fields for a swallowing assessment.
func (s *SwallowingAssessment) Validate() error {
	if s.PatientNHI == "" {
		return fmt.Errorf("speech: patient NHI is required")
	}
	if !nhi.ValidateNHI(s.PatientNHI) {
		return fmt.Errorf("speech: invalid patient NHI: %s", s.PatientNHI)
	}
	if s.ClinicianID == "" {
		return fmt.Errorf("speech: clinician ID is required")
	}
	if s.Date == 0 {
		return fmt.Errorf("speech: assessment date is required")
	}
	return nil
}