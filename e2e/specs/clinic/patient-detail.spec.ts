import { test, expect } from '../../fixtures/auth';
import { PatientDetailPage } from '../../pages/PatientDetailPage';
import { TEST_PATIENT, TEST_PATIENT_LIST } from '../../fixtures/test-data';

test.describe('Patient detail', () => {
  test.beforeEach(async ({ page, loginAsPractitioner }) => {
    // Navigating here goes via the patient list's "View" link (see
    // PatientDetailPage.gotoFromList), so the list endpoint needs mocking too.
    await page.route('**/api/v1/patients**', async (route) => {
      if (route.request().url().includes(`/patients/${TEST_PATIENT.id}`)) return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(TEST_PATIENT_LIST),
      });
    });
    await page.route(`**/api/v1/patients/${TEST_PATIENT.id}`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(TEST_PATIENT),
      });
    });
    await page.route(`**/api/v1/patients/${TEST_PATIENT.id}/encounters`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ encounters: [] }),
      });
    });

    await loginAsPractitioner(page);
  });

  test('renders the patient banner and key clinical sections', async ({ page }) => {
    const detail = new PatientDetailPage(page);
    await detail.gotoFromList(TEST_PATIENT.name);

    await expect(detail.bannerHeading(TEST_PATIENT.name)).toBeVisible();
    await expect(page.getByText(TEST_PATIENT.nhiDisplay)).toBeVisible();
    // Appears twice: once in the banner's allergy badge, once in the overview panel's list.
    await expect(page.getByText(TEST_PATIENT.allergies[0]).first()).toBeVisible();
  });

  test('switching tabs loads the encounters panel', async ({ page }) => {
    const detail = new PatientDetailPage(page);
    await detail.gotoFromList(TEST_PATIENT.name);

    await detail.tab('Encounters').click();
    await expect(page.getByText('No encounters recorded.')).toBeVisible();
  });
});
