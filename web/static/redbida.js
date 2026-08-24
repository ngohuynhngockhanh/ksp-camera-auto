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

const REDBIDA_GRADIENT_PALETTE = [
  {
    id: 'royal-deep-blue',
    name: 'Royal Deep Blue Glow',
    desc: 'Xanh Hoàng Gia Sâu Thẳm',
    css: 'linear-gradient(135deg, #0b192c 0%, #1e3e62 50%, #000000 100%)',
    swatch: 'linear-gradient(135deg, #0b192c 0%, #1e3e62 50%, #000000 100%)',
  },
  {
    id: 'midnight-emerald',
    name: 'Midnight Emerald Cyber',
    desc: 'Ngọc Lục Bảo Cyber',
    css: 'linear-gradient(135deg, #051f20 0%, #0b2b26 40%, #163832 70%, #001414 100%)',
    swatch: 'linear-gradient(135deg, #051f20 0%, #0b2b26 40%, #163832 70%, #001414 100%)',
  },
  {
    id: 'cyberpunk-neon',
    name: 'Cyberpunk Neon',
    desc: 'Tím Dạ Quang Cyberpunk',
    css: 'linear-gradient(135deg, #1f1035 0%, #341247 45%, #0d0221 100%)',
    swatch: 'linear-gradient(135deg, #1f1035 0%, #341247 45%, #0d0221 100%)',
  },
  {
    id: 'golden-velvet',
    name: 'Golden Velvet',
    desc: 'Nhung Vàng Sang Trọng',
    css: 'linear-gradient(135deg, #2b1e05 0%, #4a3508 50%, #171003 100%)',
    swatch: 'linear-gradient(135deg, #2b1e05 0%, #4a3508 50%, #171003 100%)',
  },
  {
    id: 'obsidian-carbon',
    name: 'Obsidian Carbon',
    desc: 'Than Chì Carbon Lịch Lãm',
    css: 'linear-gradient(135deg, #121212 0%, #242424 50%, #0a0a0a 100%)',
    swatch: 'linear-gradient(135deg, #121212 0%, #242424 50%, #0a0a0a 100%)',
  },
  {
    id: 'crimson-elegance',
    name: 'Crimson Elegance',
    desc: 'Đỏ Rượu Vang Quý Phái',
    css: 'linear-gradient(135deg, #2c0b0e 0%, #52151c 50%, #140507 100%)',
    swatch: 'linear-gradient(135deg, #2c0b0e 0%, #52151c 50%, #140507 100%)',
  },
  {
    id: 'sapphire-blue',
    name: 'Sapphire Blue',
    desc: 'Lam Ngọc Tinh Tế',
    css: 'linear-gradient(135deg, #0a1128 0%, #1c2541 50%, #000814 100%)',
    swatch: 'linear-gradient(135deg, #0a1128 0%, #1c2541 50%, #000814 100%)',
  },
  {
    id: 'ruby-luxury',
    name: 'Ruby Luxury',
    desc: 'Hồng Ngọc Ruby Đẳng Cấp',
    css: 'linear-gradient(135deg, #3d0c11 0%, #68131d 50%, #200407 100%)',
    swatch: 'linear-gradient(135deg, #3d0c11 0%, #68131d 50%, #200407 100%)',
  },
];

const redbida20TabsState = {
  selectedTab: 'C01',
  tabs: [],
  viewMode: 'visual',
};

function removeVietnameseTones(str) {
  if (!str || typeof str !== 'string') return '';
  return str
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[đĐ]/g, m => (m === 'đ' ? 'd' : 'D'));
}

function sanitizeCleanTitle(title) {
  return removeVietnameseTones(title).replace(/[^a-zA-Z0-9]/g, '');
}

function generateSmartHashtags(title) {
  const clean = sanitizeCleanTitle(title);
  if (!clean) return '#BILLIARDSlive #INUTlive #highlightsports';
  return `#${clean} #BILLIARDSlive #INUTlive #highlightsports`;
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
  ['redbida-refresh', 'redbida-apply', 'redbida-preset-gen-btn', 'redbida-autofix-all-btn', 'redbida-audit-btn'].forEach(id => {
    const button = document.getElementById(id);
    if (button) button.disabled = busy;
  });
}

function redbidaValuesEqual(left, right) {
  try { return JSON.stringify(left) === JSON.stringify(right); } catch (e) { return left === right; }
}

function getEffectiveValue(key) {
  if (redbidaState.drafts.has(key)) {
    return redbidaState.drafts.get(key);
  }
  const item = redbidaState.values.get(key);
  return item ? item.value : undefined;
}

