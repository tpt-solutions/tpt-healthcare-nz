import type { Page } from '@playwright/test';

export class AdminAuditPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Audit Log' });
  }

  immutableNote() {
    return this.page.getByText('Audit records are immutable');
  }

  actionFilter() {
    return this.page.getByLabel('Action');
  }

  resourceFilter() {
    return this.page.getByLabel('Resource Type');
  }

  practitionerFilter() {
    return this.page.getByLabel('Practitioner / Actor');
  }

  eventCountText() {
    return this.page.getByText(/events matching filters/);
  }

  exportCsvButton() {
    return this.page.getByRole('button', { name: 'Export CSV' });
  }

  previousPageButton() {
    return this.page.getByRole('button', { name: 'Previous' });
  }

  nextPageButton() {
    return this.page.getByRole('button', { name: 'Next' });
  }
}
