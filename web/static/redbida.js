/* RedBida / OTA-MQTT settings console. Node-RED 2023 is survey-only. */

const redbidaState = {
  metas: [],
  values: new Map(),
  drafts: new Map(),
  results: new Map(),
  loaded: false,
  sourceWarning: '',
  busy: false,
};

function redbidaMeta(key) {
  return redbidaState.metas.find(m => m.key === key) || null;
}

function redbidaSetMessage(text, kind) {
  const el = document.getElementById('redbida-msg');
  if (!el) return;
  el.textContent = text || '';
  el.className = text ? `msg ${kind || ''}` : 'msg';
}

function redbidaValueText(value) {
  if (value === undefined) return '—';
  if (typeof value === 'string') {
    const match = value.match(/^data:(image\/(?:png|jpeg|webp));base64,(.*)$/s);
    if (match) {
      const bytes = Math.max(0, Math.floor(match[2].length * 3 / 4));
      return `[${match[1]} data URL · ${(bytes / 1024).toFixed(1)} KiB]`;
    }
    return value;
  }
  try { return JSON.stringify(value); } catch (e) { return String(value); }
}

function redbidaSetBusy(busy) {
  redbidaState.busy = busy;
  ['redbida-refresh', 'redbida-apply'].forEach(id => {
    const button = document.getElementById(id);
    if (button) button.disabled = busy;
  });
}

function redbidaValuesEqual(left, right) {
  try { return JSON.stringify(left) === JSON.stringify(right); } catch (e) { return left === right; }
}

function redbidaGroups() {
  return Array.from(new Set(redbidaState.metas.map(m => m.group))).sort((a, b) => a.localeCompare(b, 'vi'));
}

function redbidaRenderGroups() {
  const select = document.getElementById('redbida-group');
  if (!select) return;
  const selected = select.value;
  select.innerHTML = '<option value="">Tất cả nhóm</option>' + redbidaGroups().map(g => `<option value="${escapeHtml(g)}">${escapeHtml(g)}</option>`).join('');
  select.value = selected;
}

function redbidaFilteredMetas() {
  const q = (document.getElementById('redbida-search')?.value || '').trim().toLocaleLowerCase('vi');
  const group = document.getElementById('redbida-group')?.value || '';
  const dirtyOnly = document.getElementById('redbida-dirty-only')?.checked;
  return redbidaState.metas.filter(meta => {
    if (group && meta.group !== group) return false;
    if (q && !(meta.key + ' ' + meta.label + ' ' + meta.group).toLocaleLowerCase('vi').includes(q)) return false;
    if (dirtyOnly && !redbidaState.drafts.has(meta.key)) return false;
    return true;
  });
}

function redbidaEditor(meta, value) {
  if (!meta.editable) return `<code class="redbida-protected-value">${escapeHtml(redbidaValueText(value))}</code>`;
  const draft = redbidaState.drafts.has(meta.key) ? redbidaState.drafts.get(meta.key) : value;
  if (meta.valueType === 'boolean') {
    return `<select data-red-key="${escapeHtml(meta.key)}"><option value="true" ${draft === true ? 'selected' : ''}>true</option><option value="false" ${draft === false ? 'selected' : ''}>false</option></select>`;
  }
  if (meta.valueType === 'json') {
    let text = '';
    try { text = JSON.stringify(draft, null, 2); } catch (e) { text = String(draft ?? ''); }
    return `<textarea data-red-key="${escapeHtml(meta.key)}" rows="3">${escapeHtml(text)}</textarea>`;
  }
  const type = meta.valueType === 'number' ? 'number' : 'text';
  const file = meta.valueType === 'image' ? `<input class="redbida-file" type="file" accept="image/png,image/jpeg,image/webp" data-red-file="${escapeHtml(meta.key)}">` : '';
  const preview = meta.valueType === 'image' && typeof draft === 'string' && draft.startsWith('data:image/') ? `<img class="redbida-logo-preview" src="${escapeHtml(draft)}" alt="preview">` : '';
  return `<div class="redbida-editor"><input type="${type}" data-red-key="${escapeHtml(meta.key)}" value="${escapeHtml(draft == null ? '' : String(draft))}">${file}${preview}</div>`;
}

