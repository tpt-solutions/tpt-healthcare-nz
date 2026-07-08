import type { Page } from '@playwright/test';

export class PortalAppointmentsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'My Appointments' });
  }

  requestAppointmentButton() {
    return this.page.getByRole('button', { name: 'Request Appointment' });
  }

  upcomingTab() {
    return this.page.getByRole('button', { name: 'upcoming' });
  }

  pastTab() {
    return this.page.getByRole('button', { name: 'past' });
  }

  appointmentCard(practitioner: string) {
    return this.page.getByRole('heading', { name: practitioner }).locator('..').locator('..');
  }

  cancelAppointmentLink() {
    return this.page.getByRole('button', { name: 'Cancel appointment' });
  }

  confirmCancelButton() {
    return this.page.getByRole('button', { name: 'Yes, cancel' });
  }

  keepAppointmentButton() {
    return this.page.getByRole('button', { name: 'Keep appointment' });
  }

  requestModal() {
    return this.page.getByText('Request New Appointment');
  }

  requestModalReasonInput() {
    return this.page.locator('textarea').first();
  }

  sendRequestButton() {
    return this.page.getByRole('button', { name: 'Send request' });
  }
}
