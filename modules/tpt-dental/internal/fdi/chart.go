// Package fdi implements the FDI World Dental Federation two-digit notation system
// for tooth identification, supporting both permanent (1-8) and deciduous (A-K) dentitions.
//
// FDI notation uses a two-digit format:
//   - First digit: quadrant (1-4 permanent upper-right to lower-left, 5-8 deciduous same order)
//   - Second digit: tooth number within quadrant (1-8 for permanent, 1-5 for deciduous)
package fdi

import (
	"fmt"
	"strings"
)

// DentitionType classifies the tooth set.
type DentitionType string

const (
	DentitionPermanent DentitionType = "permanent"
	DentitionDeciduous DentitionType = "deciduous"
)

// Quadrant identifies one of eight dental quadrants.
type Quadrant int

const (
	QuadrantPermanentUpperRight Quadrant = 1
	QuadrantPermanentUpperLeft  Quadrant = 2
	QuadrantPermanentLowerLeft  Quadrant = 3
	QuadrantPermanentLowerRight Quadrant = 4
	QuadrantDeciduousUpperRight Quadrant = 5
	QuadrantDeciduousUpperLeft  Quadrant = 6
	QuadrantDeciduousLowerLeft  Quadrant = 7
	QuadrantDeciduousLowerRight Quadrant = 8
)

// ToothCode represents an FDI two-digit tooth notation.
type ToothCode struct {
	Quadrant Quadrant      `json:"quadrant"`
	Number   int           `json:"number"` // 1-8 permanent, 1-5 deciduous
	Raw      string        `json:"raw"`    // original two-digit string e.g. "18"
}

// Tooth contains full identification and metadata for a tooth.
type Tooth struct {
	Code         ToothCode     `json:"code"`
	Dentition    DentitionType `json:"dentition"`
	Name         string        `json:"name"`         // e.g. "Upper Right Central Incisor"
	Abbreviation string        `json:"abbreviation"` // e.g. "UR1"
	UniversalNum int           `json:"universalNum"` // Universal numbering system (1-32)
	IsPrimary    bool          `json:"isPrimary"`
}

// toothDef holds static data for each tooth position.
type toothDef struct {
	name         string
	abbreviation string
	universalNum int
}

// permanentToothNames maps (quadrant, number) -> toothDef for permanent teeth.
var permanentToothNames = map[int]map[int]toothDef{
	1: { // Upper Right — FDI 11=central incisor … 18=third molar
		1: {"Upper Right Central Incisor", "UR1", 8},
		2: {"Upper Right Lateral Incisor", "UR2", 7},
		3: {"Upper Right Canine (Cuspid)", "UR3", 6},
		4: {"Upper Right First Premolar", "UR4", 5},
		5: {"Upper Right Second Premolar", "UR5", 4},
		6: {"Upper Right First Molar", "UR6", 3},
		7: {"Upper Right Second Molar", "UR7", 2},
		8: {"Upper Right Third Molar (Wisdom)", "UR8", 1},
	},
	2: { // Upper Left
		1: {"Upper Left Central Incisor", "UL1", 9},
		2: {"Upper Left Lateral Incisor", "UL2", 10},
		3: {"Upper Left Canine (Cuspid)", "UL3", 11},
		4: {"Upper Left First Premolar", "UL4", 12},
		5: {"Upper Left Second Premolar", "UL5", 13},
		6: {"Upper Left First Molar", "UL6", 14},
		7: {"Upper Left Second Molar", "UL7", 15},
		8: {"Upper Left Third Molar (Wisdom)", "UL8", 16},
	},
	3: { // Lower Left — FDI 31=central incisor … 38=third molar
		1: {"Lower Left Central Incisor", "LL1", 24},
		2: {"Lower Left Lateral Incisor", "LL2", 23},
		3: {"Lower Left Canine (Cuspid)", "LL3", 22},
		4: {"Lower Left First Premolar", "LL4", 21},
		5: {"Lower Left Second Premolar", "LL5", 20},
		6: {"Lower Left First Molar", "LL6", 19},
		7: {"Lower Left Second Molar", "LL7", 18},
		8: {"Lower Left Third Molar (Wisdom)", "LL8", 17},
	},
	4: { // Lower Right
		1: {"Lower Right Central Incisor", "LR1", 25},
		2: {"Lower Right Lateral Incisor", "LR2", 26},
		3: {"Lower Right Canine (Cuspid)", "LR3", 27},
		4: {"Lower Right First Premolar", "LR4", 28},
		5: {"Lower Right Second Premolar", "LR5", 29},
		6: {"Lower Right First Molar", "LR6", 30},
		7: {"Lower Right Second Molar", "LR7", 31},
		8: {"Lower Right Third Molar (Wisdom)", "LR8", 32},
	},
}

