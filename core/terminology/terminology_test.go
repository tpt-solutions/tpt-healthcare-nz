package terminology

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSNOMEDCSV(t *testing.T) {
	csvContent := "id,active,fsn,preferredTerm,parentIds\n" +
		"22298006,1,Myocardial infarction (disorder),Heart attack,64572001\n" +
		"64572001,1,Disease (disorder),Disease,\n" +
		"12345678,0,Retired concept (disorder),Retired,64572001\n"
	path := writeTempFile(t, "snomed.csv", csvContent)
	store, err := LoadSNOMEDCSV(path)
	require.NoError(t, err)
	require.NotNil(t, store)

	t.Run("lookup active concept", func(t *testing.T) {
		c, ok := store.Lookup("22298006")
		require.True(t, ok)
		assert.Equal(t, "Myocardial infarction (disorder)", c.FSN)
		assert.Equal(t, "Heart attack", c.PreferredTerm)
		assert.True(t, c.Active)
	})

	t.Run("lookup parent concept", func(t *testing.T) {
		c, ok := store.Lookup("64572001")
		require.True(t, ok)
		assert.Equal(t, "Disease (disorder)", c.FSN)
	})

	t.Run("inactive concept stored with Active=false", func(t *testing.T) {
		c, ok := store.Lookup("12345678")
		require.True(t, ok)
		assert.False(t, c.Active)
	})

	t.Run("lookup not found", func(t *testing.T) {
		_, ok := store.Lookup("99999999")
		assert.False(t, ok)
	})
}

func TestSNOMEDStore_Search(t *testing.T) {
	csvContent := "id,active,fsn,preferredTerm,parentIds\n" +
		"22298006,1,Myocardial infarction (disorder),Heart attack,64572001\n" +
		"195967002,1,Hypertensive disorder (disorder),High blood pressure,64572001\n" +
		"38341003,1,Hypertension (disorder),Hypertension,64572001\n"
	path := writeTempFile(t, "snomed.csv", csvContent)
	store, err := LoadSNOMEDCSV(path)
	require.NoError(t, err)

	t.Run("search by fsn", func(t *testing.T) {
		results := store.Search("infarction", 0)
		assert.Len(t, results, 1)
		assert.Equal(t, "22298006", results[0].ID)
	})

	t.Run("search case-insensitive", func(t *testing.T) {
		results := store.Search("HYPERTENSION", 0)
		// "Hypertension (disorder)" FSN matches, "Hypertension" PreferredTerm matches same concept
		// "Hypertensive disorder" does NOT contain "hypertension" substring
		assert.Len(t, results, 1)
	})

	t.Run("search with limit", func(t *testing.T) {
		results := store.Search("disorder", 1)
		assert.Len(t, results, 1)
	})

	t.Run("search no matches", func(t *testing.T) {
		results := store.Search("xyz123", 0)
		assert.Empty(t, results)
	})
}

func TestSNOMEDStore_IsA(t *testing.T) {
	csvContent := "id,active,fsn,preferredTerm,parentIds\n" +
		"A,1,Level A (disorder),A,\n" +
		"B,1,Level B (disorder),B,A\n" +
		"C,1,Level C (disorder),C,B\n"
	path := writeTempFile(t, "snomed.csv", csvContent)
	store, err := LoadSNOMEDCSV(path)
	require.NoError(t, err)

	t.Run("self identity", func(t *testing.T) {
		assert.True(t, store.IsA("A", "A"))
	})

	t.Run("direct parent", func(t *testing.T) {
		assert.True(t, store.IsA("B", "A"))
	})

	t.Run("grandparent", func(t *testing.T) {
		assert.True(t, store.IsA("C", "A"))
	})

	t.Run("reverse not ancestor", func(t *testing.T) {
		assert.False(t, store.IsA("A", "C"))
	})

	t.Run("missing concept", func(t *testing.T) {
		assert.False(t, store.IsA("X", "A"))
	})
}

func TestLoadLOINC(t *testing.T) {
	csvContent := "LOINC_NUM,COMPONENT,PROPERTY,TIME_ASPCT,SYSTEM,SCALE_TYP,METHOD_TYP,LONG_COMMON_NAME,CLASSTYPE\n" +
		"2160-0,Creatinine,MCnc,Pt,Ser/Plas,Qn,,Creatinine [Mass/volume] in Serum or Plasma,1\n" +
		"2345-7,Glucose,MCnc,Pt,BldVn,Qn,,Glucose [Mass/volume] in Blood,1\n"
	path := writeTempFile(t, "loinc.csv", csvContent)
	store, err := LoadLOINC(path)
	require.NoError(t, err)
	require.NotNil(t, store)

	t.Run("lookup", func(t *testing.T) {
		c, ok := store.Lookup("2160-0")
		require.True(t, ok)
		assert.Equal(t, "Creatinine", c.Component)
		assert.Equal(t, "Ser/Plas", c.System)
	})

	t.Run("lookup not found", func(t *testing.T) {
		_, ok := store.Lookup("99999-9")
		assert.False(t, ok)
	})
}

