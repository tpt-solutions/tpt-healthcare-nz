package ddi

import (
	"context"
	"strings"
	"time"
)

// localRule is a single hard-coded interaction rule for offline use.
type localRule struct {
	// substance1 and substance2 are lowercase generic name substrings.
	substance1  string
	substance2  string
	severity    Severity
	description string
}

// knownInteractions is a curated list of the most clinically important drug
// interactions for the NZ primary care context. This serves as an offline
// fallback when the PHARMAC API is unavailable. It is intentionally conservative:
// only contraindicated and major interactions are included to minimise alert
// fatigue.
var knownInteractions = []localRule{
	{"warfarin", "aspirin", SeverityMajor,
		"Concurrent use of warfarin and aspirin significantly increases bleeding risk. Monitor INR closely; consider gastroprotection."},
	{"warfarin", "ibuprofen", SeverityMajor,
		"NSAIDs inhibit platelet function and may displace warfarin from plasma proteins, increasing anticoagulation and bleeding risk."},
	{"warfarin", "naproxen", SeverityMajor,
		"NSAIDs inhibit platelet function and may displace warfarin from plasma proteins, increasing anticoagulation and bleeding risk."},
	{"warfarin", "fluconazole", SeverityContraindicated,
		"Fluconazole potently inhibits CYP2C9, the primary metaboliser of S-warfarin, causing dramatic INR elevation. Avoid combination; use topical antifungal if possible."},
	{"warfarin", "metronidazole", SeverityContraindicated,
		"Metronidazole inhibits CYP2C9 and CYP3A4, markedly potentiating warfarin. Avoid; or halve warfarin dose and monitor INR every 2–3 days."},
	{"ssri", "tramadol", SeverityMajor,
		"Risk of serotonin syndrome: agitation, hyperthermia, tachycardia, clonus. Avoid concurrent use; if necessary, start tramadol at lowest dose and monitor closely."},
	{"lithium", "ibuprofen", SeverityContraindicated,
		"NSAIDs reduce renal lithium clearance, causing toxicity (coarse tremor, ataxia, confusion). Avoid all NSAIDs in patients on lithium; use paracetamol for analgesia."},
	{"lithium", "naproxen", SeverityContraindicated,
		"NSAIDs reduce renal lithium clearance, causing toxicity. Avoid."},
	{"lithium", "diclofenac", SeverityContraindicated,
		"NSAIDs reduce renal lithium clearance, causing toxicity. Avoid."},
	{"metformin", "contrast", SeverityMajor,
		"Iodinated contrast media may cause acute kidney injury, increasing metformin accumulation and risk of lactic acidosis. Withhold metformin 48h before and after contrast procedures."},
	{"ace inhibitor", "potassium", SeverityMajor,
		"ACE inhibitors reduce aldosterone and increase serum potassium. Adding potassium supplements risks hyperkalaemia. Monitor electrolytes."},
	{"ace inhibitor", "spironolactone", SeverityMajor,
		"Dual renin-angiotensin blockade significantly increases hyperkalaemia risk. Use only with close electrolyte monitoring (eGFR, K+)."},
	{"simvastatin", "clarithromycin", SeverityContraindicated,
		"Clarithromycin (CYP3A4 inhibitor) markedly increases simvastatin plasma levels, raising rhabdomyolysis risk. Withhold simvastatin during clarithromycin course."},
	{"simvastatin", "erythromycin", SeverityMajor,
		"CYP3A4 inhibition elevates simvastatin levels; rhabdomyolysis risk. Withhold simvastatin or switch to pravastatin/rosuvastatin."},
	{"methotrexate", "trimethoprim", SeverityContraindicated,
		"Both drugs inhibit dihydrofolate reductase, causing additive bone marrow toxicity (pancytopenia). Combination is contraindicated; use alternative antibiotics."},
	{"methotrexate", "nsaid", SeverityMajor,
		"NSAIDs reduce renal methotrexate clearance, increasing toxicity risk (mucositis, myelosuppression). Avoid; monitor if unavoidable."},
	{"clopidogrel", "omeprazole", SeverityModerate,
		"Omeprazole inhibits CYP2C19, reducing conversion of clopidogrel to its active metabolite and potentially diminishing antiplatelet effect. Consider pantoprazole as an alternative PPI."},
	{"digoxin", "amiodarone", SeverityMajor,
		"Amiodarone inhibits P-glycoprotein and CYP3A4/2D6, increasing digoxin levels. Reduce digoxin dose by 30–50% and monitor ECG and digoxin levels."},
	{"quinolone", "antacid", SeverityModerate,
		"Divalent cations (calcium, magnesium, aluminium) chelate quinolone antibiotics, reducing oral absorption by up to 90%. Separate administration by at least 2 hours."},
	{"sotalol", "azithromycin", SeverityContraindicated,
		"Both drugs prolong QTc interval; concurrent use significantly increases torsades de pointes risk. Contraindicated: use an alternative antibiotic."},
	{"sotalol", "clarithromycin", SeverityContraindicated,
		"QTc-prolonging combination; torsades de pointes risk. Contraindicated."},
	{"haloperidol", "lithium", SeverityMajor,
		"Rare reports of irreversible neurotoxicity (encephalopathy, extrapyramidal effects) at therapeutic lithium levels. Monitor closely if combination unavoidable."},
	{"maoi", "tramadol", SeverityContraindicated,
		"Risk of potentially fatal serotonin syndrome and/or seizures. Contraindicated; allow 14-day washout after stopping MAOI before starting tramadol."},
	{"maoi", "ssri", SeverityContraindicated,
		"Life-threatening serotonin syndrome. Contraindicated; allow washout period (14 days after stopping MAOI, or 5 weeks for fluoxetine)."},
}

