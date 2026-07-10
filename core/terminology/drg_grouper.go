package terminology

import (
	"fmt"
	"strings"
)

// DRGGrouper performs basic AR-DRG (Australian Refined Diagnosis Related Group) grouping.
// This is a simplified grouper for NZ hospital billing; production systems would
// use the full AIHW/NZ DRG grouper software.
type DRGGrouper struct{}

// DRGResult contains the DRG assignment.
type DRGResult struct {
	MDC          string  `json:"mdc"`          // Major Diagnostic Category
	DRGCode      string  `json:"drgCode"`      // e.g. "B65Z"
	DRGName      string  `json:"drgName"`
	Weight       float64 `json:"weight"`       // relative weight
	BasePrice    float64 `json:"basePrice"`    // NZD
	Complexity   string  `json:"complexity"`   // basic, moderate, high
	LOS          int     `json:"los"`          // expected length of stay (days)
}

// GroupDRG assigns an AR-DRG from principal diagnosis code (ICD-10-AM).
// This implements a simplified first-pass grouper covering the most common NZ
// hospital DRG groups. A production implementation would use the full AR-DRG
// classification with MCC/CC comorbidity adjustments.
func (g *DRGGrouper) GroupDRG(principalDiagnosis string, age int, hasMCC bool) DRGResult {
	code := strings.ToUpper(strings.TrimSpace(principalDiagnosis))

	// Determine MDC from first letter
	mdc := ""
	if len(code) > 0 {
		mdc = string(code[0])
	}

	result := DRGResult{
		MDC:        mdc,
		DRGCode:    fmt.Sprintf("%s00Z", mdc),
		DRGName:    fmt.Sprintf("Other %s", mdcChapter(mdc)),
		Weight:     1.0,
		BasePrice:  4000.0,
		Complexity: "basic",
		LOS:        5,
	}

	// MDC-specific DRG assignments for common codes
	switch mdc {
	case "I": // Circulatory
		result = g.groupCirculatory(code, hasMCC)
	case "J": // Respiratory
		result = g.groupRespiratory(code, hasMCC)
	case "K": // Digestive
		result = g.groupDigestive(code, hasMCC)
	case "M": // Musculoskeletal
		result = g.groupMusculoskeletal(code, hasMCC)
	case "N": // Skin
		result = g.groupSkin(code, hasMCC)
	case "F": // Eye
		result = g.groupEye(code, hasMCC)
	case "G": // Male reproductive
		result = g.groupMaleRepro(code, hasMCC)
	case "H": // Female reproductive
		result = g.groupFemaleRepro(code, hasMCC)
	case "L": // Hepatobiliary
		result = g.groupHepatobiliary(code, hasMCC)
	case "S": // Injury
		result = g.groupInjury(code, hasMCC)
	case "T": // Burns
		result.DRGCode = "T60Z"
		result.DRGName = "Burns"
		result.Weight = 2.5
		result.BasePrice = 10000
		result.LOS = 14
	}

	// Age adjustment: paediatric and geriatric are more complex.
	// age < 0 means "unknown" (e.g. age not resolvable from the admission) and
	// is not adjusted for.
	if age >= 0 && age < 1 {
		result.Weight *= 2.0
		result.BasePrice *= 2.0
		result.LOS += 3
		result.Complexity = "high"
	} else if age >= 0 && age < 5 {
		result.Weight *= 1.3
		result.LOS += 1
	}

	// MCC adjustment
	if hasMCC {
		result.Weight *= 1.5
		result.BasePrice *= 1.5
		result.Complexity = "high"
		result.LOS += 2
	}

	return result
}

