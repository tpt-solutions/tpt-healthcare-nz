package fdi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSurfaceName(t *testing.T) {
	tests := []struct {
		code SurfaceCode
		want string
	}{
		{SurfaceMesial, "Mesial"},
		{SurfaceDistal, "Distal"},
		{SurfaceOcclusal, "Occlusal"},
		{SurfaceIncisal, "Incisal"},
		{SurfaceBuccal, "Buccal"},
		{SurfaceLingual, "Lingual"},
		{SurfacePalatal, "Palatal"},
		{SurfaceLabial, "Labial"},
		{SurfaceVestibular, "Vestibular"},
		{SurfaceCervical, "Cervical"},
		{SurfaceCode("X"), "Unknown(X)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, SurfaceName(tt.code))
		})
	}
}

func TestParseSurfaceCombination(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []SurfaceCode
		wantErr bool
	}{
		{"single M", "M", []SurfaceCode{SurfaceMesial}, false},
		{"MOD sorted", "MOD", []SurfaceCode{SurfaceMesial, SurfaceOcclusal, SurfaceDistal}, false},
		{"two-letter La", "La", []SurfaceCode{SurfaceLabial}, false},
		{"La mixed with single", "MLaD", []SurfaceCode{SurfaceMesial, SurfaceLabial, SurfaceDistal}, false},
		{"empty string", "", nil, true},
		{"whitespace only", "  ", nil, true},
		{"unknown code X", "X", nil, true},
		{"unknown in combo", "MXD", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSurfaceCombination(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatSurfaceCombination(t *testing.T) {
	tests := []struct {
		name  string
		codes []SurfaceCode
		want  string
	}{
		{"nil", nil, ""},
		{"empty", []SurfaceCode{}, ""},
		{"single", []SurfaceCode{SurfaceMesial}, "M"},
		{"MOD unsorted input", []SurfaceCode{SurfaceOcclusal, SurfaceMesial, SurfaceDistal}, "MOD"},
		{"with Labial", []SurfaceCode{SurfaceDistal, SurfaceLabial, SurfaceMesial}, "MLaD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatSurfaceCombination(tt.codes))
		})
	}
}

func TestIsAnterior(t *testing.T) {
	assert.True(t, IsAnterior("incisor"))
	assert.True(t, IsAnterior("canine"))
	assert.False(t, IsAnterior("premolar"))
	assert.False(t, IsAnterior("molar"))
	assert.False(t, IsAnterior(""))
}

func TestSurfaceApplicable(t *testing.T) {
	tests := []struct {
		surface    SurfaceCode
		toothClass string
		want       bool
	}{
		{SurfaceOcclusal, "molar", true},
		{SurfaceOcclusal, "premolar", true},
		{SurfaceOcclusal, "incisor", false},
		{SurfaceOcclusal, "canine", false},
		{SurfaceIncisal, "incisor", true},
		{SurfaceIncisal, "canine", true},
		{SurfaceIncisal, "molar", false},
		{SurfaceIncisal, "premolar", false},
		{SurfaceBuccal, "molar", true},
		{SurfaceBuccal, "incisor", true},
		{SurfaceLingual, "molar", true},
		{SurfaceLingual, "incisor", true},
		{SurfaceMesial, "molar", true},
		{SurfaceDistal, "incisor", true},
		{SurfacePalatal, "molar", false},
		{SurfaceVestibular, "molar", false},
		{SurfaceCervical, "molar", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.surface)+"_"+tt.toothClass, func(t *testing.T) {
			assert.Equal(t, tt.want, SurfaceApplicable(tt.surface, tt.toothClass))
		})
	}
}
