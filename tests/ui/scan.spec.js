const { test, expect } = require('@playwright/test');
const { openApp, ndjson } = require('./fixtures');

test('normalizes a Lechange/LC discovery row for add and password testing', async ({ page }) => {
  const posted = [];
  await openApp(page, {
    hash: 'scan',
    overrides: {
      '/api/scan': [{ ip: '192.168.1.44', vendor: 'LC', model: 'Lechange IPC', port: 0, via: 'onvif' }],
      '/api/scan/try-password': (route) => {
        posted.push(JSON.parse(route.request().postData() || '{}'));
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
  await page.locator('#scan-try-pass').fill('wrong-password');
  await page.locator('#scan-try-btn').click();
  await expect(page.locator('#scan-status-0')).toContainText('probe: authentication failed');
  expect(posted[0].targets[0]).toMatchObject({ ip: '192.168.1.44', vendor: 'dahua', port: 37777 });

  await row.getByRole('button', { name: 'Thêm vào kho' }).click();
  await expect(page.locator('#f-host')).toHaveValue('192.168.1.44');
  await expect(page.locator('#f-port')).toHaveValue('37777');
  await expect(page.locator('#f-vendor')).toHaveValue('dahua');
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
