import type { Page } from '@playwright/test';

export class AdminAuditPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Audit Log' });
  }

  immutableNote() {
    return this.page.getByText('Audit records are immutable');
  }

  /** Labels in this page aren't associated via for/id — find the select by its label text parent. */
  actionFilter() {
    return this.page.getByText('Action', { exact: true }).locator('..').locator('select');
  }

  resourceFilter() {
    return this.page.getByText('Resource Type', { exact: true }).locator('..').locator('select');
  }

  practitionerFilter() {
    return this.page.getByText('Practitioner / Actor', { exact: true }).locator('..').locator('select');
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
