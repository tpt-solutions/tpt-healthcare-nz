import { test, expect } from '../../fixtures/admin-auth';
import { AdminPractitionersPage } from '../../pages/admin/AdminPractitionersPage';

test.describe('Practitioners page', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Practitioners' }).click();
    await page.waitForURL('**/practitioners');
  });

  test('renders the practitioners heading and table', async ({ page }) => {
    const practitioners = new AdminPractitionersPage(page);

    await expect(practitioners.heading()).toBeVisible();
    await expect(practitioners.practitionerRow('Dr. Hemi Walker')).toBeVisible();
    await expect(practitioners.practitionerRow('Dr. Piripi Te Aho')).toBeVisible();
    await expect(practitioners.practitionerRow('Nurse Mere Parata')).toBeVisible();
  });

  test('shows APC alert banner for expiring practitioners', async ({ page }) => {
    const practitioners = new AdminPractitionersPage(page);

    await expect(practitioners.apcAlertBanner()).toBeVisible();
    await expect(page.getByText('Dr. Sione Tuilagi')).toBeVisible();
  });

  test('shows HPCA compliance note', async ({ page }) => {
    const practitioners = new AdminPractitionersPage(page);

    await expect(practitioners.hpcaNote()).toBeVisible();
  });

  test('search input filters practitioners', async ({ page }) => {
    const practitioners = new AdminPractitionersPage(page);

    await practitioners.searchInput().fill('Nurse');
    await expect(practitioners.practitionerRow('Nurse Mere Parata')).toBeVisible();
    await expect(practitioners.practitionerRow('Dr. Hemi Walker')).not.toBeVisible();
  });
});
