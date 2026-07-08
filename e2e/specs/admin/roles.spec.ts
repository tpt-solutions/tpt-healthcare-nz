import { test, expect } from '../../fixtures/admin-auth';
import { AdminRolesPage } from '../../pages/admin/AdminOperationsPage';

test.describe('Role assignments', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await page.route('**/api/v1/practice/roles', async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });
    await page.route('**/api/v1/practice/departments', async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Roles' }).click();
    await page.waitForURL('**/roles');
  });

  test('renders the roles heading and empty state', async ({ page }) => {
    const roles = new AdminRolesPage(page);

    await expect(roles.heading()).toBeVisible();
    await expect(roles.emptyState()).toBeVisible();
  });

  test('shows the role reference legend', async ({ page }) => {
    const roles = new AdminRolesPage(page);

    await expect(roles.roleLegend()).toBeVisible();
    await expect(page.getByText('Practice Admin')).toBeVisible();
    await expect(page.getByText('Clinician')).toBeVisible();
    await expect(page.getByText('Nurse')).toBeVisible();
  });

  test('clicking grant role shows the form', async ({ page }) => {
    const roles = new AdminRolesPage(page);

    await roles.grantRoleButton().click();
    await expect(page.getByText('New role assignment')).toBeVisible();
    await expect(roles.userIdInput()).toBeVisible();
    await expect(roles.roleSelect()).toBeVisible();
  });
});
