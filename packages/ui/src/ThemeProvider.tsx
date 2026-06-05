import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';
import { DEFAULT_THEME, type ThemeKey } from './themes/index';

// ── Storage key ────────────────────────────────────────────────────────────
const LS_KEY = 'tpt:theme';

// ── Types ──────────────────────────────────────────────────────────────────
interface ThemeContextValue {
  theme: ThemeKey;
  customAccent: string | null;
  /** Validate then stage a custom accent. Returns an error message or null on success. */
  validateAccent: (hex: string) => string | null;
  applyTheme: (theme: ThemeKey, customAccent?: string | null) => void;
}

// ── HSL primary scale derivation ───────────────────────────────────────────
// Lightness stops for primary-50 through primary-950.
const L_STOPS: [string, number][] = [
  ['50', 97], ['100', 94], ['200', 89], ['300', 80], ['400', 65],
  ['500', 52], ['600', 40], ['700', 32], ['800', 24], ['900', 15], ['950', 9],
];

function hexToHsl(hex: string): [number, number, number] {
  const n = parseInt(hex.slice(1), 16);
  const r = (n >> 16) / 255;
  const g = ((n >> 8) & 0xff) / 255;
  const b = (n & 0xff) / 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const l = (max + min) / 2;
  if (max === min) return [0, 0, Math.round(l * 100)];
  const d = max - min;
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
  let h = 0;
  if (max === r) h = ((g - b) / d + (g < b ? 6 : 0)) / 6;
  else if (max === g) h = ((b - r) / d + 2) / 6;
  else h = ((r - g) / d + 4) / 6;
  return [Math.round(h * 360), Math.round(s * 100), Math.round(l * 100)];
}

function buildPrimaryScale(hex: string): Record<string, string> {
  const [h, s] = hexToHsl(hex);
  const vars: Record<string, string> = {};
  for (const [stop, l] of L_STOPS) {
    vars[`--primary-${stop}`] = `hsl(${h} ${s}% ${l}%)`;
  }
  return vars;
}

// ── WCAG contrast guard ────────────────────────────────────────────────────
function linearise(c: number): number {
  return c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

function relativeLuminance(hex: string): number {
  const n = parseInt(hex.slice(1), 16);
  const r = linearise((n >> 16) / 255);
  const g = linearise(((n >> 8) & 0xff) / 255);
  const b = linearise((n & 0xff) / 255);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrastRatio(l1: number, l2: number): number {
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
}

// Clinical alert hues that custom accents must not clash with:
// red≈0°, amber≈38°, green≈142°, blue≈217°
const LOCKED_HUES = [0, 38, 142, 217];
const HUE_EXCLUSION_ZONE = 20; // degrees

export function validateAccentHex(hex: string): string | null {
  if (!/^#[0-9a-fA-F]{6}$/.test(hex)) return 'Must be a 6-digit hex colour e.g. #0d9488';

  // Enforce WCAG AA against white (#ffffff, luminance = 1).
  const accentL = relativeLuminance(hex);
  const whiteL = 1;
  if (contrastRatio(accentL, whiteL) < 4.5) {
    return 'Colour is too light — minimum 4.5:1 contrast against white required (WCAG AA). Try a darker shade.';
  }

  // Reject hues that clash with locked clinical alert colours.
  const [h] = hexToHsl(hex);
  for (const locked of LOCKED_HUES) {
    const diff = Math.min(Math.abs(h - locked), 360 - Math.abs(h - locked));
    if (diff < HUE_EXCLUSION_ZONE) {
      return `This colour is too close to a clinical alert colour (hue ${locked}°). Choose a different hue to avoid confusion with urgent/warning/safe indicators.`;
    }
  }
  return null;
}

// ── localStorage persistence ───────────────────────────────────────────────
interface Persisted { theme: ThemeKey; customAccent: string | null }

function readCache(): Persisted {
  try {
    const raw = localStorage.getItem(LS_KEY);
    if (raw) return JSON.parse(raw) as Persisted;
  } catch { /* ignore */ }
  return { theme: DEFAULT_THEME, customAccent: null };
}

function writeCache(p: Persisted) {
  try { localStorage.setItem(LS_KEY, JSON.stringify(p)); } catch { /* ignore */ }
}

// ── Apply to DOM ───────────────────────────────────────────────────────────
function applyToDom(theme: ThemeKey, customAccent: string | null) {
  const root = document.documentElement;
  root.dataset.theme = theme;

  // Remove any previously injected custom-accent variables.
  const existing = root.style.cssText
    .split(';')
    .filter(s => !s.trim().startsWith('--primary-'))
    .join(';');
  root.style.cssText = existing;

  if (customAccent) {
    const scale = buildPrimaryScale(customAccent);
    // Determine foreground: white if contrast ≥ 4.5 against white, else dark.
    const accentL = relativeLuminance(customAccent);
    const foreground = contrastRatio(accentL, 1) >= 4.5 ? '#ffffff' : '#0f172a';
    for (const [k, v] of Object.entries(scale)) {
      root.style.setProperty(k, v);
    }
    root.style.setProperty('--primary-foreground', foreground);
  } else {
    root.style.removeProperty('--primary-foreground');
  }
}

// ── Context ────────────────────────────────────────────────────────────────
const ThemeContext = createContext<ThemeContextValue>({
  theme: DEFAULT_THEME,
  customAccent: null,
  validateAccent: validateAccentHex,
  applyTheme: () => undefined,
});

export function useTheme() {
  return useContext(ThemeContext);
}

// ── Provider ───────────────────────────────────────────────────────────────
interface ThemeProviderProps {
  children: React.ReactNode;
  /** Base URL of the practice API. Defaults to '' (same origin). */
  apiBase?: string;
}

export function ThemeProvider({ children, apiBase = '' }: ThemeProviderProps) {
  const cached = readCache();
  const [theme, setTheme] = useState<ThemeKey>(cached.theme);
  const [customAccent, setCustomAccent] = useState<string | null>(cached.customAccent);
  const appliedRef = useRef(false);

  // Apply cached values immediately on first render to avoid flash.
  if (!appliedRef.current) {
    applyToDom(cached.theme, cached.customAccent);
    appliedRef.current = true;
  }

  // Fetch from API and reconcile with cache.
  useEffect(() => {
    const controller = new AbortController();
    fetch(`${apiBase}/api/v1/practice/settings`, { signal: controller.signal })
      .then(r => r.ok ? r.json() : null)
      .then((data: { theme?: string; customAccent?: string | null } | null) => {
        if (!data) return;
        const t = (data.theme as ThemeKey | undefined) ?? DEFAULT_THEME;
        const a = data.customAccent ?? null;
        setTheme(t);
        setCustomAccent(a);
        applyToDom(t, a);
        writeCache({ theme: t, customAccent: a });
      })
      .catch(() => { /* network unavailable — keep cached */ });
    return () => controller.abort();
  }, [apiBase]);

  const applyTheme = useCallback((t: ThemeKey, accent?: string | null) => {
    const a = accent !== undefined ? accent : customAccent;
    setTheme(t);
    setCustomAccent(a ?? null);
    applyToDom(t, a ?? null);
    writeCache({ theme: t, customAccent: a ?? null });
  }, [customAccent]);

  return (
    <ThemeContext.Provider value={{ theme, customAccent, validateAccent: validateAccentHex, applyTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}
