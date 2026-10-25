const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

test('the summary bar says what is about to change', async ({ page }) => {
  await openApp(page, { hash: 'cameras/bulk' });
  const summary = page.getByTestId('bulk-summary');
  await expect(summary).toContainText('chưa bật thiết lập nào');

  await page.locator('#p-codec-enable').check();
  await expect(summary).toContainText('Codec H.264');
  await expect(page.locator('#bulk-summary-empty')).toBeHidden();

  await page.locator('#p-bitrate-enable').check();
  await page.locator('#p-bitrate-value').fill('4096');
  await expect(summary).toContainText('Bitrate 4096 Kbps');

  await page.locator('#p-codec-enable').uncheck();
  await expect(summary).not.toContainText('Codec');
  await expect(summary).toContainText('Bitrate');
});

test('enabling a setting reveals only its own fields', async ({ page }) => {
  await openApp(page, { hash: 'cameras/bulk' });
  await expect(page.locator('#p-res-fields')).toBeHidden();
  await page.locator('#p-res-enable').check();
  await expect(page.locator('#p-res-fields')).toBeVisible();
  await expect(page.locator('#p-gop-fields')).toBeHidden();
});

test('selection survives moving between task tabs', async ({ page }) => {
  await openApp(page, { hash: 'cameras/list' });
  await page.locator('.cam-cb').first().check();

  await page.getByTestId('task-tab-bulk').click();
  await expect(page.locator('#bulk-selected-count')).toContainText('1 camera');
  await expect(page.locator('.bulk-cam-cb[value="cam-1"]')).toBeChecked();

  await page.getByTestId('task-tab-list').click();
  await expect(page.locator('.cam-cb').first()).toBeChecked();
});

test('apply streams progress and fills the results tab', async ({ page }) => {
  await openApp(page, { hash: 'cameras/list' });
  await page.locator('.cam-cb').first().check();
  await page.getByTestId('task-tab-bulk').click();
  await page.locator('#p-codec-enable').check();
  await page.getByTestId('bulk-apply').click();

  await page.getByTestId('task-tab-results').click();
  await expect(page.locator('#apply-log')).toContainText('HOÀN TẤT');
  const rows = page.getByTestId('result-list').locator('tr');
  await expect(rows).toHaveCount(1);
  await expect(rows.first()).toContainText('Cổng chính');
});

test('the password change is folded away behind a disclosure', async ({ page }) => {
  await openApp(page, { hash: 'cameras/bulk' });
  const zone = page.getByTestId('bulk-password');
  await expect(zone.getByTestId('bulk-password-apply')).toBeHidden();
  await zone.locator('summary').click();
  await expect(zone.getByTestId('bulk-password-apply')).toBeVisible();
});

test('golden template 1-click populates H.265 1080p GOP 50 Bitrate 2048 and AAC audio', async ({ page }) => {
  await openApp(page, { hash: 'cameras/bulk' });
  const goldenBtn = page.getByTestId('bulk-golden-template');
  await expect(goldenBtn).toBeVisible();
  await goldenBtn.click();

  // Codec
  await expect(page.locator('#p-codec-enable')).toBeChecked();
  await expect(page.locator('#p-codec-value')).toHaveValue('H.265');

  // Res
  await expect(page.locator('#p-res-enable')).toBeChecked();
  await expect(page.locator('#p-width')).toHaveValue('1920');
  await expect(page.locator('#p-height')).toHaveValue('1080');

  // GOP
  await expect(page.locator('#p-gop-enable')).toBeChecked();
  await expect(page.locator('#p-gop-value')).toHaveValue('50');

  // Bitrate
  await expect(page.locator('#p-bitrate-enable')).toBeChecked();
  await expect(page.locator('#p-bitrate-value')).toHaveValue('2048');
  await expect(page.locator('#p-bitrate-mode')).toHaveValue('CBR');

  // Audio
  await expect(page.locator('#p-audio-enable')).toBeChecked();

  // Summary chips
  const summary = page.getByTestId('bulk-summary');
  await expect(summary).toContainText('Codec H.265');
  await expect(summary).toContainText('1920x1080');
  await expect(summary).toContainText('GOP 50');
  await expect(summary).toContainText('Bitrate 2048 Kbps CBR');
  await expect(summary).toContainText('AAC');
});

test('firmware upgrade/downgrade disclosure inspects package, checks HWID preflight, and gates bootloader write', async ({ page }) => {
  await openApp(page, { hash: 'cameras/list' });
  await page.locator('.cam-cb').first().check();
  await page.getByTestId('task-tab-bulk').click();

  const fwZone = page.getByTestId('bulk-firmware');
  await expect(fwZone.getByTestId('fw-inspect-btn')).toBeHidden();
  await fwZone.locator('summary').click();
  await expect(fwZone.getByTestId('fw-inspect-btn')).toBeVisible();

  await fwZone.getByTestId('fw-file-input').setInputFiles({
    name: 'DH_IPC-HX3XXX_20240402.bin',
    mimeType: 'application/octet-stream',
    buffer: Buffer.from('DH\x03\x04mockfirmware'),
  });

  await expect(page.locator('#fw-sum-version')).toContainText('2.860.0000000.3.R');
  await expect(fwZone.getByTestId('fw-dryrun-btn')).toBeEnabled();
  // Bootloader package requires explicit checkbox confirmation before execute is enabled
  await expect(fwZone.getByTestId('fw-execute-btn')).toBeDisabled();

  await fwZone.getByTestId('fw-preflight-btn').click();
  await expect(fwZone.getByTestId('fw-preflight-tbody')).toContainText('KHỚP HWID');
  await expect(fwZone.getByTestId('fw-preflight-tbody')).toContainText('AD0786FPAG01146');

  await fwZone.getByTestId('fw-confirm-bootloader').check();
  await expect(fwZone.getByTestId('fw-execute-btn')).toBeEnabled();

  await fwZone.getByTestId('fw-dryrun-btn').click();
  await expect(page.locator('#apply-log')).toContainText('Kiểm chứng truyền tải 100% thành công');
});

