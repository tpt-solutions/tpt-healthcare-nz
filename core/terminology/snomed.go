// Package terminology provides loaders and in-memory stores for clinical
// terminology standards used in NZ healthcare: SNOMED CT NZ Edition, LOINC,
// ICD-10-AM, and the NZ Medicines Terminology (NZMT / NZULM).
package terminology

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// NZSNOMEDEditionOID is the SNOMED CT NZ Edition OID published by NZHIS.
const NZSNOMEDEditionOID = "32506021000036107"

// SNOMEDConcept holds the core attributes of a single SNOMED CT concept.
type SNOMEDConcept struct {
	// ID is the SNOMED CT identifier (SCTID), e.g. "22298006".
	ID string
	// FSN is the Fully Specified Name including semantic tag, e.g.
	// "Myocardial infarction (disorder)".
	FSN string
	// PreferredTerm is the NZ preferred term for this concept.
	PreferredTerm string
	// Active indicates whether the concept is currently active.
	Active bool
	// ParentIDs holds the SCTIDs of immediate IS-A parents.
	ParentIDs []string
}

// SNOMEDStore is an in-memory store of SNOMED CT concepts keyed by SCTID.
type SNOMEDStore struct {
	concepts map[string]*SNOMEDConcept
}

// LoadSNOMEDRF2 loads SNOMED CT concepts and descriptions from RF2 TSV files
// as distributed in the NZ SNOMED CT Edition release.
//
// conceptFile should be the RF2 Concept file (sct2_Concept_*.txt).
// descFile should be the RF2 Description file (sct2_Description_*.txt).
//
// Only active concepts (active=1 in the concept file) are loaded.
// The Description file is used to populate FSN and PreferredTerm fields.
func LoadSNOMEDRF2(conceptFile, descFile string) (*SNOMEDStore, error) {
	store := &SNOMEDStore{
		concepts: make(map[string]*SNOMEDConcept),
	}

	// Pass 1: load concepts.
	if err := store.loadConcepts(conceptFile); err != nil {
		return nil, fmt.Errorf("snomed: load concepts: %w", err)
	}

	// Pass 2: load descriptions.
	if err := store.loadDescriptions(descFile); err != nil {
		return nil, fmt.Errorf("snomed: load descriptions: %w", err)
	}

	return store, nil
}

// loadConcepts reads the RF2 Concept TSV file and populates the store.
// RF2 Concept file columns (tab-separated):
//   id  effectiveTime  active  moduleId  definitionStatusId
func (s *SNOMEDStore) loadConcepts(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	// Skip header row.
	if _, err := r.ReadString('\n'); err != nil && err != io.EOF {
		return fmt.Errorf("read header: %w", err)
	}

	for {
		line, err := r.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			cols := strings.Split(line, "\t")
			if len(cols) < 3 {
				if err == io.EOF {
					break
				}
				continue
			}
			id := cols[0]
			active := cols[2] == "1"
			if active {
				s.concepts[id] = &SNOMEDConcept{
					ID:     id,
					Active: true,
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// loadDescriptions reads the RF2 Description TSV file and updates concepts
// with FSN and PreferredTerm values.
//
// RF2 Description file columns (tab-separated):
//
//	id  effectiveTime  active  moduleId  conceptId  languageCode
//	typeId  term  caseSignificanceId
//
// typeId 900000000000003001 = FSN
// typeId 900000000000013009 = Synonym (preferred term selection is done via
// the Language refset; here we use the first active synonym as preferred).
func (s *SNOMEDStore) loadDescriptions(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	// Skip header.
	if _, err := r.ReadString('\n'); err != nil && err != io.EOF {
		return fmt.Errorf("read header: %w", err)
	}

	const (
		typeIDFSN     = "900000000000003001"
		typeIDSynonym = "900000000000013009"
	)

	for {
		line, err := r.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			cols := strings.Split(line, "\t")
			if len(cols) < 8 {
				if err == io.EOF {
					break
				}
				continue
			}
			active := cols[2] == "1"
			if !active {
				if err == io.EOF {
					break
				}
				continue
			}
			conceptID := cols[4]
			typeID := cols[6]
			term := cols[7]

			concept, exists := s.concepts[conceptID]
			if !exists {
				if err == io.EOF {
					break
				}
				continue
			}

			switch typeID {
			case typeIDFSN:
				concept.FSN = term
			case typeIDSynonym:
				if concept.PreferredTerm == "" {
					concept.PreferredTerm = term
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Lookup returns the SNOMEDConcept for the given SCTID and true, or nil and
// false if not found.
func (s *SNOMEDStore) Lookup(sctid string) (*SNOMEDConcept, bool) {
	c, ok := s.concepts[sctid]
	return c, ok
}

// Search performs a case-insensitive substring search across FSN and
// PreferredTerm fields of all concepts. Results are returned in arbitrary
// order up to limit. A limit <= 0 returns all matches.
func (s *SNOMEDStore) Search(query string, limit int) []*SNOMEDConcept {
	q := strings.ToLower(query)
	var results []*SNOMEDConcept
	for _, c := range s.concepts {
		if strings.Contains(strings.ToLower(c.FSN), q) ||
			strings.Contains(strings.ToLower(c.PreferredTerm), q) {
			results = append(results, c)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results
}

// IsA returns true if the concept identified by sctid is the same as, or a
// descendant of, parentSCTID. This performs a simple lookup via ParentIDs and
// walks up the hierarchy (breadth-first). For large hierarchies a pre-built
// transitive closure index should be used instead.
func (s *SNOMEDStore) IsA(sctid, parentSCTID string) bool {
	if sctid == parentSCTID {
		return true
	}
	visited := make(map[string]bool)
	queue := []string{sctid}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		concept, ok := s.concepts[current]
		if !ok {
			continue
		}
		for _, pid := range concept.ParentIDs {
			if pid == parentSCTID {
				return true
			}
			if !visited[pid] {
				queue = append(queue, pid)
			}
		}
	}
	return false
}

// LoadSNOMEDCSV is a convenience loader for a simple CSV export with columns:
//   id,active,fsn,preferredTerm,parentIds
//
// parentIds is pipe-separated ("|"). This format is useful for testing and
// for working with SNOMED subsets.
func LoadSNOMEDCSV(csvPath string) (*SNOMEDStore, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	store := &SNOMEDStore{concepts: make(map[string]*SNOMEDConcept)}
	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	// Skip header.
	if _, err := r.Read(); err != nil {
		return nil, err
	}

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 4 {
			continue
		}
		c := &SNOMEDConcept{
			ID:            strings.TrimSpace(rec[0]),
			Active:        strings.TrimSpace(rec[1]) == "1",
			FSN:           strings.TrimSpace(rec[2]),
			PreferredTerm: strings.TrimSpace(rec[3]),
		}
		if len(rec) >= 5 && rec[4] != "" {
			for _, pid := range strings.Split(rec[4], "|") {
				if p := strings.TrimSpace(pid); p != "" {
					c.ParentIDs = append(c.ParentIDs, p)
				}
			}
		}
		store.concepts[c.ID] = c
	}
	return store, nil
}
