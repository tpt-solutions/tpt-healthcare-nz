// Package ddi provides drug-drug and drug-allergy interaction checking for
// clinical decision support at the point of prescribing. The Checker interface
// abstracts the backing data source so that a local NZMT-derived database,
// the PHARMAC interaction API, or a remote MIMS service can be swapped without
// changing call sites.
//
// Integration point: call Checker.Check before persisting a MedicationRequest.
// If any Interaction with Severity >= SeverityMajor is returned, the prescribing
// handler must surface the alert to the clinician before allowing the request
// to proceed.
package ddi

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Severity classifies how clinically significant an interaction is.
type Severity int

const (
	SeverityNone           Severity = 0
	SeverityMinor          Severity = 1
	SeverityModerate       Severity = 2
	SeverityMajor          Severity = 3
	SeverityContraindicated Severity = 4
)

func (s Severity) String() string {
	switch s {
	case SeverityMinor:
		return "minor"
	case SeverityModerate:
		return "moderate"
	case SeverityMajor:
		return "major"
	case SeverityContraindicated:
		return "contraindicated"
	default:
		return "none"
	}
}

// SeverityFromString parses a severity label as returned by upstream APIs.
func SeverityFromString(s string) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "contraindicated":
		return SeverityContraindicated
	case "major":
		return SeverityMajor
	case "moderate":
		return SeverityModerate
	case "minor":
		return SeverityMinor
	default:
		return SeverityNone
	}
}

// InteractionKind distinguishes whether the interaction is between two drugs
// or between a drug and a recorded patient allergy.
type InteractionKind string

const (
	KindDrugDrug   InteractionKind = "drug-drug"
	KindDrugAllergy InteractionKind = "drug-allergy"
)

// Interaction describes a clinically significant interaction found during the check.
type Interaction struct {
	// Kind indicates whether this is a drug-drug or drug-allergy interaction.
	Kind InteractionKind `json:"kind"`
	// Drug1 is the NZULM of the first (or only) medicine involved.
	Drug1 string `json:"drug1"`
	// Drug2 is the NZULM of the second medicine, or the allergy substance name for drug-allergy interactions.
	Drug2 string `json:"drug2"`
	// Severity classifies the clinical risk.
	Severity Severity `json:"severity"`
	// SeverityLabel is the human-readable severity string.
	SeverityLabel string `json:"severityLabel"`
	// Description is a plain-language summary of the interaction mechanism and
	// recommended management.
	Description string `json:"description"`
	// Source identifies the data source that produced this interaction record
	// (e.g. "pharmac-api", "nzmt-local", "mims").
	Source string `json:"source"`
	// CheckedAt records when the check was performed.
	CheckedAt time.Time `json:"checkedAt"`
}

// CheckRequest describes the input for an interaction check.
type CheckRequest struct {
	// NZULMs is the list of NZULM identifiers for all medicines to check against
	// each other (drug-drug). At least one must be provided.
	NZULMs []string
	// PatientAllergies is the list of allergy substance names or SNOMED codes
	// recorded for the patient. May be empty when no allergies are recorded.
	PatientAllergies []string
	// ProposedNZULM is the NZULM of the medicine being prescribed. It will be
	// checked against all existing NZULMs and all patient allergies.
	ProposedNZULM string
}

// Checker is the interface that all DDI backends must implement.
type Checker interface {
	// Check performs drug-drug and drug-allergy interaction analysis.
	// Returns all interactions found; an empty slice means no interactions.
	// An error means the check could not be performed (e.g. service unavailable).
	Check(ctx context.Context, req CheckRequest) ([]Interaction, error)
}

// MultiChecker tries each Checker in order and merges the results, deduplicating
// by (Drug1, Drug2, Kind). Use it to combine a fast local cache with a
// comprehensive remote service.
type MultiChecker struct {
	checkers []Checker
}

// NewMultiChecker creates a MultiChecker that calls each checker in order.
// If any checker returns an error it is skipped; at least one must succeed.
func NewMultiChecker(checkers ...Checker) *MultiChecker {
	return &MultiChecker{checkers: checkers}
}

// Check calls each underlying Checker and merges deduplicated results.
func (m *MultiChecker) Check(ctx context.Context, req CheckRequest) ([]Interaction, error) {
	seen := make(map[string]struct{})
	var merged []Interaction
	var lastErr error

	for _, c := range m.checkers {
		results, err := c.Check(ctx, req)
		if err != nil {
			lastErr = err
			continue
		}
		for _, ix := range results {
			key := fmt.Sprintf("%s|%s|%s", ix.Kind, ix.Drug1, ix.Drug2)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, ix)
		}
	}

	if len(merged) == 0 && lastErr != nil {
		return nil, fmt.Errorf("ddi: all checkers failed: %w", lastErr)
	}
	return merged, nil
}

// Filter returns only interactions at or above the given severity threshold.
func Filter(interactions []Interaction, minSeverity Severity) []Interaction {
	out := interactions[:0:0]
	for _, ix := range interactions {
		if ix.Severity >= minSeverity {
			out = append(out, ix)
		}
	}
	return out
}

// HighestSeverity returns the maximum Severity across all interactions, or
// SeverityNone when the slice is empty.
func HighestSeverity(interactions []Interaction) Severity {
	max := SeverityNone
	for _, ix := range interactions {
		if ix.Severity > max {
			max = ix.Severity
		}
	}
	return max
}
