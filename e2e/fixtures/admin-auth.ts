import { test as base, type Page } from '@playwright/test';

export const TEST_ADMIN = {
  email: 'admin@aucklandcitymedical.nz',
  password: 'Test1234!',
} as const;

interface AdminAuthFixtures {
  /** Logs in as the admin user and lands on /dashboard. */
  loginAsAdmin: (page: Page) => Promise<void>;
}

export const test = base.extend<AdminAuthFixtures>({
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
