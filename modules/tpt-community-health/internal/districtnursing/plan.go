// Package districtnursing implements district nursing care planning and
// visit documentation for NZ community nursing practice.
package districtnursing

import (
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// PlanType categorises district nursing care plans.
type PlanType string

const (
	PlanWoundCare    PlanType = "wound_care"
	PlanPalliative   PlanType = "palliative"
	PlanDiabetes     PlanType = "diabetes"
	PlanHeartFailure PlanType = "heart_failure"
	PlanCOPD         PlanType = "copd"
	PlanPostSurgical PlanType = "post_surgical"
	PlanPostAcute    PlanType = "post_acute"
	PlanMedication   PlanType = "medication_management"
)

// PlanStatus tracks the lifecycle of a care plan.
type PlanStatus string

const (
	PlanDraft       PlanStatus = "draft"
	PlanActive      PlanStatus = "active"
	PlanUnderReview PlanStatus = "under_review"
	PlanCompleted   PlanStatus = "completed"
	PlanSuspended   PlanStatus = "suspended"
)

// RiskLevel indicates patient risk stratification.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskModerate RiskLevel = "moderate"
	RiskHigh     RiskLevel = "high"
	RiskVeryHigh RiskLevel = "very_high"
)

// VisitType categorises nursing visits.
type VisitType string

const (
	VisitScheduled   VisitType = "scheduled"
	VisitUnscheduled VisitType = "unscheduled"
	VisitUrgent      VisitType = "urgent"
)

// VisitStatus tracks visit lifecycle.
type VisitStatus string

const (
	VisitStatusScheduled  VisitStatus = "scheduled"
	VisitStatusInProgress VisitStatus = "in_progress"
	VisitStatusCompleted  VisitStatus = "completed"
	VisitStatusCancelled  VisitStatus = "cancelled"
)

// AdminRoute indicates medication administration route.
type AdminRoute string

const (
	RouteOral       AdminRoute = "oral"
	RouteIM         AdminRoute = "im"
	RouteIV         AdminRoute = "iv"
	RouteSC         AdminRoute = "sc"
	RouteTopical    AdminRoute = "topical"
	RouteInhalation AdminRoute = "inhalation"
)

// AdminStatus tracks medication administration status.
type AdminStatus string

const (
	AdminScheduled    AdminStatus = "scheduled"
	AdminAdministered AdminStatus = "administered"
	AdminRefused      AdminStatus = "refused"
	AdminOmitted      AdminStatus = "omitted"
	AdminHeld         AdminStatus = "held"
)

// WoundCause categorises wound aetiology.
type WoundCause string

const (
	WoundPressure WoundCause = "pressure_injury"
	WoundVenous   WoundCause = "venous"
	WoundArterial WoundCause = "arterial"
	WoundDiabetic WoundCause = "diabetic"
	WoundSurgical WoundCause = "surgical"
	WoundTrauma   WoundCause = "trauma"
)

// CarePlan represents a district nursing care plan.
type CarePlan struct {
	ID           string     `json:"id"`
	PatientNHI   string     `json:"patientNhi"`
	ClinicianID  string     `json:"clinicianId"`
	PracticeID   string     `json:"practiceId"`
	PlanName     string     `json:"planName"`
	PlanType     PlanType   `json:"planType"`
	Status       PlanStatus `json:"status"`
	StartDate    int64      `json:"startDate"`
	ReviewDate   int64      `json:"reviewDate"`
	EndDate      int64      `json:"endDate,omitempty"`
	Goals        []string   `json:"goals,omitempty"`
	RiskLevel    RiskLevel  `json:"riskLevel"`
	ConsentGiven bool       `json:"consentGiven"`
	ConsentDate  int64      `json:"consentDate,omitempty"`
	DHBFunded    bool       `json:"dhbFunded"`
	FundingCode  string     `json:"fundingCode,omitempty"`
	CreatedAt    int64      `json:"createdAt"`
	UpdatedAt    int64      `json:"updatedAt"`
}

// NursingVisit represents a district nursing visit.
type NursingVisit struct {
	ID                      string            `json:"id"`
	CarePlanID              string            `json:"carePlanId"`
	PatientNHI              string            `json:"patientNhi"`
	ClinicianID             string            `json:"clinicianId"`
	VisitDate               int64             `json:"visitDate"`
	VisitType               VisitType         `json:"visitType"`
	VisitStatus             VisitStatus       `json:"visitStatus"`
	VitalSigns              *VitalSigns       `json:"vitalSigns,omitempty"`
	WoundAssessments        []WoundAssessment `json:"woundAssessments,omitempty"`
	MedicationsAdministered []MedicationAdmin `json:"medicationsAdministered,omitempty"`
	Observations            string            `json:"observations,omitempty"`
	PatientEducation        []string          `json:"patientEducation,omitempty"`
	EquipmentCheck          []string          `json:"equipmentCheck,omitempty"`
	NextVisitDate           int64             `json:"nextVisitDate,omitempty"`
	NextVisitReason         string            `json:"nextVisitReason,omitempty"`
	Concerns                []string          `json:"concerns,omitempty"`
	Escalations             string            `json:"escalations,omitempty"`
	CreatedAt               int64             `json:"createdAt"`
	UpdatedAt               int64             `json:"updatedAt"`
}

