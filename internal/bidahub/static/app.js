/* ============================================================================
   iNut Bida Fleet Hub (:2090) — Impeccable Operate SPA Controller (app.js)
   Views: #fleet, #box/{box_id}, #hardening, #cameras, #redbida, #handover, #analytics
   Zero external runtime dependencies; XSS-safe DOM rendering; Pure-JS SVG Icons, QR & Charts.
   ============================================================================ */
(function () {
  'use strict';

  var GOLDEN_PILLAR_META = [
    { id: 'zram_unbounded', step: 1, short: 'B1: ZRAM 0', name: 'ZRAM mem_limit=0 & swappiness=80' },
    { id: 'pm2_memory_guard', step: 2, short: 'B2: PM2 Guard', name: 'PM2 Memory Guard (280M/120M) + /dev/null logs' },
    { id: 'sysrq_disabled', step: 3, short: 'B3: Sysrq=0', name: 'kernel.sysrq=0 & gỡ sysrq-trigger suicide' },
    { id: 'camera_watchdog', step: 4, short: 'B4: Watchdog', name: 'shinobi-camera-watchdog.timer (60s pgrep ffmpeg)' },
    { id: 'storage_guard', step: 5, short: 'B5: Storage', name: 'shinobi-storage-guard (15m xả đĩa 90% -> 80%)' },
    { id: 'udev_automount', step: 6, short: 'B6: udev', name: 'udev automount noatime,users,errors=continue' },
    { id: 'clean_fstab', step: 7, short: 'B7: fstab', name: 'Clean /etc/fstab (CẤM /dev/mmcblk0p1 treo boot)' },
    { id: 'shinobi_hardened', step: 8, short: 'B8: Shinobi', name: 'doSnapshot:false + monitor.js fix + frag_keyframe+empty_moov' }
  ];

  /* Uniform inline SVG icon library (stroke-width 1.75, zero emoji tofu) */
  var SVG_PATHS = {
    server: '<rect x="2" y="3" width="20" height="7" rx="2"/><rect x="2" y="14" width="20" height="7" rx="2"/><line x1="6" y1="6.5" x2="6.01" y2="6.5"/><line x1="6" y1="17.5" x2="6.01" y2="17.5"/>',
    shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="m9 12 2 2 4-4"/>',
    video: '<path d="m22 8-6 4 6 4V8Z"/><rect x="2" y="6" width="14" height="12" rx="2"/>',
    sliders: '<line x1="4" y1="21" x2="4" y2="14"/><line x1="4" y1="10" x2="4" y2="3"/><line x1="12" y1="21" x2="12" y2="12"/><line x1="12" y1="8" x2="12" y2="3"/><line x1="20" y1="21" x2="20" y2="16"/><line x1="20" y1="12" x2="20" y2="3"/><line x1="2" y1="14" x2="6" y2="14"/><line x1="10" y1="8" x2="14" y2="8"/><line x1="18" y1="16" x2="22" y2="16"/>',
    qr: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><path d="M14 14h3v3h-3z"/><path d="M17 17h4v4h-4z"/>',
    chart: '<line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/>',
    check: '<polyline points="20 6 9 17 4 12"/>',
    warn: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>',
    wrench: '<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
    unlock: '<rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 9.9-1"/>',
    zap: '<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/>',
    external: '<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/>',
    key: '<circle cx="7.5" cy="15.5" r="5.5"/><path d="m21 2-9.6 9.6"/><path d="m15.5 7.5 3 3L22 7l-3-3"/>',
    phone: '<rect x="5" y="2" width="14" height="20" rx="2"/><line x1="12" y1="18" x2="12.01" y2="18"/>',
    monitor: '<rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>',
    tablet: '<rect x="4" y="2" width="16" height="20" rx="2"/><line x1="12" y1="18" x2="12.01" y2="18"/>',
    globe: '<circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>',
    cpu: '<rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M15 2v2"/><path d="M15 20v2"/><path d="M2 15h2"/><path d="M2 9h2"/><path d="M20 15h2"/><path d="M20 9h2"/><path d="M9 2v2"/><path d="M9 20v2"/>',
    pool: '<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="3"/>',
    pickle: '<circle cx="12" cy="12" r="9"/><circle cx="9.5" cy="9.5" r="0.9" fill="currentColor"/><circle cx="14.5" cy="9.5" r="0.9" fill="currentColor"/><circle cx="12" cy="14.5" r="0.9" fill="currentColor"/>',
    carom: '<circle cx="9" cy="10" r="4"/><circle cx="16" cy="14" r="4"/>',
    box: '<path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/>',
    flame: '<path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z"/>',
    sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/>',
    moon: '<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>',
    trophy: '<path d="M6 9H4.5a2.5 2.5 0 0 1 0-5H6"/><path d="M18 9h1.5a2.5 2.5 0 0 0 0-5H18"/><path d="M4 22h16"/><path d="M10 14.66V17c0 .55-.47.98-.97 1.21C7.85 18.75 7 20.24 7 22"/><path d="M14 14.66V17c0 .55.47.98.97 1.21C16.15 18.75 17 20.24 17 22"/><path d="M18 2H6v7a6 6 0 0 0 12 0V2Z"/>',
    info: '<circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/>',
    building: '<path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z"/><path d="M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2"/><path d="M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2"/>',
    x: '<line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>',
    chevronDown: '<polyline points="6 9 12 15 18 9"/>',
    chevronRight: '<polyline points="9 18 15 12 9 6"/>',
    chevronLeft: '<polyline points="15 18 9 12 15 6"/>',
    more: '<circle cx="12" cy="12" r="1.2" fill="currentColor"/><circle cx="19" cy="12" r="1.2" fill="currentColor"/><circle cx="5" cy="12" r="1.2" fill="currentColor"/>',
    users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>',
    scissors: '<circle cx="6" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><line x1="20" y1="4" x2="8.12" y2="15.88"/><line x1="14.47" y1="14.48" x2="20" y2="20"/><line x1="8.12" y1="8.12" x2="12" y2="12"/>',
    share: '<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>',
    compass: '<circle cx="12" cy="12" r="10"/><polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76"/>',
    funnel: '<polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>',
    play: '<circle cx="12" cy="12" r="10"/><polygon points="10 8 16 12 10 16 10 8"/>'
  };

  function svgIcon(name, cls) {
    var body = SVG_PATHS[name] || SVG_PATHS.info;
    var className = cls ? ('ui-icon ' + cls) : 'ui-icon';
    return '<svg class="' + className + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + body + '</svg>';
  }

  var defaultListPageSize = 20;

  var VIEW_LABELS = {
    fleet: 'Đội Box',
    hardening: '8 Bước',
    cameras: 'Camera',
    redbida: 'RedBida',
    handover: 'Bàn Giao',
    analytics: 'Thống Kê'
  };

  var state = {
    activeView: 'fleet',
    drawerBoxId: null,
    fleetFilter: 'all',
    fleetSearch: '',
    groupByClub: true,
    collapsedClubs: {},
    expandedMobileBoxIds: {},
    expandedHardeningIds: {},
    expandedCameraIds: {},
    expandedRedbidaDomainIds: {},
    expandedRedbidaBoxIds: {},
    lastFleetRenderSig: '',
    openCardMenuId: null,
    pendingFleetRender: false,
    mobileKpiExpanded: false,
    kpi: {},
    clubs: [],
    boxes: [],
    hardeningBoxes: [],
    hardeningSummary: {},
    hardeningFilter: 'all',
    hardeningSearch: '',
    cameras: [],
    camerasSummary: {},
    selectedCameraIds: {},
    redbidaBoxes: [],
    redbidaDomains: [],
    redbidaMergedDomains: [],
    redbidaConflicts: [],
    redbidaBrandFilter: 'all',
    redbidaSearch: '',
    handoverPacks: [],
    handoverSearch: '',
    activeHandoverPack: null,
    analyticsRange: '7d',
    analyticsFrom: '',
    analyticsTo: '',
    analyticsCompare: true,
    analyticsClubId: '',
    analyticsBoxId: '',
    analyticsSport: 'all',
    analyticsSource: 'all',
    analyticsAiOnly: false,
    analyticsGroupMode: 'clubs',
    analyticsSortCol: 'qr_views',
    analyticsSortDir: 'desc',
    analyticsDrillClubId: '',
    analyticsDrillBoxId: '',
    analyticsDrillLabel: '',
    analyticsTrendMetric: 'unique_visitors',
    analyticsHeatMode: 'heatmap',
    analyticsRtHotTab: 'clubs',
    analyticsRtData: null,
    analyticsRtTimer: null,
    analyticsRtReqSeq: 0,
    analyticsTopSearch: '',
    analyticsCamerasSearch: '',
    expandedAnalyticsTopIds: {},
    expandedAnalyticsCamIds: {},
    analyticsOverview: null,
    analyticsCameras: [],
    analyticsReqSeq: 0,
    analyticsCamReqSeq: 0,
    lastAnalyticsClubSig: '',
    cmdSelectedIdx: 0,
    sseConnected: false,
    pollTimer: null,
    pagination: {
      fleet: { page: 1, pageSize: defaultListPageSize },
      hardening: { page: 1, pageSize: defaultListPageSize },
      cameras: { page: 1, pageSize: 20 },
      redbidaDomains: { page: 1, pageSize: defaultListPageSize },
      redbidaBoxes: { page: 1, pageSize: defaultListPageSize },
      handoverClubs: { page: 1, pageSize: 10 },
      analyticsTop: { page: 1, pageSize: 10 },
      analyticsCameras: { page: 1, pageSize: 10 }
    }
  };

  /* --------------------------------------------------------------------------
     Utilities: Escaping, Formatters, Toast, Fetch Wrapper & Pure-JS SVG QR
     -------------------------------------------------------------------------- */
  function esc(val) {
    if (val === null || val === undefined) return '';
    return String(val)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  var escapeHtml = esc;

  /* --------------------------------------------------------------------------
     Fast Vietnamese Diacritic-Insensitive Search & Reusable Pagination Engine
     -------------------------------------------------------------------------- */
  function normalizeVN(str) {
    return String(str || '')
      .trim()
      .toLowerCase()
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
      .replace(/[đĐ]/g, 'd');
  }

  function indexItemSearch(item, fields) {
    if (!item) return item;
    var parts = [];
    for (var i = 0; i < fields.length; i++) {
      var v = fields[i];
      if (v !== null && v !== undefined && v !== '') parts.push(String(v));
    }
    var raw = parts.join(' ').toLowerCase();
    item._searchRaw = raw;
    item._searchNorm = normalizeVN(raw);
    return item;
  }

  function parseSearchQuery(query) {
    var trimmed = String(query || '').trim().toLowerCase();
    if (!trimmed) return null;
    var words = trimmed.split(/\s+/).filter(Boolean);
    if (!words.length) return null;
    return words.map(function (w) {
      return { raw: w, norm: normalizeVN(w) };
    });
  }

  function matchesSearch(item, tokens, fallbackFields) {
    if (!tokens || !tokens.length) return true;
    if (!item) return false;
    if (item._searchNorm === undefined && fallbackFields) {
      indexItemSearch(item, fallbackFields);
    }
    var raw = item._searchRaw || '';
    var norm = item._searchNorm || '';
    for (var i = 0; i < tokens.length; i++) {
      var t = tokens[i];
      if (raw.indexOf(t.raw) === -1 && (!t.norm || norm.indexOf(t.norm) === -1)) {
        return false;
      }
    }
    return true;
  }

  function makeDebounced(fn, waitMs) {
    var timer = null;
    var wrapped = function () {
      var ctx = this;
      var args = arguments;
      clearTimeout(timer);
      timer = setTimeout(function () {
        timer = null;
        fn.apply(ctx, args);
      }, waitMs);
    };
    wrapped.flush = function () {
      if (timer) {
        clearTimeout(timer);
        timer = null;
      }
      fn();
    };
    return wrapped;
  }

  function syncSearchBoxState(inputId, matchCountId, matched, total) {
    var input = document.getElementById(inputId);
    if (input) {
      var wrap = input.closest('.search-box');
      var hasVal = String(input.value || '').length > 0;
      if (wrap) {
        wrap.classList.toggle('has-value', hasVal);
        var clearBtn = wrap.querySelector('.search-clear-btn');
        if (clearBtn) clearBtn.hidden = !hasVal;
      }
    }
    if (matchCountId) {
      var pill = document.getElementById(matchCountId);
      if (pill) pill.textContent = fmtInt(matched) + '/' + fmtInt(total);
    }
  }

  function resetTabPage(tabKey) {
    if (state.pagination[tabKey]) {
      state.pagination[tabKey].page = 1;
    }
  }

  function paginateItems(items, tabKey) {
    var pg = state.pagination[tabKey];
    if (!pg) {
      pg = { page: 1, pageSize: 20 };
      state.pagination[tabKey] = pg;
    }
    var total = items.length;
    var pageSize = Number(pg.pageSize) || 20;
    var totalPages = Math.max(1, Math.ceil(total / pageSize));
    if (pg.page > totalPages) pg.page = totalPages;
    if (pg.page < 1) pg.page = 1;
    var startIdx = (pg.page - 1) * pageSize;
    var endIdx = Math.min(total, startIdx + pageSize);
    return {
      items: items.slice(startIdx, endIdx),
      page: pg.page,
      pageSize: pageSize,
      total: total,
      totalPages: totalPages,
      start: total === 0 ? 0 : startIdx + 1,
      end: endIdx
    };
  }

  function buildPageNumbers(page, totalPages, isMobile) {
    if (totalPages <= 1) return [1];
    var sibling = isMobile ? 0 : 1;
    var set = {};
    set[1] = true;
    set[totalPages] = true;
    for (var p = Math.max(1, page - sibling); p <= Math.min(totalPages, page + sibling); p++) {
      set[p] = true;
    }
    if (!isMobile && totalPages <= 7) {
      for (var k = 1; k <= totalPages; k++) set[k] = true;
    }
    var nums = Object.keys(set).map(Number).sort(function (a, b) { return a - b; });
    var out = [];
    for (var i = 0; i < nums.length; i++) {
      if (i > 0) {
        var gap = nums[i] - nums[i - 1];
        if (gap === 2) out.push(nums[i - 1] + 1);
        else if (gap > 2) out.push('...');
      }
      out.push(nums[i]);
    }
    return out;
  }

  function renderPaginationBar(tabKey, meta, unitLabel, extraSuffix) {
    if (!meta || meta.total === 0) return '';
    var isMob = typeof window !== 'undefined' && window.innerWidth < 768;
    var pages = buildPageNumbers(meta.page, meta.totalPages, isMob);
    var sizes = [10, 20, 50, 100];

    var optionsHtml = sizes.map(function (sz) {
      return '<option value="' + sz + '"' + (sz === meta.pageSize ? ' selected' : '') + '>' + sz + '</option>';
    }).join('');

    var pageBtnsHtml = pages.map(function (p) {
      if (p === '...') return '<span class="pager-ellipsis" aria-hidden="true">…</span>';
      var isAct = p === meta.page;
      return '<button type="button" class="pager-btn' + (isAct ? ' active' : '') + '" data-pager-key="' + esc(tabKey) + '" data-pager-page="' + p + '"' + (isAct ? ' aria-current="page"' : '') + '>' + p + '</button>';
    }).join('');

    return '<div class="pagination-bar tabular-nums" data-pagination-bar="' + esc(tabKey) + '">' +
      '<div class="pager-info">' +
        '<span class="pager-summary">Hiển thị <strong>' + esc(fmtInt(meta.start)) + '–' + esc(fmtInt(meta.end)) + '</strong> / <strong>' + esc(fmtInt(meta.total)) + '</strong> ' + esc(unitLabel || 'mục') + (extraSuffix ? (' <span class="text-muted">' + esc(extraSuffix) + '</span>') : '') + '</span>' +
        '<label class="pager-size-wrap">' +
          '<span>Số dòng</span>' +
          '<select class="select pager-size-select" data-pager-size="' + esc(tabKey) + '" aria-label="Số mục mỗi trang">' + optionsHtml + '</select>' +
        '</label>' +
      '</div>' +
      '<div class="pager-nav" role="navigation" aria-label="Phân trang ' + esc(unitLabel || '') + '">' +
        '<button type="button" class="pager-btn" data-pager-key="' + esc(tabKey) + '" data-pager-page="' + (meta.page - 1) + '"' + (meta.page <= 1 ? ' disabled' : '') + ' aria-label="Trang trước">' + svgIcon('chevronLeft', 'ui-icon-sm') + '</button>' +
        pageBtnsHtml +
        '<button type="button" class="pager-btn" data-pager-key="' + esc(tabKey) + '" data-pager-page="' + (meta.page + 1) + '"' + (meta.page >= meta.totalPages ? ' disabled' : '') + ' aria-label="Trang sau">' + svgIcon('chevronRight', 'ui-icon-sm') + '</button>' +
      '</div>' +
    '</div>';
  }

  function scrollToPagerSection(tabKey) {
    var targetMap = {
      fleet: 'fleet-command-dock',
      hardening: 'hardening-matrix-container',
      cameras: 'view-cameras',
      redbidaDomains: 'redbida-domains-container',
      redbidaBoxes: 'redbida-boxes-container',
      handoverClubs: 'handover-club-picker',
      analyticsTop: 'analytics-top-table-wrap',
      analyticsCameras: 'analytics-cameras-table-wrap'
    };
    var el = document.getElementById(targetMap[tabKey]);
    if (!el) return;
    var rect = el.getBoundingClientRect();
    var offset = (window.innerWidth < 768) ? 50 : 108;
    var targetY = Math.max(0, window.scrollY + rect.top - offset);
    window.scrollTo({ top: targetY, behavior: 'smooth' });
  }

  function renderByPagerKey(tabKey, shouldScroll) {
    if (tabKey === 'fleet') renderFleetMatrix({ force: true });
    else if (tabKey === 'hardening') renderHardeningMatrix();
    else if (tabKey === 'cameras') renderCamerasTable();
    else if (tabKey === 'redbidaDomains' || tabKey === 'redbidaBoxes') renderRedbidaView();
    else if (tabKey === 'handoverClubs') renderHandoverClubPicker();
    else if (tabKey === 'analyticsTop') renderAnalyticsTopTable(state.analyticsOverview || {});
    else if (tabKey === 'analyticsCameras') renderAnalyticsCamerasTable();
    if (shouldScroll) scrollToPagerSection(tabKey);
  }

  function fmtNum(val, decimals) {
    var n = Number(val);
    if (val === null || val === undefined || val === '' || isNaN(n)) {
      n = 0;
    }
    return decimals !== undefined ? n.toFixed(decimals) : String(Math.round(n));
  }

  function fmtInt(val) {
    var n = Number(val);
    if (val === null || val === undefined || val === '' || isNaN(n)) n = 0;
    return Math.round(n).toLocaleString('vi-VN');
  }

  function fmtCompact(val) {
    var n = Number(val) || 0;
    if (n >= 1000000) return (n / 1000000).toFixed(1).replace(/\.0$/, '') + 'M';
    if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
    return String(Math.round(n));
  }

  function fmtBytes(bytes) {
    var n = Number(bytes) || 0;
    if (n <= 0) return '0 B';
    var units = ['B', 'KB', 'MB', 'GB', 'TB'];
    var idx = 0;
    while (n >= 1024 && idx < units.length - 1) {
      n = n / 1024;
      idx++;
    }
    var dec = idx === 0 ? 0 : (n >= 100 ? 0 : (n >= 10 ? 1 : 2));
    return n.toFixed(dec) + ' ' + units[idx];
  }

  function fmtCutMinutes(seconds) {
    var s = Number(seconds) || 0;
    if (s <= 0) return '0 phút';
    if (s < 60) return Math.round(s) + ' giây';
    var mins = s / 60;
    if (mins < 10) return mins.toFixed(1) + ' phút';
    return Math.round(mins).toLocaleString('vi-VN') + ' phút';
  }

  function fmtIngestTime(ts) {
    if (!ts) return '--';
    var str = String(ts).trim();
    var d = new Date(str.indexOf('T') !== -1 || str.indexOf('Z') !== -1 ? str : str.replace(' ', 'T'));
    if (isNaN(d.getTime())) return str.slice(0, 16);
    var pad = function (x) { return (x < 10 ? '0' : '') + x; };
    return pad(d.getHours()) + ':' + pad(d.getMinutes()) + ' ' + pad(d.getDate()) + '/' + pad(d.getMonth() + 1);
  }

  function showToast(msg, isError) {
    var stack = document.getElementById('toast-stack');
    if (!stack) return;
    var el = document.createElement('div');
    el.className = 'toast-msg' + (isError ? ' error' : '');
    el.innerHTML = svgIcon(isError ? 'warn' : 'check', isError ? 'text-crimson' : 'text-emerald') +
      '<span>' + esc(msg) + '</span>';
    stack.appendChild(el);
    setTimeout(function () {
      if (el.parentNode) el.parentNode.removeChild(el);
    }, 3600);
  }

  function apiFetch(url, opts) {
    var options = Object.assign({
      credentials: 'same-origin',
      headers: { 'Accept': 'application/json' }
    }, opts || {});
    if (options.body && typeof options.body === 'object' && !(options.body instanceof FormData)) {
      options.headers = Object.assign({ 'Content-Type': 'application/json' }, options.headers);
      options.body = JSON.stringify(options.body);
    }
    return fetch(url, options).then(function (res) {
      if (res.status === 401) {
        window.location.href = '/login';
        throw new Error('Unauthorized');
      }
      return res.json().catch(function () { return {}; }).then(function (data) {
        if (!res.ok) {
          throw new Error((data && (data.message || data.error)) || ('HTTP ' + res.status));
        }
        return data;
      });
    });
  }

  /* Pure-JS deterministic SVG QR matrix fallback (zero external dependencies) */
  function generateFallbackQrSvg(text) {
    var str = String(text || 'https://inut-bida.p2p.inut.io.vn');
    var size = 25;
    var grid = [];
    for (var r = 0; r < size; r++) {
      grid[r] = [];
      for (var c = 0; c < size; c++) grid[r][c] = false;
    }
    function drawFinder(rowOffset, colOffset) {
      for (var r = 0; r < 7; r++) {
        for (var c = 0; c < 7; c++) {
          var border = (r === 0 || r === 6 || c === 0 || c === 6);
          var center = (r >= 2 && r <= 4 && c >= 2 && c <= 4);
          grid[rowOffset + r][colOffset + c] = (border || center);
        }
      }
    }
    drawFinder(0, 0);
    drawFinder(0, size - 7);
    drawFinder(size - 7, 0);
    for (var i = 8; i < size - 8; i++) {
      grid[6][i] = (i % 2 === 0);
      grid[i][6] = (i % 2 === 0);
    }
    var hash = 2166136261;
    for (var k = 0; k < str.length; k++) {
      hash ^= str.charCodeAt(k);
      hash = Math.imul(hash, 16777619) >>> 0;
    }
    var seed = hash || 1337;
    for (var rr = 0; rr < size; rr++) {
      for (var cc = 0; cc < size; cc++) {
        var inFinderTopLeft = (rr < 8 && cc < 8);
        var inFinderTopRight = (rr < 8 && cc >= size - 8);
        var inFinderBotLeft = (rr >= size - 8 && cc < 8);
        if (inFinderTopLeft || inFinderTopRight || inFinderBotLeft || rr === 6 || cc === 6) continue;
        seed = (Math.imul(seed, 1664525) + 1013904223 + str.charCodeAt((rr * size + cc) % str.length)) >>> 0;
        grid[rr][cc] = ((seed & 3) === 0 || (seed & 7) === 3);
      }
    }
    var rects = [];
    for (var y = 0; y < size; y++) {
      for (var x = 0; x < size; x++) {
        if (grid[y][x]) {
          rects.push('<rect x="' + (x + 2) + '" y="' + (y + 2) + '" width="1" height="1" fill="#071a24"/>');
        }
      }
    }
    var total = size + 4;
    return '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ' + total + ' ' + total + '" shape-rendering="crispEdges">' +
      '<rect width="' + total + '" height="' + total + '" fill="#ffffff"/>' +
      rects.join('') +
      '</svg>';
  }

  function renderSafeQrInto(containerId, backendSvg, urlText) {
    var el = document.getElementById(containerId);
    if (!el) return;
    if (backendSvg && typeof backendSvg === 'string' && backendSvg.indexOf('<svg') !== -1 && backendSvg.toLowerCase().indexOf('<script') === -1) {
      el.innerHTML = backendSvg;
    } else {
      el.innerHTML = generateFallbackQrSvg(urlText);
    }
  }

  /* --------------------------------------------------------------------------
     Theme Management (#eef5f4 Light / #071a24 Dark)
     -------------------------------------------------------------------------- */
  function syncThemeIcon() {
    var icon = document.getElementById('theme-toggle-icon');
    var btn = document.getElementById('theme-toggle');
    var isLight = document.documentElement.getAttribute('data-theme') === 'light';
    if (icon) {
      icon.innerHTML = isLight ? SVG_PATHS.moon : SVG_PATHS.sun;
    }
    if (btn) {
      var label = isLight ? 'Chuyển sang giao diện Tối' : 'Chuyển sang giao diện Sáng';
      btn.setAttribute('title', label);
      btn.setAttribute('aria-label', label);
    }
  }

  function initTheme() {
    syncThemeIcon();
    var toggleBtn = document.getElementById('theme-toggle');
    if (!toggleBtn) return;
    toggleBtn.addEventListener('click', function () {
      var cur = document.documentElement.getAttribute('data-theme') === 'light' ? 'light' : 'dark';
      var next = cur === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', next);
      localStorage.setItem('bidahub-theme', next);
      localStorage.setItem('kspcam-theme', next);
      syncThemeIcon();
    });
  }

  /* --------------------------------------------------------------------------
     View 1: #fleet — KPI Cards, Sticky Cockpit, Club-Aware Pagination & /device/86 Mobile Cards
     -------------------------------------------------------------------------- */
  function loadFleetSummary(opts) {
    var fromSSE = !!(opts && opts.fromSSE);
    return apiFetch('/fleet/api/summary').then(function (data) {
      state.kpi = data.kpi || {};
      state.clubs = Array.isArray(data.clubs) ? data.clubs : [];
      var boxes = Array.isArray(data.boxes) ? data.boxes : [];
      boxes.forEach(function (b) {
        indexItemSearch(b, [
          b.id, b.box_id, b.name, b.club_name, b.club_id,
          b.vpn_ip, b.lan_ip, b.shinobi_group_key,
          b.store_domain, b.cstore_domain, b.soc_model, b.branding,
          b.online ? 'online truc tuyen' : 'offline mat ket noi'
        ]);
      });
      state.boxes = boxes;
      renderTopbarLedsAndKpis();
      if (fromSSE && state.openCardMenuId) {
        state.pendingFleetRender = true;
      } else {
        renderFleetMatrix({ force: !fromSSE });
      }
      populateAnalyticsClubFilter();
    }).catch(function (err) {
      if (!fromSSE) {
        showToast('Không thể tải tổng quan đội thiết bị: ' + err.message, true);
      }
    });
  }

  var fetchSummary = loadFleetSummary;

  function renderTopbarLedsAndKpis() {
    var boxes = state.boxes || [];
    var kpi = state.kpi || {};
    var clubsTotal = kpi.clubs_total || state.clubs.length || 0;
    var boxesTotal = kpi.boxes_total || kpi.total_boxes || boxes.length || 0;
    var boxesOnline = kpi.boxes_online !== undefined ? kpi.boxes_online : boxes.filter(function (b) { return b.online !== false; }).length;
    var boxesOffline = kpi.boxes_offline !== undefined ? kpi.boxes_offline : Math.max(0, boxesTotal - boxesOnline);

    var warnBoxes = boxes.filter(function (b) {
      var ffExp = b.ffmpeg_expected || (b.cameras_count ? b.cameras_count * 2 : 0);
      var ffWarn = ffExp > 0 && (b.ffmpeg_count || 0) < ffExp;
      var hWarn = (b.hardening_score !== undefined ? b.hardening_score : 8) < 8;
      var sdWarn = (b.sd_used_pct || 0) >= 85;
      var ramWarn = (b.ram_used_pct || b.ram_pct || 0) >= 85;
      var cpuWarn = (b.cpu_pct || 0) >= 85;
      return ffWarn || hWarn || sdWarn || ramWarn || cpuWarn;
    }).length;

    var ledOnline = document.getElementById('led-online-count');
    var ledWarn = document.getElementById('led-warn-count');
    var ledOffline = document.getElementById('led-offline-count');
    if (ledOnline) ledOnline.textContent = String(boxesOnline);
    if (ledWarn) ledWarn.textContent = String(warnBoxes);
    if (ledOffline) ledOffline.textContent = String(boxesOffline);

    var navBadgeFleet = document.getElementById('nav-badge-fleet');
    if (navBadgeFleet) navBadgeFleet.textContent = boxesOnline + '/' + boxesTotal;

    /* KPI 1: Clubs & Boxes (Online / Offline — concise, zero pill clipping) */
    var kpi1Card = document.getElementById('kpi-clubs-boxes');
    var kpi1Val = document.getElementById('kpi-clubs-boxes-val');
    var kpi1Sub = document.getElementById('kpi-clubs-boxes-sub');
    if (kpi1Card) kpi1Card.classList.toggle('danger', boxesOffline > 0);
    if (kpi1Val) {
      kpi1Val.innerHTML = esc(boxesOnline) + '<span class="kpi-val-sub">/ ' + esc(boxesTotal) + ' Online</span>';
    }
    if (kpi1Sub) {
      var offStatus = boxesOffline > 0
        ? '<span class="text-crimson" style="font-weight:700;">' + esc(boxesOffline) + ' Offline</span>'
        : '<span class="text-emerald" style="font-weight:700;">0 Offline</span>';
      kpi1Sub.innerHTML = '<span>' + esc(clubsTotal) + ' CLB</span>' + offStatus;
    }

    /* KPI 2: Cameras & FFmpeg */
    var camsTotal = kpi.cameras_total !== undefined ? kpi.cameras_total : boxes.reduce(function (acc, b) { return acc + (b.cameras_count || 0); }, 0);
    var ffActual = kpi.ffmpeg_total !== undefined ? kpi.ffmpeg_total : boxes.reduce(function (acc, b) { return acc + (b.ffmpeg_count || 0); }, 0);
    var ffExpTotal = kpi.ffmpeg_expected !== undefined ? kpi.ffmpeg_expected : boxes.reduce(function (acc, b) { return acc + (b.ffmpeg_expected || (b.cameras_count || 0) * 2); }, 0);
    var kpi2Val = document.getElementById('kpi-cameras-ffmpeg-val');
    var kpi2Sub = document.getElementById('kpi-cameras-ffmpeg-sub');
    if (kpi2Val) {
      kpi2Val.innerHTML = esc(fmtInt(camsTotal)) + '<span class="kpi-val-sub">Cam</span>';
    }
    if (kpi2Sub) {
      kpi2Sub.innerHTML = '<span>FFmpeg ' + esc(ffActual) + '/' + esc(ffExpTotal) + '</span><span>Kép: ' + esc(kpi.double_stream_count || 0) + '</span>';
    }

    var navBadgeCam = document.getElementById('nav-badge-cameras');
    if (navBadgeCam && state.cameras.length === 0 && camsTotal > 0) {
      navBadgeCam.textContent = String(camsTotal);
    }

    /* KPI 3: SD Health */
    var sdWarns = kpi.sd_warning_count !== undefined ? kpi.sd_warning_count : boxes.filter(function (b) { return (b.sd_used_pct || 0) >= 85; }).length;
    var avgSd = boxes.length ? Math.round(boxes.reduce(function (s, b) { return s + (b.sd_used_pct || 42); }, 0) / boxes.length) : 0;
    var kpi3Card = document.getElementById('kpi-sd-health');
    var kpi3Val = document.getElementById('kpi-sd-health-val');
    var kpi3Sub = document.getElementById('kpi-sd-health-sub');
    if (kpi3Card) kpi3Card.classList.toggle('warn', sdWarns > 0);
    if (kpi3Val) {
      kpi3Val.innerHTML = sdWarns === 0
        ? ('TB ' + esc(avgSd) + '%<span class="kpi-val-sub">· 0 đầy</span>')
        : ('<span class="text-amber">' + esc(sdWarns) + ' Box</span><span class="kpi-val-sub">&gt;85%</span>');
    }
    if (kpi3Sub) kpi3Sub.innerHTML = '<span>f2fs / ext4</span><span>errors=continue</span>';

    /* KPI 4: Hardening 8-Step */
    var fullHardened = kpi.hardening_full !== undefined ? kpi.hardening_full : boxes.filter(function (b) { return (b.hardening_score || 0) >= 8; }).length;
    var kpi4Card = document.getElementById('kpi-hardening');
    var kpi4Val = document.getElementById('kpi-hardening-val');
    var kpi4Sub = document.getElementById('kpi-hardening-sub');
    if (kpi4Card) kpi4Card.classList.toggle('warn', boxesTotal > 0 && fullHardened < boxesTotal);
    if (kpi4Val) kpi4Val.innerHTML = esc(fullHardened) + '/' + esc(boxesTotal) + '<span class="kpi-val-sub">8/8</span>';
    if (kpi4Sub) kpi4Sub.innerHTML = '<span>TB ' + esc(kpi.hardening_avg || '8.0') + '/8</span><span>0 Format</span>';
    var navBadgeHardening = document.getElementById('nav-badge-hardening');
    if (navBadgeHardening) navBadgeHardening.textContent = fullHardened + '/' + boxesTotal;

    /* KPI 5: Domains HTTP 200 & FRPC */
    var domOk = kpi.domains_ok !== undefined ? kpi.domains_ok : boxes.filter(function (b) { return (b.store_http_code || 200) === 200; }).length;
    var domConflict = kpi.domains_conflict || 0;
    var kpi5Card = document.getElementById('kpi-domains');
    var kpi5Val = document.getElementById('kpi-domains-val');
    var kpi5Sub = document.getElementById('kpi-domains-sub');
    if (kpi5Card) kpi5Card.classList.toggle('danger', domConflict > 0);
    if (kpi5Val) kpi5Val.innerHTML = esc(domOk) + '<span class="kpi-val-sub">HTTP 200</span>';
    if (kpi5Sub) {
      kpi5Sub.innerHTML = '<span>store-* / c-store-*</span><span class="' + (domConflict > 0 ? 'text-crimson' : 'text-emerald') + '">Xung đột: ' + esc(domConflict) + '</span>';
    }

    /* KPI 6: Average RTT */
    var onlineBoxes = boxes.filter(function (b) { return b.online && b.rtt_ms > 0; });
    var avgRtt = onlineBoxes.length ? Math.round(onlineBoxes.reduce(function (s, b) { return s + (b.rtt_ms || 18); }, 0) / onlineBoxes.length) : 0;
    var maxRtt = onlineBoxes.length ? Math.round(onlineBoxes.reduce(function (m, b) { return Math.max(m, b.rtt_ms || 0); }, 0)) : 0;
    var kpi6Val = document.getElementById('kpi-rtt-val');
    var kpi6Sub = document.getElementById('kpi-rtt-sub');
    if (kpi6Val) kpi6Val.innerHTML = esc(avgRtt) + '<span class="kpi-val-sub">ms TB</span>';
    if (kpi6Sub) kpi6Sub.innerHTML = '<span>Yamux · VPN 77.88.x</span><span>Max ' + esc(maxRtt) + 'ms</span>';

    /* Sync Compact Mobile KPI Bar + 42px Sticky Cockpit Bar (/device/86) */
    var onlinePct = boxesTotal > 0 ? Math.round((boxesOnline * 100) / boxesTotal) : 0;
    var mkpiDot = document.getElementById('mkpi-led-dot');
    var mkpiOnline = document.getElementById('mkpi-online');
    var mkpiClubs = document.getElementById('mkpi-clubs');
    var mkpiCams = document.getElementById('mkpi-cams');
    var mkpiHard = document.getElementById('mkpi-hardening');
    if (mkpiDot) mkpiDot.className = 'led-dot ' + (boxesOffline > 0 ? 'amber' : 'emerald');
    if (mkpiOnline) mkpiOnline.textContent = boxesOnline + '/' + boxesTotal + ' Online';
    if (mkpiClubs) mkpiClubs.textContent = clubsTotal + ' CLB';
    if (mkpiCams) mkpiCams.textContent = fmtInt(camsTotal) + ' Cam';
    if (mkpiHard) mkpiHard.textContent = fullHardened + '/' + boxesTotal + ' 8/8';

    var cDot = document.getElementById('cockpit-status-dot');
    var cRatio = document.getElementById('cockpit-online-ratio');
    var cPct = document.getElementById('cockpit-online-pct');
    var cOff = document.getElementById('cockpit-offline-pill');
    if (cDot) cDot.className = 'led-dot ' + (boxesOffline > 0 ? 'crimson' : 'emerald');
    if (cRatio) cRatio.textContent = boxesOnline + '/' + boxesTotal;
    if (cPct) cPct.textContent = '(' + onlinePct + '%)';
    if (cOff) {
      if (boxesOffline > 0) {
        cOff.style.display = 'inline-flex';
        cOff.textContent = boxesOffline + ' Off';
      } else {
        cOff.style.display = 'none';
      }
    }
  }

  function buildFleetRenderSignature(visibleBoxes, extraKey) {
    var parts = [extraKey];
    for (var i = 0; i < visibleBoxes.length; i++) {
      var b = visibleBoxes[i];
      parts.push([
        b.id || b.box_id || '',
        b.online ? '1' : '0',
        b.rtt_ms || 0,
        b.cpu_pct || 0,
        b.temp_c || 0,
        b.ram_used_mb || 0,
        b.ram_available_mb || 0,
        b.swap_used_mb || 0,
        b.sd_used_pct || 0,
        b.ffmpeg_count || 0,
        b.ffmpeg_expected || 0,
        b.hardening_score !== undefined ? b.hardening_score : 8,
        b.rx_mbps || 0,
        b.uptime_human || '',
        b.portal_token || ''
      ].join('|'));
    }
    return parts.join('\n');
  }

  function renderFleetActiveFilters(matchedCount) {
    var bar = document.getElementById('fleet-active-filters');
    if (!bar) return;
    var chips = [];
    var q = (state.fleetSearch || '').trim();
    var filterLabels = {
      online: 'Trực tuyến',
      offline: 'Mất kết nối',
      'ffmpeg-warn': 'Cảnh báo FFmpeg',
      'hardening-warn': 'Cần vá Golden',
      'multi-box': 'CLB Nhiều Box'
    };
    if (q) {
      chips.push('<span class="active-filter-chip"><span>Từ khóa: "' + esc(q) + '" (' + esc(fmtInt(matchedCount)) + ')</span><button type="button" data-remove-fleet-filter="search" aria-label="Bỏ từ khóa">' + svgIcon('x', 'ui-icon-sm') + '</button></span>');
    }
    if (state.fleetFilter && state.fleetFilter !== 'all') {
      chips.push('<span class="active-filter-chip"><span>Lọc: ' + esc(filterLabels[state.fleetFilter] || state.fleetFilter) + '</span><button type="button" data-remove-fleet-filter="status" aria-label="Bỏ lọc trạng thái">' + svgIcon('x', 'ui-icon-sm') + '</button></span>');
    }
    if (!chips.length) {
      bar.hidden = true;
      bar.innerHTML = '';
      return;
    }
    bar.hidden = false;
    bar.innerHTML = chips.join('') +
      '<button type="button" class="btn btn-xs" data-remove-fleet-filter="all">Xóa lọc</button>';
  }

  function syncFleetModeControls() {
    var cb = document.getElementById('group-by-club-toggle');
    if (cb) cb.checked = !!state.groupByClub;
    var modeWrap = document.getElementById('fleet-view-mode');
    if (modeWrap) {
      modeWrap.querySelectorAll('[data-fleet-mode]').forEach(function (el) {
        var mode = el.getAttribute('data-fleet-mode');
        el.classList.toggle('active', (mode === 'club') === !!state.groupByClub);
      });
    }
  }

  function renderFleetMatrix(opts) {
    var force = !!(opts && opts.force);
    var container = document.getElementById('fleet-matrix-container');
    if (!container) return;
    var boxes = state.boxes || [];

    var clubCounts = {};
    boxes.forEach(function (b) {
      var cid = b.club_id || b.club_name || 'default';
      clubCounts[cid] = (clubCounts[cid] || 0) + 1;
    });

    var countAll = boxes.length;
    var countOnline = 0;
    var countOffline = 0;
    var countFfmpeg = 0;
    var countHardening = 0;
    var countMultiBox = 0;

    boxes.forEach(function (b) {
      if (b.online) countOnline++; else countOffline++;
      var ffExp = b.ffmpeg_expected || (b.cameras_count ? b.cameras_count * 2 : 0);
      if (ffExp > 0 && (b.ffmpeg_count || 0) < ffExp) countFfmpeg++;
      if ((b.hardening_score !== undefined ? b.hardening_score : 8) < 8) countHardening++;
      var cid = b.club_id || b.club_name || 'default';
      if ((clubCounts[cid] || 0) > 1) countMultiBox++;
    });

    var setChipCount = function (id, val) {
      var el = document.getElementById(id);
      if (el) el.textContent = String(val);
    };
    setChipCount('chip-count-all', countAll);
    setChipCount('chip-count-online', countOnline);
    setChipCount('chip-count-offline', countOffline);
    setChipCount('chip-count-ffmpeg', countFfmpeg);
    setChipCount('chip-count-hardening', countHardening);
    setChipCount('chip-count-multibox', countMultiBox);

    var offBtn = document.getElementById('chip-btn-offline');
    var ffBtn = document.getElementById('chip-btn-ffmpeg');
    var hdBtn = document.getElementById('chip-btn-hardening');
    if (offBtn) offBtn.classList.toggle('has-alert-crimson', countOffline > 0);
    if (ffBtn) ffBtn.classList.toggle('has-alert-amber', countFfmpeg > 0);
    if (hdBtn) hdBtn.classList.toggle('has-alert-amber', countHardening > 0);

    var tokens = parseSearchQuery(state.fleetSearch);
    var filtered = boxes.filter(function (b) {
      var cid = b.club_id || b.club_name || 'default';
      var ffExp = b.ffmpeg_expected || (b.cameras_count ? b.cameras_count * 2 : 0);
      if (state.fleetFilter === 'online' && !b.online) return false;
      if (state.fleetFilter === 'offline' && b.online) return false;
      if (state.fleetFilter === 'ffmpeg-warn' && !(ffExp > 0 && (b.ffmpeg_count || 0) < ffExp)) return false;
      if (state.fleetFilter === 'hardening-warn' && !((b.hardening_score !== undefined ? b.hardening_score : 8) < 8)) return false;
      if (state.fleetFilter === 'multi-box' && !((clubCounts[cid] || 0) > 1)) return false;
      return matchesSearch(b, tokens, [
        b.id, b.name, b.club_name, b.club_id, b.vpn_ip, b.lan_ip,
        b.shinobi_group_key, b.store_domain, b.cstore_domain, b.soc_model, b.branding
      ]);
    });

    syncSearchBoxState('fleet-search', 'fleet-match-count', filtered.length, boxes.length);
    renderFleetActiveFilters(filtered.length);
    syncFleetModeControls();

    if (filtered.length === 0) {
      var emptySig = 'EMPTY|' + state.fleetFilter + '|' + state.fleetSearch;
      if (!force && state.lastFleetRenderSig === emptySig) return;
      state.lastFleetRenderSig = emptySig;
      container.innerHTML = '<div class="glass-panel"><div class="empty-state">' +
        '<div class="empty-state-icon">' + svgIcon('search') + '</div>' +
        '<div><strong>Không có Box nào khớp bộ lọc hiện tại.</strong></div>' +
        '<div class="text-muted" style="font-size:12.5px;">Thử xóa từ khóa tìm kiếm hoặc chọn bộ lọc "Tất cả".</div>' +
        ((state.fleetSearch || state.fleetFilter !== 'all')
          ? '<button type="button" class="btn btn-xs btn-cyan" data-remove-fleet-filter="all">Đặt lại bộ lọc</button>'
          : '') +
      '</div></div>';
      return;
    }

    var hasActiveFilter = !!(tokens && tokens.length) || state.fleetFilter !== 'all';

    if (state.groupByClub) {
      var groupsMap = {};
      var groupOrder = [];
      filtered.forEach(function (b) {
        var cid = b.club_id || b.club_name || 'club';
        if (!groupsMap[cid]) {
          groupsMap[cid] = {
            club_id: cid,
            club_name: b.club_name || cid,
            branding: b.branding || 'billiards',
            portal_token: b.portal_token || '',
            boxes: []
          };
          groupOrder.push(groupsMap[cid]);
        }
        groupsMap[cid].boxes.push(b);
      });

      /* Club-aware pagination: slice whole clubs so a club is NEVER split across pages */
      var pg = paginateItems(groupOrder, 'fleet');
      var pageGroups = pg.items;
      var visibleBoxes = [];
      var collapsedFlags = [];
      var expandedCount = 0;

      pageGroups.forEach(function (grp) {
        var isCollapsed = !hasActiveFilter && !!state.collapsedClubs[grp.club_id];
        if (!isCollapsed) expandedCount++;
        collapsedFlags.push(grp.club_id + ':' + (isCollapsed ? '0' : '1'));
        grp.boxes.forEach(function (b) { visibleBoxes.push(b); });
      });

      var expandedFlags = visibleBoxes.filter(function (b) {
        return !!state.expandedMobileBoxIds[b.id || b.box_id || ''];
      }).map(function (b) { return b.id || b.box_id || ''; });

      var sig = buildFleetRenderSignature(visibleBoxes, [
        'CLUB', state.fleetFilter, state.fleetSearch, pg.page, pg.pageSize,
        filtered.length, groupOrder.length, collapsedFlags.join(','), expandedFlags.join(',')
      ].join('|'));

      if (!force && sig === state.lastFleetRenderSig) return;
      state.lastFleetRenderSig = sig;

      var anyRowExpanded = expandedFlags.length > 0;
      var anyClubOpen = expandedCount > 0;
      var ledgerHtml = '<div class="club-ledger-strip">' +
        '<div class="club-ledger-meta">' +
          '<span><strong>' + esc(fmtInt(groupOrder.length)) + '</strong> CLB</span>' +
          '<span class="mkpi-sep">·</span>' +
          '<span><strong>' + esc(fmtInt(filtered.length)) + '</strong> Box</span>' +
          '<span class="ledger-desktop-only"><span class="mkpi-sep">·</span> <span>Đang mở <strong>' + esc(expandedCount) + '/' + esc(pageGroups.length) + '</strong> CLB trên trang</span></span>' +
        '</div>' +
        '<div class="toolbar-group desktop-only-inline">' +
          '<button type="button" class="btn btn-xs" data-clubs-action="expand-all">Mở toàn bộ</button>' +
          '<button type="button" class="btn btn-xs" data-clubs-action="collapse-all">Thu gọn</button>' +
        '</div>' +
        '<div class="mobile-only-inline">' +
          '<button type="button" class="btn btn-xs ledger-mini-btn" data-mobile-rows-action="toggle-all">' + (anyRowExpanded ? 'Thu dòng' : 'Mở dòng') + '</button>' +
          '<button type="button" class="btn btn-xs ledger-mini-btn" data-clubs-action="' + (anyClubOpen ? 'collapse-all' : 'expand-all') + '">' + (anyClubOpen ? 'Thu CLB' : 'Mở CLB') + '</button>' +
        '</div>' +
      '</div>';

      var groupsHtml = pageGroups.map(function (grp) {
        var isPickle = String(grp.branding || '').toLowerCase() === 'pickleball' || grp.club_name.toLowerCase().indexOf('pickle') !== -1;
        var brandBadge = isPickle
          ? '<span class="badge badge-amber club-badge-desktop">' + svgIcon('pickle', 'ui-icon-sm') + '<span>Pickleball</span></span>'
          : '<span class="badge badge-cyan club-badge-desktop">' + svgIcon('pool', 'ui-icon-sm') + '<span>Billiards</span></span>';
        var mobileBrandIcon = isPickle
          ? '<span class="club-brand-dot-mobile text-amber" title="Pickleball">' + svgIcon('pickle', 'ui-icon-sm') + '</span>'
          : '';
        var onlineInClub = grp.boxes.filter(function (x) { return x.online; }).length;
        var offlineInClub = grp.boxes.length - onlineInClub;
        var multiBadge = grp.boxes.length > 1
          ? '<span class="badge badge-purple club-badge-desktop">' + grp.boxes.length + ' Box (' + onlineInClub + '/' + grp.boxes.length + ' Online)</span>'
          : '<span class="badge club-badge-desktop ' + (onlineInClub > 0 ? 'badge-emerald' : 'badge-crimson') + '">' + (onlineInClub > 0 ? '1/1 Online' : '0/1 Offline') + '</span>';
        var clubOffBadge = offlineInClub > 0
          ? '<span class="badge badge-crimson club-badge-desktop">' + offlineInClub + ' Offline</span>'
          : '';
        var mobileCountTone = offlineInClub > 0 ? 'badge-crimson' : (grp.boxes.length > 1 ? 'badge-purple' : 'badge-emerald');
        var mobileCountBadge = '<span class="badge club-count-mobile ' + mobileCountTone + '">' + esc(onlineInClub) + '/' + esc(grp.boxes.length) + '</span>';
        var portalBtn = grp.portal_token
          ? '<a href="/portal/' + encodeURIComponent(grp.portal_token) + '/" target="_blank" rel="noopener" class="btn btn-xs club-portal-btn" title="Mở Cổng Chủ Quán: ' + esc(grp.club_name) + '" aria-label="Cổng Chủ Quán ' + esc(grp.club_name) + '">' + svgIcon('key', 'ui-icon-sm') + '<span class="club-portal-label">Cổng Chủ Quán</span></a>'
          : '';
        var isCollapsed = !hasActiveFilter && !!state.collapsedClubs[grp.club_id];

        return '<div class="club-section' + (isCollapsed ? ' is-collapsed' : '') + '" data-club-id="' + esc(grp.club_id) + '">' +
          '<div class="club-header is-collapsible" data-toggle-club="' + esc(grp.club_id) + '" role="button" tabindex="0" aria-expanded="' + (!isCollapsed ? 'true' : 'false') + '">' +
            '<div class="club-title-group">' +
              '<span class="club-chevron" aria-hidden="true">' + svgIcon('chevronDown', 'ui-icon-sm') + '</span>' +
              mobileBrandIcon +
              '<h3 class="club-title">' + esc(grp.club_name) + '</h3>' +
              brandBadge +
              multiBadge +
              clubOffBadge +
            '</div>' +
            '<div class="club-header-right">' +
              mobileCountBadge +
              portalBtn +
            '</div>' +
          '</div>' +
          '<div class="club-body">' +
            renderBoxesTableHtml(grp.boxes) +
            renderBoxesMobileCardsHtml(grp.boxes, true) +
          '</div>' +
        '</div>';
      }).join('');

      var extraSuffix = '(' + fmtInt(visibleBoxes.length) + ' / ' + fmtInt(filtered.length) + ' Box)';
      container.innerHTML = ledgerHtml + groupsHtml + renderPaginationBar('fleet', pg, 'CLB', extraSuffix);
    } else {
      var pgFlat = paginateItems(filtered, 'fleet');
      var flatBoxes = pgFlat.items;
      var flatExpandedFlags = flatBoxes.filter(function (b) {
        return !!state.expandedMobileBoxIds[b.id || b.box_id || ''];
      }).map(function (b) { return b.id || b.box_id || ''; });
      var flatSig = buildFleetRenderSignature(flatBoxes, [
        'FLAT', state.fleetFilter, state.fleetSearch, pgFlat.page, pgFlat.pageSize, filtered.length, flatExpandedFlags.join(',')
      ].join('|'));

      if (!force && flatSig === state.lastFleetRenderSig) return;
      state.lastFleetRenderSig = flatSig;

      var flatLedgerHtml = '<div class="club-ledger-strip mobile-only-ledger">' +
        '<div class="club-ledger-meta">' +
          '<span><strong>' + esc(fmtInt(filtered.length)) + '</strong> Box</span>' +
          '<span class="mkpi-sep">·</span>' +
          '<span>Trang ' + esc(pgFlat.page) + '/' + esc(pgFlat.totalPages) + '</span>' +
        '</div>' +
        '<div class="mobile-only-inline">' +
          '<button type="button" class="btn btn-xs ledger-mini-btn" data-mobile-rows-action="toggle-all">' + (flatExpandedFlags.length > 0 ? 'Thu dòng' : 'Mở dòng') + '</button>' +
        '</div>' +
      '</div>';

      container.innerHTML =
        flatLedgerHtml +
        '<div class="club-section">' +
          renderBoxesTableHtml(flatBoxes) +
          renderBoxesMobileCardsHtml(flatBoxes, false) +
        '</div>' +
        renderPaginationBar('fleet', pgFlat, 'Box');
    }
  }

  /* 7-column consolidated Fleet table — Desktop (>=768px) */
  function renderBoxesTableHtml(boxList) {
    var rows = boxList.map(function (b) {
      var boxId = b.id || b.box_id || '';
      var isOnline = !!b.online;
      var rowClass = 'clickable-row' + (isOnline ? '' : ' row-offline');
      var onlineBadge = isOnline
        ? '<span class="badge badge-emerald emerald"><span class="led-dot led emerald"></span>ONLINE · ' + esc(b.rtt_ms || 0) + 'ms</span>'
        : '<span class="badge badge-crimson crimson"><span class="led-dot led crimson"></span>OFFLINE</span>';
      var uptimeSub = '<div class="cell-sub mono">' + escapeHtml(b.uptime_human || (isOnline ? '--' : 'Mất kết nối')) + '</div>';

      var cpuPctVal = Number(b.cpu_pct || 0);
      var cpuBarClass = cpuPctVal >= 85 ? 'danger' : (cpuPctVal >= 65 ? 'warn' : 'ok');
      var tempC = b.temp_c || 54;
      var cpuCores = b.cpu_cores || 4;

      var ramTotal = b.ram_total_mb || 1945;
      var ramUsed = b.ram_used_mb || (b.ram_available_mb > 0 && ramTotal > b.ram_available_mb ? (ramTotal - b.ram_available_mb) : 0);
      var ramAvail = b.ram_available_mb || Math.max(0, ramTotal - ramUsed);
      var ramPct = b.ram_used_pct || b.ram_pct || (ramTotal > 0 ? Math.round((ramUsed * 100) / ramTotal) : 0);
      var ramBarClass = ramPct >= 88 ? 'danger' : (ramPct >= 75 ? 'warn' : 'ok');

      var swapTotal = b.swap_total_mb || b.zram_total_mb || 0;
      var swapUsed = b.swap_used_mb !== undefined ? b.swap_used_mb : (b.zram_used_mb || 0);
      var swapFree = b.swap_free_mb || Math.max(0, swapTotal - swapUsed);
      var zramPct = b.zram_pct || (swapTotal > 0 ? Math.round((swapUsed * 100) / swapTotal) : 0);

      var sdPct = b.sd_used_pct || 38;
      var sdFs = b.sd_fs_type || 'f2fs';
      var sdTone = sdPct >= 88 ? 'text-crimson' : (sdPct >= 75 ? 'text-amber' : 'text-emerald');

      var ffAct = b.ffmpeg_count !== undefined ? b.ffmpeg_count : (b.cameras_count || 6) * 2;
      var ffExp = b.ffmpeg_expected !== undefined ? b.ffmpeg_expected : (b.cameras_count || 6) * 2;
      var ffBadge = (ffAct >= ffExp && ffExp > 0)
        ? '<span class="badge badge-emerald">FF ' + esc(ffAct) + '/' + esc(ffExp) + '</span>'
        : '<span class="badge badge-amber">FF ' + esc(ffAct) + '/' + esc(ffExp) + '</span>';

      var hScore = b.hardening_score !== undefined ? b.hardening_score : 8;
      var hMax = b.hardening_max || 8;
      var hBadge = hScore >= hMax
        ? '<span class="badge badge-emerald">' + svgIcon('shield', 'ui-icon-sm') + '<span>' + esc(hScore) + '/' + esc(hMax) + '</span></span>'
        : '<span class="badge badge-amber">' + svgIcon('warn', 'ui-icon-sm') + '<span>' + esc(hScore) + '/' + esc(hMax) + '</span></span>';

      var rxMbps = b.rx_mbps !== undefined ? Number(b.rx_mbps).toFixed(1) : '0.0';

      return '<tr class="' + rowClass + '" data-open-box="' + esc(boxId) + '">' +
        '<td data-label="Trạm / CLB">' +
          '<div class="cell-primary mono">' + esc(boxId) + '</div>' +
          '<div class="cell-sub">' + esc(b.name || b.club_name || '') + '</div>' +
        '</td>' +
        '<td data-label="Mạng (VPN / LAN)">' +
          '<div class="mono cell-primary">' + esc(b.vpn_ip || '--') + '</div>' +
          '<div class="cell-sub mono">LAN ' + esc(b.lan_ip || '--') + ' · grp ' + esc(b.shinobi_group_key || '--') + '</div>' +
        '</td>' +
        '<td data-label="Trạng Thái">' +
          '<div>' + onlineBadge + '</div>' +
          uptimeSub +
        '</td>' +
        '<td data-label="CPU / Nhiệt">' +
          '<div class="mono"><strong>' + fmtNum(b.cpu_pct, 1) + '%</strong> · ' + fmtNum(tempC, 1) + '°C</div>' +
          '<div class="mini-meter"><div class="mini-meter-fill ' + cpuBarClass + '" style="width:' + Math.min(100, Math.max(0, cpuPctVal)) + '%"></div></div>' +
          '<div class="cell-sub mono">Load ' + fmtNum(b.load_avg_1m, 2) + ' (' + esc(cpuCores) + 'C) · ' + esc(b.soc_model || 'ARM64') + '</div>' +
        '</td>' +
        '<td data-label="RAM / Swap / SD">' +
          '<div class="mono"><strong>RAM ' + esc(ramUsed) + '/' + esc(ramTotal) + 'M (' + esc(ramPct) + '%)</strong> · Avail ' + esc(ramAvail) + 'M</div>' +
          '<div class="mini-meter"><div class="mini-meter-fill ' + ramBarClass + '" style="width:' + Math.min(100, Math.max(0, ramPct)) + '%"></div></div>' +
          '<div class="cell-sub mono">Heap ' + fmtNum(b.heap_alloc_mb, 1) + 'M · Swap ' + esc(swapUsed) + '/' + esc(swapTotal) + 'M (Free ' + esc(swapFree) + 'M · ' + esc(zramPct) + '%)</div>' +
          '<div class="cell-sub mono">SD <span class="' + sdTone + '" style="font-weight:700;">' + esc(sdPct) + '%</span> (' + esc(b.sd_total_gb || 64) + 'G ' + esc(sdFs) + ') · ksp ' + fmtNum(b.kspcam_rss_mb, 0) + 'M · cam ' + fmtNum(b.camera_rss_mb, 0) + 'M</div>' +
        '</td>' +
        '<td data-label="FFmpeg & Golden">' +
          '<div class="badge-row">' + ffBadge + hBadge + '</div>' +
          '<div class="cell-sub mono">RX ' + esc(rxMbps) + ' Mbps</div>' +
        '</td>' +
        '<td data-label="Thao Tác">' +
          '<div class="action-btn-row">' +
            '<button type="button" class="btn btn-xs btn-cyan" data-open-box="' + esc(boxId) + '">Chi tiết</button>' +
            '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/kspcam/" target="_blank" rel="noopener" class="btn btn-xs">kspcam</a>' +
            '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/shinobi/" target="_blank" rel="noopener" class="btn btn-xs">Shinobi</a>' +
            '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/nodered/" target="_blank" rel="noopener" class="btn btn-xs">Node-RED</a>' +
          '</div>' +
        '</td>' +
      '</tr>';
    }).join('');

    return '<div class="table-scroll desktop-only-table"><table class="scada-table fleet-table tabular-nums">' +
      '<thead><tr>' +
        '<th>Trạm / CLB</th>' +
        '<th>Mạng (VPN / LAN)</th>' +
        '<th>Trạng Thái</th>' +
        '<th>CPU / Nhiệt</th>' +
        '<th>RAM / Swap / SD</th>' +
        '<th>FFmpeg &amp; Golden</th>' +
        '<th>Thao Tác</th>' +
      '</tr></thead>' +
      '<tbody>' + rows + '</tbody>' +
    '</table></div>';
  }

  /* Compact Scan-First Expandable Accordion Rows — Mobile (<768px) */
  function formatCompactBoxName(b, isGrouped) {
    var boxId = b.id || b.box_id || '';
    var rawName = String(b.name || '').trim();
    var clubName = String(b.club_name || '').trim();
    if (!rawName) return clubName || boxId;
    if (isGrouped && clubName && rawName.length > clubName.length && rawName.indexOf(clubName) === 0) {
      var trimmed = rawName.slice(clubName.length).replace(/^[\s\-—–·:]+/, '').trim();
      if (trimmed) return trimmed;
    }
    return rawName;
  }

  function renderBoxesMobileCardsHtml(boxList, isGrouped) {
    var cards = boxList.map(function (b) {
      var boxId = b.id || b.box_id || '';
      var isOnline = !!b.online;
      var isExpanded = !!state.expandedMobileBoxIds[boxId];
      var cpuPctVal = Number(b.cpu_pct || 0);
      var cpuBarClass = cpuPctVal >= 85 ? 'danger' : (cpuPctVal >= 65 ? 'warn' : 'ok');
      var tempC = Number(b.temp_c || 54);
      var cpuCores = b.cpu_cores || 4;

      var ramTotal = b.ram_total_mb || 1945;
      var ramUsed = b.ram_used_mb || (b.ram_available_mb > 0 && ramTotal > b.ram_available_mb ? (ramTotal - b.ram_available_mb) : 0);
      var ramAvail = b.ram_available_mb || Math.max(0, ramTotal - ramUsed);
      var ramPct = b.ram_used_pct || b.ram_pct || (ramTotal > 0 ? Math.round((ramUsed * 100) / ramTotal) : 0);
      var ramBarClass = ramPct >= 88 ? 'danger' : (ramPct >= 75 ? 'warn' : 'ok');

      var swapTotal = b.swap_total_mb || b.zram_total_mb || 0;
      var swapUsed = b.swap_used_mb !== undefined ? b.swap_used_mb : (b.zram_used_mb || 0);
      var sdPct = Number(b.sd_used_pct || 38);
      var sdFs = b.sd_fs_type || 'f2fs';
      var sdTone = sdPct >= 88 ? 'text-crimson' : (sdPct >= 75 ? 'text-amber' : 'text-emerald');

      var ffAct = b.ffmpeg_count !== undefined ? b.ffmpeg_count : (b.cameras_count || 6) * 2;
      var ffExp = b.ffmpeg_expected !== undefined ? b.ffmpeg_expected : (b.cameras_count || 6) * 2;
      var ffOk = ffAct >= ffExp && ffExp > 0;
      var hScore = b.hardening_score !== undefined ? b.hardening_score : 8;
      var hMax = b.hardening_max || 8;
      var hOk = hScore >= hMax;
      var rxMbps = b.rx_mbps !== undefined ? Number(b.rx_mbps).toFixed(1) : '0.0';
      var camCount = b.cameras_count || 0;
      var rttMs = Number(b.rtt_ms || 0);

      /* Threshold-only tinting on Line 2 so anomalies pop immediately while scanning 8-15 nodes */
      var rttCls = rttMs >= 300 ? 'text-crimson mrow-alert-txt' : (rttMs >= 150 ? 'text-amber mrow-alert-txt' : '');
      var ramCls = ramPct >= 88 ? 'text-crimson mrow-alert-txt' : (ramPct >= 75 ? 'text-amber mrow-alert-txt' : '');
      var sdCls = sdPct >= 88 ? 'text-crimson mrow-alert-txt' : (sdPct >= 75 ? 'text-amber mrow-alert-txt' : '');
      var tempCls = tempC >= 75 ? 'text-crimson mrow-alert-txt' : (tempC >= 65 ? 'text-amber mrow-alert-txt' : '');

      var displayName = formatCompactBoxName(b, isGrouped);

      var metricsLine = isOnline
        ? ('<span class="mrow-sep">·</span>' +
           '<span class="' + rttCls + '">' + esc(Math.round(rttMs)) + 'ms</span>' +
           '<span class="mrow-sep">·</span>' +
           '<span class="' + ramCls + '">RAM ' + esc(ramPct) + '%</span>' +
           '<span class="mrow-sep">·</span>' +
           '<span class="' + sdCls + '">SD ' + esc(Math.round(sdPct)) + '%</span>' +
           '<span class="mrow-sep">·</span>' +
           '<span class="' + tempCls + '">' + esc(Math.round(tempC)) + '°C</span>')
        : ('<span class="mrow-sep">·</span>' +
           '<span class="text-crimson mrow-alert-txt">OFFLINE</span>' +
           '<span class="mrow-sep">·</span>' +
           '<span>' + esc(b.vpn_ip || '--') + '</span>' +
           '<span class="mrow-sep">·</span>' +
           '<span class="' + sdCls + '">SD ' + esc(Math.round(sdPct)) + '%</span>');

      var portalAction = b.portal_token
        ? '<a href="/portal/' + encodeURIComponent(b.portal_token) + '/" target="_blank" rel="noopener" class="btn mrow-act-btn">' + svgIcon('key', 'ui-icon-sm text-amber') + '<span>Cổng CQ</span></a>'
        : '';

      return '<article class="fleet-mcard mobile-box-card' + (isOnline ? '' : ' is-offline') + (isExpanded ? ' is-expanded' : '') + '" data-card-box="' + esc(boxId) + '" data-compact-row="fleet">' +
        '<div class="mrow-head" data-toggle-mrow="fleet" data-mrow-id="' + esc(boxId) + '" role="button" tabindex="0" aria-expanded="' + (isExpanded ? 'true' : 'false') + '">' +
          '<div class="mrow-line1">' +
            '<div class="mrow-primary">' +
              '<span class="led-dot ' + (isOnline ? 'emerald' : 'crimson') + '"></span>' +
              '<span class="mrow-name">' + esc(displayName) + '</span>' +
            '</div>' +
            '<div class="mrow-pills">' +
              (!isOnline ? '<span class="mrow-pill pill-crimson">OFF</span>' : '') +
              '<span class="mrow-pill ' + (ffOk ? 'pill-ok' : 'pill-amber') + '">FF ' + esc(ffAct) + '/' + esc(ffExp) + '</span>' +
              '<span class="mrow-pill ' + (hOk ? 'pill-quiet' : 'pill-amber') + '">' + esc(hScore) + '/' + esc(hMax) + '</span>' +
              '<span class="mrow-chevron" aria-hidden="true">' + svgIcon('chevronDown', 'ui-icon-sm') + '</span>' +
            '</div>' +
          '</div>' +
          '<div class="mrow-line2 mono">' +
            '<span class="mrow-id">' + esc(boxId) + '</span>' +
            '<span class="mrow-metrics">' + metricsLine + '</span>' +
          '</div>' +
        '</div>' +

        '<div class="mrow-body">' +
          '<div class="mcard-tel-grid">' +
            '<div class="mcard-tel-cell">' +
              '<span class="mcard-tel-label">RAM</span>' +
              '<span class="mcard-tel-val mono">' + esc(ramPct) + '%</span>' +
              '<div class="mini-meter" style="max-width:100%;"><div class="mini-meter-fill ' + ramBarClass + '" style="width:' + Math.min(100, Math.max(0, ramPct)) + '%"></div></div>' +
              '<span class="mcard-tel-sub mono">' + esc(ramUsed) + '/' + esc(ramTotal) + 'M · Avail ' + esc(ramAvail) + 'M</span>' +
            '</div>' +
            '<div class="mcard-tel-cell">' +
              '<span class="mcard-tel-label">CPU / Nhiệt</span>' +
              '<span class="mcard-tel-val mono">' + fmtNum(b.cpu_pct, 1) + '% · ' + fmtNum(tempC, 1) + '°C</span>' +
              '<div class="mini-meter" style="max-width:100%;"><div class="mini-meter-fill ' + cpuBarClass + '" style="width:' + Math.min(100, Math.max(0, cpuPctVal)) + '%"></div></div>' +
              '<span class="mcard-tel-sub mono">Load ' + fmtNum(b.load_avg_1m, 2) + ' (' + esc(cpuCores) + 'C)</span>' +
            '</div>' +
            '<div class="mcard-tel-cell">' +
              '<span class="mcard-tel-label">SD / Swap</span>' +
              '<span class="mcard-tel-val mono"><span class="' + sdTone + '">SD ' + esc(Math.round(sdPct)) + '%</span> · ' + esc(b.sd_total_gb || 64) + 'G ' + esc(sdFs) + '</span>' +
              '<span class="mcard-tel-sub mono">Swap ' + esc(swapUsed) + '/' + esc(swapTotal) + 'M</span>' +
            '</div>' +
            '<div class="mcard-tel-cell">' +
              '<span class="mcard-tel-label">Lưu lượng / Heap</span>' +
              '<span class="mcard-tel-val mono">RX ' + esc(rxMbps) + ' Mbps</span>' +
              '<span class="mcard-tel-sub mono">Heap ' + fmtNum(b.heap_alloc_mb, 1) + 'M · ' + esc(camCount) + ' Cam</span>' +
            '</div>' +
          '</div>' +

          '<div class="mcard-net-strip">' +
            '<span class="mcard-net-chip">VPN ' + esc(b.vpn_ip || '--') + '</span>' +
            '<span class="mcard-net-chip">LAN ' + esc(b.lan_ip || '--') + '</span>' +
            '<span class="mcard-net-chip">grp ' + esc(b.shinobi_group_key || '--') + '</span>' +
            '<span class="mcard-net-chip">' + esc(b.soc_model || 'ARM64') + ' · ' + esc(b.uptime_human || (isOnline ? '--' : 'Offline')) + '</span>' +
          '</div>' +

          '<div class="mrow-actions">' +
            '<button type="button" class="btn btn-cyan mrow-act-btn" data-open-box="' + esc(boxId) + '">Chi tiết</button>' +
            '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/kspcam/" target="_blank" rel="noopener" class="btn mrow-act-btn">kspcam</a>' +
            '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/shinobi/" target="_blank" rel="noopener" class="btn mrow-act-btn">Shinobi</a>' +
            '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/nodered/" target="_blank" rel="noopener" class="btn mrow-act-btn">Node-RED</a>' +
            portalAction +
          '</div>' +
        '</div>' +
      '</article>';
    }).join('');

    return '<div class="mobile-card-list compact-list mobile-only-cards">' + cards + '</div>';
  }

  /* --------------------------------------------------------------------------
     View 2: #box/{box_id} — Slide-over Box Detail Drawer (#box-drawer)
     -------------------------------------------------------------------------- */
  function openBoxDrawer(boxId, updateHash) {
    if (!boxId) return;
    state.drawerBoxId = boxId;
    if (updateHash !== false) {
      history.replaceState(null, '', '#box/' + encodeURIComponent(boxId));
    }
    var drawer = document.getElementById('box-drawer');
    var backdrop = document.getElementById('box-drawer-backdrop');
    if (drawer) drawer.classList.add('open');
    if (backdrop) backdrop.classList.add('open');

    var titleEl = document.getElementById('drawer-box-title');
    var subEl = document.getElementById('drawer-box-sub');
    if (titleEl) titleEl.textContent = 'Đang tải ' + boxId + '...';
    if (subEl) subEl.textContent = 'GET /fleet/api/boxes/' + boxId;

    apiFetch('/fleet/api/boxes/' + encodeURIComponent(boxId)).then(function (data) {
      renderBoxDrawerContent(boxId, data);
    }).catch(function (err) {
      showToast('Không thể tải chi tiết Box ' + boxId + ': ' + err.message, true);
    });
  }

  function closeBoxDrawer() {
    state.drawerBoxId = null;
    var drawer = document.getElementById('box-drawer');
    var backdrop = document.getElementById('box-drawer-backdrop');
    if (drawer) drawer.classList.remove('open');
    if (backdrop) backdrop.classList.remove('open');
    if (window.location.hash.indexOf('#box/') === 0) {
      history.replaceState(null, '', '#' + (state.activeView || 'fleet'));
    }
  }

  function renderBoxDrawerContent(boxId, data) {
    var b = data.box || data || {};
    var club = data.club || {};
    var hardening = data.hardening || {};
    var monitors = Array.isArray(data.monitors) ? data.monitors : [];
    var cameras = Array.isArray(data.cameras) ? data.cameras : [];
    var isOnline = b.online !== false;

    var titleEl = document.getElementById('drawer-box-title');
    var subEl = document.getElementById('drawer-box-sub');
    var statusBadge = document.getElementById('drawer-box-status-badge');
    var socBadge = document.getElementById('drawer-soc-badge');
    if (titleEl) titleEl.textContent = (b.name || boxId) + ' (' + boxId + ')';
    if (subEl) subEl.textContent = 'CLB: ' + (b.club_name || club.name || '--') + ' · VPN: ' + (b.vpn_ip || '--') + ' · LAN: ' + (b.lan_ip || '--') + ' · Group: ' + (b.shinobi_group_key || '--');
    if (statusBadge) {
      statusBadge.className = 'badge ' + (isOnline ? 'badge-emerald emerald' : 'badge-crimson crimson');
      statusBadge.innerHTML = '<span class="led-dot led ' + (isOnline ? 'emerald' : 'crimson') + '"></span>' +
        (isOnline ? ('ONLINE · ' + esc(b.rtt_ms || 0) + 'ms') : 'OFFLINE');
    }
    if (socBadge) socBadge.textContent = (b.soc_model || 'ARM64') + ' · ' + (b.arch || 'aarch64');

    /* 1-Click Reverse Proxy Launchers */
    var proxyEl = document.getElementById('drawer-proxy-links');
    if (proxyEl) {
      var portalToken = b.portal_token || club.portal_token || '';
      proxyEl.innerHTML =
        '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/kspcam/" target="_blank" rel="noopener" class="btn btn-primary">' + svgIcon('sliders', 'ui-icon-sm') + '<span>Mở kspcam (:2028 SSO)</span></a>' +
        '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/shinobi/" target="_blank" rel="noopener" class="btn btn-cyan">' + svgIcon('video', 'ui-icon-sm') + '<span>Mở Shinobi (:8080)</span></a>' +
        '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/nodered/" target="_blank" rel="noopener" class="btn">' + svgIcon('zap', 'ui-icon-sm') + '<span>Mở Node-RED (:1880)</span></a>' +
        (portalToken ? '<a href="/portal/' + encodeURIComponent(portalToken) + '/" target="_blank" rel="noopener" class="btn btn-amber">' + svgIcon('key', 'ui-icon-sm') + '<span>Cổng Chủ Quán</span></a>' : '');
    }

    /* Real-Time System Resources (On/Off · CPU · RAM · Heap · Avail · Swap) */
    var telEl = document.getElementById('drawer-telemetry');
    if (telEl) {
      var cpuPctVal = Number(b.cpu_pct || 0);
      var cpuBarClass = cpuPctVal >= 85 ? 'danger' : (cpuPctVal >= 65 ? 'warn' : 'ok');

      var ramTotal = b.ram_total_mb || 1945;
      var ramUsed = b.ram_used_mb || (b.ram_available_mb > 0 && ramTotal > b.ram_available_mb ? (ramTotal - b.ram_available_mb) : 0);
      var ramAvail = b.ram_available_mb || Math.max(0, ramTotal - ramUsed);
      var ramPct = b.ram_used_pct || b.ram_pct || (ramTotal > 0 ? Math.round((ramUsed * 100) / ramTotal) : 0);
      var ramBarClass = ramPct >= 88 ? 'danger' : (ramPct >= 75 ? 'warn' : 'ok');

      var swapTotal = b.swap_total_mb || b.zram_total_mb || 0;
      var swapUsed = b.swap_used_mb !== undefined ? b.swap_used_mb : (b.zram_used_mb || 0);
      var swapFree = b.swap_free_mb || Math.max(0, swapTotal - swapUsed);
      var zramPct = b.zram_pct || (swapTotal > 0 ? Math.round((swapUsed * 100) / swapTotal) : 0);

      var sdPct = b.sd_used_pct || 38;
      var sdBarClass = sdPct >= 88 ? 'danger' : (sdPct >= 75 ? 'warn' : 'ok');
      var telSource = b.telemetry_source || 'live';

      telEl.innerHTML =
        '<div class="telemetry-item">' +
          '<div class="telemetry-item-label">Kết nối, RTT &amp; Uptime</div>' +
          '<div class="telemetry-item-val">' +
            (isOnline
              ? '<span class="badge badge-emerald emerald"><span class="led-dot led emerald"></span>ONLINE</span> <span class="mono" style="margin-left:6px;">' + esc(b.rtt_ms || 0) + ' ms</span>'
              : '<span class="badge badge-crimson crimson"><span class="led-dot led crimson"></span>OFFLINE</span> <span style="margin-left:6px;">Mất kết nối</span>') +
          '</div>' +
          '<div class="mono telemetry-item-meta">Uptime: ' + esc(b.uptime_human || '--') + ' · Nguồn: ' + esc(telSource) + '</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div class="telemetry-item-label">CPU, Load Average &amp; Nhiệt độ SoC</div>' +
          '<div class="telemetry-item-val mono">CPU ' + fmtNum(b.cpu_pct, 1) + '% · ' + fmtNum(b.temp_c || 54, 1) + '°C</div>' +
          '<div class="mini-meter" style="max-width:100%;"><div class="mini-meter-fill ' + cpuBarClass + '" style="width:' + Math.min(100, Math.max(0, cpuPctVal)) + '%"></div></div>' +
          '<div class="mono telemetry-item-meta">Load ' + fmtNum(b.load_avg_1m, 2) + ' (' + esc(b.cpu_cores || 4) + ' nhân) · ' + esc(b.soc_model || 'ARM64') + '</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div class="telemetry-item-label">Bộ nhớ RAM (Used / Total / Available)</div>' +
          '<div class="telemetry-item-val mono">' + esc(ramUsed) + ' / ' + esc(ramTotal) + ' MB (' + esc(ramPct) + '%)</div>' +
          '<div class="mini-meter" style="max-width:100%;"><div class="mini-meter-fill ' + ramBarClass + '" style="width:' + Math.min(100, Math.max(0, ramPct)) + '%"></div></div>' +
          '<div class="mono telemetry-item-meta text-emerald">RAM Available: ' + esc(ramAvail) + ' MB</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div class="telemetry-item-label">Go Runtime Heap &amp; Tiến trình</div>' +
          '<div class="telemetry-item-val mono">Heap ' + fmtNum(b.heap_alloc_mb, 1) + ' / ' + fmtNum(b.heap_sys_mb, 1) + ' MB · ' + esc(b.goroutines || 0) + ' gr</div>' +
          '<div class="mono telemetry-item-meta text-cyan">kspcam RSS: ' + fmtNum(b.kspcam_rss_mb, 1) + ' MB · camera.js: ' + fmtNum(b.camera_rss_mb, 1) + ' MB</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div class="telemetry-item-label">Swap / ZRAM (Used / Total / Free)</div>' +
          '<div class="telemetry-item-val mono">' + esc(swapUsed) + ' / ' + esc(swapTotal) + ' MB (' + esc(zramPct) + '%)</div>' +
          '<div class="mono telemetry-item-meta text-emerald">Swap Free: ' + esc(swapFree) + ' MB · ZRAM mem_limit=0</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div class="telemetry-item-label">Thẻ Nhớ Video (' + esc(b.sd_fs_type || 'f2fs') + ') &amp; Lưu Lượng</div>' +
          '<div class="telemetry-item-val mono">SD ' + esc(sdPct) + '% (' + esc(b.sd_used_gb || 0) + '/' + esc(b.sd_total_gb || 64) + ' GB) · RX ' + fmtNum(b.rx_mbps || 0, 1) + ' Mbps</div>' +
          '<div class="mini-meter" style="max-width:100%;"><div class="mini-meter-fill ' + sdBarClass + '" style="width:' + Math.min(100, Math.max(0, sdPct)) + '%"></div></div>' +
          '<div class="mono telemetry-item-meta text-cyan">' + esc(b.sd_mount_opts || 'errors=continue,noatime') + ' · ' + esc(b.store_domain || '--') + ' (' + esc(b.store_http_code || 200) + ')</div>' +
        '</div>';
    }

    /* Shinobi Monitors vs Live FFmpeg */
    var ffBadge = document.getElementById('drawer-ffmpeg-badge');
    var ffAct = b.ffmpeg_count !== undefined ? b.ffmpeg_count : cameras.length * 2;
    var ffExp = b.ffmpeg_expected !== undefined ? b.ffmpeg_expected : cameras.length * 2;
    if (ffBadge) ffBadge.textContent = 'FFmpeg: ' + ffAct + ' / ' + ffExp;

    var monEl = document.getElementById('drawer-monitors');
    if (monEl) {
      var list = monitors.length ? monitors : cameras;
      if (!list.length) {
        monEl.innerHTML = '<div class="text-secondary" style="font-size:12.5px;">Đang ghi hình ổn định (' + esc(ffAct) + ' tiến trình ffmpeg).</div>';
      } else {
        monEl.innerHTML = '<div class="table-scroll"><table class="scada-table">' +
          '<thead><tr><th>Monitor / Bàn</th><th>Host / Kênh</th><th>Codec</th><th>Trạng thái</th></tr></thead>' +
          '<tbody>' + list.map(function (m) {
            var rec = m.recording !== false && m.mode !== 'stop';
            return '<tr>' +
              '<td><strong>' + esc(m.name || m.table_name || m.mid || 'Camera') + '</strong></td>' +
              '<td class="mono">' + esc(m.host || m.ip || '192.168.1.x') + ':' + esc(m.port || 37777) + '</td>' +
              '<td class="mono">' + esc(m.codec || 'H.265') + ' · frag_keyframe+empty_moov</td>' +
              '<td>' + (rec
                ? '<span class="badge badge-emerald"><span class="led-dot emerald"></span>RECORDING</span>'
                : '<span class="badge badge-amber">STANDBY</span>') + '</td>' +
            '</tr>';
          }).join('') + '</tbody></table></div>';
      }
    }

    /* 8-Step Hardening Checklist in Drawer */
    var pillars = (hardening && Array.isArray(hardening.pillars)) ? hardening.pillars : GOLDEN_PILLAR_META.map(function (p) {
      return { id: p.id, step: p.step, name: p.name, status: 'pass', detail: 'Chuẩn Golden SOP' };
    });
    var hardEl = document.getElementById('drawer-hardening');
    if (hardEl) {
      hardEl.innerHTML = '<div class="pillar-list">' +
        pillars.map(function (p) {
          var ok = p.status === 'pass' || p.status === 'ok' || p.ok === true;
          return '<div class="pillar-row">' +
            '<div class="pillar-info">' +
              '<div class="pillar-name">Bước ' + esc(p.step) + ': ' + esc(p.name) + '</div>' +
              '<div class="pillar-detail mono">' + esc(p.detail || p.command_preview || '') + '</div>' +
            '</div>' +
            (ok
              ? '<span class="badge badge-emerald">' + svgIcon('check', 'ui-icon-sm') + '<span>ĐẠT</span></span>'
              : '<span class="badge badge-amber">' + svgIcon('warn', 'ui-icon-sm') + '<span>CẦN VÁ</span></span>') +
          '</div>';
        }).join('') +
      '</div>';
    }
  }

  /* --------------------------------------------------------------------------
     View 3: #hardening — 8-Step Golden Hardening Matrix & Safe 1-Click Remediator
     -------------------------------------------------------------------------- */
  function loadHardeningView() {
    return apiFetch('/fleet/api/hardening').then(function (data) {
      var list = Array.isArray(data.boxes) ? data.boxes : [];
      list.forEach(function (item) {
        indexItemSearch(item, [
          item.box_id, item.id, item.box_name, item.club_name, item.vpn_ip
        ]);
      });
      state.hardeningBoxes = list;
      state.hardeningSummary = data.summary || {};
      renderHardeningMatrix();
    }).catch(function (err) {
      showToast('Lỗi tải dữ liệu Golden Hardening: ' + err.message, true);
    });
  }

  function renderHardeningMatrix() {
    var container = document.getElementById('hardening-matrix-container');
    if (!container) return;
    var list = state.hardeningBoxes || [];
    var tokens = parseSearchQuery(state.hardeningSearch);

    var filtered = list.filter(function (item) {
      var sc = item.score !== undefined ? item.score : 8;
      var mx = item.max_score || 8;
      if (state.hardeningFilter === 'needs_fix' && sc >= mx) return false;
      if (state.hardeningFilter === 'compliant' && sc < mx) return false;
      return matchesSearch(item, tokens, [
        item.box_id, item.id, item.box_name, item.club_name, item.vpn_ip
      ]);
    });

    syncSearchBoxState('hardening-search', 'hardening-match-count', filtered.length, list.length);

    if (!filtered.length) {
      container.innerHTML = '<div class="empty-state">' +
        '<div class="empty-state-icon">' + svgIcon('shield') + '</div>' +
        '<div><strong>Không có Box nào phù hợp bộ lọc chuẩn hóa.</strong></div>' +
      '</div>';
      return;
    }

    var pg = paginateItems(filtered, 'hardening');
    var pageItems = pg.items;

    var rowsHtml = pageItems.map(function (item) {
      var boxId = item.box_id || item.id || '';
      var pillars = Array.isArray(item.pillars) ? item.pillars : [];
      var pillarMap = {};
      pillars.forEach(function (p) { pillarMap[p.id || ('step_' + p.step)] = p; });

      var cells = GOLDEN_PILLAR_META.map(function (meta) {
        var p = pillarMap[meta.id] || pillars[meta.step - 1] || { status: 'pass', detail: meta.name };
        var ok = p.status === 'pass' || p.status === 'ok' || p.ok === true;
        return '<td title="' + esc(p.detail || meta.name) + '">' +
          (ok
            ? '<span class="badge badge-emerald">' + svgIcon('check', 'ui-icon-sm') + '<span>Đạt</span></span>'
            : '<span class="badge badge-amber">' + svgIcon('warn', 'ui-icon-sm') + '<span>Vá</span></span>') +
        '</td>';
      }).join('');

      var score = item.score !== undefined ? item.score : 8;
      var maxScore = item.max_score || 8;

      return '<tr>' +
        '<td>' +
          '<div class="mono cell-primary">' + esc(boxId) + '</div>' +
          '<div class="cell-sub">' + esc(item.club_name || item.box_name || '') + '</div>' +
        '</td>' +
        '<td>' +
          (score >= maxScore
            ? '<span class="badge badge-emerald">' + esc(score) + '/' + esc(maxScore) + '</span>'
            : '<span class="badge badge-amber">' + esc(score) + '/' + esc(maxScore) + '</span>') +
        '</td>' +
        cells +
        '<td>' +
          '<div class="action-btn-row">' +
            '<button type="button" class="btn btn-xs btn-primary" data-remediate-box="' + esc(boxId) + '">' + svgIcon('wrench', 'ui-icon-sm') + '<span>Vá 8 Bước</span></button>' +
            '<button type="button" class="btn btn-xs" data-inspect-box="' + esc(boxId) + '">' + svgIcon('search', 'ui-icon-sm') + '<span>Kiểm tra</span></button>' +
          '</div>' +
        '</td>' +
      '</tr>';
    }).join('');

    var mobileCardsHtml = pageItems.map(function (item) {
      var boxId = item.box_id || item.id || '';
      var pillars = Array.isArray(item.pillars) ? item.pillars : [];
      var pillarMap = {};
      pillars.forEach(function (p) { pillarMap[p.id || ('step_' + p.step)] = p; });
      var score = item.score !== undefined ? item.score : 8;
      var maxScore = item.max_score || 8;
      var isOk = score >= maxScore;
      var isExpanded = !!state.expandedHardeningIds[boxId];
      var failedSteps = [];

      var miniPills = GOLDEN_PILLAR_META.map(function (meta) {
        var p = pillarMap[meta.id] || pillars[meta.step - 1] || { status: 'pass', detail: meta.name };
        var ok = p.status === 'pass' || p.status === 'ok' || p.ok === true;
        if (!ok) failedSteps.push('B' + meta.step);
        return '<span class="pillar-mini-pill ' + (ok ? 'ok' : 'warn') + '" title="' + esc(meta.name + ': ' + (p.detail || '')) + '">' +
          svgIcon(ok ? 'check' : 'warn', 'ui-icon-sm') + '<span>B' + esc(meta.step) + '</span>' +
        '</span>';
      }).join('');

      var statusMeta = failedSteps.length
        ? '<span class="text-amber mrow-alert-txt">Thiếu ' + esc(failedSteps.join(',')) + '</span>'
        : '<span>Đủ 8/8 · 0-Format</span>';

      return '<article class="hardening-mcard compact-mrow' + (isExpanded ? ' is-expanded' : '') + '" data-compact-row="hardening" data-mrow-key="' + esc(boxId) + '">' +
        '<div class="mrow-head" data-toggle-mrow="hardening" data-mrow-id="' + esc(boxId) + '" role="button" tabindex="0" aria-expanded="' + (isExpanded ? 'true' : 'false') + '">' +
          '<div class="mrow-line1">' +
            '<div class="mrow-primary">' +
              '<span class="led-dot ' + (isOk ? 'emerald' : 'amber') + '"></span>' +
              '<span class="mrow-name">' + esc(item.club_name || item.box_name || boxId) + '</span>' +
            '</div>' +
            '<div class="mrow-pills">' +
              '<span class="mrow-pill ' + (isOk ? 'pill-ok' : 'pill-amber') + '">' + esc(score) + '/' + esc(maxScore) + '</span>' +
              '<span class="mrow-chevron" aria-hidden="true">' + svgIcon('chevronDown', 'ui-icon-sm') + '</span>' +
            '</div>' +
          '</div>' +
          '<div class="mrow-line2 mono">' +
            '<span class="mrow-id">' + esc(boxId) + (item.vpn_ip ? (' · ' + esc(item.vpn_ip)) : '') + '</span>' +
            '<span class="mrow-metrics"><span class="mrow-sep">·</span>' + statusMeta + '</span>' +
          '</div>' +
        '</div>' +
        '<div class="mrow-body">' +
          '<div class="pillar-mini-grid">' + miniPills + '</div>' +
          '<div class="mrow-actions">' +
            '<button type="button" class="btn btn-primary mrow-act-btn" data-remediate-box="' + esc(boxId) + '">' + svgIcon('wrench', 'ui-icon-sm') + '<span>Vá 8 Bước</span></button>' +
            '<button type="button" class="btn mrow-act-btn" data-inspect-box="' + esc(boxId) + '">' + svgIcon('search', 'ui-icon-sm') + '<span>Kiểm tra</span></button>' +
            '<button type="button" class="btn btn-cyan mrow-act-btn" data-open-box="' + esc(boxId) + '">Chi tiết</button>' +
          '</div>' +
        '</div>' +
      '</article>';
    }).join('');

    var headersHtml = GOLDEN_PILLAR_META.map(function (m) {
      return '<th title="' + esc(m.name) + '">' + esc(m.short) + '</th>';
    }).join('');

    container.innerHTML =
      '<div class="table-scroll desktop-only-table"><table class="scada-table tabular-nums">' +
        '<thead><tr>' +
          '<th>Box / CLB</th>' +
          '<th>Điểm</th>' +
          headersHtml +
          '<th>Thao Tác An Toàn</th>' +
        '</tr></thead>' +
        '<tbody>' + rowsHtml + '</tbody>' +
      '</table></div>' +
      '<div class="mobile-card-list compact-list mobile-only-cards">' + mobileCardsHtml + '</div>' +
      renderPaginationBar('hardening', pg, 'Box');
  }

  function remediateBox(boxId) {
    if (!boxId) return Promise.resolve();
    return apiFetch('/fleet/api/boxes/' + encodeURIComponent(boxId) + '/hardening/remediate', {
      method: 'POST',
      body: { steps: [1, 2, 3, 4, 5, 6, 7, 8] }
    }).then(function () {
      showToast('Đã thực thi chuẩn hóa 8 bước Golden Hardening (Zero Format) cho ' + boxId);
      loadHardeningView();
      loadFleetSummary();
      if (state.drawerBoxId === boxId) openBoxDrawer(boxId, false);
    }).catch(function (err) {
      showToast('Lỗi vá Golden Hardening ' + boxId + ': ' + err.message, true);
    });
  }

  function inspectBoxHardening(boxId) {
    if (!boxId) return Promise.resolve();
    return apiFetch('/fleet/api/boxes/' + encodeURIComponent(boxId) + '/hardening/inspect', {
      method: 'POST',
      body: {}
    }).then(function () {
      showToast('Đã kiểm toán lại 8 bước cho ' + boxId);
      loadHardeningView();
    }).catch(function (err) {
      showToast('Lỗi kiểm toán ' + boxId + ': ' + err.message, true);
    });
  }

  /* --------------------------------------------------------------------------
     View 4: #cameras — Multi-Box Camera & NVR Inventory + Bulk Golden H.265
     -------------------------------------------------------------------------- */
  function loadCamerasView() {
    return apiFetch('/fleet/api/cameras').then(function (data) {
      var list = Array.isArray(data.cameras) ? data.cameras : [];
      list.forEach(function (c) {
        indexItemSearch(c, [
          c.id, c.box_id, c.club_name, c.club_id, c.name, c.table_name,
          c.host, c.port, c.nvr_host, c.vendor, c.codec, c.audio_codec
        ]);
      });
      state.cameras = list;
      state.camerasSummary = data.summary || { double_stream_count: data.double_stream_count || 0 };
      var navBadgeCam = document.getElementById('nav-badge-cameras');
      if (navBadgeCam) navBadgeCam.textContent = String(state.cameras.length);
      populateCameraFilterSelects();
      renderCamerasTable();
    }).catch(function (err) {
      showToast('Lỗi tải kho camera toàn đội: ' + err.message, true);
    });
  }

  function populateCameraFilterSelects() {
    var clubSelect = document.getElementById('cam-club-filter');
    var boxSelect = document.getElementById('cam-box-filter');
    if (!clubSelect || !boxSelect) return;

    var curClub = clubSelect.value;
    var curBox = boxSelect.value;
    var clubsSeen = {};
    var boxesSeen = {};

    state.cameras.forEach(function (c) {
      if (c.club_name) clubsSeen[c.club_name] = true;
      if (c.box_id) boxesSeen[c.box_id] = c.club_name || c.box_id;
    });

    clubSelect.innerHTML = '<option value="">Tất cả CLB (' + Object.keys(clubsSeen).length + ')</option>' +
      Object.keys(clubsSeen).sort().map(function (name) {
        return '<option value="' + esc(name) + '">' + esc(name) + '</option>';
      }).join('');
    clubSelect.value = curClub;

    boxSelect.innerHTML = '<option value="">Tất cả Box (' + Object.keys(boxesSeen).length + ')</option>' +
      Object.keys(boxesSeen).sort().map(function (bid) {
        return '<option value="' + esc(bid) + '">' + esc(bid) + ' — ' + esc(boxesSeen[bid]) + '</option>';
      }).join('');
    boxSelect.value = curBox;
  }

  function getFilteredCameras() {
    var clubVal = (document.getElementById('cam-club-filter') || {}).value || '';
    var boxVal = (document.getElementById('cam-box-filter') || {}).value || '';
    var vendorVal = (document.getElementById('cam-vendor-filter') || {}).value || '';
    var onlyDouble = !!(document.getElementById('cam-doublestream-filter') || {}).checked;
    var q = (document.getElementById('cam-search-input') || {}).value || '';
    var tokens = parseSearchQuery(q);

    return (state.cameras || []).filter(function (c) {
      if (clubVal && c.club_name !== clubVal && c.club_id !== clubVal) return false;
      if (boxVal && c.box_id !== boxVal) return false;
      if (vendorVal && String(c.vendor || '').toLowerCase() !== vendorVal.toLowerCase()) return false;
      if (onlyDouble && !c.double_streaming) return false;
      return matchesSearch(c, tokens, [
        c.id, c.box_id, c.club_name, c.club_id, c.name, c.table_name,
        c.host, c.port, c.nvr_host, c.vendor, c.codec, c.audio_codec
      ]);
    });
  }

  function getCameraId(c) {
    return c.id || (c.box_id + ':' + c.host + ':' + (c.port || 37777));
  }

  function syncCameraSelectAllState(pageItems) {
    var camSelectAll = document.getElementById('cam-select-all');
    if (!camSelectAll) return;
    if (!pageItems.length) {
      camSelectAll.checked = false;
      camSelectAll.indeterminate = false;
      return;
    }
    var checkedOnPage = 0;
    pageItems.forEach(function (c) {
      if (state.selectedCameraIds[getCameraId(c)]) checkedOnPage++;
    });
    camSelectAll.checked = checkedOnPage === pageItems.length;
    camSelectAll.indeterminate = checkedOnPage > 0 && checkedOnPage < pageItems.length;
  }

  function renderCamerasTable() {
    var tbody = document.getElementById('cameras-tbody');
    var mobileWrap = document.getElementById('cameras-mobile-cards');
    var pagerWrap = document.getElementById('cameras-pagination');
    var sumBar = document.getElementById('cameras-summary-bar');
    if (!tbody) return;

    var filtered = getFilteredCameras();
    var doubleCount = filtered.filter(function (c) { return c.double_streaming; }).length;
    var h265Count = filtered.filter(function (c) { return String(c.codec || '').toUpperCase() === 'H.265'; }).length;
    var selectedCount = Object.keys(state.selectedCameraIds).filter(function (k) { return state.selectedCameraIds[k]; }).length;

    syncSearchBoxState('cam-search-input', 'cameras-match-count', filtered.length, state.cameras.length);

    if (sumBar) {
      sumBar.innerHTML =
        '<div class="badge-row">' +
          '<span class="badge badge-cyan">Khớp lọc: ' + esc(fmtInt(filtered.length)) + ' / ' + esc(fmtInt(state.cameras.length)) + ' Camera</span>' +
          '<span class="badge badge-emerald">Chuẩn H.265: ' + esc(fmtInt(h265Count)) + '</span>' +
          (doubleCount > 0
            ? '<span class="badge badge-amber">' + svgIcon('warn', 'ui-icon-sm') + '<span>Phát hiện Double-Streaming: ' + esc(fmtInt(doubleCount)) + ' kênh</span></span>'
            : '<span class="badge badge-emerald">' + svgIcon('check', 'ui-icon-sm') + '<span>0 Double-Streaming</span></span>') +
          (selectedCount > 0
            ? '<span class="badge badge-purple">Đã chọn: ' + esc(fmtInt(selectedCount)) + '</span>'
            : '') +
        '</div>' +
        '<div class="mono text-secondary" style="font-size:12px;">' +
          'Chuẩn Bida Golden: H.265 · 30 FPS · 1536 Kbps CBR · GOP 50 · AAC' +
        '</div>';
    }

    if (!filtered.length) {
      tbody.innerHTML = '<tr><td colspan="10" class="analytics-empty-cell">Không có camera nào khớp bộ lọc hiện tại.</td></tr>';
      if (mobileWrap) {
        mobileWrap.innerHTML = '<div class="empty-state">' +
          '<div class="empty-state-icon">' + svgIcon('video') + '</div>' +
          '<div><strong>Không có camera nào khớp bộ lọc hiện tại.</strong></div>' +
        '</div>';
      }
      if (pagerWrap) pagerWrap.innerHTML = '';
      syncCameraSelectAllState([]);
      return;
    }

    var pg = paginateItems(filtered, 'cameras');
    var pageItems = pg.items;
    syncCameraSelectAllState(pageItems);

    tbody.innerHTML = pageItems.map(function (c) {
      var cid = getCameraId(c);
      var checked = state.selectedCameraIds[cid] ? 'checked' : '';
      var isH265 = String(c.codec || '').toUpperCase() === 'H.265';
      return '<tr>' +
        '<td><input type="checkbox" class="cam-row-check" data-cam-id="' + esc(cid) + '" ' + checked + '></td>' +
        '<td class="mono cell-primary">' + esc(cid) + '</td>' +
        '<td>' +
          '<div>' + esc(c.club_name || '--') + '</div>' +
          '<div class="mono cell-sub">' + esc(c.box_id) + '</div>' +
        '</td>' +
        '<td><strong>' + esc(c.table_name || c.name || 'Bàn Bida') + '</strong></td>' +
        '<td><span class="badge badge-neutral">' + esc(c.vendor || 'Dahua') + '</span></td>' +
        '<td class="mono">' + (c.nvr_host ? (esc(c.nvr_host) + ' (CH' + esc(c.nvr_channel || 1) + ')') : 'Direct LAN') + '</td>' +
        '<td>' +
          (c.online !== false
            ? '<span class="badge badge-emerald"><span class="led-dot emerald"></span>ONLINE' + (c.recording !== false ? ' · REC' : '') + '</span>'
            : '<span class="badge badge-crimson"><span class="led-dot crimson"></span>OFFLINE</span>') +
        '</td>' +
        '<td class="mono">' +
          (isH265
            ? '<span class="badge badge-emerald">H.265</span>'
            : '<span class="badge badge-amber">' + esc(c.codec || 'H.264') + '</span>') +
          ' ' + esc(c.fps || 30) + 'fps · ' + esc(c.bitrate_kbps || 1536) + 'kbps ' + esc(c.rc_mode || 'CBR') + ' · GOP ' + esc(c.gop || 50) +
        '</td>' +
        '<td class="mono">' + esc(c.audio_codec || 'AAC') + '</td>' +
        '<td>' +
          (c.double_streaming
            ? '<span class="badge badge-amber">' + svgIcon('warn', 'ui-icon-sm') + '<span>DOUBLE-STREAM</span></span>'
            : '<span class="badge badge-emerald">' + svgIcon('check', 'ui-icon-sm') + '<span>ĐƠN LUỒNG</span></span>') +
        '</td>' +
      '</tr>';
    }).join('');

    if (mobileWrap) {
      mobileWrap.innerHTML = pageItems.map(function (c) {
        var cid = getCameraId(c);
        var checked = state.selectedCameraIds[cid] ? 'checked' : '';
        var isH265 = String(c.codec || '').toUpperCase() === 'H.265';
        var isOnline = c.online !== false;
        var isExpanded = !!state.expandedCameraIds[cid];
        var hostPort = (c.host || '--') + ':' + (c.port || 37777);
        var fpsVal = c.fps || 30;
        var kbpsVal = c.bitrate_kbps || 1536;

        return '<article class="cam-mcard compact-mrow' + (isExpanded ? ' is-expanded' : '') + '" data-compact-row="camera" data-mrow-key="' + esc(cid) + '">' +
          '<div class="mrow-head" data-toggle-mrow="camera" data-mrow-id="' + esc(cid) + '" role="button" tabindex="0" aria-expanded="' + (isExpanded ? 'true' : 'false') + '">' +
            '<div class="mrow-line1">' +
              '<div class="mrow-primary">' +
                '<label class="cam-mrow-check-stop" aria-label="Chọn ' + esc(c.table_name || c.name || cid) + '">' +
                  '<input type="checkbox" class="cam-row-check" data-cam-id="' + esc(cid) + '" ' + checked + '>' +
                '</label>' +
                '<span class="led-dot ' + (isOnline ? 'emerald' : 'crimson') + '"></span>' +
                '<span class="mrow-name">' + esc(c.table_name || c.name || 'Bàn Bida') + '</span>' +
              '</div>' +
              '<div class="mrow-pills">' +
                (c.double_streaming ? '<span class="mrow-pill pill-amber">2x</span>' : '') +
                '<span class="mrow-pill ' + (isH265 ? 'pill-ok' : 'pill-amber') + '">' + esc(c.codec || 'H.264') + '</span>' +
                '<span class="mrow-chevron" aria-hidden="true">' + svgIcon('chevronDown', 'ui-icon-sm') + '</span>' +
              '</div>' +
            '</div>' +
            '<div class="mrow-line2 mono">' +
              '<span class="mrow-id">' + esc(c.club_name || c.box_id || '--') + '</span>' +
              '<span class="mrow-metrics">' +
                '<span class="mrow-sep">·</span>' +
                '<span>' + esc(hostPort) + '</span>' +
                '<span class="mrow-sep">·</span>' +
                '<span class="' + (c.double_streaming ? 'text-amber mrow-alert-txt' : '') + '">' + esc(fpsVal) + 'f/' + esc(kbpsVal) + 'k</span>' +
              '</span>' +
            '</div>' +
          '</div>' +
          '<div class="mrow-body">' +
            '<div class="mrow-meta-line mono">' +
              '<span>' + esc(cid) + '</span>' +
              '<span>' + (c.nvr_host ? ('NVR ' + esc(c.nvr_host) + ' CH' + esc(c.nvr_channel || 1)) : 'Direct LAN') + '</span>' +
            '</div>' +
            '<div class="badge-row">' +
              (isOnline
                ? '<span class="badge badge-emerald"><span class="led-dot emerald"></span>ONLINE' + (c.recording !== false ? ' · REC' : '') + '</span>'
                : '<span class="badge badge-crimson"><span class="led-dot crimson"></span>OFFLINE</span>') +
              '<span class="badge badge-neutral">' + esc(c.vendor || 'Dahua') + '</span>' +
              '<span class="badge badge-neutral mono">' + esc(fpsVal) + 'fps · ' + esc(kbpsVal) + 'k ' + esc(c.rc_mode || 'CBR') + ' · GOP ' + esc(c.gop || 50) + ' · ' + esc(c.audio_codec || 'AAC') + '</span>' +
              (c.double_streaming
                ? '<span class="badge badge-amber">' + svgIcon('warn', 'ui-icon-sm') + '<span>DOUBLE-STREAM</span></span>'
                : '<span class="badge badge-emerald">' + svgIcon('check', 'ui-icon-sm') + '<span>Đơn luồng</span></span>') +
            '</div>' +
            (c.box_id
              ? '<div class="mrow-actions"><button type="button" class="btn btn-cyan mrow-act-btn" data-open-box="' + esc(c.box_id) + '">Chi tiết Box ' + esc(c.box_id) + '</button></div>'
              : '') +
          '</div>' +
        '</article>';
      }).join('');
    }

    if (pagerWrap) {
      pagerWrap.innerHTML = renderPaginationBar('cameras', pg, 'Camera');
    }
  }

  function applyBulkGoldenH265() {
    var filtered = getFilteredCameras();
    var selectedIds = Object.keys(state.selectedCameraIds).filter(function (k) { return state.selectedCameraIds[k]; });
    if (selectedIds.length === 0) {
      selectedIds = filtered.map(function (c) { return c.id || (c.box_id + ':' + c.host + ':' + (c.port || 37777)); });
    }
    var boxVal = (document.getElementById('cam-box-filter') || {}).value || '';
    var payload = {
      box_id: boxVal,
      camera_ids: selectedIds,
      codec: 'H.265',
      fps: 30,
      bitrate_kbps: 1536,
      rc_mode: 'CBR',
      gop: 50,
      audio_codec: 'AAC',
      resolve_double_stream: true,
      profile: {
        codec: 'H.265',
        fps: 30,
        bitrate_kbps: 1536,
        rc_mode: 'CBR',
        gop: 50,
        audio_codec: 'AAC',
        resolve_double_stream: true
      }
    };

    return apiFetch('/fleet/api/cameras/bulk-apply', {
      method: 'POST',
      body: payload
    }).then(function (res) {
      showToast('Đã áp dụng Chuẩn Bida Golden H.265 (30fps · 1536kbps CBR · GOP 50 · AAC) cho ' + (res.updated || selectedIds.length) + ' camera!');
      loadCamerasView();
      loadFleetSummary();
    }).catch(function (err) {
      showToast('Lỗi áp dụng Golden H.265: ' + err.message, true);
    });
  }

  /* --------------------------------------------------------------------------
     View 5: #redbida — 17-Rule Inspector, Branding Guard & FRPC Auditor
     -------------------------------------------------------------------------- */
  function buildMergedRedbidaDomains(boxes, domains) {
    var byBoxMap = {};
    var domList = [];

    function ensureDomRow(boxId, clubName, branding) {
      var key = boxId || clubName || 'unknown';
      if (!byBoxMap[key]) {
        byBoxMap[key] = {
          box_id: boxId || '--',
          club_name: clubName || '',
          branding: String(branding || 'billiards').toLowerCase(),
          store_domain: '',
          cstore_domain: '',
          store_http_code: 200,
          cstore_http_code: 200,
          conflict: false,
          shinobi_group_key: '',
          ggcode: '',
          virtual_ip: ''
        };
        domList.push(byBoxMap[key]);
      }
      return byBoxMap[key];
    }

    (boxes || []).forEach(function (b) {
      var row = ensureDomRow(b.box_id || b.id, b.club_name || b.name, b.branding);
      if (b.store_domain) row.store_domain = b.store_domain;
      if (b.cstore_domain) row.cstore_domain = b.cstore_domain;
      if (b.store_http_code) row.store_http_code = b.store_http_code;
      if (b.cstore_http_code) row.cstore_http_code = b.cstore_http_code;
      if (b.domain_conflict) row.conflict = true;
      var keys = b.keys || {};
      if (keys.shinobi_group_key || b.shinobi_group_key) row.shinobi_group_key = keys.shinobi_group_key || b.shinobi_group_key;
      if (keys.ggcode || b.ggcode) row.ggcode = keys.ggcode || b.ggcode;
      if (keys.eth0_virtual_ip || b.virtual_ip) row.virtual_ip = keys.eth0_virtual_ip || b.virtual_ip;
    });

    (domains || []).forEach(function (d) {
      var row = ensureDomRow(d.box_id, d.club_name || d.box_name, d.branding);
      var kind = String(d.kind || '').toLowerCase();
      var cleanHost = function (entry) {
        var raw = entry.full_url || entry.domain || entry.subdomain || '';
        return String(raw).replace(/^https?:\/\//i, '');
      };
      if (d.store_domain) {
        row.store_domain = d.store_domain;
        if (d.store_http_code) row.store_http_code = d.store_http_code;
      }
      if (d.cstore_domain) {
        row.cstore_domain = d.cstore_domain;
        if (d.cstore_http_code) row.cstore_http_code = d.cstore_http_code;
      }
      if (kind === 'store') {
        var sh = cleanHost(d);
        if (sh && !row.store_domain) row.store_domain = sh;
        if (d.http_code) row.store_http_code = d.http_code;
      } else if (kind === 'c-store' || kind === 'cstore') {
        var ch = cleanHost(d);
        if (ch && !row.cstore_domain) row.cstore_domain = ch;
        if (d.http_code) row.cstore_http_code = d.http_code;
      }
      if (d.conflict) row.conflict = true;
    });

    domList.forEach(function (d) {
      indexItemSearch(d, [
        d.box_id, d.club_name, d.store_domain, d.cstore_domain,
        d.shinobi_group_key, d.ggcode, d.virtual_ip, d.branding
      ]);
    });
    return domList;
  }

  function loadRedbidaView() {
    return apiFetch('/fleet/api/redbida').then(function (data) {
      var boxes = Array.isArray(data.boxes) ? data.boxes : [];
      var domains = Array.isArray(data.domains) ? data.domains : [];
      boxes.forEach(function (b) {
        var keys = b.keys || {};
        indexItemSearch(b, [
          b.box_id, b.id, b.club_name, b.box_name, b.store_domain, b.cstore_domain,
          keys.shinobi_group_key || b.shinobi_group_key,
          keys.ggcode || b.ggcode,
          keys.eth0_virtual_ip || keys.end0_virtual_ip || b.virtual_ip,
          b.branding
        ]);
      });
      state.redbidaBoxes = boxes;
      state.redbidaDomains = domains;
      state.redbidaMergedDomains = buildMergedRedbidaDomains(boxes, domains);
      state.redbidaConflicts = Array.isArray(data.conflicts) ? data.conflicts : [];
      renderRedbidaView();
    }).catch(function (err) {
      showToast('Lỗi tải dữ liệu RedBida & FRPC: ' + err.message, true);
    });
  }

  function renderRedbidaView() {
    var domContainer = document.getElementById('redbida-domains-container');
    var boxContainer = document.getElementById('redbida-boxes-container');
    var domCountEl = document.getElementById('redbida-domains-count');
    var boxCountEl = document.getElementById('redbida-boxes-count');
    var tokens = parseSearchQuery(state.redbidaSearch);

    var domList = state.redbidaMergedDomains || [];
    var domFiltered = domList.filter(function (d) {
      if (state.redbidaBrandFilter === 'billiards' && d.branding !== 'billiards') return false;
      if (state.redbidaBrandFilter === 'pickleball' && d.branding !== 'pickleball') return false;
      if (state.redbidaBrandFilter === 'conflict' && !d.conflict) return false;
      return matchesSearch(d, tokens, [d.box_id, d.club_name, d.store_domain, d.cstore_domain, d.branding]);
    });

    var rBoxes = (state.redbidaBoxes || []).filter(function (b) {
      var br = String(b.branding || 'billiards').toLowerCase();
      if (state.redbidaBrandFilter === 'billiards' && br !== 'billiards') return false;
      if (state.redbidaBrandFilter === 'pickleball' && br !== 'pickleball') return false;
      if (state.redbidaBrandFilter === 'conflict' && !b.domain_conflict) return false;
      return matchesSearch(b, tokens, [b.box_id || b.id, b.club_name, b.store_domain, b.cstore_domain, b.shinobi_group_key, b.branding]);
    });

    syncSearchBoxState('redbida-search', 'redbida-match-count', rBoxes.length, (state.redbidaBoxes || []).length);
    if (domCountEl) domCountEl.textContent = fmtInt(domFiltered.length) + ' / ' + fmtInt(domList.length) + ' tên miền';
    if (boxCountEl) boxCountEl.textContent = fmtInt(rBoxes.length) + ' / ' + fmtInt((state.redbidaBoxes || []).length) + ' Box';

    /* Render FRPC Subdomain Auditor with independent pagination */
    if (domContainer) {
      if (!domFiltered.length) {
        domContainer.innerHTML = '<div class="empty-state">' +
          '<div class="empty-state-icon">' + svgIcon('globe') + '</div>' +
          '<div><strong>Không có tên miền FRPC nào khớp bộ lọc.</strong></div>' +
        '</div>';
      } else {
        var domPg = paginateItems(domFiltered, 'redbidaDomains');
        var domDesktopRows = domPg.items.map(function (d) {
          var sCode = d.store_http_code || 200;
          var cCode = d.cstore_http_code || 200;
          return '<tr>' +
            '<td><strong class="mono">' + esc(d.box_id) + '</strong> · ' + esc(d.club_name || '') + '</td>' +
            '<td class="mono">' + esc(d.store_domain || '--') + '</td>' +
            '<td><span class="badge ' + (sCode === 200 ? 'badge-emerald' : 'badge-amber') + '">HTTP ' + esc(sCode) + '</span></td>' +
            '<td class="mono">' + esc(d.cstore_domain || '--') + '</td>' +
            '<td><span class="badge ' + (cCode === 200 ? 'badge-emerald' : 'badge-amber') + '">HTTP ' + esc(cCode) + '</span></td>' +
            '<td>' + (d.conflict
              ? '<span class="badge badge-crimson">' + svgIcon('warn', 'ui-icon-sm') + '<span>TRÙNG SUBDOMAIN</span></span>'
              : '<span class="badge badge-emerald">' + svgIcon('check', 'ui-icon-sm') + '<span>DUY NHẤT</span></span>') + '</td>' +
          '</tr>';
        }).join('');

        var domMobileCards = domPg.items.map(function (d) {
          var sCode = d.store_http_code || 200;
          var cCode = d.cstore_http_code || 200;
          var domKey = d.box_id || d.club_name || '';
          var isExpanded = !!state.expandedRedbidaDomainIds[domKey];
          var httpOk = sCode === 200 && cCode === 200;
          var dotTone = d.conflict ? 'crimson' : (httpOk ? 'emerald' : 'amber');
          var httpLabel = httpOk ? '200' : (sCode + '/' + cCode);

          return '<article class="redbida-mcard compact-mrow' + (isExpanded ? ' is-expanded' : '') + '" data-compact-row="rbDomain" data-mrow-key="' + esc(domKey) + '">' +
            '<div class="mrow-head" data-toggle-mrow="rbDomain" data-mrow-id="' + esc(domKey) + '" role="button" tabindex="0" aria-expanded="' + (isExpanded ? 'true' : 'false') + '">' +
              '<div class="mrow-line1">' +
                '<div class="mrow-primary">' +
                  '<span class="led-dot ' + dotTone + '"></span>' +
                  '<span class="mrow-name">' + esc(d.club_name || d.box_id) + '</span>' +
                '</div>' +
                '<div class="mrow-pills">' +
                  '<span class="mrow-pill ' + (httpOk ? 'pill-quiet' : 'pill-amber') + '">' + esc(httpLabel) + '</span>' +
                  (d.conflict
                    ? '<span class="mrow-pill pill-crimson">Trùng</span>'
                    : '<span class="mrow-pill pill-ok">OK</span>') +
                  '<span class="mrow-chevron" aria-hidden="true">' + svgIcon('chevronDown', 'ui-icon-sm') + '</span>' +
                '</div>' +
              '</div>' +
              '<div class="mrow-line2 mono">' +
                '<span class="mrow-id">' + esc(d.box_id) + ' · ' + esc(d.store_domain || '--') + '</span>' +
              '</div>' +
            '</div>' +
            '<div class="mrow-body">' +
              '<div class="redbida-domain-row">' +
                '<span class="redbida-domain-host">' + esc(d.store_domain || '--') + '</span>' +
                '<span class="badge ' + (sCode === 200 ? 'badge-emerald' : 'badge-amber') + '">HTTP ' + esc(sCode) + '</span>' +
              '</div>' +
              '<div class="redbida-domain-row">' +
                '<span class="redbida-domain-host">' + esc(d.cstore_domain || '--') + '</span>' +
                '<span class="badge ' + (cCode === 200 ? 'badge-emerald' : 'badge-amber') + '">HTTP ' + esc(cCode) + '</span>' +
              '</div>' +
              '<div class="mrow-actions">' +
                '<button type="button" class="btn btn-cyan mrow-act-btn" data-open-redbida-edit="' + esc(d.box_id) + '">' + svgIcon('unlock', 'ui-icon-sm') + '<span>Mở Khóa</span></button>' +
                '<button type="button" class="btn mrow-act-btn" data-open-box="' + esc(d.box_id) + '">Chi tiết</button>' +
              '</div>' +
            '</div>' +
          '</article>';
        }).join('');

        domContainer.innerHTML =
          '<div class="table-scroll desktop-only-table"><table class="scada-table tabular-nums">' +
            '<thead><tr><th>Box ID / CLB</th><th>Subdomain Store</th><th>HTTP</th><th>Subdomain C-Store</th><th>HTTP</th><th>Kiểm Toán</th></tr></thead>' +
            '<tbody>' + domDesktopRows + '</tbody>' +
          '</table></div>' +
          '<div class="mobile-card-list compact-list mobile-only-cards">' + domMobileCards + '</div>' +
          renderPaginationBar('redbidaDomains', domPg, 'tên miền');
      }
    }

    /* Render 17-Rule Inspector & Branding Guard with independent pagination */
    if (boxContainer) {
      if (!rBoxes.length) {
        boxContainer.innerHTML = '<div class="empty-state">' +
          '<div class="empty-state-icon">' + svgIcon('sliders') + '</div>' +
          '<div><strong>Không có Box nào khớp bộ lọc RedBida.</strong></div>' +
        '</div>';
      } else {
        var boxPg = paginateItems(rBoxes, 'redbidaBoxes');
        var boxDesktopRows = boxPg.items.map(function (b) {
          var boxId = b.box_id || b.id || '';
          var isPickle = String(b.branding || '').toLowerCase() === 'pickleball';
          var score = b.score !== undefined ? b.score : (b.redbida_score || 17);
          var maxScore = b.max_score || 17;
          var keys = b.keys || {};
          var vip = keys.eth0_virtual_ip || keys.end0_virtual_ip || b.virtual_ip || '192.168.1.250';
          var grp = keys.shinobi_group_key || b.shinobi_group_key || '--';
          var gg = keys.ggcode || b.ggcode || 'GG-BIDA';

          return '<tr>' +
            '<td>' +
              '<div class="mono cell-primary">' + esc(boxId) + '</div>' +
              '<div class="cell-sub">' + esc(b.club_name || b.box_name || '') + '</div>' +
            '</td>' +
            '<td>' +
              (isPickle
                ? '<span class="badge badge-amber">' + svgIcon('pickle', 'ui-icon-sm') + '<span>Pickleball (Preserved)</span></span>'
                : '<span class="badge badge-cyan">' + svgIcon('pool', 'ui-icon-sm') + '<span>Billiards (RedBida)</span></span>') +
            '</td>' +
            '<td>' +
              '<span class="badge ' + (score >= maxScore ? 'badge-emerald' : 'badge-amber') + '">' + esc(score) + '/' + esc(maxScore) + '</span>' +
            '</td>' +
            '<td class="mono" style="font-size:12px;">' +
              'VIP: <code>' + esc(vip) + '</code> · Grp: <code>' + esc(grp) + '</code> · GG: <code>' + esc(gg) + '</code> · [C01..C20]' +
            '</td>' +
            '<td>' +
              '<div class="action-btn-row">' +
                '<button type="button" class="btn btn-xs btn-cyan" data-open-redbida-edit="' + esc(boxId) + '">' + svgIcon('unlock', 'ui-icon-sm') + '<span>Mở Khóa</span></button>' +
                '<button type="button" class="btn btn-xs btn-primary" data-autofix-redbida="' + esc(boxId) + '">' + svgIcon('zap', 'ui-icon-sm') + '<span>Auto-Fix 17</span></button>' +
              '</div>' +
            '</td>' +
          '</tr>';
        }).join('');

        var boxMobileCards = boxPg.items.map(function (b) {
          var boxId = b.box_id || b.id || '';
          var isPickle = String(b.branding || '').toLowerCase() === 'pickleball';
          var score = b.score !== undefined ? b.score : (b.redbida_score || 17);
          var maxScore = b.max_score || 17;
          var isOk = score >= maxScore;
          var isExpanded = !!state.expandedRedbidaBoxIds[boxId];
          var keys = b.keys || {};
          var vip = keys.eth0_virtual_ip || keys.end0_virtual_ip || b.virtual_ip || '192.168.1.250';
          var grp = keys.shinobi_group_key || b.shinobi_group_key || '--';
          var gg = keys.ggcode || b.ggcode || 'GG-BIDA';

          return '<article class="redbida-mcard compact-mrow' + (isExpanded ? ' is-expanded' : '') + '" data-compact-row="rbBox" data-mrow-key="' + esc(boxId) + '">' +
            '<div class="mrow-head" data-toggle-mrow="rbBox" data-mrow-id="' + esc(boxId) + '" role="button" tabindex="0" aria-expanded="' + (isExpanded ? 'true' : 'false') + '">' +
              '<div class="mrow-line1">' +
                '<div class="mrow-primary">' +
                  svgIcon(isPickle ? 'pickle' : 'pool', 'ui-icon-sm ' + (isPickle ? 'text-amber' : 'text-cyan')) +
                  '<span class="mrow-name">' + esc(b.club_name || b.box_name || boxId) + '</span>' +
                '</div>' +
                '<div class="mrow-pills">' +
                  '<span class="mrow-pill ' + (isOk ? 'pill-ok' : 'pill-amber') + '">' + esc(score) + '/' + esc(maxScore) + '</span>' +
                  '<span class="mrow-chevron" aria-hidden="true">' + svgIcon('chevronDown', 'ui-icon-sm') + '</span>' +
                '</div>' +
              '</div>' +
              '<div class="mrow-line2 mono">' +
                '<span class="mrow-id">' + esc(boxId) + ' · ' + esc(vip) + ' · ' + esc(grp) + '</span>' +
              '</div>' +
            '</div>' +
            '<div class="mrow-body">' +
              '<div class="badge-row">' +
                (isPickle
                  ? '<span class="badge badge-amber">' + svgIcon('pickle', 'ui-icon-sm') + '<span>Pickleball</span></span>'
                  : '<span class="badge badge-cyan">' + svgIcon('pool', 'ui-icon-sm') + '<span>Billiards</span></span>') +
                '<span class="mcard-net-chip">VIP ' + esc(vip) + '</span>' +
                '<span class="mcard-net-chip">Grp ' + esc(grp) + '</span>' +
                '<span class="mcard-net-chip">GG ' + esc(gg) + '</span>' +
                '<span class="mcard-net-chip">[C01..C20]</span>' +
              '</div>' +
              '<div class="mrow-actions">' +
                '<button type="button" class="btn btn-cyan mrow-act-btn" data-open-redbida-edit="' + esc(boxId) + '">' + svgIcon('unlock', 'ui-icon-sm') + '<span>Mở Khóa</span></button>' +
                '<button type="button" class="btn btn-primary mrow-act-btn" data-autofix-redbida="' + esc(boxId) + '">' + svgIcon('zap', 'ui-icon-sm') + '<span>Auto-Fix 17</span></button>' +
                '<button type="button" class="btn mrow-act-btn" data-open-box="' + esc(boxId) + '">Chi tiết</button>' +
              '</div>' +
            '</div>' +
          '</article>';
        }).join('');

        boxContainer.innerHTML =
          '<div class="table-scroll desktop-only-table"><table class="scada-table tabular-nums">' +
            '<thead><tr>' +
              '<th>Box ID / CLB</th>' +
              '<th>Thương Hiệu (Branding Guard)</th>' +
              '<th>Điểm 17-Rule</th>' +
              '<th>Khóa Mở Kiểm Soát (VIP / Group / GGCode / C01..C20)</th>' +
              '<th>Thao Tác</th>' +
            '</tr></thead>' +
            '<tbody>' + boxDesktopRows + '</tbody>' +
          '</table></div>' +
          '<div class="mobile-card-list compact-list mobile-only-cards">' + boxMobileCards + '</div>' +
          renderPaginationBar('redbidaBoxes', boxPg, 'Box');
      }
    }
  }

  function openRedbidaModal(boxId) {
    var item = (state.redbidaBoxes || []).find(function (b) { return (b.box_id || b.id) === boxId; }) || { box_id: boxId };
    var keys = item.keys || {};
    document.getElementById('rb-edit-box-id').value = boxId;
    document.getElementById('redbida-modal-sub').textContent = boxId + ' — ' + (item.club_name || item.box_name || '');
    document.getElementById('rb-edit-group-key').value = keys.shinobi_group_key || item.shinobi_group_key || '';
    document.getElementById('rb-edit-ggcode').value = keys.ggcode || item.ggcode || 'INUT-2026';
    document.getElementById('rb-edit-eth0-vip').value = keys.eth0_virtual_ip || item.virtual_ip || '192.168.1.250';
    document.getElementById('rb-edit-end0-vip').value = keys.end0_virtual_ip || item.virtual_ip || '192.168.1.250';
    document.getElementById('rb-edit-c01').value = keys['[C01]'] || keys.C01 || 'Bàn 01 - H.265';
    document.getElementById('rb-edit-branding').value = (item.branding || 'billiards').toUpperCase();
    document.getElementById('rb-edit-frpc').value = keys.frpc_config || ('subdomain = ' + (item.store_domain || 'store-bida').split('.')[0]);

    var modal = document.getElementById('redbida-editor-modal');
    if (modal) modal.classList.add('open');
  }

  function submitRedbidaApply(boxId, autoFixOnly) {
    if (!boxId) return;
    var payload = {
      auto_fix: true,
      unlocked: true,
      keys: autoFixOnly ? {} : {
        shinobi_group_key: document.getElementById('rb-edit-group-key').value,
        ggcode: document.getElementById('rb-edit-ggcode').value,
        eth0_virtual_ip: document.getElementById('rb-edit-eth0-vip').value,
        end0_virtual_ip: document.getElementById('rb-edit-end0-vip').value,
        frpc_config: document.getElementById('rb-edit-frpc').value,
        '[C01]': document.getElementById('rb-edit-c01').value
      }
    };
    apiFetch('/fleet/api/boxes/' + encodeURIComponent(boxId) + '/redbida/apply', {
      method: 'POST',
      body: payload
    }).then(function () {
      showToast('Đã áp dụng RedBida 17-Rule & bảo toàn Branding cho ' + boxId);
      var modal = document.getElementById('redbida-editor-modal');
      if (modal) modal.classList.remove('open');
      loadRedbidaView();
      loadFleetSummary();
    }).catch(function (err) {
      showToast('Lỗi cập nhật RedBida: ' + err.message, true);
    });
  }

  /* --------------------------------------------------------------------------
     View 6: #handover — 1-Click Handover Pack Generator + Paginated Club Quick-Pick + SVG QR
     -------------------------------------------------------------------------- */
  function loadHandoverView(clubId) {
    var url = clubId ? ('/fleet/api/handover/' + encodeURIComponent(clubId)) : '/fleet/api/handover';
    return apiFetch(url).then(function (data) {
      if (Array.isArray(data.packs) && data.packs.length > 0) {
        data.packs.forEach(function (p) {
          indexItemSearch(p, [
            p.club_id, p.id, p.club_name, p.name, p.store_url, p.cstore_url,
            p.box_id, Array.isArray(p.box_ids) ? p.box_ids.join(' ') : ''
          ]);
        });
        state.handoverPacks = data.packs;
      }
      var pack = data.pack || (state.handoverPacks && state.handoverPacks[0]) || data;
      state.activeHandoverPack = pack;
      renderHandoverView();
    }).catch(function (err) {
      showToast('Lỗi tải Gói Bàn Giao: ' + err.message, true);
    });
  }

  function renderHandoverClubPicker() {
    var picker = document.getElementById('handover-club-picker');
    var packs = state.handoverPacks || [];
    var pack = state.activeHandoverPack || packs[0] || {};
    var currentId = pack.club_id || pack.id || '';
    var tokens = parseSearchQuery(state.handoverSearch);

    var filtered = packs.filter(function (p) {
      return matchesSearch(p, tokens, [p.club_id, p.id, p.club_name, p.name, p.store_url, p.cstore_url]);
    });

    syncSearchBoxState('handover-club-search', 'handover-match-count', filtered.length, packs.length);
    if (!picker) return;

    if (!packs.length) {
      picker.innerHTML = '';
      return;
    }

    if (!filtered.length) {
      picker.innerHTML = '<div class="empty-state" style="padding:14px;">' +
        '<div><strong>Không tìm thấy CLB bàn giao nào khớp từ khóa.</strong></div>' +
      '</div>';
      return;
    }

    var pg = paginateItems(filtered, 'handoverClubs');
    var pillsHtml = pg.items.map(function (p) {
      var pid = p.club_id || p.id || '';
      var isAct = pid === currentId;
      var domain = String(p.store_url || '').replace(/^https?:\/\//i, '');
      var boxCnt = Array.isArray(p.box_ids) ? p.box_ids.length : (p.boxes_count || 0);
      var subText = (boxCnt > 0 ? (boxCnt + ' Box · ') : '') + (domain || pid);
      return '<button type="button" class="handover-club-pill' + (isAct ? ' active' : '') + '" data-pick-handover-club="' + esc(pid) + '">' +
        '<span class="handover-club-pill-name">' + esc(p.club_name || p.name || pid) + '</span>' +
        '<span class="handover-club-pill-sub mono">' + esc(subText) + '</span>' +
      '</button>';
    }).join('');

    picker.innerHTML =
      '<div class="handover-club-grid">' + pillsHtml + '</div>' +
      renderPaginationBar('handoverClubs', pg, 'CLB');
  }

  function renderHandoverView() {
    var sel = document.getElementById('handover-club-select');
    var packs = state.handoverPacks || [];
    var pack = state.activeHandoverPack || packs[0] || {};

    if (sel && packs.length > 0) {
      var currentId = pack.club_id || pack.id || '';
      sel.innerHTML = packs.map(function (p) {
        var pid = p.club_id || p.id || '';
        return '<option value="' + esc(pid) + '" ' + (pid === currentId ? 'selected' : '') + '>' +
          esc(p.club_name || p.name || pid) +
        '</option>';
      }).join('');
    }

    renderHandoverClubPicker();

    var storeUrl = pack.store_url || 'https://store-bida.video.io.vn';
    var cstoreUrl = pack.cstore_url || 'https://c-store-bida.video.io.vn';
    var portalUrl = pack.portal_url || ('/portal/' + (pack.portal_token || '') + '/');
    var clipUrl = pack.clip_url || ('/clip/' + (pack.clip_token || ''));

    var templateText = pack.zalo_template || pack.template || [
      'BAN GIAO HE THONG CAMERA & DIEM SO INUT BIDA — ' + (pack.club_name || 'CLB Bida'),
      '--------------------------------------------------',
      '1. Trang Xem Ban & Diem So: ' + storeUrl,
      '2. Trang Xem Camera & Tua Lai Video 72h: ' + cstoreUrl,
      '3. Cong Quan Ly Danh Rieng Cho Chu Quan (Read-Only): ' + portalUrl,
      '4. Link Tai Clip QR Ben Vung (HMAC): ' + clipUrl,
      '--------------------------------------------------',
      'He thong da chuan hoa 8 buoc Golden Hardening (H.265 30fps, Fragmented MP4 chong loi mat dien, tu dong don the nho 90%->80%).'
    ].join('\n');

    var txtEl = document.getElementById('handover-template-text');
    if (txtEl) txtEl.value = templateText;

    var setLink = function (id, href, label) {
      var a = document.getElementById(id);
      if (!a) return;
      a.href = href;
      if (label !== undefined) a.textContent = label;
    };
    setLink('handover-store-link', storeUrl, storeUrl);
    setLink('handover-cstore-link', cstoreUrl, cstoreUrl);
    setLink('handover-portal-link', portalUrl, portalUrl);
    setLink('handover-open-portal-btn', portalUrl);
    setLink('handover-clip-link', clipUrl, clipUrl);

    renderSafeQrInto('qr-store-container', pack.qr_store_svg, storeUrl);
    renderSafeQrInto('qr-cstore-container', pack.qr_cstore_svg, cstoreUrl);
    renderSafeQrInto('qr-portal-container', pack.qr_portal_svg, portalUrl);
  }

  /* --------------------------------------------------------------------------
     View 7: #analytics — Flagship GA4 Analytics Console (Realtime, Compare, Funnel, 7x24 Heatmap & Drill-Down)
     -------------------------------------------------------------------------- */
  var GA_TREND_METRICS = {
    unique_visitors: { label: 'Người dùng', color: 'var(--emerald)', deltaKey: 'unique_visitors' },
    sessions:        { label: 'Số phiên',   color: 'var(--cyan)',    deltaKey: 'sessions' },
    qr_views:        { label: 'Quét QR',    color: 'var(--cyan)',    deltaKey: 'qr_views' },
    downloads_cuts:  { label: 'Tải & Cắt',  color: 'var(--brand-amber)', deltaKey: 'clips_total' },
    ai_analyzes:     { label: 'AI Studio',  color: 'var(--purple)',  deltaKey: 'ai_analyzes' }
  };

  function populateAnalyticsClubFilter(extraClubs) {
    var sel = document.getElementById('analytics-club-filter');
    if (!sel) return;
    var curVal = state.analyticsClubId || sel.value || '';
    var seen = {};
    var list = [];

    function addClub(cid, name) {
      if (!cid || seen[cid]) return;
      seen[cid] = true;
      list.push({ id: cid, name: name || cid });
    }

    (state.clubs || []).forEach(function (c) {
      addClub(c.id || c.club_id, c.name || c.club_name);
    });
    (state.boxes || []).forEach(function (b) {
      addClub(b.club_id, b.club_name);
    });
    (extraClubs || []).forEach(function (c) {
      addClub(c.club_id || c.id, c.club_name || c.name);
    });

    list.sort(function (a, b) {
      return String(a.name).localeCompare(String(b.name), 'vi');
    });

    var sig = list.map(function (c) { return c.id + ':' + c.name; }).join('|');
    if (sig === state.lastAnalyticsClubSig && sel.options.length > 1) {
      if (sel.value !== curVal) sel.value = curVal;
      return;
    }
    state.lastAnalyticsClubSig = sig;

    sel.innerHTML = '<option value="">Tất cả CLB (' + list.length + ')</option>' +
      list.map(function (c) {
        return '<option value="' + esc(c.id) + '">' + esc(c.name) + '</option>';
      }).join('');
    sel.value = curVal;
  }

  function resolveAnalyticsClubName(clubId) {
    if (!clubId) return '';
    var ov = state.analyticsOverview || {};
    var topClubs = Array.isArray(ov.top_clubs) ? ov.top_clubs : [];
    for (var i = 0; i < topClubs.length; i++) {
      if (topClubs[i].club_id === clubId) return topClubs[i].club_name || clubId;
    }
    var clubs = state.clubs || [];
    for (var j = 0; j < clubs.length; j++) {
      if ((clubs[j].id || clubs[j].club_id) === clubId) return clubs[j].name || clubs[j].club_name || clubId;
    }
    var boxes = state.boxes || [];
    for (var k = 0; k < boxes.length; k++) {
      if (boxes[k].club_id === clubId && boxes[k].club_name) return boxes[k].club_name;
    }
    return clubId;
  }

  function buildAnalyticsQueryParams(opts) {
    var cfg = opts || {};
    var parts = [];
    if (cfg.includeRange !== false) {
      if (state.analyticsRange === 'custom' && state.analyticsFrom && state.analyticsTo) {
        parts.push('from=' + encodeURIComponent(state.analyticsFrom));
        parts.push('to=' + encodeURIComponent(state.analyticsTo));
      } else {
        parts.push('range=' + encodeURIComponent(state.analyticsRange || '7d'));
      }
    }
    var effClub = cfg.clubId !== undefined ? cfg.clubId : (state.analyticsClubId || '');
    var effBox = cfg.boxId !== undefined ? cfg.boxId : (state.analyticsBoxId || '');
    if (effClub) parts.push('club_id=' + encodeURIComponent(effClub));
    if (effBox) parts.push('box_id=' + encodeURIComponent(effBox));
    if (state.analyticsSport && state.analyticsSport !== 'all') {
      parts.push('sport=' + encodeURIComponent(state.analyticsSport));
    }
    if (cfg.includeSource !== false && state.analyticsSource && state.analyticsSource !== 'all') {
      parts.push('source=' + encodeURIComponent(state.analyticsSource));
    }
    if (cfg.includeAi !== false && state.analyticsAiOnly) {
      parts.push('ai_only=1');
    }
    if (cfg.includeCompare && state.analyticsCompare) {
      parts.push('compare=1');
    }
    if (cfg.limit) {
      parts.push('limit=' + encodeURIComponent(String(cfg.limit)));
    }
    return parts.join('&');
  }

  function updateAnalyticsExportLink() {
    var link = document.getElementById('analytics-export-csv');
    if (!link) return;
    var effClub = state.analyticsDrillClubId || state.analyticsClubId || '';
    var effBox = state.analyticsDrillBoxId || state.analyticsBoxId || '';
    var qs = buildAnalyticsQueryParams({ clubId: effClub, boxId: effBox, includeCompare: false });
    link.href = '/fleet/api/analytics/export.csv?' + qs;
  }

  function syncAnalyticsBreadcrumb() {
    var bar = document.getElementById('analytics-breadcrumb-bar');
    var trail = document.getElementById('analytics-breadcrumb-trail');
    var drillInd = document.getElementById('analytics-drill-indicator');
    var drillLbl = document.getElementById('analytics-drill-label');

    var activeClub = state.analyticsClubId || state.analyticsDrillClubId || '';
    var activeBox = state.analyticsBoxId || state.analyticsDrillBoxId || '';
    var clubLabel = state.analyticsDrillLabel || resolveAnalyticsClubName(activeClub);

    if (bar && trail) {
      if (activeClub || activeBox) {
        var html = '<button type="button" class="ga-crumb-btn" data-ga-crumb="all">Toàn hệ thống</button>';
        if (activeClub) {
          html += '<span class="ga-crumb-sep" aria-hidden="true">›</span>';
          if (activeBox) {
            html += '<button type="button" class="ga-crumb-btn" data-ga-crumb="club" data-crumb-club="' + esc(activeClub) + '">' + esc(resolveAnalyticsClubName(activeClub)) + '</button>';
          } else {
            html += '<span class="ga-crumb-current">' + esc(clubLabel || activeClub) + '</span>';
          }
        }
        if (activeBox) {
          html += '<span class="ga-crumb-sep" aria-hidden="true">›</span><span class="ga-crumb-current mono">' + esc(activeBox) + '</span>';
        }
        trail.innerHTML = html;
        bar.hidden = false;
      } else {
        bar.hidden = true;
        trail.innerHTML = '';
      }
    }

    if (drillInd && drillLbl) {
      if (activeClub || activeBox) {
        drillLbl.textContent = 'Đang lọc: ' + (activeBox || clubLabel || activeClub);
        drillInd.style.display = 'inline-flex';
      } else {
        drillInd.style.display = 'none';
      }
    }
  }

  function drillDownAnalytics(clubId, boxId, label) {
    state.analyticsClubId = clubId || '';
    state.analyticsBoxId = boxId || '';
    state.analyticsDrillClubId = clubId || '';
    state.analyticsDrillBoxId = boxId || '';
    state.analyticsDrillLabel = label || resolveAnalyticsClubName(clubId) || boxId || '';

    var clubSel = document.getElementById('analytics-club-filter');
    if (clubSel) clubSel.value = state.analyticsClubId;

    if (clubId && !boxId) {
      state.analyticsRtHotTab = 'tables';
      syncRtHotTabsUi();
    }
    resetTabPage('analyticsTop');
    resetTabPage('analyticsCameras');
    syncAnalyticsBreadcrumb();
    loadAnalyticsView();
  }

  function clearAnalyticsDrillDown() {
    state.analyticsClubId = '';
    state.analyticsBoxId = '';
    state.analyticsDrillClubId = '';
    state.analyticsDrillBoxId = '';
    state.analyticsDrillLabel = '';
    state.analyticsRtHotTab = 'clubs';
    syncRtHotTabsUi();

    var clubSel = document.getElementById('analytics-club-filter');
    if (clubSel) clubSel.value = '';

    resetTabPage('analyticsTop');
    resetTabPage('analyticsCameras');
    syncAnalyticsBreadcrumb();
    loadAnalyticsView();
  }

  function syncRtHotTabsUi() {
    var tabs = document.getElementById('ga-rt-hot-tabs');
    if (!tabs) return;
    tabs.querySelectorAll('.segmented-btn[data-rt-hot]').forEach(function (btn) {
      btn.classList.toggle('active', btn.getAttribute('data-rt-hot') === state.analyticsRtHotTab);
    });
  }

  function fmtShortDateVN(iso) {
    var s = String(iso || '');
    if (s.length >= 10) return s.slice(8, 10) + '/' + s.slice(5, 7);
    return s;
  }

  function fmtTimeNowHM(iso) {
    if (!iso) {
      var now = new Date();
      var pad0 = function (n) { return (n < 10 ? '0' : '') + n; };
      return pad0(now.getHours()) + ':' + pad0(now.getMinutes()) + ':' + pad0(now.getSeconds());
    }
    var d = new Date(iso);
    if (isNaN(d.getTime())) return String(iso).slice(11, 19) || '--';
    var pad = function (n) { return (n < 10 ? '0' : '') + n; };
    return pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
  }

  /* --------------------------------------------------------------------------
     GA4 Realtime Poller (ONLY active when #view-analytics is active & visible)
     -------------------------------------------------------------------------- */
  function stopAnalyticsRealtimePolling() {
    if (state.analyticsRtTimer) {
      clearInterval(state.analyticsRtTimer);
      state.analyticsRtTimer = null;
    }
  }

  function startAnalyticsRealtimePolling(fetchImmediately) {
    stopAnalyticsRealtimePolling();
    if (state.activeView !== 'analytics' || document.hidden) return;
    if (fetchImmediately !== false) loadAnalyticsRealtime();
    state.analyticsRtTimer = setInterval(function () {
      if (state.activeView !== 'analytics' || document.hidden) {
        stopAnalyticsRealtimePolling();
        return;
      }
      loadAnalyticsRealtime();
    }, 15000);
  }

  function loadAnalyticsRealtime() {
    if (state.activeView !== 'analytics') return Promise.resolve();
    var seq = ++state.analyticsRtReqSeq;
    var qs = buildAnalyticsQueryParams({ includeRange: false, includeSource: false, includeAi: false, includeCompare: false });
    return apiFetch('/fleet/api/analytics/realtime' + (qs ? ('?' + qs) : '')).then(function (data) {
      if (seq !== state.analyticsRtReqSeq) return;
      state.analyticsRtData = data || {};
      renderAnalyticsRealtime();
    }).catch(function () {
      if (seq !== state.analyticsRtReqSeq) return;
      var upd = document.getElementById('ga-rt-updated');
      if (upd) upd.textContent = 'Mất kết nối tạm thời · thử lại sau 15s';
    });
  }

  function renderAnalyticsRealtime() {
    var rt = state.analyticsRtData || {};
    var a5 = Number(rt.active_5m) || 0;
    var a30 = Number(rt.active_30m) || 0;
    var live = Number(rt.live) || 0;
    var vod = Number(rt.vod) || 0;
    var cut = Number(rt.cut) || 0;
    var ai = Number(rt.ai) || 0;
    var activeClubs = Number(rt.active_clubs) || 0;
    var activeBoxes = Number(rt.active_boxes) || 0;

    var dot = document.getElementById('ga-rt-live-dot');
    if (dot) dot.classList.toggle('idle', a5 === 0 && a30 === 0);

    var scopeBadge = document.getElementById('ga-rt-scope-badge');
    if (scopeBadge) {
      scopeBadge.className = 'badge ' + (activeBoxes > 0 ? 'badge-emerald' : 'badge-neutral');
      scopeBadge.textContent = fmtInt(activeClubs) + ' CLB · ' + fmtInt(activeBoxes) + ' Box';
    }

    var updEl = document.getElementById('ga-rt-updated');
    if (updEl) {
      updEl.textContent = 'Cập nhật ' + fmtTimeNowHM(rt.updated_at) + ' · 15s';
    }

    var elA5 = document.getElementById('ga-rt-active5');
    var elA30 = document.getElementById('ga-rt-active30-pill');
    var elStatus = document.getElementById('ga-rt-active-status');
    if (elA5) elA5.textContent = fmtInt(a5);
    if (elA30) elA30.textContent = fmtInt(a30) + ' người (30 phút)';
    if (elStatus) {
      elStatus.textContent = a5 > 0
        ? 'Đang có khách tương tác trực tiếp'
        : (a30 > 0 ? 'Có khách trong 30 phút gần nhất' : 'Chưa có khách trong 5 phút qua');
    }

    var elLive = document.getElementById('ga-rt-live');
    var elVod = document.getElementById('ga-rt-vod');
    var elCut = document.getElementById('ga-rt-cut');
    var elAi = document.getElementById('ga-rt-ai');
    if (elLive) elLive.textContent = fmtInt(live);
    if (elVod) elVod.textContent = fmtInt(vod);
    if (elCut) elCut.textContent = fmtInt(cut);
    if (elAi) elAi.textContent = fmtInt(ai);

    var navBadge = document.getElementById('nav-badge-analytics');
    if (navBadge && a5 > 0) {
      navBadge.textContent = fmtCompact(a5) + ' LIVE';
    }

    /* Pure Inline SVG 30-Bar Minute Chart (-30p -> Hiện tại) */
    var rawBars = Array.isArray(rt.minute_bars_30) ? rt.minute_bars_30 : [];
    var bars30 = [];
    for (var i = 0; i < 30; i++) {
      var srcIdx = i - (30 - rawBars.length);
      bars30.push(srcIdx >= 0 ? (Number(rawBars[srcIdx]) || 0) : 0);
    }
    var maxBar = 0;
    bars30.forEach(function (v) { if (v > maxBar) maxBar = v; });

    var peakEl = document.getElementById('ga-rt-bar-peak');
    if (peakEl) peakEl.textContent = 'Đỉnh: ' + fmtInt(maxBar) + '/p';

    var chartWrap = document.getElementById('ga-rt-minute-chart');
    if (chartWrap) {
      var W = 420;
      var H = 192;
      var padL = 28;
      var padR = 8;
      var padT = 12;
      var padB = 22;
      var plotW = W - padL - padR;
      var plotH = H - padT - padB;
      var yMax = maxBar > 0 ? Math.max(2, Math.ceil(maxBar * 1.15)) : 4;
      var slotW = plotW / 30;
      var barW = Math.max(5.5, slotW - 3.2);
      var yBase = padT + plotH;

      var gridHtml = '';
      [0, 0.33, 0.66, 1].forEach(function (frac, gi) {
        var y = yBase - plotH * frac;
        var val = Math.round(yMax * frac);
        gridHtml += '<line x1="' + padL + '" y1="' + y.toFixed(1) + '" x2="' + (W - padR) + '" y2="' + y.toFixed(1) + '" stroke="var(--border)" stroke-width="1"' + (gi > 0 ? ' stroke-dasharray="3 3"' : '') + '/>' +
          '<text x="' + (padL - 5) + '" y="' + (y + 3.5).toFixed(1) + '" text-anchor="end" font-size="9.5" font-family="var(--font-mono)" fill="var(--text-muted)">' + esc(fmtCompact(val)) + '</text>';
      });

      var barsHtml = bars30.map(function (val, idx) {
        var minsAgo = 29 - idx;
        var minLabel = minsAgo === 0 ? 'Phút hiện tại' : ('-' + minsAgo + ' phút trước');
        var xSlot = padL + idx * slotW;
        var xBar = xSlot + (slotW - barW) / 2;
        var hBar = val > 0 ? Math.max(4, (val / yMax) * plotH) : 2;
        var yBar = yBase - hBar;
        var isCurrent = idx === 29;
        var isPeak = val > 0 && val === maxBar;
        var fill = val === 0
          ? 'var(--border)'
          : (isCurrent ? 'var(--emerald)' : (isPeak ? 'var(--brand-amber)' : 'var(--cyan)'));
        var tipBody = 'Hoạt động: ' + fmtInt(val) + ' người';

        return '<g class="svg-bar-group" tabindex="0" data-ga-tip-title="' + esc(minLabel) + '" data-ga-tip-body="' + esc(tipBody) + '">' +
          '<title>' + esc(minLabel + ': ' + fmtInt(val) + ' người') + '</title>' +
          '<rect class="svg-hover-bg" x="' + xSlot.toFixed(1) + '" y="' + padT + '" width="' + slotW.toFixed(1) + '" height="' + plotH + '" rx="2" fill="transparent"/>' +
          '<rect x="' + xBar.toFixed(1) + '" y="' + yBar.toFixed(1) + '" width="' + barW.toFixed(1) + '" height="' + hBar.toFixed(1) + '" rx="2" fill="' + fill + '"/>' +
        '</g>';
      }).join('');

      var axisTicks = [
        { idx: 0, label: '-30p', anchor: 'start' },
        { idx: 10, label: '-20p', anchor: 'middle' },
        { idx: 20, label: '-10p', anchor: 'middle' },
        { idx: 29, label: 'Bây giờ', anchor: 'end' }
      ].map(function (tk) {
        var x = tk.idx === 0 ? padL : (tk.idx === 29 ? (W - padR) : (padL + (tk.idx + 0.5) * slotW));
        return '<text x="' + x.toFixed(1) + '" y="' + (H - 5) + '" text-anchor="' + tk.anchor + '" font-size="9.5" font-family="var(--font-mono)" fill="' + (tk.idx === 29 ? 'var(--emerald)' : 'var(--text-muted)') + '" font-weight="' + (tk.idx === 29 ? '700' : '400') + '">' + tk.label + '</text>';
      }).join('');

      chartWrap.innerHTML = '<svg class="analytics-svg" viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="Biểu đồ 30 cột từng phút người đang xem">' +
        gridHtml + barsHtml + axisTicks +
      '</svg>';
    }

    /* Top 5 Hot Clubs / Tables */
    var hotListEl = document.getElementById('ga-rt-hot-list');
    if (hotListEl) {
      var hotClubs = Array.isArray(rt.hot_clubs) ? rt.hot_clubs.slice(0, 5) : [];
      var hotTables = Array.isArray(rt.hot_tables) ? rt.hot_tables.slice(0, 5) : [];

      var tabMode = state.analyticsRtHotTab;
      if (tabMode === 'clubs' && !hotClubs.length && hotTables.length) tabMode = 'tables';
      if (tabMode === 'tables' && !hotTables.length && hotClubs.length) tabMode = 'clubs';

      if (tabMode === 'tables') {
        if (!hotTables.length) {
          hotListEl.innerHTML = '<div class="ga-rt-hot-empty">Chưa có bàn nào có khách trong 5 phút qua.</div>';
        } else {
          var maxT = Math.max(1, Number(hotTables[0].active) || 1);
          hotListEl.innerHTML = hotTables.map(function (t) {
            var n = Number(t.active) || 0;
            var pct = Math.max(8, Math.round((n / maxT) * 100));
            return '<button type="button" class="ga-rt-hot-item" data-ga-drill-club="' + esc(t.club_id || '') + '" data-ga-drill-box="' + esc(t.box_id || '') + '" data-ga-drill-label="' + esc(t.club_name || t.box_name || t.box_id || '') + '">' +
              '<div class="ga-rt-hot-top">' +
                '<span class="ga-rt-hot-name">' + esc(t.table_name || t.camera_id || 'Bàn') + '</span>' +
                '<span class="badge badge-emerald mono">' + esc(fmtInt(n)) + ' xem</span>' +
              '</div>' +
              '<div class="ga-rt-hot-sub">' + esc(t.club_name || t.club_id || '--') + ' · <span class="mono">' + esc(t.camera_id || '') + '</span></div>' +
              '<div class="ga-rt-hot-bar"><div class="ga-rt-hot-bar-fill" style="width:' + pct + '%"></div></div>' +
            '</button>';
          }).join('');
        }
      } else {
        if (!hotClubs.length) {
          hotListEl.innerHTML = '<div class="ga-rt-hot-empty">Chưa có quán nào có khách trong 5 phút qua.</div>';
        } else {
          var maxC = Math.max(1, Number(hotClubs[0].a5) || Number(hotClubs[0].a30) || 1);
          hotListEl.innerHTML = hotClubs.map(function (c) {
            var n5 = Number(c.a5) || 0;
            var n30 = Number(c.a30) || 0;
            var pct = Math.max(8, Math.round(((n5 || n30) / maxC) * 100));
            var subBits = ['30p: ' + fmtInt(n30)];
            if (Number(c.live) > 0) subBits.push('Live ' + fmtInt(c.live));
            if (Number(c.cut) > 0) subBits.push('Cắt ' + fmtInt(c.cut));
            if (Number(c.ai) > 0) subBits.push('AI ' + fmtInt(c.ai));
            return '<button type="button" class="ga-rt-hot-item" data-ga-drill-club="' + esc(c.club_id || '') + '" data-ga-drill-box="" data-ga-drill-label="' + esc(c.club_name || c.club_id || '') + '">' +
              '<div class="ga-rt-hot-top">' +
                '<span class="ga-rt-hot-name">' + esc(c.club_name || c.club_id || '--') + '</span>' +
                '<span class="badge badge-emerald mono">' + esc(fmtInt(n5)) + ' xem</span>' +
              '</div>' +
              '<div class="ga-rt-hot-sub mono">' + esc(subBits.join(' · ')) + '</div>' +
              '<div class="ga-rt-hot-bar"><div class="ga-rt-hot-bar-fill" style="width:' + pct + '%"></div></div>' +
            '</button>';
          }).join('');
        }
      }
    }
  }

  /* --------------------------------------------------------------------------
     GA4 Overview & Cameras Fetchers
     -------------------------------------------------------------------------- */
  function loadAnalyticsView() {
    populateAnalyticsClubFilter();
    updateAnalyticsExportLink();
    syncAnalyticsBreadcrumb();

    var seq = ++state.analyticsReqSeq;
    var qs = buildAnalyticsQueryParams({ includeCompare: true });

    var pOverview = apiFetch('/fleet/api/analytics/overview?' + qs).then(function (data) {
      if (seq !== state.analyticsReqSeq) return;
      state.analyticsOverview = data || {};
      if (Array.isArray(data && data.top_clubs) && data.top_clubs.length) {
        populateAnalyticsClubFilter(data.top_clubs);
      }
      renderAnalyticsOverview();
    }).catch(function (err) {
      if (seq !== state.analyticsReqSeq) return;
      showToast('Không thể tải thống kê Analytics: ' + err.message, true);
    });

    var pCameras = loadAnalyticsCameras();
    var pRt = state.activeView === 'analytics' ? loadAnalyticsRealtime() : Promise.resolve();
    return Promise.all([pOverview, pCameras, pRt]);
  }

  function loadAnalyticsCameras() {
    updateAnalyticsExportLink();
    var seq = ++state.analyticsCamReqSeq;
    var effClub = state.analyticsDrillClubId || state.analyticsClubId || '';
    var effBox = state.analyticsDrillBoxId || state.analyticsBoxId || '';

    var qs = buildAnalyticsQueryParams({
      clubId: effClub,
      boxId: effBox,
      includeSource: false,
      includeAi: false,
      includeCompare: false,
      limit: 200
    });

    return apiFetch('/fleet/api/analytics/cameras?' + qs).then(function (data) {
      if (seq !== state.analyticsCamReqSeq) return;
      var list = Array.isArray(data && data.cameras) ? data.cameras : [];
      list.forEach(function (c) {
        indexItemSearch(c, [c.table_name, c.camera_id, c.club_name, c.club_id, c.box_name, c.box_id]);
      });
      state.analyticsCameras = list;
      renderAnalyticsCamerasTable();
    }).catch(function (err) {
      if (seq !== state.analyticsCamReqSeq) return;
      state.analyticsCameras = [];
      renderAnalyticsCamerasTable();
      showToast('Không thể tải Top Bàn / Camera: ' + err.message, true);
    });
  }

  /* --------------------------------------------------------------------------
     GA4 KPI Cards (Delta % Pills + Mini Inline SVG Sparklines)
     -------------------------------------------------------------------------- */
  function renderDeltaPill(elId, deltaVal, show) {
    var el = document.getElementById(elId);
    if (!el) return;
    if (!show || deltaVal === null || deltaVal === undefined || isNaN(Number(deltaVal))) {
      el.hidden = true;
      el.textContent = '';
      el.className = 'ga-delta-pill';
      return;
    }
    var v = Number(deltaVal);
    var cls = 'flat';
    var txt = '0.0%';
    if (v > 0.05) {
      cls = 'up';
      txt = '+' + v.toFixed(1) + '% ↑';
    } else if (v < -0.05) {
      cls = 'down';
      txt = v.toFixed(1) + '% ↓';
    }
    el.className = 'ga-delta-pill ' + cls;
    el.textContent = txt;
    el.hidden = false;
  }

  function renderMiniSparkline(elId, values, colorVar) {
    var el = document.getElementById(elId);
    if (!el) return;
    var arr = Array.isArray(values) ? values.map(function (v) { return Number(v) || 0; }) : [];
    if (!arr.length) {
      el.innerHTML = '';
      return;
    }
    var W = 66;
    var H = 24;
    var pad = 2;
    var maxV = 0;
    var minV = Infinity;
    arr.forEach(function (v) {
      if (v > maxV) maxV = v;
      if (v < minV) minV = v;
    });
    if (arr.length === 1) {
      var ySingle = maxV > 0 ? (H / 2) : (H - 3);
      el.innerHTML = '<svg viewBox="0 0 ' + W + ' ' + H + '"><line x1="2" y1="' + ySingle + '" x2="' + (W - 2) + '" y2="' + ySingle + '" stroke="' + colorVar + '" stroke-width="1.8" stroke-linecap="round"/><circle cx="' + (W - 3) + '" cy="' + ySingle + '" r="2.2" fill="' + colorVar + '"/></svg>';
      return;
    }
    var span = maxV - minV;
    var pts = arr.map(function (v, idx) {
      var x = pad + (idx / (arr.length - 1)) * (W - pad * 2);
      var y = maxV === 0
        ? (H - 3)
        : (span === 0 ? (H / 2) : (H - pad - ((v - minV) / span) * (H - pad * 2 - 2)));
      return x.toFixed(1) + ',' + y.toFixed(1);
    });
    var lastPt = pts[pts.length - 1].split(',');
    el.innerHTML = '<svg viewBox="0 0 ' + W + ' ' + H + '">' +
      '<polyline fill="none" stroke="' + colorVar + '" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" points="' + pts.join(' ') + '"/>' +
      '<circle cx="' + lastPt[0] + '" cy="' + lastPt[1] + '" r="2.1" fill="' + colorVar + '"/>' +
    '</svg>';
  }

  function renderAnalyticsOverview() {
    var data = state.analyticsOverview || {};
    var kpi = data.kpi || {};
    var cohorts = data.cohorts || {};
    var comp = (state.analyticsCompare && data.comparison) ? data.comparison : null;
    var deltas = (comp && comp.deltas_pct) ? comp.deltas_pct : {};
    var series = Array.isArray(data.daily_series) ? data.daily_series : [];

    var fromInp = document.getElementById('analytics-date-from');
    var toInp = document.getElementById('analytics-date-to');
    if (fromInp && data.from_date && document.activeElement !== fromInp) fromInp.value = data.from_date;
    if (toInp && data.to_date && document.activeElement !== toInp) toInp.value = data.to_date;

    var winEl = document.getElementById('analytics-date-window');
    if (winEl) {
      if (data.from_date && data.to_date) {
        var winStr = fmtShortDateVN(data.from_date) + ' → ' + fmtShortDateVN(data.to_date);
        if (comp && comp.from && comp.to) {
          winStr += ' vs ' + fmtShortDateVN(comp.from) + ' → ' + fmtShortDateVN(comp.to);
        }
        winEl.textContent = winStr + ' (GMT+7)';
      } else {
        winEl.textContent = 'Khung: ' + (state.analyticsRange || '7d').toUpperCase() + ' (GMT+7)';
      }
    }

    var qrViews = Number(kpi.qr_views) || 0;
    var pageViews = Number(kpi.page_views) || 0;
    var uniqueVisitors = Number(kpi.unique_visitors) || 0;
    var sessions = Number(kpi.sessions || cohorts.sessions) || 0;
    var engRate = Number(cohorts.engagement_rate_pct !== undefined ? cohorts.engagement_rate_pct : kpi.engagement_rate_pct) || 0;
    var viewsPerSess = Number(cohorts.views_per_session) || (sessions > 0 ? ((pageViews || qrViews) / sessions) : 0);
    var retPct = Number(cohorts.returning_pct) || 0;
    var newPct = uniqueVisitors > 0 ? Math.max(0, 100 - retPct) : 0;

    var clipDownloads = Number(kpi.clip_downloads) || 0;
    var cutClips = Number(kpi.cut_clips) || 0;
    var clipsTotal = Number(kpi.clips_total) || (clipDownloads + cutClips);
    var cutSeconds = Number(kpi.cut_seconds) || 0;

    var aiOpens = Number(kpi.ai_studio_opens) || 0;
    var aiAnalyzes = Number(kpi.ai_analyzes) || 0;
    var aiPool = Number(kpi.ai_pool_runs) || 0;
    var aiCarom = Number(kpi.ai_carom_runs) || 0;
    var aiPick = Number(kpi.ai_pickleball_runs) || 0;
    var aiExportsTotal = Number(kpi.ai_exports_total) || ((Number(kpi.ai_exports_ios) || 0) + (Number(kpi.ai_exports_desktop) || 0));

    var aiErrors = Number(kpi.ai_errors) || 0;
    var httpErrors = Number(kpi.http_errors) || 0;
    var totalErrors = httpErrors + aiErrors;
    var errRate = Number(kpi.error_rate_pct) || 0;
    var bytesOut = Number(kpi.bytes_out) || 0;
    var activeClubs = Number(kpi.active_clubs) || 0;
    var activeBoxes = Number(kpi.active_boxes) || 0;

    var totalActivity = qrViews + pageViews + uniqueVisitors + sessions + clipsTotal + aiOpens + aiAnalyzes + bytesOut;

    var navBadge = document.getElementById('nav-badge-analytics');
    if (navBadge) {
      var rtA5 = Number(state.analyticsRtData && state.analyticsRtData.active_5m) || 0;
      if (rtA5 > 0) navBadge.textContent = fmtCompact(rtA5) + ' LIVE';
      else navBadge.textContent = qrViews > 0 ? fmtCompact(qrViews) : 'LIVE';
    }

    var showDelta = !!comp;

    /* KPI 1: Active Users (Mới X% · Quay lại Y%) */
    var visVal = document.getElementById('akpi-visitors-val');
    var visSub = document.getElementById('akpi-visitors-sub');
    if (visVal) visVal.textContent = fmtInt(uniqueVisitors);
    if (visSub) {
      visSub.innerHTML = '<span>Mới <strong>' + esc(newPct.toFixed(0)) + '%</strong></span><span>Quay lại <strong class="text-emerald">' + esc(retPct.toFixed(0)) + '%</strong></span>';
    }
    renderDeltaPill('akpi-visitors-delta', deltas.unique_visitors, showDelta);
    renderMiniSparkline('akpi-visitors-spark', series.map(function (d) { return d.unique_visitors; }), 'var(--emerald)');

    /* KPI 2: Sessions (Tỷ lệ tương tác % · TB trang/phiên) */
    var sessVal = document.getElementById('akpi-sessions-val');
    var sessSub = document.getElementById('akpi-sessions-sub');
    if (sessVal) sessVal.textContent = fmtInt(sessions);
    if (sessSub) {
      sessSub.innerHTML = '<span>Tương tác <strong class="text-cyan">' + esc(engRate.toFixed(1)) + '%</strong></span><span>' + esc(viewsPerSess.toFixed(1)) + ' trang/phiên</span>';
    }
    renderDeltaPill('akpi-sessions-delta', deltas.sessions, showDelta);
    renderMiniSparkline('akpi-sessions-spark', series.map(function (d) { return d.sessions; }), 'var(--cyan)');

    /* KPI 3: QR & Page Views */
    var qrVal = document.getElementById('akpi-qr-val');
    var qrSub = document.getElementById('akpi-qr-sub');
    if (qrVal) qrVal.innerHTML = esc(fmtInt(qrViews)) + '<span class="kpi-val-sub">' + esc(fmtInt(pageViews)) + ' lượt xem trang</span>';
    if (qrSub) qrSub.innerHTML = '<span>' + esc(fmtInt(activeClubs)) + ' CLB · ' + esc(fmtInt(activeBoxes)) + ' Box</span><span>Live ' + esc(fmtInt(kpi.live_hls || 0)) + '</span>';
    renderDeltaPill('akpi-qr-delta', deltas.qr_views, showDelta);
    renderMiniSparkline('akpi-qr-spark', series.map(function (d) { return d.qr_views; }), 'var(--cyan)');

    /* KPI 4: Clip & CheckVAR Conversions */
    var convBase = Math.max(sessions, qrViews, pageViews);
    var convPct = convBase > 0 ? ((clipsTotal / convBase) * 100).toFixed(1) : '0.0';
    var clipVal = document.getElementById('akpi-clips-val');
    var clipSub = document.getElementById('akpi-clips-sub');
    if (clipVal) clipVal.textContent = fmtInt(clipsTotal);
    if (clipSub) {
      clipSub.innerHTML = '<span title="' + esc(fmtCutMinutes(cutSeconds)) + ' cắt">Tải ' + esc(fmtInt(clipDownloads)) + ' · Cắt ' + esc(fmtInt(cutClips)) + '</span><span class="text-emerald" style="font-weight:700;">' + esc(convPct) + '% CĐ</span>';
    }
    renderDeltaPill('akpi-clips-delta', deltas.clips_total, showDelta);
    renderMiniSparkline('akpi-clips-spark', series.map(function (d) { return d.downloads_cuts; }), 'var(--brand-amber)');

    /* KPI 5: AI Studio & Trọng Tài (Pool / Carom / Pickleball · Xuất) */
    var aiVal = document.getElementById('akpi-ai-val');
    var aiSub = document.getElementById('akpi-ai-sub');
    if (aiVal) aiVal.innerHTML = esc(fmtInt(aiAnalyzes)) + '<span class="kpi-val-sub">' + esc(fmtInt(aiOpens)) + ' lượt mở AI</span>';
    if (aiSub) {
      aiSub.innerHTML = '<span>P ' + esc(fmtCompact(aiPool)) + ' · C ' + esc(fmtCompact(aiCarom)) + ' · Pk ' + esc(fmtCompact(aiPick)) + '</span><span>Xuất ' + esc(fmtInt(aiExportsTotal)) + '</span>';
    }
    renderDeltaPill('akpi-ai-delta', deltas.ai_analyzes, showDelta);
    renderMiniSparkline('akpi-ai-spark', series.map(function (d) { return d.ai_analyzes; }), 'var(--purple)');

    /* KPI 6: Bandwidth & Stability (GB · % lỗi) */
    var bwCard = document.getElementById('akpi-bw-card');
    var bwIcon = document.getElementById('akpi-bw-icon');
    var bwVal = document.getElementById('akpi-bw-val');
    var bwSub = document.getElementById('akpi-bw-sub');
    var isErrDanger = totalErrors >= 20 || errRate >= 3;
    var isErrWarn = !isErrDanger && totalErrors > 0;
    if (bwCard) {
      bwCard.classList.toggle('danger', isErrDanger);
      bwCard.classList.toggle('warn', isErrWarn);
    }
    if (bwIcon) {
      bwIcon.className = 'kpi-icon ' + (isErrDanger ? 'crimson' : (isErrWarn ? 'amber' : 'emerald'));
    }
    if (bwVal) bwVal.textContent = fmtBytes(bytesOut);
    if (bwSub) {
      var errTxtCls = isErrDanger ? 'text-crimson' : (isErrWarn ? 'text-amber' : 'text-emerald');
      bwSub.innerHTML = '<span>Lỗi: ' + esc(fmtInt(totalErrors)) + ' (HTTP ' + esc(fmtInt(httpErrors)) + ' · AI ' + esc(fmtInt(aiErrors)) + ')</span><span class="' + errTxtCls + '" style="font-weight:700;">' + esc(errRate.toFixed(2)) + '%</span>';
    }
    renderDeltaPill('akpi-bw-delta', deltas.bytes_out, showDelta);
    renderMiniSparkline('akpi-bw-spark', series.map(function (d) { return d.bytes_out; }), 'var(--cyan)');

    var emptyBanner = document.getElementById('analytics-empty-banner');
    if (emptyBanner) {
      emptyBanner.style.display = totalActivity === 0 ? 'flex' : 'none';
    }

    renderAnalyticsDailyTrend(data);
    renderAnalyticsFunnel(data);
    renderAnalyticsHeatmap(data);
    renderAnalyticsPeakHours(data);
    renderAnalyticsBreakdowns(data);
    renderAnalyticsTopTable(data);
  }

  /* --------------------------------------------------------------------------
     GA4 Visual Panel 1: Metric Switcher Trend Chart (+ Dashed Previous Period)
     -------------------------------------------------------------------------- */
  function renderAnalyticsDailyTrend(data) {
    var wrap = document.getElementById('analytics-daily-chart');
    if (!wrap) return;
    var kpi = data.kpi || {};
    var cohorts = data.cohorts || {};
    var series = Array.isArray(data.daily_series) ? data.daily_series : [];
    var comp = (state.analyticsCompare && data.comparison) ? data.comparison : null;
    var prevSeries = (comp && Array.isArray(comp.daily_series)) ? comp.daily_series : [];

    /* Populate metric switcher tab totals */
    var tabVals = {
      unique_visitors: Number(kpi.unique_visitors) || 0,
      sessions: Number(kpi.sessions || cohorts.sessions) || 0,
      qr_views: Number(kpi.qr_views) || 0,
      downloads_cuts: Number(kpi.clips_total) || ((Number(kpi.clip_downloads) || 0) + (Number(kpi.cut_clips) || 0)),
      ai_analyzes: Number(kpi.ai_analyzes) || 0
    };
    Object.keys(tabVals).forEach(function (k) {
      var el = document.getElementById('ga-tab-val-' + k);
      if (el) el.textContent = fmtCompact(tabVals[k]);
    });

    var metricKey = GA_TREND_METRICS[state.analyticsTrendMetric] ? state.analyticsTrendMetric : 'unique_visitors';
    var meta = GA_TREND_METRICS[metricKey];

    var curLegendLbl = document.getElementById('ga-trend-cur-label');
    var prevLegendEl = document.getElementById('ga-trend-prev-legend');
    var prevLegendLbl = document.getElementById('ga-trend-prev-label');
    var solidLine = document.querySelector('#ga-trend-legend .ga-legend-line.solid');
    if (solidLine) solidLine.style.borderTopColor = meta.color;
    if (curLegendLbl) {
      curLegendLbl.textContent = (data.from_date && data.to_date)
        ? (meta.label + ' (' + fmtShortDateVN(data.from_date) + '–' + fmtShortDateVN(data.to_date) + ')')
        : meta.label;
    }
    if (prevLegendEl && prevLegendLbl) {
      if (comp && prevSeries.length) {
        prevLegendEl.hidden = false;
        prevLegendLbl.textContent = 'Kỳ trước (' + fmtShortDateVN(comp.from) + '–' + fmtShortDateVN(comp.to) + ')';
      } else {
        prevLegendEl.hidden = true;
      }
    }

    var W = 620;
    var H = 228;
    var padL = 44;
    var padR = 14;
    var padT = 16;
    var padB = 28;
    var plotW = W - padL - padR;
    var plotH = H - padT - padB;
    var yBase = padT + plotH;

    var maxVal = 0;
    series.forEach(function (d) {
      var v = Number(d[metricKey]) || 0;
      if (v > maxVal) maxVal = v;
    });
    if (comp) {
      prevSeries.forEach(function (d) {
        var v = Number(d[metricKey]) || 0;
        if (v > maxVal) maxVal = v;
      });
    }

    var yMax = maxVal > 0 ? Math.ceil(maxVal * 1.15) : 4;
    var gridLines = [];
    for (var t = 0; t <= 4; t++) {
      var frac = t / 4;
      var y = padT + plotH * (1 - frac);
      gridLines.push(
        '<line x1="' + padL + '" y1="' + y.toFixed(1) + '" x2="' + (W - padR) + '" y2="' + y.toFixed(1) + '" stroke="var(--border)" stroke-width="1"' + (t > 0 ? ' stroke-dasharray="3 3"' : '') + '/>' +
        '<text x="' + (padL - 8) + '" y="' + (y + 3.5).toFixed(1) + '" text-anchor="end" font-size="10" font-family="var(--font-mono)" fill="var(--text-muted)">' + esc(fmtCompact(Math.round(yMax * frac))) + '</text>'
      );
    }

    if (!series.length || maxVal === 0) {
      wrap.innerHTML = '<svg class="analytics-svg" viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="Xu hướng ' + esc(meta.label) + '">' +
        gridLines.join('') +
        '<text x="' + (padL + plotW / 2).toFixed(1) + '" y="' + (padT + plotH / 2).toFixed(1) + '" text-anchor="middle" font-size="12" fill="var(--text-muted)">Chưa có dữ liệu ' + esc(meta.label.toLowerCase()) + ' trong khung thời gian này</text>' +
      '</svg>';
      return;
    }

    var n = series.length;
    var slotW = plotW / n;
    var curPoints = [];
    var prevPoints = [];
    var groups = [];
    var labelStep = Math.max(1, Math.ceil(n / 8));
    var gradId = 'ga-trend-grad-' + metricKey;

    series.forEach(function (d, idx) {
      var curV = Number(d[metricKey]) || 0;
      var prevRow = (comp && idx < prevSeries.length) ? prevSeries[idx] : null;
      var prevV = prevRow ? (Number(prevRow[metricKey]) || 0) : null;

      var xSlot = padL + idx * slotW;
      var cx = xSlot + slotW / 2;
      var cy = yBase - (curV / yMax) * plotH;
      curPoints.push(cx.toFixed(1) + ',' + cy.toFixed(1));

      var py = null;
      if (prevV !== null) {
        py = yBase - (prevV / yMax) * plotH;
        prevPoints.push(cx.toFixed(1) + ',' + py.toFixed(1));
      }

      var dateStr = String(d.date || '');
      var shortDate = fmtShortDateVN(dateStr);
      var showTick = (idx % labelStep === 0 || idx === n - 1);

      var tipLines = [meta.label + ': ' + fmtInt(curV)];
      if (prevRow) {
        var dPct = prevV > 0 ? (((curV - prevV) / prevV) * 100) : (curV > 0 ? 100 : 0);
        var dSign = dPct > 0 ? '+' : '';
        tipLines.push('Kỳ trước (' + fmtShortDateVN(prevRow.date) + '): ' + fmtInt(prevV) + ' (' + dSign + dPct.toFixed(1) + '%)');
      }
      var tipBody = tipLines.join(' | ');

      var singleBars = '';
      if (n === 1) {
        var bw = Math.min(48, slotW * 0.22);
        var hCur = curV > 0 ? Math.max(4, (curV / yMax) * plotH) : 0;
        singleBars += '<rect x="' + (cx - bw - 4).toFixed(1) + '" y="' + (yBase - hCur).toFixed(1) + '" width="' + bw.toFixed(1) + '" height="' + hCur.toFixed(1) + '" rx="4" fill="' + meta.color + '"/>';
        if (prevV !== null && prevV > 0) {
          var hPrev = Math.max(4, (prevV / yMax) * plotH);
          singleBars += '<rect x="' + (cx + 4).toFixed(1) + '" y="' + (yBase - hPrev).toFixed(1) + '" width="' + bw.toFixed(1) + '" height="' + hPrev.toFixed(1) + '" rx="4" fill="var(--text-muted)" opacity="0.45" stroke="var(--text-secondary)" stroke-dasharray="3 2"/>';
        }
      }

      groups.push(
        '<g class="svg-bar-group" tabindex="0" data-ga-tip-title="' + esc(dateStr) + '" data-ga-tip-body="' + esc(tipBody) + '">' +
          '<title>' + esc(dateStr + ' — ' + tipBody) + '</title>' +
          '<rect class="svg-hover-bg" x="' + xSlot.toFixed(1) + '" y="' + padT + '" width="' + slotW.toFixed(1) + '" height="' + plotH + '" rx="3" fill="transparent"/>' +
          singleBars +
          (n >= 2 && py !== null ? '<circle cx="' + cx.toFixed(1) + '" cy="' + py.toFixed(1) + '" r="' + (n > 31 ? '1.8' : '2.6') + '" fill="var(--surface)" stroke="var(--text-secondary)" stroke-width="1.4"/>' : '') +
          (n >= 2 ? '<circle cx="' + cx.toFixed(1) + '" cy="' + cy.toFixed(1) + '" r="' + (n > 31 ? '2.2' : '3.4') + '" fill="' + meta.color + '" stroke="var(--surface)" stroke-width="1.5"/>' : '') +
          (showTick ? '<text x="' + cx.toFixed(1) + '" y="' + (H - 8) + '" text-anchor="middle" font-size="10" font-family="var(--font-mono)" fill="var(--text-muted)">' + esc(shortDate) + '</text>' : '') +
        '</g>'
      );
    });

    var linesHtml = '';
    if (n >= 2) {
      var firstX = (padL + slotW / 2).toFixed(1);
      var lastX = (padL + (n - 0.5) * slotW).toFixed(1);
      var polyCoords = firstX + ',' + yBase + ' ' + curPoints.join(' ') + ' ' + lastX + ',' + yBase;
      linesHtml += '<defs><linearGradient id="' + gradId + '" x1="0" y1="0" x2="0" y2="1">' +
        '<stop offset="0%" stop-color="' + meta.color + '" stop-opacity="0.22"/>' +
        '<stop offset="100%" stop-color="' + meta.color + '" stop-opacity="0.02"/>' +
      '</linearGradient></defs>' +
      '<polygon points="' + polyCoords + '" fill="url(#' + gradId + ')"/>';

      if (prevPoints.length >= 2) {
        linesHtml += '<polyline class="ga-compare-line" fill="none" stroke="var(--text-secondary)" stroke-width="1.8" stroke-dasharray="5 4" stroke-linecap="round" stroke-linejoin="round" opacity="0.8" points="' + prevPoints.join(' ') + '"/>';
      }
      linesHtml += '<polyline fill="none" stroke="' + meta.color + '" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" points="' + curPoints.join(' ') + '"/>';
    }

    wrap.innerHTML = '<svg class="analytics-svg" viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="Biểu đồ xu hướng ' + esc(meta.label) + '">' +
      gridLines.join('') +
      linesHtml +
      groups.join('') +
    '</svg>';
  }

  /* --------------------------------------------------------------------------
     GA4 Visual Panel 2: Conversion Funnel (Phễu Hành Trình Khách)
     -------------------------------------------------------------------------- */
  function renderAnalyticsFunnel(data) {
    var wrap = document.getElementById('ga-funnel-chart');
    var e2eBadge = document.getElementById('ga-funnel-e2e-badge');
    if (!wrap) return;

    var steps = Array.isArray(data.funnel) && data.funnel.length ? data.funnel : [
      { step: 'qr_views', label: 'Quét QR / Mở trang', count: 0, pct_of_top: 0, drop_pct: 0 },
      { step: 'watch_timeline', label: 'Xem Live / Timeline', count: 0, pct_of_top: 0, drop_pct: 0 },
      { step: 'download_cut', label: 'Tải Clip / Cắt CheckVAR', count: 0, pct_of_top: 0, drop_pct: 0 },
      { step: 'ai_studio', label: 'AI Studio / Xuất Video', count: 0, pct_of_top: 0, drop_pct: 0 }
    ];

    var topCount = Number(steps[0].count) || 0;
    var clipStep = steps[2] || steps[steps.length - 1];
    var e2ePct = Number(clipStep && clipStep.pct_of_top) || 0;
    if (e2eBadge) {
      e2eBadge.textContent = e2ePct.toFixed(1) + '% CĐ Clip';
    }

    wrap.innerHTML = steps.map(function (st, idx) {
      var cnt = Number(st.count) || 0;
      var pct = Number(st.pct_of_top) || 0;
      var drop = Number(st.drop_pct) || 0;
      var barW = topCount > 0 && cnt > 0 ? Math.max(3, Math.min(100, pct)) : (idx === 0 && cnt > 0 ? 100 : 0);
      var prevCount = idx > 0 ? (Number(steps[idx - 1].count) || 0) : cnt;
      var stepKeep = idx > 0 ? (prevCount > 0 ? Math.min(100, (cnt / prevCount) * 100) : 0) : 100;

      var dropRow = idx > 0
        ? '<div class="ga-funnel-drop-row mono">' +
            '<span>↳ Giữ chân từ bước ' + idx + ': <strong class="text-emerald">' + esc(stepKeep.toFixed(1)) + '%</strong></span>' +
            '<span>·</span>' +
            '<span>Rơi: <strong class="' + (drop > 65 ? 'text-amber' : 'text-secondary') + '">-' + esc(drop.toFixed(1)) + '%</strong></span>' +
          '</div>'
        : '';

      return dropRow +
        '<div class="ga-funnel-step">' +
          '<div class="ga-funnel-step-head">' +
            '<span class="ga-funnel-step-left">' +
              '<span class="ga-funnel-idx">' + (idx + 1) + '</span>' +
              '<span class="ga-funnel-step-name">' + esc(st.label || st.step) + '</span>' +
            '</span>' +
            '<span class="ga-funnel-step-nums mono">' +
              '<strong>' + esc(fmtInt(cnt)) + '</strong>' +
              '<span class="badge badge-neutral">' + esc(pct.toFixed(1)) + '%</span>' +
            '</span>' +
          '</div>' +
          '<div class="ga-funnel-track" title="' + esc((st.label || '') + ': ' + fmtInt(cnt) + ' (' + pct.toFixed(1) + '%)') + '">' +
            '<div class="ga-funnel-bar step-' + idx + '" style="width:' + barW.toFixed(1) + '%"></div>' +
          '</div>' +
        '</div>';
    }).join('');
  }

  /* --------------------------------------------------------------------------
     GA4 Visual Panel 3: 7x24 Heatmap Matrix (Thứ 2 -> CN x 0h -> 23h GMT+7)
     -------------------------------------------------------------------------- */
  function renderAnalyticsHeatmap(data) {
    var gridEl = document.getElementById('ga-heatmap-grid');
    var badge = document.getElementById('analytics-peak-badge');
    var readout = document.getElementById('ga-heatmap-readout');
    if (!gridEl) return;

    var hm = data.heatmap_7x24 || {};
    var days = Array.isArray(hm.days) && hm.days.length === 7 ? hm.days : ['T2', 'T3', 'T4', 'T5', 'T6', 'T7', 'CN'];
    var matrix = Array.isArray(hm.matrix) && hm.matrix.length === 7 ? hm.matrix : days.map(function () { return new Array(24).fill(0); });
    var maxCell = Number(hm.max_cell) || 0;
    if (maxCell === 0) {
      matrix.forEach(function (row) {
        if (Array.isArray(row)) row.forEach(function (v) { if (Number(v) > maxCell) maxCell = Number(v); });
      });
    }
    var peak = hm.peak_day_hour || {};
    var peakDayIdx = peak.value > 0 ? Number(peak.day_index) : -1;
    var peakHour = peak.value > 0 ? Number(peak.hour) : -1;

    if (badge && state.analyticsHeatMode === 'heatmap') {
      if (peak && Number(peak.value) > 0) {
        var hp = (Number(peak.hour) < 10 ? '0' : '') + Number(peak.hour);
        badge.className = 'badge badge-amber';
        badge.innerHTML = svgIcon('flame', 'ui-icon-sm') + '<span>Đỉnh: ' + esc(peak.day_label || days[peakDayIdx] || '') + ' · ' + esc(hp) + ':00 (' + esc(fmtInt(peak.value)) + ')</span>';
      } else {
        badge.className = 'badge badge-neutral';
        badge.textContent = 'Đỉnh: Chưa có dữ liệu';
      }
    }

    if (readout) {
      if (peak && Number(peak.value) > 0) {
        var ph = (Number(peak.hour) < 10 ? '0' : '') + Number(peak.hour);
        readout.textContent = 'Giờ vàng nhất: ' + (peak.day_label || '') + ' lúc ' + ph + ':00–' + ph + ':59 (' + fmtInt(peak.value) + ' hoạt động)';
      } else {
        readout.textContent = 'Chưa có dữ liệu phân bổ theo 7×24 giờ trong khung này';
      }
    }

    var dayNamesFull = ['Thứ 2', 'Thứ 3', 'Thứ 4', 'Thứ 5', 'Thứ 6', 'Thứ 7', 'Chủ Nhật'];
    var rowsHtml = days.map(function (dayLbl, dIdx) {
      var row = Array.isArray(matrix[dIdx]) ? matrix[dIdx] : [];
      var isWeekend = dIdx >= 5;
      var cells = [];
      for (var h = 0; h < 24; h++) {
        var v = Number(row[h]) || 0;
        var lv = 0;
        if (v > 0 && maxCell > 0) {
          var ratio = v / maxCell;
          if (ratio > 0.78) lv = 4;
          else if (ratio > 0.50) lv = 3;
          else if (ratio > 0.24) lv = 2;
          else lv = 1;
        }
        var isPeakCell = (dIdx === peakDayIdx && h === peakHour && v > 0);
        var hPad = (h < 10 ? '0' : '') + h;
        var tipTitle = (dayNamesFull[dIdx] || dayLbl) + ' · ' + hPad + ':00–' + hPad + ':59 (GMT+7)';
        var tipBody = fmtInt(v) + ' lượt hoạt động' + (maxCell > 0 ? (' · ' + Math.round((v / maxCell) * 100) + '% đỉnh') : '');
        cells.push(
          '<div class="ga-heat-cell lv-' + lv + (isPeakCell ? ' is-peak' : '') + '" tabindex="0" role="gridcell" aria-label="' + esc(tipTitle + ': ' + tipBody) + '" data-ga-tip-title="' + esc(tipTitle) + '" data-ga-tip-body="' + esc(tipBody) + '" data-ga-heat-readout="' + esc(tipTitle + ' — ' + tipBody) + '"></div>'
        );
      }
      return '<div class="ga-heat-row" role="row">' +
        '<span class="ga-heat-day' + (isWeekend ? ' weekend' : '') + '">' + esc(dayLbl) + '</span>' +
        cells.join('') +
      '</div>';
    }).join('');

    var axisTicks = [0, 3, 6, 9, 12, 15, 18, 21, 23];
    var axisHours = axisTicks.map(function (hr) {
      var mobHide = (hr === 3 || hr === 9 || hr === 15 || hr === 21);
      return '<span class="ga-heat-hour' + (mobHide ? ' mob-hide' : '') + '">' + hr + 'h</span>';
    });

    gridEl.innerHTML = rowsHtml + '<div class="ga-heat-axis">' + axisHours.join('') + '</div>';
  }

  function buildPeakHoursSvgHtml(peakHours, opts) {
    var cfg = opts || {};
    var W = cfg.width || 640;
    var H = cfg.height || 225;
    var padL = cfg.padL || 44;
    var padR = cfg.padR || 12;
    var padT = cfg.padT || 14;
    var padB = cfg.padB || 28;
    var plotW = W - padL - padR;
    var plotH = H - padT - padB;

    var byHour = {};
    (Array.isArray(peakHours) ? peakHours : []).forEach(function (row) {
      var hr = Number(row.hour);
      if (hr >= 0 && hr <= 23) byHour[hr] = row;
    });

    var hours = [];
    var maxTotal = 0;
    var peakHour = -1;
    var peakVal = 0;

    for (var h = 0; h < 24; h++) {
      var r = byHour[h] || {};
      var qr = Number(r.qr_views) || 0;
      var dc = Number(r.downloads_cuts) || 0;
      var ai = Number(r.ai_runs) || 0;
      var uv = Number(r.unique_visitors) || 0;
      var bo = Number(r.bytes_out) || 0;
      var tot = qr + dc + ai;
      hours.push({ hour: h, qr: qr, dc: dc, ai: ai, uv: uv, bo: bo, tot: tot });
      if (tot > maxTotal) maxTotal = tot;
      if (tot > peakVal) {
        peakVal = tot;
        peakHour = h;
      }
    }

    var yMax = maxTotal > 0 ? Math.ceil(maxTotal * 1.12) : 4;
    var gridLines = [];
    var ticks = 4;
    for (var t = 0; t <= ticks; t++) {
      var frac = t / ticks;
      var y = padT + plotH * (1 - frac);
      var lblVal = Math.round(yMax * frac);
      gridLines.push(
        '<line x1="' + padL + '" y1="' + y.toFixed(1) + '" x2="' + (W - padR) + '" y2="' + y.toFixed(1) + '" stroke="var(--border)" stroke-width="1" ' + (t > 0 ? 'stroke-dasharray="3 3"' : '') + '/>' +
        '<text x="' + (padL - 8) + '" y="' + (y + 3.5).toFixed(1) + '" text-anchor="end" font-size="10" font-family="var(--font-mono)" fill="var(--text-muted)">' + esc(fmtCompact(lblVal)) + '</text>'
      );
    }

    var slotW = plotW / 24;
    var barW = Math.max(7, slotW - 6);
    var bars = [];

    hours.forEach(function (item, idx) {
      var xSlot = padL + idx * slotW;
      var xBar = xSlot + (slotW - barW) / 2;
      var isPeak = (idx === peakHour && peakVal > 0);
      var yBase = padT + plotH;

      var hQr = item.qr > 0 ? Math.max(2, (item.qr / yMax) * plotH) : 0;
      var hDc = item.dc > 0 ? Math.max(2, (item.dc / yMax) * plotH) : 0;
      var hAi = item.ai > 0 ? Math.max(2, (item.ai / yMax) * plotH) : 0;
      var hSum = hQr + hDc + hAi;
      if (hSum > plotH && hSum > 0) {
        var scale = plotH / hSum;
        hQr *= scale;
        hDc *= scale;
        hAi *= scale;
      }

      var yQr = yBase - hQr;
      var yDc = yQr - hDc;
      var yAi = yDc - hAi;

      var hrPad = (item.hour < 10 ? '0' : '') + item.hour;
      var tipTitle = hrPad + ':00–' + hrPad + ':59 (GMT+7)';
      var tipBody = 'QR: ' + fmtInt(item.qr) + ' · Khách: ' + fmtInt(item.uv) + ' · Tải/Cắt: ' + fmtInt(item.dc) + ' · AI: ' + fmtInt(item.ai);

      var peakBg = isPeak
        ? '<rect x="' + (xSlot + 1).toFixed(1) + '" y="' + padT + '" width="' + (slotW - 2).toFixed(1) + '" height="' + plotH + '" rx="4" fill="var(--amber-bg)" stroke="var(--amber-border)" stroke-width="1"/>'
        : '';

      var nearPeak = (peakHour >= 0 && peakVal > 0 && Math.abs(idx - peakHour) <= 1);
      var showLbl = isPeak || (!nearPeak && (idx % 3 === 0 || idx === 23));
      var xLbl = showLbl
        ? '<text x="' + (xSlot + slotW / 2).toFixed(1) + '" y="' + (H - 8) + '" text-anchor="middle" font-size="10" font-family="var(--font-mono)" font-weight="' + (isPeak ? '700' : '400') + '" fill="' + (isPeak ? 'var(--brand-amber)' : 'var(--text-muted)') + '">' + item.hour + 'h</text>'
        : '';

      bars.push(
        '<g class="svg-bar-group" tabindex="0" data-ga-tip-title="' + esc(tipTitle) + '" data-ga-tip-body="' + esc(tipBody) + '">' +
          '<title>' + esc(tipTitle + ' — ' + tipBody) + '</title>' +
          peakBg +
          '<rect class="svg-hover-bg" x="' + (xSlot + 1).toFixed(1) + '" y="' + padT + '" width="' + (slotW - 2).toFixed(1) + '" height="' + plotH + '" rx="3" fill="transparent"/>' +
          (hQr > 0 ? '<rect x="' + xBar.toFixed(1) + '" y="' + yQr.toFixed(1) + '" width="' + barW.toFixed(1) + '" height="' + hQr.toFixed(1) + '" rx="2" fill="var(--cyan)"/>' : '') +
          (hDc > 0 ? '<rect x="' + xBar.toFixed(1) + '" y="' + yDc.toFixed(1) + '" width="' + barW.toFixed(1) + '" height="' + hDc.toFixed(1) + '" rx="2" fill="var(--emerald)"/>' : '') +
          (hAi > 0 ? '<rect x="' + xBar.toFixed(1) + '" y="' + yAi.toFixed(1) + '" width="' + barW.toFixed(1) + '" height="' + hAi.toFixed(1) + '" rx="2" fill="var(--purple)"/>' : '') +
          xLbl +
        '</g>'
      );
    });

    var emptyOverlay = maxTotal === 0
      ? '<text x="' + (padL + plotW / 2).toFixed(1) + '" y="' + (padT + plotH / 2).toFixed(1) + '" text-anchor="middle" font-size="12" fill="var(--text-muted)">Chưa có lưu lượng theo khung giờ</text>'
      : '';

    return {
      peakHour: peakHour,
      peakVal: peakVal,
      svg: '<svg class="analytics-svg" viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="Biểu đồ khung giờ vàng 0h đến 23h">' +
        gridLines.join('') +
        bars.join('') +
        emptyOverlay +
      '</svg>'
    };
  }

  function renderAnalyticsPeakHours(data) {
    var wrap = document.getElementById('analytics-peak-chart');
    var badge = document.getElementById('analytics-peak-badge');
    if (!wrap) return;
    var res = buildPeakHoursSvgHtml(data.peak_hours || [], { width: 640, height: 225 });
    wrap.innerHTML = res.svg;
    if (badge && state.analyticsHeatMode === 'bars') {
      if (res.peakHour >= 0 && res.peakVal > 0) {
        var hp = (res.peakHour < 10 ? '0' : '') + res.peakHour;
        badge.className = 'badge badge-amber';
        badge.innerHTML = svgIcon('flame', 'ui-icon-sm') + '<span>Đỉnh: ' + esc(hp) + ':00 (' + esc(fmtInt(res.peakVal)) + ' lượt)</span>';
      } else {
        badge.className = 'badge badge-neutral';
        badge.textContent = 'Đỉnh: Chưa có dữ liệu';
      }
    }
  }

  /* --------------------------------------------------------------------------
     GA4 Visual Panel 4: Acquisition Channels + OS/Device + AI Studio Pills
     -------------------------------------------------------------------------- */
  function renderProgressRow(iconName, label, count, total, colorClass, extraText) {
    var pct = total > 0 ? ((count / total) * 100) : 0;
    var widthPct = count > 0 ? Math.max(2, Math.min(100, pct)) : 0;
    return '<div class="breakdown-row">' +
      '<div class="breakdown-row-head">' +
        '<span class="breakdown-label">' + svgIcon(iconName, 'ui-icon-sm text-' + (colorClass === 'muted' ? 'muted' : colorClass)) +
          '<span>' + esc(label) + (extraText ? ' <span class="mono text-muted" style="font-size:11px;">' + esc(extraText) + '</span>' : '') + '</span>' +
        '</span>' +
        '<span class="mono"><strong>' + esc(fmtInt(count)) + '</strong> <span class="text-muted">(' + pct.toFixed(1) + '%)</span></span>' +
      '</div>' +
      '<div class="breakdown-track"><div class="breakdown-fill ' + colorClass + '" style="width:' + widthPct.toFixed(1) + '%"></div></div>' +
    '</div>';
  }

  function renderAnalyticsBreakdowns(data) {
    var acq = data.acquisition || {};
    var os = data.os_breakdown || {};
    var kpi = data.kpi || {};

    var acqQr = Number(acq.qr) || 0;
    var acqShare = Number(acq.share) || 0;
    var acqApp = Number(acq.app) || 0;
    var acqDirect = Number(acq.direct) || 0;
    var acqTotal = acqQr + acqShare + acqApp + acqDirect;

    var acqTotalEl = document.getElementById('ga-acq-total');
    if (acqTotalEl) acqTotalEl.textContent = fmtInt(acqTotal) + ' phiên';

    var acqEl = document.getElementById('ga-acquisition-breakdown');
    if (acqEl) {
      acqEl.innerHTML =
        renderProgressRow('qr', 'QR Tại Bàn', acqQr, acqTotal, 'cyan') +
        renderProgressRow('share', 'Link Chia Sẻ', acqShare, acqTotal, 'emerald') +
        renderProgressRow('tablet', 'Kiosk App', acqApp, acqTotal, 'amber') +
        renderProgressRow('compass', 'Trực tiếp / AI', acqDirect, acqTotal, 'purple');
    }

    var ios = Number(os.ios) || 0;
    var android = Number(os.android) || 0;
    var desktop = Number(os.desktop) || 0;
    var app = Number(os.app) || 0;
    var other = Number(os.other) || 0;
    var osTotal = ios + android + desktop + app + other;

    var osTotalEl = document.getElementById('analytics-os-total');
    if (osTotalEl) osTotalEl.textContent = fmtInt(osTotal) + ' lượt';

    var osEl = document.getElementById('analytics-os-breakdown');
    if (osEl) {
      osEl.innerHTML =
        renderProgressRow('phone', 'iOS', ios, osTotal, 'cyan') +
        renderProgressRow('phone', 'Android', android, osTotal, 'emerald') +
        renderProgressRow('monitor', 'Desktop', desktop, osTotal, 'purple') +
        renderProgressRow('tablet', 'Kiosk', app, osTotal, 'amber') +
        (other > 0 ? renderProgressRow('globe', 'Khác', other, osTotal, 'muted') : '');
    }

    var aiBoxesBadge = document.getElementById('analytics-ai-boxes-badge');
    if (aiBoxesBadge) aiBoxesBadge.textContent = fmtInt(kpi.ai_active_boxes || 0) + ' Box AI';

    var pillsEl = document.getElementById('analytics-ai-pills');
    if (pillsEl) {
      pillsEl.innerHTML =
        '<span class="badge badge-emerald">Pool: ' + esc(fmtInt(kpi.ai_pool_runs || 0)) + '</span>' +
        '<span class="badge badge-amber">Carom: ' + esc(fmtInt(kpi.ai_carom_runs || 0)) + '</span>' +
        '<span class="badge badge-cyan">Pickleball: ' + esc(fmtInt(kpi.ai_pickleball_runs || 0)) + '</span>' +
        '<span class="badge badge-neutral">Cú đánh: ' + esc(fmtInt(kpi.ai_shots || 0)) + ' · Lỗ: ' + esc(fmtInt(kpi.ai_pots || 0)) + '</span>' +
        '<span class="badge badge-purple">Xuất iOS ' + esc(fmtInt(kpi.ai_exports_ios || 0)) + ' / PC ' + esc(fmtInt(kpi.ai_exports_desktop || 0)) + '</span>';
    }
  }

  /* --------------------------------------------------------------------------
     Searchable, Paginated Drill-Down Tables (Desktop Table + 50px Compact Mobile Rows)
     -------------------------------------------------------------------------- */
  function renderSourceBadges(item, isClub) {
    var sources = [];
    if (isClub) {
      if (Array.isArray(item.sources)) sources = item.sources;
      else if (typeof item.sources === 'string' && item.sources.trim()) sources = item.sources.split(',');
      else if (item.source) sources = [item.source];
    } else if (item.source) {
      sources = [item.source];
    }

    var out = [];
    var seen = {};
    sources.forEach(function (s) {
      var clean = String(s || '').trim();
      if (!clean || seen[clean]) return;
      seen[clean] = true;
      var low = clean.toLowerCase();
      if (low === 'redbida-go') out.push('<span class="badge badge-cyan">redbida-go</span>');
      else if (low === 'nodered' || low === 'node-red') out.push('<span class="badge badge-neutral">Node-RED</span>');
      else out.push('<span class="badge badge-neutral">' + esc(clean) + '</span>');
    });
    if (!out.length) out.push('<span class="badge badge-neutral">--</span>');
    if (item.ai_enabled) {
      out.push('<span class="badge badge-purple">' + svgIcon('cpu', 'ui-icon-sm') + '<span>AI</span></span>');
    }
    return '<div class="badge-row">' + out.join('') + '</div>';
  }

  function getAnalyticsSortValue(item, col, isClub) {
    if (col === 'name') {
      return String(isClub ? (item.club_name || item.club_id || '') : (item.box_name || item.box_id || '')).toLowerCase();
    }
    if (col === 'downloads_cuts') return (Number(item.clip_downloads) || 0) + (Number(item.cut_clips) || 0);
    if (col === 'live_vod') return (Number(item.live_hls) || 0) + (Number(item.vod_plays) || 0);
    if (col === 'errors') return (Number(item.http_errors) || 0) + (Number(item.ai_errors) || 0);
    return Number(item[col]) || 0;
  }

  function renderAnalyticsTopTable(data) {
    var wrap = document.getElementById('analytics-top-table-wrap');
    if (!wrap) return;
    var isClub = state.analyticsGroupMode !== 'boxes';
    var raw = isClub
      ? (Array.isArray(data.top_clubs) ? data.top_clubs : [])
      : (Array.isArray(data.top_boxes) ? data.top_boxes : []);

    raw.forEach(function (item) {
      indexItemSearch(item, [item.club_name, item.club_id, item.box_name, item.box_id, item.branding, item.source, Array.isArray(item.sources) ? item.sources.join(' ') : '']);
    });

    var tokens = parseSearchQuery(state.analyticsTopSearch);
    var filtered = tokens ? raw.filter(function (item) { return matchesSearch(item, tokens); }) : raw;
    syncSearchBoxState('analytics-top-search', 'analytics-top-match-count', filtered.length, raw.length);

    var titleEl = document.getElementById('analytics-top-title');
    if (titleEl) {
      titleEl.innerHTML = svgIcon('trophy', 'text-amber') + '<span>' +
        (isClub ? ('Top Quán / CLB (' + esc(fmtInt(filtered.length)) + ')') : ('Top Thiết Bị Box (' + esc(fmtInt(filtered.length)) + ')')) +
      '</span>';
    }

    var sorted = filtered.slice().sort(function (a, b) {
      var va = getAnalyticsSortValue(a, state.analyticsSortCol, isClub);
      var vb = getAnalyticsSortValue(b, state.analyticsSortCol, isClub);
      var cmp = (typeof va === 'string' && typeof vb === 'string') ? va.localeCompare(vb, 'vi') : (va - vb);
      return state.analyticsSortDir === 'asc' ? cmp : -cmp;
    });

    function thSort(col, label) {
      var active = state.analyticsSortCol === col;
      var arrow = active ? (state.analyticsSortDir === 'asc' ? ' ▲' : ' ▼') : '';
      return '<th class="sortable-th' + (active ? ' sort-active' : '') + '" data-analytics-sort="' + esc(col) + '">' + esc(label) + arrow + '</th>';
    }

    var pgTop = paginateItems(sorted, 'analyticsTop');
    var rowsHtml = '';
    var mobileRowsHtml = '';

    if (!sorted.length) {
      rowsHtml = '<tr><td colspan="10" class="analytics-empty-cell">Không tìm thấy quán/box nào phù hợp trong khung thời gian này.</td></tr>';
      mobileRowsHtml = '<div class="empty-state" style="padding:20px 12px;">Không tìm thấy quán/box nào phù hợp.</div>';
    } else {
      rowsHtml = pgTop.items.map(function (item) {
        var isSelected = isClub
          ? (state.analyticsClubId && !state.analyticsBoxId && state.analyticsClubId === item.club_id)
          : (state.analyticsBoxId && state.analyticsBoxId === item.box_id);
        var errs = (Number(item.http_errors) || 0) + (Number(item.ai_errors) || 0);
        var dcTotal = (Number(item.clip_downloads) || 0) + (Number(item.cut_clips) || 0);
        var sessions = Number(item.sessions) || 0;
        var engaged = Number(item.engaged_sessions) || 0;
        var engPct = sessions > 0 ? Math.round((engaged / sessions) * 100) : 0;
        var drillClub = item.club_id || '';
        var drillBox = isClub ? '' : (item.box_id || '');
        var drillLabel = isClub
          ? (item.club_name || item.club_id || '')
          : ((item.box_name || item.box_id || '') + (item.club_name ? (' · ' + item.club_name) : ''));

        var brandTag = item.branding === 'pickleball'
          ? ' <span class="badge badge-amber">Pickleball</span>'
          : '';
        var nameCell = isClub
          ? ('<td><div class="cell-primary">' + esc(item.club_name || item.club_id || '--') + brandTag + '</div>' +
             '<div class="cell-sub mono">' + esc(item.club_id || '') + ' · ' + esc(fmtInt(item.boxes_count || 0)) + ' Box</div></td>')
          : ('<td><div class="mono cell-primary">' + esc(item.box_id || '--') + brandTag + '</div>' +
             '<div class="cell-sub">' + esc(item.box_name || '') + (item.club_name ? (' · ' + esc(item.club_name)) : '') + '</div></td>');

        return '<tr class="clickable-row' + (isSelected ? ' row-selected' : '') + '"' +
          ' data-drill-club="' + esc(drillClub) + '"' +
          ' data-drill-box="' + esc(drillBox) + '"' +
          ' data-drill-label="' + esc(drillLabel) + '">' +
          nameCell +
          '<td>' + renderSourceBadges(item, isClub) + '</td>' +
          '<td><strong>' + esc(fmtInt(item.unique_visitors)) + '</strong>' +
            '<div class="cell-sub">Quay lại ' + esc(fmtInt(item.returning_visitors || 0)) + '</div></td>' +
          '<td><strong>' + esc(fmtInt(sessions)) + '</strong>' +
            '<div class="cell-sub">' + esc(engPct) + '% tương tác</div></td>' +
          '<td><strong class="text-cyan">' + esc(fmtInt(item.qr_views)) + '</strong></td>' +
          '<td>' +
            '<strong class="text-emerald">' + esc(fmtInt(dcTotal)) + '</strong>' +
            '<div class="cell-sub">Tải ' + esc(fmtInt(item.clip_downloads)) + ' · Cắt ' + esc(fmtInt(item.cut_clips)) + '</div>' +
          '</td>' +
          '<td>' +
            '<strong class="text-purple">' + esc(fmtInt(item.ai_analyzes)) + '</strong>' +
            '<div class="cell-sub">' + esc(fmtInt(item.ai_shots)) + ' cú · ' + esc(fmtInt(item.ai_exports)) + ' xuất</div>' +
          '</td>' +
          '<td class="mono">' + esc(fmtBytes(item.bytes_out)) + '</td>' +
          '<td>' + (errs === 0
            ? '<span class="badge badge-emerald">0</span>'
            : '<span class="badge ' + (errs >= 10 ? 'badge-crimson' : 'badge-amber') + '">' + esc(fmtInt(errs)) + '</span>') +
          '</td>' +
          '<td class="mono text-muted" style="font-size:12px;">' + esc(fmtIngestTime(item.last_ingest_at)) + '</td>' +
        '</tr>';
      }).join('');

      /* Compact 50px Mobile Expandable Rows */
      mobileRowsHtml = pgTop.items.map(function (item) {
        var rowId = (isClub ? 'club:' : 'box:') + (isClub ? (item.club_id || item.club_name) : (item.box_id || item.box_name));
        var isExp = !!state.expandedAnalyticsTopIds[rowId];
        var dcTotal = (Number(item.clip_downloads) || 0) + (Number(item.cut_clips) || 0);
        var sessions = Number(item.sessions) || 0;
        var engaged = Number(item.engaged_sessions) || 0;
        var engPct = sessions > 0 ? Math.round((engaged / sessions) * 100) : 0;
        var errs = (Number(item.http_errors) || 0) + (Number(item.ai_errors) || 0);
        var title = isClub ? (item.club_name || item.club_id || '--') : (item.box_id || '--');
        var drillClub = item.club_id || '';
        var drillBox = isClub ? '' : (item.box_id || '');
        var drillLabel = isClub ? title : ((item.box_name || item.box_id || '') + (item.club_name ? (' · ' + item.club_name) : ''));

        return '<article class="analytics-mcard compact-mrow' + (isExp ? ' is-expanded' : '') + '">' +
          '<div class="mrow-head" role="button" tabindex="0" aria-expanded="' + (isExp ? 'true' : 'false') + '" data-toggle-mrow="analyticsTop" data-mrow-id="' + esc(rowId) + '">' +
            '<div class="mrow-line1">' +
              '<div class="mrow-primary"><span class="mrow-name">' + esc(title) + '</span></div>' +
              '<div class="mrow-pills">' +
                '<span class="mrow-pill pill-cyan">QR ' + esc(fmtCompact(item.qr_views)) + '</span>' +
                '<span class="mrow-pill pill-ok">' + esc(fmtCompact(item.unique_visitors)) + ' kh</span>' +
                '<span class="mrow-chevron" aria-hidden="true">' + svgIcon('chevronDown', 'ui-icon-sm') + '</span>' +
              '</div>' +
            '</div>' +
            '<div class="mrow-line2 no-indent mono">' +
              '<span class="mrow-metrics">' +
                '<span>Phiên ' + esc(fmtCompact(sessions)) + '</span>' +
                '<span class="mrow-sep">·</span><span>Cắt/Tải ' + esc(fmtCompact(dcTotal)) + '</span>' +
                '<span class="mrow-sep">·</span><span>AI ' + esc(fmtCompact(item.ai_analyzes)) + '</span>' +
                '<span class="mrow-sep">·</span><span>' + esc(fmtBytes(item.bytes_out)) + '</span>' +
              '</span>' +
            '</div>' +
          '</div>' +
          '<div class="mrow-body">' +
            '<div class="mcard-tel-grid">' +
              '<div class="mcard-tel-cell"><span class="mcard-tel-label">Khách &amp; Quay lại</span><span class="mcard-tel-val">' + esc(fmtInt(item.unique_visitors)) + '</span><span class="mcard-tel-sub">Quay lại: ' + esc(fmtInt(item.returning_visitors || 0)) + '</span></div>' +
              '<div class="mcard-tel-cell"><span class="mcard-tel-label">Phiên &amp; Tương tác</span><span class="mcard-tel-val">' + esc(fmtInt(sessions)) + '</span><span class="mcard-tel-sub">' + esc(engPct) + '% tương tác</span></div>' +
              '<div class="mcard-tel-cell"><span class="mcard-tel-label">Clip &amp; CheckVAR</span><span class="mcard-tel-val text-emerald">' + esc(fmtInt(dcTotal)) + '</span><span class="mcard-tel-sub">Tải ' + esc(fmtInt(item.clip_downloads)) + ' · Cắt ' + esc(fmtInt(item.cut_clips)) + '</span></div>' +
              '<div class="mcard-tel-cell"><span class="mcard-tel-label">AI Studio &amp; Lỗi</span><span class="mcard-tel-val text-purple">' + esc(fmtInt(item.ai_analyzes)) + '</span><span class="mcard-tel-sub">Xuất ' + esc(fmtInt(item.ai_exports)) + ' · Lỗi ' + esc(fmtInt(errs)) + '</span></div>' +
            '</div>' +
            '<div class="mrow-meta-line">' + renderSourceBadges(item, isClub) + '<span class="mono text-muted">' + esc(fmtIngestTime(item.last_ingest_at)) + '</span></div>' +
            '<div class="mrow-actions">' +
              '<button type="button" class="btn btn-cyan mrow-act-btn" data-ga-drill-club="' + esc(drillClub) + '" data-ga-drill-box="' + esc(drillBox) + '" data-ga-drill-label="' + esc(drillLabel) + '">Lọc chi tiết ' + (isClub ? 'quán này' : 'box này') + '</button>' +
            '</div>' +
          '</div>' +
        '</article>';
      }).join('');
    }

    wrap.innerHTML =
      '<div class="table-scroll desktop-only-table"><table class="scada-table tabular-nums">' +
        '<thead><tr>' +
          thSort('name', isClub ? 'CLB / Quán' : 'Box / CLB') +
          '<th>Nguồn &amp; AI</th>' +
          thSort('unique_visitors', 'Khách') +
          thSort('sessions', 'Phiên') +
          thSort('qr_views', 'Quét QR') +
          thSort('downloads_cuts', 'Tải & Cắt') +
          thSort('ai_analyzes', 'AI Studio') +
          thSort('bytes_out', 'Băng Thông') +
          thSort('errors', 'Lỗi') +
          '<th>Đẩy Gần Nhất</th>' +
        '</tr></thead>' +
        '<tbody>' + rowsHtml + '</tbody>' +
      '</table></div>' +
      '<div class="mobile-card-list compact-list mobile-only-cards tabular-nums">' + mobileRowsHtml + '</div>' +
      renderPaginationBar('analyticsTop', pgTop, isClub ? 'CLB' : 'Box');
  }

  function renderAnalyticsCamerasTable() {
    var wrap = document.getElementById('analytics-cameras-table-wrap');
    var countBadge = document.getElementById('analytics-cameras-count');
    if (!wrap) return;
    var raw = Array.isArray(state.analyticsCameras) ? state.analyticsCameras : [];
    var tokens = parseSearchQuery(state.analyticsCamerasSearch);
    var cams = tokens ? raw.filter(function (c) { return matchesSearch(c, tokens); }) : raw;
    syncSearchBoxState('analytics-cameras-search', 'analytics-cameras-match-count', cams.length, raw.length);
    if (countBadge) countBadge.textContent = fmtInt(cams.length) + ' bàn';

    var pgCam = paginateItems(cams, 'analyticsCameras');
    var rowsHtml = '';
    var mobileRowsHtml = '';

    if (!cams.length) {
      rowsHtml = '<tr><td colspan="10" class="analytics-empty-cell">Chưa có dữ liệu từng bàn / camera phù hợp trong khung thời gian này.</td></tr>';
      mobileRowsHtml = '<div class="empty-state" style="padding:20px 12px;">Chưa có dữ liệu bàn / camera phù hợp.</div>';
    } else {
      rowsHtml = pgCam.items.map(function (c, idx) {
        var rankNum = pgCam.start + idx;
        var aiErr = Number(c.ai_errors) || 0;
        return '<tr class="clickable-row" data-ga-drill-club="' + esc(c.club_id || '') + '" data-ga-drill-box="" data-ga-drill-label="' + esc(c.club_name || c.club_id || '') + '">' +
          '<td class="mono text-muted" style="font-weight:700;">#' + rankNum + '</td>' +
          '<td>' +
            '<div class="cell-primary">' + esc(c.table_name || c.camera_id || 'Camera') + '</div>' +
            '<div class="cell-sub mono">' + esc(c.camera_id || '--') + '</div>' +
          '</td>' +
          '<td>' +
            '<div>' + esc(c.club_name || c.club_id || '--') + '</div>' +
            '<div class="cell-sub mono">' + esc(c.box_id || '--') + (c.box_name && c.box_name !== c.box_id ? (' · ' + esc(c.box_name)) : '') + '</div>' +
          '</td>' +
          '<td><strong class="text-cyan">' + esc(fmtInt(c.qr_views)) + '</strong></td>' +
          '<td>' + esc(fmtInt(c.unique_visitors)) + '</td>' +
          '<td>' + esc(fmtInt(c.live_hls)) + ' <span class="text-muted">/</span> ' + esc(fmtInt(c.vod_plays)) + '</td>' +
          '<td><strong class="text-emerald">' + esc(fmtInt(c.clip_downloads)) + '</strong></td>' +
          '<td>' +
            '<strong>' + esc(fmtInt(c.cut_clips)) + '</strong>' +
            '<div class="cell-sub">Timeline ' + esc(fmtInt(c.timeline_opens)) + ' · ' + esc(fmtCutMinutes(c.cut_seconds)) + '</div>' +
          '</td>' +
          '<td>' +
            '<strong class="text-purple">' + esc(fmtInt(c.ai_analyzes)) + '</strong>' +
            '<div class="cell-sub">' + esc(fmtInt(c.ai_shots)) + ' cú · ' + esc(fmtInt(c.ai_pots)) + ' lỗ · ' + esc(fmtInt(c.ai_exports)) + ' xuất' +
              (aiErr > 0 ? (' · <span class="text-crimson">' + esc(fmtInt(aiErr)) + ' lỗi</span>') : '') +
            '</div>' +
          '</td>' +
          '<td class="mono">' + esc(fmtBytes(c.bytes_out)) + '</td>' +
        '</tr>';
      }).join('');

      /* Compact 50px Mobile Expandable Rows */
      mobileRowsHtml = pgCam.items.map(function (c, idx) {
        var rankNum = pgCam.start + idx;
        var rowId = (c.box_id || 'b') + ':' + (c.camera_id || idx);
        var isExp = !!state.expandedAnalyticsCamIds[rowId];
        var dcTotal = (Number(c.clip_downloads) || 0) + (Number(c.cut_clips) || 0);
        return '<article class="analytics-mcard compact-mrow' + (isExp ? ' is-expanded' : '') + '">' +
          '<div class="mrow-head" role="button" tabindex="0" aria-expanded="' + (isExp ? 'true' : 'false') + '" data-toggle-mrow="analyticsCam" data-mrow-id="' + esc(rowId) + '">' +
            '<div class="mrow-line1">' +
              '<div class="mrow-primary"><span class="mono text-muted" style="font-size:11px;font-weight:700;">#' + rankNum + '</span><span class="mrow-name">' + esc(c.table_name || c.camera_id || 'Camera') + '</span></div>' +
              '<div class="mrow-pills">' +
                '<span class="mrow-pill pill-cyan">QR ' + esc(fmtCompact(c.qr_views)) + '</span>' +
                '<span class="mrow-pill pill-ok">Clip ' + esc(fmtCompact(dcTotal)) + '</span>' +
                '<span class="mrow-chevron" aria-hidden="true">' + svgIcon('chevronDown', 'ui-icon-sm') + '</span>' +
              '</div>' +
            '</div>' +
            '<div class="mrow-line2 no-indent mono">' +
              '<span class="mrow-id">' + esc(c.club_name || c.club_id || '--') + '</span>' +
              '<span class="mrow-metrics">' +
                '<span class="mrow-sep">·</span><span>' + esc(fmtCompact(c.unique_visitors)) + ' kh</span>' +
                '<span class="mrow-sep">·</span><span>AI ' + esc(fmtCompact(c.ai_analyzes)) + '</span>' +
                '<span class="mrow-sep">·</span><span>' + esc(fmtBytes(c.bytes_out)) + '</span>' +
              '</span>' +
            '</div>' +
          '</div>' +
          '<div class="mrow-body">' +
            '<div class="mcard-tel-grid">' +
              '<div class="mcard-tel-cell"><span class="mcard-tel-label">Live / VOD</span><span class="mcard-tel-val">' + esc(fmtInt(c.live_hls)) + ' / ' + esc(fmtInt(c.vod_plays)) + '</span><span class="mcard-tel-sub">Khách: ' + esc(fmtInt(c.unique_visitors)) + '</span></div>' +
              '<div class="mcard-tel-cell"><span class="mcard-tel-label">Tải &amp; CheckVAR</span><span class="mcard-tel-val text-emerald">' + esc(fmtInt(c.clip_downloads)) + ' / ' + esc(fmtInt(c.cut_clips)) + '</span><span class="mcard-tel-sub">' + esc(fmtCutMinutes(c.cut_seconds)) + '</span></div>' +
              '<div class="mcard-tel-cell"><span class="mcard-tel-label">AI Studio</span><span class="mcard-tel-val text-purple">' + esc(fmtInt(c.ai_analyzes)) + '</span><span class="mcard-tel-sub">' + esc(fmtInt(c.ai_shots)) + ' cú · ' + esc(fmtInt(c.ai_exports)) + ' xuất</span></div>' +
              '<div class="mcard-tel-cell"><span class="mcard-tel-label">Camera / Box</span><span class="mcard-tel-val mono">' + esc(c.camera_id || '--') + '</span><span class="mcard-tel-sub mono">' + esc(c.box_id || '--') + '</span></div>' +
            '</div>' +
            (c.club_id ? ('<div class="mrow-actions"><button type="button" class="btn btn-cyan mrow-act-btn" data-ga-drill-club="' + esc(c.club_id) + '" data-ga-drill-box="" data-ga-drill-label="' + esc(c.club_name || c.club_id) + '">Lọc CLB: ' + esc(c.club_name || c.club_id) + '</button></div>') : '') +
          '</div>' +
        '</article>';
      }).join('');
    }

    wrap.innerHTML =
      '<div class="table-scroll desktop-only-table"><table class="scada-table tabular-nums">' +
        '<thead><tr>' +
          '<th>#</th>' +
          '<th>Bàn / Camera</th>' +
          '<th>CLB / Box</th>' +
          '<th>Quét QR</th>' +
          '<th>Khách</th>' +
          '<th>Live / VOD</th>' +
          '<th>Tải Clip</th>' +
          '<th>Cắt CheckVAR</th>' +
          '<th>AI Studio (Chạy / Cú / Xuất)</th>' +
          '<th>Băng Thông</th>' +
        '</tr></thead>' +
        '<tbody>' + rowsHtml + '</tbody>' +
      '</table></div>' +
      '<div class="mobile-card-list compact-list mobile-only-cards tabular-nums">' + mobileRowsHtml + '</div>' +
      renderPaginationBar('analyticsCameras', pgCam, 'bàn');
  }

  /* --------------------------------------------------------------------------
     Shared Floating Tooltip Controller for GA4 SVG Charts & 7x24 Heatmap
     -------------------------------------------------------------------------- */
  function initAnalyticsTooltip() {
    var view = document.getElementById('view-analytics');
    var tip = document.getElementById('ga-float-tooltip');
    if (!view || !tip) return;

    function placeTip(x, y) {
      var pad = 12;
      var vw = window.innerWidth || 1024;
      var vh = window.innerHeight || 768;
      var w = tip.offsetWidth || 200;
      var h = tip.offsetHeight || 52;
      var left = x + 14;
      if (left + w + pad > vw) left = Math.max(pad, x - w - 14);
      var top = y - h - 12;
      if (top < pad) top = Math.min(vh - h - pad, y + 16);
      tip.style.left = Math.round(left) + 'px';
      tip.style.top = Math.round(top) + 'px';
    }

    function showTipFor(target, clientX, clientY) {
      var title = target.getAttribute('data-ga-tip-title') || '';
      var body = target.getAttribute('data-ga-tip-body') || '';
      if (!title && !body) return;
      var lines = body.split(' | ').map(function (ln) {
        return '<div class="ga-tip-line">' + esc(ln) + '</div>';
      }).join('');
      tip.innerHTML = '<div class="ga-tip-title">' + esc(title) + '</div>' + lines;
      tip.hidden = false;
      if (clientX !== undefined && clientY !== undefined) {
        placeTip(clientX, clientY);
      } else {
        var r = target.getBoundingClientRect();
        placeTip(r.left + r.width / 2, r.top);
      }
      var heatMsg = target.getAttribute('data-ga-heat-readout');
      if (heatMsg) {
        var ro = document.getElementById('ga-heatmap-readout');
        if (ro) ro.textContent = heatMsg;
      }
    }

    function hideTip() {
      tip.hidden = true;
    }

    view.addEventListener('mouseover', function (e) {
      var target = e.target.closest('[data-ga-tip-title]');
      if (target) showTipFor(target, e.clientX, e.clientY);
    });
    view.addEventListener('mousemove', function (e) {
      if (tip.hidden) return;
      var target = e.target.closest('[data-ga-tip-title]');
      if (!target) { hideTip(); return; }
      placeTip(e.clientX, e.clientY);
    });
    view.addEventListener('mouseout', function (e) {
      if (!e.relatedTarget || !e.relatedTarget.closest || !e.relatedTarget.closest('[data-ga-tip-title]')) {
        hideTip();
      }
    });
    view.addEventListener('focusin', function (e) {
      var target = e.target.closest('[data-ga-tip-title]');
      if (target) showTipFor(target);
    });
    view.addEventListener('focusout', hideTip);
    view.addEventListener('click', function (e) {
      var cell = e.target.closest('[data-ga-heat-readout]');
      if (cell) showTipFor(cell, e.clientX, e.clientY);
    });
  }

  /* --------------------------------------------------------------------------
     Navigation, Command Palette (Ctrl+K + / Shortcut) & Real-Time SSE Connection
     -------------------------------------------------------------------------- */
  function focusActiveViewSearch() {
    var map = {
      fleet: 'fleet-search',
      hardening: 'hardening-search',
      cameras: 'cam-search-input',
      redbida: 'redbida-search',
      handover: 'handover-club-search',
      analytics: 'analytics-top-search'
    };
    var id = map[state.activeView];
    var input = id ? document.getElementById(id) : null;
    if (input) {
      input.scrollIntoView({ behavior: 'smooth', block: 'center' });
      input.focus();
      if (typeof input.select === 'function' && input.value) input.select();
      return true;
    }
    return false;
  }

  function switchView(viewName, updateHash) {
    var validViews = ['fleet', 'hardening', 'cameras', 'redbida', 'handover', 'analytics'];
    if (validViews.indexOf(viewName) === -1) viewName = 'fleet';
    state.activeView = viewName;

    document.querySelectorAll('.nav-tab[data-view]').forEach(function (btn) {
      btn.classList.toggle('active', btn.getAttribute('data-view') === viewName);
    });
    document.querySelectorAll('.view-panel[data-view-panel]').forEach(function (panel) {
      panel.classList.toggle('active', panel.getAttribute('data-view-panel') === viewName);
    });

    var cockpitLabel = document.getElementById('cockpit-view-label');
    if (cockpitLabel) cockpitLabel.textContent = VIEW_LABELS[viewName] || 'Đội Box';

    if (updateHash !== false) {
      history.replaceState(null, '', '#' + viewName);
    }

    if (viewName !== 'analytics') {
      stopAnalyticsRealtimePolling();
    }

    if (viewName === 'fleet') loadFleetSummary();
    else if (viewName === 'hardening') loadHardeningView();
    else if (viewName === 'cameras') loadCamerasView();
    else if (viewName === 'redbida') loadRedbidaView();
    else if (viewName === 'handover') loadHandoverView();
    else if (viewName === 'analytics') {
      loadAnalyticsView();
      startAnalyticsRealtimePolling(false);
    }
  }

  function handleHashRoute() {
    var h = (window.location.hash || '#fleet').replace(/^#/, '');
    if (h.indexOf('box/') === 0) {
      var boxId = decodeURIComponent(h.slice(4));
      if (!state.boxes.length) loadFleetSummary();
      openBoxDrawer(boxId, false);
      return;
    }
    closeBoxDrawer();
    switchView(h || 'fleet', false);
  }

  var openCmdPaletteFn = null;

  function initCommandPalette() {
    var modal = document.getElementById('cmd-palette');
    var input = document.getElementById('cmd-search-input');
    var results = document.getElementById('cmd-results');
    var openBtn = document.getElementById('cmd-palette-btn');
    var closeBtn = document.getElementById('cmd-close-btn');
    var macroBar = document.getElementById('cmd-macro-bar');

    function openCmd() {
      if (!modal) return;
      state.cmdSelectedIdx = 0;
      modal.classList.add('open');
      renderCmdResults('');
      if (input) {
        input.value = '';
        input.focus();
      }
    }
    openCmdPaletteFn = openCmd;

    function closeCmd() {
      if (modal) modal.classList.remove('open');
    }

    function applyFleetFilterChipUi(filterVal) {
      var chipsWrap = document.getElementById('fleet-filter-chips');
      if (!chipsWrap) return;
      chipsWrap.querySelectorAll('.filter-chip[data-filter]').forEach(function (c) {
        c.classList.toggle('active', c.getAttribute('data-filter') === filterVal);
      });
    }

    function selectCmdRow(row) {
      if (!row) return;
      var t = row.getAttribute('data-cmd-type');
      var target = row.getAttribute('data-cmd-target');
      closeCmd();
      if (t === 'box') {
        openBoxDrawer(target, true);
      } else if (t === 'club') {
        state.fleetFilter = 'all';
        state.fleetSearch = target;
        applyFleetFilterChipUi('all');
        resetTabPage('fleet');
        var fs = document.getElementById('fleet-search');
        if (fs) fs.value = target;
        switchView('fleet', true);
        renderFleetMatrix({ force: true });
      } else {
        switchView(target, true);
      }
    }

    function renderCmdResults(query) {
      if (!results) return;
      var tokens = parseSearchQuery(query);
      var items = [
        { type: 'view', icon: 'server', target: 'fleet', title: 'Đội Box (Fleet Matrix)', sub: '#fleet' },
        { type: 'view', icon: 'shield', target: 'hardening', title: 'Chuẩn Hóa 8 Bước (Golden Hardening)', sub: '#hardening' },
        { type: 'view', icon: 'video', target: 'cameras', title: 'Kho Camera & Chuẩn Golden H.265', sub: '#cameras' },
        { type: 'view', icon: 'sliders', target: 'redbida', title: 'RedBida 17-Rule & Kiểm Toán FRPC', sub: '#redbida' },
        { type: 'view', icon: 'qr', target: 'handover', title: 'Gói Bàn Giao Zalo & Mã QR Chủ Quán', sub: '#handover' },
        { type: 'view', icon: 'chart', target: 'analytics', title: 'Thống Kê & Analytics (QR · CheckVAR · AI)', sub: '#analytics' }
      ];

      (state.clubs || []).forEach(function (c) {
        var cname = c.name || c.club_name || c.id || '';
        if (!cname) return;
        var bCount = Array.isArray(c.box_ids) ? c.box_ids.length : (c.boxes_count || 0);
        items.push({
          type: 'club',
          icon: 'building',
          target: cname,
          title: 'CLB: ' + cname,
          sub: (bCount ? (bCount + ' Box · ') : '') + (c.id || '')
        });
      });

      (state.boxes || []).forEach(function (b) {
        items.push({
          type: 'box',
          icon: 'box',
          target: b.id,
          title: b.id + ' — ' + (b.club_name || b.name || ''),
          sub: 'VPN: ' + (b.vpn_ip || '--') + ' · grp: ' + (b.shinobi_group_key || '--'),
          _searchRaw: b._searchRaw,
          _searchNorm: b._searchNorm
        });
      });

      var matched = items.filter(function (it) {
        return matchesSearch(it, tokens, [it.title, it.sub, it.target]);
      }).slice(0, 20);

      if (!matched.length) {
        results.innerHTML = '<div class="empty-state" style="padding:24px 12px;">Không tìm thấy kết quả phù hợp.</div>';
        return;
      }

      if (state.cmdSelectedIdx >= matched.length) state.cmdSelectedIdx = 0;

      results.innerHTML = matched.map(function (it, idx) {
        var selClass = idx === state.cmdSelectedIdx ? ' selected' : '';
        return '<div class="cmd-item' + selClass + '" data-cmd-type="' + esc(it.type) + '" data-cmd-target="' + esc(it.target) + '">' +
          '<div class="cmd-item-main">' + svgIcon(it.icon, 'ui-icon-sm text-cyan') + '<span>' + esc(it.title) + '</span></div>' +
          '<div class="mono cmd-item-sub">' + esc(it.sub) + '</div>' +
        '</div>';
      }).join('');
    }

    if (openBtn) openBtn.addEventListener('click', openCmd);
    if (closeBtn) closeBtn.addEventListener('click', closeCmd);
    if (modal) {
      modal.addEventListener('click', function (e) {
        if (e.target === modal) closeCmd();
      });
    }
    if (macroBar) {
      macroBar.addEventListener('click', function (e) {
        var chip = e.target.closest('[data-cmd-macro]');
        if (!chip) return;
        var macro = chip.getAttribute('data-cmd-macro');
        closeCmd();
        if (macro === 'pickleball') {
          state.redbidaBrandFilter = 'pickleball';
          resetTabPage('redbidaDomains');
          resetTabPage('redbidaBoxes');
          var rbChips = document.getElementById('redbida-brand-chips');
          if (rbChips) {
            rbChips.querySelectorAll('.filter-chip[data-brand]').forEach(function (c) {
              c.classList.toggle('active', c.getAttribute('data-brand') === 'pickleball');
            });
          }
          switchView('redbida', true);
          return;
        }
        state.fleetFilter = macro || 'all';
        if (macro === 'all') {
          state.fleetSearch = '';
          var fs = document.getElementById('fleet-search');
          if (fs) fs.value = '';
        }
        applyFleetFilterChipUi(state.fleetFilter);
        resetTabPage('fleet');
        switchView('fleet', true);
        renderFleetMatrix({ force: true });
      });
    }
    if (input) {
      input.addEventListener('input', function () {
        state.cmdSelectedIdx = 0;
        renderCmdResults(input.value);
      });
      input.addEventListener('keydown', function (e) {
        var rows = results ? results.querySelectorAll('.cmd-item') : [];
        if (!rows.length) return;
        if (e.key === 'ArrowDown') {
          e.preventDefault();
          state.cmdSelectedIdx = (state.cmdSelectedIdx + 1) % rows.length;
          rows.forEach(function (r, i) { r.classList.toggle('selected', i === state.cmdSelectedIdx); });
          rows[state.cmdSelectedIdx].scrollIntoView({ block: 'nearest' });
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
          state.cmdSelectedIdx = (state.cmdSelectedIdx - 1 + rows.length) % rows.length;
          rows.forEach(function (r, i) { r.classList.toggle('selected', i === state.cmdSelectedIdx); });
          rows[state.cmdSelectedIdx].scrollIntoView({ block: 'nearest' });
        } else if (e.key === 'Enter') {
          e.preventDefault();
          selectCmdRow(rows[state.cmdSelectedIdx] || rows[0]);
        }
      });
    }
    if (results) {
      results.addEventListener('click', function (e) {
        var row = e.target.closest('.cmd-item');
        if (row) selectCmdRow(row);
      });
    }

    document.addEventListener('keydown', function (e) {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        openCmd();
        return;
      }
      if (e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        var active = document.activeElement;
        var tag = active ? active.tagName : '';
        var isTyping = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || (active && active.isContentEditable);
        var anyModalOpen = !!document.querySelector('.modal-backdrop.open');
        if (!isTyping && !anyModalOpen) {
          e.preventDefault();
          if (!focusActiveViewSearch()) openCmd();
        }
        return;
      }
      if (e.key === 'Escape') {
        closeCmd();
        closeBoxDrawer();
        closeAllCardMenus();
        var rbModal = document.getElementById('redbida-editor-modal');
        if (rbModal) rbModal.classList.remove('open');
        closeChangePasswordModal();
      }
    });
  }

  function clearChangePasswordFields() {
    ['cpCurrentPassword', 'cpNewPassword', 'cpConfirmPassword'].forEach(function (id) {
      var el = document.getElementById(id);
      if (el) el.value = '';
    });
    try {
      ['password', 'admin_pass', 'viewer_pass', 'bidahub-password', 'kspcam-password'].forEach(function (k) {
        localStorage.removeItem(k);
        sessionStorage.removeItem(k);
      });
    } catch (_e) {}
  }

  function setChangePasswordError(msg) {
    var errEl = document.getElementById('cpErrorMsg');
    if (!errEl) return;
    if (!msg) {
      errEl.textContent = '';
      errEl.style.display = 'none';
    } else {
      errEl.textContent = msg;
      errEl.style.display = 'block';
    }
  }

  function openChangePasswordModal() {
    clearChangePasswordFields();
    setChangePasswordError('');
    var modal = document.getElementById('changePasswordModal');
    if (modal) modal.classList.add('open');
    var curEl = document.getElementById('cpCurrentPassword');
    if (curEl) curEl.focus();
  }

  function closeChangePasswordModal() {
    clearChangePasswordFields();
    setChangePasswordError('');
    var modal = document.getElementById('changePasswordModal');
    if (modal) modal.classList.remove('open');
  }

  function initChangePasswordModal() {
    clearChangePasswordFields();
    var openBtn = document.getElementById('btnChangePassword') || document.getElementById('btnOpenChangePassword');
    var closeBtn = document.getElementById('btnCloseChangePassword');
    var cancelBtn = document.getElementById('btnCancelChangePassword');
    var modal = document.getElementById('changePasswordModal');
    var form = document.getElementById('changePasswordForm');

    if (openBtn) openBtn.addEventListener('click', openChangePasswordModal);
    if (closeBtn) closeBtn.addEventListener('click', closeChangePasswordModal);
    if (cancelBtn) cancelBtn.addEventListener('click', closeChangePasswordModal);
    if (modal) {
      modal.addEventListener('click', function (e) {
        if (e.target === modal) closeChangePasswordModal();
      });
    }

    if (form) {
      form.addEventListener('submit', function (e) {
        e.preventDefault();
        setChangePasswordError('');

        var curEl = document.getElementById('cpCurrentPassword');
        var newEl = document.getElementById('cpNewPassword');
        var confirmEl = document.getElementById('cpConfirmPassword');

        var currentPassword = curEl ? curEl.value : '';
        var newPassword = newEl ? newEl.value : '';
        var confirmPassword = confirmEl ? confirmEl.value : '';

        /* Immediately clear all three password inputs after reading values */
        if (curEl) curEl.value = '';
        if (newEl) newEl.value = '';
        if (confirmEl) confirmEl.value = '';
        clearChangePasswordFields();

        if (!currentPassword) {
          setChangePasswordError('Vui lòng nhập mật khẩu hiện tại.');
          return;
        }
        if (!newPassword || newPassword.length < 6) {
          setChangePasswordError('Mật khẩu mới phải có ít nhất 6 ký tự.');
          return;
        }
        if (newPassword !== confirmPassword) {
          setChangePasswordError('Mật khẩu xác nhận không khớp.');
          return;
        }

        var submitBtn = document.getElementById('btnSubmitChangePassword');
        if (submitBtn) submitBtn.disabled = true;

        fetch('/api/auth/change-password', {
          method: 'POST',
          credentials: 'same-origin',
          headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json'
          },
          body: JSON.stringify({
            current_password: currentPassword,
            new_password: newPassword
          })
        }).then(function (res) {
          return res.json().catch(function () { return {}; }).then(function (data) {
            if (!res.ok) {
              throw new Error((data && (data.message || data.error)) || ('Lỗi đổi mật khẩu (HTTP ' + res.status + ')'));
            }
            return data;
          });
        }).then(function (data) {
          closeChangePasswordModal();
          showToast((data && data.message) || 'Đổi mật khẩu thành công! Đang chuyển hướng về trang đăng nhập...');
          setTimeout(function () {
            window.location.href = '/login';
          }, 600);
        }).catch(function (err) {
          setChangePasswordError(err.message || 'Không thể cập nhật mật khẩu.');
          showToast(err.message || 'Không thể cập nhật mật khẩu.', true);
        }).finally(function () {
          if (submitBtn) submitBtn.disabled = false;
        });
      });
    }
  }

  function setSseBadge(isLive) {
    state.sseConnected = isLive;
    var badge = document.getElementById('sse-badge');
    var txt = document.getElementById('sse-badge-text');
    if (!badge || !txt) return;
    if (isLive) {
      badge.classList.remove('polling');
      txt.textContent = 'LIVE SSE';
    } else {
      badge.classList.add('polling');
      txt.textContent = 'POLLING 15s';
    }
  }

  function closeAllCardMenus() {
    var wasOpen = !!state.openCardMenuId;
    state.openCardMenuId = null;
    document.querySelectorAll('.mcard-popover.open').forEach(function (pop) {
      pop.classList.remove('open');
    });
    document.querySelectorAll('[data-card-menu-btn][aria-expanded="true"]').forEach(function (btn) {
      btn.setAttribute('aria-expanded', 'false');
    });
    if (wasOpen && state.pendingFleetRender) {
      state.pendingFleetRender = false;
      renderFleetMatrix({ force: false });
    }
  }

  function initMobileKpiToggle() {
    var btn = document.getElementById('mobile-kpi-toggle');
    var grid = document.getElementById('fleet-kpi-grid');
    var lbl = document.getElementById('mobile-kpi-toggle-label');
    if (!btn || !grid) return;
    btn.addEventListener('click', function () {
      state.mobileKpiExpanded = !state.mobileKpiExpanded;
      grid.classList.toggle('mobile-expanded', state.mobileKpiExpanded);
      btn.setAttribute('aria-expanded', state.mobileKpiExpanded ? 'true' : 'false');
      if (lbl) lbl.textContent = state.mobileKpiExpanded ? 'Thu gọn' : 'KPI';
    });
  }

  function initStickyCockpitBar() {
    var bar = document.getElementById('sticky-cockpit-bar');
    var sentinel = document.getElementById('cockpit-sentinel');
    var findBtn = document.getElementById('cockpit-search-btn');
    if (!bar) return;

    var shown = false;
    function setCockpitVisible(val) {
      var isMobile = window.innerWidth < 768;
      var next = isMobile && val;
      if (next === shown) return;
      shown = next;
      bar.classList.toggle('visible', next);
      bar.setAttribute('aria-hidden', next ? 'false' : 'true');
    }

    function checkScrollHysteresis() {
      if (window.innerWidth >= 768) {
        setCockpitVisible(false);
        return;
      }
      var y = window.scrollY || window.pageYOffset || 0;
      if (!shown && y > 110) setCockpitVisible(true);
      else if (shown && y < 90) setCockpitVisible(false);
    }

    if ('IntersectionObserver' in window && sentinel) {
      var obs = new IntersectionObserver(function (entries) {
        if (!entries || !entries.length) return;
        var entry = entries[0];
        if (window.innerWidth >= 768) {
          setCockpitVisible(false);
          return;
        }
        var y = window.scrollY || window.pageYOffset || 0;
        if (!entry.isIntersecting && y > 95) setCockpitVisible(true);
        else if (entry.isIntersecting && y < 90) setCockpitVisible(false);
      }, { threshold: 0 });
      obs.observe(sentinel);
    }

    window.addEventListener('scroll', checkScrollHysteresis, { passive: true });
    window.addEventListener('resize', checkScrollHysteresis, { passive: true });

    if (findBtn) {
      findBtn.addEventListener('click', function () {
        if (!focusActiveViewSearch() && openCmdPaletteFn) {
          openCmdPaletteFn();
        }
      });
    }
  }

  var sseRefreshTimer = null;
  function scheduleRealtimeSummaryRefresh() {
    clearTimeout(sseRefreshTimer);
    sseRefreshTimer = setTimeout(function () {
      sseRefreshTimer = null;
      fetchSummary({ fromSSE: true });
      if (state.drawerBoxId) {
        apiFetch('/fleet/api/boxes/' + encodeURIComponent(state.drawerBoxId)).then(function (data) {
          renderBoxDrawerContent(state.drawerBoxId, data);
        }).catch(function () {});
      }
    }, 200);
  }

  function connectSSE() {
    if (window.EventSource) {
      try {
        var es = new EventSource('/fleet/api/events');
        es.onopen = function () {
          setSseBadge(true);
        };
        var onEvent = function (ev) {
          setSseBadge(true);
          if (ev && ev.data) {
            try {
              var parsed = JSON.parse(ev.data);
              var rs = parsed && parsed.realtime_summary;
              if (rs && typeof rs.active_5m === 'number') {
                var navBadge = document.getElementById('nav-badge-analytics');
                if (navBadge && rs.active_5m > 0) {
                  navBadge.textContent = fmtCompact(rs.active_5m) + ' LIVE';
                }
              }
            } catch (_ignore) {}
          }
          scheduleRealtimeSummaryRefresh();
        };
        es.addEventListener('fleet_update', onEvent);
        es.addEventListener('heartbeat', onEvent);
        es.onmessage = onEvent;
        es.onerror = function () {
          setSseBadge(false);
        };
      } catch (_e) {
        setSseBadge(false);
      }
    } else {
      setSseBadge(false);
    }

    state.pollTimer = setInterval(function () {
      if (state.activeView === 'fleet') fetchSummary({ fromSSE: true });
    }, 15000);
  }

  var initRealtimeSseAndPoller = connectSSE;

  function getCompactRowMap(kind) {
    if (kind === 'fleet') return state.expandedMobileBoxIds;
    if (kind === 'hardening') return state.expandedHardeningIds;
    if (kind === 'camera') return state.expandedCameraIds;
    if (kind === 'rbDomain') return state.expandedRedbidaDomainIds;
    if (kind === 'rbBox') return state.expandedRedbidaBoxIds;
    if (kind === 'analyticsTop') return state.expandedAnalyticsTopIds;
    if (kind === 'analyticsCam') return state.expandedAnalyticsCamIds;
    return null;
  }

  function syncFleetMobileRowsToggleBtn() {
    var matrixContainer = document.getElementById('fleet-matrix-container');
    if (!matrixContainer) return;
    var toggleBtn = matrixContainer.querySelector('[data-mobile-rows-action="toggle-all"]');
    if (!toggleBtn) return;
    var anyOpen = !!matrixContainer.querySelector('.mobile-box-card.is-expanded');
    toggleBtn.textContent = anyOpen ? 'Thu dòng' : 'Mở dòng';
  }

  function toggleCompactMobileRow(headEl) {
    if (!headEl) return;
    var kind = headEl.getAttribute('data-toggle-mrow');
    var id = headEl.getAttribute('data-mrow-id');
    var map = getCompactRowMap(kind);
    if (!map || !id) return;
    var nextExpanded = !map[id];
    if (nextExpanded) map[id] = true;
    else delete map[id];

    var rowEl = headEl.closest('article');
    if (rowEl) rowEl.classList.toggle('is-expanded', nextExpanded);
    headEl.setAttribute('aria-expanded', nextExpanded ? 'true' : 'false');

    if (kind === 'fleet') {
      state.lastFleetRenderSig = '';
      syncFleetMobileRowsToggleBtn();
    }
  }

  /* --------------------------------------------------------------------------
     Wire Event Listeners & Boot
     -------------------------------------------------------------------------- */
  function bindEvents() {
    /* Nav Tabs */
    document.querySelectorAll('.nav-tab[data-view]').forEach(function (btn) {
      btn.addEventListener('click', function () {
        switchView(btn.getAttribute('data-view'), true);
      });
    });

    /* Global Delegated Controls: Compact Mobile Accordion Rows + Pagination */
    document.addEventListener('click', function (e) {
      var pageBtn = e.target.closest('[data-pager-key][data-pager-page]');
      if (pageBtn && !pageBtn.disabled) {
        var key = pageBtn.getAttribute('data-pager-key');
        var page = Number(pageBtn.getAttribute('data-pager-page')) || 1;
        if (state.pagination[key] && state.pagination[key].page !== page) {
          state.pagination[key].page = Math.max(1, page);
          renderByPagerKey(key, true);
        }
        return;
      }

      /* Compact mobile row tap-to-expand across Fleet, Hardening, Cameras & RedBida */
      if (!e.target.closest('a, button, input, .cam-mrow-check-stop')) {
        var mrowHead = e.target.closest('[data-toggle-mrow]');
        if (mrowHead) {
          toggleCompactMobileRow(mrowHead);
          return;
        }
      }

      /* Mobile card overflow popover close when clicking outside */
      if (state.openCardMenuId && !e.target.closest('.mcard-foot-actions')) {
        closeAllCardMenus();
      }
    });

    document.addEventListener('keydown', function (e) {
      if (e.key !== 'Enter' && e.key !== ' ') return;
      if (e.target && (e.target.tagName === 'INPUT' || e.target.tagName === 'BUTTON' || e.target.tagName === 'A')) return;
      var mrowHead = e.target.closest ? e.target.closest('[data-toggle-mrow]') : null;
      if (!mrowHead) return;
      e.preventDefault();
      toggleCompactMobileRow(mrowHead);
    });

    document.addEventListener('change', function (e) {
      var sizeSel = e.target.closest('[data-pager-size]');
      if (!sizeSel) return;
      var key = sizeSel.getAttribute('data-pager-size');
      var val = Number(sizeSel.value) || 20;
      if (state.pagination[key]) {
        state.pagination[key].pageSize = val;
        state.pagination[key].page = 1;
        renderByPagerKey(key, false);
      }
    });

    /* Instant Clear (×) Buttons for All 5 Search Boxes */
    document.addEventListener('click', function (e) {
      var clearBtn = e.target.closest('[data-clear-search]');
      if (!clearBtn) return;
      var targetId = clearBtn.getAttribute('data-clear-search');
      var inp = document.getElementById(targetId);
      if (!inp) return;
      inp.value = '';
      if (targetId === 'fleet-search') {
        state.fleetSearch = '';
        resetTabPage('fleet');
        renderFleetMatrix({ force: true });
      } else if (targetId === 'hardening-search') {
        state.hardeningSearch = '';
        resetTabPage('hardening');
        renderHardeningMatrix();
      } else if (targetId === 'cam-search-input') {
        resetTabPage('cameras');
        renderCamerasTable();
      } else if (targetId === 'redbida-search') {
        state.redbidaSearch = '';
        resetTabPage('redbidaDomains');
        resetTabPage('redbidaBoxes');
        renderRedbidaView();
      } else if (targetId === 'handover-club-search') {
        state.handoverSearch = '';
        resetTabPage('handoverClubs');
        renderHandoverClubPicker();
      } else if (targetId === 'analytics-top-search') {
        state.analyticsTopSearch = '';
        resetTabPage('analyticsTop');
        renderAnalyticsTopTable(state.analyticsOverview || {});
      } else if (targetId === 'analytics-cameras-search') {
        state.analyticsCamerasSearch = '';
        resetTabPage('analyticsCameras');
        renderAnalyticsCamerasTable();
      }
      inp.focus();
    });

    /* Poll Now button */
    var pollBtn = document.getElementById('poll-now-btn');
    if (pollBtn) {
      pollBtn.addEventListener('click', function () {
        pollBtn.disabled = true;
        apiFetch('/fleet/api/poll', { method: 'POST', body: {} }).then(function () {
          showToast('Đã thăm dò tức thì toàn bộ đội Box Bida / Pickleball!');
          return loadFleetSummary();
        }).catch(function (err) {
          showToast('Lỗi Poll Now: ' + err.message, true);
        }).finally(function () {
          pollBtn.disabled = false;
        });
      });
    }

    /* Fleet Filter Chips, Fast Debounced Search, Mode Switch & Active Filter Removal */
    var chipsWrap = document.getElementById('fleet-filter-chips');
    if (chipsWrap) {
      chipsWrap.addEventListener('click', function (e) {
        var chip = e.target.closest('.filter-chip[data-filter]');
        if (!chip) return;
        state.fleetFilter = chip.getAttribute('data-filter');
        chipsWrap.querySelectorAll('.filter-chip').forEach(function (c) {
          c.classList.toggle('active', c === chip);
        });
        resetTabPage('fleet');
        renderFleetMatrix({ force: true });
      });
    }

    var fleetSearch = document.getElementById('fleet-search');
    if (fleetSearch) {
      var applyFleetSearch = makeDebounced(function () {
        state.fleetSearch = fleetSearch.value;
        resetTabPage('fleet');
        renderFleetMatrix({ force: true });
      }, 80);
      fleetSearch.addEventListener('input', function () {
        var wrap = fleetSearch.closest('.search-box');
        if (wrap) {
          var hasVal = fleetSearch.value.length > 0;
          wrap.classList.toggle('has-value', hasVal);
          var cb = wrap.querySelector('.search-clear-btn');
          if (cb) cb.hidden = !hasVal;
        }
        applyFleetSearch();
      });
      fleetSearch.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          applyFleetSearch.flush();
        }
      });
    }

    var fleetActiveFilters = document.getElementById('fleet-active-filters');
    if (fleetActiveFilters) {
      fleetActiveFilters.addEventListener('click', function (e) {
        var btn = e.target.closest('[data-remove-fleet-filter]');
        if (!btn) return;
        var which = btn.getAttribute('data-remove-fleet-filter');
        if (which === 'search' || which === 'all') {
          state.fleetSearch = '';
          if (fleetSearch) fleetSearch.value = '';
        }
        if (which === 'status' || which === 'all') {
          state.fleetFilter = 'all';
          if (chipsWrap) {
            chipsWrap.querySelectorAll('.filter-chip[data-filter]').forEach(function (c) {
              c.classList.toggle('active', c.getAttribute('data-filter') === 'all');
            });
          }
        }
        resetTabPage('fleet');
        renderFleetMatrix({ force: true });
      });
    }

    var grpToggle = document.getElementById('group-by-club-toggle');
    if (grpToggle) {
      grpToggle.addEventListener('change', function () {
        state.groupByClub = !!grpToggle.checked;
        resetTabPage('fleet');
        renderFleetMatrix({ force: true });
      });
    }

    var fleetViewMode = document.getElementById('fleet-view-mode');
    if (fleetViewMode) {
      fleetViewMode.addEventListener('click', function (e) {
        if (e.target && e.target.id === 'group-by-club-toggle') return;
        var flatBtn = e.target.closest('button[data-fleet-mode="flat"]');
        if (flatBtn) {
          if (state.groupByClub) {
            state.groupByClub = false;
            resetTabPage('fleet');
            renderFleetMatrix({ force: true });
          }
          return;
        }
        var clubLbl = e.target.closest('[data-fleet-mode="club"]');
        if (clubLbl) {
          e.preventDefault();
          if (!state.groupByClub) {
            state.groupByClub = true;
            resetTabPage('fleet');
            renderFleetMatrix({ force: true });
          }
        }
      });
    }

    /* Delegated click inside #fleet-matrix-container (Clubs Collapse, Ledger Actions, Mobile Popover, Open Box Drawer) */
    var matrixContainer = document.getElementById('fleet-matrix-container');
    if (matrixContainer) {
      matrixContainer.addEventListener('click', function (e) {
        var resetBtn = e.target.closest('[data-remove-fleet-filter]');
        if (resetBtn) {
          state.fleetSearch = '';
          state.fleetFilter = 'all';
          if (fleetSearch) fleetSearch.value = '';
          if (chipsWrap) {
            chipsWrap.querySelectorAll('.filter-chip[data-filter]').forEach(function (c) {
              c.classList.toggle('active', c.getAttribute('data-filter') === 'all');
            });
          }
          resetTabPage('fleet');
          renderFleetMatrix({ force: true });
          return;
        }

        var mobileRowsBtn = e.target.closest('[data-mobile-rows-action]');
        if (mobileRowsBtn) {
          var cards = matrixContainer.querySelectorAll('.mobile-box-card[data-card-box]');
          var anyExpanded = !!matrixContainer.querySelector('.mobile-box-card.is-expanded');
          cards.forEach(function (card) {
            var bid = card.getAttribute('data-card-box');
            if (!bid) return;
            if (anyExpanded) {
              delete state.expandedMobileBoxIds[bid];
              card.classList.remove('is-expanded');
            } else {
              state.expandedMobileBoxIds[bid] = true;
              card.classList.add('is-expanded');
            }
            var hd = card.querySelector('.mrow-head');
            if (hd) hd.setAttribute('aria-expanded', anyExpanded ? 'false' : 'true');
          });
          state.lastFleetRenderSig = '';
          syncFleetMobileRowsToggleBtn();
          return;
        }

        var ledgerBtn = e.target.closest('[data-clubs-action]');
        if (ledgerBtn) {
          var act = ledgerBtn.getAttribute('data-clubs-action');
          var sections = matrixContainer.querySelectorAll('.club-section[data-club-id]');
          sections.forEach(function (sec) {
            var cid = sec.getAttribute('data-club-id');
            if (!cid) return;
            if (act === 'collapse-all') state.collapsedClubs[cid] = true;
            else delete state.collapsedClubs[cid];
          });
          renderFleetMatrix({ force: true });
          return;
        }

        var moreBtn = e.target.closest('[data-card-menu-btn]');
        if (moreBtn) {
          e.stopPropagation();
          var bid = moreBtn.getAttribute('data-card-menu-btn');
          var wasSame = state.openCardMenuId === bid;
          closeAllCardMenus();
          if (!wasSame) {
            var pop = moreBtn.parentElement ? moreBtn.parentElement.querySelector('[data-card-menu="' + bid + '"]') : null;
            if (pop) {
              pop.classList.add('open');
              moreBtn.setAttribute('aria-expanded', 'true');
              state.openCardMenuId = bid;
            }
          }
          return;
        }

        if (e.target.closest('a')) return;

        var clubToggle = e.target.closest('[data-toggle-club]');
        if (clubToggle) {
          var clubId = clubToggle.getAttribute('data-toggle-club');
          if (clubId) {
            if (state.collapsedClubs[clubId]) delete state.collapsedClubs[clubId];
            else state.collapsedClubs[clubId] = true;
            renderFleetMatrix({ force: true });
          }
          return;
        }

        var trigger = e.target.closest('[data-open-box]');
        if (!trigger) return;
        closeAllCardMenus();
        openBoxDrawer(trigger.getAttribute('data-open-box'), true);
      });

      matrixContainer.addEventListener('keydown', function (e) {
        if (e.key !== 'Enter' && e.key !== ' ') return;
        var clubToggle = e.target.closest('[data-toggle-club]');
        if (!clubToggle) return;
        e.preventDefault();
        var clubId = clubToggle.getAttribute('data-toggle-club');
        if (clubId) {
          if (state.collapsedClubs[clubId]) delete state.collapsedClubs[clubId];
          else state.collapsedClubs[clubId] = true;
          renderFleetMatrix({ force: true });
        }
      });
    }

    var drawerClose = document.getElementById('drawer-close-btn');
    var drawerBackdrop = document.getElementById('box-drawer-backdrop');
    if (drawerClose) drawerClose.addEventListener('click', closeBoxDrawer);
    if (drawerBackdrop) drawerBackdrop.addEventListener('click', closeBoxDrawer);

    var drawerRemediateBtn = document.getElementById('drawer-remediate-btn');
    if (drawerRemediateBtn) {
      drawerRemediateBtn.addEventListener('click', function () {
        if (state.drawerBoxId) remediateBox(state.drawerBoxId);
      });
    }

    /* Hardening events */
    var hardFilter = document.getElementById('hardening-filter');
    var hardSearch = document.getElementById('hardening-search');
    if (hardFilter) {
      hardFilter.addEventListener('change', function () {
        state.hardeningFilter = hardFilter.value;
        resetTabPage('hardening');
        renderHardeningMatrix();
      });
    }
    if (hardSearch) {
      var applyHardSearch = makeDebounced(function () {
        state.hardeningSearch = hardSearch.value;
        resetTabPage('hardening');
        renderHardeningMatrix();
      }, 80);
      hardSearch.addEventListener('input', function () {
        var wrap = hardSearch.closest('.search-box');
        if (wrap) {
          var hasVal = hardSearch.value.length > 0;
          wrap.classList.toggle('has-value', hasVal);
          var cb = wrap.querySelector('.search-clear-btn');
          if (cb) cb.hidden = !hasVal;
        }
        applyHardSearch();
      });
      hardSearch.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          applyHardSearch.flush();
        }
      });
    }

    var hardContainer = document.getElementById('hardening-matrix-container');
    if (hardContainer) {
      hardContainer.addEventListener('click', function (e) {
        var remBtn = e.target.closest('[data-remediate-box]');
        if (remBtn) {
          remediateBox(remBtn.getAttribute('data-remediate-box'));
          return;
        }
        var inspBtn = e.target.closest('[data-inspect-box]');
        if (inspBtn) {
          inspectBoxHardening(inspBtn.getAttribute('data-inspect-box'));
          return;
        }
        var openBtn = e.target.closest('[data-open-box]');
        if (openBtn) {
          openBoxDrawer(openBtn.getAttribute('data-open-box'), true);
        }
      });
    }

    var bulkRemBtn = document.getElementById('bulk-remediate-btn');
    if (bulkRemBtn) {
      bulkRemBtn.addEventListener('click', function () {
        var targets = (state.hardeningBoxes || []).filter(function (b) { return (b.score || 0) < (b.max_score || 8); });
        if (!targets.length && state.boxes.length) {
          targets = state.boxes.slice(0, 3).map(function (b) { return { box_id: b.id }; });
        }
        Promise.all(targets.map(function (t) {
          return apiFetch('/fleet/api/boxes/' + encodeURIComponent(t.box_id || t.id) + '/hardening/remediate', {
            method: 'POST',
            body: { steps: [1, 2, 3, 4, 5, 6, 7, 8] }
          });
        })).then(function () {
          showToast('Đã vá tự động 8 bước Golden Hardening (Zero Format) cho toàn đội!');
          loadHardeningView();
          loadFleetSummary();
        }).catch(function (err) {
          showToast('Lỗi vá hàng loạt: ' + err.message, true);
        });
      });
    }

    /* Cameras events */
    ['cam-club-filter', 'cam-box-filter', 'cam-vendor-filter', 'cam-doublestream-filter'].forEach(function (id) {
      var el = document.getElementById(id);
      if (el) {
        el.addEventListener('change', function () {
          resetTabPage('cameras');
          renderCamerasTable();
        });
      }
    });

    var camSearch = document.getElementById('cam-search-input');
    if (camSearch) {
      var applyCamSearch = makeDebounced(function () {
        resetTabPage('cameras');
        renderCamerasTable();
      }, 80);
      camSearch.addEventListener('input', function () {
        var wrap = camSearch.closest('.search-box');
        if (wrap) {
          var hasVal = camSearch.value.length > 0;
          wrap.classList.toggle('has-value', hasVal);
          var cb = wrap.querySelector('.search-clear-btn');
          if (cb) cb.hidden = !hasVal;
        }
        applyCamSearch();
      });
      camSearch.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          applyCamSearch.flush();
        }
      });
    }

    var camSelectAll = document.getElementById('cam-select-all');
    if (camSelectAll) {
      camSelectAll.addEventListener('change', function () {
        var pg = paginateItems(getFilteredCameras(), 'cameras');
        pg.items.forEach(function (c) {
          state.selectedCameraIds[getCameraId(c)] = camSelectAll.checked;
        });
        renderCamerasTable();
      });
    }

    function onCameraRowCheckChange(e) {
      var cb = e.target.closest('.cam-row-check');
      if (!cb) return;
      state.selectedCameraIds[cb.getAttribute('data-cam-id')] = cb.checked;
      renderCamerasTable();
    }
    var camTbody = document.getElementById('cameras-tbody');
    var camMobileCards = document.getElementById('cameras-mobile-cards');
    if (camTbody) camTbody.addEventListener('change', onCameraRowCheckChange);
    if (camMobileCards) {
      camMobileCards.addEventListener('change', onCameraRowCheckChange);
      camMobileCards.addEventListener('click', function (e) {
        var openBtn = e.target.closest('[data-open-box]');
        if (openBtn) {
          openBoxDrawer(openBtn.getAttribute('data-open-box'), true);
        }
      });
    }

    var bulkH265Btn = document.getElementById('bulk-golden-h265-btn');
    if (bulkH265Btn) {
      bulkH265Btn.addEventListener('click', applyBulkGoldenH265);
    }

    /* RedBida events */
    var rbChips = document.getElementById('redbida-brand-chips');
    if (rbChips) {
      rbChips.addEventListener('click', function (e) {
        var chip = e.target.closest('.filter-chip[data-brand]');
        if (!chip) return;
        state.redbidaBrandFilter = chip.getAttribute('data-brand');
        rbChips.querySelectorAll('.filter-chip').forEach(function (c) {
          c.classList.toggle('active', c === chip);
        });
        resetTabPage('redbidaDomains');
        resetTabPage('redbidaBoxes');
        renderRedbidaView();
      });
    }
    var rbSearch = document.getElementById('redbida-search');
    if (rbSearch) {
      var applyRbSearch = makeDebounced(function () {
        state.redbidaSearch = rbSearch.value;
        resetTabPage('redbidaDomains');
        resetTabPage('redbidaBoxes');
        renderRedbidaView();
      }, 80);
      rbSearch.addEventListener('input', function () {
        var wrap = rbSearch.closest('.search-box');
        if (wrap) {
          var hasVal = rbSearch.value.length > 0;
          wrap.classList.toggle('has-value', hasVal);
          var cb = wrap.querySelector('.search-clear-btn');
          if (cb) cb.hidden = !hasVal;
        }
        applyRbSearch();
      });
      rbSearch.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          applyRbSearch.flush();
        }
      });
    }

    var rbDomainsContainer = document.getElementById('redbida-domains-container');
    if (rbDomainsContainer) {
      rbDomainsContainer.addEventListener('click', function (e) {
        var editBtn = e.target.closest('[data-open-redbida-edit]');
        if (editBtn) {
          openRedbidaModal(editBtn.getAttribute('data-open-redbida-edit'));
          return;
        }
        var openBtn = e.target.closest('[data-open-box]');
        if (openBtn) {
          openBoxDrawer(openBtn.getAttribute('data-open-box'), true);
        }
      });
    }

    var rbBoxesContainer = document.getElementById('redbida-boxes-container');
    if (rbBoxesContainer) {
      rbBoxesContainer.addEventListener('click', function (e) {
        var editBtn = e.target.closest('[data-open-redbida-edit]');
        if (editBtn) {
          openRedbidaModal(editBtn.getAttribute('data-open-redbida-edit'));
          return;
        }
        var autoBtn = e.target.closest('[data-autofix-redbida]');
        if (autoBtn) {
          submitRedbidaApply(autoBtn.getAttribute('data-autofix-redbida'), true);
          return;
        }
        var openBtn = e.target.closest('[data-open-box]');
        if (openBtn) {
          openBoxDrawer(openBtn.getAttribute('data-open-box'), true);
        }
      });
    }

    var rbModal = document.getElementById('redbida-editor-modal');
    var rbModalClose = document.getElementById('redbida-modal-close');
    if (rbModalClose && rbModal) {
      rbModalClose.addEventListener('click', function () {
        rbModal.classList.remove('open');
      });
    }
    if (rbModal) {
      rbModal.addEventListener('click', function (e) {
        if (e.target === rbModal) rbModal.classList.remove('open');
      });
    }
    var rbForm = document.getElementById('redbida-editor-form');
    if (rbForm) {
      rbForm.addEventListener('submit', function (e) {
        e.preventDefault();
        submitRedbidaApply(document.getElementById('rb-edit-box-id').value, false);
      });
    }
    var rbAutofixBtn = document.getElementById('rb-autofix-btn');
    if (rbAutofixBtn) {
      rbAutofixBtn.addEventListener('click', function () {
        submitRedbidaApply(document.getElementById('rb-edit-box-id').value, true);
      });
    }

    /* Handover events (Select + Fast Diacritic-Insensitive Club Search + Quick-Pick Strip) */
    var handoverSel = document.getElementById('handover-club-select');
    if (handoverSel) {
      handoverSel.addEventListener('change', function () {
        loadHandoverView(handoverSel.value);
      });
    }
    var handoverSearch = document.getElementById('handover-club-search');
    if (handoverSearch) {
      var applyHandoverSearch = makeDebounced(function () {
        state.handoverSearch = handoverSearch.value;
        resetTabPage('handoverClubs');
        renderHandoverClubPicker();
      }, 80);
      handoverSearch.addEventListener('input', function () {
        var wrap = handoverSearch.closest('.search-box');
        if (wrap) {
          var hasVal = handoverSearch.value.length > 0;
          wrap.classList.toggle('has-value', hasVal);
          var cb = wrap.querySelector('.search-clear-btn');
          if (cb) cb.hidden = !hasVal;
        }
        applyHandoverSearch();
      });
      handoverSearch.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          applyHandoverSearch.flush();
        }
      });
    }
    var handoverPicker = document.getElementById('handover-club-picker');
    if (handoverPicker) {
      handoverPicker.addEventListener('click', function (e) {
        var pill = e.target.closest('[data-pick-handover-club]');
        if (!pill) return;
        var cid = pill.getAttribute('data-pick-handover-club');
        if (cid) loadHandoverView(cid);
      });
    }

    var copyHandoverBtn = document.getElementById('copy-handover-btn');
    if (copyHandoverBtn) {
      copyHandoverBtn.addEventListener('click', function () {
        var txt = (document.getElementById('handover-template-text') || {}).value || '';
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(txt).then(function () {
            showToast('Đã sao chép gói bàn giao Zalo/Telegram vào bộ nhớ tạm!');
          });
        } else {
          showToast('Đã sao chép gói bàn giao!');
        }
      });
    }
    var copyClipBtn = document.getElementById('copy-clip-url-btn');
    if (copyClipBtn) {
      copyClipBtn.addEventListener('click', function () {
        var clipLink = document.getElementById('handover-clip-link');
        var url = clipLink ? clipLink.href : '';
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(url).then(function () {
            showToast('Đã sao chép Link Clip QR bền vững!');
          });
        }
      });
    }

    /* Analytics GA4 events */
    var aRangeChips = document.getElementById('analytics-range-chips');
    if (aRangeChips) {
      aRangeChips.addEventListener('click', function (e) {
        var chip = e.target.closest('.filter-chip[data-range]');
        if (!chip) return;
        state.analyticsRange = chip.getAttribute('data-range') || '7d';
        state.analyticsFrom = '';
        state.analyticsTo = '';
        aRangeChips.querySelectorAll('.filter-chip').forEach(function (c) {
          c.classList.toggle('active', c === chip);
        });
        resetTabPage('analyticsTop');
        resetTabPage('analyticsCameras');
        loadAnalyticsView();
      });
    }

    var aDateFrom = document.getElementById('analytics-date-from');
    var aDateTo = document.getElementById('analytics-date-to');
    function onAnalyticsCustomDateChange() {
      var f = aDateFrom ? aDateFrom.value : '';
      var t = aDateTo ? aDateTo.value : '';
      if (!f || !t || f > t) return;
      state.analyticsRange = 'custom';
      state.analyticsFrom = f;
      state.analyticsTo = t;
      if (aRangeChips) {
        aRangeChips.querySelectorAll('.filter-chip').forEach(function (c) {
          c.classList.remove('active');
        });
      }
      resetTabPage('analyticsTop');
      resetTabPage('analyticsCameras');
      loadAnalyticsView();
    }
    if (aDateFrom) aDateFrom.addEventListener('change', onAnalyticsCustomDateChange);
    if (aDateTo) aDateTo.addEventListener('change', onAnalyticsCustomDateChange);

    var aCompareToggle = document.getElementById('analytics-compare-toggle');
    if (aCompareToggle) {
      aCompareToggle.addEventListener('change', function () {
        state.analyticsCompare = !!aCompareToggle.checked;
        loadAnalyticsView();
      });
    }

    var aClubFilter = document.getElementById('analytics-club-filter');
    if (aClubFilter) {
      aClubFilter.addEventListener('change', function () {
        var val = aClubFilter.value || '';
        if (val) {
          drillDownAnalytics(val, '', resolveAnalyticsClubName(val));
        } else {
          clearAnalyticsDrillDown();
        }
      });
    }

    var aSportFilter = document.getElementById('analytics-sport-filter');
    if (aSportFilter) {
      aSportFilter.addEventListener('change', function () {
        state.analyticsSport = aSportFilter.value || 'all';
        resetTabPage('analyticsTop');
        resetTabPage('analyticsCameras');
        loadAnalyticsView();
      });
    }

    var aSourceFilter = document.getElementById('analytics-source-filter');
    if (aSourceFilter) {
      aSourceFilter.addEventListener('change', function () {
        state.analyticsSource = aSourceFilter.value || 'all';
        resetTabPage('analyticsTop');
        resetTabPage('analyticsCameras');
        loadAnalyticsView();
      });
    }

    var aAiOnly = document.getElementById('analytics-ai-only');
    if (aAiOnly) {
      aAiOnly.addEventListener('change', function () {
        state.analyticsAiOnly = !!aAiOnly.checked;
        resetTabPage('analyticsTop');
        resetTabPage('analyticsCameras');
        loadAnalyticsView();
      });
    }

    var aRefreshBtn = document.getElementById('analytics-refresh-btn');
    if (aRefreshBtn) {
      aRefreshBtn.addEventListener('click', function () {
        loadAnalyticsView();
      });
    }

    var aCrumbReset = document.getElementById('analytics-breadcrumb-reset');
    if (aCrumbReset) {
      aCrumbReset.addEventListener('click', clearAnalyticsDrillDown);
    }
    var aCrumbTrail = document.getElementById('analytics-breadcrumb-trail');
    if (aCrumbTrail) {
      aCrumbTrail.addEventListener('click', function (e) {
        var btn = e.target.closest('[data-ga-crumb]');
        if (!btn) return;
        var kind = btn.getAttribute('data-ga-crumb');
        if (kind === 'all') clearAnalyticsDrillDown();
        else if (kind === 'club') {
          var cid = btn.getAttribute('data-crumb-club') || '';
          if (cid) drillDownAnalytics(cid, '', resolveAnalyticsClubName(cid));
        }
      });
    }

    var aRtHotTabs = document.getElementById('ga-rt-hot-tabs');
    if (aRtHotTabs) {
      aRtHotTabs.addEventListener('click', function (e) {
        var btn = e.target.closest('.segmented-btn[data-rt-hot]');
        if (!btn) return;
        state.analyticsRtHotTab = btn.getAttribute('data-rt-hot') || 'clubs';
        syncRtHotTabsUi();
        renderAnalyticsRealtime();
      });
    }

    var aRtHotList = document.getElementById('ga-rt-hot-list');
    if (aRtHotList) {
      aRtHotList.addEventListener('click', function (e) {
        var item = e.target.closest('[data-ga-drill-club]');
        if (!item) return;
        drillDownAnalytics(
          item.getAttribute('data-ga-drill-club') || '',
          item.getAttribute('data-ga-drill-box') || '',
          item.getAttribute('data-ga-drill-label') || ''
        );
      });
    }

    var aMetricTabs = document.getElementById('ga-trend-metric-tabs');
    if (aMetricTabs) {
      aMetricTabs.addEventListener('click', function (e) {
        var tab = e.target.closest('.ga-metric-tab[data-trend-metric]');
        if (!tab) return;
        state.analyticsTrendMetric = tab.getAttribute('data-trend-metric') || 'unique_visitors';
        aMetricTabs.querySelectorAll('.ga-metric-tab').forEach(function (b) {
          var isAct = b === tab;
          b.classList.toggle('active', isAct);
          b.setAttribute('aria-selected', isAct ? 'true' : 'false');
        });
        renderAnalyticsDailyTrend(state.analyticsOverview || {});
      });
    }

    var aHeatTabs = document.getElementById('ga-heat-mode-tabs');
    if (aHeatTabs) {
      aHeatTabs.addEventListener('click', function (e) {
        var btn = e.target.closest('.segmented-btn[data-heat-mode]');
        if (!btn) return;
        state.analyticsHeatMode = btn.getAttribute('data-heat-mode') || 'heatmap';
        aHeatTabs.querySelectorAll('.segmented-btn').forEach(function (b) {
          b.classList.toggle('active', b === btn);
        });
        var hmView = document.getElementById('ga-heatmap-view');
        var barView = document.getElementById('analytics-peak-chart');
        if (hmView) hmView.hidden = state.analyticsHeatMode !== 'heatmap';
        if (barView) barView.hidden = state.analyticsHeatMode !== 'bars';
        if (state.analyticsHeatMode === 'heatmap') renderAnalyticsHeatmap(state.analyticsOverview || {});
        else renderAnalyticsPeakHours(state.analyticsOverview || {});
      });
    }

    var aTopSearch = document.getElementById('analytics-top-search');
    if (aTopSearch) {
      var applyATopSearch = makeDebounced(function () {
        state.analyticsTopSearch = aTopSearch.value;
        resetTabPage('analyticsTop');
        renderAnalyticsTopTable(state.analyticsOverview || {});
      }, 80);
      aTopSearch.addEventListener('input', function () {
        var wrap = aTopSearch.closest('.search-box');
        if (wrap) {
          var hasVal = aTopSearch.value.length > 0;
          wrap.classList.toggle('has-value', hasVal);
          var cb = wrap.querySelector('.search-clear-btn');
          if (cb) cb.hidden = !hasVal;
        }
        applyATopSearch();
      });
      aTopSearch.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); applyATopSearch.flush(); }
      });
    }

    var aCamSearch = document.getElementById('analytics-cameras-search');
    if (aCamSearch) {
      var applyACamSearch = makeDebounced(function () {
        state.analyticsCamerasSearch = aCamSearch.value;
        resetTabPage('analyticsCameras');
        renderAnalyticsCamerasTable();
      }, 80);
      aCamSearch.addEventListener('input', function () {
        var wrap = aCamSearch.closest('.search-box');
        if (wrap) {
          var hasVal = aCamSearch.value.length > 0;
          wrap.classList.toggle('has-value', hasVal);
          var cb = wrap.querySelector('.search-clear-btn');
          if (cb) cb.hidden = !hasVal;
        }
        applyACamSearch();
      });
      aCamSearch.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); applyACamSearch.flush(); }
      });
    }

    var aGroupToggle = document.getElementById('analytics-group-toggle');
    if (aGroupToggle) {
      aGroupToggle.addEventListener('click', function (e) {
        var btn = e.target.closest('.segmented-btn[data-group]');
        if (!btn) return;
        state.analyticsGroupMode = btn.getAttribute('data-group') || 'clubs';
        aGroupToggle.querySelectorAll('.segmented-btn').forEach(function (b) {
          b.classList.toggle('active', b === btn);
        });
        resetTabPage('analyticsTop');
        renderAnalyticsTopTable(state.analyticsOverview || {});
      });
    }

    var aTopWrap = document.getElementById('analytics-top-table-wrap');
    if (aTopWrap) {
      aTopWrap.addEventListener('click', function (e) {
        var th = e.target.closest('th[data-analytics-sort]');
        if (th) {
          var col = th.getAttribute('data-analytics-sort');
          if (state.analyticsSortCol === col) {
            state.analyticsSortDir = state.analyticsSortDir === 'desc' ? 'asc' : 'desc';
          } else {
            state.analyticsSortCol = col;
            state.analyticsSortDir = (col === 'name') ? 'asc' : 'desc';
          }
          resetTabPage('analyticsTop');
          renderAnalyticsTopTable(state.analyticsOverview || {});
          return;
        }
        var mDrillBtn = e.target.closest('button[data-ga-drill-club]');
        if (mDrillBtn) {
          drillDownAnalytics(
            mDrillBtn.getAttribute('data-ga-drill-club') || '',
            mDrillBtn.getAttribute('data-ga-drill-box') || '',
            mDrillBtn.getAttribute('data-ga-drill-label') || ''
          );
          return;
        }
        var row = e.target.closest('tr[data-drill-club], tr[data-drill-box]');
        if (!row) return;
        var dClub = row.getAttribute('data-drill-club') || '';
        var dBox = row.getAttribute('data-drill-box') || '';
        var dLabel = row.getAttribute('data-drill-label') || '';

        var sameClub = (dClub === state.analyticsClubId && !dBox && !state.analyticsBoxId);
        var sameBox = (dBox && dBox === state.analyticsBoxId);
        if (sameClub || sameBox) {
          clearAnalyticsDrillDown();
        } else {
          drillDownAnalytics(dClub, dBox, dLabel);
        }
      });
    }

    var aCamWrap = document.getElementById('analytics-cameras-table-wrap');
    if (aCamWrap) {
      aCamWrap.addEventListener('click', function (e) {
        var btn = e.target.closest('button[data-ga-drill-club]');
        if (btn) {
          drillDownAnalytics(
            btn.getAttribute('data-ga-drill-club') || '',
            '',
            btn.getAttribute('data-ga-drill-label') || ''
          );
          return;
        }
        var row = e.target.closest('tr[data-ga-drill-club]');
        if (row) {
          var cid = row.getAttribute('data-ga-drill-club') || '';
          if (cid && cid !== state.analyticsClubId) {
            drillDownAnalytics(cid, '', row.getAttribute('data-ga-drill-label') || '');
          }
        }
      });
    }

    var aClearDrillBtn = document.getElementById('analytics-clear-drill-btn');
    if (aClearDrillBtn) {
      aClearDrillBtn.addEventListener('click', clearAnalyticsDrillDown);
    }

    /* Pause/Resume GA4 Realtime polling on tab visibility changes (Zero background spam) */
    document.addEventListener('visibilitychange', function () {
      if (document.hidden) {
        stopAnalyticsRealtimePolling();
      } else if (state.activeView === 'analytics') {
        startAnalyticsRealtimePolling(true);
      }
    });

    window.addEventListener('hashchange', handleHashRoute);
  }

  /* Initialize Application */
  document.addEventListener('DOMContentLoaded', function () {
    initTheme();
    initCommandPalette();
    initChangePasswordModal();
    initMobileKpiToggle();
    initStickyCockpitBar();
    initAnalyticsTooltip();
    bindEvents();
    loadFleetSummary().then(function () {
      handleHashRoute();
    });
    initRealtimeSseAndPoller();
  });
})();
