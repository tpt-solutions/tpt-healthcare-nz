// NZ-specific FHIR extension URIs and helper utilities.
// Extension URLs align with https://standards.digital.health.nz/ and the NZ Base IG.

import type { Patient, Practitioner, Extension, Identifier } from "./r5.js";

// ---------------------------------------------------------------------------
// Base URI
// ---------------------------------------------------------------------------

export const NZ_BASE_URI = "https://standards.digital.health.nz/";

// ---------------------------------------------------------------------------
// Identifier Systems
// ---------------------------------------------------------------------------

/** National Health Index (NHI) identifier system. */
export const NHI_SYSTEM = "https://standards.digital.health.nz/ns/nhi-id";

/** Health Practitioner Index — Common Person Number (HPI CPN). */
export const HPI_CPN_SYSTEM = "https://standards.digital.health.nz/ns/hpi-person-id";

/** Health Practitioner Index — Organisation ID. */
export const HPI_ORG_SYSTEM = "https://standards.digital.health.nz/ns/hpi-organisation-id";

/** Health Practitioner Index — Facility ID. */
export const HPI_FACILITY_SYSTEM = "https://standards.digital.health.nz/ns/hpi-facility-id";

/** National Enrolment Service (NES) enrolment ID. */
export const NES_ENROLMENT_SYSTEM = "https://standards.digital.health.nz/ns/nes-enrolment-id";

/** ACC claim identifier system. */
export const ACC_CLAIM_SYSTEM = "https://standards.digital.health.nz/ns/acc-claim-id";

/** PHARMAC New Zealand Universal List of Medicines (NZULM) identifier system. */
export const NZULM_SYSTEM = "https://standards.digital.health.nz/ns/nzmt-id";

// ---------------------------------------------------------------------------
// Extension URLs
// ---------------------------------------------------------------------------

/** NZ Ethnicity extension URL (aligned with NZ Base IG). */
export const NZ_ETHNICITY_EXTENSION_URL =
  "http://hl7.org.nz/fhir/StructureDefinition/nz-ethnicity";

/** NZ Iwi affiliation extension URL. */
export const NZ_IWI_EXTENSION_URL =
  "http://hl7.org.nz/fhir/StructureDefinition/nz-iwi";

/** NZ citizenship status extension URL. */
export const NZ_CITIZENSHIP_EXTENSION_URL =
  "http://hl7.org.nz/fhir/StructureDefinition/nz-citizenship";

/** NZ residency status extension URL. */
export const NZ_RESIDENCY_EXTENSION_URL =
  "http://hl7.org.nz/fhir/StructureDefinition/nz-residency";

/** NZ Patient preferred GP extension URL. */
export const NZ_PREFERRED_GP_EXTENSION_URL =
  "http://hl7.org.nz/fhir/StructureDefinition/nz-preferred-gp";

/** Death date extension URL (NZ Te Whatu Ora supplement). */
export const NZ_DEATH_DATE_EXTENSION_URL =
  "http://hl7.org.nz/fhir/StructureDefinition/nz-death-date";

// ---------------------------------------------------------------------------
// Terminology / CodeSystem URIs
// ---------------------------------------------------------------------------

/** NZ Ethnicity code system (Stats NZ Level 4). */
export const NZ_ETHNICITY_CODESYSTEM =
  "https://standards.digital.health.nz/ns/ethnic-group-level-4-code";

/** NZ Iwi affiliation code system. */
export const NZ_IWI_CODESYSTEM =
  "https://standards.digital.health.nz/ns/iwi-code";

/** NZ Health Speciality code system (HISO 10072). */
export const NZ_HEALTH_SPECIALITY_CODESYSTEM =
  "https://standards.digital.health.nz/ns/health-speciality-code";

/** NZ DHB (District Health Board) code system. */
export const NZ_DHB_CODESYSTEM =
  "https://standards.digital.health.nz/ns/dhb-code";

