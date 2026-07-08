import { test, expect } from '../../fixtures/admin-auth';
import { AdminOnboardingPage } from '../../pages/admin/AdminOperationsPage';

test.describe('Onboarding wizard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/onboarding');
  });

  test('renders the onboarding heading and step indicator', async ({ page }) => {
    const onboarding = new AdminOnboardingPage(page);

    await expect(onboarding.heading()).toBeVisible();
    await expect(onboarding.stepIndicator()).toBeVisible();
    await expect(onboarding.continueButton()).toBeVisible();
  });

  test('clicking continue advances to the next step', async ({ page }) => {
    const onboarding = new AdminOnboardingPage(page);

    await onboarding.continueButton().click();
    await page.waitForTimeout(500);
    await expect(page.getByText('Step 2 of 7')).toBeVisible();
    await expect(page.getByText('Departments')).toBeVisible();
  });

  test('clicking back returns to the previous step', async ({ page }) => {
    const onboarding = new AdminOnboardingPage(page);

    await onboarding.continueButton().click();
    await page.waitForTimeout(500);
    await onboarding.backButton().click();
    await expect(page.getByText('Step 1 of 7')).toBeVisible();
  });

  test('back button is disabled on step 1', async ({ page }) => {
    const onboarding = new AdminOnboardingPage(page);

    await expect(onboarding.backButton()).toBeDisabled();
  });
});
