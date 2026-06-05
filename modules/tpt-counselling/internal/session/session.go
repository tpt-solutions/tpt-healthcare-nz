// Package session provides counselling session note management.
package session

type Session struct {
	ID               string   `json:"id"`
	ClientNHI        string   `json:"clientNhi"`
	ClinicianID      string   `json:"clinicianId"`
	PracticeID       string   `json:"practiceId"`
	SessionDate      int64    `json:"sessionDate"`
	SessionNumber    int      `json:"sessionNumber"`
	Modality         string   `json:"modality"`        // CBT, ACT, DBT, person_centred, psychodynamic, EMDR, etc.
	Mode             string   `json:"mode"`            // in_person, video, phone
	DurationMin      int      `json:"durationMin"`
	PresentingIssue  string   `json:"presentingIssue"`
	ClinicalNotes    string   `json:"clinicalNotes"`
	RiskAssessment   string   `json:"riskAssessment,omitempty"`
	Intervention     string   `json:"intervention"`
	Outcome          string   `json:"outcome"`
	Homework         string   `json:"homework,omitempty"`
	NextSessionDate  int64    `json:"nextSessionDate,omitempty"`
	BillingType      string   `json:"billingType"`    // eap, private, acc, pro_bono
	FeeInCents       int      `json:"feeInCents"`
	CreatedAt        int64    `json:"createdAt"`
	UpdatedAt        int64    `json:"updatedAt"`
}