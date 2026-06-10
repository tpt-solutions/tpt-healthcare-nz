package api

import (
	"strings"

	"github.com/PhillipC05/tpt-healthcare/core/episurv"
)

// icd10ToNotifiable maps ICD-10-AM diagnosis codes to EpiSurv notifiable
// conditions. Returns false when the code is not a notifiable disease.
// Matching uses a prefix check so both the base code (e.g. "B05") and any
// sub-classification (e.g. "B05.9") are recognised.
func icd10ToNotifiable(code string) (episurv.NotifiableCondition, bool) {
	upper := strings.ToUpper(strings.TrimSpace(code))
	// Strip sub-classification for prefix matching.
	base := upper
	if i := strings.IndexByte(upper, '.'); i > 0 {
		base = upper[:i]
	}

	switch {
	// Measles B05
	case base == "B05":
		return episurv.ConditionMeasles, true
	// Mumps B26
	case base == "B26":
		return episurv.ConditionMumps, true
	// Rubella B06
	case base == "B06":
		return episurv.ConditionRubella, true
	// Whooping cough / pertussis A37
	case base == "A37":
		return episurv.ConditionPertussis, true
	// Tuberculosis A15–A19
	case base >= "A15" && base <= "A19":
		return episurv.ConditionTuberculosis, true
	// Salmonella A02
	case base == "A02":
		return episurv.ConditionSalmonella, true
	// Campylobacteriosis A04.5
	case upper == "A04.5":
		return episurv.ConditionCampylobacteriosis, true
	// Listeriosis A32
	case base == "A32":
		return episurv.ConditionListeriosis, true
	// Legionellosis A48.1, A48.2
	case upper == "A48.1" || upper == "A48.2":
		return episurv.ConditionLegionellosis, true
	// Hepatitis A B15
	case base == "B15":
		return episurv.ConditionHepatitisA, true
	// Hepatitis B B16
	case base == "B16":
		return episurv.ConditionHepatitisB, true
	// Hepatitis C B17.1
	case upper == "B17.1":
		return episurv.ConditionHepatitisC, true
	// HIV/AIDS B20–B24, Z21
	case (base >= "B20" && base <= "B24") || base == "Z21":
		return episurv.ConditionHIV, true
	// Gonorrhoea A54
	case base == "A54":
		return episurv.ConditionGonorrhoea, true
	// Syphilis A50–A53
	case base >= "A50" && base <= "A53":
		return episurv.ConditionSyphilis, true
	// COVID-19 U07.1, U07.2
	case upper == "U07.1" || upper == "U07.2":
		return episurv.ConditionCOVID19, true
	// Influenza J09, J10, J11
	case base == "J09" || base == "J10" || base == "J11":
		return episurv.ConditionInfluenza, true
	// Meningococcal disease A39
	case base == "A39":
		return episurv.ConditionMeningococcal, true
	// Tetanus A33–A35
	case base >= "A33" && base <= "A35":
		return episurv.ConditionTetanus, true
	// Typhoid fever A01.0
	case upper == "A01.0" || base == "A01":
		return episurv.ConditionTyphi, true
	// Cryptosporidiosis A07.2
	case upper == "A07.2":
		return episurv.ConditionCryptosporidiosis, true
	// Giardiasis A07.1
	case upper == "A07.1":
		return episurv.ConditionGiardiasis, true
	// Acute rheumatic fever I00–I02
	case base >= "I00" && base <= "I02":
		return episurv.ConditionRheumaticFever, true
	default:
		return "", false
	}
}