// VitalSigns captures standard vital measurements.
type VitalSigns struct {
	Temperature            float64 `json:"temperature,omitempty"` // Celsius
	BloodPressureSystolic  int     `json:"bpSystolic,omitempty"`
	BloodPressureDiastolic int     `json:"bpDiastolic,omitempty"`
	HeartRate              int     `json:"heartRate,omitempty"` // bpm
	SpO2                   float64 `json:"spo2,omitempty"`      // %
	PainScore              int     `json:"painScore,omitempty"` // 0-10
	WeightKg               float64 `json:"weightKg,omitempty"`
	RespiratoryRate        int     `json:"respiratoryRate,omitempty"`
	BloodGlucose           float64 `json:"bloodGlucose,omitempty"` // mmol/L
}

// WoundAssessment represents a wound assessment during a visit.
type WoundAssessment struct {
	ID                string         `json:"id"`
	WoundSite         string         `json:"woundSite"`
	WoundCause        WoundCause     `json:"woundCause,omitempty"`
	LengthCM          float64        `json:"lengthCm"`
	WidthCM           float64        `json:"widthCm"`
	DepthCM           float64        `json:"depthCm"`
	UnderminingCM     float64        `json:"underminingCm,omitempty"`
	TissueType        string         `json:"tissueType"`
	ExudateAmount     string         `json:"exudateAmount,omitempty"`
	ExudateType       string         `json:"exudateType,omitempty"`
	Odour             bool           `json:"odour"`
	SignsInfection    InfectionSigns `json:"signsInfection,omitempty"`
	PeriwoundSkin     string         `json:"periwoundSkin,omitempty"`
	PainScore         int            `json:"painScore,omitempty"`
	DressingApplied   string         `json:"dressingApplied,omitempty"`
	CleansingSolution string         `json:"cleansingSolution,omitempty"`
	Debridement       bool           `json:"debridement"`
	DebridementType   string         `json:"debridementType,omitempty"`
	PhotosTaken       bool           `json:"photosTaken"`
	PhotoURLs         []string       `json:"photoUrls,omitempty"`
	NextReviewDate    int64          `json:"nextReviewDate,omitempty"`
	CreatedAt         int64          `json:"createdAt"`
	UpdatedAt         int64          `json:"updatedAt"`
}

// InfectionSigns documents clinical signs of infection.
type InfectionSigns struct {
	Erythema        bool `json:"erythema"`
	Oedema          bool `json:"oedema"`
	Heat            bool `json:"heat"`
	Pain            bool `json:"pain"`
	PurulentExudate bool `json:"purulentExudate"`
	Odour           bool `json:"odour"`
}

// MedicationAdmin represents a medication administration record.
type MedicationAdmin struct {
	ID                   string      `json:"id"`
	MedicationName       string      `json:"medicationName"`
	Dose                 string      `json:"dose"`
	Route                AdminRoute  `json:"route"`
	Frequency            string      `json:"frequency"`
	PrescribedBy         string      `json:"prescribedBy,omitempty"`
	AdministeredAt       int64       `json:"administeredAt,omitempty"`
	AdministrationStatus AdminStatus `json:"administrationStatus"`
	OmissionReason       string      `json:"omissionReason,omitempty"`
	Observations         string      `json:"observations,omitempty"`
	SideEffectsNoted     []string    `json:"sideEffectsNoted,omitempty"`
	CreatedAt            int64       `json:"createdAt"`
	UpdatedAt            int64       `json:"updatedAt"`
}

// NewCarePlan creates a new care plan with defaults.
func NewCarePlan() *CarePlan {
	now := time.Now().UnixMilli()
	return &CarePlan{
		Status:       PlanDraft,
		RiskLevel:    RiskLow,
		ConsentGiven: false,
		DHBFunded:    false,
		Goals:        []string{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Validate checks required fields for a care plan.
func (p *CarePlan) Validate() error {
	if p.PatientNHI == "" {
		return fmt.Errorf("districtnursing: patient NHI is required")
	}
	if !nhi.ValidateNHI(p.PatientNHI) {
		return fmt.Errorf("districtnursing: invalid patient NHI: %s", p.PatientNHI)
	}
	if p.ClinicianID == "" {
		return fmt.Errorf("districtnursing: clinician ID is required")
	}
	if p.PlanName == "" {
		return fmt.Errorf("districtnursing: plan name is required")
	}
	if p.PlanType == "" {
		return fmt.Errorf("districtnursing: plan type is required")
	}
	if p.StartDate == 0 {
		return fmt.Errorf("districtnursing: start date is required")
	}
	return nil
}

// NewNursingVisit creates a new nursing visit with defaults.
func NewNursingVisit() *NursingVisit {
	now := time.Now().UnixMilli()
	return &NursingVisit{
		VisitStatus:             VisitStatusScheduled,
		WoundAssessments:        []WoundAssessment{},
		MedicationsAdministered: []MedicationAdmin{},
		PatientEducation:        []string{},
		Concerns:                []string{},
		CreatedAt:               now,
		UpdatedAt:               now,
	}
}

// Validate checks required fields for a nursing visit.
func (v *NursingVisit) Validate() error {
	if v.CarePlanID == "" {
		return fmt.Errorf("districtnursing: care plan ID is required")
	}
	if v.PatientNHI == "" {
		return fmt.Errorf("districtnursing: patient NHI is required")
	}
	if !nhi.ValidateNHI(v.PatientNHI) {
		return fmt.Errorf("districtnursing: invalid patient NHI: %s", v.PatientNHI)
	}
	if v.ClinicianID == "" {
		return fmt.Errorf("districtnursing: clinician ID is required")
	}
	if v.VisitDate == 0 {
		return fmt.Errorf("districtnursing: visit date is required")
	}
	return nil
}