function parse20TabsIni(iniString, defaultTitle) {
  const title = defaultTitle || 'CX King Luxury';
  const tabMap = new Map();
  const lines = (iniString || '').split('\n');
  let currentSection = null;

  lines.forEach(line => {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith(';') || trimmed.startsWith('#')) return;
    const secMatch = trimmed.match(/^\[([a-zA-Z0-9_]+)\]$/);
    if (secMatch) {
      currentSection = secMatch[1].toUpperCase();
      if (!tabMap.has(currentSection)) {
        tabMap.set(currentSection, {
          id: currentSection,
          stream_label: 'Video Trực tiếp',
          vid_list_label: 'Danh sách highlight',
          vid_play_label: title,
          list_refresh_label: 'Cập nhật highlight',
        });
      }
      return;
    }
    if (currentSection && trimmed.includes('=')) {
      const eqIdx = trimmed.indexOf('=');
      const k = trimmed.slice(0, eqIdx).trim();
      const v = trimmed.slice(eqIdx + 1).trim();
      const tabObj = tabMap.get(currentSection);
      if (tabObj && Object.prototype.hasOwnProperty.call(tabObj, k)) {
        tabObj[k] = v;
      }
    }
  });

  const tabs = [];
  for (let i = 1; i <= 20; i++) {
    const id = `C${String(i).padStart(2, '0')}`;
    if (tabMap.has(id)) {
      tabs.push(tabMap.get(id));
    } else {
      tabs.push({
        id,
        stream_label: 'Video Trực tiếp',
        vid_list_label: 'Danh sách highlight',
        vid_play_label: title,
        list_refresh_label: 'Cập nhật highlight',
      });
    }
  }
  return tabs;
}

function serialize20TabsIni(tabs) {
  return tabs.map(t =>
    `[${t.id}]\n` +
    `stream_label=${t.stream_label || 'Video Trực tiếp'}\n` +
    `vid_list_label=${t.vid_list_label || 'Danh sách highlight'}\n` +
    `vid_play_label=${t.vid_play_label || 'CX King Luxury'}\n` +
    `list_refresh_label=${t.list_refresh_label || 'Cập nhật highlight'}`
  ).join('\n\n');
}