/** NZ ACC Read code system. */
export const NZ_ACC_READ_CODESYSTEM =
  "https://standards.digital.health.nz/ns/acc-read-code";

/** NZ NZMT (NZ Medicines Terminology) code system. */
export const NZMT_CODESYSTEM =
  "https://standards.digital.health.nz/ns/nzmt-code";

/** SNOMED CT (NZ edition). */
export const SNOMED_CT_NZ_SYSTEM =
  "http://snomed.info/sct";

/** LOINC. */
export const LOINC_SYSTEM = "http://loinc.org";

/** ICD-10-AM. */
export const ICD10AM_SYSTEM =
  "https://standards.digital.health.nz/ns/icd-10-am-code";

// ---------------------------------------------------------------------------
// NZ Extension types
// ---------------------------------------------------------------------------

/** Typed shape of the NZ ethnicity extension value (valueCoding). */
export interface NZEthnicityExtension {
  url: typeof NZ_ETHNICITY_EXTENSION_URL;
  valueCoding: {
    system: typeof NZ_ETHNICITY_CODESYSTEM;
    code: string;
    display?: string;
  };
}

/** Typed shape of the NZ iwi extension value (valueCoding). */
export interface NZIwiExtension {
  url: typeof NZ_IWI_EXTENSION_URL;
  valueCoding: {
    system: typeof NZ_IWI_CODESYSTEM;
    code: string;
    display?: string;
  };
}

/** Typed shape of the NZ citizenship extension (complex extension). */
export interface NZCitizenshipExtension {
  url: typeof NZ_CITIZENSHIP_EXTENSION_URL;
  extension: Array<
    | { url: "status"; valueCodeableConcept: { coding: Array<{ system: string; code: string; display?: string }> } }
    | { url: "source"; valueCodeableConcept: { coding: Array<{ system: string; code: string; display?: string }> } }
  >;
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

/**
 * Returns the NHI identifier value from a Patient resource, or undefined if
 * the patient has no NHI identifier.
 *
 * The NHI is stored as an Identifier with system = NHI_SYSTEM
 * (https://standards.digital.health.nz/ns/nhi-id).
 */
export function getNHI(patient: Patient): string | undefined {
  if (!patient.identifier) return undefined;
  const nhiIdentifier = patient.identifier.find(
    (id: Identifier) => id.system === NHI_SYSTEM
  );
  return nhiIdentifier?.value;
}

/**
 * Returns the HPI CPN (Common Person Number) from a Practitioner resource,
 * or undefined if the practitioner has no HPI CPN identifier.
 *
 * The CPN is stored as an Identifier with system = HPI_CPN_SYSTEM
 * (https://standards.digital.health.nz/ns/hpi-person-id).
 */
export function getHPICPN(practitioner: Practitioner): string | undefined {
  if (!practitioner.identifier) return undefined;
  const cpnIdentifier = practitioner.identifier.find(
    (id: Identifier) => id.system === HPI_CPN_SYSTEM
  );
  return cpnIdentifier?.value;
}

/**
 * Returns all NZ ethnicity extensions from a Patient resource.
 * A patient may have multiple ethnicity entries per Stats NZ Level 4.
 */
export function getNZEthnicities(patient: Patient): NZEthnicityExtension[] {
  if (!patient.extension) return [];
  return patient.extension.filter(
    (ext: Extension): ext is NZEthnicityExtension =>
      ext.url === NZ_ETHNICITY_EXTENSION_URL
  ) as NZEthnicityExtension[];
}

/**
 * Returns all NZ iwi affiliation extensions from a Patient resource.
 */
export function getNZIwiAffiliations(patient: Patient): NZIwiExtension[] {
  if (!patient.extension) return [];
  return patient.extension.filter(
    (ext: Extension): ext is NZIwiExtension =>
      ext.url === NZ_IWI_EXTENSION_URL
  ) as NZIwiExtension[];
}
