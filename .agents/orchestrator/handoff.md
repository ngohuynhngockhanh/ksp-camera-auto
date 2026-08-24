# Soft Handoff — Project Orchestrator (Generation 1 to Generation 2)

## 1. Observation & Milestone State
- **Phase 0 (Survey)**: 100% DONE. 3 Explorers mapped `/#cameras`, `/#redbida`, and Testing/Build/Deployment infrastructure.
- **Milestone 1 (`/#cameras` Full Overhaul)**: 100% DONE & PASSED.
  - View Switcher (Table & Glassmorphism Card Grid with snapshot thumbnails, vendor badges, status indicators, `localStorage` persistence).
  - Quick Actions Toolbar (Instant Live stream, Snapshot lightbox modal, Quick PTZ modal with arrow/WASD keyboard navigation, Reboot with confirmation, NTP host time sync).
  - Camera Detail Workspace (Left column live MJPEG with fullscreen support, Right column 7 tabs, Wi-Fi RSSI signal meters).
  - Smart Bulk Wizard with Golden Template 1-click apply and Hardware Safety Limits Inspector.
  - NVR Diagnostics & sub-channel scanning.
  - Verified across 15 challenger tests and 75 full Playwright tests. Gate status: **PASS**.
- **Milestone 2 (`/#redbida` Full Overhaul)**: Iteration 1 Completed, ready for minor remediation.
  - Implemented Golden Standard Inspector 15 keys, % Chuẩn Bida progress bar, 1-click auto-fix per key and auto-fix all.
  - Implemented 8 Curated CSS Gradient Palette & Live Canvas Preview simulation.
  - Implemented Visual 20-Tab INI Matrix Editor `[C01]`..`[C20]` with 2-way sync, raw INI toggle, 1-click venue name sync, and quick copy stream URL.
  - Implemented Smart Unicode Hashtags Generator (NFC/NFD diacritics removal).
  - Implemented Key Management toolbar with group pills, fast search, and inline logo/gradient previews.
  - Reviewer 1 & 2: **APPROVE**. Forensic Auditor: **CLEAN**.
  - Challengers 1 & 2 found 3 regex/validation edge cases:
    1. `ui_bg.fix`: replace `.replace(/;\s*$/, '')` with `.replace(/[;\s]+$/, '')` in `web/static/redbida.js` (lines 236, 694, 802, 947) to strip multiple trailing semicolons. If non-gradient, fallback to default standard gradient.
    2. `custom_hashtags.check`: add `/i` flag (or `[a-zA-Z0-9_#\s...]`) to catch uppercase Vietnamese accented characters (e.g. `#QUÁNBIDA`).
    3. `company_name.check`: clean fallback when `ui_title` is unset/empty.
- **Milestone 3 (Testing, Multi-Arch Build, Deployment & Git)**: Ready to execute after M2 pass.

## 2. Active Subagents
- All 16 Generation 1 subagents have finished and are retired.

## 3. Pending Decisions & Immediate Next Steps for Successor
1. **Milestone 2 Remediation**: Spawn a worker to apply the 3 quick regex/fallback fixes in `web/static/redbida.js`. Run `npx playwright test tests/ui/redbida_m2_adversarial.spec.js` and all suites to verify 100% PASS. Mark M2 DONE.
2. **Milestone 3 Execution**:
   - Run 100% Go unit tests (`go test ./...`) and 100% Playwright UI test suites (`npx playwright test`).
   - Run multi-arch static binary build (`make build-all`) with `CGO_ENABLED=0` for `linux/amd64`, `linux/arm64`, and `linux/armv7`.
   - Deploy to remote edge nodes `inut_204_164` (192.168.204.164) and `inut_204_163` (192.168.204.163) via Ansible / SSH / SCP. Verify `kspcam.service` status and public health checks (`http://ksp-cam-inut-204-164.video.io.vn/healthz` -> 200 OK).
   - Commit all changes with clean commit message and push to git `main`.
   - Deliver full completion report to user.

## 4. Key Artifacts
- `/home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md` — Authoritative request
- `/home/ksp/ksp-camera-auto/PROJECT.md` — Global architecture & feature inventory
- `/home/ksp/ksp-camera-auto/.agents/orchestrator/GATE_STATUS.md` — Gate status
- `/home/ksp/ksp-camera-auto/.agents/orchestrator/progress.md` — Status checklist
- `/home/ksp/ksp-camera-auto/.agents/orchestrator/BRIEFING.md` — State index