// deciduousToothNames maps (quadrant, number) -> toothDef for deciduous teeth.
var deciduousToothNames = map[int]map[int]toothDef{
	5: { // Upper Right Deciduous — FDI 51=central incisor … 55=second molar
		1: {"Upper Right Central Incisor (Deciduous)", "UR-A", 5},
		2: {"Upper Right Lateral Incisor (Deciduous)", "UR-B", 4},
		3: {"Upper Right Canine (Deciduous)", "UR-C", 3},
		4: {"Upper Right First Molar (Deciduous)", "UR-D", 2},
		5: {"Upper Right Second Molar (Deciduous)", "UR-E", 1},
	},
	6: { // Upper Left Deciduous
		1: {"Upper Left Central Incisor (Deciduous)", "UL-A", 6},
		2: {"Upper Left Lateral Incisor (Deciduous)", "UL-B", 7},
		3: {"Upper Left Canine (Deciduous)", "UL-C", 8},
		4: {"Upper Left First Molar (Deciduous)", "UL-D", 9},
		5: {"Upper Left Second Molar (Deciduous)", "UL-E", 10},
	},
	7: { // Lower Left Deciduous — FDI 71=central incisor … 75=second molar
		1: {"Lower Left Central Incisor (Deciduous)", "LL-A", 15},
		2: {"Lower Left Lateral Incisor (Deciduous)", "LL-B", 14},
		3: {"Lower Left Canine (Deciduous)", "LL-C", 13},
		4: {"Lower Left First Molar (Deciduous)", "LL-D", 12},
		5: {"Lower Left Second Molar (Deciduous)", "LL-E", 11},
	},
	8: { // Lower Right Deciduous
		1: {"Lower Right Central Incisor (Deciduous)", "LR-A", 16},
		2: {"Lower Right Lateral Incisor (Deciduous)", "LR-B", 17},
		3: {"Lower Right Canine (Deciduous)", "LR-C", 18},
		4: {"Lower Right First Molar (Deciduous)", "LR-D", 19},
		5: {"Lower Right Second Molar (Deciduous)", "LR-E", 20},
	},
}

// ParseTooth parses an FDI two-digit string (e.g. "18", "55") into a ToothCode.
// Returns an error if the format is invalid or the tooth does not exist.
func ParseTooth(fdiCode string) (ToothCode, error) {
	fdiCode = strings.TrimSpace(fdiCode)
	if len(fdiCode) != 2 {
		return ToothCode{}, fmt.Errorf("fdi: invalid tooth code %q: must be exactly 2 digits", fdiCode)
	}

	quadrant := int(fdiCode[0] - '0')
	number := int(fdiCode[1] - '0')

	if quadrant < 1 || quadrant > 8 {
		return ToothCode{}, fmt.Errorf("fdi: invalid quadrant %d in code %q", quadrant, fdiCode)
	}
	if number < 1 || number > 8 {
		return ToothCode{}, fmt.Errorf("fdi: invalid tooth number %d in code %q", number, fdiCode)
	}

	// Validate the combination exists.
	if _, err := LookupTooth(fdiCode); err != nil {
		return ToothCode{}, err
	}

	return ToothCode{
		Quadrant: Quadrant(quadrant),
		Number:   number,
		Raw:      fdiCode,
	}, nil
}

// MustParseTooth is like ParseTooth but panics on error; for use in static contexts.
func MustParseTooth(fdiCode string) ToothCode {
	tc, err := ParseTooth(fdiCode)
	if err != nil {
		panic(err)
	}
	return tc
}