function redbidaRender() {
  const tbody = document.getElementById('redbida-tbody');
  if (!tbody) return;
  const metas = redbidaFilteredMetas();
  if (!metas.length) {
    tbody.innerHTML = '<tr><td colspan="6" class="empty-hint">Không có key khớp bộ lọc.</td></tr>';
    return;
  }
  tbody.innerHTML = metas.map(meta => {
    const item = redbidaState.values.get(meta.key);
    const value = item ? item.value : undefined;
    const dirty = redbidaState.drafts.has(meta.key);
    const result = redbidaState.results.get(meta.key);
    let status = item && item.exists ? 'Đã đọc' : 'Chưa có';
    if (result?.error) status = `Lỗi: ${result.error}`;
    else if (dirty) status = 'Đã sửa';
    else if (result?.applied) status = result.verified === false ? 'Đã ghi' : 'Đã xác minh';
    return `<tr data-red-row="${escapeHtml(meta.key)}" class="${dirty ? 'redbida-dirty' : ''}">
      <td data-label="Nhóm">${escapeHtml(meta.group)}</td>
      <td data-label="Key"><code>${escapeHtml(meta.key)}</code><br><span class="muted">${escapeHtml(meta.label)}</span></td>
      <td data-label="Risk"><span class="badge redbida-risk-${escapeHtml(meta.risk)}">${escapeHtml(meta.risk)}</span></td>
      <td data-label="Hiện tại"><code class="redbida-current">${escapeHtml(redbidaValueText(value))}</code></td>
      <td data-label="Giá trị mới">${redbidaEditor(meta, value)}</td>
      <td data-label="Trạng thái" class="redbida-row-status">${escapeHtml(status)}</td>
    </tr>`;
  }).join('');
  tbody.querySelectorAll('[data-red-key]').forEach(input => {
    input.addEventListener('input', () => redbidaCaptureDraft(input));
    input.addEventListener('change', () => redbidaCaptureDraft(input));
  });
  tbody.querySelectorAll('[data-red-file]').forEach(input => {
    input.addEventListener('change', () => redbidaReadImage(input));
  });
}

function redbidaCaptureDraft(input) {
  const key = input.dataset.redKey;
  const meta = redbidaMeta(key);
  if (!meta) return;
  let value = input.value;
  if (meta.valueType === 'boolean') value = value === 'true';
  if (meta.valueType === 'number') value = Number(value);
  if (meta.valueType === 'json') {
    try { value = JSON.parse(value); } catch (e) {
      redbidaState.drafts.delete(key);
      redbidaState.results.set(key, { key, error: 'JSON chưa hợp lệ' });
      redbidaSetMessage(`${key}: JSON chưa hợp lệ`, 'err');
      const row = Array.from(document.querySelectorAll('[data-red-row]')).find(el => el.dataset.redRow === key);
      if (row) {
        row.classList.remove('redbida-dirty');
        const status = row.querySelector('.redbida-row-status');
        if (status) status.textContent = 'Lỗi: JSON chưa hợp lệ';
      }
      return;
    }
  }
  const current = redbidaState.values.get(key)?.value;
  if (redbidaValuesEqual(value, current)) redbidaState.drafts.delete(key);
  else redbidaState.drafts.set(key, value);
  redbidaState.results.delete(key);
  const row = Array.from(document.querySelectorAll('[data-red-row]')).find(el => el.dataset.redRow === key);
  if (row) {
    row.classList.toggle('redbida-dirty', redbidaState.drafts.has(key));
    const status = row.querySelector('.redbida-row-status');
    if (status) status.textContent = redbidaState.drafts.has(key) ? 'Đã sửa' : 'Đã đọc';
  }
}

function redbidaReadImage(input) {
  const file = input.files && input.files[0];
  if (!file) return;
  if (file.size > 512 * 1024) {
    redbidaSetMessage('Logo quá lớn (tối đa 512 KiB).', 'err');
    input.value = '';
    return;
  }
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
    redbidaSetMessage('Logo phải là PNG, JPEG hoặc WebP.', 'err');
    input.value = '';
    return;
  }
  const reader = new FileReader();
  reader.onload = () => {
    redbidaState.drafts.set(input.dataset.redFile, reader.result);
    redbidaState.results.delete(input.dataset.redFile);
    redbidaRender();
  };
  reader.readAsDataURL(file);
}

async function redbidaLoadCatalog() {
  const payload = await api('/api/redbida/catalog');
  redbidaState.metas = Array.isArray(payload.keys) ? payload.keys : [];
  redbidaState.sourceWarning = payload.sourceAvailable === false ? 'Không đọc được thư mục key; đang dùng catalog dự phòng.' : '';
  redbidaRenderGroups();
  document.getElementById('redbida-key-count').textContent = String(redbidaState.metas.length);
}

