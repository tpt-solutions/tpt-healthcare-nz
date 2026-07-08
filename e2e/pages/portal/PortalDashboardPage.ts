import type { Page } from '@playwright/test';

export class PortalDashboardPage {
  constructor(private readonly page: Page) {}

  greeting() {
    return this.page.getByRole('heading', { name: /Kia ora/ });
  }

  nextAppointmentCard() {
    return this.page.getByText('Next Appointment').locator('..');
  }

  activeMedicationsCard() {
    return this.page.getByText('Active Medications').locator('..');
  }

  resultsToReviewCard() {
    return this.page.getByText('Results to Review').locator('..');
  }

  upcomingAppointmentsSection() {
    return this.page.getByRole('heading', { name: 'Upcoming Appointments' });
  }

  recentResultsSection() {
    return this.page.getByRole('heading', { name: 'Recent Test Results' });
  }

  checkInBanner() {
    return this.page.getByText('You have an appointment today');
  }

  checkInButton() {
    return this.page.getByRole('button', { name: 'Check in' });
  }
}
