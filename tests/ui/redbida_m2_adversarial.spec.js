const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test.describe('RedBida M2 Adversarial Stress Testing & Edge Cases', () => {

  test('1. Golden Standard Inspector & 1-Click Auto-Fix Stress Test', async ({ page }) => {
    // Start with all 15 keys completely out-of-spec or corrupted
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
            { key: 'ui_title', value: '', exists: true }, // Invalid: empty string
            { key: 'company_name', value: '', exists: true }, // Invalid: empty string
            { key: 'ui_bg', value: 'linear-gradient(135deg, #0b192c, #000);;;  ', exists: true }, // Invalid: trailing semicolons & spaces
            { key: 'custom_hashtags', value: '#QuánBidaHoàngGia #Bida', exists: true }, // Invalid: has Vietnamese accents & missing standard tags
            { key: 'ui_tabs_links', value: '[C01]\nvid_play_label=Test', exists: true }, // Invalid: missing [C20]
            { key: 'camera_count', value: 25, exists: true }, // Invalid: >20
            { key: 'toolbar_show_count', value: 8, exists: true }, // Invalid: does not match camera_count
            { key: 'video_config', value: 'range=24', exists: true }, // Invalid: must be range=72
            { key: 'hls_using_go2rtc', value: false, exists: true }, // Invalid: must be true
            { key: 'hls_using_go2rtc_livestream', value: false, exists: true }, // Invalid: must be true
            { key: 'hls_using_go2rtc_tiktok', value: false, exists: true }, // Invalid: must be true
            { key: 'ui_scoreboard', value: false, exists: true }, // Invalid: must be true
            { key: 'logo_header', value: 'invalid-scheme://foo.bar/img.png', exists: true }, // Invalid: wrong URL scheme
            { key: 'logo_header_text', value: '   ', exists: true }, // Invalid: whitespace only
            { key: 'button_generate_go2rtc_stream', value: false, exists: true }, // Invalid: must be true
          ],
        },
      },
    });

    // 1.1 Verify initial audit score is 0% (0/15 passed)
    const auditRes = await page.evaluate(() => window.redbidaAuditGoldenStandard());
    expect(auditRes.passedCount).toBe(0);
    expect(auditRes.total).toBe(15);
    expect(auditRes.score).toBe(0);

    // Verify DOM updates for 0%
    await expect(page.locator('#redbida-standard-score')).toHaveText('0%');
    await expect(page.locator('#redbida-standard-sub')).toHaveText('0/15 key đạt chuẩn');
    await expect(page.locator('#redbida-inspector-badge')).toHaveText(/0% Chuẩn Bida \(Lệch Chuẩn\)/);
    await expect(page.locator('#redbida-inspector-badge')).toHaveClass(/badge-danger/);

    // 1.2 Test individual key fixes sequentially and check intermediate % calculations
    // Fix ui_title -> 1/15 = 7%
    await page.evaluate(() => window.redbidaAutoFixKey('ui_title'));
    const auditAfterTitle = await page.evaluate(() => window.redbidaAuditGoldenStandard());
    expect(auditAfterTitle.passedCount).toBeGreaterThanOrEqual(1);

    // Fix ui_bg -> should strip trailing semicolons
    await page.evaluate(() => window.redbidaAutoFixKey('ui_bg'));
    const effectiveBg = await page.evaluate(() => window.redbidaState.drafts.get('ui_bg'));
    expect(effectiveBg.endsWith(';')).toBe(false);
    expect(effectiveBg).toContain('linear-gradient');

    // Fix video_config -> should be 'range=72'
    await page.evaluate(() => window.redbidaAutoFixKey('video_config'));
    const effectiveVideoConfig = await page.evaluate(() => window.redbidaState.drafts.get('video_config'));
    expect(effectiveVideoConfig).toBe('range=72');

    // 1.3 Test 1-Click Fix All
    const fixAllBtn = page.locator('#redbida-autofix-all-btn');
    await fixAllBtn.click();

    // Verify 100% score (15/15)
    const auditFinal = await page.evaluate(() => window.redbidaAuditGoldenStandard());
    expect(auditFinal.passedCount).toBe(15);
    expect(auditFinal.total).toBe(15);
    expect(auditFinal.score).toBe(100);

    await expect(page.locator('#redbida-standard-score')).toHaveText('100%');
    await expect(page.locator('#redbida-standard-sub')).toHaveText('15/15 key đạt chuẩn');
    await expect(page.locator('#redbida-inspector-badge')).toHaveText(/100% Chuẩn Bida \(Hoàn Hảo\)/);
    await expect(page.locator('#redbida-inspector-badge')).toHaveClass(/badge-success/);

    // 1.4 Verify diff card updated with all drafted changes
    const diffCard = page.locator('#redbida-preset-diff');
    await expect(diffCard).toBeVisible();
    await expect(diffCard).toContainText('15 tham số');
  });

  test('2. Curated 8 CSS Gradient Palette & Live Canvas Preview Adversarial Test', async ({ page }) => {
    await openApp(page, {
      hash: 'redbida',
    });

    // 2.1 Verify exact 8 presets defined in REDBIDA_GRADIENT_PALETTE
    const palette = await page.evaluate(() => window.REDBIDA_GRADIENT_PALETTE);
    expect(palette).toHaveLength(8);

    const expectedIds = [
      'royal-deep-blue',
      'midnight-emerald',
      'cyberpunk-neon',
      'golden-velvet',
      'obsidian-carbon',
      'crimson-elegance',
      'sapphire-blue',
      'ruby-luxury',
    ];
    palette.forEach((p, idx) => {
      expect(p.id).toBe(expectedIds[idx]);
      expect(p.css).toContain('linear-gradient');
      expect(p.css.endsWith(';')).toBe(false); // MUST NOT end with semicolon
    });

    // 2.2 Test clicking all 8 swatches in the DOM
    const swatches = page.locator('#redbida-preset-swatches .redbida-swatch');
    await expect(swatches).toHaveCount(8);

    for (let i = 0; i < 8; i++) {
      await swatches.nth(i).click();
      await expect(swatches.nth(i)).toHaveClass(/active/);
      
      // Check other swatches do not have active class
      for (let j = 0; j < 8; j++) {
        if (j !== i) {
          await expect(swatches.nth(j)).not.toHaveClass(/active/);
        }
      }

      // Check input value and live canvas preview
      const inputVal = await page.locator('#redbida-preset-bg').inputValue();
      expect(inputVal).toBe(palette[i].css);
      expect(inputVal.endsWith(';')).toBe(false);

      const canvasBg = await page.locator('#redbida-preset-bg-preview').evaluate(el => el.style.background);
      expect(canvasBg).toBeTruthy();
    }

    // 2.3 Test custom color / gradient with trailing semicolons and spaces
    const customBg = 'linear-gradient(90deg, #ff0055 0%, #0000ff 100%);;;   ';
    await page.locator('#redbida-preset-bg').fill(customBg);
    await page.locator('#redbida-preset-bg').dispatchEvent('input');

    const cleanPreviewBg = await page.locator('#redbida-preset-bg-preview').evaluate(el => el.style.background);
    expect(cleanPreviewBg).toBeTruthy();
    // Swatches should be deactivated when custom background doesn't match
    const activeSwatches = page.locator('#redbida-preset-swatches .redbida-swatch.active');
    await expect(activeSwatches).toHaveCount(0);
  });

  test('3. Smart Hashtag Generator with Complex Vietnamese Diacritics & Special Characters', async ({ page }) => {
    await openApp(page, {
      hash: 'redbida',
    });

    const testCases = [
      {
        input: 'CLB Bida Hoàng Gia Sài Gòn & Q.1',
        expectedClean: 'CLBBidaHoangGiaSaiGonQ1',
      },
      {
        input: 'Quán Bida 88 - Bắn Là Trúng! (Chi Nhánh 3)',
        expectedClean: 'QuanBida88BanLaTrungChiNhanh3',
      },
      {
        input: 'Đặng Huỳnh Ngọc Khánh @ Bida Đỉnh Cao',
        expectedClean: 'DangHuynhNgocKhanhBidaDinhCao',
      },
      {
        input: 'Café & Billiards 3D ✨ Thư Giãn',
        expectedClean: 'CafeBilliards3DThuGian',
      },
      {
        input: 'CLB Bida Triệu Đô (Út Tịch, Q. Tân Bình)',
        expectedClean: 'CLBBidaTrieuDoUtTichQTanBinh',
      },
      {
        input: 'Bida Lỗ & Bida 3 Băng Sài Gòn!',
        expectedClean: 'BidaLoBida3BangSaiGon',
      },
    ];

    for (const tc of testCases) {
      await page.locator('#redbida-preset-title').fill(tc.input);
      await page.locator('#redbida-preset-title').dispatchEvent('input');

      const expectedFullTag = `#${tc.expectedClean} #BILLIARDSlive #INUTlive #highlightsports`;
      
      const hashtagsPreview = page.locator('#redbida-preset-hashtags-preview');
      await expect(hashtagsPreview).toContainText(`#${tc.expectedClean}`);
      await expect(hashtagsPreview).toContainText('#BILLIARDSlive');
      await expect(hashtagsPreview).toContainText('#INUTlive');
      await expect(hashtagsPreview).toContainText('#highlightsports');

      const canvasHashtags = page.locator('#redbida-canvas-hashtags');
      await expect(canvasHashtags).toContainText(`#${tc.expectedClean}`);
    }

    // Test edge cases in JavaScript engine directly:
    const jsResults = await page.evaluate(() => {
      return {
        empty: window.generateSmartHashtags(''),
        spacesOnly: window.generateSmartHashtags('   '),
        specialOnly: window.generateSmartHashtags('!@#$%^&*()_+'),
        mixedAccents: window.generateSmartHashtags('ĂÂĐÊÔƠƯ àáảãạ ềếểễệ'),
        dAndD: window.removeVietnameseTones('đĐ'),
      };
    });

    expect(jsResults.empty).toBe('#BILLIARDSlive #INUTlive #highlightsports');
    expect(jsResults.spacesOnly).toBe('#BILLIARDSlive #INUTlive #highlightsports');
    expect(jsResults.specialOnly).toBe('#BILLIARDSlive #INUTlive #highlightsports');
    expect(jsResults.mixedAccents).toBe('#AADEOOUaaaaaeeeee #BILLIARDSlive #INUTlive #highlightsports');
    expect(jsResults.dAndD).toBe('dD');
  });

  test('4. Visual 20-Tab INI Editor 2-Way Sync & Edge Cases', async ({ page }) => {
    await openApp(page, {
      hash: 'redbida',
    });

    // 4.1 Verify exactly 20 tabs initialized
    const matrixButtons = page.locator('#redbida-tab-matrix-grid .redbida-matrix-btn');
    await expect(matrixButtons).toHaveCount(20);

    // 4.2 Test selecting various tables
    for (const tabNum of [1, 7, 13, 20]) {
      const pad = String(tabNum).padStart(2, '0');
      const tabId = `C${pad}`;
      await matrixButtons.nth(tabNum - 1).click();
      await expect(matrixButtons.nth(tabNum - 1)).toHaveClass(/active/);
      await expect(page.locator('#redbida-current-tab-title')).toContainText(tabId);
    }

    // 4.3 Test editing a table's individual fields
    await matrixButtons.nth(2).click(); // C03
    await page.locator('#redbida-tab-play-label').fill('Bàn 03 VIP');
    await page.locator('#redbida-tab-play-label').dispatchEvent('input');

    // Toggle to raw INI and verify C03 updated
    const toggleBtn = page.locator('#redbida-tab-view-toggle');
    await toggleBtn.click();
    let rawIni = await page.locator('#redbida-tab-raw-ini').inputValue();
    expect(rawIni).toContain('[C03]');
    expect(rawIni).toContain('vid_play_label=Bàn 03 VIP');

    // 4.4 Test editing Raw INI text and switching back to Visual
    const modifiedRawIni = rawIni.replace('vid_play_label=Bàn 03 VIP', 'vid_play_label=Bàn 03 Super VIP');
    await page.locator('#redbida-tab-raw-ini').fill(modifiedRawIni);
    await page.locator('#redbida-tab-raw-ini').dispatchEvent('input');

    await toggleBtn.click(); // Back to Visual
    await matrixButtons.nth(2).click(); // C03
    await expect(page.locator('#redbida-tab-play-label')).toHaveValue('Bàn 03 Super VIP');

    // 4.5 Test 1-Click Sync Title across all 20 tables
    await page.locator('#redbida-preset-title').fill('CLB Bida Hoàng Gia Sài Gòn');
    await page.locator('#redbida-tab-sync-title-btn').click();

    // Verify C01 through C20 have the synchronized title
    await toggleBtn.click();
    rawIni = await page.locator('#redbida-tab-raw-ini').inputValue();
    for (let i = 1; i <= 20; i++) {
      const pad = String(i).padStart(2, '0');
      expect(rawIni).toContain(`[C${pad}]`);
    }
    const matches = rawIni.match(/vid_play_label=CLB Bida Hoàng Gia Sài Gòn/g);
    expect(matches).toHaveLength(20);
  });
});
