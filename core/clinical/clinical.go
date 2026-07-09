// Package clinical provides clinical decision support, scoring engines,
// and dosing calculators for NZ hospital practice.
package clinical

import (
	"fmt"
	"math"
)

// ---------------------------------------------------------------------------
// EWS — National Early Warning Score (NEWS2) for adults
// ---------------------------------------------------------------------------

// EWSInput holds the vital signs needed for NEWS2 calculation.
type EWSInput struct {
	RespirationRate int     `json:"respirationRate"`
	SpO2Scale       int     `json:"spo2Scale"`       // 1 or 2 (NEWS2 has two scales)
	SpO2Percent     float64 `json:"spo2Percent"`
	AirOrOxygen     string  `json:"airOrOxygen"`      // "air" or "oxygen"
	SystolicBP      int     `json:"systolicBp"`
	HeartRate       int     `json:"heartRate"`
	Temperature     float64 `json:"temperature"`      // Celsius
	Consciousness   string  `json:"consciousness"`     // "alert", "CVPU", "voice", "pain", "unresponsive"
}

// EWSResult contains the NEWS2 score and clinical risk.
type EWSResult struct {
	TotalScore   int    `json:"totalScore"`
	ClinicalRisk string `json:"clinicalRisk"` // "low", "low-medium", "medium", "high"
	IndividualScores map[string]int `json:"individualScores"`
}

// CalculateEWS computes the NEWS2 score from vital signs.
func CalculateEWS(in EWSInput) EWSResult {
	scores := make(map[string]int)
	scores["respiration"] = scoreRespiration(in.RespirationRate)
	scores["spo2"] = scoreSpO2(in.SpO2Scale, in.SpO2Percent)
	scores["air_or_oxygen"] = scoreAirOrOxygen(in.AirOrOxygen)
	scores["systolic_bp"] = scoreSystolicBP(in.SystolicBP)
	scores["heart_rate"] = scoreHeartRate(in.HeartRate)
	scores["temperature"] = scoreTemperature(in.Temperature)
	scores["consciousness"] = scoreConsciousness(in.Consciousness)

	total := 0
	for _, v := range scores {
		total += v
	}

	return EWSResult{
		TotalScore:      total,
		ClinicalRisk:    ewsRisk(total),
		IndividualScores: scores,
	}
}

func scoreRespiration(rr int) int {
	switch {
	case rr <= 8: return 3
	case rr <= 11: return 1
	case rr <= 20: return 0
	case rr <= 24: return 2
	default: return 3
	}
}

func scoreSpO2(scale int, spo2 float64) int {
	if scale == 2 {
		switch {
		case spo2 <= 83: return 3
		case spo2 <= 86: return 2
		case spo2 <= 87: return 1
		case spo2 <= 88: return 1 // scale 2: 88% = 1
		default: return 0
		}
	}
	// Scale 1 (default)
	switch {
	case spo2 <= 91: return 3
	case spo2 <= 92: return 2
	case spo2 <= 93: return 1
	case spo2 <= 94: return 0
	case spo2 <= 95: return 1 // borderline
	default: return 0
	}
}

func scoreAirOrOxygen(air string) int {
	if air == "oxygen" {
		return 2
	}
	return 0
}

func scoreSystolicBP(bp int) int {
	switch {
	case bp <= 90: return 3
	case bp <= 100: return 2
	case bp <= 110: return 1
	case bp <= 219: return 0
	default: return 3
	}
}

func scoreHeartRate(hr int) int {
	switch {
	case hr <= 40: return 3
	case hr <= 50: return 1
	case hr <= 90: return 0
	case hr <= 110: return 1
	case hr <= 130: return 2
	default: return 3
	}
}

func scoreTemperature(temp float64) int {
	switch {
	case temp <= 35.0: return 3
	case temp <= 36.0: return 1
	case temp <= 38.0: return 0
	case temp <= 39.0: return 1
	default: return 2
	}
}

