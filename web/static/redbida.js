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

function removeVietnameseTones(str) {
  if (!str || typeof str !== 'string') return '';
  return str
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[đĐ]/g, m => (m === 'đ' ? 'd' : 'D'));
}

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
  ['redbida-refresh', 'redbida-apply', 'redbida-preset-gen-btn'].forEach(id => {
    const button = document.getElementById(id);
    if (button) button.disabled = busy;
  });
}

function redbidaValuesEqual(left, right) {
  try { return JSON.stringify(left) === JSON.stringify(right); } catch (e) { return left === right; }
}

function redbidaUpdateMetrics() {
  const draftCountEl = document.getElementById('redbida-draft-count');
  if (draftCountEl) {
    draftCountEl.textContent = String(redbidaState.drafts.size);
  }
  const keyCountEl = document.getElementById('redbida-key-count');
  if (keyCountEl) {
    keyCountEl.textContent = String(redbidaState.metas.length || 0);
  }
  const brokerStatusEl = document.getElementById('redbida-broker-status');
  if (brokerStatusEl) {
    brokerStatusEl.textContent = '127.0.0.1:12369';
  }
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
  if (meta.key === 'ui_tabs_links') {
    const textVal = draft == null ? '' : String(draft);
    return `<div class="redbida-editor"><textarea data-red-key="${escapeHtml(meta.key)}" rows="4" style="font-family:var(--font-mono, monospace);font-size:0.8rem;">${escapeHtml(textVal)}</textarea></div>`;
  }
  const type = meta.valueType === 'number' ? 'number' : 'text';
  const file = meta.valueType === 'image' ? `<input class="redbida-file" type="file" accept="image/png,image/jpeg,image/webp" data-red-file="${escapeHtml(meta.key)}">` : '';
  let preview = '';
  if (meta.valueType === 'image' && typeof draft === 'string' && draft.length > 0) {
    if (draft.startsWith('data:image/') || draft.startsWith('http://') || draft.startsWith('https://') || draft.startsWith('/')) {
      preview = `<div class="redbida-checkerboard" style="display:inline-flex;padding:3px;border-radius:4px;margin-top:4px;"><img class="redbida-logo-preview" src="${escapeHtml(draft)}" alt="preview"></div>`;
    }
  }

  let bgPreview = '';
  if (meta.key === 'ui_bg') {
    const bgVal = draft == null ? '' : String(draft);
    bgPreview = `<div class="redbida-gradient-preview-wrap" style="margin-top:6px;"><div class="redbida-gradient-preview redbida-row-bg-preview" data-preview-key="ui_bg" style="background: ${escapeHtml(bgVal || 'transparent')}; height: 32px; min-height: 32px; padding: 0.25rem 0.5rem; border-radius: 4px; display: flex; align-items: center; justify-content: space-between;"><span class="redbida-preview-title" style="font-size:0.75rem; font-weight:600; text-shadow:0 1px 2px rgba(0,0,0,0.8);">Gradient Preview</span><span style="font-size:0.68rem; opacity:0.8; font-family:var(--font-mono, monospace);">ui_bg</span></div></div>`;
  }

  return `<div class="redbida-editor"><input type="${type}" data-red-key="${escapeHtml(meta.key)}" value="${escapeHtml(draft == null ? '' : String(draft))}">${file}${preview}${bgPreview}</div>`;
}

function redbidaRender() {
  const tbody = document.getElementById('redbida-tbody');
  if (!tbody) return;
  const metas = redbidaFilteredMetas();
  if (!metas.length) {
    tbody.innerHTML = '<tr><td colspan="6" class="empty-hint">Không có key khớp bộ lọc.</td></tr>';
    redbidaUpdateMetrics();
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
  redbidaUpdateMetrics();
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
      redbidaUpdateMetrics();
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
  if (key === 'ui_bg') {
    const rowPreview = document.querySelector('[data-preview-key="ui_bg"]');
    if (rowPreview) rowPreview.style.background = String(value || 'transparent');
  }
  redbidaUpdateMetrics();
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
    redbidaUpdateMetrics();
  };
  reader.readAsDataURL(file);
}

function redbidaUpdatePresetPreview(title, bg) {
  const preview = document.getElementById('redbida-preset-bg-preview');
  if (!preview) return;
  if (bg) preview.style.background = bg;
  const titleEl = preview.querySelector('.redbida-preview-title');
  if (titleEl && title) titleEl.textContent = title;
}

