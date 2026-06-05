// Package optical implements optical dispensing workflows for spectacle and contact
// lens orders, including frame selection, lens type, measurements, and dispensing
// records for NZ optometry practice.
package optical

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-vision/internal/refraction"
)

// OrderStatus tracks the lifecycle of an optical dispensing order.
type OrderStatus string

const (
	OrderPending    OrderStatus = "pending"
	OrderLabSent    OrderStatus = "lab_sent"
	OrderInLab      OrderStatus = "in_lab"
	OrderReceived   OrderStatus = "received"
	OrderReady      OrderStatus = "ready_for_collection"
	OrderCollected  OrderStatus = "collected"
	OrderCancelled  OrderStatus = "cancelled"
	OrderWarranty   OrderStatus = "warranty_claim"
)

// FrameType describes the type of spectacle frame.
type FrameType string

const (
	FrameFullRim    FrameType = "full_rim"
	FrameSemiRim    FrameType = "semi_rimless"
	FrameRimless    FrameType = "rimless"
	FrameChildrens  FrameType = "childrens"
	FrameSafety     FrameType = "safety"
)

// LensType describes the lens construction.
type LensType string

const (
	LensSingleVision LensType = "single_vision"
	LensBifocal      LensType = "bifocal"
	LensProgressive  LensType = "progressive"
	LensOccupational LensType = "occupational"
	LensPhotochromic LensType = "photochromic"
	LensPolarised    LensType = "polarised"
	LensClipOn       LensType = "clip_on"
)

// ContactLensType describes contact lens modalities.
type ContactLensType string

const (
	CLDailyDisposable  ContactLensType = "daily_disposable"
	CLWeekly           ContactLensType = "weekly"
	CLBiWeekly         ContactLensType = "bi_weekly"
	CLMonthly          ContactLensType = "monthly"
	CLQuarterly        ContactLensType = "quarterly"
	CLYearly           ContactLensType = "yearly"
	CLRGP              ContactLensType = "rgp"         // rigid gas permeable
	CLOrthoK           ContactLensType = "ortho_k"     // orthokeratology
	CLScleral          ContactLensType = "scleral"
)

// Measurement holds dispensing measurements for spectacle fitting.
type Measurement struct {
	PD                  float64 `json:"pd"`                  // pupillary distance (mm)
	PDNear              float64 `json:"pdNear,omitempty"`    // near PD (mm)
	SegmentHeight       float64 `json:"segmentHeight,omitempty"` // mm
	BackVertexDistance  float64 `json:"backVertexDistance,omitempty"` // BVD mm
	PantoscopicTilt     float64 `json:"pantoscopicTilt,omitempty"` // degrees
	FaceFormAngle       float64 `json:"faceFormAngle,omitempty"`    // degrees
}

// FrameDetails captures the selected frame information.
type FrameDetails struct {
	FrameType   FrameType `json:"frameType"`
	Brand       string    `json:"brand"`
	Model       string    `json:"model"`
	Colour      string    `json:"colour"`
	Size        string    `json:"size,omitempty"` // e.g. "54-16-140"
	FramePrice  float64   `json:"framePrice"`
}

// LensOrder captures the lens specification for a dispensing order.
type LensOrder struct {
	LensType       LensType               `json:"lensType"`
	Index          refraction.LensIndex   `json:"index"`
	Coatings       []refraction.LensCoating `json:"coatings,omitempty"`
	Tint           string                 `json:"tint,omitempty"` // e.g. "brown", "grey"
	LensPrice      float64                `json:"lensPrice"`
}

// ContactLensOrder captures a contact lens order.
type ContactLensOrder struct {
	Type         ContactLensType `json:"type"`
	Brand        string          `json:"brand"`
	BaseCurve    float64         `json:"baseCurve"`    // mm
	Diameter     float64         `json:"diameter"`     // mm
	PowerRight   float64         `json:"powerRight"`   // dioptres
	PowerLeft    float64         `json:"powerLeft"`    // dioptres
	CylRight     float64         `json:"cylRight,omitempty"`
	CylLeft      float64         `json:"cylLeft,omitempty"`
	AxisRight    int             `json:"axisRight,omitempty"`
	AxisLeft     int             `json:"axisLeft,omitempty"`
	Qty          int             `json:"qty"`
	PricePerBox  float64         `json:"pricePerBox"`
}

