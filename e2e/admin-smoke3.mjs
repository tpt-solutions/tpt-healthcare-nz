import { chromium } from '@playwright/test';
const browser = await chromium.launch();
const context = await browser.newContext({ serviceWorkers: 'block' });
const page = await context.newPage();
page.on('pageerror', e => console.log('PAGEERROR:', e.message));
await page.goto('http://localhost:4176/login');
await page.waitForTimeout(1500);
console.log('BODY TEXT:', (await page.locator('body').innerText()).slice(0, 300));
await browser.close();
