package repo

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchEngine_Build_EmptyParams(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{ResourceType: "Patient"})
	require.NoError(t, err)
	assert.Empty(t, query)
	assert.Empty(t, args)
}

func TestSearchEngine_Build_ParamID(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"_id": {"patient-123"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "resource_id IN")
	assert.Contains(t, query, "$1")
	assert.Equal(t, "patient-123", args[0])
}

func TestSearchEngine_Build_ParamID_Multiple(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"_id": {"p1", "p2", "p3"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "$1")
	assert.Contains(t, query, "$2")
	assert.Contains(t, query, "$3")
	assert.Len(t, args, 3)
}

func TestSearchEngine_Build_ParamLastUpdated_DefaultEq(t *testing.T) {
	engine := NewSearchEngine()
	_, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"_lastUpdated": {"2026-01-15"}},
	})
	require.NoError(t, err)
	assert.Len(t, args, 1)
}

func TestSearchEngine_Build_ParamLastUpdated_GtPrefix(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"_lastUpdated": {"gt2026-01-15"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, ">")
	assert.Len(t, args, 1)
}

func TestSearchEngine_Build_ParamLastUpdated_LePrefix(t *testing.T) {
	engine := NewSearchEngine()
	query, _, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"_lastUpdated": {"le2026-01-15"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "<=")
}

func TestSearchEngine_Build_ParamLastUpdated_InvalidDate(t *testing.T) {
	engine := NewSearchEngine()
	_, _, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"_lastUpdated": {"not-a-date"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid _lastUpdated")
}

func TestSearchEngine_Build_ParamName(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"name": {"Smith"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "ILIKE")
	assert.Equal(t, "%Smith%", args[0])
}

func TestSearchEngine_Build_ParamFamily(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"family": {"Jones"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "ILIKE")
	assert.Equal(t, "%Jones%", args[0])
}

func TestSearchEngine_Build_ParamGiven(t *testing.T) {
	engine := NewSearchEngine()
	_, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"given": {"John"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "%John%", args[0])
}

func TestSearchEngine_Build_ParamGender(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"gender": {"male"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "data->>'gender'")
	assert.Equal(t, "male", args[0])
}

func TestSearchEngine_Build_ParamStatus(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Encounter",
		Params:       map[string][]string{"status": {"finished"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "data->>'status'")
	assert.Equal(t, "finished", args[0])
}

func TestSearchEngine_Build_ParamContains(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Encounter",
		Params:       map[string][]string{"patient": {"Patient/pat-123"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "@>")
	assert.Contains(t, args[0], "Patient/pat-123")
}

func TestSearchEngine_Build_ParamIdentifier_ValueOnly(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"identifier": {"ZAC1234"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "identifier")
	assert.Equal(t, "ZAC1234", args[0])
}

func TestSearchEngine_Build_ParamIdentifier_SystemValue(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"identifier": {"https://standards.digital.health.nz/ns/nhi-id|ZAC1234"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "@>")
	assert.Contains(t, args[0], "ZAC1234")
}

func TestSearchEngine_Build_UnknownParam(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"unknown_param": {"value"}},
	})
	require.NoError(t, err)
	assert.Empty(t, query)
	assert.Empty(t, args)
}

func TestSearchEngine_Build_MultipleParams(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{
			"gender": {"female"},
			"name":   {"Smith"},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "gender")
	assert.Contains(t, query, "name")
	// gender=1 arg, name=2 args (family match + given match)
	assert.Len(t, args, 3)
}

func TestSearchEngine_RegisterParam(t *testing.T) {
	engine := NewSearchEngine()
	engine.RegisterParam("Patient", "custom_field", paramDef{
		Kind:    paramJSONBText,
		JSONPath: "customField",
	})

	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"custom_field": {"test-value"}},
	})
	require.NoError(t, err)
	assert.Contains(t, query, "customField")
	assert.Equal(t, "test-value", args[0])
}

func TestSearchEngine_Build_EmptyValues(t *testing.T) {
	engine := NewSearchEngine()
	query, args, err := engine.Build(SearchParams{
		ResourceType: "Patient",
		Params:       map[string][]string{"name": {}},
	})
	require.NoError(t, err)
	assert.Empty(t, query)
	assert.Empty(t, args)
}

func TestParsePrefix(t *testing.T) {
	tests := []struct {
		input   string
		op      string
		dateStr string
	}{
		{"eq2026-01-15", "=", "2026-01-15"},
		{"ne2026-01-15", "<>", "2026-01-15"},
		{"lt2026-01-15", "<", "2026-01-15"},
		{"le2026-01-15", "<=", "2026-01-15"},
		{"gt2026-01-15", ">", "2026-01-15"},
		{"ge2026-01-15", ">=", "2026-01-15"},
		{"2026-01-15", "=", "2026-01-15"},
		{"x", "=", "x"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			op, dateStr := parsePrefix(tt.input)
			assert.Equal(t, tt.op, op)
			assert.Equal(t, tt.dateStr, dateStr)
		})
	}
}

func TestSearchParams_Construction(t *testing.T) {
	id := uuid.New()
	sp := SearchParams{
		ResourceType: "Patient",
		TenantID:     id,
		Count:        10,
		Offset:       20,
		Params:       map[string][]string{"gender": {"male"}},
	}
	assert.Equal(t, "Patient", sp.ResourceType)
	assert.Equal(t, id, sp.TenantID)
	assert.Equal(t, 10, sp.Count)
	assert.Equal(t, 20, sp.Offset)
	assert.Equal(t, "male", sp.Params["gender"][0])
}
