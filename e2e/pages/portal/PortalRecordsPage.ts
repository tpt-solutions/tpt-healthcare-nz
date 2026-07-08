import type { Page } from '@playwright/test';

export class PortalRecordsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'My Health Records' });
  }

  nhiBanner() {
    return this.page.getByText('National Health Index (NHI)');
  }

  retentionNote() {
    return this.page.getByText('HIPC Rule 6');
  }

  encountersTab() {
    return this.page.getByRole('button', { name: 'Encounters' });
  }

  diagnosesTab() {
    return this.page.getByRole('button', { name: 'Diagnoses' });
  }

  immunisationsTab() {
    return this.page.getByRole('button', { name: 'Immunisations' });
  }

  encounterCard(practitioner: string) {
    return this.page.getByRole('heading', { name: practitioner }).locator('..').locator('..');
  }

  diagnosisRow(condition: string) {
    return this.page.getByRole('row', { name: new RegExp(condition) });
  }

  immunisationCard(vaccine: string) {
    return this.page.getByText(vaccine).locator('..');
  }
}
