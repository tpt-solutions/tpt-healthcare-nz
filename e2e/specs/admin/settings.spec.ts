import { test, expect } from '../../fixtures/admin-auth';
import { AdminSettingsPage } from '../../pages/admin/AdminSettingsPage';

test.describe('Settings page', () => {
  test.beforeEach(async ({ page, loginAsAdmin }) => {
    await loginAsAdmin(page);
    await page.getByRole('link', { name: 'Settings' }).click();
    await page.waitForURL('**/settings');
  });

  test('renders the settings heading and save button', async ({ page }) => {
    const settings = new AdminSettingsPage(page);

    await expect(settings.heading()).toBeVisible();
    await expect(settings.saveButton()).toBeVisible();
  });

  test('shows practice identity fields with defaults', async ({ page }) => {
    // Labels in this page aren't associated via for/id, so use the input's
    // current value to locate it — the field is pre-filled with stub data.
    const nameInput = page.locator('input').filter({ hasText: '' }).nth(0);
    await expect(nameInput).toHaveValue('Auckland City Medical Centre');

    const hpiInput = page.locator('input.font-mono').first();
    await expect(hpiInput).toHaveValue('F0K068-C');
  });

  test('save button shows confirmation after click', async ({ page }) => {
    const settings = new AdminSettingsPage(page);

    await settings.saveButton().click();
    await expect(settings.savedConfirmation()).toBeVisible();
  });

  test('shows scheduling settings section', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Scheduling' })).toBeVisible();
    // The appointment duration input is a number input with value 15
    const durationInput = page.locator('input[type="number"]').first();
    await expect(durationInput).toHaveValue('15');
  });

  test('shows device security section with auto-lock options', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Device Security' })).toBeVisible();
    await expect(page.getByText('Auto-lock after inactivity')).toBeVisible();
  });

  test('shows appearance/theme section', async ({ page }) => {
    const settings = new AdminSettingsPage(page);

    await expect(settings.appearanceSection()).toBeVisible();
    await expect(settings.themeButtons().first()).toBeVisible();
  });

  test('shows feature flags section', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Feature Flags' })).toBeVisible();
    await expect(page.getByText('Telehealth appointments')).toBeVisible();
    await expect(page.getByText('ACC claiming')).toBeVisible();
    await expect(page.getByText('HIPC consent gate')).toBeVisible();
  });
});
