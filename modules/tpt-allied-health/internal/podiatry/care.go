// Package podiatry implements podiatry assessment and treatment planning
// for NZ podiatry practice, including ACC-funded treatment and high-risk foot care.
package podiatry

import (
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// AssessmentType categorises the type of podiatry assessment.
type AssessmentType string

const (
	AssessmentGeneral          AssessmentType = "general"
	AssessmentDiabeticFoot     AssessmentType = "diabetic_foot"
	AssessmentVascular         AssessmentType = "vascular"
	AssessmentNeurological     AssessmentType = "neurological"
	AssessmentBiomechanical    AssessmentType = "biomechanical"
	AssessmentWound            AssessmentType = "wound"
	AssessmentNailSurgery      AssessmentType = "nail_surgery"
	AssessmentFootwear         AssessmentType = "footwear"
	AssessmentPaediatric       AssessmentType = "paediatric"
	AssessmentSports           AssessmentType = "sports"
	AssessmentHighRiskFoot     AssessmentType = "high_risk_foot"
	AssessmentPrePostOp        AssessmentType = "pre_post_operative"
)

// TreatmentType categorises podiatry treatments.
type TreatmentType string

const (
	TreatmentNailCare           TreatmentType = "nail_care"
	TreatmentCallusDebridement  TreatmentType = "callus_debridement"
	TreatmentCornEnucleation    TreatmentType = "corn_enucleation"
	TreatmentWoundDebridement   TreatmentType = "wound_debridement"
	TreatmentWoundDressing      TreatmentType = "wound_dressing"
	TreatmentOffloading         TreatmentType = "offloading"
	TreatmentOrthoticTherapy    TreatmentType = "orthotic_therapy"
	TreatmentFootwearModification TreatmentType = "footwear_modification"
	TreatmentNailSurgery        TreatmentType = "nail_surgery"
	TreatmentInjectionTherapy   TreatmentType = "injection_therapy"
	TreatmentShockwaveTherapy   TreatmentType = "shockwave_therapy"
	TreatmentExerciseTherapy    TreatmentType = "exercise_therapy"
	TreatmentEducation          TreatmentType = "education"
)

// Assessment represents a podiatry assessment.
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
	RiskCategory    RiskCategory      `json:"riskCategory,omitempty"`
	VascularStatus  VascularStatus    `json:"vascularStatus,omitempty"`
	NeurologicalStatus NeurologicalStatus `json:"neurologicalStatus,omitempty"`
	Recommendations []Recommendation  `json:"recommendations"`
	OutcomeMeasures []OutcomeMeasure  `json:"outcomeMeasures"`
	Status          AssessmentStatus  `json:"status"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// RiskCategory for diabetic foot risk stratification (NHS/IWGDF classification).
type RiskCategory string

const (
	RiskCategoryLow       RiskCategory = "low"        // 0 - No risk factors
	RiskCategoryModerate  RiskCategory = "moderate"   // 1 - One risk factor
	RiskCategoryHigh      RiskCategory = "high"       // 2 - Two risk factors
	RiskCategoryVeryHigh  RiskCategory = "very_high"  // 3 - Previous ulcer/amputation
	RiskCategoryActive    RiskCategory = "active"     // Active ulceration
)

// VascularStatus documents vascular assessment findings.
type VascularStatus string

const (
	VascularNormal      VascularStatus = "normal"
	VascularReduced     VascularStatus = "reduced"
	VascularAbsent      VascularStatus = "absent"
	VascularMonophasic  VascularStatus = "monophasic"
	VascularBiphasic    VascularStatus = "biphasic"
	VascularTriphasic   VascularStatus = "triphasic"
)

// NeurologicalStatus documents neurological assessment findings.
type NeurologicalStatus string

const (
	NeurologicalIntact      NeurologicalStatus = "intact"
	NeurologicalReduced     NeurologicalStatus = "reduced"
	NeurologicalAbsent      NeurologicalStatus = "absent"
	NeurologicalHyperesthesia NeurologicalStatus = "hyperesthesia"
	NeurologicalAllodynia   NeurologicalStatus = "allodynia"
)

// AssessmentStatus tracks assessment lifecycle.
type AssessmentStatus string

const (
	AssessmentScheduled AssessmentStatus = "scheduled"
	AssessmentInProgress AssessmentStatus = "in_progress"
	AssessmentCompleted AssessmentStatus = "completed"
	AssessmentCancelled AssessmentStatus = "cancelled"
	AssessmentOnHold    AssessmentStatus = "on_hold"
)

// Recommendation represents a podiatry recommendation.
type Recommendation struct {
	ID              string            `json:"id"`
	Description     string            `json:"description"`
	Priority        RecommendationPriority `json:"priority"`
	Type            TreatmentType     `json:"type,omitempty"`
	Frequency       string            `json:"frequency,omitempty"`
	Duration        string            `json:"duration,omitempty"`
	Equipment       string            `json:"equipment,omitempty"`
	ReferralTo      string            `json:"referralTo,omitempty"` // vascular, orthopaedic, diabetes team
	FundingSource   string            `json:"fundingSource,omitempty"` // ACC, MOH, DHB, private
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

// OutcomeMeasure represents a standardised podiatry outcome measure.
type OutcomeMeasure struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`           // e.g., "VPT", "ABPI", "WoundArea", "ManchesterScale"
	Domain        string  `json:"domain"`         // e.g., "vascular", "neurological", "wound", "function"
	Score         float64 `json:"score"`
	Unit          string  `json:"unit,omitempty"` // mmHg, cm2, volts, score
	Date          int64   `json:"date"`
	Interpretation string `json:"interpretation,omitempty"`
	CreatedAt     int64   `json:"createdAt"`
}

