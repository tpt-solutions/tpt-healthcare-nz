import { test as base, expect, type Page, type Locator } from '@playwright/test';

interface VisualFixtures {
  /** Capture a screenshot with consistent settings for visual regression. */
  captureScreenshot: (
    page: Page,
    name: string,
    options?: { fullPage?: boolean; mask?: Locator[] },
  ) => Promise<void>;
  /** Disable CSS animations and transitions for deterministic snapshots. */
  disableAnimations: (page: Page) => Promise<void>;
  /** Wait for all fonts to finish loading. */
  waitForFonts: (page: Page) => Promise<void>;
}

export const test = base.extend<VisualFixtures>({
  captureScreenshot: async ({}, use) => {
    await use(async (page, name, options = {}) => {
      // Wait for network to settle and layout to stabilise.
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(100);

      const screenshotOptions: Record<string, unknown> = {
        fullPage: options.fullPage ?? false,
        animations: 'disabled',
      };
      if (options.mask) {
        screenshotOptions.mask = options.mask;
      }

      await expect(page).toHaveScreenshot(`${name}.png`, screenshotOptions);
    });
  },

  disableAnimations: async ({}, use) => {
    await use(async (page) => {
      await page.addStyleTag({
        content: `
          *, *::before, *::after {
            animation-duration: 0s !important;
            animation-delay: 0s !important;
            transition-duration: 0s !important;
            transition-delay: 0s !important;
          }
        `,
      });
    });
  },

  waitForFonts: async ({}, use) => {
    await use(async (page) => {
      await page.evaluate(() => document.fonts.ready);
    });
  },
});

export { expect } from '@playwright/test';
