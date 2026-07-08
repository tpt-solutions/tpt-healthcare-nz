import { test, expect } from '../../fixtures/admin-auth';
import { AdminBudgetPage } from '../../pages/admin/AdminOperationsPage';

test.describe('Budget variance', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Budget' }).click();
    await page.waitForURL('**/budget');
  });

  test('renders the budget heading and summary cards', async ({ page }) => {
    const budget = new AdminBudgetPage(page);

    await expect(budget.heading()).toBeVisible();
    await expect(budget.totalPlannedCard()).toBeVisible();
    await expect(budget.varianceCard()).toBeVisible();
  });

  test('shows the variance table with budget lines', async ({ page }) => {
    await expect(page.getByText('Staff').first()).toBeVisible();
    await expect(page.getByText('Supplies').first()).toBeVisible();
    await expect(page.getByText('Total planned')).toBeVisible();
    await expect(page.getByText('Total actual')).toBeVisible();
  });
});
