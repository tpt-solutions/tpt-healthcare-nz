import { test, expect } from '@playwright/test';

test.describe('Patient secure messages', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email address').fill('patient@example.nz');
    await page.getByLabel('Password').fill('anything');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL('**/dashboard');
    await page.getByRole('link', { name: 'Messages' }).click();
    await page.waitForURL('**/messages');
  });

  test('renders the messages heading with unread badge', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Secure Messages' })).toBeVisible();
    await expect(page.getByText('1 new')).toBeVisible();
  });

  test('shows encryption notice', async ({ page }) => {
    await expect(page.getByText('All messages are end-to-end encrypted')).toBeVisible();
  });

  test('shows message list with subjects', async ({ page }) => {
    await expect(page.getByText('Your HbA1c results')).toBeVisible();
    await expect(page.getByText('Appointment reminder')).toBeVisible();
    await expect(page.getByText('Question about Lisinopril')).toBeVisible();
  });

  test('clicking a message shows its body', async ({ page }) => {
    await page.getByText('Your HbA1c results').click();
    await expect(page.getByText('From: Dr. Hemi Walker')).toBeVisible();
    await expect(page.getByText('52 mmol/mol')).toBeVisible();
  });

  test('new message modal opens and closes', async ({ page }) => {
    await page.getByRole('button', { name: 'New Message' }).click();
    await expect(page.getByText('New Message').last()).toBeVisible();
    await expect(page.getByPlaceholder('Message subject...')).toBeVisible();
    await expect(page.getByPlaceholder('Write your message here...')).toBeVisible();

    await page.getByRole('button', { name: 'Cancel' }).click();
  });
});
