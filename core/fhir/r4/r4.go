// Package r4 provides stub types for HL7 FHIR R4 resources.
//
// TODO: replace these type aliases with a full FHIR R4 struct library once
// one is selected (e.g. google/fhir or a generated SDK from the Te Whatu Ora
// FHIR capability statement).
package r4

// Patient represents a FHIR R4 Patient resource as a generic property bag.
// All standard FHIR JSON fields (resourceType, id, name, birthDate, gender,
// identifier, etc.) are accessible via map key lookups.
//
// TODO: replace with proper FHIR R4 typed struct.
type Patient = map[string]any
