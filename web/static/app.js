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
};

// Nav config shared by sidebar / bottom-nav / drawer. Kho camera / Chỉnh
// hàng loạt / Kết quả live on one page (#cameras) so users don't have to
// jump tabs mid-workflow.
const NAV_ITEMS = [
  { hash: 'dashboard', label: 'Tổng quan', short: 'Tổng quan', icon: ICONS.home, bottom: true },
  { hash: 'scan', label: 'Quét mạng', short: 'Quét', icon: ICONS.radar, bottom: true },
  { hash: 'cameras', label: 'Kho camera', short: 'Camera', icon: ICONS.camera, bottom: true },
  { hash: 'import', label: 'Nhập Shinobi', short: 'Nhập', icon: ICONS.upload, bottom: true },
];
// Old bookmarks/links to the now-merged tabs still land on #cameras.
const HASH_ALIASES = { bulk: 'cameras', results: 'cameras' };

/* ---------- generic helpers ---------- */

function escapeHtml(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, ch => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[ch]));
}
function cssEscape(s) { return String(s).replace(/[^a-zA-Z0-9_-]/g, '_'); }

function timeoutSec() {
  return parseInt(document.getElementById('g-timeout').value, 10) || 30;
}

function setBusy(btn, busy, busyLabel) {
  if (!btn) return;
  if (busy) {
    btn.dataset.label = btn.dataset.label || btn.textContent;
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner"></span>' + escapeHtml(busyLabel || btn.dataset.label);
  } else {
    btn.disabled = false;
    btn.textContent = btn.dataset.label || btn.textContent;
  }
}

/* ---------- toast ---------- */

function showToast(message, type) {
  const box = document.getElementById('toast-container');
  const el = document.createElement('div');
  el.className = 'toast' + (type ? ' ' + type : '');
  el.textContent = message;
  box.appendChild(el);
  setTimeout(() => el.remove(), 4000);
}

/* ---------- confirm dialog (replaces alert()/confirm()) ---------- */

function showConfirm(title, message, opts) {
  opts = opts || {};
  const dlg = document.getElementById('confirm-dialog');
  document.getElementById('confirm-title').textContent = title;
  document.getElementById('confirm-message').textContent = message;
  const okBtn = document.getElementById('confirm-ok');
  okBtn.textContent = opts.okLabel || 'Xác nhận';
  okBtn.className = 'btn' + (opts.danger ? ' btn-danger' : '');
  dlg.showModal();
  okBtn.focus();
  return new Promise(resolve => {
    const cancelBtn = document.getElementById('confirm-cancel');
    function cleanup(result) {
      okBtn.removeEventListener('click', onOk);
      cancelBtn.removeEventListener('click', onCancel);
      dlg.removeEventListener('cancel', onCancel);
      dlg.close();
      resolve(result);
    }
    function onOk() { cleanup(true); }
    function onCancel(ev) { if (ev) ev.preventDefault(); cleanup(false); }
    okBtn.addEventListener('click', onOk);
    cancelBtn.addEventListener('click', onCancel);
    dlg.addEventListener('cancel', onCancel);
  });
}

/* ---------- API ---------- */

const jsonHeaders = { 'Accept': 'application/json', 'Content-Type': 'application/json' };

async function api(path, opts) {
  const res = await fetch(path, Object.assign({ headers: jsonHeaders }, opts || {}));
  if (res.status === 401) {
    location.href = '/login';
    throw new Error('unauthorized');
  }
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch (e) { /* not JSON */ }
  if (!res.ok) {
    const msg = (data && data.error) ? data.error : (text || res.statusText);
    throw new Error(msg);
  }
  return data;
}

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

function setProgress(index, total, label) {
  const bar = document.getElementById('apply-progress');
  const fill = document.getElementById('apply-progress-fill');
  const text = document.getElementById('apply-progress-label');
  if (index == null) { bar.classList.remove('active'); text.textContent = ''; return; }
  bar.classList.add('active');
  fill.style.width = Math.round((index / total) * 100) + '%';
  text.textContent = label || '';
}

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

/* ---------- routing ---------- */

