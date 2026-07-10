import type {
  FhirResource,
  Bundle,
  OperationOutcome,
  Patient,
  Subscription,
} from "@tpt/fhir-types";

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

export interface ClientConfig {
  /** Base URL of the tpt-health-interop service, e.g. https://interop.tpt.health */
  baseURL: string;
  /** Te Whatu Ora / practice tenant identifier. Sent as X-Tenant-ID header. */
  tenantID: string;
  /** Async callback that returns a valid Bearer token for the current session. */
  getToken: () => Promise<string>;
}

// ---------------------------------------------------------------------------
// Error types
// ---------------------------------------------------------------------------

export class TptApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly statusText: string,
    public readonly operationOutcome: OperationOutcome | null,
    public readonly url: string,
  ) {
    const detail =
      operationOutcome?.issue?.[0]?.diagnostics ??
      operationOutcome?.issue?.[0]?.details?.text ??
      statusText;
    super(`TptApiError ${status} ${url}: ${detail}`);
    this.name = "TptApiError";
  }
}

// ---------------------------------------------------------------------------
// FHIR search / NHI types
// ---------------------------------------------------------------------------

export type FhirSearchParams = Record<
  string,
  string | number | boolean | undefined
>;

export interface MatchParams {
  /** NHI number as a search parameter */
  nhi?: string | undefined;
  /** Given name(s) */
  given?: string | undefined;
  /** Family name */
  family?: string | undefined;
  /** Date of birth (ISO 8601) */
  birthdate?: string | undefined;
  /** Gender code */
  gender?: string | undefined;
}

// ---------------------------------------------------------------------------
// Terminology result types
// ---------------------------------------------------------------------------

export interface CodingResult {
  system: string;
  code: string;
  display: string;
  version?: string;
}

export interface TerminologySearchResult {
  total: number;
  results: CodingResult[];
}

// ---------------------------------------------------------------------------
// Subscription types
// ---------------------------------------------------------------------------

export interface SubscriptionCreateParams {
  /** FHIR R5 topic canonical URL */
  topic: string;
  /** Notification endpoint URL */
  endpoint: string;
  /** Endpoint MIME type, defaults to application/fhir+json */
  contentType?: string;
  /** Filter criteria, e.g. "Patient?_id=NHI123" */
  filterBy?: Array<{ resourceType: string; filterParameter: string; value: string }>;
  /** Payload content level: "empty" | "id-only" | "full-resource" */
  content?: "empty" | "id-only" | "full-resource";
  /** Channel-type specific headers (e.g. Authorization for the endpoint) */
  channelHeaders?: string[];
}

// ---------------------------------------------------------------------------
// Main client
// ---------------------------------------------------------------------------

export class TptApiClient {
  constructor(private readonly config: ClientConfig) {}

  // -------------------------------------------------------------------------
  // Core request helper
  // -------------------------------------------------------------------------

  private async request<T>(
    method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH",
    path: string,
    options: {
      body?: unknown;
      searchParams?: FhirSearchParams;
      headers?: Record<string, string>;
    } = {},
  ): Promise<T> {
    const { baseURL, tenantID, getToken } = this.config;

    // Build URL
    const url = new URL(
      path.startsWith("/") ? path : `/${path}`,
      baseURL.endsWith("/") ? baseURL : `${baseURL}/`,
    );

    if (options.searchParams) {
      for (const [key, value] of Object.entries(options.searchParams)) {
        if (value !== undefined) {
          url.searchParams.set(key, String(value));
        }
      }
    }

    // Resolve auth token
    const token = await getToken();

    const headers: Record<string, string> = {
      Accept: "application/fhir+json",
      "X-Tenant-ID": tenantID,
      Authorization: `Bearer ${token}`,
      ...options.headers,
    };

    if (options.body !== undefined) {
      headers["Content-Type"] = "application/fhir+json";
    }

    const response = await fetch(url.toString(), {
      method,
      headers,
      body: options.body !== undefined ? JSON.stringify(options.body) : null,
    });

    if (!response.ok) {
      let outcome: OperationOutcome | null = null;
      try {
        const ct = response.headers.get("content-type") ?? "";
        if (ct.includes("fhir") || ct.includes("json")) {
          const json = (await response.json()) as { resourceType?: string };
          if (json.resourceType === "OperationOutcome") {
            outcome = json as OperationOutcome;
          }
        }
      } catch {
        // ignore parse failure — we'll surface the HTTP status instead
      }
      throw new TptApiError(
        response.status,
        response.statusText,
        outcome,
        url.toString(),
      );
    }

    // 204 No Content
    if (response.status === 204) {
      return undefined as unknown as T;
    }

    return response.json() as Promise<T>;
  }

  // -------------------------------------------------------------------------
  // FHIR namespace
  // -------------------------------------------------------------------------

  readonly fhir = {
    /**
     * Read a FHIR resource by type and logical ID.
     * GET /fhir/R5/{resourceType}/{id}
     */
    read: <T extends FhirResource = FhirResource>(
      resourceType: string,
      id: string,
    ): Promise<T> =>
      this.request<T>("GET", `fhir/R5/${resourceType}/${id}`),

    /**
     * Create a FHIR resource (server-assigned ID).
     * POST /fhir/R5/{resourceType}
     */
    create: <T extends FhirResource = FhirResource>(
      resourceType: string,
      resource: Omit<T, "id">,
    ): Promise<T> =>
      this.request<T>("POST", `fhir/R5/${resourceType}`, { body: resource }),

    /**
     * Update (or conditional create) a FHIR resource.
     * PUT /fhir/R5/{resourceType}/{id}
     */
    update: <T extends FhirResource = FhirResource>(
      resourceType: string,
      id: string,
      resource: T,
    ): Promise<T> =>
      this.request<T>("PUT", `fhir/R5/${resourceType}/${id}`, {
        body: resource,
      }),

    /**
     * Delete a FHIR resource.
     * DELETE /fhir/R5/{resourceType}/{id}
     */
    delete: (resourceType: string, id: string): Promise<void> =>
      this.request<void>("DELETE", `fhir/R5/${resourceType}/${id}`),

    /**
     * Search a FHIR resource type.
     * GET /fhir/R5/{resourceType}?{params}
     */
    search: <T extends FhirResource = FhirResource>(
      resourceType: string,
      params: FhirSearchParams = {},
    ): Promise<Bundle<T>> =>
      this.request<Bundle<T>>("GET", `fhir/R5/${resourceType}`, {
        searchParams: params,
      }),
  };

