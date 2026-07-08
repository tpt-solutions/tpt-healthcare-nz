import { test, expect } from '../../fixtures/admin-auth';
import { AdminDepartmentsPage } from '../../pages/admin/AdminOperationsPage';

test.describe('Departments', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await page.route('**/api/v1/practice/departments', async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Departments' }).click();
    await page.waitForURL('**/departments');
  });

  test('renders the departments heading and empty state', async ({ page }) => {
    const departments = new AdminDepartmentsPage(page);

    await expect(departments.heading()).toBeVisible();
    await expect(departments.emptyState()).toBeVisible();
  });

  test('clicking add department shows the form', async ({ page }) => {
    const departments = new AdminDepartmentsPage(page);

    await departments.addDepartmentButton().click();
    await expect(page.getByText('New department')).toBeVisible();
    await expect(departments.nameInput()).toBeVisible();
    await expect(departments.codeInput()).toBeVisible();
  });
});