function currentHash() {
  let h = (location.hash || '#dashboard').slice(1);
  if (HASH_ALIASES[h]) h = HASH_ALIASES[h];
  return NAV_ITEMS.some(n => n.hash === h) ? h : 'dashboard';
}

function setRoute() {
  const hash = currentHash();
  document.querySelectorAll('.view').forEach(v => v.classList.toggle('active', v.dataset.view === hash));
  const item = NAV_ITEMS.find(n => n.hash === hash);
  document.getElementById('view-title').textContent = item ? item.label : '';
  document.querySelectorAll('[data-nav-hash]').forEach(el => {
    el.classList.toggle('active', el.dataset.navHash === hash);
  });
  closeDrawer();
  if (hash === 'cameras') renderBulkSelection();
  if (hash === 'dashboard') renderDashboard();
}

function goto(hash) { location.hash = '#' + hash; }

/* ---------- nav rendering (sidebar / bottom-nav / drawer) ---------- */

function buildNav() {
  const sidebar = document.getElementById('sidebar-nav');
  sidebar.innerHTML = NAV_ITEMS.map(n => `
    <a class="nav-link" href="#${n.hash}" data-nav-hash="${n.hash}">
      ${n.icon}<span>${n.label}</span>
    </a>`).join('');

  const bottomnav = document.getElementById('bottomnav');
  bottomnav.innerHTML = NAV_ITEMS.map(n => `
    <a class="bottomnav-item" href="#${n.hash}" data-nav-hash="${n.hash}">${n.icon}<span>${n.short || n.label}</span></a>
  `).join('') + `
    <button class="bottomnav-item" id="drawer-open-btn" type="button">${ICONS.dots}<span>Menu</span></button>
  `;

  const drawer = document.getElementById('drawer-nav');
  drawer.innerHTML = `
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
    <tr class="skeleton-row"><td colspan="7"><span class="skeleton" style="width:100%;display:block"></span></td></tr>
  `).join('');
}

function renderCameras() {
  const tbody = document.getElementById('cam-tbody');
  if (!cameras.length) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty-hint">Chưa có camera nào. Thêm ở form phía trên.</td></tr>';
    renderDashboard();
    return;
  }
  const checked = new Set(Array.from(document.querySelectorAll('.cam-cb:checked')).map(cb => cb.value));
  tbody.innerHTML = cameras.map(c => `
    <tr data-id="${escapeHtml(c.id)}">
      <td class="cell-check"><input type="checkbox" class="cam-cb" value="${escapeHtml(c.id)}" ${checked.has(c.id) ? 'checked' : ''} aria-label="Chọn camera"></td>
      <td data-label="Tên" class="cell-name">
        <span class="cell-name-text">${escapeHtml(c.name || '(chưa đặt tên)')}</span>
        <button class="btn-icon" data-action="rename-inline" data-id="${escapeHtml(c.id)}" title="Sửa nhanh tên trong kho" aria-label="Sửa tên">${ICONS.edit}</button>
      </td>
      <td data-label="Host">${escapeHtml(c.host)}</td>
      <td data-label="Cổng">${c.port}</td>
      <td data-label="Hãng">${escapeHtml(c.vendor)}</td>
      <td data-label="Thông tin luồng" class="probe-box" id="probe-${cssEscape(c.id)}">${fmtStreamInfo(probeCache[c.id]) || '<span class="muted">chưa dò</span>'}</td>
      <td class="actions-cell">
        <button class="btn btn-secondary" data-action="probe" data-id="${escapeHtml(c.id)}">Dò</button>
        <button class="btn btn-secondary" data-action="view" data-id="${escapeHtml(c.id)}">Xem hình</button>
        <button class="btn btn-secondary" data-action="view-all" data-id="${escapeHtml(c.id)}">Tất cả kênh</button>
        <button class="btn btn-secondary" data-action="edit" data-id="${escapeHtml(c.id)}">Sửa</button>
        <button class="btn btn-danger" data-action="delete" data-id="${escapeHtml(c.id)}">Xóa</button>
      </td>
    </tr>
  `).join('');
  renderDashboard();
}

