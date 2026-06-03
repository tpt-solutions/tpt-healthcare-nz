// Minimal FHIR R4 types for NHI/NES API compatibility.
// For full R5 types use r5.ts. Translation between R4 and R5 is handled
// server-side in core/fhir/translate/.

export type R4FHIRDateTime = string;
export type R4FHIRDate = string;
export type R4FHIRInstant = string;
export type R4FHIRCode = string;
export type R4FHIRUri = string;
export type R4FHIROId = string;
export type R4FHIRMarkdown = string;
export type R4FHIRBase64Binary = string;
export type R4FHIRPositiveInt = number;
export type R4FHIRUnsignedInt = number;
export type R4FHIRDecimal = number;
export type R4FHIRBoolean = boolean;

export interface R4Extension {
  url: R4FHIRUri;
  valueString?: string;
  valueCode?: R4FHIRCode;
  valueBoolean?: R4FHIRBoolean;
  valueCoding?: R4Coding;
  valueCodeableConcept?: R4CodeableConcept;
  valueReference?: R4Reference;
  valueIdentifier?: R4Identifier;
  extension?: R4Extension[];
}

export interface R4Meta {
  versionId?: R4FHIROId;
  lastUpdated?: R4FHIRInstant;
  source?: R4FHIRUri;
  profile?: string[];
  tag?: R4Coding[];
  security?: R4Coding[];
}

export interface R4Coding {
  system?: R4FHIRUri;
  version?: string;
  code?: R4FHIRCode;
  display?: string;
  userSelected?: R4FHIRBoolean;
}

export interface R4CodeableConcept {
  coding?: R4Coding[];
  text?: string;
}

export interface R4Identifier {
  use?: "usual" | "official" | "temp" | "secondary" | "old";
  type?: R4CodeableConcept;
  system?: R4FHIRUri;
  value?: string;
  period?: R4Period;
  assigner?: R4Reference;
}

export interface R4HumanName {
  use?: "usual" | "official" | "temp" | "nickname" | "anonymous" | "old" | "maiden";
  text?: string;
  family?: string;
  given?: string[];
  prefix?: string[];
  suffix?: string[];
  period?: R4Period;
}

export interface R4Address {
  use?: "home" | "work" | "temp" | "old" | "billing";
  type?: "postal" | "physical" | "both";
  text?: string;
  line?: string[];
  city?: string;
  district?: string;
  state?: string;
  postalCode?: string;
  country?: string;
  period?: R4Period;
}

export interface R4ContactPoint {
  system?: "phone" | "fax" | "email" | "pager" | "url" | "sms" | "other";
  value?: string;
  use?: "home" | "work" | "temp" | "old" | "mobile";
  rank?: R4FHIRPositiveInt;
  period?: R4Period;
}

export interface R4Reference {
  reference?: string;
  type?: R4FHIRUri;
  identifier?: R4Identifier;
  display?: string;
}

export interface R4Period {
  start?: R4FHIRDateTime;
  end?: R4FHIRDateTime;
}

export interface R4Quantity {
  value?: R4FHIRDecimal;
  comparator?: "<" | "<=" | ">=" | ">";
  unit?: string;
  system?: R4FHIRUri;
  code?: R4FHIRCode;
}

export interface R4Narrative {
  status: "generated" | "extensions" | "additional" | "empty";
  div: string;
}

// ---------------------------------------------------------------------------
// R4 Resources
// ---------------------------------------------------------------------------

export interface R4ResourceBase {
  resourceType: string;
  id?: string;
  meta?: R4Meta;
  implicitRules?: R4FHIRUri;
  language?: R4FHIRCode;
}

export interface R4DomainResource extends R4ResourceBase {
  text?: R4Narrative;
  contained?: R4ResourceBase[];
  extension?: R4Extension[];
  modifierExtension?: R4Extension[];
}

/** Minimal FHIR R4 Patient — sufficient for NHI API interactions. */
export interface R4Patient extends R4DomainResource {
  resourceType: "Patient";
  identifier?: R4Identifier[];
  active?: R4FHIRBoolean;
  name?: R4HumanName[];
  telecom?: R4ContactPoint[];
  gender?: "male" | "female" | "other" | "unknown";
  birthDate?: R4FHIRDate;
  deceasedBoolean?: R4FHIRBoolean;
  deceasedDateTime?: R4FHIRDateTime;
  address?: R4Address[];
  maritalStatus?: R4CodeableConcept;
  multipleBirthBoolean?: R4FHIRBoolean;
  multipleBirthInteger?: number;
  contact?: Array<{
    relationship?: R4CodeableConcept[];
    name?: R4HumanName;
    telecom?: R4ContactPoint[];
    address?: R4Address;
    gender?: "male" | "female" | "other" | "unknown";
    organization?: R4Reference;
    period?: R4Period;
  }>;
  communication?: Array<{ language: R4CodeableConcept; preferred?: R4FHIRBoolean }>;
  generalPractitioner?: R4Reference[];
  managingOrganization?: R4Reference;
  link?: Array<{ other: R4Reference; type: "replaced-by" | "replaces" | "refer" | "seealso" }>;
}

/** Minimal FHIR R4 Practitioner — sufficient for HPI API interactions. */
export interface R4Practitioner extends R4DomainResource {
  resourceType: "Practitioner";
  identifier?: R4Identifier[];
  active?: R4FHIRBoolean;
  name?: R4HumanName[];
  telecom?: R4ContactPoint[];
  address?: R4Address[];
  gender?: "male" | "female" | "other" | "unknown";
  birthDate?: R4FHIRDate;
  qualification?: Array<{
    identifier?: R4Identifier[];
    code: R4CodeableConcept;
    period?: R4Period;
    issuer?: R4Reference;
  }>;
  communication?: R4CodeableConcept[];
}

/** Minimal FHIR R4 Bundle. */
export interface R4Bundle extends R4ResourceBase {
  resourceType: "Bundle";
  identifier?: R4Identifier;
  type:
    | "document"
    | "message"
    | "transaction"
    | "transaction-response"
    | "batch"
    | "batch-response"
    | "history"
    | "searchset"
    | "collection";
  timestamp?: R4FHIRInstant;
  total?: R4FHIRUnsignedInt;
  link?: Array<{ relation: string; url: R4FHIRUri }>;
  entry?: Array<{
    fullUrl?: R4FHIRUri;
    resource?: R4ResourceBase;
    search?: { mode?: "match" | "include" | "outcome"; score?: R4FHIRDecimal };
    request?: { method: "GET" | "HEAD" | "POST" | "PUT" | "DELETE" | "PATCH"; url: R4FHIRUri };
    response?: { status: string; location?: R4FHIRUri; etag?: string; lastModified?: R4FHIRInstant };
  }>;
}

/** Minimal FHIR R4 OperationOutcome. */
export interface R4OperationOutcome extends R4DomainResource {
  resourceType: "OperationOutcome";
  issue: Array<{
    severity: "fatal" | "error" | "warning" | "information";
    code: string;
    details?: R4CodeableConcept;
    diagnostics?: string;
    location?: string[];
    expression?: string[];
  }>;
}
