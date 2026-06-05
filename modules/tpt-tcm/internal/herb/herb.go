// Package herb provides Chinese herb dispensing and prescription management for TCM.
package herb

type Herb struct {
	ID             string `json:"id"`
	PinYin         string `json:"pinYin"`
	LatinName      string `json:"latinName"`
	EnglishName    string `json:"englishName"`
	ChineseChar    string `json:"chineseChar"`
	Category       string `json:"category"`       // jie_biao, qing_re, xie_xia, qu_feng, etc.
	Nature         string `json:"nature"`         // han, liang, wen, re, ping
	Taste          string `json:"taste"`           // xin, gan, suan, ku, xian
	Meridian       string `json:"meridian"`        // lung, spleen, liver, kidney, heart, etc.
	Dosage         string `json:"dosage"`          // standard dosage range
	Contraindications string `json:"contraindications,omitempty"`
	Active         bool   `json:"active"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

type Prescription struct {
	ID          string        `json:"id"`
	PatientNHI  string        `json:"patientNhi"`
	ClinicianID string        `json:"clinicianId"`
	PracticeID  string        `json:"practiceId"`
	Name        string        `json:"name"`         // prescription name
	Herbs       []HerbDose    `json:"herbs"`
	Decoction   string        `json:"decoction"`    // preparation instructions
	Frequency   string        `json:"frequency"`    // daily dose frequency
	Courses     int           `json:"courses"`      // number of courses
	Status      string        `json:"status"`       // active, completed, discontinued
	CreatedAt   int64         `json:"createdAt"`
	UpdatedAt   int64         `json:"updatedAt"`
}

type HerbDose struct {
	HerbID   string `json:"herbId"`
	Name     string `json:"name"`
	AmountG  string `json:"amountG"` // amount in grams
	Role     string `json:"role"`    // jun, chen, zuo, shi (emperor, minister, assistant, guide)
}