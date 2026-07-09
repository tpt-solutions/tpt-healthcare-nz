// Package acc implements ACC (Accident Compensation Corporation) optical/vision
// claim management for NZ optometry and ophthalmology practice. This covers
// ACC-funded eye examinations, spectacles after eye injury, and related claims.
package acc

import (
	"fmt"
	"time"
)

// ClaimType categorises the ACC vision claim.
type ClaimType string

const (
	ClaimEyeExam              ClaimType = "eye_examination"
	ClaimSpectacleAfterInjury ClaimType = "spectacle_after_injury"
	ClaimContactLensInjury    ClaimType = "contact_lens_injury"
	ClaimSurgicalCorrection   ClaimType = "surgical_correction"
	ClaimFollowUp             ClaimType = "follow_up"
)

// ClaimStatus tracks the lifecycle of an ACC vision claim.
type ClaimStatus string

const (
	StatusDraft         ClaimStatus = "draft"
	StatusReadyToSubmit ClaimStatus = "ready_to_submit"
	StatusSubmitted     ClaimStatus = "submitted"
	StatusAccepted      ClaimStatus = "accepted"
	StatusPartiallyPaid ClaimStatus = "partially_paid"
	StatusDeclined      ClaimStatus = "declined"
	StatusRequiresInfo  ClaimStatus = "requires_info"
	StatusAppealed      ClaimStatus = "appealed"
)

// TreatmentProvider identifies the clinician type for the ACC claim.
type TreatmentProvider string

const (
	ProviderOptometrist      TreatmentProvider = "optometrist"
	ProviderOphthalmologist  TreatmentProvider = "ophthalmologist"
	ProviderOpticalDispenser TreatmentProvider = "optical_dispenser"
	ProviderGP               TreatmentProvider = "gp"
)

// InjuryDetails captures the accident/injury information for ACC.
type InjuryDetails struct {
	AccidentDate int64  `json:"accidentDate"`
	InjuryType   string `json:"injuryType"`          // e.g. "corneal abrasion", "foreign body", "blunt trauma"
	InjuryCause  string `json:"injuryCause"`         // brief description
	AccNumber    string `json:"accNumber,omitempty"` // ACC claim number if pre-existing
	LodgedBy     string `json:"lodgedBy"`            // usually the GP or optometrist
}

// ClaimItem represents a single line item on the ACC claim (one procedure/visit).
type ClaimItem struct {
	LineNumber    int     `json:"lineNumber"`
	ServiceDate   int64   `json:"serviceDate"`
	ProcedureCode string  `json:"procedureCode"` // NZ ACC schedule code
	Description   string  `json:"description"`
	Amount        float64 `json:"amount"` // GST-exclusive
	GSTAmount     float64 `json:"gstAmount"`
	TotalAmount   float64 `json:"totalAmount"`
}

