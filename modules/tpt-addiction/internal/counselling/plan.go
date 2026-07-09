// Package counselling provides treatment plan and relapse models.
package counselling

import "time"

// TreatmentPlan is the overarching recovery plan for an addiction patient.
type TreatmentPlan struct {
	ID            string         `json:"id"`
	PatientNHI    string         `json:"patientNhi"`
	ProgrammeID   string         `json:"programmeId"` // linked methadone programme if applicable
	ClinicianID   string         `json:"clinicianId"`
	PracticeID    string         `json:"practiceId"`
	StartDate     time.Time      `json:"startDate"`
	ReviewDate    time.Time      `json:"reviewDate"`
	Status        string         `json:"status"` // active, completed, discontinued
	Goals         []Goal         `json:"goals"`
	RelapseEvents []RelapseEvent `json:"relapseEvents,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

// Goal is a SMART goal within a treatment plan.
type Goal struct {
	ID          string     `json:"id"`
	PlanID      string     `json:"planId"`
	Description string     `json:"description"`
	TargetDate  *time.Time `json:"targetDate,omitempty"`
	Status      string     `json:"status"` // not_started, in_progress, achieved, revised
	Evidence    string     `json:"evidence,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// RelapseEvent records a substance use relapse during treatment.
type RelapseEvent struct {
	ID            string    `json:"id"`
	PlanID        string    `json:"planId"`
	OccurredAt    time.Time `json:"occurredAt"`
	SubstanceUsed string    `json:"substanceUsed"`
	TriggerNotes  string    `json:"triggerNotes,omitempty"`
	Severity      string    `json:"severity"`               // mild, moderate, severe
	Intervention  string    `json:"intervention,omitempty"` // what was done in response
	PlanModified  bool      `json:"planModified"`           // true if plan was adjusted as a result
	CreatedAt     time.Time `json:"createdAt"`
}
