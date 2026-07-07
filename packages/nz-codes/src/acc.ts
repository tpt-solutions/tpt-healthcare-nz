// ACC (Accident Compensation Corporation) codes and utilities for NZ healthcare.
// ACC claim types and diagnosis codes align with the ACC provider portal standards.

// ---------------------------------------------------------------------------
// ACC Form Numbers
// ---------------------------------------------------------------------------

/** ACC45 — General Medical Certificate (injury claim form). */
export const ACC45 = "ACC45";

/** ACC6 — Claim for Treatment Injury. */
export const ACC6 = "ACC6";

/** ACC18 — Accredited Employer Programme claim. */
export const ACC18 = "ACC18";

/** ACC2152 — Purchase Order for treatment services. */
export const ACC2152 = "ACC2152";

/** ACC32 — Claim for hearing loss. */
export const ACC32 = "ACC32";

// ---------------------------------------------------------------------------
// Identifier Systems
// ---------------------------------------------------------------------------

export {
  ACC_CLAIM_SYSTEM,
  ACC_PURCHASE_ORDER_SYSTEM,
  ACC_PROVIDER_SYSTEM,
} from "./uris.js";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** The ACC funding / claim form type. */
export type ACCFundingType = "ReadCode" | "ACC45" | "ACC6" | "ACC18";

/** ACC claim status as returned by the ACC claims API. */
export type ACCClaimStatus =
  | "lodged"
  | "registered"
  | "accepted"
  | "declined"
  | "suspended"
  | "closed";

/** Simplified representation of an ACC claim reference. */
export interface ACCClaimReference {
  claimNumber: string;
  formType: ACCFundingType;
  status?: ACCClaimStatus;
}

// ---------------------------------------------------------------------------
// ACC Read/Diagnosis Code Mappings
// ---------------------------------------------------------------------------

/**
 * Sample ACC diagnosis code mappings (ACC Read Code -> description).
 * These are a representative subset of the ACC Read Code schedule.
 * The full schedule is maintained by ACC and should be loaded from the
 * ACC provider portal or the PHARMAC formulary integration.
 */
export const ACC_DIAGNOSIS_CODES: Map<string, string> = new Map([
  ["S30", "Superficial injury of abdomen, lower back and pelvis"],
  ["S40", "Superficial injury of shoulder and upper arm"],
  ["S50", "Superficial injury of elbow and forearm"],
  ["S60", "Superficial injury of wrist and hand"],
  ["S70", "Superficial injury of hip and thigh"],
  ["S80", "Superficial injury of knee and lower leg"],
  ["S90", "Superficial injury of ankle and foot"],
  ["T14", "Injury of unspecified body region"],
  ["M54", "Dorsalgia (back pain)"],
  ["S93", "Dislocation, sprain and strain of joints and ligaments of ankle and foot"],
]);

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Returns the description for a given ACC Read/diagnosis code, or undefined
 * if the code is not in the sample mapping.
 */
export function getACCDiagnosisDescription(code: string): string | undefined {
  return ACC_DIAGNOSIS_CODES.get(code.toUpperCase());
}

/**
 * Returns true if the given string is a known ACC form number.
 */
export function isKnownACCFormNumber(
  form: string
): form is typeof ACC45 | typeof ACC6 | typeof ACC18 | typeof ACC2152 | typeof ACC32 {
  return [ACC45, ACC6, ACC18, ACC2152, ACC32].includes(form);
}
