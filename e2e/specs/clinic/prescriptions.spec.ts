import { test, expect } from '../../fixtures/auth';
import { ClinicPrescriptionsPage } from '../../pages/ClinicPrescriptionsPage';

const MOCK_PRESCRIPTIONS = [
  {
    id: 'rx-1',
    patientId: 'patient-001',
    patientName: 'Aroha Ngata',
    patientNhiDisplay: 'ZZZ0032',
    medicationName: 'Metformin 500 mg',
    medicationCode: '10037281000116105',
    dose: '500 mg',
    frequency: 'Twice daily',
    route: 'Oral',
    startDate: '2026-05-05',
    prescriber: 'Dr. Hemi Walker',
    status: 'active',
    repeatsRemaining: 3,
    totalRepeats: 5,
    dispensedCount: 2,
  },
  {
    id: 'rx-2',
    patientId: 'patient-002',
    patientName: 'Tama Wilson',
    patientNhiDisplay: 'ABC1234',
    medicationName: 'Lisinopril 10 mg',
    medicationCode: '10119001000116100',
    dose: '10 mg',
    frequency: 'Once daily',
    route: 'Oral',
    startDate: '2026-04-12',
    prescriber: 'Dr. Hemi Walker',
    status: 'active',
    repeatsRemaining: 5,
    totalRepeats: 6,
    dispensedCount: 1,
  },
  {
    id: 'rx-3',
    patientId: 'patient-001',
    patientName: 'Aroha Ngata',
    patientNhiDisplay: 'ZZZ0032',
    medicationName: 'Amoxicillin 500 mg',
    dose: '500 mg',
    frequency: 'Three times daily',
    route: 'Oral',
    startDate: '2025-11-20',
    endDate: '2025-11-27',
    prescriber: 'Dr. Hemi Walker',
    status: 'completed',
    repeatsRemaining: 0,
    totalRepeats: 0,
    dispensedCount: 1,
  },
];

test.describe('Prescriptions page', () => {
  test.beforeEach(async ({ page, loginAsPractitioner }) => {
    await page.route('**/api/v1/prescriptions**', async (route) => {
      const url = new URL(route.request().url());
      const status = url.searchParams.get('status') ?? '';
      const search = url.searchParams.get('search') ?? '';

      let filtered = MOCK_PRESCRIPTIONS;
      if (status) filtered = filtered.filter(rx => rx.status === status);
      if (search) {
        const q = search.toLowerCase();
        filtered = filtered.filter(rx =>
          rx.medicationName.toLowerCase().includes(q) ||
          rx.patientName.toLowerCase().includes(q),
        );
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ prescriptions: filtered, total: filtered.length }),
      });
    });

    await loginAsPractitioner(page);
    await page.getByRole('link', { name: 'Prescriptions' }).click();
    await page.waitForURL('**/prescriptions');
  });

  test('renders the prescriptions heading', async ({ page }) => {
    const rx = new ClinicPrescriptionsPage(page);

    await expect(rx.heading()).toBeVisible();
  });

  test('shows filter controls', async ({ page }) => {
    const rx = new ClinicPrescriptionsPage(page);

    await expect(rx.searchInput()).toBeVisible();
    await expect(rx.statusFilter()).toBeVisible();
  });

  test('defaults to active status filter', async ({ page }) => {
    const rx = new ClinicPrescriptionsPage(page);

    await expect(rx.statusFilter()).toHaveValue('active');
  });

  test('displays active prescriptions with details', async ({ page }) => {
    const rx = new ClinicPrescriptionsPage(page);

    await expect(rx.resultCount()).toBeVisible();
    await expect(rx.prescriptionRow('Metformin 500 mg')).toBeVisible();
    await expect(rx.prescriptionRow('Lisinopril 10 mg')).toBeVisible();
  });

  test('shows medication details: dose, frequency, route', async ({ page }) => {
    await expect(page.getByText('500 mg — Twice daily')).toBeVisible();
    await expect(page.getByText('Oral').first()).toBeVisible();
  });

  test('shows repeats remaining', async ({ page }) => {
    await expect(page.getByText('3 / 5')).toBeVisible();
    await expect(page.getByText('5 / 6')).toBeVisible();
  });

  test('shows prescriber name', async ({ page }) => {
    await expect(page.getByText('Dr. Hemi Walker').first()).toBeVisible();
  });

  test('shows NZMT medication codes', async ({ page }) => {
    await expect(page.getByText('10037281000116105')).toBeVisible();
  });

  test('patient name links to patient detail', async ({ page }) => {
    const link = page.getByRole('link', { name: 'Aroha Ngata' }).first();
    await expect(link).toHaveAttribute('href', '/patients/patient-001');
  });

  test('filtering by status shows only matching prescriptions', async ({ page }) => {
    const rx = new ClinicPrescriptionsPage(page);

    await rx.statusFilter().selectOption('completed');
    await expect(page.getByText('Amoxicillin 500 mg')).toBeVisible();
    await expect(rx.prescriptionRow('Metformin 500 mg')).not.toBeVisible();
  });

  test('search filters by medication name', async ({ page }) => {
    const rx = new ClinicPrescriptionsPage(page);

    await rx.searchInput().fill('Lisinopril');
    await expect(rx.prescriptionRow('Lisinopril 10 mg')).toBeVisible();
    await expect(rx.prescriptionRow('Metformin 500 mg')).not.toBeVisible();
  });

  test('search filters by patient name', async ({ page }) => {
    const rx = new ClinicPrescriptionsPage(page);

    await rx.searchInput().fill('Tama');
    await expect(rx.prescriptionRow('Lisinopril 10 mg')).toBeVisible();
    await expect(rx.prescriptionRow('Metformin 500 mg')).not.toBeVisible();
  });

  test('shows empty state when no results match', async ({ page }) => {
    const rx = new ClinicPrescriptionsPage(page);

    await rx.searchInput().fill('NonexistentDrug');
    await expect(rx.emptyState()).toBeVisible();
  });
});