// Claim is a complete ACC vision claim record.
type Claim struct {
	ID          string            `json:"id"`
	PatientNHI  string            `json:"patientNhi"`
	ClinicianID string            `json:"clinicianId"`
	PracticeID  string            `json:"practiceId"`
	ClaimType   ClaimType         `json:"claimType"`
	Status      ClaimStatus       `json:"status"`
	Provider    TreatmentProvider `json:"provider"`

	// Linked records
	PrescriptionID string `json:"prescriptionId,omitempty"`
	ExamID         string `json:"examId,omitempty"`
	DispensingID   string `json:"dispensingId,omitempty"`

	Injury InjuryDetails `json:"injury"`
	Items  []ClaimItem   `json:"items"`

	// Financials
	TotalClaimed float64 `json:"totalClaimed"`
	GSTTotal     float64 `json:"gstTotal"`
	TotalIncGST  float64 `json:"totalIncGst"`
	AmountPaid   float64 `json:"amountPaid"`
	Outstanding  float64 `json:"outstanding"`

	// Submission tracking
	SubmittedDate int64  `json:"submittedDate,omitempty"`
	ResponseDate  int64  `json:"responseDate,omitempty"`
	DeclineReason string `json:"declineReason,omitempty"`
	Notes         string `json:"notes,omitempty"`

	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

// NewClaim creates a new ACC vision claim with defaults.
func NewClaim() *Claim {
	now := time.Now().UnixMilli()
	return &Claim{
		Status:    StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks required fields for an ACC claim.
func (c *Claim) Validate() error {
	if c.PatientNHI == "" {
		return fmt.Errorf("acc: patient NHI is required")
	}
	if c.ClinicianID == "" {
		return fmt.Errorf("acc: clinician ID is required")
	}
	if c.Injury.AccidentDate == 0 {
		return fmt.Errorf("acc: accident date is required")
	}
	if c.Injury.InjuryType == "" {
		return fmt.Errorf("acc: injury type is required")
	}
	if len(c.Items) == 0 {
		return fmt.Errorf("acc: at least one claim item is required")
	}
	return nil
}

// AddItem appends a line item to the claim and recalculates totals.
func (c *Claim) AddItem(item ClaimItem) {
	item.LineNumber = len(c.Items) + 1
	c.Items = append(c.Items, item)
	c.recalculate()
}

func (c *Claim) recalculate() {
	var itemsTotal, gstTotal float64
	for _, item := range c.Items {
		itemsTotal += item.Amount
		gstTotal += item.GSTAmount
	}
	c.TotalClaimed = itemsTotal
	c.GSTTotal = gstTotal
	c.TotalIncGST = itemsTotal + gstTotal
	c.Outstanding = c.TotalIncGST - c.AmountPaid
}

// ---------------------------------------------------------------------------
// NZ ACC Vision Schedule — common procedure codes
// ---------------------------------------------------------------------------

// ProcedureCode returns a description for common NZ ACC vision codes.
// These are illustrative; actual codes should use the current ACC Schedule.
type ProcedureCode string

const (
	ProcComprehensiveExam ProcedureCode = "OPT101" // Comprehensive eye examination
	ProcIntermediateExam  ProcedureCode = "OPT102" // Intermediate eye examination
	ProcVisualField       ProcedureCode = "OPT201" // Visual field test
	ProcOCT               ProcedureCode = "OPT301" // OCT scan
	ProcContactLensRemove ProcedureCode = "OPT401" // Remove corneal foreign body
	ProcContactLensPatch  ProcedureCode = "OPT402" // Eye patching after injury
	ProcSpectacleReplace  ProcedureCode = "OPT501" // Spectacle replacement after injury
	ProcCLReplace         ProcedureCode = "OPT502" // Contact lens replacement after injury
)

// ProcedureDescriptions maps codes to descriptions.
var ProcedureDescriptions = map[ProcedureCode]string{
	ProcComprehensiveExam: "Comprehensive eye examination (ACC-funded)",
	ProcIntermediateExam:  "Intermediate eye examination (ACC-funded)",
	ProcVisualField:       "Visual field examination",
	ProcOCT:               "Optical coherence tomography scan",
	ProcContactLensRemove: "Removal of corneal foreign body",
	ProcContactLensPatch:  "Eye patching / padding",
	ProcSpectacleReplace:  "Spectacle replacement due to injury",
	ProcCLReplace:         "Contact lens replacement due to injury",
}

// ---------------------------------------------------------------------------
// FHIR R5 Mapping
// ---------------------------------------------------------------------------

// ToFHIRClaim converts the ACC claim to a FHIR R5 Claim resource.
func (c *Claim) ToFHIRClaim() map[string]any {
	createdTime := time.UnixMilli(c.CreatedAt).Format(time.RFC3339)

	claim := map[string]any{
		"resourceType": "Claim",
		"id":           c.ID,
		"meta": map[string]any{
			"versionId":   fmt.Sprintf("%d", 1),
			"lastUpdated": time.UnixMilli(c.UpdatedAt).Format(time.RFC3339),
			"profile": []string{
				"https://nzfhir.org/StructureDefinition/nz-acc-vision-claim",
			},
		},
		"status": "active",
		"type": map[string]any{
			"coding": []map[string]any{
				{
					"system":  "http://terminology.hl7.org/CodeSystem/claim-type",
					"code":    "professional",
					"display": "Professional",
				},
			},
		},
		"use": "claim",
		"patient": map[string]any{
			"reference": fmt.Sprintf("Patient/%s", c.PatientNHI),
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  c.PatientNHI,
			},
		},
		"created": createdTime,
		"provider": map[string]any{
			"reference": fmt.Sprintf("Practitioner/%s", c.ClinicianID),
		},
		"priority": map[string]any{
			"coding": []map[string]any{
				{
					"system":  "http://terminology.hl7.org/CodeSystem/processpriority",
					"code":    "normal",
					"display": "Normal",
				},
			},
		},
		"fundsReserveRequested": map[string]any{
			"coding": []map[string]any{
				{
					"system":  "http://terminology.hl7.org/CodeSystem/fundsreserve",
					"code":    "patient",
					"display": "Patient",
				},
			},
		},
		"item": []map[string]any{},
		"total": map[string]any{
			"value":    c.TotalIncGST,
			"currency": "NZD",
		},
	}

	// Add claim items
	for _, item := range c.Items {
		claimItem := map[string]any{
			"sequence": item.LineNumber,
			"productOrService": map[string]any{
				"coding": []map[string]any{
					{
						"system":  "https://nzfhir.org/CodeSystem/acc-vision-procedure",
						"code":    item.ProcedureCode,
						"display": item.Description,
					},
				},
			},
			"servicedDate": time.UnixMilli(item.ServiceDate).Format(time.RFC3339),
			"net": map[string]any{
				"value":    item.Amount,
				"currency": "NZD",
			},
			"adjudication": []map[string]any{
				{
					"category": map[string]any{
						"coding": []map[string]any{
							{
								"system":  "http://terminology.hl7.org/CodeSystem/adjudication",
								"code":    "submitted",
								"display": "Submitted Amount",
							},
						},
					},
					"amount": map[string]any{
						"value":    item.TotalAmount,
						"currency": "NZD",
					},
				},
			},
		}
		claim["item"] = append(claim["item"].([]map[string]any), claimItem)
	}

	// Add accident extension
	claim["accident"] = map[string]any{
		"date": time.UnixMilli(c.Injury.AccidentDate).Format(time.RFC3339),
		"type": map[string]any{
			"coding": []map[string]any{
				{
					"system":  "https://nzfhir.org/CodeSystem/acc-injury-type",
					"code":    c.Injury.InjuryType,
					"display": c.Injury.InjuryType,
				},
			},
			"text": c.Injury.InjuryCause,
		},
	}

	// Add NZ-specific extensions
	claim["extension"] = []map[string]any{
		{
			"url":       "https://nzfhir.org/StructureDefinition/nz-acc-claim-type",
			"valueCode": string(c.ClaimType),
		},
		{
			"url":       "https://nzfhir.org/StructureDefinition/nz-acc-claim-status",
			"valueCode": string(c.Status),
		},
		{
			"url":       "https://nzfhir.org/StructureDefinition/nz-acc-provider-type",
			"valueCode": string(c.Provider),
		},
		{
			"url":         "https://nzfhir.org/StructureDefinition/nz-acc-claim-number",
			"valueString": c.Injury.AccNumber,
		},
		{
			"url":         "https://nzfhir.org/StructureDefinition/nz-acc-lodged-by",
			"valueString": c.Injury.LodgedBy,
		},
	}

	if c.SubmittedDate > 0 {
		claim["extension"] = append(claim["extension"].([]map[string]any), map[string]any{
			"url":           "https://nzfhir.org/StructureDefinition/nz-acc-submitted-date",
			"valueDateTime": time.UnixMilli(c.SubmittedDate).Format(time.RFC3339),
		})
	}

	if c.ResponseDate > 0 {
		claim["extension"] = append(claim["extension"].([]map[string]any), map[string]any{
			"url":           "https://nzfhir.org/StructureDefinition/nz-acc-response-date",
			"valueDateTime": time.UnixMilli(c.ResponseDate).Format(time.RFC3339),
		})
	}

	if c.DeclineReason != "" {
		claim["extension"] = append(claim["extension"].([]map[string]any), map[string]any{
			"url":         "https://nzfhir.org/StructureDefinition/nz-acc-decline-reason",
			"valueString": c.DeclineReason,
		})
	}

	return claim
}
