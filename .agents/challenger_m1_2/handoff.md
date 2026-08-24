# Adversarial Challenge Report — Milestone 1 (M1: Full Overhaul of `/#cameras`)

**Agent**: Challenger 2 (Empirical Challenger: Critic & Specialist)  
**Target Milestone**: Milestone 1 (Full Overhaul of `/#cameras`)  
**Verdict**: **`REQUEST_CHANGES`**

---

## 1. Observation

### A. Codebase & Test Suite Executions
1. **Go Test Suite**: Executed `/home/ksp/inut-rk3528-browswer/wpebuild/godl/go/bin/go test -count=1 ./...`.
   - Result: 100% PASS across all Go packages (`internal/bulk`, `internal/camera`, `internal/config`, `internal/dahua`, `internal/discovery`, `internal/hik`, `internal/importer`, `internal/isapi`, `internal/mcp`, `internal/nvrhealth`, `internal/redbida`, `internal/server`, `internal/shinobi`, `internal/tiandy`, `web`).
2. **Empirical Adversarial Test Suite**: Authored and executed dedicated stress harness `tests/ui/m1_challenger2.spec.js` probing edge cases across Camera Detail fullscreen, PTZ keyboard navigation, Quick PTZ modal, Wi-Fi RSSI gauge, NVR diagnostics & watchdog, and Grid card DOM resilience.
3. **Console & Runtime Stability**: 0 uncaught JavaScript exceptions across all flows.

### B. Confirmed Defect: Grid View Card Checkbox Event Interception
- **File**: `web/static/app.js`, Line 506 & Line 998:
  ```javascript
  // Line 506 in renderCameras():
  <label class="cam-card-check" title="Chọn camera" onclick="event.stopPropagation()">
    <input type="checkbox" class="cam-card-cb" value="${escapeHtml(c.id)}" ${isChecked ? 'checked' : ''}>
  </label>
  ```
  ```javascript
  // Line 990-1008 in #cam-grid event listener:
  document.getElementById('cam-grid')?.addEventListener('click', async (ev) => {
    ...
    const cb = ev.target.closest('.cam-card-cb');
    if (cb) {
      ev.stopPropagation();
      setCameraSelected(cb.value, cb.checked);
      return;
    }
    const card = ev.target.closest('.cam-card[data-id]');
    if (card) {
      gotoCameraDetail(card.dataset.id, 'osd');
    }
  });
  ```
- **Observed Behavior**:
  - The inline attribute `onclick="event.stopPropagation()"` on `<label class="cam-card-check">` absorbs the click event before it can bubble to `#cam-grid`.
  - `#cam-grid` does not listen to `change` events, and its `click` listener is completely bypassed when clicking the checkbox or label.
  - As a result, checking or unchecking a camera's checkbox in Grid Card view never invokes `setCameraSelected(cb.value, cb.checked)`.
  - The camera is not added to `selectedCameraSet`, `#bulk-selected-count` remains "Chưa chọn camera nào.", and selection is not synchronized with Table view (`.cam-cb`) or Bulk Operations (`.bulk-cam-cb`).

---

## 2. Logic Chain

1. **Step 1 — Fullscreen Toggle & Live Stream**:
   - Tested `#cd-live-fullscreen` clicking when preview is stopped (targeting `#ce-preview-img-wrap`) vs running (targeting `#cd-live`). Verified cross-browser fallbacks (`requestFullscreen` and `webkitRequestFullscreen`). Verified stream persists and does not disconnect. -> **PASS**
2. **Step 2 — PTZ Keyboard Shortcuts & Quick PTZ Modal**:
   - Tested Arrow keys & WASD keydown (`start: true`) and keyup (`start: false`). Verified speed value is properly retrieved from input or defaults to 5.
   - Tested focus guard: Typed text in OSD input fields (`<input class="ce-osd-line">`) and verified 0 PTZ commands were dispatched.
   - Tested `#quick-ptz-dialog`: Verified 8-direction pad buttons (`.qptz-btn[data-ptz]`), speed slider (1–8), and "Mở cấu hình PTZ đầy đủ" deep-link navigation to `#cameras/cam/<id>/ptz`. -> **PASS**
