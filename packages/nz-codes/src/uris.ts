// NZ FHIR identifier system URIs and terminology code system URIs.
// All constants align with https://standards.digital.health.nz/ and the NZ Base IG.

// ---------------------------------------------------------------------------
// Base
// ---------------------------------------------------------------------------

/** Root base URI for all NZ health digital standards. */
export const NZ_BASE_URI = "https://standards.digital.health.nz/";

// ---------------------------------------------------------------------------
// Patient / Person Identifiers
// ---------------------------------------------------------------------------

/** National Health Index (NHI) identifier system. */
export const NHI_SYSTEM = "https://standards.digital.health.nz/ns/nhi-id";

/** NHI dormant (superseded) identifier system. */
export const NHI_DORMANT_SYSTEM =
  "https://standards.digital.health.nz/ns/nhi-id-dormant";

// ---------------------------------------------------------------------------
// Health Practitioner Index (HPI)
// ---------------------------------------------------------------------------

/** HPI Common Person Number (CPN) — uniquely identifies a health practitioner. */
export const HPI_CPN_SYSTEM =
  "https://standards.digital.health.nz/ns/hpi-person-id";

/** HPI Organisation ID — uniquely identifies a health organisation. */
export const HPI_ORG_SYSTEM =
  "https://standards.digital.health.nz/ns/hpi-organisation-id";

/** HPI Facility ID — uniquely identifies a health facility. */
export const HPI_FACILITY_SYSTEM =
  "https://standards.digital.health.nz/ns/hpi-facility-id";

/** HPI Registration Authority code system (MCNZ, NCNZ, Pharmacy Council, etc.). */
export const HPI_REGISTRATION_AUTHORITY_CODESYSTEM =
  "https://standards.digital.health.nz/ns/registration-authority-code";

/** HPI Scope of Practice code system. */
export const HPI_SCOPE_OF_PRACTICE_CODESYSTEM =
  "https://standards.digital.health.nz/ns/scope-of-practice-code";

/** HPI Annual Practising Certificate (APC) condition code system. */
export const HPI_APC_CONDITION_CODESYSTEM =
  "https://standards.digital.health.nz/ns/apc-condition-code";

// ---------------------------------------------------------------------------
// National Enrolment Service (NES)
// ---------------------------------------------------------------------------

/** NES enrolment ID system. */
export const NES_ENROLMENT_SYSTEM =
  "https://standards.digital.health.nz/ns/nes-enrolment-id";

/** NES enrolment status code system. */
export const NES_ENROLMENT_STATUS_CODESYSTEM =
  "https://standards.digital.health.nz/ns/nes-enrolment-status-code";

// ---------------------------------------------------------------------------
// ACC (Accident Compensation Corporation)
// ---------------------------------------------------------------------------

/** ACC claim identifier system. */
export const ACC_CLAIM_SYSTEM =
  "https://standards.digital.health.nz/ns/acc-claim-id";

/** ACC purchase order identifier system. */
export const ACC_PURCHASE_ORDER_SYSTEM =
  "https://standards.digital.health.nz/ns/acc-purchase-order-id";

/** ACC provider identifier system. */
export const ACC_PROVIDER_SYSTEM =
  "https://standards.digital.health.nz/ns/acc-provider-id";

/** ACC Read code system (ACC injury diagnosis codes). */
export const ACC_READ_CODESYSTEM =
  "https://standards.digital.health.nz/ns/acc-read-code";

/** ACC contract code system. */
export const ACC_CONTRACT_CODESYSTEM =
  "https://standards.digital.health.nz/ns/acc-contract-code";

// ---------------------------------------------------------------------------
// PHARMAC / Medicines
// ---------------------------------------------------------------------------

/** PHARMAC New Zealand Universal List of Medicines (NZULM / NZMT) identifier system. */
export const NZULM_SYSTEM = "https://standards.digital.health.nz/ns/nzmt-id";

/** NZMT (NZ Medicines Terminology) code system. */
export const NZMT_CODESYSTEM = "https://standards.digital.health.nz/ns/nzmt-code";

/** PHARMAC subsidy schedule code system. */
export const PHARMAC_SCHEDULE_CODESYSTEM =
  "https://standards.digital.health.nz/ns/pharmac-schedule-code";

// ---------------------------------------------------------------------------
// NZ Health Specialties
// ---------------------------------------------------------------------------

/** NZ Health Speciality code system (HISO 10072). */
export const NZ_HEALTH_SPECIALITY_CODESYSTEM =
  "https://standards.digital.health.nz/ns/health-speciality-code";

// ---------------------------------------------------------------------------
// Ethnicity and Iwi
// ---------------------------------------------------------------------------

/** NZ Ethnicity code system — Stats NZ Level 4 (most granular). */
export const NZ_ETHNICITY_LEVEL4_CODESYSTEM =
  "https://standards.digital.health.nz/ns/ethnic-group-level-4-code";

/** NZ Ethnicity code system — Stats NZ Level 2 (standard reporting). */
export const NZ_ETHNICITY_LEVEL2_CODESYSTEM =
  "https://standards.digital.health.nz/ns/ethnic-group-level-2-code";

