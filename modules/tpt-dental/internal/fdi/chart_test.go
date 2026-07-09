package fdi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTooth(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantQ   Quadrant
		wantN   int
		wantRaw string
		wantErr bool
	}{
		{"valid permanent upper right", "18", 1, 8, "18", false},
		{"valid permanent upper left", "21", 2, 1, "21", false},
		{"valid permanent lower left", "36", 3, 6, "36", false},
		{"valid permanent lower right", "41", 4, 1, "41", false},
		{"valid deciduous upper right", "55", 5, 5, "55", false},
		{"valid deciduous lower right", "81", 8, 1, "81", false},
		{"trims whitespace", " 18 ", 1, 8, "18", false},
		{"empty string", "", 0, 0, "", true},
		{"single char", "1", 0, 0, "", true},
		{"three chars", "111", 0, 0, "", true},
		{"invalid quadrant 9", "91", 0, 0, "", true},
		{"invalid tooth number 0", "10", 0, 0, "", true},
		{"invalid tooth number 9", "19", 0, 0, "", true},
		{"deciduous tooth 6 does not exist", "56", 0, 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseTooth(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantQ, tc.Quadrant)
			assert.Equal(t, tt.wantN, tc.Number)
			assert.Equal(t, tt.wantRaw, tc.Raw)
		})
	}
}

func TestMustParseTooth(t *testing.T) {
	tc := MustParseTooth("18")
	assert.Equal(t, Quadrant(1), tc.Quadrant)
	assert.Equal(t, 8, tc.Number)

	assert.Panics(t, func() { MustParseTooth("invalid") })
}

func TestLookupTooth(t *testing.T) {
	tooth, err := LookupTooth("11")
	require.NoError(t, err)
	assert.Equal(t, "Upper Right Central Incisor", tooth.Name)
	assert.Equal(t, "UR1", tooth.Abbreviation)
	assert.Equal(t, 8, tooth.UniversalNum)
	assert.Equal(t, DentitionPermanent, tooth.Dentition)
	assert.False(t, tooth.IsPrimary)

	tooth, err = LookupTooth("51")
	require.NoError(t, err)
	assert.Equal(t, DentitionDeciduous, tooth.Dentition)
	assert.True(t, tooth.IsPrimary)
	assert.Equal(t, "UR-A", tooth.Abbreviation)

	_, err = LookupTooth("XX")
	assert.Error(t, err)
}

func TestQuadrantMethods(t *testing.T) {
	tests := []struct {
		q         Quadrant
		perm      bool
		decid     bool
		arch      string
		side      string
	}{
		{1, true, false, "upper", "right"},
		{2, true, false, "upper", "left"},
		{3, true, false, "lower", "left"},
		{4, true, false, "lower", "right"},
		{5, false, true, "upper", "right"},
		{6, false, true, "upper", "left"},
		{7, false, true, "lower", "left"},
		{8, false, true, "lower", "right"},
		{0, false, false, "unknown", "unknown"},
		{9, false, false, "unknown", "unknown"},
	}
	for _, tt := range tests {
		t.Run(string(rune('0'+tt.q)), func(t *testing.T) {
			assert.Equal(t, tt.perm, tt.q.IsPermanent())
			assert.Equal(t, tt.decid, tt.q.IsDeciduous())
			assert.Equal(t, tt.arch, tt.q.Arch())
			assert.Equal(t, tt.side, tt.q.Side())
		})
	}
}

func TestToothClass(t *testing.T) {
	tests := []struct {
		number int
		want   string
	}{
		{1, "incisor"},
		{2, "incisor"},
		{3, "canine"},
		{4, "premolar"},
		{5, "premolar"},
		{6, "molar"},
		{7, "molar"},
		{8, "molar"},
		{9, "molar"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, ToothClass(tt.number))
		})
	}
}

func TestValidToothCode(t *testing.T) {
	assert.True(t, ValidToothCode("18"))
	assert.True(t, ValidToothCode("55"))
	assert.False(t, ValidToothCode(""))
	assert.False(t, ValidToothCode("XX"))
}

func TestAllPermanentTeeth(t *testing.T) {
	teeth := AllPermanentTeeth()
	assert.Len(t, teeth, 32)
	// First should be UR8 (quadrant 1, descending from 8)
	assert.Equal(t, "18", teeth[0])
	// All should be valid
	for _, code := range teeth {
		assert.True(t, ValidToothCode(code), "expected valid code: %s", code)
	}
}

func TestAllDeciduousTeeth(t *testing.T) {
	teeth := AllDeciduousTeeth()
	assert.Len(t, teeth, 20)
	for _, code := range teeth {
		assert.True(t, ValidToothCode(code), "expected valid code: %s", code)
	}
}
