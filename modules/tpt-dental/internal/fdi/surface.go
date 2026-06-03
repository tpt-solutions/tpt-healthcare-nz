// Package fdi surface codes represent the five tooth surfaces documented in
// dental charting, plus multi-surface combinations used in procedure coding.
//
// Standard surfaces (ISO 3950 / FDI):
//   - M = Mesial (towards midline)
//   - D = Distal (away from midline)
//   - O = Occlusal (biting surface of posterior teeth)
//   - I = Incisal (biting edge of anterior teeth)
//   - B = Buccal (cheek side of posterior teeth)
//   - L = Lingual (tongue side) / Palatal (palate side of upper teeth)
//   - V = Vestibular (generic labial/buccal)
//
// Multi-surface combinations are common in restorative coding:
//   - MO = Mesio-occlusal
//   - MOD = Mesio-occluso-distal
//   - DO = Disto-occlusal
//   - BL = Bucco-lingual
package fdi

import (
	"fmt"
	"sort"
	"strings"
)

// SurfaceCode represents a single tooth surface.
type SurfaceCode string

// Standard single-surface codes.
const (
	SurfaceMesial    SurfaceCode = "M"
	SurfaceDistal    SurfaceCode = "D"
	SurfaceOcclusal  SurfaceCode = "O"
	SurfaceIncisal   SurfaceCode = "I"
	SurfaceBuccal    SurfaceCode = "B"
	SurfaceLingual   SurfaceCode = "L"
	SurfacePalatal   SurfaceCode = "P"
	SurfaceLabial    SurfaceCode = "La"
	SurfaceVestibular SurfaceCode = "V"
	SurfaceCervical  SurfaceCode = "C"
)

// SurfaceName returns the full name of a surface code.
func SurfaceName(code SurfaceCode) string {
	switch code {
	case SurfaceMesial:
		return "Mesial"
	case SurfaceDistal:
		return "Distal"
	case SurfaceOcclusal:
		return "Occlusal"
	case SurfaceIncisal:
		return "Incisal"
	case SurfaceBuccal:
		return "Buccal"
	case SurfaceLingual:
		return "Lingual"
	case SurfacePalatal:
		return "Palatal"
	case SurfaceLabial:
		return "Labial"
	case SurfaceVestibular:
		return "Vestibular"
	case SurfaceCervical:
		return "Cervical"
	default:
		return fmt.Sprintf("Unknown(%s)", string(code))
	}
}

// ToothSurfaceRecord records the status of a specific surface on a tooth.
type ToothSurfaceRecord struct {
	ToothCode string     `json:"toothCode"` // FDI two-digit code
	Surface   string     `json:"surface"`   // surface code (possibly multi-surface e.g. "MOD")
	Status    string     `json:"status"`    // healthy, carious, filled, missing, fissure_sealant, crown, bridge_abutment, implant, etc.
	Note      string     `json:"note,omitempty"`
	Timestamp int64      `json:"timestamp"` // unix epoch ms
	Clinician string     `json:"clinician,omitempty"` // HPI CPN of the recording clinician
}

// ChartCellStatus represents the clinical status of a tooth in dental charting.
type ChartCellStatus string

const (
	StatusHealthy        ChartCellStatus = "healthy"
	StatusCarious        ChartCellStatus = "carious"
	StatusFilled         ChartCellStatus = "filled"
	StatusMissing        ChartCellStatus = "missing"
	StatusUnerupted      ChartCellStatus = "unerupted"
	StatusImpacted       ChartCellStatus = "impacted"
	StatusCrown          ChartCellStatus = "crown"
	StatusBridge         ChartCellStatus = "bridge"
	StatusImplant        ChartCellStatus = "implant"
	StatusRootCanal      ChartCellStatus = "root_canal"
	StatusFissureSealant ChartCellStatus = "fissure_sealant"
	StatusPartialDenture ChartCellStatus = "partial_denture"
	StatusFullDenture    ChartCellStatus = "full_denture"
	StatusFractured      ChartCellStatus = "fractured"
	StatusMobile         ChartCellStatus = "mobile"
	StatusDiastema       ChartCellStatus = "diastema"
	StatusRotation       ChartCellStatus = "rotation"
)

