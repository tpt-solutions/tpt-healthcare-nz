import { test as base, type Page } from '@playwright/test';
import { TEST_ADMIN } from './admin-auth';

interface AdminVisualAuthFixtures {
  /** Mock admin APIs for visual regression tests. */
  mockAdminVisualApi: void;
  /** Navigate to a logged-in admin page. */
  loginAdmin: (page: Page) => Promise<void>;
}

export const test = base.extend<AdminVisualAuthFixtures>({
  mockAdminVisualApi: [
    async ({ page }, use) => {
      const routes: Array<{ url: string; body: unknown }> = [
        { url: '**/api/v1/practice/system/backup', body: [] },
        { url: '**/api/v1/audit-events**', body: { events: [] } },
        { url: '**/api/v1/practice/settings', body: { ok: true } },
        { url: '**/api/v1/admin/applications**', body: { applications: [] } },
        { url: '**/api/v1/admin/tenants', body: { tenants: [] } },
        { url: '**/api/v1/practice/roster', body: [] },
        { url: '**/api/v1/practice/rooms/bookings', body: [] },
        { url: '**/api/v1/practice/leave', body: [] },
        { url: '**/api/v1/practice/inventory', body: [] },
        { url: '**/api/v1/practice/roles', body: [] },
        { url: '**/api/v1/health/providers', body: { providers: [] } },
        { url: '**/api/v1/practice/onboarding/step/**', body: { ok: true } },
      ];
      for (const { url, body } of routes) {
        await page.route(url, async (route) => {
          if (route.request().method() !== 'GET') return route.fallback();
          await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
        });
      }
      await use();
    },
    { auto: true },
  ],

  loginAdmin: async ({ page }, use) => {
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
