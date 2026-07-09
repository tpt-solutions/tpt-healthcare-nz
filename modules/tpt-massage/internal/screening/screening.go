// Package screening provides contraindication screening for massage therapy.
package screening

type Screening struct {
	ID                     string `json:"id"`
	PatientNHI             string `json:"patientNhi"`
	Date                   int64  `json:"date"`
	ClinicianID            string `json:"clinicianId"`
	Pregnant               bool   `json:"pregnant"`
	RecentSurgery          bool   `json:"recentSurgery"`
	BloodClots             bool   `json:"bloodClots"`
	Osteoporosis           bool   `json:"osteoporosis"`
	Cancer                 bool   `json:"cancer"`
	HeartCondition         bool   `json:"heartCondition"`
	HighBloodPressure      bool   `json:"highBloodPressure"`
	Diabetes               bool   `json:"diabetes"`
	Epilepsy               bool   `json:"epilepsy"`
	SkinConditions         bool   `json:"skinConditions"`
	Allergies              string `json:"allergies,omitempty"`
	Medications            string `json:"medications,omitempty"`
	ContraindicationsFound bool   `json:"contraindicationsFound"`
	Notes                  string `json:"notes,omitempty"`
	ClearedForTreatment    bool   `json:"clearedForTreatment"`
	CreatedAt              int64  `json:"createdAt"`
	UpdatedAt              int64  `json:"updatedAt"`
}
