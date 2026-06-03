// FHIR R5 TypeScript types matching the Go structs in core/fhir/r5/
// Do not hand-edit — keep in sync with tools/gen-fhir-types output.

// ---------------------------------------------------------------------------
// Base primitive aliases
// ---------------------------------------------------------------------------

export type FHIRDateTime = string;
export type FHIRDate = string;
export type FHIRInstant = string;
export type FHIRTime = string;
export type FHIRCode = string;
export type FHIRUri = string;
export type FHIRUrl = string;
export type FHIRCanonical = string;
export type FHIROID = string;
export type FHIRMarkdown = string;
export type FHIRBase64Binary = string;
export type FHIRPositiveInt = number;
export type FHIRUnsignedInt = number;
export type FHIRDecimal = number;
export type FHIRInteger = number;
export type FHIRBoolean = boolean;

// ---------------------------------------------------------------------------
// Base types
// ---------------------------------------------------------------------------

export interface Extension {
  url: FHIRUri;
  valueString?: string;
  valueCode?: FHIRCode;
  valueBoolean?: FHIRBoolean;
  valueInteger?: FHIRInteger;
  valueDecimal?: FHIRDecimal;
  valueDateTime?: FHIRDateTime;
  valueDate?: FHIRDate;
  valueCoding?: Coding;
  valueCodeableConcept?: CodeableConcept;
  valueReference?: Reference;
  valueIdentifier?: Identifier;
  valuePeriod?: Period;
  valueQuantity?: Quantity;
  extension?: Extension[];
}

export interface Meta {
  versionId?: FHIROID;
  lastUpdated?: FHIRInstant;
  source?: FHIRUri;
  profile?: FHIRCanonical[];
  security?: Coding[];
  tag?: Coding[];
  extension?: Extension[];
}

export interface Narrative {
  status: "generated" | "extensions" | "additional" | "empty";
  div: string;
}

export interface Coding {
  system?: FHIRUri;
  version?: string;
  code?: FHIRCode;
  display?: string;
  userSelected?: FHIRBoolean;
  extension?: Extension[];
}

export interface CodeableConcept {
  coding?: Coding[];
  text?: string;
  extension?: Extension[];
}

export interface Identifier {
  use?: "usual" | "official" | "temp" | "secondary" | "old";
  type?: CodeableConcept;
  system?: FHIRUri;
  value?: string;
  period?: Period;
  assigner?: Reference;
  extension?: Extension[];
}

export interface HumanName {
  use?: "usual" | "official" | "temp" | "nickname" | "anonymous" | "old" | "maiden";
  text?: string;
  family?: string;
  given?: string[];
  prefix?: string[];
  suffix?: string[];
  period?: Period;
  extension?: Extension[];
}

export interface Address {
  use?: "home" | "work" | "temp" | "old" | "billing";
  type?: "postal" | "physical" | "both";
  text?: string;
  line?: string[];
  city?: string;
  district?: string;
  state?: string;
  postalCode?: string;
  country?: string;
  period?: Period;
  extension?: Extension[];
}

export interface ContactPoint {
  system?: "phone" | "fax" | "email" | "pager" | "url" | "sms" | "other";
  value?: string;
  use?: "home" | "work" | "temp" | "old" | "mobile";
  rank?: FHIRPositiveInt;
  period?: Period;
  extension?: Extension[];
}

export interface Reference {
  reference?: string;
  type?: FHIRUri;
  identifier?: Identifier;
  display?: string;
  extension?: Extension[];
}

export interface Period {
  start?: FHIRDateTime;
  end?: FHIRDateTime;
  extension?: Extension[];
}

export interface Quantity {
  value?: FHIRDecimal;
  comparator?: "<" | "<=" | ">=" | ">" | "ad";
  unit?: string;
  system?: FHIRUri;
  code?: FHIRCode;
  extension?: Extension[];
}

export interface Annotation {
  authorReference?: Reference;
  authorString?: string;
  time?: FHIRDateTime;
  text: FHIRMarkdown;
  extension?: Extension[];
}

// ---------------------------------------------------------------------------
// Domain Resources
// ---------------------------------------------------------------------------