// ToothChartEntry is the complete charting state for one tooth.
type ToothChartEntry struct {
	ToothCode string           `json:"toothCode"`
	Status    ChartCellStatus  `json:"status"`
	Surfaces  []string         `json:"surfaces,omitempty"` // affected surfaces
	Mobility  int              `json:"mobility,omitempty"` // 0-3 Miller classification
	Note      string           `json:"note,omitempty"`
	UpdatedAt int64            `json:"updatedAt"`
}

// DentalChart represents the full dental chart for a patient at a point in time.
type DentalChart struct {
	PatientNHI  string             `json:"patientNhi"`
	Dentition   DentitionType      `json:"dentition"` // which set is being charted
	Entries     []ToothChartEntry  `json:"entries"`
	ChartDate   int64              `json:"chartDate"`
	ClinicianID string             `json:"clinicianId"`
	PracticeID  string             `json:"practiceId"`
	VisitID     string             `json:"visitId,omitempty"` // FHIR Encounter ID
}

// ParseSurfaceCombination parses a multi-surface string (e.g. "MOD", "MOB")
// into individual SurfaceCodes, sorted in standard order (mesial -> distal).
func ParseSurfaceCombination(s string) ([]SurfaceCode, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("surface: empty combination string")
	}

	// Handle known two-letter codes like "La" (Labial).
	runes := []rune(s)
	codes := make([]SurfaceCode, 0, len(runes))
	i := 0
	for i < len(runes) {
		c := string(runes[i])
		// Check for two-letter codes.
		if i+1 < len(runes) {
			twoLetter := c + string(runes[i+1])
			if isValidSurface(SurfaceCode(twoLetter)) {
				codes = append(codes, SurfaceCode(twoLetter))
				i += 2
				continue
			}
		}
		if !isValidSurface(SurfaceCode(c)) {
			return nil, fmt.Errorf("surface: unknown surface code %q in %q", c, s)
		}
		codes = append(codes, SurfaceCode(c))
		i++
	}

	// Sort into standard order.
	sort.Slice(codes, func(i, j int) bool {
		return surfaceOrder(codes[i]) < surfaceOrder(codes[j])
	})

	return codes, nil
}

// FormatSurfaceCombination formats a list of surfaces to a standard string, e.g., "MOD".
func FormatSurfaceCombination(codes []SurfaceCode) string {
	sorted := make([]SurfaceCode, len(codes))
	copy(sorted, codes)
	sort.Slice(sorted, func(i, j int) bool {
		return surfaceOrder(sorted[i]) < surfaceOrder(sorted[j])
	})
	var b strings.Builder
	for _, c := range sorted {
		b.WriteString(string(c))
	}
	return b.String()
}

func isValidSurface(code SurfaceCode) bool {
	switch code {
	case SurfaceMesial, SurfaceDistal, SurfaceOcclusal, SurfaceIncisal,
		SurfaceBuccal, SurfaceLingual, SurfacePalatal, SurfaceLabial,
		SurfaceVestibular, SurfaceCervical:
		return true
	default:
		return false
	}
}

// surfaceOrder returns an ordering weight for standard surface sequencing.
func surfaceOrder(code SurfaceCode) int {
	switch code {
	case SurfaceMesial:
		return 1
	case SurfaceIncisal, SurfaceOcclusal:
		return 2
	case SurfaceBuccal, SurfaceVestibular, SurfaceLabial:
		return 3
	case SurfaceLingual, SurfacePalatal:
		return 4
	case SurfaceDistal:
		return 5
	case SurfaceCervical:
		return 6
	default:
		return 99
	}
}

// IsAnterior returns true if the tooth class is incisor or canine.
func IsAnterior(toothClass string) bool {
	return toothClass == "incisor" || toothClass == "canine"
}

// SurfaceApplicable returns true if the surface is relevant to the given tooth class.
func SurfaceApplicable(surface SurfaceCode, toothClass string) bool {
	switch surface {
	case SurfaceOcclusal:
		return toothClass == "premolar" || toothClass == "molar"
	case SurfaceIncisal:
		return IsAnterior(toothClass)
	case SurfaceBuccal, SurfaceLingual, SurfaceMesial, SurfaceDistal:
		return true
	default:
		return false
	}
}