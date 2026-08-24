const { test, expect } = require('@playwright/test');
const { openApp, CAMERAS } = require('./fixtures');

test.describe('M1 Challenger 2: Adversarial Stress Tests for /#cameras', () => {
  let pageErrors = [];
  let consoleErrors = [];

  test.beforeEach(async ({ page }) => {
    pageErrors = [];
    consoleErrors = [];
    page.on('pageerror', err => pageErrors.push(err.message));
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
  });

  test.afterEach(() => {
    // Filter out expected 404s for optional assets like logo.png
    const criticalConsoleErrors = consoleErrors.filter(e => !e.includes('404') && !e.includes('logo.png'));
    expect(pageErrors, `Uncaught JS exceptions: ${pageErrors.join(', ')}`).toEqual([]);
    expect(criticalConsoleErrors, `Critical console errors: ${criticalConsoleErrors.join(', ')}`).toEqual([]);
  });

  // -------------------------------------------------------------
  // 1. Camera Detail Workspace: Fullscreen & Live MJPEG
  // -------------------------------------------------------------
  test('Camera Detail: Live MJPEG preview fullscreen toggle and stream survival', async ({ page }) => {
    await openApp(page, { hash: 'cameras/cam/cam-1/ptz' });
    await expect(page.getByTestId('camera-detail')).toBeVisible();

    const fullscreenBtn = page.locator('#cd-live-fullscreen');
    await expect(fullscreenBtn).toBeVisible();

    // Verify fullscreen toggle handles mock requestFullscreen safely
    await page.evaluate(() => {
      window.__fsCalled = false;
      const wrap = document.getElementById('ce-preview-img-wrap');
      if (wrap) {
        wrap.requestFullscreen = async () => { window.__fsCalled = true; };
      }
    });

    await fullscreenBtn.click();
    const fsTriggered = await page.evaluate(() => window.__fsCalled);
    expect(fsTriggered).toBe(true);

    // Start Live Preview
    await page.getByTestId('live-start').click();
    await expect(page.locator('#cd-live')).toBeVisible();

    // With stream running, fullscreen should target the live img
    await page.evaluate(() => {
      window.__liveFsCalled = false;
      const live = document.getElementById('cd-live');
      if (live) {
        live.requestFullscreen = async () => { window.__liveFsCalled = true; };
      }
    });

    await fullscreenBtn.click();
    const liveFsTriggered = await page.evaluate(() => window.__liveFsCalled);
    expect(liveFsTriggered).toBe(true);
  });

  // -------------------------------------------------------------
  // 2. PTZ Keyboard Shortcuts & Quick PTZ Modal Interactions
  // -------------------------------------------------------------
  test('PTZ: Keyboard shortcuts trigger PTZ commands in PTZ tab, but ignore input fields', async ({ page }) => {
    const ptzCalls = [];
    await openApp(page, {
      hash: 'cameras/cam/cam-1/ptz',
      overrides: {
        '/api/ptz': route => {
          ptzCalls.push(JSON.parse(route.request().postData()));
          route.fulfill({ contentType: 'application/json', body: '{"ok":true}' });
        },
      },
    });

    await expect(page.locator('#ce-panel-ptz')).toBeVisible();

    // 1. Press ArrowUp -> should send start: true, then keyup -> start: false
    await page.keyboard.down('ArrowUp');
    await expect.poll(() => ptzCalls.length).toBeGreaterThan(0);
    expect(ptzCalls[ptzCalls.length - 1]).toMatchObject({ code: 'Up', start: true, id: 'cam-1' });

    await page.keyboard.up('ArrowUp');
    await expect.poll(() => ptzCalls.length).toBeGreaterThan(1);
    expect(ptzCalls[ptzCalls.length - 1]).toMatchObject({ code: 'Up', start: false, id: 'cam-1' });

    // 2. Press 'KeyA' (Left) via WASD
    await page.keyboard.down('KeyA');
    await expect.poll(() => ptzCalls.length).toBeGreaterThan(2);
    expect(ptzCalls[ptzCalls.length - 1]).toMatchObject({ code: 'Left', start: true });

    await page.keyboard.up('KeyA');
    await expect.poll(() => ptzCalls.length).toBeGreaterThan(3);
    expect(ptzCalls[ptzCalls.length - 1]).toMatchObject({ code: 'Left', start: false });

    // 3. ADVERSARIAL: Switch to OSD tab and type 'w', 'a', 's', 'd', 'ArrowUp' in an input
    const callCountBeforeTyping = ptzCalls.length;
    await page.getByTestId('detail-tab-osd').click();
    await expect(page.locator('#ce-panel-osd')).toBeVisible();

    const osdInput = page.locator('.ce-osd-line').first();
    await osdInput.focus();
    await page.keyboard.type('WASD up down left right');

    // Verify NO new PTZ calls were dispatched while typing in text input
    expect(ptzCalls.length).toBe(callCountBeforeTyping);
  });

  test('Quick PTZ Modal: 8-direction pad, speed slider, keyboard control, and goto-detail navigation', async ({ page }) => {
    const ptzCalls = [];
    await openApp(page, {
      hash: 'cameras/list',
      overrides: {
        '/api/ptz': route => {
          ptzCalls.push(JSON.parse(route.request().postData()));
          route.fulfill({ contentType: 'application/json', body: '{"ok":true}' });
        },
      },
    });

    const firstRow = page.getByTestId('camera-row').first();
    await firstRow.locator('[data-action="quick-ptz"]').click();

    const dlg = page.locator('#quick-ptz-dialog');
    await expect(dlg).toBeVisible();
    await expect(page.locator('#quick-ptz-title')).toContainText('Cổng chính');

    // Test speed slider change
    const speedSlider = page.locator('#quick-ptz-speed');
    await speedSlider.fill('8');

    // Test clicking pad button (Right) with .qptz-btn
    const rightBtn = dlg.locator('.qptz-btn[data-ptz="Right"]');
    await rightBtn.dispatchEvent('pointerdown', { pointerId: 1 });
    await expect.poll(() => ptzCalls.length).toBeGreaterThan(0);
    expect(ptzCalls[ptzCalls.length - 1]).toMatchObject({
      id: 'cam-1',
      code: 'Right',
      speed: 8,
      start: true,
    });

    await rightBtn.dispatchEvent('pointerup', { pointerId: 1 });
    await expect.poll(() => ptzCalls.length).toBeGreaterThan(1);
    expect(ptzCalls[ptzCalls.length - 1]).toMatchObject({
      id: 'cam-1',
      code: 'Right',
      speed: 8,
      start: false,
    });

    // Blur speed input so keyboard navigation is active
    await speedSlider.blur();
    await dlg.locator('#quick-ptz-pad').focus();

    // Test Keyboard navigation inside Quick PTZ modal (e.g. ArrowDown)
    await page.keyboard.down('ArrowDown');
    await expect.poll(() => ptzCalls.length).toBeGreaterThan(2);
    expect(ptzCalls[ptzCalls.length - 1]).toMatchObject({
      id: 'cam-1',
      code: 'Down',
      speed: 8,
      start: true,
    });

    await page.keyboard.up('ArrowDown');
    await expect.poll(() => ptzCalls.length).toBeGreaterThan(3);
    expect(ptzCalls[ptzCalls.length - 1]).toMatchObject({
      id: 'cam-1',
      code: 'Down',
      speed: 8,
      start: false,
    });

    // Test "Mở cấu hình PTZ đầy đủ"
    await page.locator('#quick-ptz-goto-detail').click();
    await expect(dlg).toBeHidden();
    await expect(page.getByTestId('camera-detail')).toBeVisible();
    expect(decodeURIComponent(new URL(page.url()).hash)).toBe('#cameras/cam/cam-1/ptz');
  });

  // -------------------------------------------------------------
  // 3. Wi-Fi Scanning RSSI Gauge Rendering & Edge Cases
  // -------------------------------------------------------------
  test('Wi-Fi Scanning: Renders multi-tier RSSI gauges, handles XSS in SSID, and populates form on click', async ({ page }) => {
    await openApp(page, {
      hash: 'cameras/cam/cam-1/network',
      overrides: {
        '/api/wifi-scan': route => {
          route.fulfill({
            contentType: 'application/json',
            body: JSON.stringify({
              devices: [
                { ssid: 'Bida-VIP-5G', linkQuality: 92, signalLevel: -45, security: 'WPA2' },
                { ssid: 'Bida-Guest', linkQuality: 65, signalLevel: -65, security: 'WPA2' },
                { ssid: 'Weak-Signal-AP', linkQuality: 30, signalLevel: -85, security: 'WPA' },
                { ssid: '<script>alert("xss")</script>', linkQuality: 15, signalLevel: -90, security: 'Open' },
              ],
            }),
          });
        },
      },
    });

    await expect(page.getByTestId('detail-network-body')).toBeVisible();
    const scanBtn = page.locator('#net-wifi-scan-btn');
    await expect(scanBtn).toBeVisible();

    await scanBtn.click();
    const results = page.locator('#net-wifi-scan-results');
    await expect(results).toBeVisible();

    const meters = results.locator('.wifi-rssi-meter');
    await expect(meters).toHaveCount(4);

    // 1. High Quality AP (92%) -> active-high bars
    const highMeter = meters.nth(0);
    await expect(highMeter).toContainText('Bida-VIP-5G');
    await expect(highMeter).toContainText('(92%)');
    await expect(highMeter.locator('.wifi-signal-bar.active-high')).toHaveCount(4);

    // 2. Med Quality AP (65%) -> active-med bars
    const medMeter = meters.nth(1);
    await expect(medMeter).toContainText('Bida-Guest');
    await expect(medMeter).toContainText('(65%)');
    await expect(medMeter.locator('.wifi-signal-bar.active-med')).toHaveCount(2);

    // 3. Low Quality AP (30%) -> active-low bar
    const lowMeter = meters.nth(2);
    await expect(lowMeter).toContainText('Weak-Signal-AP');
    await expect(lowMeter).toContainText('(30%)');
    await expect(lowMeter.locator('.wifi-signal-bar.active-low')).toHaveCount(1);

    // 4. XSS Security check in SSID
    const xssMeter = meters.nth(3);
    await expect(xssMeter).toContainText('<script>alert("xss")</script>');
    // Verify no literal script tag was injected into DOM
    expect(await page.locator('#net-wifi-scan-results script').count()).toBe(0);

    // 5. Click chip to populate input
    await highMeter.click();
    await expect(page.locator('#net-wifi-ssid')).toHaveValue('Bida-VIP-5G');
  });

  // -------------------------------------------------------------
  // 4. NVR Diagnostics, Sub-Channel Mapping & Watchdog
  // -------------------------------------------------------------
  test('NVR Diagnostics: Health timeline, sub-channel scanning, and self-healing watchdog toggle', async ({ page }) => {
    const watchdogCalls = [];
    const linkCalls = [];

    await openApp(page, {
      hash: 'cameras/nvr',
      overrides: {
        '/api/nvr/watchdog': route => {
          watchdogCalls.push(JSON.parse(route.request().postData()));
          route.fulfill({ contentType: 'application/json', body: '{"ok":true}' });
        },
        '/api/nvr/link': route => {
          linkCalls.push(JSON.parse(route.request().postData()));
          route.fulfill({ contentType: 'application/json', body: '{"ok":true}' });
        },
      },
    });

    const nvrList = page.getByTestId('nvr-list');
    await expect(nvrList).toBeVisible();

    // 1. Check Watchdog toggle interaction
    const watchdogCb = page.getByTestId('nvr-watchdog').first();
    await expect(watchdogCb).toBeChecked();
    await watchdogCb.uncheck();

    await expect.poll(() => watchdogCalls.length).toBe(1);
    expect(watchdogCalls[0]).toMatchObject({ id: 'nvr-1', enabled: false });

    // 2. Check NVR Scan Dialog & Channel linking
    await page.getByTestId('nvr-link-open').click();
    const linkDlg = page.locator('#nvr-link-dialog');
    await expect(linkDlg).toBeVisible();

    await page.locator('#nvr-host').fill('192.168.1.253');
    await page.locator('#nvr-port').fill('37777');
    await page.locator('#nvr-user').fill('admin');
    await page.locator('#nvr-pass').fill('Admin123');
    await page.locator('#nvr-scan-btn').click();

    await expect(page.locator('#nvr-tbody tr')).toHaveCount(2);
    await page.locator('#nvr-save-btn').click();

    await expect.poll(() => linkCalls.length).toBe(1);
    expect(linkCalls[0].nvr).toMatchObject({ host: '192.168.1.253', port: 37777 });
    await expect(linkDlg).toBeHidden();
  });

  // -------------------------------------------------------------
  // 5. Grid View / Table View Selector & DOM Consistency
  // -------------------------------------------------------------
  test('DOM Resilience: Simultaneous table & grid synchronization with vendor badges and quick actions', async ({ page }) => {
    await openApp(page, {
      hash: 'cameras/list',
      overrides: {
        '/api/probe': route => route.fulfill({
          contentType: 'application/json',
          body: JSON.stringify({
            streams: [{ channel: 1, stream: 0, width: 1920, height: 1080, fps: 25, compression: 'H.264', audioCodec: 'AAC', audioEnable: true }],
          }),
        }),
      },
    });

    // Table view assertions
    const tableRows = page.getByTestId('camera-row');
    await expect(tableRows).toHaveCount(3);

    // Probe first camera to populate probeCache
    const standardRow = tableRows.first();
    await standardRow.locator('.row-menu > summary').click();
    await standardRow.getByRole('button', { name: 'Dò cấu hình' }).click();
    await expect(standardRow.locator('.probe-box')).toContainText('1920x1080');

    // Switch to Grid View
    await page.locator('#cam-view-grid-btn').click();
    const gridCards = page.getByTestId('camera-card');
    await expect(gridCards).toHaveCount(3);

    // Verify vendor badges on cards (case insensitive)
    const firstCard = gridCards.first();
    await expect(firstCard.locator('.cam-card-vendor-badge')).toContainText(/dahua/i);
    await expect(firstCard.locator('.cam-spec-tag').filter({ hasText: 'H.264' })).toHaveCount(1);
    await expect(firstCard.locator('.cam-spec-tag').filter({ hasText: '1920x1080' })).toHaveCount(1);
    await expect(firstCard.locator('.cam-spec-tag').filter({ hasText: 'AAC' })).toHaveCount(1);

    // Verify quick action buttons on card
    await expect(firstCard.locator('[data-action="quick-live"]')).toBeVisible();
    await expect(firstCard.locator('[data-action="quick-snap"]')).toBeVisible();
    await expect(firstCard.locator('[data-action="quick-ptz"]')).toBeVisible();

    // Verify selecting checkbox in grid syncs selection count
    await firstCard.locator('.cam-card-cb').check();
    await expect(page.locator('#bulk-selected-count')).toContainText('1 camera');

    // Switch back to table view and verify checkbox is still checked
    await page.locator('#cam-view-table-btn').click();
    await expect(tableRows.first().locator('.cam-cb')).toBeChecked();
  });
});
