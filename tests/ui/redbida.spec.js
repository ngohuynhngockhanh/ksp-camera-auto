const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test('RedBida console groups keys and keeps protected values read-only', async ({ page }) => {
  await openApp(page, { hash: 'redbida' });
  await expect(page.getByRole('heading', { name: 'RedBida / OTA-MQTT' })).toBeVisible();
  await expect(page.locator('#redbida-key-count')).toHaveText('5');
  await expect(page.locator('[data-red-row="mqtt_password"] input')).toHaveCount(0);
  await expect(page.locator('[data-red-row="logo_header"]')).toContainText('Branding / Logo');
  await expect(page.locator('[data-red-row="show_toolbar"] select')).toHaveCount(1);
  await expect(page.locator('#redbida-ntp-status')).toHaveText('NTP tốt');
});

test('RedBida uses value metadata returned by refresh', async ({ page }) => {
  await openApp(page, {
    hash: 'redbida',
    overrides: {
      '/api/redbida/catalog': {
        keys: [{ key: 'show_toolbar', label: 'show toolbar', group: 'UI / Display', risk: 'editable', valueType: 'string', editable: true, secret: false }],
      },
      '/api/redbida/refresh': {
        values: [{ key: 'show_toolbar', value: true, exists: true, meta: { key: 'show_toolbar', label: 'show toolbar', group: 'UI / Display', risk: 'editable', valueType: 'boolean', editable: true, secret: false } }],
      },
    },
  });
  await expect(page.locator('[data-red-row="show_toolbar"] select')).toHaveCount(1);
});

test('RedBida search filters and logo upload submits through apply API', async ({ page }) => {
  let submitted;
  await openApp(page, {
    hash: 'redbida',
    overrides: {
      '/api/redbida/apply': async (route) => {
        submitted = JSON.parse(route.request().postData() || '{}');
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ results: [{ key: 'logo_header', applied: true, changed: true }] }) });
      },
    },
  });
  await page.locator('#redbida-search').fill('logo');
  await expect(page.locator('#redbida-tbody tr')).toHaveCount(2);
  await page.locator('[data-red-file="logo_header"]').setInputFiles({
    name: 'logo.png',
    mimeType: 'image/png',
    buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=', 'base64'),
  });
  await expect(page.locator('.redbida-logo-preview')).toBeVisible();
  await page.getByTestId('redbida-apply').click();
  await expect.poll(() => submitted && submitted.changes && submitted.changes.logo_header).toMatch(/^data:image\/png;base64,/);
});

test('RedBida keeps failed drafts and clears verified keys after a partial apply', async ({ page }) => {
  await openApp(page, {
    hash: 'redbida',
    overrides: {
      '/api/redbida/apply': {
        results: [
          { key: 'logo_header', applied: true, verified: true, changed: true, newValue: 'https://example.test/new.png' },
          { key: 'logo_livestream', applied: false, acknowledged: true, verified: false, readBack: true, changed: true, newValue: 'https://example.test/device.png', error: 'read-back mismatch' },
        ],
      },
    },
  });
  await page.locator('[data-red-row="logo_header"] [data-red-key="logo_header"]').fill('https://example.test/new.png');
  await page.locator('[data-red-row="logo_livestream"] [data-red-key="logo_livestream"]').fill('https://example.test/live.png');
  await page.getByTestId('redbida-apply').click();
  await expect(page.locator('[data-red-row="logo_header"]')).not.toHaveClass(/redbida-dirty/);
  await expect(page.locator('[data-red-row="logo_livestream"]')).toHaveClass(/redbida-dirty/);
  await expect(page.locator('[data-red-row="logo_livestream"] .redbida-row-status')).toContainText('read-back mismatch');
  await expect(page.locator('[data-red-row="logo_livestream"] .redbida-current')).toContainText('https://example.test/device.png');
});

