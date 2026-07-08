import { test, expect } from '../../fixtures/admin-auth';
import { AdminACCProviderPage } from '../../pages/admin/AdminOperationsPage';

test.describe('ACC Provider registration', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'ACC Provider' }).click();
    await page.waitForURL('**/acc-provider');
  });

  test('renders the ACC provider heading and input', async ({ page }) => {
    const accProvider = new AdminACCProviderPage(page);

    await expect(accProvider.heading()).toBeVisible();
    await expect(accProvider.providerNumberInput()).toBeVisible();
    await expect(accProvider.verifyButton()).toBeVisible();
  });

  test('shows help section about getting a provider number', async ({ page }) => {
    const accProvider = new AdminACCProviderPage(page);

    await expect(accProvider.helpSection()).toBeVisible();
    await expect(page.getByText('0800 222 070')).toBeVisible();
  });

  test('verifying with empty input shows an error', async ({ page }) => {
    const accProvider = new AdminACCProviderPage(page);

    await accProvider.verifyButton().click();
    await expect(page.getByText('Enter an ACC provider number')).toBeVisible();
  });

  test('verifying a provider number shows the result', async ({ page }) => {
    const accProvider = new AdminACCProviderPage(page);

    await accProvider.providerNumberInput().fill('P12345');
    await accProvider.verifyButton().click();

    await expect(accProvider.verificationResult()).toBeVisible();
    await expect(page.getByText('Active')).toBeVisible();
    await expect(page.getByText('Wellington Acupuncture Clinic Ltd')).toBeVisible();
  });
});
