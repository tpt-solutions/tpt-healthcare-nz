import { test, expect } from '../../fixtures/admin-auth';
import { AdminClinicsPage } from '../../pages/admin/AdminClinicsPage';

test.describe('Clinics management', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    // Clinics page loads data from the API, so mock it
    await page.route('**/api/v1/admin/applications**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          applications: [
            {
              id: 'app-1',
              practice_name: 'Wellington Medical Centre',
              hpi_facility_id: 'F0W0123',
              contact_name: 'Dr. Sarah Jones',
              contact_email: 'sarah@wellingtonmedical.nz',
              status: 'pending',
              submitted_at: '2026-06-01T10:00:00Z',
            },
          ],
        }),
      });
    });
    await page.route('**/api/v1/admin/tenants', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          tenants: [
            {
              id: 'tenant-1',
              name: 'Auckland City Medical Centre',
              hpi_facility_id: 'F0K068-C',
              status: 'active',
              contact_email: 'admin@aucklandcitymedical.nz',
              contact_name: 'Tama Parata',
              created_at: '2025-01-15T00:00:00Z',
            },
          ],
        }),
      });
    });

    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Clinics' }).click();
    await page.waitForURL('**/clinics');
  });

  test('renders the clinics heading and applications tab', async ({ page }) => {
    const clinics = new AdminClinicsPage(page);

    await expect(clinics.heading()).toBeVisible();
    await expect(clinics.applicationsTab()).toBeVisible();
    await expect(clinics.tenantsTab()).toBeVisible();
  });

  test('shows pending applications with approve/reject buttons', async ({ page }) => {
    const clinics = new AdminClinicsPage(page);

    await expect(clinics.applicationRow('Wellington Medical Centre')).toBeVisible();
    await expect(clinics.approveButton('Wellington Medical Centre')).toBeVisible();
    await expect(clinics.rejectButton('Wellington Medical Centre')).toBeVisible();
  });

  test('switching to tenants tab shows active clinics', async ({ page }) => {
    const clinics = new AdminClinicsPage(page);

    await clinics.tenantsTab().click();
    await expect(page.getByText('Auckland City Medical Centre')).toBeVisible();
    await expect(page.getByText('F0K068-C')).toBeVisible();
  });

  test('clicking approve opens the review modal', async ({ page }) => {
    const clinics = new AdminClinicsPage(page);

    await clinics.approveButton('Wellington Medical Centre').click();
    await expect(page.getByText('Approve Application')).toBeVisible();
    await expect(clinics.modalNotesInput()).toBeVisible();
    await expect(clinics.modalConfirmButton()).toBeVisible();
    await expect(clinics.modalCancelButton()).toBeVisible();
  });

  test('clicking reject opens the review modal', async ({ page }) => {
    const clinics = new AdminClinicsPage(page);

    await clinics.rejectButton('Wellington Medical Centre').click();
    await expect(page.getByText('Reject Application')).toBeVisible();
  });
});
