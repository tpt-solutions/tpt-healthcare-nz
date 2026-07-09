// Package acc provides ACC dental-specific claim types and validation for
// injury-related dental treatment in New Zealand.
//
// ACC covers dental injuries from accidents, including:
//   - Fractured/chipped teeth from trauma
//   - Dental treatment needed due to injury
//   - Post-accident restorative work
//   - TMJ injuries from accidents
//
// Claim types reference ACC dental subsidy schedule codes (A1-K1).
package acc

import (
	"encoding/json"
	"fmt"
	"time"
)

// ClaimType identifies the type of ACC dental claim.
type ClaimType string

const (
	ClaimTypeDentalInjury    ClaimType = "dental_injury"    // ACC45 for dental injury
	ClaimTypeTreatmentInjury ClaimType = "treatment_injury" // ACC6 for dental treatment injury
	ClaimTypeTMJ             ClaimType = "tmj_injury"       // TMJ/dental accident claim
)

// ClaimStatus tracks the lifecycle of an ACC dental claim.
type ClaimStatus string

const (
	ClaimDraft     ClaimStatus = "draft"
	ClaimSubmitted ClaimStatus = "submitted"
	ClaimAccepted  ClaimStatus = "accepted"
	ClaimDeclined  ClaimStatus = "declined"
	ClaimPartial   ClaimStatus = "partial" // partially accepted
)

// ToothInjury describes a specific tooth injury on a claim.
type ToothInjury struct {
	ToothCode    string `json:"toothCode"`    // FDI two-digit code
	Surface      string `json:"surface"`      // affected surfaces e.g. "MOD"
	InjuryType   string `json:"injuryType"`   // fracture, avulsion, luxation, subluxation, soft_tissue
	Diagnosis    string `json:"diagnosis"`    // clinical diagnosis
	ACCProcedure string `json:"accProcedure"` // ACC treatment code (A1-K1)
	DCNZCode     string `json:"dcnzCode"`     // DCNZ procedure code performed
	FeeInCents   int    `json:"feeInCents"`   // fee charged in NZ cents
}