// LocalChecker is an offline DDI Checker that uses a hard-coded interaction rule
// set derived from high-risk interactions common in NZ primary care. It is
// intended as a fast first-pass check and fallback when the PHARMAC API is
// unavailable.
//
// Matching uses substring search on the NZULM generic name (passed in via
// GenericNames) because NZULM codes alone are opaque identifiers. Callers
// should resolve NZULMs to generic names before calling Check.
type LocalChecker struct {
	// GenericNames maps NZULM → lowercase generic name. Populate from PHARMAC
	// lookups or the local NZMT terminology loader.
	GenericNames map[string]string
}

// NewLocalChecker creates a LocalChecker. genericNames may be nil or empty;
// checks will still run but only substance names found in the map will match.
func NewLocalChecker(genericNames map[string]string) *LocalChecker {
	if genericNames == nil {
		genericNames = make(map[string]string)
	}
	return &LocalChecker{GenericNames: genericNames}
}

// Check evaluates the proposed medicine against the existing medicines and
// patient allergies using the local rule set.
func (l *LocalChecker) Check(ctx context.Context, req CheckRequest) ([]Interaction, error) {
	now := time.Now().UTC()
	var results []Interaction

	// Resolve NZULMs to generic names.
	proposed := strings.ToLower(l.genericName(req.ProposedNZULM))
	existing := make([]string, 0, len(req.NZULMs))
	for _, nzulm := range req.NZULMs {
		if g := l.genericName(nzulm); g != "" {
			existing = append(existing, strings.ToLower(g))
		}
	}

	// Drug-drug: check proposed against all existing.
	for _, rule := range knownInteractions {
		if proposed == "" {
			break
		}
		for _, existingGeneric := range existing {
			if matchesRule(rule, proposed, existingGeneric) {
				results = append(results, Interaction{
					Kind:          KindDrugDrug,
					Drug1:         req.ProposedNZULM,
					Drug2:         existingGeneric,
					Severity:      rule.severity,
					SeverityLabel: rule.severity.String(),
					Description:   rule.description,
					Source:        "nzmt-local",
					CheckedAt:     now,
				})
			}
		}
	}

	// Drug-allergy: simple substance containment check.
	for _, allergen := range req.PatientAllergies {
		allergenLower := strings.ToLower(allergen)
		if proposed != "" && strings.Contains(proposed, allergenLower) {
			results = append(results, Interaction{
				Kind:          KindDrugAllergy,
				Drug1:         req.ProposedNZULM,
				Drug2:         allergen,
				Severity:      SeverityContraindicated,
				SeverityLabel: SeverityContraindicated.String(),
				Description: "Patient has a recorded allergy to '" + allergen +
					"'. The proposed medicine (" + proposed + ") contains this substance.",
				Source:    "allergy-crosscheck",
				CheckedAt: now,
			})
		}
	}

	return results, nil
}

func (l *LocalChecker) genericName(nzulm string) string {
	if name, ok := l.GenericNames[nzulm]; ok {
		return name
	}
	return nzulm // fall back to using the NZULM itself (e.g. when it's already a name)
}

// matchesRule returns true when substance1 and substance2 appear in either
// combination of a and b (order-independent).
func matchesRule(rule localRule, a, b string) bool {
	return (strings.Contains(a, rule.substance1) && strings.Contains(b, rule.substance2)) ||
		(strings.Contains(a, rule.substance2) && strings.Contains(b, rule.substance1))
}