export interface ResourceBase {
  resourceType: string;
  id?: FHIROID;
  meta?: Meta;
  implicitRules?: FHIRUri;
  language?: FHIRCode;
}

export interface DomainResource extends ResourceBase {
  text?: Narrative;
  contained?: ResourceBase[];
  extension?: Extension[];
  modifierExtension?: Extension[];
}

// ---------------------------------------------------------------------------
// Patient
// ---------------------------------------------------------------------------

export interface PatientContact {
  relationship?: CodeableConcept[];
  name?: HumanName;
  telecom?: ContactPoint[];
  address?: Address;
  gender?: AdministrativeGender;
  organization?: Reference;
  period?: Period;
  extension?: Extension[];
}

export interface PatientCommunication {
  language: CodeableConcept;
  preferred?: FHIRBoolean;
  extension?: Extension[];
}

export type AdministrativeGender = "male" | "female" | "other" | "unknown";

/** FHIR R5 Patient with NZ extensions (NZETHNIC, iwi affiliation). */
export interface Patient extends DomainResource {
  resourceType: "Patient";
  identifier?: Identifier[];
  active?: FHIRBoolean;
  name?: HumanName[];
  telecom?: ContactPoint[];
  gender?: AdministrativeGender;
  birthDate?: FHIRDate;
  deceasedBoolean?: FHIRBoolean;
  deceasedDateTime?: FHIRDateTime;
  address?: Address[];
  maritalStatus?: CodeableConcept;
  multipleBirthBoolean?: FHIRBoolean;
  multipleBirthInteger?: FHIRInteger;
  contact?: PatientContact[];
  communication?: PatientCommunication[];
  generalPractitioner?: Reference[];
  managingOrganization?: Reference;
  link?: Array<{
    other: Reference;
    type: "replaced-by" | "replaces" | "refer" | "seealso";
    extension?: Extension[];
  }>;
  // NZ-specific extensions are carried in extension[] using URLs from nz-extensions.ts
}

// ---------------------------------------------------------------------------
// Practitioner
// ---------------------------------------------------------------------------

export interface PractitionerQualification {
  identifier?: Identifier[];
  code: CodeableConcept;
  period?: Period;
  issuer?: Reference;
  extension?: Extension[];
}

export interface Practitioner extends DomainResource {
  resourceType: "Practitioner";
  identifier?: Identifier[];
  active?: FHIRBoolean;
  name?: HumanName[];
  telecom?: ContactPoint[];
  gender?: AdministrativeGender;
  birthDate?: FHIRDate;
  address?: Address[];
  qualification?: PractitionerQualification[];
  communication?: PatientCommunication[];
}

// ---------------------------------------------------------------------------
// Encounter
// ---------------------------------------------------------------------------

export type EncounterStatus =
  | "planned"
  | "in-progress"
  | "on-hold"
  | "discharged"
  | "completed"
  | "cancelled"
  | "discontinued"
  | "entered-in-error"
  | "unknown";

export interface EncounterParticipant {
  type?: CodeableConcept[];
  period?: Period;
  actor?: Reference;
  extension?: Extension[];
}

export interface EncounterDiagnosis {
  condition?: CodeableConcept[];
  use?: CodeableConcept[];
  extension?: Extension[];
}

export interface Encounter extends DomainResource {
  resourceType: "Encounter";
  identifier?: Identifier[];
  status: EncounterStatus;
  class?: CodeableConcept[];
  priority?: CodeableConcept;
  type?: CodeableConcept[];
  serviceType?: Reference[];
  subject?: Reference;
  subjectStatus?: CodeableConcept;
  episodeOfCare?: Reference[];
  basedOn?: Reference[];
  careTeam?: Reference[];
  partOf?: Reference;
  serviceProvider?: Reference;
  participant?: EncounterParticipant[];
  appointment?: Reference[];
  virtualService?: Array<{ channelType?: Coding; address?: string; extension?: Extension[] }>;
  actualPeriod?: Period;
  plannedStartDate?: FHIRDateTime;
  plannedEndDate?: FHIRDateTime;
  length?: Quantity;
  reason?: Array<{ use?: CodeableConcept[]; value?: CodeableConcept[]; extension?: Extension[] }>;
  diagnosis?: EncounterDiagnosis[];
  account?: Reference[];
  dietPreference?: CodeableConcept[];
  specialArrangement?: CodeableConcept[];
  specialCourtesy?: CodeableConcept[];
  admission?: {
    preAdmissionIdentifier?: Identifier;
    origin?: Reference;
    admitSource?: CodeableConcept;
    reAdmission?: CodeableConcept;
    destination?: Reference;
    dischargeDisposition?: CodeableConcept;
    extension?: Extension[];
  };
  location?: Array<{
    location: Reference;
    status?: "planned" | "active" | "reserved" | "completed";
    form?: CodeableConcept;
    period?: Period;
    extension?: Extension[];
  }>;
}

