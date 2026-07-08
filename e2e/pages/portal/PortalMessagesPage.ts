import type { Page } from '@playwright/test';

export class PortalMessagesPage {
  constructor(private readonly page: Page) {}

  heading() {
    return this.page.getByRole('heading', { name: 'Secure Messages' });
  }

  unreadBadge() {
    return this.page.getByText(/\d+ new/);
  }

  encryptionNotice() {
    return this.page.getByText('All messages are end-to-end encrypted');
  }

  newMessageButton() {
    return this.page.getByRole('button', { name: 'New Message' });
  }

  messageListItem(subject: string) {
    return this.page.getByText(subject).locator('..');
  }

  messageBody(subject: string) {
    return this.page.getByRole('heading', { name: subject }).locator('..');
  }

  composeModal() {
    return this.page.getByText('New Message').last();
  }

  composeSubjectInput() {
    return this.page.getByPlaceholder('Message subject...');
  }

  composeBodyInput() {
    return this.page.getByPlaceholder('Write your message here...');
  }

  sendButton() {
    return this.page.getByRole('button', { name: 'Send' });
  }

  composeCancelButton() {
    return this.page.getByRole('button', { name: 'Cancel' });
  }
}
