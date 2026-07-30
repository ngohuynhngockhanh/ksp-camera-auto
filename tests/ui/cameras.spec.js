const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test.beforeEach(async ({ page }) => { await openApp(page, { hash: 'cameras/list' }); });

test('search and vendor filter narrow the table', async ({ page }) => {
  const rows = page.getByTestId('camera-row');
  await expect(rows).toHaveCount(3);

  await page.getByTestId('camera-search').fill('cổng');
  await expect(rows).toHaveCount(1);

  await page.getByTestId('camera-search').fill('');
  await page.getByTestId('camera-vendor-filter').selectOption('hikvision');
  await expect(rows).toHaveCount(1);
  await expect(rows.first()).toContainText('Kho hàng');
});

// Sortable headers only exist on the desktop table; .reflow drops the thead
// under 767px, so there is nothing to click on mobile.
test('sorting by host reorders the table', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop');
  await page.locator('#cam-table th[data-sort="host"]').click();
  const first = page.getByTestId('camera-row').first();
  await expect(first).toContainText('192.168.1.10');
  await page.locator('#cam-table th[data-sort="host"]').click();
  await expect(page.getByTestId('camera-row').first()).toContainText('192.168.1.253');
});

test('clicking a row opens that camera detail page', async ({ page }) => {
  await page.getByTestId('camera-row').first().click();
  await expect(page.getByTestId('camera-detail')).toBeVisible();
  await expect(page.locator('#detail-name')).toHaveText('Cổng chính');
  // The task tabs and page heading step aside in detail mode.
  await expect(page.locator('#camera-task-tabs')).toBeHidden();
  expect(decodeURIComponent(new URL(page.url()).hash)).toBe('#cameras/cam/cam-1/osd');

  await page.getByTestId('detail-back').click();
  await expect(page.getByTestId('camera-detail')).toBeHidden();
  await expect(page.locator('#camera-task-tabs')).toBeVisible();
});

test('the row checkbox selects without opening the detail page', async ({ page }) => {
  await page.locator('.cam-cb').first().check();
  await expect(page.getByTestId('camera-detail')).toBeHidden();
  await expect(page.locator('#bulk-selected-count')).toContainText('1 camera');
});

test('add dialog posts a new camera', async ({ page }) => {
  const posted = [];
  await page.route('**/api/cameras', route => {
    if (route.request().method() === 'POST') {
      posted.push(JSON.parse(route.request().postData()));
      return route.fulfill({ contentType: 'application/json', body: '{}' });
    }
    return route.fallback();
  });

  await expect(page.locator('#camera-form-dialog')).toBeHidden();
  await page.getByTestId('camera-add-open').click();
  await expect(page.locator('#camera-form-dialog')).toBeVisible();
  await expect(page.locator('#camera-form-title')).toHaveText('Thêm camera');

  await page.getByTestId('form-name').fill('Sân sau');
  await page.getByTestId('form-host').fill('192.168.1.99');
  await page.getByTestId('form-port').fill('37777');
  await page.getByTestId('form-submit').click();

  await expect(page.locator('#camera-form-dialog')).toBeHidden();
  expect(posted).toHaveLength(1);
  expect(posted[0]).toMatchObject({ name: 'Sân sau', host: '192.168.1.99', port: 37777 });
});

test('edit mode locks the address until it is deliberately unlocked', async ({ page }) => {
  await page.getByTestId('camera-row').first().locator('.row-menu > summary').click();
  await page.getByRole('button', { name: 'Sửa thông tin kho' }).click();

  await expect(page.locator('#camera-form-title')).toHaveText('Sửa "Cổng chính"');
  await expect(page.getByTestId('form-host')).toBeDisabled();
  await expect(page.getByTestId('form-port')).toBeDisabled();
  await expect(page.getByTestId('form-host')).toHaveValue('192.168.1.10');

  await page.getByTestId('form-unlock').click();
  await expect(page.getByTestId('form-host')).toBeEnabled();
  await expect(page.locator('#add-msg')).toContainText('tạo một camera mới');
});

test('inline rename saves the inventory label only', async ({ page }) => {
  const posted = [];
  await page.route('**/api/cameras', route => {
    if (route.request().method() === 'POST') {
      posted.push(JSON.parse(route.request().postData()));
      return route.fulfill({ contentType: 'application/json', body: '{}' });
    }
    return route.fallback();
  });

  await page.getByTestId('camera-row').first()
    .locator('button[data-action="rename-inline"]').click();
  const input = page.locator('.cell-name input');
  await input.fill('Cổng trước');
  await input.press('Enter');

  await expect.poll(() => posted.length).toBe(1);
  expect(posted[0]).toMatchObject({ id: 'cam-1', name: 'Cổng trước' });
});

test('keeps detected Dahua ports exact and renders a QR for a probed serial number', async ({ page }) => {
  const dahuaCameras = [
    {
      id: 'dahua-37777', name: 'Dahua chuẩn', host: '192.168.1.20', port: 37777,
      vendor: 'dahua', username: 'admin', password: 'one',
    },
    {
      id: 'dahua-8888', name: 'KBVision 8888', host: '192.168.1.21', port: 8888,
      vendor: 'dahua', username: 'admin', password: 'two',
    },
  ];
  await page.route('**/api/cameras', route => route.fulfill({
    contentType: 'application/json', body: JSON.stringify(dahuaCameras),
  }));
  await page.route('**/api/probe', route => route.fulfill({
    contentType: 'application/json', body: JSON.stringify({
      streams: [{ channel: 1, stream: 0, width: 1920, height: 1080, compression: 'H.264' }],
      serialNumber: '8K01234PAZ56789',
      port: 37777,
    }),
  }));
  await page.reload();
  await page.waitForFunction(() => window.__kspReady === true);

  const rows = page.getByTestId('camera-row');
  const standardRow = rows.filter({ hasText: 'Dahua chuẩn' });
  const kbvisionRow = rows.filter({ hasText: 'KBVision 8888' });
  await expect(standardRow.locator('[data-label="Cổng"]')).toHaveText('37777');
  await expect(kbvisionRow.locator('[data-label="Cổng"]')).toHaveText('8888');

  await standardRow.locator('.row-menu > summary').click();
  await standardRow.getByRole('button', { name: 'Dò cấu hình' }).click();

  const refreshed = page.getByTestId('camera-row').filter({ hasText: 'Dahua chuẩn' });
  await expect(refreshed.locator('[data-label="Cổng"]')).toHaveText('37777');
  await expect(refreshed.locator('[data-label="SN / QR"]')).toContainText('8K01234PAZ56789');
  await expect(refreshed.getByTestId('camera-serial-qr').locator('canvas:visible, img:visible')).toHaveCount(1);
});
