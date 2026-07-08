import { test, expect } from '../../fixtures/admin-auth';
import { AdminInventoryPage } from '../../pages/admin/AdminOperationsPage';

test.describe('Inventory', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await page.route('**/api/v1/practice/inventory', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Inventory' }).click();
    await page.waitForURL('**/inventory');
  });

  test('renders the inventory heading and empty state', async ({ page }) => {
    const inventory = new AdminInventoryPage(page);

    await expect(inventory.heading()).toBeVisible();
    await expect(inventory.emptyState()).toBeVisible();
  });

  test('shows the add stock item button', async ({ page }) => {
    const inventory = new AdminInventoryPage(page);

    await expect(inventory.addStockItemButton()).toBeVisible();
  });
});