// LookupTooth returns the full Tooth metadata for a given FDI code string.
func LookupTooth(fdiCode string) (Tooth, error) {
	fdiCode = strings.TrimSpace(fdiCode)
	if len(fdiCode) != 2 {
		return Tooth{}, fmt.Errorf("fdi: invalid tooth code %q", fdiCode)
	}
	quadrant := int(fdiCode[0] - '0')
	number := int(fdiCode[1] - '0')

	var isPrimary bool
	var dentition DentitionType
	var lookup map[int]map[int]toothDef

	if quadrant >= 1 && quadrant <= 4 {
		dentition = DentitionPermanent
		isPrimary = false
		lookup = permanentToothNames
	} else {
		dentition = DentitionDeciduous
		isPrimary = true
		lookup = deciduousToothNames
	}

	quadDef, ok := lookup[quadrant]
	if !ok {
		return Tooth{}, fmt.Errorf("fdi: unknown quadrant %d in code %q", quadrant, fdiCode)
	}
	def, ok := quadDef[number]
	if !ok {
		return Tooth{}, fmt.Errorf("fdi: tooth %d does not exist in quadrant %d", number, quadrant)
	}

	return Tooth{
		Code: ToothCode{
			Quadrant: Quadrant(quadrant),
			Number:   number,
			Raw:      fdiCode,
		},
		Dentition:    dentition,
		Name:         def.name,
		Abbreviation: def.abbreviation,
		UniversalNum: def.universalNum,
		IsPrimary:    isPrimary,
	}, nil
}

// IsPermanent returns true if the quadrant indicates permanent dentition.
func (q Quadrant) IsPermanent() bool {
	return q >= 1 && q <= 4
}

// IsDeciduous returns true if the quadrant indicates deciduous (primary) dentition.
func (q Quadrant) IsDeciduous() bool {
	return q >= 5 && q <= 8
}

// Arch returns "upper" or "lower" for the quadrant.
func (q Quadrant) Arch() string {
	switch q {
	case 1, 2, 5, 6:
		return "upper"
	case 3, 4, 7, 8:
		return "lower"
	default:
		return "unknown"
	}
}

// Side returns "right" or "left" for the quadrant.
func (q Quadrant) Side() string {
	switch q {
	case 1, 4, 5, 8:
		return "right"
	case 2, 3, 6, 7:
		return "left"
	default:
		return "unknown"
	}
}

// ToothClass returns the tooth class: incisor, canine, premolar, or molar.
func ToothClass(number int) string {
	switch {
	case number <= 2:
		return "incisor"
	case number == 3:
		return "canine"
	case number == 4 || number == 5:
		return "premolar"
	case number >= 6:
		return "molar"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Validators
// ---------------------------------------------------------------------------

// ValidToothCode returns true if the FDI code string is valid.
func ValidToothCode(fdiCode string) bool {
	_, err := ParseTooth(fdiCode)
	return err == nil
}

// AllPermanentTeeth returns FDI codes for all 32 permanent teeth in standard charting order.
// Upper arch left-to-right: UR (18→11) then UL (21→28).
// Lower arch left-to-right: LR (48→41) then LL (31→38).
func AllPermanentTeeth() []string {
	codes := make([]string, 0, 32)
	for _, q := range []int{1, 2, 3, 4} {
		if q == 2 || q == 4 {
			// UL and LR: ascending from midline outward (1→8)
			for n := 1; n <= 8; n++ {
				codes = append(codes, fmt.Sprintf("%d%d", q, n))
			}
		} else {
			// UR and LL: descending from distal to midline (8→1)
			for n := 8; n >= 1; n-- {
				codes = append(codes, fmt.Sprintf("%d%d", q, n))
			}
		}
	}
	return codes
}

// AllDeciduousTeeth returns FDI codes for all 20 deciduous teeth in order.
func AllDeciduousTeeth() []string {
	codes := make([]string, 0, 20)
	for _, q := range []int{5, 6, 7, 8} {
		for n := 5; n >= 1; n-- {
			codes = append(codes, fmt.Sprintf("%d%d", q, n))
		}
	}
	return codes
}