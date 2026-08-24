/* ksp-camera-auto — dashboard logic. Vanilla JS, no build step, no deps. */

/* ---------- inline icons (Tabler-style, stroke 2, currentColor) ---------- */

const ICONS = {
  home: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 10.5 12 4l8 6.5"/><path d="M6 9.5V20h12V9.5"/><path d="M10 20v-6h4v6"/></svg>',
  radar: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 12l6-6"/><path d="M12 3a9 9 0 1 0 9 9"/><path d="M12 7a5 5 0 1 0 5 5"/></svg>',
  camera: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="6" width="12" height="12" rx="2"/><path d="M15 10l6-3v10l-6-3"/></svg>',
  upload: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21V9"/><path d="M7 14l5-5 5 5"/><path d="M5 21h14"/></svg>',
  dots: '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" stroke="none"><circle cx="5" cy="12" r="1.6"/><circle cx="12" cy="12" r="1.6"/><circle cx="19" cy="12" r="1.6"/></svg>',
  sun: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>',
  moon: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8z"/></svg>',
  logout: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="M16 17l5-5-5-5"/><path d="M21 12H9"/></svg>',
  edit: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>',
  reload: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-3-6.7"/><path d="M21 3v6h-6"/></svg>',
  help: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M9.5 9.2a2.6 2.6 0 0 1 5.1.8c0 1.6-2.6 2.2-2.6 3.5"/><path d="M12 17h.01"/></svg>',
  video: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 8-6 4 6 4V8Z"/><rect width="14" height="12" x="2" y="6" rx="2"/></svg>',
  settings: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-1.8 1.8-.06-.06a1.7 1.7 0 0 0-1.88-.34 1.7 1.7 0 0 0-1.03 1.56V20h-2.55v-.1a1.7 1.7 0 0 0-1.03-1.56 1.7 1.7 0 0 0-1.88.34l-.06.06-1.8-1.8.06-.06A1.7 1.7 0 0 0 8.1 15a1.7 1.7 0 0 0-1.56-1.03H6v-2.55h.1A1.7 1.7 0 0 0 7.66 10a1.7 1.7 0 0 0-.34-1.88l-.06-.06 1.8-1.8.06.06A1.7 1.7 0 0 0 11 6.1a1.7 1.7 0 0 0 1.03-1.56V4h2.55v.1A1.7 1.7 0 0 0 15.6 5.66a1.7 1.7 0 0 0 1.88-.34l.06-.06 1.8 1.8-.06.06A1.7 1.7 0 0 0 18.9 9a1.7 1.7 0 0 0 1.56 1.03h.1v2.55h-.1A1.7 1.7 0 0 0 18.9 15Z"/></svg>',
};

// Nav config shared by sidebar / bottom-nav / drawer. Kho camera / Chỉnh
// hàng loạt / Kết quả live on one page (#cameras) so users don't have to
// jump tabs mid-workflow.
const NAV_ITEMS = [
  { hash: 'dashboard', label: 'Tổng quan', short: 'Tổng quan', icon: ICONS.home, bottom: true },
  { hash: 'scan', label: 'Quét mạng', short: 'Quét', icon: ICONS.radar, bottom: true },
  { hash: 'cameras', label: 'Kho camera', short: 'Camera', icon: ICONS.camera, bottom: true },
  { hash: 'review', label: 'Xem lại', short: 'Xem lại', icon: ICONS.radar, bottom: true },
  { hash: 'shinobi', label: 'Shinobi NVR', short: 'Shinobi', icon: ICONS.video, bottom: false },
  { hash: 'redbida', label: 'RedBida / OTA', short: 'RedBida', icon: ICONS.settings, bottom: false },
  // bottom: false — mobile bottom nav stays at 4 items + Menu so it doesn't
  // get crowded; import (an occasional setup action, unlike the other four
  // which are used every visit) is reachable from the sidebar and the drawer.
  { hash: 'import', label: 'Nhập Shinobi', short: 'Nhập', icon: ICONS.upload, bottom: false },
  { hash: 'help', label: 'Trợ giúp', short: 'Trợ giúp', icon: ICONS.help, bottom: false },
];
// Old bookmarks/links to the now-merged or renamed tabs still land somewhere
// sensible. 'cameras/devices' was split: per-camera config moved to the camera
// detail page, the NVR linking stayed behind as 'cameras/nvr'.
const HASH_ALIASES = {
  bulk: 'cameras/bulk',
  results: 'cameras/results',
  'cameras/devices': 'cameras/nvr',
};
const CAMERA_TASKS = ['list', 'bulk', 'nvr', 'results'];
// Tabs of the camera detail page (#cameras/cam/<encodedId>/<tab>).
const DETAIL_TABS = ['osd', 'picture', 'video', 'audio', 'network', 'ptz', 'maint'];

// streamPost POSTs a JSON body and consumes the "data: <event>\n\n" SSE
// stream, rendering each event to the live log + progress bar. Used by
// apply + password.
async function streamPost(url, body) {
  const resp = await fetch(url, { method: 'POST', headers: jsonHeaders, body: JSON.stringify(body) });
  if (resp.status === 401) {
    location.href = '/login';
    throw new Error('unauthorized');
  }
  if (!resp.ok || !resp.body) {
    const text = await resp.text().catch(() => '');
    let msg = text || resp.statusText;
    try { const j = JSON.parse(text); if (j && j.error) msg = j.error; } catch (e) { /* not JSON */ }
    throw new Error(msg);
  }

  const results = [];
  const byId = {};
  const reader = resp.body.getReader();
  const decoder = new TextDecoder();
  let buf = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    const parts = buf.split('\n\n');
    buf = parts.pop(); // last chunk may be incomplete; keep it for next read
    for (const part of parts) {
      const line = part.split('\n').find(l => l.startsWith('data: '));
      if (!line) continue;
      let ev;
      try { ev = JSON.parse(line.slice(6)); } catch (e) { continue; }
      handleEvent(ev, results, byId);
    }
  }
  return results;
}

function handleEvent(ev, results, byId) {
  if (ev.type === 'device_start') {
    setProgress(ev.index, ev.total, `Đang xử lý ${ev.index}/${ev.total}: ${ev.name || ev.deviceId}`);
    logLine(`▶ [${ev.index}/${ev.total}] ${escapeHtml(ev.name || ev.deviceId)} (${escapeHtml(ev.host || '')}) — bắt đầu`, 'l-info');
    const r = { deviceId: ev.deviceId, name: ev.name, host: ev.host, ok: true, steps: [] };
    byId[ev.deviceId] = r;
    results.push(r);
  } else if (ev.type === 'step') {
    const cls = ev.ok ? 'l-ok' : 'l-err';
    const text = ev.ok
      ? escapeHtml(ev.detail || '')
      : '✗ ' + escapeHtml(ev.detail || ev.err || '') + (ev.detail && ev.err ? ` — ${escapeHtml(ev.err)}` : '');
    logLine(`&nbsp;&nbsp;&nbsp;• ${escapeHtml(ev.step)}: ${text}`, cls);
    const r = byId[ev.deviceId];
    if (r) r.steps.push({ step: ev.step, detail: ev.detail, ok: ev.ok, err: ev.err });
  } else if (ev.type === 'device_done') {
    const r = byId[ev.deviceId] || { deviceId: ev.deviceId, name: ev.name, host: ev.host, steps: [] };
    r.ok = ev.ok;
    r.err = ev.err;
    if (ev.ok) {
      logLine(`✓ ${escapeHtml(ev.name || ev.deviceId)}: HOÀN TẤT`, 'l-ok');
    } else {
      logLine(`✗ ${escapeHtml(ev.name || ev.deviceId)}: LỖI — ${escapeHtml(ev.err || '')}`, 'l-err');
    }
  } else if (ev.type === 'done') {
    logLine('— Xong tất cả —', 'l-info');
    setProgress(null);
  }
}

/* ---------- progress bar + live log ---------- */

// Both long-running flows share ui-core's progressBar; they used to carry two
// copies of the same three-element bookkeeping.
const applyProgress = progressBar('apply-progress');
const setProgress = (index, total, label) => applyProgress.set(index, total, label);

function logTime() {
  const d = new Date();
  const p = n => String(n).padStart(2, '0');
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}
function logLine(text, cls) {
  const box = document.getElementById('apply-log');
  const line = document.createElement('div');
  line.innerHTML = `<span class="l-time">[${logTime()}]</span> <span class="${cls || ''}">${text}</span>`;
  box.appendChild(line);
  box.scrollTop = box.scrollHeight;
}
function clearLog() { document.getElementById('apply-log').innerHTML = ''; }

/* ---------- state ---------- */

let cameras = [];
const selectedCameraSet = new Set();
const probeCache = {}; // id -> streamInfo[]
let scanResults = [];
let lastRun = null; // { type, total, ok, fail, time }

function fmtStreamInfo(list) {
  if (!list || !list.length) return '';
  return list.map(s => {
    const ch = (s.channel ? 'K' + s.channel + ' ' : '');
    const label = ['main', 'sub1', 'sub2'][s.stream] || ('s' + s.stream);
    const name = s.name ? ` "${escapeHtml(s.name)}"` : '';
    const audio = s.audioEnable ? (s.audioCodec || 'on') : 'tắt';
    const codec = s.compression ? (s.compression + (s.profile ? '/' + s.profile : '')) : '';
    const fps = (s.fps > 0 ? s.fps + 'fps' : 'fps theo nguồn');
    const gop = (s.gop > 0 ? ' · GOP ' + s.gop : '');
    const bitrate = (s.bitrateKbps > 0 ? ' · ' + s.bitrateKbps + 'Kbps' + (s.bitrateMode ? ' ' + s.bitrateMode : '') : '');
    const osd = (s.osdLines || []).filter(Boolean);
    const osdLine = osd.length ? `<br>&nbsp;&nbsp;OSD: ${osd.map(escapeHtml).join(' / ')}` : '';
    return `${ch}${label}${name}: ${s.width}x${s.height} ${codec} · ${fps} · audio ${audio} · smart ${s.smartCodec ? 'bật' : 'tắt'}${gop}${bitrate}${osdLine}`;
  }).join('<br>');
}

function rememberProbeResult(id, payload) {
  const result = Array.isArray(payload) ? { streams: payload } : (payload || {});
  const streams = Array.isArray(result.streams) ? result.streams : [];
  probeCache[id] = streams;
  cameras = cameras.map(c => c.id === id ? Object.assign({}, c, {
    port: result.port || c.port,
    serialNumber: result.serialNumber || c.serialNumber || '',
  }) : c);
  return streams;
}

function cameraSerialHtml(c) {
  if (c.vendor !== 'dahua') return '<span class="muted">–</span>';
  if (!c.serialNumber) return '<span class="muted">chưa dò SN</span>';
  return `<div class="camera-serial"><code>${escapeHtml(c.serialNumber)}</code>` +
    `<div class="camera-serial-qr" data-testid="camera-serial-qr" data-serial="${escapeHtml(c.serialNumber)}"></div></div>`;
}

function renderCameraSerialQRCodes(root) {
  if (typeof QRCode === 'undefined') return;
  root.querySelectorAll('.camera-serial-qr[data-serial]').forEach(el => {
    el.innerHTML = '';
    new QRCode(el, {
      text: el.dataset.serial,
      width: 88,
      height: 88,
      correctLevel: QRCode.CorrectLevel.M,
    });
  });
}

/* ---------- routing ---------- */

function currentHash() {
  // Keep only the view part: "#help/wifi" routes to the help view, and
  // help.js reads the article id from the full location.hash itself.
  let raw = (location.hash || '#dashboard').slice(1);
  if (HASH_ALIASES[raw]) raw = HASH_ALIASES[raw];
  const h = raw.split('/')[0];
  return NAV_ITEMS.some(n => n.hash === h) ? h : 'dashboard';
}

// cameraHashParts returns the alias-resolved segments after '#cameras/'.
function cameraHashParts() {
  let raw = (location.hash || '').slice(1);
  if (HASH_ALIASES[raw]) raw = HASH_ALIASES[raw];
  const parts = raw.split('/');
  return parts[0] === 'cameras' ? parts.slice(1) : [];
}

// currentDetail parses '#cameras/cam/<encodedId>/<tab>' into {id, tab}, or
// null when the hash addresses a plain task tab instead. Camera ids contain a
// colon (host:port) so they are percent-encoded, same as review.js does.
function currentDetail() {
  const parts = cameraHashParts();
  if (parts[0] !== 'cam' || !parts[1]) return null;
  let id;
  try { id = decodeURIComponent(parts[1]); } catch (e) { id = parts[1]; }
  const tab = DETAIL_TABS.includes(parts[2]) ? parts[2] : 'osd';
  return { id, tab };
}

function currentCameraTask() {
  const parts = cameraHashParts();
  return CAMERA_TASKS.includes(parts[0]) ? parts[0] : 'list';
}

function setCameraTask(task) {
  location.hash = '#cameras/' + (CAMERA_TASKS.includes(task) ? task : 'list');
}

function cameraDetailHash(id, tab) {
  return '#cameras/cam/' + encodeURIComponent(id) + '/' + (DETAIL_TABS.includes(tab) ? tab : 'osd');
}

function gotoCameraDetail(id, tab) { location.hash = cameraDetailHash(id, tab); }

function renderCameraTask() {
  const detail = currentDetail();
  const task = currentCameraTask();
  // Detail mode takes over the whole camera workspace: task panels, the tab
  // strip and the page heading all step aside so one camera fills the screen.
  document.querySelectorAll('[data-camera-panel]').forEach(el => {
    el.hidden = !!detail || el.dataset.cameraPanel !== task;
  });
  document.getElementById('camera-detail').hidden = !detail;
  document.getElementById('camera-task-tabs').hidden = !!detail;
  document.querySelector('#view-cameras .page-heading').hidden = !!detail;
  document.querySelectorAll('[data-camera-task]').forEach(el => {
    const active = !detail && el.dataset.cameraTask === task;
    el.classList.toggle('active', active);
    if (active) el.setAttribute('aria-current', 'page');
    else el.removeAttribute('aria-current');
  });
  if (detail) { openCameraDetail(detail); return; }
  closeCameraDetail();
  if (task === 'bulk') renderBulkSelection();
  if (task === 'nvr') renderNvrList();
}

function setRoute() {
  if (resolveLegacyHash()) return;
  let hash = currentHash();
  // A viewer is locked to the review view — bounce any other route back.
  if (appRole === 'viewer' && hash !== 'review') { location.hash = '#review'; return; }
  if (!appRedbidaEnabled && hash === 'redbida') { location.hash = '#dashboard'; return; }
  document.querySelectorAll('.view').forEach(v => v.classList.toggle('active', v.dataset.view === hash));
  const item = NAV_ITEMS.find(n => n.hash === hash);
  document.getElementById('view-title').textContent = item ? item.label : '';
  document.querySelectorAll('[data-nav-hash]').forEach(el => {
    el.classList.toggle('active', el.dataset.navHash === hash);
  });
  closeDrawer();
  if (hash === 'cameras') renderCameraTask();
  if (hash === 'dashboard') renderDashboard();
  if (hash === 'shinobi') renderShinobiView();
  if (hash === 'redbida') {
    if (window.redbidaOnShow) window.redbidaOnShow();
    else window.addEventListener('load', () => { if (window.redbidaOnShow) window.redbidaOnShow(); }, { once: true });
  }
  if (hash === 'review') {
    // reviewOnShow may not exist yet if review.js is still loading (a viewer is
    // forced here during init, before the later <script> runs) — retry on load.
    if (window.reviewOnShow) window.reviewOnShow();
    else window.addEventListener('load', () => { if (window.reviewOnShow) window.reviewOnShow(); }, { once: true });
  }
}

function goto(hash) { location.hash = '#' + hash; }

/* ---------- nav rendering (sidebar / bottom-nav / drawer) ---------- */

// appRole gates the UI: a "viewer" login only sees the "Xem lại" (review) view.
let appRole = 'admin';
let appRedbidaEnabled = false;
function navItems() {
  if (appRole === 'viewer') return NAV_ITEMS.filter(n => n.hash === 'review');
  return NAV_ITEMS.filter(n => n.hash !== 'redbida' || appRedbidaEnabled);
}

function buildNav() {
  const items = navItems();
  const sidebar = document.getElementById('sidebar-nav');
  sidebar.innerHTML = items.map(n => `
    <a class="nav-link" href="#${n.hash}" data-nav-hash="${n.hash}">
      ${n.icon}<span>${n.label}</span>
    </a>`).join('');

  const bottomnav = document.getElementById('bottomnav');
  bottomnav.innerHTML = items.filter(n => n.bottom !== false).map(n => `
    <a class="bottomnav-item" href="#${n.hash}" data-nav-hash="${n.hash}">${n.icon}<span>${n.short || n.label}</span></a>
  `).join('') + `
    <button class="bottomnav-item" id="drawer-open-btn" type="button">${ICONS.dots}<span>Menu</span></button>
  `;

  const drawer = document.getElementById('drawer-nav');
  // Anything not in the bottom nav (bottom: false) still needs a way in on
  // mobile — list it here so it isn't stranded.
  drawer.innerHTML = items.filter(n => n.bottom === false).map(n => `
    <a class="drawer-item" href="#${n.hash}" data-nav-hash="${n.hash}">${n.icon}<span>${n.label}</span></a>
  `).join('') + `
    <button class="drawer-item" id="drawer-theme-btn" type="button">${ICONS.moon}<span>Đổi giao diện sáng/tối</span></button>
    <a class="drawer-item" href="/logout">${ICONS.logout}<span>Đăng xuất</span></a>
  `;
}

function openDrawer() {
  document.getElementById('drawer').classList.add('open');
  document.getElementById('drawer-backdrop').classList.add('open');
}
function closeDrawer() {
  document.getElementById('drawer').classList.remove('open');
  document.getElementById('drawer-backdrop').classList.remove('open');
}

/* ---------- theme ---------- */

function effectiveTheme() {
  const attr = document.documentElement.getAttribute('data-theme');
  if (attr) return attr;
  return window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
}
function setTheme(t) {
  document.documentElement.setAttribute('data-theme', t);
  localStorage.setItem('kspcam-theme', t);
}
function toggleTheme() { setTheme(effectiveTheme() === 'dark' ? 'light' : 'dark'); }

/* ---------- dashboard ---------- */

function renderDashboard() {
  document.getElementById('stat-total').textContent = cameras.length;
  document.getElementById('stat-dahua').textContent = cameras.filter(c => c.vendor === 'dahua').length;
  document.getElementById('stat-hik').textContent = cameras.filter(c => c.vendor === 'hikvision').length;
  const lastEl = document.getElementById('stat-lastrun');
  const recentEl = document.getElementById('dash-recent');
  if (!lastRun) {
    lastEl.textContent = '–';
    lastEl.className = 'stat-value';
    recentEl.textContent = 'Chưa có hoạt động nào trong phiên này.';
    recentEl.className = 'muted';
    return;
  }
  lastEl.textContent = `${lastRun.ok} OK / ${lastRun.fail} lỗi`;
  lastEl.className = 'stat-value ' + (lastRun.fail ? 'fail' : 'ok');
  const label = lastRun.type === 'password' ? 'Đổi mật khẩu' : 'Áp dụng cấu hình';
  recentEl.innerHTML = `${escapeHtml(label)} lúc ${escapeHtml(lastRun.time)} — ${lastRun.total} camera: ` +
    `<span style="color:var(--success)">${lastRun.ok} thành công</span>` +
    (lastRun.fail ? `, <span style="color:var(--danger)">${lastRun.fail} lỗi</span>` : '.');
}

/* ---------- cameras table ---------- */

function renderCameraSkeleton() {
  const tbody = document.getElementById('cam-tbody');
  tbody.innerHTML = Array.from({ length: 3 }).map(() => `
    <tr class="skeleton-row"><td colspan="10"><span class="skeleton" style="width:100%;display:block"></span></td></tr>
  `).join('');
}

let camSort = { key: 'name', dir: 1 };
const camCollator = new Intl.Collator('vi', { numeric: true, sensitivity: 'base' });

function camSortVal(c, key) {
  switch (key) {
    case 'name': return `${c.name || ''} ${c.channelName || ''}`.trim();
    case 'host': return c.host || '';
    case 'port': return c.port || 0;
    case 'vendor': return c.vendor || '';
    case 'username': return c.username || '';
    case 'password': return c.password || '';
    case 'stream': return (fmtStreamInfo(probeCache[c.id]) || '').replace(/<[^>]*>/g, '');
    default: return '';
  }
}

let cameraViewMode = localStorage.getItem('kspcam_cam_view_mode') || 'table';

function setCameraViewMode(mode) {
  cameraViewMode = (mode === 'grid') ? 'grid' : 'table';
  localStorage.setItem('kspcam_cam_view_mode', cameraViewMode);
  const tableBtn = document.getElementById('cam-view-table-btn');
  const gridBtn = document.getElementById('cam-view-grid-btn');
  const tableWrap = document.getElementById('cam-table-wrap') || document.getElementById('cam-table');
  const gridWrap = document.getElementById('cam-grid');
  if (tableBtn) tableBtn.classList.toggle('active', cameraViewMode === 'table');
  if (gridBtn) gridBtn.classList.toggle('active', cameraViewMode === 'grid');
  if (tableWrap) tableWrap.hidden = (cameraViewMode === 'grid');
  if (gridWrap) gridWrap.hidden = (cameraViewMode === 'table');
}

function sortedCameras() {
  const { key, dir } = camSort;
  return cameras.slice().sort((a, b) => {
    const va = camSortVal(a, key), vb = camSortVal(b, key);
    if (typeof va === 'number' && typeof vb === 'number') return (va - vb) * dir;
    return camCollator.compare(String(va), String(vb)) * dir;
  });
}

