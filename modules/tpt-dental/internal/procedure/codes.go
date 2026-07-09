// Package procedure provides NZ dental procedure codes and fee schedules.
// It includes the Dental Council of New Zealand (DCNZ) codes and ACC dental
// treatment codes used for claiming.
package procedure

import (
	"encoding/json"
	"fmt"
)

// CodeSystem identifies the coding system used for a procedure.
type CodeSystem string

const (
	CodeSystemDCNZ   CodeSystem = "dcnz"   // Dental Council NZ procedure codes
	CodeSystemACC    CodeSystem = "acc"    // ACC dental treatment codes
	CodeSystemSNOMED CodeSystem = "snomed" // SNOMED CT
	CodeSystemNZULM  CodeSystem = "nzulm"  // NZ Medicines Terminology
)

// ProcedureCategory groups dental procedures by type.
type ProcedureCategory string

const (
	CategoryExamination    ProcedureCategory = "examination"
	CategoryRadiology      ProcedureCategory = "radiology"
	CategoryPreventive     ProcedureCategory = "preventive"
	CategoryRestorative    ProcedureCategory = "restorative"
	CategoryEndodontics    ProcedureCategory = "endodontics"
	CategoryPeriodontics   ProcedureCategory = "periodontics"
	CategoryProsthodontics ProcedureCategory = "prosthodontics" // crowns, bridges, dentures
	CategoryOralSurgery    ProcedureCategory = "oral_surgery"
	CategoryOrthodontics   ProcedureCategory = "orthodontics"
	CategoryPaedodontics   ProcedureCategory = "paedodontics"
	CategorySedation       ProcedureCategory = "sedation"
	CategoryACC            ProcedureCategory = "acc" // ACC-funded dental treatment
	CategoryGeneral        ProcedureCategory = "general"
	CategoryOther          ProcedureCategory = "other"
)

// ProcedureCode defines a dental procedure code with associated metadata.
type ProcedureCode struct {
	Code           string            `json:"code"`
	System         CodeSystem        `json:"system"`
	Category       ProcedureCategory `json:"category"`
	ShortName      string            `json:"shortName"`
	FullName       string            `json:"fullName"`
	Fee            int               `json:"fee"` // standard fee in NZ cents, 0 = variable
	RequiresXRay   bool              `json:"requiresXRay"`
	IsACCClaimable bool              `json:"isAccClaimable"` // eligible for ACC subsidy
}

// ---------------------------------------------------------------------------
// DCNZ Procedure Codes
// ---------------------------------------------------------------------------

