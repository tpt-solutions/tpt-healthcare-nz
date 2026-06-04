/**
 * LockScreen — full-screen PIN entry overlay for clinical PWAs.
 *
 * Features:
 *   · On-screen numeric keypad (works on tablets without keyboard)
 *   · 6 dot progress indicators
 *   · Shake animation on wrong PIN
 *   · Cooldown timer after 3 wrong attempts
 *   · Wipe message after 5 wrong attempts
 *   · First-time PIN setup flow (enter + confirm)
 */

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { usePIN } from './pin-context.js';

const PIN_LENGTH = 6;

interface SetPINProps {
  onSet: (pin: string) => Promise<void>;
}

function SetPINFlow({ onSet }: SetPINProps) {
  const [step, setStep] = useState<'enter' | 'confirm'>('enter');
  const [firstPin, setFirstPin] = useState('');
  const [digits, setDigits] = useState('');
  const [error, setError] = useState('');

  const handleDigit = (d: string) => {
    if (digits.length >= PIN_LENGTH) return;
    const next = digits + d;
    setDigits(next);
    if (next.length === PIN_LENGTH) {
      setTimeout(() => {
        if (step === 'enter') {
          setFirstPin(next);
          setDigits('');
          setStep('confirm');
        } else {
          if (next === firstPin) {
            onSet(next);
          } else {
            setError('PINs do not match — please try again');
            setDigits('');
            setStep('enter');
            setFirstPin('');
          }
        }
      }, 120);
    }
  };

  return (
    <div className="flex flex-col items-center gap-6">
      <h2 className="text-xl font-bold text-white">
        {step === 'enter' ? 'Set a 6-digit PIN' : 'Confirm your PIN'}
      </h2>
      <p className="text-white/70 text-sm text-center max-w-xs">
        {step === 'enter'
          ? 'You will need this PIN each time you open the app or after a few seconds away.'
          : 'Enter the same PIN again to confirm.'}
      </p>
      {error && <p className="text-red-300 text-sm">{error}</p>}
      <DotIndicator count={digits.length} />
      <Keypad onDigit={handleDigit} onBackspace={() => setDigits((d) => d.slice(0, -1))} />
    </div>
  );
}

interface UnlockFlowProps {
  failedAttempts: number;
  cooldownUntil: number | null;
  isWiped: boolean;
  onUnlock: (pin: string) => Promise<boolean>;
}

function UnlockFlow({ failedAttempts, cooldownUntil, isWiped, onUnlock }: UnlockFlowProps) {
  const [digits, setDigits] = useState('');
  const [shake, setShake] = useState(false);
  const [cooldownLeft, setCooldownLeft] = useState(0);

  useEffect(() => {
    if (!cooldownUntil) return;
    const update = () => {
      const left = Math.max(0, Math.ceil((cooldownUntil - Date.now()) / 1000));
      setCooldownLeft(left);
    };
    update();
    const id = setInterval(update, 500);
    return () => clearInterval(id);
  }, [cooldownUntil]);

  const handleDigit = useCallback(
    (d: string) => {
      if (cooldownLeft > 0) return;
      if (digits.length >= PIN_LENGTH) return;
      const next = digits + d;
      setDigits(next);
      if (next.length === PIN_LENGTH) {
        setTimeout(async () => {
          const ok = await onUnlock(next);
          if (!ok) {
            setShake(true);
            setTimeout(() => setShake(false), 600);
            setDigits('');
          }
        }, 120);
      }
    },
    [digits, cooldownLeft, onUnlock]
  );

  if (isWiped) {
    return (
      <div className="flex flex-col items-center gap-4 text-center">
        <div className="h-16 w-16 rounded-full bg-red-500/20 flex items-center justify-center">
          <svg className="h-8 w-8 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
          </svg>
        </div>
        <h2 className="text-xl font-bold text-white">Device data cleared</h2>
        <p className="text-white/70 text-sm max-w-xs">
          Too many incorrect PIN attempts. Cached patient data has been securely wiped.
          Please sign in again.
        </p>
        <button
          onClick={() => window.location.replace('/login')}
          className="mt-4 px-6 py-3 bg-white text-gray-900 font-semibold rounded-xl"
        >
          Sign in
        </button>
      </div>
    );
  }

  const remaining = MAX_ATTEMPTS - failedAttempts;

  return (
    <div className="flex flex-col items-center gap-6">
      <h2 className="text-xl font-bold text-white">Enter your PIN</h2>

      {failedAttempts > 0 && (
        <p className="text-red-300 text-sm">
          {cooldownLeft > 0
            ? `Too many attempts — try again in ${cooldownLeft}s`
            : `Incorrect PIN — ${remaining} attempt${remaining !== 1 ? 's' : ''} remaining`}
        </p>
      )}

      <div className={shake ? 'animate-shake' : ''}>
        <DotIndicator count={digits.length} />
      </div>

      <Keypad
        onDigit={handleDigit}
        onBackspace={() => setDigits((d) => d.slice(0, -1))}
        disabled={cooldownLeft > 0}
      />
    </div>
  );
}

