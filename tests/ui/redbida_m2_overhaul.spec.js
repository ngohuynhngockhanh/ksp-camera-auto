const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test.describe('RedBida Milestone 2 UI Overhaul E2E Suite', () => {
  test('Golden Standard Inspector calculates score, supports single-key fix and 1-click fix all', async ({ page }) => {
    await openApp(page, {
      hash: 'redbida',
      overrides: {
        '/api/redbida/catalog': {
          keys: [
            { key: 'ui_title', label: 'Tên quán', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'company_name', label: 'Tên công ty', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'ui_bg', label: 'Màu nền CSS', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'custom_hashtags', label: 'Hashtag', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'ui_tabs_links', label: '20 tab INI', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'camera_count', label: 'Số camera', group: 'Video & Streaming', risk: 'editable', valueType: 'number', editable: true },
            { key: 'toolbar_show_count', label: 'Số camera toolbar', group: 'Video & Streaming', risk: 'editable', valueType: 'number', editable: true },
            { key: 'video_config', label: 'Video config', group: 'Video & Streaming', risk: 'editable', valueType: 'string', editable: true },
            { key: 'hls_using_go2rtc', label: 'Go2RTC HLS', group: 'Video & Streaming', risk: 'editable', valueType: 'boolean', editable: true },
            { key: 'hls_using_go2rtc_livestream', label: 'Go2RTC Live', group: 'Video & Streaming', risk: 'editable', valueType: 'boolean', editable: true },
            { key: 'hls_using_go2rtc_tiktok', label: 'Go2RTC TikTok', group: 'Video & Streaming', risk: 'editable', valueType: 'boolean', editable: true },
            { key: 'ui_scoreboard', label: 'Bảng điểm', group: 'Branding / Logo', risk: 'editable', valueType: 'boolean', editable: true },
            { key: 'logo_header', label: 'Logo header', group: 'Branding / Logo', risk: 'editable', valueType: 'image', editable: true },
            { key: 'logo_header_text', label: 'Logo text', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'button_generate_go2rtc_stream', label: 'Trigger Go2RTC', group: 'Video & Streaming', risk: 'editable', valueType: 'boolean', editable: true },
          ],
        },
        '/api/redbida/refresh': {
          values: [
            { key: 'ui_title', value: 'CX King Luxury', exists: true },
            { key: 'camera_count', value: 8, exists: true },
            // Other keys missing / out of spec
          ],
        },
      },
    });

    // Verify Inspector Panel is rendered
    const inspectorPanel = page.locator('#redbida-inspector-panel');
    await expect(inspectorPanel).toBeVisible();

    // Check progress bar and score badge exist
    const progressBar = page.locator('#redbida-inspector-progress-bar');
    await expect(progressBar).toBeVisible();
    const scoreBadge = page.locator('#redbida-inspector-badge');
    await expect(scoreBadge).toBeVisible();

    // Test single-key quick fix: Click "⚡ Sửa nhanh" for video_config
    const fixVideoConfigBtn = page.locator('[data-autofix-key="video_config"]');
    await expect(fixVideoConfigBtn).toBeVisible();
    await fixVideoConfigBtn.click();

    // Verify video_config draft populated
    await expect(page.locator('[data-red-row="video_config"]')).toHaveClass(/redbida-dirty/);
    await expect(page.locator('[data-check-key="video_config"]')).toHaveClass(/passed/);

    // Test 1-Click Fix All
    const fixAllBtn = page.locator('#redbida-autofix-all-btn');
    await fixAllBtn.click();

    // Verify 100% score achieved
    await expect(scoreBadge).toHaveText(/100% Chuẩn Bida/);
    await expect(page.locator('#redbida-standard-score')).toHaveText('100%');
    await expect(progressBar).toHaveCSS('width', /(100%|[0-9]+px)/);

    // Verify diff card appeared
    await expect(page.locator('#redbida-preset-diff')).toBeVisible();
  });

  test('Curated 8 CSS Gradient Palette and Live Canvas Preview reflect selection with no trailing semicolon', async ({ page }) => {
    await openApp(page, {
      hash: 'redbida',
    });

    // Check all 8 swatches exist
    const swatches = page.locator('#redbida-preset-swatches .redbida-swatch');
    await expect(swatches).toHaveCount(8);

    // Click 3rd swatch (Cyberpunk Neon)
    await swatches.nth(2).click();
    await expect(swatches.nth(2)).toHaveClass(/active/);
    const bgVal = await page.locator('#redbida-preset-bg').inputValue();
    expect(bgVal).toContain('linear-gradient');
    expect(bgVal.endsWith(';')).toBe(false);

    // Check Live Canvas Preview background updated
    const canvasPreview = page.locator('#redbida-preset-bg-preview');
    await expect(canvasPreview).toBeVisible();

    // Click 8th swatch (Ruby Luxury)
    await swatches.nth(7).click();
    await expect(swatches.nth(7)).toHaveClass(/active/);
    const bgVal8 = await page.locator('#redbida-preset-bg').inputValue();
    expect(bgVal8).toContain('#3d0c11');
    expect(bgVal8.endsWith(';')).toBe(false);
  });

  test('Visual 20-Tab INI Editor allows per-table inspection, 1-click title sync, and raw INI toggle', async ({ page }) => {
    await openApp(page, {
      hash: 'redbida',
    });

    // Check Visual 20-Tab Panel is visible
    const tab20Panel = page.locator('#redbida-20tab-panel');
    await expect(tab20Panel).toBeVisible();

    // Check 20 table matrix buttons exist
    const matrixButtons = page.locator('#redbida-tab-matrix-grid .redbida-matrix-btn');
    await expect(matrixButtons).toHaveCount(20);

    // Select Table 5 (C05)
    await matrixButtons.nth(4).click();
    await expect(matrixButtons.nth(4)).toHaveClass(/active/);
    await expect(page.locator('#redbida-current-tab-title')).toContainText('C05');

    // Edit Table 5 display label
    await page.locator('#redbida-tab-play-label').fill('Bàn VIP 05 Hoàng Gia');

    // 1-Click Sync title to all 20 tables
    await page.locator('#redbida-tab-sync-title-btn').click();
    await expect(page.locator('#redbida-tab-play-label')).toHaveValue('CX King Luxury');

    // Toggle Raw INI view
    const toggleBtn = page.locator('#redbida-tab-view-toggle');
    await toggleBtn.click();
    await expect(page.locator('#redbida-tab-raw-wrap')).toBeVisible();
    const rawIniText = await page.locator('#redbida-tab-raw-ini').inputValue();
    expect(rawIniText).toContain('[C01]');
    expect(rawIniText).toContain('[C20]');
    expect(rawIniText).toContain('vid_play_label=CX King Luxury');

    // Toggle back to Visual Form
    await toggleBtn.click();
    await expect(page.locator('#redbida-tab-visual-wrap')).toBeVisible();
  });

  test('Smart Hashtag Generator strips Vietnamese diacritics in real-time as user types', async ({ page }) => {
    await openApp(page, {
      hash: 'redbida',
    });

    const titleInput = page.locator('#redbida-preset-title');
    await titleInput.fill('Bida Đẳng Cấp Sài Gòn & Luxury');

    // Verify realtime hashtags preview
    const hashtagsPreview = page.locator('#redbida-preset-hashtags-preview');
    await expect(hashtagsPreview).toContainText('#BidaDangCapSaiGonLuxury');
    await expect(hashtagsPreview).toContainText('#BILLIARDSlive');
    await expect(hashtagsPreview).toContainText('#INUTlive');
    await expect(hashtagsPreview).toContainText('#highlightsports');

    // Verify canvas preview hashtags updated
    const canvasHashtags = page.locator('#redbida-canvas-hashtags');
    await expect(canvasHashtags).toContainText('#BidaDangCapSaiGonLuxury');
  });

  test('Group filtering pills in toolbar filter key table quickly', async ({ page }) => {
    await openApp(page, {
      hash: 'redbida',
      overrides: {
        '/api/redbida/catalog': {
          keys: [
            { key: 'ui_title', label: 'Tên quán', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true },
            { key: 'camera_count', label: 'Số camera', group: 'Video & Streaming', risk: 'editable', valueType: 'number', editable: true },
            { key: 'shinobi_token', label: 'Shinobi Token', group: 'Security / Credentials', risk: 'protected', valueType: 'string', editable: false },
          ],
        },
      },
    });

    // Click "Branding" pill
    const brandingPill = page.locator('#redbida-group-pills button[data-pill-group="Branding / Logo"]');
    await brandingPill.click();
    await expect(page.locator('#redbida-group')).toHaveValue('Branding / Logo');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(1);
    await expect(page.locator('[data-red-row="ui_title"]')).toBeVisible();

    // Click "Tất cả" pill
    const allPill = page.locator('#redbida-group-pills button[data-pill-group=""]');
    await allPill.click();
    await expect(page.locator('#redbida-group')).toHaveValue('');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(3);
  });
});
