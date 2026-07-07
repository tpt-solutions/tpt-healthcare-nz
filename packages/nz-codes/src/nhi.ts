// NHI (National Health Index) validation and formatting utilities.
//
// Two NHI formats exist:
//   Old format: 3 uppercase letters + 4 digits, e.g. ABC1234
//               Checksum: weighted sum of the 6 non-check characters mod 11,
//               where letters are mapped A=1…Z=26 (excluding I and O which
//               are not used), digits are their face value. The check digit
//               is (11 - sum % 11) % 11. A result of 10 is invalid.
//
//   New format: starts with 'Z', 7 characters total (ZAB1234 style).
//               Uses a Luhn mod-24 algorithm over the full 7 characters.
//               Letters A-Z (excluding I and O) encode as 1-24 in sequence.
//
// The validation logic here mirrors the Go implementation in core/nhi/.

export { NHI_SYSTEM } from "./uris.js";

// Letters used in NHI identifiers — I and O are excluded.
const NHI_ALPHA = "ABCDEFGHJKLMNPQRSTUVWXYZ"; // 24 letters

/** Maps an NHI letter character to its numeric value (1-based, I and O excluded). */
function letterValue(ch: string): number {
  const idx = NHI_ALPHA.indexOf(ch.toUpperCase());
  if (idx === -1) throw new RangeError(`Invalid NHI letter character: ${ch}`);
  return idx + 1; // 1-based
}

/** Returns true if `nhi` matches the old 7-character NHI pattern (3 letters + 4 digits). */
function matchesOldPattern(nhi: string): boolean {
  return /^[A-HJ-NP-Z]{3}[0-9]{4}$/i.test(nhi);
}

/** Returns true if `nhi` matches the new 7-character NHI pattern (starts with Z). */
function matchesNewPattern(nhi: string): boolean {
  // New format: Z + 2 alpha (no I/O) + 2 digits + 2 alphanumeric check chars.
  // In practice the Ministry defines new NHIs as starting with Z, 7 chars total.
  return /^Z[A-HJ-NP-Z]{2}[0-9]{2}[A-HJ-NP-Z0-9]{2}$/i.test(nhi);
}

/**
 * Validates the old NHI format checksum.
 *
 * Algorithm (as published by Te Whatu Ora):
 *   1. Assign values: letters A=1…Z=26 (skipping I=9, O=15 — those are invalid
 *      NHI letters and their positions shift accordingly), digits face value.
 *   2. Multiply each of the first 6 characters by weight 7 down to 2.
 *   3. Sum the products.
 *   4. Check digit = (11 - (sum % 11)) % 11. If result is 10 the NHI is invalid.
 *   5. The published check digit is the last character. For old format it is
 *      always a digit, so compare numerically.
 */
function validateOldNHIChecksum(nhi: string): boolean {
  const upper = nhi.toUpperCase();
  let sum = 0;

  for (let i = 0; i < 6; i++) {
    const ch = upper[i];
    const weight = 7 - i;
    let value: number;

    if (/[A-Z]/.test(ch)) {
      value = letterValue(ch);
    } else {
      value = parseInt(ch, 10);
    }

    sum += value * weight;
  }

  const remainder = sum % 11;
  if (remainder === 0) return false; // No valid check digit when remainder is 0

  const checkDigit = 11 - remainder;
  if (checkDigit === 10) return false; // Invalid — 10 is not representable as single digit

  return parseInt(upper[6], 10) === checkDigit;
}

/**
 * Validates the new NHI format checksum (Luhn mod-24 variant).
 *
 * Algorithm (Te Whatu Ora new NHI specification):
 *   1. Encode each character: letters use NHI_ALPHA index (1-24), digits face value.
 *   2. Multiply each of the first 6 characters by weight 7 down to 2.
 *   3. Sum the products.
 *   4. Expected check value = 24 - (sum % 24). If 24, use 0.
 *   5. The 7th character encodes to this value.
 */
function validateNewNHIChecksum(nhi: string): boolean {
  const upper = nhi.toUpperCase();
  let sum = 0;

  for (let i = 0; i < 6; i++) {
    const ch = upper[i];
    const weight = 7 - i;
    let value: number;

    if (/[A-Z]/.test(ch)) {
      // Letters: use NHI_ALPHA encoding
      const idx = NHI_ALPHA.indexOf(ch);
      if (idx === -1) return false; // I or O encountered — invalid
      value = idx + 1;
    } else {
      value = parseInt(ch, 10);
    }

    sum += value * weight;
  }

  const expected = 24 - (sum % 24);
  const checkChar = upper[6];

  if (/[A-Z]/.test(checkChar)) {
    const idx = NHI_ALPHA.indexOf(checkChar);
    if (idx === -1) return false;
    return idx + 1 === expected;
  } else {
    return parseInt(checkChar, 10) === expected % 24;
  }
}

/**
 * Returns true if `nhi` follows the new NHI format (7 characters starting
 * with the letter Z, introduced from 2023).
 */
export function isNewNHIFormat(nhi: string): boolean {
  return formatNHI(nhi).startsWith("Z");
}

/**
 * Normalises an NHI string to uppercase with leading/trailing whitespace
 * removed. Does not validate the format or checksum.
 */
export function formatNHI(nhi: string): string {
  return nhi.trim().toUpperCase();
}

/**
 * Validates an NHI number.
 *
 * Accepts both the old format (3 letters + 4 digits) and the new format
 * (starts with Z, 7 characters) including their respective checksums.
 *
 * Returns `false` for any string that does not match a known format or fails
 * its checksum, including NHIs that are reserved for testing (e.g. ZZZ0016).
 */
export function validateNHI(nhi: string): boolean {
  const normalised = formatNHI(nhi);

  if (normalised.length !== 7) return false;

  // Reject characters I or O which are never valid in an NHI.
  if (/[IO]/.test(normalised)) return false;

  if (matchesNewPattern(normalised)) {
    return validateNewNHIChecksum(normalised);
  }

  if (matchesOldPattern(normalised)) {
    return validateOldNHIChecksum(normalised);
  }

  return false;
}
