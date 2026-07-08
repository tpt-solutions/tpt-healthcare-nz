import { test, expect } from '@playwright/test';

test.describe('Patient health records', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email address').fill('patient@example.nz');
    await page.getByLabel('Password').fill('anything');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL('**/dashboard');
    await page.getByRole('link', { name: 'Health Records' }).click();
    await page.waitForURL('**/records');
  });

  test('renders the records heading and NHI banner', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'My Health Records' })).toBeVisible();
    await expect(page.getByText('National Health Index (NHI)')).toBeVisible();
    await expect(page.getByText('ZZZ0032')).toBeVisible();
  });

  test('shows HIPC data retention note', async ({ page }) => {
    await expect(page.getByText('HIPC Rule 6')).toBeVisible();
    await expect(page.getByText('Data Retention')).toBeVisible();
  });

  test('shows encounters tab by default with encounter data', async ({ page }) => {
    await expect(page.getByText('Dr. Hemi Walker').first()).toBeVisible();
    await expect(page.getByText('HbA1c follow-up')).toBeVisible();
  });

  test('switching to diagnoses tab shows conditions', async ({ page }) => {
    await page.getByRole('button', { name: 'Diagnoses' }).click();
    await expect(page.getByText('Type 2 diabetes mellitus')).toBeVisible();
    await expect(page.getByText('Essential (primary) hypertension')).toBeVisible();
    await expect(page.getByText('E11')).toBeVisible();
  });

  test('switching to immunisations tab shows vaccine records', async ({ page }) => {
    await page.getByRole('button', { name: 'Immunisations' }).click();
    await expect(page.getByText('Influenza vaccine')).toBeVisible();
    await expect(page.getByText('COVID-19 XBB.1.5 booster')).toBeVisible();
    await expect(page.getByText('Tdap (Boostrix)')).toBeVisible();
  });
});
