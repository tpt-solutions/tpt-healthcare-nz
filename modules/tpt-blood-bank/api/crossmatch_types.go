package api

import "time"

// CrossmatchStatus represents the current state of a cross-match request.
type CrossmatchStatus string

const (
	CrossmatchStatusPending      CrossmatchStatus = "pending"
	CrossmatchStatusMatched      CrossmatchStatus = "matched"
	CrossmatchStatusIssued       CrossmatchStatus = "issued"
	CrossmatchStatusTransfused   CrossmatchStatus = "transfused"
	CrossmatchStatusCancelled    CrossmatchStatus = "cancelled"
	CrossmatchStatusIncompatible CrossmatchStatus = "incompatible"
)

// Crossmatch represents a cross-match request linking a patient to blood products.
type Crossmatch struct {
	ID              string           `json:"id"`
	TenantID        string           `json:"tenantId"`
	PatientID       string           `json:"patientId"`
	PatientNHI      string           `json:"patientNhi"`
	PatientABO      string           `json:"patientAbo"`
	PatientRhD      string           `json:"patientRhd"`
	AntibodyScreen  string           `json:"antibodyScreen"`  // negative, positive
	ProductUnitIDs  []string         `json:"productUnitIds"`  // IDs of matched product units
	Status          CrossmatchStatus `json:"status"`
	Compatibility   string           `json:"compatibility"`   // compatible, incompatible, emergency-release
	RequestedBy     string           `json:"requestedBy"`
	IssuedBy        *string          `json:"issuedBy,omitempty"`
	TransfusedBy    *string          `json:"transfusedBy,omitempty"`
	EmergencyReason *string          `json:"emergencyReason,omitempty"`
	Notes           string           `json:"notes,omitempty"`
	RequestedAt     time.Time        `json:"requestedAt"`
	IssuedAt        *time.Time       `json:"issuedAt,omitempty"`
	TransfusedAt    *time.Time       `json:"transfusedAt,omitempty"`
	CancelledAt     *time.Time       `json:"cancelledAt,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

// crossmatchCreateRequest is the body for POST /api/v1/crossmatches.
type crossmatchCreateRequest struct {
	PatientID      string   `json:"patientId"`
	PatientNHI     string   `json:"patientNhi"`
	PatientABO     string   `json:"patientAbo"`
	PatientRhD     string   `json:"patientRhd"`
	AntibodyScreen string   `json:"antibodyScreen"`
	ProductUnitIDs []string `json:"productUnitIds"`
	RequestedBy    string   `json:"requestedBy"`
	Notes          string   `json:"notes,omitempty"`
}

// crossmatchIssueRequest is the body for POST /api/v1/crossmatches/{id}/issue.
type crossmatchIssueRequest struct {
	IssuedBy string `json:"issuedBy"`
}

// crossmatchTransfuseRequest is the body for POST /api/v1/crossmatches/{id}/transfuse.
type crossmatchTransfuseRequest struct {
	TransfusedBy string `json:"transfusedBy"`
	Notes        string `json:"notes,omitempty"`
}

// crossmatchCancelRequest is the body for POST /api/v1/crossmatches/{id}/cancel.
type crossmatchCancelRequest struct {
	Reason string `json:"reason"`
}

// crossmatchEmergencyRequest is the body for POST /api/v1/crossmatches/{id}/emergency.
type crossmatchEmergencyRequest struct {
	ApprovedBy     string `json:"approvedBy"`
	ClinicalReason string `json:"clinicalReason"`
}

// ABOCompatibilityTable defines which ABO donor types are compatible with each recipient type.
// Key = patient ABO, Value = list of compatible donor ABOs.
var ABOCompatibilityTable = map[string][]string{
	"O":  {"O"},
	"A":  {"A", "O"},
	"B":  {"B", "O"},
	"AB": {"A", "B", "AB", "O"},
}

// RhDCompatible returns true if the donor RhD is compatible with the patient.
// RhD-negative patients must receive RhD-negative blood to avoid alloimmunisation.
// RhD-positive patients may receive either RhD-positive or RhD-negative blood.
func RhDCompatible(patientRhD, donorRhD string) bool {
	if patientRhD == "NEGATIVE" && donorRhD == "POSITIVE" {
		return false
	}
	return true
}
