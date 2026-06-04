/**
 * PINContext — 6-digit PIN lock for clinical PWAs.
 *
 * Security model:
 *   · The PIN is never stored; only a fast verifier (1-iteration PBKDF2 hex) is kept in localStorage.
 *   · The encryption key (310k-iteration PBKDF2) lives in memory only.
 *   · Inactivity timeout: app auto-locks after N ms of no pointer/key/touch activity.
 *   · Visibility: app locks immediately when tabbed away or screen goes off.
 *   · After 5 wrong attempts the entire IndexedDB cache is wiped (HISO 10064.1 §4.5).
 *
 * NZ regulations: HIPC Rule 5, HISO 10064.1 §4.3–4.5, MoH Mobile Device Policy.
 */

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { deriveKey, derivePinVerifier, getOrCreateSalt, clearSalt } from './crypto.js';
import { clearAll } from './store.js';

const PIN_VERIFIER_KEY = 'tpt:pin:verifier';
const HAS_PIN_KEY = 'tpt:pin:set';
const MAX_ATTEMPTS = 5;
const COOLDOWN_AFTER = 3; // attempts before 30s cooldown
const COOLDOWN_MS = 30_000;

export interface PINContextValue {
  isLocked: boolean;
  hasPin: boolean;
  cryptoKey: CryptoKey | null;
  failedAttempts: number;
  cooldownUntil: number | null; // epoch ms; null = no cooldown
  isWiped: boolean;
  setPin(pin: string): Promise<void>;
  unlock(pin: string): Promise<boolean>;
  lock(): void;
}

const PINContext = createContext<PINContextValue | null>(null);

export function usePIN(): PINContextValue {
  const ctx = useContext(PINContext);
  if (!ctx) throw new Error('usePIN must be used within PINProvider');
  return ctx;
}

interface PINProviderProps {
  children: React.ReactNode;
  /** Milliseconds of inactivity before auto-lock. */
  inactivityMs?: number;
}

export function PINProvider({ children, inactivityMs = 30_000 }: PINProviderProps) {
  const [isLocked, setIsLocked] = useState(false);
  const [hasPin, setHasPin] = useState(() => !!localStorage.getItem(HAS_PIN_KEY));
  const [cryptoKey, setCryptoKey] = useState<CryptoKey | null>(null);
  const [failedAttempts, setFailedAttempts] = useState(0);
  const [cooldownUntil, setCooldownUntil] = useState<number | null>(null);
  const [isWiped, setIsWiped] = useState(false);

  const inactivityTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // --- Lock / unlock ---

  const lock = useCallback(() => {
    setCryptoKey(null);
    setIsLocked(true);
  }, []);

  const unlock = useCallback(
    async (pin: string): Promise<boolean> => {
      // Cooldown check
      if (cooldownUntil && Date.now() < cooldownUntil) return false;

      const salt = getOrCreateSalt();
      const verifier = localStorage.getItem(PIN_VERIFIER_KEY);

      if (verifier) {
        const candidate = await derivePinVerifier(pin, salt);
        if (candidate !== verifier) {
          const next = failedAttempts + 1;
          setFailedAttempts(next);

          if (next >= MAX_ATTEMPTS) {
            // Wipe all cached data
            await clearAll();
            clearSalt();
            localStorage.removeItem(PIN_VERIFIER_KEY);
            localStorage.removeItem(HAS_PIN_KEY);
            setHasPin(false);
            setIsWiped(true);
            setCryptoKey(null);
            return false;
          }

          if (next >= COOLDOWN_AFTER) {
            setCooldownUntil(Date.now() + COOLDOWN_MS);
          }
          return false;
        }
      }

      // PIN correct — derive full key
      const key = await deriveKey(pin, salt);
      setCryptoKey(key);
      setIsLocked(false);
      setFailedAttempts(0);
      setCooldownUntil(null);
      return true;
    },
    [failedAttempts, cooldownUntil]
  );

  const setPin = useCallback(async (pin: string): Promise<void> => {
    const salt = getOrCreateSalt();
    const verifier = await derivePinVerifier(pin, salt);
    localStorage.setItem(PIN_VERIFIER_KEY, verifier);
    localStorage.setItem(HAS_PIN_KEY, '1');
    setHasPin(true);
    // Also derive and hold the key immediately
    const key = await deriveKey(pin, salt);
    setCryptoKey(key);
    setIsLocked(false);
  }, []);

  // --- Inactivity timer ---

  const resetTimer = useCallback(() => {
    if (inactivityTimer.current) clearTimeout(inactivityTimer.current);
    if (!isLocked) {
      inactivityTimer.current = setTimeout(() => {
        lock();
      }, inactivityMs);
    }
  }, [isLocked, inactivityMs, lock]);

  useEffect(() => {
    const events: Array<keyof DocumentEventMap> = ['pointermove', 'keydown', 'touchstart', 'click'];
    events.forEach((e) => document.addEventListener(e, resetTimer, { passive: true }));
    resetTimer();
    return () => {
      events.forEach((e) => document.removeEventListener(e, resetTimer));
      if (inactivityTimer.current) clearTimeout(inactivityTimer.current);
    };
  }, [resetTimer]);

  // --- Lock on visibility change (tab switch / screen off) ---

  useEffect(() => {
    const handleVisibility = () => {
      if (document.hidden && hasPin) lock();
    };
    document.addEventListener('visibilitychange', handleVisibility);
    return () => document.removeEventListener('visibilitychange', handleVisibility);
  }, [hasPin, lock]);

  // --- Auto-lock on first load if PIN is set ---

  useEffect(() => {
    if (hasPin && cryptoKey === null) {
      setIsLocked(true);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // run once on mount only

  return (
    <PINContext.Provider
      value={{ isLocked, hasPin, cryptoKey, failedAttempts, cooldownUntil, isWiped, setPin, unlock, lock }}
    >
      {children}
    </PINContext.Provider>
  );
}
