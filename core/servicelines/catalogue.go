// Package servicelines defines the catalogue of clinical service lines a
// tenant may run (emergency department, ICU, oncology, etc.) and resolves
// the module, ward-type, triage-scale, and formulary defaults each service
// line implies.
//
// A tenant is not forced into a single fixed per-hospital template: real
// facilities are combinations (e.g. one campus with both adult and
// paediatric wards, or a single-service community clinic), so a tenant
// selects any set of service lines and the defaults for each are unioned.
// See core/tenant for the underlying Tenant model this extends.
package servicelines

import "sort"

// ServiceLine describes one clinical service line in the catalogue.
type ServiceLine struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Modules are the module directory names (e.g. "tpt-hospital") this
	// service line's clinical workflows depend on.
	Modules []string `json:"modules"`
	// WardTypes are the hospital_wards.ward_type values (see
	// modules/tpt-hospital/db/migrate/002_wards.sql) sensible for this
	// service line. Empty for service lines with no ward concept.
	WardTypes []string `json:"wardTypes"`
	// TriageScale is the acuity/triage scale used by this service line,
	// e.g. "ATS" for Emergency Department. Empty if not applicable.
	TriageScale string `json:"triageScale,omitempty"`
	// FormularySubset are PHARMAC therapeutic-group tags relevant to this
	// service line, used to narrow the formulary shown to clinicians.
	FormularySubset []string `json:"formularySubset"`
}

// Catalogue IDs. These are the only valid values for a tenant's selected
// service lines and are stored verbatim in tenant_service_lines.service_line_id.
const (
	EmergencyDepartment    = "emergency_department"
	ICU                    = "icu"
	NICUPICU               = "nicu_picu"
	TheatreSurgical        = "theatre_surgical"
	Oncology               = "oncology"
	Maternity              = "maternity"
	MentalHealth           = "mental_health"
	GeneralMedicalSurgical = "general_medical_surgical"
	Outpatients            = "outpatients"
	AgedCare               = "aged_care"
	RenalDialysis          = "renal_dialysis"
	PalliativeCare         = "palliative_care"
	PharmacyDispensing     = "pharmacy_dispensing"
	Radiology              = "radiology"
	Pathology              = "pathology"
	PrimaryCare            = "primary_care"
)

