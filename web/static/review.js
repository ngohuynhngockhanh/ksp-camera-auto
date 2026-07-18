/* Xem lại video — timeline review view.
 * Uses vis-timeline for the segment timeline + three draggable custom-time bars
 * (red playhead, green cut-start, yellow cut-end). Reuses global helpers from
 * app.js: api(), escapeHtml(), showToast(), timeoutSec(). Dahua-only. */
(function () {
  let timeline = null;      // vis.Timeline
  let items = null;         // vis.DataSet
  let cam = null;           // {id, name, host, vendor}
  let inited = false;
  let winStart = null, winEnd = null; // current loaded window (Date)
  let draggingPlayhead = false;       // true while the red marker is being dragged
  let previewBase = null;             // absolute time (Date) of the loaded preview clip's start
  let maxHours = 72;                  // review-window cap (from /api/config)
  const PREVIEW_LEN = 120;            // seconds of video loaded per preview clip (scrub granularity)

  const $ = (id) => document.getElementById(id);
  const pad = (n) => String(n).padStart(2, '0');

  // "2026-07-18 20:00:00" (device-local) -> Date (local wall clock, no tz shift).
  function parseDev(s) { return new Date(s.replace(' ', 'T')); }
  // Date -> "2026-07-18T20:00:00" for the /api/playback params.
  function fmtParam(d) {
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  }
  function fmtClock(d) { return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`; }

  const QUICK = [
    { m: 5, t: '5 phút' }, { m: 10, t: '10 phút' }, { m: 20, t: '20 phút' }, { m: 40, t: '40 phút' },
    { m: 60, t: '1 giờ' }, { m: 120, t: '2 giờ' }, { m: 180, t: '3 giờ' }, { m: 240, t: '4 giờ' },
  ];

  // reviewOnShow is called by app.js setRoute() each time the view opens.
  window.reviewOnShow = function () {
    if (!inited) { init(); inited = true; return; }
    // Honor a preselected camera when opened from the maintenance panel button.
    if (window._rvPreselect && window._rvCams) {
      const c = window._rvCams.find(c => c.id === window._rvPreselect);
      window._rvPreselect = null;
      if (c && c.id !== (cam && cam.id)) { $('rv-cam').value = c.id; cam = c; load(60); }
    }
  };

  async function init() {
    try { const cfg = await api('/api/config'); if (cfg && cfg.maxReviewHours) maxHours = cfg.maxReviewHours; } catch (e) { /* keep default 72 */ }
    // Populate camera dropdown (Dahua only — playback is Dahua-only).
    const sel = $('rv-cam');
    try {
      const cams = (await api('/api/cameras') || []).filter(c => c.vendor === 'dahua');
      if (!cams.length) { sel.innerHTML = '<option>Không có camera Dahua</option>'; return; }
      sel.innerHTML = cams.map(c => `<option value="${escapeHtml(c.id)}">${escapeHtml(c.name || c.host)}</option>`).join('');
      window._rvCams = cams;
      sel.addEventListener('change', () => { cam = cams.find(c => c.id === sel.value); load(60); });
      cam = (window._rvPreselect && cams.find(c => c.id === window._rvPreselect)) || cams[0];
      window._rvPreselect = null;
      sel.value = cam.id;
    } catch (e) { sel.innerHTML = '<option>Lỗi tải camera</option>'; return; }

    // Quick-time buttons.
    $('rv-quick').innerHTML = QUICK.map(q => `<button class="btn btn-sm btn-secondary" type="button" data-min="${q.m}">${q.t} trước</button>`).join('');
    $('rv-quick').querySelectorAll('[data-min]').forEach(b => b.addEventListener('click', () => load(parseInt(b.dataset.min, 10))));
    $('rv-reload').addEventListener('click', () => load(currentWindowMinutes()));

    buildTimeline();
    wireControls();
    load(60); // default: last hour
  }

  function currentWindowMinutes() {
    if (winStart && winEnd) return Math.max(1, Math.round((winEnd - winStart) / 60000));
    return 60;
  }

  function buildTimeline() {
    items = new vis.DataSet([]);
    timeline = new vis.Timeline($('rv-timeline'), items, {
      stack: false, showCurrentTime: false, selectable: false,
      zoomMin: 1000 * 10, zoomMax: 1000 * 60 * 60 * maxHours, height: 90,
      moveable: true, zoomable: true,
      margin: { item: 2 },
    });
    // Draggable markers: cut-start (green), cut-end (yellow), playhead (red).
    const now = new Date();
    timeline.addCustomTime(new Date(now.getTime() - 55 * 60000), 'cutStart');
    timeline.addCustomTime(new Date(now.getTime() - 50 * 60000), 'cutEnd');
    timeline.addCustomTime(now, 'playhead');
    timeline.setCustomTimeMarker('▶', 'playhead', false);
    // While dragging the red marker, suppress the timeupdate auto-follow so it
    // doesn't snap back; on release, load a short preview clip at the new spot.
    timeline.on('timechange', (p) => { if (p.id === 'playhead') draggingPlayhead = true; clampMarkers(p.id); updateRange(); });
    timeline.on('timechanged', (p) => {
      clampMarkers(p.id); updateRange();
      if (p.id === 'playhead') { draggingPlayhead = false; loadPreview(markerTime('playhead')); }
    });
    // Click on the timeline background = move the playhead there and preview.
    timeline.on('click', (props) => {
      if (!props.time || props.what === 'custom-time') return;
      timeline.setCustomTime(props.time, 'playhead');
      loadPreview(props.time);
    });
  }

  function markerTime(id) { return timeline.getCustomTime(id); }

  // clampMarkers keeps cutStart <= cutEnd: if the just-moved marker crossed the
  // other, push the other along so the cut range never inverts.
  function clampMarkers(moved) {
    if (moved !== 'cutStart' && moved !== 'cutEnd') return;
    const a = markerTime('cutStart'), b = markerTime('cutEnd');
    if (a > b) {
      if (moved === 'cutStart') timeline.setCustomTime(a, 'cutEnd');
      else timeline.setCustomTime(b, 'cutStart');
    }
  }

  function updateRange() {
    let a = markerTime('cutStart'), b = markerTime('cutEnd');
    if (a > b) { const t = a; a = b; b = t; }
    const secs = Math.round((b - a) / 1000);
    $('rv-range').textContent = `${fmtClock(a)} → ${fmtClock(b)} (${secs}s${secs > 3600 ? ' ~' + (secs / 3600).toFixed(1) + 'h' : ''})`;
    return { start: a, end: b };
  }

  // loadPreview streams a short clip [at, at+PREVIEW_LEN] into the player so the
  // native seek bar / ±seconds work smoothly (a full multi-hour range can't be
  // scrubbed). The red playhead marks this clip's start.
  function loadPreview(at) {
    if (!cam) return;
    const start = at instanceof Date ? at : new Date(at);
    const end = new Date(start.getTime() + PREVIEW_LEN * 1000);
    const ch = parseInt($('rv-channel').value, 10) || 0;
    const v = $('rv-video');
    previewBase = start;
    v.src = `/api/playback?id=${encodeURIComponent(cam.id)}&channel=${ch}&start=${encodeURIComponent(fmtParam(start))}&end=${encodeURIComponent(fmtParam(end))}`;
    v.playbackRate = parseFloat($('rv-speed').value) || 1;
    v.play().catch(() => {});
  }

  async function load(minutes) {
    if (!cam) return;
    const end = new Date();
    const start = new Date(end.getTime() - minutes * 60000);
    winStart = start; winEnd = end;
    timeline.setWindow(start, end, { animation: false });
    // place cut markers inside the window, playhead at start
    timeline.setCustomTime(new Date(start.getTime() + (end - start) * 0.1), 'cutStart');
    timeline.setCustomTime(new Date(start.getTime() + (end - start) * 0.2), 'cutEnd');
    timeline.setCustomTime(start, 'playhead');
    updateRange();
    $('rv-msg').textContent = 'Đang tải danh sách bản ghi…'; $('rv-msg').className = 'msg';
    try {
      const ch = parseInt($('rv-channel').value, 10) || 0;
      const q = `id=${encodeURIComponent(cam.id)}&channel=${ch}&start=${encodeURIComponent(fmtParam(start))}&end=${encodeURIComponent(fmtParam(end))}&timeoutSeconds=${timeoutSec()}`;
      const res = await api('/api/recordings?' + q);
      const recs = (res && res.recordings) || [];
      items.clear();
      items.add(recs.map((r, i) => ({
        id: i, start: parseDev(r.startTime), end: parseDev(r.endTime),
        type: 'range', className: (r.events && r.events.length) ? 'rv-ev' : 'rv-rec',
        title: `${r.startTime} → ${r.endTime} (${r.duration}s)${(r.events && r.events.length) ? ' · ' + r.events.join(',') : ''}`,
      })));
      $('rv-msg').textContent = recs.length ? `${recs.length} đoạn ghi.` : 'Không có bản ghi trong khoảng này.';
      $('rv-msg').className = recs.length ? 'msg ok' : 'msg';
    } catch (e) { $('rv-msg').textContent = 'Lỗi: ' + e.message; $('rv-msg').className = 'msg err'; }
  }

  function cutParams() {
    const { start, end } = updateRange();
    const ch = parseInt($('rv-channel').value, 10) || 0;
    return { id: cam.id, channel: ch, start: fmtParam(start), end: fmtParam(end), startDate: start, endDate: end };
  }

  function playbackURL(p, extra) {
    let u = `/api/playback?id=${encodeURIComponent(p.id)}&channel=${p.channel}&start=${encodeURIComponent(p.start)}&end=${encodeURIComponent(p.end)}`;
    return u + (extra || '');
  }

  function wireControls() {
    const v = $('rv-video');
    // ▶ Phát previews from the red playhead's current position (native controls
    // give play/pause + seek bar within the loaded short clip).
    $('rv-play').addEventListener('click', () => loadPreview(markerTime('playhead')));
    document.querySelectorAll('#view-review [data-seek]').forEach(b =>
      b.addEventListener('click', () => { v.currentTime = Math.max(0, v.currentTime + parseFloat(b.dataset.seek)); }));
    $('rv-speed').addEventListener('change', () => { v.playbackRate = parseFloat($('rv-speed').value); });
    // The red playhead follows playback — but never while the user is dragging it.
    v.addEventListener('timeupdate', () => {
      if (draggingPlayhead || !previewBase) return;
      timeline.setCustomTime(new Date(previewBase.getTime() + v.currentTime * 1000), 'playhead');
    });
    // Auto-next: when a preview clip ends, continue with the next clip.
    v.addEventListener('ended', () => {
      if (!$('rv-auto').checked || !previewBase) return;
      loadPreview(new Date(previewBase.getTime() + PREVIEW_LEN * 1000));
    });
    $('rv-download').addEventListener('click', () => download(true));
    $('rv-download-mp4').addEventListener('click', () => download(false));
    $('rv-qr').addEventListener('click', showQR);
    $('rv-qr-close').addEventListener('click', () => { $('rv-qr-modal').hidden = true; });
  }

  function download(fast) {
    if (!cam) return;
    const p = cutParams();
    if (p.endDate <= p.startDate) { showToast('Chọn đoạn cắt hợp lệ.', 'err'); return; }
    window.location.href = playbackURL(p, (fast ? '&fast=1' : '') + '&download=1');
    showToast('Đang tải… (đoạn dài có thể mất chút thời gian)', 'ok');
  }

  async function showQR() {
    if (!cam) return;
    const p = cutParams();
    if (p.endDate <= p.startDate) { showToast('Chọn đoạn cắt hợp lệ.', 'err'); return; }
    try {
      const q = `id=${encodeURIComponent(p.id)}&channel=${p.channel}&start=${encodeURIComponent(p.start)}&end=${encodeURIComponent(p.end)}&fast=1&download=1`;
      const tok = await api('/api/playback-token?' + q);
      const url = `${location.origin}${playbackURL(p, '&fast=1&download=1')}&exp=${tok.exp}&token=${tok.token}`;
      const box = $('rv-qr-canvas'); box.innerHTML = '';
      $('rv-qr-modal').hidden = false; // show first so QRCode can size the element
      try {
        // correctLevel L = max data capacity (the tokenized URL is long); H overflows.
        new QRCode(box, { text: url, width: 240, height: 240, correctLevel: QRCode.CorrectLevel.L });
      } catch (err) {
        box.innerHTML = '<p style="color:#c0392b">Link quá dài cho QR.</p>';
      }
      // Always show a tappable link fallback (works even if the QR fails to render).
      const link = document.getElementById('rv-qr-link');
      if (link) { link.href = url; link.textContent = 'Hoặc bấm vào đây để tải trên máy này'; }
    } catch (e) { showToast('Lỗi tạo QR: ' + e.message, 'err'); }
  }
})();