async function loadCameras() {
  renderCameraSkeleton();
  try {
    cameras = await api('/api/cameras');
    renderCameras();
  } catch (e) {
    document.getElementById('cam-tbody').innerHTML =
      `<tr><td colspan="7"><span class="msg err">Lỗi tải danh sách: ${escapeHtml(e.message)}</span></td></tr>`;
  }
}

function selectedCameraIds() {
  return Array.from(document.querySelectorAll('.cam-cb:checked')).map(cb => cb.value);
}

function renderBulkSelection() {
  const ids = selectedCameraIds();
  const countEl = document.getElementById('bulk-selected-count');
  const chipsEl = document.getElementById('bulk-selected-chips');
  document.getElementById('apply-count').textContent = ids.length ? ids.length + ' camera đã chọn' : '';
  if (!ids.length) {
    countEl.textContent = 'Chưa chọn camera nào.';
    chipsEl.innerHTML = '';
    return;
  }
  countEl.textContent = ids.length + ' camera đã chọn:';
  chipsEl.innerHTML = ids.map(id => {
    const c = cameras.find(x => x.id === id);
    return `<span class="chip">${escapeHtml(c ? (c.name || c.host) : id)}</span>`;
  }).join('');
}

/* ---------- add / edit / delete / probe ---------- */

document.getElementById('add-form').addEventListener('submit', async (ev) => {
  ev.preventDefault();
  const msg = document.getElementById('add-msg');
  msg.textContent = ''; msg.className = 'msg';
  const body = {
    name: document.getElementById('f-name').value.trim(),
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
    ev.target.reset();
    document.getElementById('f-vendor').value = body.vendor;
    msg.textContent = 'Đã thêm camera.';
    msg.className = 'msg ok';
    showToast('Đã lưu camera.', 'ok');
    await loadCameras();
  } catch (e) {
    msg.textContent = 'Lỗi: ' + e.message;
    msg.className = 'msg err';
    showToast('Lỗi: ' + e.message, 'err');
  } finally {
    setBusy(btn, false);
  }
});

document.getElementById('cam-tbody').addEventListener('click', async (ev) => {
  const btn = ev.target.closest('button[data-action]');
  if (!btn) return;
  const id = btn.dataset.id;
  if (btn.dataset.action === 'edit') {
    const c = cameras.find(x => x.id === id);
    if (!c) return;
    document.getElementById('f-name').value = c.name || '';
    document.getElementById('f-host').value = c.host || '';
    document.getElementById('f-port').value = c.port || '';
    document.getElementById('f-vendor').value = c.vendor || 'dahua';
    document.getElementById('f-username').value = c.username || '';
    const pw = document.getElementById('f-password');
    pw.value = '';
    pw.placeholder = 'để trống = giữ mật khẩu cũ';
    const m = document.getElementById('add-msg');
    m.className = 'msg'; m.textContent = 'Đang sửa "' + (c.name || c.host) + '". Đổi thông tin rồi bấm "Thêm/Lưu camera". (Đổi host/cổng sẽ tạo mục mới.)';
    goto('cameras');
    document.getElementById('add-form').scrollIntoView({ behavior: 'smooth', block: 'center' });
    document.getElementById('f-name').focus();
    if (c.vendor === 'dahua') {
      openNetworkCard(c);
    } else {
      closeNetworkCard();
    }
    return;
  }
  if (btn.dataset.action === 'delete') {
    const c = cameras.find(x => x.id === id);
    const ok = await showConfirm('Xóa camera', `Xóa camera "${c ? (c.name || c.host) : id}" khỏi kho?`, { danger: true, okLabel: 'Xóa' });
    if (!ok) return;
    btn.disabled = true;
    try {
      await api('/api/cameras/delete', { method: 'POST', body: JSON.stringify({ id, timeoutSeconds: timeoutSec() }) });
      delete probeCache[id];
      showToast('Đã xóa camera.', 'ok');
      await loadCameras();
    } catch (e) {
      showToast('Lỗi xóa: ' + e.message, 'err');
      btn.disabled = false;
    }
  } else if (btn.dataset.action === 'probe') {
    btn.disabled = true;
    const cell = document.getElementById('probe-' + cssEscape(id));
    cell.innerHTML = '<span class="muted">đang dò...</span>';
    try {
      const info = await api('/api/probe', { method: 'POST', body: JSON.stringify({ id, timeoutSeconds: timeoutSec() }) });
      probeCache[id] = info;
      cell.innerHTML = fmtStreamInfo(info);
    } catch (e) {
      cell.innerHTML = `<span class="msg err">${escapeHtml(e.message)}</span>`;
    } finally {
      btn.disabled = false;
    }
  } else if (btn.dataset.action === 'view') {
    openGallery(buildTiles([id]));
  } else if (btn.dataset.action === 'view-all') {
    await viewAllChannels(id, btn);
  } else if (btn.dataset.action === 'rename-inline') {
    startInlineRename(btn.closest('.cell-name'), id);
  }
});

