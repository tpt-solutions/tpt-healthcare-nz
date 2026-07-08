import { test, expect } from '../../fixtures/admin-auth';

test.describe('Budget variance', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Budget' }).click();
    await page.waitForURL('**/budget');
  });

  test('renders the budget heading and summary cards', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Budget Variance' })).toBeVisible();
    await expect(page.getByText('Total planned')).toBeVisible();
    await expect(page.getByText('Total actual')).toBeVisible();
    // Variance appears in heading, card, and table — just check one instance
    await expect(page.getByText('Variance').first()).toBeVisible();
  });

  test('shows the variance table with budget lines', async ({ page }) => {
    await expect(page.getByText('Staff').first()).toBeVisible();
    await expect(page.getByText('Supplies').first()).toBeVisible();
  });
});
