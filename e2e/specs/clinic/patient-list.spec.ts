import { test, expect } from '../../fixtures/auth';
import { PatientListPage } from '../../pages/PatientListPage';
import { TEST_PATIENT, TEST_PATIENT_LIST } from '../../fixtures/test-data';

test.describe('Patient list', () => {
  test.beforeEach(async ({ page, loginAsPractitioner }) => {
    await page.route('**/api/v1/patients**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(TEST_PATIENT_LIST),
      });
    });

    await loginAsPractitioner(page);
  });

  test('shows the mocked patient list on load', async ({ page }) => {
    const patientList = new PatientListPage(page);
    await patientList.goto();

    await expect(patientList.patientRow(TEST_PATIENT.name)).toBeVisible();
    await expect(patientList.viewLinkForPatient(TEST_PATIENT.name)).toBeVisible();
  });

  test('name search re-queries and re-renders results', async ({ page }) => {
    const patientList = new PatientListPage(page);
    await patientList.goto();

    await patientList.nameSearchInput().fill(TEST_PATIENT.name);
    await page.getByRole('button', { name: 'Search' }).click();

    await expect(patientList.patientRow(TEST_PATIENT.name)).toBeVisible();
  });
});
