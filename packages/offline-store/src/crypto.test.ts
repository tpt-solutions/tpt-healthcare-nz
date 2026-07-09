import { describe, it, expect, beforeEach, vi } from "vitest";
import { deriveKey, encrypt, decrypt, derivePinVerifier } from "./crypto.js";

// Web Crypto API is available in Node 18+ globals

describe("encrypt / decrypt round-trip", () => {
  let key: CryptoKey;
  const salt = new Uint8Array(32).fill(42);

  beforeEach(async () => {
    key = await deriveKey("1234", salt);
  });

  it("encrypts and decrypts back to original plaintext", async () => {
    const plaintext = "Patient ZAC1234 has diagnosis M54";
    const blob = await encrypt(key, plaintext);
    const decrypted = await decrypt(key, blob);
    expect(decrypted).toBe(plaintext);
  });

  it("produces different ciphertext each time (random IV)", async () => {
    const blob1 = await encrypt(key, "same text");
    const blob2 = await encrypt(key, "same text");
    // Different IVs → different ciphertext
    expect(blob1.iv).not.toEqual(blob2.iv);
    expect(new Uint8Array(blob1.ciphertext)).not.toEqual(new Uint8Array(blob2.ciphertext));
  });

  it("decrypts empty string", async () => {
    const blob = await encrypt(key, "");
    const decrypted = await decrypt(key, blob);
    expect(decrypted).toBe("");
  });

  it("decrypts long text", async () => {
    const long = "A".repeat(10000);
    const blob = await encrypt(key, long);
    const decrypted = await decrypt(key, blob);
    expect(decrypted).toBe(long);
  });

  it("decrypt with wrong key throws", async () => {
    const blob = await encrypt(key, "secret");
    const wrongKey = await deriveKey("9999", salt);
    await expect(decrypt(wrongKey, blob)).rejects.toThrow();
  });
});

describe("deriveKey", () => {
  const salt = new Uint8Array(32).fill(1);

  it("returns a CryptoKey", async () => {
    const k = await deriveKey("pin", salt);
    expect(k).toBeDefined();
    expect(k.type).toBe("secret");
  });

  it("same pin+salt produces same key", async () => {
    const k1 = await deriveKey("pin", salt);
    const k2 = await deriveKey("pin", salt);
    // Both should successfully decrypt the same blob
    const blob = await encrypt(k1, "test");
    const dec = await decrypt(k2, blob);
    expect(dec).toBe("test");
  });

  it("different pins produce different keys", async () => {
    const k1 = await deriveKey("pin1", salt);
    const k2 = await deriveKey("pin2", salt);
    const blob = await encrypt(k1, "test");
    await expect(decrypt(k2, blob)).rejects.toThrow();
  });
});

describe("derivePinVerifier", () => {
  const salt = new Uint8Array(32).fill(7);

  it("returns a hex string", async () => {
    const verifier = await derivePinVerifier("1234", salt);
    expect(verifier).toMatch(/^[0-9a-f]{64}$/);
  });

  it("same pin+salt produces same verifier", async () => {
    const v1 = await derivePinVerifier("1234", salt);
    const v2 = await derivePinVerifier("1234", salt);
    expect(v1).toBe(v2);
  });

  it("different pins produce different verifiers", async () => {
    const v1 = await derivePinVerifier("1234", salt);
    const v2 = await derivePinVerifier("5678", salt);
    expect(v1).not.toBe(v2);
  });
});
