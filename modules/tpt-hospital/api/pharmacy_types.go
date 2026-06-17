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
	ID              string                `json:"id"`
	AdmissionID     string                `json:"admissionId"`
	PatientID       string                `json:"patientId"`
	PrescriberHPI   string                `json:"prescriberHpi"`
	GenericName     string                `json:"genericName"`
	BrandName       string                `json:"brandName,omitempty"`
	NZMTCode        string                `json:"nzmtCode,omitempty"` // NZMT identifier
	Dose            string                `json:"dose"`               // e.g. "500 mg"
	Route           RouteOfAdministration `json:"route"`
	Frequency       string                `json:"frequency"`           // e.g. "BD", "8-hourly", "PRN"
	MaxDailyDose    string                `json:"maxDailyDose,omitempty"`
	Indication      string                `json:"indication,omitempty"`
	StartDate       time.Time             `json:"startDate"`
	EndDate         *time.Time            `json:"endDate,omitempty"`
	Status          InpatientMedStatus    `json:"status"`
	IsIV            bool                  `json:"isIv"`
	IVRate          string                `json:"ivRate,omitempty"` // e.g. "100 mL/hr"
	AllergiesChecked bool                 `json:"allergiesChecked"`
	TenantID        string                `json:"tenantId"`
	CeasedAt        *time.Time            `json:"ceasedAt,omitempty"`
	CeasedReason    string                `json:"ceasedReason,omitempty"`
	CreatedAt       time.Time             `json:"createdAt"`
	UpdatedAt       time.Time             `json:"updatedAt"`
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
	TenantID       string                `json:"tenantId"`
	AdministeredAt time.Time             `json:"administeredAt"`
}

// MedReconciliation is the structured comparison of home medications
// against the inpatient chart, performed on admission and discharge.
type MedReconciliation struct {
	ID                 string    `json:"id"`
	AdmissionID        string    `json:"admissionId"`
	ClinicianHPI       string    `json:"clinicianHpi"`
	ReconciliationType string    `json:"type"` // "admission" or "discharge"
	HomeMedications    []string  `json:"homeMedications"`  // from community / NZMT
	ChartMedications   []string  `json:"chartMedications"` // inpatient chart
	Discrepancies      []string  `json:"discrepancies,omitempty"`
	ActionsTaken       []string  `json:"actionsTaken,omitempty"`
	ClinicalNotes      string    `json:"clinicalNotes,omitempty"`
	TenantID           string    `json:"tenantId"`
	CompletedAt        time.Time `json:"completedAt"`
}

type medPrescribeRequest struct {
	PrescriberHPI string                `json:"prescriberHpi"`
	GenericName   string                `json:"genericName"`
	BrandName     string                `json:"brandName,omitempty"`
	NZMTCode      string                `json:"nzmtCode,omitempty"`
	Dose          string                `json:"dose"`
	Route         RouteOfAdministration `json:"route"`
	Frequency     string                `json:"frequency"`
	MaxDailyDose  string                `json:"maxDailyDose,omitempty"`
	Indication    string                `json:"indication,omitempty"`
	StartDate     time.Time             `json:"startDate"`
	EndDate       *time.Time            `json:"endDate,omitempty"`
	IsIV          bool                  `json:"isIv,omitempty"`
	IVRate        string                `json:"ivRate,omitempty"`
}

type medAdminRequest struct {
	AdministeredBy string                `json:"administeredBy"`
	ActualDose     string                `json:"actualDose"`
	Route          RouteOfAdministration `json:"route,omitempty"`
	Notes          string                `json:"notes,omitempty"`
	Withheld       bool                  `json:"withheld,omitempty"`
	WithheldReason string                `json:"withheldReason,omitempty"`
}

type medCeaseRequest struct {
	Reason string `json:"reason"`
}

type medReconcileRequest struct {
	ClinicianHPI       string   `json:"clinicianHpi"`
	ReconciliationType string   `json:"type"`
	HomeMedications    []string `json:"homeMedications"`
	Discrepancies      []string `json:"discrepancies,omitempty"`
	ActionsTaken       []string `json:"actionsTaken,omitempty"`
	ClinicalNotes      string   `json:"clinicalNotes,omitempty"`
}
