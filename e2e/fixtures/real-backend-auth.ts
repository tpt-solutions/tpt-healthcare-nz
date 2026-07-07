import { test as base, type Page } from '@playwright/test';
import { TEST_PRACTITIONER } from './test-data';

interface RealBackendAuthFixtures {
  /** Logs in against the real tpt-doctor backend (no request mocking) and
   * leaves the page on /dashboard. Requires the dev-auth-enabled stack from
   * deploy/docker-compose.dev.yml to be running (see CONTRIBUTING.md). */
  loginAsPractitioner: (page: Page) => Promise<void>;
}

// Unlike fixtures/auth.ts, this file does NOT stub any network requests —
// specs importing `test` from here hit the real backend over the network.
export const test = base.extend<RealBackendAuthFixtures>({
  loginAsPractitioner: async ({}, use) => {
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
