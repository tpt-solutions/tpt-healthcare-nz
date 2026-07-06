import { defineConfig, devices } from '@playwright/test';

const CLINIC_PORT = 4173;
const PORTAL_PORT = 4174;

export default defineConfig({
  testDir: './specs',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['html', { open: 'never' }], ['list']] : 'list',
  use: {
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    // Both apps register a PWA service worker on preview builds. Block it so
    // page.route() network mocks aren't shadowed by workbox's own fetch handling.
    serviceWorkers: 'block',
  },
  projects: [
    {
      name: 'clinic',
      testDir: './specs/clinic',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: `http://localhost:${CLINIC_PORT}`,
      },
    },
    {
      name: 'portal',
      testDir: './specs/portal',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: `http://localhost:${PORTAL_PORT}`,
      },
    },
  ],
  webServer: [
    {
      // Uses the Vite dev server rather than `vite build && vite preview`:
      // tpt-clinic's production build currently hangs during bundling for
      // reasons unrelated to e2e (pre-existing issue, out of scope for this
      // suite — see todo.md). The dev server compiles on demand and avoids it.
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
  ],
});
