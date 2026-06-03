package billing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateTotals(t *testing.T) {
	now := time.Now()

	accService := ServiceCode{
		Code:          "ACC-CONSULT-GP",
		Description:   "ACC GP Consultation",
		FundingType:   FundingACC,
		UnitFee:       5000, // $50.00
		SubsidyAmount: 4000, // $40.00 ACC subsidy
	}
	phoService := ServiceCode{
		Code:          "PHO-CONSULT-STD",
		Description:   "PHO Standard Consultation",
		FundingType:   FundingPHO,
		UnitFee:       3500, // $35.00
		SubsidyAmount: 3000, // $30.00 PHO capitation subsidy
	}
	privateService := ServiceCode{
		Code:          "PRIV-CONSULT",
		Description:   "Private Consultation",
		FundingType:   FundingPrivate,
		UnitFee:       12000, // $120.00
		SubsidyAmount: 0,     // no subsidy
	}

	tests := []struct {
		name              string
		lines             []BillingLine
		wantTotal         int64
		wantSubsidy       int64
		wantPatientAmount int64
	}{
		{
			name:              "empty lines",
			lines:             []BillingLine{},
			wantTotal:         0,
			wantSubsidy:       0,
			wantPatientAmount: 0,
		},
		{
			name: "single ACC line quantity 1",
			lines: []BillingLine{
				{ServiceCode: accService, Quantity: 1, ProviderID: "HPI12345", Date: now},
			},
			wantTotal:         5000,
			wantSubsidy:       4000,
			wantPatientAmount: 1000,
		},
		{
			name: "single ACC line quantity 3",
			lines: []BillingLine{
				{ServiceCode: accService, Quantity: 3, ProviderID: "HPI12345", Date: now},
			},
			wantTotal:         15000,
			wantSubsidy:       12000,
			wantPatientAmount: 3000,
		},
		{
			name: "mixed PHO and private lines",
			lines: []BillingLine{
				{ServiceCode: phoService, Quantity: 1, ProviderID: "HPI12345", Date: now},
				{ServiceCode: privateService, Quantity: 1, ProviderID: "HPI12345", Date: now},
			},
			// total = 3500 + 12000 = 15500; subsidy = 3000 + 0 = 3000; patient = 12500
			wantTotal:         15500,
			wantSubsidy:       3000,
			wantPatientAmount: 12500,
		},
		{
			name: "subsidy exceeds total clamps patient amount to zero",
			lines: []BillingLine{
				{
					ServiceCode: ServiceCode{
						UnitFee:       100,
						SubsidyAmount: 200, // subsidy greater than fee
					},
					Quantity: 1,
					Date:     now,
				},
			},
			wantTotal:         100,
			wantSubsidy:       200,
			wantPatientAmount: 0, // clamped, not negative
		},
		{
			name: "multiple lines with mixed quantities",
			lines: []BillingLine{
				{ServiceCode: accService, Quantity: 2, ProviderID: "HPI00001", Date: now},
				{ServiceCode: phoService, Quantity: 1, ProviderID: "HPI00002", Date: now},
			},
			// total = 5000*2 + 3500*1 = 13500; subsidy = 4000*2 + 3000*1 = 11000; patient = 2500
			wantTotal:         13500,
			wantSubsidy:       11000,
			wantPatientAmount: 2500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, subsidy, patientAmount := CalculateTotals(tt.lines)
			assert.Equal(t, tt.wantTotal, total, "total mismatch")
			assert.Equal(t, tt.wantSubsidy, subsidy, "subsidy mismatch")
			assert.Equal(t, tt.wantPatientAmount, patientAmount, "patientAmount mismatch")
		})
	}
}

func TestFormatNZD(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{0, "$0.00"},
		{100, "$1.00"},
		{1050, "$10.50"},
		{123456, "$1234.56"},
		{1, "$0.01"},
		{99, "$0.99"},
		// Negative amounts.
		{-50, "-$0.50"},
		{-100, "-$1.00"},
		{-9999, "-$99.99"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatNZD(tt.cents)
			assert.Equal(t, tt.want, got, "FormatNZD(%d)", tt.cents)
		})
	}
}