func scoreConsciousness(c string) int {
	if c == "alert" {
		return 0
	}
	return 3 // CVPU or any other non-alert state
}

func ewsRisk(score int) string {
	switch {
	case score == 0: return "low"
	case score <= 4: return "low-medium"
	case score <= 6: return "medium"
	default: return "high"
	}
}

// ---------------------------------------------------------------------------
// PEWS — Paediatric Early Warning Score
// ---------------------------------------------------------------------------

// PEWSInput holds vital signs for paediatric scoring.
type PEWSInput struct {
	HeartRate       int     `json:"heartRate"`
	RespirationRate int     `json:"respirationRate"`
	SpO2Percent     float64 `json:"spo2Percent"`
	SystolicBP      int     `json:"systolicBp"`
	Temperature     float64 `json:"temperature"`
	Consciousness   string  `json:"consciousness"`     // "alert", "voice", "pain", "unresponsive"
	Behaviour       string  `json:"behaviour"`          // "normal", "irritable", "lethargic", "confused"
	FluidIntake     string  `json:"fluidIntake"`        // "normal", "reduced", "nil", "IV"
	PainScore       int     `json:"painScore"`          // 0-10
}

// PEWSResult contains the PEWS score and escalation recommendation.
type PEWSResult struct {
	TotalScore      int    `json:"totalScore"`
	Escalation      string `json:"escalation"` // "none", "ward-review", "rapid-response", "picu-referral"
	IndividualScores map[string]int `json:"individualScores"`
}

// CalculatePEWS computes the Paediatric Early Warning Score.
// Age-stratified thresholds are simplified; real implementation would
// use NZ Paediatric Trigger Tool age bands.
func CalculatePEWS(in PEWSInput) PEWSResult {
	scores := make(map[string]int)
	scores["heart_rate"] = pewsHeartRate(in.HeartRate)
	scores["respiration"] = pewsRespiration(in.RespirationRate)
	scores["spo2"] = pewsSpO2(in.SpO2Percent)
	scores["blood_pressure"] = pewsBP(in.SystolicBP)
	scores["temperature"] = pewsTemp(in.Temperature)
	scores["consciousness"] = pewsConsciousness(in.Consciousness)
	scores["behaviour"] = pewsBehaviour(in.Behaviour)
	scores["fluid_intake"] = pewsFluidIntake(in.FluidIntake)
	scores["pain"] = pewsPain(in.PainScore)

	total := 0
	for _, v := range scores {
		total += v
	}

	return PEWSResult{
		TotalScore:      total,
		Escalation:      pewsEscalation(total),
		IndividualScores: scores,
	}
}

func pewsHeartRate(hr int) int {
	switch {
	case hr <= 60: return 1
	case hr <= 120: return 0
	case hr <= 160: return 1
	default: return 2
	}
}

func pewsRespiration(rr int) int {
	switch {
	case rr <= 12: return 0
	case rr <= 30: return 0
	default: return 2
	}
}

func pewsSpO2(spo2 float64) int {
	switch {
	case spo2 >= 95: return 0
	case spo2 >= 92: return 1
	default: return 2
	}
}

func pewsBP(bp int) int {
	switch {
	case bp <= 70: return 2
	case bp <= 90: return 1
	default: return 0
	}
}

func pewsTemp(temp float64) int {
	switch {
	case temp <= 36.0: return 1
	case temp <= 38.0: return 0
	default: return 1
	}
}

func pewsConsciousness(c string) int {
	if c == "alert" {
		return 0
	}
	return 2
}

func pewsBehaviour(b string) int {
	switch b {
	case "normal": return 0
	case "irritable": return 1
	default: return 2
	}
}

func pewsFluidIntake(f string) int {
	switch f {
	case "normal": return 0
	case "reduced": return 1
	default: return 2
	}
}

func pewsPain(pain int) int {
	if pain <= 3 {
		return 0
	}
	if pain <= 6 {
		return 1
	}
	return 2
}

