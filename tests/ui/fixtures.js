/* Shared Playwright fixtures.
 *
 * The UI is served by `python3 -m http.server` (see playwright.config.js), so
 * the Go backend never runs — every /api/* call must be mocked here or it 404s
 * and the feature under test silently renders an error state. mockApi() covers
 * the whole surface app.js + review.js touch; a spec that needs a different
 * answer passes an override rather than re-routing by hand.
 *
 * Response shapes mirror internal/server/api.go. Keep them in sync: a shape
 * drift here turns into a green test over a broken UI.
 */

const CAMERAS = [
  {
    id: 'cam-1', name: 'Cổng chính', host: '192.168.1.10', port: 37777,
    vendor: 'dahua', username: 'admin', password: 'secret-one', nvrId: 'nvr-1', nvrChannel: 1, noStorage: true,
  },
  {
    id: 'cam-2', name: 'Kho hàng', host: '192.168.1.11', port: 80,
    vendor: 'hikvision', username: 'operator', password: 'secret-two',
  },
  {
    id: 'nvr-1', name: 'Đầu ghi tầng 1', host: '192.168.1.253', port: 37777,
    vendor: 'dahua', username: 'admin', password: 'secret-nvr', isNvr: true, nvrWatchdog: true, nvrSyncTimeFromHost: true,
  },
];

// 1x1 transparent GIF — enough for <img> to fire `load` without a real JPEG.
const PIXEL = Buffer.from(
  'R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7', 'base64');

const STREAM_INFO = [
  {
    channel: 1, stream: 0, width: 1920, height: 1080, fps: 25,
    compression: 'H.264', profile: 'Main', audioCodec: 'G.711A',
    audioEnable: false, smartCodec: false, gop: 50, bitrateKbps: 2048,
    bitrateMode: 'CBR', name: 'Cổng chính', osdLines: ['KSP', 'Cổng chính'],
  },
];

