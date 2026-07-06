import type { Page } from '@playwright/test';

export class NewPatientPage {
  constructor(private readonly page: Page) {}

  /** Navigates via the AppShell header link rather than page.goto() — the app
   * keeps its JWT in memory only, so a full page navigation would de-auth. */
  async goto() {
    await this.page.getByRole('link', { name: 'New Patient' }).click();
    await this.page.waitForURL('**/patients/new');
  }

  async fillRequiredFields(fields: { firstName: string; lastName: string; dateOfBirth: string; gender: string }) {
    await this.page.locator('#firstName').fill(fields.firstName);
    await this.page.locator('#lastName').fill(fields.lastName);
    await this.page.locator('#dob').fill(fields.dateOfBirth);
    await this.page.locator('#gender').selectOption(fields.gender);
  }

  async submit() {
    await this.page.getByRole('button', { name: 'Register Patient' }).click();
  }
}
