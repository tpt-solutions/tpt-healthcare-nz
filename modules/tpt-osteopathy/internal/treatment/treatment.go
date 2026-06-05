// Package treatment provides osteopathic treatment record types.
package treatment

// Treatment records an osteopathic treatment session.
type Treatment struct {
	ID             string     `json:"id"`
	PatientNHI     string     `json:"patientNhi"`
	ClinicianID    string     `json:"clinicianId"`
	PracticeID     string     `json:"practiceId"`
	VisitID        string     `json:"visitId,omitempty"`
	AssessmentID   string     `json:"assessmentId,omitempty"`
	TreatmentDate  int64      `json:"treatmentDate"`
	Techniques     []Technique `json:"techniques"`
	RegionsTreated []string   `json:"regionsTreated"`
	DurationMin    int        `json:"durationMin"`
	ResponseToTx   string     `json:"responseToTreatment"` // immediate patient response
	HomeExercises  string     `json:"homeExercises,omitempty"`
	Advice         string     `json:"advice,omitempty"`
	FollowUpWeeks  int        `json:"followUpWeeks,omitempty"`
	Outcome        string     `json:"outcome"`
	Notes          string     `json:"notes,omitempty"`
	CreatedAt      int64      `json:"createdAt"`
	UpdatedAt      int64      `json:"updatedAt"`
}

// Technique documents an osteopathic technique applied during treatment.
type Technique struct {
	Name        string `json:"name"`       // HVLA, MET, counterstrain, craniosacral, fascial, visceral, lymphatic
	Region      string `json:"region"`
	Description string `json:"description,omitempty"`
	Repetitions int    `json:"repetitions,omitempty"`
	Response    string `json:"response,omitempty"` // patient's response to technique
}
