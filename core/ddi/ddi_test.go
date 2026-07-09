package ddi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityNone, "none"},
		{SeverityMinor, "minor"},
		{SeverityModerate, "moderate"},
		{SeverityMajor, "major"},
		{SeverityContraindicated, "contraindicated"},
		{Severity(99), "none"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.severity.String())
		})
	}
}

func TestSeverityFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected Severity
	}{
		{"contraindicated", SeverityContraindicated},
		{"CONTRAINDICATED", SeverityContraindicated},
		{" major ", SeverityMajor},
		{"moderate", SeverityModerate},
		{"minor", SeverityMinor},
		{"unknown", SeverityNone},
		{"", SeverityNone},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SeverityFromString(tt.input))
		})
	}
}

func TestFilter(t *testing.T) {
	interactions := []Interaction{
		{Severity: SeverityMinor},
		{Severity: SeverityModerate},
		{Severity: SeverityMajor},
		{Severity: SeverityContraindicated},
	}

	t.Run("empty slice", func(t *testing.T) {
		assert.Empty(t, Filter(nil, SeverityMinor))
	})

	t.Run("all above", func(t *testing.T) {
		result := Filter(interactions, SeverityMinor)
		assert.Len(t, result, 4)
	})

	t.Run("some above", func(t *testing.T) {
		result := Filter(interactions, SeverityMajor)
		assert.Len(t, result, 2)
	})

	t.Run("exact threshold", func(t *testing.T) {
		result := Filter(interactions, SeverityContraindicated)
		assert.Len(t, result, 1)
	})

	t.Run("above all", func(t *testing.T) {
		result := Filter(interactions, Severity(5))
		assert.Empty(t, result)
	})
}

func TestHighestSeverity(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, SeverityNone, HighestSeverity(nil))
	})

	t.Run("single", func(t *testing.T) {
		assert.Equal(t, SeverityMajor, HighestSeverity([]Interaction{{Severity: SeverityMajor}}))
	})

	t.Run("mixed", func(t *testing.T) {
		interactions := []Interaction{
			{Severity: SeverityMinor},
			{Severity: SeverityContraindicated},
			{Severity: SeverityModerate},
		}
		assert.Equal(t, SeverityContraindicated, HighestSeverity(interactions))
	})
}

func TestLocalChecker_Check(t *testing.T) {
	genericNames := map[string]string{
		"warf001":    "warfarin",
		"asp001":     "aspirin",
		"fluc001":    "fluconazole",
		"ibu001":     "ibuprofen",
		"tram001":    "tramadol",
		"fluox001":   "fluoxetine",
		"ssri001":    "ssri",
	}
	checker := NewLocalChecker(genericNames)

	t.Run("warfarin + aspirin = major", func(t *testing.T) {
		result, err := checker.Check(context.Background(), CheckRequest{
			NZULMs:         []string{"warf001"},
			ProposedNZULM:  "asp001",
		})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, SeverityMajor, result[0].Severity)
		assert.Equal(t, KindDrugDrug, result[0].Kind)
	})

	t.Run("warfarin + fluconazole = contraindicated", func(t *testing.T) {
		result, err := checker.Check(context.Background(), CheckRequest{
			NZULMs:         []string{"warf001"},
			ProposedNZULM:  "fluc001",
		})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, SeverityContraindicated, result[0].Severity)
	})

	t.Run("order independence", func(t *testing.T) {
		result, err := checker.Check(context.Background(), CheckRequest{
			NZULMs:         []string{"asp001"},
			ProposedNZULM:  "warf001",
		})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, SeverityMajor, result[0].Severity)
	})

	t.Run("no interaction", func(t *testing.T) {
		result, err := checker.Check(context.Background(), CheckRequest{
			NZULMs:         []string{"asp001"},
			ProposedNZULM:  "asp001",
		})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("drug-allergy match", func(t *testing.T) {
		result, err := checker.Check(context.Background(), CheckRequest{
			NZULMs:         []string{},
			PatientAllergies: []string{"aspirin"},
			ProposedNZULM:   "asp001",
		})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, SeverityContraindicated, result[0].Severity)
		assert.Equal(t, KindDrugAllergy, result[0].Kind)
	})

	t.Run("drug-allergy no match", func(t *testing.T) {
		result, err := checker.Check(context.Background(), CheckRequest{
			NZULMs:         []string{},
			PatientAllergies: []string{"penicillin"},
			ProposedNZULM:   "asp001",
		})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("NZULM not in map falls back to raw value", func(t *testing.T) {
		result, err := checker.Check(context.Background(), CheckRequest{
			NZULMs:         []string{"unknown-drug"},
			ProposedNZULM:  "asp001",
		})
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestMultiChecker_Check(t *testing.T) {
	t.Run("merge results from two checkers", func(t *testing.T) {
		c1 := &DDIChecker{Interactions: []Interaction{
			{Kind: KindDrugDrug, Drug1: "A", Drug2: "B", Severity: SeverityMinor},
		}}
		c2 := &DDIChecker{Interactions: []Interaction{
			{Kind: KindDrugDrug, Drug1: "C", Drug2: "D", Severity: SeverityMajor},
		}}

		mc := NewMultiChecker(c1, c2)
		result, err := mc.Check(context.Background(), CheckRequest{})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("deduplication", func(t *testing.T) {
		c1 := &DDIChecker{Interactions: []Interaction{
			{Kind: KindDrugDrug, Drug1: "A", Drug2: "B", Severity: SeverityMinor},
		}}
		c2 := &DDIChecker{Interactions: []Interaction{
			{Kind: KindDrugDrug, Drug1: "A", Drug2: "B", Severity: SeverityMajor},
		}}

		mc := NewMultiChecker(c1, c2)
		result, err := mc.Check(context.Background(), CheckRequest{})
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})

	t.Run("first checker fails, second succeeds", func(t *testing.T) {
		c1 := &DDIChecker{Err: assert.AnError}
		c2 := &DDIChecker{Interactions: []Interaction{
			{Kind: KindDrugDrug, Drug1: "A", Drug2: "B"},
		}}

		mc := NewMultiChecker(c1, c2)
		result, err := mc.Check(context.Background(), CheckRequest{})
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})

	t.Run("all checkers fail", func(t *testing.T) {
		c1 := &DDIChecker{Err: assert.AnError}
		c2 := &DDIChecker{Err: assert.AnError}

		mc := NewMultiChecker(c1, c2)
		_, err := mc.Check(context.Background(), CheckRequest{})
		assert.Error(t, err)
	})
}

// DDIChecker is a test helper implementing ddi.Checker.
type DDIChecker struct {
	Interactions []Interaction
	Err          error
}

func (c *DDIChecker) Check(_ context.Context, _ CheckRequest) ([]Interaction, error) {
	return c.Interactions, c.Err
}
