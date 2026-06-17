package api

import "time"

// AdmissionStatus enumerates FHIR R5 Encounter status values for inpatient admissions.
type AdmissionStatus string

const (
	AdmissionStatusAdmitted    AdmissionStatus = "admitted"
	AdmissionStatusInHospital  AdmissionStatus = "in-hospital"
	AdmissionStatusTransferred AdmissionStatus = "transferred"
	AdmissionStatusDischarged  AdmissionStatus = "discharged"
	AdmissionStatusCancelled   AdmissionStatus = "cancelled"
)

// AdmissionType distinguishes the clinical pathway for the admission.
type AdmissionType string

const (
	AdmissionTypeElective  AdmissionType = "elective"
	AdmissionTypeEmergency AdmissionType = "emergency"
	AdmissionTypeMaternity AdmissionType = "maternity"
	AdmissionTypeDayStay   AdmissionType = "day-stay"
	AdmissionTypeRehab     AdmissionType = "rehabilitation"
	AdmissionTypeTransfer  AdmissionType = "transfer-in"
)

// DischargeDestination records where the patient went after leaving hospital.
type DischargeDestination string

const (
	DischargeDestinationHome          DischargeDestination = "home"
	DischargeDestinationAgedCare      DischargeDestination = "aged-care"
	DischargeDestinationRehab         DischargeDestination = "rehabilitation"
	DischargeDestinationOtherHospital DischargeDestination = "other-hospital"
	DischargeDestinationDeceased      DischargeDestination = "deceased"
)

// Admission represents an inpatient hospital stay, aligned to FHIR R5 Encounter.
type Admission struct {
	ID                      string               `json:"id"`
	PatientID               string               `json:"patientId"`
	PatientNHI              string               `json:"patientNhi"`
	AdmittingClinicianHPI   string               `json:"admittingClinicianHpi"`
	ResponsibleClinicianHPI string               `json:"responsibleClinicianHpi,omitempty"`
	AdmissionType           AdmissionType        `json:"admissionType"`
	Status                  AdmissionStatus      `json:"status"`
	WardID                  string               `json:"wardId,omitempty"`
	BedID                   string               `json:"bedId,omitempty"`
	AdmissionReason         string               `json:"admissionReason"`
	PrimaryDiagnosis        string               `json:"primaryDiagnosis,omitempty"` // ICD-10-AM
	ACCClaimNumber          string               `json:"accClaimNumber,omitempty"`
	ReferringFacilityHPI    string               `json:"referringFacilityHpi,omitempty"`
	DischargeDestination    DischargeDestination `json:"dischargeDestination,omitempty"`
	DischargeNotes          string               `json:"dischargeNotes,omitempty"`
	TenantID                string               `json:"tenantId"`
	AdmittedAt              time.Time            `json:"admittedAt"`
	DischargedAt            *time.Time           `json:"dischargedAt,omitempty"`
	CreatedAt               time.Time            `json:"createdAt"`
	UpdatedAt               time.Time            `json:"updatedAt"`
}

// DischargeSummary is the clinical document produced at patient discharge.
type DischargeSummary struct {
	ID                  string     `json:"id"`
	AdmissionID         string     `json:"admissionId"`
	PatientID           string     `json:"patientId"`
	AuthorHPI           string     `json:"authorHpi"`
	AdmissionDate       time.Time  `json:"admissionDate"`
	DischargeDate       time.Time  `json:"dischargeDate"`
	PrimaryDiagnosis    string     `json:"primaryDiagnosis"`
	SecondaryDiagnoses  []string   `json:"secondaryDiagnoses"`
	ProceduresPerformed []string   `json:"proceduresPerformed"`
	ClinicalSummary     string     `json:"clinicalSummary"`
	DischargeCondition  string     `json:"dischargeCondition"` // good, fair, poor, critical
	FollowUpPlan        string     `json:"followUpPlan"`
	Medications         []string   `json:"medications"` // medication names on discharge
	GPNotified          bool       `json:"gpNotified"`
	GPNotifiedAt        *time.Time `json:"gpNotifiedAt,omitempty"`
	TenantID            string     `json:"tenantId"`
	CreatedAt           time.Time  `json:"createdAt"`
}

type admissionCreateRequest struct {
	PatientID             string        `json:"patientId"`
	PatientNHI            string        `json:"patientNhi"`
	AdmittingClinicianHPI string        `json:"admittingClinicianHpi"`
	AdmissionType         AdmissionType `json:"admissionType"`
	AdmissionReason       string        `json:"admissionReason"`
	WardID                string        `json:"wardId,omitempty"`
	BedID                 string        `json:"bedId,omitempty"`
	ACCClaimNumber        string        `json:"accClaimNumber,omitempty"`
	ReferringFacilityHPI  string        `json:"referringFacilityHpi,omitempty"`
}

type admissionUpdateRequest struct {
	ResponsibleClinicianHPI string `json:"responsibleClinicianHpi,omitempty"`
	WardID                  string `json:"wardId,omitempty"`
	BedID                   string `json:"bedId,omitempty"`
	PrimaryDiagnosis        string `json:"primaryDiagnosis,omitempty"`
	AdmissionReason         string `json:"admissionReason,omitempty"`
}

type dischargeRequest struct {
	Destination    DischargeDestination `json:"destination"`
	DischargeNotes string               `json:"dischargeNotes,omitempty"`
}

type transferRequest struct {
	ToWardID string `json:"toWardId"`
	ToBedID  string `json:"toBedId"`
	Reason   string `json:"reason,omitempty"`
}

type dischargeSummaryCreateRequest struct {
	AuthorHPI           string   `json:"authorHpi"`
	PrimaryDiagnosis    string   `json:"primaryDiagnosis"`
	SecondaryDiagnoses  []string `json:"secondaryDiagnoses,omitempty"`
	ProceduresPerformed []string `json:"proceduresPerformed,omitempty"`
	ClinicalSummary     string   `json:"clinicalSummary"`
	DischargeCondition  string   `json:"dischargeCondition"`
	FollowUpPlan        string   `json:"followUpPlan"`
	Medications         []string `json:"medications,omitempty"`
}
