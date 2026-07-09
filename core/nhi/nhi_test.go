package nhi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMatchParameters(t *testing.T) {
	t.Run("empty params", func(t *testing.T) {
		result := buildMatchParameters(MatchParams{})
		assert.NotNil(t, result)
		assert.Equal(t, "Parameters", result["resourceType"])
	})

	t.Run("with name", func(t *testing.T) {
		result := buildMatchParameters(MatchParams{
			GivenName:  "John",
			FamilyName: "Smith",
		})
		assert.NotNil(t, result)
	})

	t.Run("with gender", func(t *testing.T) {
		result := buildMatchParameters(MatchParams{Gender: "male"})
		assert.NotNil(t, result)
	})

	t.Run("with birthdate", func(t *testing.T) {
		result := buildMatchParameters(MatchParams{
			BirthDate: time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC),
		})
		assert.NotNil(t, result)
	})

	t.Run("with address", func(t *testing.T) {
		result := buildMatchParameters(MatchParams{
			Address: "123 Test St, Auckland",
		})
		assert.NotNil(t, result)
	})
}

func TestValidateNHI_ExtendedCases(t *testing.T) {
	tests := []struct {
		name  string
		nhi   string
		valid bool
	}{
		{"valid old format", "ZAC1234", true},
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"too short", "ZAC12", false},
		{"too long", "ZAC12345", false},
		{"lowercase old", "zab1230", false},
		{"contains I", "ZAI1234", false},
		{"contains O", "ZAO1234", false},
		{"valid new format", "ZAA00AA", true},
		{"new format lowercase passes via ToUpper", "zaa00aa", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, ValidateNHI(tt.nhi))
		})
	}
}

func TestMatchParams_Construction(t *testing.T) {
	mp := MatchParams{
		GivenName:  "John",
		FamilyName: "Smith",
		Gender:     "male",
		BirthDate:  time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC),
		Address:    "123 Test St, Auckland",
	}
	assert.Equal(t, "John", mp.GivenName)
	assert.Equal(t, "Smith", mp.FamilyName)
	assert.Equal(t, "male", mp.Gender)
}

func TestNHI_Client_New(t *testing.T) {
	client := New("http://test.com/", func(_ context.Context) (string, error) { return "tok", nil })
	require.NotNil(t, client)
}
