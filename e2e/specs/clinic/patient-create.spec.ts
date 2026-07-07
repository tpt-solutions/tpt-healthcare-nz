import { test as mockedTest, expect as mockedExpect } from '../../fixtures/auth';
import { test as realBackendTest, expect as realBackendExpect } from '../../fixtures/real-backend-auth';
import { NewPatientPage } from '../../pages/NewPatientPage';
import { TEST_PATIENT } from '../../fixtures/test-data';

// Set E2E_REAL_BACKEND=true to run this spec against a real tpt-doctor +
// Postgres stack instead of the mocked API boundary. Start the stack first:
//   docker compose -f deploy/docker-compose.dev.yml up -d postgres redis tpt-doctor
//   E2E_REAL_BACKEND=true pnpm test:e2e --grep "New patient registration"
// See CONTRIBUTING.md for details. Default `pnpm test:e2e` / CI runs the
// mocked variant below and needs no backend at all.
const useRealBackend = process.env.E2E_REAL_BACKEND === 'true';

if (!useRealBackend) {
  mockedTest.describe('New patient registration (mocked API)', () => {
    mockedTest.beforeEach(async ({ page, loginAsPractitioner }) => {
      await loginAsPractitioner(page);
    });

    mockedTest('submits the required fields and navigates to the new patient record', async ({ page }) => {
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
      mockedExpect(submittedBody).toMatchObject({ firstName: 'Jane', lastName: 'Test', gender: 'female' });
    });
  });
} else {
  realBackendTest.describe('New patient registration (real backend)', () => {
    realBackendTest.beforeEach(async ({ page, loginAsPractitioner }) => {
      await loginAsPractitioner(page);
    });

    realBackendTest('submits the required fields and persists a real patient record', async ({ page }) => {
      const newPatient = new NewPatientPage(page);
      await newPatient.goto();
      await newPatient.fillRequiredFields({
        firstName: 'Jane',
        lastName: 'Test',
        dateOfBirth: '1985-04-12',
        gender: 'female',
      });
      await newPatient.submit();

      // No id is known ahead of time — tpt-doctor assigns a real UUID.
      await page.waitForURL(/\/patients\/[0-9a-f-]{36}$/i);
      realBackendExpect(page.url()).toMatch(/\/patients\/[0-9a-f-]{36}$/i);
    });
  });
}