// ---------------------------------------------------------------------------
// Observation
// ---------------------------------------------------------------------------

export type ObservationStatus =
  | "registered"
  | "preliminary"
  | "final"
  | "amended"
  | "corrected"
  | "cancelled"
  | "entered-in-error"
  | "unknown";

export interface ObservationComponent {
  code: CodeableConcept;
  valueQuantity?: Quantity;
  valueCodeableConcept?: CodeableConcept;
  valueString?: string;
  valueBoolean?: FHIRBoolean;
  valueInteger?: FHIRInteger;
  valueDateTime?: FHIRDateTime;
  dataAbsentReason?: CodeableConcept;
  interpretation?: CodeableConcept[];
  referenceRange?: ObservationReferenceRange[];
  extension?: Extension[];
}

export interface ObservationReferenceRange {
  low?: Quantity;
  high?: Quantity;
  normalValue?: CodeableConcept;
  type?: CodeableConcept;
  appliesTo?: CodeableConcept[];
  age?: { low?: Quantity; high?: Quantity };
  text?: FHIRMarkdown;
  extension?: Extension[];
}

export interface Observation extends DomainResource {
  resourceType: "Observation";
  identifier?: Identifier[];
  instantiatesCanonical?: FHIRCanonical;
  basedOn?: Reference[];
  triggeredBy?: Array<{ observation: Reference; type: "reflex" | "repeat" | "re-run"; reason?: string; extension?: Extension[] }>;
  partOf?: Reference[];
  status: ObservationStatus;
  category?: CodeableConcept[];
  code: CodeableConcept;
  subject?: Reference;
  focus?: Reference[];
  encounter?: Reference;
  effectiveDateTime?: FHIRDateTime;
  effectivePeriod?: Period;
  effectiveTiming?: { event?: FHIRDateTime[]; repeat?: unknown; code?: CodeableConcept };
  effectiveInstant?: FHIRInstant;
  issued?: FHIRInstant;
  performer?: Reference[];
  valueQuantity?: Quantity;
  valueCodeableConcept?: CodeableConcept;
  valueString?: string;
  valueBoolean?: FHIRBoolean;
  valueInteger?: FHIRInteger;
  valueDateTime?: FHIRDateTime;
  dataAbsentReason?: CodeableConcept;
  interpretation?: CodeableConcept[];
  note?: Annotation[];
  bodySite?: CodeableConcept;
  bodyStructure?: Reference;
  method?: CodeableConcept;
  specimen?: Reference;
  device?: Reference;
  referenceRange?: ObservationReferenceRange[];
  hasMember?: Reference[];
  derivedFrom?: Reference[];
  component?: ObservationComponent[];
}

// ---------------------------------------------------------------------------
// Condition
// ---------------------------------------------------------------------------

export type ConditionVerificationStatus = "unconfirmed" | "provisional" | "differential" | "confirmed" | "refuted" | "entered-in-error";

export interface Condition extends DomainResource {
  resourceType: "Condition";
  identifier?: Identifier[];
  clinicalStatus?: CodeableConcept;
  verificationStatus?: CodeableConcept;
  category?: CodeableConcept[];
  severity?: CodeableConcept;
  code?: CodeableConcept;
  bodySite?: CodeableConcept[];
  subject: Reference;
  encounter?: Reference;
  onsetDateTime?: FHIRDateTime;
  onsetAge?: Quantity;
  onsetPeriod?: Period;
  onsetRange?: { low?: Quantity; high?: Quantity };
  onsetString?: string;
  abatementDateTime?: FHIRDateTime;
  abatementAge?: Quantity;
  abatementPeriod?: Period;
  abatementRange?: { low?: Quantity; high?: Quantity };
  abatementString?: string;
  recordedDate?: FHIRDateTime;
  participant?: Array<{ function?: CodeableConcept; actor: Reference; extension?: Extension[] }>;
  stage?: Array<{
    summary?: CodeableConcept;
    assessment?: Reference[];
    type?: CodeableConcept;
    extension?: Extension[];
  }>;
  evidence?: Array<{ concept?: CodeableConcept[]; reference?: Reference[]; extension?: Extension[] }>;
  note?: Annotation[];
}

