import type { Page } from '@playwright/test';

export class PortalConsentPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Manage My Consent' });
  }

  hipcExplainer() {
    return this.page.getByText('Your rights under the Health Information Privacy Code');
  }

  grantedCount() {
    return this.page.getByText('Consents Granted').locator('..').locator('p.text-2xl');
  }

  revokedCount() {
    return this.page.getByText('Consents Revoked').locator('..').locator('p.text-2xl');
  }

  consentItem(title: string) {
    return this.page.getByText(title).locator('..');
  }

  detailsButton(title: string) {
    return this.consentItem(title).getByRole('button', { name: 'Details' });
  }

  revokeButton(title: string) {
    return this.consentItem(title).getByRole('button', { name: 'Revoke' });
  }

  grantButton(title: string) {
    return this.consentItem(title).getByRole('button', { name: 'Grant' });
  }

  confirmGrantButton() {
    return this.page.getByRole('button', { name: 'Grant consent' });
  }

  confirmRevokeButton() {
    return this.page.getByRole('button', { name: 'Revoke consent' });
  }

  cancelModalButton() {
    return this.page.getByRole('button', { name: 'Cancel' }).last();
  }
}
