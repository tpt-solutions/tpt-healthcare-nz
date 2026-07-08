import { test, expect } from '@playwright/test';

test.describe('Patient prescriptions', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email address').fill('patient@example.nz');
    await page.getByLabel('Password').fill('anything');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL('**/dashboard');
    // Scope to sidebar nav to avoid matching "View all prescriptions" link on dashboard
    await page.locator('nav').getByRole('link', { name: 'Prescriptions' }).click();
    await page.waitForURL('**/prescriptions');
  });

  test('renders the prescriptions heading', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'My Prescriptions' })).toBeVisible();
  });

  test('shows PHARMAC subsidy note', async ({ page }) => {
    await expect(page.getByText('PHARMAC-subsidised medicines')).toBeVisible();
  });

  test('shows active prescriptions with details', async ({ page }) => {
    await expect(page.getByText(/Active Prescriptions/)).toBeVisible();
    await expect(page.getByText('Metformin 500 mg tablets')).toBeVisible();
    await expect(page.getByText('Lisinopril 10 mg tablets')).toBeVisible();
  });

  test('shows PHARMAC Subsidised badge', async ({ page }) => {
    await expect(page.getByText('PHARMAC Subsidised').first()).toBeVisible();
  });

  test('shows past prescriptions', async ({ page }) => {
    await expect(page.getByText('Past Prescriptions').first()).toBeVisible();
    await expect(page.getByText('Amoxicillin 500 mg capsules')).toBeVisible();
  });

  test('shows prescription instructions', async ({ page }) => {
    await expect(page.getByText('Take one tablet twice daily with food')).toBeVisible();
  });

  test('shows NZMT codes for prescriptions', async ({ page }) => {
    await expect(page.getByText('10037281000116105')).toBeVisible();
  });
});
