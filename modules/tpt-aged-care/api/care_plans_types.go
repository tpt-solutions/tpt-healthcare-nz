package api

import "time"

// CarePlanType distinguishes the care setting.
type CarePlanType string

const (
	PlanTypeResidential  CarePlanType = "residential"   // aged residential care facility
	PlanTypeHomeCare     CarePlanType = "home-care"      // home-based support
	PlanTypeDayProgramme CarePlanType = "day-programme"
	PlanTypeRespite      CarePlanType = "respite"
)

// CarePlanStatus tracks the lifecycle of a care plan.
type CarePlanStatus string

const (
	CarePlanActive    CarePlanStatus = "active"
	CarePlanOnHold    CarePlanStatus = "on-hold"
	CarePlanCompleted CarePlanStatus = "completed"
	CarePlanRevoked   CarePlanStatus = "revoked"
)

// GoalStatus tracks an individual care goal.
type GoalStatus string

const (
	GoalInProgress GoalStatus = "in-progress"
	GoalAchieved   GoalStatus = "achieved"
	GoalAbandoned  GoalStatus = "abandoned"
	GoalOnHold     GoalStatus = "on-hold"
)

// CareGoal represents a single goal within a care plan.
type CareGoal struct {
	ID           string     `json:"id"`
	Description  string     `json:"description"`
	Status       GoalStatus `json:"status"`
	TargetDate   string     `json:"targetDate,omitempty"` // YYYY-MM-DD
	AchievedDate string     `json:"achievedDate,omitempty"`
	Notes        string     `json:"notes,omitempty"`
}

// CareIntervention records a planned intervention or care task.
type CareIntervention struct {
	Description string `json:"description"`
	Frequency   string `json:"frequency,omitempty"` // e.g. "daily", "weekly"
	Responsible string `json:"responsible,omitempty"`
}

// CarePlan represents an aged care plan for a patient.
type CarePlan struct {
	ID             string             `json:"id"`
	PatientID      string             `json:"patientId"`
	PatientNHI     string             `json:"patientNhi"`
	TenantID       string             `json:"tenantId"`
	ResponsibleHPI string             `json:"responsibleHpi"`
	PlanType       CarePlanType       `json:"planType"`
	Status         CarePlanStatus     `json:"status"`
	Goals          []CareGoal         `json:"goals"`
	Interventions  []CareIntervention `json:"interventions"`
	// ClinicalNotes is AES-256-GCM encrypted at rest.
	ClinicalNotes  string             `json:"clinicalNotes,omitempty"`
	StartDate      string             `json:"startDate"` // YYYY-MM-DD
	EndDate        string             `json:"endDate,omitempty"`
	NextReviewDate string             `json:"nextReviewDate,omitempty"`
	FacilityName   string             `json:"facilityName,omitempty"` // for residential plans
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

type carePlanCreateRequest struct {
	PatientID      string             `json:"patientId"`
	PatientNHI     string             `json:"patientNhi"`
	ResponsibleHPI string             `json:"responsibleHpi"`
	PlanType       CarePlanType       `json:"planType"`
	Goals          []CareGoal         `json:"goals,omitempty"`
	Interventions  []CareIntervention `json:"interventions,omitempty"`
	ClinicalNotes  string             `json:"clinicalNotes,omitempty"`
	StartDate      string             `json:"startDate"`
	EndDate        string             `json:"endDate,omitempty"`
	NextReviewDate string             `json:"nextReviewDate,omitempty"`
	FacilityName   string             `json:"facilityName,omitempty"`
}

type carePlanUpdateRequest struct {
	Status         CarePlanStatus     `json:"status,omitempty"`
	Goals          []CareGoal         `json:"goals,omitempty"`
	Interventions  []CareIntervention `json:"interventions,omitempty"`
	ClinicalNotes  string             `json:"clinicalNotes,omitempty"`
	EndDate        string             `json:"endDate,omitempty"`
	NextReviewDate string             `json:"nextReviewDate,omitempty"`
	FacilityName   string             `json:"facilityName,omitempty"`
}

type carePlanRecord struct {
	ID             string
	PatientID      string
	PatientNHI     string
	TenantID       string
	ResponsibleHPI string
	PlanType       CarePlanType
	Status         CarePlanStatus
	Goals          []CareGoal
	Interventions  []CareIntervention
	NotesEncrypted []byte
	StartDate      string
	EndDate        string
	NextReviewDate string
	FacilityName   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
