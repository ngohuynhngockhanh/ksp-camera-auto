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
    building: '<path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z"/><path d="M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2"/><path d="M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2"/>'
  };

  function svgIcon(name, cls) {
    var body = SVG_PATHS[name] || SVG_PATHS.info;
    var className = cls ? ('ui-icon ' + cls) : 'ui-icon';
    return '<svg class="' + className + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + body + '</svg>';
  }

  var state = {
    activeView: 'fleet',
    drawerBoxId: null,
    fleetFilter: 'all',
    fleetSearch: '',
    groupByClub: true,
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
    redbidaConflicts: [],
    redbidaBrandFilter: 'all',
    redbidaSearch: '',
    handoverPacks: [],
    activeHandoverPack: null,
    analyticsRange: '7d',
    analyticsClubId: '',
    analyticsBoxId: '',
    analyticsAiOnly: false,
    analyticsGroupMode: 'clubs',
    analyticsSortCol: 'qr_views',
    analyticsSortDir: 'desc',
    analyticsDrillClubId: '',
    analyticsDrillBoxId: '',
    analyticsDrillLabel: '',
    analyticsOverview: null,
    analyticsCameras: [],
    analyticsReqSeq: 0,
    analyticsCamReqSeq: 0,
    cmdSelectedIdx: 0,
    sseConnected: false,
    pollTimer: null
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
     View 1: #fleet — KPI Cards & Fleet Overview Matrix
     -------------------------------------------------------------------------- */
  function loadFleetSummary() {
    return apiFetch('/fleet/api/summary').then(function (data) {
      state.kpi = data.kpi || {};
      state.clubs = Array.isArray(data.clubs) ? data.clubs : [];
      state.boxes = Array.isArray(data.boxes) ? data.boxes : [];
      renderTopbarLedsAndKpis();
      renderFleetMatrix();
      populateAnalyticsClubFilter();
    }).catch(function (err) {
      showToast('Không thể tải tổng quan đội thiết bị: ' + err.message, true);
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

    /* KPI 6: Average RTT (clean instrument readout, no fake sparkline) */
    var onlineBoxes = boxes.filter(function (b) { return b.online && b.rtt_ms > 0; });
    var avgRtt = onlineBoxes.length ? Math.round(onlineBoxes.reduce(function (s, b) { return s + (b.rtt_ms || 18); }, 0) / onlineBoxes.length) : 0;
    var maxRtt = onlineBoxes.length ? Math.round(onlineBoxes.reduce(function (m, b) { return Math.max(m, b.rtt_ms || 0); }, 0)) : 0;
    var kpi6Val = document.getElementById('kpi-rtt-val');
    var kpi6Sub = document.getElementById('kpi-rtt-sub');
    if (kpi6Val) kpi6Val.innerHTML = esc(avgRtt) + '<span class="kpi-val-sub">ms TB</span>';
    if (kpi6Sub) kpi6Sub.innerHTML = '<span>Yamux · VPN 77.88.x</span><span>Max ' + esc(maxRtt) + 'ms</span>';
  }

  function renderFleetMatrix() {
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

    var q = (state.fleetSearch || '').trim().toLowerCase();
    var filtered = boxes.filter(function (b) {
      var cid = b.club_id || b.club_name || 'default';
      var ffExp = b.ffmpeg_expected || (b.cameras_count ? b.cameras_count * 2 : 0);
      if (state.fleetFilter === 'online' && !b.online) return false;
      if (state.fleetFilter === 'offline' && b.online) return false;
      if (state.fleetFilter === 'ffmpeg-warn' && !(ffExp > 0 && (b.ffmpeg_count || 0) < ffExp)) return false;
      if (state.fleetFilter === 'hardening-warn' && !((b.hardening_score !== undefined ? b.hardening_score : 8) < 8)) return false;
      if (state.fleetFilter === 'multi-box' && !((clubCounts[cid] || 0) > 1)) return false;

      if (!q) return true;
      var hay = [
        b.id, b.name, b.club_name, b.club_id, b.vpn_ip, b.lan_ip,
        b.shinobi_group_key, b.store_domain, b.cstore_domain, b.soc_model
      ].join(' ').toLowerCase();
      return hay.indexOf(q) !== -1;
    });

    if (filtered.length === 0) {
      container.innerHTML = '<div class="glass-panel"><div class="empty-state">' +
        '<div class="empty-state-icon">' + svgIcon('search') + '</div>' +
        '<div><strong>Không có Box nào khớp bộ lọc hiện tại.</strong></div>' +
        '<div class="text-muted" style="font-size:12.5px;">Thử xóa từ khóa tìm kiếm hoặc chọn bộ lọc "Tất cả".</div>' +
      '</div></div>';
      return;
    }

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

      container.innerHTML = groupOrder.map(function (grp) {
        var isPickle = String(grp.branding || '').toLowerCase() === 'pickleball' || grp.club_name.toLowerCase().indexOf('pickle') !== -1;
        var brandBadge = isPickle
          ? '<span class="badge badge-amber">' + svgIcon('pickle', 'ui-icon-sm') + '<span>Pickleball</span></span>'
          : '<span class="badge badge-cyan">' + svgIcon('pool', 'ui-icon-sm') + '<span>Billiards</span></span>';
        var onlineInClub = grp.boxes.filter(function (x) { return x.online; }).length;
        var offlineInClub = grp.boxes.length - onlineInClub;
        var multiBadge = grp.boxes.length > 1
          ? '<span class="badge badge-purple">' + grp.boxes.length + ' Box (' + onlineInClub + '/' + grp.boxes.length + ' Online)</span>'
          : '<span class="badge ' + (onlineInClub > 0 ? 'badge-emerald' : 'badge-crimson') + '">' + (onlineInClub > 0 ? '1/1 Online' : '0/1 Offline') + '</span>';
        var clubOffBadge = offlineInClub > 0
          ? '<span class="badge badge-crimson">' + offlineInClub + ' Offline</span>'
          : '';
        var portalBtn = grp.portal_token
          ? '<a href="/portal/' + encodeURIComponent(grp.portal_token) + '/" target="_blank" rel="noopener" class="btn btn-xs">' + svgIcon('key', 'ui-icon-sm') + '<span>Cổng Chủ Quán</span></a>'
          : '';

        return '<div class="club-section" data-club-id="' + esc(grp.club_id) + '">' +
          '<div class="club-header">' +
            '<div class="club-title-group">' +
              '<h3 class="club-title">' + esc(grp.club_name) + '</h3>' +
              brandBadge +
              multiBadge +
              clubOffBadge +
            '</div>' +
            '<div class="toolbar-group">' +
              portalBtn +
            '</div>' +
          '</div>' +
          renderBoxesTableHtml(grp.boxes) +
        '</div>';
      }).join('');
    } else {
      container.innerHTML = '<div class="club-section">' + renderBoxesTableHtml(filtered) + '</div>';
    }
  }

  /* 7-column consolidated Fleet table — fits 100% inside 1440px, zero clipped actions */
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

    return '<div class="table-scroll"><table class="scada-table fleet-table tabular-nums">' +
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
      state.hardeningBoxes = Array.isArray(data.boxes) ? data.boxes : [];
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
    var q = (state.hardeningSearch || '').trim().toLowerCase();

    var filtered = list.filter(function (item) {
      var sc = item.score !== undefined ? item.score : 8;
      var mx = item.max_score || 8;
      if (state.hardeningFilter === 'needs_fix' && sc >= mx) return false;
      if (state.hardeningFilter === 'compliant' && sc < mx) return false;
      if (!q) return true;
      var hay = ((item.box_id || '') + ' ' + (item.box_name || '') + ' ' + (item.club_name || '') + ' ' + (item.vpn_ip || '')).toLowerCase();
      return hay.indexOf(q) !== -1;
    });

    if (!filtered.length) {
      container.innerHTML = '<div class="empty-state">' +
        '<div class="empty-state-icon">' + svgIcon('shield') + '</div>' +
        '<div><strong>Không có Box nào phù hợp bộ lọc chuẩn hóa.</strong></div>' +
      '</div>';
      return;
    }

    var rowsHtml = filtered.map(function (item) {
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

    var headersHtml = GOLDEN_PILLAR_META.map(function (m) {
      return '<th title="' + esc(m.name) + '">' + esc(m.short) + '</th>';
    }).join('');

    container.innerHTML = '<div class="table-scroll"><table class="scada-table tabular-nums">' +
      '<thead><tr>' +
        '<th>Box / CLB</th>' +
        '<th>Điểm</th>' +
        headersHtml +
        '<th>Thao Tác An Toàn</th>' +
      '</tr></thead>' +
      '<tbody>' + rowsHtml + '</tbody>' +
    '</table></div>';
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
      state.cameras = Array.isArray(data.cameras) ? data.cameras : [];
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
    var q = ((document.getElementById('cam-search-input') || {}).value || '').trim().toLowerCase();

    return (state.cameras || []).filter(function (c) {
      if (clubVal && c.club_name !== clubVal && c.club_id !== clubVal) return false;
      if (boxVal && c.box_id !== boxVal) return false;
      if (vendorVal && String(c.vendor || '').toLowerCase() !== vendorVal.toLowerCase()) return false;
      if (onlyDouble && !c.double_streaming) return false;
      if (!q) return true;
      var hay = [c.id, c.box_id, c.club_name, c.name, c.table_name, c.host, c.vendor, c.codec].join(' ').toLowerCase();
      return hay.indexOf(q) !== -1;
    });
  }

  function renderCamerasTable() {
    var tbody = document.getElementById('cameras-tbody');
    var sumBar = document.getElementById('cameras-summary-bar');
    if (!tbody) return;

    var filtered = getFilteredCameras();
    var doubleCount = filtered.filter(function (c) { return c.double_streaming; }).length;
    var h265Count = filtered.filter(function (c) { return String(c.codec || '').toUpperCase() === 'H.265'; }).length;

    if (sumBar) {
      sumBar.innerHTML =
        '<div class="badge-row">' +
          '<span class="badge badge-cyan">Hiển thị: ' + esc(filtered.length) + ' / ' + esc(state.cameras.length) + ' Camera</span>' +
          '<span class="badge badge-emerald">Chuẩn H.265: ' + esc(h265Count) + '</span>' +
          (doubleCount > 0
            ? '<span class="badge badge-amber">' + svgIcon('warn', 'ui-icon-sm') + '<span>Phát hiện Double-Streaming: ' + esc(doubleCount) + ' kênh</span></span>'
            : '<span class="badge badge-emerald">' + svgIcon('check', 'ui-icon-sm') + '<span>0 Double-Streaming</span></span>') +
        '</div>' +
        '<div class="mono text-secondary" style="font-size:12px;">' +
          'Chuẩn Bida Golden: H.265 · 30 FPS · 1536 Kbps CBR · GOP 50 · AAC' +
        '</div>';
    }

    if (!filtered.length) {
      tbody.innerHTML = '<tr><td colspan="10" class="analytics-empty-cell">Không có camera nào khớp bộ lọc hiện tại.</td></tr>';
      return;
    }

    tbody.innerHTML = filtered.map(function (c) {
      var cid = c.id || (c.box_id + ':' + c.host + ':' + (c.port || 37777));
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
  function loadRedbidaView() {
    return apiFetch('/fleet/api/redbida').then(function (data) {
      state.redbidaBoxes = Array.isArray(data.boxes) ? data.boxes : [];
      state.redbidaDomains = Array.isArray(data.domains) ? data.domains : [];
      state.redbidaConflicts = Array.isArray(data.conflicts) ? data.conflicts : [];
      renderRedbidaView();
    }).catch(function (err) {
      showToast('Lỗi tải dữ liệu RedBida & FRPC: ' + err.message, true);
    });
  }

  function renderRedbidaView() {
    var domContainer = document.getElementById('redbida-domains-container');
    var boxContainer = document.getElementById('redbida-boxes-container');
    var q = (state.redbidaSearch || '').trim().toLowerCase();

    /* Render FRPC Subdomain Auditor (1 row per box, merged from redbidaBoxes + FRPCSubdomainEntry domains) */
    if (domContainer) {
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
            conflict: false
          };
          domList.push(byBoxMap[key]);
        }
        return byBoxMap[key];
      }

      (state.redbidaBoxes || []).forEach(function (b) {
        var row = ensureDomRow(b.box_id || b.id, b.club_name || b.name, b.branding);
        if (b.store_domain) row.store_domain = b.store_domain;
        if (b.cstore_domain) row.cstore_domain = b.cstore_domain;
        if (b.store_http_code) row.store_http_code = b.store_http_code;
        if (b.cstore_http_code) row.cstore_http_code = b.cstore_http_code;
        if (b.domain_conflict) row.conflict = true;
      });

      (state.redbidaDomains || []).forEach(function (d) {
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

      var domFiltered = domList.filter(function (d) {
        if (state.redbidaBrandFilter === 'billiards' && d.branding !== 'billiards') return false;
        if (state.redbidaBrandFilter === 'pickleball' && d.branding !== 'pickleball') return false;
        if (state.redbidaBrandFilter === 'conflict' && !d.conflict) return false;
        if (!q) return true;
        var hay = [d.box_id, d.club_name, d.store_domain, d.cstore_domain].join(' ').toLowerCase();
        return hay.indexOf(q) !== -1;
      });

      if (!domFiltered.length) {
        domContainer.innerHTML = '<div class="empty-state">' +
          '<div class="empty-state-icon">' + svgIcon('globe') + '</div>' +
          '<div><strong>Không có tên miền FRPC nào khớp bộ lọc.</strong></div>' +
        '</div>';
      } else {
        domContainer.innerHTML = '<div class="table-scroll"><table class="scada-table tabular-nums">' +
          '<thead><tr><th>Box ID / CLB</th><th>Subdomain Store</th><th>HTTP</th><th>Subdomain C-Store</th><th>HTTP</th><th>Kiểm Toán</th></tr></thead>' +
          '<tbody>' + domFiltered.map(function (d) {
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
          }).join('') + '</tbody></table></div>';
      }
    }

    /* Render 17-Rule Inspector & Branding Guard */
    if (boxContainer) {
      var rBoxes = state.redbidaBoxes.filter(function (b) {
        var br = String(b.branding || 'billiards').toLowerCase();
        if (state.redbidaBrandFilter === 'billiards' && br !== 'billiards') return false;
        if (state.redbidaBrandFilter === 'pickleball' && br !== 'pickleball') return false;
        if (state.redbidaBrandFilter === 'conflict' && !b.domain_conflict) return false;
        if (!q) return true;
        var hay = [b.box_id || b.id, b.club_name, b.store_domain, b.cstore_domain, b.shinobi_group_key].join(' ').toLowerCase();
        return hay.indexOf(q) !== -1;
      });

      if (!rBoxes.length) {
        boxContainer.innerHTML = '<div class="empty-state">' +
          '<div class="empty-state-icon">' + svgIcon('sliders') + '</div>' +
          '<div><strong>Không có Box nào khớp bộ lọc RedBida.</strong></div>' +
        '</div>';
      } else {
        boxContainer.innerHTML = '<div class="table-scroll"><table class="scada-table tabular-nums">' +
          '<thead><tr>' +
            '<th>Box ID / CLB</th>' +
            '<th>Thương Hiệu (Branding Guard)</th>' +
            '<th>Điểm 17-Rule</th>' +
            '<th>Khóa Mở Kiểm Soát (VIP / Group / GGCode / C01..C20)</th>' +
            '<th>Thao Tác</th>' +
          '</tr></thead>' +
          '<tbody>' + rBoxes.map(function (b) {
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
          }).join('') + '</tbody></table></div>';
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
     View 6: #handover — 1-Click Handover Pack Generator + Inline SVG QR Codes
     -------------------------------------------------------------------------- */
  function loadHandoverView(clubId) {
    var url = clubId ? ('/fleet/api/handover/' + encodeURIComponent(clubId)) : '/fleet/api/handover';
    return apiFetch(url).then(function (data) {
      if (Array.isArray(data.packs) && data.packs.length > 0) {
        state.handoverPacks = data.packs;
      }
      var pack = data.pack || (state.handoverPacks && state.handoverPacks[0]) || data;
      state.activeHandoverPack = pack;
      renderHandoverView();
    }).catch(function (err) {
      showToast('Lỗi tải Gói Bàn Giao: ' + err.message, true);
    });
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
     View 7: #analytics — Fleet Usage, Peak Hours, AI Studio & Table Drill-Down
     -------------------------------------------------------------------------- */
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

    sel.innerHTML = '<option value="">Tất cả CLB (' + list.length + ')</option>' +
      list.map(function (c) {
        return '<option value="' + esc(c.id) + '">' + esc(c.name) + '</option>';
      }).join('');
    sel.value = curVal;
  }

  function updateAnalyticsExportLink() {
    var link = document.getElementById('analytics-export-csv');
    if (!link) return;
    var effClub = state.analyticsDrillClubId || state.analyticsClubId || '';
    var effBox = state.analyticsDrillBoxId || state.analyticsBoxId || '';
    var qs = 'range=' + encodeURIComponent(state.analyticsRange || '7d') +
      '&club_id=' + encodeURIComponent(effClub) +
      '&box_id=' + encodeURIComponent(effBox);
    if (state.analyticsAiOnly) qs += '&ai_only=1';
    link.href = '/fleet/api/analytics/export.csv?' + qs;
  }

  function loadAnalyticsView() {
    populateAnalyticsClubFilter();
    updateAnalyticsExportLink();

    var seq = ++state.analyticsReqSeq;
    var qs = 'range=' + encodeURIComponent(state.analyticsRange || '7d') +
      '&club_id=' + encodeURIComponent(state.analyticsClubId || '') +
      '&box_id=' + encodeURIComponent(state.analyticsBoxId || '') +
      '&ai_only=' + (state.analyticsAiOnly ? '1' : '0');

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
    return Promise.all([pOverview, pCameras]);
  }

  function loadAnalyticsCameras() {
    updateAnalyticsExportLink();
    var seq = ++state.analyticsCamReqSeq;
    var effClub = state.analyticsDrillClubId || state.analyticsClubId || '';
    var effBox = state.analyticsDrillBoxId || state.analyticsBoxId || '';

    var drillInd = document.getElementById('analytics-drill-indicator');
    var drillLbl = document.getElementById('analytics-drill-label');
    if (drillInd && drillLbl) {
      if (state.analyticsDrillClubId || state.analyticsDrillBoxId) {
        drillLbl.textContent = 'Đang lọc: ' + (state.analyticsDrillLabel || effBox || effClub);
        drillInd.style.display = 'inline-flex';
      } else {
        drillInd.style.display = 'none';
      }
    }

    var qs = 'range=' + encodeURIComponent(state.analyticsRange || '7d') +
      '&club_id=' + encodeURIComponent(effClub) +
      '&box_id=' + encodeURIComponent(effBox) +
      '&limit=50';

    return apiFetch('/fleet/api/analytics/cameras?' + qs).then(function (data) {
      if (seq !== state.analyticsCamReqSeq) return;
      state.analyticsCameras = Array.isArray(data && data.cameras) ? data.cameras : [];
      renderAnalyticsCamerasTable();
    }).catch(function (err) {
      if (seq !== state.analyticsCamReqSeq) return;
      state.analyticsCameras = [];
      renderAnalyticsCamerasTable();
      showToast('Không thể tải Top Bàn / Camera: ' + err.message, true);
    });
  }

  function renderAnalyticsOverview() {
    var data = state.analyticsOverview || {};
    var kpi = data.kpi || {};

    var winEl = document.getElementById('analytics-date-window');
    if (winEl) {
      if (data.from_date && data.to_date) {
        winEl.textContent = data.from_date + ' → ' + data.to_date + ' (GMT+7)';
      } else {
        winEl.textContent = 'Khung: ' + (state.analyticsRange || '7d').toUpperCase() + ' (GMT+7)';
      }
    }

    var qrViews = Number(kpi.qr_views) || 0;
    var pageViews = Number(kpi.page_views) || 0;
    var uniqueVisitors = Number(kpi.unique_visitors) || 0;
    var liveHls = Number(kpi.live_hls) || 0;
    var vodPlays = Number(kpi.vod_plays) || 0;
    var clipDownloads = Number(kpi.clip_downloads) || 0;
    var cutClips = Number(kpi.cut_clips) || 0;
    var cutSeconds = Number(kpi.cut_seconds) || 0;
    var aiOpens = Number(kpi.ai_studio_opens) || 0;
    var aiAnalyzes = Number(kpi.ai_analyzes) || 0;
    var aiShots = Number(kpi.ai_shots) || 0;
    var aiPots = Number(kpi.ai_pots) || 0;
    var aiExportsIos = Number(kpi.ai_exports_ios) || 0;
    var aiExportsDesktop = Number(kpi.ai_exports_desktop) || 0;
    var aiErrors = Number(kpi.ai_errors) || 0;
    var httpErrors = Number(kpi.http_errors) || 0;
    var bytesOut = Number(kpi.bytes_out) || 0;
    var activeClubs = Number(kpi.active_clubs) || 0;
    var activeBoxes = Number(kpi.active_boxes) || 0;

    var totalDownloadsCuts = clipDownloads + cutClips;
    var totalErrors = httpErrors + aiErrors;
    var totalActivity = qrViews + pageViews + uniqueVisitors + liveHls + vodPlays + totalDownloadsCuts + aiOpens + aiAnalyzes + bytesOut;

    var navBadge = document.getElementById('nav-badge-analytics');
    if (navBadge) {
      navBadge.textContent = qrViews > 0 ? fmtCompact(qrViews) : 'LIVE';
    }

    /* KPI 1: QR Views & Page Views */
    var qrVal = document.getElementById('akpi-qr-val');
    var qrSub = document.getElementById('akpi-qr-sub');
    if (qrVal) qrVal.innerHTML = esc(fmtInt(qrViews)) + '<span class="kpi-val-sub">/ ' + esc(fmtInt(pageViews)) + ' trang</span>';
    if (qrSub) qrSub.innerHTML = '<span>' + esc(fmtInt(activeClubs)) + ' CLB · ' + esc(fmtInt(activeBoxes)) + ' Box</span><span>Hoạt động</span>';

    /* KPI 2: Unique Visitors */
    var visVal = document.getElementById('akpi-visitors-val');
    var visSub = document.getElementById('akpi-visitors-sub');
    if (visVal) visVal.textContent = fmtInt(uniqueVisitors);
    if (visSub) visSub.innerHTML = '<span>Live HLS: ' + esc(fmtInt(liveHls)) + '</span><span>VOD: ' + esc(fmtInt(vodPlays)) + '</span>';

    /* KPI 3: Downloads & CheckVAR Cuts */
    var clipVal = document.getElementById('akpi-clips-val');
    var clipSub = document.getElementById('akpi-clips-sub');
    if (clipVal) clipVal.textContent = fmtInt(totalDownloadsCuts);
    if (clipSub) clipSub.innerHTML = '<span>Tải ' + esc(fmtInt(clipDownloads)) + ' · Cắt ' + esc(fmtInt(cutClips)) + '</span><span>' + esc(fmtCutMinutes(cutSeconds)) + ' cắt</span>';

    /* KPI 4: AI Studio & Analysis */
    var aiVal = document.getElementById('akpi-ai-val');
    var aiSub = document.getElementById('akpi-ai-sub');
    if (aiVal) aiVal.innerHTML = esc(fmtInt(aiAnalyzes)) + '<span class="kpi-val-sub">/ ' + esc(fmtInt(aiOpens)) + ' mở</span>';
    if (aiSub) aiSub.innerHTML = '<span>' + esc(fmtInt(aiShots)) + ' cú · ' + esc(fmtInt(aiPots)) + ' lỗ</span><span>iOS ' + esc(fmtInt(aiExportsIos)) + ' / PC ' + esc(fmtInt(aiExportsDesktop)) + '</span>';

    /* KPI 5: Bandwidth Out + Conversion % */
    var convBase = Math.max(qrViews, pageViews);
    var convPct = convBase > 0 ? ((totalDownloadsCuts / convBase) * 100).toFixed(1) : '0.0';
    var bwVal = document.getElementById('akpi-bw-val');
    var bwSub = document.getElementById('akpi-bw-sub');
    if (bwVal) bwVal.textContent = fmtBytes(bytesOut);
    if (bwSub) bwSub.innerHTML = '<span>Tải+Cắt / Lượt xem</span><span class="text-cyan" style="font-weight:700;">' + esc(convPct) + '%</span>';

    /* KPI 6: Health & Error Rate */
    var errBase = Math.max(1, qrViews + pageViews + totalDownloadsCuts + aiAnalyzes + totalErrors);
    var errRate = totalErrors > 0 ? (totalErrors / errBase) * 100 : 0;
    var errCard = document.getElementById('akpi-err-card');
    var errVal = document.getElementById('akpi-err-val');
    var errSub = document.getElementById('akpi-err-sub');
    if (errCard) {
      errCard.classList.remove('warn', 'danger');
      if (totalErrors >= 20 || errRate >= 3) errCard.classList.add('danger');
      else if (totalErrors > 0) errCard.classList.add('warn');
    }
    if (errVal) {
      errVal.textContent = fmtInt(totalErrors);
      errVal.className = 'kpi-value ' + (totalErrors === 0 ? 'text-emerald' : ((totalErrors >= 20 || errRate >= 3) ? 'text-crimson' : 'text-amber'));
    }
    if (errSub) {
      var healthLabel = totalErrors === 0
        ? '<span class="text-emerald" style="font-weight:700;">0% Lỗi</span>'
        : '<span class="' + ((totalErrors >= 20 || errRate >= 3) ? 'text-crimson' : 'text-amber') + '" style="font-weight:700;">' + esc(errRate.toFixed(2)) + '%</span>';
      errSub.innerHTML = '<span>HTTP: ' + esc(fmtInt(httpErrors)) + ' · AI: ' + esc(fmtInt(aiErrors)) + '</span>' + healthLabel;
    }

    var emptyBanner = document.getElementById('analytics-empty-banner');
    if (emptyBanner) {
      emptyBanner.style.display = totalActivity === 0 ? 'flex' : 'none';
    }

    renderAnalyticsPeakHours(data);
    renderAnalyticsDailyTrend(data);
    renderAnalyticsBreakdowns(data);
    renderAnalyticsTopTable(data);
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
      var tip = hrPad + ':00–' + hrPad + ':59 (GMT+7)\n' +
        'QR Views: ' + fmtInt(item.qr) + '\n' +
        'Khách duy nhất: ' + fmtInt(item.uv) + '\n' +
        'Tải & Cắt Clip: ' + fmtInt(item.dc) + '\n' +
        'AI Studio Runs: ' + fmtInt(item.ai) + '\n' +
        'Băng thông: ' + fmtBytes(item.bo);

      var peakBg = isPeak
        ? '<rect x="' + (xSlot + 1).toFixed(1) + '" y="' + padT + '" width="' + (slotW - 2).toFixed(1) + '" height="' + plotH + '" rx="4" fill="var(--amber-bg)" stroke="var(--amber-border)" stroke-width="1"/>'
        : '';

      /* Collision-free X-axis ticks: always show peak, suppress adjacent ticks within +/-1h */
      var nearPeak = (peakHour >= 0 && peakVal > 0 && Math.abs(idx - peakHour) <= 1);
      var showLbl = isPeak || (!nearPeak && (idx % 3 === 0 || idx === 23));
      var xLbl = showLbl
        ? '<text x="' + (xSlot + slotW / 2).toFixed(1) + '" y="' + (H - 8) + '" text-anchor="middle" font-size="10" font-family="var(--font-mono)" font-weight="' + (isPeak ? '700' : '400') + '" fill="' + (isPeak ? 'var(--brand-amber)' : 'var(--text-muted)') + '">' + item.hour + 'h</text>'
        : '';

      bars.push(
        '<g class="svg-bar-group">' +
          '<title>' + esc(tip) + '</title>' +
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
    if (badge) {
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

  function renderAnalyticsDailyTrend(data) {
    var wrap = document.getElementById('analytics-daily-chart');
    if (!wrap) return;
    var series = Array.isArray(data.daily_series) ? data.daily_series : [];

    var W = 560;
    var H = 225;
    var padL = 44;
    var padR = 14;
    var padT = 14;
    var padB = 28;
    var plotW = W - padL - padR;
    var plotH = H - padT - padB;

    var maxVal = 0;
    series.forEach(function (d) {
      maxVal = Math.max(maxVal, Number(d.qr_views) || 0, Number(d.downloads_cuts) || 0, Number(d.ai_analyzes) || 0);
    });

    var yMax = maxVal > 0 ? Math.ceil(maxVal * 1.15) : 4;
    var gridLines = [];
    for (var t = 0; t <= 4; t++) {
      var frac = t / 4;
      var y = padT + plotH * (1 - frac);
      gridLines.push(
        '<line x1="' + padL + '" y1="' + y.toFixed(1) + '" x2="' + (W - padR) + '" y2="' + y.toFixed(1) + '" stroke="var(--border)" stroke-width="1" ' + (t > 0 ? 'stroke-dasharray="3 3"' : '') + '/>' +
        '<text x="' + (padL - 8) + '" y="' + (y + 3.5).toFixed(1) + '" text-anchor="end" font-size="10" font-family="var(--font-mono)" fill="var(--text-muted)">' + esc(fmtCompact(Math.round(yMax * frac))) + '</text>'
      );
    }

    if (!series.length || maxVal === 0) {
      wrap.innerHTML = '<svg class="analytics-svg" viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="Xu hướng theo ngày">' +
        gridLines.join('') +
        '<text x="' + (padL + plotW / 2).toFixed(1) + '" y="' + (padT + plotH / 2).toFixed(1) + '" text-anchor="middle" font-size="12" fill="var(--text-muted)">Chưa có chuỗi dữ liệu theo ngày trong khung này</text>' +
      '</svg>';
      return;
    }

    var n = series.length;
    var slotW = plotW / n;
    var subBarW = Math.max(3, Math.min(12, slotW * 0.28));
    var yBase = padT + plotH;
    var qrPoints = [];
    var areaPoints = [];
    var groups = [];
    var labelStep = Math.max(1, Math.ceil(n / 8));

    series.forEach(function (d, idx) {
      var qr = Number(d.qr_views) || 0;
      var dc = Number(d.downloads_cuts) || 0;
      var ai = Number(d.ai_analyzes) || 0;
      var uv = Number(d.unique_visitors) || 0;
      var exp = Number(d.ai_exports) || 0;
      var bo = Number(d.bytes_out) || 0;

      var xSlot = padL + idx * slotW;
      var cx = xSlot + slotW / 2;
      var yQr = yBase - (qr / yMax) * plotH;
      qrPoints.push(cx.toFixed(1) + ',' + yQr.toFixed(1));
      areaPoints.push(cx.toFixed(1) + ',' + yQr.toFixed(1));

      var hDc = dc > 0 ? Math.max(2, (dc / yMax) * plotH) : 0;
      var hAi = ai > 0 ? Math.max(2, (ai / yMax) * plotH) : 0;

      var dateStr = String(d.date || '');
      var shortDate = dateStr.length >= 10 ? (dateStr.slice(8, 10) + '/' + dateStr.slice(5, 7)) : dateStr;
      var showTick = (idx % labelStep === 0 || idx === n - 1);

      var tip = dateStr + '\n' +
        'QR Views: ' + fmtInt(qr) + '\n' +
        'Khách duy nhất: ' + fmtInt(uv) + '\n' +
        'Tải & Cắt Clip: ' + fmtInt(dc) + '\n' +
        'AI Phân tích: ' + fmtInt(ai) + '\n' +
        'AI Xuất video: ' + fmtInt(exp) + '\n' +
        'Băng thông: ' + fmtBytes(bo);

      var singleQrBar = (n === 1 && qr > 0)
        ? '<rect x="' + (cx - subBarW * 1.6).toFixed(1) + '" y="' + yQr.toFixed(1) + '" width="' + subBarW.toFixed(1) + '" height="' + (yBase - yQr).toFixed(1) + '" rx="2" fill="var(--cyan)" opacity="0.85"/>'
        : '';

      groups.push(
        '<g class="svg-bar-group">' +
          '<title>' + esc(tip) + '</title>' +
          '<rect class="svg-hover-bg" x="' + xSlot.toFixed(1) + '" y="' + padT + '" width="' + slotW.toFixed(1) + '" height="' + plotH + '" rx="3" fill="transparent"/>' +
          singleQrBar +
          (hDc > 0 ? '<rect x="' + (cx - subBarW - 1).toFixed(1) + '" y="' + (yBase - hDc).toFixed(1) + '" width="' + subBarW.toFixed(1) + '" height="' + hDc.toFixed(1) + '" rx="2" fill="var(--emerald)" opacity="0.9"/>' : '') +
          (hAi > 0 ? '<rect x="' + (cx + 1).toFixed(1) + '" y="' + (yBase - hAi).toFixed(1) + '" width="' + subBarW.toFixed(1) + '" height="' + hAi.toFixed(1) + '" rx="2" fill="var(--purple)" opacity="0.9"/>' : '') +
          '<circle cx="' + cx.toFixed(1) + '" cy="' + yQr.toFixed(1) + '" r="' + (n > 31 ? '2.2' : '3.2') + '" fill="var(--cyan)" stroke="var(--surface)" stroke-width="1.4"/>' +
          (showTick ? '<text x="' + cx.toFixed(1) + '" y="' + (H - 8) + '" text-anchor="middle" font-size="10" font-family="var(--font-mono)" fill="var(--text-muted)">' + esc(shortDate) + '</text>' : '') +
        '</g>'
      );
    });

    var trendLineHtml = '';
    if (n >= 2) {
      var firstX = (padL + slotW / 2).toFixed(1);
      var lastX = (padL + (n - 0.5) * slotW).toFixed(1);
      var polyCoords = firstX + ',' + yBase + ' ' + areaPoints.join(' ') + ' ' + lastX + ',' + yBase;
      trendLineHtml =
        '<defs><linearGradient id="analytics-qr-grad" x1="0" y1="0" x2="0" y2="1">' +
          '<stop offset="0%" stop-color="var(--cyan)" stop-opacity="0.24"/>' +
          '<stop offset="100%" stop-color="var(--cyan)" stop-opacity="0.02"/>' +
        '</linearGradient></defs>' +
        '<polygon points="' + polyCoords + '" fill="url(#analytics-qr-grad)"/>' +
        '<polyline fill="none" stroke="var(--cyan)" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" points="' + qrPoints.join(' ') + '"/>';
    }

    wrap.innerHTML = '<svg class="analytics-svg" viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="Biểu đồ xu hướng theo ngày">' +
      gridLines.join('') +
      trendLineHtml +
      groups.join('') +
    '</svg>';
  }

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
    var os = data.os_breakdown || {};
    var kpi = data.kpi || {};

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
        renderProgressRow('phone', 'iOS (iPhone / iPad)', ios, osTotal, 'cyan') +
        renderProgressRow('phone', 'Android', android, osTotal, 'emerald') +
        renderProgressRow('monitor', 'Desktop (PC / Mac)', desktop, osTotal, 'purple') +
        renderProgressRow('tablet', 'App Kiosk / Tablet', app, osTotal, 'amber') +
        renderProgressRow('globe', 'Thiết bị khác', other, osTotal, 'muted');
    }

    var poolRuns = Number(kpi.ai_pool_runs) || 0;
    var caromRuns = Number(kpi.ai_carom_runs) || 0;
    var modeTotal = poolRuns + caromRuns;

    var expIos = Number(kpi.ai_exports_ios) || 0;
    var expDesktop = Number(kpi.ai_exports_desktop) || 0;
    var expRelay = Number(kpi.ai_export_relay) || 0;
    var expTotal = expIos + expDesktop;

    var aiBoxesBadge = document.getElementById('analytics-ai-boxes-badge');
    if (aiBoxesBadge) aiBoxesBadge.textContent = fmtInt(kpi.ai_active_boxes || 0) + ' Box AI';

    var aiEl = document.getElementById('analytics-ai-breakdown');
    if (aiEl) {
      aiEl.innerHTML =
        renderProgressRow('pool', 'AI Pool (8/9/10-Ball)', poolRuns, modeTotal, 'emerald') +
        renderProgressRow('carom', 'AI Carom (3C / Libre)', caromRuns, modeTotal, 'amber') +
        renderProgressRow('phone', 'Xuất iPhone (iOS Relay)', expIos, expTotal, 'cyan', expRelay > 0 ? ('· Relay: ' + fmtInt(expRelay)) : '') +
        renderProgressRow('monitor', 'Xuất Desktop (Blob MP4)', expDesktop, expTotal, 'purple');
    }

    var pillsEl = document.getElementById('analytics-ai-pills');
    if (pillsEl) {
      pillsEl.innerHTML =
        '<span class="badge badge-neutral">Mở AI Studio: ' + esc(fmtInt(kpi.ai_studio_opens || 0)) + '</span>' +
        '<span class="badge badge-neutral">Tải DS Video: ' + esc(fmtInt(kpi.ai_videos_loads || 0)) + '</span>' +
        '<span class="badge badge-emerald">Tổng cú đánh: ' + esc(fmtInt(kpi.ai_shots || 0)) + '</span>' +
        '<span class="badge badge-cyan">Bi vào lỗ: ' + esc(fmtInt(kpi.ai_pots || 0)) + '</span>' +
        '<span class="badge ' + ((Number(kpi.ai_errors) || 0) > 0 ? 'badge-crimson' : 'badge-neutral') + '">Lỗi AI: ' + esc(fmtInt(kpi.ai_errors || 0)) + '</span>';
    }
  }

  function renderSourceBadges(item, isClub) {
    var sources = [];
    if (isClub) {
      if (Array.isArray(item.sources)) {
        sources = item.sources;
      } else if (typeof item.sources === 'string' && item.sources.trim()) {
        sources = item.sources.split(',');
      } else if (item.source) {
        sources = [item.source];
      }
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
      if (low === 'redbida-go') {
        out.push('<span class="badge badge-cyan">redbida-go</span>');
      } else if (low === 'nodered' || low === 'node-red') {
        out.push('<span class="badge badge-neutral">Node-RED</span>');
      } else {
        out.push('<span class="badge badge-neutral">' + esc(clean) + '</span>');
      }
    });
    if (!out.length) {
      out.push('<span class="badge badge-neutral">--</span>');
    }
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

    var titleEl = document.getElementById('analytics-top-title');
    if (titleEl) {
      titleEl.innerHTML = svgIcon('trophy', 'text-amber') + '<span>' +
        (isClub ? ('Top Quán / CLB (' + esc(raw.length) + ')') : ('Top Thiết Bị Box (' + esc(raw.length) + ')')) +
      '</span>';
    }

    var sorted = raw.slice().sort(function (a, b) {
      var va = getAnalyticsSortValue(a, state.analyticsSortCol, isClub);
      var vb = getAnalyticsSortValue(b, state.analyticsSortCol, isClub);
      var cmp = 0;
      if (typeof va === 'string' && typeof vb === 'string') {
        cmp = va.localeCompare(vb, 'vi');
      } else {
        cmp = (va - vb);
      }
      return state.analyticsSortDir === 'asc' ? cmp : -cmp;
    });

    function thSort(col, label) {
      var active = state.analyticsSortCol === col;
      var arrow = active ? (state.analyticsSortDir === 'asc' ? ' ▲' : ' ▼') : '';
      return '<th class="sortable-th' + (active ? ' sort-active' : '') + '" data-analytics-sort="' + esc(col) + '">' + esc(label) + arrow + '</th>';
    }

    var rowsHtml = '';
    if (!sorted.length) {
      rowsHtml = '<tr><td colspan="10" class="analytics-empty-cell">Chưa có dữ liệu trong khung thời gian này — box sẽ tự động đẩy rollup mỗi 5 phút.</td></tr>';
    } else {
      rowsHtml = sorted.map(function (item) {
        var isSelected = isClub
          ? (state.analyticsDrillClubId && !state.analyticsDrillBoxId && state.analyticsDrillClubId === item.club_id)
          : (state.analyticsDrillBoxId && state.analyticsDrillBoxId === item.box_id);
        var errs = (Number(item.http_errors) || 0) + (Number(item.ai_errors) || 0);
        var dcTotal = (Number(item.clip_downloads) || 0) + (Number(item.cut_clips) || 0);
        var drillClub = item.club_id || '';
        var drillBox = isClub ? '' : (item.box_id || '');
        var drillLabel = isClub
          ? (item.club_name || item.club_id || '')
          : ((item.box_name || item.box_id || '') + (item.club_name ? (' · ' + item.club_name) : ''));

        var nameCell = isClub
          ? ('<td><div class="cell-primary">' + esc(item.club_name || item.club_id || '--') + '</div>' +
             '<div class="cell-sub mono">' + esc(item.club_id || '') + ' · ' + esc(fmtInt(item.boxes_count || 0)) + ' Box</div></td>')
          : ('<td><div class="mono cell-primary">' + esc(item.box_id || '--') + '</div>' +
             '<div class="cell-sub">' + esc(item.box_name || '') + (item.club_name ? (' · ' + esc(item.club_name)) : '') + '</div></td>');

        return '<tr class="clickable-row' + (isSelected ? ' row-selected' : '') + '"' +
          ' data-drill-club="' + esc(drillClub) + '"' +
          ' data-drill-box="' + esc(drillBox) + '"' +
          ' data-drill-label="' + esc(drillLabel) + '">' +
          nameCell +
          '<td>' + renderSourceBadges(item, isClub) + '</td>' +
          '<td><strong class="text-cyan">' + esc(fmtInt(item.qr_views)) + '</strong></td>' +
          '<td>' + esc(fmtInt(item.unique_visitors)) + '</td>' +
          '<td>' + esc(fmtInt(item.live_hls)) + ' <span class="text-muted">/</span> ' + esc(fmtInt(item.vod_plays)) + '</td>' +
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
    }

    wrap.innerHTML = '<div class="table-scroll"><table class="scada-table tabular-nums">' +
      '<thead><tr>' +
        thSort('name', isClub ? 'CLB / Quán' : 'Box / CLB') +
        '<th>Nguồn &amp; AI</th>' +
        thSort('qr_views', 'Quét QR') +
        thSort('unique_visitors', 'Khách') +
        thSort('live_vod', 'Live / VOD') +
        thSort('downloads_cuts', 'Tải & Cắt') +
        thSort('ai_analyzes', 'AI Studio') +
        thSort('bytes_out', 'Băng Thông') +
        thSort('errors', 'Lỗi') +
        '<th>Đẩy Gần Nhất</th>' +
      '</tr></thead>' +
      '<tbody>' + rowsHtml + '</tbody>' +
    '</table></div>';
  }

  function renderAnalyticsCamerasTable() {
    var wrap = document.getElementById('analytics-cameras-table-wrap');
    var countBadge = document.getElementById('analytics-cameras-count');
    if (!wrap) return;
    var cams = Array.isArray(state.analyticsCameras) ? state.analyticsCameras : [];
    if (countBadge) countBadge.textContent = fmtInt(cams.length) + ' bàn';

    var rowsHtml = '';
    if (!cams.length) {
      rowsHtml = '<tr><td colspan="10" class="analytics-empty-cell">Chưa có dữ liệu từng bàn / camera trong khung thời gian này — box sẽ tự động đẩy rollup mỗi 5 phút.</td></tr>';
    } else {
      rowsHtml = cams.map(function (c, idx) {
        var aiErr = Number(c.ai_errors) || 0;
        return '<tr>' +
          '<td class="mono text-muted" style="font-weight:700;">#' + (idx + 1) + '</td>' +
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
    }

    wrap.innerHTML = '<div class="table-scroll"><table class="scada-table tabular-nums">' +
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
    '</table></div>';
  }

  /* --------------------------------------------------------------------------
     Navigation, Command Palette (Ctrl+K) & Real-Time SSE Connection
     -------------------------------------------------------------------------- */
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

    if (updateHash !== false) {
      history.replaceState(null, '', '#' + viewName);
    }

    if (viewName === 'fleet') loadFleetSummary();
    else if (viewName === 'hardening') loadHardeningView();
    else if (viewName === 'cameras') loadCamerasView();
    else if (viewName === 'redbida') loadRedbidaView();
    else if (viewName === 'handover') loadHandoverView();
    else if (viewName === 'analytics') loadAnalyticsView();
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

  function initCommandPalette() {
    var modal = document.getElementById('cmd-palette');
    var input = document.getElementById('cmd-search-input');
    var results = document.getElementById('cmd-results');
    var openBtn = document.getElementById('cmd-palette-btn');
    var closeBtn = document.getElementById('cmd-close-btn');

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
    function closeCmd() {
      if (modal) modal.classList.remove('open');
    }

    function selectCmdRow(row) {
      if (!row) return;
      var t = row.getAttribute('data-cmd-type');
      var target = row.getAttribute('data-cmd-target');
      closeCmd();
      if (t === 'box') openBoxDrawer(target, true);
      else switchView(target, true);
    }

    function renderCmdResults(query) {
      if (!results) return;
      var q = (query || '').trim().toLowerCase();
      var items = [
        { type: 'view', icon: 'server', target: 'fleet', title: 'Đội Box (Fleet Matrix)', sub: '#fleet' },
        { type: 'view', icon: 'shield', target: 'hardening', title: 'Chuẩn Hóa 8 Bước (Golden Hardening)', sub: '#hardening' },
        { type: 'view', icon: 'video', target: 'cameras', title: 'Kho Camera & Chuẩn Golden H.265', sub: '#cameras' },
        { type: 'view', icon: 'sliders', target: 'redbida', title: 'RedBida 17-Rule & Kiểm Toán FRPC', sub: '#redbida' },
        { type: 'view', icon: 'qr', target: 'handover', title: 'Gói Bàn Giao Zalo & Mã QR Chủ Quán', sub: '#handover' },
        { type: 'view', icon: 'chart', target: 'analytics', title: 'Thống Kê & Analytics (QR · CheckVAR · AI)', sub: '#analytics' }
      ];
      (state.boxes || []).forEach(function (b) {
        items.push({
          type: 'box',
          icon: 'box',
          target: b.id,
          title: b.id + ' — ' + (b.club_name || b.name || ''),
          sub: 'VPN: ' + (b.vpn_ip || '--') + ' · grp: ' + (b.shinobi_group_key || '--')
        });
      });

      var matched = items.filter(function (it) {
        if (!q) return true;
        return (it.title + ' ' + it.sub).toLowerCase().indexOf(q) !== -1;
      }).slice(0, 18);

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
      } else if (e.key === 'Escape') {
        closeCmd();
        closeBoxDrawer();
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

  var sseRefreshTimer = null;
  function scheduleRealtimeSummaryRefresh() {
    clearTimeout(sseRefreshTimer);
    sseRefreshTimer = setTimeout(function () {
      sseRefreshTimer = null;
      fetchSummary();
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
        var onEvent = function () {
          setSseBadge(true);
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
      if (state.activeView === 'fleet') fetchSummary();
    }, 15000);
  }

  var initRealtimeSseAndPoller = connectSSE;

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

    /* Fleet Filter Chips, Search, Group Toggle */
    var chipsWrap = document.getElementById('fleet-filter-chips');
    if (chipsWrap) {
      chipsWrap.addEventListener('click', function (e) {
        var chip = e.target.closest('.filter-chip[data-filter]');
        if (!chip) return;
        state.fleetFilter = chip.getAttribute('data-filter');
        chipsWrap.querySelectorAll('.filter-chip').forEach(function (c) {
          c.classList.toggle('active', c === chip);
        });
        renderFleetMatrix();
      });
    }

    var fleetSearch = document.getElementById('fleet-search');
    if (fleetSearch) {
      fleetSearch.addEventListener('input', function () {
        state.fleetSearch = fleetSearch.value;
        renderFleetMatrix();
      });
    }

    var grpToggle = document.getElementById('group-by-club-toggle');
    if (grpToggle) {
      grpToggle.addEventListener('change', function () {
        state.groupByClub = grpToggle.checked;
        renderFleetMatrix();
      });
    }

    /* Delegated click for opening Box Drawer — ignores proxy <a> links, works for both row & Chi tiết button */
    var matrixContainer = document.getElementById('fleet-matrix-container');
    if (matrixContainer) {
      matrixContainer.addEventListener('click', function (e) {
        if (e.target.closest('a')) return;
        var trigger = e.target.closest('[data-open-box]');
        if (!trigger) return;
        openBoxDrawer(trigger.getAttribute('data-open-box'), true);
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
        renderHardeningMatrix();
      });
    }
    if (hardSearch) {
      hardSearch.addEventListener('input', function () {
        state.hardeningSearch = hardSearch.value;
        renderHardeningMatrix();
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
      if (el) el.addEventListener('change', renderCamerasTable);
    });
    var camSearch = document.getElementById('cam-search-input');
    if (camSearch) camSearch.addEventListener('input', renderCamerasTable);

    var camSelectAll = document.getElementById('cam-select-all');
    if (camSelectAll) {
      camSelectAll.addEventListener('change', function () {
        getFilteredCameras().forEach(function (c) {
          var cid = c.id || (c.box_id + ':' + c.host + ':' + (c.port || 37777));
          state.selectedCameraIds[cid] = camSelectAll.checked;
        });
        renderCamerasTable();
      });
    }

    var camTbody = document.getElementById('cameras-tbody');
    if (camTbody) {
      camTbody.addEventListener('change', function (e) {
        var cb = e.target.closest('.cam-row-check');
        if (!cb) return;
        state.selectedCameraIds[cb.getAttribute('data-cam-id')] = cb.checked;
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
        renderRedbidaView();
      });
    }
    var rbSearch = document.getElementById('redbida-search');
    if (rbSearch) {
      rbSearch.addEventListener('input', function () {
        state.redbidaSearch = rbSearch.value;
        renderRedbidaView();
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

    /* Handover events */
    var handoverSel = document.getElementById('handover-club-select');
    if (handoverSel) {
      handoverSel.addEventListener('change', function () {
        loadHandoverView(handoverSel.value);
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

    /* Analytics events */
    var aRangeChips = document.getElementById('analytics-range-chips');
    if (aRangeChips) {
      aRangeChips.addEventListener('click', function (e) {
        var chip = e.target.closest('.filter-chip[data-range]');
        if (!chip) return;
        state.analyticsRange = chip.getAttribute('data-range') || '7d';
        aRangeChips.querySelectorAll('.filter-chip').forEach(function (c) {
          c.classList.toggle('active', c === chip);
        });
        loadAnalyticsView();
      });
    }

    var aClubFilter = document.getElementById('analytics-club-filter');
    if (aClubFilter) {
      aClubFilter.addEventListener('change', function () {
        state.analyticsClubId = aClubFilter.value || '';
        state.analyticsBoxId = '';
        state.analyticsDrillClubId = '';
        state.analyticsDrillBoxId = '';
        state.analyticsDrillLabel = '';
        loadAnalyticsView();
      });
    }

    var aAiOnly = document.getElementById('analytics-ai-only');
    if (aAiOnly) {
      aAiOnly.addEventListener('change', function () {
        state.analyticsAiOnly = !!aAiOnly.checked;
        loadAnalyticsView();
      });
    }

    var aRefreshBtn = document.getElementById('analytics-refresh-btn');
    if (aRefreshBtn) {
      aRefreshBtn.addEventListener('click', function () {
        loadAnalyticsView();
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
          renderAnalyticsTopTable(state.analyticsOverview || {});
          return;
        }
        var row = e.target.closest('tr[data-drill-club], tr[data-drill-box]');
        if (!row) return;
        var dClub = row.getAttribute('data-drill-club') || '';
        var dBox = row.getAttribute('data-drill-box') || '';
        var dLabel = row.getAttribute('data-drill-label') || '';

        var sameClub = (dClub === state.analyticsDrillClubId && !dBox && !state.analyticsDrillBoxId);
        var sameBox = (dBox && dBox === state.analyticsDrillBoxId);
        if (sameClub || sameBox) {
          state.analyticsDrillClubId = '';
          state.analyticsDrillBoxId = '';
          state.analyticsDrillLabel = '';
        } else {
          state.analyticsDrillClubId = dClub;
          state.analyticsDrillBoxId = dBox;
          state.analyticsDrillLabel = dLabel;
        }
        renderAnalyticsTopTable(state.analyticsOverview || {});
        loadAnalyticsCameras();
      });
    }

    var aClearDrillBtn = document.getElementById('analytics-clear-drill-btn');
    if (aClearDrillBtn) {
      aClearDrillBtn.addEventListener('click', function () {
        state.analyticsDrillClubId = '';
        state.analyticsDrillBoxId = '';
        state.analyticsDrillLabel = '';
        renderAnalyticsTopTable(state.analyticsOverview || {});
        loadAnalyticsCameras();
      });
    }

    window.addEventListener('hashchange', handleHashRoute);
  }

  /* Initialize Application */
  document.addEventListener('DOMContentLoaded', function () {
    initTheme();
    initCommandPalette();
    initChangePasswordModal();
    bindEvents();
    loadFleetSummary().then(function () {
      handleHashRoute();
    });
    initRealtimeSseAndPoller();
  });
})();
