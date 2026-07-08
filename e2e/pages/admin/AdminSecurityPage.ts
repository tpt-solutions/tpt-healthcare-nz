import type { Page } from '@playwright/test';

export class AdminSecurityPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Security & Compliance' });
  }

  complianceChecklist() {
    return this.page.getByRole('heading', { name: 'HIPC Compliance Checklist' });
  }

  breachNotificationLog() {
    return this.page.getByRole('heading', { name: /Breach Notification Log/ });
  }

  dataRetentionSection() {
    return this.page.getByRole('heading', { name: 'Data Retention Status' });
  }

  activeSessionsSection() {
    return this.page.getByRole('heading', { name: 'Active Sessions' });
  }

  terminateButton(userName: string) {
    return this.page.getByRole('row', { name: new RegExp(userName) }).getByRole('button', { name: 'Terminate' });
  }
}
