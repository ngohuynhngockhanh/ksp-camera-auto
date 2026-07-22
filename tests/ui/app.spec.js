const { test, expect } = require('@playwright/test');

const cameraFixtures = [
  { id: 'cam-1', name: 'Cổng chính', host: '192.168.1.10', port: 37777, vendor: 'dahua', username: 'admin', password: 'secret-one' },
  { id: 'cam-2', name: 'Kho hàng', host: '192.168.1.11', port: 80, vendor: 'hikvision', username: 'operator', password: 'secret-two' },
];

async function openApp(page, cameras = []) {
  await page.route('**/api/config', route => route.fulfill({
    contentType: 'application/json',
    body: JSON.stringify({ role: 'admin', maxReviewHours: 72 }),
  }));
  await page.route('**/api/cameras', route => route.fulfill({
    contentType: 'application/json',
    body: JSON.stringify(cameras),
  }));
  await page.goto('/index.html');
}

test('desktop navigation and theme are usable', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop');
  await openApp(page);

  await expect(page.locator('#view-dashboard')).toBeVisible();
  await page.getByRole('link', { name: 'Kho camera' }).click();
  await expect(page.locator('#view-cameras')).toBeVisible();

  await page.locator('#theme-toggle').click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', /dark|light/);
});

test('camera workspace keeps selection across task tabs', async ({ page }) => {
  await openApp(page, cameraFixtures);
  await page.goto('/index.html#cameras/list');

  await expect(page.locator('[data-camera-panel="list"]:visible')).toHaveCount(1);
  await expect(page.locator('.camera-add-card')).toBeHidden();
  await page.locator('#camera-add-open').click();
  await expect(page.locator('.camera-add-card')).toBeVisible();

  await page.locator('#camera-search').fill('cổng');
  await expect(page.locator('#cam-tbody tr')).toHaveCount(1);
  await page.locator('.cam-cb').check();
  await page.getByRole('link', { name: 'Chỉnh hàng loạt' }).click();
  await expect(page.locator('#bulk-selected-count')).toContainText('1 camera');
  await expect(page.locator('.bulk-cam-cb[value="cam-1"]')).toBeChecked();

  await page.getByRole('link', { name: 'Kết quả' }).click();
  await expect(page.locator('[data-camera-panel="results"]')).toBeVisible();
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