func pewsEscalation(score int) string {
	switch {
	case score <= 1: return "none"
	case score <= 3: return "ward-review"
	case score <= 5: return "rapid-response"
	default: return "picu-referral"
	}
}

// ---------------------------------------------------------------------------
// Paediatric Dosing Calculator
// ---------------------------------------------------------------------------

// DoseInput specifies parameters for weight-based dosing.
type DoseInput struct {
	DrugName       string  `json:"drugName"`
	WeightKg       float64 `json:"weightKg"`
	DosePerKg      float64 `json:"dosePerKg"`      // mg/kg per dose
	Frequency      string  `json:"frequency"`       // e.g. "BD", "TDS", "QDS", "PRN"
	MaxSingleDose  float64 `json:"maxSingleDose"`   // mg, safety cap
	MaxDailyDose   float64 `json:"maxDailyDose"`    // mg/day, safety cap
	Route          string  `json:"route"`            // oral, IV, IM, SC, inhaled, topical
	AgeMonths      int     `json:"ageMonths"`
}

// DoseResult contains the calculated dose.
type DoseResult struct {
	DrugName        string  `json:"drugName"`
	CalculatedDose  float64 `json:"calculatedDose"`  // mg per dose
	CappedDose      float64 `json:"cappedDose"`      // after max-dose cap applied
	DailyDose       float64 `json:"dailyDose"`       // mg/day based on frequency
	DosesPerDay     int     `json:"dosesPerDay"`
	DosePerKg       float64 `json:"dosePerKg"`
	WeightKg        float64 `json:"weightKg"`
	Route           string  `json:"route"`
	CappingApplied  bool    `json:"cappingApplied"`
	Warnings        []string `json:"warnings,omitempty"`
}

// CalculatePaedDose computes weight-based paediatric dosing with safety caps.
func CalculatePaedDose(in DoseInput) DoseResult {
	dosesPerDay := frequencyToCount(in.Frequency)
	rawDose := in.WeightKg * in.DosePerKg
	cappedDose := rawDose
	cappingApplied := false
	var warnings []string

	if in.MaxSingleDose > 0 && rawDose > in.MaxSingleDose {
		cappedDose = in.MaxSingleDose
		cappingApplied = true
		warnings = append(warnings, fmt.Sprintf("single dose capped from %.1fmg to %.1fmg", rawDose, in.MaxSingleDose))
	}

	dailyDose := cappedDose * float64(dosesPerDay)
	if in.MaxDailyDose > 0 && dailyDose > in.MaxDailyDose {
		dailyDose = in.MaxDailyDose
		cappedDose = dailyDose / float64(dosesPerDay)
		cappingApplied = true
		warnings = append(warnings, fmt.Sprintf("daily dose capped to %.1fmg/day", in.MaxDailyDose))
	}

	if in.AgeMonths < 1 {
		warnings = append(warnings, "neonatal patient — verify dose with neonatal formulary")
	}

	return DoseResult{
		DrugName:       in.DrugName,
		CalculatedDose: math.Round(rawDose*10) / 10,
		CappedDose:     math.Round(cappedDose*10) / 10,
		DailyDose:      math.Round(dailyDose*10) / 10,
		DosesPerDay:    dosesPerDay,
		DosePerKg:      in.DosePerKg,
		WeightKg:       in.WeightKg,
		Route:          in.Route,
		CappingApplied: cappingApplied,
		Warnings:       warnings,
	}
}

func frequencyToCount(freq string) int {
	switch freq {
	case "QDS", "qds", "QID", "qid": return 4
	case "TDS", "tds", "TID", "tid": return 3
	case "BD", "bd", "BID", "bid": return 2
	case "OD", "od", "QD", "qd", "daily": return 1
	case "PRN", "prn": return 1
	case "nocte", "NOCTE": return 1
	case " mane", "MANE": return 1
	default: return 1
	}
}
