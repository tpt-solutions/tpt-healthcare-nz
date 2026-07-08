import { test, expect } from '../../fixtures/admin-auth';
import { AdminIntegrationsPage } from '../../pages/admin/AdminOperationsPage';

test.describe('Integrations', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await page.route('**/api/v1/health/providers', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ providers: [] }),
      });
    });

    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'All providers' }).click();
    await page.waitForURL('**/integrations');
  });

  test('renders the integrations heading and empty state', async ({ page }) => {
    const integrations = new AdminIntegrationsPage(page);

    await expect(integrations.heading()).toBeVisible();
    await expect(integrations.emptyState()).toBeVisible();
  });

  test('shows the refresh status button', async ({ page }) => {
    const integrations = new AdminIntegrationsPage(page);

    await expect(integrations.refreshButton()).toBeVisible();
  });
});
