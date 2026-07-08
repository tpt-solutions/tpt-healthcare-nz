import type { Page } from '@playwright/test';

export class ClinicPrescriptionsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Prescriptions' });
  }

  searchInput() {
    return this.page.locator('#rx-search');
  }

  statusFilter() {
    return this.page.locator('#rx-status');
  }

  resultCount() {
    return this.page.getByText(/\d+ prescriptions?/);
  }

  prescriptionRow(medication: string) {
    return this.page.getByRole('row', { name: new RegExp(medication) });
  }

  emptyState() {
    return this.page.getByText('No prescriptions found.');
  }

  loadingState() {
    return this.page.getByText('Loading…');
  }
}
