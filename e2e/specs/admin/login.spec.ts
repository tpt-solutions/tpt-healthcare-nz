import { test, expect } from '../../fixtures/admin-auth';
import { AdminLoginPage } from '../../pages/admin/AdminLoginPage';
import { TEST_ADMIN } from '../../fixtures/admin-auth';

test.describe('Admin login', () => {
  test('valid credentials redirect to the dashboard', async ({ page }) => {
    const loginPage = new AdminLoginPage(page);
    await loginPage.goto();
    await loginPage.login(TEST_ADMIN.email, TEST_ADMIN.password);

    await page.waitForURL('**/dashboard');
    await expect(page).toHaveURL(/\/dashboard$/);
  });

  test('empty email shows a browser validation error', async ({ page }) => {
    const loginPage = new AdminLoginPage(page);
    await loginPage.goto();

    await loginPage.submitButton().click();
    // The HTML required attribute prevents submission, so we stay on /login
    await expect(page).toHaveURL(/\/login$/);
  });

  test('the login page shows the Admin Portal branding', async ({ page }) => {
    const loginPage = new AdminLoginPage(page);
    await loginPage.goto();

    await expect(page.getByRole('heading', { name: 'Admin Portal' })).toBeVisible();
    await expect(page.getByText('TPT Healthcare Practice Administration')).toBeVisible();
  });
});
