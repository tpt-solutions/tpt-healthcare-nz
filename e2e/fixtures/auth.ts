import { test as base, type Page } from '@playwright/test';
import { TEST_PRACTITIONER } from './test-data';

interface AuthFixtures {
  /** Stubs POST /api/v1/auth/token so login succeeds only for TEST_PRACTITIONER's credentials. */
  mockAuth: void;
  /** Logs in as the mocked test practitioner and leaves the page on /dashboard. */
  loginAsPractitioner: (page: Page) => Promise<void>;
}

export const test = base.extend<AuthFixtures>({
  mockAuth: [
    async ({ page }, use) => {
      await page.route('**/api/v1/auth/token', async (route) => {
        const body = route.request().postDataJSON() as { email: string; password: string };

        if (body.email === TEST_PRACTITIONER.email && body.password === TEST_PRACTITIONER.password) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              access_token: TEST_PRACTITIONER.accessToken,
              user: TEST_PRACTITIONER.user,
            }),
          });
        } else {
          await route.fulfill({
            status: 401,
            contentType: 'application/json',
            body: JSON.stringify({ message: 'Invalid credentials' }),
          });
        }
      });

      await use();
    },
    { auto: true },
  ],

  loginAsPractitioner: async ({ page }, use) => {
    await use(async (targetPage: Page) => {
      await targetPage.goto('/login');
      await targetPage.getByLabel('Email address').fill(TEST_PRACTITIONER.email);
      await targetPage.getByLabel('Password').fill(TEST_PRACTITIONER.password);
      await targetPage.getByRole('button', { name: 'Sign in' }).click();
      await targetPage.waitForURL('**/dashboard');
    });
  },
});

export { expect } from '@playwright/test';
