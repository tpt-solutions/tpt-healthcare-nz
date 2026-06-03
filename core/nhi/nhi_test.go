package nhi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateNHI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid old-format NHIs (3 letters + 4 digits, checksum verified).
		{name: "valid old format ZAC5361", input: "ZAC5361", want: true},
		{name: "valid old format ABC1235", input: "ABC1235", want: true},
		{name: "valid old format AAA0004", input: "AAA0004", want: true},
		// Invalid: too short.
		{name: "too short AB1234", input: "AB1234", want: false},
		// Invalid: digits before letters.
		{name: "wrong format 123ABCD", input: "123ABCD", want: false},
		// Invalid: empty string.
		{name: "empty string", input: "", want: false},
		// Invalid: all whitespace normalises to empty.
		{name: "only whitespace", input: "   ", want: false},
		// Invalid: too long.
		{name: "too long ABCD12345", input: "ABCD12345", want: false},
		// Invalid: old format containing I — letterValues skips I.
		{name: "old format with letter I at pos 0", input: "IAB1234", want: false},
		// Invalid: old format containing O — letterValues skips O.
		{name: "old format with letter O at pos 0", input: "OAB1234", want: false},
		// New format: Z + 2 letters + 2 digits + 2 letters — structure-only check, always valid.
		{name: "new format ZAB01CD", input: "ZAB01CD", want: true},
		{name: "new format lowercase zab01cd", input: "zab01cd", want: true},
		// New format with I and O is permitted (no checksum restriction on new format).
		{name: "new format ZIB01CD", input: "ZIB01CD", want: true},
		{name: "new format ZOB01CD", input: "ZOB01CD", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateNHI(tt.input)
			assert.Equal(t, tt.want, got, "ValidateNHI(%q)", tt.input)
		})
	}
}

func TestNewFormatNHI(t *testing.T) {
	// New-format NHIs (Z + 2 letters + 2 digits + 2 letters) pass structure-only
	// validation since the real checksum is enforced server-side by Te Whatu Ora.
	cases := []struct {
		input string
		valid bool
	}{
		// Boundary values for new format.
		{"ZAA00AA", true},
		{"ZZZ99ZZ", true},
		{"ZAB12CD", true},
		// One character short — falls back to old-format check and fails checksum.
		{"ZAB12C", false},
		// One character too many.
		{"ZAB12CDE", false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.valid, ValidateNHI(tc.input),
				"ValidateNHI(%q) new-format structural check", tc.input)
		})
	}
}
