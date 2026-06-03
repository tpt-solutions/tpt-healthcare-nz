package terminology

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// ICD10Code represents a single ICD-10-AM code entry.
// ICD-10-AM is the Australian Modification of ICD-10 used in NZ for morbidity
// and mortality coding.
type ICD10Code struct {
	// Code is the ICD-10-AM code, e.g. "I21.0".
	Code string
	// Description is the human-readable description.
	Description string
	// Category is the 3-character category code, e.g. "I21".
	Category string
	// Chapter is the chapter name derived from the top-level code block.
	Chapter string
	// Valid indicates whether this code is valid for use in NZ coding.
	Valid bool
}

// ICD10Store is an in-memory store of ICD-10-AM codes keyed by code string.
type ICD10Store struct {
	codes map[string]*ICD10Code
}

// icd10Chapters maps the first character(s) of an ICD-10 code to a chapter
// name per the ICD-10 structure used in the Australian Modification.
var icd10Chapters = []struct {
	prefix  string
	chapter string
}{
	{"A", "Certain infectious and parasitic diseases"},
	{"B", "Certain infectious and parasitic diseases"},
	{"C", "Neoplasms"},
	{"D0", "Neoplasms"},
	{"D1", "Neoplasms"},
	{"D2", "Neoplasms"},
	{"D3", "Neoplasms"},
	{"D4", "Neoplasms"},
	{"D5", "Diseases of the blood and blood-forming organs"},
	{"D6", "Diseases of the blood and blood-forming organs"},
	{"D7", "Diseases of the blood and blood-forming organs"},
	{"D8", "Diseases of the blood and blood-forming organs"},
	{"E", "Endocrine, nutritional and metabolic diseases"},
	{"F", "Mental and behavioural disorders"},
	{"G", "Diseases of the nervous system"},
	{"H0", "Diseases of the eye and adnexa"},
	{"H1", "Diseases of the eye and adnexa"},
	{"H2", "Diseases of the eye and adnexa"},
	{"H3", "Diseases of the eye and adnexa"},
	{"H4", "Diseases of the eye and adnexa"},
	{"H5", "Diseases of the eye and adnexa"},
	{"H6", "Diseases of the ear and mastoid process"},
	{"H7", "Diseases of the ear and mastoid process"},
	{"H8", "Diseases of the ear and mastoid process"},
	{"H9", "Diseases of the ear and mastoid process"},
	{"I", "Diseases of the circulatory system"},
	{"J", "Diseases of the respiratory system"},
	{"K", "Diseases of the digestive system"},
	{"L", "Diseases of the skin and subcutaneous tissue"},
	{"M", "Diseases of the musculoskeletal system and connective tissue"},
	{"N", "Diseases of the genitourinary system"},
	{"O", "Pregnancy, childbirth and the puerperium"},
	{"P", "Certain conditions originating in the perinatal period"},
	{"Q", "Congenital malformations, deformations and chromosomal abnormalities"},
	{"R", "Symptoms, signs and abnormal clinical and laboratory findings"},
	{"S", "Injury, poisoning and certain other consequences of external causes"},
	{"T", "Injury, poisoning and certain other consequences of external causes"},
	{"V", "External causes of morbidity and mortality"},
	{"W", "External causes of morbidity and mortality"},
	{"X", "External causes of morbidity and mortality"},
	{"Y", "External causes of morbidity and mortality"},
	{"Z", "Factors influencing health status and contact with health services"},
	{"U", "Codes for special purposes"},
}

// LoadICD10AM loads ICD-10-AM codes from a simple code:description CSV/text
// file. Each non-blank, non-comment line must be in the format:
//
//	CODE,Description
//
// or the legacy colon-delimited format:
//
//	CODE:Description
//
// Lines beginning with "#" are treated as comments and skipped.
// This is the format used for simple ICD-10-AM subset exports and testing.
// For full AM releases, provide the official ACCS/IHPA AM distribution.
func LoadICD10AM(xmlOrCSVPath string) (*ICD10Store, error) {
	f, err := os.Open(xmlOrCSVPath)
	if err != nil {
		return nil, fmt.Errorf("icd10am: open %s: %w", xmlOrCSVPath, err)
	}
	defer f.Close()

	store := &ICD10Store{codes: make(map[string]*ICD10Code)}

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Skip CSV header row.
		if lineNo == 1 && (strings.HasPrefix(strings.ToUpper(line), "CODE") ||
			strings.HasPrefix(strings.ToUpper(line), "ICD")) {
			continue
		}

		var code, description string
		if idx := strings.IndexByte(line, ','); idx >= 0 {
			code = strings.TrimSpace(line[:idx])
			description = strings.TrimSpace(line[idx+1:])
		} else if idx := strings.IndexByte(line, ':'); idx >= 0 {
			code = strings.TrimSpace(line[:idx])
			description = strings.TrimSpace(line[idx+1:])
		} else {
			continue
		}

		if code == "" {
			continue
		}

		// Derive category (first 3 chars of code, ignoring dot).
		stripped := strings.ReplaceAll(code, ".", "")
		category := code
		if len(stripped) >= 3 {
			// Preserve dot notation for category, e.g. "I21" from "I21.0".
			if dotIdx := strings.IndexByte(code, '.'); dotIdx > 0 {
				category = code[:dotIdx]
			} else if len(code) > 3 {
				category = code[:3]
			}
		}

		entry := &ICD10Code{
			Code:        code,
			Description: description,
			Category:    category,
			Chapter:     chapterForCode(code),
			Valid:        true,
		}
		store.codes[code] = entry
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("icd10am: scan: %w", err)
	}

	return store, nil
}

// Lookup returns the ICD10Code for the given code string and true, or nil and
// false if not found. Lookup is case-sensitive to match AM convention.
func (s *ICD10Store) Lookup(code string) (*ICD10Code, bool) {
	c, ok := s.codes[strings.TrimSpace(code)]
	return c, ok
}

// Search performs a case-insensitive substring search across Code and
// Description fields. Results are returned in arbitrary order up to limit.
// A limit <= 0 returns all matches.
func (s *ICD10Store) Search(query string, limit int) []*ICD10Code {
	q := strings.ToLower(query)
	var results []*ICD10Code
	for _, c := range s.codes {
		if strings.Contains(strings.ToLower(c.Code), q) ||
			strings.Contains(strings.ToLower(c.Description), q) {
			results = append(results, c)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results
}

// Chapter returns the ICD-10 chapter name for the given code.
// Returns an empty string if the code prefix is not recognised.
func (s *ICD10Store) Chapter(code string) string {
	return chapterForCode(code)
}

// chapterForCode derives the chapter name from the ICD-10-AM code by matching
// the longest prefix against the icd10Chapters table.
func chapterForCode(code string) string {
	if len(code) == 0 {
		return ""
	}
	best := ""
	bestLen := 0
	upper := strings.ToUpper(code)
	for _, entry := range icd10Chapters {
		pfx := strings.ToUpper(entry.prefix)
		if strings.HasPrefix(upper, pfx) && len(pfx) > bestLen {
			best = entry.chapter
			bestLen = len(pfx)
		}
	}
	return best
}
