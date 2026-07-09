import { describe, it, expect, vi, beforeEach } from "vitest";
import { TptApiClient, TptApiError } from "./client.js";

// ---------------------------------------------------------------------------
// TptApiError
// ---------------------------------------------------------------------------

describe("TptApiError", () => {
  it("formats error message with statusText", () => {
    const err = new TptApiError(404, "Not Found", null, "https://api.test/fhir/R5/Patient/123");
    expect(err.message).toContain("404");
    expect(err.message).toContain("Not Found");
    expect(err.message).toContain("https://api.test/fhir/R5/Patient/123");
    expect(err.name).toBe("TptApiError");
    expect(err.status).toBe(404);
  });

  it("extracts detail from OperationOutcome", () => {
    const outcome = {
      resourceType: "OperationOutcome" as const,
      issue: [
        {
          severity: "error",
          code: "not-found",
          diagnostics: "Patient ZAC1234 not found",
        },
      ],
    };
    const err = new TptApiError(404, "Not Found", outcome, "https://api.test/nhi/ZAC1234");
    expect(err.message).toContain("Patient ZAC1234 not found");
    expect(err.operationOutcome).toBe(outcome);
  });
});

// ---------------------------------------------------------------------------
// TptApiClient — mocked fetch
// ---------------------------------------------------------------------------

