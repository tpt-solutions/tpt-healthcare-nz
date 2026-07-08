import { defineConfig, devices } from '@playwright/test';

const CLINIC_PORT = 4173;
const PORTAL_PORT = 4174;
const ADMIN_PORT = 4175;

// Device profiles used across the matrix.
const desktopChrome = devices['Desktop Chrome'];
const desktopFirefox = devices['Desktop Firefox'];
const desktopWebKit = devices['Desktop Safari'];
const mobileChrome = devices['Pixel 5'];
const mobileSafari = devices['iPhone 13'];

// Shared screenshot comparison thresholds.
const snapshotConfig = {
  maxDiffPixelRatio: 0.01,
  animations: 'disabled' as const,
};

export default defineConfig({
  testDir: './specs',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['html', { open: 'never' }], ['list']] : 'list',
  snapshotPathTemplate:
    '{testDir}/__screenshots__/{testFilePath}/{arg}{ext}',
  use: {
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    // Block PWA service workers so page.route() mocks aren't shadowed.
    serviceWorkers: 'block',
  },

  projects: [
    // ── Clinic ──────────────────────────────────────────────
    {
      name: 'clinic',
      testDir: './specs/clinic',
      use: { ...desktopChrome, baseURL: `http://localhost:${CLINIC_PORT}` },
    },
    {
      name: 'clinic:firefox',
      testDir: './specs/clinic',
      use: { ...desktopFirefox, baseURL: `http://localhost:${CLINIC_PORT}` },
    },
    {
      name: 'clinic:webkit',
      testDir: './specs/clinic',
      use: { ...desktopWebKit, baseURL: `http://localhost:${CLINIC_PORT}` },
    },
    {
      name: 'clinic:mobile-chrome',
      testDir: './specs/clinic',
      use: { ...mobileChrome, baseURL: `http://localhost:${CLINIC_PORT}` },
    },
    {
      name: 'clinic:mobile-safari',
      testDir: './specs/clinic',
      use: { ...mobileSafari, baseURL: `http://localhost:${CLINIC_PORT}` },
    },

    // ── Portal ──────────────────────────────────────────────
    {
      name: 'portal',
      testDir: './specs/portal',
      use: { ...desktopChrome, baseURL: `http://localhost:${PORTAL_PORT}` },
    },
    {
      name: 'portal:firefox',
      testDir: './specs/portal',
      use: { ...desktopFirefox, baseURL: `http://localhost:${PORTAL_PORT}` },
    },
    {
      name: 'portal:webkit',
      testDir: './specs/portal',
      use: { ...desktopWebKit, baseURL: `http://localhost:${PORTAL_PORT}` },
    },
    {
      name: 'portal:mobile-chrome',
      testDir: './specs/portal',
      use: { ...mobileChrome, baseURL: `http://localhost:${PORTAL_PORT}` },
    },
    {
      name: 'portal:mobile-safari',
      testDir: './specs/portal',
      use: { ...mobileSafari, baseURL: `http://localhost:${PORTAL_PORT}` },
    },

    // ── Admin ───────────────────────────────────────────────
    {
      name: 'admin',
      testDir: './specs/admin',
      use: { ...desktopChrome, baseURL: `http://localhost:${ADMIN_PORT}` },
    },
    {
      name: 'admin:firefox',
      testDir: './specs/admin',
      use: { ...desktopFirefox, baseURL: `http://localhost:${ADMIN_PORT}` },
    },
    {
      name: 'admin:webkit',
      testDir: './specs/admin',
      use: { ...desktopWebKit, baseURL: `http://localhost:${ADMIN_PORT}` },
    },
    {
      name: 'admin:mobile-chrome',
      testDir: './specs/admin',
      use: { ...mobileChrome, baseURL: `http://localhost:${ADMIN_PORT}` },
    },
    {
      name: 'admin:mobile-safari',
      testDir: './specs/admin',
      use: { ...mobileSafari, baseURL: `http://localhost:${ADMIN_PORT}` },
    },

    // ── Visual Regression (Desktop Chrome only) ─────────────
    {
      name: 'visual-clinic',
      testDir: './specs/visual',
      use: {
        ...desktopChrome,
        baseURL: `http://localhost:${CLINIC_PORT}`,
        ...snapshotConfig,
      },
    },
    {
      name: 'visual-portal',
      testDir: './specs/visual',
      use: {
        ...desktopChrome,
        baseURL: `http://localhost:${PORTAL_PORT}`,
        ...snapshotConfig,
      },
    },
    {
      name: 'visual-admin',
      testDir: './specs/visual',
      use: {
        ...desktopChrome,
        baseURL: `http://localhost:${ADMIN_PORT}`,
        ...snapshotConfig,
      },
    },

    // ── Visual Regression: Mobile (Pixel 5) ─────────────────
    {
      name: 'visual-clinic:mobile',
      testDir: './specs/visual',
      use: {
        ...mobileChrome,
        baseURL: `http://localhost:${CLINIC_PORT}`,
        ...snapshotConfig,
      },
    },
    {
      name: 'visual-portal:mobile',
      testDir: './specs/visual',
      use: {
        ...mobileChrome,
        baseURL: `http://localhost:${PORTAL_PORT}`,
        ...snapshotConfig,
      },
    },
    {
      name: 'visual-admin:mobile',
      testDir: './specs/visual',
      use: {
        ...mobileChrome,
        baseURL: `http://localhost:${ADMIN_PORT}`,
        ...snapshotConfig,
      },
    },
  ],

  webServer: [
    {
      command: `pnpm --filter tpt-clinic exec vite --port ${CLINIC_PORT} --strictPort`,
      url: `http://localhost:${CLINIC_PORT}`,
      cwd: '..',
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
    },
    {
      command: `pnpm --filter tpt-portal exec vite --port ${PORTAL_PORT} --strictPort`,
      url: `http://localhost:${PORTAL_PORT}`,
      cwd: '..',
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
    },
    {
      command: `pnpm --filter tpt-admin exec vite --port ${ADMIN_PORT} --strictPort`,
      url: `http://localhost:${ADMIN_PORT}`,
      cwd: '..',
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
    },
  ],
});
