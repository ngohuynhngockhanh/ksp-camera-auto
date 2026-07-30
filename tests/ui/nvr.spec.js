const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test('the tab lists NVRs with their linked cameras', async ({ page }) => {
  await openApp(page, {
    hash: 'cameras/nvr',
    overrides: {
      '/api/cameras': [
        { id: 'nvr-1', name: 'Đầu ghi tầng 1', host: '192.168.1.253', port: 37777, vendor: 'dahua', username: 'admin', password: 'x', isNvr: true },
        { id: 'cam-9', name: 'Hành lang', host: '192.168.1.9', port: 37777, vendor: 'dahua', username: 'admin', password: 'x', nvrId: 'nvr-1', nvrChannel: 4, noStorage: true },
      ],
    },
  });

  const rows = page.getByTestId('nvr-list').locator('tr');
  await expect(rows).toHaveCount(1);
  await expect(rows.first()).toContainText('Đầu ghi tầng 1');
  await expect(rows.first()).toContainText('Hành lang → K4');
});

test('each NVR row links straight to its storage page', async ({ page }) => {
  await openApp(page, { hash: 'cameras/nvr' });
  await page.getByTestId('nvr-open-maint').first().click();
  await expect(page.getByTestId('camera-detail')).toBeVisible();
  await expect(page.getByTestId('detail-tab-maint')).toHaveClass(/active/);
  await expect(page.getByTestId('detail-maint-body')).toContainText('Ổ cứng đầu ghi');
});

test('NVR list shows health controls and camera share links', async ({ page }) => {
  await openApp(page, { hash: 'cameras/nvr' });
  await expect(page.getByTestId('nvr-status').first()).toContainText('Tốt');
  await expect(page.getByTestId('nvr-watchdog').first()).toBeChecked();
  await expect(page.getByTestId('nvr-sync-time').first()).toBeChecked();
  await expect(page.getByTestId('nvr-camera-link').first()).toHaveAttribute('href', /#review\/cam-1/);
});

test('maintenance health shows uptime coverage and check-now action', async ({ page }) => {
  await openApp(page, { hash: 'cameras/nvr' });
  await page.getByTestId('nvr-open-maint').first().click();
  await expect(page.getByTestId('nvr-health-panel')).toContainText('uptime 120 phút');
  await expect(page.getByTestId('nvr-health-panel')).toContainText('coverage 98.3%');
  await page.getByTestId('nvr-check-now').click();
  await expect(page.getByTestId('nvr-health-panel')).toContainText('Ghi hình tốt');
});

test('the link dialog scans channels and suggests cameras', async ({ page }) => {
  await openApp(page, { hash: 'cameras/nvr' });
  await page.getByTestId('nvr-link-open').click();
  const dlg = page.locator('#nvr-link-dialog');
  await expect(dlg).toBeVisible();

  await page.locator('#nvr-host').fill('192.168.1.253');
  await page.locator('#nvr-port').fill('37777');
  await page.locator('#nvr-user').fill('admin');
  await page.locator('#nvr-pass').fill('secret');
  await page.locator('#nvr-scan-btn').click();

  const rows = page.locator('#nvr-tbody tr');
  await expect(rows).toHaveCount(2);
  await expect(rows.first().locator('.nvr-cam-sel')).toHaveValue('cam-1');
  await expect(rows.nth(1).locator('.nvr-nostore')).toBeChecked();
  await expect(page.locator('#nvr-save-btn')).toBeEnabled();
});

test('the empty state names the way out', async ({ page }) => {
  await openApp(page, { hash: 'cameras/nvr', overrides: { '/api/cameras': [] } });
  await expect(page.getByTestId('nvr-list')).toContainText('Chưa có đầu ghi nào');
});