// DCNZCodes returns all Dental Council NZ procedure codes.
func DCNZCodes() []ProcedureCode {
	return []ProcedureCode{
		// Examination
		{Code: "011", System: "dcnz", Category: CategoryExamination, ShortName: "Comprehensive Exam", FullName: "Comprehensive oral examination", Fee: 0, IsACCClaimable: true},
		{Code: "012", System: "dcnz", Category: CategoryExamination, ShortName: "Periodic Exam", FullName: "Periodic oral examination", Fee: 0, IsACCClaimable: true},
		{Code: "013", System: "dcnz", Category: CategoryExamination, ShortName: "Emergency Exam", FullName: "Emergency oral examination", Fee: 0, IsACCClaimable: true},
		{Code: "014", System: "dcnz", Category: CategoryExamination, ShortName: "Consultation", FullName: "Specialist consultation", Fee: 0, IsACCClaimable: true},

		// Radiographs
		{Code: "021", System: "dcnz", Category: CategoryRadiology, ShortName: "OPG", FullName: "Orthopantomogram (panoramic X-ray)", Fee: 0, IsACCClaimable: true},
		{Code: "022", System: "dcnz", Category: CategoryRadiology, ShortName: "BWX 2", FullName: "Two bitewing radiographs", Fee: 0, RequiresXRay: true, IsACCClaimable: true},
		{Code: "023", System: "dcnz", Category: CategoryRadiology, ShortName: "BWX 4", FullName: "Four bitewing radiographs", Fee: 0, RequiresXRay: true, IsACCClaimable: true},
		{Code: "024", System: "dcnz", Category: CategoryRadiology, ShortName: "Periapical", FullName: "Single periapical radiograph", Fee: 0, RequiresXRay: true, IsACCClaimable: true},

		// Preventative
		{Code: "031", System: "dcnz", Category: CategoryPreventive, ShortName: "Scale & Polish", FullName: "Scale and polish (prophylaxis)", Fee: 0, IsACCClaimable: true},
		{Code: "032", System: "dcnz", Category: CategoryPreventive, ShortName: "Fissure Sealant", FullName: "Fissure sealant per tooth", Fee: 0, IsACCClaimable: true},
		{Code: "033", System: "dcnz", Category: CategoryPreventive, ShortName: "Topical Fluoride", FullName: "Topical fluoride application", Fee: 0, IsACCClaimable: false},
		{Code: "034", System: "dcnz", Category: CategoryPreventive, ShortName: "Oral Hygiene Instr.", FullName: "Oral hygiene instruction", Fee: 0, IsACCClaimable: false},

		// Restorative (amalgam)
		{Code: "111", System: "dcnz", Category: CategoryRestorative, ShortName: "Amalgam 1-surface", FullName: "Amalgam restoration - one surface", Fee: 0, IsACCClaimable: true},
		{Code: "112", System: "dcnz", Category: CategoryRestorative, ShortName: "Amalgam 2-surface", FullName: "Amalgam restoration - two surfaces", Fee: 0, IsACCClaimable: true},
		{Code: "113", System: "dcnz", Category: CategoryRestorative, ShortName: "Amalgam 3-surface+", FullName: "Amalgam restoration - three or more surfaces", Fee: 0, IsACCClaimable: true},

		// Restorative (resin composite / white fillings)
		{Code: "121", System: "dcnz", Category: CategoryRestorative, ShortName: "Composite 1-surface ant.", FullName: "Resin composite - one surface anterior", Fee: 0, IsACCClaimable: true},
		{Code: "122", System: "dcnz", Category: CategoryRestorative, ShortName: "Composite 2-surface ant.", FullName: "Resin composite - two surfaces anterior", Fee: 0, IsACCClaimable: true},
		{Code: "123", System: "dcnz", Category: CategoryRestorative, ShortName: "Composite 3-surface+ ant.", FullName: "Resin composite - three or more surfaces anterior", Fee: 0, IsACCClaimable: true},
		{Code: "124", System: "dcnz", Category: CategoryRestorative, ShortName: "Composite 1-surface post.", FullName: "Resin composite - one surface posterior", Fee: 0, IsACCClaimable: true},
		{Code: "125", System: "dcnz", Category: CategoryRestorative, ShortName: "Composite 2-surface post.", FullName: "Resin composite - two surfaces posterior", Fee: 0, IsACCClaimable: true},
		{Code: "126", System: "dcnz", Category: CategoryRestorative, ShortName: "Composite 3-surface+ post.", FullName: "Resin composite - three or more surfaces posterior", Fee: 0, IsACCClaimable: true},

		// Restorative (other)
		{Code: "131", System: "dcnz", Category: CategoryRestorative, ShortName: "Glass Ionomer", FullName: "Glass ionomer / RMGI restoration", Fee: 0, IsACCClaimable: true},
		{Code: "132", System: "dcnz", Category: CategoryRestorative, ShortName: "Core build-up", FullName: "Core build-up / foundation restoration", Fee: 0, IsACCClaimable: true},

		// Endodontics
		{Code: "211", System: "dcnz", Category: CategoryEndodontics, ShortName: "RCT 1-canal", FullName: "Root canal therapy - one canal", Fee: 0, IsACCClaimable: true},
		{Code: "212", System: "dcnz", Category: CategoryEndodontics, ShortName: "RCT 2-canals", FullName: "Root canal therapy - two canals", Fee: 0, IsACCClaimable: true},
		{Code: "213", System: "dcnz", Category: CategoryEndodontics, ShortName: "RCT 3-canals", FullName: "Root canal therapy - three canals", Fee: 0, IsACCClaimable: true},
		{Code: "214", System: "dcnz", Category: CategoryEndodontics, ShortName: "RCT 4+ canals", FullName: "Root canal therapy - four or more canals", Fee: 0, IsACCClaimable: true},
		{Code: "221", System: "dcnz", Category: CategoryEndodontics, ShortName: "Pulp capping", FullName: "Direct pulp capping", Fee: 0, IsACCClaimable: true},
		{Code: "222", System: "dcnz", Category: CategoryEndodontics, ShortName: "Apexification", FullName: "Apexification / apexogenesis", Fee: 0, IsACCClaimable: true},

		// Periodontics
		{Code: "311", System: "dcnz", Category: CategoryPeriodontics, ShortName: "Scaling/root planing 1", FullName: "Scaling and root planing - one sextant", Fee: 0, IsACCClaimable: true},
		{Code: "312", System: "dcnz", Category: CategoryPeriodontics, ShortName: "Gingivectomy", FullName: "Gingivectomy / gingivoplasty", Fee: 0, IsACCClaimable: true},
		{Code: "313", System: "dcnz", Category: CategoryPeriodontics, ShortName: "Periodontal surgery", FullName: "Periodontal flap surgery", Fee: 0, IsACCClaimable: true},
		{Code: "321", System: "dcnz", Category: CategoryPeriodontics, ShortName: "Crown lengthening", FullName: "Crown lengthening procedure", Fee: 0, IsACCClaimable: false},
		{Code: "322", System: "dcnz", Category: CategoryPeriodontics, ShortName: "Gum graft", FullName: "Free gingival / connective tissue graft", Fee: 0, IsACCClaimable: false},

		// Oral Surgery
		{Code: "411", System: "dcnz", Category: CategoryOralSurgery, ShortName: "Extraction simple", FullName: "Simple extraction", Fee: 0, IsACCClaimable: true},
		{Code: "412", System: "dcnz", Category: CategoryOralSurgery, ShortName: "Extraction surgical", FullName: "Surgical extraction", Fee: 0, IsACCClaimable: true},
		{Code: "413", System: "dcnz", Category: CategoryOralSurgery, ShortName: "Wisdom tooth simple", FullName: "Third molar extraction - simple", Fee: 0, IsACCClaimable: true},
		{Code: "414", System: "dcnz", Category: CategoryOralSurgery, ShortName: "Wisdom tooth surgical", FullName: "Third molar extraction - surgical", Fee: 0, IsACCClaimable: true},
		{Code: "415", System: "dcnz", Category: CategoryOralSurgery, ShortName: "Wisdom tooth complex", FullName: "Third molar extraction - complex/bony impaction", Fee: 0, IsACCClaimable: true},
		{Code: "421", System: "dcnz", Category: CategoryOralSurgery, ShortName: "Abscess drainage", FullName: "Incision and drainage of abscess", Fee: 0, IsACCClaimable: true},
		{Code: "422", System: "dcnz", Category: CategoryOralSurgery, ShortName: "Biopsy", FullName: "Biopsy of oral tissue", Fee: 0, IsACCClaimable: true},
		{Code: "423", System: "dcnz", Category: CategoryOralSurgery, ShortName: "Alveoloplasty", FullName: "Alveoloplasty", Fee: 0, IsACCClaimable: true},
		{Code: "424", System: "dcnz", Category: CategoryOralSurgery, ShortName: "Frenectomy", FullName: "Frenectomy / frenotomy", Fee: 0, IsACCClaimable: false},

		// Prosthodontics - Crowns
		{Code: "511", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Crown - full cast", FullName: "Full cast metal crown", Fee: 0, IsACCClaimable: false},
		{Code: "512", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Crown - PFM", FullName: "Porcelain fused to metal crown", Fee: 0, IsACCClaimable: false},
		{Code: "513", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Crown - all ceramic", FullName: "All-ceramic crown", Fee: 0, IsACCClaimable: false},
		{Code: "514", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Crown - zirconia", FullName: "Zirconia crown", Fee: 0, IsACCClaimable: false},

		// Prosthodontics - Bridges
		{Code: "521", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Bridge pontic", FullName: "Bridge pontic (per unit)", Fee: 0, IsACCClaimable: false},
		{Code: "522", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Maryland bridge", FullName: "Resin-bonded bridge (Maryland)", Fee: 0, IsACCClaimable: false},

		// Prosthodontics - Dentures
		{Code: "531", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Full upper denture", FullName: "Complete upper denture", Fee: 0, IsACCClaimable: true},
		{Code: "532", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Full lower denture", FullName: "Complete lower denture", Fee: 0, IsACCClaimable: true},
		{Code: "533", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Partial denture", FullName: "Partial acrylic denture (per arch)", Fee: 0, IsACCClaimable: true},
		{Code: "534", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Chrome partial denture", FullName: "Chrome cobalt partial denture (per arch)", Fee: 0, IsACCClaimable: true},
		{Code: "535", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Denture reline", FullName: "Denture reline (per arch)", Fee: 0, IsACCClaimable: true},
		{Code: "536", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Denture repair", FullName: "Denture repair", Fee: 0, IsACCClaimable: true},
		{Code: "537", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Implants", FullName: "Dental implant placement (per implant)", Fee: 0, IsACCClaimable: false},
		{Code: "538", System: "dcnz", Category: CategoryProsthodontics, ShortName: "Implant abutment", FullName: "Implant abutment / crown connection", Fee: 0, IsACCClaimable: false},

		// Orthodontics
		{Code: "611", System: "dcnz", Category: CategoryOrthodontics, ShortName: "Ortho assessment", FullName: "Orthodontic assessment", Fee: 0, IsACCClaimable: false},
		{Code: "612", System: "dcnz", Category: CategoryOrthodontics, ShortName: "Fixed appliances", FullName: "Fixed orthodontic appliances (per arch)", Fee: 0, IsACCClaimable: false},
		{Code: "613", System: "dcnz", Category: CategoryOrthodontics, ShortName: "Removable appliance", FullName: "Removable orthodontic appliance", Fee: 0, IsACCClaimable: false},
		{Code: "614", System: "dcnz", Category: CategoryOrthodontics, ShortName: "Retainer", FullName: "Orthodontic retainer (per arch)", Fee: 0, IsACCClaimable: false},
		{Code: "615", System: "dcnz", Category: CategoryOrthodontics, ShortName: "Clear aligners", FullName: "Clear aligner therapy (per arch)", Fee: 0, IsACCClaimable: false},

		// Paedodontics
		{Code: "711", System: "dcnz", Category: CategoryPaedodontics, ShortName: "Pulpotomy", FullName: "Pulpotomy (primary tooth)", Fee: 0, IsACCClaimable: true},
		{Code: "712", System: "dcnz", Category: CategoryPaedodontics, ShortName: "Pulpectomy", FullName: "Pulpectomy (primary tooth)", Fee: 0, IsACCClaimable: true},
		{Code: "713", System: "dcnz", Category: CategoryPaedodontics, ShortName: "SSC crown", FullName: "Stainless steel crown (primary tooth)", Fee: 0, IsACCClaimable: true},
		{Code: "714", System: "dcnz", Category: CategoryPaedodontics, ShortName: "Space maintainer", FullName: "Space maintainer", Fee: 0, IsACCClaimable: true},

		// Sedation
		{Code: "811", System: "dcnz", Category: CategorySedation, ShortName: "N2O sedation", FullName: "Inhalation sedation (nitrous oxide)", Fee: 0, IsACCClaimable: false},
		{Code: "812", System: "dcnz", Category: CategorySedation, ShortName: "IV sedation", FullName: "Intravenous sedation", Fee: 0, IsACCClaimable: false},
		{Code: "813", System: "dcnz", Category: CategorySedation, ShortName: "GA", FullName: "General anaesthesia", Fee: 0, IsACCClaimable: false},

		// General / Miscellaneous
		{Code: "911", System: "dcnz", Category: CategoryGeneral, ShortName: "Mouthguard", FullName: "Sports mouthguard (custom)", Fee: 0, IsACCClaimable: false},
		{Code: "912", System: "dcnz", Category: CategoryGeneral, ShortName: "Nightguard", FullName: "Occlusal splint / nightguard", Fee: 0, IsACCClaimable: false},
		{Code: "913", System: "dcnz", Category: CategoryGeneral, ShortName: "Whitening", FullName: "Tooth whitening / bleaching", Fee: 0, IsACCClaimable: false},
		{Code: "921", System: "dcnz", Category: CategoryGeneral, ShortName: "TMJ assessment", FullName: "Temporomandibular joint assessment", Fee: 0, IsACCClaimable: true},
		{Code: "922", System: "dcnz", Category: CategoryGeneral, ShortName: "TMJ splint", FullName: "TMJ splint therapy", Fee: 0, IsACCClaimable: true},
		{Code: "931", System: "dcnz", Category: CategoryGeneral, ShortName: "Smoking cessation", FullName: "Smoking cessation counselling", Fee: 0, IsACCClaimable: false},
	}
}

// ---------------------------------------------------------------------------
// ACC Dental Treatment Codes
// ---------------------------------------------------------------------------

// ACCDentalCode represents an ACC-funded dental treatment code for injury-related care.
type ACCDentalCode struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	MaxSubsidy  int    `json:"maxSubsidy"` // maximum ACC subsidy in NZ cents
}

// ACCDentalCodes returns all current ACC dental treatment subsidy codes.
func ACCDentalCodes() []ACCDentalCode {
	return []ACCDentalCode{
		{Code: "A1", Description: "Examination and report (ACC claim), per claim", MaxSubsidy: 0},
		{Code: "A2", Description: "Radiograph - periapical or bitewing, per film", MaxSubsidy: 0},
		{Code: "A3", Description: "Panoramic radiograph (OPG)", MaxSubsidy: 0},
		{Code: "B1", Description: "Extraction - simple, per tooth", MaxSubsidy: 0},
		{Code: "B2", Description: "Extraction - surgical, per tooth", MaxSubsidy: 0},
		{Code: "B3", Description: "Extraction - impacted/partial bony, per tooth", MaxSubsidy: 0},
		{Code: "C1", Description: "Restoration - amalgam, one surface", MaxSubsidy: 0},
		{Code: "C2", Description: "Restoration - amalgam, two surfaces", MaxSubsidy: 0},
		{Code: "C3", Description: "Restoration - amalgam, three or more surfaces", MaxSubsidy: 0},
		{Code: "D1", Description: "Restoration - resin composite, one surface anterior", MaxSubsidy: 0},
		{Code: "D2", Description: "Restoration - resin composite, two surfaces anterior", MaxSubsidy: 0},
		{Code: "D3", Description: "Restoration - resin composite, three or more surfaces anterior", MaxSubsidy: 0},
		{Code: "E1", Description: "Restoration - resin composite, one surface posterior", MaxSubsidy: 0},
		{Code: "E2", Description: "Restoration - resin composite, two surfaces posterior", MaxSubsidy: 0},
		{Code: "E3", Description: "Restoration - resin composite, three or more surfaces posterior", MaxSubsidy: 0},
		{Code: "F1", Description: "Root canal therapy - single canal", MaxSubsidy: 0},
		{Code: "F2", Description: "Root canal therapy - two canals", MaxSubsidy: 0},
		{Code: "F3", Description: "Root canal therapy - three canals", MaxSubsidy: 0},
		{Code: "F4", Description: "Root canal therapy - four or more canals", MaxSubsidy: 0},
		{Code: "G1", Description: "Crown - full cast metal (ACC claimant)", MaxSubsidy: 0},
		{Code: "G2", Description: "Crown - porcelain fused to metal", MaxSubsidy: 0},
		{Code: "H1", Description: "Denture - complete upper", MaxSubsidy: 0},
		{Code: "H2", Description: "Denture - complete lower", MaxSubsidy: 0},
		{Code: "H3", Description: "Denture - partial (per arch)", MaxSubsidy: 0},
		{Code: "H4", Description: "Denture - chrome cobalt partial", MaxSubsidy: 0},
		{Code: "H5", Description: "Denture repair - per arch", MaxSubsidy: 0},
		{Code: "H6", Description: "Denture reline - per arch", MaxSubsidy: 0},
		{Code: "I1", Description: "Incision and drainage of abscess", MaxSubsidy: 0},
		{Code: "I2", Description: "Biopsy of oral tissue", MaxSubsidy: 0},
		{Code: "I3", Description: "Alveoloplasty", MaxSubsidy: 0},
		{Code: "J1", Description: "Periodontal treatment - per sextant", MaxSubsidy: 0},
		{Code: "K1", Description: "TMJ - assessment and splint therapy", MaxSubsidy: 0},
	}
}

// ---------------------------------------------------------------------------
// Lookup Helpers
// ---------------------------------------------------------------------------

var (
	dcnzByCode   map[string]ProcedureCode
	accCodesByID map[string]ACCDentalCode
)

func init() {
	dcnzByCode = make(map[string]ProcedureCode)
	for _, c := range DCNZCodes() {
		dcnzByCode[c.Code] = c
	}
	accCodesByID = make(map[string]ACCDentalCode)
	for _, c := range ACCDentalCodes() {
		accCodesByID[c.Code] = c
	}
}

// LookupDCNZ returns the DCNZ procedure code details by code string.
func LookupDCNZ(code string) (ProcedureCode, bool) {
	c, ok := dcnzByCode[code]
	return c, ok
}

// LookupACCDental returns the ACC dental code details by code string.
func LookupACCDental(code string) (ACCDentalCode, bool) {
	c, ok := accCodesByID[code]
	return c, ok
}

// ProceduresByCategory returns all DCNZ codes in the given category.
func ProceduresByCategory(cat ProcedureCategory) []ProcedureCode {
	var res []ProcedureCode
	for _, c := range DCNZCodes() {
		if c.Category == cat {
			res = append(res, c)
		}
	}
	return res
}

// DCNZToJSON returns the DCNZ code list as a JSON string.
func DCNZToJSON() (string, error) {
	b, err := json.MarshalIndent(DCNZCodes(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("procedure: marshal DCNZ codes: %w", err)
	}
	return string(b), nil
}