function renderCameras() {
  const tbody = document.getElementById('cam-tbody');
  const grid = document.getElementById('cam-grid');
  document.querySelectorAll('#cam-table th.sortable').forEach(th => {
    th.classList.toggle('sort-asc', th.dataset.sort === camSort.key && camSort.dir === 1);
    th.classList.toggle('sort-desc', th.dataset.sort === camSort.key && camSort.dir === -1);
  });
  const query = (document.getElementById('camera-search').value || '').trim().toLocaleLowerCase('vi');
  const vendor = document.getElementById('camera-vendor-filter').value;
  const visible = sortedCameras().filter(c => {
    if (vendor && c.vendor !== vendor) return false;
    if (!query) return true;
    return [c.name, c.channelName, c.host, c.username].some(v => String(v || '').toLocaleLowerCase('vi').includes(query));
  });
  document.getElementById('camera-list-count').textContent = `${visible.length}/${cameras.length} camera`;
  if (!cameras.length) {
    tbody.innerHTML = '<tr><td colspan="10" class="empty-hint">Chưa có camera nào. Bấm “Thêm camera”.</td></tr>';
    if (grid) grid.innerHTML = '<p class="empty-hint">Chưa có camera nào. Bấm “Thêm camera”.</p>';
    renderDashboard();
    return;
  }
  if (!visible.length) {
    tbody.innerHTML = '<tr><td colspan="10" class="empty-hint">Không có camera khớp bộ lọc.</td></tr>';
    if (grid) grid.innerHTML = '<p class="empty-hint">Không có camera khớp bộ lọc.</p>';
    renderDashboard();
    return;
  }
  tbody.innerHTML = visible.map(c => `
    <tr data-id="${escapeHtml(c.id)}" data-testid="camera-row" tabindex="0" aria-label="Mở cấu hình ${escapeHtml(c.name || c.host)}">
      <td class="cell-check"><input type="checkbox" class="cam-cb" value="${escapeHtml(c.id)}" ${selectedCameraSet.has(c.id) ? 'checked' : ''} aria-label="Chọn ${escapeHtml(c.name || c.host)}"></td>
      <td data-label="Tên" class="cell-name">
        <span class="cell-name-text">${escapeHtml(c.name || '(chưa đặt tên)')}</span>${c.channelName ? '<span class="muted"> · ' + escapeHtml(c.channelName) + '</span>' : ''}
        ${c.isNvr ? '<span class="badge">NVR</span>' : ''}
        ${(c.noStorage && c.nvrId) ? `<span class="badge ok" title="Xem lại/tải lấy từ đầu ghi, kênh ${c.nvrChannel || '?'}">⛁ đầu ghi</span>` : ''}
        <button class="btn-icon" data-action="rename-inline" data-id="${escapeHtml(c.id)}" title="Sửa nhanh tên trong kho" aria-label="Sửa tên">${ICONS.edit}</button>
      </td>
      <td data-label="Host">${escapeHtml(c.host)}</td>
      <td data-label="Cổng">${c.port}</td>
      <td data-label="Hãng">${escapeHtml(c.vendor)}</td>
      <td data-label="SN / QR">${cameraSerialHtml(c)}</td>
      <td data-label="Tài khoản">${escapeHtml(c.username || '')}</td>
      <td data-label="Mật khẩu"><span class="password-cell"><code data-password-for="${escapeHtml(c.id)}">••••••••</code><button class="btn-icon password-toggle" data-action="reveal-pass" data-id="${escapeHtml(c.id)}" aria-label="Hiện mật khẩu">Hiện</button></span></td>
      <td data-label="Thông tin luồng" class="probe-box" id="probe-${cssEscape(c.id)}">${fmtStreamInfo(probeCache[c.id]) || '<span class="muted">chưa dò</span>'}</td>
      <td class="actions-cell">
        <span class="row-quick-actions">
          <button class="btn-icon quick-btn" data-action="quick-live" data-id="${escapeHtml(c.id)}" title="Xem Live MJPEG">👁️</button>
          <button class="btn-icon quick-btn" data-action="quick-snap" data-id="${escapeHtml(c.id)}" title="Chụp ảnh Snapshot">📷</button>
          <button class="btn-icon quick-btn" data-action="quick-ptz" data-id="${escapeHtml(c.id)}" title="Bàn xoay PTZ">🎮</button>
          <button class="btn-icon quick-btn" data-action="quick-reboot" data-id="${escapeHtml(c.id)}" title="Khởi động lại">🔄</button>
          <button class="btn-icon quick-btn" data-action="quick-sync-time" data-id="${escapeHtml(c.id)}" title="Đồng bộ giờ NTP">⏰</button>
        </span>
        <button class="btn btn-secondary" data-action="view" data-id="${escapeHtml(c.id)}">Xem hình</button>
        <details class="row-menu">
          <summary class="btn btn-secondary" aria-label="Thao tác khác cho ${escapeHtml(c.name || c.host)}">⋯</summary>
          <div class="row-menu-items">
            <button class="btn btn-secondary" data-action="detail" data-id="${escapeHtml(c.id)}">Cấu hình chi tiết</button>
            <button class="btn btn-secondary" data-action="probe" data-id="${escapeHtml(c.id)}">Dò cấu hình</button>
            <button class="btn btn-secondary" data-action="view-all" data-id="${escapeHtml(c.id)}">Xem tất cả kênh</button>
            <button class="btn btn-secondary" data-action="edit" data-id="${escapeHtml(c.id)}">Sửa thông tin kho</button>
            <button class="btn btn-danger" data-action="delete" data-id="${escapeHtml(c.id)}">Xóa khỏi kho</button>
          </div>
        </details>
      </td>
    </tr>
  `).join('');

  if (grid) {
    grid.innerHTML = visible.map(c => {
      const streams = probeCache[c.id] || [];
      const main = streams.find(s => s.stream === 0) || streams[0];
      const resTag = main ? `${main.width}x${main.height}` : '';
      const fpsTag = (main && main.fps > 0) ? `${main.fps}fps` : '';
      const codecTag = main ? main.compression : '';
      const audioTag = (main && main.audioEnable) ? (main.audioCodec || 'AAC') : '';
      const vendorClass = `vendor-${c.vendor || 'dahua'}`;
      const isChecked = selectedCameraSet.has(c.id);
      const snapUrl = `/api/snapshot?id=${encodeURIComponent(c.id)}&channel=0&stream=1&_r=${Date.now()}`;
      return `
      <div class="cam-card ${isChecked ? 'selected' : ''}" data-id="${escapeHtml(c.id)}" data-testid="camera-card" tabindex="0" aria-label="Camera ${escapeHtml(c.name || c.host)}">
        <div class="cam-card-thumb-wrap">
          <img class="cam-card-thumb" src="${snapUrl}" alt="${escapeHtml(c.name || c.host)}" loading="lazy" onerror="this.onerror=null; this.parentElement.innerHTML='<div class=\\'cam-card-thumb-fallback\\'><span>📷 Không có ảnh</span><small class=\\'muted\\'>${escapeHtml(c.host)}</small></div>';">
          <div class="cam-card-badge-overlay">
            <label class="cam-card-check" title="Chọn camera">
              <input type="checkbox" class="cam-card-cb" value="${escapeHtml(c.id)}" ${isChecked ? 'checked' : ''}>
            </label>
            <span class="cam-card-vendor-badge ${vendorClass}">${escapeHtml(c.vendor)}</span>
          </div>
        </div>
        <div class="cam-card-body">
          <div class="cam-card-title-row">
            <h4 class="cam-card-title" title="${escapeHtml(c.name || c.host)}">${escapeHtml(c.name || '(chưa đặt tên)')}</h4>
            ${c.isNvr ? '<span class="badge">NVR</span>' : ''}
            ${(c.noStorage && c.nvrId) ? '<span class="badge ok" title="Xem lại từ đầu ghi">⛁ NVR</span>' : ''}
          </div>
          <div class="cam-card-meta">
            <span>${escapeHtml(c.host)}:${c.port}</span>
            ${c.channelName ? `<span class="muted">· ${escapeHtml(c.channelName)}</span>` : ''}
          </div>
          <div class="cam-card-specs">
            ${resTag ? `<span class="cam-spec-tag">${resTag}</span>` : ''}
            ${fpsTag ? `<span class="cam-spec-tag">${fpsTag}</span>` : ''}
            ${codecTag ? `<span class="cam-spec-tag">${codecTag}</span>` : ''}
            ${audioTag ? `<span class="cam-spec-tag" style="color:var(--success)">🔊 ${audioTag}</span>` : ''}
          </div>
          <div class="cam-card-actions">
            <button class="btn-icon" data-action="quick-live" data-id="${escapeHtml(c.id)}" title="Xem Live Stream">👁️</button>
            <button class="btn-icon" data-action="quick-snap" data-id="${escapeHtml(c.id)}" title="Chụp ảnh tức thời">📷</button>
            <button class="btn-icon" data-action="quick-ptz" data-id="${escapeHtml(c.id)}" title="Điều khiển PTZ">🎮</button>
            <button class="btn-icon" data-action="quick-reboot" data-id="${escapeHtml(c.id)}" title="Khởi động lại">🔄</button>
            <button class="btn-icon" data-action="quick-sync-time" data-id="${escapeHtml(c.id)}" title="Đồng bộ giờ NTP">⏰</button>
            <button class="btn-icon" data-action="detail" data-id="${escapeHtml(c.id)}" title="Cấu hình chi tiết">⚙</button>
          </div>
        </div>
      </div>`;
    }).join('');
  }

  renderCameraSerialQRCodes(tbody);
  setCameraViewMode(cameraViewMode);
  renderDashboard();
}

async function loadCameras() {
  renderCameraSkeleton();
  try {
    cameras = await api('/api/cameras');
    for (const id of selectedCameraSet) if (!cameras.some(c => c.id === id)) selectedCameraSet.delete(id);
    renderCameras();
    renderBulkSelection();
    if (currentCameraTask() === 'nvr') renderNvrList();
    setRoute();
  } catch (e) {
    document.getElementById('cam-tbody').innerHTML =
      `<tr><td colspan="7"><span class="msg err">Lỗi tải danh sách: ${escapeHtml(e.message)}</span></td></tr>`;
  }
}

// resolveLegacyHash rewrites the one deep link that can't be a static alias:
// '#cameras/devices/nvr-storage' meant "open the first NVR's storage page".
// It runs from setRoute (not just at boot, as it used to), so pasting the link
// into an already-loaded tab works instead of silently doing nothing. Returns
// true when it redirected — the caller should stop and wait for the new hash.
function resolveLegacyHash() {
  const raw = (location.hash || '').slice(1);
  if (raw !== 'cameras/devices/nvr-storage') return false;
  const nvr = cameras.find(x => x.isNvr);
  if (!nvr) {
    // Inventory not loaded yet: loadCameras() calls setRoute again once it is.
    if (cameras.length) { location.hash = '#cameras/nvr'; return true; }
    return false;
  }
  location.hash = cameraDetailHash(nvr.id, 'maint');
  return true;
}

/* ---------- NVR link: cameras with no storage play back from the NVR ---------- */
(function () {
  const dlg = () => document.getElementById('nvr-link-dialog');
  let scanRows = [];
  function camOptions(selectedId) {
    return ['<option value="">— không gán —</option>'].concat(
      cameras.filter(c => !c.isNvr).map(c =>
        `<option value="${escapeHtml(c.id)}"${c.id === selectedId ? ' selected' : ''}>${escapeHtml(c.name || c.id)} (${escapeHtml(c.host)})</option>`)
    ).join('');
  }
  function renderRows() {
    const tb = document.getElementById('nvr-tbody');
    if (!scanRows.length) { tb.innerHTML = '<tr><td colspan="4" class="empty-hint">Không đọc được kênh nào.</td></tr>'; return; }
    tb.innerHTML = scanRows.map((r, i) => `
      <tr>
        <td data-label="Kênh">${r.nvrChannel}</td>
        <td data-label="Cam ở đầu ghi">${escapeHtml(r.nvrCamName || '')}<br><span class="muted">${escapeHtml(r.nvrCamIP || '')}</span></td>
        <td data-label="Gán camera"><select data-row="${i}" class="nvr-cam-sel">${camOptions(r.suggestedCameraId)}</select></td>
        <td data-label="Không bộ nhớ"><input type="checkbox" data-row="${i}" class="nvr-nostore"${r.noStorage ? ' checked' : ''}></td>
      </tr>`).join('');
  }
  const nvrBody = () => ({
    host: document.getElementById('nvr-host').value.trim(),
    port: parseInt(document.getElementById('nvr-port').value, 10) || 0,
    username: document.getElementById('nvr-user').value.trim(),
    password: document.getElementById('nvr-pass').value,
    vendor: document.getElementById('nvr-vendor').value,
  });
  document.getElementById('nvr-open-btn').addEventListener('click', () => {
    const nvr = cameras.find(c => c.isNvr);
    document.getElementById('nvr-host').value = nvr ? nvr.host : '';
    document.getElementById('nvr-port').value = nvr ? nvr.port : 37777;
    document.getElementById('nvr-user').value = nvr ? nvr.username : 'admin';
    document.getElementById('nvr-pass').value = '';
    document.getElementById('nvr-vendor').value = (nvr && (nvr.vendor === 'dahua' || nvr.vendor === 'hikvision')) ? nvr.vendor : 'hikvision';
    scanRows = [];
    document.getElementById('nvr-tbody').innerHTML = '<tr><td colspan="4" class="empty-hint">Nhập đầu ghi rồi bấm "Quét đầu ghi".</td></tr>';
    document.getElementById('nvr-save-btn').disabled = true;
    document.getElementById('nvr-scan-msg').textContent = '';
    dlg().showModal();
  });
  document.getElementById('nvr-close-btn').addEventListener('click', () => dlg().close());
  document.getElementById('nvr-scan-btn').addEventListener('click', async (ev) => {
    if (!document.getElementById('nvr-host').value.trim()) { showToast('Nhập host đầu ghi.', 'err'); return; }
    const msg = document.getElementById('nvr-scan-msg');
    ev.target.disabled = true; msg.textContent = 'Đang quét (kiểm tra bộ nhớ từng cam, có thể mất chút)...';
    try {
      const res = await api('/api/nvr/scan', { method: 'POST', body: JSON.stringify(Object.assign(nvrBody(), { timeoutSeconds: timeoutSec() })) });
      scanRows = res.rows || [];
      renderRows();
      document.getElementById('nvr-save-btn').disabled = !scanRows.length;
      msg.textContent = `${scanRows.length} kênh. Kiểm tra & sửa gán rồi bấm Lưu.`;
    } catch (e) { msg.textContent = 'Lỗi: ' + e.message; }
    finally { ev.target.disabled = false; }
  });
  document.getElementById('nvr-save-btn').addEventListener('click', async (ev) => {
    const mappings = scanRows.map((r, i) => {
      const sel = document.querySelector(`.nvr-cam-sel[data-row="${i}"]`);
      const cb = document.querySelector(`.nvr-nostore[data-row="${i}"]`);
      return { cameraId: sel ? sel.value : '', nvrChannel: r.nvrChannel, nvrName: r.nvrCamName || '', noStorage: cb ? cb.checked : false };
    }).filter(m => m.cameraId);
    if (!mappings.length) { showToast('Chưa gán camera nào.', 'err'); return; }
    ev.target.disabled = true;
    try {
      const res = await api('/api/nvr/link', { method: 'POST', body: JSON.stringify({ nvr: Object.assign(nvrBody(), { name: '' }), mappings }) });
      showToast(`Đã liên kết ${res.linked} camera với đầu ghi.`, 'ok');
      dlg().close();
      await loadCameras();
    } catch (e) { showToast('Lỗi lưu: ' + e.message, 'err'); }
    finally { ev.target.disabled = false; }
  });
})();

// "Dò tên kênh (tất cả)" — probe every Dahua camera's channel/OSD title and save
// it, so the review dropdown shows "Camera01 - <channel name>".
(function () {
  const btn = document.getElementById('probe-names-btn');
  if (!btn) return;
  btn.addEventListener('click', async () => {
    const msg = document.getElementById('probe-names-msg');
    btn.disabled = true; msg.textContent = 'Đang dò tên kênh (có thể mất chút)...';
    try {
      const res = await api('/api/channel-names', { method: 'POST', body: JSON.stringify({ ids: [], timeoutSeconds: timeoutSec() }) });
      msg.textContent = `Đã dò ${res.count} tên kênh.`;
      await loadCameras();
    } catch (e) { msg.textContent = 'Lỗi: ' + e.message; }
    finally { btn.disabled = false; }
  });
})();

function selectedCameraIds() {
  return Array.from(selectedCameraSet);
}

function setCameraSelected(id, selected) {
  if (selected) selectedCameraSet.add(id);
  else selectedCameraSet.delete(id);
  document.querySelectorAll('.cam-cb, .bulk-cam-cb, .cam-card-cb').forEach(cb => {
    if (cb.value === id) cb.checked = selected;
  });
  document.querySelectorAll(`.cam-card[data-id="${CSS.escape(id)}"]`).forEach(card => {
    card.classList.toggle('selected', selected);
  });
  renderBulkSelection();
}

// renderNvrList fills the Đầu ghi tab: one row per NVR in the inventory, with
// how many cameras fall back to it and a direct link to its storage page.
function renderNvrList() {
  const tbody = document.getElementById('nvr-list-tbody');
  const nvrs = cameras.filter(c => c.isNvr);
  if (!nvrs.length) {
    tbody.innerHTML = '<tr><td colspan="5" class="empty-hint">Chưa có đầu ghi nào trong kho. Bấm “Quét &amp; liên kết đầu ghi” để thêm.</td></tr>';
    return;
  }
  tbody.innerHTML = nvrs.map(n => {
    const linked = cameras.filter(c => c.nvrId === n.id);
    const detail = linked.length
      ? linked.map(c => `<a data-testid="nvr-camera-link" href="#review/${encodeURIComponent(c.id)}/${Math.max(0, Number(c.nvrChannel || 1) - 1)}">${escapeHtml(c.name || c.host)} → K${c.nvrChannel || '?'}</a>`).join('<br>')
      : '<span class="muted">chưa liên kết camera nào</span>';
    return `<tr data-id="${escapeHtml(n.id)}">
      <td data-label="Đầu ghi">${escapeHtml(n.name || n.host)}</td>
      <td data-label="Host">${escapeHtml(n.host)}:${n.port}</td>
      <td data-label="Camera đã liên kết">${detail}</td>
      <td data-label="Sức khỏe"><span class="badge" data-testid="nvr-status" data-nvr-status="${escapeHtml(n.id)}">Đang kiểm tra</span><br><span class="muted" data-nvr-next="${escapeHtml(n.id)}"></span></td>
      <td class="actions-cell nvr-actions">
        <label class="checkbox-row compact"><input type="checkbox" data-testid="nvr-watchdog" data-nvr-watchdog="${escapeHtml(n.id)}" ${n.nvrWatchdog ? 'checked' : ''}> Tự sửa ghi hình</label>
        <label class="checkbox-row compact"><input type="checkbox" data-testid="nvr-sync-time" data-nvr-sync="${escapeHtml(n.id)}" ${n.nvrSyncTimeFromHost ? 'checked' : ''}> Lấy giờ từ INUT</label>
        <button class="btn btn-sm" type="button" data-nvr-check="${escapeHtml(n.id)}">Kiểm tra ngay</button>
        <a class="btn btn-secondary btn-sm" href="${cameraDetailHash(n.id, 'maint')}" data-testid="nvr-open-maint">HDD &amp; ghi hình</a>
      </td>
    </tr>`;
  }).join('');
  tbody.querySelectorAll('[data-nvr-watchdog],[data-nvr-sync]').forEach(input => input.addEventListener('change', () => saveNvrWatchdog(input.closest('tr'))));
  tbody.querySelectorAll('[data-nvr-check]').forEach(btn => btn.addEventListener('click', () => checkNvrHealth(btn.dataset.nvrCheck, btn)));
  nvrs.forEach(n => loadNvrListHealth(n.id));
}

function nvrStatusLabel(status) {
  return ({ healthy: 'Tốt', repairing: 'Đang sửa', warning: 'Cảnh báo', critical: 'Lỗi ghi hình', unknown: 'Chưa rõ' })[status] || 'Chưa rõ';
}

async function loadNvrListHealth(id) {
  const badge = document.querySelector(`[data-nvr-status="${CSS.escape(id)}"]`);
  if (!badge) return;
  try {
    const h = await api(`/api/nvr/health?id=${encodeURIComponent(id)}`);
    badge.textContent = nvrStatusLabel(h.status);
    badge.className = `badge nvr-health-${h.status || 'unknown'}`;
    const next = document.querySelector(`[data-nvr-next="${CSS.escape(id)}"]`);
    if (next) next.textContent = h.nextCheck ? `Lần tới: ${new Date(h.nextCheck).toLocaleTimeString('vi-VN')}` : 'Watchdog đang tắt';
  } catch (e) { badge.textContent = 'Không tải được'; badge.className = 'badge nvr-health-critical'; }
}

async function saveNvrWatchdog(row) {
  const id = row.dataset.id;
  const enabled = row.querySelector('[data-nvr-watchdog]').checked;
  const syncTimeFromHost = row.querySelector('[data-nvr-sync]').checked;
  try {
    await api('/api/nvr/watchdog', { method: 'POST', body: JSON.stringify({ id, enabled, syncTimeFromHost }) });
    const n = cameras.find(x => x.id === id); if (n) { n.nvrWatchdog = enabled; n.nvrSyncTimeFromHost = syncTimeFromHost; }
    showToast(enabled ? 'Đã bật watchdog ghi hình.' : 'Đã tắt watchdog ghi hình.', 'ok');
    await loadNvrListHealth(id);
  } catch (e) { showToast('Lỗi lưu watchdog: ' + e.message, 'err'); }
}

async function checkNvrHealth(id, button) {
  if (button) button.disabled = true;
  try { await api('/api/nvr/health/check', { method: 'POST', body: JSON.stringify({ id }) }); await loadNvrListHealth(id); }
  catch (e) { showToast('Kiểm tra NVR lỗi: ' + e.message, 'err'); }
  finally { if (button) button.disabled = false; }
}

function renderBulkSelection() {
  const ids = selectedCameraIds();
  const countEl = document.getElementById('bulk-selected-count');
  const chipsEl = document.getElementById('bulk-selected-chips');
  const deleteBtn = document.getElementById('bulk-delete-cameras-btn');
  if (deleteBtn) {
    deleteBtn.disabled = ids.length === 0;
    deleteBtn.textContent = ids.length ? `Xóa các cam đã chọn (${ids.length})` : 'Xóa các cam đã chọn';
  }
  const selectAll = document.getElementById('select-all');
  const visibleChecks = Array.from(document.querySelectorAll('.cam-cb'));
  const selectedVisible = visibleChecks.filter(cb => cb.checked).length;
  if (selectAll) {
    selectAll.checked = visibleChecks.length > 0 && selectedVisible === visibleChecks.length;
    selectAll.indeterminate = selectedVisible > 0 && selectedVisible < visibleChecks.length;
  }
  document.getElementById('apply-count').textContent = ids.length ? ids.length + ' camera đã chọn' : '';
  if (!ids.length) {
    countEl.textContent = 'Chưa chọn camera nào (0 camera).';
    chipsEl.innerHTML = '';
  } else {
    countEl.textContent = ids.length + ' camera đã chọn:';
    chipsEl.innerHTML = ids.map(id => {
      const c = cameras.find(x => x.id === id);
      return `<button type="button" class="chip chip-btn" data-unselect-camera="${escapeHtml(id)}" title="Bỏ chọn">${escapeHtml(c ? (c.name || c.host) : id)} ×</button>`;
    }).join('');
  }
  document.getElementById('bulk-camera-picker').innerHTML = cameras.length ? cameras.map(c => `
    <label class="bulk-camera-option">
      <input type="checkbox" class="bulk-cam-cb" value="${escapeHtml(c.id)}" ${selectedCameraSet.has(c.id) ? 'checked' : ''}>
      <span><strong>${escapeHtml(c.name || '(chưa đặt tên)')}</strong><small>${escapeHtml(c.host)} · ${escapeHtml(c.vendor)}</small></span>
    </label>`).join('') : '<p class="empty-hint">Chưa có camera trong kho.</p>';
}

/* ---------- add / edit / delete / probe ---------- */

// The camera form is one <dialog> in two explicit modes. Editing used to reuse
// the inline "add" card, which quietly created a SECOND inventory entry when
// host/port changed (the id is host:port). Now editing locks those two fields
// behind a deliberate "Đổi địa chỉ" click that says what will happen.
let cameraFormMode = 'add'; // 'add' | 'edit'

function openCameraForm(cam) {
  cameraFormMode = cam ? 'edit' : 'add';
  const dlg = document.getElementById('camera-form-dialog');
  const msg = document.getElementById('add-msg');
  const locked = document.getElementById('camera-form-locked');
  msg.textContent = ''; msg.className = 'msg';
  document.getElementById('camera-form-title').textContent =
    cam ? `Sửa "${cam.name || cam.host}"` : 'Thêm camera';
  document.getElementById('add-submit-btn').textContent = cam ? 'Lưu thay đổi' : 'Thêm camera';
  document.getElementById('f-name').value = cam ? (cam.name || '') : '';
  document.getElementById('f-host').value = cam ? (cam.host || '') : '';
  document.getElementById('f-port').value = cam ? (cam.port || '') : '';
  document.getElementById('f-vendor').value = cam ? (cam.vendor || 'dahua') : 'dahua';
  document.getElementById('f-username').value = cam ? (cam.username || '') : '';
  const pw = document.getElementById('f-password');
  pw.value = cam ? (cam.password || '') : '';
  pw.placeholder = cam ? 'để trống = giữ mật khẩu cũ' : '••••••';
  setCameraFormAddressLocked(!!cam);
  locked.hidden = !cam;
  openDialog(dlg);
  document.getElementById('f-name').focus();
}

function setCameraFormAddressLocked(lock) {
  document.getElementById('f-host').disabled = lock;
  document.getElementById('f-port').disabled = lock;
}

document.getElementById('camera-add-open').addEventListener('click', () => openCameraForm(null));
document.getElementById('camera-form-cancel').addEventListener('click',
  () => closeDialog(document.getElementById('camera-form-dialog')));
document.getElementById('camera-form-unlock').addEventListener('click', () => {
  setCameraFormAddressLocked(false);
  const m = document.getElementById('add-msg');
  m.className = 'msg';
  m.textContent = 'Lưu với địa chỉ mới sẽ tạo một camera mới; camera cũ vẫn còn trong kho.';
});

