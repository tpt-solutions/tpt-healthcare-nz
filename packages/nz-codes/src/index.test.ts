import { describe, it, expect } from "vitest";
import {
  validateNHI,
  formatNHI,
  isNewNHIFormat,
  getACCDiagnosisDescription,
  isKnownACCFormNumber,
  isNZIdentifierSystem,
  ACC45,
  ACC6,
  ACC18,
  ACC2152,
  ACC32,
  NHI_SYSTEM,
  HPI_CPN_SYSTEM,
  SNOMED_CT_SYSTEM,
} from "./index.js";

// ---------------------------------------------------------------------------
// NHI validation
// ---------------------------------------------------------------------------

describe("validateNHI", () => {
  it("rejects empty string", () => {
    expect(validateNHI("")).toBe(false);
  });

  it("rejects too-short input", () => {
    expect(validateNHI("ABC12")).toBe(false);
  });

  it("rejects too-long input", () => {
    expect(validateNHI("ABC12345")).toBe(false);
  });

  it("rejects input with I or O", () => {
    expect(validateNHI("IAI1234")).toBe(false);
    expect(validateNHI("OAO1234")).toBe(false);
  });

  it("accepts valid old-format NHI (ABC1234)", () => {
    // ABC1234: A=1*7 + B=2*6 + C=3*5 + 1*4 + 2*3 + 3*2 = 7+12+15+4+6+6 = 50
    // 50 % 11 = 6, 11-6 = 5. Check digit 4 ≠ 5 → invalid
    // Let me compute: A=1,B=2,C=3, digits 1,2,3
    // sum = 1*7 + 2*6 + 3*5 + 1*4 + 2*3 + 3*2 = 7+12+15+4+6+6 = 50
    // 50 % 11 = 6, check = 11-6 = 5. Actual check digit is 4. So ABC1234 is invalid.
    // Let me use a known-valid NHI: ZAC1234
    // Z=26*7 + A=1*6 + C=3*5 + 1*4 + 2*3 + 3*2 = 182+6+15+4+6+6 = 219
    // 219 % 11 = 10, check = 11-10 = 1. Actual is 4. Invalid.
    // I need to find a valid one by computing the check digit.
    // Let's compute for "AAA" + digits: A=1, A=1, A=1, d1, d2, d3, check
    // sum = 1*7 + 1*6 + 1*5 + d1*4 + d2*3 + d3*2
    // For AAA000?: sum = 7+6+5+0+0+0 = 18, 18%11=7, check=11-7=4 → AAA0004
    expect(validateNHI("AAA0004")).toBe(true);
  });

  it("rejects old-format NHI with wrong checksum", () => {
    expect(validateNHI("AAA0000")).toBe(false);
  });

  it("accepts valid new-format NHI starting with Z", () => {
    // Z=24, A=1, B=2, 0, 0, 0
    // sum = 24*7 + 1*6 + 2*5 + 0*4 + 0*3 + 0*2 = 168+6+10 = 184
    // expected = 24 - (184 % 24) = 24 - 16 = 8
    // H = NHI_ALPHA index 7, value = 8. Match!
    expect(validateNHI("ZAB000H")).toBe(true);
  });

  it("rejects new-format NHI with wrong checksum", () => {
    expect(validateNHI("ZAB000A")).toBe(false);
  });

  it("normalises lowercase to uppercase", () => {
    expect(validateNHI("aaa0004")).toBe(true);
  });

  it("trims whitespace", () => {
    expect(validateNHI("  AAA0004  ")).toBe(true);
  });
});

describe("formatNHI", () => {
  it("uppercases and trims", () => {
    expect(formatNHI("  abc1234  ")).toBe("ABC1234");
  });

  it("handles already uppercase", () => {
    expect(formatNHI("ZAC1234")).toBe("ZAC1234");
  });
});

describe("isNewNHIFormat", () => {
  it("returns true for Z-prefixed NHI", () => {
    expect(isNewNHIFormat("ZAB000H")).toBe(true);
  });

  it("returns false for old-format NHI", () => {
    expect(isNewNHIFormat("AAA0004")).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// ACC helpers
// ---------------------------------------------------------------------------

describe("getACCDiagnosisDescription", () => {
  it("returns description for known code", () => {
    expect(getACCDiagnosisDescription("S30")).toBe(
      "Superficial injury of abdomen, lower back and pelvis"
    );
  });

  it("is case-insensitive", () => {
    expect(getACCDiagnosisDescription("s30")).toBe(
      "Superficial injury of abdomen, lower back and pelvis"
    );
  });

  it("returns undefined for unknown code", () => {
    expect(getACCDiagnosisDescription("ZZZ")).toBeUndefined();
  });
});

describe("isKnownACCFormNumber", () => {
  it("returns true for ACC45", () => {
    expect(isKnownACCFormNumber(ACC45)).toBe(true);
  });

  it("returns true for ACC6", () => {
    expect(isKnownACCFormNumber(ACC6)).toBe(true);
  });

  it("returns true for ACC18", () => {
    expect(isKnownACCFormNumber(ACC18)).toBe(true);
  });

  it("returns true for ACC2152", () => {
    expect(isKnownACCFormNumber(ACC2152)).toBe(true);
  });

  it("returns true for ACC32", () => {
    expect(isKnownACCFormNumber(ACC32)).toBe(true);
  });

  it("returns false for unknown form", () => {
    expect(isKnownACCFormNumber("ACC999")).toBe(false);
  });

  it("returns false for empty string", () => {
    expect(isKnownACCFormNumber("")).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// URI helpers
// ---------------------------------------------------------------------------

describe("isNZIdentifierSystem", () => {
  it("returns true for NHI system", () => {
    expect(isNZIdentifierSystem(NHI_SYSTEM)).toBe(true);
  });

  it("returns true for HPI CPN system", () => {
    expect(isNZIdentifierSystem(HPI_CPN_SYSTEM)).toBe(true);
  });

  it("returns false for SNOMED (not NZ-specific)", () => {
    expect(isNZIdentifierSystem(SNOMED_CT_SYSTEM)).toBe(false);
  });

  it("returns false for random string", () => {
    expect(isNZIdentifierSystem("https://example.com")).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// ACC constants
// ---------------------------------------------------------------------------

describe("ACC constants", () => {
  it("ACC45 is correct value", () => {
    expect(ACC45).toBe("ACC45");
  });

  it("ACC6 is correct value", () => {
    expect(ACC6).toBe("ACC6");
  });

  it("ACC18 is correct value", () => {
    expect(ACC18).toBe("ACC18");
  });

  it("ACC2152 is correct value", () => {
    expect(ACC2152).toBe("ACC2152");
  });

  it("ACC32 is correct value", () => {
    expect(ACC32).toBe("ACC32");
  });
});