/** NZ Iwi affiliation code system. */
export const NZ_IWI_CODESYSTEM =
  "https://standards.digital.health.nz/ns/iwi-code";

// ---------------------------------------------------------------------------
// DHB / Region
// ---------------------------------------------------------------------------

/** NZ District Health Board (DHB) code system (legacy pre-2022 Te Whatu Ora). */
export const NZ_DHB_CODESYSTEM =
  "https://standards.digital.health.nz/ns/dhb-code";

/** NZ Health New Zealand (Te Whatu Ora) commission region code system. */
export const NZ_COMMISSION_REGION_CODESYSTEM =
  "https://standards.digital.health.nz/ns/nz-health-region-code";

// ---------------------------------------------------------------------------
// Standard Terminology (international, used in NZ context)
// ---------------------------------------------------------------------------

/** SNOMED CT — NZ edition base URI. */
export const SNOMED_CT_SYSTEM = "http://snomed.info/sct";

/** LOINC code system. */
export const LOINC_SYSTEM = "http://loinc.org";

/** ICD-10-AM (Australian Modification, used in NZ). */
export const ICD10AM_SYSTEM =
  "https://standards.digital.health.nz/ns/icd-10-am-code";

/** ICD-10 (WHO edition). */
export const ICD10_SYSTEM = "http://hl7.org/fhir/sid/icd-10";

/** NZMT AMT (Australian Medicines Terminology, used as upstream for NZMT). */
export const AMT_SYSTEM = "http://snomed.info/sct"; // AMT is published as a SNOMED CT edition

// ---------------------------------------------------------------------------
// HL7 / FHIR Standard Code Systems
// ---------------------------------------------------------------------------

/** FHIR resource types code system. */
export const FHIR_RESOURCE_TYPES_SYSTEM =
  "http://hl7.org/fhir/resource-types";

/** FHIR identifier use code system. */
export const FHIR_IDENTIFIER_USE_SYSTEM =
  "http://hl7.org/fhir/identifier-use";

/** HL7 v3 MaritalStatus code system. */
export const HL7_MARITAL_STATUS_SYSTEM =
  "http://terminology.hl7.org/CodeSystem/v3-MaritalStatus";

/** HL7 v2 Table 0001 — Administrative Sex. */
export const HL7_ADMIN_GENDER_SYSTEM =
  "http://hl7.org/fhir/administrative-gender";

/** FHIR observation category code system. */
export const FHIR_OBSERVATION_CATEGORY_SYSTEM =
  "http://terminology.hl7.org/CodeSystem/observation-category";

/** FHIR condition clinical status code system. */
export const FHIR_CONDITION_CLINICAL_SYSTEM =
  "http://terminology.hl7.org/CodeSystem/condition-clinical";

/** FHIR condition verification status code system. */
export const FHIR_CONDITION_VERIFICATION_SYSTEM =
  "http://terminology.hl7.org/CodeSystem/condition-ver-status";

// ---------------------------------------------------------------------------
// NZ Extension URLs
// ---------------------------------------------------------------------------

/** NZ Ethnicity extension URL. */
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

/** NZ death date extension URL. */
export const NZ_DEATH_DATE_EXTENSION_URL =
  "http://hl7.org.nz/fhir/StructureDefinition/nz-death-date";

/** NZ preferred GP extension URL. */
export const NZ_PREFERRED_GP_EXTENSION_URL =
  "http://hl7.org.nz/fhir/StructureDefinition/nz-preferred-gp";

/** NZ GP enrolment extension URL. */
export const NZ_GP_ENROLMENT_EXTENSION_URL =
  "http://hl7.org.nz/fhir/StructureDefinition/enrolling-practice";

// ---------------------------------------------------------------------------
// Lookup helpers
// ---------------------------------------------------------------------------

/** All NZ identifier system URIs as a record for lookup/validation. */
export const NZ_IDENTIFIER_SYSTEMS = {
  nhi: NHI_SYSTEM,
  nhiDormant: NHI_DORMANT_SYSTEM,
  hpiCPN: HPI_CPN_SYSTEM,
  hpiOrg: HPI_ORG_SYSTEM,
  hpiFacility: HPI_FACILITY_SYSTEM,
  nesEnrolment: NES_ENROLMENT_SYSTEM,
  accClaim: ACC_CLAIM_SYSTEM,
  accPurchaseOrder: ACC_PURCHASE_ORDER_SYSTEM,
  accProvider: ACC_PROVIDER_SYSTEM,
  nzulm: NZULM_SYSTEM,
} as const;

export type NZIdentifierSystemKey = keyof typeof NZ_IDENTIFIER_SYSTEMS;
export type NZIdentifierSystemUri =
  (typeof NZ_IDENTIFIER_SYSTEMS)[NZIdentifierSystemKey];

/**
 * Returns true if the given URI is a known NZ identifier system URI.
 */
export function isNZIdentifierSystem(uri: string): uri is NZIdentifierSystemUri {
  return Object.values(NZ_IDENTIFIER_SYSTEMS).includes(
    uri as NZIdentifierSystemUri
  );
}
