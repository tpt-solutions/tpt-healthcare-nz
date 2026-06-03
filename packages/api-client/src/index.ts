// Main client class and config type
export { TptApiClient, TptApiError } from "./client";
export type {
  ClientConfig,
  FhirSearchParams,
  MatchParams,
  CodingResult,
  TerminologySearchResult,
  SubscriptionCreateParams,
} from "./client";

// Re-export FHIR types that callers are likely to need alongside the client
export type {
  FhirResource,
  Bundle,
  BundleEntry,
  OperationOutcome,
  OperationOutcomeIssue,
  Patient,
  Practitioner,
  Subscription,
} from "@tpt/fhir-types";
