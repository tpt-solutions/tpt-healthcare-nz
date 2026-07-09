package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/PhillipC05/tpt-healthcare/core/terminology"
)

const (
	defaultTermLimit = 20
	maxTermLimit     = 100
)

// TerminologyHandler serves SNOMED CT, LOINC, ICD-10-AM and NZMT/NZULM
// terminology lookup and search endpoints.
//
// The backing store interfaces are intentionally simple stubs. Wire in a real
// terminology service (e.g. NCTS/Ontoserver) by implementing TermStore and
// passing it to newTerminologyHandler.
type TerminologyHandler struct {
	store TermStore
}

// TermStore is the interface a terminology back-end must satisfy.
type TermStore interface {
	// SearchSNOMED returns SNOMED CT concepts matching query, up to limit results.
	SearchSNOMED(query string, limit int) ([]TermConcept, error)
	// GetSNOMED returns the SNOMED CT concept for the given SCTID.
	GetSNOMED(sctid string) (*TermConcept, error)
	// SearchLOINC returns LOINC terms matching query, up to limit results.
	SearchLOINC(query string, limit int) ([]TermConcept, error)
	// GetLOINC returns the LOINC term for the given code.
	GetLOINC(code string) (*TermConcept, error)
	// SearchICD10AM returns ICD-10-AM entries matching query, up to limit results.
	SearchICD10AM(query string, limit int) ([]TermConcept, error)
	// SearchNZMT returns NZMT/NZULM entries matching query, up to limit results.
	SearchNZMT(query string, limit int) ([]TermConcept, error)
}

// TermConcept is a generic code/display pair for any terminology system.
type TermConcept struct {
	// System is the canonical URI for the code system (e.g. http://snomed.info/sct).
	System string `json:"system"`
	// Code is the concept/term code.
	Code string `json:"code"`
	// Display is the preferred display label.
	Display string `json:"display"`
	// Definition is an optional definition or description.
	Definition string `json:"definition,omitempty"`
}

// stubTermStore is a no-op TermStore used when no real store is wired in.
type stubTermStore struct{}

func (stubTermStore) SearchSNOMED(query string, limit int) ([]TermConcept, error) {
	return []TermConcept{}, nil
}
func (stubTermStore) GetSNOMED(sctid string) (*TermConcept, error) {
	return nil, fmt.Errorf("SNOMED CT concept %q not found", sctid)
}
func (stubTermStore) SearchLOINC(query string, limit int) ([]TermConcept, error) {
	return []TermConcept{}, nil
}
func (stubTermStore) GetLOINC(code string) (*TermConcept, error) {
	return nil, fmt.Errorf("LOINC code %q not found", code)
}
func (stubTermStore) SearchICD10AM(query string, limit int) ([]TermConcept, error) {
	return []TermConcept{}, nil
}
func (stubTermStore) SearchNZMT(query string, limit int) ([]TermConcept, error) {
	return []TermConcept{}, nil
}

// coreTermStore adapts the in-memory terminology stores in core/terminology
// (loaded from SNOMED CT RF2, LOINC, ICD-10-AM and NZMT/NZULM source files)
// to the TermStore interface. Any of the four backing stores may be nil, in
// which case lookups/searches against that code system return "not found" /
// empty results rather than erroring — this lets a deployment enable only
// the terminology systems it has data files for.
type coreTermStore struct {
	snomed *terminology.SNOMEDStore
	loinc  *terminology.LOINCStore
	icd10  *terminology.ICD10Store
	nzmt   *terminology.NZMTStore
}

// newCoreTermStore returns a TermStore backed by the given terminology
// stores. Pass nil for any store whose source data has not been loaded.
func newCoreTermStore(snomed *terminology.SNOMEDStore, loinc *terminology.LOINCStore, icd10 *terminology.ICD10Store, nzmt *terminology.NZMTStore) TermStore {
	return &coreTermStore{snomed: snomed, loinc: loinc, icd10: icd10, nzmt: nzmt}
}

