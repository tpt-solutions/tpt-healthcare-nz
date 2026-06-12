// Package scoring provides clinical early-warning score calculators.
// NEWS2 (National Early Warning Score 2) is the standard deterioration
// detector used in NZ hospital settings under HNZAS accreditation requirements.
package scoring

import (
	"context"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/events"
	"github.com/google/uuid"
)

// VitalSigns holds a single set of patient observations used for NEWS2 calculation.
// Zero values are treated as "not measured" and excluded from scoring.
type VitalSigns struct {
	// RespiratoryRate in breaths per minute.
	RespiratoryRate float64
	// SpO2 is peripheral oxygen saturation as a percentage (0–100).
	SpO2 float64
	// SupplementalOxygen is true when the patient is receiving any supplemental O2.
	SupplementalOxygen bool
	// SystolicBP in mmHg.
	SystolicBP float64
	// HeartRate in beats per minute.
	HeartRate float64
	// Temperature in degrees Celsius.
	Temperature float64
	// AVPU consciousness level: "alert", "voice", "pain", "unresponsive".
	AVPU string
	// NewConfusion is true when acute confusion is present (adds 3 to consciousness score).
	NewConfusion bool

	// PatientID is the FHIR Patient resource ID (used for event publishing).
	PatientID string
	// EncounterID is the current FHIR Encounter resource ID.
	EncounterID string
	// RecordedAt is when the vitals were taken.
	RecordedAt time.Time
}

// NEWS2Score holds the component and aggregate NEWS2 scores.
type NEWS2Score struct {
	// RespRate is the respiratory rate sub-score (0, 1, 2, or 3).
	RespRate int
	// SpO2Score is the oxygen saturation sub-score using Scale 1 (standard).
	SpO2Score int
	// SupplementalO2 is 2 if the patient is on supplemental oxygen, else 0.
	SupplementalO2 int
	// SystolicBPScore is the systolic BP sub-score.
	SystolicBPScore int
	// HeartRateScore is the heart rate sub-score.
	HeartRateScore int
	// TempScore is the temperature sub-score.
	TempScore int
	// ConsciousnessScore is 0 (alert) or 3 (CVPU / new confusion).
	ConsciousnessScore int

	// Total is the aggregate NEWS2 score.
	Total int
	// ClinicalRisk maps to: 0=low, 1–4=low-medium, 5–6=medium, ≥7=high.
	ClinicalRisk string
	// EscalationAction is the recommended clinical response per the NEWS2 protocol.
	EscalationAction string

	// Input holds the vital signs used for this calculation.
	Input VitalSigns
	// CalculatedAt is the timestamp of the calculation.
	CalculatedAt time.Time
}

// Calculate computes the NEWS2 score from a VitalSigns set.
// Only parameters with non-zero values contribute to the score; this allows
// partial scoring when not all vitals have been recorded.
func Calculate(v VitalSigns) NEWS2Score {
	s := NEWS2Score{Input: v, CalculatedAt: time.Now().UTC()}

	if v.RespiratoryRate > 0 {
		s.RespRate = scoreRespRate(v.RespiratoryRate)
	}
	if v.SpO2 > 0 {
		s.SpO2Score = scoreSpO2(v.SpO2)
	}
	if v.SupplementalOxygen {
		s.SupplementalO2 = 2
	}
	if v.SystolicBP > 0 {
		s.SystolicBPScore = scoreSystolicBP(v.SystolicBP)
	}
	if v.HeartRate > 0 {
		s.HeartRateScore = scoreHeartRate(v.HeartRate)
	}
	if v.Temperature > 0 {
		s.TempScore = scoreTemperature(v.Temperature)
	}
	s.ConsciousnessScore = scoreConsciousness(v.AVPU, v.NewConfusion)

	s.Total = s.RespRate + s.SpO2Score + s.SupplementalO2 +
		s.SystolicBPScore + s.HeartRateScore + s.TempScore + s.ConsciousnessScore

	s.ClinicalRisk, s.EscalationAction = riskAndAction(s.Total, s.RespRate, s.ConsciousnessScore)
	return s
}

// DeteriorationEvent is published to the domain events bus when a NEWS2 score
// crosses a threshold that requires clinical escalation.
const DeteriorationEventType = "patient.deterioration.news2"

