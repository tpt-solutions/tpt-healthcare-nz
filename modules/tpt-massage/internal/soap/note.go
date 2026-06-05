// Package soap provides SOAP note documentation for massage therapy.
package soap

type SOAPNote struct {
	ID          string   `json:"id"`
	PatientNHI  string   `json:"patientNhi"`
	VisitID     string   `json:"visitId"`
	ClinicianID string   `json:"clinicianId"`
	PracticeID  string   `json:"practiceId"`
	Date        int64    `json:"date"`
	Subjective  string   `json:"subjective"`  // patient's description
	Objective   string   `json:"objective"`   // therapist's observations
	Assessment  string   `json:"assessment"`  // clinical impression
	Plan        string   `json:"plan"`        // treatment plan
	AreaTreated string   `json:"areaTreated"`
	Techniques  []string `json:"techniques"`  // swedish, deep_tissue, sports, trigger_point, myofascial, etc.
	DurationMin int      `json:"durationMin"`
	Outcome     string   `json:"outcome"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
}