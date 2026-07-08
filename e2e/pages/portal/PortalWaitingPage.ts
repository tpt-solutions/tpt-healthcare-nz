import type { Page } from '@playwright/test';

export class PortalWaitingPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Check in' });
  }

  nhiInput() {
    return this.page.locator('input[type="text"]').first();
  }

  checkInButton() {
    return this.page.getByRole('button', { name: /Check in/ });
  }

  queuePosition() {
    return this.page.getByText(/#\d+/);
  }

  estimatedWait() {
    return this.page.getByText(/min wait/);
  }

  shareLocationToggle() {
    return this.page.getByRole('switch', { name: /Share your location/ });
  }

  calledHeading() {
    return this.page.getByRole('heading', { name: 'Your turn!' });
  }

  doneButton() {
    return this.page.getByRole('button', { name: 'Done' });
  }
}
