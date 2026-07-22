const { test, expect } = require('@playwright/test');

async function openApp(page) {
  await page.route('**/api/config', route => route.fulfill({
    contentType: 'application/json',
    body: JSON.stringify({ role: 'admin', maxReviewHours: 72 }),
  }));
  await page.route('**/api/cameras', route => route.fulfill({
    contentType: 'application/json',
    body: '[]',
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
