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
    const settings = new AdminSettingsPage(page);

    await expect(settings.practiceNameInput()).toHaveValue('Auckland City Medical Centre');
    await expect(settings.hpiFacilityIdInput()).toHaveValue('F0K068-C');
  });

  test('save button shows confirmation after click', async ({ page }) => {
    const settings = new AdminSettingsPage(page);

    await settings.saveButton().click();
    await expect(settings.savedConfirmation()).toBeVisible();
  });

  test('shows scheduling settings section', async ({ page }) => {
    await expect(page.getByText('Scheduling')).toBeVisible();
    await expect(page.getByLabel('Default Appointment Duration (minutes)')).toHaveValue('15');
  });

  test('shows device security section with auto-lock options', async ({ page }) => {
    const settings = new AdminSettingsPage(page);

    await expect(page.getByText('Device Security')).toBeVisible();
    await expect(settings.autoLockSelect()).toBeVisible();
  });

  test('shows appearance/theme section', async ({ page }) => {
    const settings = new AdminSettingsPage(page);

    await expect(settings.appearanceSection()).toBeVisible();
    await expect(settings.themeButtons().first()).toBeVisible();
  });

  test('shows feature flags section', async ({ page }) => {
    const settings = new AdminSettingsPage(page);

    await expect(page.getByText('Feature Flags')).toBeVisible();
    await expect(settings.telehealthToggle()).toBeVisible();
    await expect(settings.accClaimingToggle()).toBeVisible();
    await expect(settings.hipcConsentToggle()).toBeVisible();
  });
});
