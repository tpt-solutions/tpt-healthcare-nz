package acc

import "time"

type Claim struct {
	ID            string    `json:"id"`
	PatientNHI    string    `json:"patientNhi"`
	ProviderHPI   string    `json:"providerHpi"`
	PracticeID    string    `json:"practiceId"`
	AccidentDate  time.Time `json:"accidentDate"`
	InjuryDesc    string    `json:"injuryDesc"`
	BodyRegion    string    `json:"bodyRegion"`
	SessionCount  int       `json:"sessionCount"`
	TotalFee      int       `json:"totalFee"`
	Status        string    `json:"status"`
	ACCClaimNumber string   `json:"accClaimNumber,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}