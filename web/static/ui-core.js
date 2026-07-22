/* Shared UI primitives. Loaded before feature scripts; no build step required. */

function escapeHtml(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, ch => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[ch]));
}

function cssEscape(s) { return String(s).replace(/[^a-zA-Z0-9_-]/g, '_'); }

function timeoutSec() {
  const input = document.getElementById('g-timeout');
  return parseInt(input && input.value, 10) || 30;
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
