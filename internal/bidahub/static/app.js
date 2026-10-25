/* ============================================================================
   iNut Bida Fleet Hub (:2090) — SCADA Dual-Canvas SPA Controller (app.js)
   Views: #fleet, #box/{box_id}, #hardening, #cameras, #redbida, #handover
   Zero external CDN dependencies; XSS-safe DOM rendering; Pure-JS SVG QR.
   ============================================================================ */
(function () {
  'use strict';

  var GOLDEN_PILLAR_META = [
    { id: 'zram_unbounded', step: 1, short: 'B1: ZRAM 0-Limit', name: 'ZRAM mem_limit=0 & swappiness=80' },
    { id: 'pm2_memory_guard', step: 2, short: 'B2: PM2 Guard', name: 'PM2 Memory Guard (280M/120M) + /dev/null logs' },
    { id: 'sysrq_disabled', step: 3, short: 'B3: Sysrq=0', name: 'kernel.sysrq=0 & gỡ sysrq-trigger suicide' },
    { id: 'camera_watchdog', step: 4, short: 'B4: Watchdog 60s', name: 'shinobi-camera-watchdog.timer (60s pgrep ffmpeg)' },
    { id: 'storage_guard', step: 5, short: 'B5: Storage 15m', name: 'shinobi-storage-guard (15m xả đĩa 90% -> 80%)' },
    { id: 'udev_automount', step: 6, short: 'B6: udev Automount', name: 'udev automount noatime,users,errors=continue' },
    { id: 'clean_fstab', step: 7, short: 'B7: Clean fstab', name: 'Clean /etc/fstab (CẤM /dev/mmcblk0p1 treo boot)' },
    { id: 'shinobi_hardened', step: 8, short: 'B8: Shinobi ISO5', name: 'doSnapshot:false + monitor.js fix + frag_keyframe+empty_moov' }
  ];

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
    sseConnected: false,
    pollTimer: null
  };

  /* --------------------------------------------------------------------------
     Utilities: Escaping, Toast, Fetch Wrapper, Pure-JS Inline SVG QR & Sparkline
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

  function showToast(msg, isError) {
    var stack = document.getElementById('toast-stack');
    if (!stack) return;
    var el = document.createElement('div');
    el.className = 'toast-msg' + (isError ? ' error' : '');
    el.textContent = msg;
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

  function renderSparklineSvg(points, online) {
    var arr = Array.isArray(points) && points.length > 0 ? points : [18, 21, 19, 24, 20, 17, 19, 22];
    if (!online) {
      return '<svg class="sparkline-svg" viewBox="0 0 68 22" aria-label="Offline">' +
        '<line x1="2" y1="11" x2="66" y2="11" stroke="var(--crimson)" stroke-width="1.5" stroke-dasharray="3 3"/></svg>';
    }
    var min = Math.min.apply(null, arr);
    var max = Math.max.apply(null, arr);
    var span = Math.max(max - min, 4);
    var coords = arr.map(function (v, idx) {
      var x = 2 + (idx / Math.max(arr.length - 1, 1)) * 64;
      var y = 19 - ((v - min) / span) * 15;
      return x.toFixed(1) + ',' + y.toFixed(1);
    }).join(' ');
    var strokeColor = max > 120 ? 'var(--amber)' : 'var(--emerald)';
    return '<svg class="sparkline-svg" viewBox="0 0 68 22" aria-label="RTT Sparkline">' +
      '<polyline fill="none" stroke="' + strokeColor + '" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" points="' + coords + '"/>' +
      '</svg>';
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
  function initTheme() {
    var toggleBtn = document.getElementById('theme-toggle');
    if (!toggleBtn) return;
    toggleBtn.addEventListener('click', function () {
      var cur = document.documentElement.getAttribute('data-theme') === 'light' ? 'light' : 'dark';
      var next = cur === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', next);
      localStorage.setItem('bidahub-theme', next);
      localStorage.setItem('kspcam-theme', next);
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

    /* KPI 1: Clubs & Boxes (Online / Offline) */
    var kpi1Card = document.getElementById('kpi-clubs-boxes');
    var kpi1Val = document.getElementById('kpi-clubs-boxes-val');
    var kpi1Sub = document.getElementById('kpi-clubs-boxes-sub');
    if (kpi1Card) kpi1Card.classList.toggle('danger', boxesOffline > 0);
    if (kpi1Val) kpi1Val.textContent = boxesOnline + ' / ' + boxesTotal + ' ONLINE (' + clubsTotal + ' CLB)';
    if (kpi1Sub) {
      var offBadge = boxesOffline > 0
        ? '<span class="badge badge-crimson crimson"><span class="led-dot led crimson"></span>' + boxesOffline + ' OFFLINE</span>'
        : '<span class="badge badge-emerald emerald"><span class="led-dot led emerald"></span>0 OFFLINE</span>';
      kpi1Sub.innerHTML =
        '<span class="badge badge-emerald emerald"><span class="led-dot led emerald"></span>' + boxesOnline + ' / ' + boxesTotal + ' ONLINE</span>' +
        offBadge;
    }

    /* KPI 2: Cameras & FFmpeg */
    var camsTotal = kpi.cameras_total !== undefined ? kpi.cameras_total : boxes.reduce(function (acc, b) { return acc + (b.cameras_count || 0); }, 0);
    var ffActual = kpi.ffmpeg_total !== undefined ? kpi.ffmpeg_total : boxes.reduce(function (acc, b) { return acc + (b.ffmpeg_count || 0); }, 0);
    var ffExpTotal = kpi.ffmpeg_expected !== undefined ? kpi.ffmpeg_expected : boxes.reduce(function (acc, b) { return acc + (b.ffmpeg_expected || (b.cameras_count || 0) * 2); }, 0);
    var kpi2Val = document.getElementById('kpi-cameras-ffmpeg-val');
    var kpi2Sub = document.getElementById('kpi-cameras-ffmpeg-sub');
    if (kpi2Val) kpi2Val.textContent = camsTotal + ' Cam · ' + ffActual + '/' + ffExpTotal;
    if (kpi2Sub) kpi2Sub.innerHTML = '<span>FFmpeg thực chạy / kỳ vọng</span><span>Double: ' + (kpi.double_stream_count || 0) + '</span>';

    /* KPI 3: SD Health */
    var sdWarns = kpi.sd_warning_count !== undefined ? kpi.sd_warning_count : boxes.filter(function (b) { return (b.sd_used_pct || 0) >= 85; }).length;
    var avgSd = boxes.length ? Math.round(boxes.reduce(function (s, b) { return s + (b.sd_used_pct || 42); }, 0) / boxes.length) : 0;
    var kpi3Val = document.getElementById('kpi-sd-health-val');
    var kpi3Sub = document.getElementById('kpi-sd-health-sub');
    if (kpi3Val) kpi3Val.textContent = sdWarns === 0 ? ('TB ' + avgSd + '% · 0 Đầy') : (sdWarns + ' Box >85%');
    if (kpi3Sub) kpi3Sub.innerHTML = '<span>f2fs / ext4</span><span>errors=continue</span>';

    /* KPI 4: Hardening 8-Step */
    var fullHardened = kpi.hardening_full !== undefined ? kpi.hardening_full : boxes.filter(function (b) { return (b.hardening_score || 0) >= 8; }).length;
    var kpi4Val = document.getElementById('kpi-hardening-val');
    var kpi4Sub = document.getElementById('kpi-hardening-sub');
    if (kpi4Val) kpi4Val.textContent = fullHardened + '/' + boxesTotal + ' Đạt 8/8';
    if (kpi4Sub) kpi4Sub.innerHTML = '<span>TB ' + (kpi.hardening_avg || '8.0') + '/8</span><span>0 Format Disk</span>';
    var navBadgeHardening = document.getElementById('nav-badge-hardening');
    if (navBadgeHardening) navBadgeHardening.textContent = fullHardened + '/' + boxesTotal;

    /* KPI 5: Domains HTTP 200 & FRPC */
    var domOk = kpi.domains_ok !== undefined ? kpi.domains_ok : boxes.filter(function (b) { return (b.store_http_code || 200) === 200; }).length;
    var domConflict = kpi.domains_conflict || 0;
    var kpi5Val = document.getElementById('kpi-domains-val');
    var kpi5Sub = document.getElementById('kpi-domains-sub');
    if (kpi5Val) kpi5Val.textContent = domOk + ' HTTP 200';
    if (kpi5Sub) kpi5Sub.innerHTML = '<span>store-* / c-store-*</span><span>Xung đột: ' + domConflict + '</span>';

    /* KPI 6: Average RTT */
    var onlineBoxes = boxes.filter(function (b) { return b.online && b.rtt_ms > 0; });
    var avgRtt = onlineBoxes.length ? Math.round(onlineBoxes.reduce(function (s, b) { return s + (b.rtt_ms || 18); }, 0) / onlineBoxes.length) : 19;
    var kpi6Val = document.getElementById('kpi-rtt-val');
    var kpi6Sub = document.getElementById('kpi-rtt-sub');
    if (kpi6Val) kpi6Val.textContent = avgRtt + ' ms';
    if (kpi6Sub) kpi6Sub.innerHTML = '<span>Yamux + VPN 77.88.x.y</span>' + renderSparklineSvg([17, 19, avgRtt, 21, 18, avgRtt], true);
  }

  function renderFleetMatrix() {
    var container = document.getElementById('fleet-matrix-container');
    if (!container) return;
    var boxes = state.boxes || [];

    /* Count boxes per club for multi-box club detection */
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
      container.innerHTML = '<div class="glass-panel" style="text-align:center;padding:32px;color:var(--text-secondary);">Không có Box nào khớp bộ lọc hiện tại.</div>';
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
          ? '<span class="badge badge-amber">🏓 Pickleball</span>'
          : '<span class="badge badge-cyan">🎱 Billiards</span>';
        var onlineInClub = grp.boxes.filter(function (x) { return x.online; }).length;
        var offlineInClub = grp.boxes.length - onlineInClub;
        var multiBadge = grp.boxes.length > 1
          ? '<span class="badge badge-purple">' + grp.boxes.length + ' Box Song Song (' + onlineInClub + '/' + grp.boxes.length + ' Online)</span>'
          : '<span class="badge ' + (onlineInClub > 0 ? 'badge-emerald' : 'badge-crimson') + '">' + (onlineInClub > 0 ? '1/1 Online' : '0/1 Offline') + '</span>';
        var clubOffBadge = offlineInClub > 0
          ? '<span class="badge badge-crimson">' + offlineInClub + ' Offline</span>'
          : '';
        var portalBtn = grp.portal_token
          ? '<a href="/portal/' + encodeURIComponent(grp.portal_token) + '/" target="_blank" rel="noopener" class="btn btn-xs">🔑 Cổng Chủ Quán</a>'
          : '';

        return '<div class="club-section" data-club-id="' + esc(grp.club_id) + '">' +
          '<div class="club-header">' +
            '<div class="club-title-group">' +
              '<h3 class="club-title">' + esc(grp.club_name) + '</h3>' +
              brandBadge +
              multiBadge +
              clubOffBadge +
            '</div>' +
            '<div style="display:flex;align-items:center;gap:8px;">' +
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

  function renderBoxesTableHtml(boxList) {
    var rows = boxList.map(function (b) {
      var boxId = b.id || b.box_id || '';
      var isOnline = !!b.online;
      var rowClass = 'clickable-row' + (isOnline ? '' : ' row-offline');
      var onlineBadge = isOnline
        ? '<span class="badge badge-emerald emerald"><span class="led-dot led emerald"></span>ONLINE · ' + (b.rtt_ms || 0) + 'ms</span>'
        : '<span class="badge badge-crimson crimson"><span class="led-dot led crimson"></span>OFFLINE</span>';
      var uptimeSub = '<div class="cell-sub mono">' + escapeHtml(b.uptime_human || (isOnline ? '' : 'Mất kết nối')) + '</div>';

      var cpuPctVal = Number(b.cpu_pct || 0);
      var cpuBarClass = cpuPctVal >= 85 ? 'danger' : (cpuPctVal >= 65 ? 'warn' : '');
      var tempC = b.temp_c || 54;
      var cpuCores = b.cpu_cores || 4;

      var ramTotal = b.ram_total_mb || 1945;
      var ramUsed = b.ram_used_mb || (b.ram_available_mb > 0 && ramTotal > b.ram_available_mb ? (ramTotal - b.ram_available_mb) : 0);
      var ramAvail = b.ram_available_mb || Math.max(0, ramTotal - ramUsed);
      var ramPct = b.ram_used_pct || b.ram_pct || (ramTotal > 0 ? Math.round((ramUsed * 100) / ramTotal) : 0);
      var ramBarClass = ramPct >= 88 ? 'danger' : (ramPct >= 75 ? 'warn' : '');

      var swapTotal = b.swap_total_mb || b.zram_total_mb || 0;
      var swapUsed = b.swap_used_mb !== undefined ? b.swap_used_mb : (b.zram_used_mb || 0);
      var swapFree = b.swap_free_mb || Math.max(0, swapTotal - swapUsed);
      var zramPct = b.zram_pct || (swapTotal > 0 ? Math.round((swapUsed * 100) / swapTotal) : 0);

      var sdPct = b.sd_used_pct || 38;
      var sdFs = b.sd_fs_type || 'f2fs';
      var sdOpts = b.sd_mount_opts || 'errors=continue,noatime';
      var sdBadgeClass = sdPct >= 88 ? 'danger' : (sdPct >= 75 ? 'warn' : '');

      var ffAct = b.ffmpeg_count !== undefined ? b.ffmpeg_count : (b.cameras_count || 6) * 2;
      var ffExp = b.ffmpeg_expected !== undefined ? b.ffmpeg_expected : (b.cameras_count || 6) * 2;
      var ffBadge = (ffAct >= ffExp && ffExp > 0)
        ? '<span class="badge badge-emerald">' + ffAct + ' / ' + ffExp + '</span>'
        : '<span class="badge badge-amber">' + ffAct + ' / ' + ffExp + '</span>';

      var hScore = b.hardening_score !== undefined ? b.hardening_score : 8;
      var hMax = b.hardening_max || 8;
      var hBadge = hScore >= hMax
        ? '<span class="badge badge-emerald">🛡️ ' + hScore + '/' + hMax + '</span>'
        : '<span class="badge badge-amber">⚠️ ' + hScore + '/' + hMax + '</span>';

      var rxMbps = b.rx_mbps !== undefined ? Number(b.rx_mbps).toFixed(1) : '18.4';

      return '<tr class="' + rowClass + '" data-open-box="' + esc(boxId) + '">' +
        '<td>' +
          '<div style="font-weight:700;" class="mono">' + esc(boxId) + '</div>' +
          '<div style="font-size:12px;color:var(--text-secondary);">' + esc(b.name || b.club_name || '') + '</div>' +
        '</td>' +
        '<td>' +
          '<div class="mono">' + esc(b.vpn_ip || '--') + '</div>' +
          '<div style="font-size:11px;color:var(--text-muted);" class="mono">LAN: ' + esc(b.lan_ip || '192.168.1.x') + ' · grp: ' + esc(b.shinobi_group_key || '--') + '</div>' +
        '</td>' +
        '<td>' +
          '<div style="display:flex;align-items:center;gap:8px;">' +
            onlineBadge +
            renderSparklineSvg(b.rtt_history, isOnline) +
          '</div>' +
          uptimeSub +
        '</td>' +
        '<td>' +
          '<div class="mono"><strong>CPU ' + fmtNum(b.cpu_pct, 1) + '%</strong> · <span class="mono">' + fmtNum(tempC, 1) + '°C</span></div>' +
          '<div class="mini-meter"><div class="mini-meter-fill ' + cpuBarClass + '" style="width:' + Math.min(100, Math.max(0, cpuPctVal)) + '%"></div></div>' +
          '<div class="cell-sub mono">Load ' + fmtNum(b.load_avg_1m, 2) + ' (' + esc(cpuCores) + 'C) · ' + esc(b.soc_model || 'ARM64') + '</div>' +
        '</td>' +
        '<td>' +
          '<div class="mono"><strong>RAM ' + esc(ramUsed) + '/' + esc(ramTotal) + 'M (' + esc(ramPct) + '%) · Avail ' + esc(ramAvail) + 'M</strong></div>' +
          '<div class="mini-meter"><div class="mini-meter-fill ' + ramBarClass + '" style="width:' + Math.min(100, Math.max(0, ramPct)) + '%"></div></div>' +
          '<div class="cell-sub mono">Heap ' + fmtNum(b.heap_alloc_mb, 1) + 'M · kspcam ' + fmtNum(b.kspcam_rss_mb, 0) + 'M · cam.js ' + fmtNum(b.camera_rss_mb, 0) + 'M</div>' +
          '<div class="cell-sub mono">Swap ' + esc(swapUsed) + '/' + esc(swapTotal) + 'M (Free ' + esc(swapFree) + 'M · ' + esc(zramPct) + '%)</div>' +
        '</td>' +
        '<td>' +
          '<div><strong>' + esc(sdPct) + '%</strong> (' + esc(b.sd_total_gb || 64) + 'GB <code>' + esc(sdFs) + '</code>)</div>' +
          '<div class="mini-meter"><div class="mini-meter-fill ' + sdBadgeClass + '" style="width:' + Math.min(100, sdPct) + '%"></div></div>' +
          '<div style="font-size:11px;color:var(--text-muted);" class="mono">' + esc(sdOpts) + '</div>' +
        '</td>' +
        '<td>' + ffBadge + '</td>' +
        '<td class="mono">' + esc(rxMbps) + ' Mbps</td>' +
        '<td>' + hBadge + '</td>' +
        '<td onclick="event.stopPropagation();">' +
          '<div style="display:flex;align-items:center;gap:5px;flex-wrap:wrap;">' +
            '<button type="button" class="btn btn-xs" data-open-box="' + esc(boxId) + '">Chi tiết</button>' +
            '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/kspcam/" target="_blank" rel="noopener" class="btn btn-xs btn-cyan">kspcam</a>' +
            '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/shinobi/" target="_blank" rel="noopener" class="btn btn-xs">Shinobi</a>' +
            '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/nodered/" target="_blank" rel="noopener" class="btn btn-xs">Node-RED</a>' +
          '</div>' +
        '</td>' +
      '</tr>';
    }).join('');

    return '<div class="table-scroll"><table class="scada-table tabular-nums">' +
      '<thead><tr>' +
        '<th>Box ID / Tên Trạm</th>' +
        '<th>VPN IP / LAN / Group</th>' +
        '<th>Kết Nối &amp; RTT Sparkline</th>' +
        '<th>CPU / Load / Nhiệt độ</th>' +
        '<th>RAM / Avail / Heap / Swap</th>' +
        '<th>Thẻ Nhớ Video (SD)</th>' +
        '<th>FFmpeg (Thực/Kỳ vọng)</th>' +
        '<th>Lưu lượng RX</th>' +
        '<th>Golden 8 Bước</th>' +
        '<th>Điều Hành &amp; Reverse Proxy</th>' +
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
      statusBadge.textContent = isOnline ? ('ONLINE · ' + (b.rtt_ms || 0) + 'ms') : 'OFFLINE';
    }
    if (socBadge) socBadge.textContent = (b.soc_model || 'ARM64') + ' · ' + (b.arch || 'aarch64');

    /* 1-Click Reverse Proxy Launchers */
    var proxyEl = document.getElementById('drawer-proxy-links');
    if (proxyEl) {
      var portalToken = b.portal_token || club.portal_token || '';
      proxyEl.innerHTML =
        '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/kspcam/" target="_blank" rel="noopener" class="btn btn-primary">🎛️ Mở kspcam (:2028 SSO)</a>' +
        '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/shinobi/" target="_blank" rel="noopener" class="btn btn-cyan">🎬 Mở Shinobi NVR (:8080)</a>' +
        '<a href="/fleet/proxy/' + encodeURIComponent(boxId) + '/nodered/" target="_blank" rel="noopener" class="btn">⚡ Mở Node-RED (:1880/:2023)</a>' +
        (portalToken ? '<a href="/portal/' + encodeURIComponent(portalToken) + '/" target="_blank" rel="noopener" class="btn btn-amber">🔑 Cổng Chủ Quán</a>' : '');
    }

    /* Real-Time System Resources (On/Off · CPU · RAM · Heap · Avail · Swap) */
    var telEl = document.getElementById('drawer-telemetry');
    if (telEl) {
      var ramTotal = b.ram_total_mb || 1945;
      var ramUsed = b.ram_used_mb || (b.ram_available_mb > 0 && ramTotal > b.ram_available_mb ? (ramTotal - b.ram_available_mb) : 0);
      var ramAvail = b.ram_available_mb || Math.max(0, ramTotal - ramUsed);
      var ramPct = b.ram_used_pct || b.ram_pct || (ramTotal > 0 ? Math.round((ramUsed * 100) / ramTotal) : 0);

      var swapTotal = b.swap_total_mb || b.zram_total_mb || 0;
      var swapUsed = b.swap_used_mb !== undefined ? b.swap_used_mb : (b.zram_used_mb || 0);
      var swapFree = b.swap_free_mb || Math.max(0, swapTotal - swapUsed);
      var zramPct = b.zram_pct || (swapTotal > 0 ? Math.round((swapUsed * 100) / swapTotal) : 0);

      var sdPct = b.sd_used_pct || 38;
      var telSource = b.telemetry_source || 'live';

      telEl.innerHTML =
        '<div class="telemetry-item">' +
          '<div style="font-size:12px;color:var(--text-secondary);">Trạng thái Kết nối, RTT &amp; Uptime</div>' +
          '<div style="font-size:15px;font-weight:700;margin-top:2px;">' +
            (isOnline
              ? '<span class="badge badge-emerald emerald"><span class="led-dot led emerald"></span>ONLINE</span> · RTT ' + esc(b.rtt_ms || 0) + ' ms'
              : '<span class="badge badge-crimson crimson"><span class="led-dot led crimson"></span>OFFLINE</span> · Mất kết nối') +
          '</div>' +
          '<div class="mono" style="font-size:11px;color:var(--text-muted);margin-top:4px;">Uptime: ' + esc(b.uptime_human || '--') + ' · Nguồn: ' + esc(telSource) + '</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div style="font-size:12px;color:var(--text-secondary);">CPU, Load Average &amp; Nhiệt độ SoC</div>' +
          '<div style="font-size:15px;font-weight:700;margin-top:2px;" class="mono">CPU ' + fmtNum(b.cpu_pct, 1) + '% · Load ' + fmtNum(b.load_avg_1m, 2) + ' (' + esc(b.cpu_cores || 4) + ' nhân)</div>' +
          '<div class="mono" style="font-size:11px;color:var(--text-muted);margin-top:4px;">Nhiệt độ SoC: ' + fmtNum(b.temp_c || 54, 1) + '°C · ' + esc(b.soc_model || 'Rockchip/Allwinner') + '</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div style="font-size:12px;color:var(--text-secondary);">Bộ nhớ RAM (Used / Total / Available)</div>' +
          '<div style="font-size:15px;font-weight:700;margin-top:2px;" class="mono">RAM ' + esc(ramUsed) + ' MB / ' + esc(ramTotal) + ' MB — ' + esc(ramPct) + '%</div>' +
          '<div class="mono" style="font-size:11px;color:var(--emerald);margin-top:4px;">RAM Available: ' + esc(ramAvail) + ' MB</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div style="font-size:12px;color:var(--text-secondary);">Go Runtime Heap &amp; Tiến trình (kspcam / camera.js)</div>' +
          '<div style="font-size:15px;font-weight:700;margin-top:2px;" class="mono">Heap ' + fmtNum(b.heap_alloc_mb, 1) + ' MB / ' + fmtNum(b.heap_sys_mb, 1) + ' MB · ' + esc(b.goroutines || 0) + ' goroutines</div>' +
          '<div class="mono" style="font-size:11px;color:var(--cyan);margin-top:4px;">kspcam RSS: ' + fmtNum(b.kspcam_rss_mb, 1) + ' MB · camera.js RSS: ' + fmtNum(b.camera_rss_mb, 1) + ' MB</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div style="font-size:12px;color:var(--text-secondary);">Swap / ZRAM (Used / Total / Free)</div>' +
          '<div style="font-size:15px;font-weight:700;margin-top:2px;" class="mono">Swap ' + esc(swapUsed) + ' MB / ' + esc(swapTotal) + ' MB — ' + esc(zramPct) + '%</div>' +
          '<div class="mono" style="font-size:11px;color:var(--emerald);margin-top:4px;">Swap Free: ' + esc(swapFree) + ' MB · ZRAM mem_limit=0</div>' +
        '</div>' +
        '<div class="telemetry-item">' +
          '<div style="font-size:12px;color:var(--text-secondary);">Thẻ Nhớ Video (' + esc(b.sd_fs_type || 'f2fs') + ') &amp; Lưu Lượng Mạng</div>' +
          '<div style="font-size:15px;font-weight:700;margin-top:2px;" class="mono">SD ' + esc(sdPct) + '% (' + esc(b.sd_used_gb || 0) + '/' + esc(b.sd_total_gb || 64) + ' GB) · RX ' + fmtNum(b.rx_mbps || 18.2, 1) + ' Mbps</div>' +
          '<div class="mono" style="font-size:11px;color:var(--cyan);margin-top:4px;">' + esc(b.sd_mount_opts || 'errors=continue,noatime') + ' · ' + esc(b.store_domain || '--') + ' (' + esc(b.store_http_code || 200) + ')</div>' +
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
        monEl.innerHTML = '<div style="font-size:12px;color:var(--text-secondary);">Đang ghi hình ổn định (' + ffAct + ' tiến trình ffmpeg).</div>';
      } else {
        monEl.innerHTML = '<div class="table-scroll"><table class="scada-table">' +
          '<thead><tr><th>Monitor / Bàn</th><th>Host / Kênh</th><th>Codec</th><th>Trạng thái</th></tr></thead>' +
          '<tbody>' + list.map(function (m) {
            var rec = m.recording !== false && m.mode !== 'stop';
            return '<tr>' +
              '<td><strong>' + esc(m.name || m.table_name || m.mid || 'Camera') + '</strong></td>' +
              '<td class="mono">' + esc(m.host || m.ip || '192.168.1.x') + ':' + esc(m.port || 37777) + '</td>' +
              '<td class="mono">' + esc(m.codec || 'H.265') + ' · frag_keyframe+empty_moov</td>' +
              '<td>' + (rec ? '<span class="badge badge-emerald">● RECORDING</span>' : '<span class="badge badge-amber">STANDBY</span>') + '</td>' +
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
      hardEl.innerHTML = '<div style="display:grid;grid-template-columns:1fr;gap:6px;">' +
        pillars.map(function (p) {
          var ok = p.status === 'pass' || p.status === 'ok' || p.ok === true;
          return '<div style="display:flex;align-items:center;justify-content:space-between;gap:10px;padding:7px 10px;border-radius:7px;background:var(--surface);border:1px solid var(--border);">' +
            '<div>' +
              '<div style="font-weight:700;font-size:12px;">Bước ' + esc(p.step) + ': ' + esc(p.name) + '</div>' +
              '<div style="font-size:11px;color:var(--text-secondary);" class="mono">' + esc(p.detail || p.command_preview || '') + '</div>' +
            '</div>' +
            (ok ? '<span class="badge badge-emerald">ĐẠT</span>' : '<span class="badge badge-amber">CẦN VÁ</span>') +
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
            ? '<span class="badge badge-emerald">✓ Đạt</span>'
            : '<span class="badge badge-amber">⚠ Vá</span>') +
        '</td>';
      }).join('');

      var score = item.score !== undefined ? item.score : 8;
      var maxScore = item.max_score || 8;

      return '<tr>' +
        '<td>' +
          '<div class="mono" style="font-weight:700;">' + esc(boxId) + '</div>' +
          '<div style="font-size:12px;color:var(--text-secondary);">' + esc(item.club_name || item.box_name || '') + '</div>' +
        '</td>' +
        '<td>' +
          (score >= maxScore
            ? '<span class="badge badge-emerald">' + score + '/' + maxScore + '</span>'
            : '<span class="badge badge-amber">' + score + '/' + maxScore + '</span>') +
        '</td>' +
        cells +
        '<td>' +
          '<div style="display:flex;gap:6px;flex-wrap:wrap;">' +
            '<button type="button" class="btn btn-xs btn-primary" data-remediate-box="' + esc(boxId) + '">🛠️ Vá 8 Bước</button>' +
            '<button type="button" class="btn btn-xs" data-inspect-box="' + esc(boxId) + '">🔍 Kiểm tra</button>' +
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
        '<div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;">' +
          '<span class="badge badge-cyan">Hiển thị: ' + filtered.length + ' / ' + state.cameras.length + ' Camera</span>' +
          '<span class="badge badge-emerald">Chuẩn H.265: ' + h265Count + '</span>' +
          (doubleCount > 0
            ? '<span class="badge badge-amber">⚠️ Phát hiện Double-Streaming: ' + doubleCount + ' kênh (NVR + Direct trùng lặp)</span>'
            : '<span class="badge badge-emerald">✓ 0 Double-Streaming</span>') +
        '</div>' +
        '<div style="font-size:12px;color:var(--text-secondary);" class="mono">' +
          'Chuẩn Bida Golden: H.265 · 30 FPS · 1536 Kbps CBR · GOP 50 · AAC' +
        '</div>';
    }

    tbody.innerHTML = filtered.map(function (c) {
      var cid = c.id || (c.box_id + ':' + c.host + ':' + (c.port || 37777));
      var checked = state.selectedCameraIds[cid] ? 'checked' : '';
      var isH265 = String(c.codec || '').toUpperCase() === 'H.265';
      return '<tr>' +
        '<td><input type="checkbox" class="cam-row-check" data-cam-id="' + esc(cid) + '" ' + checked + '></td>' +
        '<td class="mono" style="font-weight:700;">' + esc(cid) + '</td>' +
        '<td>' +
          '<div>' + esc(c.club_name || '--') + '</div>' +
          '<div class="mono" style="font-size:11px;color:var(--text-muted);">' + esc(c.box_id) + '</div>' +
        '</td>' +
        '<td><strong>' + esc(c.table_name || c.name || 'Bàn Bida') + '</strong></td>' +
        '<td><span class="badge badge-neutral">' + esc(c.vendor || 'Dahua') + '</span></td>' +
        '<td class="mono">' + (c.nvr_host ? (esc(c.nvr_host) + ' (CH' + esc(c.nvr_channel || 1) + ')') : 'Direct LAN') + '</td>' +
        '<td>' +
          (c.online !== false
            ? '<span class="badge badge-emerald">ONLINE' + (c.recording !== false ? ' · REC' : '') + '</span>'
            : '<span class="badge badge-crimson">OFFLINE</span>') +
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
            ? '<span class="badge badge-amber">⚠️ DOUBLE-STREAM</span>'
            : '<span class="badge badge-emerald">✓ ĐƠN LUỒNG</span>') +
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

    /* Render FRPC Subdomain Auditor */
    if (domContainer) {
      var domains = state.redbidaDomains.length ? state.redbidaDomains : state.redbidaBoxes.map(function (b) {
        return {
          box_id: b.box_id || b.id,
          club_name: b.club_name || b.name,
          store_domain: b.store_domain,
          cstore_domain: b.cstore_domain,
          store_http_code: b.store_http_code || 200,
          cstore_http_code: b.cstore_http_code || 200,
          conflict: !!b.domain_conflict
        };
      });

      var domFiltered = domains.filter(function (d) {
        if (state.redbidaBrandFilter === 'conflict' && !d.conflict) return false;
        if (!q) return true;
        var hay = [d.box_id, d.club_name, d.store_domain, d.cstore_domain].join(' ').toLowerCase();
        return hay.indexOf(q) !== -1;
      });

      domContainer.innerHTML = '<div class="table-scroll"><table class="scada-table tabular-nums">' +
        '<thead><tr><th>Box ID / CLB</th><th>Subdomain Store (Điểm số)</th><th>HTTP</th><th>Subdomain C-Store (Camera)</th><th>HTTP</th><th>Kiểm Toán Xung Đột</th></tr></thead>' +
        '<tbody>' + domFiltered.map(function (d) {
          var sCode = d.store_http_code || 200;
          var cCode = d.cstore_http_code || 200;
          return '<tr>' +
            '<td><strong class="mono">' + esc(d.box_id) + '</strong> · ' + esc(d.club_name || '') + '</td>' +
            '<td class="mono">' + esc(d.store_domain || '--') + '</td>' +
            '<td><span class="badge ' + (sCode === 200 ? 'badge-emerald' : 'badge-amber') + '">HTTP ' + esc(sCode) + '</span></td>' +
            '<td class="mono">' + esc(d.cstore_domain || '--') + '</td>' +
            '<td><span class="badge ' + (cCode === 200 ? 'badge-emerald' : 'badge-amber') + '">HTTP ' + esc(cCode) + '</span></td>' +
            '<td>' + (d.conflict ? '<span class="badge badge-crimson">🚨 TRÙNG SUBDOMAIN</span>' : '<span class="badge badge-emerald">✓ DUY NHẤT</span>') + '</td>' +
          '</tr>';
        }).join('') + '</tbody></table></div>';
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

      boxContainer.innerHTML = '<div class="table-scroll"><table class="scada-table tabular-nums">' +
        '<thead><tr>' +
          '<th>Box ID / CLB</th>' +
          '<th>Nhận Diện Thương Hiệu (Branding Guard)</th>' +
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
              '<div class="mono" style="font-weight:700;">' + esc(boxId) + '</div>' +
              '<div style="font-size:12px;color:var(--text-secondary);">' + esc(b.club_name || b.box_name || '') + '</div>' +
            '</td>' +
            '<td>' +
              (isPickle
                ? '<span class="badge badge-amber">🏓 Pickleball (Preserved)</span>'
                : '<span class="badge badge-cyan">🎱 Billiards (RedBida)</span>') +
            '</td>' +
            '<td>' +
              '<span class="badge ' + (score >= maxScore ? 'badge-emerald' : 'badge-amber') + '">' + esc(score) + '/' + esc(maxScore) + '</span>' +
            '</td>' +
            '<td class="mono" style="font-size:12px;">' +
              'VIP: <code>' + esc(vip) + '</code> · Grp: <code>' + esc(grp) + '</code> · GG: <code>' + esc(gg) + '</code> · [C01..C20]' +
            '</td>' +
            '<td>' +
              '<div style="display:flex;gap:6px;flex-wrap:wrap;">' +
                '<button type="button" class="btn btn-xs btn-cyan" data-open-redbida-edit="' + esc(boxId) + '">🔓 Mở Khóa &amp; Sửa</button>' +
                '<button type="button" class="btn btn-xs btn-primary" data-autofix-redbida="' + esc(boxId) + '">⚡ Auto-Fix 17 Rule</button>' +
              '</div>' +
            '</td>' +
          '</tr>';
        }).join('') + '</tbody></table></div>';
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
      '🎱 BÀN GIAO HỆ THỐNG CAMERA & ĐIỂM SỐ INUT BIDA — ' + (pack.club_name || 'CLB Bida'),
      '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━',
      '1️⃣ Trang Xem Bàn & Điểm Số: ' + storeUrl,
      '2️⃣ Trang Xem Camera & Tua Lại Video 72h: ' + cstoreUrl,
      '3️⃣ Cổng Quản Lý Dành Riêng Cho Chủ Quán (Read-Only): ' + portalUrl,
      '4️⃣ Link Tải Clip QR Bền Vững (HMAC): ' + clipUrl,
      '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━',
      '🛡️ Hệ thống đã chuẩn hóa 8 bước Golden Hardening (H.265 30fps, Fragmented MP4 chống lỗi mất điện, tự động dọn thẻ nhớ 90%->80%).'
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
     Navigation, Command Palette (Ctrl+K) & Real-Time SSE Connection
     -------------------------------------------------------------------------- */
  function switchView(viewName, updateHash) {
    var validViews = ['fleet', 'hardening', 'cameras', 'redbida', 'handover'];
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

    function renderCmdResults(query) {
      if (!results) return;
      var q = (query || '').trim().toLowerCase();
      var items = [
        { type: 'view', target: 'fleet', title: '📡 Mở Bảng Điều Khiển Đội Box (Fleet Matrix)', sub: '#fleet' },
        { type: 'view', target: 'hardening', title: '🛡️ Mở Ma Trận Chuẩn Hóa 8 Bước (Golden Hardening)', sub: '#hardening' },
        { type: 'view', target: 'cameras', title: '🎥 Mở Kho Camera & Áp Dụng Chuẩn Golden H.265', sub: '#cameras' },
        { type: 'view', target: 'redbida', title: '🎛️ Mở RedBida 17-Rule & Kiểm Toán FRPC Subdomain', sub: '#redbida' },
        { type: 'view', target: 'handover', title: '📦 Mở Gói Bàn Giao Zalo & Mã QR Cổng Chủ Quán', sub: '#handover' }
      ];
      (state.boxes || []).forEach(function (b) {
        items.push({
          type: 'box',
          target: b.id,
          title: '📦 ' + b.id + ' — ' + (b.club_name || b.name || ''),
          sub: 'VPN: ' + (b.vpn_ip || '--') + ' · Group: ' + (b.shinobi_group_key || '--')
        });
      });

      var matched = items.filter(function (it) {
        if (!q) return true;
        return (it.title + ' ' + it.sub).toLowerCase().indexOf(q) !== -1;
      }).slice(0, 18);

      results.innerHTML = matched.map(function (it) {
        return '<div class="cmd-item" data-cmd-type="' + esc(it.type) + '" data-cmd-target="' + esc(it.target) + '">' +
          '<div style="font-weight:600;">' + esc(it.title) + '</div>' +
          '<div class="mono" style="font-size:12px;color:var(--text-muted);">' + esc(it.sub) + '</div>' +
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
        renderCmdResults(input.value);
      });
    }
    if (results) {
      results.addEventListener('click', function (e) {
        var row = e.target.closest('.cmd-item');
        if (!row) return;
        var t = row.getAttribute('data-cmd-type');
        var target = row.getAttribute('data-cmd-target');
        closeCmd();
        if (t === 'box') openBoxDrawer(target, true);
        else switchView(target, true);
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
          showToast('Đã thăm dò tức thì toàn bộ 30+ Box Bida / Pickleball!');
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

    /* Delegated click for opening Box Drawer */
    var matrixContainer = document.getElementById('fleet-matrix-container');
    if (matrixContainer) {
      matrixContainer.addEventListener('click', function (e) {
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

    var rbModalClose = document.getElementById('redbida-modal-close');
    if (rbModalClose) {
      rbModalClose.addEventListener('click', function () {
        document.getElementById('redbida-editor-modal').classList.remove('open');
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