func (c *coreTermStore) SearchSNOMED(query string, limit int) ([]TermConcept, error) {
	if c.snomed == nil {
		return []TermConcept{}, nil
	}
	results := c.snomed.Search(query, limit)
	out := make([]TermConcept, 0, len(results))
	for _, r := range results {
		out = append(out, TermConcept{Code: r.ID, Display: snomedDisplay(r), Definition: r.FSN})
	}
	return out, nil
}

func (c *coreTermStore) GetSNOMED(sctid string) (*TermConcept, error) {
	if c.snomed == nil {
		return nil, fmt.Errorf("SNOMED CT concept %q not found", sctid)
	}
	r, ok := c.snomed.Lookup(sctid)
	if !ok {
		return nil, fmt.Errorf("SNOMED CT concept %q not found", sctid)
	}
	return &TermConcept{Code: r.ID, Display: snomedDisplay(r), Definition: r.FSN}, nil
}

func (c *coreTermStore) SearchLOINC(query string, limit int) ([]TermConcept, error) {
	if c.loinc == nil {
		return []TermConcept{}, nil
	}
	results := c.loinc.Search(query, limit)
	out := make([]TermConcept, 0, len(results))
	for _, r := range results {
		out = append(out, TermConcept{Code: r.LOINC, Display: r.LongCommonName, Definition: r.Component})
	}
	return out, nil
}

func (c *coreTermStore) GetLOINC(code string) (*TermConcept, error) {
	if c.loinc == nil {
		return nil, fmt.Errorf("LOINC code %q not found", code)
	}
	r, ok := c.loinc.Lookup(code)
	if !ok {
		return nil, fmt.Errorf("LOINC code %q not found", code)
	}
	return &TermConcept{Code: r.LOINC, Display: r.LongCommonName, Definition: r.Component}, nil
}

func (c *coreTermStore) SearchICD10AM(query string, limit int) ([]TermConcept, error) {
	if c.icd10 == nil {
		return []TermConcept{}, nil
	}
	results := c.icd10.Search(query, limit)
	out := make([]TermConcept, 0, len(results))
	for _, r := range results {
		out = append(out, TermConcept{Code: r.Code, Display: r.Description})
	}
	return out, nil
}

func (c *coreTermStore) SearchNZMT(query string, limit int) ([]TermConcept, error) {
	if c.nzmt == nil {
		return []TermConcept{}, nil
	}
	results := c.nzmt.Search(query, limit)
	out := make([]TermConcept, 0, len(results))
	for _, r := range results {
		display := r.BrandName
		if display == "" {
			display = r.GenericName
		}
		out = append(out, TermConcept{Code: r.NZULM, Display: display, Definition: r.GenericName})
	}
	return out, nil
}

// snomedDisplay prefers the NZ preferred term, falling back to the fully
// specified name when no preferred term was loaded.
func snomedDisplay(c *terminology.SNOMEDConcept) string {
	if c.PreferredTerm != "" {
		return c.PreferredTerm
	}
	return c.FSN
}

// newTerminologyHandler returns a TerminologyHandler. Pass a nil store to use
// the no-op stub (returns empty results).
func newTerminologyHandler(stores ...TermStore) *TerminologyHandler {
	var store TermStore = stubTermStore{}
	if len(stores) > 0 && stores[0] != nil {
		store = stores[0]
	}
	return &TerminologyHandler{store: store}
}

// router returns a mux with all terminology routes registered.
func (h *TerminologyHandler) router() http.Handler {
	mux := http.NewServeMux()

	// SNOMED CT
	mux.HandleFunc("/api/v1/terminology/snomed/search", h.handleSNOMEDSearch)
	mux.HandleFunc("/api/v1/terminology/snomed/", h.handleSNOMEDGet)

	// LOINC
	mux.HandleFunc("/api/v1/terminology/loinc/search", h.handleLOINCSearch)
	mux.HandleFunc("/api/v1/terminology/loinc/", h.handleLOINCGet)

	// ICD-10-AM
	mux.HandleFunc("/api/v1/terminology/icd10/search", h.handleICD10Search)

	// NZMT/NZULM
	mux.HandleFunc("/api/v1/terminology/nzmt/search", h.handleNZMTSearch)

	return mux
}

