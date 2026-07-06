import { test, expect } from '../../fixtures/auth';
import { NewPatientPage } from '../../pages/NewPatientPage';
import { TEST_PATIENT } from '../../fixtures/test-data';

test.describe('New patient registration', () => {
  test.beforeEach(async ({ page, loginAsPractitioner }) => {
    await loginAsPractitioner(page);
  });

  test('submits the required fields and navigates to the new patient record', async ({ page }) => {
    let submittedBody: Record<string, unknown> | undefined;

    await page.route('**/api/v1/patients', async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      submittedBody = route.request().postDataJSON() as Record<string, unknown>;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: TEST_PATIENT.id }),
      });
    });
    await page.route(`**/api/v1/patients/${TEST_PATIENT.id}`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(TEST_PATIENT),
      });
    });

    const newPatient = new NewPatientPage(page);
    await newPatient.goto();
    await newPatient.fillRequiredFields({
      firstName: 'Jane',
      lastName: 'Test',
      dateOfBirth: '1985-04-12',
      gender: 'female',
    });
    await newPatient.submit();

    await page.waitForURL(`**/patients/${TEST_PATIENT.id}`);
    expect(submittedBody).toMatchObject({ firstName: 'Jane', lastName: 'Test', gender: 'female' });
  });
});