func TestLOINCStore_Search(t *testing.T) {
	csvContent := "LOINC_NUM,COMPONENT,PROPERTY,TIME_ASPCT,SYSTEM,SCALE_TYP,METHOD_TYP,LONG_COMMON_NAME,CLASSTYPE\n" +
		"2160-0,Creatinine,MCnc,Pt,Ser/Plas,Qn,,Creatinine [Mass/volume] in Serum or Plasma,1\n" +
		"2345-7,Glucose,MCnc,Pt,BldVn,Qn,,Glucose [Mass/volume] in Blood,1\n"
	path := writeTempFile(t, "loinc.csv", csvContent)
	store, err := LoadLOINC(path)
	require.NoError(t, err)

	t.Run("search", func(t *testing.T) {
		results := store.Search("creatinine", 0)
		assert.Len(t, results, 1)
	})

	t.Run("search no matches", func(t *testing.T) {
		results := store.Search("xyz", 0)
		assert.Empty(t, results)
	})
}

func TestLoadICD10AM(t *testing.T) {
	csvContent := "I21.0,Acute transmural myocardial infarction of anterior wall\n" +
		"I21.1,Acute transmural myocardial infarction of inferior wall\n" +
		"J45.9,Asthma, unspecified\n"
	path := writeTempFile(t, "icd10.csv", csvContent)
	store, err := LoadICD10AM(path)
	require.NoError(t, err)
	require.NotNil(t, store)

	t.Run("lookup", func(t *testing.T) {
		c, ok := store.Lookup("I21.0")
		require.True(t, ok)
		assert.Contains(t, c.Description, "myocardial infarction")
		assert.Equal(t, "I21", c.Category)
	})

	t.Run("lookup not found", func(t *testing.T) {
		_, ok := store.Lookup("Z99.9")
		assert.False(t, ok)
	})
}

func TestICD10Store_Search(t *testing.T) {
	csvContent := "I21.0,Acute transmural myocardial infarction\n" +
		"J45.9,Asthma, unspecified\n"
	path := writeTempFile(t, "icd10.csv", csvContent)
	store, err := LoadICD10AM(path)
	require.NoError(t, err)

	results := store.Search("asthma", 0)
	assert.Len(t, results, 1)
}

func TestICD10Store_Chapter(t *testing.T) {
	csvContent := "I21.0,Acute transmural myocardial infarction\n" +
		"J45.9,Asthma, unspecified\n" +
		"S39.012,Injury of lower back\n"
	path := writeTempFile(t, "icd10.csv", csvContent)
	store, err := LoadICD10AM(path)
	require.NoError(t, err)

	ch := store.Chapter("I21.0")
	assert.Contains(t, ch, "circulatory")

	chJ := store.Chapter("J45.9")
	assert.Contains(t, chJ, "respiratory")
}

func TestLoadNZMT(t *testing.T) {
	csvContent := "NZULM,BrandName,GenericName,DoseForm,Strength,RouteOfAdmin\n" +
		"WARF001,Warfarin Tabs,Warfarin,Tablet,5mg,Oral\n" +
		"ASP001,Aspro Clear,Aspirin,Tablet,300mg,Oral\n"
	path := writeTempFile(t, "nzmt.csv", csvContent)
	store, err := LoadNZMT(path)
	require.NoError(t, err)
	require.NotNil(t, store)

	t.Run("lookup", func(t *testing.T) {
		p, ok := store.Lookup("WARF001")
		require.True(t, ok)
		assert.Equal(t, "Warfarin Tabs", p.BrandName)
		assert.Equal(t, "Warfarin", p.GenericName)
	})

	t.Run("lookup not found", func(t *testing.T) {
		_, ok := store.Lookup("XXXXXX")
		assert.False(t, ok)
	})
}

func TestNZMTStore_Search(t *testing.T) {
	csvContent := "NZULM,BrandName,GenericName,DoseForm,Strength,RouteOfAdmin\n" +
		"WARF001,Warfarin Tabs,Warfarin,Tablet,5mg,Oral\n" +
		"ASP001,Aspro Clear,Aspirin,Tablet,300mg,Oral\n"
	path := writeTempFile(t, "nzmt.csv", csvContent)
	store, err := LoadNZMT(path)
	require.NoError(t, err)

	t.Run("search by brand", func(t *testing.T) {
		results := store.Search("Aspro", 0)
		assert.Len(t, results, 1)
	})

	t.Run("search by generic", func(t *testing.T) {
		results := store.Search("warfarin", 0)
		assert.Len(t, results, 1)
	})
}

func TestNZMTStore_ByGenericName(t *testing.T) {
	csvContent := "NZULM,BrandName,GenericName,DoseForm,Strength,RouteOfAdmin\n" +
		"WARF001,Warfarin Tabs,Warfarin,Tablet,5mg,Oral\n" +
		"WARF002,Marevan,Warfarin,Tablet,3mg,Oral\n" +
		"ASP001,Aspro Clear,Aspirin,Tablet,300mg,Oral\n"
	path := writeTempFile(t, "nzmt.csv", csvContent)
	store, err := LoadNZMT(path)
	require.NoError(t, err)

	results := store.ByGenericName("Warfarin")
	assert.Len(t, results, 2)
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}
