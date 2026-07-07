/**
 * AES-256-GCM encryption helpers for PHI cached in IndexedDB.
 *
 * Key derivation: PBKDF2-SHA-256 with 310 000 iterations (NIST SP 800-132 2023).
 * Keys are non-extractable CryptoKey objects — they never leave the JS heap.
 *
 * Regulatory context:
 *   HISO 10064.1 §4.3 — encryption required for PHI on portable devices
 *   HIPC Rule 5       — health information must be secured against unauthorised access
 */

const PBKDF2_ITERATIONS = 310_000;
const SALT_LOCAL_KEY = 'tpt:crypto:salt';

/** Retrieve or generate the stable per-device salt (non-sensitive; public). */
export function getOrCreateSalt(): Uint8Array {
  const stored = localStorage.getItem(SALT_LOCAL_KEY);
  if (stored) {
    return Uint8Array.from(atob(stored), (c) => c.charCodeAt(0));
  }
  const salt = crypto.getRandomValues(new Uint8Array(32));
  localStorage.setItem(SALT_LOCAL_KEY, btoa(String.fromCharCode(...salt)));
  return salt;
}

/** Clear the device salt — called on full wipe to invalidate all cached ciphertext. */
export function clearSalt(): void {
  localStorage.removeItem(SALT_LOCAL_KEY);
}

/**
 * Derive an AES-256-GCM CryptoKey from the user's PIN.
 * The key is non-extractable — it cannot be serialised or exported.
 */
export async function deriveKey(pin: string, salt: Uint8Array): Promise<CryptoKey> {
  const enc = new TextEncoder();
  const keyMaterial = await crypto.subtle.importKey(
    'raw',
    enc.encode(pin),
    { name: 'PBKDF2' },
    false,
    ['deriveKey']
  );
  return crypto.subtle.deriveKey(
    {
      name: 'PBKDF2',
      salt: salt as BufferSource,
      iterations: PBKDF2_ITERATIONS,
      hash: 'SHA-256',
    },
    keyMaterial,
    { name: 'AES-GCM', length: 256 },
    false, // non-extractable
    ['encrypt', 'decrypt']
  );
}

export interface EncryptedBlob {
  iv: Uint8Array;
  ciphertext: ArrayBuffer;
}

/** Encrypt a plaintext string under the given key. */
export async function encrypt(key: CryptoKey, plaintext: string): Promise<EncryptedBlob> {
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const ciphertext = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    key,
    new TextEncoder().encode(plaintext)
  );
  return { iv, ciphertext };
}

/** Decrypt an EncryptedBlob back to a plaintext string. Throws on wrong key. */
export async function decrypt(key: CryptoKey, blob: EncryptedBlob): Promise<string> {
  const plainBuffer = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: blob.iv as BufferSource },
    key,
    blob.ciphertext
  );
  return new TextDecoder().decode(plainBuffer);
}

/**
 * Derive a fast PIN verifier for checking correctness before full key derivation.
 * Uses 1 PBKDF2 iteration — fast enough for UX, still PIN-dependent.
 * Stored as hex in localStorage; not usable as an encryption key.
 */
export async function derivePinVerifier(pin: string, salt: Uint8Array): Promise<string> {
  const enc = new TextEncoder();
  const km = await crypto.subtle.importKey('raw', enc.encode(pin), { name: 'PBKDF2' }, false, ['deriveBits']);
  const bits = await crypto.subtle.deriveBits(
    { name: 'PBKDF2', salt: salt as BufferSource, iterations: 1, hash: 'SHA-256' },
    km,
    256
  );
  return Array.from(new Uint8Array(bits))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}