// DispensingOrder is a complete optical dispensing record.
type DispensingOrder struct {
	ID              string            `json:"id"`
	TenantID        string            `json:"tenantId"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	DispenserID     string            `json:"dispenserId"`
	PracticeID      string            `json:"practiceId"`
	PrescriptionID  string            `json:"prescriptionId"` // links to refraction.Prescription
	Status          OrderStatus       `json:"status"`
	OrderDate       int64             `json:"orderDate"`
	DueDate         int64             `json:"dueDate"`
	CollectedDate   int64             `json:"collectedDate,omitempty"`
	FHIRResource    json.RawMessage   `json:"fhirResource,omitempty"`
	FHIRVersion     int               `json:"fhirVersion,omitempty"`

	// Spectacle details (mutually exclusive with contact lens)
	Frame           *FrameDetails      `json:"frame,omitempty"`
	Lens            *LensOrder         `json:"lens,omitempty"`
	Measurements    *Measurement       `json:"measurements,omitempty"`

	// Contact lens details
	ContactLens     *ContactLensOrder  `json:"contactLens,omitempty"`

	// Pricing
	TotalPrice      float64            `json:"totalPrice"`
	DepositPaid     float64            `json:"depositPaid"`
	BalanceDue      float64            `json:"balanceDue"`
	FundedByACC     bool               `json:"fundedByAcc"`
	FundedByDHB     bool               `json:"fundedByDhb"`
	AccClaimID      string             `json:"accClaimId,omitempty"`

	// Warranty
	WarrantyMonths  int                `json:"warrantyMonths,omitempty"`

	Notes           string             `json:"notes,omitempty"`
	CreatedAt       int64              `json:"createdAt"`
	UpdatedAt       int64              `json:"updatedAt"`
}

// NewDispensingOrder creates a new DispensingOrder with defaults.
func NewDispensingOrder() *DispensingOrder {
	now := time.Now().UnixMilli()
	return &DispensingOrder{
		Status:     OrderPending,
		OrderDate:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// Validate checks required fields for a dispensing order.
func (o *DispensingOrder) Validate() error {
	if o.PatientNHI == "" {
		return fmt.Errorf("optical: patient NHI is required")
	}
	if o.ClinicianID == "" {
		return fmt.Errorf("optical: clinician ID is required")
	}
	if o.PrescriptionID == "" {
		return fmt.Errorf("optical: prescription ID is required")
	}
	if o.Frame == nil && o.ContactLens == nil {
		return fmt.Errorf("optical: either frame/lens or contact lens details are required")
	}
	return nil
}

// ---------------------------------------------------------------------------
// NZ Ministry of Health spectacle subsidy check helpers
// ---------------------------------------------------------------------------

// SubsidyEligible checks if a patient likely qualifies for the NZ MoH
// spectacle subsidy (children under 16, or community services card holders).
// This is a placeholder — real implementation would query patient demographics.
func SubsidyEligible(age int, hasCommunityCard bool) bool {
	return age < 16 || hasCommunityCard
}

// ---------------------------------------------------------------------------
// FHIR R5 Mapping
// ---------------------------------------------------------------------------

// ToFHIRMedicationDispense converts the dispensing order to a FHIR R5 MedicationDispense
// resource for contact lens orders, or Device resource for spectacle orders.
func (o *DispensingOrder) ToFHIRMedicationDispense() map[string]any {
	orderTime := time.UnixMilli(o.OrderDate).Format(time.RFC3339)
	
	dispense := map[string]any{
		"resourceType": "MedicationDispense",
		"id":           o.ID,
		"meta": map[string]any{
			"versionId": fmt.Sprintf("%d", o.FHIRVersion),
			"lastUpdated": time.UnixMilli(o.UpdatedAt).Format(time.RFC3339),
			"profile": []string{
				"https://nzfhir.org/StructureDefinition/nz-optical-dispense",
			},
		},
		"status": o.statusToFHIR(),
		"subject": map[string]any{
			"reference": fmt.Sprintf("Patient/%s", o.PatientNHI),
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  o.PatientNHI,
			},
		},
		"performer": []map[string]any{
			{
				"actor": map[string]any{
					"reference": fmt.Sprintf("Practitioner/%s", o.DispenserID),
				},
			},
		},
		"authorizingPrescription": []map[string]any{
			{
				"reference": fmt.Sprintf("MedicationRequest/%s", o.PrescriptionID),
			},
		},
		"type": map[string]any{
			"coding": []map[string]any{
				{
					"system":  "http://terminology.hl7.org/CodeSystem/medicationdispense-category",
					"code":    "community",
					"display": "Community Dispense",
				},
			},
		},
		"quantity": map[string]any{
			"value":  1,
			"unit":   "unit",
			"system": "http://unitsofmeasure.org",
			"code":   "1",
		},
		"daysSupply": map[string]any{
			"value":  365,
			"unit":   "day",
			"system": "http://unitsofmeasure.org",
			"code":   "d",
		},
		"whenPrepared": orderTime,
		"whenHandedOver": orderTime,
		"note": []map[string]any{},
	}

	if o.Notes != "" {
		dispense["note"] = append(dispense["note"].([]map[string]any), map[string]any{
			"text": o.Notes,
		})
	}

	// Add extensions for NZ-specific data
	dispense["extension"] = []map[string]any{
		{
			"url": "https://nzfhir.org/StructureDefinition/nz-optical-dispense-type",
			"valueCode": o.dispenseTypeCode(),
		},
		{
			"url": "https://nzfhir.org/StructureDefinition/nz-optical-dispense-funded-acc",
			"valueBoolean": o.FundedByACC,
		},
		{
			"url": "https://nzfhir.org/StructureDefinition/nz-optical-dispense-funded-dhb",
			"valueBoolean": o.FundedByDHB,
		},
		{
			"url": "https://nzfhir.org/StructureDefinition/nz-optical-dispense-total-price",
			"valueMoney": map[string]any{
				"value":  o.TotalPrice,
				"currency": "NZD",
			},
		},
	}

	return dispense
}

func (o *DispensingOrder) statusToFHIR() string {
	switch o.Status {
	case OrderPending:
		return "preparation"
	case OrderLabSent, OrderInLab:
		return "in-progress"
	case OrderReceived, OrderReady:
		return "completed"
	case OrderCollected:
		return "completed"
	case OrderCancelled:
		return "cancelled"
	case OrderWarranty:
		return "completed"
	default:
		return "preparation"
	}
}

func (o *DispensingOrder) dispenseTypeCode() string {
	if o.ContactLens != nil {
		return "contact_lens"
	}
	return "spectacle"
}