// TreatmentPlan represents a podiatry treatment plan.
type TreatmentPlan struct {
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
	Goals           []TreatmentGoal   `json:"goals"`
	Interventions   []PlannedIntervention `json:"interventions"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// PlanStatus tracks treatment plan lifecycle.
type PlanStatus string

const (
	PlanStatusDraft       PlanStatus = "draft"
	PlanStatusActive      PlanStatus = "active"
	PlanStatusUnderReview PlanStatus = "under_review"
	PlanStatusCompleted   PlanStatus = "completed"
	PlanStatusDiscontinued PlanStatus = "discontinued"
	PlanStatusOnHold      PlanStatus = "on_hold"
)

// TreatmentGoal represents a goal in the treatment plan.
type TreatmentGoal struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Domain      string    `json:"domain"` // e.g., "wound_healing", "pain_reduction", "mobility", "prevention"
	TargetDate  int64     `json:"targetDate"`
	Criteria    string    `json:"criteria"`
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
	ID              string        `json:"id"`
	Type            TreatmentType `json:"type"`
	Description     string        `json:"description"`
	Frequency       string        `json:"frequency"`
	Duration        string        `json:"duration"`
	Location        string        `json:"location"` // clinic, home, residential_care
	Materials       string        `json:"materials,omitempty"`
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

// SessionNote represents a podiatry session note.
type SessionNote struct {
	ID              string            `json:"id"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	PracticeID      string            `json:"practiceId"`
	TreatmentPlanID string            `json:"treatmentPlanId"`
	SessionDate     int64             `json:"sessionDate"`
	SessionNumber   int               `json:"sessionNumber"`
	Location        string            `json:"location"`
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

// WoundAssessment represents a detailed wound assessment.
type WoundAssessment struct {
	ID              string            `json:"id"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	PracticeID      string            `json:"practiceId"`
	Date            int64             `json:"date"`
	Location        string            `json:"location"` // anatomical location
	Side            string            `json:"side"`     // left, right, bilateral
	WoundType       WoundType         `json:"woundType"`
	Dimensions      WoundDimensions   `json:"dimensions"`
	TissueType      TissueType        `json:"tissueType"`
	Exudate         ExudateLevel      `json:"exudate"`
	InfectionSigns  InfectionSigns    `json:"infectionSigns"`
	PeriwoundSkin   string            `json:"periwoundSkin"`
	PainScore       int               `json:"painScore"` // 0-10
	Treatment       string            `json:"treatment"`
	DressingPlan    string            `json:"dressingPlan"`
	Offloading      string            `json:"offloading"`
	Referrals       string            `json:"referrals,omitempty"`
	ReviewDate      int64             `json:"reviewDate"`
	Status          AssessmentStatus  `json:"status"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// WoundType categorises wound aetiology.
type WoundType string

const (
	WoundTypeDiabeticFoot   WoundType = "diabetic_foot_ulcer"
	WoundTypeVenous         WoundType = "venous_leg_ulcer"
	WoundTypeArterial       WoundType = "arterial_ulcer"
	WoundTypePressure       WoundType = "pressure_injury"
	WoundTypeSurgical       WoundType = "surgical_wound"
	WoundTypeTrauma         WoundType = "trauma_burn"
	WoundTypeMoisture       WoundType = "moisture_associated"
)

// WoundDimensions records wound measurements.
type WoundDimensions struct {
	Length   float64 `json:"length"`   // cm
	Width    float64 `json:"width"`    // cm
	Depth    float64 `json:"depth"`    // cm
	Undermining float64 `json:"undermining,omitempty"` // cm
	SinusTract  float64 `json:"sinusTract,omitempty"`  // cm
	Area       float64 `json:"area"`     // cm2 (calculated)
	Volume     float64 `json:"volume,omitempty"` // ml
}

// TissueType documents wound bed tissue (TIME principles).
type TissueType string

const (
	TissueNecrotic   TissueType = "necrotic"
	TissueSloughy    TissueType = "sloughy"
	TissueGranulating TissueType = "granulating"
	TissueEpithelialising TissueType = "epithelialising"
)

// ExudateLevel documents exudate amount.
type ExudateLevel string

const (
	ExudateNone     ExudateLevel = "none"
	ExudateLow      ExudateLevel = "low"
	ExudateModerate ExudateLevel = "moderate"
	ExudateHigh     ExudateLevel = "high"
)

// InfectionSigns documents clinical signs of infection.
type InfectionSigns struct {
	Erythema       bool   `json:"erythema"`
	Oedema         bool   `json:"oedema"`
	Heat           bool   `json:"heat"`
	Pain           bool   `json:"pain"`
	PurulentExudate bool  `json:"purulentExudate"`
	Odour          bool   `json:"odour"`
	Fever          bool   `json:"fever"`
	WBCElevated    bool   `json:"wbcElevated"`
	CRPElevated    bool   `json:"crpElevated"`
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
		return fmt.Errorf("podiatry: patient NHI is required")
	}
	if !nhi.ValidateNHI(a.PatientNHI) {
		return fmt.Errorf("podiatry: invalid patient NHI: %s", a.PatientNHI)
	}
	if a.ClinicianID == "" {
		return fmt.Errorf("podiatry: clinician ID is required")
	}
	if a.Type == "" {
		return fmt.Errorf("podiatry: assessment type is required")
	}
	if a.Date == 0 {
		return fmt.Errorf("podiatry: assessment date is required")
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

// NewTreatmentPlan creates a new treatment plan with defaults.
func NewTreatmentPlan() *TreatmentPlan {
	now := time.Now().UnixMilli()
	return &TreatmentPlan{
		Status:        PlanStatusDraft,
		Goals:         []TreatmentGoal{},
		Interventions: []PlannedIntervention{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Validate checks required fields for a treatment plan.
func (p *TreatmentPlan) Validate() error {
	if p.PatientNHI == "" {
		return fmt.Errorf("podiatry: patient NHI is required")
	}
	if !nhi.ValidateNHI(p.PatientNHI) {
		return fmt.Errorf("podiatry: invalid patient NHI: %s", p.PatientNHI)
	}
	if p.ClinicianID == "" {
		return fmt.Errorf("podiatry: clinician ID is required")
	}
	if p.StartDate == 0 {
		return fmt.Errorf("podiatry: start date is required")
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

// AddIntervention adds an intervention to the plan.
func (p *TreatmentPlan) AddIntervention(intervention PlannedIntervention) {
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
		return fmt.Errorf("podiatry: patient NHI is required")
	}
	if !nhi.ValidateNHI(s.PatientNHI) {
		return fmt.Errorf("podiatry: invalid patient NHI: %s", s.PatientNHI)
	}
	if s.ClinicianID == "" {
		return fmt.Errorf("podiatry: clinician ID is required")
	}
	if s.SessionDate == 0 {
		return fmt.Errorf("podiatry: session date is required")
	}
	return nil
}

// NewWoundAssessment creates a new wound assessment with defaults.
func NewWoundAssessment() *WoundAssessment {
	now := time.Now().UnixMilli()
	return &WoundAssessment{
		Status:    AssessmentScheduled,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks required fields for a wound assessment.
func (w *WoundAssessment) Validate() error {
	if w.PatientNHI == "" {
		return fmt.Errorf("podiatry: patient NHI is required")
	}
	if !nhi.ValidateNHI(w.PatientNHI) {
		return fmt.Errorf("podiatry: invalid patient NHI: %s", w.PatientNHI)
	}
	if w.ClinicianID == "" {
		return fmt.Errorf("podiatry: clinician ID is required")
	}
	if w.Date == 0 {
		return fmt.Errorf("podiatry: assessment date is required")
	}
	if w.Location == "" {
		return fmt.Errorf("podiatry: wound location is required")
	}
	return nil
}