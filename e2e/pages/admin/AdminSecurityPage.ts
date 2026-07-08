import type { Page } from '@playwright/test';

export class AdminSecurityPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Security & Compliance' });
  }

  complianceChecklist() {
    return this.page.getByText('HIPC Compliance Checklist');
  }

  breachNotificationLog() {
    return this.page.getByText('Breach Notification Log');
  }

  dataRetentionSection() {
    return this.page.getByText('Data Retention Status');
  }

  activeSessionsSection() {
    return this.page.getByText('Active Sessions');
  }

  complianceCount() {
    return this.page.locator('.bg-green-50, .bg-red-50').first().locator('p.text-2xl');
  }

  terminateButton(userName: string) {
    return this.page.getByRole('row', { name: new RegExp(userName) }).getByRole('button', { name: 'Terminate' });
  }
}
