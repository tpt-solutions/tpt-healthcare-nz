import { test, expect } from '../../fixtures/admin-auth';
import { AdminDashboardPage } from '../../pages/admin/AdminDashboardPage';

test.describe('Admin dashboard', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
  });

  test('renders the practice name and KPI cards', async ({ page }) => {
    const dashboard = new AdminDashboardPage(page);

    await expect(dashboard.heading()).toBeVisible();
    await expect(dashboard.practitionersCard()).toBeVisible();
    await expect(dashboard.enrolledPatientsCard()).toBeVisible();
    await expect(dashboard.accClaimsCard()).toBeVisible();
    await expect(dashboard.overdueAuditCard()).toBeVisible();
  });

  test('shows the recent ACC claims table', async ({ page }) => {
    const dashboard = new AdminDashboardPage(page);

    await expect(dashboard.recentAccClaimsSection()).toBeVisible();
    await expect(page.getByText('ACC-2026-00891')).toBeVisible();
    await expect(page.getByText('M. Tūhoe')).toBeVisible();
  });

  test('shows the practitioner APC status table', async ({ page }) => {
    const dashboard = new AdminDashboardPage(page);

    await expect(dashboard.practitionerApcSection()).toBeVisible();
    await expect(page.getByText('Dr. Hemi Walker')).toBeVisible();
    await expect(page.getByText('APC Current').first()).toBeVisible();
  });

  test('shows the capitation cycle notice', async ({ page }) => {
    const dashboard = new AdminDashboardPage(page);

    await expect(dashboard.capitationNotice()).toBeVisible();
  });

  test('shows the backup status widget', async ({ page }) => {
    const dashboard = new AdminDashboardPage(page);

    await expect(dashboard.backupWidget()).toBeVisible();
  });
});