// ---------------------------------------------------------------------------
// MedicationRequest
// ---------------------------------------------------------------------------

export type MedicationRequestStatus =
  | "active"
  | "on-hold"
  | "ended"
  | "stopped"
  | "completed"
  | "cancelled"
  | "entered-in-error"
  | "draft"
  | "unknown";

export interface MedicationRequest extends DomainResource {
  resourceType: "MedicationRequest";
  identifier?: Identifier[];
  basedOn?: Reference[];
  priorPrescription?: Reference;
  groupIdentifier?: Identifier;
  status: MedicationRequestStatus;
  statusReason?: CodeableConcept;
  statusChanged?: FHIRDateTime;
  intent: "proposal" | "plan" | "order" | "original-order" | "reflex-order" | "filler-order" | "instance-order" | "option";
  category?: CodeableConcept[];
  priority?: "routine" | "urgent" | "asap" | "stat";
  doNotPerform?: FHIRBoolean;
  medication: CodeableConcept | Reference;
  subject: Reference;
  informationSource?: Reference[];
  encounter?: Reference;
  supportingInformation?: Reference[];
  authoredOn?: FHIRDateTime;
  requester?: Reference;
  reported?: FHIRBoolean;
  performerType?: CodeableConcept;
  performer?: Reference[];
  device?: CodeableConcept[];
  recorder?: Reference;
  reason?: Array<{ concept?: CodeableConcept; reference?: Reference; extension?: Extension[] }>;
  courseOfTherapyType?: CodeableConcept;
  insurance?: Reference[];
  note?: Annotation[];
  renderedDosageInstruction?: FHIRMarkdown;
  effectiveDosePeriod?: Period;
  dosageInstruction?: Array<{
    sequence?: FHIRInteger;
    text?: string;
    additionalInstruction?: CodeableConcept[];
    patientInstruction?: string;
    timing?: unknown;
    asNeeded?: FHIRBoolean;
    asNeededFor?: CodeableConcept;
    site?: CodeableConcept;
    route?: CodeableConcept;
    method?: CodeableConcept;
    doseAndRate?: Array<{ type?: CodeableConcept; doseQuantity?: Quantity; rateQuantity?: Quantity; extension?: Extension[] }>;
    maxDosePerPeriod?: Array<{ numerator?: Quantity; denominator?: Quantity }>;
    maxDosePerAdministration?: Quantity;
    maxDosePerLifetime?: Quantity;
    extension?: Extension[];
  }>;
  dispenseRequest?: {
    initialFill?: { quantity?: Quantity; duration?: Quantity; extension?: Extension[] };
    dispenseInterval?: Quantity;
    validityPeriod?: Period;
    numberOfRepeatsAllowed?: FHIRUnsignedInt;
    quantity?: Quantity;
    expectedSupplyDuration?: Quantity;
    dispenser?: Reference;
    dispenserInstruction?: Annotation[];
    doseAdministrationAid?: CodeableConcept;
    extension?: Extension[];
  };
  substitution?: {
    allowedBoolean?: FHIRBoolean;
    allowedCodeableConcept?: CodeableConcept;
    reason?: CodeableConcept;
    extension?: Extension[];
  };
  eventHistory?: Reference[];
}

// ---------------------------------------------------------------------------
// DiagnosticReport
// ---------------------------------------------------------------------------

export type DiagnosticReportStatus =
  | "registered"
  | "partial"
  | "preliminary"
  | "modified"
  | "final"
  | "amended"
  | "corrected"
  | "appended"
  | "cancelled"
  | "entered-in-error"
  | "unknown";