async function redbidaRefresh(confirmDirty = false) {
  if (redbidaState.busy) return;
  if (confirmDirty && redbidaState.drafts.size && !window.confirm('Đọc lại sẽ bỏ các thay đổi chưa submit. Tiếp tục?')) return;
  redbidaSetBusy(true);
  try {
    redbidaSetMessage('Đang đọc key từ ota-mqtt…');
    const payload = await api('/api/redbida/refresh', { method: 'POST', body: JSON.stringify({ keys: redbidaState.metas.map(m => m.key) }) });
    redbidaState.values.clear();
    const refreshedMeta = new Map();
    (payload.values || []).forEach(item => {
      redbidaState.values.set(item.key, item);
      if (item.meta) refreshedMeta.set(item.key, item.meta);
    });
    redbidaState.metas = redbidaState.metas.map(meta => refreshedMeta.get(meta.key) || meta);
    redbidaState.drafts.clear();
    redbidaState.results.clear();
    redbidaState.loaded = true;
    const warning = redbidaState.sourceWarning ? ` ${redbidaState.sourceWarning}` : '';
    redbidaSetMessage(`Đã đọc ${redbidaState.values.size} key lúc ${payload.refreshedAt || ''}.${warning}`, redbidaState.sourceWarning ? '' : 'ok');
    redbidaRender();
  } finally {
    redbidaSetBusy(false);
  }
}

async function redbidaApply() {
  if (redbidaState.busy) return;
  if (!redbidaState.drafts.size) {
    redbidaSetMessage('Chưa có thay đổi để submit.', '');
    return;
  }
  const needsConfirm = Array.from(redbidaState.drafts.keys()).some(key => redbidaMeta(key)?.risk === 'confirm-required');
  if (needsConfirm && !window.confirm('Batch có key ảnh hưởng restart/reboot. Xác nhận submit?')) return;
  const changes = Object.fromEntries(redbidaState.drafts.entries());
  redbidaSetBusy(true);
  try {
    redbidaSetMessage('Đang submit tới ota-mqtt…');
    const payload = await api('/api/redbida/apply', { method: 'POST', body: JSON.stringify({ changes, confirmed: needsConfirm }) });
    const results = payload.results || [];
    results.forEach(item => {
      redbidaState.results.set(item.key, item);
      const previous = redbidaState.values.get(item.key) || {};
      if (item.readBack === true && Object.prototype.hasOwnProperty.call(item, 'newValue')) {
        redbidaState.values.set(item.key, { ...previous, key: item.key, value: item.newValue, exists: true, meta: item.meta || previous.meta });
      }
      if (item.applied) redbidaState.drafts.delete(item.key);
    });
    const failed = results.filter(item => item.error || !item.applied);
    const verified = results.filter(item => item.applied).length;
    redbidaSetMessage(failed.length ? `${verified} key đã xác minh, ${failed.length} key lỗi.` : `Đã ghi và đọc lại xác minh ${verified} key.`, failed.length ? 'err' : 'ok');
    redbidaRender();
  } finally {
    redbidaSetBusy(false);
  }
}

async function redbidaTimeStatus() {
  const payload = await api('/api/redbida/time-status');
  document.getElementById('redbida-time-status').textContent = payload.hostTime || '–';
  document.getElementById('redbida-ntp-status').textContent = payload.ntpSynchronized ? 'NTP tốt' : 'Chưa tin cậy';
  document.getElementById('redbida-node-status').textContent = payload.nodeRedReadOnly ? '2023 chỉ khảo sát' : '–';
}

async function redbidaOnShow() {
  if (!redbidaState.loaded) {
    try { await redbidaLoadCatalog(); await redbidaRefresh(); await redbidaTimeStatus(); }
    catch (err) { redbidaSetMessage('Lỗi RedBida: ' + err.message, 'err'); }
  }
}

window.redbidaOnShow = redbidaOnShow;

document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('redbida-refresh')?.addEventListener('click', () => redbidaRefresh(true).catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-apply')?.addEventListener('click', () => redbidaApply().catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-time-refresh')?.addEventListener('click', () => redbidaTimeStatus().catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-search')?.addEventListener('input', redbidaRender);
  document.getElementById('redbida-group')?.addEventListener('change', redbidaRender);
  document.getElementById('redbida-dirty-only')?.addEventListener('change', redbidaRender);
});
