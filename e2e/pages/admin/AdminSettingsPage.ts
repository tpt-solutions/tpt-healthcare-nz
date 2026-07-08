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

  /** Practice name input — label isn't associated via for/id, so locate by position. */
  practiceNameInput() {
    return this.page.locator('input').filter({ hasText: '' }).nth(0);
  }

  hpiFacilityIdInput() {
    return this.page.locator('input.font-mono').first();
  }

  autoLockSelect() {
    return this.page.getByText('Auto-lock after inactivity').locator('..').locator('select');
  }

  appearanceSection() {
    return this.page.getByText('Appearance').first();
  }

  themeButtons() {
    return this.page.locator('.rounded-xl.border-2.p-3');
  }
}
