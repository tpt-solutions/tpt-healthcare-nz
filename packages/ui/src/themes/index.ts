export type ThemeKey = 'clinical' | 'indigo' | 'sage' | 'calm' | 'bloom' | 'amber' | 'ocean' | 'mono';

export interface ThemeMeta {
  key: ThemeKey;
  label: string;
  description: string;
  /** Swatch hex values shown in the picker gallery [primary-600, primary-300, secondary-100] */
  swatches: [string, string, string];
}

export const THEMES: ThemeMeta[] = [
  {
    key: 'clinical',
    label: 'Clinical',
    description: 'Clean teal — GP, hospital, pharmacy',
    swatches: ['#0d9488', '#5eead4', '#f1f5f9'],
  },
  {
    key: 'indigo',
    label: 'Indigo',
    description: 'Authoritative indigo — specialist, occupational health',
    swatches: ['#4f46e5', '#a5b4fc', '#f3f4f6'],
  },
  {
    key: 'sage',
    label: 'Sage',
    description: 'Grounding forest green — allied health, naturopathy, TCM',
    swatches: ['#16a34a', '#86efac', '#f5f5f4'],
  },
  {
    key: 'calm',
    label: 'Calm',
    description: 'Gentle periwinkle — mental health, counselling, palliative',
    swatches: ['#7c3aed', '#c4b5fd', '#f1f5f9'],
  },
  {
    key: 'bloom',
    label: 'Bloom',
    description: 'Soft rose — midwifery, aged care, disability',
    swatches: ['#be185d', '#fda4af', '#f5f5f4'],
  },
  {
    key: 'amber',
    label: 'Amber',
    description: 'Warm amber — acupuncture, massage, chiropractic, nutrition',
    swatches: ['#d97706', '#fcd34d', '#f5f5f4'],
  },
  {
    key: 'ocean',
    label: 'Ocean',
    description: 'Fresh cerulean — dental, vision, optometry',
    swatches: ['#0284c7', '#7dd3fc', '#fef9c3'],
  },
  {
    key: 'mono',
    label: 'Mono',
    description: 'Minimal blue-grey — admin, billing, radiology',
    swatches: ['#475569', '#cbd5e1', '#f8fafc'],
  },
];

export const DEFAULT_THEME: ThemeKey = 'clinical';
