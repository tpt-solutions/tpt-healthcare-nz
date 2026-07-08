import { test, expect } from '@playwright/test';

test.describe('Patient appointments', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email address').fill('patient@example.nz');
    await page.getByLabel('Password').fill('anything');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL('**/dashboard');
    await page.locator('nav').getByRole('link', { name: 'Appointments' }).click();
    await page.waitForURL('**/appointments');
  });

  test('renders the appointments heading and request button', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'My Appointments' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Request Appointment' })).toBeVisible();
  });

  test('shows upcoming appointments by default', async ({ page }) => {
    await expect(page.getByText('Dr. Hemi Walker').first()).toBeVisible();
    await expect(page.getByText('General Practice').first()).toBeVisible();
  });

  test('switching to past tab shows past appointments', async ({ page }) => {
    await page.getByRole('button', { name: 'past' }).click();
    await expect(page.getByText('Follow-up')).toBeVisible();
    await expect(page.getByText('HbA1c review')).toBeVisible();
  });

  test('request appointment modal opens and closes', async ({ page }) => {
    await page.getByRole('button', { name: 'Request Appointment' }).click();
    await expect(page.getByText('Request New Appointment')).toBeVisible();
    await expect(page.getByText('Reason for visit')).toBeVisible();

    // Close modal
    await page.getByRole('button', { name: 'Cancel' }).click();
    await expect(page.getByText('Request New Appointment')).not.toBeVisible();
  });
});