func (g *DRGGrouper) groupCirculatory(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "I", Complexity: "moderate", LOS: 7}
	if strings.HasPrefix(code, "I21") || strings.HasPrefix(code, "I22") {
		r.DRGCode = "I15Z"
		r.DRGName = "AMI without MCC"
		r.Weight = 2.8
		r.BasePrice = 11200
		r.LOS = 6
	} else if strings.HasPrefix(code, "I63") || strings.HasPrefix(code, "I64") {
		r.DRGCode = "I60Z"
		r.DRGName = "Stroke"
		r.Weight = 2.2
		r.BasePrice = 8800
		r.LOS = 8
	} else if strings.HasPrefix(code, "I50") {
		r.DRGCode = "I29Z"
		r.DRGName = "Heart failure"
		r.Weight = 1.5
		r.BasePrice = 6000
		r.LOS = 6
	} else if strings.HasPrefix(code, "I48") {
		r.DRGCode = "I30Z"
		r.DRGName = "Atrial fibrillation"
		r.Weight = 1.0
		r.BasePrice = 4000
		r.LOS = 3
	} else {
		r.DRGCode = "I99Z"
		r.DRGName = "Other circulatory"
		r.Weight = 1.5
		r.BasePrice = 6000
	}
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
		r.Complexity = "high"
		r.LOS += 2
	}
	return r
}

func (g *DRGGrouper) groupRespiratory(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "J", Complexity: "moderate", LOS: 5}
	if strings.HasPrefix(code, "J18") || strings.HasPrefix(code, "J15") {
		r.DRGCode = "J01Z"
		r.DRGName = "Pneumonia"
		r.Weight = 1.8
		r.BasePrice = 7200
		r.LOS = 6
	} else if strings.HasPrefix(code, "J44") || strings.HasPrefix(code, "J43") {
		r.DRGCode = "J05Z"
		r.DRGName = "COPD"
		r.Weight = 1.3
		r.BasePrice = 5200
		r.LOS = 5
	} else {
		r.DRGCode = "J99Z"
		r.DRGName = "Other respiratory"
		r.Weight = 1.2
		r.BasePrice = 4800
	}
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
		r.Complexity = "high"
	}
	return r
}

func (g *DRGGrouper) groupDigestive(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "K", Complexity: "moderate", LOS: 5}
	if strings.HasPrefix(code, "K35") || strings.HasPrefix(code, "K37") {
		r.DRGCode = "K01Z"
		r.DRGName = "Appendicectomy"
		r.Weight = 1.5
		r.BasePrice = 6000
		r.LOS = 3
	} else if strings.HasPrefix(code, "K80") {
		r.DRGCode = "K03Z"
		r.DRGName = "Cholecystectomy"
		r.Weight = 1.8
		r.BasePrice = 7200
		r.LOS = 4
	} else if strings.HasPrefix(code, "K25") || strings.HasPrefix(code, "K26") {
		r.DRGCode = "K05Z"
		r.DRGName = "Gastric/duodenal ulcer"
		r.Weight = 1.4
		r.BasePrice = 5600
		r.LOS = 5
	} else {
		r.DRGCode = "K99Z"
		r.DRGName = "Other digestive"
		r.Weight = 1.3
		r.BasePrice = 5200
	}
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
		r.Complexity = "high"
	}
	return r
}

func (g *DRGGrouper) groupMusculoskeletal(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "M", Complexity: "moderate", LOS: 7}
	if strings.HasPrefix(code, "S72") {
		r.DRGCode = "M01Z"
		r.DRGName = "Hip fracture"
		r.Weight = 3.0
		r.BasePrice = 12000
		r.LOS = 10
	} else if strings.HasPrefix(code, "S82") {
		r.DRGCode = "M03Z"
		r.DRGName = "Knee fracture"
		r.Weight = 2.0
		r.BasePrice = 8000
		r.LOS = 7
	} else if strings.HasPrefix(code, "M17") || strings.HasPrefix(code, "M16") {
		r.DRGCode = "M05Z"
		r.DRGName = "Joint replacement"
		r.Weight = 3.5
		r.BasePrice = 14000
		r.LOS = 8
	} else {
		r.DRGCode = "M99Z"
		r.DRGName = "Other musculoskeletal"
		r.Weight = 1.5
		r.BasePrice = 6000
	}
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
		r.Complexity = "high"
	}
	return r
}

