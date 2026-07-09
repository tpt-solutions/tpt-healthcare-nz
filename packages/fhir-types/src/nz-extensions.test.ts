import { describe, it, expect } from "vitest";
import {
  getNHI,
  getHPICPN,
  getNZEthnicities,
  getNZIwiAffiliations,
  NHI_SYSTEM,
  HPI_CPN_SYSTEM,
  NZ_ETHNICITY_EXTENSION_URL,
  NZ_IWI_EXTENSION_URL,
} from "./nz-extensions.js";
import type { Patient, Practitioner } from "./r5.js";

// ---------------------------------------------------------------------------
// getNHI
// ---------------------------------------------------------------------------

describe("getNHI", () => {
  it("returns NHI value when present", () => {
    const patient: Patient = {
      resourceType: "Patient",
      identifier: [
        { system: NHI_SYSTEM, value: "ZAC1234" },
        { system: "https://example.com/other", value: "OTHER" },
      ],
    };
    expect(getNHI(patient)).toBe("ZAC1234");
  });

  it("returns undefined when no identifiers", () => {
    const patient: Patient = { resourceType: "Patient" };
    expect(getNHI(patient)).toBeUndefined();
  });

  it("returns undefined when no NHI identifier", () => {
    const patient: Patient = {
      resourceType: "Patient",
      identifier: [{ system: "https://example.com/other", value: "OTHER" }],
    };
    expect(getNHI(patient)).toBeUndefined();
  });

  it("returns undefined when identifier has no value", () => {
    const patient: Patient = {
      resourceType: "Patient",
      identifier: [{ system: NHI_SYSTEM }],
    };
    expect(getNHI(patient)).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// getHPICPN
// ---------------------------------------------------------------------------

describe("getHPICPN", () => {
  it("returns HPI CPN when present", () => {
    const practitioner: Practitioner = {
      resourceType: "Practitioner",
      identifier: [
        { system: HPI_CPN_SYSTEM, value: "987654321" },
      ],
    };
    expect(getHPICPN(practitioner)).toBe("987654321");
  });

  it("returns undefined when no identifiers", () => {
    const practitioner: Practitioner = { resourceType: "Practitioner" };
    expect(getHPICPN(practitioner)).toBeUndefined();
  });

  it("returns undefined when no HPI CPN", () => {
    const practitioner: Practitioner = {
      resourceType: "Practitioner",
      identifier: [{ system: "https://example.com/other", value: "X" }],
    };
    expect(getHPICPN(practitioner)).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// getNZEthnicities
// ---------------------------------------------------------------------------

describe("getNZEthnicities", () => {
  it("returns ethnicity extensions", () => {
    const patient: Patient = {
      resourceType: "Patient",
      extension: [
        {
          url: NZ_ETHNICITY_EXTENSION_URL,
          valueCoding: { system: "https://standards.digital.health.nz/ns/ethnic-group-level-4-code", code: "11111", display: "New Zealand European" },
        },
        {
          url: "https://example.com/other",
          valueString: "other",
        },
      ],
    };
    const result = getNZEthnicities(patient);
    expect(result).toHaveLength(1);
    expect(result[0].valueCoding.code).toBe("11111");
  });

  it("returns empty array when no extensions", () => {
    const patient: Patient = { resourceType: "Patient" };
    expect(getNZEthnicities(patient)).toEqual([]);
  });

  it("returns empty array when no ethnicity extensions", () => {
    const patient: Patient = {
      resourceType: "Patient",
      extension: [
        { url: "https://example.com/other", valueString: "x" },
      ],
    };
    expect(getNZEthnicities(patient)).toEqual([]);
  });

  it("returns multiple ethnicity entries", () => {
    const patient: Patient = {
      resourceType: "Patient",
      extension: [
        {
          url: NZ_ETHNICITY_EXTENSION_URL,
          valueCoding: { system: "https://standards.digital.health.nz/ns/ethnic-group-level-4-code", code: "11111" },
        },
        {
          url: NZ_ETHNICITY_EXTENSION_URL,
          valueCoding: { system: "https://standards.digital.health.nz/ns/ethnic-group-level-4-code", code: "21111" },
        },
      ],
    };
    expect(getNZEthnicities(patient)).toHaveLength(2);
  });
});

// ---------------------------------------------------------------------------
// getNZIwiAffiliations
// ---------------------------------------------------------------------------

describe("getNZIwiAffiliations", () => {
  it("returns iwi extensions", () => {
    const patient: Patient = {
      resourceType: "Patient",
      extension: [
        {
          url: NZ_IWI_EXTENSION_URL,
          valueCoding: { system: "https://standards.digital.health.nz/ns/iwi-code", code: "T2" },
        },
      ],
    };
    const result = getNZIwiAffiliations(patient);
    expect(result).toHaveLength(1);
    expect(result[0].valueCoding.code).toBe("T2");
  });

  it("returns empty array when no iwi extensions", () => {
    const patient: Patient = { resourceType: "Patient" };
    expect(getNZIwiAffiliations(patient)).toEqual([]);
  });
});
