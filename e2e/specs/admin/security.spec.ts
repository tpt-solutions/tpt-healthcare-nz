import { test, expect } from '../../fixtures/admin-auth';
import { AdminSecurityPage } from '../../pages/admin/AdminSecurityPage';

test.describe('Security & compliance page', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Security' }).click();
    await page.waitForURL('**/security');
  });

  test('renders the security heading', async ({ page }) => {
    const security = new AdminSecurityPage(page);

    await expect(security.heading()).toBeVisible();
  });

  test('shows HIPC compliance checklist', async ({ page }) => {
    const security = new AdminSecurityPage(page);

    await expect(security.complianceChecklist()).toBeVisible();
    // Verify some key checks are present
    await expect(page.getByText('PHI encrypted at rest')).toBeVisible();
    await expect(page.getByText('TLS 1.2+ on all external connections')).toBeVisible();
    await expect(page.getByText('Audit trail for all health record access')).toBeVisible();
  });

  test('shows breach notification log section', async ({ page }) => {
    const security = new AdminSecurityPage(page);

    await expect(security.breachNotificationLog()).toBeVisible();
    await expect(page.getByText('Privacy Act 2020 s113')).toBeVisible();
  });

  test('shows data retention status table', async ({ page }) => {
    const security = new AdminSecurityPage(page);

    await expect(security.dataRetentionSection()).toBeVisible();
    await expect(page.getByText('audit_events')).toBeVisible();
    await expect(page.getByText('10 years').first()).toBeVisible();
  });

  test('shows active sessions table', async ({ page }) => {
    const security = new AdminSecurityPage(page);

    await expect(security.activeSessionsSection()).toBeVisible();
    await expect(page.getByText('Dr. Hemi Walker').first()).toBeVisible();
    await expect(page.getByText('10.1.2.45')).toBeVisible();
  });
});