document.getElementById('add-form').addEventListener('submit', async (ev) => {
  ev.preventDefault();
  const msg = document.getElementById('add-msg');
  msg.textContent = ''; msg.className = 'msg';
  const body = {
    name: document.getElementById('f-name').value.trim(),
    // .value still reads correctly on a disabled input; disabling only stops
    // the user editing it and keeps it out of the form's own submission set.
    host: document.getElementById('f-host').value.trim(),
    port: parseInt(document.getElementById('f-port').value, 10) || 0,
    vendor: document.getElementById('f-vendor').value,
    username: document.getElementById('f-username').value,
    password: document.getElementById('f-password').value,
  };
  const btn = document.getElementById('add-submit-btn');
  setBusy(btn, true, 'Đang lưu...');
  try {
    await api('/api/cameras', { method: 'POST', body: JSON.stringify(body) });
    showToast(cameraFormMode === 'edit' ? 'Đã lưu thay đổi.' : 'Đã thêm camera.', 'ok');
    closeDialog(document.getElementById('camera-form-dialog'));
    await loadCameras();
  } catch (e) {
    msg.textContent = 'Lỗi: ' + e.message;
    msg.className = 'msg err';
    showToast('Lỗi: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
});

// Clicking the row opens the camera detail page; the checkbox, the inline
// rename input and every action control opt out via this guard.
function rowOpensDetail(ev) {
  if (ev.target.closest('button, input, select, textarea, a, summary, details')) return null;
  const tr = ev.target.closest('tr[data-id], .cam-card[data-id]');
  return tr ? tr.dataset.id : null;
}

async function syncDeviceTime(c) {
  if (!c) return;
  try {
    const timeStr = (new Date()).toISOString().replace('T', ' ').slice(0, 19);
    await api('/api/device-time', {
      method: 'POST',
      body: JSON.stringify({
        id: c.id,
        currentTime: timeStr,
        ntpEnable: true,
        timeoutSeconds: timeoutSec(),
      }),
    });
    showToast(`Đã đồng bộ giờ máy chủ cho "${c.name || c.host}".`, 'ok');
  } catch (e) {
    showToast('Lỗi đồng bộ giờ: ' + e.message, 'err');
  }
}

async function handleCameraAction(action, id, btn) {
  const c = cameras.find(x => x.id === id);
  if (!id) return;
  const menu = btn?.closest('details.row-menu');
  if (menu) menu.open = false;

  if (action === 'detail') { gotoCameraDetail(id, 'osd'); return; }
  if (action === 'edit') { if (c) openCameraForm(c); return; }
  if (action === 'delete') {
    const ok = await showConfirm('Xóa camera', `Xóa camera "${c ? (c.name || c.host) : id}" khỏi kho?`, { danger: true, okLabel: 'Xóa' });
    if (!ok) return;
    if (btn) btn.disabled = true;
    try {
      await api('/api/cameras/delete', { method: 'POST', body: JSON.stringify({ id, timeoutSeconds: timeoutSec() }) });
      delete probeCache[id];
      showToast('Đã xóa camera.', 'ok');
      await loadCameras();
    } catch (e) {
      showToast('Lỗi xóa: ' + e.message, 'err');
      if (btn) btn.disabled = false;
    }
    return;
  }
  if (action === 'probe') {
    if (btn) btn.disabled = true;
    const cell = document.getElementById('probe-' + cssEscape(id));
    if (cell) cell.innerHTML = '<span class="muted">đang dò...</span>';
    try {
      rememberProbeResult(id, await api('/api/probe', { method: 'POST', body: JSON.stringify({ id, timeoutSeconds: timeoutSec() }) }));
      renderCameras();
    } catch (e) {
      if (cell) cell.innerHTML = `<span class="msg err">${escapeHtml(e.message)}</span>`;
    } finally {
      if (btn) btn.disabled = false;
    }
    return;
  }
  if (action === 'view') {
    openGallery(buildTiles([id]));
    return;
  }
  if (action === 'view-all') {
    await viewAllChannels(id, btn);
    return;
  }
  if (action === 'rename-inline') {
    startInlineRename(btn?.closest('.cell-name'), id);
    return;
  }
  if (action === 'reveal-pass') {
    const code = Array.from(document.querySelectorAll('[data-password-for]')).find(el => el.dataset.passwordFor === id);
    if (!c || !code) return;
    const revealed = btn.dataset.revealed === 'true';
    code.textContent = revealed ? '••••••••' : (c.password || '(trống)');
    btn.dataset.revealed = revealed ? 'false' : 'true';
    btn.textContent = revealed ? 'Hiện' : 'Ẩn';
    btn.setAttribute('aria-label', revealed ? 'Hiện mật khẩu' : 'Ẩn mật khẩu');
    return;
  }
  if (action === 'quick-live') {
    gotoCameraDetail(id, 'osd');
    setTimeout(() => {
      if (detailLive && !detailLive.running()) detailLive.start();
    }, 250);
    return;
  }
  if (action === 'quick-snap') {
    const snapUrl = `/api/snapshot?id=${encodeURIComponent(id)}&channel=0&stream=0&timeoutSeconds=${timeoutSec()}&_r=${Date.now()}`;
    const lb = document.getElementById('lightbox-dialog');
    const img = document.getElementById('lightbox-img');
    const lbl = document.getElementById('lightbox-label');
    if (lb && img && lbl) {
      lbl.textContent = `Snapshot: ${c ? (c.name || c.host) : id} — ${new Date().toLocaleTimeString('vi-VN')}`;
      img.src = snapUrl;
      openDialog(lb);
    }
    return;
  }
  if (action === 'quick-ptz') {
    if (c) openQuickPtz(c);
    return;
  }
  if (action === 'quick-reboot') {
    if (c) rebootDevice(c);
    return;
  }
  if (action === 'quick-sync-time') {
    if (c) syncDeviceTime(c);
    return;
  }
}

document.getElementById('cam-tbody').addEventListener('click', async (ev) => {
  const rowId = rowOpensDetail(ev);
  if (rowId) { gotoCameraDetail(rowId, 'osd'); return; }
  const btn = ev.target.closest('button[data-action]');
  if (!btn) return;
  const id = btn.dataset.id;
  await handleCameraAction(btn.dataset.action, id, btn);
});

document.getElementById('cam-grid')?.addEventListener('click', async (ev) => {
  const btn = ev.target.closest('button[data-action]');
  if (btn) {
    ev.stopPropagation();
    const id = btn.dataset.id;
    await handleCameraAction(btn.dataset.action, id, btn);
    return;
  }
  const checkLabel = ev.target.closest('.cam-card-check');
  if (checkLabel) {
    ev.stopPropagation();
    const cb = checkLabel.querySelector('.cam-card-cb');
    if (cb) setCameraSelected(cb.value, cb.checked);
    return;
  }
  const cb = ev.target.closest('.cam-card-cb');
  if (cb) {
    ev.stopPropagation();
    setCameraSelected(cb.value, cb.checked);
    return;
  }
  if (ev.target.closest('.cam-card-actions')) return;
  const card = ev.target.closest('.cam-card[data-id]');
  if (card) {
    gotoCameraDetail(card.dataset.id, 'osd');
  }
});

document.getElementById('cam-grid')?.addEventListener('change', (ev) => {
  if (ev.target.classList.contains('cam-card-cb')) {
    setCameraSelected(ev.target.value, ev.target.checked);
  }
});

document.getElementById('camera-search').addEventListener('input', renderCameras);
document.getElementById('camera-vendor-filter').addEventListener('change', renderCameras);
// Keyboard parity with the row click.
document.getElementById('cam-tbody').addEventListener('keydown', (ev) => {
  if (ev.key !== 'Enter' || ev.target.tagName !== 'TR') return;
  const id = ev.target.dataset.id;
  if (id) { ev.preventDefault(); gotoCameraDetail(id, 'osd'); }
});
document.getElementById('bulk-camera-picker').addEventListener('change', ev => {
  if (ev.target.classList.contains('bulk-cam-cb')) setCameraSelected(ev.target.value, ev.target.checked);
});
document.getElementById('bulk-selected-chips').addEventListener('click', ev => {
  const btn = ev.target.closest('[data-unselect-camera]');
  if (btn) setCameraSelected(btn.dataset.unselectCamera, false);
});

document.getElementById('bulk-delete-cameras-btn').addEventListener('click', async () => {
  const ids = selectedCameraIds();
  if (!ids.length) return;
  const labels = ids.map(id => {
    const c = cameras.find(x => x.id === id);
    return c ? (c.name || c.host || id) : id;
  });
  const preview = labels.slice(0, 5).join(', ') + (labels.length > 5 ? ` và ${labels.length - 5} camera khác` : '');
  const ok = await showConfirm(
    'Xóa camera hàng loạt',
    `Xóa ${ids.length} camera khỏi kho?\n${preview}`,
    { danger: true, okLabel: 'Xóa tất cả' },
  );
  if (!ok) return;

  const btn = document.getElementById('bulk-delete-cameras-btn');
  setBusy(btn, true, 'Đang xóa...');
  try {
    const result = await api('/api/cameras/delete-bulk', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    });
    for (const id of ids) {
      selectedCameraSet.delete(id);
      delete probeCache[id];
    }
    const deleted = Number.isFinite(result && result.deleted) ? result.deleted : ids.length;
    const skipped = Number.isFinite(result && result.skipped) ? result.skipped : 0;
    const suffix = skipped ? `, bỏ qua ${skipped} camera không còn trong kho` : '';
    showToast(`Đã xóa ${deleted} camera khỏi kho${suffix}.`, 'ok');
    await loadCameras();
  } catch (e) {
    showToast('Lỗi xóa: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
    renderBulkSelection();
  }
});

/* ---------- Mạng (Dahua/KBVision): static IP + Wi-Fi, device-level ---------- */

// This is the ONLY network editor. The camera detail dialog used to carry a
// second, near-identical one (renderCeNetwork, ids ce-net-*) with its own save
// handlers; that copy is gone and its Mạng tab now mounts these ids.
let networkEditDevice = null; // device whose Mạng/Bảo trì tabs are loaded
let lastWiFiConfig = null; // last-fetched WLan config, kept so a static-IP save doesn't have to re-fetch/hide it

function closeNetworkCard() {
  networkEditDevice = null;
  lastWiFiConfig = null;
}

async function openNetworkCard(c) {
  networkEditDevice = c;
  lastWiFiConfig = null;
  const body = document.getElementById('net-body');
  const msg = document.getElementById('net-msg');
  msg.textContent = ''; msg.className = 'msg';
  body.innerHTML = '<p class="muted">Đang tải cấu hình mạng...</p>';
  try {
    const q = `id=${encodeURIComponent(c.id)}&timeoutSeconds=${timeoutSec()}`;
    const net = await api('/api/network?' + q);
    let wifi = null;
    try { wifi = await api('/api/wifi?' + q); } catch (e) { /* no Wi-Fi radio / not supported — Wi-Fi section just won't show */ }
    lastWiFiConfig = wifi;
    renderNetworkBody(net, wifi);
  } catch (e) {
    body.innerHTML = '';
    msg.textContent = 'Lỗi tải cấu hình mạng: ' + e.message;
    msg.className = 'msg err';
  }
}

// formatBytes renders a byte count as a human-readable GB/MB string.
function formatBytes(n) {
  if (!n || n < 0) return '0';
  const gb = n / (1024 * 1024 * 1024);
  if (gb >= 1) return gb.toFixed(1) + ' GB';
  return (n / (1024 * 1024)).toFixed(0) + ' MB';
}

function formatDiskCapacity(n) {
  if (!n || n < 0) return '0 GB';
  return `${(n / 1e9).toFixed(1)} GB (${(n / (1024 * 1024 * 1024)).toFixed(1)} GiB)`;
}

// renderMaintenance fills the "Bảo trì" section: a reboot button (all
// vendors), plus storage status/format and scheduled auto-reboot (Dahua only).
async function renderMaintenance(c) {
  const el = document.getElementById('net-maint');
  const isDahua = c.vendor === 'dahua';
  let html = `<div class="row"><button class="btn btn-danger" type="button" id="maint-reboot-btn">Khởi động lại camera</button></div>`;
  if (isDahua) {
    html += `<div class="card-title section-gap">${c.isNvr ? 'Ổ cứng đầu ghi (NVR)' : 'Thẻ nhớ / Lưu trữ'}</div><div id="maint-storage"><p class="muted">Đang đọc bộ nhớ...</p></div>`;
    if (c.isNvr) html += `<div class="card-title section-gap">Tình trạng ghi hình</div><div id="maint-record-health"><p class="muted">Đang kiểm tra bản ghi gần nhất...</p></div>`;
    html += `<div class="card-title section-gap">Ngày giờ &amp; NTP</div><div id="maint-device-time"><p class="muted">Đang đọc đồng hồ thiết bị...</p></div>`;
    html += `<div class="card-title section-gap">Tự khởi động lại</div><div id="maint-autoreboot"><p class="muted">Đang đọc lịch...</p></div>`;
    html += `<div class="card-title section-gap">Xem lại video</div><div id="maint-playback"></div>`;
  }
  el.innerHTML = html;
  document.getElementById('maint-reboot-btn').addEventListener('click', () => rebootDevice(c));
  if (!isDahua) return;
  const q = `id=${encodeURIComponent(c.id)}&timeoutSeconds=${timeoutSec()}`;
  // Storage
  try {
    const s = await api('/api/storage?' + q);
    const devs = (s && s.devices) || [];
    if (!devs.length) {
      document.getElementById('maint-storage').innerHTML = '<p class="muted">Không phát hiện thẻ nhớ (chưa gắn thẻ).</p>';
    } else {
      let sh = '';
      for (const d of devs) {
        const details = d.details || [];
        const total = details.reduce((sum, x) => sum + Number(x.totalBytes || 0), 0);
        const used = details.reduce((sum, x) => sum + Number(x.usedBytes || 0), 0);
        const err = details.some(x => x.isError || x.isNeedFormat) || String(d.state || '').toLowerCase() !== 'success';
        const writable = details.length > 0 && details.every(x => x.type === 'ReadWrite');
        const health = !err && writable ? '<span style="color:var(--success)">HEALTHY · đọc/ghi tốt</span>' : '<span style="color:var(--danger)">CẦN KIỂM TRA / FORMAT</span>';
        sh += `<p>Thiết bị <b>${escapeHtml(d.name)}</b> · trạng thái: ${escapeHtml(d.state || '?')} · ${health}<br>
          Tổng dung lượng: <b>${formatDiskCapacity(total)}</b> · đã dùng: ${formatBytes(used)} · ${details.length} vùng lưu trữ.<br>
          <span class="muted">Ổ “500 GB” của nhà sản xuất tương đương khoảng 466 GiB; firmware đầu ghi này chia thành nhiều vùng nên phải cộng toàn bộ, không chỉ vùng đầu.</span></p>
          <div class="row"><button class="btn btn-danger" type="button" data-fmt="${escapeHtml(d.name)}">Format ${escapeHtml(d.name)} (xoá sạch dữ liệu)</button></div>`;
      }
      const box = document.getElementById('maint-storage');
      box.innerHTML = sh;
      box.querySelectorAll('button[data-fmt]').forEach(b => b.addEventListener('click', () => formatStorage(c, b.getAttribute('data-fmt'))));
    }
  } catch (e) {
    document.getElementById('maint-storage').innerHTML = `<p class="msg err">Lỗi đọc bộ nhớ: ${escapeHtml(e.message)}</p>`;
  }
  if (c.isNvr) await renderRecordHealth(c);
  await renderDeviceTime(c);
  // Auto-reboot
  try {
    const ar = await api('/api/autoreboot?' + q);
    const h = (ar.hour == null ? 4 : ar.hour), m = (ar.minute == null ? 0 : ar.minute);
    document.getElementById('maint-autoreboot').innerHTML = `
      <div class="checkbox-row"><input type="checkbox" id="ar-enable" ${ar.enable ? 'checked' : ''}><label for="ar-enable">Bật tự khởi động lại hằng ngày</label></div>
      <div class="row">
        <div class="field field-sm"><label for="ar-hour">Giờ (0–23)</label><input id="ar-hour" type="number" min="0" max="23" value="${h}"></div>
        <div class="field field-sm"><label for="ar-min">Phút (0–59)</label><input id="ar-min" type="number" min="0" max="59" value="${m}"></div>
      </div>
      <button class="btn btn-secondary" type="button" id="ar-save-btn">Lưu lịch tự khởi động lại</button>`;
    document.getElementById('ar-save-btn').addEventListener('click', () => saveAutoReboot(c));
  } catch (e) {
    document.getElementById('maint-autoreboot').innerHTML = `<p class="msg err">Lỗi đọc lịch: ${escapeHtml(e.message)}</p>`;
  }
  renderPlayback(c);
}

let deviceTimeRefreshTimer = null;

async function renderDeviceTime(c) {
  const el = document.getElementById('maint-device-time');
  if (!el) return;
  const q = `id=${encodeURIComponent(c.id)}&timeoutSeconds=${timeoutSec()}`;
  try {
    const t = await api('/api/device-time?' + q);
    const deviceMs = new Date((t.currentTime || '').replace(' ', 'T')).getTime();
    const driftSeconds = Number.isFinite(deviceMs) ? Math.round((deviceMs - Date.now()) / 1000) : null;
    const driftAbs = driftSeconds == null ? null : Math.abs(driftSeconds);
    const driftText = driftAbs == null ? 'Không tính được độ lệch giờ.' :
      driftAbs <= 300 ? `ĐÚNG GIỜ · lệch ${driftAbs} giây` :
      `SAI GIỜ · ${driftSeconds > 0 ? 'nhanh' : 'chậm'} ${Math.floor(driftAbs / 3600)} giờ ${Math.floor((driftAbs % 3600) / 60)} phút`;
    const driftClass = driftAbs != null && driftAbs <= 300 ? 'ok' : 'err';
    el.innerHTML = `
      <div class="msg ${driftClass}"><b>Tự kiểm tra: ${escapeHtml(driftText)}</b>${driftClass === 'err' ? '<br>Video có thể xuất hiện sai ngày hoặc mất khỏi khoảng thời gian đang xem.' : ''}</div>
      <p>Giờ hiện tại trên thiết bị: <b>${escapeHtml(t.currentTime || '?')}</b></p>
      <div class="form-grid">
        <div class="field"><label for="dt-value">Ngày giờ thiết bị</label><input id="dt-value" type="datetime-local" step="1" value="${escapeHtml((t.currentTime || '').replace(' ', 'T'))}"></div>
        <div class="field"><label for="dt-timezone">Mã timezone Dahua</label><input id="dt-timezone" type="number" value="${Number(t.timeZone || 0)}"></div>
        <div class="field"><label for="dt-timezone-desc">Tên timezone</label><input id="dt-timezone-desc" value="${escapeHtml(t.timeZoneDesc || '')}" placeholder="Bangkok"></div>
        <div class="field"><label for="dt-ntp-address">Máy chủ NTP</label><input id="dt-ntp-address" value="${escapeHtml(t.ntpAddress || 'time.google.com')}"></div>
        <div class="field"><label for="dt-ntp-port">Cổng NTP</label><input id="dt-ntp-port" type="number" min="1" max="65535" value="${Number(t.ntpPort || 123)}"></div>
        <div class="field"><label for="dt-ntp-period">Chu kỳ đồng bộ (phút)</label><input id="dt-ntp-period" type="number" min="1" value="${Number(t.updatePeriod || 60)}"></div>
      </div>
      <div class="checkbox-row"><input id="dt-ntp-enable" type="checkbox" ${t.ntpEnable ? 'checked' : ''}><label for="dt-ntp-enable">Bật tự động đồng bộ NTP</label></div>
      <p class="muted">Việt Nam trên firmware này: mã 12, tên Bangkok (UTC+7). Sai ngày giờ sẽ làm video xuất hiện nhầm ngày trên timeline.</p>
      <div class="row">
        <button class="btn btn-secondary" type="button" id="dt-browser-btn">Lấy giờ máy đang mở web</button>
        <button class="btn btn-primary" type="button" id="dt-save-btn">Lưu &amp; kiểm tra lại</button>
        <button class="btn" type="button" id="dt-refresh-btn">Đọc lại</button>
      </div>`;
    document.getElementById('dt-browser-btn').addEventListener('click', () => {
      document.getElementById('dt-value').value = localDateTime(new Date());
    });
    document.getElementById('dt-refresh-btn').addEventListener('click', () => renderDeviceTime(c));
    document.getElementById('dt-save-btn').addEventListener('click', () => saveDeviceTime(c));
    clearTimeout(deviceTimeRefreshTimer);
    deviceTimeRefreshTimer = setTimeout(() => renderDeviceTime(c), 60000);
  } catch (e) {
    el.innerHTML = `<p class="msg err">Lỗi đọc ngày giờ/NTP: ${escapeHtml(e.message)}</p>`;
    clearTimeout(deviceTimeRefreshTimer);
    deviceTimeRefreshTimer = setTimeout(() => renderDeviceTime(c), 60000);
  }
}

async function saveDeviceTime(c) {
  const value = document.getElementById('dt-value').value;
  const body = {
    id: c.id,
    currentTime: value ? value.replace('T', ' ') : '',
    ntpEnable: document.getElementById('dt-ntp-enable').checked,
    ntpAddress: document.getElementById('dt-ntp-address').value.trim(),
    ntpPort: parseInt(document.getElementById('dt-ntp-port').value, 10) || 123,
    updatePeriod: parseInt(document.getElementById('dt-ntp-period').value, 10) || 60,
    timeZone: parseInt(document.getElementById('dt-timezone').value, 10) || 0,
    timeZoneDesc: document.getElementById('dt-timezone-desc').value.trim(),
    timeoutSeconds: timeoutSec()
  };
  try {
    const saved = await api('/api/device-time', { method: 'POST', body: JSON.stringify(body) });
    showToast(`Đã lưu. Giờ thiết bị: ${saved.currentTime || body.currentTime}`, 'ok');
    await renderDeviceTime(c);
  } catch (e) {
    showToast('Lỗi lưu ngày giờ/NTP: ' + e.message, 'err');
  }
}

function localDateTime(d) {
  const p = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

// A recent-media check catches the common failure where the HDD is healthy
// but the NVR's channel schedule/manual record mode is disabled.
async function renderRecordHealth(c, selectedChannel = 0) {
  const el = document.getElementById('maint-record-health');
  if (!el) return;
  try {
    const h = await api(`/api/nvr/health?id=${encodeURIComponent(c.id)}`);
    const reasons = (h.reasons || []).map(x => escapeHtml(x.message || x.code)).join('<br>');
    const rows = (h.channels || []).map(ch => `<tr><td>K${Number(ch.channel)+1} · ${escapeHtml(ch.name || '')}</td><td>${ch.recordEnabled && ch.timing24x7 ? 'Bật 24/7' : '<b class="danger-text">Tắt / sai lịch</b>'}</td><td>${ch.latestEnd && !String(ch.latestEnd).startsWith('0001-') ? escapeHtml(new Date(ch.latestEnd).toLocaleString('vi-VN')) : 'Chưa có'}</td><td>${h.uptimeMinutes ? `uptime ${Number(h.uptimeMinutes)} phút · recorded ${Number(ch.recordedMinutes || 0)} phút · coverage ${Number(ch.coveragePercent || 0).toFixed(1)}%` : `Uptime không được firmware hỗ trợ · recorded 24h: ${Number(ch.recordedMinutes || 0)} phút`}</td><td><a data-testid="nvr-camera-link" href="#review/${encodeURIComponent(c.id)}/${Number(ch.channel)}">Xem lại</a></td></tr>`).join('');
    el.innerHTML = `<div data-testid="nvr-health-panel">
      <div class="msg ${h.status === 'healthy' ? 'ok' : 'err'}"><b>${h.status === 'healthy' ? 'Ghi hình tốt' : nvrStatusLabel(h.status)}</b>${reasons ? '<br>'+reasons : ''}</div>
      <div class="nvr-health-summary">HDD: <b>${h.storageHealthy ? 'Healthy' : 'Lỗi'}</b> · ${formatDiskCapacity(h.storageTotalBytes || 0)} · đang tăng dữ liệu: <b>${h.storageGrowing ? 'Có' : 'Chưa thấy'}</b><br>INUT: ${escapeHtml(h.hostTime || '?')} (${h.hostTimeTrusted ? 'NTP tốt' : 'chưa tin cậy'}) · NVR: ${escapeHtml(h.nvrTime || '?')} · lệch ${Math.abs(Number(h.clockDriftSeconds || 0))} giây</div>
      <div class="row"><button class="btn btn-primary" type="button" data-testid="nvr-check-now" id="nvr-check-now">Kiểm tra ngay</button><span class="muted">Lần cuối: ${escapeHtml(h.lastCheck || '?')} · lần tới: ${escapeHtml(h.nextCheck || 'watchdog tắt')}</span></div>
      <div class="table-wrap"><table class="reflow"><thead><tr><th>Kênh</th><th>Ghi</th><th>Clip mới nhất</th><th>Từ lúc bật đầu ghi</th><th></th></tr></thead><tbody>${rows || '<tr><td colspan="5">Không có kênh.</td></tr>'}</tbody></table></div>
    </div>`;
    document.getElementById('nvr-check-now').addEventListener('click', async ev => { await checkNvrHealth(c.id, ev.currentTarget); await renderRecordHealth(c, selectedChannel); });
  } catch (e) {
    el.innerHTML = `<div class="msg err">Không kiểm tra được sức khỏe NVR: ${escapeHtml(e.message)}.</div>`;
  }
}

// Playback lives in the dedicated timeline view; maintenance only links there.
function renderPlayback(c) {
  const el = document.getElementById('maint-playback');
  if (!el) return;
  el.innerHTML = `
    <p class="muted">Xem lại &amp; tải video (timeline, cắt đoạn, tải nhanh) đã chuyển sang trang riêng.</p>
    <button class="btn btn-primary" type="button" id="pb-open-review">Mở trang Xem lại →</button>
  `;
  document.getElementById('pb-open-review').addEventListener('click', () => {
    if (c && c.id) window._rvPreselect = c.id;
    goto('review');
  });
}

async function rebootDevice(c) {
  const label = c.name || c.host;
  const ok = await showConfirm('Khởi động lại', `Khởi động lại camera "${label}"? Camera sẽ mất kết nối ~30–60s.`, { danger: true, okLabel: 'Khởi động lại' });
  if (!ok) return;
  try {
    const res = await api('/api/reboot', { method: 'POST', body: JSON.stringify({ id: c.id, timeoutSeconds: timeoutSec() }) });
    showToast((res && res.note) || 'Đã gửi lệnh khởi động lại.', 'ok');
  } catch (e) { showToast('Lỗi khởi động lại: ' + e.message, 'err'); }
}

async function formatStorage(c, name) {
  const label = c.name || c.host;
  const ok = await showConfirm('Format bộ nhớ', `Format "${name}" trên thiết bị "${label}"? Toàn bộ dữ liệu ghi hình sẽ bị XOÁ SẠCH. Chỉ làm khi trạng thái báo cần format/lỗi.`, { danger: true, okLabel: 'Format (xoá sạch)' });
  if (!ok) return;
  try {
    const res = await api('/api/storage', { method: 'POST', body: JSON.stringify({ id: c.id, name, timeoutSeconds: timeoutSec() }) });
    showToast((res && res.note) || 'Đã gửi lệnh format.', 'ok');
    setTimeout(() => renderMaintenance(c), 4000);
  } catch (e) { showToast('Lỗi format: ' + e.message, 'err'); }
}

async function saveAutoReboot(c) {
  const enable = document.getElementById('ar-enable').checked;
  const hour = parseInt(document.getElementById('ar-hour').value, 10) || 0;
  const minute = parseInt(document.getElementById('ar-min').value, 10) || 0;
  try {
    await api('/api/autoreboot', { method: 'POST', body: JSON.stringify({ id: c.id, enable, day: 0, hour, minute, timeoutSeconds: timeoutSec() }) });
    showToast(enable ? `Đã bật tự khởi động lại ${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')} hằng ngày.` : 'Đã tắt tự khởi động lại.', 'ok');
  } catch (e) { showToast('Lỗi lưu lịch: ' + e.message, 'err'); }
}

function renderNetworkBody(net, wifi) {
  const body = document.getElementById('net-body');
  const ifaceNames = Object.keys(net.interfaces || {});
  if (!ifaceNames.length) {
    body.innerHTML = '<p class="muted">Không đọc được interface mạng nào.</p>';
    return;
  }
  const defaultIface = (net.defaultInterface && net.interfaces[net.defaultInterface]) ? net.defaultInterface : ifaceNames[0];
  const iface = net.interfaces[defaultIface];
  const dns = Array.isArray(iface.DnsServers) ? iface.DnsServers : [];

  let html = '';
  if (ifaceNames.length > 1) {
    html += `<div class="field field-sm"><label for="net-iface">Interface</label>
      <select id="net-iface">${ifaceNames.map(n => `<option value="${escapeHtml(n)}" ${n === defaultIface ? 'selected' : ''}>${escapeHtml(n)}</option>`).join('')}</select></div>`;
  } else {
    html += `<input type="hidden" id="net-iface" value="${escapeHtml(defaultIface)}">`;
  }
  html += `
    <div class="checkbox-row"><input type="checkbox" id="net-dhcp" ${iface.DhcpEnable ? 'checked' : ''}><label for="net-dhcp">Dùng DHCP (tự động lấy IP)</label></div>
    <div class="setting-body row" id="net-static-fields" ${iface.DhcpEnable ? 'hidden' : ''}>
      <div class="field field-sm"><label for="net-ip">Địa chỉ IP</label><input id="net-ip" value="${escapeHtml(iface.IPAddress || '')}" placeholder="192.168.1.10"></div>
      <div class="field field-sm"><label for="net-mask">Subnet mask</label><input id="net-mask" value="${escapeHtml(iface.SubnetMask || '')}" placeholder="255.255.255.0"></div>
      <div class="field field-sm"><label for="net-gw">Gateway</label><input id="net-gw" value="${escapeHtml(iface.DefaultGateway || '')}" placeholder="192.168.1.1"></div>
      <div class="field field-sm"><label for="net-dns1">DNS 1</label><input id="net-dns1" value="${escapeHtml(dns[0] || '')}" placeholder="8.8.8.8"></div>
      <div class="field field-sm"><label for="net-dns2">DNS 2</label><input id="net-dns2" value="${escapeHtml(dns[1] || '')}" placeholder="1.1.1.1"></div>
    </div>
    <p class="muted" id="net-macmtu">MAC: ${escapeHtml(iface.PhysicalAddress || '–')} · MTU: ${escapeHtml(String(iface.MTU == null ? '–' : iface.MTU))}</p>
    <div class="checkbox-row"><input type="checkbox" id="net-confirm-risk"><label for="net-confirm-risk">Tôi hiểu đổi IP/gateway sai có thể khiến camera mất kết nối, phải vào tận nơi để sửa lại.</label></div>
    <button class="btn btn-danger" type="button" id="net-save-btn" disabled>Lưu cấu hình mạng</button>
  `;

  html += `<div class="card-title section-gap">Wi-Fi</div>`;
  const wifiIfaceNames = wifi ? Object.keys(wifi) : [];
  if (!wifiIfaceNames.length) {
    html += `<p class="muted">Thiết bị không có/không đọc được cấu hình Wi-Fi (có thể không có ăng-ten Wi-Fi).</p>`;
  } else {
    const wifiIfaceName = wifiIfaceNames[0];
    const wifiIface = wifi[wifiIfaceName];
    html += `<input type="hidden" id="net-wifi-iface" value="${escapeHtml(wifiIfaceName)}">`;
    html += `
      <div class="field field-sm"><label for="net-wifi-ssid">SSID</label><input id="net-wifi-ssid" value="${escapeHtml(wifiIface.SSID || '')}"></div>
      <div class="field field-sm"><label for="net-wifi-pass">Mật khẩu Wi-Fi (để trống = giữ nguyên)</label><input id="net-wifi-pass" type="text" placeholder="nhập mật khẩu Wi-Fi mới"></div>
      <div class="row"><button class="btn btn-secondary" type="button" id="net-wifi-scan-btn">Quét Wi-Fi</button></div>
      <div id="net-wifi-scan-results"></div>
      <div class="checkbox-row"><input type="checkbox" id="net-wifi-confirm-risk"><label for="net-wifi-confirm-risk">Tôi hiểu đổi Wi-Fi sai có thể khiến camera mất kết nối.</label></div>
      <button class="btn btn-danger" type="button" id="net-wifi-save-btn" disabled>Lưu Wi-Fi</button>
    `;
  }
  body.innerHTML = html;

  const dhcpCb = document.getElementById('net-dhcp');
  dhcpCb.addEventListener('change', () => { document.getElementById('net-static-fields').hidden = dhcpCb.checked; });

  // Switching the interface dropdown reloads every field with that interface's
  // own config (previously the form kept showing the default interface's IP).
  const ifaceSel = document.getElementById('net-iface');
  if (ifaceSel && ifaceSel.tagName === 'SELECT') {
    ifaceSel.addEventListener('change', () => {
      const ic = net.interfaces[ifaceSel.value] || {};
      const idns = Array.isArray(ic.DnsServers) ? ic.DnsServers : [];
      dhcpCb.checked = !!ic.DhcpEnable;
      document.getElementById('net-static-fields').hidden = !!ic.DhcpEnable;
      document.getElementById('net-ip').value = ic.IPAddress || '';
      document.getElementById('net-mask').value = ic.SubnetMask || '';
      document.getElementById('net-gw').value = ic.DefaultGateway || '';
      document.getElementById('net-dns1').value = idns[0] || '';
      document.getElementById('net-dns2').value = idns[1] || '';
      const macmtu = document.getElementById('net-macmtu');
      if (macmtu) macmtu.textContent = `MAC: ${ic.PhysicalAddress || '–'} · MTU: ${ic.MTU == null ? '–' : ic.MTU}`;
    });
  }
  const netConfirm = document.getElementById('net-confirm-risk');
  const netSaveBtn = document.getElementById('net-save-btn');
  netConfirm.addEventListener('change', () => { netSaveBtn.disabled = !netConfirm.checked; });
  netSaveBtn.addEventListener('click', saveStaticIP);

  if (wifiIfaceNames.length) {
    const wifiConfirm = document.getElementById('net-wifi-confirm-risk');
    const wifiSaveBtn = document.getElementById('net-wifi-save-btn');
    wifiConfirm.addEventListener('change', () => { wifiSaveBtn.disabled = !wifiConfirm.checked; });
    wifiSaveBtn.addEventListener('click', saveWiFi);
    document.getElementById('net-wifi-scan-btn').addEventListener('click', scanWiFi);
  }
}

async function saveStaticIP() {
  if (!networkEditDevice) return;
  const label = networkEditDevice.name || networkEditDevice.host;
  const ok = await showConfirm('Đổi cấu hình mạng', `Xác nhận đổi cấu hình mạng camera "${label}"? Nếu IP/gateway sai, camera có thể mất kết nối và cần vào tận nơi để sửa lại.`, { danger: true, okLabel: 'Đổi IP' });
  if (!ok) return;
  const btn = document.getElementById('net-save-btn');
  const msg = document.getElementById('net-msg');
  const dhcpEnable = document.getElementById('net-dhcp').checked;
  const body = {
    id: networkEditDevice.id,
    interface: document.getElementById('net-iface').value,
    dhcpEnable,
    ipAddress: document.getElementById('net-ip').value.trim(),
    subnetMask: document.getElementById('net-mask').value.trim(),
    gateway: document.getElementById('net-gw').value.trim(),
    dns: [document.getElementById('net-dns1').value.trim(), document.getElementById('net-dns2').value.trim()].filter(Boolean),
    timeoutSeconds: timeoutSec(),
  };
  setBusy(btn, true, 'Đang lưu...');
  msg.textContent = ''; msg.className = 'msg';
  try {
    const res = await api('/api/network', { method: 'POST', body: JSON.stringify(body) });
    const note = res && res.note ? res.note : 'Đã lưu cấu hình mạng.';
    msg.textContent = note;
    msg.className = 'msg ok';
    showToast(note, 'ok');
  } catch (e) {
    msg.textContent = 'Lỗi: ' + e.message;
    msg.className = 'msg err';
    showToast('Lỗi lưu mạng: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
}

async function saveWiFi() {
  if (!networkEditDevice) return;
  const ssid = document.getElementById('net-wifi-ssid').value.trim();
  if (!ssid) { showToast('Cần nhập SSID.', 'err'); return; }
  const label = networkEditDevice.name || networkEditDevice.host;
  const ok = await showConfirm('Đổi Wi-Fi', `Xác nhận đổi Wi-Fi camera "${label}" sang SSID "${ssid}"? Nếu sai, camera có thể mất kết nối.`, { danger: true, okLabel: 'Đổi Wi-Fi' });
  if (!ok) return;
  const btn = document.getElementById('net-wifi-save-btn');
  const msg = document.getElementById('net-msg');
  const body = {
    id: networkEditDevice.id,
    interface: document.getElementById('net-wifi-iface').value,
    ssid,
    password: document.getElementById('net-wifi-pass').value,
    timeoutSeconds: timeoutSec(),
  };
  setBusy(btn, true, 'Đang lưu...');
  msg.textContent = ''; msg.className = 'msg';
  try {
    await api('/api/wifi', { method: 'POST', body: JSON.stringify(body) });
    msg.textContent = 'Đã lưu Wi-Fi.';
    msg.className = 'msg ok';
    showToast('Đã lưu Wi-Fi.', 'ok');
  } catch (e) {
    msg.textContent = 'Lỗi: ' + e.message;
    msg.className = 'msg err';
    showToast('Lỗi lưu Wi-Fi: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
}

async function scanWiFi() {
  if (!networkEditDevice) return;
  const btn = document.getElementById('net-wifi-scan-btn');
  const results = document.getElementById('net-wifi-scan-results');
  setBusy(btn, true, 'Đang quét...');
  results.innerHTML = '<p class="muted">Đang quét Wi-Fi (có thể mất vài giây)...</p>';
  try {
    const res = await api('/api/wifi-scan', { method: 'POST', body: JSON.stringify({ id: networkEditDevice.id, timeoutSeconds: timeoutSec() }) });
    const devices = res.devices || [];
    if (!devices.length) {
      results.innerHTML = '<p class="muted">Không tìm thấy mạng Wi-Fi nào.</p>';
    } else {
      results.innerHTML = '<div class="chip-list">' + devices.map(d => {
        const q = d.linkQuality || 0;
        const cls = q >= 70 ? 'active-high' : q >= 40 ? 'active-med' : 'active-low';
        const h1 = q >= 25 ? cls : '';
        const h2 = q >= 50 ? cls : '';
        const h3 = q >= 75 ? cls : '';
        const h4 = q >= 90 ? cls : '';
        return `<button type="button" class="chip chip-btn wifi-rssi-meter" data-wifi-ssid="${escapeHtml(d.ssid)}">
          <span class="wifi-signal-bars">
            <span class="wifi-signal-bar ${h1}" style="height:4px"></span>
            <span class="wifi-signal-bar ${h2}" style="height:7px"></span>
            <span class="wifi-signal-bar ${h3}" style="height:10px"></span>
            <span class="wifi-signal-bar ${h4}" style="height:14px"></span>
          </span>
          <span>${escapeHtml(d.ssid)}</span>
          <span class="muted mono">(${q}%)</span>
        </button>`;
      }).join('') + '</div>';
      results.querySelectorAll('[data-wifi-ssid]').forEach(el => {
        el.addEventListener('click', () => { document.getElementById('net-wifi-ssid').value = el.dataset.wifiSsid; });
      });
    }
  } catch (e) {
    results.innerHTML = `<p class="msg err">Lỗi quét: ${escapeHtml(e.message)}</p>`;
  } finally {
    setBusy(btn, false);
  }
}

// startInlineRename swaps the Tên cell's text for an input; Enter/blur saves
// (via the existing POST /api/cameras upsert — inventory label only, never
// touches the device), Escape cancels. Guards against a double-submit when
// Enter's save is immediately followed by the resulting blur.
function startInlineRename(cell, id) {
  if (!cell) return;
  const c = cameras.find(x => x.id === id);
  if (!c) return;
  const oldName = c.name || '';
  cell.innerHTML = `<input class="cell-name-input" value="${escapeHtml(oldName)}">`;
  const input = cell.querySelector('input');
  input.focus();
  input.select();
  let done = false;
  const finish = async (save) => {
    if (done) return;
    done = true;
    const newName = input.value.trim();
    if (!save || newName === oldName) { renderCameras(); return; }
    try {
      await api('/api/cameras', {
        method: 'POST',
        body: JSON.stringify({
          id: c.id, name: newName, host: c.host, port: c.port,
          vendor: c.vendor, username: c.username, password: '',
        }),
      });
      showToast('Đã đổi tên.', 'ok');
      await loadCameras();
    } catch (e) {
      showToast('Lỗi đổi tên: ' + e.message, 'err');
      renderCameras();
    }
  };
  input.addEventListener('keydown', (kev) => {
    if (kev.key === 'Enter') finish(true);
    else if (kev.key === 'Escape') finish(false);
  });
  input.addEventListener('blur', () => finish(true));
}

document.querySelector('#cam-table thead').addEventListener('click', (ev) => {
  const th = ev.target.closest('th.sortable');
  if (!th) return;
  const key = th.dataset.sort;
  if (camSort.key === key) camSort.dir = -camSort.dir;
  else camSort = { key, dir: 1 };
  renderCameras();
});

document.getElementById('select-all').addEventListener('change', (ev) => {
  const isChecked = ev.target.checked;
  const visibleChecks = document.querySelectorAll('.cam-cb');
  if (visibleChecks.length) {
    visibleChecks.forEach(cb => {
      if (isChecked) selectedCameraSet.add(cb.value);
      else selectedCameraSet.delete(cb.value);
    });
  } else {
    cameras.forEach(c => {
      if (isChecked) selectedCameraSet.add(c.id);
      else selectedCameraSet.delete(c.id);
    });
  }
  document.querySelectorAll('.cam-cb, .bulk-cam-cb, .cam-card-cb').forEach(cb => {
    cb.checked = selectedCameraSet.has(cb.value);
  });
  document.querySelectorAll('.cam-card').forEach(card => {
    const id = card.dataset.id;
    card.classList.toggle('selected', id ? selectedCameraSet.has(id) : isChecked);
  });
  renderBulkSelection();
});

document.getElementById('cam-tbody').addEventListener('change', (ev) => {
  if (ev.target.classList.contains('cam-cb')) setCameraSelected(ev.target.value, ev.target.checked);
});

/* ---------- bulk-edit form wiring ---------- */

// One spec per bulk setting, instead of six near-identical wireToggle calls
// plus six hand-written summary strings. `summary` renders the chip shown in
// the "Sẽ đổi" bar so the tab answers "what am I about to change?" without
// scrolling through every card.
const BULK_SETTINGS = [
  { key: 'codec', enable: 'p-codec-enable', fields: 'p-codec-fields',
    summary: () => 'Codec ' + document.getElementById('p-codec-value').value },
  { key: 'res', enable: 'p-res-enable', fields: 'p-res-fields',
    summary: () => `Độ phân giải ${document.getElementById('p-width').value}x${document.getElementById('p-height').value}` },
  { key: 'smart', enable: 'p-smart-enable', fields: 'p-smart-fields',
    summary: () => 'Smart Codec ' + (document.getElementById('p-smart-value').value === 'on' ? 'bật' : 'tắt') },
  { key: 'gop', enable: 'p-gop-enable', fields: 'p-gop-fields',
    summary: () => 'GOP ' + document.getElementById('p-gop-value').value },
  { key: 'bitrate', enable: 'p-bitrate-enable', fields: 'p-bitrate-fields',
    summary: () => {
      const mode = document.getElementById('p-bitrate-mode').value;
      return `Bitrate ${document.getElementById('p-bitrate-value').value} Kbps${mode ? ' ' + mode : ''}`;
    } },
  { key: 'osd', enable: 'p-osd-enable', fields: 'p-osd-fields',
    summary: () => {
      const n = ['p-osd-line1', 'p-osd-line2'].filter(id => document.getElementById(id).value.trim()).length;
      return n ? `OSD ${n} dòng` : 'Xoá OSD';
    } },
  { key: 'audio', enable: 'p-audio-enable', fields: null, summary: () => 'Bật âm thanh AAC' },
];

function checkBulkSafety() {
  const alertEl = document.getElementById('bulk-safety-alert');
  if (!alertEl) return;
  const warnings = [];

  const resEnabled = document.getElementById('p-res-enable') && document.getElementById('p-res-enable').checked;
  const w = parseInt(document.getElementById('p-width')?.value, 10) || 0;
  const h = parseInt(document.getElementById('p-height')?.value, 10) || 0;

  const bitrateEnabled = document.getElementById('p-bitrate-enable') && document.getElementById('p-bitrate-enable').checked;
  const bitrate = parseInt(document.getElementById('p-bitrate-value')?.value, 10) || 0;

  const gopEnabled = document.getElementById('p-gop-enable') && document.getElementById('p-gop-enable').checked;
  const gop = parseInt(document.getElementById('p-gop-value')?.value, 10) || 0;

  if (bitrateEnabled && bitrate > 8192) {
    warnings.push(`Bitrate ${bitrate} Kbps quá cao (vượt ngưỡng 8192 Kbps an toàn), có thể gây nghẽn băng thông switch/NVR.`);
  }
  if (resEnabled && (w >= 3840 || h >= 2160) && bitrateEnabled && bitrate < 2048) {
    warnings.push('Độ phân giải 4K (3840x2160) với Bitrate quá thấp (< 2048 Kbps) có thể làm vỡ hạt khung hình.');
  }
  if (gopEnabled && gop > 200) {
    warnings.push(`Khoảng I-frame GOP ${gop} quá lớn (khuyến nghị 50-100), sẽ làm tăng độ trễ khi xem trực tiếp.`);
  }

  if (warnings.length) {
    alertEl.hidden = false;
    alertEl.innerHTML = `<span>⚠️ <b>Cảnh báo an toàn phần cứng:</b> ${warnings.map(escapeHtml).join(' · ')}</span>`;
  } else {
    alertEl.hidden = true;
    alertEl.innerHTML = '';
  }
}

function applyGoldenTemplate() {
  const cEnable = document.getElementById('p-codec-enable');
  const cVal = document.getElementById('p-codec-value');
  const cFields = document.getElementById('p-codec-fields');
  if (cEnable) {
    cEnable.checked = true;
    if (cFields) cFields.hidden = false;
    if (cVal) cVal.value = 'H.264';
  }

  const rEnable = document.getElementById('p-res-enable');
  const rFields = document.getElementById('p-res-fields');
  const rPreset = document.getElementById('p-res-preset');
  const wInput = document.getElementById('p-width');
  const hInput = document.getElementById('p-height');
  if (rEnable) {
    rEnable.checked = true;
    if (rFields) rFields.hidden = false;
    if (rPreset) rPreset.value = '1920x1080';
    if (wInput) wInput.value = '1920';
    if (hInput) hInput.value = '1080';
  }

  const gEnable = document.getElementById('p-gop-enable');
  const gFields = document.getElementById('p-gop-fields');
  const gVal = document.getElementById('p-gop-value');
  if (gEnable) {
    gEnable.checked = true;
    if (gFields) gFields.hidden = false;
    if (gVal) gVal.value = '50';
  }

  const bEnable = document.getElementById('p-bitrate-enable');
  const bFields = document.getElementById('p-bitrate-fields');
  const bVal = document.getElementById('p-bitrate-value');
  const bMode = document.getElementById('p-bitrate-mode');
  if (bEnable) {
    bEnable.checked = true;
    if (bFields) bFields.hidden = false;
    if (bVal) bVal.value = '2048';
    if (bMode) bMode.value = 'CBR';
  }

  const aEnable = document.getElementById('p-audio-enable');
  if (aEnable) {
    aEnable.checked = true;
  }

  renderBulkSummary();
  showToast('⚡ Đã nạp cấu hình Chuẩn Bida (Golden Template)!', 'ok');
}

function renderBulkSummary() {
  const chips = BULK_SETTINGS
    .filter(sp => document.getElementById(sp.enable).checked)
    .map(sp => `<button type="button" class="chip chip-btn" data-bulk-jump="setting-${sp.key}">${escapeHtml(sp.summary())}</button>`);
  document.getElementById('bulk-summary-chips').innerHTML = chips.join('');
  document.getElementById('bulk-summary-empty').hidden = chips.length > 0;
  checkBulkSafety();
}

document.getElementById('bulk-golden-template-btn')?.addEventListener('click', applyGoldenTemplate);

BULK_SETTINGS.forEach(sp => {
  const enable = document.getElementById(sp.enable);
  const fields = sp.fields ? document.getElementById(sp.fields) : null;
  enable.addEventListener('change', () => {
    if (fields) fields.hidden = !enable.checked;
    renderBulkSummary();
  });
});
// Any value edit inside the bulk panel refreshes the chips' text.
document.querySelectorAll('[data-camera-panel="bulk"]').forEach(panel => {
  panel.addEventListener('input', renderBulkSummary);
  panel.addEventListener('change', renderBulkSummary);
});
document.getElementById('bulk-summary-chips').addEventListener('click', (ev) => {
  const chip = ev.target.closest('[data-bulk-jump]');
  if (!chip) return;
  const target = document.getElementById(chip.dataset.bulkJump);
  if (target) target.scrollIntoView({ behavior: 'smooth', block: 'center' });
});

const codecEnable = document.getElementById('p-codec-enable');
const resEnable = document.getElementById('p-res-enable');
const widthInput = document.getElementById('p-width');
const heightInput = document.getElementById('p-height');
const smartEnable = document.getElementById('p-smart-enable');
const gopEnable = document.getElementById('p-gop-enable');
const bitrateEnable = document.getElementById('p-bitrate-enable');

document.getElementById('p-res-preset').addEventListener('change', (ev) => {
  if (ev.target.value === 'custom') return;
  const [w, h] = ev.target.value.split('x').map(Number);
  widthInput.value = w;
  heightInput.value = h;
});

// probe every selected camera sequentially (gentle on slow DVRs), updating
// each row's stream-info cell + the probe cache.
document.getElementById('probe-selected-btn').addEventListener('click', async () => {
  const ids = selectedCameraIds();
  const msg = document.getElementById('apply-msg');
  if (!ids.length) { msg.className = 'msg err'; msg.textContent = 'Chưa chọn camera nào để dò.'; return; }
  const btn = document.getElementById('probe-selected-btn');
  setBusy(btn, true, 'Đang dò...');
  let ok = 0, fail = 0;
  for (let i = 0; i < ids.length; i++) {
    const id = ids[i];
    msg.className = 'msg'; msg.textContent = `Đang dò ${i + 1}/${ids.length}: ${id} ...`;
    setProgress(i + 1, ids.length, `Đang dò ${i + 1}/${ids.length}`);
    const cell = document.getElementById('probe-' + cssEscape(id));
    if (cell) cell.innerHTML = '<span class="muted">đang dò...</span>';
    try {
      rememberProbeResult(id, await api('/api/probe', { method: 'POST', body: JSON.stringify({ id, timeoutSeconds: timeoutSec() }) }));
      renderCameras();
      ok++;
    } catch (e) {
      if (cell) cell.innerHTML = `<span class="msg err">${escapeHtml(e.message)}</span>`;
      fail++;
    }
  }
  setProgress(null);
  msg.className = fail ? 'msg err' : 'msg ok';
  msg.textContent = `Dò xong: ${ok} OK, ${fail} lỗi.`;
  showToast(`Dò xong: ${ok} OK, ${fail} lỗi.`, fail ? 'err' : 'ok');
  setBusy(btn, false);
});

/* ---------- snapshot gallery ---------- */

// buildTiles expands a list of camera ids by the current "Kênh" spec +
// stream picker (both already used by the bulk-edit profile), so "Xem hình"
// (one row) and "Xem hình hàng loạt" (selected rows) share one code path.
function buildTiles(ids) {
  const channels = parseChannels(document.getElementById('p-channel').value);
  const streamsSel = Array.from(document.querySelectorAll('.stream-cb:checked')).map(cb => parseInt(cb.value, 10));
  const streams = streamsSel.length ? streamsSel : [0];
  const tiles = [];
  for (const id of ids) {
    const c = cameras.find(x => x.id === id);
    const camName = c ? (c.name || c.host) : id;
    for (const ch of channels) {
      for (const st of streams) {
        tiles.push({ camId: id, camName, channel: ch, stream: st, streamLabel: ['main', 'sub1', 'sub2'][st] || ('s' + st) });
      }
    }
  }
  return tiles;
}

// viewAllChannels probes a device to discover every channel it reports (main
// stream of each), then opens the gallery over that full channel grid — the
// NVR use case, independent of the "Kênh" spec field.
async function viewAllChannels(id, btn) {
  const c = cameras.find(x => x.id === id);
  const camName = c ? (c.name || c.host) : id;
  if (btn) btn.disabled = true;
  try {
    const info = probeCache[id] || rememberProbeResult(id, await api('/api/probe', { method: 'POST', body: JSON.stringify({ id, timeoutSeconds: timeoutSec() }) }));
    const channels = Array.from(new Set(info.map(s => s.channel))).sort((a, b) => a - b);
    if (!channels.length) { showToast('Không tìm thấy kênh nào (dò thử trước).', 'err'); return; }
    // StreamInfo.channel is 1-based; the snapshot API's channel param is
    // 0-based (matches Profile.Channel).
    openGallery(channels.map(ch => ({ camId: id, camName, channel: ch - 1, stream: 0, streamLabel: 'main' })));
  } catch (e) {
    showToast('Lỗi dò kênh: ' + e.message, 'err');
  } finally {
    if (btn) btn.disabled = false;
  }
}

let galleryTiles = [];
let galleryTileURLs = []; // current blob: URL per tile index (null until loaded), so the lightbox can reuse it without re-fetching
let galleryObjectURLs = []; // every blob: URL ever handed out, revoked on next open/close to avoid leaks

function revokeGalleryURLs() {
  galleryObjectURLs.forEach(u => URL.revokeObjectURL(u));
  galleryObjectURLs = [];
}

function openGallery(tiles) {
  if (!tiles.length) { showToast('Không có kênh/camera nào để xem hình.', 'err'); return; }
  galleryTiles = tiles;
  galleryTileURLs = tiles.map(() => null);
  revokeGalleryURLs();
  const grid = document.getElementById('gallery-grid');
  grid.innerHTML = tiles.map((t, i) => `
    <div class="gallery-tile gallery-tile-loading" data-idx="${i}">
      <div class="gallery-tile-img-wrap"><span class="spinner"></span></div>
      <div class="gallery-tile-label">${escapeHtml(t.camName)} · K${t.channel + 1} ${t.streamLabel}</div>
      <button class="btn btn-secondary btn-block" data-gallery-edit="${i}">Sửa tên &amp; OSD</button>
    </div>
  `).join('');
  document.getElementById('gallery-dialog').showModal();
  loadGalleryTilesBatched(tiles);
}

// GALLERY_BATCH_SIZE caps how many snapshot fetches run at once. Loading all
// tiles in parallel can overwhelm an embedded camera/NVR's small HTTP server
// (a "Tất cả kênh" grid can have dozens of tiles); loading one at a time is
// gentle but slow for a large NVR. Batches of 4 balance the two — matches
// the "gentle on slow devices" philosophy used elsewhere (probe-selected,
// bulk apply) without being needlessly slow.
const GALLERY_BATCH_SIZE = 4;

async function loadGalleryTilesBatched(tiles) {
  for (let i = 0; i < tiles.length; i += GALLERY_BATCH_SIZE) {
    if (!document.getElementById('gallery-dialog').open) return; // closed mid-loop
    const batch = tiles.slice(i, i + GALLERY_BATCH_SIZE);
    await Promise.all(batch.map((t, j) => loadGalleryTile(i + j, t, false)));
  }
}

// loadGalleryTile fetches the JPEG itself (rather than a plain <img src=...>)
// so a failure's actual server error text (device unreachable, wrong port,
// etc.) can be shown right in the tile — this is a debug tool, a generic
// "couldn't load" isn't useful. cacheBust forces a fresh request on retry.
async function loadGalleryTile(i, t, cacheBust) {
  const tile = document.querySelector(`#gallery-grid .gallery-tile[data-idx="${i}"]`);
  if (!tile) return;
  const wrap = tile.querySelector('.gallery-tile-img-wrap');
  tile.classList.remove('gallery-tile-error');
  tile.classList.add('gallery-tile-loading');
  galleryTileURLs[i] = null;
  wrap.innerHTML = '<span class="spinner"></span>';
  let q = `id=${encodeURIComponent(t.camId)}&channel=${t.channel}&stream=${t.stream}&timeoutSeconds=${timeoutSec()}`;
  if (cacheBust) q += '&_r=' + Date.now();
  try {
    const resp = await fetch('/api/snapshot?' + q);
    if (resp.status === 401) { location.href = '/login'; return; }
    if (!resp.ok) {
      const text = await resp.text().catch(() => '');
      let msg = text || resp.statusText;
      try { const j = JSON.parse(text); if (j && j.error) msg = j.error; } catch (e) { /* not JSON */ }
      throw new Error(msg);
    }
    const blob = await resp.blob();
    const url = URL.createObjectURL(blob);
    galleryObjectURLs.push(url);
    galleryTileURLs[i] = url;
    wrap.innerHTML = `<img src="${url}" alt="${escapeHtml(t.camName)} K${t.channel + 1} ${escapeHtml(t.streamLabel)}">` +
      `<button class="gallery-tile-reload" data-tile-reload="${i}" title="Tải lại" aria-label="Tải lại ảnh">${ICONS.reload}</button>`;
    tile.classList.remove('gallery-tile-loading');
    // If the lightbox is currently showing this same tile, refresh it too.
    if (document.getElementById('lightbox-dialog').open && lightboxIdx === i) {
      document.getElementById('lightbox-img').src = url;
    }
  } catch (e) {
    tile.classList.remove('gallery-tile-loading');
    tile.classList.add('gallery-tile-error');
    wrap.innerHTML = `<div class="gallery-tile-err-msg">${escapeHtml(e.message)}</div><div class="gallery-tile-retry">Bấm để thử lại</div>`;
  }
}

document.getElementById('gallery-grid').addEventListener('click', (ev) => {
  const editBtn = ev.target.closest('[data-gallery-edit]');
  if (editBtn) {
    const tile = galleryTiles[parseInt(editBtn.dataset.galleryEdit, 10)];
    closeDialog(document.getElementById('gallery-dialog'));
    // The channel list is fetched asynchronously, so hand the wanted channel to
    // openCameraDetail rather than poking the <select> before it has options.
    pendingDetailChannel = tile.channel || 0;
    gotoCameraDetail(tile.camId, 'osd');
    return;
  }
  const reloadBtn = ev.target.closest('[data-tile-reload]');
  if (reloadBtn) {
    const i = parseInt(reloadBtn.dataset.tileReload, 10);
    loadGalleryTile(i, galleryTiles[i], true);
    return;
  }
  const errTile = ev.target.closest('.gallery-tile.gallery-tile-error');
  if (errTile && ev.target.closest('.gallery-tile-img-wrap')) {
    const i = parseInt(errTile.dataset.idx, 10);
    loadGalleryTile(i, galleryTiles[i], true);
    return;
  }
  const img = ev.target.closest('.gallery-tile-img-wrap img');
  if (img) {
    const tile = img.closest('.gallery-tile');
    openLightbox(parseInt(tile.dataset.idx, 10));
  }
});

document.getElementById('gallery-close').addEventListener('click', () => document.getElementById('gallery-dialog').close());
document.getElementById('gallery-dialog').addEventListener('close', revokeGalleryURLs);

/* ---------- lightbox (click a tile to view full-size) ---------- */

let lightboxIdx = null;

function openLightbox(i) {
  const url = galleryTileURLs[i];
  if (!url) return;
  lightboxIdx = i;
  const t = galleryTiles[i];
  document.getElementById('lightbox-img').src = url;
  document.getElementById('lightbox-label').textContent = `${t.camName} · K${t.channel + 1} ${t.streamLabel}`;
  document.getElementById('lightbox-dialog').showModal();
}

document.getElementById('lightbox-reload').addEventListener('click', () => {
  if (lightboxIdx == null) return;
  loadGalleryTile(lightboxIdx, galleryTiles[lightboxIdx], true);
});
document.getElementById('lightbox-close').addEventListener('click', () => document.getElementById('lightbox-dialog').close());
document.getElementById('lightbox-dialog').addEventListener('click', (ev) => {
  if (ev.target.id === 'lightbox-dialog') document.getElementById('lightbox-dialog').close(); // backdrop click
});

/* ---------- per-channel name/OSD edit panel ---------- */

let channelEditTile = null;
let ceActiveTab = 'name';
let picturePayload = null; // { color, options } as last fetched from /api/picture, for diffing on save
let pictureMode = 'lite'; // 'lite' (curated WB/Flip/Rotate/DayNightColor) or 'full' (generic editor)

// switchPictureMode toggles between the curated "Cơ bản" panel and the full
// generic settings editor within the Chỉnh màu tab. Both panels stay
// rendered simultaneously (loadPictureTab fills both); this just shows one.
function switchPictureMode(mode) {
  pictureMode = mode;
  document.querySelectorAll('#ce-picture-mode-tabs .tab-btn').forEach(b => b.classList.toggle('active', b.dataset.pfMode === mode));
  document.getElementById('ce-picture-lite-body').hidden = mode !== 'lite';
  document.getElementById('ce-picture-body').hidden = mode !== 'full';
  document.getElementById('ce-picture-full-hint').hidden = mode !== 'full';
}

document.getElementById('ce-picture-mode-tabs').addEventListener('click', (ev) => {
  const btn = ev.target.closest('.tab-btn');
  if (btn) switchPictureMode(btn.dataset.pfMode);
});

// Live-update a range slider's <output> readout as it's dragged. Delegated
// on the lite panel so it covers sliders rendered later (backlight family).
document.getElementById('ce-picture-lite-body').addEventListener('input', (ev) => {
  if (ev.target.type !== 'range') return;
  const out = ev.target.parentElement.querySelector('.pf-range-out');
  if (out) out.textContent = ev.target.value;
});

/* ---------- PTZ pad (Dahua) ---------- */

// sendPTZ fires one /api/ptz command. start=true begins motion, false stops
// it; failures surface in the PTZ tab's message line (a stop that fails is
// logged but not surfaced, to avoid masking the more useful start error).
async function sendPTZ(code, start) {
  if (!channelEditTile) return;
  try {
    await api('/api/ptz', {
      method: 'POST',
      body: JSON.stringify({
        id: channelEditTile.camId, channel: channelEditTile.channel, code,
        speed: parseInt(document.getElementById('ce-ptz-speed').value, 10) || 5,
        start, timeoutSeconds: timeoutSec(),
      }),
    });
    if (start) { const m = document.getElementById('ce-ptz-msg'); m.textContent = ''; m.className = 'msg'; }
  } catch (e) {
    if (start) {
      const m = document.getElementById('ce-ptz-msg');
      m.textContent = 'Lỗi PTZ: ' + e.message + ' (camera này có cơ cấu PTZ không?)';
      m.className = 'msg err';
    }
  }
}

// Wire every [data-ptz] button as press-and-hold: pointerdown starts motion,
// pointerup/leave/cancel stops it. ptzActive guards against a missed "up"
// (e.g. pointer left the button) sending a stop for a code that never
// started.
let ptzActive = null;
function ptzStart(code) {
  if (ptzActive) return;
  ptzActive = code;
  sendPTZ(code, true);
}
function ptzStop() {
  if (!ptzActive) return;
  const code = ptzActive;
  ptzActive = null;
  sendPTZ(code, false);
}
document.getElementById('ce-panel-ptz').addEventListener('pointerdown', (ev) => {
  const btn = ev.target.closest('[data-ptz]');
  if (!btn) return;
  ev.preventDefault();
  btn.setPointerCapture?.(ev.pointerId);
  ptzStart(btn.dataset.ptz);
});
document.getElementById('ce-panel-ptz').addEventListener('pointerup', ptzStop);
document.getElementById('ce-panel-ptz').addEventListener('pointercancel', ptzStop);
document.getElementById('ce-panel-ptz').addEventListener('pointerleave', ptzStop);
// Belt-and-suspenders: if the user routes away mid-press, stop the camera.
window.addEventListener('hashchange', ptzStop);

/* ---------- Quick PTZ Dialog controller & PTZ keyboard navigation ---------- */
let quickPtzCam = null;
let quickPtzLive = null;
let quickPtzActive = null;

function initQuickPtzDialog() {
  const dlg = document.getElementById('quick-ptz-dialog');
  if (!dlg) return;
  const liveImg = document.getElementById('quick-ptz-live');
  const startBtn = document.getElementById('quick-ptz-live-start');
  const stopBtn = document.getElementById('quick-ptz-live-stop');
  const pad = document.getElementById('quick-ptz-pad');
  const speedInput = document.getElementById('quick-ptz-speed');

  quickPtzLive = livePreview({
    img: liveImg,
    start: startBtn,
    stop: stopBtn,
  }, () => quickPtzCam ? { id: quickPtzCam.id, channel: 0 } : null);

  document.getElementById('quick-ptz-close')?.addEventListener('click', () => {
    if (quickPtzLive) quickPtzLive.stop();
    closeDialog(dlg);
  });
  document.getElementById('quick-ptz-goto-detail')?.addEventListener('click', () => {
    if (quickPtzCam) {
      const id = quickPtzCam.id;
      if (quickPtzLive) quickPtzLive.stop();
      closeDialog(dlg);
      gotoCameraDetail(id, 'ptz');
    }
  });

  const sendQuickPTZ = (code, start) => {
    if (!quickPtzCam) return;
    api('/api/ptz', {
      method: 'POST',
      body: JSON.stringify({
        id: quickPtzCam.id,
        channel: 0,
        code,
        speed: parseInt(speedInput?.value, 10) || 5,
        start,
        timeoutSeconds: timeoutSec(),
      }),
    }).catch(() => {});
  };

  if (pad) {
    pad.addEventListener('pointerdown', (ev) => {
      const btn = ev.target.closest('[data-ptz]');
      if (!btn) return;
      ev.preventDefault();
      btn.setPointerCapture?.(ev.pointerId);
      quickPtzActive = btn.dataset.ptz;
      sendQuickPTZ(quickPtzActive, true);
    });
    pad.addEventListener('pointerup', () => {
      if (!quickPtzActive) return;
      const code = quickPtzActive;
      quickPtzActive = null;
      sendQuickPTZ(code, false);
    });
    pad.addEventListener('pointercancel', () => {
      if (!quickPtzActive) return;
      const code = quickPtzActive;
      quickPtzActive = null;
      sendQuickPTZ(code, false);
    });
  }

  dlg.querySelectorAll('.quick-ptz-controls button[data-ptz]').forEach(btn => {
    if (btn.closest('#quick-ptz-pad')) return;
    btn.addEventListener('pointerdown', (ev) => {
      ev.preventDefault();
      btn.setPointerCapture?.(ev.pointerId);
      quickPtzActive = btn.dataset.ptz;
      sendQuickPTZ(quickPtzActive, true);
    });
    btn.addEventListener('pointerup', () => {
      if (!quickPtzActive) return;
      const code = quickPtzActive;
      quickPtzActive = null;
      sendQuickPTZ(code, false);
    });
    btn.addEventListener('pointercancel', () => {
      if (!quickPtzActive) return;
      const code = quickPtzActive;
      quickPtzActive = null;
      sendQuickPTZ(code, false);
    });
  });
}

function openQuickPtz(c) {
  quickPtzCam = c;
  const dlg = document.getElementById('quick-ptz-dialog');
  if (!dlg) return;
  const title = document.getElementById('quick-ptz-title');
  if (title) title.textContent = `Điều khiển PTZ nhanh — ${c.name || c.host}`;
  const wrap = document.getElementById('quick-ptz-img-wrap');
  if (wrap) {
    wrap.innerHTML = `<img src="/api/snapshot?id=${encodeURIComponent(c.id)}&channel=0&stream=1&_r=${Date.now()}" alt="${escapeHtml(c.name || c.host)}">`;
  }
  openDialog(dlg, {
    onClose: () => {
      if (quickPtzLive) quickPtzLive.stop();
      quickPtzCam = null;
    }
  });
}

// PTZ Keyboard navigation (Arrow keys & WASD)
window.addEventListener('keydown', (ev) => {
  if (['INPUT', 'TEXTAREA', 'SELECT'].includes(ev.target.tagName)) return;
  const inDetailPTZ = ceActiveTab === 'ptz' && !document.getElementById('camera-detail').hidden;
  const inQuickPTZ = document.getElementById('quick-ptz-dialog')?.open;
  if (!inDetailPTZ && !inQuickPTZ) return;

  const keyMap = {
    'ArrowUp': 'Up', 'KeyW': 'Up', 'w': 'Up', 'W': 'Up',
    'ArrowDown': 'Down', 'KeyS': 'Down', 's': 'Down', 'S': 'Down',
    'ArrowLeft': 'Left', 'KeyA': 'Left', 'a': 'Left', 'A': 'Left',
    'ArrowRight': 'Right', 'KeyD': 'Right', 'd': 'Right', 'D': 'Right',
  };
  const code = keyMap[ev.code] || keyMap[ev.key];
  if (code && !ev.repeat) {
    ev.preventDefault();
    if (inQuickPTZ && quickPtzCam) {
      quickPtzActive = code;
      const speed = parseInt(document.getElementById('quick-ptz-speed')?.value, 10) || 5;
      api('/api/ptz', { method: 'POST', body: JSON.stringify({ id: quickPtzCam.id, channel: 0, code, speed, start: true, timeoutSeconds: timeoutSec() }) }).catch(() => {});
    } else if (inDetailPTZ && channelEditTile) {
      ptzStart(code);
    }
  }
});

window.addEventListener('keyup', (ev) => {
  if (['INPUT', 'TEXTAREA', 'SELECT'].includes(ev.target.tagName)) return;
  const inDetailPTZ = ceActiveTab === 'ptz' && !document.getElementById('camera-detail').hidden;
  const inQuickPTZ = document.getElementById('quick-ptz-dialog')?.open;
  if (!inDetailPTZ && !inQuickPTZ) return;

  const keyMap = {
    'ArrowUp': 'Up', 'KeyW': 'Up', 'w': 'Up', 'W': 'Up',
    'ArrowDown': 'Down', 'KeyS': 'Down', 's': 'Down', 'S': 'Down',
    'ArrowLeft': 'Left', 'KeyA': 'Left', 'a': 'Left', 'A': 'Left',
    'ArrowRight': 'Right', 'KeyD': 'Right', 'd': 'Right', 'D': 'Right',
  };
  const code = keyMap[ev.code] || keyMap[ev.key];
  if (code) {
    if (inQuickPTZ && quickPtzCam) {
      quickPtzActive = null;
      const speed = parseInt(document.getElementById('quick-ptz-speed')?.value, 10) || 5;
      api('/api/ptz', { method: 'POST', body: JSON.stringify({ id: quickPtzCam.id, channel: 0, code, speed, start: false, timeoutSeconds: timeoutSec() }) }).catch(() => {});
    } else if (inDetailPTZ && channelEditTile) {
      ptzStop();
    }
  }
});

/* ---------- live view (MJPEG over the DVRIP snapshot, no ffmpeg) ---------- */
// One preview owned by the detail page's left column. It deliberately keeps
// running across tab switches: the whole point of the detail layout is to
// watch the picture while changing colour / OSD / PTZ. Previously each tab had
// its own start/extend/stop trio and switching tabs killed the stream.
const detailLive = livePreview({
  img: document.getElementById('cd-live'),
  start: document.getElementById('cd-live-start'),
  extend: document.getElementById('cd-live-extend'),
  stop: document.getElementById('cd-live-stop'),
  status: document.getElementById('cd-live-status'),
}, () => channelEditTile ? { id: channelEditTile.camId, channel: channelEditTile.channel } : null);

function stopLive() { detailLive.stop(); }

document.getElementById('cd-live-fullscreen')?.addEventListener('click', () => {
  const wrap = document.getElementById('ce-preview-img-wrap');
  const live = document.getElementById('cd-live');
  const target = (live && !live.hidden) ? live : wrap;
  if (!document.fullscreenElement) {
    if (target.requestFullscreen) target.requestFullscreen();
    else if (target.webkitRequestFullscreen) target.webkitRequestFullscreen();
  } else {
    if (document.exitFullscreen) document.exitFullscreen();
  }
});

// Lazy-load sentinels for the detail page's data tabs (null until the tab is
// first opened for the current camera; reset in openCameraDetail).
let videoPayload = null, audioPayload = null, networkPayload = null, maintPayload = null;

// switchCeTab shows one detail panel and lazily fetches its data. The live
// preview is NOT stopped here — it belongs to the page, not to a tab.
function switchCeTab(tab) {
  if (!DETAIL_TABS.includes(tab)) tab = 'osd';
  ceActiveTab = tab;
  document.querySelectorAll('#ce-tabs .tab-btn').forEach(b => {
    const on = b.dataset.ceTab === tab;
    b.classList.toggle('active', on);
    b.setAttribute('aria-selected', on ? 'true' : 'false');
  });
  DETAIL_TABS.forEach(t => {
    const panel = document.getElementById('ce-panel-' + t);
    if (panel) panel.hidden = t !== tab;
  });
  if (!channelEditTile) return;
  if (tab === 'picture' && !picturePayload) loadPictureTab(channelEditTile);
  if (tab === 'video' && !videoPayload) loadVideoTab(channelEditTile);
  if (tab === 'audio' && !audioPayload) loadAudioTab(channelEditTile);
  if (tab === 'network' && !networkPayload) {
    networkPayload = {};
    const cam = cameras.find(x => x.id === channelEditTile.camId);
    if (cam) openNetworkCard(cam);
  }
  if (tab === 'maint' && !maintPayload) {
    maintPayload = {};
    const cam = cameras.find(x => x.id === channelEditTile.camId);
    if (cam) renderMaintenance(cam);
  }
}

// Tab clicks drive the hash so the detail page is linkable and the back button
// steps through tabs, rather than switching panels behind the URL's back.
document.getElementById('ce-tabs').addEventListener('click', (ev) => {
  const btn = ev.target.closest('.tab-btn');
  if (!btn || btn.disabled || !channelEditTile) return;
  gotoCameraDetail(channelEditTile.camId, btn.dataset.ceTab);
});

// ceObjectURLs tracks blob: URLs handed to the preview image so they can be
// revoked on the next load/dialog close, same leak-avoidance as the gallery.
let ceObjectURLs = [];

async function loadCePreview(tile, cacheBust) {
  const wrap = document.getElementById('ce-preview-img-wrap');
  wrap.innerHTML = '<span class="spinner"></span>';
  ceObjectURLs.forEach(u => URL.revokeObjectURL(u));
  ceObjectURLs = [];
  let q = `id=${encodeURIComponent(tile.camId)}&channel=${tile.channel}&stream=${tile.stream}&timeoutSeconds=${timeoutSec()}`;
  if (cacheBust) q += '&_r=' + Date.now();
  try {
    const resp = await fetch('/api/snapshot?' + q);
    if (resp.status === 401) { location.href = '/login'; return; }
    if (!resp.ok) {
      const text = await resp.text().catch(() => '');
      let msg = text || resp.statusText;
      try { const j = JSON.parse(text); if (j && j.error) msg = j.error; } catch (e) { /* not JSON */ }
      throw new Error(msg);
    }
    const blob = await resp.blob();
    const url = URL.createObjectURL(blob);
    ceObjectURLs.push(url);
    wrap.innerHTML = `<img src="${url}" alt="${escapeHtml(tile.camName)} K${tile.channel + 1}">`;
  } catch (e) {
    wrap.innerHTML = `<div class="gallery-tile-err-msg">${escapeHtml(e.message)}</div>`;
  }
}

document.getElementById('ce-preview-reload').addEventListener('click', () => {
  if (channelEditTile) loadCePreview(channelEditTile, true);
});

/* ---------- Modal: Video tab (read via /api/probe, write via /api/apply) ---------- */
const CE_STREAM_LABELS = { 0: 'Chính (main)', 1: 'Phụ 1 (sub)', 2: 'Phụ 2 (sub2)' };
const CE_CODECS = ['H.265', 'H.264', 'H.264H', 'H.264B', 'MJPG'];
let fpsCapTimer = null;

async function refreshCeFPSCapability(tile, fs, stream) {
  const input = fs.querySelector('.cv-fps'), hint = fs.querySelector('.cv-fps-hint');
  const body = {
    id: tile.camId, channel: tile.channel, stream,
    width: parseInt(fs.querySelector('.cv-w').value, 10) || 0,
    height: parseInt(fs.querySelector('.cv-h').value, 10) || 0,
    codec: fs.querySelector('.cv-codec').value,
    timeoutSeconds: timeoutSec(),
  };
  try {
    const cap = await api('/api/fps-capability', { method: 'POST', body: JSON.stringify(body) });
    const max = parseInt(cap.maxFps, 10) || Math.max(parseInt(input.value, 10) || 0, 20);
    input.max = max;
    if ((parseInt(input.value, 10) || 0) > max) input.value = max;
    hint.textContent = `Tối đa ${max} FPS` + (cap.source === 'fallback' ? ' (fallback)' : '');
  } catch (_) {
    const fallback = Math.max(parseInt(input.value, 10) || 0, 20);
    input.max = fallback;
    hint.textContent = `Tối đa ${fallback} FPS (fallback)`;
  }
}

async function loadVideoTab(tile) {
  videoPayload = {}; // mark loaded
  const body = document.getElementById('ce-vid-body'), msg = document.getElementById('ce-vid-msg');
  body.innerHTML = '<span class="spinner"></span>'; msg.textContent = '';
  try {
    const infos = rememberProbeResult(tile.camId, await api('/api/probe', { method: 'POST', body: JSON.stringify({ id: tile.camId, timeoutSeconds: timeoutSec() }) }));
    const mine = (infos || []).filter(s => s.channel === tile.channel + 1);
    if (!mine.length) { body.innerHTML = '<p class="muted">Không đọc được thông số video cho kênh này.</p>'; return; }
    body.innerHTML = mine.map(s => {
      const st = s.stream;
      return `<fieldset class="ce-vid-stream" data-stream="${st}">
        <legend>${escapeHtml(CE_STREAM_LABELS[st] || ('Stream ' + st))}</legend>
        <div class="row">
          <div class="field field-sm"><label>Rộng</label><input type="number" class="cv-w"></div>
          <div class="field field-sm"><label>Cao</label><input type="number" class="cv-h"></div>
          <div class="field field-sm"><label>Codec</label><select class="cv-codec">${CE_CODECS.map(c => `<option>${c}</option>`).join('')}</select></div>
          <div class="field field-sm"><label>FPS</label><input type="number" class="cv-fps" min="1" step="1"><span class="muted cv-fps-hint"></span></div>
        </div>
        <div class="row">
          <div class="field field-sm"><label>Bitrate (Kbps)</label><input type="number" class="cv-br"></div>
          <div class="field field-sm"><label>Kiểu</label><select class="cv-brmode"><option value="">(giữ)</option><option>CBR</option><option>VBR</option></select></div>
          <div class="field field-sm"><label>GOP (I-frame)</label><input type="number" class="cv-gop"></div>
        </div>
        <label class="checkbox-row"><input type="checkbox" class="cv-smart"> Smart Codec (H.26x+)</label>
        <button class="btn btn-primary" type="button" data-save-stream="${st}">Lưu stream này</button>
      </fieldset>`;
    }).join('');
    mine.forEach(s => {
      const fs = body.querySelector(`.ce-vid-stream[data-stream="${s.stream}"]`);
      fs.querySelector('.cv-w').value = s.width || '';
      fs.querySelector('.cv-h').value = s.height || '';
      fs.querySelector('.cv-codec').value = s.compression || 'H.265';
      fs.querySelector('.cv-fps').value = s.fps || 20;
      fs.querySelector('.cv-br').value = s.bitrateKbps || '';
      fs.querySelector('.cv-brmode').value = s.bitrateMode || '';
      fs.querySelector('.cv-gop').value = s.gop || '';
      fs.querySelector('.cv-smart').checked = !!s.smartCodec;
      refreshCeFPSCapability(tile, fs, s.stream);
      fs.querySelectorAll('.cv-w,.cv-h,.cv-codec').forEach(el => el.addEventListener('change', () => {
        clearTimeout(fpsCapTimer);
        fpsCapTimer = setTimeout(() => refreshCeFPSCapability(tile, fs, s.stream), 250);
      }));
    });
    body.querySelectorAll('[data-save-stream]').forEach(btn =>
      btn.addEventListener('click', () => saveCeVideoStream(tile, parseInt(btn.dataset.saveStream, 10), btn)));
  } catch (e) { body.innerHTML = ''; msg.textContent = 'Lỗi: ' + e.message; msg.className = 'msg err'; }
}

async function saveCeVideoStream(tile, stream, btn) {
  const fs = document.getElementById('ce-vid-body').querySelector(`.ce-vid-stream[data-stream="${stream}"]`);
  const msg = document.getElementById('ce-vid-msg');
  const profile = {
    setResolution: true, width: parseInt(fs.querySelector('.cv-w').value, 10) || 0, height: parseInt(fs.querySelector('.cv-h').value, 10) || 0,
    setCodec: true, codec: fs.querySelector('.cv-codec').value, codecProfile: '',
    setFps: true, fps: parseInt(fs.querySelector('.cv-fps').value, 10) || 0,
    setBitrate: true, bitrate: parseInt(fs.querySelector('.cv-br').value, 10) || 0, bitrateMode: fs.querySelector('.cv-brmode').value,
    setGop: true, gop: parseInt(fs.querySelector('.cv-gop').value, 10) || 0,
    setSmartCodec: true, smartCodec: fs.querySelector('.cv-smart').checked,
    streams: [stream], channels: [tile.channel],
  };
  btn.disabled = true; msg.textContent = 'Đang lưu...'; msg.className = 'msg';
  try {
    const res = await streamApply([tile.camId], profile);
    const r = res && res[0];
    const bad = r && r.steps ? r.steps.filter(s => !s.ok) : [];
    if (r && r.ok && !bad.length) { msg.textContent = 'Đã lưu stream ' + stream + '.'; msg.className = 'msg ok'; await loadVideoTab(tile); }
    else { msg.textContent = 'Một số bước lỗi: ' + (bad.map(s => s.step + (s.err ? ' (' + s.err + ')' : '')).join('; ') || (r && r.err) || 'không rõ'); msg.className = 'msg err'; }
  } catch (e) { msg.textContent = 'Lỗi: ' + e.message; msg.className = 'msg err'; }
  finally { btn.disabled = false; }
}

/* ---------- Modal: Audio tab ---------- */
async function loadAudioTab(tile) {
  audioPayload = {};
  const body = document.getElementById('ce-aud-body'), msg = document.getElementById('ce-aud-msg');
  body.innerHTML = '<span class="spinner"></span>'; msg.textContent = '';
  try {
    const infos = rememberProbeResult(tile.camId, await api('/api/probe', { method: 'POST', body: JSON.stringify({ id: tile.camId, timeoutSeconds: timeoutSec() }) }));
    const mine = (infos || []).filter(s => s.channel === tile.channel + 1);
    if (!mine.length) { body.innerHTML = '<p class="muted">Không đọc được thông số âm thanh.</p>'; return; }
    body.innerHTML = `<table class="mini-table"><thead><tr><th>Stream</th><th>Codec</th><th>Bật</th></tr></thead><tbody>${
      mine.map(s => `<tr><td>${escapeHtml(CE_STREAM_LABELS[s.stream] || s.stream)}</td><td>${escapeHtml(s.audioCodec || '—')}</td><td>${s.audioEnable ? '✓' : '—'}</td></tr>`).join('')
    }</tbody></table>
    <p class="muted section-gap">Bật âm thanh AAC cho stream chính (chuẩn, iPhone/Android nghe được).</p>
    <button class="btn btn-primary" type="button" id="ce-aud-aac">Bật audio AAC (stream chính)</button>`;
    document.getElementById('ce-aud-aac').addEventListener('click', async (ev) => {
      ev.target.disabled = true; msg.textContent = 'Đang bật...'; msg.className = 'msg';
      try {
        const res = await streamApply([tile.camId], { setAudioAAC: true, streams: [0], channels: [tile.channel] });
        const r = res && res[0];
        if (r && r.ok) { msg.textContent = 'Đã bật AAC.'; msg.className = 'msg ok'; audioPayload = null; loadAudioTab(tile); }
        else { msg.textContent = 'Lỗi: ' + ((r && r.err) || 'không rõ'); msg.className = 'msg err'; }
      } catch (e) { msg.textContent = 'Lỗi: ' + e.message; msg.className = 'msg err'; }
      finally { ev.target.disabled = false; }
    });
  } catch (e) { body.innerHTML = ''; msg.textContent = 'Lỗi: ' + e.message; msg.className = 'msg err'; }
}

/* ---------- camera detail page ---------- */

// openCameraDetail mounts the detail page for '#cameras/cam/<id>/<tab>'. It
// replaces the old openChannelEdit() modal: same panels, but routed, with one
// shared live preview and an explicit reason on every unavailable tab.
async function openCameraDetail(sel) {
  const cam = cameras.find(x => x.id === sel.id);
  if (!cam) {
    // Inventory may still be loading; loadCameras() re-runs setRoute when done.
    if (cameras.length) {
      showToast('Không tìm thấy camera trong kho.', 'err');
      location.hash = '#cameras/list';
    }
    return;
  }

  const sameCamera = channelEditTile && channelEditTile.camId === cam.id;
  if (!sameCamera) {
    detailLive.stop();
    channelEditTile = { camId: cam.id, camName: cam.name || cam.host, channel: 0, stream: 0 };
    picturePayload = null;
    videoPayload = null; audioPayload = null; networkPayload = null; maintPayload = null;
    closeNetworkCard();
    document.getElementById('detail-name').textContent = cam.name || cam.host;
    document.getElementById('detail-meta').textContent =
      `${cam.host}:${cam.port} · ${cam.vendor}${cam.isNvr ? ' · đầu ghi' : ''}`;
    ['ce-msg', 'ce-picture-msg', 'ce-vid-msg', 'ce-aud-msg', 'ce-net-msg', 'ce-ptz-msg']
      .forEach(id => { const el = document.getElementById(id); if (el) { el.textContent = ''; el.className = 'msg'; } });
    ['ce-picture-body', 'ce-picture-lite-body', 'ce-vid-body', 'ce-aud-body', 'ce-osd-fields']
      .forEach(id => { const el = document.getElementById(id); if (el) el.innerHTML = ''; });
    document.getElementById('ce-name').value = '';
    document.getElementById('ce-osd-hint').textContent = '';
    switchPictureMode('lite');
    applyDetailCapabilities(cam);
    await populateDetailChannels(cam);
    loadCePreview(channelEditTile, false);
    loadChannelInfo();
  }
  switchCeTab(sel.tab);
}

function closeCameraDetail() {
  if (!channelEditTile) return;
  detailLive.stop();
  channelEditTile = null;
  ceObjectURLs.forEach(u => URL.revokeObjectURL(u));
  ceObjectURLs = [];
  closeNetworkCard();
}

// applyDetailCapabilities disables tabs the vendor can't serve — and says why.
// The modal used to just hide them, so a feature silently vanished with no
// hint that it exists at all for other vendors.
function applyDetailCapabilities(cam) {
  const isDahua = cam.vendor === 'dahua';
  const canEncode = ['dahua', 'hikvision', 'tiandy'].includes(cam.vendor);
  const canNetwork = ['dahua', 'hikvision', 'tiandy'].includes(cam.vendor);
  const dahuaOnly = `Chỉ Dahua/KBVision hỗ trợ qua DVRIP — camera này là ${cam.vendor}.`;
  const caps = {
    picture: [isDahua, dahuaOnly],
    ptz: [isDahua, dahuaOnly],
    audio: [isDahua, dahuaOnly],
    maint: [isDahua, dahuaOnly],
    video: [canEncode, `Chưa hỗ trợ đọc/ghi cấu hình mã hoá cho ${cam.vendor}.`],
    network: [canNetwork, `Chưa hỗ trợ đọc/ghi cấu hình mạng cho ${cam.vendor}.`],
  };
  Object.entries(caps).forEach(([tab, [ok, why]]) => {
    const btn = document.getElementById('ce-tab-btn-' + tab);
    if (!btn) return;
    btn.disabled = !ok;
    btn.title = ok ? '' : why;
    btn.classList.toggle('tab-unavailable', !ok);
  });
}

// populateDetailChannels fills the channel picker. A plain camera has one
// channel; an NVR's channel list comes from the device itself rather than a
// guessed 1..32 range.
// Set by callers that know which channel they want (e.g. the snapshot gallery)
// before routing to the detail page; consumed once by populateDetailChannels.
let pendingDetailChannel = 0;

async function populateDetailChannels(cam) {
  const select = document.getElementById('detail-channel');
  select.innerHTML = '<option value="0">Kênh 1</option>';
  if (!cam.isNvr) { select.disabled = true; return; }
  select.disabled = false;
  try {
    const res = await api(`/api/nvr/channels?id=${encodeURIComponent(cam.id)}&timeoutSeconds=${timeoutSec()}`);
    const chans = (res && res.channels) || [];
    if (chans.length) {
      select.innerHTML = chans.map(ch => {
        const n = ch.channel || 0; // device reports 1-based; tiles are 0-based
        const label = ch.name ? `Kênh ${n} — ${escapeHtml(ch.name)}` : `Kênh ${n}`;
        return `<option value="${Math.max(0, n - 1)}">${label}</option>`;
      }).join('');
    }
  } catch (e) {
    // Channel list is a nicety; a single-channel fallback still works.
  }
  if (pendingDetailChannel) {
    const wanted = String(pendingDetailChannel);
    if (Array.from(select.options).some(o => o.value === wanted)) {
      select.value = wanted;
      channelEditTile.channel = pendingDetailChannel;
    }
    pendingDetailChannel = 0;
  }
}

document.getElementById('detail-channel').addEventListener('change', (ev) => {
  if (!channelEditTile) return;
  channelEditTile.channel = parseInt(ev.target.value, 10) || 0;
  // Every per-channel payload is now stale.
  picturePayload = null; videoPayload = null; audioPayload = null;
  const wasLive = detailLive.running();
  detailLive.stop();
  loadCePreview(channelEditTile, true);
  loadChannelInfo();
  switchCeTab(ceActiveTab);
  if (wasLive) detailLive.start();
});

// loadChannelInfo fills the Tên & OSD tab for the current channel.
async function loadChannelInfo() {
  const tile = channelEditTile;
  if (!tile) return;
  const msg = document.getElementById('ce-msg');
  const nameInput = document.getElementById('ce-name');
  const osdFields = document.getElementById('ce-osd-fields');
  const osdHint = document.getElementById('ce-osd-hint');
  nameInput.value = '';
  osdFields.innerHTML = '';
  osdHint.textContent = '';
  msg.textContent = 'Đang tải...'; msg.className = 'msg';
  try {
    const q = `id=${encodeURIComponent(tile.camId)}&channel=${tile.channel}&timeoutSeconds=${timeoutSec()}`;
    const info = await api('/api/channel-info?' + q);
    if (channelEditTile !== tile) return; // user moved on while we waited
    nameInput.value = info.name || '';
    if (info.osdSupported) {
      const lines = (info.osdLines && info.osdLines.length ? info.osdLines : ['', '', '', '']);
      const osdEnabled = info.osdEnabled || [];
      osdFields.innerHTML = lines.map((line, i) => {
        // Default matches the server's own fallback (enable exactly the
        // lines with text) when the device hasn't reported enable state.
        const on = i < osdEnabled.length ? osdEnabled[i] : !!line;
        return `
        <div class="field field-sm ce-osd-row">
          <label for="ce-osd-line-${i}">Dòng OSD ${i + 1}</label>
          <div class="ce-osd-controls">
            <input class="ce-osd-line" id="ce-osd-line-${i}" value="${escapeHtml(line || '')}">
            <label class="checkbox-row" title="Hiện trên hình">
              <input type="checkbox" class="ce-osd-enable" ${on ? 'checked' : ''}><span class="muted">Hiện</span>
            </label>
          </div>
        </div>`;
      }).join('');
    } else {
      osdHint.textContent = 'Camera này không hỗ trợ (hoặc chưa xác minh) chỉnh OSD qua API — xem docs/GOTCHAS.md.';
    }
    msg.textContent = ''; msg.className = 'msg';
  } catch (e) {
    if (channelEditTile !== tile) return;
    msg.textContent = 'Lỗi tải: ' + e.message;
    msg.className = 'msg err';
  }
}

document.getElementById('ce-save').addEventListener('click', async () => {
  if (!channelEditTile) return;
  const btn = document.getElementById('ce-save');
  const msg = document.getElementById('ce-msg');
  const name = document.getElementById('ce-name').value;
  const lines = Array.from(document.querySelectorAll('.ce-osd-line')).map(el => el.value);
  const enabled = Array.from(document.querySelectorAll('.ce-osd-enable')).map(el => el.checked);
  setBusy(btn, true, 'Đang lưu...');
  msg.textContent = ''; msg.className = 'msg';
  try {
    await api('/api/channel-name', {
      method: 'POST',
      body: JSON.stringify({ id: channelEditTile.camId, channel: channelEditTile.channel, name, timeoutSeconds: timeoutSec() }),
    });
    if (lines.length) {
      const res = await api('/api/osd', {
        method: 'POST',
        body: JSON.stringify({ id: channelEditTile.camId, channel: channelEditTile.channel, lines, enabled, timeoutSeconds: timeoutSec() }),
      });
      msg.textContent = `Đã lưu tên. OSD: áp dụng ${res.appliedLines}/${lines.length} dòng.`;
    } else {
      msg.textContent = 'Đã lưu tên.';
    }
    msg.className = 'msg ok';
    showToast('Đã lưu xuống camera.', 'ok');
    delete probeCache[channelEditTile.camId];
  } catch (e) {
    msg.textContent = 'Lỗi: ' + e.message;
    msg.className = 'msg err';
    showToast('Lỗi lưu: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
});

/* ---------- Chỉnh màu tab (Dahua-only: VideoColor + VideoInOptions) ---------- */

// PICTURE_ENUMS renders a <select> instead of a raw input for fields whose
// valid values are documented in dahua_http_api_for_ipcsd-v1.40.pdf, keyed by
// the field's leaf name (same enum applies whether it's top-level or nested
// under NightOptions/NormalOptions, e.g. "WhiteBalance" and
// "NightOptions.WhiteBalance" share this list). Each entry is either a plain
// string (value === label) or a [value, label] pair.
const PICTURE_ENUMS = {
  WhiteBalance: ['Disable', 'Auto', 'Custom', 'Sunny', 'Cloudy', 'Home', 'Office', 'Night'],
  Rotate90: [['0', 'Không xoay'], ['1', 'Xoay 90° thuận'], ['2', 'Xoay 90° ngược']],
  DayNightColor: [['0', 'Luôn màu'], ['1', 'Tự chuyển theo độ sáng'], ['2', 'Luôn đen trắng']],
  AntiFlicker: [['0', 'Ngoài trời'], ['1', 'Chống nhấp nháy 50Hz'], ['2', 'Chống nhấp nháy 60Hz']],
  ExposureMode: [['0', 'Tự động'], ['1', 'Ưu tiên Gain'], ['2', 'Ưu tiên phơi sáng'], ['4', 'Thủ công']],
  SwitchMode: [['0', 'Luôn ban ngày'], ['1', 'Theo độ sáng'], ['2', 'Theo giờ'], ['3', 'Luôn ban đêm'], ['4', 'Luôn ban ngày phụ']],
};

function getNested(obj, path) {
  return path.split('.').reduce((o, k) => (o && typeof o === 'object') ? o[k] : undefined, obj);
}
function setNested(obj, path, value) {
  const parts = path.split('.');
  let cur = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    cur[parts[i]] = cur[parts[i]] || {};
    cur = cur[parts[i]];
  }
  cur[parts[parts.length - 1]] = value;
}

// pictureFieldRow renders one editable row for fullKey (dot-path within its
// section, e.g. "NightOptions.GainRed") holding value. section is "color" or
// "options", stamped as data-pf-section so collectPictureChanges knows which
// half of the POST body a change belongs to.
function pictureFieldRow(fullKey, leafKey, value, section) {
  const id = 'pf-' + fullKey.replace(/\./g, '-');
  const enumDef = PICTURE_ENUMS[leafKey];
  let input;
  if (enumDef) {
    const opts = enumDef.map(o => Array.isArray(o) ? o : [o, o]);
    input = `<select id="${id}" data-pf-key="${escapeHtml(fullKey)}" data-pf-section="${section}">` +
      opts.map(([v, label]) => `<option value="${escapeHtml(v)}" ${String(value) === v ? 'selected' : ''}>${escapeHtml(label)}</option>`).join('') +
      `</select>`;
  } else if (typeof value === 'boolean') {
    input = `<input type="checkbox" id="${id}" data-pf-key="${escapeHtml(fullKey)}" data-pf-section="${section}" data-pf-type="bool" ${value ? 'checked' : ''}>`;
  } else if (typeof value === 'number') {
    input = `<input type="number" id="${id}" data-pf-key="${escapeHtml(fullKey)}" data-pf-section="${section}" data-pf-type="number" value="${value}">`;
  } else {
    input = `<input type="text" id="${id}" data-pf-key="${escapeHtml(fullKey)}" data-pf-section="${section}" data-pf-type="string" value="${escapeHtml(String(value == null ? '' : value))}">`;
  }
  return `<div class="field field-sm pf-row"><label for="${id}">${escapeHtml(leafKey)}</label>${input}</div>`;
}

// ROTATE90_OPTIONS backs the "Cơ bản" tab's rotate stepper (prev/next
// buttons cycling through the 3 values the device actually supports,
// instead of a raw dropdown — "step xoay").
const ROTATE90_OPTIONS = [['0', 'Không xoay'], ['1', 'Xoay 90° thuận'], ['2', 'Xoay 90° ngược']];

function renderRotateStepper(value) {
  let idx = ROTATE90_OPTIONS.findIndex(([v]) => v === String(value));
  if (idx < 0) idx = 0;
  return `
    <div class="field field-sm pf-row">
      <label>Xoay ảnh</label>
      <div class="pf-stepper">
        <button type="button" class="pf-stepper-btn" data-pf-rotate-prev aria-label="Xoay trước">&lt;</button>
        <span class="pf-stepper-label" id="pf-rotate-label">${escapeHtml(ROTATE90_OPTIONS[idx][1])}</span>
        <button type="button" class="pf-stepper-btn" data-pf-rotate-next aria-label="Xoay sau">&gt;</button>
        <input type="hidden" id="pf-lite-Rotate90" data-pf-key="Rotate90" data-pf-section="options" value="${ROTATE90_OPTIONS[idx][0]}">
      </div>
    </div>`;
}

// wireRotateStepper hooks the prev/next buttons rendered by
// renderRotateStepper — called once after inserting it into the DOM (the
// buttons don't exist before that).
function wireRotateStepper() {
  const hidden = document.getElementById('pf-lite-Rotate90');
  const label = document.getElementById('pf-rotate-label');
  const prev = document.querySelector('[data-pf-rotate-prev]');
  const next = document.querySelector('[data-pf-rotate-next]');
  if (!hidden || !prev || !next) return;
  const step = (delta) => {
    let idx = ROTATE90_OPTIONS.findIndex(([v]) => v === hidden.value);
    idx = (idx + delta + ROTATE90_OPTIONS.length) % ROTATE90_OPTIONS.length;
    hidden.value = ROTATE90_OPTIONS[idx][0];
    label.textContent = ROTATE90_OPTIONS[idx][1];
  };
  prev.addEventListener('click', () => step(-1));
  next.addEventListener('click', () => step(1));
}

// renderLiteFields is the "Cơ bản" (lite) panel: a curated subset of
// VideoInOptions — White Balance, Flip, Rotate (as a stepper), and the
// day/night color mode — reusing pictureFieldRow so these stay visually and
// behaviorally consistent with the full editor's equivalent fields.
function renderLiteFields(options) {
  options = options || {};
  // Backlight family (Dahua VideoInOptions): three independent intensity
  // fields the device returns, per dahua_http_api_for_ipcsd-v1.40.pdf §4.3.3
  // — Backlight (BLC compensation, grade 0-n, 0=off), GlareInhibition (HLC
  // highlight suppression, 0-100, 0=off), WideDynamicRange (WDR, 0-100,
  // 0=off). Rendered as sliders only when the field is actually present in
  // the device's response, so a camera that doesn't support one simply won't
  // show it (rather than us guessing a synthetic "mode" enum that varies by
  // firmware).
  let backlight = '';
  if ('Backlight' in options) backlight += pictureRangeRow('Backlight', 'Bù ngược sáng (BLC)', toNum(options.Backlight, 0), 'options', 100);
  if ('GlareInhibition' in options) backlight += pictureRangeRow('GlareInhibition', 'Chống chói (HLC)', toNum(options.GlareInhibition, 0), 'options', 100);
  if ('WideDynamicRange' in options) backlight += pictureRangeRow('WideDynamicRange', 'Dải tương phản rộng (WDR)', toNum(options.WideDynamicRange, 0), 'options', 100);

  return '<div class="pf-section-body">' +
    pictureFieldRow('WhiteBalance', 'WhiteBalance', options.WhiteBalance, 'options') +
    pictureFieldRow('Flip', 'Flip', !!options.Flip, 'options') +
    renderRotateStepper(options.Rotate90) +
    pictureFieldRow('DayNightColor', 'DayNightColor', options.DayNightColor, 'options') +
    pictureFieldRow('ExposureMode', 'ExposureMode', options.ExposureMode, 'options') +
    backlight +
    '</div>';
}

// toNum coerces v to a number for a number-typed pf row, defaulting when the
// device didn't return the field at all.
function toNum(v, def) {
  return (typeof v === 'number') ? v : def;
}

// pictureRangeRow renders a slider (with a live value readout) for an integer
// intensity field. Carries the same data-pf-key/data-pf-type="number"
// attributes pictureFieldRow uses, so collectPictureChanges picks it up with
// no special handling. 0 means the feature is off for all three backlight
// fields, so the slider naturally doubles as an on/off + intensity control.
function pictureRangeRow(fullKey, label, value, section, max) {
  const id = 'pf-' + fullKey.replace(/\./g, '-');
  return `<div class="field field-sm pf-row" style="flex-basis:100%">
    <label for="${id}">${escapeHtml(label)}</label>
    <div class="row" style="align-items:center;gap:10px;flex-wrap:nowrap">
      <input type="range" id="${id}" data-pf-key="${escapeHtml(fullKey)}" data-pf-section="${section}" data-pf-type="number"
        min="0" max="${max}" value="${value}" style="flex:1 1 auto">
      <output class="pf-range-out" for="${id}" style="min-width:2.5em;text-align:right">${value}</output>
    </div>
  </div>`;
}

// renderObjectFields recurses one level into nested objects (e.g.
// FlashControl) inline, skipping keys in skipKeys (used to carve
// NightOptions/NormalOptions out into their own top-level sections) and
// arrays (e.g. BacklightRegion — not worth a bespoke UI; untouched fields are
// simply never sent, so this doesn't affect saving).
function renderObjectFields(obj, pathPrefix, skipKeys, section) {
  let html = '';
  for (const [k, v] of Object.entries(obj || {})) {
    if (skipKeys && skipKeys.includes(k)) continue;
    const fullKey = pathPrefix ? pathPrefix + '.' + k : k;
    if (Array.isArray(v)) continue;
    if (v !== null && typeof v === 'object') {
      const inner = renderObjectFields(v, fullKey, null, section);
      if (inner) html += `<div class="pf-subgroup"><div class="pf-subgroup-title">${escapeHtml(k)}</div>${inner}</div>`;
    } else {
      html += pictureFieldRow(fullKey, k, v, section);
    }
  }
  return html;
}

function collapsibleSection(title, innerHtml, extraClass, openByDefault) {
  if (!innerHtml) return '';
  return `<details class="pf-section ${extraClass || ''}" ${openByDefault ? 'open' : ''}>
    <summary>${escapeHtml(title)}</summary>
    <div class="pf-section-body">${innerHtml}</div>
  </details>`;
}

// applyCapsDisabling greys out/locks fields (or whole sections) the device's
// GetVideoInputCaps response says it doesn't support, so a user can't
// "successfully" set something the device will silently ignore. caps keys
// are best-effort text/plain-decoded strings (see dahua.parseCapsLines), so
// every comparison is against the string "false"/"0".
function applyCapsDisabling(caps) {
  if (!caps) return;
  const lockIfFalse = (leafKey) => {
    if (caps[leafKey] !== 'false') return;
    document.querySelectorAll(`[data-pf-key="${leafKey}"], [data-pf-key$=".${leafKey}"]`).forEach(el => {
      el.disabled = true;
      el.closest('.pf-row').title = 'Camera này báo không hỗ trợ trường này (caps.' + leafKey + ')';
    });
  };
  ['Flip', 'Mirror', 'Rotate90', 'DayNightColor'].forEach(lockIfFalse);
  if (caps.WhiteBalance === '0') {
    document.querySelectorAll('[data-pf-key="WhiteBalance"], [data-pf-key$=".WhiteBalance"]').forEach(el => { el.disabled = true; });
  }
  // The rotate stepper's prev/next buttons aren't [data-pf-key] elements
  // themselves (only the hidden input backing them is) — grey them out too
  // when that hidden input got locked above.
  document.querySelectorAll('.pf-stepper [data-pf-key="Rotate90"]').forEach(hidden => {
    if (hidden.disabled) {
      hidden.closest('.pf-stepper').querySelectorAll('button').forEach(b => { b.disabled = true; });
    }
  });
  const body = document.getElementById('ce-picture-body');
  if (caps.SetColor === 'false') {
    const sec = body.querySelector('.pf-section-color');
    if (sec) sec.classList.add('pf-section-disabled');
  }
  if (caps.NightOptions === 'false') {
    const sec = body.querySelector('.pf-section-night');
    if (sec) sec.classList.add('pf-section-disabled');
  }
}

async function loadPictureTab(tile) {
  const body = document.getElementById('ce-picture-body');
  const liteBody = document.getElementById('ce-picture-lite-body');
  const msg = document.getElementById('ce-picture-msg');
  body.innerHTML = '<span class="muted">Đang tải...</span>';
  liteBody.innerHTML = '<span class="muted">Đang tải...</span>';
  msg.textContent = ''; msg.className = 'msg';
  try {
    const q = `id=${encodeURIComponent(tile.camId)}&channel=${tile.channel}&timeoutSeconds=${timeoutSec()}`;
    const info = await api('/api/picture?' + q);
    picturePayload = { color: info.color || {}, options: info.options || {} };
    const capsHint = info.capsError
      ? `<p class="muted">Không đọc được thông tin hỗ trợ (caps): ${escapeHtml(info.capsError)} — mọi trường vẫn hiện, camera có thể bỏ qua trường không hỗ trợ khi lưu.</p>`
      : '';
    const options = info.options || {};
    liteBody.innerHTML = renderLiteFields(options);
    wireRotateStepper();
    body.innerHTML = capsHint +
      collapsibleSection('Màu sắc', renderObjectFields(info.color, '', null, 'color'), 'pf-section-color', true) +
      collapsibleSection('Ảnh chung', renderObjectFields(options, '', ['NightOptions', 'NormalOptions'], 'options'), 'pf-section-options', true) +
      collapsibleSection('Ban đêm (NightOptions)', renderObjectFields(options.NightOptions, 'NightOptions', null, 'options'), 'pf-section-night', false) +
      collapsibleSection('Ban ngày phụ (NormalOptions)', renderObjectFields(options.NormalOptions, 'NormalOptions', null, 'options'), 'pf-section-normal', false);
    if (!body.innerHTML) body.innerHTML = '<p class="muted">Camera không trả về trường nào.</p>';
    applyCapsDisabling(info.caps);
  } catch (e) {
    body.innerHTML = '';
    liteBody.innerHTML = '';
    msg.textContent = 'Lỗi tải: ' + e.message;
    msg.className = 'msg err';
  }
}

// collectPictureChanges diffs every rendered field against picturePayload
// (the last GET), returning only what actually changed — SetPicture merges
// this onto the live device config server-side, so untouched fields are
// never overwritten with stale client-side copies.
function collectPictureChanges() {
  const color = {};
  const options = {};
  const containerId = pictureMode === 'lite' ? 'ce-picture-lite-body' : 'ce-picture-body';
  document.querySelectorAll('#' + containerId + ' [data-pf-key]').forEach(el => {
    if (el.disabled) return;
    const key = el.dataset.pfKey;
    const section = el.dataset.pfSection;
    let value;
    if (el.tagName === 'SELECT') {
      value = el.value;
    } else if (el.dataset.pfType === 'bool') {
      value = el.checked;
    } else if (el.dataset.pfType === 'number') {
      value = parseFloat(el.value);
      if (Number.isNaN(value)) return;
    } else {
      value = el.value;
    }
    const original = getNested(section === 'color' ? picturePayload.color : picturePayload.options, key);
    if (String(original) === String(value)) return;
    setNested(section === 'color' ? color : options, key, value);
  });
  return { color, options };
}

document.getElementById('ce-picture-save').addEventListener('click', async () => {
  if (!channelEditTile || !picturePayload) return;
  const btn = document.getElementById('ce-picture-save');
  const msg = document.getElementById('ce-picture-msg');
  const { color, options } = collectPictureChanges();
  if (!Object.keys(color).length && !Object.keys(options).length) {
    msg.textContent = 'Không có thay đổi nào để lưu.';
    msg.className = 'msg';
    return;
  }
  setBusy(btn, true, 'Đang lưu...');
  msg.textContent = ''; msg.className = 'msg';
  try {
    const res = await api('/api/picture', {
      method: 'POST',
      body: JSON.stringify({ id: channelEditTile.camId, channel: channelEditTile.channel, color, options, timeoutSeconds: timeoutSec() }),
    });
    picturePayload = { color: res.color || {}, options: res.options || {} };
    msg.textContent = 'Đã lưu. Đang tải lại để xác nhận...';
    msg.className = 'msg ok';
    showToast('Đã lưu chỉnh màu.', 'ok');
    await loadPictureTab(channelEditTile);
  } catch (e) {
    msg.textContent = 'Lỗi: ' + e.message;
    msg.className = 'msg err';
    showToast('Lỗi lưu chỉnh màu: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
});

document.getElementById('view-selected-btn').addEventListener('click', () => {
  const ids = selectedCameraIds();
  if (!ids.length) {
    const msg = document.getElementById('apply-msg');
    msg.className = 'msg err'; msg.textContent = 'Chưa chọn camera nào để xem hình.';
    return;
  }
  openGallery(buildTiles(ids));
});

function buildProfile() {
  const streams = Array.from(document.querySelectorAll('.stream-cb:checked')).map(cb => parseInt(cb.value, 10));
  return {
    setCodec: codecEnable.checked,
    codec: document.getElementById('p-codec-value').value,
    codecProfile: '',
    setResolution: resEnable.checked,
    width: parseInt(widthInput.value, 10) || 0,
    height: parseInt(heightInput.value, 10) || 0,
    setSmartCodec: smartEnable.checked,
    smartCodec: document.getElementById('p-smart-value').value === 'on',
    setGop: gopEnable.checked,
    gop: parseInt(document.getElementById('p-gop-value').value, 10) || 0,
    setBitrate: bitrateEnable.checked,
    bitrate: parseInt(document.getElementById('p-bitrate-value').value, 10) || 0,
    bitrateMode: document.getElementById('p-bitrate-mode').value,
    setAudioAAC: document.getElementById('p-audio-enable').checked,
    setOsd: document.getElementById('p-osd-enable').checked,
    osdLines: [
      document.getElementById('p-osd-line1').value.trim(),
      document.getElementById('p-osd-line2').value.trim(),
    ],
    streams: streams.length ? streams : [0],
    channels: parseChannels(document.getElementById('p-channel').value),
  };
}

// parseChannels turns a 1-based spec ("1", "1-8", "1,3,5", "1-3,7") into a
// 0-based channel array for the API. Empty -> [0] (channel 1).
function parseChannels(s) {
  s = (s || '').trim();
  if (!s) return [0];
  const out = new Set();
  s.split(',').forEach(part => {
    part = part.trim();
    if (!part) return;
    const m = part.match(/^(\d+)\s*-\s*(\d+)$/);
    if (m) {
      let a = parseInt(m[1], 10), b = parseInt(m[2], 10);
      if (a > b) { const t = a; a = b; b = t; }
      for (let i = a; i <= b; i++) if (i >= 1) out.add(i - 1);
    } else {
      const n = parseInt(part, 10);
      if (!isNaN(n) && n >= 1) out.add(n - 1);
    }
  });
  return out.size ? Array.from(out).sort((a, b) => a - b) : [0];
}

function renderResults(results) {
  const tbody = document.getElementById('result-tbody');
  if (!results || !results.length) {
    tbody.innerHTML = '<tr><td colspan="4" class="empty-hint">Chưa có kết quả.</td></tr>';
    return;
  }
  tbody.innerHTML = results.map(r => {
    const badge = r.ok
      ? '<span class="badge ok">✓ Thành công</span>'
      : '<span class="badge fail">✗ Thất bại</span>';
    let detail = '';
    if (r.steps && r.steps.length) {
      detail += '<ul class="steps">' + r.steps.map(s =>
        `<li class="${s.ok ? 'ok' : 'fail'}">${escapeHtml(s.step)}: ${escapeHtml(s.detail || '')}${s.err ? ' — ' + escapeHtml(s.err) : ''}</li>`
      ).join('') + '</ul>';
    }
    if (r.err) detail += `<div class="msg err">${escapeHtml(r.err)}</div>`;
    return `
      <tr>
        <td data-label="Camera">${escapeHtml(r.name || r.deviceId)}</td>
        <td data-label="Host">${escapeHtml(r.host || '')}</td>
        <td data-label="Trạng thái">${badge}</td>
        <td data-label="Chi tiết">${detail}</td>
      </tr>
    `;
  }).join('');
}

async function streamApply(ids, profile) {
  return streamPost('/api/apply', { deviceIds: ids, profile, timeoutSeconds: timeoutSec() });
}

document.getElementById('apply-btn').addEventListener('click', async () => {
  const ids = selectedCameraIds();
  const msg = document.getElementById('apply-msg');
  msg.textContent = ''; msg.className = 'msg';
  if (!ids.length) {
    msg.textContent = 'Chọn ít nhất một camera ở bảng Kho camera.';
    msg.className = 'msg err';
    return;
  }
  const profile = buildProfile();
  if (!profile.setCodec && !profile.setResolution && !profile.setSmartCodec && !profile.setAudioAAC && !profile.setGop && !profile.setBitrate && !profile.setOsd) {
    msg.textContent = 'Chọn ít nhất một thiết lập để thay đổi.';
    msg.className = 'msg err';
    return;
  }
  const btn = document.getElementById('apply-btn');
  setCameraTask('results');
  setBusy(btn, true, 'Đang áp dụng...');
  clearLog();
  msg.textContent = `Đang áp dụng tuần tự cho ${ids.length} camera...`;
  try {
    const results = await streamApply(ids, profile);
    renderResults(results);
    msg.textContent = 'Hoàn tất.';
    msg.className = 'msg ok';
    const ok = results.filter(r => r.ok).length;
    const fail = results.length - ok;
    lastRun = { type: 'apply', total: results.length, ok, fail, time: logTime() };
    showToast(`Áp dụng xong: ${ok} OK, ${fail} lỗi.`, fail ? 'err' : 'ok');
    for (const r of results) if (r.ok) delete probeCache[r.deviceId];
    renderCameras();
  } catch (e) {
    msg.textContent = 'Lỗi: ' + e.message;
    msg.className = 'msg err';
    showToast('Lỗi: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
});

document.getElementById('pw-btn').addEventListener('click', async () => {
  const ids = selectedCameraIds();
  const msg = document.getElementById('apply-msg');
  msg.textContent = ''; msg.className = 'msg';
  if (!ids.length) { msg.textContent = 'Chọn ít nhất một camera ở bảng Kho camera.'; msg.className = 'msg err'; return; }
  const user = (document.getElementById('pw-user').value || 'admin').trim();
  const pass = document.getElementById('pw-pass').value;
  if (!pass) { msg.textContent = 'Nhập mật khẩu mới.'; msg.className = 'msg err'; return; }
  const ok = await showConfirm(
    'Đổi mật khẩu camera',
    `Đổi mật khẩu ${ids.length} camera thành tài khoản "${user}"?\nKho sẽ tự cập nhật để vẫn kết nối được.`,
    { danger: true, okLabel: 'Đổi mật khẩu' }
  );
  if (!ok) return;
  const btn = document.getElementById('pw-btn');
  setCameraTask('results');
  setBusy(btn, true, 'Đang đổi...');
  clearLog();
  msg.textContent = `Đang đổi mật khẩu ${ids.length} camera...`;
  try {
    const results = await streamPost('/api/password', {
      deviceIds: ids, newUsername: user, newPassword: pass, timeoutSeconds: timeoutSec(),
    });
    renderResults(results);
    msg.textContent = 'Hoàn tất.'; msg.className = 'msg ok';
    const okCount = results.filter(r => r.ok).length;
    const failCount = results.length - okCount;
    lastRun = { type: 'password', total: results.length, ok: okCount, fail: failCount, time: logTime() };
    showToast(`Đổi mật khẩu xong: ${okCount} OK, ${failCount} lỗi.`, failCount ? 'err' : 'ok');
    for (const r of results) if (r.ok) delete probeCache[r.deviceId];
    renderCameras();
  } catch (e) {
    msg.textContent = 'Lỗi: ' + e.message; msg.className = 'msg err';
    showToast('Lỗi: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
});

/* ---------- network scan ---------- */

// Discovery payloads are intentionally best-effort.  ONVIF devices often
// omit a vendor/port, and Dahua OEMs such as Lechange are variously reported
// as "LC", "Lechange", or "Dahua".  Normalize only the aliases we can map
// safely so the add-to-inventory and credential-test paths receive a vendor
// the backend understands.
function scanVendorClass(value) {
  const text = String(value == null ? '' : value).trim().toLocaleLowerCase();
  if (!text) return '';
  if (/(^|[^a-z0-9])(dahua|kbvision|lechange|imou)([^a-z0-9]|$)/.test(text) ||
      /(^|[^a-z0-9])ipc[-_](?:h|c|f|a|b|k|p|w)[a-z0-9_-]*/.test(text) ||
      /(^|[^a-z0-9])lc(?:[-_\s]|$)/.test(text) ||
      (/(^|[^a-z0-9])ip[_-]?camera([^a-z0-9]|$)/.test(text) &&
        /(^|[^a-z0-9])general([^a-z0-9]|$)/.test(text))) return 'dahua';
  if (/(^|[^a-z0-9])hikvision([^a-z0-9]|$)/.test(text) || text === 'hik' || text.startsWith('hik-')) return 'hikvision';
  if (/(^|[^a-z0-9])tiandy([^a-z0-9]|$)/.test(text)) return 'tiandy';
  return '';
}

function scanPortNumber(value) {
  const n = Number.parseInt(value, 10);
  return Number.isInteger(n) && n > 0 && n <= 65535 ? n : 0;
}

function normalizeScanResult(raw) {
  const result = raw && typeof raw === 'object' ? Object.assign({}, raw) : {};
  const hints = [result.vendor, result.model, result.name, result.manufacturer, result.deviceType, result.via];
  let vendor = scanVendorClass(result.vendor);
  if (!vendor) vendor = scanVendorClass(hints.join(' '));
  const reportedPort = scanPortNumber(result.port);
  // A private Dahua/Hik port is a stronger signal than an absent vendor.
  if (!vendor && [37777, 37778, 8888].includes(reportedPort)) vendor = 'dahua';
  if (!vendor && reportedPort === 8000) vendor = 'hikvision';
  if (!vendor) vendor = String(result.vendor == null ? '' : result.vendor).trim();
  let port = reportedPort;
  if (!port && vendor === 'dahua') port = 37777;
  if (!port && vendor === 'hikvision') port = 8000;
  return Object.assign(result, { vendor, port });
}

function scanResultList(payload) {
  if (Array.isArray(payload)) return payload;
  if (!payload || typeof payload !== 'object') return [];
  for (const key of ['devices', 'results', 'data']) {
    if (Array.isArray(payload[key])) return payload[key];
  }
  return [];
}

function renderScanResults() {
  const tbody = document.getElementById('scan-tbody');
  if (!scanResults.length) {
    tbody.innerHTML = '<tr><td colspan="9" class="empty-hint">Chưa quét.</td></tr>';
  } else {
    tbody.innerHTML = scanResults.map((r, i) => `
      <tr>
        <td><input type="checkbox" class="scan-cb" data-scan-idx="${i}" aria-label="Chọn ${escapeHtml(r.ip)}"></td>
        <td data-label="IP">${escapeHtml(r.ip)}</td>
        <td data-label="Cổng">${r.port ? escapeHtml(String(r.port)) : ''}</td>
        <td data-label="Hãng">${escapeHtml(r.vendor || '')}</td>
        <td data-label="Model">${escapeHtml(r.model || '')}</td>
        <td data-label="MAC">${escapeHtml(r.mac || '')}</td>
        <td data-label="Nguồn">${escapeHtml(r.via || '')}</td>
        <td data-label="Trạng thái" id="scan-status-${i}"></td>
        <td class="actions-cell"><button class="btn btn-secondary" data-scan-add="${i}">Thêm vào kho</button></td>
      </tr>
    `).join('');
  }
  updateScanTrySelection();
}

// updateScanTrySelection enables/disables the bulk try-password button and
// updates its selected-count label based on how many .scan-cb rows are
// ticked — mirrors the existing selectedCameraIds()/bulk-selected pattern
// used by the camera table.
function updateScanTrySelection() {
  const n = document.querySelectorAll('.scan-cb:checked').length;
  document.getElementById('scan-try-btn').disabled = n === 0;
  document.getElementById('scan-try-count').textContent = n ? `${n} thiết bị đã chọn` : '';
}

async function runScan(body, btn) {
  const msg = document.getElementById('scan-msg');
  msg.textContent = ''; msg.className = 'msg';
  setBusy(btn, true, 'Đang quét...');
  try {
    const payload = await api('/api/scan', { method: 'POST', body: JSON.stringify(body) });
    scanResults = scanResultList(payload).map(normalizeScanResult);
    renderScanResults();
    msg.textContent = scanResults.length ? `Tìm thấy ${scanResults.length} thiết bị.` : 'Không tìm thấy thiết bị nào.';
    msg.className = scanResults.length ? 'msg ok' : 'msg';
  } catch (e) {
    msg.textContent = 'Lỗi quét: ' + e.message;
    msg.className = 'msg err';
    showToast('Lỗi quét: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
}

document.getElementById('scan-lan-btn').addEventListener('click', () => {
  runScan({ method: 'all' }, document.getElementById('scan-lan-btn'));
});

document.getElementById('scan-nmap-btn').addEventListener('click', () => {
  const subnet = document.getElementById('scan-subnet').value.trim();
  const msg = document.getElementById('scan-msg');
  if (!/^\d{1,3}(\.\d{1,3}){3}\/\d{1,2}$/.test(subnet)) {
    msg.textContent = 'Nhập subnet dạng CIDR, ví dụ 192.168.1.0/24.';
    msg.className = 'msg err';
    return;
  }
  runScan({ method: 'nmap', subnet }, document.getElementById('scan-nmap-btn'));
});

document.getElementById('scan-tbody').addEventListener('click', async (ev) => {
  const btn = ev.target.closest('button[data-scan-add]');
  if (!btn) return;
  const index = parseInt(btn.dataset.scanAdd, 10);
  const r = scanResults[index];
  if (!r) return;
  const status = document.getElementById('scan-status-' + index);
  const username = document.getElementById('scan-try-user').value.trim();
  const password = document.getElementById('scan-try-pass').value;
  setBusy(btn, true, 'Đang lưu...');
  try {
    await api('/api/cameras', {
      method: 'POST',
      body: JSON.stringify({
        name: r.model || r.name || '',
        host: r.ip || '',
        port: r.port || 0,
        vendor: r.vendor || '',
        username,
        password,
      }),
    });
    if (status) status.innerHTML = '<span class="badge ok">Đã lưu</span>';
    showToast('Đã thêm/cập nhật camera trong kho.', 'ok');
    // Refresh the hidden inventory view without leaving the scan workflow.
    await loadCameras();
  } catch (e) {
    if (status) status.innerHTML = `<span class="badge fail">Lỗi: ${escapeHtml(e.message)}</span>`;
    showToast('Lỗi lưu camera: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
});
document.getElementById('scan-tbody').addEventListener('change', (ev) => {
  if (ev.target.classList.contains('scan-cb')) updateScanTrySelection();
});
document.getElementById('scan-select-all').addEventListener('change', (ev) => {
  document.querySelectorAll('.scan-cb').forEach(cb => { cb.checked = ev.target.checked; });
  updateScanTrySelection();
});

/* ---------- Thử mật khẩu hàng loạt (Quét mạng) ---------- */

const scanTryProgress = progressBar('scan-try-progress');
const setScanTryProgress = (index, total, label) => scanTryProgress.set(index, total, label);

// scanStatusBadge renders one row's Trạng thái cell: green "OK" on success,
// red with the error (title tooltip) on failure.
function scanStatusBadge(ev) {
  if (ev.ok) return '<span class="badge ok">OK</span>';
  const err = String((ev && (ev.err || ev.error || ev.message || ev.detail)) || '').trim();
  const title = err ? ` title="${escapeHtml(err)}"` : '';
  const detail = err ? `<span class="scan-status-error">: ${escapeHtml(err)}</span>` : '';
  return `<span class="badge fail"${title}>Lỗi</span>${detail}`;
}

document.getElementById('scan-try-btn').addEventListener('click', async () => {
  const idxs = Array.from(document.querySelectorAll('.scan-cb:checked')).map(cb => parseInt(cb.dataset.scanIdx, 10));
  if (!idxs.length) return;
  const username = document.getElementById('scan-try-user').value.trim();
  const password = document.getElementById('scan-try-pass').value;
  if (!username) { showToast('Cần nhập tài khoản.', 'err'); return; }

  const targets = idxs.map(i => {
    const r = scanResults[i];
    return { ip: r.ip, vendor: r.vendor || '', port: r.port || 0, label: r.model || r.mac || '' };
  });
  // idxByIP maps back to the scanResults index so a "result" event (keyed by
  // ip) can find its row again — two selected rows could share an IP only if
  // the same device showed up via two discovery methods, in which case both
  // get updated together, which is harmless.
  const idxByIP = {};
  idxs.forEach(i => {
    const ip = scanResults[i].ip;
    (idxByIP[ip] = idxByIP[ip] || []).push(i);
  });

  const btn = document.getElementById('scan-try-btn');
  const msg = document.getElementById('scan-try-msg');
  msg.textContent = ''; msg.className = 'msg';
  idxs.forEach(i => { document.getElementById('scan-status-' + i).innerHTML = ''; });
  setBusy(btn, true, 'Đang thử...');
  let okCount = 0;
  try {
    const resp = await fetch('/api/scan/try-password', {
      method: 'POST', headers: jsonHeaders,
      body: JSON.stringify({ targets, username, password, timeoutSeconds: timeoutSec() }),
    });
    if (resp.status === 401) { location.href = '/login'; return; }
    if (!resp.ok || !resp.body) {
      const text = await resp.text().catch(() => '');
      let m = text || resp.statusText;
      try { const j = JSON.parse(text); if (j && j.error) m = j.error; } catch (e) { /* not JSON */ }
      throw new Error(m);
    }
    const reader = resp.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const parts = buf.split('\n\n');
      buf = parts.pop();
      for (const part of parts) {
        const line = part.split('\n').find(l => l.startsWith('data: '));
        if (!line) continue;
        let ev;
        try { ev = JSON.parse(line.slice(6)); } catch (e) { continue; }
        if (ev.type === 'start') {
          setScanTryProgress(ev.index, ev.total, `Đang thử ${ev.index}/${ev.total}: ${ev.ip}`);
        } else if (ev.type === 'result') {
          if (ev.ok) okCount++;
          (idxByIP[ev.ip] || []).forEach(i => {
            const cell = document.getElementById('scan-status-' + i);
            if (cell) cell.innerHTML = scanStatusBadge(ev);
          });
        } else if (ev.type === 'done') {
          setScanTryProgress(null);
        }
      }
    }
    msg.textContent = `Xong: ${okCount}/${idxs.length} đăng nhập được.`;
    msg.className = okCount ? 'msg ok' : 'msg';
  } catch (e) {
    msg.textContent = 'Lỗi: ' + e.message;
    msg.className = 'msg err';
    showToast('Lỗi thử mật khẩu: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
    setScanTryProgress(null);
  }
});

/* ---------- Shinobi import ---------- */

document.getElementById('imp-file').addEventListener('change', (ev) => {
  const f = ev.target.files && ev.target.files[0];
  if (!f) return;
  const reader = new FileReader();
  reader.onload = () => {
    document.getElementById('imp-json').value = reader.result || '';
    const m = document.getElementById('imp-msg');
    m.className = 'msg'; m.textContent = `Đã nạp file "${f.name}". Bấm "Nhập vào kho".`;
  };
  reader.onerror = () => {
    const m = document.getElementById('imp-msg');
    m.className = 'msg err'; m.textContent = 'Không đọc được file.';
  };
  reader.readAsText(f);
});

document.getElementById('imp-btn').addEventListener('click', async () => {
  const msg = document.getElementById('imp-msg');
  const raw = document.getElementById('imp-json').value.trim();
  if (!raw) { msg.className = 'msg err'; msg.textContent = 'Dán JSON Shinobi vào đã.'; return; }
  const btn = document.getElementById('imp-btn');
  setBusy(btn, true, 'Đang nhập...');
  msg.className = 'msg'; msg.textContent = 'Đang nhập...';
  try {
    const res = await api('/api/import', {
      method: 'POST',
      body: JSON.stringify({
        json: raw,
        hikPort: parseInt(document.getElementById('imp-hik-port').value, 10) || 80,
        dahuaPort: parseInt(document.getElementById('imp-dahua-port').value, 10) || 37777,
      }),
    });
    const n = (res.added || []).length;
    msg.className = 'msg ok';
    msg.textContent = `Đã nhập ${n} camera` + (res.skipped ? `, bỏ qua ${res.skipped} (thiếu host).` : '.');
    showToast(`Đã nhập ${n} camera.`, 'ok');
    document.getElementById('imp-json').value = '';
    await loadCameras();
  } catch (e) {
    msg.className = 'msg err';
    msg.textContent = 'Lỗi: ' + e.message;
    showToast('Lỗi: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
});

/* ---------- Shinobi NVR Management ---------- */

let shinobiStatusCache = null;
let shinobiMonitorsCache = [];

async function renderShinobiView() {
  await Promise.all([loadShinobiStatus(), loadShinobiMonitors()]);
  populateShinobiPresetDropdown();
}

async function loadShinobiStatus() {
  const statBadge = document.getElementById('shinobi-stat-badge');
  const statUrl = document.getElementById('shinobi-stat-url');
  const statGroup = document.getElementById('shinobi-stat-group');
  const statCount = document.getElementById('shinobi-stat-count');
  const dashLink = document.getElementById('shinobi-open-dashboard');
  const msg = document.getElementById('shinobi-status-msg');

  if (!statBadge) return;
  statBadge.className = 'badge';
  statBadge.textContent = 'Đang kiểm tra…';
  if (msg) { msg.textContent = ''; msg.className = 'msg'; }

  try {
    const st = await api('/api/shinobi/status');
    shinobiStatusCache = st;
    if (!st.configured) {
      statBadge.className = 'badge badge-warn';
      statBadge.textContent = 'Chưa cấu hình';
      statUrl.textContent = '(trống trong config.yaml)';
      statGroup.textContent = '–';
      statCount.textContent = '0';
      dashLink.style.display = 'none';
      if (msg) {
        msg.className = 'msg info';
        msg.textContent = 'Shinobi chưa được cấu hình API URL / API Key trong file config.yaml.';
      }
    } else if (st.connected) {
      statBadge.className = 'badge badge-ok';
      statBadge.textContent = 'Đã kết nối ●';
      statUrl.textContent = st.apiUrl || '–';
      statGroup.textContent = st.groupKey || '–';
      statCount.textContent = String(st.monitorCount || 0);
      dashLink.style.display = 'inline-block';
      dashLink.href = st.apiUrl || '#';
    } else {
      statBadge.className = 'badge badge-err';
      statBadge.textContent = 'Mất kết nối ●';
      statUrl.textContent = st.apiUrl || '–';
      statGroup.textContent = st.groupKey || '–';
      statCount.textContent = '–';
      dashLink.style.display = 'inline-block';
      dashLink.href = st.apiUrl || '#';
      if (msg && st.error) {
        msg.className = 'msg err';
        msg.textContent = 'Không kết nối được tới Shinobi API: ' + st.error;
      }
    }
  } catch (err) {
    statBadge.className = 'badge badge-err';
    statBadge.textContent = 'Lỗi kết nối';
    if (msg) {
      msg.className = 'msg err';
      msg.textContent = 'Lỗi truy vấn trạng thái: ' + err.message;
    }
  }
}

async function loadShinobiMonitors() {
  const tbody = document.getElementById('shinobi-tbody');
  if (!tbody) return;
  tbody.innerHTML = '<tr><td colspan="8" class="empty-hint">Đang tải danh sách monitor…</td></tr>';

  try {
    const mons = await api('/api/shinobi/monitors');
    shinobiMonitorsCache = Array.isArray(mons) ? mons : [];
    renderShinobiMonitorsTable(shinobiMonitorsCache);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="8" class="empty-hint" style="color:var(--err);">Lỗi tải monitors: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function renderShinobiMonitorsTable(mons) {
  const tbody = document.getElementById('shinobi-tbody');
  if (!tbody) return;
  if (!mons || mons.length === 0) {
    tbody.innerHTML = '<tr><td colspan="8" class="empty-hint">Chưa có monitor nào trên Shinobi. Bấm "+ Thêm Monitor mới" hoặc "Đồng bộ từ KSP-Cam sang Shinobi".</td></tr>';
    return;
  }

  tbody.innerHTML = mons.map(m => {
    const mode = m.mode || 'idle';
    let modeBadge = '<span class="badge">' + escapeHtml(mode) + '</span>';
    if (mode === 'record') modeBadge = '<span class="badge badge-ok">Ghi hình (record)</span>';
    else if (mode === 'start') modeBadge = '<span class="badge badge-ok">Xem (start)</span>';
    else if (mode === 'stop') modeBadge = '<span class="badge badge-warn">Tắt (stop)</span>';

    const vcodec = (m.details && m.details.vcodec) || 'copy';
    const host = m.host || (m.details && m.details.auto_host ? m.details.auto_host.split('@')[1] || m.details.auto_host : '–');
    const port = m.port || (m.details && m.details.port) || '554';

    return `<tr>
      <td><code>${escapeHtml(m.mid)}</code></td>
      <td><strong>${escapeHtml(m.name || m.mid)}</strong></td>
      <td>${escapeHtml(host)}</td>
      <td>${escapeHtml(port)}</td>
      <td>${modeBadge}</td>
      <td><code>${escapeHtml(vcodec)}</code></td>
      <td><span class="muted">${escapeHtml(m.type || 'h264')}</span></td>
      <td style="text-align:right;">
        <div class="row" style="justify-content:flex-end;gap:0.35rem;">
          <button class="btn btn-xs btn-secondary" onclick="shinobiSetState('${escapeHtml(m.mid)}', 'record')" title="Bật ghi hình">Ghi</button>
          <button class="btn btn-xs btn-secondary" onclick="shinobiSetState('${escapeHtml(m.mid)}', 'start')" title="Xem luồng">Xem</button>
          <button class="btn btn-xs btn-secondary" onclick="shinobiSetState('${escapeHtml(m.mid)}', 'stop')" title="Dừng stream">Tắt</button>
          <button class="btn btn-xs btn-secondary" onclick="openShinobiVideosModal('${escapeHtml(m.mid)}')" title="Xem video đã lưu">Video</button>
          <button class="btn btn-xs btn-secondary" onclick="openShinobiEditModal('${escapeHtml(m.mid)}')" title="Chỉnh sửa">${ICONS.edit || 'Sửa'}</button>
          <button class="btn btn-xs btn-danger" onclick="shinobiDeleteMonitor('${escapeHtml(m.mid)}')" title="Xóa monitor">✕</button>
        </div>
      </td>
    </tr>`;
  }).join('');
}

async function shinobiSetState(mid, state) {
  try {
    await api('/api/shinobi/monitors', {
      method: 'POST',
      body: JSON.stringify({ action: 'state', monitorId: mid, state: state }),
    });
    showToast(`Đã chuyển monitor ${mid} sang chế độ ${state}`, 'ok');
    await loadShinobiMonitors();
    await loadShinobiStatus();
  } catch (err) {
    showToast(`Lỗi đổi trạng thái: ${err.message}`, 'err');
  }
}

async function shinobiDeleteMonitor(mid) {
  const confirmed = await showConfirm('Xóa Monitor Shinobi', `Bạn có chắc muốn xóa monitor "${mid}" khỏi Shinobi?`, { danger: true, okLabel: 'Xóa Monitor' });
  if (!confirmed) return;
  try {
    await api('/api/shinobi/monitors', {
      method: 'POST',
      body: JSON.stringify({ action: 'delete', monitorId: mid }),
    });
    showToast(`Đã xóa monitor ${mid}`, 'ok');
    await loadShinobiMonitors();
    await loadShinobiStatus();
  } catch (err) {
    showToast(`Lỗi xóa monitor: ${err.message}`, 'err');
  }
}

function populateShinobiPresetDropdown() {
  const select = document.getElementById('mon-preset');
  if (!select) return;
  select.innerHTML = '<option value="">-- Chọn camera trong kho để tự điền --</option>' +
    cameras.map(c => `<option value="${escapeHtml(c.id)}">${escapeHtml(c.name || c.id)} (${escapeHtml(c.host)} - ${escapeHtml(c.vendor)})</option>`).join('');

  select.onchange = () => {
    const dev = cameras.find(c => c.id === select.value);
    if (!dev) return;
    const sanitizedHost = dev.host.replace(/[^a-zA-Z0-9_-]/g, '_');
    const mid = dev.nvrChannel > 0 ? `cam_${sanitizedHost}_c${dev.nvrChannel}` : `cam_${sanitizedHost}_${dev.port || 37777}`;
    let path = `/cam/realmonitor?channel=${dev.nvrChannel || 1}&subtype=0`;
    if (dev.vendor === 'hikvision') {
      path = `/Streaming/Channels/${(dev.nvrChannel || 1) * 100 + 1}`;
    }

    document.getElementById('mon-mid').value = mid;
    document.getElementById('mon-name').value = dev.name || dev.id;
    document.getElementById('mon-host').value = dev.host;
    document.getElementById('mon-port').value = '554';
    document.getElementById('mon-path').value = path;
    document.getElementById('mon-user').value = dev.username || 'admin';
    document.getElementById('mon-pass').value = dev.password || '';
    document.getElementById('mon-mode').value = 'record';
    document.getElementById('mon-vcodec').value = 'copy';
  };
}

function openShinobiEditModal(mid) {
  const dlg = document.getElementById('shinobi-edit-modal');
  const title = document.getElementById('shinobi-edit-title');
  const form = document.getElementById('shinobi-edit-form');
  const msg = document.getElementById('shinobi-modal-msg');
  if (msg) { msg.textContent = ''; msg.className = 'msg'; }

  populateShinobiPresetDropdown();

  if (mid) {
    title.textContent = `Chỉnh sửa Monitor: ${mid}`;
    const m = shinobiMonitorsCache.find(x => x.mid === mid);
    if (m) {
      document.getElementById('mon-mid').value = m.mid;
      document.getElementById('mon-mid').readOnly = true;
      document.getElementById('mon-name').value = m.name || m.mid;
      document.getElementById('mon-host').value = m.host || '';
      document.getElementById('mon-port').value = m.port || (m.details && m.details.port) || '554';
      document.getElementById('mon-path').value = m.path || '';
      document.getElementById('mon-user').value = (m.details && m.details.muser) || '';
      document.getElementById('mon-pass').value = (m.details && m.details.mpass) || '';
      document.getElementById('mon-mode').value = m.mode || 'record';
      document.getElementById('mon-vcodec').value = (m.details && m.details.vcodec) || 'copy';
    }
  } else {
    title.textContent = 'Thêm Monitor mới';
    form.reset();
    document.getElementById('mon-mid').readOnly = false;
    document.getElementById('mon-port').value = '554';
    document.getElementById('mon-mode').value = 'record';
    document.getElementById('mon-vcodec').value = 'copy';
  }

  openDialog(dlg);
}

async function openShinobiVideosModal(mid) {
  const dlg = document.getElementById('shinobi-videos-modal');
  const title = document.getElementById('shinobi-videos-title');
  const tbody = document.getElementById('shinobi-videos-tbody');
  title.textContent = `Video đã ghi hình: ${mid}`;
  tbody.innerHTML = '<tr><td colspan="5" class="empty-hint">Đang tải danh sách video…</td></tr>';
  openDialog(dlg);

  try {
    const videos = await api(`/api/shinobi/videos?mid=${encodeURIComponent(mid)}&limit=50`);
    if (!videos || videos.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty-hint">Chưa có video bản ghi nào được lưu trên Shinobi.</td></tr>';
      return;
    }

    tbody.innerHTML = videos.map(v => {
      const timeStr = v.time ? new Date(v.time).toLocaleString('vi-VN') : '–';
      const endStr = v.end ? new Date(v.end).toLocaleTimeString('vi-VN') : '–';
      const sizeMB = v.size > 0 ? (v.size / (1024 * 1024)).toFixed(1) + ' MB' : '–';
      const href = v.href || (shinobiStatusCache && shinobiStatusCache.apiUrl ? `${shinobiStatusCache.apiUrl}${v.href}` : '#');

      return `<tr>
        <td><code>${escapeHtml(v.filename || v.mid)}</code></td>
        <td>${escapeHtml(timeStr)}</td>
        <td>${escapeHtml(endStr)}</td>
        <td>${escapeHtml(sizeMB)}</td>
        <td>
          <a class="btn btn-xs btn-secondary" href="${escapeHtml(href)}" target="_blank" rel="noopener">Tải video ↗</a>
        </td>
      </tr>`;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty-hint" style="color:var(--err);">Lỗi tải video: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function wireShinobiTabEvents() {
  const refreshBtn = document.getElementById('shinobi-refresh-btn');
  if (refreshBtn) refreshBtn.addEventListener('click', renderShinobiView);

  const checkBtn = document.getElementById('shinobi-check-conn-btn');
  if (checkBtn) checkBtn.addEventListener('click', loadShinobiStatus);

  const addBtn = document.getElementById('shinobi-add-btn');
  if (addBtn) addBtn.addEventListener('click', () => openShinobiEditModal(null));

  const editClose = document.getElementById('shinobi-edit-close');
  if (editClose) editClose.addEventListener('click', () => closeDialog(document.getElementById('shinobi-edit-modal')));

  const editCancel = document.getElementById('shinobi-modal-cancel');
  if (editCancel) editCancel.addEventListener('click', () => closeDialog(document.getElementById('shinobi-edit-modal')));

  const vidsClose = document.getElementById('shinobi-videos-close');
  if (vidsClose) vidsClose.addEventListener('click', () => closeDialog(document.getElementById('shinobi-videos-modal')));

  // Manual Trigger 1: Push Sync (cameras.yaml -> Shinobi)
  const syncToBtn = document.getElementById('shinobi-sync-to-btn');
  if (syncToBtn) {
    syncToBtn.addEventListener('click', async () => {
      const msg = document.getElementById('shinobi-sync-msg');
      setBusy(syncToBtn, true, 'Đang đồng bộ sang Shinobi…');
      if (msg) { msg.textContent = ''; msg.className = 'msg'; }
      try {
        const report = await api('/api/shinobi/sync-to-shinobi', { method: 'POST' });
        const errCount = (report.errors && report.errors.length) || 0;
        let txt = `Đồng bộ sang Shinobi thành công: Tạo mới ${report.created || 0}, Cập nhật ${report.updated || 0}, Giữ nguyên ${report.unchanged || 0}.`;
        if (errCount > 0) txt += ` (Gặp ${errCount} lỗi: ${report.errors.join('; ')})`;
        if (msg) {
          msg.className = errCount > 0 ? 'msg warn' : 'msg ok';
          msg.textContent = txt;
        }
        showToast(txt, errCount > 0 ? 'warn' : 'ok');
        await renderShinobiView();
      } catch (err) {
        if (msg) {
          msg.className = 'msg err';
          msg.textContent = 'Lỗi đồng bộ sang Shinobi: ' + err.message;
        }
        showToast('Lỗi đồng bộ: ' + err.message, 'err');
      } finally {
        setBusy(syncToBtn, false);
      }
    });
  }

  // Manual Trigger 2: Pull Sync (Shinobi -> cameras.yaml)
  const syncFromBtn = document.getElementById('shinobi-sync-from-btn');
  if (syncFromBtn) {
    syncFromBtn.addEventListener('click', async () => {
      const msg = document.getElementById('shinobi-sync-msg');
      setBusy(syncFromBtn, true, 'Đang đồng bộ về KSP-Cam…');
      if (msg) { msg.textContent = ''; msg.className = 'msg'; }
      try {
        const report = await api('/api/shinobi/sync-from-shinobi', { method: 'POST' });
        const errCount = (report.errors && report.errors.length) || 0;
        let txt = `Đồng bộ từ Shinobi về KSP-Cam thành công: Thêm mới ${report.added || 0}, Đã có/Bỏ qua ${report.skipped || 0}.`;
        if (errCount > 0) txt += ` (Gặp ${errCount} lỗi: ${report.errors.join('; ')})`;
        if (msg) {
          msg.className = errCount > 0 ? 'msg warn' : 'msg ok';
          msg.textContent = txt;
        }
        showToast(txt, errCount > 0 ? 'warn' : 'ok');
        await loadCameras();
        await renderShinobiView();
      } catch (err) {
        if (msg) {
          msg.className = 'msg err';
          msg.textContent = 'Lỗi đồng bộ từ Shinobi: ' + err.message;
        }
        showToast('Lỗi đồng bộ: ' + err.message, 'err');
      } finally {
        setBusy(syncFromBtn, false);
      }
    });
  }

  // Form Submit Add/Edit
  const editForm = document.getElementById('shinobi-edit-form');
  if (editForm) {
    editForm.addEventListener('submit', async (ev) => {
      ev.preventDefault();
      const modalMsg = document.getElementById('shinobi-modal-msg');
      const submitBtn = document.getElementById('shinobi-modal-submit');
      if (modalMsg) { modalMsg.textContent = ''; modalMsg.className = 'msg'; }

      const mid = document.getElementById('mon-mid').value.trim();
      const name = document.getElementById('mon-name').value.trim();
      const host = document.getElementById('mon-host').value.trim();
      const port = document.getElementById('mon-port').value.trim() || '554';
      const path = document.getElementById('mon-path').value.trim();
      const user = document.getElementById('mon-user').value.trim();
      const pass = document.getElementById('mon-pass').value.trim();
      const mode = document.getElementById('mon-mode').value;
      const vcodec = document.getElementById('mon-vcodec').value;

      let autoHost = `rtsp://${host}:${port}${path}`;
      if (user && pass) {
        autoHost = `rtsp://${encodeURIComponent(user)}:${encodeURIComponent(pass)}@${host}:${port}${path}`;
      } else if (user) {
        autoHost = `rtsp://${encodeURIComponent(user)}@${host}:${port}${path}`;
      }

      const mon = {
        mid: mid,
        name: name,
        type: 'h264',
        mode: mode,
        host: host,
        port: port,
        protocol: 'rtsp',
        path: path,
        ext: 'mp4',
        details: {
          auto_host: autoHost,
          muser: user,
          mpass: pass,
          port: port,
          protocol: 'rtsp',
          stream_type: 'mp4',
          stream_flv_type: 'ws',
          vcodec: vcodec,
          acodec: 'copy',
          record_vcodec: vcodec,
          record_acodec: 'aac',
        },
      };

      const isEdit = document.getElementById('mon-mid').readOnly;
      const action = isEdit ? 'edit' : 'add';

      setBusy(submitBtn, true);
      try {
        await api('/api/shinobi/monitors', {
          method: 'POST',
          body: JSON.stringify({ action: action, monitorId: mid, monitor: mon }),
        });
        showToast(isEdit ? `Đã cập nhật monitor ${mid}` : `Đã thêm monitor ${mid}`, 'ok');
        closeDialog(document.getElementById('shinobi-edit-modal'));
        await renderShinobiView();
      } catch (err) {
        if (modalMsg) {
          modalMsg.className = 'msg err';
          modalMsg.textContent = 'Lỗi: ' + err.message;
        }
      } finally {
        setBusy(submitBtn, false);
      }
    });
  }
}

/* ---------- init ---------- */

async function init() {
  // Learn the session role first so the nav/views can be gated for a viewer.
  try {
    const cfg = await api('/api/config');
    if (cfg && cfg.role) appRole = cfg.role;
    appRedbidaEnabled = cfg?.redbidaEnabled === true;
  } catch (e) { /* keep optional surfaces disabled */ }
  if (appRole === 'viewer') {
    document.body.classList.add('role-viewer');
    if (currentHash() !== 'review') location.hash = '#review';
  }
  buildNav();
  wireShinobiTabEvents();
  initQuickPtzDialog();

  document.getElementById('cam-view-table-btn')?.addEventListener('click', () => setCameraViewMode('table'));
  document.getElementById('cam-view-grid-btn')?.addEventListener('click', () => setCameraViewMode('grid'));
  setCameraViewMode(cameraViewMode);

  const themeBtn = document.getElementById('theme-toggle');
  themeBtn.innerHTML = `<span class="icon-sun">${ICONS.sun}</span><span class="icon-moon">${ICONS.moon}</span>`;
  themeBtn.addEventListener('click', toggleTheme);

  document.addEventListener('click', (ev) => {
    const goBtn = ev.target.closest('[data-goto]');
    if (goBtn) { goto(goBtn.dataset.goto); return; }
    if (ev.target.closest('#drawer-open-btn')) { openDrawer(); return; }
    if (ev.target.closest('#drawer-theme-btn')) { toggleTheme(); return; }
    if (ev.target.closest('.drawer-item[href^="#"]')) { closeDrawer(); return; }
  });
  document.getElementById('drawer-backdrop').addEventListener('click', closeDrawer);

  wireSettingsPopover();
  renderBulkSummary();

  window.addEventListener('hashchange', setRoute);
  setRoute();

  if (appRole !== 'viewer') await loadCameras();
  // Boot barrier for e2e: everything the first paint depends on has landed.
  window.__kspReady = true;
}

// The per-device timeout is global (every API call reads it), so it lives in a
// topbar popover rather than buried at the bottom of the bulk-edit card, and
// persists across reloads.
function wireSettingsPopover() {
  const btn = document.getElementById('settings-toggle');
  const pop = document.getElementById('settings-popover');
  const input = document.getElementById('g-timeout');
  const saved = parseInt(localStorage.getItem(TIMEOUT_KEY), 10);
  if (saved > 0) input.value = saved;
  input.addEventListener('change', () => {
    const v = parseInt(input.value, 10);
    if (v > 0) localStorage.setItem(TIMEOUT_KEY, String(v));
  });
  btn.addEventListener('click', (ev) => {
    ev.stopPropagation();
    pop.hidden = !pop.hidden;
    btn.setAttribute('aria-expanded', pop.hidden ? 'false' : 'true');
  });
  document.addEventListener('click', (ev) => {
    if (pop.hidden || pop.contains(ev.target) || ev.target === btn) return;
    pop.hidden = true;
    btn.setAttribute('aria-expanded', 'false');
  });
}

init();
