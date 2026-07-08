import type { Page } from '@playwright/test';

export class AdminDashboardPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { level: 1 });
  }

  practitionersCard() {
    return this.page.getByText('Practitioners').locator('..');
  }

  enrolledPatientsCard() {
    return this.page.getByText('Enrolled Patients').locator('..');
  }

  accClaimsCard() {
    return this.page.getByText('ACC Claims').locator('..');
  }

  overdueAuditCard() {
    return this.page.getByText('Overdue Audit Items').locator('..');
  }

  recentAccClaimsSection() {
    return this.page.getByRole('heading', { name: 'Recent ACC Claims' });
  }

  practitionerApcSection() {
    return this.page.getByRole('heading', { name: 'Practitioner APC Status' });
  }

  capitationNotice() {
    return this.page.getByText('Next PHO Capitation Cycle');
  }

  backupWidget() {
    return this.page.getByText('Database Backup');
  }
}
