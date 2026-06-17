package api

import "time"

// BloodGroup represents an ABO/RhD blood type.
type BloodGroup string

const (
	BloodGroupAPos  BloodGroup = "A+"
	BloodGroupANeg  BloodGroup = "A-"
	BloodGroupBPos  BloodGroup = "B+"
	BloodGroupBNeg  BloodGroup = "B-"
	BloodGroupABPos BloodGroup = "AB+"
	BloodGroupABNeg BloodGroup = "AB-"
	BloodGroupOPos  BloodGroup = "O+"
	BloodGroupONeg  BloodGroup = "O-"
)

// ValidBloodGroups contains all recognised NZ blood group values.
var ValidBloodGroups = map[BloodGroup]bool{
	BloodGroupAPos:  true,
	BloodGroupANeg:  true,
	BloodGroupBPos:  true,
	BloodGroupBNeg:  true,
	BloodGroupABPos: true,
	BloodGroupABNeg: true,
	BloodGroupOPos:  true,
	BloodGroupONeg:  true,
}

// DeferralReason describes why a donor is temporarily or permanently deferred.
type DeferralReason string

const (
	DeferralLowHaemoglobin   DeferralReason = "low-haemoglobin"
	DeferralRecentTravel     DeferralReason = "recent-travel"
	DeferralMedicalCondition DeferralReason = "medical-condition"
	DeferralMedication       DeferralReason = "medication"
	DeferralTattooPiercing   DeferralReason = "tattoo-piercing"
	DeferralUnderweight      DeferralReason = "underweight"
	DeferralBehaviouralRisk  DeferralReason = "behavioural-risk"
	DeferralPermanent        DeferralReason = "permanent"
)

// DeferralDuration maps reasons to standard deferral periods (in days).
// Zero means permanent deferral.
var DeferralDuration = map[DeferralReason]int{
	DeferralLowHaemoglobin:   180,
	DeferralRecentTravel:     120,
	DeferralMedicalCondition: 0, // assessed case-by-case
	DeferralMedication:       0, // assessed case-by-case
	DeferralTattooPiercing:   120,
	DeferralUnderweight:      0, // permanent until weight gain confirmed
	DeferralBehaviouralRisk:  365,
	DeferralPermanent:        0, // permanent
}

// DonorStatus represents a donor's current eligibility state.
type DonorStatus string

const (
	DonorStatusActive    DonorStatus = "active"
	DonorStatusDeferred  DonorStatus = "deferred"
	DonorStatusPermanent DonorStatus = "permanent"
	DonorStatusInactive  DonorStatus = "inactive"
)

// Donor is the domain model for a blood donor.
type Donor struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenantId"`
	NHI             string     `json:"nhi,omitempty"`
	BloodGroup      BloodGroup `json:"bloodGroup"`
	RhD             string     `json:"rhd"` // "POSITIVE" or "NEGATIVE"
	Status          DonorStatus `json:"status"`
	DeferralReason  *string    `json:"deferralReason,omitempty"`
	DeferralEndDate *time.Time `json:"deferralEndDate,omitempty"`
	TotalDonations  int        `json:"totalDonations"`
	LastDonationAt  *time.Time `json:"lastDonationAt,omitempty"`
	HaemoglobinGDL  *float64   `json:"haemoglobinGdl,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// DonationRecord tracks a single donation event.
type DonationRecord struct {
	ID            string    `json:"id"`
	DonorID       string    `json:"donorId"`
	ProductUnitID string    `json:"productUnitId"`
	VolumeML      int       `json:"volumeMl"`
	DonationType  string    `json:"donationType"` // whole-blood, apheresis-platelets, apheresis-plasma
	CollectedAt   time.Time `json:"collectedAt"`
	CreatedAt     time.Time `json:"createdAt"`
}

// donorCreateRequest is the body for POST /api/v1/donors.
type donorCreateRequest struct {
	NHI        string     `json:"nhi"`
	BloodGroup BloodGroup `json:"bloodGroup"`
}

// donorUpdateRequest is the body for PUT /api/v1/donors/{id}.
type donorUpdateRequest struct {
	BloodGroup     *BloodGroup `json:"bloodGroup,omitempty"`
	HaemoglobinGDL *float64    `json:"haemoglobinGdl,omitempty"`
}

// donorDeferRequest is the body for POST /api/v1/donors/{id}/defer.
type donorDeferRequest struct {
	Reason  DeferralReason `json:"reason"`
	Details string         `json:"details,omitempty"`
}
