// scheduler.js — Frontend controller for camera task scheduler
(function () {
  'use strict';

  let cachedJobs = [];
  let cachedHistory = [];
  let schedulerEventsBound = false;

  function formatTime(isoStr) {
    if (!isoStr || isoStr.startsWith('0001')) return '—';
    try {
      const d = new Date(isoStr);
      if (isNaN(d.getTime())) return '—';
      const pad = n => String(n).padStart(2, '0');
      return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())} ${pad(d.getDate())}/${pad(d.getMonth() + 1)}`;
    } catch (e) {
      return '—';
    }
  }

  function formatDays(days) {
    if (!days || days.length === 0 || days.length === 7) return 'Hàng ngày';
    const dayNames = { 0: 'CN', 1: 'T2', 2: 'T3', 3: 'T4', 4: 'T5', 5: 'T6', 6: 'T7' };
    const sorted = [...days].sort((a, b) => a - b);
    return sorted.map(d => dayNames[d] || d).join(', ');
  }

  function dayNightColorLabel(code) {
    switch (code) {
      case 0:
        return '0 - Full Color (Luôn màu)';
      case 1:
        return '1 - Auto (Tự động)';
      case 2:
        return '2 - B&W (Đen trắng)';
      default:
        return `Mã ${code}`;
    }
  }

  function resolveCameraDisplay(id) {
    const camList = window.cameras || [];
    const found = camList.find(c => c.id === id || c.host === id);
    if (found) {
      return `${found.name || found.id} (${found.host})`;
    }
    return id;
  }

  async function ensureCamerasLoaded() {
    if (!window.cameras || window.cameras.length === 0) {
      try {
        window.cameras = await api('/api/cameras');
      } catch (e) {
        console.warn('scheduler: failed to load cameras inventory', e);
      }
    }
    return window.cameras || [];
  }

  function renderCameraCheckboxes(selectedIds = []) {
    const container = document.getElementById('sched-form-cameras');
    if (!container) return;
    const camList = window.cameras || [];
    if (camList.length === 0) {
      container.innerHTML = '<span class="muted" style="padding:8px;">Không có camera nào trong kho.</span>';
      return;
    }

    const selectedSet = new Set(selectedIds || []);
    container.innerHTML = camList.map(cam => {
      const isChecked = selectedSet.has(cam.id) || selectedSet.has(cam.host);
      const isDahua = (cam.vendor || '').toLowerCase() === 'dahua';
      const vendorTag = cam.vendor ? `<span style="font-size:0.72rem; color:var(--muted); margin-left:4px;">[${escapeHtml(cam.vendor)}]</span>` : '';
      const compatTag = !isDahua ? `<span style="font-size:0.70rem; color:var(--warning, #eab308); margin-left:4px;">(khuyên dùng Dahua)</span>` : '';
      return `
        <label class="sched-cam-item" data-vendor="${escapeHtml(cam.vendor || '')}" data-id="${escapeHtml(cam.id)}">
          <input type="checkbox" value="${escapeHtml(cam.id)}" class="sched-cam-cb" ${isChecked ? 'checked' : ''}>
          <span>
            <strong>${escapeHtml(cam.name || cam.id)}</strong>
            <span style="font-size:0.78rem; color:var(--muted);">${escapeHtml(cam.host)}</span>
            ${vendorTag}${compatTag}
          </span>
        </label>
      `;
    }).join('');
  }

  function openSchedulerModal(job) {
    const modal = document.getElementById('scheduler-job-modal');
    if (!modal) return;

    const titleEl = document.getElementById('scheduler-modal-title');
    const idInput = document.getElementById('sched-form-id');
    const nameInput = document.getElementById('sched-form-name');
    const timeInput = document.getElementById('sched-form-time');
    const actionSelect = document.getElementById('sched-form-action-type');
    const colorSelect = document.getElementById('sched-form-daynight-color');
    const lightSelect = document.getElementById('sched-form-daynight-light');
    const chanInput = document.getElementById('sched-form-channel');
    const msgEl = document.getElementById('sched-form-msg');
    const dayCheckboxes = document.querySelectorAll('#sched-form-days input[type="checkbox"]');

    if (msgEl) {
      msgEl.className = 'msg';
      msgEl.textContent = '';
    }

    if (!job) {
      // Create Mode
      if (titleEl) titleEl.textContent = 'Thêm lịch trình mới';
      if (idInput) idInput.value = '';
      if (nameInput) nameInput.value = '';
      if (timeInput) timeInput.value = '07:00';
      if (actionSelect) actionSelect.value = 'picture_day_night';
      if (colorSelect) colorSelect.value = '0';
      if (lightSelect) lightSelect.value = 'auto';
      if (chanInput) chanInput.value = '0';

      dayCheckboxes.forEach(cb => { cb.checked = true; });
      renderCameraCheckboxes([]);
    } else {
      // Edit Mode
      if (titleEl) titleEl.textContent = `Chỉnh sửa: ${job.name || job.id}`;
      if (idInput) idInput.value = job.id || '';
      if (nameInput) nameInput.value = job.name || '';
      if (timeInput) timeInput.value = (job.schedule && job.schedule.time) ? job.schedule.time : '07:00';
      if (actionSelect) actionSelect.value = job.actionType || 'picture_day_night';

      if (job.actionType === 'picture_day_night' && job.action) {
        if (colorSelect) colorSelect.value = String(job.action.dayNightColor != null ? job.action.dayNightColor : 0);
        if (lightSelect) lightSelect.value = (job.action.supplementLight || 'auto');
        if (chanInput) chanInput.value = String(job.action.channel != null ? job.action.channel : 0);
      } else {
        if (colorSelect) colorSelect.value = '0';
        if (lightSelect) lightSelect.value = 'auto';
        if (chanInput) chanInput.value = '0';
      }

      if (enabledInput) enabledInput.checked = !!job.enabled;

      const jobDays = job.schedule && Array.isArray(job.schedule.daysOfWeek) ? job.schedule.daysOfWeek : [];
      if (jobDays.length === 0) {
        dayCheckboxes.forEach(cb => { cb.checked = true; });
      } else {
        const daySet = new Set(jobDays);
        dayCheckboxes.forEach(cb => {
          cb.checked = daySet.has(parseInt(cb.value, 10));
        });
      }

      const selectedIds = (job.action && Array.isArray(job.action.deviceIds)) ? job.action.deviceIds : [];
      renderCameraCheckboxes(selectedIds);
    }

    updateActionTypeUI();
    openDialog(modal);
  }

  function updateActionTypeUI() {
    const actionSelect = document.getElementById('sched-form-action-type');
    const dayNightBox = document.getElementById('sched-daynight-fields');
    const rebootBox = document.getElementById('sched-reboot-fields');
    if (!actionSelect) return;

    if (actionSelect.value === 'picture_day_night') {
      if (dayNightBox) dayNightBox.style.display = 'block';
      if (rebootBox) rebootBox.style.display = 'none';
    } else {
      if (dayNightBox) dayNightBox.style.display = 'none';
      if (rebootBox) rebootBox.style.display = 'block';
    }
  }

  async function handleJobFormSubmit(ev) {
    ev.preventDefault();
    const idInput = document.getElementById('sched-form-id');
    const nameInput = document.getElementById('sched-form-name');
    const timeInput = document.getElementById('sched-form-time');
    const actionSelect = document.getElementById('sched-form-action-type');
    const colorSelect = document.getElementById('sched-form-daynight-color');
    const lightSelect = document.getElementById('sched-form-daynight-light');
    const chanInput = document.getElementById('sched-form-channel');
    const submitBtn = document.getElementById('sched-modal-submit');
    const msgEl = document.getElementById('sched-form-msg');

    const isEdit = idInput && idInput.value.trim() !== '';
    const name = nameInput ? nameInput.value.trim() : '';
    const timeVal = timeInput ? timeInput.value.trim() : '';
    const actionType = actionSelect ? actionSelect.value : 'picture_day_night';
    const enabled = enabledInput ? enabledInput.checked : true;

    if (!name) {
      if (msgEl) { msgEl.className = 'msg err'; msgEl.textContent = 'Vui lòng nhập tên lịch trình.'; }
      return;
    }
    if (!timeVal) {
      if (msgEl) { msgEl.className = 'msg err'; msgEl.textContent = 'Vui lòng chọn giờ chạy (HH:MM).'; }
      return;
    }

    // Days of week
    const selectedDays = [];
    document.querySelectorAll('#sched-form-days input[type="checkbox"]:checked').forEach(cb => {
      selectedDays.push(parseInt(cb.value, 10));
    });

    // Selected cameras
    const selectedCams = [];
    document.querySelectorAll('#sched-form-cameras input.sched-cam-cb:checked').forEach(cb => {
      if (cb.value) selectedCams.push(cb.value);
    });

    if (selectedCams.length === 0) {
      if (msgEl) { msgEl.className = 'msg err'; msgEl.textContent = 'Vui lòng chọn ít nhất 1 camera áp dụng.'; }
      return;
    }

    let actionObj = {};
    if (actionType === 'picture_day_night') {
      actionObj = {
        deviceIds: selectedCams,
        dayNightColor: colorSelect ? parseInt(colorSelect.value, 10) : 0,
        supplementLight: lightSelect ? lightSelect.value : 'auto',
        channel: chanInput ? parseInt(chanInput.value, 10) : 0
      };
    } else {
      actionObj = {
        deviceIds: selectedCams
      };
    }

    const payload = {
      name: name,
      enabled: enabled,
      actionType: actionType,
      schedule: {
        time: timeVal,
        daysOfWeek: selectedDays
      },
      action: actionObj
    };
    if (isEdit) {
      payload.id = idInput.value.trim();
    }

    btnBusy(submitBtn, true, 'Đang lưu...');
    if (msgEl) { msgEl.className = 'msg'; msgEl.textContent = ''; }

    try {
      const endpoint = isEdit ? '/api/scheduler/jobs/update' : '/api/scheduler/jobs';
      await api(endpoint, {
        method: 'POST',
        body: JSON.stringify(payload)
      });
      showToast(isEdit ? 'Đã cập nhật lịch trình.' : 'Đã thêm lịch trình mới.', 'ok');
      closeDialog(document.getElementById('scheduler-job-modal'));
      await renderSchedulerPanel(true);
    } catch (err) {
      if (msgEl) {
        msgEl.className = 'msg err';
        msgEl.textContent = 'Lỗi lưu: ' + err.message;
      }
      showToast('Lỗi lưu lịch trình: ' + err.message, 'err');
    } finally {
      btnBusy(submitBtn, false);
    }
  }

  function initSchedulerEvents() {
    if (schedulerEventsBound) return;
    schedulerEventsBound = true;

    // Open add modal
    const addBtn = document.getElementById('scheduler-add-btn');
    if (addBtn) {
      addBtn.addEventListener('click', async () => {
        await ensureCamerasLoaded();
        openSchedulerModal(null);
      });
    }

    // Refresh button
    const refreshBtn = document.getElementById('scheduler-refresh-btn');
    if (refreshBtn) {
      refreshBtn.addEventListener('click', async () => {
        btnBusy(refreshBtn, true, 'Đang tải...');
        await renderSchedulerPanel(true);
        btnBusy(refreshBtn, false);
        showToast('Đã làm mới dữ liệu lịch trình.', 'ok');
      });
    }

    // Clear history button
    const clearHistBtn = document.getElementById('scheduler-clear-history-btn');
    if (clearHistBtn) {
      clearHistBtn.addEventListener('click', async () => {
        if (!confirm('Bạn có chắc muốn xóa toàn bộ nhật ký lịch sử thực thi?')) return;
        try {
          await api('/api/scheduler/history', { method: 'DELETE' });
          showToast('Đã xóa nhật ký lịch sử.', 'ok');
          await renderSchedulerPanel(true);
        } catch (err) {
          showToast('Lỗi xóa nhật ký: ' + err.message, 'err');
        }
      });
    }

    // Modal close & cancel
    const modalClose = document.getElementById('scheduler-modal-close');
    if (modalClose) {
      modalClose.addEventListener('click', () => closeDialog(document.getElementById('scheduler-job-modal')));
    }
    const modalCancel = document.getElementById('sched-modal-cancel');
    if (modalCancel) {
      modalCancel.addEventListener('click', () => closeDialog(document.getElementById('scheduler-job-modal')));
    }

    // Action type dropdown change
    const actionSelect = document.getElementById('sched-form-action-type');
    if (actionSelect) {
      actionSelect.addEventListener('change', updateActionTypeUI);
    }

    // Days all button
    const daysAllBtn = document.getElementById('sched-days-all-btn');
    if (daysAllBtn) {
      daysAllBtn.addEventListener('click', () => {
        document.querySelectorAll('#sched-form-days input[type="checkbox"]').forEach(cb => { cb.checked = true; });
      });
    }

    // Camera quick select buttons
    const camAllBtn = document.getElementById('sched-cam-all-btn');
    if (camAllBtn) {
      camAllBtn.addEventListener('click', () => {
        document.querySelectorAll('#sched-form-cameras input.sched-cam-cb').forEach(cb => { cb.checked = true; });
      });
    }
    const camNoneBtn = document.getElementById('sched-cam-none-btn');
    if (camNoneBtn) {
      camNoneBtn.addEventListener('click', () => {
        document.querySelectorAll('#sched-form-cameras input.sched-cam-cb').forEach(cb => { cb.checked = false; });
      });
    }
    const camDahuaBtn = document.getElementById('sched-cam-dahua-btn');
    if (camDahuaBtn) {
      camDahuaBtn.addEventListener('click', () => {
        document.querySelectorAll('#sched-form-cameras .sched-cam-item').forEach(item => {
          const v = (item.dataset.vendor || '').toLowerCase();
          const cb = item.querySelector('input.sched-cam-cb');
          if (cb) {
            cb.checked = (v === 'dahua' || v.includes('kbvision') || v.includes('imou'));
          }
        });
      });
    }

    // Form submit
    const jobForm = document.getElementById('scheduler-job-form');
    if (jobForm) {
      jobForm.addEventListener('submit', handleJobFormSubmit);
    }
  }

  async function renderSchedulerPanel(force) {
    initSchedulerEvents();
    await ensureCamerasLoaded();

    const jobsTbody = document.getElementById('scheduler-jobs-tbody');
    const histTbody = document.getElementById('scheduler-history-tbody');
    const statTotal = document.getElementById('scheduler-stat-total');
    const statActive = document.getElementById('scheduler-stat-active');
    const statLastRun = document.getElementById('scheduler-stat-last-run');
    const statRunning = document.getElementById('scheduler-stat-running');

    try {
      const [jobsRes, histRes] = await Promise.all([
        api('/api/scheduler/jobs'),
        api('/api/scheduler/history')
      ]);

      cachedJobs = Array.isArray(jobsRes) ? jobsRes : (jobsRes.jobs || []);
      cachedHistory = Array.isArray(histRes) ? histRes : (histRes.history || []);

      // Update stats
      if (statTotal) statTotal.textContent = cachedJobs.length;
      if (statActive) statActive.textContent = cachedJobs.filter(j => j.enabled).length;
      if (statRunning) {
        const isRunning = jobsRes && jobsRes.running !== false;
        statRunning.textContent = isRunning ? '🟢 Đang chạy' : '⚪ Đã tắt';
        statRunning.style.color = isRunning ? 'var(--success, #10b981)' : 'var(--muted, #94a3b8)';
      }

      if (statLastRun) {
        if (cachedHistory.length > 0) {
          statLastRun.textContent = formatTime(cachedHistory[0].triggeredAt);
        } else {
          statLastRun.textContent = '—';
        }
      }

      // Render Jobs Table
      if (jobsTbody) {
        if (cachedJobs.length === 0) {
          jobsTbody.innerHTML = '<tr><td colspan="8" class="empty-hint">Chưa có lịch trình nào. Bấm "+ Thêm lịch trình" để tạo mới.</td></tr>';
        } else {
          jobsTbody.innerHTML = cachedJobs.map(j => {
            const isDNHour = j.actionType === 'picture_day_night';
            let actionHtml = '';
            if (isDNHour) {
              const colorCode = j.action ? j.action.dayNightColor : 0;
              const ch = j.action ? (j.action.channel || 0) : 0;
              const suppLight = (j.action && j.action.supplementLight && j.action.supplementLight !== 'auto')
                ? `<div style="font-size:0.72rem; color:var(--muted);">Đèn: ${escapeHtml(j.action.supplementLight)}</div>`
                : '';
              actionHtml = `
                <div><span class="badge info">Chỉnh màu</span> <strong>${escapeHtml(dayNightColorLabel(colorCode))}</strong></div>
                <div style="font-size:0.78rem; color:var(--muted); margin-top:2px;">Kênh ${ch} (read-back verify)</div>
                ${suppLight}
              `;
            } else if (j.actionType === 'reboot') {
              actionHtml = `<div><span class="badge warn">Khởi động lại (Reboot)</span></div>`;
            } else {
              actionHtml = `<div><span class="badge">${escapeHtml(j.actionType || 'custom')}</span></div>`;
            }

            const daysStr = formatDays(j.schedule ? j.schedule.daysOfWeek : []);
            const timeStr = j.schedule ? j.schedule.time : '—';

            // Targets
            const targetIds = (j.action && Array.isArray(j.action.deviceIds)) ? j.action.deviceIds : [];
            const targetsHtml = targetIds.map(tid => {
              return `<span class="chip" style="font-size:0.75rem; padding:1px 6px;" title="${escapeHtml(tid)}">${escapeHtml(resolveCameraDisplay(tid))}</span>`;
            }).join(' ') || '<span class="muted">—</span>';

            // Next run
            const nextRunHtml = (j.enabled && j.nextRunAt) ? formatTime(j.nextRunAt) : '<span class="muted">—</span>';

            // Last status
            let lastStatusBadge = '<span class="badge info">Chưa chạy</span>';
            if (j.lastStatus === 'success') {
              lastStatusBadge = '<span class="badge ok" title="' + escapeHtml(j.lastMessage || '') + '">✓ Thành công</span>';
            } else if (j.lastStatus === 'error') {
              lastStatusBadge = '<span class="badge fail" title="' + escapeHtml(j.lastMessage || '') + '">✗ Lỗi</span>';
            } else if (j.lastStatus === 'running') {
              lastStatusBadge = '<span class="badge warn">⏳ Đang chạy</span>';
            }

            // Status & toggle button
            const statusHtml = j.enabled
              ? `<span class="badge ok">Đang bật</span> <button type="button" class="btn btn-sm btn-secondary sched-btn-toggle" data-id="${escapeHtml(j.id)}" data-enabled="false" style="padding:2px 6px; font-size:0.72rem; margin-left:4px;">Tắt</button>`
              : `<span class="badge warn">Đã tắt</span> <button type="button" class="btn btn-sm btn-secondary sched-btn-toggle" data-id="${escapeHtml(j.id)}" data-enabled="true" style="padding:2px 6px; font-size:0.72rem; margin-left:4px;">Bật</button>`;

            return `
              <tr data-id="${escapeHtml(j.id)}">
                <td>
                  <strong>${escapeHtml(j.name || j.id)}</strong>
                  <div style="font-size:0.72rem; color:var(--muted); font-family:monospace;">${escapeHtml(j.id)}</div>
                </td>
                <td>${actionHtml}</td>
                <td>
                  <strong style="font-size:0.95rem;">${escapeHtml(timeStr)}</strong>
                  <div style="font-size:0.78rem; color:var(--muted);">${escapeHtml(daysStr)}</div>
                </td>
                <td><div style="display:flex; flex-wrap:wrap; gap:4px; max-width:220px;">${targetsHtml}</div></td>
                <td><span style="font-size:0.82rem;">${nextRunHtml}</span></td>
                <td>${lastStatusBadge}</td>
                <td>${statusHtml}</td>
                <td style="text-align: right;">
                  <div style="display:inline-flex; gap:4px; align-items:center;">
                    <button type="button" class="btn btn-sm btn-primary sched-btn-run" data-id="${escapeHtml(j.id)}" data-testid="job-run-${escapeHtml(j.id)}" title="Chạy thử nghiệm tác vụ ngay lập tức">▶ Chạy ngay</button>
                    <button type="button" class="btn btn-sm btn-secondary sched-btn-edit" data-id="${escapeHtml(j.id)}" data-testid="job-edit-${escapeHtml(j.id)}" title="Sửa lịch trình">✏️</button>
                    <button type="button" class="btn btn-sm btn-danger sched-btn-delete" data-id="${escapeHtml(j.id)}" data-testid="job-delete-${escapeHtml(j.id)}" title="Xóa lịch trình">🗑️</button>
                  </div>
                </td>
              </tr>
            `;
          }).join('');

          // Bind job action buttons
          jobsTbody.querySelectorAll('.sched-btn-toggle').forEach(btn => {
            btn.addEventListener('click', async () => {
              const id = btn.dataset.id;
              const willEnable = btn.dataset.enabled === 'true';
              btn.disabled = true;
              try {
                await api('/api/scheduler/jobs/toggle', {
                  method: 'POST',
                  body: JSON.stringify({ id: id, enabled: willEnable })
                });
                showToast(`Đã ${willEnable ? 'bật' : 'tắt'} lịch trình.`, 'ok');
                await renderSchedulerPanel(true);
              } catch (err) {
                showToast('Lỗi bật/tắt lịch trình: ' + err.message, 'err');
                btn.disabled = false;
              }
            });
          });

          jobsTbody.querySelectorAll('.sched-btn-run').forEach(btn => {
            btn.addEventListener('click', async () => {
              const id = btn.dataset.id;
              btnBusy(btn, true, 'Đang chạy...');
              try {
                const res = await api('/api/scheduler/jobs/run', {
                  method: 'POST',
                  body: JSON.stringify({ id: id })
                });
                if (res && res.record && res.record.status === 'success') {
                  showToast(`Chạy thành công: ${res.record.message || 'OK'}`, 'ok');
                } else {
                  const errMsg = (res && res.error) || (res && res.record && res.record.error) || 'Có lỗi khi chạy tác vụ';
                  showToast(`Kết quả: ${errMsg}`, 'warn');
                }
                await renderSchedulerPanel(true);
              } catch (err) {
                showToast('Lỗi khi chạy tác vụ: ' + err.message, 'err');
                btnBusy(btn, false);
              }
            });
          });

          jobsTbody.querySelectorAll('.sched-btn-edit').forEach(btn => {
            btn.addEventListener('click', () => {
              const id = btn.dataset.id;
              const job = cachedJobs.find(j => j.id === id);
              if (job) openSchedulerModal(job);
            });
          });

          jobsTbody.querySelectorAll('.sched-btn-delete').forEach(btn => {
            btn.addEventListener('click', async () => {
              const id = btn.dataset.id;
              const job = cachedJobs.find(j => j.id === id);
              const jobName = job ? (job.name || job.id) : id;
              if (!confirm(`Bạn có chắc muốn xóa lịch trình "${jobName}"?`)) return;

              btn.disabled = true;
              try {
                await api('/api/scheduler/jobs/delete', {
                  method: 'POST',
                  body: JSON.stringify({ id: id })
                });
                showToast('Đã xóa lịch trình.', 'ok');
                await renderSchedulerPanel(true);
              } catch (err) {
                showToast('Lỗi xóa lịch trình: ' + err.message, 'err');
                btn.disabled = false;
              }
            });
          });
        }
      }

      // Render History Table
      if (histTbody) {
        if (cachedHistory.length === 0) {
          histTbody.innerHTML = '<tr><td colspan="6" class="empty-hint">Chưa có lịch sử thực thi nào.</td></tr>';
        } else {
          histTbody.innerHTML = cachedHistory.map(h => {
            const statusBadge = h.status === 'success'
              ? '<span class="badge ok">✓ Thành công</span>'
              : '<span class="badge fail">✗ Thất bại</span>';

            const durMs = h.durationMs != null ? h.durationMs : Math.round((h.duration || 0) / 1000000);
            const errHtml = h.error ? `<div style="color:var(--danger); font-size:0.78rem; margin-top:2px;">${escapeHtml(h.error)}</div>` : '';

            return `
              <tr>
                <td><span style="font-family:monospace; font-size:0.82rem;">${escapeHtml(formatTime(h.triggeredAt))}</span></td>
                <td><strong>${escapeHtml(h.jobName || h.jobId)}</strong></td>
                <td><span class="badge ${h.actionType === 'reboot' ? 'warn' : 'info'}">${escapeHtml(h.actionType)}</span></td>
                <td><span style="font-size:0.82rem; color:var(--muted);">${durMs} ms</span></td>
                <td>${statusBadge}</td>
                <td>
                  <div style="font-size:0.85rem;">${escapeHtml(h.message || '—')}</div>
                  ${errHtml}
                </td>
              </tr>
            `;
          }).join('');
        }
      }

    } catch (err) {
      console.error('scheduler: render error', err);
      const msgBox = document.getElementById('scheduler-msg');
      if (msgBox) {
        msgBox.className = 'msg err';
        msgBox.textContent = 'Lỗi tải thông tin lịch trình: ' + err.message;
      }
    }
  }

  // Export to window for app.js and HTML
  window.renderSchedulerPanel = renderSchedulerPanel;
  window.openSchedulerModal = openSchedulerModal;
})();