describe("TptApiClient", () => {
  const config = {
    baseURL: "https://api.test",
    tenantID: "tenant-1",
    getToken: vi.fn().mockResolvedValue("test-token"),
  };

  beforeEach(() => {
    vi.restoreAllMocks();
    config.getToken.mockResolvedValue("test-token");
  });

  function mockFetch(status: number, body: unknown, headers?: Record<string, string>) {
    const response = {
      ok: status >= 200 && status < 300,
      status,
      statusText: status === 200 ? "OK" : "Error",
      headers: new Map(Object.entries({ "content-type": "application/fhir+json", ...headers })),
      json: () => Promise.resolve(body),
    };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
    return response;
  }

  // -------------------------------------------------------------------------
  // fhir.read
  // -------------------------------------------------------------------------

  describe("fhir.read", () => {
    it("sends GET with correct URL and headers", async () => {
      const patient = { resourceType: "Patient", id: "ZAC1234" };
      mockFetch(200, patient);

      const client = new TptApiClient(config);
      const result = await client.fhir.read("Patient", "ZAC1234");

      expect(result).toEqual(patient);
      expect(fetch).toHaveBeenCalledTimes(1);
      const [url, init] = (fetch as any).mock.calls[0];
      expect(url).toBe("https://api.test/fhir/R5/Patient/ZAC1234");
      expect(init.method).toBe("GET");
      expect(init.headers["Authorization"]).toBe("Bearer test-token");
      expect(init.headers["X-Tenant-ID"]).toBe("tenant-1");
      expect(init.headers["Accept"]).toBe("application/fhir+json");
    });
  });

  // -------------------------------------------------------------------------
  // fhir.search
  // -------------------------------------------------------------------------

  describe("fhir.search", () => {
    it("appends search params as query string", async () => {
      const bundle = { resourceType: "Bundle", total: 0, entry: [] };
      mockFetch(200, bundle);

      const client = new TptApiClient(config);
      await client.fhir.search("Patient", { gender: "male", name: "Smith" });

      const [url] = (fetch as any).mock.calls[0];
      expect(url).toContain("gender=male");
      expect(url).toContain("name=Smith");
    });

    it("omits undefined params", async () => {
      const bundle = { resourceType: "Bundle", total: 0, entry: [] };
      mockFetch(200, bundle);

      const client = new TptApiClient(config);
      await client.fhir.search("Patient", { gender: "male", name: undefined });

      const [url] = (fetch as any).mock.calls[0];
      expect(url).toContain("gender=male");
      expect(url).not.toContain("name=");
    });
  });

  // -------------------------------------------------------------------------
  // fhir.create
  // -------------------------------------------------------------------------

  describe("fhir.create", () => {
    it("sends POST with JSON body", async () => {
      const created = { resourceType: "Patient", id: "new-id" };
      mockFetch(201, created);

      const client = new TptApiClient(config);
      const resource = { resourceType: "Patient" as const, name: [] };
      const result = await client.fhir.create("Patient", resource);

      expect(result).toEqual(created);
      const [url, init] = (fetch as any).mock.calls[0];
      expect(url).toBe("https://api.test/fhir/R5/Patient");
      expect(init.method).toBe("POST");
      expect(JSON.parse(init.body)).toEqual(resource);
    });
  });

  // -------------------------------------------------------------------------
  // fhir.update
  // -------------------------------------------------------------------------

  describe("fhir.update", () => {
    it("sends PUT with resource body", async () => {
      const updated = { resourceType: "Patient", id: "ZAC1234" };
      mockFetch(200, updated);

      const client = new TptApiClient(config);
      await client.fhir.update("Patient", "ZAC1234", updated as any);

      const [url, init] = (fetch as any).mock.calls[0];
      expect(url).toBe("https://api.test/fhir/R5/Patient/ZAC1234");
      expect(init.method).toBe("PUT");
    });
  });

  // -------------------------------------------------------------------------
  // fhir.delete
  // -------------------------------------------------------------------------

  describe("fhir.delete", () => {
    it("sends DELETE", async () => {
      mockFetch(204, undefined);

      const client = new TptApiClient(config);
      await client.fhir.delete("Patient", "ZAC1234");

      const [url, init] = (fetch as any).mock.calls[0];
      expect(url).toBe("https://api.test/fhir/R5/Patient/ZAC1234");
      expect(init.method).toBe("DELETE");
    });
  });

  // -------------------------------------------------------------------------
  // nhi.lookup
  // -------------------------------------------------------------------------

  describe("nhi.lookup", () => {
    it("URL-encodes the NHI", async () => {
      const patient = { resourceType: "Patient", id: "ZAC1234" };
      mockFetch(200, patient);

      const client = new TptApiClient(config);
      await client.nhi.lookup("ZAC1234");

      const [url] = (fetch as any).mock.calls[0];
      expect(url).toBe("https://api.test/nhi/ZAC1234");
    });
  });

  // -------------------------------------------------------------------------
  // nhi.match
  // -------------------------------------------------------------------------

  describe("nhi.match", () => {
    it("sends POST with Parameters body", async () => {
      const bundle = { resourceType: "Bundle", entry: [] };
      mockFetch(200, bundle);

      const client = new TptApiClient(config);
      await client.nhi.match({ given: "John", family: "Smith" });

      const [url, init] = (fetch as any).mock.calls[0];
      expect(url).toBe("https://api.test/nhi/$match");
      expect(init.method).toBe("POST");
      const body = JSON.parse(init.body);
      expect(body.resourceType).toBe("Parameters");
    });

    it("filters out undefined params", async () => {
      const bundle = { resourceType: "Bundle", entry: [] };
      mockFetch(200, bundle);

      const client = new TptApiClient(config);
      await client.nhi.match({ given: "John", family: undefined });

      const [, init] = (fetch as any).mock.calls[0];
      const body = JSON.parse(init.body);
      expect(body.parameter).toHaveLength(1);
      expect(body.parameter[0].name).toBe("given");
    });
  });

  // -------------------------------------------------------------------------
  // terminology
  // -------------------------------------------------------------------------

  describe("terminology", () => {
    it("snomedSearch sends GET with q param", async () => {
      const result = { total: 1, results: [] };
      mockFetch(200, result);

      const client = new TptApiClient(config);
      await client.terminology.snomedSearch("heart attack");

      const [url] = (fetch as any).mock.calls[0];
      expect(url).toContain("q=heart+attack");
    });

    it("loincSearch sends GET with q param", async () => {
      const result = { total: 0, results: [] };
      mockFetch(200, result);

      const client = new TptApiClient(config);
      await client.terminology.loincSearch("creatinine");

      const [url] = (fetch as any).mock.calls[0];
      expect(url).toContain("q=creatinine");
    });

    it("icd10Search sends GET with q param", async () => {
      const result = { total: 0, results: [] };
      mockFetch(200, result);

      const client = new TptApiClient(config);
      await client.terminology.icd10Search("asthma");

      const [url] = (fetch as any).mock.calls[0];
      expect(url).toContain("q=asthma");
    });

    it("nzmtSearch sends GET with q param", async () => {
      const result = { total: 0, results: [] };
      mockFetch(200, result);

      const client = new TptApiClient(config);
      await client.terminology.nzmtSearch("paracetamol");

      const [url] = (fetch as any).mock.calls[0];
      expect(url).toContain("q=paracetamol");
    });
  });

  // -------------------------------------------------------------------------
  // subscriptions
  // -------------------------------------------------------------------------

  describe("subscriptions", () => {
    it("list sends GET to Subscription endpoint", async () => {
      const bundle = { resourceType: "Bundle", entry: [] };
      mockFetch(200, bundle);

      const client = new TptApiClient(config);
      await client.subscriptions.list();

      const [url, init] = (fetch as any).mock.calls[0];
      expect(url).toBe("https://api.test/fhir/R5/Subscription");
      expect(init.method).toBe("GET");
    });

    it("create builds Subscription resource and sends POST", async () => {
      const sub = { resourceType: "Subscription", id: "sub-1" };
      mockFetch(201, sub);

      const client = new TptApiClient(config);
      await client.subscriptions.create({
        topic: "https://example.com/topic/patient-update",
        endpoint: "https://hook.example.com",
      });

      const [url, init] = (fetch as any).mock.calls[0];
      expect(url).toBe("https://api.test/fhir/R5/Subscription");
      expect(init.method).toBe("POST");
      const body = JSON.parse(init.body);
      expect(body.resourceType).toBe("Subscription");
      expect(body.status).toBe("requested");
      expect(body.topic).toBe("https://example.com/topic/patient-update");
      expect(body.endpoint).toBe("https://hook.example.com");
    });

    it("create includes filterBy when provided", async () => {
      const sub = { resourceType: "Subscription", id: "sub-2" };
      mockFetch(201, sub);

      const client = new TptApiClient(config);
      await client.subscriptions.create({
        topic: "https://example.com/topic",
        endpoint: "https://hook.example.com",
        filterBy: [{ resourceType: "Patient", filterParameter: "_id", value: "ZAC1234" }],
      });

      const [, init] = (fetch as any).mock.calls[0];
      const body = JSON.parse(init.body);
      expect(body.filterBy).toHaveLength(1);
      expect(body.filterBy[0].resourceType).toBe("Patient");
    });

    it("delete sends DELETE", async () => {
      mockFetch(204, undefined);

      const client = new TptApiClient(config);
      await client.subscriptions.delete("sub-1");

      const [url, init] = (fetch as any).mock.calls[0];
      expect(url).toBe("https://api.test/fhir/R5/Subscription/sub-1");
      expect(init.method).toBe("DELETE");
    });
  });

  // -------------------------------------------------------------------------
  // Error handling
  // -------------------------------------------------------------------------

  describe("error handling", () => {
    it("throws TptApiError on non-OK response", async () => {
      mockFetch(404, { resourceType: "OperationOutcome", issue: [{ severity: "error", diagnostics: "not found" }] });

      const client = new TptApiClient(config);
      await expect(client.fhir.read("Patient", "MISSING")).rejects.toThrow(TptApiError);
    });

    it("throws TptApiError on network-level non-OK without OperationOutcome", async () => {
      const response = {
        ok: false,
        status: 500,
        statusText: "Internal Server Error",
        headers: new Map([["content-type", "text/plain"]]),
        json: () => Promise.reject(new Error("not json")),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));

      const client = new TptApiClient(config);
      await expect(client.fhir.read("Patient", "123")).rejects.toThrow("500");
    });
  });

  // -------------------------------------------------------------------------
  // Auth token
  // -------------------------------------------------------------------------

  describe("auth token", () => {
    it("calls getToken for every request", async () => {
      mockFetch(200, { resourceType: "Patient", id: "1" });

      const client = new TptApiClient(config);
      await client.fhir.read("Patient", "1");
      await client.fhir.read("Patient", "2");

      expect(config.getToken).toHaveBeenCalledTimes(2);
    });

    it("propagates getToken errors", async () => {
      config.getToken.mockRejectedValue(new Error("auth failed"));

      const client = new TptApiClient(config);
      await expect(client.fhir.read("Patient", "1")).rejects.toThrow("auth failed");
    });
  });
});
