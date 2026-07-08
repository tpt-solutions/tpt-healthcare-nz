import type { Page } from '@playwright/test';

export class AdminClinicsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Clinics' });
  }

  applicationsTab() {
    return this.page.getByRole('button', { name: 'Applications' });
  }

  tenantsTab() {
    return this.page.getByRole('button', { name: 'Active Clinics' });
  }

  statusFilterButton(label: string) {
    return this.page.getByRole('button', { name: new RegExp(`^${label}$`, 'i') });
  }

  applicationRow(practiceName: string) {
    return this.page.getByRole('row', { name: new RegExp(practiceName) });
  }

  approveButton(practiceName: string) {
    return this.applicationRow(practiceName).getByRole('button', { name: 'Approve' });
  }

  rejectButton(practiceName: string) {
    return this.applicationRow(practiceName).getByRole('button', { name: 'Reject' });
  }

  modalConfirmButton() {
    return this.page.getByRole('button', { name: /Approve & Provision|Reject/ });
  }

  modalCancelButton() {
    return this.page.getByRole('button', { name: 'Cancel' });
  }

  modalNotesInput() {
    return this.page.locator('textarea');
  }
}