const MAX_ATTEMPTS = 5;

function DotIndicator({ count }: { count: number }) {
  return (
    <div className="flex gap-3">
      {Array.from({ length: PIN_LENGTH }).map((_, i) => (
        <div
          key={i}
          className={`h-4 w-4 rounded-full border-2 transition-colors ${
            i < count ? 'bg-white border-white' : 'border-white/50 bg-transparent'
          }`}
        />
      ))}
    </div>
  );
}

const KEYS = [
  ['1', '2', '3'],
  ['4', '5', '6'],
  ['7', '8', '9'],
  ['', '0', '⌫'],
];

interface KeypadProps {
  onDigit(d: string): void;
  onBackspace(): void;
  disabled?: boolean;
}

function Keypad({ onDigit, onBackspace, disabled }: KeypadProps) {
  return (
    <div className="grid gap-3">
      {KEYS.map((row, ri) => (
        <div key={ri} className="flex gap-3 justify-center">
          {row.map((k, ki) => {
            if (k === '') return <div key={ki} className="h-16 w-16" />;
            const isBack = k === '⌫';
            return (
              <button
                key={ki}
                onClick={() => (isBack ? onBackspace() : onDigit(k))}
                disabled={disabled}
                className={`h-16 w-16 rounded-full text-xl font-semibold transition-colors select-none ${
                  disabled
                    ? 'bg-white/10 text-white/30 cursor-not-allowed'
                    : 'bg-white/20 text-white active:bg-white/40 hover:bg-white/30'
                }`}
              >
                {k}
              </button>
            );
          })}
        </div>
      ))}
    </div>
  );
}

/**
 * LockScreenOverlay — renders over the app when locked.
 * Renders null when app is unlocked, so there is zero overhead in normal operation.
 */
export function LockScreenOverlay() {
  const { isLocked, hasPin, failedAttempts, cooldownUntil, isWiped, setPin, unlock } = usePIN();

  if (!isLocked && !isWiped) return null;

  return (
    <div
      className="fixed inset-0 z-[9999] flex flex-col items-center justify-center bg-teal-700"
      style={{ WebkitTapHighlightColor: 'transparent' }}
    >
      {/* Logo */}
      <div className="mb-10 flex flex-col items-center gap-2">
        <div className="h-14 w-14 rounded-2xl bg-white/20 flex items-center justify-center">
          <span className="text-2xl font-black text-white">T</span>
        </div>
        <p className="text-white/60 text-sm">TPT Health</p>
      </div>

      {!hasPin || isWiped ? (
        <SetPINFlow onSet={setPin} />
      ) : (
        <UnlockFlow
          failedAttempts={failedAttempts}
          cooldownUntil={cooldownUntil}
          isWiped={isWiped}
          onUnlock={unlock}
        />
      )}
    </div>
  );
}
