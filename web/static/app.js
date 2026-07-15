/* ksp-camera-auto — dashboard logic. Vanilla JS, no build step, no deps. */

/* ---------- inline icons (Tabler-style, stroke 2, currentColor) ---------- */

const ICONS = {
  home: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 10.5 12 4l8 6.5"/><path d="M6 9.5V20h12V9.5"/><path d="M10 20v-6h4v6"/></svg>',
  radar: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 12l6-6"/><path d="M12 3a9 9 0 1 0 9 9"/><path d="M12 7a5 5 0 1 0 5 5"/></svg>',
  camera: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="6" width="12" height="12" rx="2"/><path d="M15 10l6-3v10l-6-3"/></svg>',
  sliders: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="3" y1="6" x2="21" y2="6"/><circle cx="9" cy="6" r="2"/><line x1="3" y1="12" x2="21" y2="12"/><circle cx="15" cy="12" r="2"/><line x1="3" y1="18" x2="21" y2="18"/><circle cx="7" cy="18" r="2"/></svg>',
  upload: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21V9"/><path d="M7 14l5-5 5 5"/><path d="M5 21h14"/></svg>',
  list: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 6h11M9 12h11M9 18h11"/><path d="M4 6l1.4 1.4L8 4.8"/><path d="M4 12l1.4 1.4L8 10.8"/><path d="M4 18l1.4 1.4L8 16.8"/></svg>',
  dots: '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" stroke="none"><circle cx="5" cy="12" r="1.6"/><circle cx="12" cy="12" r="1.6"/><circle cx="19" cy="12" r="1.6"/></svg>',
  sun: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>',
  moon: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8z"/></svg>',
  logout: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="M16 17l5-5-5-5"/><path d="M21 12H9"/></svg>',
};

// Nav config shared by sidebar / bottom-nav / drawer.
const NAV_ITEMS = [
  { hash: 'dashboard', label: 'Tổng quan', short: 'Tổng quan', icon: ICONS.home, bottom: true },
  { hash: 'scan', label: 'Quét mạng', short: 'Quét', icon: ICONS.radar, bottom: true },
  { hash: 'cameras', label: 'Kho camera', short: 'Camera', icon: ICONS.camera, bottom: true },
  { hash: 'bulk', label: 'Chỉnh hàng loạt', short: 'Hàng loạt', icon: ICONS.sliders, bottom: true },
  { hash: 'import', label: 'Nhập Shinobi', icon: ICONS.upload, bottom: false },
  { hash: 'results', label: 'Kết quả', icon: ICONS.list, bottom: false, badge: true },
];

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
    const audio = s.audioEnable ? (s.audioCodec || 'on') : 'tắt';
    const codec = s.compression ? (s.compression + (s.profile ? '/' + s.profile : '')) : '';
    const fps = (s.fps > 0 ? s.fps + 'fps' : 'fps theo nguồn');
    const gop = (s.gop > 0 ? ' · GOP ' + s.gop : '');
    const bitrate = (s.bitrateKbps > 0 ? ' · ' + s.bitrateKbps + 'Kbps' + (s.bitrateMode ? ' ' + s.bitrateMode : '') : '');
    return `${ch}${label}: ${s.width}x${s.height} ${codec} · ${fps} · audio ${audio} · smart ${s.smartCodec ? 'bật' : 'tắt'}${gop}${bitrate}`;
  }).join('<br>');
}

/* ---------- routing ---------- */

function currentHash() {
  const h = (location.hash || '#dashboard').slice(1);
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
  if (hash === 'bulk') renderBulkSelection();
  if (hash === 'dashboard') renderDashboard();
}

function goto(hash) { location.hash = '#' + hash; }

/* ---------- nav rendering (sidebar / bottom-nav / drawer) ---------- */

function badgeSpan() { return '<span class="nav-badge" data-badge-for="results" hidden>0</span>'; }

function buildNav() {
  const sidebar = document.getElementById('sidebar-nav');
  sidebar.innerHTML = NAV_ITEMS.map(n => `
    <a class="nav-link" href="#${n.hash}" data-nav-hash="${n.hash}">
      ${n.icon}<span>${n.label}</span>${n.badge ? badgeSpan() : ''}
    </a>`).join('');

  const bottomItems = NAV_ITEMS.filter(n => n.bottom);
  const bottomnav = document.getElementById('bottomnav');
  bottomnav.innerHTML = bottomItems.map(n => `
    <a class="bottomnav-item" href="#${n.hash}" data-nav-hash="${n.hash}">${n.icon}<span>${n.short || n.label}</span></a>
  `).join('') + `
    <button class="bottomnav-item" id="drawer-open-btn" type="button">${ICONS.dots}<span>Menu</span>${badgeSpan()}</button>
  `;

  const drawerItems = NAV_ITEMS.filter(n => !n.bottom);
  const drawer = document.getElementById('drawer-nav');
  drawer.innerHTML = drawerItems.map(n => `
    <a class="drawer-item" href="#${n.hash}" data-nav-hash="${n.hash}">${n.icon}<span>${n.label}</span>${n.badge ? badgeSpan() : ''}</a>
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

function updateResultBadge(count) {
  document.querySelectorAll('[data-badge-for="results"]').forEach(el => {
    el.textContent = String(count);
    el.hidden = !count;
  });
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
      <td data-label="Tên">${escapeHtml(c.name || '(chưa đặt tên)')}</td>
      <td data-label="Host">${escapeHtml(c.host)}</td>
      <td data-label="Cổng">${c.port}</td>
      <td data-label="Hãng">${escapeHtml(c.vendor)}</td>
      <td data-label="Thông tin luồng" class="probe-box" id="probe-${cssEscape(c.id)}">${fmtStreamInfo(probeCache[c.id]) || '<span class="muted">chưa dò</span>'}</td>
      <td class="actions-cell">
        <button class="btn btn-secondary" data-action="probe" data-id="${escapeHtml(c.id)}">Dò</button>
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
  }
});

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
    updateResultBadge(0);
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
  updateResultBadge(results.length);
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
