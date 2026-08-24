const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test.describe('RedBida Milestone 3 Knowledge Hub & Preset Generator E2E', () => {
  test('1-Click Preset Generator populates drafts, updates metrics, renders diff card, and allows instant apply', async ({ page }) => {
    let submitted;
    await openApp(page, {
      hash: 'redbida',
      overrides: {
        '/api/redbida/catalog': {
          keys: [
            { key: 'ui_title', label: 'Tên quán', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true, secret: false },
            { key: 'company_name', label: 'Tên công ty', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true, secret: false },
            { key: 'ui_bg', label: 'Màu nền CSS', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true, secret: false },
            { key: 'custom_hashtags', label: 'Hashtag', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true, secret: false },
            { key: 'ui_tabs_links', label: '20 tab INI', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true, secret: false },
            { key: 'camera_count', label: 'Số camera', group: 'Video & Streaming', risk: 'editable', valueType: 'number', editable: true, secret: false },
            { key: 'toolbar_show_count', label: 'Số camera toolbar', group: 'Video & Streaming', risk: 'editable', valueType: 'number', editable: true, secret: false },
            { key: 'video_config', label: 'Video config', group: 'Video & Streaming', risk: 'editable', valueType: 'string', editable: true, secret: false },
            { key: 'hls_using_go2rtc', label: 'Go2RTC HLS', group: 'Video & Streaming', risk: 'editable', valueType: 'boolean', editable: true, secret: false },
            { key: 'hls_using_go2rtc_livestream', label: 'Go2RTC Live', group: 'Video & Streaming', risk: 'editable', valueType: 'boolean', editable: true, secret: false },
            { key: 'hls_using_go2rtc_tiktok', label: 'Go2RTC TikTok', group: 'Video & Streaming', risk: 'editable', valueType: 'boolean', editable: true, secret: false },
            { key: 'ui_scoreboard', label: 'Bảng điểm', group: 'Branding / Logo', risk: 'editable', valueType: 'boolean', editable: true, secret: false },
            { key: 'logo_header', label: 'Logo header', group: 'Branding / Logo', risk: 'editable', valueType: 'image', editable: true, secret: false },
            { key: 'logo_header_text', label: 'Logo text', group: 'Branding / Logo', risk: 'editable', valueType: 'string', editable: true, secret: false },
            { key: 'button_generate_go2rtc_stream', label: 'Trigger Go2RTC', group: 'Video & Streaming', risk: 'editable', valueType: 'boolean', editable: true, secret: false },
          ],
        },
        '/api/redbida/refresh': {
          values: [
            { key: 'ui_title', value: 'Old Title', exists: true },
            { key: 'camera_count', value: 4, exists: true },
          ],
        },
        '/api/redbida/apply': async (route) => {
          submitted = JSON.parse(route.request().postData() || '{}');
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              results: Object.keys(submitted.changes || {}).map(k => ({
                key: k,
                applied: true,
                verified: true,
                changed: true,
                newValue: submitted.changes[k],
              })),
            }),
          });
        },
      },
    });

    // Verify Knowledge Hub & Preset Panel are visible
    await expect(page.locator('#redbida-knowledge-hub')).toBeVisible();
    await expect(page.locator('#redbida-preset-panel')).toBeVisible();

    // Fill Preset Form
    await page.locator('#redbida-preset-title').fill('Bida Hoàng Gia CS2');
    await page.locator('#redbida-preset-count').fill('12');
    
    // Select second gradient swatch (Cyber Emerald)
    const swatch2 = page.locator('#redbida-preset-swatches .redbida-swatch').nth(1);
    await swatch2.click();
    await expect(swatch2).toHaveClass(/active/);
    await expect(page.locator('#redbida-preset-bg')).toHaveValue(/linear-gradient/);

    // Click Generate Preset Button
    await page.locator('#redbida-preset-gen-btn').click();

    // Verify Diff card appeared
    const diffCard = page.locator('#redbida-preset-diff');
    await expect(diffCard).toBeVisible();
    await expect(diffCard).toContainText('15 tham số');
    await expect(diffCard).toContainText('#BidaHoangGiaCS2 #BILLIARDSlive #INUTlive #highlightsports');

    // Verify draft count updated in metric card
    await expect(page.locator('#redbida-draft-count')).toHaveText('15');

    // Verify table rows marked dirty
    await expect(page.locator('[data-red-row="ui_title"]')).toHaveClass(/redbida-dirty/);
    await expect(page.locator('[data-red-row="custom_hashtags"]')).toHaveClass(/redbida-dirty/);

    // Submit via Diff Card "Áp Dụng Ngay"
    await page.locator('#redbida-preset-submit-now').click();

    // Assert submitted payload to apply API
    await expect.poll(() => submitted && submitted.changes).toBeTruthy();
    expect(submitted.changes.ui_title).toBe('Bida Hoàng Gia CS2');
    expect(submitted.changes.company_name).toBe('Bida Hoàng Gia CS2');
    expect(submitted.changes.camera_count).toBe(12);
    expect(submitted.changes.toolbar_show_count).toBe(12);
    expect(submitted.changes.custom_hashtags).toBe('#BidaHoangGiaCS2 #BILLIARDSlive #INUTlive #highlightsports');
    expect(submitted.changes.video_config).toBe('range=72');
    expect(submitted.changes.hls_using_go2rtc).toBe(true);
    expect(submitted.changes.button_generate_go2rtc_stream).toBe(true);
    expect(submitted.changes.ui_tabs_links).toContain('vid_play_label=Bida Hoàng Gia CS2');
    expect(submitted.changes.ui_tabs_links).toContain('[C01]');
    expect(submitted.changes.ui_tabs_links).toContain('[C20]');

    // Draft count should reset to 0 after full verify
    await expect(page.locator('#redbida-draft-count')).toHaveText('0');
  });

  test('4-Pillar Hub filter buttons and collapsible toggles function properly', async ({ page }) => {
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
        '/api/redbida/refresh': {
          values: [
            { key: 'ui_title', value: 'CX King', exists: true },
            { key: 'camera_count', value: 8, exists: true },
            { key: 'shinobi_token', value: 'tok123', exists: true },
          ],
        },
      },
    });

    // Test Pillar 1 Filter Button (Branding)
    await page.locator('.redbida-pillar-card.pillar-branding .redbida-pillar-btn').click();
    await expect(page.locator('#redbida-group')).toHaveValue('Branding / Logo');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(1);
    await expect(page.locator('[data-red-row="ui_title"]')).toBeVisible();

    // Test Pillar 2 Filter Button (Streaming)
    await page.locator('.redbida-pillar-card.pillar-streaming .redbida-pillar-btn').click();
    await expect(page.locator('#redbida-group')).toHaveValue('Video & Streaming');
    await expect(page.locator('#redbida-tbody tr')).toHaveCount(1);
    await expect(page.locator('[data-red-row="camera_count"]')).toBeVisible();

    // Test Reset Preset Button
    await page.locator('#redbida-preset-reset-btn').click();
    await expect(page.locator('#redbida-preset-title')).toHaveValue('CX King Luxury');
    await expect(page.locator('#redbida-preset-count')).toHaveValue('8');

    // Test Collapsible Toggles
    const presetPanel = page.locator('#redbida-preset-panel');
    await page.locator('#redbida-toggle-preset').click();
    await expect(presetPanel).toBeHidden();
    await page.locator('#redbida-toggle-preset').click();
    await expect(presetPanel).toBeVisible();

    const hubPanel = page.locator('#redbida-knowledge-hub');
    await page.locator('#redbida-toggle-hub').click();
    await expect(hubPanel).toBeHidden();
    await page.locator('#redbida-toggle-hub').click();
    await expect(hubPanel).toBeVisible();
  });
});