// 15 Golden Standard Rules Specification
const GOLDEN_STANDARD_RULES = [
  {
    key: 'ui_title',
    label: 'Tên Quán Bida (ui_title)',
    desc: 'Tên quán bida không được rỗng',
    group: 'UI / Display',
    check: (val) => typeof val === 'string' && val.trim().length > 0,
    fix: (cur) => {
      const presetTitle = document.getElementById('redbida-preset-title')?.value?.trim();
      return presetTitle || (typeof cur === 'string' && cur.trim()) || 'CX King Luxury';
    },
  },
  {
    key: 'company_name',
    label: 'Tên Công Ty (company_name)',
    desc: 'Phải khớp chính xác với ui_title',
    group: 'Branding / Logo',
    check: (val) => {
      const title = getEffectiveValue('ui_title');
      if (typeof title === 'string' && title.trim().length > 0) {
        return val === title;
      }
      return typeof val === 'string' && val.trim().length > 0;
    },
    fix: () => {
      return getEffectiveValue('ui_title') || 'CX King Luxury';
    },
  },
  {
    key: 'ui_bg',
    label: 'Theme Gradient (ui_bg)',
    desc: 'CSS Gradient hợp lệ, KHÔNG có dấu ";" ở cuối',
    group: 'UI / Display',
    check: (val) => typeof val === 'string' && val.includes('gradient') && !/;\s*$/.test(val.trim()),
    fix: (cur) => {
      if (typeof cur === 'string' && cur.includes('gradient')) {
        return cur.replace(/[;\s]+$/, '').trim();
      }
      return REDBIDA_GRADIENT_PALETTE[0].css;
    },
  },
  {
    key: 'custom_hashtags',
    label: 'Hashtag Chuẩn (custom_hashtags)',
    desc: 'Không dấu tiếng Việt, chứa #BILLIARDSlive #INUTlive #highlightsports',
    group: 'Branding / Logo',
    check: (val) => {
      if (typeof val !== 'string' || !val.trim()) return false;
      const hasVN = /[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]/i.test(val);
      return !hasVN && val.includes('#BILLIARDSlive') && val.includes('#INUTlive') && val.includes('#highlightsports');
    },
    fix: () => {
      const title = getEffectiveValue('ui_title') || 'CX King Luxury';
      return generateSmartHashtags(title);
    },
  },
  {
    key: 'ui_tabs_links',
    label: '20 Tab INI (ui_tabs_links)',
    desc: 'Đầy đủ 20 section [C01]..[C20] và vid_play_label khớp tên quán',
    group: 'UI / Display',
    check: (val) => {
      if (typeof val !== 'string') return false;
      return val.includes('[C01]') && val.includes('[C20]') && val.includes('vid_play_label=');
    },
    fix: (cur) => {
      const title = getEffectiveValue('ui_title') || 'CX King Luxury';
      const tabs = parse20TabsIni(cur, title);
      tabs.forEach(t => { t.vid_play_label = title; });
      return serialize20TabsIni(tabs);
    },
  },
  {
    key: 'camera_count',
    label: 'Số Camera Bida (camera_count)',
    desc: 'Số nguyên dương từ 1 đến 20',
    group: 'Livestream',
    check: (val) => typeof val === 'number' && Number.isInteger(val) && val >= 1 && val <= 20,
    fix: (cur) => (typeof cur === 'number' && cur >= 1 && cur <= 20 ? cur : 8),
  },
  {
    key: 'toolbar_show_count',
    label: 'Số Camera Toolbar (toolbar_show_count)',
    desc: 'Phải bằng chính xác camera_count',
    group: 'Livestream',
    check: (val) => {
      const cc = getEffectiveValue('camera_count');
      return typeof val === 'number' && (cc == null || val === Number(cc));
    },
    fix: () => {
      const cc = getEffectiveValue('camera_count');
      return Number(cc) || 8;
    },
  },
  {
    key: 'video_config',
    label: 'Cấu Hình Video (video_config)',
    desc: 'Giới hạn tra cứu highlight 72h ("range=72")',
    group: 'Livestream',
    check: (val) => val === 'range=72',
    fix: () => 'range=72',
  },
  {
    key: 'hls_using_go2rtc',
    label: 'Go2RTC Core (hls_using_go2rtc)',
    desc: 'Bật HLS Go2RTC độ trễ thấp (<1s)',
    group: 'Livestream',
    check: (val) => val === true,
    fix: () => true,
  },
  {
    key: 'hls_using_go2rtc_livestream',
    label: 'Go2RTC Live (hls_using_go2rtc_livestream)',
    desc: 'Bật Go2RTC Livestream Passthrough',
    group: 'Livestream',
    check: (val) => val === true,
    fix: () => true,
  },
  {
    key: 'hls_using_go2rtc_tiktok',
    label: 'Go2RTC TikTok (hls_using_go2rtc_tiktok)',
    desc: 'Bật Go2RTC luồng dọc TikTok',
    group: 'Livestream',
    check: (val) => val === true,
    fix: () => true,
  },
  {
    key: 'ui_scoreboard',
    label: 'Bảng Điểm Bida (ui_scoreboard)',
    desc: 'Kích hoạt widget bảng điểm trận đấu',
    group: 'UI / Display',
    check: (val) => val === true,
    fix: () => true,
  },
  {
    key: 'logo_header',
    label: 'Logo Header (logo_header)',
    desc: 'URL hoặc data URL hình ảnh hợp lệ',
    group: 'Branding / Logo',
    check: (val) => typeof val === 'string' && (val.startsWith('http://') || val.startsWith('https://') || val.startsWith('data:image/') || val.startsWith('/')),
    fix: () => 'https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png',
  },
  {
    key: 'logo_header_text',
    label: 'Slogan Quán (logo_header_text)',
    desc: 'Slogan chuẩn "Billiard Live - Tải clip bàn bida và livestream"',
    group: 'Branding / Logo',
    check: (val) => typeof val === 'string' && val.trim().length > 0,
    fix: () => 'Billiard Live - Tải clip bàn bida và livestream',
  },
  {
    key: 'button_generate_go2rtc_stream',
    label: 'Trigger Go2RTC (button_generate_go2rtc_stream)',
    desc: 'Cờ kích hoạt sinh luồng Go2RTC (true)',
    group: 'Livestream',
    check: (val) => val === true,
    fix: () => true,
  },
];

