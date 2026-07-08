import type { Page } from '@playwright/test';

export class PortalPrescriptionsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'My Prescriptions' });
  }

  pharmacNote() {
    return this.page.getByText('PHARMAC-subsidised medicines');
  }

  activePrescriptionsSection() {
    return this.page.getByText(/Active Prescriptions \(\d+\)/);
  }

  pastPrescriptionsSection() {
    return this.page.getByText('Past Prescriptions');
  }

  prescriptionCard(medication: string) {
    return this.page.getByText(medication).locator('..');
  }

  subsidisedBadge() {
    return this.page.getByText('PHARMAC Subsidised').first();
  }
}
