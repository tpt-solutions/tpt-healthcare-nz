import { test, expect } from '../../fixtures/admin-auth';
import { AdminRoomsPage } from '../../pages/admin/AdminOperationsPage';

test.describe('Room bookings', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await page.route('**/api/v1/practice/rooms/bookings', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Rooms' }).click();
    await page.waitForURL('**/rooms');
  });

  test('renders the rooms heading and empty state', async ({ page }) => {
    const rooms = new AdminRoomsPage(page);

    await expect(rooms.heading()).toBeVisible();
    await expect(rooms.emptyState()).toBeVisible();
  });

  test('shows the book room button', async ({ page }) => {
    const rooms = new AdminRoomsPage(page);

    await expect(rooms.bookRoomButton()).toBeVisible();
  });
});