function redbidaAuditGoldenStandard() {
  let passedCount = 0;
  const auditResults = [];

  GOLDEN_STANDARD_RULES.forEach(rule => {
    const val = getEffectiveValue(rule.key);
    const passed = rule.check(val);
    if (passed) passedCount++;
    auditResults.push({
      rule,
      value: val,
      passed,
    });
  });

  const total = GOLDEN_STANDARD_RULES.length;
  const score = Math.round((passedCount / total) * 100);

  // Update Score Metric Card
  const scoreMetric = document.getElementById('redbida-standard-score');
  if (scoreMetric) scoreMetric.textContent = `${score}%`;
  const subMetric = document.getElementById('redbida-standard-sub');
  if (subMetric) subMetric.textContent = `${passedCount}/${total} key đạt chuẩn`;

  // Update Inspector Panel elements
  const scoreText = document.getElementById('redbida-inspector-score-text');
  if (scoreText) scoreText.textContent = `${passedCount} / ${total} Key Đạt Chuẩn (${score}%)`;

  const progressBar = document.getElementById('redbida-inspector-progress-bar');
  const badge = document.getElementById('redbida-inspector-badge');

  if (progressBar) {
    progressBar.style.width = `${score}%`;
    if (score === 100) {
      progressBar.style.background = 'linear-gradient(90deg, #10b981, #059669)';
    } else if (score >= 70) {
      progressBar.style.background = 'linear-gradient(90deg, #f59e0b, #d97706)';
    } else {
      progressBar.style.background = 'linear-gradient(90deg, #ef4444, #dc2626)';
    }
  }

  if (badge) {
    if (score === 100) {
      badge.className = 'badge badge-success';
      badge.textContent = '100% Chuẩn Bida (Hoàn Hảo)';
    } else if (score >= 70) {
      badge.className = 'badge badge-warning';
      badge.textContent = `${score}% Chuẩn Bida (Cần Hiệu Chỉnh)`;
    } else {
      badge.className = 'badge badge-danger';
      badge.textContent = `${score}% Chuẩn Bida (Lệch Chuẩn)`;
    }
  }

  // Render Checklist Items
  const checklistWrap = document.getElementById('redbida-checklist-items');
  if (checklistWrap) {
    checklistWrap.innerHTML = auditResults.map(item => {
      const valStr = redbidaValueText(item.value);
      const isPass = item.passed;
      const statusBadge = isPass
        ? '<span class="badge badge-success" style="font-size:0.72rem;">✅ Đạt chuẩn</span>'
        : '<span class="badge badge-danger" style="font-size:0.72rem;">❌ Lệch chuẩn</span>';

      const fixButton = isPass
        ? ''
        : `<button class="btn btn-sm redbida-autofix-btn" data-autofix-key="${escapeHtml(item.rule.key)}" type="button" style="font-size:0.72rem;padding:2px 8px;">⚡ Sửa nhanh</button>`;

      return `
        <div class="redbida-checklist-row ${isPass ? 'passed' : 'failed'}" data-check-key="${escapeHtml(item.rule.key)}">
          <div class="redbida-check-col-info">
            <div style="display:flex;align-items:center;gap:6px;flex-wrap:wrap;">
              <code>${escapeHtml(item.rule.key)}</code>
              ${statusBadge}
            </div>
            <div class="muted" style="font-size:0.74rem;margin-top:2px;">${escapeHtml(item.rule.desc)}</div>
          </div>
          <div class="redbida-check-col-val">
            <span class="muted" style="font-size:0.7rem;">Hiện tại:</span>
            <code class="redbida-check-cur-val" title="${escapeHtml(valStr)}">${escapeHtml(valStr.length > 30 ? valStr.slice(0, 27) + '…' : valStr)}</code>
          </div>
          <div class="redbida-check-col-action">
            ${fixButton}
          </div>
        </div>
      `;
    }).join('');

    checklistWrap.querySelectorAll('.redbida-autofix-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        redbidaAutoFixKey(btn.dataset.autofixKey);
      });
    });
  }

  return { score, passedCount, total, auditResults };
}

function redbidaAutoFixKey(key) {
  const rule = GOLDEN_STANDARD_RULES.find(r => r.key === key);
  if (!rule) return;
  const cur = getEffectiveValue(key);
  const fixedVal = rule.fix(cur);
  redbidaState.drafts.set(key, fixedVal);
  redbidaState.results.delete(key);

  if (key === 'ui_title') {
    redbidaState.drafts.set('company_name', fixedVal);
    redbidaState.drafts.set('custom_hashtags', generateSmartHashtags(fixedVal));
    const ini = getEffectiveValue('ui_tabs_links');
    const tabs = parse20TabsIni(ini, fixedVal);
    tabs.forEach(t => { t.vid_play_label = fixedVal; });
    redbidaState.drafts.set('ui_tabs_links', serialize20TabsIni(tabs));
  }

  redbidaRender();
  redbidaUpdateMetrics();
  redbidaAuditGoldenStandard();
  redbidaSetMessage(`Đã sửa nhanh key "${key}" về giá trị chuẩn Golden Template.`, 'ok');
}

function redbidaAutoFixAll() {
  let fixCount = 0;
  GOLDEN_STANDARD_RULES.forEach(rule => {
    const cur = getEffectiveValue(rule.key);
    const passed = rule.check(cur);
    if (!passed) {
      fixCount++;
    }
    const fixedVal = rule.fix(cur);
    redbidaState.drafts.set(rule.key, fixedVal);
    redbidaState.results.delete(rule.key);
  });

  const titleVal = getEffectiveValue('ui_title') || 'CX King Luxury';
  redbidaState.drafts.set('company_name', titleVal);
  redbidaState.drafts.set('custom_hashtags', generateSmartHashtags(titleVal));
  const ini = getEffectiveValue('ui_tabs_links');
  const tabs = parse20TabsIni(ini, titleVal);
  tabs.forEach(t => { t.vid_play_label = titleVal; });
  redbidaState.drafts.set('ui_tabs_links', serialize20TabsIni(tabs));

  const cc = getEffectiveValue('camera_count') || 8;
  redbidaState.drafts.set('camera_count', Number(cc) || 8);
  redbidaState.drafts.set('toolbar_show_count', Number(cc) || 8);

  redbidaRender();
  redbidaUpdateMetrics();
  redbidaAuditGoldenStandard();
  redbidaRenderPresetDiff(Object.fromEntries(redbidaState.drafts.entries()));
  redbidaSetMessage(`⚡ Đã tự động sửa ${fixCount} key lệch chuẩn về 100% Golden Standard. Sẵn sàng Submit!`, 'ok');
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
  redbidaAuditGoldenStandard();
  redbidaUpdateFloatingBar();
}