const DEFAULTS = {
  '/api/config': { role: 'admin', maxReviewHours: 72, redbidaEnabled: true },
  '/api/cameras': CAMERAS,
  '/api/probe': { streams: STREAM_INFO, serialNumber: '8K01234PAZ56789', port: 37777 },
  '/api/fps-capability': { currentFps: 25, maxFps: 25, source: 'capability' },
  '/api/channel-info': {
    name: 'Cổng chính', osdLines: ['KSP', 'Cổng chính'],
    osdEnabled: [true, true], osdSupported: true,
  },
  '/api/channel-name': { ok: true },
  '/api/osd': { ok: true, appliedLines: 2 },
  '/api/picture': {
    color: { Brightness: 50, Contrast: 50, Saturation: 50, Hue: 50, Gamma: 50 },
    options: { Flip: false, Mirror: false, Rotate90: 0, BacklightMode: 'Off' },
    caps: {},
  },
  // dahua.NetworkConfig: interfaces keyed by name, device-cased field names.
  '/api/network': {
    defaultInterface: 'eth0',
    interfaces: {
      eth0: {
        DhcpEnable: false, IPAddress: '192.168.1.10', SubnetMask: '255.255.255.0',
        DefaultGateway: '192.168.1.1', DnsServers: ['8.8.8.8', '1.1.1.1'],
        PhysicalAddress: '3c:ef:8c:00:11:22', MTU: 1500,
      },
    },
  },
  '/api/wifi': { wlan0: { SSID: 'KSP-WIFI', Enable: true, EncryptionType: 'WPA2-PSK' } },
  '/api/wifi-scan': { devices: [{ ssid: 'KSP-WIFI', linkQuality: 78, authMode: 'WPA2-PSK' }] },
  '/api/storage': {
    devices: [{
      name: 'SD-0', state: 'Success',
      details: [{
        path: '/mnt/sd', type: 'ReadWrite',
        totalBytes: 128000000000, usedBytes: 42000000000,
        isError: false, isNeedFormat: false,
      }],
    }],
  },
  '/api/autoreboot': { enable: true, day: 0, hour: 2, minute: 30 },
  '/api/device-time': {
    currentTime: '2026-07-26 09:00:00', ntpEnable: true, ntpAddress: 'pool.ntp.org',
    ntpPort: 123, updatePeriod: 60, timeZone: 7, timeZoneDesc: 'GMT+07:00',
  },
  '/api/ptz': { ok: true },
  '/api/reboot': { ok: true, note: 'đã gửi lệnh khởi động lại.' },
  '/api/nvr/channels': { channels: [{ channel: 1, name: 'Kênh 1' }, { channel: 2, name: 'Kênh 2' }] },
  '/api/nvr/health': {
    id: 'nvr-1', status: 'healthy', reasons: [], reachable: true,
    watchdogEnabled: true, syncTimeFromHost: true, hostTimeTrusted: true,
    hostTime: '2026-07-26 23:40:00', nvrTime: '2026-07-26 23:39:58', clockDriftSeconds: -2,
    uptimeMinutes: 120, bootTime: '2026-07-26T21:40:00+07:00', storageHealthy: true,
    storageTotalBytes: 482000000000, storageUsedBytes: 8000000000, storageGrowing: true,
    channels: [{ channel: 0, name: 'Bãi xe', enabled: true, recordEnabled: true, timing24x7: true, latestEnd: '2026-07-26T23:39:00+07:00', staleMinutes: 1, recordedMinutes: 118, coveragePercent: 98.3 }],
    lastCheck: '2026-07-26T23:40:00+07:00', nextCheck: '2026-07-26T23:50:00+07:00',
  },
  '/api/nvr/health/check': { ok: true },
  '/api/nvr/watchdog': { ok: true },
  '/api/nvr/scan': {
    nvr: CAMERAS[2],
    rows: [
      {
        nvrChannel: 1, nvrCamIP: '192.168.1.10', nvrCamName: 'IPC-1', enable: true,
        suggestedCameraId: 'cam-1', suggestedCameraName: 'Cổng chính', noStorage: false,
      },
      {
        nvrChannel: 2, nvrCamIP: '192.168.1.11', nvrCamName: 'IPC-2', enable: true,
        suggestedCameraId: '', suggestedCameraName: '', noStorage: true,
      },
    ],
  },
  '/api/nvr/link': { ok: true, nvrId: 'nvr-1', linked: 2 },
  '/api/channel-names': { ok: true, names: { 'cam-1': 'Cổng chính' }, count: 1 },
  '/api/scan': { devices: [] },
  '/api/scan/try-password': { ok: true },
  '/api/import': { added: 0, skipped: 0 },
  '/api/recordings': { recordings: [] },
  '/api/export-progress': { done: 0, total: 0, phase: '', error: '' },
  '/api/playback-token': { token: 'tok', exp: '2026-07-26T12:00:00Z' },
  '/api/redbida/catalog': {
    keys: [
      { key: 'logo_header', label: 'logo header', group: 'Branding / Logo', risk: 'editable', valueType: 'image', editable: true, secret: false },
      { key: 'logo_livestream', label: 'logo livestream', group: 'Branding / Logo', risk: 'editable', valueType: 'image', editable: true, secret: false },
      { key: 'show_toolbar', label: 'show toolbar', group: 'UI / Display', risk: 'editable', valueType: 'boolean', editable: true, secret: false },
      { key: 'mqtt_password', label: 'mqtt password', group: 'Security / Credentials', risk: 'read-only-protected', valueType: 'string', editable: false, secret: true },
      { key: 'button_reboot', label: 'button reboot', group: 'Schedule / Maintenance', risk: 'confirm-required', valueType: 'boolean', editable: true, secret: false },
    ],
  },
  '/api/redbida/refresh': {
    values: [
      { key: 'logo_header', value: 'https://example.test/logo.png', exists: true, meta: { key: 'logo_header', label: 'logo header', group: 'Branding / Logo', risk: 'editable', valueType: 'image', editable: true, secret: false } },
      { key: 'logo_livestream', value: '', exists: true, meta: { key: 'logo_livestream', label: 'logo livestream', group: 'Branding / Logo', risk: 'editable', valueType: 'image', editable: true, secret: false } },
      { key: 'show_toolbar', value: true, exists: true, meta: { key: 'show_toolbar', label: 'show toolbar', group: 'UI / Display', risk: 'editable', valueType: 'boolean', editable: true, secret: false } },
      { key: 'mqtt_password', value: '********', exists: true, meta: { key: 'mqtt_password', label: 'mqtt password', group: 'Security / Credentials', risk: 'read-only-protected', valueType: 'string', editable: false, secret: true } },
      { key: 'button_reboot', value: false, exists: true, meta: { key: 'button_reboot', label: 'button reboot', group: 'Schedule / Maintenance', risk: 'confirm-required', valueType: 'boolean', editable: true, secret: false } },
    ],
    refreshedAt: '2026-08-24T08:00:00Z',
  },
  '/api/redbida/apply': {
    results: [{ key: 'logo_header', applied: true, changed: true, oldValue: 'old', newValue: 'new', meta: { key: 'logo_header', risk: 'editable' } }],
    appliedAt: '2026-08-24T08:00:01Z',
  },
  '/api/redbida/time-status': {
    hostTime: '2026-08-24 15:00:00', ntpSynchronized: true, nodeRedReadOnly: true, driftThresholdSeconds: 60,
  },
};

