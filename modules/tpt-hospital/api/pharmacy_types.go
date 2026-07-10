package api

import "time"

// InpatientMedStatus tracks the state of a medication on an inpatient chart.
type InpatientMedStatus string

const (
	InpatientMedStatusActive    InpatientMedStatus = "active"
	InpatientMedStatusOnHold    InpatientMedStatus = "on-hold"
	InpatientMedStatusCeased    InpatientMedStatus = "ceased"
	InpatientMedStatusCompleted InpatientMedStatus = "completed"
)

// RouteOfAdministration enumerates common drug delivery routes.
type RouteOfAdministration string

const (
	RouteOral    RouteOfAdministration = "oral"
	RouteIV      RouteOfAdministration = "intravenous"
	RouteIM      RouteOfAdministration = "intramuscular"
	RouteSC      RouteOfAdministration = "subcutaneous"
	RouteTopical RouteOfAdministration = "topical"
	RouteINH     RouteOfAdministration = "inhaled"
	RouteNasal   RouteOfAdministration = "intranasal"
	RouteRectal  RouteOfAdministration = "rectal"
	RouteSL      RouteOfAdministration = "sublingual"
)

// InpatientMedication is a single medication on an inpatient medication chart.
type InpatientMedication struct {
	ID               string                `json:"id"`
	AdmissionID      string                `json:"admissionId"`
	PatientID        string                `json:"patientId"`
	PrescriberHPI    string                `json:"prescriberHpi"`
	GenericName      string                `json:"genericName"`
	BrandName        string                `json:"brandName,omitempty"`
	NZMTCode         string                `json:"nzmtCode,omitempty"` // NZMT identifier
	Dose             string                `json:"dose"`               // e.g. "500 mg"
	Route            RouteOfAdministration `json:"route"`
	Frequency        string                `json:"frequency"` // e.g. "BD", "8-hourly", "PRN"
	MaxDailyDose     string                `json:"maxDailyDose,omitempty"`
	Indication       string                `json:"indication,omitempty"`
	StartDate        time.Time             `json:"startDate"`
	EndDate          *time.Time            `json:"endDate,omitempty"`
	Status           InpatientMedStatus    `json:"status"`
	IsIV             bool                  `json:"isIv"`
	IVRate           string                `json:"ivRate,omitempty"` // e.g. "100 mL/hr"
	AllergiesChecked bool                  `json:"allergiesChecked"`
	// Barcode is the GTIN/NZULM barcode payload printed on the dispensed pack,
	// used for bedside eMAR scan verification against the chart.
	Barcode                string     `json:"barcode,omitempty"`
	IsControlledDrug       bool       `json:"isControlledDrug"`
	ControlledDrugSchedule string     `json:"controlledDrugSchedule,omitempty"` // e.g. "CD-A", "CD-B", "CD-C" (Misuse of Drugs Regulations 1977)
	TenantID               string     `json:"tenantId"`
	CeasedAt               *time.Time `json:"ceasedAt,omitempty"`
	CeasedReason           string     `json:"ceasedReason,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

// MedAdministrationRecord documents a single administration of a medication.
type MedAdministrationRecord struct {
	ID             string                `json:"id"`
	MedicationID   string                `json:"medicationId"`
	AdmissionID    string                `json:"admissionId"`
	AdministeredBy string                `json:"administeredBy"` // nurse HPI
	ActualDose     string                `json:"actualDose"`
	Route          RouteOfAdministration `json:"route"`
	Notes          string                `json:"notes,omitempty"`
	Withheld       bool                  `json:"withheld"`
	WithheldReason string                `json:"withheldReason,omitempty"`
	// Five-rights / eMAR barcode verification fields.
	PatientBarcodeScanned string `json:"patientBarcodeScanned,omitempty"`
	MedBarcodeScanned     string `json:"medBarcodeScanned,omitempty"`
	VerificationMethod    string `json:"verificationMethod"` // "barcode" or "manual"
	FiveRightsConfirmed   bool   `json:"fiveRightsConfirmed"`
	// WitnessHPI is the second clinician who co-signs administration of a
	// controlled drug (required when the medication is a controlled drug).
	WitnessHPI     string    `json:"witnessHpi,omitempty"`
	TenantID       string    `json:"tenantId"`
	AdministeredAt time.Time `json:"administeredAt"`
}

// ControlledDrugRegisterEntry is a single append-only line in the ward
// controlled-drug (Misuse of Drugs Regulations schedule) register.
type ControlledDrugRegisterEntry struct {
	ID             string    `json:"id"`
	AdmissionID    string    `json:"admissionId"`
	MedicationID   string    `json:"medicationId,omitempty"`
	DrugName       string    `json:"drugName"`
	Schedule       string    `json:"schedule"`
	Action         string    `json:"action"` // administered, wasted, returned, stock-received, stock-count
	Quantity       float64   `json:"quantity"`
	BalanceAfter   float64   `json:"balanceAfter"`
	AdministeredBy string    `json:"administeredBy"`
	WitnessHPI     string    `json:"witnessHpi"`
	Notes          string    `json:"notes,omitempty"`
	TenantID       string    `json:"tenantId"`
	RecordedAt     time.Time `json:"recordedAt"`
}

type controlledDrugEntryRequest struct {
	DrugName       string  `json:"drugName"`
	Schedule       string  `json:"schedule"`
	Action         string  `json:"action"`
	Quantity       float64 `json:"quantity"`
	AdministeredBy string  `json:"administeredBy"`
	WitnessHPI     string  `json:"witnessHpi"`
	Notes          string  `json:"notes,omitempty"`
}

// MedReconciliation is the structured comparison of home medications
// against the inpatient chart, performed on admission and discharge.
type MedReconciliation struct {
	ID                 string    `json:"id"`
	AdmissionID        string    `json:"admissionId"`
	ClinicianHPI       string    `json:"clinicianHpi"`
	ReconciliationType string    `json:"type"`             // "admission" or "discharge"
	HomeMedications    []string  `json:"homeMedications"`  // from community / NZMT
	ChartMedications   []string  `json:"chartMedications"` // inpatient chart
	Discrepancies      []string  `json:"discrepancies,omitempty"`
	ActionsTaken       []string  `json:"actionsTaken,omitempty"`
	ClinicalNotes      string    `json:"clinicalNotes,omitempty"`
	TenantID           string    `json:"tenantId"`
	CompletedAt        time.Time `json:"completedAt"`
}

type medPrescribeRequest struct {
	PrescriberHPI          string                `json:"prescriberHpi"`
	GenericName            string                `json:"genericName"`
	BrandName              string                `json:"brandName,omitempty"`
	NZMTCode               string                `json:"nzmtCode,omitempty"`
	Barcode                string                `json:"barcode,omitempty"`
	Dose                   string                `json:"dose"`
	Route                  RouteOfAdministration `json:"route"`
	Frequency              string                `json:"frequency"`
	MaxDailyDose           string                `json:"maxDailyDose,omitempty"`
	Indication             string                `json:"indication,omitempty"`
	StartDate              time.Time             `json:"startDate"`
	EndDate                *time.Time            `json:"endDate,omitempty"`
	IsIV                   bool                  `json:"isIv,omitempty"`
	IVRate                 string                `json:"ivRate,omitempty"`
	IsControlledDrug       bool                  `json:"isControlledDrug,omitempty"`
	ControlledDrugSchedule string                `json:"controlledDrugSchedule,omitempty"`
}

type medAdminRequest struct {
	AdministeredBy string                `json:"administeredBy"`
	ActualDose     string                `json:"actualDose"`
	Route          RouteOfAdministration `json:"route,omitempty"`
	Notes          string                `json:"notes,omitempty"`
	Withheld       bool                  `json:"withheld,omitempty"`
	WithheldReason string                `json:"withheldReason,omitempty"`
	// PatientBarcode/MedBarcode are the raw scans from a bedside barcode
	// scanner (patient wristband NHI barcode, and dispensed-pack barcode).
	// When both are present, the server performs five-rights verification
	// before recording the administration.
	PatientBarcode string `json:"patientBarcode,omitempty"`
	MedBarcode     string `json:"medBarcode,omitempty"`
	// WitnessHPI is required when administering a controlled drug.
	WitnessHPI string `json:"witnessHpi,omitempty"`
}

type medCeaseRequest struct {
	Reason string `json:"reason"`
}

// bedsideVerifyRequest is the bedside barcode verification request.
type bedsideVerifyRequest struct {
	PatientNHI     string `json:"patientNhi"`
	PatientBarcode string `json:"patientBarcode,omitempty"`
	MedBarcode     string `json:"medBarcode,omitempty"`
}

type medReconcileRequest struct {
	ClinicianHPI       string   `json:"clinicianHpi"`
	ReconciliationType string   `json:"type"`
	HomeMedications    []string `json:"homeMedications"`
	Discrepancies      []string `json:"discrepancies,omitempty"`
	ActionsTaken       []string `json:"actionsTaken,omitempty"`
	ClinicalNotes      string   `json:"clinicalNotes,omitempty"`
}

// ── IV Pump / Smart Infusion Integration Types ───────────────────────────────

// IVPumpType classifies smart infusion pump types for integration.
type IVPumpType string

const (
	IVPumpTypeStandard IVPumpType = "standard"
	IVPumpTypeSmart    IVPumpType = "smart"
	IVPumpTypeSyringe  IVPumpType = "syringe"
	IVPumpTypePCA      IVPumpType = "pca"
	IVPumpTypeECMO     IVPumpType = "ecmo"
)

// IVPumpStatus tracks the current status of an IV pump.
type IVPumpStatus string

const (
	IVPumpStatusRunning   IVPumpStatus = "running"
	IVPumpStatusPaused    IVPumpStatus = "paused"
	IVPumpStatusStopped   IVPumpStatus = "stopped"
	IVPumpStatusAlarm     IVPumpStatus = "alarm"
	IVPumpStatusCompleted IVPumpStatus = "completed"
	IVPumpStatusError     IVPumpStatus = "error"
)

// IVInfusionRecord documents a smart pump infusion session linked to an inpatient medication.
type IVInfusionRecord struct {
	ID              string       `json:"id"`
	MedicationID    string       `json:"medicationId"`
	AdmissionID     string       `json:"admissionId"`
	PumpIdentifier  string       `json:"pumpIdentifier"`
	PumpType        IVPumpType   `json:"pumpType"`
	Rate            string       `json:"rate"`
	Concentration   string       `json:"concentration,omitempty"`
	VTBI            *float64     `json:"vtbi,omitempty"`
	VolumeInfused   *float64     `json:"volumeInfused,omitempty"`
	DoseInfused     *float64     `json:"doseInfused,omitempty"`
	StartedAt       time.Time    `json:"startedAt"`
	StoppedAt       *time.Time   `json:"stoppedAt,omitempty"`
	Status          IVPumpStatus `json:"status"`
	PausedAt        *time.Time   `json:"pausedAt,omitempty"`
	LabelText       string       `json:"labelText,omitempty"`
	SafetySoftLimit *float64     `json:"safetySoftLimit,omitempty"`
	SafetyHardLimit *float64     `json:"safetyHardLimit,omitempty"`
	TenantID        string       `json:"tenantId"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

// IVLinkRequest is the request body for linking an IV pump to a medication.
type IVLinkRequest struct {
	PumpIdentifier  string     `json:"pumpIdentifier"`
	PumpType        IVPumpType `json:"pumpType"`
	Rate            string     `json:"rate"`
	Concentration   string     `json:"concentration,omitempty"`
	VTBI            *float64   `json:"vtbi,omitempty"`
	LabelText       string     `json:"labelText,omitempty"`
	SafetySoftLimit *float64   `json:"safetySoftLimit,omitempty"`
	SafetyHardLimit *float64   `json:"safetyHardLimit,omitempty"`
}

// IVStatusUpdate is the request body for updating pump status.
type IVStatusUpdate struct {
	Status        IVPumpStatus `json:"status"`
	VolumeInfused *float64     `json:"volumeInfused,omitempty"`
	DoseInfused   *float64     `json:"doseInfused,omitempty"`
}

// NewIVInfusionRecord creates a new infusion record.
func NewIVInfusionRecord(medicationID, admissionID, pumpIdentifier string, pumpType IVPumpType, rate string) *IVInfusionRecord {
	now := time.Now().UTC()
	return &IVInfusionRecord{
		MedicationID:   medicationID,
		AdmissionID:    admissionID,
		PumpIdentifier: pumpIdentifier,
		PumpType:       pumpType,
		Rate:           rate,
		Status:         IVPumpStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
