package terminology

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// LOINCCode represents a single LOINC code entry from the Loinc.csv file.
type LOINCCode struct {
	// LOINC is the LOINC code, e.g. "2345-7".
	LOINC string
	// Component is the LOINC Part 1 – the analyte, e.g. "Glucose".
	Component string
	// Property is the LOINC Part 2 – kind of property, e.g. "MCnc".
	Property string
	// TimeAspect is the LOINC Part 3 – timing, e.g. "Pt" (point in time).
	TimeAspect string
	// System is the LOINC Part 4 – system/specimen, e.g. "Ser/Plas".
	System string
	// Scale is the LOINC Part 5 – type of scale, e.g. "Qn".
	Scale string
	// Method is the LOINC Part 6 – method type (may be empty).
	Method string
	// LongCommonName is the human-readable long name, e.g.
	// "Glucose [Mass/volume] in Serum or Plasma".
	LongCommonName string
	// ClassType is the LOINC class, e.g. "CHEM", "MICRO".
	ClassType string
}

// LOINCStore is an in-memory store of LOINC codes keyed by LOINC code string.
type LOINCStore struct {
	codes map[string]*LOINCCode
}

// loincColumnIndex stores the resolved column indices from the Loinc.csv header.
type loincColumnIndex struct {
	loinc          int
	component      int
	property       int
	timeAspect     int
	system         int
	scale          int
	method         int
	longCommonName int
	classType      int
}

// requiredLOINCColumns maps the canonical Loinc.csv header name to the field
// it populates.
var requiredLOINCColumns = []string{
	"LOINC_NUM",
	"COMPONENT",
	"PROPERTY",
	"TIME_ASPCT",
	"SYSTEM",
	"SCALE_TYP",
	"METHOD_TYP",
	"LONG_COMMON_NAME",
	"CLASSTYPE",
}

// LoadLOINC reads a Loinc.csv file (as distributed by the Regenstrief
// Institute) and returns an in-memory LOINCStore.
//
// The Loinc.csv format uses a comma-separated header row; this function is
// tolerant of missing optional columns.
func LoadLOINC(csvPath string) (*LOINCStore, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("loinc: open %s: %w", csvPath, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	// Read header and resolve column indices.
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("loinc: read header: %w", err)
	}

	idx := loincColumnIndex{
		loinc:          -1,
		component:      -1,
		property:       -1,
		timeAspect:     -1,
		system:         -1,
		scale:          -1,
		method:         -1,
		longCommonName: -1,
		classType:      -1,
	}

	for i, col := range header {
		switch strings.ToUpper(strings.TrimSpace(col)) {
		case "LOINC_NUM":
			idx.loinc = i
		case "COMPONENT":
			idx.component = i
		case "PROPERTY":
			idx.property = i
		case "TIME_ASPCT":
			idx.timeAspect = i
		case "SYSTEM":
			idx.system = i
		case "SCALE_TYP":
			idx.scale = i
		case "METHOD_TYP":
			idx.method = i
		case "LONG_COMMON_NAME":
			idx.longCommonName = i
		case "CLASSTYPE":
			idx.classType = i
		}
	}

	if idx.loinc == -1 {
		return nil, fmt.Errorf("loinc: required column LOINC_NUM not found in header")
	}

	store := &LOINCStore{codes: make(map[string]*LOINCCode)}

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("loinc: read row: %w", err)
		}

		getCol := func(i int) string {
			if i < 0 || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}

		code := getCol(idx.loinc)
		if code == "" {
			continue
		}

		store.codes[code] = &LOINCCode{
			LOINC:          code,
			Component:      getCol(idx.component),
			Property:       getCol(idx.property),
			TimeAspect:     getCol(idx.timeAspect),
			System:         getCol(idx.system),
			Scale:          getCol(idx.scale),
			Method:         getCol(idx.method),
			LongCommonName: getCol(idx.longCommonName),
			ClassType:      getCol(idx.classType),
		}
	}

	return store, nil
}

// Lookup returns the LOINCCode for the given LOINC code string and true, or
// nil and false if not found.
func (s *LOINCStore) Lookup(loinc string) (*LOINCCode, bool) {
	c, ok := s.codes[strings.TrimSpace(loinc)]
	return c, ok
}

// Search performs a case-insensitive substring search across LongCommonName
// and Component fields. Results are returned in arbitrary order up to limit.
// A limit <= 0 returns all matches.
func (s *LOINCStore) Search(query string, limit int) []*LOINCCode {
	q := strings.ToLower(query)
	var results []*LOINCCode
	for _, c := range s.codes {
		if strings.Contains(strings.ToLower(c.LongCommonName), q) ||
			strings.Contains(strings.ToLower(c.Component), q) {
			results = append(results, c)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results
}
