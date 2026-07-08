import { test as base, type Page } from '@playwright/test';

export const TEST_ADMIN = {
  email: 'admin@aucklandcitymedical.nz',
  password: 'Test1234!',
} as const;

interface AdminAuthFixtures {
  /** Intercepts common admin API endpoints so pages render without a backend. */
  mockAdminApi: void;
  /** Logs in as the admin user and lands on /dashboard. */
  loginAsAdmin: (page: Page) => Promise<void>;
}

export const test = base.extend<AdminAuthFixtures>({
  mockAdminApi: [
    async ({ page }, use) => {
      // The admin app's Vite dev server proxies /api to localhost:8080,
      // which is not running during e2e. Intercept at the Playwright level
      // so pages render their stub data instead of hanging on fetch errors.
      await page.route('**/api/v1/practice/system/backup', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });
      await page.route('**/api/v1/audit-events**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ events: [] }),
        });
      });
      await page.route('**/api/v1/practice/settings', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ok: true }),
        });
      });
      await page.route('**/api/v1/admin/applications**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ applications: [] }),
        });
      });
      await page.route('**/api/v1/admin/tenants', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ tenants: [] }),
        });
      });
      await page.route('**/api/v1/practice/roster', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });
      await page.route('**/api/v1/practice/rooms/bookings', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });
      await page.route('**/api/v1/practice/leave', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });
      await page.route('**/api/v1/practice/inventory', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });
      await page.route('**/api/v1/practice/departments', async (route) => {
        if (route.request().method() !== 'GET') return route.fallback();
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });
      await page.route('**/api/v1/practice/roles', async (route) => {
        if (route.request().method() !== 'GET') return route.fallback();
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });
      await page.route('**/api/v1/health/providers', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ providers: [] }),
        });
      });
      await page.route('**/api/v1/practice/onboarding/step/**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ok: true }),
        });
      });
      await use();
    },
    { auto: true },
  ],

  loginAsAdmin: async ({ page }, use) => {
    await use(async (targetPage: Page) => {
      await targetPage.goto('/login');
      await targetPage.getByLabel('Work email').fill(TEST_ADMIN.email);
      await targetPage.getByLabel('Password').fill(TEST_ADMIN.password);
      await targetPage.getByRole('button', { name: 'Sign in' }).click();
      await targetPage.waitForURL('**/dashboard');
    });
  },
});

export { expect } from '@playwright/test';
