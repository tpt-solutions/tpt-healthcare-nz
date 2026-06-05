// Package ophthalmology provides data types for ophthalmic examination findings
// including anterior/posterior segment exams, tonometry, pachymetry, visual fields,
// and retinal/OCT imaging relevant to NZ ophthalmology practice.
package ophthalmology

import (
	"encoding/json"
	"fmt"
	"time"
)

// ExamType categorises the type of ophthalmic examination.
type ExamType string

const (
	ExamComprehensive   ExamType = "comprehensive"
	ExamFollowUp        ExamType = "follow_up"
	ExamGlaucoma        ExamType = "glaucoma"
	ExamRetina          ExamType = "retina"
	ExamCataract        ExamType = "cataract"
	ExamCornea          ExamType = "cornea"
	ExamNeuroOphthalmic ExamType = "neuro_ophthalmic"
	ExamPaediatric      ExamType = "paediatric"
	ExamEmergency       ExamType = "emergency"
)

// TonometryMethod documents the technique used for intraocular pressure measurement.
type TonometryMethod string

const (
	TonoGoldmann  TonometryMethod = "goldmann_applanation"
	TonoNonContact TonometryMethod = "non_contact" // air puff
	TonoRebound   TonometryMethod = "rebound"      // iCare
	TonoTonopen   TonometryMethod = "tonopen"
	TonoDigital   TonometryMethod = "digital_palpation"
)

// IOPReading holds a single intraocular pressure measurement.
type IOPReading struct {
	Method    TonometryMethod `json:"method"`
	RightEye  float64         `json:"rightEye"`  // mmHg
	LeftEye   float64         `json:"leftEye"`   // mmHg
	Time      int64           `json:"time"`       // epoch ms
	Notes     string          `json:"notes,omitempty"`
}

// GradingScale is a standard clinical grading (0–4) used for cataract, cells, flare, etc.
type GradingScale int

const (
	GradeNone   GradingScale = 0
	GradeTrace  GradingScale = 1
	GradeMild   GradingScale = 2
	GradeModerate GradingScale = 3
	GradeSevere GradingScale = 4
)

// LensStatus describes the natural lens or IOL status.
type LensStatus string

const (
	LensPhakic     LensStatus = "phakic"
	LensPSC        LensStatus = "posterior_subcapsular"
	LensNuclear    LensStatus = "nuclear_sclerotic"
	LensCortical   LensStatus = "cortical"
	LensPCIOL      LensStatus = "pc_iol"       // posterior chamber IOL
	LensACIOL      LensStatus = "ac_iol"       // anterior chamber IOL
	LensAphakic    LensStatus = "aphakic"
	LensPseudophakic LensStatus = "pseudophakic"
)

// OpticDiscAppearance describes optic nerve head findings.
type OpticDiscAppearance string

const (
	DiscNormal       OpticDiscAppearance = "normal"
	DiscCupped       OpticDiscAppearance = "cupped"
	DiscPale         OpticDiscAppearance = "pale"
	DiscOedematous   OpticDiscAppearance = "oedematous"
	DiscDrusen       OpticDiscAppearance = "drusen"
	DiscTilted       OpticDiscAppearance = "tilted"
)

// CupDiscRatio expresses the vertical cup-to-disc ratio (0.0 to 1.0).
type CupDiscRatio float64

// MacularStatus describes central retinal findings.
type MacularStatus string

const (
	MaculaNormal    MacularStatus = "normal"
	MaculaOedema    MacularStatus = "macular_oedema"
	MaculaDrusen    MacularStatus = "macular_drusen"
	MaculaCNV       MacularStatus = "choroidal_neovascularisation"
	MaculaHole      MacularStatus = "macular_hole"
	MaculaPucker    MacularStatus = "epiretinal_membrane"
	MaculaScar      MacularStatus = "macular_scar"
)