export interface DiagnosticReport extends DomainResource {
  resourceType: "DiagnosticReport";
  identifier?: Identifier[];
  basedOn?: Reference[];
  status: DiagnosticReportStatus;
  category?: CodeableConcept[];
  code: CodeableConcept;
  subject?: Reference;
  encounter?: Reference;
  effectiveDateTime?: FHIRDateTime;
  effectivePeriod?: Period;
  issued?: FHIRInstant;
  performer?: Reference[];
  resultsInterpreter?: Reference[];
  specimen?: Reference[];
  result?: Reference[];
  note?: Annotation[];
  study?: Reference[];
  supportingInfo?: Array<{ type: CodeableConcept; reference: Reference; extension?: Extension[] }>;
  media?: Array<{ comment?: string; link: Reference; extension?: Extension[] }>;
  composition?: Reference;
  conclusion?: FHIRMarkdown;
  conclusionCode?: CodeableConcept[];
  presentedForm?: Array<{
    contentType?: FHIRCode;
    language?: FHIRCode;
    data?: FHIRBase64Binary;
    url?: FHIRUrl;
    size?: FHIRUnsignedInt;
    hash?: FHIRBase64Binary;
    title?: string;
    creation?: FHIRDateTime;
    extension?: Extension[];
  }>;
}

// ---------------------------------------------------------------------------
// ServiceRequest
// ---------------------------------------------------------------------------

export type ServiceRequestStatus = "draft" | "active" | "on-hold" | "revoked" | "completed" | "entered-in-error" | "unknown";
export type ServiceRequestIntent =
  | "proposal"
  | "plan"
  | "directive"
  | "order"
  | "original-order"
  | "reflex-order"
  | "filler-order"
  | "instance-order"
  | "option";

export interface ServiceRequest extends DomainResource {
  resourceType: "ServiceRequest";
  identifier?: Identifier[];
  instantiatesCanonical?: FHIRCanonical[];
  instantiatesUri?: FHIRUri[];
  basedOn?: Reference[];
  replaces?: Reference[];
  requisition?: Identifier;
  status: ServiceRequestStatus;
  intent: ServiceRequestIntent;
  category?: CodeableConcept[];
  priority?: "routine" | "urgent" | "asap" | "stat";
  doNotPerform?: FHIRBoolean;
  code?: CodeableConcept | Reference;
  orderDetail?: Array<{ parameterFocus?: CodeableConcept | Reference; parameter: Array<{ code: CodeableConcept; value: unknown }>; extension?: Extension[] }>;
  quantityQuantity?: Quantity;
  quantityRatio?: { numerator?: Quantity; denominator?: Quantity };
  quantityRange?: { low?: Quantity; high?: Quantity };
  subject: Reference;
  focus?: Reference[];
  encounter?: Reference;
  occurrenceDateTime?: FHIRDateTime;
  occurrencePeriod?: Period;
  asNeeded?: FHIRBoolean;
  asNeededFor?: CodeableConcept;
  authoredOn?: FHIRDateTime;
  requester?: Reference;
  performerType?: CodeableConcept;
  performer?: Reference[];
  location?: Array<{ concept?: CodeableConcept; reference?: Reference; extension?: Extension[] }>;
  reason?: Array<{ concept?: CodeableConcept; reference?: Reference; extension?: Extension[] }>;
  insurance?: Reference[];
  supportingInfo?: Reference[];
  specimen?: Reference[];
  bodySite?: CodeableConcept[];
  bodyStructure?: Reference;
  note?: Annotation[];
  patientInstruction?: Array<{ instructionMarkdown?: FHIRMarkdown; instructionReference?: Reference; extension?: Extension[] }>;
  relevantHistory?: Reference[];
}

// ---------------------------------------------------------------------------
// Immunization
// ---------------------------------------------------------------------------

export type ImmunizationStatus = "completed" | "entered-in-error" | "not-done";

