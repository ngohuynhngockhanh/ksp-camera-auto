const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

// Resource 404s are expected: the static test server has no /logo.png poster
// and no generated help bundle. Only script errors should fail a spec.
function collectScriptErrors(page) {
  const errors = [];
  page.on('pageerror', e => errors.push(String(e)));
  page.on('console', m => {
    if (m.type() !== 'error') return;
    if (/Failed to load resource/.test(m.text())) return;
    errors.push(m.text());
  });
  return errors;
}

test('boots clean and routes between top-level views', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop');
  const errors = collectScriptErrors(page);
  await openApp(page);

  await expect(page.locator('#view-dashboard')).toBeVisible();
  await page.getByRole('link', { name: 'Kho camera' }).click();
  await expect(page.locator('#view-cameras')).toBeVisible();
  await page.getByRole('link', { name: 'Quét mạng' }).click();
  await expect(page.locator('#view-scan')).toBeVisible();

  expect(errors).toEqual([]);
});

test('theme toggle writes the root theme attribute', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop');
  await openApp(page);
  await page.locator('#theme-toggle').click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', /dark|light/);
});

test('legacy camera hashes still resolve', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop');
  await openApp(page);

  // #cameras/devices was split; the NVR half kept the tab.
  await page.evaluate(() => { location.hash = '#cameras/devices'; });
  await expect(page.getByTestId('task-tab-nvr')).toHaveClass(/active/);

  // #bulk / #results predate the merged camera workspace.
  await page.evaluate(() => { location.hash = '#bulk'; });
  await expect(page.getByTestId('task-tab-bulk')).toHaveClass(/active/);
});

test('nvr-storage deep link opens the NVR maintenance tab after load', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop');
  await openApp(page);
  // The bug this covers: the shortcut used to be resolved only during boot, so
  // navigating to it in an already-loaded tab silently did nothing.
  await page.evaluate(() => { location.hash = '#cameras/devices/nvr-storage'; });
  await expect(page.getByTestId('camera-detail')).toBeVisible();
  await expect(page.locator('#detail-name')).toHaveText('Đầu ghi tầng 1');
  await expect(page.getByTestId('detail-tab-maint')).toHaveClass(/active/);
});

test('global timeout lives in the topbar and persists', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop');
  await openApp(page);
  await expect(page.getByTestId('settings-popover')).toBeHidden();
  await page.getByTestId('settings-toggle').click();
  await expect(page.getByTestId('settings-popover')).toBeVisible();
  await page.getByTestId('settings-timeout').fill('45');
  await page.getByTestId('settings-timeout').blur();
  expect(await page.evaluate(() => localStorage.getItem('kspcam-timeout'))).toBe('45');
});

test('mobile bottom navigation and drawer expose every view', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'mobile');
  await openApp(page);

  await page.locator('#bottomnav').getByText('Camera', { exact: true }).click();
  await expect(page.locator('#view-cameras')).toBeVisible();
  await page.getByRole('button', { name: 'Menu' }).click();
  await expect(page.locator('#drawer')).toHaveClass(/open/);
  await expect(page.locator('#drawer')).toContainText('Nhập Shinobi');
  await expect(page.locator('#drawer')).toContainText('Trợ giúp');
});
