package acc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDentalClaimValidate_EmptyClaim(t *testing.T) {
	c := &DentalClaim{}
	result := c.Validate()
	assert.False(t, result.Valid)
	assert.GreaterOrEqual(t, len(result.Errors), 6)

	codes := make(map[string]bool)
	for _, e := range result.Errors {
		codes[e.Code] = true
	}
	assert.True(t, codes["MISSING_NHI"])
	assert.True(t, codes["MISSING_HPI"])
	assert.True(t, codes["MISSING_PRACTICE"])
	assert.True(t, codes["MISSING_ACCIDENT_DATE"])
	assert.True(t, codes["MISSING_TEETH"])
	assert.True(t, codes["MISSING_FORM"])
}

func TestDentalClaimValidate_FutureDate(t *testing.T) {
	c := &DentalClaim{
		PatientNHI:     "ZAB000H",
		ProviderHPI:    "1234567",
		PracticeID:     "P001",
		AccidentDate:   time.Now().Add(24 * time.Hour),
		ACCFormNumber:  "ACC45",
		Teeth:          []ToothInjury{{ToothCode: "11", FeeInCents: 1000}},
	}
	result := c.Validate()
	assert.False(t, result.Valid)
	codes := make(map[string]bool)
	for _, e := range result.Errors {
		codes[e.Code] = true
	}
	assert.True(t, codes["FUTURE_ACCIDENT_DATE"])
}

func TestDentalClaimValidate_ToothErrors(t *testing.T) {
	c := &DentalClaim{
		PatientNHI:    "ZAB000H",
		ProviderHPI:   "1234567",
		PracticeID:    "P001",
		AccidentDate:  time.Now(),
		ACCFormNumber: "ACC45",
		Teeth: []ToothInjury{
			{ToothCode: "", FeeInCents: 0},
		},
	}
	result := c.Validate()
	codes := make(map[string]bool)
	for _, e := range result.Errors {
		codes[e.Code] = true
	}
	assert.True(t, codes["MISSING_TOOTH_CODE"])
	assert.True(t, codes["INVALID_FEE"])
}

func TestDentalClaimValidate_RecalculatesTotalFee(t *testing.T) {
	c := &DentalClaim{
		PatientNHI:    "ZAB000H",
		ProviderHPI:   "1234567",
		PracticeID:    "P001",
		AccidentDate:  time.Now(),
		ACCFormNumber: "ACC45",
		Teeth: []ToothInjury{
			{ToothCode: "11", FeeInCents: 500},
			{ToothCode: "21", FeeInCents: 750},
		},
	}
	result := c.Validate()
	assert.True(t, result.Valid)
	assert.Equal(t, 1250, c.TotalFee)
}

func TestDentalClaimValidate_ValidClaim(t *testing.T) {
	c := &DentalClaim{
		PatientNHI:    "ZAB000H",
		ProviderHPI:   "1234567",
		PracticeID:    "P001",
		AccidentDate:  time.Now().Add(-24 * time.Hour),
		ACCFormNumber: "ACC45",
		Teeth: []ToothInjury{
			{ToothCode: "11", FeeInCents: 500, ACCProcedure: "C1", DCNZCode: "111"},
		},
	}
	result := c.Validate()
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestBuildACCClaimPayload(t *testing.T) {
	c := &DentalClaim{
		PatientNHI:    "ZAB000H",
		ProviderHPI:   "1234567",
		AccidentDate:  time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		ACCFormNumber: "ACC45",
		TotalFee:      1250,
		Teeth: []ToothInjury{
			{ToothCode: "11", Surface: "MOD", ACCProcedure: "C1", DCNZCode: "111", FeeInCents: 500, Diagnosis: "Crown fracture"},
			{ToothCode: "21", Surface: "I", ACCProcedure: "D1", DCNZCode: "121", FeeInCents: 750, Diagnosis: "Enamel fracture"},
		},
	}
	payload := c.BuildACCClaimPayload()
	assert.Equal(t, "Claim", payload["resourceType"])

	diagnoses := payload["diagnosis"].([]any)
	assert.Len(t, diagnoses, 2)

	items := payload["item"].([]any)
	assert.Len(t, items, 2)

	total := payload["total"].(map[string]any)
	assert.Equal(t, 12.5, total["value"])
}

func TestToJSON(t *testing.T) {
	c := &DentalClaim{
		ID:     "claim-1",
		Status: ClaimDraft,
	}
	s, err := c.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, s, "claim-1")
}

func TestDentalInjuryTypes(t *testing.T) {
	types := DentalInjuryTypes()
	assert.Len(t, types, 12)
	for _, dt := range types {
		assert.NotEmpty(t, dt.Code)
		assert.NotEmpty(t, dt.Description)
	}
}
