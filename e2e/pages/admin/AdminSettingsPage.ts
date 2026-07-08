import type { Page } from '@playwright/test';

export class AdminSettingsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Practice Settings' });
  }

  saveButton() {
    return this.page.getByRole('button', { name: /Save Changes|Saved/ });
  }

  savedConfirmation() {
    return this.page.getByText('Settings saved successfully');
  }

  practiceNameInput() {
    return this.page.getByLabel('Practice Name');
  }

  hpiFacilityIdInput() {
    return this.page.getByLabel('HPI Facility ID');
  }

  autoLockSelect() {
    return this.page.getByLabel('Auto-lock after inactivity');
  }

  telehealthToggle() {
    return this.page.getByText('Telehealth appointments').locator('..');
  }

  accClaimingToggle() {
    return this.page.getByText('ACC claiming').locator('..');
  }

  hipcConsentToggle() {
    return this.page.getByText('HIPC consent gate').locator('..');
  }

  appearanceSection() {
    return this.page.getByText('Appearance').first();
  }

  themeButtons() {
    return this.page.locator('.rounded-xl.border-2.p-3');
  }
}