function redbidaRenderPresetDiff(changes) {
  const diffEl = document.getElementById('redbida-preset-diff');
  if (!diffEl) return;

  const entries = Object.entries(changes);
  const rowsHtml = entries.map(([key, val]) => {
    let displayVal = redbidaValueText(val);
    if (typeof val === 'string' && val.length > 80) {
      displayVal = val.slice(0, 77) + '…';
    }
    const meta = redbidaMeta(key);
    const riskBadge = meta ? `<span class="badge redbida-risk-${escapeHtml(meta.risk)}">${escapeHtml(meta.risk)}</span>` : '';
    return `<tr>
      <td><code>${escapeHtml(key)}</code></td>
      <td>${riskBadge}</td>
      <td><code class="redbida-diff-val">${escapeHtml(displayVal)}</code></td>
    </tr>`;
  }).join('');

  diffEl.innerHTML = `
    <div class="card redbida-diff-card" style="margin-top:1rem;">
      <div class="card-head-row">
        <div>
          <div class="card-title" style="font-size:0.95rem;font-weight:600;">📋 Bản Nháp Preset Sẵn Sàng (${entries.length} tham số)</div>
          <div class="card-sub" style="font-size:0.8rem;">Các giá trị dưới đây đã được nạp vào bản nháp (drafts) và sẵn sàng submit lên MQTT /private/i_sets.</div>
        </div>
        <div style="display:flex;gap:0.5rem;">
          <button class="btn btn-sm" id="redbida-preset-submit-now" type="button">🚀 Áp Dụng Ngay</button>
          <button class="btn btn-secondary btn-sm" id="redbida-preset-diff-close" type="button">Đóng</button>
        </div>
      </div>
      <div class="table-wrap" style="max-height:260px;overflow-y:auto;margin-top:0.5rem;">
        <table class="redbida-table" style="font-size:0.82rem;">
          <thead><tr><th>Key</th><th>Risk</th><th>Giá trị chuẩn mới</th></tr></thead>
          <tbody>${rowsHtml}</tbody>
        </table>
      </div>
    </div>
  `;
  diffEl.style.display = 'block';

  document.getElementById('redbida-preset-submit-now')?.addEventListener('click', () => {
    redbidaApply().catch(err => redbidaSetMessage(err.message, 'err'));
  });
  document.getElementById('redbida-preset-diff-close')?.addEventListener('click', () => {
    diffEl.style.display = 'none';
  });
}

function redbidaGeneratePreset() {
  const titleInput = document.getElementById('redbida-preset-title');
  const countInput = document.getElementById('redbida-preset-count');
  const bgInput = document.getElementById('redbida-preset-bg');

  const title = (titleInput?.value || 'CX King Luxury').trim();
  const count = parseInt(countInput?.value, 10) || 8;
  const rawBg = (bgInput?.value || 'radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )').trim();
  const bg = rawBg.replace(/;\s*$/, '').trim();

  // Clean & sanitize hashtags using removeVietnameseTones + strip non-alphanumerics
  const cleanTitle = removeVietnameseTones(title).replace(/[^a-zA-Z0-9]/g, '');
  const customHashtags = cleanTitle
    ? `#${cleanTitle} #BILLIARDSlive #INUTlive #highlightsports`
    : '#BILLIARDSlive #INUTlive #highlightsports';

  // Generate 20-tab INI ui_tabs_links: [C01] to [C20] with vid_play_label = <ui_title>
  const sections = [];
  for (let i = 1; i <= 20; i++) {
    const pad = String(i).padStart(2, '0');
    sections.push(
      `[C${pad}]\n` +
      `stream_label=Video Trực tiếp\n` +
      `vid_list_label=Danh sách highlight\n` +
      `vid_play_label=${title}\n` +
      `list_refresh_label=Cập nhật highlight`
    );
  }
  const iniTabs = sections.join('\n\n');

  // Populate standard parameters
  const presetChanges = {
    ui_title: title,
    company_name: title,
    ui_bg: bg,
    custom_hashtags: customHashtags,
    ui_tabs_links: iniTabs,
    camera_count: count,
    toolbar_show_count: count,
    video_config: 'range=72',
    hls_using_go2rtc: true,
    hls_using_go2rtc_livestream: true,
    hls_using_go2rtc_tiktok: true,
    ui_scoreboard: true,
    logo_header: 'https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png',
    logo_header_text: 'Billiard Live - Tải clip bàn bida và livestream',
    button_generate_go2rtc_stream: true,
  };

  // Set values into redbidaState.drafts
  Object.entries(presetChanges).forEach(([key, val]) => {
    redbidaState.drafts.set(key, val);
    redbidaState.results.delete(key);
  });

  // Re-render table and update metrics
  redbidaRender();
  redbidaUpdateMetrics();

  // Render visual diff preview card in #redbida-preset-diff
  redbidaRenderPresetDiff(presetChanges);

  // Update live preview in preset card
  redbidaUpdatePresetPreview(title, bg);

  redbidaSetMessage(`Đã sinh preset 1-click thành công cho "${title}" (${Object.keys(presetChanges).length} tham số). Bấm "Submit thay đổi" để áp dụng lên OTA-MQTT.`, 'ok');
}

