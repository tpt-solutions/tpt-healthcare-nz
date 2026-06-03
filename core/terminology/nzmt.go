package terminology

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// NZMTProduct represents a single product from the NZ Medicines Terminology
// (NZMT), also known as the NZ Universal List of Medicines (NZULM).
//
// The NZMT is the authoritative NZ medicine reference published by the NZ
// Medicines Classification team and maintained by NZULM.org.nz.
type NZMTProduct struct {
	// NZULM is the unique NZ Universal List of Medicines identifier.
	NZULM string
	// BrandName is the registered brand/trade name, e.g. "Panadol".
	BrandName string
	// GenericName is the INN/generic name, e.g. "paracetamol".
	GenericName string
	// DoseForm is the pharmaceutical dose form, e.g. "tablet".
	DoseForm string
	// Strength is the active ingredient strength, e.g. "500 mg".
	Strength string
	// RouteOfAdmin is the route of administration, e.g. "oral".
	RouteOfAdmin string
	// Active indicates whether the product is currently listed.
	Active bool
}

// NZMTStore is an in-memory store of NZMT products keyed by NZULM identifier.
type NZMTStore struct {
	products map[string]*NZMTProduct
}

// nzmtColumnIndex holds resolved header column indices for the NZMT CSV.
type nzmtColumnIndex struct {
	nzulm        int
	brandName    int
	genericName  int
	doseForm     int
	strength     int
	routeOfAdmin int
	active       int
}

// LoadNZMT reads an NZMT CSV export file and returns an in-memory NZMTStore.
//
// The expected CSV format (as exported from NZULM) has a header row with the
// following recognised column names (case-insensitive):
//
//	NZULM, BrandName, GenericName, DoseForm, Strength, RouteOfAdmin, Active
//
// Column order is flexible; unrecognised columns are ignored.
func LoadNZMT(csvPath string) (*NZMTStore, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("nzmt: open %s: %w", csvPath, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("nzmt: read header: %w", err)
	}

	idx := nzmtColumnIndex{
		nzulm:        -1,
		brandName:    -1,
		genericName:  -1,
		doseForm:     -1,
		strength:     -1,
		routeOfAdmin: -1,
		active:       -1,
	}

	for i, col := range header {
		switch strings.ToLower(strings.TrimSpace(col)) {
		case "nzulm", "nzulm_id", "id":
			if idx.nzulm == -1 {
				idx.nzulm = i
			}
		case "brandname", "brand_name", "brand":
			idx.brandName = i
		case "genericname", "generic_name", "generic", "inn":
			idx.genericName = i
		case "doseform", "dose_form", "form":
			idx.doseForm = i
		case "strength":
			idx.strength = i
		case "routeofadmin", "route_of_admin", "route":
			idx.routeOfAdmin = i
		case "active", "status":
			idx.active = i
		}
	}

	if idx.nzulm == -1 {
		return nil, fmt.Errorf("nzmt: required column NZULM not found in header")
	}

	store := &NZMTStore{products: make(map[string]*NZMTProduct)}

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("nzmt: read row: %w", err)
		}

		getCol := func(i int) string {
			if i < 0 || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}

		nzulm := getCol(idx.nzulm)
		if nzulm == "" {
			continue
		}

		activeStr := getCol(idx.active)
		active := activeStr == "" || // default to active if column absent
			strings.EqualFold(activeStr, "true") ||
			strings.EqualFold(activeStr, "yes") ||
			activeStr == "1" ||
			strings.EqualFold(activeStr, "active")

		store.products[nzulm] = &NZMTProduct{
			NZULM:        nzulm,
			BrandName:    getCol(idx.brandName),
			GenericName:  getCol(idx.genericName),
			DoseForm:     getCol(idx.doseForm),
			Strength:     getCol(idx.strength),
			RouteOfAdmin: getCol(idx.routeOfAdmin),
			Active:       active,
		}
	}

	return store, nil
}

// Lookup returns the NZMTProduct for the given NZULM identifier and true, or
// nil and false if not found.
func (s *NZMTStore) Lookup(nzulm string) (*NZMTProduct, bool) {
	p, ok := s.products[strings.TrimSpace(nzulm)]
	return p, ok
}

// Search performs a case-insensitive substring search across BrandName and
// GenericName fields. Results are returned in arbitrary order up to limit.
// A limit <= 0 returns all matches.
func (s *NZMTStore) Search(query string, limit int) []*NZMTProduct {
	q := strings.ToLower(query)
	var results []*NZMTProduct
	for _, p := range s.products {
		if strings.Contains(strings.ToLower(p.BrandName), q) ||
			strings.Contains(strings.ToLower(p.GenericName), q) {
			results = append(results, p)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results
}

// ByGenericName returns all products whose GenericName contains name
// (case-insensitive substring match). Returns all matching products with no
// limit applied.
func (s *NZMTStore) ByGenericName(name string) []*NZMTProduct {
	q := strings.ToLower(strings.TrimSpace(name))
	var results []*NZMTProduct
	for _, p := range s.products {
		if strings.Contains(strings.ToLower(p.GenericName), q) {
			results = append(results, p)
		}
	}
	return results
}
