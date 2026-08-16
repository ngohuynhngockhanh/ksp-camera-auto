const { test, expect } = require('@playwright/test');
const { openApp, ndjson } = require('./fixtures');

test('normalizes a Lechange/LC discovery row for save and password testing', async ({ page }) => {
  const posted = [];
  await openApp(page, {
    hash: 'scan',
    overrides: {
      '/api/scan': [{ ip: '192.168.1.44', vendor: 'LC', model: 'Lechange IPC', port: 0, via: 'onvif' }],
      '/api/cameras': (route) => {
        if (route.request().method() === 'POST') {
          posted.push(JSON.parse(route.request().postData() || '{}'));
          return route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ id: '192.168.1.44:37777', host: '192.168.1.44', port: 37777, vendor: 'dahua' }),
          });
        }
        return route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
      },
      '/api/scan/try-password': (route) => {
        return route.fulfill({
          status: 200,
          contentType: 'text/event-stream',
          body: ndjson([
            { type: 'start', index: 1, total: 1, ip: '192.168.1.44' },
            { type: 'result', index: 1, total: 1, ip: '192.168.1.44', ok: false, err: 'probe: authentication failed' },
            { type: 'done', total: 1 },
          ]),
        });
      },
    },
  });

  await page.getByRole('button', { name: 'Quét LAN (ONVIF/Dahua/Hik)' }).click();
  const row = page.locator('#scan-tbody tr').first();
  await expect(row.locator('[data-label="Hãng"]')).toHaveText('dahua');
  await expect(row.locator('[data-label="Cổng"]')).toHaveText('37777');

  await row.locator('.scan-cb').check();
  await page.locator('#scan-try-pass').fill('wrong-pass');
  await page.locator('#scan-try-btn').click();
  await expect(page.locator('#scan-status-0')).toContainText('probe: authentication failed');
  expect(posted[0]).toBeUndefined();

  await row.getByRole('button', { name: 'Thêm vào kho' }).click();
  await expect(page).toHaveURL(/#scan$/);
  await expect(page.locator('#scan-status-0')).toContainText('Đã lưu');
  expect(posted[0]).toMatchObject({
    name: 'Lechange IPC', host: '192.168.1.44', port: 37777,
    vendor: 'dahua', username: 'admin', password: 'wrong-pass',
  });
});

test('keeps the scan page and exposes add-to-inventory errors', async ({ page }) => {
  await openApp(page, {
    hash: 'scan',
    overrides: {
      '/api/scan': [{ ip: '192.168.1.46', vendor: 'dahua', model: 'IPC-ERR', port: 37777 }],
      '/api/cameras': (route) => {
        if (route.request().method() === 'POST') {
          return route.fulfill({
            status: 500,
            contentType: 'application/json',
            body: JSON.stringify({ error: 'inventory unavailable' }),
          });
        }
        return route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
      },
    },
  });

  await page.getByRole('button', { name: 'Quét LAN (ONVIF/Dahua/Hik)' }).click();
  const row = page.locator('#scan-tbody tr').first();
  await row.getByRole('button', { name: 'Thêm vào kho' }).click();
  await expect(page).toHaveURL(/#scan$/);
  await expect(page.locator('#scan-status-0')).toContainText('inventory unavailable');
  await expect(row.getByRole('button', { name: 'Thêm vào kho' })).toBeEnabled();
});

test('accepts the object-shaped scan response used by older fixtures', async ({ page }) => {
  await openApp(page, {
    hash: 'scan',
    overrides: {
      '/api/scan': { devices: [{ ip: '192.168.1.45', vendor: 'dahua', port: 0 }] },
    },
  });

  await page.getByRole('button', { name: 'Quét LAN (ONVIF/Dahua/Hik)' }).click();
  await expect(page.locator('#scan-tbody tr')).toHaveCount(1);
  await expect(page.locator('#scan-tbody [data-label="Cổng"]')).toHaveText('37777');
});

test('normalizes the generic Lechange ONVIF identity used by Dahua OEMs', async ({ page }) => {
  await openApp(page, {
    hash: 'scan',
    overrides: {
      '/api/scan': [{ ip: '192.168.1.211', model: 'IP_Camera', name: 'General', via: 'onvif' }],
    },
  });

  await page.getByRole('button', { name: 'Quét LAN (ONVIF/Dahua/Hik)' }).click();
  const row = page.locator('#scan-tbody tr').first();
  await expect(row.locator('[data-label="Hãng"]')).toHaveText('dahua');
  await expect(row.locator('[data-label="Cổng"]')).toHaveText('37777');
});
