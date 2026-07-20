/* Xem lại video — timeline review view.
 * Uses vis-timeline for the segment timeline + three draggable custom-time bars
 * (red playhead, green cut-start, yellow cut-end). Reuses global helpers from
 * app.js: api(), escapeHtml(), showToast(), timeoutSec(). Dahua + Hikvision
 * (any vendor whose /api/recordings + /api/playback support recordings). */
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
    { m: 20, t: '20 phút' }, { m: 60, t: '1 giờ' }, { m: 180, t: '3 giờ' }, { m: 360, t: '6 giờ' },
    { m: 720, t: '12 giờ' }, { m: 1440, t: '24 giờ' }, { m: 2880, t: '48 giờ' },
  ];
  // datetime-local wants "YYYY-MM-DDTHH:MM"
  function fmtLocal(d) { return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`; }
  function fmtLocalSec(d) { return `${fmtLocal(d)}:${pad(d.getSeconds())}`; }
  let editingCut = false; // true while the user is typing in a cut time input

  // reviewOnShow is called by app.js setRoute() each time the view opens.
  window.reviewOnShow = function () {
    if (!inited) { init(); inited = true; return; }
    // Honor a preselected camera when opened from the maintenance panel button.
    if (window._rvPreselect && window._rvCams) {
      const c = window._rvCams.find(c => c.id === window._rvPreselect);
      window._rvPreselect = null;
      if (c && c.id !== (cam && cam.id)) { $('rv-cam').value = c.id; cam = c; updateDownloadLabel(); load(60); }
    }
  };

  async function init() {
    try { const cfg = await api('/api/config'); if (cfg && cfg.maxReviewHours) maxHours = cfg.maxReviewHours; } catch (e) { /* keep default 72 */ }
    // Populate camera dropdown (playback works for Dahua and Hikvision —
    // any vendor camera.Open()'s Camera implements camera.Recorder for).
    const sel = $('rv-cam');
    try {
      const cams = (await api('/api/cameras') || []).filter(c => c.vendor === 'dahua' || c.vendor === 'hikvision' || c.vendor === 'tiandy');
      if (!cams.length) { sel.innerHTML = '<option>Không có camera hỗ trợ xem lại</option>'; return; }
      sel.innerHTML = cams.map(c => {
        const chan = c.channelName || c.nvrName || '';
        const label = (c.name || c.host) + (chan ? ' - ' + chan : '');
        return `<option value="${escapeHtml(c.id)}">${escapeHtml(label)}</option>`;
      }).join('');
      window._rvCams = cams;
      sel.addEventListener('change', () => { cam = cams.find(c => c.id === sel.value); updateDownloadLabel(); load(60); refreshDays(); });
      cam = (window._rvPreselect && cams.find(c => c.id === window._rvPreselect)) || cams[0];
      window._rvPreselect = null;
      sel.value = cam.id;
      updateDownloadLabel();
    } catch (e) { sel.innerHTML = '<option>Lỗi tải camera</option>'; return; }

    // Quick-time buttons.
    $('rv-quick').innerHTML = QUICK.map(q => `<button class="btn btn-sm btn-secondary" type="button" data-min="${q.m}">${q.t} trước</button>`).join('');
    $('rv-quick').querySelectorAll('[data-min]').forEach(b => b.addEventListener('click', () => load(parseInt(b.dataset.min, 10))));
    $('rv-reload').addEventListener('click', () => { load(currentWindowMinutes()); refreshDays(); });
    $('rv-load-range').addEventListener('click', loadFormRange);
    $('rv-load-day').addEventListener('click', () => loadDay($('rv-date').value));

    buildTimeline();
    wireControls();
    load(60); // default: last hour
    refreshDays();
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
    // Keep the cut time pickers in sync with the markers (unless the user is
    // currently typing in them).
    if (!editingCut) { $('rv-cut-from').value = fmtLocalSec(a); $('rv-cut-to').value = fmtLocalSec(b); }
    return { start: a, end: b };
  }

  // applyCutInputs moves the cut markers to the typed datetime-local values.
  function applyCutInputs() {
    const s = $('rv-cut-from').value, e = $('rv-cut-to').value;
    if (!s || !e) return;
    let a = new Date(s), b = new Date(e);
    if (isNaN(a) || isNaN(b)) return;
    if (a > b) { const t = a; a = b; b = t; }
    timeline.setCustomTime(a, 'cutStart');
    timeline.setCustomTime(b, 'cutEnd');
    // Widen the timeline window if the typed cut falls outside it.
    if (winStart && winEnd && (a < winStart || b > winEnd)) {
      const pad = (b - a) * 0.2 || 60000;
      timeline.setWindow(new Date(a.getTime() - pad), new Date(b.getTime() + pad), { animation: false });
    }
    updateRange();
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

  function load(minutes) {
    const end = new Date();
    loadRange(new Date(end.getTime() - minutes * 60000), end);
  }

  // ---- "Chia 5 phút": slice the currently loaded window [winStart, winEnd]
  // into consecutive 5-minute segments for quick click-to-preview browsing.
  // Purely client-side; each click just reuses playbackURL() like loadPreview
  // does, with the segment's own [start,end] instead of the fixed 120s clip. ----
  const SPLIT5_MIN = 5;
  const SPLIT5_CAP = 288; // 288 * 5min = 24h — keep the rendered list sane
  let split5Segments = [];

  function fmtHM(d) { return `${pad(d.getHours())}:${pad(d.getMinutes())}`; }

  function buildSplit5() {
    if (!cam || !winStart || !winEnd) { showToast('Chưa tải khoảng thời gian nào.', 'err'); return; }
    const segMs = SPLIT5_MIN * 60000;
    const count = Math.ceil((winEnd - winStart) / segMs);
    if (count <= 0) { showToast('Khoảng thời gian không hợp lệ.', 'err'); return; }
    if (count > SPLIT5_CAP) {
      showToast(`Khoảng đang xem quá lớn để chia 5 phút (${count} đoạn). Hãy chọn khoảng tối đa ${SPLIT5_CAP * SPLIT5_MIN / 60} giờ.`, 'err');
      return;
    }
    split5Segments = [];
    for (let i = 0; i < count; i++) {
      const s = new Date(winStart.getTime() + i * segMs);
      const e = new Date(Math.min(s.getTime() + segMs, winEnd.getTime()));
      split5Segments.push({ start: s, end: e });
    }
    const list = $('rv-split5-list');
    list.hidden = false;
    list.innerHTML = split5Segments.map((seg, i) =>
      `<button class="btn btn-sm btn-secondary rv-split5-item" type="button" data-idx="${i}">${fmtHM(seg.start)}–${fmtHM(seg.end)}</button>`).join('');
    list.querySelectorAll('.rv-split5-item').forEach(b =>
      b.addEventListener('click', () => loadSplit5(parseInt(b.dataset.idx, 10))));
    showToast(`Đã chia thành ${count} đoạn 5 phút.`, 'ok');
  }

  // loadSplit5 previews segment i in the existing <video> element, exactly
  // like loadPreview() does for the rolling 120s clip, and moves the red
  // playhead marker to the segment's start.
  function loadSplit5(i) {
    const seg = split5Segments[i];
    if (!seg || !cam) return;
    document.querySelectorAll('#rv-split5-list .rv-split5-item').forEach((b, idx) => {
      b.classList.toggle('btn-primary', idx === i);
      b.classList.toggle('btn-secondary', idx !== i);
    });
    const ch = parseInt($('rv-channel').value, 10) || 0;
    const v = $('rv-video');
    previewBase = seg.start;
    v.src = playbackURL({ id: cam.id, channel: ch, start: fmtParam(seg.start), end: fmtParam(seg.end) });
    v.playbackRate = parseFloat($('rv-speed').value) || 1;
    v.play().catch(() => {});
    timeline.setCustomTime(seg.start, 'playhead');
  }

  // loadDay loads a whole calendar day (capped at "now"). dateStr = "YYYY-MM-DD".
  function loadDay(dateStr) {
    if (!dateStr) return;
    const start = new Date(dateStr + 'T00:00:00');
    let end = new Date(start.getTime() + 24 * 3600 * 1000);
    if (end > new Date()) end = new Date();
    loadRange(start, end);
  }

  // loadFormRange reads the two datetime-local inputs and loads that exact range.
  function loadFormRange() {
    const s = $('rv-from').value, e = $('rv-to').value;
    if (!s || !e) { showToast('Chọn giờ bắt đầu và kết thúc.', 'err'); return; }
    const start = new Date(s), end = new Date(e);
    if (!(end > start)) { showToast('Kết thúc phải sau bắt đầu.', 'err'); return; }
    if (end - start > maxHours * 3600 * 1000) { showToast(`Khoảng tối đa ${maxHours} giờ.`, 'err'); return; }
    loadRange(start, end);
  }

  // refreshDays lists which calendar days have footage in the last maxHours as
  // clickable chips (a quick "which days have recordings" picker).
  async function refreshDays() {
    if (!cam) return;
    const el = $('rv-days'); if (!el) return;
    const end = new Date();
    const start = new Date(end.getTime() - maxHours * 3600 * 1000);
    const ch = parseInt($('rv-channel').value, 10) || 0;
    try {
      const q = `id=${encodeURIComponent(cam.id)}&channel=${ch}&start=${encodeURIComponent(fmtParam(start))}&end=${encodeURIComponent(fmtParam(end))}&timeoutSeconds=${timeoutSec()}`;
      const res = await api('/api/recordings?' + q);
      const days = [...new Set(((res && res.recordings) || []).map(r => r.startTime.slice(0, 10)))].sort().reverse();
      el.innerHTML = days.length
        ? 'Ngày có bản ghi: ' + days.map(d => `<button class="btn btn-sm btn-secondary" type="button" data-day="${d}">${d.slice(8, 10)}/${d.slice(5, 7)}</button>`).join(' ')
        : `<span class="muted">Không thấy bản ghi trong ${maxHours}h gần đây.</span>`;
      el.querySelectorAll('[data-day]').forEach(b => b.addEventListener('click', () => loadDay(b.dataset.day)));
    } catch (e) { el.innerHTML = ''; }
  }

  async function loadRange(start, end) {
    if (!cam) return;
    winStart = start; winEnd = end;
    // Stale segments would point outside the newly loaded window — clear them.
    split5Segments = [];
    const split5List = $('rv-split5-list');
    if (split5List) { split5List.hidden = true; split5List.innerHTML = ''; }
    // keep the form inputs in sync with what's shown
    $('rv-from').value = fmtLocal(start); $('rv-to').value = fmtLocal(end);
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

  // updateDownloadLabel relabels the "native/fast" download button per the
  // selected camera's vendor: Dahua's is a byte-exact .dav (DHAV), which VLC
  // and most players on any OS open fine. Hikvision's is its own proprietary
  // IMKH container (format=dav maps to hik.StreamNative server-side — see
  // internal/server/api.go handlePlayback) — same query param, different
  // on-device format, but IMKH is NOT a real MP4: it only opens in VLC/a
  // desktop player, never on iPhone or in a browser. The label has to say so
  // plainly or people click it expecting a normal video file.
  function updateDownloadLabel() {
    const btn = $('rv-download-dav');
    if (!btn || !cam) return;
    // Tiandy has no pure-Go native-container download (StreamDav is unsupported),
    // so hide the native button entirely — only the MP4 download applies.
    if (cam.vendor === 'tiandy') { btn.hidden = true; return; }
    btn.hidden = false;
    btn.textContent = cam.vendor === 'hikvision' ? 'Tải gốc IMKH (chỉ VLC, không phát trên ĐT/trình duyệt)' : 'Tải .dav (gốc)';
  }

  function wireControls() {
    const v = $('rv-video');
    // HEVC-unsupported hint: the NVR records H.265 on every channel, and
    // Chrome/Firefox on desktop simply can't decode it (this box is too slow
    // to transcode, so there's no fallback stream to offer). When the
    // <video> fires a MEDIA_ERR_SRC_NOT_SUPPORTED, tell the user instead of
    // leaving a silently broken player. Cleared on the next successful load.
    v.addEventListener('error', () => {
      if (v.error && v.error.code === 4) { const h = $('rv-hevc-hint'); if (h) h.hidden = false; }
    });
    ['loadeddata', 'playing'].forEach(ev => v.addEventListener(ev, () => {
      const h = $('rv-hevc-hint'); if (h) h.hidden = true;
    }));
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
    // Cut time pickers: type an exact start/end for the download range.
    ['rv-cut-from', 'rv-cut-to'].forEach(id => {
      const el = $(id);
      el.addEventListener('focus', () => { editingCut = true; });
      el.addEventListener('blur', () => { editingCut = false; });
      el.addEventListener('change', () => { editingCut = false; applyCutInputs(); });
    });
    // Fast MP4: parallel RTSP chunks — ~5× faster on Hik/Tiandy (Dahua aliases
    // to its already-fast playback), same exact-cut browser-playable MP4.
    $('rv-download').addEventListener('click', () => download('&format=fastmp4'));
    $('rv-download-dav').addEventListener('click', () => download('&format=dav'));
    $('rv-split5-btn').addEventListener('click', buildSplit5);
    $('rv-qr').addEventListener('click', showQR);
    $('rv-qr-close').addEventListener('click', () => { $('rv-qr-modal').hidden = true; });
  }

  function download(extra) {
    if (!cam) return;
    const p = cutParams();
    if (p.endDate <= p.startDate) { showToast('Chọn đoạn cắt hợp lệ.', 'err'); return; }
    window.location.href = playbackURL(p, (extra || '') + '&download=1');
    const isDav = (extra || '').includes('format=dav');
    let msg = 'Đang tải… (đoạn dài có thể mất chút thời gian)';
    if (isDav) {
      msg = cam.vendor === 'hikvision'
        ? 'Đang tải bản gốc IMKH (nhanh, chất lượng đầy đủ, không cắt chính xác theo giây)… Tệp này CHỈ mở được bằng VLC trên máy tính — KHÔNG phát được trên iPhone hay trong trình duyệt.'
        : 'Đang tải .dav gốc… (cần cổng cấu hình)';
    }
    showToast(msg, 'ok');
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
