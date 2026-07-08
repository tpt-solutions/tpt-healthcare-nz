import type { Page } from '@playwright/test';

export class PortalBookAppointmentPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Book appointment' });
  }

  dateInput() {
    return this.page.locator('input[type="date"]');
  }

  nextChooseTimeButton() {
    return this.page.getByRole('button', { name: 'Next — choose time' });
  }

  timeSlotButton(time: string) {
    return this.page.getByRole('button', { name: time });
  }

  nextReasonButton() {
    return this.page.getByRole('button', { name: 'Next — reason for visit' });
  }

  reasonTextarea() {
    return this.page.locator('textarea');
  }

  reviewBookingButton() {
    return this.page.getByRole('button', { name: 'Review booking' });
  }

  confirmBookingButton() {
    return this.page.getByRole('button', { name: 'Confirm booking' });
  }

  successHeading() {
    return this.page.getByRole('heading', { name: 'Appointment booked' });
  }

  viewAppointmentsButton() {
    return this.page.getByRole('button', { name: 'View my appointments' });
  }

  backButton() {
    return this.page.locator('button').filter({ has: this.page.locator('svg') }).first();
  }
}
