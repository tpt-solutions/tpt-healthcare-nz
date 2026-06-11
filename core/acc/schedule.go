// Package acc — schedule.go provides per-discipline ACC treatment schedules,
// including funded procedure codes and maximum session caps that vary by
// treatment provider type under the ACC Treatment Provider Agreements.
package acc

import "fmt"

// Discipline identifies the allied-health treatment type for ACC scheduling.
type Discipline string

const (
	DisciplineAcupuncture  Discipline = "acupuncture"
	DisciplineChiropractic Discipline = "chiropractic"
	DisciplineMassage      Discipline = "massage"
	DisciplinePhysiotherapy Discipline = "physiotherapy"
	DisciplineOsteopathy   Discipline = "osteopathy"
	DisciplineCounselling  Discipline = "counselling"
)

// ProcedureCode is a 4-digit ACC treatment schedule procedure code.
type ProcedureCode string

const (
	// Acupuncture — Treatment Provider Agreement schedule codes.
	CodeAcupunctureInitial     ProcedureCode = "1720" // Initial assessment + treatment
	CodeAcupunctureSubsequent  ProcedureCode = "1721" // Subsequent treatment

	// Chiropractic — schedule codes.
	CodeChiropracticInitial    ProcedureCode = "1410" // Initial consultation
	CodeChiropracticSubsequent ProcedureCode = "1411" // Subsequent treatment

	// Massage therapy — schedule codes.
	CodeMassageInitial         ProcedureCode = "1310" // Initial assessment
	CodeMassageSubsequent      ProcedureCode = "1311" // Subsequent treatment

	// Physiotherapy — schedule codes (most common).
	CodePhysioInitial          ProcedureCode = "1200" // Initial physiotherapy assessment
	CodePhysioSubsequent       ProcedureCode = "1201" // Subsequent treatment
	CodePhysioGroup            ProcedureCode = "1205" // Group treatment session

	// Osteopathy — schedule codes.
	CodeOsteopathyInitial      ProcedureCode = "1510" // Initial consultation
	CodeOsteopathySubsequent   ProcedureCode = "1511" // Subsequent treatment

	// Counselling — schedule codes.
	CodeCounsellingInitial     ProcedureCode = "2103" // Initial counselling session
	CodeCounsellingSubsequent  ProcedureCode = "2104" // Subsequent counselling session
)

// SessionCap defines the ACC-funded session limits and initial approval windows
// for a discipline. These are the standard caps under the ACC Treatment Provider
// Agreements; individual purchase orders may carry different limits.
type SessionCap struct {
	// InitialGranted is the number of sessions funded without a purchase order
	// (pre-approved entitlement for most disciplines).
	InitialGranted int
	// MaxWithExtension is the total sessions available if a PO extension is granted.
	MaxWithExtension int
	// RequiresPO indicates whether any treatment requires a purchase order first.
	RequiresPO bool
}

// sessionCaps maps each discipline to its standard ACC session cap.
var sessionCaps = map[Discipline]SessionCap{
	DisciplineAcupuncture: {
		InitialGranted:   16,
		MaxWithExtension: 32,
		RequiresPO:       false,
	},
	DisciplineChiropractic: {
		InitialGranted:   16,
		MaxWithExtension: 32,
		RequiresPO:       false,
	},
	DisciplineMassage: {
		InitialGranted:   16,
		MaxWithExtension: 32,
		RequiresPO:       false,
	},
	DisciplinePhysiotherapy: {
		InitialGranted:   16,
		MaxWithExtension: 60,
		RequiresPO:       false,
	},
	DisciplineOsteopathy: {
		InitialGranted:   16,
		MaxWithExtension: 32,
		RequiresPO:       false,
	},
	DisciplineCounselling: {
		InitialGranted:   0,
		MaxWithExtension: 16,
		RequiresPO:       true, // Counselling always requires a PO from ACC.
	},
}

// disciplineCodes maps each discipline to its initial and subsequent procedure codes.
var disciplineCodes = map[Discipline][2]ProcedureCode{
	DisciplineAcupuncture:   {CodeAcupunctureInitial, CodeAcupunctureSubsequent},
	DisciplineChiropractic:  {CodeChiropracticInitial, CodeChiropracticSubsequent},
	DisciplineMassage:       {CodeMassageInitial, CodeMassageSubsequent},
	DisciplinePhysiotherapy: {CodePhysioInitial, CodePhysioSubsequent},
	DisciplineOsteopathy:    {CodeOsteopathyInitial, CodeOsteopathySubsequent},
	DisciplineCounselling:   {CodeCounsellingInitial, CodeCounsellingSubsequent},
}

// SessionCapFor returns the standard ACC session cap for a given discipline.
// Returns an error if the discipline is not registered.
func SessionCapFor(d Discipline) (SessionCap, error) {
	cap, ok := sessionCaps[d]
	if !ok {
		return SessionCap{}, fmt.Errorf("acc: unknown discipline %q", d)
	}
	return cap, nil
}

// CodesFor returns the [initialCode, subsequentCode] pair for a discipline.
// Returns an error if the discipline is not registered.
func CodesFor(d Discipline) ([2]ProcedureCode, error) {
	codes, ok := disciplineCodes[d]
	if !ok {
		return [2]ProcedureCode{}, fmt.Errorf("acc: unknown discipline %q", d)
	}
	return codes, nil
}

// ProcedureCodeFor selects the correct procedure code for a session.
// isInitial should be true for the first session on a claim, false thereafter.
func ProcedureCodeFor(d Discipline, isInitial bool) (ProcedureCode, error) {
	codes, err := CodesFor(d)
	if err != nil {
		return "", err
	}
	if isInitial {
		return codes[0], nil
	}
	return codes[1], nil
}