export interface Immunization extends DomainResource {
  resourceType: "Immunization";
  identifier?: Identifier[];
  basedOn?: Reference[];
  status: ImmunizationStatus;
  statusReason?: CodeableConcept;
  vaccineCode: CodeableConcept;
  administeredProduct?: { concept?: CodeableConcept; reference?: Reference; extension?: Extension[] };
  manufacturer?: { concept?: CodeableConcept; reference?: Reference; extension?: Extension[] };
  lotNumber?: string;
  expirationDate?: FHIRDate;
  patient: Reference;
  encounter?: Reference;
  supportingInformation?: Reference[];
  occurrenceDateTime?: FHIRDateTime;
  occurrenceString?: string;
  primarySource?: FHIRBoolean;
  informationSource?: { concept?: CodeableConcept; reference?: Reference; extension?: Extension[] };
  location?: Reference;
  site?: CodeableConcept;
  route?: CodeableConcept;
  doseQuantity?: Quantity;
  performer?: Array<{ function?: CodeableConcept; actor: Reference; extension?: Extension[] }>;
  note?: Annotation[];
  reason?: Array<{ concept?: CodeableConcept; reference?: Reference; extension?: Extension[] }>;
  isSubpotent?: FHIRBoolean;
  subpotentReason?: CodeableConcept[];
  programEligibility?: Array<{ program?: CodeableConcept; programStatus: CodeableConcept; extension?: Extension[] }>;
  fundingSource?: CodeableConcept;
  reaction?: Array<{ date?: FHIRDateTime; manifestation?: { concept?: CodeableConcept; reference?: Reference }; reported?: FHIRBoolean; extension?: Extension[] }>;
  protocolApplied?: Array<{
    series?: string;
    authority?: Reference;
    targetDisease?: CodeableConcept[];
    doseNumber: string;
    seriesDoses?: string;
    extension?: Extension[];
  }>;
}

// ---------------------------------------------------------------------------
// Claim
// ---------------------------------------------------------------------------

export type ClaimStatus = "active" | "cancelled" | "draft" | "entered-in-error";
export type ClaimUse = "claim" | "preauthorization" | "predetermination";

export interface ClaimItem {
  sequence: FHIRPositiveInt;
  traceNumber?: Identifier[];
  careTeamSequence?: FHIRPositiveInt[];
  diagnosisSequence?: FHIRPositiveInt[];
  procedureSequence?: FHIRPositiveInt[];
  informationSequence?: FHIRPositiveInt[];
  revenue?: CodeableConcept;
  category?: CodeableConcept;
  productOrService?: CodeableConcept;
  productOrServiceEnd?: CodeableConcept;
  request?: Reference[];
  modifier?: CodeableConcept[];
  programCode?: CodeableConcept[];
  servicedDate?: FHIRDate;
  servicedPeriod?: Period;
  locationCodeableConcept?: CodeableConcept;
  locationAddress?: Address;
  locationReference?: Reference;
  patientPaid?: Quantity;
  quantity?: Quantity;
  unitPrice?: Quantity;
  factor?: FHIRDecimal;
  tax?: Quantity;
  net?: Quantity;
  udi?: Reference[];
  bodySite?: Array<{ site: Array<{ concept?: CodeableConcept; reference?: Reference }>; subSite?: CodeableConcept[]; extension?: Extension[] }>;
  encounter?: Reference[];
  detail?: ClaimItem[];
  extension?: Extension[];
}