// ---------------------------------------------------------------------------
// SNOMED CT
// ---------------------------------------------------------------------------

// handleSNOMEDSearch serves GET /api/v1/terminology/snomed/search?q=...&limit=20
func (h *TerminologyHandler) handleSNOMEDSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSONError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	limit := parseLimit(r, defaultTermLimit)

	concepts, err := h.store.SearchSNOMED(q, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "SNOMED search failed: "+err.Error())
		return
	}
	writeTermResults(w, "http://snomed.info/sct", concepts)
}

// handleSNOMEDGet serves GET /api/v1/terminology/snomed/{sctid}
func (h *TerminologyHandler) handleSNOMEDGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sctid := extractLastSegment(r.URL.Path, "/api/v1/terminology/snomed/")
	if sctid == "" || sctid == "search" {
		writeJSONError(w, http.StatusBadRequest, "SCTID is required in the path")
		return
	}

	concept, err := h.store.GetSNOMED(sctid)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, concept)
}

// ---------------------------------------------------------------------------
// LOINC
// ---------------------------------------------------------------------------

// handleLOINCSearch serves GET /api/v1/terminology/loinc/search?q=...&limit=20
func (h *TerminologyHandler) handleLOINCSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSONError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	limit := parseLimit(r, defaultTermLimit)

	concepts, err := h.store.SearchLOINC(q, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "LOINC search failed: "+err.Error())
		return
	}
	writeTermResults(w, "http://loinc.org", concepts)
}

// handleLOINCGet serves GET /api/v1/terminology/loinc/{code}
func (h *TerminologyHandler) handleLOINCGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	code := extractLastSegment(r.URL.Path, "/api/v1/terminology/loinc/")
	if code == "" || code == "search" {
		writeJSONError(w, http.StatusBadRequest, "LOINC code is required in the path")
		return
	}

	concept, err := h.store.GetLOINC(code)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, concept)
}

// ---------------------------------------------------------------------------
// ICD-10-AM
// ---------------------------------------------------------------------------

// handleICD10Search serves GET /api/v1/terminology/icd10/search?q=...&limit=20
func (h *TerminologyHandler) handleICD10Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSONError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	limit := parseLimit(r, defaultTermLimit)

	concepts, err := h.store.SearchICD10AM(q, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "ICD-10-AM search failed: "+err.Error())
		return
	}
	writeTermResults(w, "http://hl7.org/fhir/sid/icd-10-am", concepts)
}

// ---------------------------------------------------------------------------
// NZMT / NZULM
// ---------------------------------------------------------------------------

// handleNZMTSearch serves GET /api/v1/terminology/nzmt/search?q=...&limit=20
func (h *TerminologyHandler) handleNZMTSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSONError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	limit := parseLimit(r, defaultTermLimit)

	concepts, err := h.store.SearchNZMT(q, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "NZMT search failed: "+err.Error())
		return
	}
	writeTermResults(w, "http://nzulm.org.nz", concepts)
}

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

// writeTermResults writes a JSON search result envelope containing a list of concepts.
func writeTermResults(w http.ResponseWriter, system string, concepts []TermConcept) {
	// Ensure system is propagated when the store doesn't set it.
	for i := range concepts {
		if concepts[i].System == "" {
			concepts[i].System = system
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"system":   system,
		"total":    len(concepts),
		"concepts": concepts,
	})
}

// parseLimit reads the "limit" query parameter, clamping to [1, maxTermLimit].
func parseLimit(r *http.Request, defaultVal int) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultVal
	}
	if n > maxTermLimit {
		return maxTermLimit
	}
	return n
}

// extractLastSegment returns the path segment after the given prefix,
// stripping any trailing slash.
func extractLastSegment(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	return strings.TrimSuffix(s, "/")
}
