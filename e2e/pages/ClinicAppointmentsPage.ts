import type { Page } from '@playwright/test';

export class ClinicAppointmentsPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Appointments' });
  }

  dateFilter() {
    return this.page.locator('#appt-date-filter');
  }

  providerFilter() {
    return this.page.locator('#provider-filter');
  }

  newAppointmentButton() {
    return this.page.getByRole('button', { name: 'New Appointment' });
  }

  appointmentRow(time: string) {
    return this.page.getByRole('row', { name: new RegExp(time) });
  }

  emptyState() {
    return this.page.getByText('No appointments for this date and provider.');
  }

  // Modal
  modalHeading() {
    return this.page.getByRole('heading', { name: 'New Appointment' });
  }

  modalPatientIdInput() {
    return this.page.locator('#appt-patient');
  }

  modalProviderSelect() {
    return this.page.locator('#appt-provider');
  }

  modalTypeSelect() {
    return this.page.locator('#appt-type');
  }

  modalDateInput() {
    return this.page.locator('#appt-date');
  }

  modalStartTimeInput() {
    return this.page.locator('#appt-start');
  }

  modalEndTimeInput() {
    return this.page.locator('#appt-end');
  }

  modalNotesInput() {
    return this.page.locator('#appt-notes');
  }

  modalCreateButton() {
    return this.page.getByRole('button', { name: 'Create Appointment' });
  }

  modalCancelButton() {
    return this.page.locator('.fixed.inset-0').getByRole('button', { name: 'Cancel' });
  }
}