// DentalClaim represents an ACC dental claim for injury-related treatment.
type DentalClaim struct {
	ID             string        `json:"id"`
	ClaimType      ClaimType     `json:"claimType"`
	AccidentDate   time.Time     `json:"accidentDate"`
	AccidentDesc   string        `json:"accidentDesc"`
	PatientNHI     string        `json:"patientNhi"`
	ProviderHPI    string        `json:"providerHpi"`
	PracticeID     string        `json:"practiceId"`
	Teeth          []ToothInjury `json:"teeth"`
	TotalFee       int           `json:"totalFee"`
	ACCSubsidy     int           `json:"accSubsidy"`
	PatientCoPay   int           `json:"patientCoPay"`
	Status         ClaimStatus   `json:"status"`
	ACCClaimNumber string        `json:"accClaimNumber,omitempty"`
	ACCFormNumber  string        `json:"accFormNumber"` // ACC45 or ACC6
	Notes          string        `json:"notes,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

// ValidationError holds per-field validation failures.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationResult is the result of validating a dental claim before submission.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Validate performs pre-submission validation of the dental ACC claim.
func (c *DentalClaim) Validate() *ValidationResult {
	result := &ValidationResult{Valid: true}

	if c.PatientNHI == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field: "patientNhi", Message: "Patient NHI is required", Code: "MISSING_NHI",
		})
	}
	if c.ProviderHPI == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field: "providerHpi", Message: "Provider HPI is required", Code: "MISSING_HPI",
		})
	}
	if c.PracticeID == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field: "practiceId", Message: "Practice ID is required", Code: "MISSING_PRACTICE",
		})
	}
	if c.AccidentDate.IsZero() {
		result.Errors = append(result.Errors, ValidationError{
			Field: "accidentDate", Message: "Accident date is required", Code: "MISSING_ACCIDENT_DATE",
		})
	}
	if c.AccidentDate.After(time.Now()) {
		result.Errors = append(result.Errors, ValidationError{
			Field: "accidentDate", Message: "Accident date cannot be in the future", Code: "FUTURE_ACCIDENT_DATE",
		})
	}
	if len(c.Teeth) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field: "teeth", Message: "At least one tooth injury must be specified", Code: "MISSING_TEETH",
		})
	}
	if c.ACCFormNumber == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field: "accFormNumber", Message: "ACC form number (ACC45 or ACC6) is required", Code: "MISSING_FORM",
		})
	}

	for i, t := range c.Teeth {
		if t.ToothCode == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("teeth[%d].toothCode", i),
				Message: "Tooth code is required",
				Code:    "MISSING_TOOTH_CODE",
			})
		}
		if t.FeeInCents <= 0 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("teeth[%d].feeInCents", i),
				Message: "Fee must be greater than 0",
				Code:    "INVALID_FEE",
			})
		}
	}

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	// Recalculate totals
	c.TotalFee = 0
	for _, t := range c.Teeth {
		c.TotalFee += t.FeeInCents
	}

	return result
}

// BuildACCClaimPayload constructs the FHIR Claim payload for ACC dental submission,
// adapting the dental-specific fields into the generic ACC claim format.
func (c *DentalClaim) BuildACCClaimPayload() map[string]any {
	diagnoses := make([]any, 0, len(c.Teeth))
	items := make([]any, 0, len(c.Teeth))

	for i, t := range c.Teeth {
		diagnoses = append(diagnoses, map[string]any{
			"sequence": i + 1,
			"diagnosisCodeableConcept": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "https://standards.digital.health.nz/ns/acc-dental-code",
						"code":   t.ACCProcedure,
					},
				},
				"text": t.Diagnosis,
			},
		})

		items = append(items, map[string]any{
			"sequence":     i + 1,
			"servicedDate": c.AccidentDate.Format("2006-01-02"),
			"productOrService": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "https://standards.digital.health.nz/ns/dcnz-procedure",
						"code":   t.DCNZCode,
					},
				},
			},
			"unitPrice": map[string]any{
				"value":    float64(t.FeeInCents) / 100.0,
				"currency": "NZD",
			},
			"net": map[string]any{
				"value":    float64(t.FeeInCents) / 100.0,
				"currency": "NZD",
			},
			"bodySite": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "https://standards.digital.health.nz/ns/fdi-tooth",
						"code":   t.ToothCode,
					},
					map[string]any{
						"system": "https://standards.digital.health.nz/ns/fdi-surface",
						"code":   t.Surface,
					},
				},
			},
		})
	}

	return map[string]any{
		"resourceType": "Claim",
		"status":       "active",
		"type": map[string]any{
			"coding": []any{
				map[string]any{
					"system": "http://terminology.hl7.org/CodeSystem/claim-type",
					"code":   "professional",
				},
			},
		},
		"use":     "claim",
		"created": time.Now().UTC().Format(time.RFC3339),
		"patient": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  c.PatientNHI,
			},
		},
		"provider": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/hpi-person-id",
				"value":  c.ProviderHPI,
			},
		},
		"priority": map[string]any{
			"coding": []any{
				map[string]any{
					"code": "normal",
				},
			},
		},
		"accident": map[string]any{
			"date":        c.AccidentDate.Format("2006-01-02"),
			"description": c.AccidentDesc,
		},
		"diagnosis": diagnoses,
		"item":      items,
		"total": map[string]any{
			"value":    float64(c.TotalFee) / 100.0,
			"currency": "NZD",
		},
		"identifier": []any{
			map[string]any{
				"system": "https://standards.digital.health.nz/ns/acc-form-number",
				"value":  c.ACCFormNumber,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// ACC Dental Injury Type Lookup
// ---------------------------------------------------------------------------

// DentalInjuryType describes a type of dental injury recognised by ACC.
type DentalInjuryType struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// DentalInjuryTypes returns the standard ACC dental injury classifications.
func DentalInjuryTypes() []DentalInjuryType {
	return []DentalInjuryType{
		{Code: "fracture_crown", Description: "Fracture of tooth crown (enamel/dentine)"},
		{Code: "fracture_crown_root", Description: "Fracture of tooth crown involving root"},
		{Code: "fracture_root", Description: "Fracture of tooth root"},
		{Code: "avulsion", Description: "Complete tooth displacement (avulsion)"},
		{Code: "luxation", Description: "Tooth luxation (displacement)"},
		{Code: "subluxation", Description: "Tooth subluxation (loosening)"},
		{Code: "intrusion", Description: "Tooth intrusion (driven into socket)"},
		{Code: "extrusion", Description: "Tooth extrusion (partial avulsion)"},
		{Code: "soft_tissue_laceration", Description: "Oral soft tissue laceration"},
		{Code: "alveolar_fracture", Description: "Alveolar bone fracture"},
		{Code: "tmj_injury", Description: "Temporomandibular joint injury"},
		{Code: "dental_prosthesis_damage", Description: "Damage to existing dental prosthesis (crown/bridge/denture)"},
	}
}

// ToJSON serialises the claim to JSON for API responses.
func (c *DentalClaim) ToJSON() (string, error) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", fmt.Errorf("acc: marshal dental claim: %w", err)
	}
	return string(b), nil
}
