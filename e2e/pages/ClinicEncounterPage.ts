import type { Page } from '@playwright/test';

export class ClinicEncounterPage {
  constructor(private readonly page: Page) {}

  heading() {
    // Use the h2 with the reason code, not the AppShell h1 title
    return this.page.locator('h2').filter({ hasText: /Encounter/ }).first();
  }

  statusBadge() {
    return this.page.locator('.badge-info').first();
  }

  patientLink() {
    return this.page.locator('a[href^="/patients/"]').first();
  }

  clinicianName() {
    return this.page.getByText('Clinician:').locator('..').locator('.font-medium');
  }

  clinicalNotesSection() {
    return this.page.getByRole('heading', { name: 'Clinical Notes' });
  }

  diagnosesSection() {
    return this.page.getByRole('heading', { name: 'Diagnoses' });
  }

  observationsSection() {
    return this.page.getByRole('heading', { name: 'Observations' });
  }

  prescriptionsSection() {
    return this.page.getByRole('heading', { name: 'Prescriptions Issued' });
  }

  observationRow(measurement: string) {
    return this.page.getByRole('row', { name: new RegExp(measurement) });
  }

  loadingSpinner() {
    return this.page.locator('.animate-spin');
  }

  errorBanner() {
    return this.page.getByText('Encounter not found');
  }
}
