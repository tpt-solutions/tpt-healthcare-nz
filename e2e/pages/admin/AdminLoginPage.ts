import type { Page } from '@playwright/test';

export class AdminLoginPage {
  constructor(private readonly page: Page) {}

  async goto() {
    await this.page.goto('/login');
  }

  async login(email: string, password: string) {
    await this.page.getByLabel('Work email').fill(email);
    await this.page.getByLabel('Password').fill(password);
    await this.page.getByRole('button', { name: 'Sign in' }).click();
  }

  errorBanner() {
    return this.page.locator('.bg-red-50');
  }

  submitButton() {
    return this.page.getByRole('button', { name: 'Sign in' });
  }
}
