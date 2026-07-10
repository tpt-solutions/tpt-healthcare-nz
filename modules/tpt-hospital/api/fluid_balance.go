package api

import (
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Fluid Balance Charting
// ---------------------------------------------------------------------------

// FluidType categorises fluid input/output.
type FluidType string

const (
	FluidIV          FluidType = "iv"
	FluidOral        FluidType = "oral"
	FluidEnteral     FluidType = "enteral"
	FluidBlood       FluidType = "blood_product"
	FluidColloid     FluidType = "colloid"
	FluidCrystalloid FluidType = "crystalloid"
	FluidTPN         FluidType = "tpn"
	FluidDrain       FluidType = "drain"
	FluidUrine       FluidType = "urine"
	FluidEmesis      FluidType = "emesis"
	FluidStool       FluidType = "stool"
	FluidOutput      FluidType = "other_output"
)

// BalanceEntry records a single fluid input or output event.
type BalanceEntry struct {
	ID            string    `json:"id"`
	AdmissionID   string    `json:"admissionId"`
	PatientNHI    string    `json:"patientNhi"`
	Direction     string    `json:"direction"` // "in" or "out"
	FluidType     FluidType `json:"fluidType"`
	VolumeML      int       `json:"volumeMl"`
	ProductName   string    `json:"productName"` // e.g. "Normal Saline", "Blood O-"
	Concentration string    `json:"concentration,omitempty"`
	RecordedBy    string    `json:"recordedBy"`
	RecordedAt    time.Time `json:"recordedAt"`
	Shift         string    `json:"shift"` // day, evening, night
	Comments      string    `json:"comments,omitempty"`
}

// FluidBalanceSummary aggregates fluid balance over a period.
type FluidBalanceSummary struct {
	AdmissionID   string    `json:"admissionId"`
	PeriodStart   time.Time `json:"periodStart"`
	PeriodEnd     time.Time `json:"periodEnd"`
	TotalInputML  int       `json:"totalInputMl"`
	TotalOutputML int       `json:"totalOutputMl"`
	NetBalanceML  int       `json:"netBalanceMl"` // positive = positive balance
	EntryCount    int       `json:"entryCount"`
}

// NewBalanceEntry creates a new fluid balance entry.
func NewBalanceEntry(admissionID, patientNHI, direction, fluidType string, volumeML int, recordedBy string) *BalanceEntry {
	now := time.Now()
	return &BalanceEntry{
		AdmissionID: admissionID,
		PatientNHI:  patientNHI,
		Direction:   direction,
		FluidType:   FluidType(fluidType),
		VolumeML:    volumeML,
		RecordedBy:  recordedBy,
		RecordedAt:  now,
		Shift:       shiftFromTime(now),
	}
}

// Validate checks required fields for a balance entry.
func (b *BalanceEntry) Validate() error {
	if b.AdmissionID == "" {
		return fmt.Errorf("fluid_balance: admission ID is required")
	}
	if b.PatientNHI == "" {
		return fmt.Errorf("fluid_balance: patient NHI is required")
	}
	if b.Direction != "in" && b.Direction != "out" {
		return fmt.Errorf("fluid_balance: direction must be 'in' or 'out'")
	}
	if b.VolumeML <= 0 {
		return fmt.Errorf("fluid_balance: volume must be positive")
	}
	return nil
}

// SumFluidBalance calculates the net balance from a list of entries.
func SumFluidBalance(entries []BalanceEntry) FluidBalanceSummary {
	summary := FluidBalanceSummary{}
	if len(entries) == 0 {
		return summary
	}
	summary.AdmissionID = entries[0].AdmissionID
	summary.PeriodStart = entries[0].RecordedAt
	summary.PeriodEnd = entries[0].RecordedAt

	for _, e := range entries {
		if e.RecordedAt.Before(summary.PeriodStart) {
			summary.PeriodStart = e.RecordedAt
		}
		if e.RecordedAt.After(summary.PeriodEnd) {
			summary.PeriodEnd = e.RecordedAt
		}
		if e.Direction == "in" {
			summary.TotalInputML += e.VolumeML
		} else {
			summary.TotalOutputML += e.VolumeML
		}
	}
	summary.NetBalanceML = summary.TotalInputML - summary.TotalOutputML
	summary.EntryCount = len(entries)
	return summary
}

func shiftFromTime(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour >= 7 && hour < 15:
		return "day"
	case hour >= 15 && hour < 23:
		return "evening"
	default:
		return "night"
	}
}
