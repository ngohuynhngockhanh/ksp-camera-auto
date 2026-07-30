const { test, expect } = require('@playwright/test');
const { openApp } = require('./fixtures');

const CAM = 'cameras/cam/cam-1';

test('tabs are routed and the live preview survives switching them', async ({ page }) => {
  await openApp(page, { hash: CAM + '/osd' });
  await expect(page.getByTestId('camera-detail')).toBeVisible();
  await expect(page.getByTestId('detail-channel-name')).toHaveValue('Cổng chính');

  // Start the stream on the OSD tab...
  await page.getByTestId('live-start').click();
  await expect(page.locator('#cd-live')).toBeVisible();
  await expect(page.getByTestId('live-stop')).toBeVisible();
  const src = await page.locator('#cd-live').getAttribute('src');

  // ...walk through every other tab; the stream must keep running. The old
  // modal stopped it on every tab change, which is what this page fixes.
  for (const tab of ['picture', 'video', 'audio', 'network', 'ptz', 'maint']) {
    await page.getByTestId('detail-tab-' + tab).click();
    await expect(page.locator('#ce-panel-' + tab)).toBeVisible();
    expect(decodeURIComponent(new URL(page.url()).hash)).toBe('#' + CAM + '/' + tab);
    await expect(page.locator('#cd-live')).toBeVisible();
  }
  expect(await page.locator('#cd-live').getAttribute('src')).toBe(src);

  await page.getByTestId('live-stop').click();
  await expect(page.locator('#cd-live')).toBeHidden();
  await expect(page.getByTestId('live-start')).toBeVisible();
});

test('a deep-linked tab opens directly', async ({ page }) => {
  await openApp(page, { hash: CAM + '/ptz' });
  await expect(page.locator('#ce-panel-ptz')).toBeVisible();
  await expect(page.locator('#ce-panel-osd')).toBeHidden();
});

test('unsupported tabs are disabled with a reason, not hidden', async ({ page }) => {
  // cam-2 is Hikvision: no picture/PTZ/audio/maintenance over DVRIP.
  await openApp(page, { hash: 'cameras/cam/cam-2/osd' });
  for (const tab of ['picture', 'ptz', 'audio', 'maint']) {
    const btn = page.getByTestId('detail-tab-' + tab);
    await expect(btn).toBeVisible();
    await expect(btn).toBeDisabled();
    await expect(btn).toHaveAttribute('title', /Chỉ Dahua/);
  }
  // Encode + network do work for Hikvision.
  await expect(page.getByTestId('detail-tab-video')).toBeEnabled();
  await expect(page.getByTestId('detail-tab-network')).toBeEnabled();
});

test('OSD lines save through /api/osd', async ({ page }) => {
  let osdBody = null;
  await openApp(page, {
    hash: CAM + '/osd',
    overrides: {
      '/api/osd': route => {
        osdBody = JSON.parse(route.request().postData());
        route.fulfill({ contentType: 'application/json', body: JSON.stringify({ ok: true, appliedLines: 2 }) });
      },
    },
  });

  await expect(page.locator('.ce-osd-line').first()).toHaveValue('KSP');
  await page.locator('.ce-osd-line').nth(1).fill('Cổng trước');
  await page.getByTestId('detail-save-osd').click();

  await expect.poll(() => osdBody && osdBody.lines[1]).toBe('Cổng trước');
  expect(osdBody.id).toBe('cam-1');
});

test('the network tab renders the single shared editor', async ({ page }) => {
  await openApp(page, { hash: CAM + '/network' });
  const body = page.getByTestId('detail-network-body');
  await expect(body).toBeVisible();
  await expect(page.locator('#net-ip')).toHaveValue('192.168.1.10');
  await expect(page.locator('#net-gw')).toHaveValue('192.168.1.1');
  // There is exactly one network editor in the document now — the modal copy
  // (ids ce-net-*) is gone.
  await expect(page.locator('#ce-net-body')).toHaveCount(0);
});

test('the maintenance tab reads storage and the reboot control', async ({ page }) => {
  await openApp(page, { hash: CAM + '/maint' });
  const body = page.getByTestId('detail-maint-body');
  await expect(body.locator('#maint-reboot-btn')).toBeVisible();
  await expect(body.locator('#maint-storage')).toContainText('SD-0');
  await expect(body.locator('#maint-storage')).toContainText('HEALTHY');
});

test('PTZ buttons drive start and stop commands', async ({ page }) => {
  const cmds = [];
  await openApp(page, {
    hash: CAM + '/ptz',
    overrides: {
      '/api/ptz': route => {
        cmds.push(JSON.parse(route.request().postData()));
        route.fulfill({ contentType: 'application/json', body: '{"ok":true}' });
      },
    },
  });

  const up = page.locator('.ptz-btn[data-ptz="Up"]');
  await up.hover();
  await page.mouse.down();
  await expect.poll(() => cmds.length).toBeGreaterThan(0);
  await page.mouse.up();
  await expect.poll(() => cmds.length).toBeGreaterThan(1);

  expect(cmds[0]).toMatchObject({ code: 'Up', start: true });
  expect(cmds[cmds.length - 1]).toMatchObject({ code: 'Up', start: false });
});

test('the NVR channel picker is populated from the device', async ({ page }) => {
  await openApp(page, { hash: 'cameras/cam/nvr-1/osd' });
  const sel = page.getByTestId('detail-channel');
  await expect(sel).toBeEnabled();
  await expect(sel.locator('option')).toHaveCount(2);
  await expect(sel.locator('option').first()).toContainText('Kênh 1');
});

test('a plain camera has no channel picker to fiddle with', async ({ page }) => {
  await openApp(page, { hash: CAM + '/osd' });
  await expect(page.getByTestId('detail-channel')).toBeDisabled();
});
