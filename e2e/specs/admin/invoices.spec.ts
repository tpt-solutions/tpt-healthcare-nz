import { test, expect } from '../../fixtures/admin-auth';
import { AdminInvoicesPage } from '../../pages/admin/AdminOperationsPage';

test.describe('Patient invoices', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Invoices' }).click();
    await page.waitForURL('**/invoices');
  });

  test('renders the invoices heading and filter buttons', async ({ page }) => {
    const invoices = new AdminInvoicesPage(page);

    await expect(invoices.heading()).toBeVisible();
    await expect(invoices.newInvoiceButton()).toBeVisible();
    await expect(invoices.filterButton('all')).toBeVisible();
    await expect(invoices.filterButton('issued')).toBeVisible();
    await expect(invoices.filterButton('paid')).toBeVisible();
    await expect(invoices.filterButton('overdue')).toBeVisible();
  });

  test('shows invoice data with status badges', async ({ page }) => {
    await expect(page.getByText('ZZZ0001')).toBeVisible();
    await expect(page.getByText('issued').first()).toBeVisible();
    await expect(page.getByText('paid').first()).toBeVisible();
    await expect(page.getByText('overdue').first()).toBeVisible();
  });

  test('filtering by status narrows results', async ({ page }) => {
    const invoices = new AdminInvoicesPage(page);

    await invoices.filterButton('overdue').click();
    await expect(page.getByText('ZZZ0003')).toBeVisible();
    await expect(page.getByText('ZZZ0001')).not.toBeVisible();
  });
});