// OphthalmicExam captures a complete ophthalmic examination record.
type OphthalmicExam struct {
	ID             string            `json:"id"`
	TenantID       string            `json:"tenantId"`
	PatientNHI     string            `json:"patientNhi"`
	ClinicianID    string            `json:"clinicianId"`
	PracticeID     string            `json:"practiceId"`
	ExamType       ExamType          `json:"examType"`
	ExamDate       int64             `json:"examDate"`
	FHIRResource   json.RawMessage   `json:"fhirResource,omitempty"`
	FHIRVersion    int               `json:"fhirVersion,omitempty"`

	// Visual acuity — recorded as Snellen fractions per eye
	VADistanceRight string             `json:"vaDistanceRight,omitempty"` // e.g. "6/6"
	VADistanceLeft  string             `json:"vaDistanceLeft,omitempty"`
	VANearRight     string             `json:"vaNearRight,omitempty"`     // e.g. "N5"
	VANearLeft      string             `json:"vaNearLeft,omitempty"`
	PinholeRight    string             `json:"pinholeRight,omitempty"`
	PinholeLeft     string             `json:"pinholeLeft,omitempty"`

	// Refraction (autorefractor or retinoscopy findings before subjective)
	AutoRefractionRight string          `json:"autoRefractionRight,omitempty"`
	AutoRefractionLeft  string          `json:"autoRefractionLeft,omitempty"`

	// Tonometry / IOP
	IOP              []IOPReading       `json:"iop,omitempty"`

	// Anterior segment
	LensRight        LensStatus         `json:"lensRight"`
	LensLeft         LensStatus         `json:"lensLeft"`
	CataractGrade    GradingScale       `json:"cataractGrade"`
	CorneaClear      bool               `json:"corneaClear"`
	AnteriorChamber  string             `json:"anteriorChamber,omitempty"` // depth descriptor

	// Posterior segment
	DiscRight        OpticDiscAppearance `json:"discRight"`
	DiscLeft         OpticDiscAppearance `json:"discLeft"`
	CDRatioRight     CupDiscRatio       `json:"cdRatioRight"`     // cup-to-disc ratio
	CDRatioLeft      CupDiscRatio       `json:"cdRatioLeft"`
	MaculaRight      MacularStatus      `json:"maculaRight"`
	MaculaLeft       MacularStatus      `json:"maculaLeft"`

	// Visual fields (normal, constricted, hemianopia, etc.)
	VisualFieldsRight string           `json:"visualFieldsRight,omitempty"`
	VisualFieldsLeft  string           `json:"visualFieldsLeft,omitempty"`

	// OCT imaging report summary
	OCTRight         string             `json:"octRight,omitempty"`
	OCTLeft          string             `json:"octLeft,omitempty"`

	// Diagnosis / impression
	Diagnosis        string             `json:"diagnosis,omitempty"`
	Plan             string             `json:"plan,omitempty"`
	ReferralRequired bool               `json:"referralRequired"`

	// Follow-up
	FollowUpDays     int                `json:"followUpDays,omitempty"`

	CreatedAt        int64              `json:"createdAt"`
	UpdatedAt        int64              `json:"updatedAt"`
}