func (g *DRGGrouper) groupSkin(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "N", Complexity: "moderate", LOS: 8}
	if strings.HasPrefix(code, "L89") {
		r.DRGCode = "N01Z"
		r.DRGName = "Pressure injury"
		r.Weight = 2.0
		r.BasePrice = 8000
		r.LOS = 12
	} else {
		r.DRGCode = "N99Z"
		r.DRGName = "Other skin"
		r.Weight = 1.2
		r.BasePrice = 4800
	}
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
	}
	return r
}

func (g *DRGGrouper) groupEye(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "F", Complexity: "basic", LOS: 2}
	if strings.HasPrefix(code, "C69") || strings.HasPrefix(code, "C70") {
		r.DRGCode = "F01Z"
		r.DRGName = "Eye neoplasm"
		r.Weight = 2.0
		r.BasePrice = 8000
		r.LOS = 4
	} else if strings.HasPrefix(code, "H25") || strings.HasPrefix(code, "H26") {
		r.DRGCode = "F03Z"
		r.DRGName = "Cataract"
		r.Weight = 0.8
		r.BasePrice = 3200
		r.LOS = 1
	} else {
		r.DRGCode = "F99Z"
		r.DRGName = "Other eye"
		r.Weight = 1.0
		r.BasePrice = 4000
	}
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
	}
	return r
}

func (g *DRGGrouper) groupMaleRepro(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "G", Complexity: "basic", LOS: 3}
	r.DRGCode = "G99Z"
	r.DRGName = "Other male reproductive"
	r.Weight = 1.2
	r.BasePrice = 4800
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
	}
	return r
}

func (g *DRGGrouper) groupFemaleRepro(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "H", Complexity: "basic", LOS: 3}
	r.DRGCode = "H99Z"
	r.DRGName = "Other female reproductive"
	r.Weight = 1.2
	r.BasePrice = 4800
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
	}
	return r
}

func (g *DRGGrouper) groupHepatobiliary(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "L", Complexity: "moderate", LOS: 5}
	if strings.HasPrefix(code, "K70") || strings.HasPrefix(code, "K74") {
		r.DRGCode = "L01Z"
		r.DRGName = "Liver disease"
		r.Weight = 2.0
		r.BasePrice = 8000
		r.LOS = 8
	} else {
		r.DRGCode = "L99Z"
		r.DRGName = "Other hepatobiliary"
		r.Weight = 1.5
		r.BasePrice = 6000
	}
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
	}
	return r
}

func (g *DRGGrouper) groupInjury(code string, hasMCC bool) DRGResult {
	r := DRGResult{MDC: "S", Complexity: "moderate", LOS: 5}
	r.DRGCode = "S99Z"
	r.DRGName = "Other injury"
	r.Weight = 1.5
	r.BasePrice = 6000
	if hasMCC {
		r.Weight *= 1.5
		r.BasePrice *= 1.5
	}
	return r
}

func mdcChapter(mdc string) string {
	chapters := map[string]string{
		"A": "Pre-MDC", "B": "Nervous system", "C": "Eye",
		"D": "ENT", "E": "Respiratory (pre-MDC)", "F": "Circulatory",
		"G": "Hepatobiliary", "H": "Musculoskeletal", "I": "Skin",
		"J": "Endocrine", "K": "Kidney/UT", "L": "Male reproductive",
		"M": "Female reproductive", "N": "OB/GYN", "P": "Neonates",
		"Q": "Blood/immunological", "R": "Myeloproliferative",
		"S": "Infectious", "T": "Injury", "U": "Burns",
		"V": "Factors", "W": "Multiple significant trauma",
		"X": "HIV", "Y": "Alcohol/drug", "Z": "Other",
	}
	if ch, ok := chapters[mdc]; ok {
		return ch
	}
	return "Unknown"
}
