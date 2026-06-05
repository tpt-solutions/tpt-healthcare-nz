// Package refraction implements NZ optometry refraction and spectacle/contact lens
// prescription management, covering sphere, cylinder, axis, prism, and ADD power
// entries for distance and near vision.
package refraction

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// Eye identifies the left or right eye.
type Eye string

const (
	EyeRight Eye = "right"
	EyeLeft  Eye = "left"
)

// PrescriptionType distinguishes between spectacle and contact lens orders.
type PrescriptionType string

const (
	Spectacle PrescriptionType = "spectacle"
	Contact   PrescriptionType = "contact"
)

// RefractionMethod documents the technique used to determine the prescription.
type RefractionMethod string

const (
	MethodRetinoscopy    RefractionMethod = "retinoscopy"
	MethodAutorefractor  RefractionMethod = "autorefractor"
	MethodSubjective     RefractionMethod = "subjective"
	MethodCycloplegic    RefractionMethod = "cycloplegic"
	MethodContactLensFit RefractionMethod = "contact_lens_fit"
)

// DistanceType indicates whether the prescription is for distance or near.
type DistanceType string

const (
	Distance DistanceType = "distance"
	Near     DistanceType = "near"
	Intermediate DistanceType = "intermediate"
)

// PrismDirection describes the base orientation of a prism correction.
type PrismDirection string

const (
	BaseIn  PrismDirection = "BU" // base up
	BaseDown PrismDirection = "BD" // base down
	BaseInn PrismDirection = "BI" // base in (nasal)
	BaseOut  PrismDirection = "BO" // base out (temporal)
)

// EyePrescription holds the full refraction values for one eye.
type EyePrescription struct {
	Sphere       float64         `json:"sphere"`       // dioptres (0.25D steps)
	Cylinder     float64         `json:"cylinder"`     // dioptres (optional)
	Axis         int             `json:"axis"`          // degrees 1–180 (optional)
	Prism        float64         `json:"prism"`         // prism dioptres (optional)
	PrismDir     PrismDirection  `json:"prismDir,omitempty"`
	ADD          float64         `json:"add"`           // near ADD power (dioptres)
	VisualAcuity string          `json:"visualAcuity"`  // e.g. "6/6", "6/9"
	Method       RefractionMethod `json:"method"`
	Notes        string          `json:"notes,omitempty"`
}

