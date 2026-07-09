// Package clinical provides tests for clinical decision support tools.
package clinical

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// EWS Tests
// ---------------------------------------------------------------------------

func TestCalculateEWS_Normal(t *testing.T) {
	in := EWSInput{
		RespirationRate: 16,
		SpO2Scale:       1,
		SpO2Percent:     98,
		AirOrOxygen:     "air",
		SystolicBP:      120,
		HeartRate:       72,
		Temperature:     36.8,
		Consciousness:   "alert",
	}
	result := CalculateEWS(in)
	assert.Equal(t, 0, result.TotalScore)
	assert.Equal(t, "low", result.ClinicalRisk)
}

func TestCalculateEWS_High(t *testing.T) {
	in := EWSInput{
		RespirationRate: 28,
		SpO2Scale:       1,
		SpO2Percent:     88,
		AirOrOxygen:     "oxygen",
		SystolicBP:      80,
		HeartRate:       140,
		Temperature:     40.0,
		Consciousness:   "CVPU",
	}
	result := CalculateEWS(in)
	assert.GreaterOrEqual(t, result.TotalScore, 10)
	assert.Equal(t, "high", result.ClinicalRisk)
}

func TestCalculateEWS_IndividualScores(t *testing.T) {
	in := EWSInput{
		RespirationRate: 8,  // score 3
		SpO2Scale:       1,
		SpO2Percent:     98, // score 0
		AirOrOxygen:     "air",
		SystolicBP:      220, // score 3
		HeartRate:       40,  // score 3
		Temperature:     34.5, // score 3
		Consciousness:   "alert",
	}
	result := CalculateEWS(in)
	assert.Equal(t, 3, result.IndividualScores["respiration"])
	assert.Equal(t, 3, result.IndividualScores["systolic_bp"])
	assert.Equal(t, 3, result.IndividualScores["heart_rate"])
	assert.Equal(t, 3, result.IndividualScores["temperature"])
	assert.Equal(t, 0, result.IndividualScores["consciousness"])
}

// ---------------------------------------------------------------------------
// PEWS Tests
// ---------------------------------------------------------------------------

func TestCalculatePEWS_Normal(t *testing.T) {
	in := PEWSInput{
		HeartRate:       90,
		RespirationRate: 20,
		SpO2Percent:     98,
		SystolicBP:      100,
		Temperature:     37.0,
		Consciousness:   "alert",
		Behaviour:       "normal",
		FluidIntake:     "normal",
		PainScore:       0,
	}
	result := CalculatePEWS(in)
	assert.Equal(t, 0, result.TotalScore)
	assert.Equal(t, "none", result.Escalation)
}

func TestCalculatePEWS_High(t *testing.T) {
	in := PEWSInput{
		HeartRate:       180,
		RespirationRate: 40,
		SpO2Percent:     88,
		SystolicBP:      60,
		Temperature:     40.0,
		Consciousness:   "unresponsive",
		Behaviour:       "lethargic",
		FluidIntake:     "nil",
		PainScore:       9,
	}
	result := CalculatePEWS(in)
	assert.GreaterOrEqual(t, result.TotalScore, 10)
	assert.Equal(t, "picu-referral", result.Escalation)
}

// ---------------------------------------------------------------------------
// Paediatric Dosing Tests
// ---------------------------------------------------------------------------

func TestCalculatePaedDose_Paracetamol(t *testing.T) {
	in := DoseInput{
		DrugName:      "Paracetamol",
		WeightKg:      20,
		DosePerKg:     15,
		Frequency:     "QDS",
		MaxSingleDose: 1000,
		MaxDailyDose:  4000,
		Route:         "oral",
		AgeMonths:     48,
	}
	result := CalculatePaedDose(in)
	assert.Equal(t, 300.0, result.CalculatedDose)
	assert.Equal(t, 300.0, result.CappedDose) // 20*15=300, under cap
	assert.Equal(t, 1200.0, result.DailyDose)  // 300*4
	assert.False(t, result.CappingApplied)
	assert.Empty(t, result.Warnings)
}

func TestCalculatePaedDose_CappingApplied(t *testing.T) {
	in := DoseInput{
		DrugName:      "Paracetamol",
		WeightKg:      80, // large child
		DosePerKg:     15,
		Frequency:     "QDS",
		MaxSingleDose: 1000,
		MaxDailyDose:  4000,
		Route:         "oral",
		AgeMonths:     180,
	}
	result := CalculatePaedDose(in)
	// 80*15=1200, but capped to 1000
	assert.Equal(t, 1200.0, result.CalculatedDose)
	assert.Equal(t, 1000.0, result.CappedDose)
	assert.True(t, result.CappingApplied)
	assert.NotEmpty(t, result.Warnings)
}

func TestCalculatePaedDose_NeonatalWarning(t *testing.T) {
	in := DoseInput{
		DrugName:  "Gentamicin",
		WeightKg:  1.5,
		DosePerKg: 5,
		Frequency: "OD",
		Route:     "IV",
		AgeMonths: 0,
	}
	result := CalculatePaedDose(in)
	assert.Equal(t, 7.5, result.CalculatedDose)
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "neonatal")
}

func TestCalculatePaedDose_DailyDoseCap(t *testing.T) {
	in := DoseInput{
		DrugName:      "Ibuprofen",
		WeightKg:      30,
		DosePerKg:     10,
		Frequency:     "TDS",
		MaxSingleDose: 400,
		MaxDailyDose:  1200,
		Route:         "oral",
		AgeMonths:     120,
	}
	result := CalculatePaedDose(in)
	// 30*10=300 per dose, 300*3=900/day, under daily cap
	assert.Equal(t, 300.0, result.CappedDose)
	assert.Equal(t, 900.0, result.DailyDose)
	assert.False(t, result.CappingApplied)
}

func TestFrequencyToCount(t *testing.T) {
	assert.Equal(t, 4, frequencyToCount("QDS"))
	assert.Equal(t, 3, frequencyToCount("TDS"))
	assert.Equal(t, 2, frequencyToCount("BD"))
	assert.Equal(t, 1, frequencyToCount("OD"))
	assert.Equal(t, 1, frequencyToCount("PRN"))
	assert.Equal(t, 1, frequencyToCount("unknown"))
}
