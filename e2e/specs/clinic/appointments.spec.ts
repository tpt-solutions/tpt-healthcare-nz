import { test, expect } from '../../fixtures/auth';
import { ClinicAppointmentsPage } from '../../pages/ClinicAppointmentsPage';

const MOCK_PROVIDERS = [
  { id: 'prov-1', name: 'Dr. Hemi Walker', role: 'GP' },
  { id: 'prov-2', name: 'Nurse Mere Parata', role: 'Nurse Practitioner' },
];

const MOCK_APPOINTMENTS = [
  {
    id: 'appt-1',
    patientId: 'patient-001',
    patientName: 'Aroha Ngata',
    patientNhiDisplay: 'ZZZ0032',
    providerId: 'prov-1',
    providerName: 'Dr. Hemi Walker',
    date: new Date().toISOString().slice(0, 10),
    startTime: '09:00',
    endTime: '09:15',
    type: 'General Consultation',
    status: 'booked',
  },
  {
    id: 'appt-2',
    patientId: 'patient-002',
    patientName: 'Tama Wilson',
    patientNhiDisplay: 'ABC1234',
    providerId: 'prov-2',
    providerName: 'Nurse Mere Parata',
    date: new Date().toISOString().slice(0, 10),
    startTime: '10:30',
    endTime: '10:45',
    type: 'Follow-up',
    status: 'arrived',
  },
];

test.describe('Appointments page', () => {
  test.beforeEach(async ({ page, loginAsPractitioner }) => {
    await page.route('**/api/v1/practitioners', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ providers: MOCK_PROVIDERS }),
      });
    });
    await page.route('**/api/v1/appointments**', async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ appointments: MOCK_APPOINTMENTS }),
      });
    });

    await loginAsPractitioner(page);
    await page.getByRole('link', { name: 'Appointments' }).click();
    await page.waitForURL('**/appointments');
  });

  test('renders the appointments heading and filter controls', async ({ page }) => {
    const appts = new ClinicAppointmentsPage(page);

    await expect(appts.heading()).toBeVisible();
    await expect(appts.dateFilter()).toBeVisible();
    await expect(appts.providerFilter()).toBeVisible();
    await expect(appts.newAppointmentButton()).toBeVisible();
  });

  test('displays mocked appointments in the table', async ({ page }) => {
    const appts = new ClinicAppointmentsPage(page);

    await expect(appts.appointmentRow('09:00 – 09:15')).toBeVisible();
    await expect(page.getByText('Aroha Ngata')).toBeVisible();
    await expect(page.locator('table').getByText('Dr. Hemi Walker')).toBeVisible();
    await expect(page.locator('table').getByText('General Consultation')).toBeVisible();
  });

  test('shows arrived status badge', async ({ page }) => {
    await expect(page.getByText('Arrived')).toBeVisible();
  });

  test('patient name links to patient detail', async ({ page }) => {
    const link = page.getByRole('link', { name: 'Aroha Ngata' });
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute('href', '/patients/patient-001');
  });

  test('clicking New Appointment opens the create modal', async ({ page }) => {
    const appts = new ClinicAppointmentsPage(page);

    await appts.newAppointmentButton().click();
    await expect(appts.modalHeading()).toBeVisible();
    await expect(appts.modalPatientIdInput()).toBeVisible();
    await expect(appts.modalProviderSelect()).toBeVisible();
    await expect(appts.modalTypeSelect()).toBeVisible();
    await expect(appts.modalDateInput()).toBeVisible();
    await expect(appts.modalStartTimeInput()).toBeVisible();
    await expect(appts.modalEndTimeInput()).toBeVisible();
  });

  test('create modal shows provider options from API', async ({ page }) => {
    const appts = new ClinicAppointmentsPage(page);

    await appts.newAppointmentButton().click();
    const options = appts.modalProviderSelect().locator('option');
    await expect(options).toHaveCount(2);
    await expect(options.first()).toContainText('Dr. Hemi Walker');
  });

  test('create modal shows appointment type options', async ({ page }) => {
    const appts = new ClinicAppointmentsPage(page);

    await appts.newAppointmentButton().click();
    const options = appts.modalTypeSelect().locator('option');
    const count = await options.count();
    expect(count).toBeGreaterThan(5);
    await expect(appts.modalTypeSelect()).toHaveValue('General Consultation');
  });

  test('submitting the create form posts to the API', async ({ page }) => {
    const appts = new ClinicAppointmentsPage(page);
    let postedBody: Record<string, unknown> | undefined;

    await page.route('**/api/v1/appointments', async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      postedBody = route.request().postDataJSON() as Record<string, unknown>;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'appt-new',
          ...postedBody,
          patientName: 'New Patient',
          patientNhiDisplay: 'XYZ9999',
          providerName: 'Dr. Hemi Walker',
          status: 'booked',
        }),
      });
    });

    await appts.newAppointmentButton().click();
    await appts.modalPatientIdInput().fill('patient-new');
    await appts.modalNotesInput().fill('Annual check-up');
    await appts.modalCreateButton().click();

    await expect(page.locator('table').getByText('New Patient')).toBeVisible();
    expect(postedBody).toBeDefined();
    expect(postedBody!['patientId']).toBe('patient-new');
  });

  test('closing the modal via Cancel hides it', async ({ page }) => {
    const appts = new ClinicAppointmentsPage(page);

    await appts.newAppointmentButton().click();
    await expect(appts.modalHeading()).toBeVisible();
    await appts.modalCancelButton().click();
    await expect(appts.modalHeading()).not.toBeVisible();
  });
});