function redbidaResetPresetForm() {
  const titleInput = document.getElementById('redbida-preset-title');
  const countInput = document.getElementById('redbida-preset-count');
  const groupKeyInput = document.getElementById('redbida-preset-groupkey');
  const bgInput = document.getElementById('redbida-preset-bg');
  const ggcodeInput = document.getElementById('redbida-preset-ggcode');

  if (titleInput) titleInput.value = 'CX King Luxury';
  if (countInput) countInput.value = '8';
  if (groupKeyInput) groupKeyInput.value = 'CX_KING_LUXURY';
  if (ggcodeInput) ggcodeInput.value = 'G-SFSDZPR95Z';
  const defaultBg = 'radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )';
  if (bgInput) bgInput.value = defaultBg;

  // Reset active swatch
  document.querySelectorAll('#redbida-preset-swatches .redbida-swatch').forEach((swatch, idx) => {
    swatch.classList.toggle('active', idx === 0);
  });

  // Update preview
  redbidaUpdatePresetPreview('CX King Luxury', defaultBg);

  // Hide diff
  const diffEl = document.getElementById('redbida-preset-diff');
  if (diffEl) diffEl.style.display = 'none';
}

function redbidaMatchGroup(target) {
  if (!target) return '';
  const select = document.getElementById('redbida-group');
  if (!select) return '';
  const options = Array.from(select.options).map(o => o.value);
  if (options.includes(target)) return target;

  const norm = s => s.toLowerCase().replace(/&/g, '/').replace(/\s+/g, '');
  const targetNorm = norm(target);
  for (const opt of options) {
    if (opt && norm(opt) === targetNorm) return opt;
  }
  if (targetNorm.includes('stream') || targetNorm.includes('video')) {
    const found = options.find(o => norm(o).includes('stream') || norm(o).includes('video') || norm(o) === 'livestream');
    if (found) return found;
  }
  if (targetNorm.includes('sched') || targetNorm.includes('maint')) {
    const found = options.find(o => norm(o).includes('sched') || norm(o).includes('maint'));
    if (found) return found;
  }
  if (targetNorm.includes('brand') || targetNorm.includes('logo')) {
    const found = options.find(o => norm(o).includes('brand') || norm(o).includes('logo'));
    if (found) return found;
  }
  if (targetNorm.includes('sec') || targetNorm.includes('cred') || targetNorm.includes('shinobi')) {
    const found = options.find(o => norm(o).includes('sec') || norm(o).includes('cred'));
    if (found) return found;
  }
  return '';
}

function redbidaInitSwatches() {
  const swatchesWrap = document.getElementById('redbida-preset-swatches');
  if (!swatchesWrap) return;
  swatchesWrap.addEventListener('click', (e) => {
    const swatch = e.target.closest('.redbida-swatch');
    if (!swatch) return;
    const bg = swatch.dataset.bg;
    if (!bg) return;

    const bgInput = document.getElementById('redbida-preset-bg');
    if (bgInput) {
      bgInput.value = bg;
    }

    swatchesWrap.querySelectorAll('.redbida-swatch').forEach(s => s.classList.remove('active'));
    swatch.classList.add('active');

    const title = document.getElementById('redbida-preset-title')?.value || 'CX King Luxury';
    redbidaUpdatePresetPreview(title, bg);
  });
}