// ndjson builds the "data: {...}\n\n" frame stream that /api/apply and
// /api/password emit and streamPost() consumes.
function ndjson(events) {
  return events.map(e => 'data: ' + JSON.stringify(e) + '\n\n').join('');
}

// applyStream is the happy-path frame sequence for N devices.
function applyStream(devices) {
  const events = [];
  devices.forEach((d, i) => {
    events.push({ type: 'device_start', deviceId: d.id, name: d.name, host: d.host, index: i + 1, total: devices.length });
    events.push({ type: 'step', deviceId: d.id, step: 'encode', detail: 'main: H.264 1920x1080', ok: true });
    events.push({ type: 'device_done', deviceId: d.id, name: d.name, host: d.host, ok: true });
  });
  events.push({ type: 'done' });
  return ndjson(events);
}

// mockApi intercepts every endpoint the UI calls. `overrides` maps a pathname
// (e.g. '/api/storage') to either a JSON-serialisable body or a function
// (route, url) => void for full control (status codes, streams, delays).
async function mockApi(page, overrides = {}) {
  const table = Object.assign({}, DEFAULTS, overrides);

  await page.route('**/api/**', route => {
    const url = new URL(route.request().url());
    const path = url.pathname;

    // Binary endpoints — snapshot returns a JPEG, live is an MJPEG <img> src.
    if (path === '/api/snapshot' || path === '/api/live') {
      return route.fulfill({ status: 200, contentType: 'image/gif', body: PIXEL });
    }
    if (path === '/api/playback') {
      return route.fulfill({ status: 200, contentType: 'video/mp4', body: Buffer.alloc(0) });
    }
    // Streaming endpoints.
    if (path === '/api/apply' || path === '/api/password') {
      const custom = table[path];
      if (typeof custom === 'function') return custom(route, url);
      const ids = JSON.parse(route.request().postData() || '{}').deviceIds || [];
      const devices = ids.map(id => CAMERAS.find(c => c.id === id) || { id, name: id, host: id });
      return route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: typeof custom === 'string' ? custom : applyStream(devices),
      });
    }

    const entry = table[path];
    if (typeof entry === 'function') return entry(route, url);
    if (entry === undefined) {
      // Loud on purpose: an unmocked endpoint is a fixture gap, not a UI bug.
      return route.fulfill({
        status: 501,
        contentType: 'application/json',
        body: JSON.stringify({ error: `fixture thiếu mock cho ${path}` }),
      });
    }
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(entry),
    });
  });
}

// openApp mocks the API then loads the SPA at `hash` (default: dashboard) and
// waits for the camera list to have rendered, so specs don't race the boot
// fetch of /api/config + /api/cameras.
async function openApp(page, { hash = '', overrides = {} } = {}) {
  await mockApi(page, overrides);
  await page.goto('/index.html' + (hash ? '#' + hash : ''));
  await page.waitForFunction(() => window.__kspReady === true);
}

module.exports = { CAMERAS, STREAM_INFO, DEFAULTS, PIXEL, mockApi, openApp, ndjson, applyStream };