function redbidaUpdateFloatingBar() {
  const bar = document.getElementById('redbida-floating-bar');
  const countEl = document.getElementById('redbida-floating-count');
  const btnCountEl = document.getElementById('redbida-floating-btn-count');
  if (!bar) return;
  const count = redbidaState.drafts.size;
  if (count > 0) {
    if (countEl) countEl.textContent = String(count);
    if (btnCountEl) btnCountEl.textContent = String(count);
    bar.style.display = 'block';
  } else {
    bar.style.display = 'none';
  }
}

function redbidaDiscardAllDrafts() {
  if (!redbidaState.drafts.size) return;
  if (!window.confirm(`Bạn có chắc muốn hủy toàn bộ ${redbidaState.drafts.size} thay đổi chưa lưu?`)) return;
  redbidaState.drafts.clear();
  redbidaState.results.clear();
  redbidaRender();
  redbidaUpdateMetrics();
  redbidaSetMessage('Đã hủy toàn bộ thay đổi chưa lưu.', '');
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
    redbidaUpdateLiveCanvas(null, String(value || ''));
  }
  if (key === 'ui_title') {
    redbidaUpdateLiveCanvas(String(value || ''), null);
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
    redbidaUpdateLiveCanvas(null, null, reader.result);
  };
  reader.readAsDataURL(file);
}

function redbidaUpdateLiveCanvas(title, bg, logoUrl, slogan, hashtagsStr) {
  const preview = document.getElementById('redbida-preset-bg-preview');
  if (!preview) return;

  if (bg) {
    const cleanBg = bg.replace(/[;\s]+$/, '').trim();
    preview.style.background = cleanBg;
  }

  const effTitle = title || document.getElementById('redbida-preset-title')?.value || getEffectiveValue('ui_title') || 'CX King Luxury';
  const titleEl = document.getElementById('redbida-canvas-title') || preview.querySelector('.redbida-preview-title');
  if (titleEl) titleEl.textContent = effTitle;

  const effSlogan = slogan || getEffectiveValue('logo_header_text') || 'Billiard Live - Tải clip bàn bida và livestream';
  const subEl = document.getElementById('redbida-canvas-sub') || preview.querySelector('.redbida-preview-sub');
  if (subEl) subEl.textContent = effSlogan;

  const effLogo = logoUrl || getEffectiveValue('logo_header') || 'https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png';
  const logoEl = document.getElementById('redbida-canvas-logo') || preview.querySelector('.redbida-canvas-logo-img');
  if (logoEl && effLogo) logoEl.src = effLogo;

  const effHashtags = hashtagsStr || generateSmartHashtags(effTitle);
  const tags = effHashtags.split(/\s+/).filter(Boolean);

  const hashtagsWrap = document.getElementById('redbida-canvas-hashtags');
  if (hashtagsWrap) {
    hashtagsWrap.innerHTML = tags.map(t => `<span class="badge" style="background:rgba(255,255,255,0.18);backdrop-filter:blur(4px);color:#fff;font-size:0.7rem;font-weight:600;">${escapeHtml(t)}</span>`).join('');
  }

  const presetHashtags = document.getElementById('redbida-preset-hashtags-preview');
  if (presetHashtags) {
    presetHashtags.innerHTML = tags.map(t => `<span class="badge redbida-hashtag-badge">${escapeHtml(t)}</span>`).join(' ');
  }

  const tabsWrap = document.getElementById('redbida-canvas-tabs');
  if (tabsWrap && !tabsWrap.children.length) {
    let tabsHtml = '';
    for (let i = 1; i <= 20; i++) {
      const pad = String(i).padStart(2, '0');
      tabsHtml += `<button type="button" class="redbida-tab-sim-item ${i === 1 ? 'active' : ''}" data-sim-tab="C${pad}">Bàn ${pad}</button>`;
    }
    tabsWrap.innerHTML = tabsHtml;
    tabsWrap.querySelectorAll('.redbida-tab-sim-item').forEach(btn => {
      btn.addEventListener('click', () => {
        tabsWrap.querySelectorAll('.redbida-tab-sim-item').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        redbidaSelect20Tab(btn.dataset.simTab);
      });
    });
  }
}

function redbidaUpdatePresetPreview(title, bg) {
  redbidaUpdateLiveCanvas(title, bg);
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
  const rawBg = (bgInput?.value || 'linear-gradient(135deg, #0b192c 0%, #1e3e62 50%, #000000 100%)').trim();
  const bg = rawBg.replace(/[;\s]+$/, '').trim();

  const customHashtags = generateSmartHashtags(title);

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

  // Populate 15 standard parameters
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
  redbidaAuditGoldenStandard();

  // Render visual diff preview card in #redbida-preset-diff
  redbidaRenderPresetDiff(presetChanges);

  // Update live canvas preview
  redbidaUpdateLiveCanvas(title, bg, null, null, customHashtags);

  // Sync 20-tab editor
  redbidaSync20TabEditorFromDraft();

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
  const defaultBg = 'linear-gradient(135deg, #0b192c 0%, #1e3e62 50%, #000000 100%)';
  if (bgInput) bgInput.value = defaultBg;

  // Reset active swatch
  document.querySelectorAll('#redbida-preset-swatches .redbida-swatch').forEach((swatch, idx) => {
    swatch.classList.toggle('active', idx === 0);
  });

  // Update preview
  redbidaUpdateLiveCanvas('CX King Luxury', defaultBg);

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
    redbidaUpdateLiveCanvas(title, bg);
  });
}

