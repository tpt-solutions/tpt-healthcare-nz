import { test, expect } from '@playwright/test';

test.describe('Waiting room / check-in', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email address').fill('patient@example.nz');
    await page.getByLabel('Password').fill('anything');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL('**/dashboard');
    await page.locator('nav').getByRole('link', { name: 'Queue / Check-in' }).click();
    await page.waitForURL('**/waiting');
  });

  test('renders the check-in heading and NHI input', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Check in' })).toBeVisible();
    await expect(page.locator('input[type="text"]').first()).toBeVisible();
    await expect(page.getByRole('button', { name: /Check in/ })).toBeVisible();
  });

  test('shows NHI privacy note', async ({ page }) => {
    await expect(page.getByText('Your NHI is used only to find your appointment')).toBeVisible();
  });

  test('check-in button is enabled for input', async ({ page }) => {
    // The check-in button should be enabled (not disabled) when the page loads
    await expect(page.getByRole('button', { name: /Check in/ })).toBeEnabled();
  });

  test('check-in with NHI makes an API call', async ({ page }) => {
    let requestReceived = false;
    await page.route('**/api/v1/queue/today/check-in', async (route) => {
      requestReceived = true;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          entryId: 'entry-1',
          position: 3,
          estimatedWaitMinutes: 15,
        }),
      });
    });
    await page.route('**/api/v1/queue/today/entries/entry-1/stream', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: '',
      });
    });

    await page.locator('input[type="text"]').first().fill('ZZZ0032');
    await page.getByRole('button', { name: /Check in/ }).click();

    await expect(page.getByText(/#\d+/)).toBeVisible();
    await expect(page.getByText(/min wait/)).toBeVisible();
    expect(requestReceived).toBe(true);
  });
});
