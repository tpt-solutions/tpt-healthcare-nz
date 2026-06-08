// Package counselling provides addiction-specific counselling domain models.
package counselling

import "time"

// SessionType distinguishes individual vs group modalities.
type SessionType string

const (
	SessionIndividual SessionType = "individual"
	SessionGroup      SessionType = "group"
	SessionFamily     SessionType = "family"
)

// CounsellingSession records an addiction counselling encounter.
type CounsellingSession struct {
	ID              string      `json:"id"`
	PatientNHI      string      `json:"patientNhi"`
	ClinicianID     string      `json:"clinicianId"`
	PracticeID      string      `json:"practiceId"`
	SessionType     SessionType `json:"sessionType"`
	GroupSessionID  *string     `json:"groupSessionId,omitempty"`
	SessionDate     time.Time   `json:"sessionDate"`
	DurationMin     int         `json:"durationMin"`
	Modality        string      `json:"modality"`        // motivational_interviewing, cbt, act, relapse_prevention, harm_reduction
	PresentingIssue string      `json:"presentingIssue"`
	ClinicalNotes   string      `json:"clinicalNotes"`
	RiskAssessment  string      `json:"riskAssessment,omitempty"`
	ReadinessScore  int         `json:"readinessScore"`  // 1-10 (URICA / Ruler-based readiness)
	HomeworkGiven   string      `json:"homeworkGiven,omitempty"`
	NextSessionDate *time.Time  `json:"nextSessionDate,omitempty"`
	BillingType     string      `json:"billingType"`     // dhb_funded, acc, private, pro_bono
	FeeInCents      int         `json:"feeInCents"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
}

// GroupSession is a scheduled group counselling event.
type GroupSession struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`        // e.g. "Relapse Prevention — Wednesday 10am"
	ClinicianID string    `json:"clinicianId"`
	PracticeID  string    `json:"practiceId"`
	ScheduledAt time.Time `json:"scheduledAt"`
	DurationMin int       `json:"durationMin"`
	Topic       string    `json:"topic"`       // relapse_prevention, grief, harm_reduction, life_skills
	MaxAttendees int      `json:"maxAttendees"`
	Attendees   []string  `json:"attendees"`   // patient NHIs
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