function redbidaInitPresetInputs() {
  const bgInput = document.getElementById('redbida-preset-bg');
  const titleInput = document.getElementById('redbida-preset-title');

  bgInput?.addEventListener('input', () => {
    const bg = bgInput.value.trim().replace(/[;\s]+$/, '');
    const title = titleInput?.value || 'CX King Luxury';
    redbidaUpdateLiveCanvas(title, bg);

    document.querySelectorAll('#redbida-preset-swatches .redbida-swatch').forEach(swatch => {
      swatch.classList.toggle('active', swatch.dataset.bg === bg);
    });
  });

  titleInput?.addEventListener('input', () => {
    const title = titleInput.value.trim() || 'CX King Luxury';
    const bg = bgInput?.value || '';
    const hashtags = generateSmartHashtags(title);
    redbidaUpdateLiveCanvas(title, bg, null, null, hashtags);
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
        document.querySelectorAll('#redbida-group-pills button').forEach(p => {
          p.classList.toggle('active', p.dataset.pillGroup === matched);
        });
        document.getElementById('redbida-table')?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    });
  });

  // Group filter pills toolbar
  document.querySelectorAll('#redbida-group-pills button').forEach(pill => {
    pill.addEventListener('click', () => {
      document.querySelectorAll('#redbida-group-pills button').forEach(p => p.classList.remove('active'));
      pill.classList.add('active');
      const groupVal = pill.dataset.pillGroup || '';
      const select = document.getElementById('redbida-group');
      if (select) {
        select.value = groupVal;
      }
      redbidaRender();
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

  document.getElementById('redbida-toggle-inspector')?.addEventListener('click', () => {
    const insp = document.getElementById('redbida-inspector-panel');
    if (!insp) return;
    const isHidden = insp.style.display === 'none' || getComputedStyle(insp).display === 'none';
    insp.style.display = isHidden ? 'block' : 'none';
  });

  document.getElementById('redbida-toggle-20tabs')?.addEventListener('click', () => {
    const tabsPanel = document.getElementById('redbida-20tab-panel');
    if (!tabsPanel) return;
    const isHidden = tabsPanel.style.display === 'none' || getComputedStyle(tabsPanel).display === 'none';
    tabsPanel.style.display = isHidden ? 'block' : 'none';
  });

  document.getElementById('redbida-toggle-checklist')?.addEventListener('click', () => {
    const items = document.getElementById('redbida-checklist-items');
    if (!items) return;
    const isHidden = items.style.display === 'none' || getComputedStyle(items).display === 'none';
    items.style.display = isHidden ? 'grid' : 'none';
  });
}

function redbidaInit20TabEditor() {
  const matrixWrap = document.getElementById('redbida-tab-matrix-grid');
  if (!matrixWrap) return;

  const currentIni = getEffectiveValue('ui_tabs_links');
  const title = getEffectiveValue('ui_title') || 'CX King Luxury';
  redbida20TabsState.tabs = parse20TabsIni(currentIni, title);

  // Render 20 matrix buttons
  matrixWrap.innerHTML = redbida20TabsState.tabs.map(t => {
    const isActive = t.id === redbida20TabsState.selectedTab;
    const num = t.id.replace(/^C/i, '');
    return `<button type="button" class="redbida-matrix-btn ${isActive ? 'active' : ''}" data-tab-id="${escapeHtml(t.id)}">
      <span class="redbida-matrix-num">Bàn ${escapeHtml(num)}</span>
      <span class="redbida-matrix-code">${escapeHtml(t.id)}</span>
    </button>`;
  }).join('');

  matrixWrap.querySelectorAll('.redbida-matrix-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      redbidaSelect20Tab(btn.dataset.tabId);
    });
  });

  // Populate currently selected tab into form
  redbidaPopulate20TabForm(redbida20TabsState.selectedTab);

  // Form input listeners
  const playLabelInput = document.getElementById('redbida-tab-play-label');
  const streamLabelInput = document.getElementById('redbida-tab-stream-label');
  const listLabelInput = document.getElementById('redbida-tab-list-label');
  const refreshLabelInput = document.getElementById('redbida-tab-refresh-label');

  const onTabFieldChange = () => {
    const curTab = redbida20TabsState.tabs.find(t => t.id === redbida20TabsState.selectedTab);
    if (!curTab) return;
    if (playLabelInput) curTab.vid_play_label = playLabelInput.value;
    if (streamLabelInput) curTab.stream_label = streamLabelInput.value;
    if (listLabelInput) curTab.vid_list_label = listLabelInput.value;
    if (refreshLabelInput) curTab.list_refresh_label = refreshLabelInput.value;

    const newIni = serialize20TabsIni(redbida20TabsState.tabs);
    redbidaState.drafts.set('ui_tabs_links', newIni);
    redbidaState.results.delete('ui_tabs_links');

    const rawTextarea = document.getElementById('redbida-tab-raw-ini');
    if (rawTextarea) rawTextarea.value = newIni;

    const rowTextarea = document.querySelector('[data-red-key="ui_tabs_links"]');
    if (rowTextarea) rowTextarea.value = newIni;

    redbidaRender();
    redbidaUpdateMetrics();
  };

  playLabelInput?.addEventListener('input', onTabFieldChange);
  streamLabelInput?.addEventListener('input', onTabFieldChange);
  listLabelInput?.addEventListener('input', onTabFieldChange);
  refreshLabelInput?.addEventListener('input', onTabFieldChange);

  // Sync title button
  document.getElementById('redbida-tab-sync-title-btn')?.addEventListener('click', () => {
    const effTitle = getEffectiveValue('ui_title') || document.getElementById('redbida-preset-title')?.value?.trim() || 'CX King Luxury';
    redbida20TabsState.tabs.forEach(t => {
      t.vid_play_label = effTitle;
    });
    const newIni = serialize20TabsIni(redbida20TabsState.tabs);
    redbidaState.drafts.set('ui_tabs_links', newIni);
    redbidaState.results.delete('ui_tabs_links');

    if (playLabelInput) playLabelInput.value = effTitle;
    const rawTextarea = document.getElementById('redbida-tab-raw-ini');
    if (rawTextarea) rawTextarea.value = newIni;

    const rowTextarea = document.querySelector('[data-red-key="ui_tabs_links"]');
    if (rowTextarea) rowTextarea.value = newIni;

    redbidaRender();
    redbidaUpdateMetrics();
    redbidaSetMessage(`Đã đồng bộ tên quán "${effTitle}" cho toàn bộ 20 bàn bida.`, 'ok');
  });

  // Copy Stream URL
  document.getElementById('redbida-tab-copy-url-btn')?.addEventListener('click', () => {
    const tabId = redbida20TabsState.selectedTab;
    const num = parseInt(tabId.replace(/^C/i, ''), 10) || 1;
    const url = `rtsp://${location.hostname || '127.0.0.1'}:554/cam/realmonitor?channel=${num}&subtype=0`;
    if (navigator.clipboard) {
      navigator.clipboard.writeText(url).then(() => {
        redbidaSetMessage(`Đã sao chép Stream URL cho Bàn ${num} (${url}) vào clipboard.`, 'ok');
      }).catch(() => {
        redbidaSetMessage(`Stream URL: ${url}`, 'ok');
      });
    } else {
      redbidaSetMessage(`Stream URL: ${url}`, 'ok');
    }
  });

  // View toggle (Visual vs Raw INI)
  document.getElementById('redbida-tab-view-toggle')?.addEventListener('click', () => {
    const visualWrap = document.getElementById('redbida-tab-visual-wrap');
    const rawWrap = document.getElementById('redbida-tab-raw-wrap');
    const toggleBtn = document.getElementById('redbida-tab-view-toggle');
    if (!visualWrap || !rawWrap) return;

    if (redbida20TabsState.viewMode === 'visual') {
      redbida20TabsState.viewMode = 'raw';
      visualWrap.style.display = 'none';
      rawWrap.style.display = 'block';
      if (toggleBtn) toggleBtn.textContent = '🎛️ Xem Giao Diện Trực Quan';
      const rawTextarea = document.getElementById('redbida-tab-raw-ini');
      if (rawTextarea) rawTextarea.value = serialize20TabsIni(redbida20TabsState.tabs);
    } else {
      redbida20TabsState.viewMode = 'visual';
      visualWrap.style.display = 'block';
      rawWrap.style.display = 'none';
      if (toggleBtn) toggleBtn.textContent = '🔄 Xem Mã INI Gốc';

      const rawTextarea = document.getElementById('redbida-tab-raw-ini');
      if (rawTextarea) {
        redbida20TabsState.tabs = parse20TabsIni(rawTextarea.value, getEffectiveValue('ui_title'));
        redbidaPopulate20TabForm(redbida20TabsState.selectedTab);
      }
    }
  });

  // Raw textarea input
  document.getElementById('redbida-tab-raw-ini')?.addEventListener('input', (e) => {
    const val = e.target.value;
    redbidaState.drafts.set('ui_tabs_links', val);
    redbidaState.results.delete('ui_tabs_links');
    redbida20TabsState.tabs = parse20TabsIni(val, getEffectiveValue('ui_title'));
    const rowTextarea = document.querySelector('[data-red-key="ui_tabs_links"]');
    if (rowTextarea) rowTextarea.value = val;
    redbidaRender();
    redbidaUpdateMetrics();
  });
}

