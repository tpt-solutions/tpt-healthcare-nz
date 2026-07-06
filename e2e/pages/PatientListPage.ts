import type { Page } from '@playwright/test';

export class PatientListPage {
  constructor(private readonly page: Page) {}

  /** Navigates via the in-app nav link rather than page.goto() — the app
   * keeps its JWT in memory only, so a full page navigation would de-auth. */
  async goto() {
    await this.page.getByRole('link', { name: 'Patients' }).click();
    await this.page.waitForURL('**/patients');
  }

  nameSearchInput() {
    return this.page.getByLabel('Search by name');
  }

  newPatientLink() {
    return this.page.getByRole('link', { name: 'New Patient' });
  }

  patientRow(name: string) {
    return this.page.getByRole('row', { name: new RegExp(name) });
  }

  viewLinkForPatient(name: string) {
    return this.patientRow(name).getByRole('link', { name: 'View' });
  }
}
