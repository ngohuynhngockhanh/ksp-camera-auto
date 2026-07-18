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
      zoomMin: 1000 * 10, zoomMax: 1000 * 60 * 60 * 24, height: 90,
      moveable: true, zoomable: true,
      margin: { item: 2 },
    });
    // Draggable markers: cut-start (green), cut-end (yellow), playhead (red).
    const now = new Date();
    timeline.addCustomTime(new Date(now.getTime() - 55 * 60000), 'cutStart');
    timeline.addCustomTime(new Date(now.getTime() - 50 * 60000), 'cutEnd');
    timeline.addCustomTime(now, 'playhead');
    timeline.setCustomTimeMarker('▶', 'playhead', false);
    timeline.on('timechange', () => updateRange());
    timeline.on('timechanged', () => updateRange());
    // Click on the timeline background moves the playhead there.
    timeline.on('click', (props) => { if (props.time) timeline.setCustomTime(props.time, 'playhead'); });
  }

  function markerTime(id) { return timeline.getCustomTime(id); }

  function updateRange() {
    let a = markerTime('cutStart'), b = markerTime('cutEnd');
    if (a > b) { const t = a; a = b; b = t; }
    const secs = Math.round((b - a) / 1000);
    $('rv-range').textContent = `${fmtClock(a)} → ${fmtClock(b)} (${secs}s${secs > 3600 ? ' ~' + (secs / 3600).toFixed(1) + 'h' : ''})`;
    return { start: a, end: b };
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
    $('rv-play').addEventListener('click', () => {
      if (!cam) return;
      const p = cutParams();
      if (p.endDate <= p.startDate) { showToast('Chọn đoạn cắt hợp lệ.', 'err'); return; }
      v.src = playbackURL(p); v.dataset.base = p.startDate.getTime();
      v.play().catch(() => {});
    });
    document.querySelectorAll('#view-review [data-seek]').forEach(b =>
      b.addEventListener('click', () => { v.currentTime = Math.max(0, v.currentTime + parseFloat(b.dataset.seek)); }));
    $('rv-speed').addEventListener('change', () => { v.playbackRate = parseFloat($('rv-speed').value); });
    // Playhead follows video during playback.
    v.addEventListener('timeupdate', () => {
      if (!v.dataset.base) return;
      timeline.setCustomTime(new Date(parseInt(v.dataset.base, 10) + v.currentTime * 1000), 'playhead');
    });
    // Auto-next: on end, jump the cut to the next segment after current end.
    v.addEventListener('ended', () => {
      if (!$('rv-auto').checked) return;
      const curEnd = markerTime('cutEnd').getTime();
      let next = null;
      items.forEach(it => { const s = new Date(it.start).getTime(); if (s >= curEnd - 1000 && (!next || s < next.start)) next = { start: s, end: new Date(it.end).getTime() }; });
      if (next) {
        timeline.setCustomTime(new Date(next.start), 'cutStart');
        timeline.setCustomTime(new Date(next.end), 'cutEnd');
        updateRange(); $('rv-play').click();
      }
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
      new QRCode(box, { text: url, width: 220, height: 220 });
      $('rv-qr-modal').hidden = false;
    } catch (e) { showToast('Lỗi tạo QR: ' + e.message, 'err'); }
  }
})();
