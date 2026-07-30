/* Shared UI primitives. Loaded before feature scripts; no build step required. */

function escapeHtml(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, ch => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[ch]));
}

function cssEscape(s) { return String(s).replace(/[^a-zA-Z0-9_-]/g, '_'); }

// timeoutSec is the per-device operation timeout every API call passes along.
// It lives in the topbar settings popover (#g-timeout) and is mirrored to
// localStorage, so a page that hasn't rendered the popover yet — or a future
// layout that drops it — still gets the last chosen value instead of throwing.
const TIMEOUT_KEY = 'kspcam-timeout';
function timeoutSec() {
  const input = document.getElementById('g-timeout');
  const fromInput = parseInt(input && input.value, 10);
  if (fromInput > 0) return fromInput;
  return parseInt(localStorage.getItem(TIMEOUT_KEY), 10) || 30;
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

function showToast(message, type) {
  const box = document.getElementById('toast-container');
  const el = document.createElement('div');
  el.className = 'toast' + (type ? ' ' + type : '');
  el.textContent = message;
  box.appendChild(el);
  setTimeout(() => el.remove(), 4000);
}

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

/* ---------- dialogs ---------- */

// openDialog centralises the <dialog> lifecycle that used to be re-implemented
// per dialog: show, close-on-backdrop-click, and a single teardown hook that
// runs however the dialog closes (button, Escape, backdrop). Returns nothing;
// close with closeDialog(el) or the dialog's own close button.
function openDialog(el, opts) {
  opts = opts || {};
  if (!el || el.open) return;
  if (!el.dataset.dlgWired) {
    el.dataset.dlgWired = '1';
    // A native <dialog>'s ::backdrop click lands on the dialog element itself;
    // clicks on real content hit a child, so target===el means the backdrop.
    el.addEventListener('click', ev => {
      if (ev.target === el && el.dataset.dlgBackdrop === '1') closeDialog(el);
    });
    el.addEventListener('close', () => {
      const fn = el._dlgOnClose;
      el._dlgOnClose = null;
      if (fn) fn();
    });
  }
  el.dataset.dlgBackdrop = opts.backdropClose === false ? '0' : '1';
  el._dlgOnClose = opts.onClose || null;
  el.showModal();
  const focusTarget = el.querySelector('[autofocus], input, select, textarea, button');
  if (focusTarget) focusTarget.focus();
}

function closeDialog(el) { if (el && el.open) el.close(); }

/* ---------- progress bars ---------- */

// progressBar wraps the three-element (<track>, <fill>, <label>) markup used by
// apply, the bulk password try and the review export. `prefix` names the ids:
// `${prefix}` (track, gets .active), `${prefix}-fill`, `${prefix}-label`.
function progressBar(prefix) {
  const bar = () => document.getElementById(prefix);
  const fill = () => document.getElementById(prefix + '-fill');
  const label = () => document.getElementById(prefix + '-label');
  return {
    // set(done, total, text) — done==null resets and hides the bar.
    set(done, total, text) {
      const b = bar(), f = fill(), l = label();
      if (!b) return;
      if (done == null) {
        b.classList.remove('active');
        if (f) f.style.width = '0%';
        if (l) l.textContent = '';
        return;
      }
      b.classList.add('active');
      const pct = total > 0 ? Math.round((done / total) * 100) : 0;
      if (f) f.style.width = Math.max(0, Math.min(100, pct)) + '%';
      if (l) l.textContent = text || '';
    },
    reset() { this.set(null); },
  };
}

/* ---------- live MJPEG preview ---------- */

// livePreview owns one /api/live <img> session. It used to exist three times
// over (PTZ / colour / OSD tabs) with a `liveEls` pointer swapping between
// them; the camera detail page keeps a single instance alive across tabs
// instead, which is the whole point — you can watch the picture while you
// change it. Server caps a session at 5 minutes; extend() reconnects.
//
// els: { img, start, extend, stop, status } DOM nodes.
// source(): returns { id, channel } or null when nothing is selected.
function livePreview(els, source) {
  const SESSION_MS = 5 * 60 * 1000;
  let timer = null, hideTimer = null, deadline = 0;

  function url() {
    const src = source();
    return `/api/live?id=${encodeURIComponent(src.id)}&channel=${src.channel}&fps=6&_r=${Date.now()}`;
  }
  function tick() {
    const left = Math.max(0, Math.round((deadline - Date.now()) / 1000));
    if (left <= 0) { api.stop(); return; }
    if (els.status) {
      els.status.textContent =
        `Đang phát • còn ${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`;
    }
  }

  const api = {
    running() { return timer != null; },
    start() {
      const src = source();
      if (!src) return;
      api.stop(); // never run two at once
      els.img.src = url();
      els.img.hidden = false;
      if (els.start) els.start.hidden = true;
      if (els.extend) els.extend.hidden = false;
      if (els.stop) els.stop.hidden = false;
      deadline = Date.now() + SESSION_MS;
      timer = setInterval(tick, 1000);
      tick();
    },
    extend() {
      if (!timer || !source()) return;
      deadline = Date.now() + SESSION_MS;
      els.img.src = url(); // reconnect for a fresh 5-min session
      tick();
    },
    stop() {
      clearInterval(timer); timer = null;
      clearTimeout(hideTimer); hideTimer = null;
      // Clearing src drops the connection, which ends the server-side stream.
      els.img.removeAttribute('src');
      els.img.hidden = true;
      if (els.start) els.start.hidden = false;
      if (els.extend) els.extend.hidden = true;
      if (els.stop) els.stop.hidden = true;
      if (els.status) els.status.textContent = '';
    },
  };

  if (els.start) els.start.addEventListener('click', () => api.start());
  if (els.extend) els.extend.addEventListener('click', () => api.extend());
  if (els.stop) els.stop.addEventListener('click', () => api.stop());
  // Auto-stop after 30s hidden — don't hold a stream nobody is watching.
  document.addEventListener('visibilitychange', () => {
    if (!timer) return;
    if (document.hidden) hideTimer = setTimeout(() => api.stop(), 30000);
    else { clearTimeout(hideTimer); hideTimer = null; }
  });

  return api;
}

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