export interface Claim extends DomainResource {
  resourceType: "Claim";
  identifier?: Identifier[];
  traceNumber?: Identifier[];
  status: ClaimStatus;
  type: CodeableConcept;
  subType?: CodeableConcept;
  use: ClaimUse;
  patient: Reference;
  billablePeriod?: Period;
  created: FHIRDateTime;
  enterer?: Reference;
  insurer?: Reference;
  provider: Reference;
  priority: CodeableConcept;
  fundsReserve?: CodeableConcept;
  related?: Array<{ claim?: Reference; relationship?: CodeableConcept; reference?: Identifier; extension?: Extension[] }>;
  prescription?: Reference;
  originalPrescription?: Reference;
  payee?: { type: CodeableConcept; party?: Reference; extension?: Extension[] };
  referral?: Reference;
  encounter?: Reference[];
  facility?: Reference;
  diagnosisRelatedGroup?: CodeableConcept;
  event?: Array<{ type: CodeableConcept; whenDateTime?: FHIRDateTime; whenPeriod?: Period; extension?: Extension[] }>;
  careTeam?: Array<{
    sequence: FHIRPositiveInt;
    provider: Reference;
    responsible?: FHIRBoolean;
    role?: CodeableConcept;
    specialty?: CodeableConcept;
    extension?: Extension[];
  }>;
  supportingInfo?: Array<{
    sequence: FHIRPositiveInt;
    category: CodeableConcept;
    code?: CodeableConcept;
    timingDate?: FHIRDate;
    timingPeriod?: Period;
    valueBoolean?: FHIRBoolean;
    valueString?: string;
    valueQuantity?: Quantity;
    valueAttachment?: { contentType?: FHIRCode; data?: FHIRBase64Binary; url?: FHIRUrl; title?: string };
    valueReference?: Reference;
    valueIdentifier?: Identifier;
    reason?: CodeableConcept;
    extension?: Extension[];
  }>;
  diagnosis?: Array<{
    sequence: FHIRPositiveInt;
    diagnosisCodeableConcept?: CodeableConcept;
    diagnosisReference?: Reference;
    type?: CodeableConcept[];
    onAdmission?: CodeableConcept;
    extension?: Extension[];
  }>;
  procedure?: Array<{
    sequence: FHIRPositiveInt;
    type?: CodeableConcept[];
    date?: FHIRDateTime;
    procedureCodeableConcept?: CodeableConcept;
    procedureReference?: Reference;
    udi?: Reference[];
    extension?: Extension[];
  }>;
  insurance: Array<{
    sequence: FHIRPositiveInt;
    focal: FHIRBoolean;
    identifier?: Identifier;
    coverage: Reference;
    businessArrangement?: string;
    preAuthRef?: string[];
    claimResponse?: Reference;
    extension?: Extension[];
  }>;
  accident?: {
    date: FHIRDate;
    type?: CodeableConcept;
    locationAddress?: Address;
    locationReference?: Reference;
    extension?: Extension[];
  };
  patientPaid?: Quantity;
  item?: ClaimItem[];
  total?: Quantity;
}

// ---------------------------------------------------------------------------
// ClaimResponse
// ---------------------------------------------------------------------------

export type ClaimResponseStatus = "active" | "cancelled" | "draft" | "entered-in-error";
export type RemittanceOutcome = "queued" | "complete" | "error" | "partial";

export interface ClaimResponse extends DomainResource {
  resourceType: "ClaimResponse";
  identifier?: Identifier[];
  traceNumber?: Identifier[];
  status: ClaimResponseStatus;
  type: CodeableConcept;
  subType?: CodeableConcept;
  use: ClaimUse;
  patient: Reference;
  created: FHIRDateTime;
  insurer?: Reference;
  requestor?: Reference;
  request?: Reference;
  outcome: RemittanceOutcome;
  decision?: CodeableConcept;
  disposition?: string;
  preAuthRef?: string;
  preAuthPeriod?: Period;
  event?: Array<{ type: CodeableConcept; whenDateTime?: FHIRDateTime; whenPeriod?: Period; extension?: Extension[] }>;
  payeeType?: CodeableConcept;
  encounter?: Reference[];
  diagnosisRelatedGroup?: CodeableConcept;
  item?: Array<{
    itemSequence: FHIRPositiveInt;
    traceNumber?: Identifier[];
    noteNumber?: FHIRPositiveInt[];
    reviewOutcome?: { decision?: CodeableConcept; reason?: CodeableConcept[]; preAuthRef?: string; preAuthPeriod?: Period };
    adjudication?: Array<{ category: CodeableConcept; reason?: CodeableConcept; amount?: Quantity; quantity?: Quantity; extension?: Extension[] }>;
    detail?: Array<{
      detailSequence: FHIRPositiveInt;
      traceNumber?: Identifier[];
      noteNumber?: FHIRPositiveInt[];
      reviewOutcome?: { decision?: CodeableConcept; reason?: CodeableConcept[] };
      adjudication?: Array<{ category: CodeableConcept; reason?: CodeableConcept; amount?: Quantity; extension?: Extension[] }>;
      subDetail?: Array<unknown>;
      extension?: Extension[];
    }>;
    extension?: Extension[];
  }>;
  addItem?: unknown[];
  adjudication?: Array<{ category: CodeableConcept; reason?: CodeableConcept; amount?: Quantity; extension?: Extension[] }>;
  total?: Array<{ category: CodeableConcept; amount: Quantity; extension?: Extension[] }>;
  payment?: {
    type?: CodeableConcept;
    adjustment?: Quantity;
    adjustmentReason?: CodeableConcept;
    date?: FHIRDate;
    amount?: Quantity;
    identifier?: Identifier;
    extension?: Extension[];
  };
  fundsReserve?: CodeableConcept;
  formCode?: CodeableConcept;
  form?: { contentType?: FHIRCode; data?: FHIRBase64Binary; url?: FHIRUrl; title?: string };
  processNote?: Array<{
    number?: FHIRPositiveInt;
    type?: CodeableConcept;
    text: string;
    language?: CodeableConcept;
    extension?: Extension[];
  }>;
  communicationRequest?: Reference[];
  insurance?: Array<{
    sequence: FHIRPositiveInt;
    focal: FHIRBoolean;
    coverage: Reference;
    businessArrangement?: string;
    claimResponse?: Reference;
    extension?: Extension[];
  }>;
  error?: Array<{
    itemSequence?: FHIRPositiveInt;
    detailSequence?: FHIRPositiveInt;
    subDetailSequence?: FHIRPositiveInt;
    expression?: string[];
    code: CodeableConcept;
    extension?: Extension[];
  }>;
}

