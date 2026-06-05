// Package diagnosis provides TCM tongue and pulse diagnosis types.
package diagnosis

type TCMDiagnosis struct {
	PatientNHI      string        `json:"patientNhi"`
	VisitID         string        `json:"visitId"`
	ClinicianID     string        `json:"clinicianId"`
	Tongue          TongueAssessment `json:"tongue"`
	Pulse           PulseAssessment  `json:"pulse"`
	Pattern         string        `json:"pattern"`         // TCM pattern differentiation
	ZangFu          string        `json:"zangFu"`          // affected zang-fu organs
	QiStagnation    bool          `json:"qiStagnation"`
	BloodStasis     bool          `json:"bloodStasis"`
	Heat            bool          `json:"heat"`
	Cold            bool          `json:"cold"`
	Deficiency      string        `json:"deficiency"`     // qi, blood, yin, yang
	Excess          string        `json:"excess"`
	CreatedAt       int64         `json:"createdAt"`
	UpdatedAt       int64         `json:"updatedAt"`
}

type TongueAssessment struct {
	BodyColor   string `json:"bodyColor"`   // pale, red, dark_red, purple
	Shape       string `json:"shape"`       // thin, swollen, cracked, tooth_marked
	Coating     string `json:"coating"`     // white, yellow, gray, black, absent
	CoatingQuality string `json:"coatingQuality"` // thin, thick, greasy, dry, moist
	Sublingual  string `json:"sublingual"`  // varicosity, normal
	Notes       string `json:"notes"`
}

type PulseAssessment struct {
	Position    string `json:"position"`    // cun, guan, chi (left/right)
	Depth       string `json:"depth"`       // floating, deep
	Rate        string `json:"rate"`        // slow, moderate, rapid
	Force       string `json:"force"`       // forceful, weak
	Quality     string `json:"quality"`     // slippery, wiry, thready, choppy, full, hollow
	Notes       string `json:"notes"`
}