function redbidaInitPresetInputs() {
  const bgInput = document.getElementById('redbida-preset-bg');
  const titleInput = document.getElementById('redbida-preset-title');

  bgInput?.addEventListener('input', () => {
    const bg = bgInput.value.trim();
    const title = titleInput?.value || 'CX King Luxury';
    redbidaUpdatePresetPreview(title, bg);

    document.querySelectorAll('#redbida-preset-swatches .redbida-swatch').forEach(swatch => {
      swatch.classList.toggle('active', swatch.dataset.bg === bg);
    });
  });

  titleInput?.addEventListener('input', () => {
    const title = titleInput.value.trim() || 'Billiard Club';
    const bg = bgInput?.value || '';
    redbidaUpdatePresetPreview(title, bg);
  });
}

function redbidaInitPillarButtons() {
  document.querySelectorAll('.redbida-pillar-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const targetGroup = btn.dataset.filterGroup || btn.dataset.pillar;
      const matched = redbidaMatchGroup(targetGroup);
      const select = document.getElementById('redbida-group');
      if (select && matched) {
        select.value = matched;
        redbidaRender();
        document.getElementById('redbida-table')?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    });
  });
}

function redbidaInitToggles() {
  document.getElementById('redbida-toggle-preset')?.addEventListener('click', () => {
    const panel = document.getElementById('redbida-preset-panel');
    if (!panel) return;
    const isHidden = panel.style.display === 'none' || getComputedStyle(panel).display === 'none';
    panel.style.display = isHidden ? 'block' : 'none';
  });

  document.getElementById('redbida-toggle-hub')?.addEventListener('click', () => {
    const hub = document.getElementById('redbida-knowledge-hub');
    if (!hub) return;
    const isHidden = hub.style.display === 'none' || getComputedStyle(hub).display === 'none';
    hub.style.display = isHidden ? 'block' : 'none';
  });
}

function redbidaTriggerGo2RTCStream() {
  redbidaState.drafts.set('button_generate_go2rtc_stream', true);
  redbidaRender();
  redbidaUpdateMetrics();
  redbidaSetMessage('Đã thêm button_generate_go2rtc_stream=true vào bản nháp. Bấm "Submit thay đổi" để áp dụng lên OTA-MQTT.', 'ok');
}

async function redbidaLoadCatalog() {
  const payload = await api('/api/redbida/catalog');
  redbidaState.metas = Array.isArray(payload.keys) ? payload.keys : [];
  redbidaState.sourceWarning = payload.sourceAvailable === false ? 'Không đọc được thư mục key; đang dùng catalog dự phòng.' : '';
  redbidaRenderGroups();
  redbidaUpdateMetrics();
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
    redbidaUpdateMetrics();
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
    redbidaUpdateMetrics();
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
    try {
      await redbidaLoadCatalog();
      await redbidaRefresh();
      await redbidaTimeStatus();
    } catch (err) {
      redbidaSetMessage('Lỗi RedBida: ' + err.message, 'err');
    }
  }
}

window.redbidaOnShow = redbidaOnShow;
window.redbidaGeneratePreset = redbidaGeneratePreset;
window.redbidaResetPresetForm = redbidaResetPresetForm;
window.redbidaTriggerGo2RTCStream = redbidaTriggerGo2RTCStream;
window.redbidaState = redbidaState;

document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('redbida-refresh')?.addEventListener('click', () => redbidaRefresh(true).catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-apply')?.addEventListener('click', () => redbidaApply().catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-time-refresh')?.addEventListener('click', () => redbidaTimeStatus().catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-preset-gen-btn')?.addEventListener('click', redbidaGeneratePreset);
  document.getElementById('redbida-preset-reset-btn')?.addEventListener('click', redbidaResetPresetForm);
  document.getElementById('redbida-search')?.addEventListener('input', redbidaRender);
  document.getElementById('redbida-group')?.addEventListener('change', redbidaRender);
  document.getElementById('redbida-dirty-only')?.addEventListener('change', redbidaRender);

  redbidaInitSwatches();
  redbidaInitPresetInputs();
  redbidaInitPillarButtons();
  redbidaInitToggles();
});
