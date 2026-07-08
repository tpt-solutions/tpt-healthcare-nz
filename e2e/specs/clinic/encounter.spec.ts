import { test, expect } from '../../fixtures/auth';
import { ClinicEncounterPage } from '../../pages/ClinicEncounterPage';
import { TEST_PRACTITIONER } from '../../fixtures/test-data';

const MOCK_ENCOUNTER = {
  id: 'enc-001',
  patientId: 'patient-001',
  patientName: 'Aroha Ngata',
  patientNhiDisplay: 'ZZZ0032',
  practitionerId: 'prov-1',
  practitionerName: 'Dr. Hemi Walker',
  date: '2026-06-01T10:00:00Z',
  status: 'finished',
  type: 'ambulatory',
  reasonCode: { system: 'http://snomed.info/sct', code: '185349003', display: 'Encounter for check up' },
  notes: 'Patient presents for routine diabetes review. HbA1c results reviewed. Metformin dose maintained.\n\nNo new concerns raised.',
  observations: [
    {
      id: 'obs-1',
      code: { system: 'http://loinc.org', code: '4548-4', display: 'HbA1c' },
      value: '52',
      unit: 'mmol/mol',
      effectiveDateTime: '2026-06-01T10:15:00Z',
      interpretation: 'H',
    },
    {
      id: 'obs-2',
      code: { system: 'http://loinc.org', code: '85354-9', display: 'Blood pressure' },
      value: '128/82',
      unit: 'mmHg',
      effectiveDateTime: '2026-06-01T10:10:00Z',
    },
  ],
  diagnoses: [
    {
      id: 'dx-1',
      code: { system: 'http://hl7.org/fhir/sid/icd-10-am', code: 'E11', display: 'Type 2 diabetes mellitus' },
      rank: 1,
      use: 'encounter-diagnosis',
    },
  ],
  prescriptionsIssued: [
    {
      id: 'rx-1',
      medicationName: 'Metformin 500 mg',
      dose: '500 mg',
      frequency: 'Twice daily',
    },
  ],
};

/**
 * The clinic auth stores JWT in module-level variables that reset on page
 * reload. We use addInitScript to re-set them after each navigation.
 */
