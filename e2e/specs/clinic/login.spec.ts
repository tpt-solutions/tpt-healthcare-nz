import { test, expect } from '../../fixtures/auth';
import { LoginPage } from '../../pages/LoginPage';
import { TEST_PRACTITIONER } from '../../fixtures/test-data';

test.describe('Clinic login', () => {
  test('valid credentials redirect to the dashboard', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(TEST_PRACTITIONER.email, TEST_PRACTITIONER.password);

    await page.waitForURL('**/dashboard');
    await expect(page).toHaveURL(/\/dashboard$/);
  });

  test('invalid credentials show an error and stay on the login page', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(TEST_PRACTITIONER.email, 'wrong-password');

    await expect(loginPage.errorBanner()).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
  });
});