3. **Step 3 — Wi-Fi RSSI Gauge Rendering & Edge Cases**:
   - Tested multi-tier signal strength rendering (>=70% `.active-high`, >=40% `.active-med`, <40% `.active-low`).
   - Stress-tested XSS injection in SSID (`<script>alert("xss")</script>`); verified clean escaping without DOM script execution.
   - Verified clicking a Wi-Fi chip populates `#net-wifi-ssid`. -> **PASS**
4. **Step 4 — NVR Diagnostics, Mapping & Watchdog**:
   - Verified NVR health timeline view, NVR channel scanning, sub-channel linking via `POST /api/nvr/link`, and watchdog toggle via `POST /api/nvr/watchdog`. -> **PASS**
5. **Step 5 — Grid View Checkbox Selection**:
   - In Grid View (`#cam-grid`), clicking the checkbox on `.cam-card` was empirically tested.
   - Because of `onclick="event.stopPropagation()"` on `<label class="cam-card-check">`, the click event does not reach the `#cam-grid` event listener.
   - Neither `selectedCameraSet` nor `#bulk-selected-count` is updated.
   - Direct consequence: Users in Grid View cannot select cameras for bulk deletion, bulk configuration, or bulk password reset. -> **FAIL (Bug Confirmed)**

---

## 3. Caveats

- All backend Go REST endpoints (`/api/cameras`, `/api/probe`, `/api/apply`, `/api/ptz`, `/api/wifi-scan`, `/api/nvr/*`) adhere strictly to interface contracts and pass 100% of unit tests.
- The defect is strictly isolated to the event handling of the Grid View card checkbox in `web/static/app.js`.

---

## 4. Conclusion & Required Changes

**Verdict**: **`REQUEST_CHANGES`**

### Required Action for Worker:
Fix the checkbox event handling on `.cam-card` in `web/static/app.js`:
1. **Option A (Recommended)**: Add a `change` event listener to `#cam-grid`:
   ```javascript
   document.getElementById('cam-grid')?.addEventListener('change', (ev) => {
     if (ev.target.classList.contains('cam-card-cb')) {
       setCameraSelected(ev.target.value, ev.target.checked);
     }
   });
   ```
2. **Option B**: Remove `onclick="event.stopPropagation()"` from `<label class="cam-card-check">` and let `#cam-grid`'s click listener handle it by stopping propagation before card detail navigation:
   ```javascript
   if (ev.target.closest('.cam-card-check, .cam-card-cb')) {
     const cb = ev.target.closest('.cam-card-check')?.querySelector('.cam-card-cb') || ev.target.closest('.cam-card-cb');
     if (cb) setCameraSelected(cb.value, cb.checked);
     return;
   }
   ```

---

## 5. Verification Method

To verify the fix:
1. **Run the Adversarial Playwright Spec**:
   ```bash
   PATH=/home/ksp/.nvm/versions/node/v24.18.1/bin:$PATH npx playwright test tests/ui/m1_challenger2.spec.js --workers=1
   ```
   *Expected outcome: 12 tests passed across desktop and mobile.*
2. **Run the Full Playwright Spec for Cameras & Detail**:
   ```bash
   PATH=/home/ksp/.nvm/versions/node/v24.18.1/bin:$PATH npx playwright test tests/ui/cameras.spec.js tests/ui/detail.spec.js tests/ui/bulk.spec.js tests/ui/nvr.spec.js --workers=1
   ```
3. **Run Go Unit Tests**:
   ```bash
   /home/ksp/inut-rk3528-browswer/wpebuild/godl/go/bin/go test -count=1 ./...
   ```
