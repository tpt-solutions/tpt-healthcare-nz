import { test, expect } from '../../fixtures/admin-auth';
import { AdminBillingPage } from '../../pages/admin/AdminBillingPage';

test.describe('Billing page', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Billing' }).click();
    await page.waitForURL('**/billing');
  });

  test('renders the billing dashboard heading', async ({ page }) => {
    const billing = new AdminBillingPage(page);

    await expect(billing.heading()).toBeVisible();
    await expect(billing.claimStatusSummary()).toBeVisible();
  });

  test('shows ACC claims with status badges', async ({ page }) => {
    const billing = new AdminBillingPage(page);

    await expect(billing.claimRow('ACC45-2026-00891')).toBeVisible();
    await expect(page.getByText('pending').first()).toBeVisible();
    await expect(page.getByText('approved').first()).toBeVisible();
    await expect(page.getByText('declined').first()).toBeVisible();
  });

  test('PHO capitation section shows enrolled patients and rate', async ({ page }) => {
    // "PHO Capitation" appears as a section heading
    await expect(page.getByRole('heading', { name: 'PHO Capitation' })).toBeVisible();
    await expect(page.getByText('4,823').first()).toBeVisible();
    await expect(page.getByText('$60.37 / patient')).toBeVisible();
  });

  test('revenue chart section is visible', async ({ page }) => {
    await expect(page.getByText('Monthly Revenue by Funding Type')).toBeVisible();
    await expect(page.getByText('June 2026 Total')).toBeVisible();
  });

  test('clicking a claim filter shows filtered results', async ({ page }) => {
    const billing = new AdminBillingPage(page);

    await billing.claimFilterButton('pending').click();
    await expect(billing.claimRow('ACC45-2026-00891')).toBeVisible();
    // Paid claims should be hidden when filtering to pending
    await expect(billing.claimRow('ACC6-2026-00887')).not.toBeVisible();
  });
});
