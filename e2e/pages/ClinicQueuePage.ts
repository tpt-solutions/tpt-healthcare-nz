import type { Page } from '@playwright/test';

export class ClinicQueuePage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: "Today's Queue" });
  }

  callNextButton() {
    return this.page.getByRole('button', { name: /Call next|Calling/ });
  }

  showMapButton() {
    return this.page.getByRole('button', { name: /Show map|Hide map/ });
  }

  emptyState() {
    return this.page.getByText('No patients in the queue yet.');
  }

  waitingBadge() {
    return this.page.getByText(/\d+ waiting/);
  }

  entryCard(initials: string) {
    return this.page.getByText(initials).locator('..').locator('..');
  }

  entryPosition(position: number) {
    return this.page.getByText(`#${position}`);
  }

  doneButton() {
    return this.page.getByRole('button', { name: 'Done' }).first();
  }

  skipButton() {
    return this.page.getByRole('button', { name: 'Skip' }).first();
  }

  loadError() {
    return this.page.getByText('Could not load the queue');
  }
}
