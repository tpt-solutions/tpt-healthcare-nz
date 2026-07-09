// Package bodycomp provides body composition tracking for dietetics and nutrition.
package bodycomp

type BodyComposition struct {
	ID           string  `json:"id"`
	PatientNHI   string  `json:"patientNhi"`
	Date         int64   `json:"date"`
	WeightKg     float64 `json:"weightKg"`
	HeightCm     float64 `json:"heightCm"`
	BMI          float64 `json:"bmi"`
	BodyFatPct   float64 `json:"bodyFatPct"`
	MuscleMass   float64 `json:"muscleMass"`
	BoneMass     float64 `json:"boneMass"`
	BodyWaterPct float64 `json:"bodyWaterPct"`
	WaistCm      float64 `json:"waistCm"`
	HipCm        float64 `json:"hipCm"`
	WHR          float64 `json:"whr"`         // waist-to-hip ratio
	VisceralFat  int     `json:"visceralFat"` // visceral fat level
	BMR          int     `json:"bmr"`         // basal metabolic rate
	MetabolicAge int     `json:"metabolicAge"`
	ClinicianID  string  `json:"clinicianId"`
	Notes        string  `json:"notes,omitempty"`
	CreatedAt    int64   `json:"createdAt"`
	UpdatedAt    int64   `json:"updatedAt"`
}
