import { test, expect } from '../../fixtures/admin-auth';
import { AdminRosterPage } from '../../pages/admin/AdminOperationsPage';

test.describe('Staff roster', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await page.route('**/api/v1/practice/roster', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Roster' }).click();
    await page.waitForURL('**/roster');
  });

  test('renders the roster heading and empty state', async ({ page }) => {
    const roster = new AdminRosterPage(page);

    await expect(roster.heading()).toBeVisible();
    await expect(roster.emptyState()).toBeVisible();
  });

  test('shows the add shift button', async ({ page }) => {
    const roster = new AdminRosterPage(page);

    await expect(roster.addShiftButton()).toBeVisible();
  });
});
