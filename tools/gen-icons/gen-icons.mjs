/**
 * gen-icons.mjs — Generate PWA icon PNGs for tpt-clinic, tpt-portal, and tpt-admin.
 *
 * Usage:
 *   cd tools/gen-icons && npm install && node gen-icons.mjs
 *   OR from repo root: make icons
 *
 * Output (per app):
 *   apps/<app>/public/icons/icon-192.png        — PWA manifest icon
 *   apps/<app>/public/icons/icon-512.png        — PWA manifest icon (large / maskable)
 *   apps/<app>/public/icons/apple-touch-icon.png — iOS home screen (180×180)
 *   apps/<app>/public/icons/badge-72.png        — Push notification badge
 *   apps/<app>/public/icons/favicon-32.png      — Browser tab favicon
 */

import sharp from 'sharp';
import { mkdir, writeFile } from 'fs/promises';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '../..');

/** App configs — accent strip colour distinguishes each app at a glance. */
const APPS = [
  {
    name: 'tpt-clinic',
    label: 'C',
    accent: '#0f766e', // teal-700 — clinical
    outDir: 'apps/tpt-clinic/public/icons',
  },
  {
    name: 'tpt-portal',
    label: 'H',
    accent: '#1d4ed8', // blue-700 — patient health
    outDir: 'apps/tpt-portal/public/icons',
  },
  {
    name: 'tpt-admin',
    label: 'A',
    accent: '#1e293b', // slate-800 — administration
    outDir: 'apps/tpt-admin/public/icons',
  },
];

/** Sizes to generate for each app. */
const SIZES = [
  { filename: 'icon-192.png',         size: 192, isMaskable: false },
  { filename: 'icon-512.png',         size: 512, isMaskable: true  },
  { filename: 'apple-touch-icon.png', size: 180, isMaskable: false },
  { filename: 'badge-72.png',         size: 72,  isMaskable: false },
  { filename: 'favicon-32.png',       size: 32,  isMaskable: false },
];

/**
 * Generate the SVG for an icon at the given size.
 *
 * Design:
 *   · Solid teal (#0d9488) rounded-rect background
 *   · White "T" lettermark centred (platform initial in subscript)
 *   · Thin accent-coloured bottom strip (identifies app at a glance)
 *   · Maskable variant adds safe-zone padding (10% on each side)
 */
function buildSVG(size, label, accent, isMaskable) {
  const pad = isMaskable ? Math.round(size * 0.10) : 0;
  const innerSize = size - pad * 2;
  const radius = Math.round(innerSize * 0.20);
  const stripH = Math.max(4, Math.round(innerSize * 0.08));

  // Letter "T" — sized relative to inner canvas
  const fontSize = Math.round(innerSize * 0.42);
  const letterY = Math.round(innerSize * 0.58);
  const cx = Math.round(innerSize / 2);

  // Small app-identifier label (bottom-right of the "T")
  const subFontSize = Math.max(8, Math.round(fontSize * 0.28));
  const subX = cx + Math.round(fontSize * 0.26);
  const subY = letterY;

  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}">
  <g transform="translate(${pad},${pad})">
    <!-- Background -->
    <rect width="${innerSize}" height="${innerSize}" rx="${radius}" ry="${radius}" fill="#0d9488"/>
    <!-- Accent bottom strip -->
    <rect y="${innerSize - stripH}" width="${innerSize}" height="${stripH}"
          rx="${Math.round(radius / 3)}" ry="${Math.round(radius / 3)}" fill="${accent}"/>
    <!-- "T" lettermark -->
    <text x="${cx}" y="${letterY}"
          font-family="system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif"
          font-size="${fontSize}" font-weight="800" fill="white"
          text-anchor="middle" dominant-baseline="auto">T</text>
    <!-- App identifier subscript -->
    <text x="${subX}" y="${subY}"
          font-family="system-ui,-apple-system,sans-serif"
          font-size="${subFontSize}" font-weight="700" fill="white" opacity="0.75"
          text-anchor="start" dominant-baseline="auto">${label}</text>
  </g>
</svg>`;
}

async function generate() {
  let total = 0;

  for (const app of APPS) {
    const outDir = resolve(ROOT, app.outDir);
    await mkdir(outDir, { recursive: true });

    for (const { filename, size, isMaskable } of SIZES) {
      const svg = buildSVG(size, app.label, app.accent, isMaskable);
      const outPath = resolve(outDir, filename);

      await sharp(Buffer.from(svg))
        .resize(size, size)
        .png({ compressionLevel: 9, adaptiveFiltering: true })
        .toFile(outPath);

      console.log(`  ✓  ${app.name}/${app.outDir.split('/').pop()}/${filename}  (${size}×${size})`);
      total++;
    }
  }

  console.log(`\nDone — ${total} icons generated.`);
}

generate().catch((err) => {
  console.error('Icon generation failed:', err);
  process.exit(1);
});
