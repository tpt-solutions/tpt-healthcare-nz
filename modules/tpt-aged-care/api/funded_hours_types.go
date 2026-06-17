package api

import "time"

// FundingType identifies the source of the funded support hours.
type FundingType string

const (
	FundingMoHHomeSupport     FundingType = "moh-home-support"    // MoH contracted home support
	FundingNASCAllocated      FundingType = "nasc-allocated"      // NASC-assigned allocation
	FundingResidentialSubsidy FundingType = "residential-subsidy" // Aged residential care subsidy
	FundingPrivate            FundingType = "private"             // Private pay
)

// AllocationStatus tracks the lifecycle of a funded hours allocation.
type AllocationStatus string

const (
	AllocationActive    AllocationStatus = "active"
	AllocationSuspended AllocationStatus = "suspended"
	AllocationExpired   AllocationStatus = "expired"
	AllocationClosed    AllocationStatus = "closed"
)

// TimesheetStatus tracks the approval state of a service delivery timesheet.
type TimesheetStatus string

const (
	TimesheetPending  TimesheetStatus = "pending"
	TimesheetApproved TimesheetStatus = "approved"
	TimesheetDisputed TimesheetStatus = "disputed"
	TimesheetVoided   TimesheetStatus = "voided"
)

// FundedHoursAllocation is a MoH / NASC allocation of funded support hours for a patient.
type FundedHoursAllocation struct {
	ID            string           `json:"id"`
	PatientID     string           `json:"patientId"`
	PatientNHI    string           `json:"patientNhi"`
	TenantID      string           `json:"tenantId"`
	ServicePlanID string           `json:"servicePlanId,omitempty"`
	FundingType   FundingType      `json:"fundingType"`
	Status        AllocationStatus `json:"status"`
	HoursPerWeek  float64          `json:"hoursPerWeek"`
	ServiceType   string           `json:"serviceType"` // e.g. "personal-care", "domestic"
	ProviderID    string           `json:"providerId,omitempty"`
	ProviderName  string           `json:"providerName,omitempty"`
	StartDate     string           `json:"startDate"` // YYYY-MM-DD
	EndDate       string           `json:"endDate,omitempty"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
}

// TimesheetEntry records individual service delivery within a timesheet.
type TimesheetEntry struct {
	Date        string  `json:"date"`        // YYYY-MM-DD
	StartTime   string  `json:"startTime"`   // HH:MM
	EndTime     string  `json:"endTime"`     // HH:MM
	HoursWorked float64 `json:"hoursWorked"`
	ServiceType string  `json:"serviceType"`
	WorkerName  string  `json:"workerName,omitempty"`
	Notes       string  `json:"notes,omitempty"`
}

// FundedHoursTimesheet records service delivery against an allocation for a pay period.
type FundedHoursTimesheet struct {
	ID            string           `json:"id"`
	AllocationID  string           `json:"allocationId"`
	PatientID     string           `json:"patientId"`
	PatientNHI    string           `json:"patientNhi"`
	TenantID      string           `json:"tenantId"`
	Status        TimesheetStatus  `json:"status"`
	PeriodStart   string           `json:"periodStart"` // YYYY-MM-DD
	PeriodEnd     string           `json:"periodEnd"`   // YYYY-MM-DD
	Entries       []TimesheetEntry `json:"entries"`
	TotalHours    float64          `json:"totalHours"`
	ApprovedByHPI string           `json:"approvedByHpi,omitempty"`
	ApprovedAt    *time.Time       `json:"approvedAt,omitempty"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
}

// FundedHoursSummary aggregates allocation and delivery stats for a patient.
type FundedHoursSummary struct {
	PatientID          string  `json:"patientId"`
	PatientNHI         string  `json:"patientNhi"`
	AllocatedPerWeek   float64 `json:"allocatedPerWeek"`
	DeliveredThisWeek  float64 `json:"deliveredThisWeek"`
	DeliveredThisMonth float64 `json:"deliveredThisMonth"`
	UnusedThisWeek     float64 `json:"unusedThisWeek"`
	ActiveAllocations  int     `json:"activeAllocations"`
}

// Internal DB records.
type allocationRecord struct {
	ID            string
	PatientID     string
	PatientNHI    string
	TenantID      string
	ServicePlanID string
	FundingType   FundingType
	Status        AllocationStatus
	HoursPerWeek  float64
	ServiceType   string
	ProviderID    string
	ProviderName  string
	StartDate     string
	EndDate       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type timesheetRecord struct {
	ID            string
	AllocationID  string
	PatientID     string
	PatientNHI    string
	TenantID      string
	Status        TimesheetStatus
	PeriodStart   string
	PeriodEnd     string
	Entries       []TimesheetEntry
	TotalHours    float64
	ApprovedByHPI string
	ApprovedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