// DeteriorationPayload is the payload of a DeteriorationEvent.
type DeteriorationPayload struct {
	PatientID        string    `json:"patientId"`
	EncounterID      string    `json:"encounterId"`
	NEWS2Score       int       `json:"news2Score"`
	ClinicalRisk     string    `json:"clinicalRisk"`
	EscalationAction string    `json:"escalationAction"`
	CalculatedAt     time.Time `json:"calculatedAt"`
}

// AlertThreshold is the NEWS2 total score at or above which a deterioration
// event is published to the domain event bus.
const AlertThreshold = 5

// CheckAndAlert calculates the NEWS2 score and, if the total is at or above
// AlertThreshold, publishes a DeteriorationEvent to the supplied event bus.
// The returned score is always valid regardless of whether an event was published.
func CheckAndAlert(ctx context.Context, bus *events.Bus, v VitalSigns) (NEWS2Score, error) {
	score := Calculate(v)

	if score.Total < AlertThreshold {
		return score, nil
	}

	payload := DeteriorationPayload{
		PatientID:        v.PatientID,
		EncounterID:      v.EncounterID,
		NEWS2Score:       score.Total,
		ClinicalRisk:     score.ClinicalRisk,
		EscalationAction: score.EscalationAction,
		CalculatedAt:     score.CalculatedAt,
	}

	if err := bus.Publish(ctx, events.Event{
		ID:          uuid.New(),
		Type:        DeteriorationEventType,
		AggregateID: v.PatientID,
		Payload:     payload,
	}); err != nil {
		return score, fmt.Errorf("news2: publishing deterioration event: %w", err)
	}

	return score, nil
}

// ---------------------------------------------------------------------------
// Sub-score functions — NEWS2 scoring table (Royal College of Physicians 2017)
// ---------------------------------------------------------------------------

func scoreRespRate(rr float64) int {
	switch {
	case rr <= 8:
		return 3
	case rr <= 11:
		return 1
	case rr <= 20:
		return 0
	case rr <= 24:
		return 2
	default:
		return 3
	}
}

func scoreSpO2(spo2 float64) int {
	// Scale 1 (no hypercapnic respiratory failure).
	switch {
	case spo2 <= 91:
		return 3
	case spo2 <= 93:
		return 2
	case spo2 <= 95:
		return 1
	default:
		return 0
	}
}

func scoreSystolicBP(sbp float64) int {
	switch {
	case sbp <= 90:
		return 3
	case sbp <= 100:
		return 2
	case sbp <= 110:
		return 1
	case sbp <= 219:
		return 0
	default:
		return 3
	}
}

func scoreHeartRate(hr float64) int {
	switch {
	case hr <= 40:
		return 3
	case hr <= 50:
		return 1
	case hr <= 90:
		return 0
	case hr <= 110:
		return 1
	case hr <= 130:
		return 2
	default:
		return 3
	}
}

func scoreTemperature(temp float64) int {
	switch {
	case temp <= 35.0:
		return 3
	case temp <= 36.0:
		return 1
	case temp <= 38.0:
		return 0
	case temp <= 39.0:
		return 1
	default:
		return 2
	}
}

func scoreConsciousness(avpu string, newConfusion bool) int {
	if newConfusion {
		return 3
	}
	switch avpu {
	case "alert", "A":
		return 0
	default:
		// V (voice), P (pain), U (unresponsive) → 3
		return 3
	}
}

func riskAndAction(total, respScore, consciousnessScore int) (risk string, action string) {
	// A single parameter score of 3 triggers a minimum medium response.
	anyThree := respScore == 3 || consciousnessScore == 3

	switch {
	case total == 0:
		return "low", "Minimum 12-hourly monitoring."
	case total <= 4 && !anyThree:
		return "low", "Minimum 4–6 hourly monitoring; inform the registering clinician."
	case total <= 4 && anyThree:
		return "low-medium", "Urgent review by competent clinician within 30 minutes; consider increasing monitoring frequency."
	case total <= 6:
		return "medium", "Urgent review by registered clinician; consider HDU. Monitor every hour."
	default:
		return "high", "Emergency assessment by clinical team with critical care competencies. Continuous monitoring."
	}
}