// Prescription is a complete refraction prescription for a patient encounter.
type Prescription struct {
	ID               string            `json:"id"`
	TenantID         string            `json:"tenantId"`
	PatientNHI       string            `json:"patientNhi"`
	ClinicianID      string            `json:"clinicianId"`
	PracticeID       string            `json:"practiceId"`
	Type             PrescriptionType  `json:"type"`
	Distance         DistanceType      `json:"distance"`
	RightEye         EyePrescription   `json:"rightEye"`
	LeftEye          EyePrescription   `json:"leftEye"`
	IssuedDate       int64             `json:"issuedDate"`
	ExpiryDate       int64             `json:"expiryDate"`
	IsCurrent        bool              `json:"isCurrent"`
	CreatedAt        int64             `json:"createdAt"`
	UpdatedAt        int64             `json:"updatedAt"`
	FHIRResource     json.RawMessage   `json:"fhirResource,omitempty"`
	FHIRVersion      int               `json:"fhirVersion,omitempty"`
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// Validate checks all prescription fields for clinical consistency per NZ optometry standards.
func (p *Prescription) Validate() error {
	if p.PatientNHI == "" {
		return fmt.Errorf("refraction: patient NHI is required")
	}
	if p.ClinicianID == "" {
		return fmt.Errorf("refraction: clinician ID is required")
	}
	if err := validateEye("right", p.RightEye); err != nil {
		return fmt.Errorf("refraction: right eye: %w", err)
	}
	if err := validateEye("left", p.LeftEye); err != nil {
		return fmt.Errorf("refraction: left eye: %w", err)
	}
	if p.IssuedDate == 0 {
		return fmt.Errorf("refraction: issued date is required")
	}
	return nil
}

func validateEye(label string, e EyePrescription) error {
	// Sphere must be a multiple of 0.25 (standard optometry)
	if math.Mod(e.Sphere*4, 1) != 0 {
		return fmt.Errorf("%s sphere %.2f must be in 0.25D steps", label, e.Sphere)
	}
	if e.Cylinder != 0 {
		if math.Mod(e.Cylinder*4, 1) != 0 {
			return fmt.Errorf("%s cylinder %.2f must be in 0.25D steps", label, e.Cylinder)
		}
		if e.Axis < 1 || e.Axis > 180 {
			return fmt.Errorf("%s axis %d must be 1–180 degrees", label, e.Axis)
		}
	}
	if e.Prism != 0 {
		if e.PrismDir == "" {
			return fmt.Errorf("%s prism direction is required when prism value is set", label)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Power calculation helpers
// ---------------------------------------------------------------------------

// SphericalEquivalent returns the spherical equivalent: sphere + (cylinder / 2).
func (e *EyePrescription) SphericalEquivalent() float64 {
	return math.Round((e.Sphere+e.Cylinder/2)*4) / 4
}

// FormatSphere formats sphere with sign, e.g. "+2.00", "-1.25", "PL" for plano.
func (e *EyePrescription) FormatSphere() string {
	if e.Sphere == 0 {
		return "PL"
	}
	sign := "+"
	if e.Sphere < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.2f", sign, e.Sphere)
}

// ---------------------------------------------------------------------------
// Test chart / LogMAR conversion helpers
// ---------------------------------------------------------------------------

// SnellenToLogMAR converts a Snellen fraction (e.g. "6/6") to LogMAR.
func SnellenToLogMAR(snellen string) (float64, error) {
	snellen = strings.TrimSpace(snellen)
	parts := strings.Split(snellen, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("refraction: invalid Snellen format %q (expected e.g. 6/6)", snellen)
	}
	var numerator, denominator float64
	if _, err := fmt.Sscanf(parts[0], "%f", &numerator); err != nil {
		return 0, fmt.Errorf("refraction: invalid Snellen numerator %q", parts[0])
	}
	if _, err := fmt.Sscanf(parts[1], "%f", &denominator); err != nil {
		return 0, fmt.Errorf("refraction: invalid Snellen denominator %q", parts[1])
	}
	if numerator == 0 || denominator == 0 {
		return 0, fmt.Errorf("refraction: Snellen fraction cannot have zero")
	}
	return math.Log10(denominator / numerator), nil
}

// ---------------------------------------------------------------------------
// Convenience constructors
// ---------------------------------------------------------------------------

// NewPrescription creates a new Prescription with issued date set to now.
func NewPrescription() *Prescription {
	now := time.Now().UnixMilli()
	return &Prescription{
		IssuedDate: now,
		ExpiryDate: time.Now().AddDate(1, 0, 0).UnixMilli(), // standard 1-year expiry
		IsCurrent:  true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// ---------------------------------------------------------------------------
// Common NZ spectacle lens categories
// ---------------------------------------------------------------------------

// LensIndex describes common spectacle lens material indices.
type LensIndex string

const (
	IndexStandard  LensIndex = "1.50" // CR-39 standard plastic
	IndexMid       LensIndex = "1.60" // mid-index
	IndexHigh      LensIndex = "1.67" // high-index
	IndexUltraHigh LensIndex = "1.74" // ultra-high-index
)

// LensCoating describes optional lens coatings.
type LensCoating string

const (
	CoatingAntiReflective LensCoating = "anti_reflective"
	CoatingScratchResist  LensCoating = "scratch_resistant"
	CoatingUVProtection   LensCoating = "uv_protection"
	CoatingBlueBlock      LensCoating = "blue_light_block"
	CoatingPhotochromic   LensCoating = "photochromic"
	CoatingPolarised      LensCoating = "polarised"
	CoatingMirror         LensCoating = "mirror"
)

// ---------------------------------------------------------------------------
// FHIR R5 Mapping
// ---------------------------------------------------------------------------

// ToFHIRObservation converts the prescription to a FHIR R5 Observation resource
// for refraction/prescription data.
func (p *Prescription) ToFHIRObservation() map[string]any {
	issuedTime := time.UnixMilli(p.IssuedDate).Format(time.RFC3339)
	
	obs := map[string]any{
		"resourceType": "Observation",
		"id":           p.ID,
		"meta": map[string]any{
			"versionId": fmt.Sprintf("%d", p.FHIRVersion),
			"lastUpdated": time.UnixMilli(p.UpdatedAt).Format(time.RFC3339),
			"profile": []string{
				"https://nzfhir.org/StructureDefinition/nz-vision-prescription",
			},
		},
		"status": "final",
		"category": []map[string]any{
			{
				"coding": []map[string]any{
					{
						"system":  "http://terminology.hl7.org/CodeSystem/observation-category",
						"code":    "vision",
						"display": "Vision",
					},
				},
			},
		},
		"code": map[string]any{
			"coding": []map[string]any{
				{
					"system":  "http://loinc.org",
					"code":    "28854-6",
					"display": "Refraction",
				},
				{
					"system":  "https://nzfhir.org/CodeSystem/vision-prescription-type",
					"code":    string(p.Type),
					"display": string(p.Type),
				},
			},
			"text": fmt.Sprintf("%s prescription for %s vision", p.Type, p.Distance),
		},
		"subject": map[string]any{
			"reference": fmt.Sprintf("Patient/%s", p.PatientNHI),
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  p.PatientNHI,
			},
		},
		"encounter": map[string]any{
			"reference": fmt.Sprintf("Encounter/%s", p.ID),
		},
		"effectiveDateTime": issuedTime,
		"issued":            issuedTime,
		"performer": []map[string]any{
			{
				"reference": fmt.Sprintf("Practitioner/%s", p.ClinicianID),
			},
		},
		"component": []map[string]any{
			p.eyeToFHIRComponent("right", p.RightEye),
			p.eyeToFHIRComponent("left", p.LeftEye),
		},
		"note": []map[string]any{},
	}

	// Add notes if present
	if p.RightEye.Notes != "" {
		obs["note"] = append(obs["note"].([]map[string]any), map[string]any{
			"text": fmt.Sprintf("Right eye: %s", p.RightEye.Notes),
		})
	}
	if p.LeftEye.Notes != "" {
		obs["note"] = append(obs["note"].([]map[string]any), map[string]any{
			"text": fmt.Sprintf("Left eye: %s", p.LeftEye.Notes),
		})
	}

	// Add extension for prescription metadata
	obs["extension"] = []map[string]any{
		{
			"url": "https://nzfhir.org/StructureDefinition/nz-vision-prescription-distance",
			"valueCode": string(p.Distance),
		},
		{
			"url": "https://nzfhir.org/StructureDefinition/nz-vision-prescription-expiry",
			"valueDateTime": time.UnixMilli(p.ExpiryDate).Format(time.RFC3339),
		},
		{
			"url": "https://nzfhir.org/StructureDefinition/nz-vision-prescription-current",
			"valueBoolean": p.IsCurrent,
		},
	}

	return obs
}

func (p *Prescription) eyeToFHIRComponent(eye string, ep EyePrescription) map[string]any {
	eyeCode := "right"
	eyeDisplay := "Right eye"
	if eye == "left" {
		eyeCode = "left"
		eyeDisplay = "Left eye"
	}

	component := map[string]any{
		"code": map[string]any{
			"coding": []map[string]any{
				{
					"system":  "http://loinc.org",
					"code":    "28854-6",
					"display": "Refraction",
				},
			},
			"text": fmt.Sprintf("%s eye refraction", eyeDisplay),
		},
		"valueQuantity": map[string]any{
			"value":  ep.Sphere,
			"unit":   "diopter",
			"system": "http://unitsofmeasure.org",
			"code":   "D",
		},
	}

	// Add cylinder and axis as sub-components
	if ep.Cylinder != 0 {
		component["component"] = []map[string]any{
			{
				"code": map[string]any{
					"coding": []map[string]any{
						{
							"system":  "http://loinc.org",
							"code":    "28855-3",
							"display": "Cylinder",
						},
					},
				},
				"valueQuantity": map[string]any{
					"value":  ep.Cylinder,
					"unit":   "diopter",
					"system": "http://unitsofmeasure.org",
					"code":   "D",
				},
			},
			{
				"code": map[string]any{
					"coding": []map[string]any{
						{
							"system":  "http://loinc.org",
							"code":    "28856-1",
							"display": "Axis",
						},
					},
				},
				"valueQuantity": map[string]any{
					"value":  ep.Axis,
					"unit":   "degree",
					"system": "http://unitsofmeasure.org",
					"code":   "deg",
				},
			},
		}
	}

	// Add prism if present
	if ep.Prism != 0 {
		if component["component"] == nil {
			component["component"] = []map[string]any{}
		}
		comp := component["component"].([]map[string]any)
		comp = append(comp, map[string]any{
			"code": map[string]any{
				"coding": []map[string]any{
					{
						"system":  "http://loinc.org",
						"code":    "28857-9",
						"display": "Prism",
					},
				},
			},
			"valueQuantity": map[string]any{
				"value":  ep.Prism,
				"unit":   "prism diopter",
				"system": "http://unitsofmeasure.org",
				"code":   "pD",
			},
		})
		component["component"] = comp
	}

	// Add ADD power if present
	if ep.ADD != 0 {
		if component["component"] == nil {
			component["component"] = []map[string]any{}
		}
		comp := component["component"].([]map[string]any)
		comp = append(comp, map[string]any{
			"code": map[string]any{
				"coding": []map[string]any{
					{
						"system":  "http://loinc.org",
						"code":    "28858-7",
						"display": "Add power",
					},
				},
			},
			"valueQuantity": map[string]any{
				"value":  ep.ADD,
				"unit":   "diopter",
				"system": "http://unitsofmeasure.org",
				"code":   "D",
			},
		})
		component["component"] = comp
	}

	// Add visual acuity
	if ep.VisualAcuity != "" {
		if component["component"] == nil {
			component["component"] = []map[string]any{}
		}
		comp := component["component"].([]map[string]any)
		comp = append(comp, map[string]any{
			"code": map[string]any{
				"coding": []map[string]any{
					{
						"system":  "http://loinc.org",
						"code":    "28859-5",
						"display": "Visual acuity",
					},
				},
			},
			"valueString": ep.VisualAcuity,
		})
		component["component"] = comp
	}

	// Add method
	if ep.Method != "" {
		if component["component"] == nil {
			component["component"] = []map[string]any{}
		}
		comp := component["component"].([]map[string]any)
		comp = append(comp, map[string]any{
			"code": map[string]any{
				"coding": []map[string]any{
					{
						"system":  "https://nzfhir.org/CodeSystem/refraction-method",
						"code":    string(ep.Method),
						"display": string(ep.Method),
					},
				},
			},
			"valueString": string(ep.Method),
		})
		component["component"] = comp
	}

	return component
}
