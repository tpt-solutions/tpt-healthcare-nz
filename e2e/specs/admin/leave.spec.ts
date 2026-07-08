import { test, expect } from '../../fixtures/admin-auth';
import { AdminLeavePage } from '../../pages/admin/AdminOperationsPage';

test.describe('Leave requests', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await page.route('**/api/v1/practice/leave', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Leave' }).click();
    await page.waitForURL('**/leave');
  });

  test('renders the leave heading and empty state', async ({ page }) => {
    const leave = new AdminLeavePage(page);

    await expect(leave.heading()).toBeVisible();
    await expect(leave.emptyState()).toBeVisible();
  });

  test('shows the request leave button', async ({ page }) => {
    const leave = new AdminLeavePage(page);

    await expect(leave.requestLeaveButton()).toBeVisible();
  });
});
