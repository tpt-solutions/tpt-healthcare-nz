import { test, expect } from '@playwright/test';

// tpt-portal's AuthContext currently stubs login client-side (any non-empty
// email succeeds, no network call) — see apps/tpt-portal/src/contexts/AuthContext.tsx.
// This spec exists to prove the shared e2e harness generalizes across apps;
// it will start exercising a real POST /api/v1/auth/token once portal auth
// is wired up for real (tracked in the deferred/follow-up list).
test.describe('Portal login', () => {
  test('signing in redirects to the dashboard', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email address').fill('patient@example.nz');
    await page.getByLabel('Password').fill('anything');
    await page.getByRole('button', { name: 'Sign in' }).click();

    await page.waitForURL('**/dashboard');
    await expect(page).toHaveURL(/\/dashboard$/);
  });
});
