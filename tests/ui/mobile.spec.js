const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test.beforeEach(async ({}, testInfo) => { test.skip(testInfo.project.name !== 'mobile'); });

test('the camera table reflows into cards', async ({ page }) => {
  await openApp(page, { hash: 'cameras/list' });
  const row = page.getByTestId('camera-row').first();
  await expect(row).toBeVisible();
  // .reflow stacks cells and labels them from data-label under 767px.
  const display = await row.evaluate(el => getComputedStyle(el).display);
  expect(display).toBe('block');
});

test('the detail page stacks preview above the tabs', async ({ page }) => {
  await openApp(page, { hash: 'cameras/cam/cam-1/osd' });
  await expect(page.getByTestId('camera-detail')).toBeVisible();
  const dir = await page.locator('.detail-layout').evaluate(el => getComputedStyle(el).flexDirection);
  expect(dir).toBe('column');
});

test('tapping a row still opens the detail page', async ({ page }) => {
  await openApp(page, { hash: 'cameras/list' });
  await page.getByTestId('camera-row').first().locator('td').nth(1).click();
  await expect(page.getByTestId('camera-detail')).toBeVisible();
});

test('the settings popover is reachable on a narrow screen', async ({ page }) => {
  await openApp(page);
  await page.getByTestId('settings-toggle').click();
  await expect(page.getByTestId('settings-popover')).toBeVisible();
  await expect(page.getByTestId('settings-timeout')).toBeVisible();
});
