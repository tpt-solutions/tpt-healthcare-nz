package servicelines

import "testing"

func TestAllReturnsCopy(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("expected non-empty catalogue")
	}
	all[0].Name = "mutated"
	if catalogue[0].Name == "mutated" {
		t.Fatal("All() must return a copy, not the underlying catalogue slice")
	}
}

func TestLookupAndValid(t *testing.T) {
	sl, ok := Lookup(EmergencyDepartment)
	if !ok {
		t.Fatal("expected emergency_department to be found")
	}
	if sl.TriageScale != "ATS" {
		t.Fatalf("expected ATS triage scale, got %q", sl.TriageScale)
	}
	if Valid("not_a_real_service_line") {
		t.Fatal("expected unknown ID to be invalid")
	}
	if !Valid(ICU) {
		t.Fatal("expected icu to be valid")
	}
}

func TestResolveModulesUnionsAndDedupes(t *testing.T) {
	// ED and general medical/surgical both imply tpt-hospital; NICU/PICU
	// additionally implies tpt-maternal-child-health.
	got := ResolveModules([]string{EmergencyDepartment, GeneralMedicalSurgical, NICUPICU})
	want := map[string]bool{"tpt-hospital": true, "tpt-maternal-child-health": true}
	if len(got) != len(want) {
		t.Fatalf("expected %d modules, got %v", len(want), got)
	}
	for _, m := range got {
		if !want[m] {
			t.Fatalf("unexpected module %q in %v", m, got)
		}
	}
}

func TestResolveModulesIgnoresUnknownIDs(t *testing.T) {
	got := ResolveModules([]string{"bogus", PharmacyDispensing})
	if len(got) != 1 || got[0] != "tpt-pharmacy" {
		t.Fatalf("expected only tpt-pharmacy, got %v", got)
	}
}

func TestResolveWardTypesUnion(t *testing.T) {
	got := ResolveWardTypes([]string{ICU, NICUPICU})
	want := map[string]bool{"icu": true, "nicu": true, "picu": true}
	if len(got) != len(want) {
		t.Fatalf("expected %d ward types, got %v", len(want), got)
	}
	for _, w := range got {
		if !want[w] {
			t.Fatalf("unexpected ward type %q", w)
		}
	}
}

func TestResolveTriageScalesOnlyIncludesDefined(t *testing.T) {
	// ICU has no triage scale; ED does.
	got := ResolveTriageScales([]string{EmergencyDepartment, ICU})
	if len(got) != 1 {
		t.Fatalf("expected exactly one triage scale, got %v", got)
	}
	if got[EmergencyDepartment] != "ATS" {
		t.Fatalf("expected ATS for emergency_department, got %q", got[EmergencyDepartment])
	}
	if _, ok := got[ICU]; ok {
		t.Fatal("icu should not have a triage scale entry")
	}
}

func TestValidateIDs(t *testing.T) {
	if err := ValidateIDs([]string{EmergencyDepartment, ICU}); err != nil {
		t.Fatalf("expected valid IDs to pass, got %v", err)
	}
	if err := ValidateIDs([]string{"nope"}); err == nil {
		t.Fatal("expected error for unknown service line")
	}
	if err := ValidateIDs([]string{ICU, ICU}); err == nil {
		t.Fatal("expected error for duplicate service line")
	}
}