/* ---------- Mạng (Dahua/KBVision): static IP + Wi-Fi, device-level ---------- */

let networkEditDevice = null; // {id, name, host, vendor, ...} of the device whose Mạng card is open
let lastWiFiConfig = null; // last-fetched WLan config, kept so a static-IP save doesn't have to re-fetch/hide it

function closeNetworkCard() {
  networkEditDevice = null;
  lastWiFiConfig = null;
  document.getElementById('network-card').hidden = true;
}

async function openNetworkCard(c) {
  networkEditDevice = c;
  lastWiFiConfig = null;
  const card = document.getElementById('network-card');
  const body = document.getElementById('net-body');
  const msg = document.getElementById('net-msg');
  document.getElementById('net-device-name').textContent = c.name || c.host;
  card.hidden = false;
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
    <p class="muted">MAC: ${escapeHtml(iface.PhysicalAddress || '–')} · MTU: ${escapeHtml(String(iface.MTU == null ? '–' : iface.MTU))}</p>
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
      <div class="field field-sm"><label for="net-wifi-pass">Mật khẩu Wi-Fi (để trống = giữ nguyên)</label><input id="net-wifi-pass" type="password" placeholder="••••••"></div>
      <div class="row"><button class="btn btn-secondary" type="button" id="net-wifi-scan-btn">Quét Wi-Fi</button></div>
      <div id="net-wifi-scan-results"></div>
      <div class="checkbox-row"><input type="checkbox" id="net-wifi-confirm-risk"><label for="net-wifi-confirm-risk">Tôi hiểu đổi Wi-Fi sai có thể khiến camera mất kết nối.</label></div>
      <button class="btn btn-danger" type="button" id="net-wifi-save-btn" disabled>Lưu Wi-Fi</button>
    `;
  }
  body.innerHTML = html;

  const dhcpCb = document.getElementById('net-dhcp');
  dhcpCb.addEventListener('change', () => { document.getElementById('net-static-fields').hidden = dhcpCb.checked; });
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
    await api('/api/network', { method: 'POST', body: JSON.stringify(body) });
    msg.textContent = 'Đã lưu cấu hình mạng.';
    msg.className = 'msg ok';
    showToast('Đã lưu cấu hình mạng.', 'ok');
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
      results.innerHTML = '<div class="chip-list">' + devices.map(d =>
        `<button type="button" class="chip chip-btn" data-wifi-ssid="${escapeHtml(d.ssid)}">${escapeHtml(d.ssid)} (${d.linkQuality}%)</button>`
      ).join('') + '</div>';
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

document.getElementById('select-all').addEventListener('change', (ev) => {
  document.querySelectorAll('.cam-cb').forEach(cb => cb.checked = ev.target.checked);
  renderBulkSelection();
});

document.getElementById('cam-tbody').addEventListener('change', (ev) => {
  if (ev.target.classList.contains('cam-cb')) renderBulkSelection();
});

/* ---------- bulk-edit form wiring ---------- */

function wireToggle(enableId, fieldsId) {
  const enable = document.getElementById(enableId);
  const fields = document.getElementById(fieldsId);
  enable.addEventListener('change', () => { fields.hidden = !enable.checked; });
}
wireToggle('p-codec-enable', 'p-codec-fields');
wireToggle('p-res-enable', 'p-res-fields');
wireToggle('p-smart-enable', 'p-smart-fields');
wireToggle('p-gop-enable', 'p-gop-fields');
wireToggle('p-bitrate-enable', 'p-bitrate-fields');

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
      const info = await api('/api/probe', { method: 'POST', body: JSON.stringify({ id, timeoutSeconds: timeoutSec() }) });
      probeCache[id] = info;
      if (cell) cell.innerHTML = fmtStreamInfo(info);
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
    const info = probeCache[id] || await api('/api/probe', { method: 'POST', body: JSON.stringify({ id, timeoutSeconds: timeoutSec() }) });
    probeCache[id] = info;
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
  if (editBtn) { openChannelEdit(galleryTiles[parseInt(editBtn.dataset.galleryEdit, 10)]); return; }
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

// switchCeTab shows the requested channel-edit-dialog panel ('name' or
// 'picture') and lazily loads the Chỉnh màu tab's data the first time it's
// opened for the current tile (picturePayload stays null until then).
function switchCeTab(tab) {
  ceActiveTab = tab;
  document.querySelectorAll('#ce-tabs .tab-btn').forEach(b => b.classList.toggle('active', b.dataset.ceTab === tab));
  document.getElementById('ce-panel-name').hidden = tab !== 'name';
  document.getElementById('ce-panel-picture').hidden = tab !== 'picture';
  if (tab === 'picture' && channelEditTile && !picturePayload) {
    loadPictureTab(channelEditTile);
  }
}

document.getElementById('ce-tabs').addEventListener('click', (ev) => {
  const btn = ev.target.closest('.tab-btn');
  if (btn && !btn.hidden) switchCeTab(btn.dataset.ceTab);
});

async function openChannelEdit(tile) {
  channelEditTile = tile;
  picturePayload = null;
  const dlg = document.getElementById('channel-edit-dialog');
  const msg = document.getElementById('ce-msg');
  const nameInput = document.getElementById('ce-name');
  const osdFields = document.getElementById('ce-osd-fields');
  const osdHint = document.getElementById('ce-osd-hint');
  document.getElementById('ce-title').textContent = `Sửa tên & OSD — ${tile.camName} K${tile.channel + 1}`;
  nameInput.value = '';
  osdFields.innerHTML = '';
  osdHint.textContent = '';
  document.getElementById('ce-picture-body').innerHTML = '';
  document.getElementById('ce-picture-msg').textContent = '';
  const cam = cameras.find(x => x.id === tile.camId);
  document.getElementById('ce-tab-btn-picture').hidden = !cam || cam.vendor !== 'dahua';
  switchCeTab('name');
  msg.textContent = 'Đang tải...'; msg.className = 'msg';
  dlg.showModal();
  try {
    const q = `id=${encodeURIComponent(tile.camId)}&channel=${tile.channel}&timeoutSeconds=${timeoutSec()}`;
    const info = await api('/api/channel-info?' + q);
    nameInput.value = info.name || '';
    if (info.osdSupported) {
      const lines = (info.osdLines && info.osdLines.length ? info.osdLines : ['', '', '', '']);
      osdFields.innerHTML = lines.map((line, i) => `
        <div class="field field-sm">
          <label>Dòng OSD ${i + 1}</label>
          <input class="ce-osd-line" value="${escapeHtml(line || '')}">
        </div>
      `).join('');
    } else {
      osdHint.textContent = 'Camera này không hỗ trợ (hoặc chưa xác minh) chỉnh OSD qua API — xem docs/GOTCHAS.md.';
    }
    msg.textContent = ''; msg.className = 'msg';
  } catch (e) {
    msg.textContent = 'Lỗi tải: ' + e.message;
    msg.className = 'msg err';
  }
}

document.getElementById('ce-cancel').addEventListener('click', () => document.getElementById('channel-edit-dialog').close());

document.getElementById('ce-save').addEventListener('click', async () => {
  if (!channelEditTile) return;
  const btn = document.getElementById('ce-save');
  const msg = document.getElementById('ce-msg');
  const name = document.getElementById('ce-name').value;
  const lines = Array.from(document.querySelectorAll('.ce-osd-line')).map(el => el.value);
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
        body: JSON.stringify({ id: channelEditTile.camId, channel: channelEditTile.channel, lines, timeoutSeconds: timeoutSec() }),
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
  const msg = document.getElementById('ce-picture-msg');
  body.innerHTML = '<span class="muted">Đang tải...</span>';
  msg.textContent = ''; msg.className = 'msg';
  try {
    const q = `id=${encodeURIComponent(tile.camId)}&channel=${tile.channel}&timeoutSeconds=${timeoutSec()}`;
    const info = await api('/api/picture?' + q);
    picturePayload = { color: info.color || {}, options: info.options || {} };
    const capsHint = info.capsError
      ? `<p class="muted">Không đọc được thông tin hỗ trợ (caps): ${escapeHtml(info.capsError)} — mọi trường vẫn hiện, camera có thể bỏ qua trường không hỗ trợ khi lưu.</p>`
      : '';
    const options = info.options || {};
    body.innerHTML = capsHint +
      collapsibleSection('Màu sắc', renderObjectFields(info.color, '', null, 'color'), 'pf-section-color', true) +
      collapsibleSection('Ảnh chung', renderObjectFields(options, '', ['NightOptions', 'NormalOptions'], 'options'), 'pf-section-options', true) +
      collapsibleSection('Ban đêm (NightOptions)', renderObjectFields(options.NightOptions, 'NightOptions', null, 'options'), 'pf-section-night', false) +
      collapsibleSection('Ban ngày phụ (NormalOptions)', renderObjectFields(options.NormalOptions, 'NormalOptions', null, 'options'), 'pf-section-normal', false);
    if (!body.innerHTML) body.innerHTML = '<p class="muted">Camera không trả về trường nào.</p>';
    applyCapsDisabling(info.caps);
  } catch (e) {
    body.innerHTML = '';
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
  document.querySelectorAll('#ce-picture-body [data-pf-key]').forEach(el => {
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
  if (!profile.setCodec && !profile.setResolution && !profile.setSmartCodec && !profile.setAudioAAC && !profile.setGop && !profile.setBitrate) {
    msg.textContent = 'Chọn ít nhất một thiết lập để thay đổi.';
    msg.className = 'msg err';
    return;
  }
  const btn = document.getElementById('apply-btn');
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

function renderScanResults() {
  const tbody = document.getElementById('scan-tbody');
  if (!scanResults.length) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty-hint">Chưa quét.</td></tr>';
    return;
  }
  tbody.innerHTML = scanResults.map((r, i) => `
    <tr>
      <td data-label="IP">${escapeHtml(r.ip)}</td>
      <td data-label="Cổng">${r.port ? escapeHtml(String(r.port)) : ''}</td>
      <td data-label="Hãng">${escapeHtml(r.vendor || '')}</td>
      <td data-label="Model">${escapeHtml(r.model || '')}</td>
      <td data-label="MAC">${escapeHtml(r.mac || '')}</td>
      <td data-label="Nguồn">${escapeHtml(r.via || '')}</td>
      <td class="actions-cell"><button class="btn btn-secondary" data-scan-add="${i}">Thêm vào kho</button></td>
    </tr>
  `).join('');
}

async function runScan(body, btn) {
  const msg = document.getElementById('scan-msg');
  msg.textContent = ''; msg.className = 'msg';
  setBusy(btn, true, 'Đang quét...');
  try {
    scanResults = await api('/api/scan', { method: 'POST', body: JSON.stringify(body) });
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

document.getElementById('scan-tbody').addEventListener('click', (ev) => {
  const btn = ev.target.closest('button[data-scan-add]');
  if (!btn) return;
  const r = scanResults[parseInt(btn.dataset.scanAdd, 10)];
  if (!r) return;
  document.getElementById('f-host').value = r.ip || '';
  document.getElementById('f-port').value = r.port || '';
  if (r.vendor === 'dahua' || r.vendor === 'hikvision') document.getElementById('f-vendor').value = r.vendor;
  document.getElementById('f-name').value = r.model || r.name || '';
  goto('cameras');
  document.getElementById('add-form').scrollIntoView({ behavior: 'smooth', block: 'center' });
  document.getElementById('f-host').focus();
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

/* ---------- init ---------- */

function init() {
  buildNav();

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

  window.addEventListener('hashchange', setRoute);
  setRoute();

  loadCameras();
}

init();
