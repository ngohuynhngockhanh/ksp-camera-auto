# Orchestration Plan: kspcam UI/UX & RedBida Overhaul

## Objectives
1. **R1: Full Overhaul of `/#cameras`**:
   - Modern Glassmorphism layout (Grid Card & Table toggle with snapshot thumbnails, vendor badges, status indicators).
   - Quick Actions Toolbar (Instant Live stream, Snapshot preview, PTZ quick nudge, Reboot, NTP sync).
   - Professional Camera Detail Workspace: Left col (MJPEG stream + Snapshot with fullscreen/auto-refresh), Right col (7 Tab control center: OSD/Channel Name, Color sliders with live preview, Video/Audio encoder, Network & Wi-Fi scanning with RSSI bars, PTZ pan-tilt with keyboard shortcuts, Storage format/status, Maintenance & Auto-Reboot schedule).
   - Smart Bulk Wizard with safety clamping alerts, Golden Template 1-click apply, progress bar.
   - NVR Diagnostics & sub-channel scanning: timeline gap health visualization, automated sub-camera mapping.
2. **R2: Full Overhaul of `/#redbida`**:
   - Golden Standard Inspector & 1-Click Auto-Fix with % progress indicator across all 15 parameters.
   - Curated 8 CSS Gradient Palette with Live Canvas Preview and instant swatch picking.
   - Visual 20-Tab INI Editor `[C01]`..`[C20]` grid view with table name synchronization to `vid_play_label` and quick copy.
   - Smart Hashtag Generator with Unicode normalization (removing Vietnamese diacritics dynamically).
   - Enhanced Key Management table with risk badges, filter/search, inline image/color preview.
3. **R3: Testing, Multi-Arch Build, Deployment & Git**:
   - Deep inspection of DOM/JS/CSS/Go backend compatibility.
   - 100% passing Go unit tests (`go test ./...`) and Playwright UI tests.
   - Multi-arch static binary build (`linux/amd64`, `linux/arm64`, `linux/armv7`).
   - Remote deployment to edge nodes `inut_204_164` and `inut_204_163` via SSH/Ansible/SCP.
   - Commit & push all changes to git `main`.

## Phased Workflow
- **Phase 0: Architecture & Codebase Survey**
  - Explorer 1: `/#cameras` frontend & backend interaction analysis (`web/static/app.js`, `web/static/ui-core.js`, `web/static/index.html`, `web/static/style.css`, `/api/cameras`, `/api/probe`, `/api/apply`, `/api/snapshot`, `/api/live`, `/api/ptz`, `/api/reboot`, `/api/storage`, `/api/recordings`, `/api/channel-info`, `/api/osd`, `/api/picture`, `/api/network`, `/api/wifi`, `/api/device-time`, `/api/autoreboot`, `/api/nvr/scan`, `/api/nvr/health`, `/api/nvr/watchdog`, camera-naming Golden Template skill).
  - Explorer 2: `/#redbida` frontend & backend analysis (`web/static/redbida.js`, `web/static/index.html`, `web/static/style.css`, `internal/redbida/`, Golden Standard Inspector, % progress, Curated 8 CSS Gradient Palette, Live Canvas Preview, Visual 20-Tab INI Editor `[C01]`..`[C20]`, Smart Hashtag Generator, Enhanced Key Management).
  - Explorer 3: Architecture, Testing & Deployment analysis (`tests/ui/`, Playwright specs, Go unit tests, Makefile / static build scripts, remote nodes `inut_204_164` / `inut_204_163`, systemd services, git repository state).
- **Phase 1: Milestone 1 — `/#cameras` Overhaul**
  - Worker -> 2 Reviewers -> 2 Challengers -> Forensic Auditor.
- **Phase 2: Milestone 2 — `/#redbida` Overhaul**
  - Worker -> 2 Reviewers -> 2 Challengers -> Forensic Auditor.
- **Phase 3: Milestone 3 — Comprehensive Testing, Multi-Arch Build, Deployment & Git**
  - Worker -> 2 Reviewers -> 2 Challengers -> Forensic Auditor.
