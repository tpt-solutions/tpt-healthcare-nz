package procedure

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDCNZCodes(t *testing.T) {
	codes := DCNZCodes()
	assert.Greater(t, len(codes), 0)
	for _, c := range codes {
		assert.NotEmpty(t, c.Code)
		assert.Equal(t, CodeSystem("dcnz"), c.System)
		assert.NotEmpty(t, c.Category)
	}
}

func TestACCDentalCodes(t *testing.T) {
	codes := ACCDentalCodes()
	assert.Greater(t, len(codes), 0)
	for _, c := range codes {
		assert.NotEmpty(t, c.Code)
		assert.NotEmpty(t, c.Description)
	}
}

func TestLookupDCNZ(t *testing.T) {
	c, ok := LookupDCNZ("011")
	assert.True(t, ok)
	assert.Equal(t, "Comprehensive Exam", c.ShortName)
	assert.Equal(t, CategoryExamination, c.Category)

	_, ok = LookupDCNZ("INVALID")
	assert.False(t, ok)

	_, ok = LookupDCNZ("")
	assert.False(t, ok)
}

func TestLookupACCDental(t *testing.T) {
	c, ok := LookupACCDental("A1")
	assert.True(t, ok)
	assert.Contains(t, c.Description, "Examination")

	_, ok = LookupACCDental("INVALID")
	assert.False(t, ok)
}

func TestProceduresByCategory(t *testing.T) {
	exams := ProceduresByCategory(CategoryExamination)
	assert.Len(t, exams, 4)
	for _, c := range exams {
		assert.Equal(t, CategoryExamination, c.Category)
	}

	empty := ProceduresByCategory("nonexistent")
	assert.Nil(t, empty)
}

func TestDCNZToJSON(t *testing.T) {
	s, err := DCNZToJSON()
	require.NoError(t, err)
	assert.Contains(t, s, "011")

	var codes []ProcedureCode
	err = json.Unmarshal([]byte(s), &codes)
	require.NoError(t, err)
	assert.Greater(t, len(codes), 0)
}
