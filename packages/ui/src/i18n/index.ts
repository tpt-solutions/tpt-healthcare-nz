/**
 * i18n — Lightweight internationalisation for the tpt patient portal and clinic app.
 *
 * Supported languages (BCP 47):
 *   en    — English (default)
 *   mi    — Te Reo Māori
 *   sm    — Samoan (Gagana Samoa)
 *   to    — Tongan (Lea Fakatonga)
 *   zh    — Mandarin Chinese (Simplified)
 *
 * Translation files live alongside this file as JSON:
 *   src/i18n/en.json
 *   src/i18n/mi.json
 *   src/i18n/sm.json
 *   src/i18n/to.json
 *   src/i18n/zh.json
 *
 * Usage:
 *   import { useTranslation } from '@tpt/ui/i18n'
 *   const { t, language, setLanguage } = useTranslation()
 *   <h1>{t('portal.dashboard.title')}</h1>
 *
 * AI-assisted translation:
 *   When the AI feature flag "translation" is enabled for the tenant, missing
 *   translation keys are dynamically filled via the /api/v1/ai/translate endpoint
 *   and cached in localStorage. This allows rapid expansion of language coverage
 *   without manual translation for every key.
 */

import { createContext, useCallback, useContext, useEffect, useState } from 'react'

export type SupportedLanguage = 'en' | 'mi' | 'sm' | 'to' | 'zh'

export const SUPPORTED_LANGUAGES: Record<SupportedLanguage, string> = {
  en: 'English',
  mi: 'Te Reo Māori',
  sm: 'Gagana Samoa',
  to: 'Lea Fakatonga',
  zh: '中文',
}

type TranslationMap = Record<string, string>

interface I18nContextValue {
  language: SupportedLanguage
  setLanguage: (lang: SupportedLanguage) => void
  t: (key: string, vars?: Record<string, string>) => string
  isLoading: boolean
}

const I18nContext = createContext<I18nContextValue>({
  language: 'en',
  setLanguage: () => undefined,
  t: (key) => key,
  isLoading: false,
})

const STORAGE_KEY = 'tpt:language'
const AI_CACHE_KEY = 'tpt:ai-translations'

type Loader = () => Promise<{ default: TranslationMap }>

const loaders: Record<SupportedLanguage, Loader> = {
  en: () => import('./en.json'),
  mi: () => import('./mi.json'),
  sm: () => import('./sm.json'),
  to: () => import('./to.json'),
  zh: () => import('./zh.json'),
}

/**
 * Detect the user's preferred language from browser settings, falling back
 * to 'en'. Only returns a language we actually support.
 */
export function detectLanguage(): SupportedLanguage {
  const stored = localStorage.getItem(STORAGE_KEY) as SupportedLanguage | null
  if (stored && stored in SUPPORTED_LANGUAGES) return stored

  const nav = navigator.language.slice(0, 2) as SupportedLanguage
  if (nav in SUPPORTED_LANGUAGES) return nav
  return 'en'
}

/**
 * useI18n — React hook that provides the current language and t() function.
 * Mount the I18nProvider at the app root; consume this hook in components.
 */
export function useTranslation(): I18nContextValue {
  return useContext(I18nContext)
}

/**
 * loadTranslations loads the translation JSON for the given language.
 * Falls back to English if the language file is not found.
 */
async function loadTranslations(lang: SupportedLanguage): Promise<TranslationMap> {
  try {
    const mod = await loaders[lang]()
    return mod.default
  } catch {
    if (lang !== 'en') {
      const fallback = await loaders.en()
      return fallback.default
    }
    return {}
  }
}

/**
 * interpolate replaces {{key}} placeholders in a translation string.
 */
function interpolate(str: string, vars: Record<string, string> = {}): string {
  return str.replace(/\{\{(\w+)\}\}/g, (_, k) => vars[k] ?? `{{${k}}}`)
}

/**
 * I18nProvider — wrap your app root with this to enable i18n.
 *
 * @param aiTranslateUrl — optional URL for AI-assisted translation fallback.
 *   When provided, keys missing from the loaded translation file are fetched
 *   from this endpoint and cached in localStorage.
 */
export function createI18nProvider(aiTranslateUrl?: string) {
  // This is a factory rather than a JSX component to keep this file framework-
  // agnostic at the type level. The actual React component is in I18nProvider.tsx.
  return { aiTranslateUrl }
}

/**
 * buildTranslator creates a t() function for the given translations map.
 * Missing keys are returned as the key itself (so the UI degrades gracefully).
 * If aiTranslateUrl is set and a key is missing, a background fetch is queued.
 */
export function buildTranslator(
  translations: TranslationMap,
  language: SupportedLanguage,
  aiTranslateUrl?: string
): (key: string, vars?: Record<string, string>) => string {
  // Load AI cache from localStorage.
  let aiCache: TranslationMap = {}
  try {
    const raw = localStorage.getItem(`${AI_CACHE_KEY}:${language}`)
    if (raw) aiCache = JSON.parse(raw)
  } catch { /* ignore */ }

  const pending = new Set<string>()

  return function t(key: string, vars?: Record<string, string>): string {
    const value = translations[key] ?? aiCache[key]
    if (value) return interpolate(value, vars)

    // Queue an AI translation for missing key.
    if (aiTranslateUrl && language !== 'en' && !pending.has(key)) {
      pending.add(key)
      // Fetch the English string first, then translate.
      const enValue = key // key itself as fallback English text
      fetch(aiTranslateUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: enValue, sourceLanguage: 'en', targetLanguage: language }),
      })
        .then(r => r.json())
        .then(({ text }: { text: string }) => {
          aiCache[key] = text
          localStorage.setItem(`${AI_CACHE_KEY}:${language}`, JSON.stringify(aiCache))
        })
        .catch(() => { /* silently fail AI translation */ })
    }

    // Return the key as a human-readable fallback.
    return key.split('.').pop() ?? key
  }
}

export { I18nContext }
export type { I18nContextValue }
