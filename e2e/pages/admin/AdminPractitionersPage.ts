import type { Page } from '@playwright/test';

export class AdminPractitionersPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Practitioners' });
  }

  searchInput() {
    return this.page.getByPlaceholder('Search by name, HPI CPN, or type...');
  }

  practitionerRow(name: string) {
    return this.page.getByRole('row', { name: new RegExp(name) });
  }

  apcAlertBanner() {
    return this.page.getByText('APC Action Required');
  }

  hpcaNote() {
    return this.page.getByText('HPCA Act 2003');
  }
}
