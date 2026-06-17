package api

import "time"

// CounsellingSession maps addiction_counselling_sessions.
type CounsellingSession struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenantId"`
	PatientNHI      string     `json:"patientNhi"`
	ClinicianID     string     `json:"clinicianId"`
	PracticeID      string     `json:"practiceId"`
	SessionType     string     `json:"sessionType"`
	GroupSessionID  *string    `json:"groupSessionId,omitempty"`
	SessionDate     time.Time  `json:"sessionDate"`
	DurationMin     int        `json:"durationMin"`
	Modality        string     `json:"modality"`
	PresentingIssue string     `json:"presentingIssue"`
	ClinicalNotes   string     `json:"clinicalNotes,omitempty"`
	RiskAssessment  string     `json:"riskAssessment,omitempty"`
	ReadinessScore  *int       `json:"readinessScore,omitempty"`
	HomeworkGiven   string     `json:"homeworkGiven,omitempty"`
	NextSessionDate *time.Time `json:"nextSessionDate,omitempty"`
	BillingType     string     `json:"billingType"`
	FeeInCents      int        `json:"feeInCents"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

const sessionSelectCols = `id, tenant_id, patient_nhi, clinician_id, practice_id,
       session_type, group_session_id, session_date, duration_min, modality,
       presenting_issue, COALESCE(clinical_notes,''), COALESCE(risk_assessment,''),
       readiness_score, COALESCE(homework_given,''), next_session_date,
       billing_type, fee_in_cents, created_at, updated_at`

func scanSession(row interface{ Scan(...any) error }, s *CounsellingSession) error {
	return row.Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.ClinicianID, &s.PracticeID,
		&s.SessionType, &s.GroupSessionID, &s.SessionDate, &s.DurationMin, &s.Modality,
		&s.PresentingIssue, &s.ClinicalNotes, &s.RiskAssessment,
		&s.ReadinessScore, &s.HomeworkGiven, &s.NextSessionDate,
		&s.BillingType, &s.FeeInCents, &s.CreatedAt, &s.UpdatedAt,
	)
}

// GroupSession maps addiction_group_sessions.
type GroupSession struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenantId"`
	Name         string    `json:"name"`
	ClinicianID  string    `json:"clinicianId"`
	PracticeID   string    `json:"practiceId"`
	ScheduledAt  time.Time `json:"scheduledAt"`
	DurationMin  int       `json:"durationMin"`
	Topic        string    `json:"topic"`
	MaxAttendees int       `json:"maxAttendees"`
	Attendees    []string  `json:"attendees"`
	Notes        string    `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

const groupSelectCols = `id, tenant_id, name, clinician_id, practice_id,
       scheduled_at, duration_min, topic, max_attendees,
       COALESCE(attendees, '{}'), COALESCE(notes,''), created_at, updated_at`

// TreatmentPlan maps addiction_treatment_plans.
type TreatmentPlan struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenantId"`
	PatientNHI  string    `json:"patientNhi"`
	ProgrammeID *string   `json:"programmeId,omitempty"`
	ClinicianID string    `json:"clinicianId"`
	PracticeID  string    `json:"practiceId"`
	StartDate   time.Time `json:"startDate"`
	ReviewDate  time.Time `json:"reviewDate"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

const planSelectCols = `id, tenant_id, patient_nhi, programme_id, clinician_id, practice_id,
       start_date, review_date, status, created_at, updated_at`

// Goal maps addiction_goals.
type Goal struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenantId"`
	PlanID      string     `json:"planId"`
	Description string     `json:"description"`
	TargetDate  *time.Time `json:"targetDate,omitempty"`
	Status      string     `json:"status"`
	Evidence    string     `json:"evidence,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// RelapseEvent maps addiction_relapses.
type RelapseEvent struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenantId"`
	PlanID        string    `json:"planId"`
	OccurredAt    time.Time `json:"occurredAt"`
	SubstanceUsed string    `json:"substanceUsed"`
	TriggerNotes  string    `json:"triggerNotes,omitempty"`
	Severity      string    `json:"severity"`
	Intervention  string    `json:"intervention,omitempty"`
	PlanModified  bool      `json:"planModified"`
	CreatedAt     time.Time `json:"createdAt"`
}