function redbidaSelect20Tab(tabId) {
  redbida20TabsState.selectedTab = tabId;
  document.querySelectorAll('#redbida-tab-matrix-grid .redbida-matrix-btn').forEach(b => {
    b.classList.toggle('active', b.dataset.tabId === tabId);
  });
  redbidaPopulate20TabForm(tabId);
}

function redbidaPopulate20TabForm(tabId) {
  const tab = redbida20TabsState.tabs.find(t => t.id === tabId);
  if (!tab) return;

  const num = tab.id.replace(/^C/i, '');
  const titleHeader = document.getElementById('redbida-current-tab-title');
  if (titleHeader) titleHeader.textContent = `Cấu hình Bàn ${num} ([${tab.id}])`;

  const playLabelInput = document.getElementById('redbida-tab-play-label');
  const streamLabelInput = document.getElementById('redbida-tab-stream-label');
  const listLabelInput = document.getElementById('redbida-tab-list-label');
  const refreshLabelInput = document.getElementById('redbida-tab-refresh-label');

  if (playLabelInput) playLabelInput.value = tab.vid_play_label || '';
  if (streamLabelInput) streamLabelInput.value = tab.stream_label || 'Video Trực tiếp';
  if (listLabelInput) listLabelInput.value = tab.vid_list_label || 'Danh sách highlight';
  if (refreshLabelInput) refreshLabelInput.value = tab.list_refresh_label || 'Cập nhật highlight';
}

