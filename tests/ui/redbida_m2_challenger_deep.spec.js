const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test.describe('Milestone 2 Adversarial Stress Suite (Challenger 2)', () => {
  test('1. Visual 20-Tab INI Editor: Exhaustive matrix iteration, per-tab editing, 1-click sync, quick copy, and visual/raw roundtrip', async ({ page }) => {
    const consoleErrors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    page.on('pageerror', err => consoleErrors.push(err.message));

    await openApp(page, {
      hash: 'redbida',
      overrides: {
        '/api/redbida/catalog': {
          keys: [
            { key: 'ui_title', label: 'Tên quán', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'ui_tabs_links', label: '20 tab INI', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
          ],
        },
        '/api/redbida/refresh': {
          values: [
            { key: 'ui_title', value: 'Bida Hoàng Gia Saigon', exists: true },
            { key: 'ui_tabs_links', value: '', exists: true },
          ],
        },
      },
    });

    const panel20 = page.locator('#redbida-20tab-panel');
    await expect(panel20).toBeVisible();

    const matrixButtons = page.locator('#redbida-tab-matrix-grid .redbida-matrix-btn');
    await expect(matrixButtons).toHaveCount(20);

    // 1.1 Test clicking every single button from C01 to C20
    for (let i = 1; i <= 20; i++) {
      const pad = String(i).padStart(2, '0');
      const expectedId = `C${pad}`;
      const btn = matrixButtons.nth(i - 1);

      await btn.click();
      await expect(btn).toHaveClass(/active/);
      await expect(page.locator('#redbida-current-tab-title')).toContainText(expectedId);
      await expect(page.locator('#redbida-current-tab-title')).toContainText(`Bàn ${pad}`);
    }

    // 1.2 Test editing multiple distinct tabs
    // Edit C03
    await matrixButtons.nth(2).click(); // C03
    await page.locator('#redbida-tab-play-label').fill('Bàn 03 Super VIP');
    await page.locator('#redbida-tab-stream-label').fill('Luồng 4K Bàn 3');
    await page.locator('#redbida-tab-list-label').fill('Top Bàn 3');
    await page.locator('#redbida-tab-refresh-label').fill('F5 Bàn 3');

    // Edit C18
    await matrixButtons.nth(17).click(); // C18
    await page.locator('#redbida-tab-play-label').fill('Bàn 18 Thi Đấu');
    await page.locator('#redbida-tab-stream-label').fill('Luồng Thi Đấu 18');

    // Switch back to C03 and verify edits persisted
    await matrixButtons.nth(2).click();
    await expect(page.locator('#redbida-tab-play-label')).toHaveValue('Bàn 03 Super VIP');
    await expect(page.locator('#redbida-tab-stream-label')).toHaveValue('Luồng 4K Bàn 3');
    await expect(page.locator('#redbida-tab-list-label')).toHaveValue('Top Bàn 3');
    await expect(page.locator('#redbida-tab-refresh-label')).toHaveValue('F5 Bàn 3');

    // Switch to C18 and verify edits persisted
    await matrixButtons.nth(17).click();
    await expect(page.locator('#redbida-tab-play-label')).toHaveValue('Bàn 18 Thi Đấu');
    await expect(page.locator('#redbida-tab-stream-label')).toHaveValue('Luồng Thi Đấu 18');

    // 1.3 Test "Quick Copy URL" for C01, C07, C20
    await matrixButtons.nth(0).click(); // C01
    await page.locator('#redbida-tab-copy-url-btn').click();
    await expect(page.locator('#redbida-msg')).toContainText('channel=1&subtype=0');

    await matrixButtons.nth(6).click(); // C07
    await page.locator('#redbida-tab-copy-url-btn').click();
    await expect(page.locator('#redbida-msg')).toContainText('channel=7&subtype=0');

    await matrixButtons.nth(19).click(); // C20
    await page.locator('#redbida-tab-copy-url-btn').click();
    await expect(page.locator('#redbida-msg')).toContainText('channel=20&subtype=0');

    // 1.4 Test "1-Click Sync Venue Name to 20 tables"
    // In our fixture, effective ui_title is 'Bida Hoàng Gia Saigon'
    await page.locator('#redbida-tab-sync-title-btn').click();
    await expect(page.locator('#redbida-msg')).toContainText('Đã đồng bộ tên quán "Bida Hoàng Gia Saigon" cho toàn bộ 20 bàn bida');

    // Verify all 20 tabs have the synced vid_play_label = 'Bida Hoàng Gia Saigon'
    for (let i = 1; i <= 20; i++) {
      const btn = matrixButtons.nth(i - 1);
      await btn.click();
      await expect(page.locator('#redbida-tab-play-label')).toHaveValue('Bida Hoàng Gia Saigon');
    }

    // Now edit ui_title in drafts via table and sync again
    const titleRowInput = page.locator('[data-red-row="ui_title"] [data-red-key="ui_title"]');
    await titleRowInput.fill('CLB Bida Master Luxury');
    await page.locator('#redbida-tab-sync-title-btn').click();
    await expect(page.locator('#redbida-msg')).toContainText('Đã đồng bộ tên quán "CLB Bida Master Luxury" cho toàn bộ 20 bàn bida');

    for (let i = 1; i <= 20; i++) {
      const btn = matrixButtons.nth(i - 1);
      await btn.click();
      await expect(page.locator('#redbida-tab-play-label')).toHaveValue('CLB Bida Master Luxury');
    }

    // 1.5 Test Switching between Visual Form and Raw INI without data corruption
    const toggleBtn = page.locator('#redbida-tab-view-toggle');
    await toggleBtn.click(); // to Raw mode
    await expect(page.locator('#redbida-tab-raw-wrap')).toBeVisible();
    await expect(page.locator('#redbida-tab-visual-wrap')).toBeHidden();

    let rawText = await page.locator('#redbida-tab-raw-ini').inputValue();
    expect(rawText).toContain('[C01]');
    expect(rawText).toContain('[C20]');
    expect(rawText).toContain('vid_play_label=CLB Bida Master Luxury');

    // Edit raw text in INI: modify C08
    const modifiedRaw = rawText.replace(
      /\[C08\][\s\S]*?(?=\n\n\[C09\])/,
      '[C08]\nstream_label=Stream Cực Nét Bàn 8\nvid_list_label=Danh sách Bàn 8\nvid_play_label=CLB Bida Master Luxury - Bàn VIP 8\nlist_refresh_label=Làm mới Bàn 8'
    );
    await page.locator('#redbida-tab-raw-ini').fill(modifiedRaw);

    // Switch back to Visual Form
    await toggleBtn.click(); // to Visual mode
    await expect(page.locator('#redbida-tab-visual-wrap')).toBeVisible();
    await expect(page.locator('#redbida-tab-raw-wrap')).toBeHidden();

    // Select C08 in Visual mode and assert the raw edits were parsed correctly
    await matrixButtons.nth(7).click(); // C08
    await expect(page.locator('#redbida-tab-play-label')).toHaveValue('CLB Bida Master Luxury - Bàn VIP 8');
    await expect(page.locator('#redbida-tab-stream-label')).toHaveValue('Stream Cực Nét Bàn 8');
    await expect(page.locator('#redbida-tab-list-label')).toHaveValue('Danh sách Bàn 8');
    await expect(page.locator('#redbida-tab-refresh-label')).toHaveValue('Làm mới Bàn 8');

    // Switch back to Raw mode again and check roundtrip integrity
    await toggleBtn.click();
    rawText = await page.locator('#redbida-tab-raw-ini').inputValue();
    expect(rawText).toContain('stream_label=Stream Cực Nét Bàn 8');
    expect(rawText).toContain('vid_play_label=CLB Bida Master Luxury - Bàn VIP 8');

    // Check zero console errors (excluding external unmocked asset network fails)
    expect(consoleErrors.filter(e => !e.includes('Failed to load resource'))).toEqual([]);
  });

  test('2. Key Management Table, Group Pills, Search, Inline Previews, and Risk Badges', async ({ page }) => {
    const consoleErrors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    page.on('pageerror', err => consoleErrors.push(err.message));

    await openApp(page, {
      hash: 'redbida',
      overrides: {
        '/api/redbida/catalog': {
          keys: [
            { key: 'ui_title', label: 'Tên quán bida', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'logo_header', label: 'Logo đỉnh', group: 'Branding / Logo', risk: 'editable', valueType: 'image', editable: true },
            { key: 'ui_bg', label: 'Màu nền gradient', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'camera_count', label: 'Số camera bida', group: 'Video & Streaming', risk: 'editable', valueType: 'number', editable: true },
            { key: 'hls_using_go2rtc', label: 'Go2RTC streaming', group: 'Video & Streaming', risk: 'editable', valueType: 'boolean', editable: true },
            { key: 'shinobi_token', label: 'Shinobi Token IP 0.0.0.0', group: 'Security / Credentials', risk: 'read-only-protected', valueType: 'string', editable: false },
            { key: 'mqtt_password', label: 'Mật khẩu MQTT broker', group: 'Security / Credentials', risk: 'read-only-protected', valueType: 'string', editable: false },
            { key: 'button_reboot', label: 'Reboot thiết bị', group: 'Schedule & Maintenance', risk: 'confirm-required', valueType: 'boolean', editable: true },
            { key: 'frpc_config', label: 'FRP Cloud Tunnel', group: 'Schedule & Maintenance', risk: 'editable', valueType: 'string', editable: true },
          ],
        },
        '/api/redbida/refresh': {
          values: [
            { key: 'ui_title', value: 'Bida Sài Gòn', exists: true },
            { key: 'logo_header', value: 'https://example.test/bida_logo.png', exists: true },
            { key: 'ui_bg', value: 'linear-gradient(135deg, #0b192c 0%, #1e3e62 50%, #000000 100%)', exists: true },
            { key: 'camera_count', value: 12, exists: true },
            { key: 'hls_using_go2rtc', value: true, exists: true },
            { key: 'shinobi_token', value: 'tok_abc123', exists: true },
            { key: 'mqtt_password', value: '********', exists: true },
            { key: 'button_reboot', value: false, exists: true },
            { key: 'frpc_config', value: '[common]\nserver_addr=1.2.3.4', exists: true },
          ],
        },
      },
    });

    const rows = page.locator('#redbida-tbody tr');
    await expect(rows).toHaveCount(9);

    // 2.1 Test Group Pills Filtering
    // Click "Branding" pill
    const brandingPill = page.locator('#redbida-group-pills button[data-pill-group="Branding / Logo"]');
    await brandingPill.click();
    await expect(brandingPill).toHaveClass(/active/);
    await expect(page.locator('#redbida-group')).toHaveValue('Branding / Logo');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(3);
    await expect(page.locator('[data-red-row="ui_title"]')).toBeVisible();
    await expect(page.locator('[data-red-row="logo_header"]')).toBeVisible();
    await expect(page.locator('[data-red-row="ui_bg"]')).toBeVisible();

    // Click "Streaming" pill
    const streamingPill = page.locator('#redbida-group-pills button[data-pill-group="Video & Streaming"]');
    await streamingPill.click();
    await expect(streamingPill).toHaveClass(/active/);
    await expect(page.locator('#redbida-group')).toHaveValue('Video & Streaming');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(2);
    await expect(page.locator('[data-red-row="camera_count"]')).toBeVisible();
    await expect(page.locator('[data-red-row="hls_using_go2rtc"]')).toBeVisible();

    // Click "Shinobi" pill
    const shinobiPill = page.locator('#redbida-group-pills button[data-pill-group="Security / Credentials"]');
    await shinobiPill.click();
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(2);
    await expect(page.locator('[data-red-row="shinobi_token"]')).toBeVisible();
    await expect(page.locator('[data-red-row="mqtt_password"]')).toBeVisible();

    // Click "Hệ thống" pill
    const sysPill = page.locator('#redbida-group-pills button[data-pill-group="Schedule & Maintenance"]');
    await sysPill.click();
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(2);
    await expect(page.locator('[data-red-row="button_reboot"]')).toBeVisible();
    await expect(page.locator('[data-red-row="frpc_config"]')).toBeVisible();

    // Reset to "Tất cả"
    const allPill = page.locator('#redbida-group-pills button[data-pill-group=""]');
    await allPill.click();
    await expect(allPill).toHaveClass(/active/);
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(9);

    // 2.2 Test Search Input
    const searchInput = page.locator('#redbida-search');

    // Case 1: Search by key prefix
    await searchInput.fill('logo_');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(1);
    await expect(page.locator('[data-red-row="logo_header"]')).toBeVisible();

    // Case 2: Case-insensitive search
    await searchInput.fill('SHINOBI');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(1);
    await expect(page.locator('[data-red-row="shinobi_token"]')).toBeVisible();

    // Case 3: Vietnamese accented search & label search
    await searchInput.fill('thiết bị');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(1);
    await expect(page.locator('[data-red-row="button_reboot"]')).toBeVisible();

    // Case 4: Key search
    await searchInput.fill('button_reboot');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(1);
    await expect(page.locator('[data-red-row="button_reboot"]')).toBeVisible();

    // Case 5: Non-matching search query
    await searchInput.fill('khongco_keynay_123');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(1);
    await expect(page.locator('#redbida-tbody tr td.empty-hint')).toContainText('Không có key khớp bộ lọc');

    // Clear search
    await searchInput.fill('');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(9);

    // 2.3 Test Inline Logo Checkerboard Preview
    const logoRow = page.locator('[data-red-row="logo_header"]');
    await expect(logoRow.locator('.redbida-checkerboard')).toBeVisible();
    const logoImg = logoRow.locator('img.redbida-logo-preview');
    await expect(logoImg).toHaveAttribute('src', 'https://example.test/bida_logo.png');

    // 2.4 Test Inline Gradient Swatch Preview for ui_bg
    const bgRow = page.locator('[data-red-row="ui_bg"]');
    const bgPreview = bgRow.locator('.redbida-row-bg-preview');
    await expect(bgPreview).toBeVisible();
    await expect(bgPreview).toContainText('Gradient Preview');

    // 2.5 Test Risk Badges
    await expect(page.locator('[data-red-row="ui_title"] .badge')).toHaveClass(/redbida-risk-editable/);
    await expect(page.locator('[data-red-row="shinobi_token"] .badge')).toHaveClass(/redbida-risk-read-only-protected/);
    await expect(page.locator('[data-red-row="button_reboot"] .badge')).toHaveClass(/redbida-risk-confirm-required/);

    expect(consoleErrors.filter(e => !e.includes('Failed to load resource'))).toEqual([]);
  });

  test('3. DOM Resilience, Rapid Interactions, Edge Cases, and Zero Console Error Guarantee', async ({ page }) => {
    const pageErrors = [];
    const consoleErrors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    page.on('pageerror', err => pageErrors.push(err.message));

    await openApp(page, {
      hash: 'redbida',
    });

    // 3.1 Rapidly click all 8 gradient swatches in succession
    const swatches = page.locator('#redbida-preset-swatches .redbida-swatch');
    for (let i = 0; i < 8; i++) {
      await swatches.nth(i).click();
      await expect(swatches.nth(i)).toHaveClass(/active/);
    }

    // 3.2 Rapidly toggle all accordion panels
    const toggles = [
      '#redbida-toggle-inspector',
      '#redbida-toggle-preset',
      '#redbida-toggle-20tabs',
      '#redbida-toggle-hub',
      '#redbida-toggle-checklist',
    ];
    for (const toggleId of toggles) {
      const toggleEl = page.locator(toggleId);
      if (await toggleEl.count() > 0) {
        await toggleEl.click();
        await toggleEl.click(); // Toggle back
      }
    }

    // 3.3 Rapidly click through all 20 matrix buttons
    const matrixButtons = page.locator('#redbida-tab-matrix-grid .redbida-matrix-btn');
    for (let i = 0; i < 20; i++) {
      await matrixButtons.nth(i).click();
    }

    // 3.4 Edge Case: Preset generation with empty title or special characters
    const titleInput = page.locator('#redbida-preset-title');
    await titleInput.fill('');
    await page.locator('#redbida-preset-gen-btn').click();
    // Default title "CX King Luxury" should be applied safely without throwing
    await expect(page.locator('#redbida-msg')).toContainText('CX King Luxury');

    await titleInput.fill('Quán Bida "Vua Phá Lưới" & [VIP] <3> #1 100%');
    await page.locator('#redbida-preset-gen-btn').click();
    await expect(page.locator('#redbida-msg')).toContainText('Đã sinh preset 1-click thành công');

    // 3.5 Edge Case: Corrupt Raw INI input handling
    const toggleBtn = page.locator('#redbida-tab-view-toggle');
    await toggleBtn.click(); // to Raw mode
    await page.locator('#redbida-tab-raw-ini').fill('corrupted invalid garbage text !!! \n [invalid_section');
    await toggleBtn.click(); // back to visual mode
    // Parser must not crash, should fallback safely and populate 20 tabs
    await expect(page.locator('#redbida-tab-visual-wrap')).toBeVisible();
    await expect(matrixButtons).toHaveCount(20);

    // Verify zero uncaught JS exceptions
    expect(pageErrors).toEqual([]);
    expect(consoleErrors.filter(e => !e.includes('Failed to load resource'))).toEqual([]);
  });
});