// NewExam creates a new OphthalmicExam with timestamps initialised.
func NewExam() *OphthalmicExam {
	now := time.Now().UnixMilli()
	return &OphthalmicExam{
		ExamDate:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks required fields for an ophthalmic exam.
func (e *OphthalmicExam) Validate() error {
	if e.PatientNHI == "" {
		return fmt.Errorf("ophthalmology: patient NHI is required")
	}
	if e.ClinicianID == "" {
		return fmt.Errorf("ophthalmology: clinician ID is required")
	}
	if e.ExamDate == 0 {
		return fmt.Errorf("ophthalmology: exam date is required")
	}
	return nil
}

// ---------------------------------------------------------------------------
// FHIR R5 Mapping
// ---------------------------------------------------------------------------

// ToFHIRDiagnosticReport converts the exam to a FHIR R5 DiagnosticReport resource
// for ophthalmic examination findings.
func (e *OphthalmicExam) ToFHIRDiagnosticReport() map[string]any {
	examTime := time.UnixMilli(e.ExamDate).Format(time.RFC3339)
	
	report := map[string]any{
		"resourceType": "DiagnosticReport",
		"id":           e.ID,
		"meta": map[string]any{
			"versionId": fmt.Sprintf("%d", e.FHIRVersion),
			"lastUpdated": time.UnixMilli(e.UpdatedAt).Format(time.RFC3339),
			"profile": []string{
				"https://nzfhir.org/StructureDefinition/nz-ophthalmic-exam",
			},
		},
		"status": "final",
		"category": []map[string]any{
			{
				"coding": []map[string]any{
					{
						"system":  "http://terminology.hl7.org/CodeSystem/v2-0074",
						"code":    "OPH",
						"display": "Ophthalmology",
					},
				},
			},
		},
		"code": map[string]any{
			"coding": []map[string]any{
				{
					"system":  "http://loinc.org",
					"code":    "28854-6",
					"display": "Ophthalmic examination",
				},
				{
					"system":  "https://nzfhir.org/CodeSystem/ophthalmic-exam-type",
					"code":    string(e.ExamType),
					"display": string(e.ExamType),
				},
			},
			"text": fmt.Sprintf("%s ophthalmic examination", e.ExamType),
		},
		"subject": map[string]any{
			"reference": fmt.Sprintf("Patient/%s", e.PatientNHI),
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  e.PatientNHI,
			},
		},
		"encounter": map[string]any{
			"reference": fmt.Sprintf("Encounter/%s", e.ID),
		},
		"effectiveDateTime": examTime,
		"issued":            time.UnixMilli(e.CreatedAt).Format(time.RFC3339),
		"performer": []map[string]any{
			{
				"reference": fmt.Sprintf("Practitioner/%s", e.ClinicianID),
			},
		},
		"result": []map[string]any{},
		"conclusion": e.Diagnosis,
		"conclusionCode": []map[string]any{},
	}

	// Add visual acuity observations
	if e.VADistanceRight != "" || e.VADistanceLeft != "" {
		report["result"] = append(report["result"].([]map[string]any), map[string]any{
			"reference": fmt.Sprintf("Observation/%s-va-distance", e.ID),
			"display":   "Distance visual acuity",
		})
	}
	if e.VANearRight != "" || e.VANearLeft != "" {
		report["result"] = append(report["result"].([]map[string]any), map[string]any{
			"reference": fmt.Sprintf("Observation/%s-va-near", e.ID),
			"display":   "Near visual acuity",
		})
	}

	// Add IOP readings
	if len(e.IOP) > 0 {
		report["result"] = append(report["result"].([]map[string]any), map[string]any{
			"reference": fmt.Sprintf("Observation/%s-iop", e.ID),
			"display":   "Intraocular pressure",
		})
	}

	// Add anterior segment findings
	report["result"] = append(report["result"].([]map[string]any), map[string]any{
		"reference": fmt.Sprintf("Observation/%s-anterior", e.ID),
		"display":   "Anterior segment examination",
	})

	// Add posterior segment findings
	report["result"] = append(report["result"].([]map[string]any), map[string]any{
		"reference": fmt.Sprintf("Observation/%s-posterior", e.ID),
		"display":   "Posterior segment examination",
	})

	// Add visual fields if present
	if e.VisualFieldsRight != "" || e.VisualFieldsLeft != "" {
		report["result"] = append(report["result"].([]map[string]any), map[string]any{
			"reference": fmt.Sprintf("Observation/%s-visual-fields", e.ID),
			"display":   "Visual fields",
		})
	}

	// Add OCT if present
	if e.OCTRight != "" || e.OCTLeft != "" {
		report["result"] = append(report["result"].([]map[string]any), map[string]any{
			"reference": fmt.Sprintf("Observation/%s-oct", e.ID),
			"display":   "OCT imaging",
		})
	}

	// Add extensions for NZ-specific data
	report["extension"] = []map[string]any{
		{
			"url": "https://nzfhir.org/StructureDefinition/nz-ophthalmic-exam-type",
			"valueCode": string(e.ExamType),
		},
		{
			"url": "https://nzfhir.org/StructureDefinition/nz-ophthalmic-exam-referral-required",
			"valueBoolean": e.ReferralRequired,
		},
	}
	if e.FollowUpDays > 0 {
		report["extension"] = append(report["extension"].([]map[string]any), map[string]any{
			"url": "https://nzfhir.org/StructureDefinition/nz-ophthalmic-exam-followup-days",
			"valueInteger": e.FollowUpDays,
		})
	}

	return report
}
