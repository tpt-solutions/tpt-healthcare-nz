import { test, expect } from '../../fixtures/auth';
import { ClinicQueuePage } from '../../pages/ClinicQueuePage';

const MOCK_QUEUE_ID = 'queue-e2e-1';

const MOCK_ENTRIES = [
  {
    id: 'entry-1',
    patientInitials: 'AN',
    position: 1,
    status: 'waiting',
    checkedInAt: new Date(Date.now() - 5 * 60000).toISOString(),
    waitMinutes: 5,
  },
  {
    id: 'entry-2',
    patientInitials: 'TW',
    position: 2,
    status: 'called',
    checkedInAt: new Date(Date.now() - 12 * 60000).toISOString(),
    calledAt: new Date(Date.now() - 2 * 60000).toISOString(),
    waitMinutes: 12,
    roomHint: 'Room 3',
  },
];

test.describe('Queue page', () => {
  test.beforeEach(async ({ page, loginAsPractitioner }) => {
    // Single catch-all for all queue API endpoints
    await page.route('**/api/v1/queue**', async (route) => {
      const url = route.request().url();
      const method = route.request().method();

      if (url.endsWith('/queue') && method === 'POST') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ id: MOCK_QUEUE_ID }),
        });
      } else if (url.includes(`/queue/${MOCK_QUEUE_ID}/stream`)) {
        await route.fulfill({
          status: 200,
          contentType: 'text/event-stream',
          body: '',
        });
      } else if (url.includes('/call-next')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ok: true }),
        });
      } else if (url.includes('/entries/') && method === 'PATCH') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ok: true }),
        });
      } else if (url.includes(`/queue/${MOCK_QUEUE_ID}`) && method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ entries: MOCK_ENTRIES }),
        });
      } else {
        await route.fallback();
      }
    });

    await loginAsPractitioner(page);
    await page.getByRole('link', { name: 'Queue' }).click();
    await page.waitForURL('**/queue');
  });

  test('renders the queue heading', async ({ page }) => {
    const queue = new ClinicQueuePage(page);

    await expect(queue.heading()).toBeVisible();
  });

  test('displays queue entries with positions and statuses', async ({ page }) => {
    const queue = new ClinicQueuePage(page);

    await expect(queue.entryPosition(1)).toBeVisible();
    await expect(queue.entryPosition(2)).toBeVisible();
    await expect(page.getByText('AN', { exact: true })).toBeVisible();
    await expect(page.getByText('TW', { exact: true })).toBeVisible();
    await expect(page.getByText('Waiting').first()).toBeVisible();
    await expect(page.getByText('Called').first()).toBeVisible();
  });

  test('shows waiting count badge', async ({ page }) => {
    const queue = new ClinicQueuePage(page);

    await expect(queue.waitingBadge()).toBeVisible();
  });

  test('Call next button is enabled when there are waiting entries', async ({ page }) => {
    const queue = new ClinicQueuePage(page);

    await expect(queue.callNextButton()).toBeEnabled();
  });

  test('clicking Call next makes a POST request', async ({ page }) => {
    const queue = new ClinicQueuePage(page);
    let callNextHit = false;

    await page.route('**/call-next', async (route) => {
      callNextHit = true;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true }),
      });
    });

    await queue.callNextButton().click();
    await page.waitForTimeout(300);
    expect(callNextHit).toBe(true);
  });

  test('showing the map toggles map visibility', async ({ page }) => {
    const queue = new ClinicQueuePage(page);

    await queue.showMapButton().click();
    await expect(page.getByText('Patient locations')).toBeVisible();

    await page.getByRole('button', { name: 'Hide map' }).click();
    await expect(page.getByText('Patient locations')).not.toBeVisible();
  });

  test('Done button updates entry status', async ({ page }) => {
    const queue = new ClinicQueuePage(page);
    let patchHit = false;

    await page.route('**/entries/*', async (route) => {
      if (route.request().method() === 'PATCH') {
        patchHit = true;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ok: true }),
        });
      } else {
        await route.fallback();
      }
    });

    await queue.doneButton().click();
    await page.waitForTimeout(300);
    expect(patchHit).toBe(true);
  });
});
