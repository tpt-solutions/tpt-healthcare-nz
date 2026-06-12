package ddi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/pharmac"
)

// PharmacChecker wraps the PHARMAC interaction API as a DDI Checker.
// It delegates drug-drug checks to pharmac.Client.CheckInteractions and
// performs drug-allergy checks via a simple substance name match against the
// PHARMAC medicine generic name.
type PharmacChecker struct {
	client *pharmac.Client
}

// NewPharmacChecker creates a Checker backed by the PHARMAC interaction API.
func NewPharmacChecker(client *pharmac.Client) *PharmacChecker {
	return &PharmacChecker{client: client}
}

// Check delegates drug-drug interactions to the PHARMAC API.
// Drug-allergy checks are performed locally by comparing the allergy substance
// name against the generic name of each requested medicine.
func (p *PharmacChecker) Check(ctx context.Context, req CheckRequest) ([]Interaction, error) {
	if p.client == nil {
		return nil, fmt.Errorf("ddi: pharmac client is nil")
	}

	now := time.Now().UTC()
	var results []Interaction

	// --- Drug-drug interactions via PHARMAC API ---
	all := req.NZULMs
	if req.ProposedNZULM != "" {
		all = append(all, req.ProposedNZULM)
	}
	if len(all) >= 2 {
		pharmInteractions, err := p.client.CheckInteractions(ctx, all)
		if err != nil {
			return nil, fmt.Errorf("ddi: pharmac CheckInteractions: %w", err)
		}
		for _, pi := range pharmInteractions {
			results = append(results, Interaction{
				Kind:          KindDrugDrug,
				Drug1:         pi.Drug1,
				Drug2:         pi.Drug2,
				Severity:      SeverityFromString(pi.Severity),
				SeverityLabel: pi.Severity,
				Description:   pi.Description,
				Source:        "pharmac-api",
				CheckedAt:     now,
			})
		}
	}

	// --- Drug-allergy checks via generic name match ---
	if len(req.PatientAllergies) > 0 && req.ProposedNZULM != "" {
		med, err := p.client.GetByNZULM(ctx, req.ProposedNZULM)
		if err == nil && med != nil {
			genericLower := strings.ToLower(med.GenericName)
			for _, allergen := range req.PatientAllergies {
				if strings.Contains(genericLower, strings.ToLower(allergen)) {
					results = append(results, Interaction{
						Kind:          KindDrugAllergy,
						Drug1:         req.ProposedNZULM,
						Drug2:         allergen,
						Severity:      SeverityContraindicated,
						SeverityLabel: SeverityContraindicated.String(),
						Description: fmt.Sprintf(
							"Patient has a recorded allergy to %q. The proposed medicine %s (%s) contains this substance.",
							allergen, med.BrandName, med.GenericName,
						),
						Source:    "allergy-crosscheck",
						CheckedAt: now,
					})
				}
			}
		}
	}

	return results, nil
}
