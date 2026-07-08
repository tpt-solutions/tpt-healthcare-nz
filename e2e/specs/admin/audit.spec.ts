import { test, expect } from '../../fixtures/admin-auth';
import { AdminAuditPage } from '../../pages/admin/AdminAuditPage';

test.describe('Audit log page', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Audit Log' }).click();
    await page.waitForURL('**/audit');
  });

  test('renders the audit log heading and immutability notice', async ({ page }) => {
    const audit = new AdminAuditPage(page);

    await expect(audit.heading()).toBeVisible();
    await expect(audit.immutableNote()).toBeVisible();
  });

  test('shows filter controls and event count', async ({ page }) => {
    const audit = new AdminAuditPage(page);

    await expect(audit.actionFilter()).toBeVisible();
    await expect(audit.resourceFilter()).toBeVisible();
    await expect(audit.practitionerFilter()).toBeVisible();
    await expect(audit.eventCountText()).toBeVisible();
  });

  test('shows audit events in the table', async ({ page }) => {
    await expect(page.getByText('Dr. Hemi Walker').first()).toBeVisible();
    await expect(page.getByText('Patient').first()).toBeVisible();
    await expect(page.getByText('read').first()).toBeVisible();
    await expect(page.getByText('create').first()).toBeVisible();
  });

  test('filtering by action narrows results', async ({ page }) => {
    const audit = new AdminAuditPage(page);

    await audit.actionFilter().selectOption('create');
    await expect(page.getByText('read').first()).not.toBeVisible();
  });

  test('export CSV button is present', async ({ page }) => {
    const audit = new AdminAuditPage(page);

    await expect(audit.exportCsvButton()).toBeVisible();
  });
});