// catalogue is the ordered, authoritative list of service lines. Order is
// display order, not significant for resolution.
var catalogue = []ServiceLine{
	{
		ID:              EmergencyDepartment,
		Name:            "Emergency Department",
		Description:     "Acute unplanned presentations, resuscitation, and short-stay observation.",
		Modules:         []string{"tpt-hospital"},
		WardTypes:       []string{"ed", "resus", "short_stay"},
		TriageScale:     "ATS",
		FormularySubset: []string{"emergency", "analgesia", "resuscitation"},
	},
	{
		ID:              ICU,
		Name:            "Intensive Care Unit",
		Description:     "Critical care for patients requiring organ support and continuous monitoring.",
		Modules:         []string{"tpt-hospital"},
		WardTypes:       []string{"icu"},
		FormularySubset: []string{"critical_care", "sedation", "vasopressors"},
	},
	{
		ID:              NICUPICU,
		Name:            "NICU / PICU",
		Description:     "Neonatal and paediatric intensive care.",
		Modules:         []string{"tpt-hospital", "tpt-maternal-child-health"},
		WardTypes:       []string{"nicu", "picu"},
		TriageScale:     "PAT",
		FormularySubset: []string{"neonatal", "paediatric_critical_care"},
	},
	{
		ID:              TheatreSurgical,
		Name:            "Theatre / Surgical Services",
		Description:     "Operating theatres, day surgery, and post-anaesthesia care.",
		Modules:         []string{"tpt-hospital"},
		WardTypes:       []string{"surgical_ward", "day_surgery", "pacu"},
		FormularySubset: []string{"anaesthesia", "surgical_prophylaxis"},
	},
	{
		ID:              Oncology,
		Name:            "Oncology",
		Description:     "Cancer diagnosis, chemotherapy, and oncology day-stay services.",
		Modules:         []string{"tpt-oncology"},
		WardTypes:       []string{"oncology_ward", "day_oncology"},
		FormularySubset: []string{"chemotherapy", "antiemetics", "palliative_symptom_control"},
	},
	{
		ID:              Maternity,
		Name:            "Maternity",
		Description:     "Antenatal, birthing, and postnatal maternity care.",
		Modules:         []string{"tpt-maternal-child-health"},
		WardTypes:       []string{"maternity", "postnatal", "nicu"},
		TriageScale:     "MEOWS",
		FormularySubset: []string{"obstetric", "neonatal"},
	},
	{
		ID:              MentalHealth,
		Name:            "Mental Health",
		Description:     "Acute and secure inpatient mental health care.",
		Modules:         []string{"tpt-mental-health"},
		WardTypes:       []string{"mental_health_acute", "mental_health_secure"},
		TriageScale:     "NMHTS",
		FormularySubset: []string{"psychotropic"},
	},
	{
		ID:              GeneralMedicalSurgical,
		Name:            "General Medical / Surgical Ward",
		Description:     "General adult inpatient wards.",
		Modules:         []string{"tpt-hospital"},
		WardTypes:       []string{"general"},
		FormularySubset: []string{"general_medical"},
	},
	{
		ID:              Outpatients,
		Name:            "Outpatients",
		Description:     "Scheduled outpatient clinics, including telehealth follow-up.",
		Modules:         []string{"tpt-doctor", "tpt-telehealth"},
		FormularySubset: []string{"general_medical"},
	},
	{
		ID:              AgedCare,
		Name:            "Aged Care",
		Description:     "Aged residential and community care, including dementia units.",
		Modules:         []string{"tpt-aged-care"},
		WardTypes:       []string{"arc_hospital", "arc_rest_home", "arc_dementia"},
		FormularySubset: []string{"geriatric"},
	},
	{
		ID:              RenalDialysis,
		Name:            "Renal / Dialysis",
		Description:     "Haemodialysis and renal outpatient services.",
		Modules:         []string{"tpt-renal"},
		WardTypes:       []string{"dialysis_unit"},
		FormularySubset: []string{"renal", "erythropoietin"},
	},
	{
		ID:              PalliativeCare,
		Name:            "Palliative Care",
		Description:     "Palliative and hospice care.",
		Modules:         []string{"tpt-palliative"},
		WardTypes:       []string{"palliative_ward", "hospice"},
		FormularySubset: []string{"palliative_symptom_control", "opioid"},
	},
	{
		ID:              PharmacyDispensing,
		Name:            "Pharmacy Dispensing",
		Description:     "On-site or affiliated dispensing pharmacy.",
		Modules:         []string{"tpt-pharmacy"},
		FormularySubset: []string{"full_formulary"},
	},
	{
		ID:              Radiology,
		Name:            "Radiology",
		Description:     "Diagnostic and interventional imaging.",
		Modules:         []string{"tpt-radiology"},
		FormularySubset: []string{"contrast_media"},
	},
	{
		ID:          Pathology,
		Name:        "Pathology",
		Description: "Laboratory and pathology services.",
		Modules:     []string{"tpt-pathology"},
	},
	{
		ID:              PrimaryCare,
		Name:            "Primary Care",
		Description:     "General practice / family medicine.",
		Modules:         []string{"tpt-doctor"},
		FormularySubset: []string{"general_medical"},
	},
}

var byID = func() map[string]ServiceLine {
	m := make(map[string]ServiceLine, len(catalogue))
	for _, sl := range catalogue {
		m[sl.ID] = sl
	}
	return m
}()

// All returns the full service-line catalogue in display order.
func All() []ServiceLine {
	out := make([]ServiceLine, len(catalogue))
	copy(out, catalogue)
	return out
}

// Lookup returns the catalogue entry for id, if it exists.
func Lookup(id string) (ServiceLine, bool) {
	sl, ok := byID[id]
	return sl, ok
}

// Valid reports whether id is a known service-line ID.
func Valid(id string) bool {
	_, ok := byID[id]
	return ok
}

// ResolveModules returns the deduplicated, sorted union of modules implied
// by the given service-line IDs. Unknown IDs are ignored.
func ResolveModules(ids []string) []string {
	return unionSorted(ids, func(sl ServiceLine) []string { return sl.Modules })
}

// ResolveWardTypes returns the deduplicated, sorted union of ward types
// implied by the given service-line IDs. Unknown IDs are ignored.
func ResolveWardTypes(ids []string) []string {
	return unionSorted(ids, func(sl ServiceLine) []string { return sl.WardTypes })
}

// ResolveFormularySubset returns the deduplicated, sorted union of formulary
// tags implied by the given service-line IDs. Unknown IDs are ignored.
func ResolveFormularySubset(ids []string) []string {
	return unionSorted(ids, func(sl ServiceLine) []string { return sl.FormularySubset })
}

// ResolveTriageScales returns the triage/acuity scale for each selected
// service line that defines one, keyed by service-line ID.
func ResolveTriageScales(ids []string) map[string]string {
	out := map[string]string{}
	for _, id := range ids {
		sl, ok := byID[id]
		if !ok || sl.TriageScale == "" {
			continue
		}
		out[id] = sl.TriageScale
	}
	return out
}

func unionSorted(ids []string, pick func(ServiceLine) []string) []string {
	seen := map[string]bool{}
	for _, id := range ids {
		sl, ok := byID[id]
		if !ok {
			continue
		}
		for _, v := range pick(sl) {
			seen[v] = true
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
