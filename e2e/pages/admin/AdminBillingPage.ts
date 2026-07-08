import type { Page } from '@playwright/test';

export class AdminBillingPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Billing Dashboard' });
  }

  claimStatusSummary() {
    return this.page.getByText('ACC Claims Status');
  }

  claimFilterButton(label: string) {
    return this.page.getByRole('button', { name: new RegExp(`^${label}$`, 'i') });
  }

  claimRow(claimRef: string) {
    return this.page.getByRole('row', { name: new RegExp(claimRef) });
  }

  phoCapitationSection() {
    return this.page.getByText('PHO Capitation');
  }

  revenueChartSection() {
    return this.page.getByText('Monthly Revenue by Funding Type');
  }
}
