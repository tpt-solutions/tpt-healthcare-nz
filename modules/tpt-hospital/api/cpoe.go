package api

import (
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// CPOE — Computerised Provider Order Entry
// ---------------------------------------------------------------------------

// OrderType categorises clinical orders.
type OrderType string

const (
	OrderTypeLab        OrderType = "laboratory"
	OrderTypeRadiology  OrderType = "radiology"
	OrderTypePathology  OrderType = "pathology"
	OrderTypeMedication OrderType = "medication"
	OrderTypeReferral   OrderType = "referral"
	OrderTypeBloodBank  OrderType = "blood_bank"
	OrderTypeMicro      OrderType = "microbiology"
	OrderTypeDiet       OrderType = "diet"
	OrderTypeActivity   OrderType = "activity"
	OrderTypeConsult    OrderType = "consult"
)

// OrderPriority indicates urgency.
type OrderPriority string

const (
	PriorityRoutine OrderPriority = "routine"
	PriorityStat    OrderPriority = "stat"
	PriorityASAP    OrderPriority = "asap"
	PriorityPreOp   OrderPriority = "pre-op"
)

// OrderStatus tracks order lifecycle.
type OrderStatus string

const (
	OrderPending       OrderStatus = "pending"
	OrderInProgress    OrderStatus = "in_progress"
	OrderCompleted     OrderStatus = "completed"
	OrderCancelled     OrderStatus = "cancelled"
	OrderOnHold        OrderStatus = "on_hold"
	OrderPartialResult OrderStatus = "partial_result"
)

// ClinicalOrder is a CPOE order linked to a hospital admission.
type ClinicalOrder struct {
	ID                 string        `json:"id"`
	AdmissionID        string        `json:"admissionId"`
	TenantID           string        `json:"tenantId"`
	PatientNHI         string        `json:"patientNhi"`
	OrderType          OrderType     `json:"orderType"`
	Priority           OrderPriority `json:"priority"`
	Status             OrderStatus   `json:"status"`
	OrderCode          string        `json:"orderCode"` // LOINC / SNOMED / local code
	OrderText          string        `json:"orderText"`
	ClinicalIndication string        `json:"clinicalIndication"`
	OrderedBy          string        `json:"orderedBy"` // HPI CPN
	OrderedAt          time.Time     `json:"orderedAt"`
	ScheduledFor       *time.Time    `json:"scheduledFor,omitempty"`
	CompletedAt        *time.Time    `json:"completedAt,omitempty"`
	CancelledAt        *time.Time    `json:"cancelledAt,omitempty"`
	CancelReason       string        `json:"cancelReason,omitempty"`
	Comments           string        `json:"comments,omitempty"`
	// lab-specific
	SpecimenType    string `json:"specimenType,omitempty"`
	ContainerType   string `json:"containerType,omitempty"`
	FastingRequired bool   `json:"fastingRequired"`
	VolumeRequired  string `json:"volumeRequired,omitempty"`
	// radiology-specific
	BodySite         string `json:"bodySite,omitempty"`
	Modality         string `json:"modality,omitempty"`
	Contrast         string `json:"contrast,omitempty"`
	PregnancyStatus  string `json:"pregnancyStatus,omitempty"`
	SedationRequired bool   `json:"sedationRequired"`
	TransportMode    string `json:"transportMode,omitempty"`
	// result linkage
	ResultID *string    `json:"resultId,omitempty"`
	ResultAt *time.Time `json:"resultAt,omitempty"`
	// HL7 dispatch tracking
	HL7PlacerOrderID string     `json:"hl7PlacerOrderId,omitempty"`
	HL7FillerOrderID string     `json:"hl7FillerOrderId,omitempty"`
	HL7DispatchedAt  *time.Time `json:"hl7DispatchedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// LabOrder extends ClinicalOrder with lab-specific fields.
type LabOrder struct {
	ClinicalOrder   `json:",inline"`
	SpecimenType    string `json:"specimenType"`  // blood, urine, CSF, tissue, etc.
	ContainerType   string `json:"containerType"` // EDTA, SST, plain, etc.
	FastingRequired bool   `json:"fastingRequired"`
	VolumeRequired  string `json:"volumeRequired"` // e.g. "3mL"
}

// RadiologyOrder extends ClinicalOrder with imaging-specific fields.
type RadiologyOrder struct {
	ClinicalOrder    `json:",inline"`
	BodySite         string `json:"bodySite"`
	Modality         string `json:"modality"`        // X-ray, CT, MRI, US, etc.
	Contrast         string `json:"contrast"`        // none, oral, iv, both
	PregnancyStatus  string `json:"pregnancyStatus"` // not_applicable, negative, positive
	SedationRequired bool   `json:"sedationRequired"`
	TransportMode    string `json:"transportMode"` // walk-in, wheelchair, stretcher
}

// OrderResult captures the result of a completed order.
type OrderResult struct {
	ID             string     `json:"id"`
	OrderID        string     `json:"orderId"`
	ResultCode     string     `json:"resultCode"`
	ResultText     string     `json:"resultText"`
	Value          string     `json:"value"`
	Units          string     `json:"units"`
	ReferenceRange string     `json:"referenceRange"`
	AbnormalFlag   string     `json:"abnormalFlag"` // H, L, HH, LL, N, A
	ResultStatus   string     `json:"resultStatus"` // final, preliminary, corrected
	PerformedBy    string     `json:"performedBy"`
	PerformedAt    time.Time  `json:"performedAt"`
	VerifiedBy     string     `json:"verifiedBy,omitempty"`
	VerifiedAt     *time.Time `json:"verifiedAt,omitempty"`
	Comments       string     `json:"comments,omitempty"`
}

// createOrderRequest is the JSON body for POST /orders.
type createOrderRequest struct {
	OrderType          string     `json:"orderType"`
	Priority           string     `json:"priority"`
	OrderCode          string     `json:"orderCode"`
	OrderText          string     `json:"orderText"`
	ClinicalIndication string     `json:"clinicalIndication"`
	ScheduledFor       *time.Time `json:"scheduledFor,omitempty"`
	Comments           string     `json:"comments,omitempty"`
	// lab-specific
	SpecimenType    string `json:"specimenType,omitempty"`
	ContainerType   string `json:"containerType,omitempty"`
	FastingRequired bool   `json:"fastingRequired"`
	VolumeRequired  string `json:"volumeRequired,omitempty"`
	// radiology-specific
	BodySite         string `json:"bodySite,omitempty"`
	Modality         string `json:"modality,omitempty"`
	Contrast         string `json:"contrast,omitempty"`
	PregnancyStatus  string `json:"pregnancyStatus,omitempty"`
	SedationRequired bool   `json:"sedationRequired"`
	TransportMode    string `json:"transportMode,omitempty"`
}

// updateOrderRequest is the JSON body for PUT /orders/{orderId}.
type updateOrderRequest struct {
	Priority     string     `json:"priority,omitempty"`
	Comments     string     `json:"comments,omitempty"`
	ScheduledFor *time.Time `json:"scheduledFor,omitempty"`
}

// cancelOrderRequest is the JSON body for POST /orders/{orderId}/cancel.
type cancelOrderRequest struct {
	Reason string `json:"reason"`
}

// completeOrderRequest is the JSON body for POST /orders/{orderId}/complete.
type completeOrderRequest struct {
	ResultID string `json:"resultId,omitempty"`
}

// NewClinicalOrder creates a new CPOE order linked to an admission.
func NewClinicalOrder(admissionID, patientNHI, orderType, orderCode, orderText, orderedBy string) *ClinicalOrder {
	now := time.Now()
	return &ClinicalOrder{
		AdmissionID: admissionID,
		PatientNHI:  patientNHI,
		OrderType:   OrderType(orderType),
		Priority:    PriorityRoutine,
		Status:      OrderPending,
		OrderCode:   orderCode,
		OrderText:   orderText,
		OrderedBy:   orderedBy,
		OrderedAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Validate checks required fields for a clinical order.
func (o *ClinicalOrder) Validate() error {
	if o.AdmissionID == "" {
		return fmt.Errorf("cpoe: admission ID is required")
	}
	if o.PatientNHI == "" {
		return fmt.Errorf("cpoe: patient NHI is required")
	}
	if o.OrderType == "" {
		return fmt.Errorf("cpoe: order type is required")
	}
	if o.OrderedAt.IsZero() {
		return fmt.Errorf("cpoe: ordered date is required")
	}
	return nil
}
