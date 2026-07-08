import { test, expect } from '@playwright/test';

test.describe('Patient consent management', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email address').fill('patient@example.nz');
    await page.getByLabel('Password').fill('anything');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL('**/dashboard');
    await page.getByRole('link', { name: 'My Consent' }).click();
    await page.waitForURL('**/consent');
  });

  test('renders the consent heading and HIPC explainer', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Manage My Consent' })).toBeVisible();
    await expect(page.getByText('Your rights under the Health Information Privacy Code')).toBeVisible();
  });

  test('shows granted and revoked consent counts', async ({ page }) => {
    await expect(page.getByText('Consents Granted')).toBeVisible();
    await expect(page.getByText('Consents Revoked')).toBeVisible();
  });

  test('shows consent items with HIPC rule badges', async ({ page }) => {
    await expect(page.getByText('Share records with my GP')).toBeVisible();
    await expect(page.getByText('HIPC Rule 11').first()).toBeVisible();
    await expect(page.getByText('HIPC Rule 10').first()).toBeVisible();
  });

  test('clicking Details expands the consent detail', async ({ page }) => {
    await page.getByText('Share records with my GP').locator('..').getByRole('button', { name: 'Details' }).click();
    await expect(page.getByText('Under HIPC Rule 11')).toBeVisible();
  });

  test('revoking a consent opens a confirmation modal', async ({ page }) => {
    await page.getByText('Share records with my GP').locator('..').getByRole('button', { name: 'Revoke' }).click();
    await expect(page.getByRole('heading', { name: 'Revoke consent?' })).toBeVisible();
    await expect(page.getByText('Revoking this consent takes effect immediately')).toBeVisible();
  });

  test('granting a revoked consent opens a confirmation modal', async ({ page }) => {
    await page.getByText('Share medication history').locator('..').getByRole('button', { name: 'Grant' }).click();
    await expect(page.getByRole('heading', { name: 'Grant consent?' })).toBeVisible();
  });
});
