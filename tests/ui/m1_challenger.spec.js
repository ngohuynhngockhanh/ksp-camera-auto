const { test, expect } = require('@playwright/test');
const { openApp, CAMERAS } = require('./fixtures');

test.describe('Milestone 1 Challenger Stress Tests', () => {

  test.beforeEach(async ({ page }) => {
    await openApp(page, { hash: 'cameras/list' });
  });

  /* --------------------------------------------------------------------------
   * 1. View Switcher & Card Grid Stress Tests
   * -------------------------------------------------------------------------- */
  test('View Switcher toggles views and persists state across page reloads', async ({ page }) => {
    const tableBtn = page.locator('#cam-view-table-btn');
    const gridBtn = page.locator('#cam-view-grid-btn');
    const tableWrap = page.locator('#cam-table-wrap');
    const grid = page.locator('#cam-grid');

    // Initial state: Table active
    await expect(tableBtn).toHaveClass(/active/);
    await expect(tableWrap).toBeVisible();
    await expect(grid).toBeHidden();

    // Switch to Grid
    await gridBtn.click();
    await expect(gridBtn).toHaveClass(/active/);
    await expect(tableBtn).not.toHaveClass(/active/);
    await expect(grid).toBeVisible();
    await expect(tableWrap).toBeHidden();
    await expect(page.getByTestId('camera-card')).toHaveCount(3);

    // Reload page with grid view stored in localStorage
    await page.reload();
    await page.waitForFunction(() => window.__kspReady === true);

    await expect(gridBtn).toHaveClass(/active/);
    await expect(grid).toBeVisible();
    await expect(tableWrap).toBeHidden();

    // Switch back to Table
    await tableBtn.click();
    await expect(tableBtn).toHaveClass(/active/);
    await expect(tableWrap).toBeVisible();
    await expect(grid).toBeHidden();
  });

  test('Checkbox synchronization between Table rows and Grid cards', async ({ page }) => {
    const tableCbFirst = page.locator('.cam-cb').first();
    const cardFirst = page.locator('.cam-card[data-id="cam-1"]');
    const cardCbFirst = cardFirst.locator('.cam-card-cb');

    // Check table checkbox
    await tableCbFirst.check();
    await expect(tableCbFirst).toBeChecked();
    await expect(cardCbFirst).toBeChecked();
    await expect(cardFirst).toHaveClass(/selected/);
    await expect(page.locator('#bulk-selected-count')).toContainText('1 camera');

    // Uncheck from Grid card
    await page.locator('#cam-view-grid-btn').click();
    await cardCbFirst.uncheck();
    await expect(cardCbFirst).not.toBeChecked();
    await expect(cardFirst).not.toHaveClass(/selected/);
    await expect(tableCbFirst).not.toBeChecked();
    await expect(page.locator('#bulk-selected-count')).toContainText('0 camera');

    // Check second card in Grid view
    const card2 = page.locator('.cam-card[data-id="cam-2"]');
    await card2.locator('.cam-card-cb').check();
    await expect(card2).toHaveClass(/selected/);
    await expect(page.locator('.cam-cb[value="cam-2"]')).toBeChecked();
    await expect(page.locator('#bulk-selected-count')).toContainText('1 camera');
  });

  test('Select All checkbox synchronization across Table and Grid views', async ({ page }) => {
    const selectAll = page.locator('#select-all');

    // Check Select All
    await selectAll.check();
    await expect(page.locator('.cam-cb')).toHaveCount(3);
    for (const cb of await page.locator('.cam-cb').all()) {
      await expect(cb).toBeChecked();
    }
    await expect(page.locator('#bulk-selected-count')).toContainText('3 camera');

    // Switch to Grid view and verify all cards have .cam-card-cb checked and .selected class
    await page.locator('#cam-view-grid-btn').click();
    const cards = page.locator('.cam-card');
    await expect(cards).toHaveCount(3);
    for (const card of await cards.all()) {
      await expect(card.locator('.cam-card-cb')).toBeChecked();
      await expect(card).toHaveClass(/selected/);
    }

    // Switch back to Table view and uncheck Select All
    await page.locator('#cam-view-table-btn').click();
    await selectAll.uncheck();
    for (const cb of await page.locator('.cam-cb').all()) {
      await expect(cb).not.toBeChecked();
    }

    // Verify grid cards updated
    for (const card of await page.locator('.cam-card').all()) {
      await expect(card.locator('.cam-card-cb')).not.toBeChecked();
      await expect(card).not.toHaveClass(/selected/);
    }
  });

  test('Search and filter across Table and Grid views with empty match handling', async ({ page }) => {
    const searchInput = page.getByTestId('camera-search');
    const vendorFilter = page.getByTestId('camera-vendor-filter');
    const rows = page.getByTestId('camera-row');
    const cards = page.getByTestId('camera-card');

    // Filter by name "Kho"
    await searchInput.fill('Kho');
    await expect(rows).toHaveCount(1);
    await expect(cards).toHaveCount(1);
    await expect(rows.first()).toContainText('Kho hàng');
    await expect(cards.first()).toContainText('Kho hàng');

    // Filter by vendor "dahua"
    await searchInput.fill('');
    await vendorFilter.selectOption('dahua');
    await expect(rows).toHaveCount(2); // cam-1 and nvr-1
    await expect(cards).toHaveCount(2);

    // Search query matching nothing
    await searchInput.fill('NonexistentCameraNameXYZ');
    await expect(rows).toHaveCount(0);
    await expect(cards).toHaveCount(0);
    await expect(page.locator('#cam-tbody .empty-hint')).toContainText('Không có camera khớp bộ lọc.');
    await expect(page.locator('#cam-grid .empty-hint')).toContainText('Không có camera khớp bộ lọc.');
  });

  test('Empty inventory gracefully displays empty hint on Table and Grid without errors', async ({ page }) => {
    await page.route('**/api/cameras', route => {
      if (route.request().method() === 'GET') {
        return route.fulfill({ contentType: 'application/json', body: '[]' });
      }
      return route.fallback();
    });

    await page.reload();
    await page.waitForFunction(() => window.__kspReady === true);

    await expect(page.locator('#cam-tbody .empty-hint')).toContainText('Chưa có camera nào');
    await expect(page.locator('#cam-grid .empty-hint')).toContainText('Chưa có camera nào');

    // Switch between views on empty inventory without errors
    await page.locator('#cam-view-grid-btn').click();
    await expect(page.locator('#cam-grid .empty-hint')).toBeVisible();
    await page.locator('#cam-view-table-btn').click();
    await expect(page.locator('#cam-tbody .empty-hint')).toBeVisible();
  });

  /* --------------------------------------------------------------------------
   * 2. Quick Actions Toolbar Tests
   * -------------------------------------------------------------------------- */
  test('Table quick actions trigger modals and actions correctly', async ({ page }) => {
    const firstRow = page.getByTestId('camera-row').first();

    // 1. Quick PTZ modal
    await firstRow.locator('[data-action="quick-ptz"]').click();
    const ptzDlg = page.locator('#quick-ptz-dialog');
    await expect(ptzDlg).toBeVisible();
    await expect(page.locator('#quick-ptz-title')).toContainText('Cổng chính');
    await ptzDlg.locator('#quick-ptz-close').click();
    await expect(ptzDlg).toBeHidden();

    // 2. Quick Snapshot Lightbox
    await firstRow.locator('[data-action="quick-snap"]').click();
    const lbDlg = page.locator('#lightbox-dialog');
    await expect(lbDlg).toBeVisible();
    await expect(page.locator('#lightbox-label')).toContainText('Snapshot: Cổng chính');
    await lbDlg.locator('#lightbox-close').click();
    await expect(lbDlg).toBeHidden();

    // 3. Quick Reboot triggers confirmation dialog
    await firstRow.locator('[data-action="quick-reboot"]').click();
    const confirmDlg = page.locator('#confirm-dialog');
    await expect(confirmDlg).toBeVisible();
    await expect(confirmDlg).toContainText('Khởi động lại');
    await page.locator('#confirm-cancel').click();
    await expect(confirmDlg).toBeHidden();

    // 4. Quick Sync Time posts to /api/device-time and shows toast
    let timeSynced = false;
    await page.route('**/api/device-time', route => {
      timeSynced = true;
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ ok: true }) });
    });
    await firstRow.locator('[data-action="quick-sync-time"]').click();
    await expect(page.locator('.toast.ok')).toContainText('Đã đồng bộ giờ máy chủ');
    expect(timeSynced).toBe(true);

    // 5. Quick Live navigates to camera detail OSD tab
    await firstRow.locator('[data-action="quick-live"]').click();
    await expect(page.getByTestId('camera-detail')).toBeVisible();
    await expect(page.locator('#detail-name')).toHaveText('Cổng chính');
  });

  test('Grid card quick actions do not bubble to open camera detail', async ({ page }) => {
    await page.locator('#cam-view-grid-btn').click();
    const firstCard = page.locator('.cam-card[data-id="cam-1"]');

    // Quick PTZ from Grid card
    await firstCard.locator('[data-action="quick-ptz"]').click();
    const ptzDlg = page.locator('#quick-ptz-dialog');
    await expect(ptzDlg).toBeVisible();
    // Camera detail should remain hidden because event.stopPropagation() prevented card click
    await expect(page.getByTestId('camera-detail')).toBeHidden();
    await ptzDlg.locator('#quick-ptz-close').click();
    await expect(ptzDlg).toBeHidden();

    // Quick Snapshot from Grid card
    await firstCard.locator('[data-action="quick-snap"]').click();
    const lbDlg = page.locator('#lightbox-dialog');
    await expect(lbDlg).toBeVisible();
    await expect(page.getByTestId('camera-detail')).toBeHidden();
    await lbDlg.locator('#lightbox-close').click();
    await expect(lbDlg).toBeHidden();

    // Clicking card checkbox does not open detail
    await firstCard.locator('.cam-card-cb').check();
    await expect(page.getByTestId('camera-detail')).toBeHidden();
    await expect(firstCard).toHaveClass(/selected/);

    // Clicking the card body itself opens camera detail
    await firstCard.locator('.cam-card-title').click();
    await expect(page.getByTestId('camera-detail')).toBeVisible();
    await expect(page.locator('#detail-name')).toHaveText('Cổng chính');
  });

  /* --------------------------------------------------------------------------
   * 3. Smart Bulk Wizard & Hardware Safety Limits Stress Tests
   * -------------------------------------------------------------------------- */
  test('Golden Template 1-click populates all standard parameters correctly', async ({ page }) => {
    await page.getByTestId('task-tab-bulk').click();

    const goldenBtn = page.getByTestId('bulk-golden-template');
    await expect(goldenBtn).toBeVisible();
    await goldenBtn.click();

    // Verify all 5 settings are enabled and populated with Golden Standard values
    await expect(page.locator('#p-codec-enable')).toBeChecked();
    await expect(page.locator('#p-codec-value')).toHaveValue('H.264');

    await expect(page.locator('#p-res-enable')).toBeChecked();
    await expect(page.locator('#p-width')).toHaveValue('1920');
    await expect(page.locator('#p-height')).toHaveValue('1080');

    await expect(page.locator('#p-gop-enable')).toBeChecked();
    await expect(page.locator('#p-gop-value')).toHaveValue('50');

    await expect(page.locator('#p-bitrate-enable')).toBeChecked();
    await expect(page.locator('#p-bitrate-value')).toHaveValue('2048');
    await expect(page.locator('#p-bitrate-mode')).toHaveValue('CBR');

    await expect(page.locator('#p-audio-enable')).toBeChecked();

    // Summary chips
    const summary = page.getByTestId('bulk-summary');
    await expect(summary).toContainText('Codec H.264');
    await expect(summary).toContainText('1920x1080');
    await expect(summary).toContainText('GOP 50');
    await expect(summary).toContainText('Bitrate 2048 Kbps CBR');
    await expect(summary).toContainText('AAC');

    // Safety alert should be hidden for Golden Template (safe configuration)
    await expect(page.locator('#bulk-safety-alert')).toBeHidden();
  });

  test('Hardware Safety Limits Inspector responds to boundary inputs and clears dynamically', async ({ page }) => {
    await page.getByTestId('task-tab-bulk').click();
    const alert = page.locator('#bulk-safety-alert');
    await expect(alert).toBeHidden();

    // 1. Bitrate boundary: > 8192
    await page.locator('#p-bitrate-enable').check();
    await page.locator('#p-bitrate-value').fill('8192');
    await expect(alert).toBeHidden(); // 8192 is safe boundary

    await page.locator('#p-bitrate-value').fill('8193');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('Bitrate 8193 Kbps quá cao');

    await page.locator('#p-bitrate-value').fill('4096');
    await expect(alert).toBeHidden();

    // 2. 4K Resolution with low bitrate (< 2048)
    await page.locator('#p-res-enable').check();
    await page.locator('#p-width').fill('3840');
    await page.locator('#p-height').fill('2160');
    await page.locator('#p-bitrate-value').fill('1024');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('Độ phân giải 4K (3840x2160) với Bitrate quá thấp');

    // Increasing bitrate to safe level (>= 2048) on 4K clears warning
    await page.locator('#p-bitrate-value').fill('4096');
    await expect(alert).toBeHidden();

    // Lowering resolution back to 1080p with 1024 bitrate does not trigger 4K warning
    await page.locator('#p-width').fill('1920');
    await page.locator('#p-height').fill('1080');
    await page.locator('#p-bitrate-value').fill('1024');
    await expect(alert).toBeHidden();

    // 3. GOP boundary: > 200
    await page.locator('#p-gop-enable').check();
    await page.locator('#p-gop-value').fill('200');
    await expect(alert).toBeHidden(); // 200 is safe boundary

    await page.locator('#p-gop-value').fill('201');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('Khoảng I-frame GOP 201 quá lớn');

    await page.locator('#p-gop-value').fill('50');
    await expect(alert).toBeHidden();

    // 4. Combined warnings
    await page.locator('#p-bitrate-value').fill('12000');
    await page.locator('#p-gop-value').fill('300');
    await expect(alert).toBeVisible();
    await expect(alert).toContainText('Bitrate 12000 Kbps quá cao');
    await expect(alert).toContainText('GOP 300 quá lớn');

    // 5. Disabling settings clears the warnings immediately
    await page.locator('#p-bitrate-enable').uncheck();
    await page.locator('#p-gop-enable').uncheck();
    await expect(alert).toBeHidden();
  });

});
