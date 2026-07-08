import { test, expect } from '@playwright/test';

test.describe('Book appointment flow', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email address').fill('patient@example.nz');
    await page.getByLabel('Password').fill('anything');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL('**/dashboard');
    await page.getByRole('link', { name: 'Book Appointment' }).click();
    await page.waitForURL('**/appointments/book');
  });

  test('renders the booking heading and date picker', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Book appointment' })).toBeVisible();
    await expect(page.locator('input[type="date"]')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Next — choose time' })).toBeVisible();
  });

  test('next button is disabled without a date', async ({ page }) => {
    await expect(page.getByRole('button', { name: 'Next — choose time' })).toBeDisabled();
  });

  test('selecting a date enables next and advances to time selection', async ({ page }) => {
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    const dateStr = tomorrow.toISOString().split('T')[0];

    await page.locator('input[type="date"]').fill(dateStr);
    await page.getByRole('button', { name: 'Next — choose time' }).click();

    // Should now show time slots
    await expect(page.getByRole('button', { name: '08:00' })).toBeVisible();
    await expect(page.getByRole('button', { name: '09:00' })).toBeVisible();
  });

  test('unavailable time slots are disabled', async ({ page }) => {
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    const dateStr = tomorrow.toISOString().split('T')[0];

    await page.locator('input[type="date"]').fill(dateStr);
    await page.getByRole('button', { name: 'Next — choose time' }).click();

    // 08:30 is marked as unavailable in the stub data
    await expect(page.getByRole('button', { name: '08:30' })).toBeDisabled();
  });

  test('full booking flow through to confirmation', async ({ page }) => {
    // Mock the booking API
    await page.route('**/api/v1/appointments', async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'appt-new',
          practitionerName: 'Dr. Hemi Walker',
          startTime: `${new Date(Date.now() + 86400000).toISOString().split('T')[0]}T09:00:00`,
          endTime: `${new Date(Date.now() + 86400000).toISOString().split('T')[0]}T09:30:00`,
          reason: 'Follow-up',
        }),
      });
    });

    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    const dateStr = tomorrow.toISOString().split('T')[0];

    // Step 1: Date
    await page.locator('input[type="date"]').fill(dateStr);
    await page.getByRole('button', { name: 'Next — choose time' }).click();

    // Step 2: Time
    await page.getByRole('button', { name: '09:00' }).click();
    await page.getByRole('button', { name: 'Next — reason for visit' }).click();

    // Step 3: Reason
    await page.locator('textarea').fill('Follow-up');
    await page.getByRole('button', { name: 'Review booking' }).click();

    // Step 4: Confirm
    await expect(page.getByText('Confirm your appointment')).toBeVisible();
    await expect(page.getByText('09:00')).toBeVisible();
    await page.getByRole('button', { name: 'Confirm booking' }).click();

    // Success
    await expect(page.getByRole('heading', { name: 'Appointment booked' })).toBeVisible();
  });
});
