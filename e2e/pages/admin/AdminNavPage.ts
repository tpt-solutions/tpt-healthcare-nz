import type { Page } from '@playwright/test';

export class AdminNavPage {
  constructor(private readonly page: Page) {}

  navLink(label: string) {
    return this.page.getByRole('link', { name: label });
  }

  async navigateTo(label: string) {
    await this.navLink(label).click();
  }

  signOutButton() {
    return this.page.getByRole('button', { name: 'Sign Out' });
  }

  async signOut() {
    await this.signOutButton().click();
    await this.page.waitForURL('**/login');
  }
}
