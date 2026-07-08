import { test, expect } from '../../fixtures/admin-auth';

test.describe('Reports page', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Reports' }).click();
    await page.waitForURL('**/reports');
  });

  test('renders the reports heading and capitation submission', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Reports' })).toBeVisible();
    await expect(page.getByText('Upcoming Capitation Submission')).toBeVisible();
    await expect(page.getByText('Q3 2026')).toBeVisible();
  });

  test('shows capitation submission stats', async ({ page }) => {
    await expect(page.getByText('4,823').first()).toBeVisible();
    await expect(page.getByText('4,710').first()).toBeVisible();
    await expect(page.getByText('Export Submission File')).toBeVisible();
  });

  test('shows age band breakdown', async ({ page }) => {
    await expect(page.getByText('Enrolled Population — Age Bands')).toBeVisible();
    await expect(page.getByText('0–4')).toBeVisible();
    await expect(page.getByText('75+')).toBeVisible();
  });

  test('shows ethnicity breakdown', async ({ page }) => {
    await expect(page.getByText('Enrolled Population — Ethnicity')).toBeVisible();
    await expect(page.getByText('Māori')).toBeVisible();
    await expect(page.getByText('Pacific')).toBeVisible();
  });

  test('shows condition prevalence', async ({ page }) => {
    await expect(page.getByText('Long-Term Condition Prevalence')).toBeVisible();
    await expect(page.getByText('Type 2 Diabetes')).toBeVisible();
    await expect(page.getByText('Hypertension')).toBeVisible();
    await expect(page.getByText('Asthma')).toBeVisible();
  });
});