test.describe('Encounter detail page', () => {
  test.beforeEach(async ({ page, loginAsPractitioner }) => {
    // Mock the encounter API
    await page.route('**/api/v1/encounters/enc-001', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_ENCOUNTER),
      });
    });

    // Login first (sets in-memory token)
    await loginAsPractitioner(page);

    // After login, the token is in module scope. We need to persist it across
    // the page.goto() reload. Use addInitScript to re-auth on next load.
    await page.addInitScript((token) => {
      // Intercept the AuthProvider's login by pre-setting sessionStorage
      // The clinic auth checks sessionStorage on mount (see AuthContext useEffect)
      // Actually it uses module-level vars — but we can hack it via the fetch mock:
      // just re-trigger login after page load by mocking the auth endpoint.
    }, TEST_PRACTITIONER.accessToken);

    // Instead of page.goto(), use the auth mock to re-authenticate after navigation.
    // Mock the auth endpoint so login succeeds on the next page load.
    await page.route('**/api/v1/auth/token', async (route) => {
      const body = route.request().postDataJSON() as { email: string; password: string };
      if (body.email === TEST_PRACTITIONER.email && body.password === TEST_PRACTITIONER.password) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            access_token: TEST_PRACTITIONER.accessToken,
            user: TEST_PRACTITIONER.user,
          }),
        });
      } else {
        await route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ message: 'Invalid' }) });
      }
    });

    // Navigate to the encounter page — this triggers a full reload which
    // clears the in-memory token. The app redirects to /login, but since we
    // mocked the auth endpoint, we can auto-login.
    await page.goto('/encounters/enc-001');

    // The app redirected to /login. Auto-login:
    await page.getByLabel('Email address').fill(TEST_PRACTITIONER.email);
    await page.getByLabel('Password').fill(TEST_PRACTITIONER.password);
    await page.getByRole('button', { name: 'Sign in' }).click();

    // After login, the app should redirect back to /encounters/enc-001
    // (via the `from` state in ProtectedRoute). Wait for the encounter to load.
    await page.waitForTimeout(1000);
  });

  test('renders the encounter heading and status', async ({ page }) => {
    const encounter = new ClinicEncounterPage(page);

    await expect(encounter.heading()).toBeVisible();
    await expect(encounter.statusBadge()).toBeVisible();
  });

  test('displays patient and clinician info', async ({ page }) => {
    await expect(page.getByText('Aroha Ngata')).toBeVisible();
    await expect(page.getByText('ZZZ0032')).toBeVisible();
    await expect(page.getByText('Dr. Hemi Walker').first()).toBeVisible();
  });

  test('shows the encounter reason code', async ({ page }) => {
    await expect(page.getByText('Encounter for check up')).toBeVisible();
  });

  test('displays clinical notes', async ({ page }) => {
    const encounter = new ClinicEncounterPage(page);

    await expect(encounter.clinicalNotesSection()).toBeVisible();
    await expect(page.getByText('routine diabetes review')).toBeVisible();
  });

  test('displays diagnoses with ICD-10 codes', async ({ page }) => {
    const encounter = new ClinicEncounterPage(page);

    await expect(encounter.diagnosesSection()).toBeVisible();
    await expect(page.getByText('Type 2 diabetes mellitus')).toBeVisible();
    await expect(page.getByText('E11')).toBeVisible();
  });

  test('displays observations in a table', async ({ page }) => {
    const encounter = new ClinicEncounterPage(page);

    await expect(encounter.observationsSection()).toBeVisible();
    await expect(encounter.observationRow('HbA1c')).toBeVisible();
    await expect(page.getByText('52').first()).toBeVisible();
    await expect(page.getByText('mmol/mol')).toBeVisible();
  });

  test('displays blood pressure observation', async ({ page }) => {
    await expect(page.getByText('Blood pressure')).toBeVisible();
    await expect(page.getByText('128/82')).toBeVisible();
  });

  test('displays prescriptions issued', async ({ page }) => {
    const encounter = new ClinicEncounterPage(page);

    await expect(encounter.prescriptionsSection()).toBeVisible();
    await expect(page.getByText('Metformin 500 mg')).toBeVisible();
    await expect(page.getByText('Twice daily')).toBeVisible();
  });

  test('patient name links to patient detail', async ({ page }) => {
    const link = page.getByRole('link', { name: 'Aroha Ngata' });
    await expect(link).toHaveAttribute('href', '/patients/patient-001');
  });

  test('View link on prescription links to prescriptions page', async ({ page }) => {
    const rxLink = page.getByRole('link', { name: 'View' });
    await expect(rxLink).toHaveAttribute('href', '/prescriptions');
  });
});

test.describe('Encounter page — error state', () => {
  test('shows error when encounter not found', async ({ page, loginAsPractitioner }) => {
    await page.route('**/api/v1/encounters/bad-id', async (route) => {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Not found' }),
      });
    });

    // Mock auth for re-login after page.goto()
    await page.route('**/api/v1/auth/token', async (route) => {
      const body = route.request().postDataJSON() as { email: string; password: string };
      if (body.email === TEST_PRACTITIONER.email) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ access_token: TEST_PRACTITIONER.accessToken, user: TEST_PRACTITIONER.user }),
        });
      } else {
        await route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ message: 'Invalid' }) });
      }
    });

    await page.goto('/encounters/bad-id');
    await page.getByLabel('Email address').fill(TEST_PRACTITIONER.email);
    await page.getByLabel('Password').fill(TEST_PRACTITIONER.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForTimeout(1000);

    await expect(page.getByText('Encounter not found')).toBeVisible();
  });
});
