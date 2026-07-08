import { test, expect } from '@playwright/test';

test.describe('Portal dashboard', () => {
  test.beforeEach(async ({ page }) => {
    // Portal uses stub auth — any non-empty email succeeds
    await page.goto('/login');
    await page.getByLabel('Email address').fill('patient@example.nz');
    await page.getByLabel('Password').fill('anything');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL('**/dashboard');
  });

  test('shows the patient greeting', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /Kia ora/ })).toBeVisible();
  });

  test('shows quick stats cards', async ({ page }) => {
    await expect(page.getByText('Next Appointment')).toBeVisible();
    await expect(page.getByText('Active Medications').first()).toBeVisible();
    await expect(page.getByText('Results to Review')).toBeVisible();
  });

  test('shows upcoming appointments section', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Upcoming Appointments' })).toBeVisible();
    await expect(page.getByText('Dr. Hemi Walker').first()).toBeVisible();
  });

  test('shows recent test results section', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Recent Test Results' })).toBeVisible();
    await expect(page.getByText('Full Blood Count')).toBeVisible();
    await expect(page.getByText('HbA1c')).toBeVisible();
  });

  test('shows active medications section', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Active Medications' })).toBeVisible();
    await expect(page.getByText('Metformin 500 mg')).toBeVisible();
    await expect(page.getByText('Lisinopril 10 mg')).toBeVisible();
  });
});
