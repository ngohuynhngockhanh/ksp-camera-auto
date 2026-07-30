const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test.beforeEach(async ({ page }) => { await openApp(page, { hash: 'review' }); });

test('source and playback controls are in two labelled groups', async ({ page }) => {
  await expect(page.getByTestId('review-source')).toBeVisible();
  await expect(page.getByTestId('review-source').locator('legend')).toHaveText('Nguồn');
  // Every playback control now has an accessible name.
  await expect(page.getByRole('button', { name: 'Lùi 10 giây' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Tiến 5 giây' })).toBeVisible();
});

test('the camera picker lists playback-capable devices', async ({ page }) => {
  const cam = page.getByTestId('review-camera');
  // Plain cameras plus one entry per NVR channel.
  await expect(cam.locator('option')).toHaveCount(4);
  await expect(cam.locator('option').first()).toContainText('Cổng chính');
  await expect(cam).toContainText('Đầu ghi tầng 1');
});

test('speed is a select showing the current rate', async ({ page }) => {
  const speed = page.getByTestId('review-speed');
  await expect(speed).toHaveValue('1');
  await speed.selectOption('2');
  await expect.poll(() => page.locator('#rv-video').evaluate(v => v.playbackRate)).toBe(2);
});

test('the QR download is a real dialog', async ({ page }) => {
  const qr = page.getByTestId('review-qr');
  await expect(qr).toBeHidden();
  // Pick a valid cut range so showQR gets past its guard.
  await page.evaluate(() => {
    document.getElementById('rv-cut-from').value = '2026-07-26T09:00:00';
    document.getElementById('rv-cut-to').value = '2026-07-26T09:01:00';
    document.getElementById('rv-cut-to').dispatchEvent(new Event('change'));
  });
  await page.locator('#rv-qr').click();
  await expect(qr).toBeVisible();
  await page.locator('#rv-qr-close').click();
  await expect(qr).toBeHidden();
});
