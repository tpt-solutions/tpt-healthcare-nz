// Package billing provides shared billing primitives for NZ healthcare,
// covering ACC, PHO, DHB, Veterans Affairs, and private funding types.
package billing

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FundingType identifies the source of funding for a healthcare service.
type FundingType string

const (
	// FundingACC covers services funded by the Accident Compensation Corporation.
	FundingACC FundingType = "ACC"

	// FundingPHO covers services funded under a Primary Health Organisation capitation contract.
	FundingPHO FundingType = "PHO"

	// FundingPrivate covers services paid for directly by the patient or their insurer.
	FundingPrivate FundingType = "PRIVATE"

	// FundingDHB covers services funded by a District Health Board (now Health NZ | Te Whatu Ora).
	FundingDHB FundingType = "DHB"

	// FundingVAC covers services funded under the Veterans' Affairs New Zealand scheme.
	FundingVAC FundingType = "VAC"
)

// ServiceCode describes a billable service item.
type ServiceCode struct {
	// Code is the schedule or tariff code identifying the service (e.g. "GP-CONSULT-STD").
	Code string

	// Description is a human-readable name for the service.
	Description string

	// FundingType indicates how this service is primarily funded.
	FundingType FundingType

	// UnitFee is the standard fee for one unit of this service, in NZD cents.
	UnitFee int64

	// SubsidyAmount is the government or funder subsidy applied per unit, in NZD cents.
	SubsidyAmount int64
}

// BillingLine represents a single line item on an invoice or claim.
type BillingLine struct {
	ServiceCode ServiceCode

	// Quantity is the number of units of the service delivered.
	Quantity int

	// ProviderID is the HPI (Health Practitioner Index) number of the treating provider.
	ProviderID string

	// Date is the date on which the service was delivered.
	Date time.Time

	// DiagnosisCode is the ICD-10-AM code related to this line.
	DiagnosisCode string

	// Notes holds any free-text clinical or billing notes for this line.
	Notes string
}

// Invoice is a billing document issued to a patient or funder.
type Invoice struct {
	// ID is the unique invoice identifier.
	ID uuid.UUID

	// TenantID is the healthcare organisation that issued the invoice.
	TenantID uuid.UUID

	// PatientNHI is the patient's National Health Index number.
	PatientNHI string

	// Lines contains the individual service line items.
	Lines []BillingLine

	// TotalAmount is the gross total of all lines before subsidy, in NZD cents.
	TotalAmount int64

	// SubsidyAmount is the total subsidy to be claimed from funders, in NZD cents.
	SubsidyAmount int64

	// PatientAmount is the amount payable by the patient (TotalAmount - SubsidyAmount), in NZD cents.
	PatientAmount int64

	// Currency is always "NZD".
	Currency string

	// Status reflects the invoice lifecycle (e.g. "draft", "issued", "paid", "cancelled").
	Status string

	// CreatedAt is the time at which the invoice was created.
	CreatedAt time.Time
}

// CalculateTotals computes the gross total, total subsidy, and patient-payable amount
// for a slice of BillingLines. All amounts are in NZD cents.
func CalculateTotals(lines []BillingLine) (total, subsidy, patientAmount int64) {
	for _, l := range lines {
		qty := int64(l.Quantity)
		total += l.ServiceCode.UnitFee * qty
		subsidy += l.ServiceCode.SubsidyAmount * qty
	}
	patientAmount = total - subsidy
	if patientAmount < 0 {
		patientAmount = 0
	}
	return total, subsidy, patientAmount
}

// ACCClaim represents a claim submitted to the Accident Compensation Corporation.
type ACCClaim struct {
	// ClaimNumber is the ACC-assigned claim reference, if known.
	ClaimNumber string

	// PurchaseOrderNumber is the ACC purchase order linked to this claim.
	PurchaseOrderNumber string

	// ProviderID is the HPI number of the treating provider or practice.
	ProviderID string

	// PatientNHI is the patient's National Health Index number.
	PatientNHI string

	// DateOfAccident is the date on which the injury or accident occurred.
	DateOfAccident time.Time

	// InjuryDescription is a plain-language description of the injury.
	InjuryDescription string

	// DiagnosisCodes is a list of ICD-10-AM codes associated with the injury.
	DiagnosisCodes []string

	// TreatmentLines lists the treatment services provided under this claim.
	TreatmentLines []BillingLine

	// Status reflects the claim lifecycle (e.g. "pending", "approved", "declined", "paid").
	Status string
}

// FormatNZD formats an amount in NZD cents as a dollar string (e.g. 1050 → "$10.50").
// Negative amounts are represented with a leading minus (e.g. -50 → "-$0.50").
func FormatNZD(cents int64) string {
	if cents < 0 {
		return fmt.Sprintf("-$%d.%02d", (-cents)/100, (-cents)%100)
	}
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}
