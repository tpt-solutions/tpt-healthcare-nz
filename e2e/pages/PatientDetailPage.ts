import type { Page } from '@playwright/test';
import { PatientListPage } from './PatientListPage';

export class PatientDetailPage {
  constructor(private readonly page: Page) {}

  /** Navigates via the patient list's "View" link rather than page.goto() —
   * the app keeps its JWT in memory only, so a full page navigation would de-auth. */
  async gotoFromList(patientName: string) {
    const list = new PatientListPage(this.page);
    await list.goto();
    await list.viewLinkForPatient(patientName).click();
    await this.page.waitForURL('**/patients/*');
  }

  tab(label: string) {
    return this.page.getByRole('button', { name: label });
  }

  /** Scoped to <h2> — the AppShell page title is also an (identically named) <h1>. */
  bannerHeading(name: string) {
    return this.page.getByRole('heading', { name, level: 2 });
  }
}