test('RedBida invalid JSON cannot submit a stale valid draft', async ({ page }) => {
  let applyCalls = 0;
  await openApp(page, {
    hash: 'redbida',
    overrides: {
      '/api/redbida/catalog': {
        keys: [{ key: 'ui_tabs_links', label: 'ui tabs links', group: 'UI / Display', risk: 'editable', valueType: 'json', editable: true, secret: false }],
      },
      '/api/redbida/refresh': {
        values: [{ key: 'ui_tabs_links', value: { a: 1 }, exists: true, meta: { key: 'ui_tabs_links', label: 'ui tabs links', group: 'UI / Display', risk: 'editable', valueType: 'json', editable: true, secret: false } }],
      },
      '/api/redbida/apply': async (route) => {
        applyCalls += 1;
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ results: [] }) });
      },
    },
  });
  const editor = page.locator('[data-red-key="ui_tabs_links"]');
  await editor.fill('{"a":2}');
  await editor.fill('{"a":');
  await page.getByTestId('redbida-apply').click();
  await expect.poll(() => applyCalls).toBe(0);
  await expect(page.locator('[data-red-row="ui_tabs_links"] .redbida-row-status')).toContainText('JSON chưa hợp lệ');
});

test('RedBida renders data URL values as a compact descriptor', async ({ page }) => {
  const dataURL = 'data:image/png;base64,' + 'A'.repeat(128 * 1024);
  await openApp(page, {
    hash: 'redbida',
    overrides: {
      '/api/redbida/catalog': {
        keys: [{ key: 'logo_header', label: 'logo header', group: 'Branding / Logo', risk: 'editable', valueType: 'image', editable: true, secret: false }],
      },
      '/api/redbida/refresh': {
        values: [{ key: 'logo_header', value: dataURL, exists: true, meta: { key: 'logo_header', label: 'logo header', group: 'Branding / Logo', risk: 'editable', valueType: 'image', editable: true, secret: false } }],
      },
    },
  });
  const current = page.locator('[data-red-row="logo_header"] .redbida-current');
  await expect(current).toContainText('image/png');
  await expect.poll(async () => (await current.textContent()).length).toBeLessThan(200);
});

test('RedBida navigation stays hidden when the integration is disabled', async ({ page }) => {
  await openApp(page, {
    hash: 'redbida',
    overrides: {
      '/api/config': { role: 'admin', maxReviewHours: 72, redbidaEnabled: false },
    },
  });
  await expect(page).toHaveURL(/#dashboard$/);
  await expect(page.locator('[data-nav-hash="redbida"]')).toHaveCount(0);
});

test('RedBida does not present rejected input as the current value', async ({ page }) => {
  await openApp(page, {
    hash: 'redbida',
    overrides: {
      '/api/redbida/apply': {
        results: [{ key: 'logo_header', applied: false, verified: false, readBack: false, newValue: 'https://example.test/rejected.png', error: 'invalid value' }],
      },
    },
  });
  await page.locator('[data-red-row="logo_header"] [data-red-key="logo_header"]').fill('https://example.test/rejected.png');
  await page.getByTestId('redbida-apply').click();
  await expect(page.locator('[data-red-row="logo_header"] .redbida-current')).toContainText('https://example.test/logo.png');
});

test('RedBida suppresses duplicate submit clicks while apply is in flight', async ({ page }) => {
  let applyCalls = 0;
  await openApp(page, {
    hash: 'redbida',
    overrides: {
      '/api/redbida/apply': async (route) => {
        applyCalls += 1;
        await new Promise(resolve => setTimeout(resolve, 100));
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ results: [{ key: 'logo_header', applied: true, verified: true, readBack: true, newValue: 'https://example.test/new.png' }] }) });
      },
    },
  });
  await page.locator('[data-red-row="logo_header"] [data-red-key="logo_header"]').fill('https://example.test/new.png');
  await page.evaluate(() => {
    const button = document.getElementById('redbida-apply');
    button.click();
    button.click();
  });
  await expect.poll(() => applyCalls).toBe(1);
});