  // -------------------------------------------------------------------------
  // NHI namespace
  // -------------------------------------------------------------------------

  readonly nhi = {
    /**
     * Retrieve a Patient resource by NHI number.
     * GET /nhi/{nhi}
     */
    lookup: (nhi: string): Promise<Patient> =>
      this.request<Patient>("GET", `nhi/${encodeURIComponent(nhi)}`),

    /**
     * Perform a demographic match against the NHI register.
     * POST /nhi/$match
     */
    match: (params: MatchParams): Promise<Bundle<Patient>> =>
      this.request<Bundle<Patient>>("POST", "nhi/$match", {
        body: {
          resourceType: "Parameters",
          parameter: Object.entries(params)
            .filter(([, v]) => v !== undefined)
            .map(([name, valueString]) => ({ name, valueString })),
        },
      }),
  };

  // -------------------------------------------------------------------------
  // Terminology namespace
  // -------------------------------------------------------------------------

  readonly terminology = {
    /**
     * Search SNOMED CT concepts (NZ edition).
     * GET /terminology/snomed?q={query}
     */
    snomedSearch: (q: string): Promise<TerminologySearchResult> =>
      this.request<TerminologySearchResult>("GET", "terminology/snomed", {
        searchParams: { q },
      }),

    /**
     * Search LOINC codes.
     * GET /terminology/loinc?q={query}
     */
    loincSearch: (q: string): Promise<TerminologySearchResult> =>
      this.request<TerminologySearchResult>("GET", "terminology/loinc", {
        searchParams: { q },
      }),

    /**
     * Search ICD-10-AM codes.
     * GET /terminology/icd10am?q={query}
     */
    icd10Search: (q: string): Promise<TerminologySearchResult> =>
      this.request<TerminologySearchResult>("GET", "terminology/icd10am", {
        searchParams: { q },
      }),

    /**
     * Search NZMT (New Zealand Medicines Terminology) codes.
     * GET /terminology/nzmt?q={query}
     */
    nzmtSearch: (q: string): Promise<TerminologySearchResult> =>
      this.request<TerminologySearchResult>("GET", "terminology/nzmt", {
        searchParams: { q },
      }),
  };

  // -------------------------------------------------------------------------
  // Subscriptions namespace
  // -------------------------------------------------------------------------

  readonly subscriptions = {
    /**
     * List all active subscriptions for this tenant.
     * GET /fhir/R5/Subscription
     */
    list: (): Promise<Bundle<Subscription>> =>
      this.request<Bundle<Subscription>>("GET", "fhir/R5/Subscription"),

    /**
     * Read a single Subscription by ID.
     * GET /fhir/R5/Subscription/{id}
     */
    get: (id: string): Promise<Subscription> =>
      this.request<Subscription>("GET", `fhir/R5/Subscription/${id}`),

    /**
     * Create a new Subscription.
     * POST /fhir/R5/Subscription
     */
    create: (params: SubscriptionCreateParams): Promise<Subscription> => {
      const resource = buildSubscriptionResource(params);
      return this.request<Subscription>("POST", "fhir/R5/Subscription", {
        body: resource,
      });
    },

    /**
     * Update an existing Subscription.
     * PUT /fhir/R5/Subscription/{id}
     */
    update: (
      id: string,
      params: Partial<SubscriptionCreateParams>,
    ): Promise<Subscription> => {
      const resource = buildSubscriptionResource({ ...params, id });
      return this.request<Subscription>(
        "PUT",
        `fhir/R5/Subscription/${id}`,
        { body: resource },
      );
    },

    /**
     * Delete a Subscription.
     * DELETE /fhir/R5/Subscription/{id}
     */
    delete: (id: string): Promise<void> =>
      this.request<void>("DELETE", `fhir/R5/Subscription/${id}`),
  };
}

// ---------------------------------------------------------------------------
// Internal builder for Subscription resources
// ---------------------------------------------------------------------------

function buildSubscriptionResource(
  params: Partial<SubscriptionCreateParams> & { id?: string },
): Partial<Subscription> & { resourceType: "Subscription" } {
  return {
    resourceType: "Subscription",
    ...(params.id ? { id: params.id } : {}),
    status: "requested",
    topic: params.topic ?? "",
    channelType: {
      system: "http://terminology.hl7.org/CodeSystem/subscription-channel-type",
      code: "rest-hook",
    },
    endpoint: params.endpoint ?? "",
    contentType: params.contentType ?? "application/fhir+json",
    content: params.content ?? "id-only",
    ...(params.filterBy?.length
      ? {
          filterBy: params.filterBy.map((f) => ({
            resourceType: f.resourceType,
            filterParameter: f.filterParameter,
            value: f.value,
          })),
        }
      : {}),
    ...(params.channelHeaders?.length
      ? { header: params.channelHeaders }
      : {}),
  };
}