function redbidaSync20TabEditorFromDraft() {
  const ini = getEffectiveValue('ui_tabs_links');
  const title = getEffectiveValue('ui_title') || 'CX King Luxury';
  redbida20TabsState.tabs = parse20TabsIni(ini, title);
  redbidaPopulate20TabForm(redbida20TabsState.selectedTab);
  const rawTextarea = document.getElementById('redbida-tab-raw-ini');
  if (rawTextarea) rawTextarea.value = ini || '';
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
    redbidaSync20TabEditorFromDraft();
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
window.redbidaRenderPresetDiff = redbidaRenderPresetDiff;
window.redbidaInitSwatches = redbidaInitSwatches;
window.redbidaInitPillarButtons = redbidaInitPillarButtons;
window.redbidaInitToggles = redbidaInitToggles;
window.redbidaLoadCatalog = redbidaLoadCatalog;
window.redbidaRefresh = redbidaRefresh;
window.redbidaApply = redbidaApply;
window.redbidaTimeStatus = redbidaTimeStatus;
window.removeVietnameseTones = removeVietnameseTones;
window.redbidaAuditGoldenStandard = redbidaAuditGoldenStandard;
window.redbidaAutoFixKey = redbidaAutoFixKey;
window.redbidaAutoFixAll = redbidaAutoFixAll;
window.redbidaUpdateFloatingBar = redbidaUpdateFloatingBar;
window.redbidaDiscardAllDrafts = redbidaDiscardAllDrafts;
window.redbidaState = redbidaState;
window.REDBIDA_GRADIENT_PALETTE = REDBIDA_GRADIENT_PALETTE;

document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('redbida-refresh')?.addEventListener('click', () => redbidaRefresh(true).catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-apply')?.addEventListener('click', () => redbidaApply().catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-time-refresh')?.addEventListener('click', () => redbidaTimeStatus().catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-preset-gen-btn')?.addEventListener('click', redbidaGeneratePreset);
  document.getElementById('redbida-preset-reset-btn')?.addEventListener('click', redbidaResetPresetForm);
  document.getElementById('redbida-search')?.addEventListener('input', redbidaRender);
  document.getElementById('redbida-group')?.addEventListener('change', () => {
    redbidaRender();
    const groupVal = document.getElementById('redbida-group')?.value || '';
    document.querySelectorAll('#redbida-group-pills button').forEach(p => {
      p.classList.toggle('active', p.dataset.pillGroup === groupVal);
    });
  });
  document.getElementById('redbida-dirty-only')?.addEventListener('change', redbidaRender);

  document.getElementById('redbida-autofix-all-btn')?.addEventListener('click', redbidaAutoFixAll);
  document.getElementById('redbida-audit-btn')?.addEventListener('click', redbidaAuditGoldenStandard);

  document.getElementById('redbida-floating-apply')?.addEventListener('click', () => redbidaApply().catch(err => redbidaSetMessage(err.message, 'err')));
  document.getElementById('redbida-floating-discard')?.addEventListener('click', redbidaDiscardAllDrafts);

  redbidaInitSwatches();
  redbidaInitPresetInputs();
  redbidaInitPillarButtons();
  redbidaInitToggles();
  redbidaInit20TabEditor();
  redbidaAuditGoldenStandard();
  redbidaUpdateLiveCanvas();
});