// ---------------------------------------------------------------------------
// Bundle
// ---------------------------------------------------------------------------

export type BundleType =
  | "document"
  | "message"
  | "transaction"
  | "transaction-response"
  | "batch"
  | "batch-response"
  | "history"
  | "searchset"
  | "collection"
  | "subscription-notification";

export interface BundleLink {
  relation: string;
  url: FHIRUri;
  extension?: Extension[];
}

export interface BundleSearch {
  mode?: "match" | "include" | "outcome";
  score?: FHIRDecimal;
  extension?: Extension[];
}

export interface BundleRequest {
  method: "GET" | "HEAD" | "POST" | "PUT" | "DELETE" | "PATCH";
  url: FHIRUri;
  ifNoneMatch?: string;
  ifModifiedSince?: FHIRInstant;
  ifMatch?: string;
  ifNoneExist?: string;
  extension?: Extension[];
}

export interface BundleResponse {
  status: string;
  location?: FHIRUri;
  etag?: string;
  lastModified?: FHIRInstant;
  outcome?: ResourceBase;
  extension?: Extension[];
}

export interface BundleEntry {
  link?: BundleLink[];
  fullUrl?: FHIRUri;
  resource?: ResourceBase;
  search?: BundleSearch;
  request?: BundleRequest;
  response?: BundleResponse;
  extension?: Extension[];
}

export interface Bundle extends ResourceBase {
  resourceType: "Bundle";
  identifier?: Identifier;
  type: BundleType;
  timestamp?: FHIRInstant;
  total?: FHIRUnsignedInt;
  link?: BundleLink[];
  entry?: BundleEntry[];
  signature?: unknown;
  issues?: ResourceBase;
}

// ---------------------------------------------------------------------------
// OperationOutcome
// ---------------------------------------------------------------------------

export type IssueSeverity = "fatal" | "error" | "warning" | "information" | "success";
export type IssueType =
  | "invalid"
  | "structure"
  | "required"
  | "value"
  | "invariant"
  | "security"
  | "login"
  | "unknown"
  | "expired"
  | "forbidden"
  | "suppressed"
  | "processing"
  | "not-supported"
  | "duplicate"
  | "multiple-matches"
  | "not-found"
  | "deleted"
  | "too-long"
  | "code-invalid"
  | "extension"
  | "too-costly"
  | "business-rule"
  | "conflict"
  | "limited-filter"
  | "transient"
  | "lock-error"
  | "no-store"
  | "exception"
  | "timeout"
  | "incomplete"
  | "throttled"
  | "informational"
  | "success";

export interface OperationOutcomeIssue {
  severity: IssueSeverity;
  code: IssueType;
  details?: CodeableConcept;
  diagnostics?: string;
  location?: string[];
  expression?: string[];
  extension?: Extension[];
}

export interface OperationOutcome extends DomainResource {
  resourceType: "OperationOutcome";
  issue: OperationOutcomeIssue[];
}
