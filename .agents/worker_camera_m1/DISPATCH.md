## 2026-08-24T14:44:06Z
You are the Implementation Worker for Milestone 1 (M1: Full Overhaul of `/#cameras`) in `ksp-camera-auto`.
Your working directory is: /home/ksp/ksp-camera-auto/.agents/worker_camera_m1

Read the following files before starting:
- `/home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md` (Specifically R1: `/#cameras` Overhaul)
- `/home/ksp/ksp-camera-auto/PROJECT.md`
- `/home/ksp/ksp-camera-auto/.agents/explorer_survey_cameras/analysis.md` (Detailed architectural analysis)
- `/home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md` (Golden Template & naming rules)

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Scope and Write Ownership:
You own and may edit:
- `web/static/index.html`
- `web/static/app.js`
- `web/static/ui-core.js`
- `web/static/style.css`

Your tasks for Milestone 1:
1. **View Switcher (Grid Cards & Table View)**:
   - Add view mode toggle (Table / Grid Cards) in `#view-cameras` toolbar.
   - Implement Glassmorphism Grid Cards view rendering: snapshot thumbnail (loaded from `/api/snapshot?id=...`), vendor badge (Dahua/Hikvision/Tiandy/ONVIF), resolution & FPS tag, status indicators (online, watchdog, no-storage), and quick actions.
   - Persist view preference in `localStorage` (`kspcam_cam_view_mode`).
   - Preserve `#cam-table`, `#cam-tbody` and all existing `data-testid` attributes for backward compatibility and Playwright test assertions.
2. **Quick Actions Toolbar (1-Click Actions)**:
   - Add 1-click quick action buttons to each camera row and grid card:
     - 👁️ Live Stream (opens live preview modal or scrolls to detail live stream)
     - 📷 Snapshot (opens instant snapshot preview modal)
     - 🎮 Quick PTZ (opens quick PTZ nudge modal or switches to detail PTZ tab)
     - 🔄 Reboot (reboots device with confirmation modal)
     - ⏰ NTP Sync (syncs device time with host)
3. **Camera Detail Workspace (Left Column Preview + Right Column 7 Tabs)**:
   - Polish left column: Live MJPEG stream (`/api/live`) with auto-reconnect, 5-minute timeout countdown & refresh button, snapshot reload, channel picker, fullscreen button.
   - Refine and ensure smooth operation of the 7 detail tabs:
     - Tab 1: OSD / Tên Kênh (`/api/channel-info`, `/api/channel-name`, `/api/osd`)
     - Tab 2: Chỉnh Màu Lite/Full với thanh trượt realtime (`/api/picture`)
     - Tab 3: Video Encoder với ceiling FPS capability check (`/api/fps-capability`)
     - Tab 4: Audio Encoder với AAC conversion options
     - Tab 5: Mạng & Quét Wi-Fi với cột sóng tín hiệu RSSI % (`/api/network`, `/api/wifi`, `/api/wifi-scan`)
     - Tab 6: Bàn xoay PTZ 8 hướng hỗ trợ press-and-hold & phím tắt bàn phím (`/api/ptz`)
     - Tab 7: Bảo trì, Định dạng ổ đĩa & Lập lịch tự động khởi động (`/api/storage`, `/api/autoreboot`, `/api/reboot`, `/api/device-time`)
4. **Smart Bulk Wizard & Golden Template 1-Click**:
   - Add button **"⚡ Áp dụng Chuẩn Bida (Golden Template)"** in the Bulk Configuration panel that auto-populates standard bida profile (H.264/H.265 baseline, Remux audio copy, GOP 50/100, 5-min segment).
   - Implement **Safety Limits Inspector**: automatically warn if selected parameters exceed hardware safe limits (e.g., FPS > 25 on 4K, or bitrate > 8192kbps).
   - Sequential progress feedback and summary during bulk apply.
5. **NVR Diagnostics & Sub-channel Scanning**:
   - Refine visual NVR timeline gap analysis and health status report.
   - Sub-channel auto scan and mapping from NVR (`/api/nvr/scan`, `/api/nvr/link`, `/api/nvr/health`, `/api/nvr/watchdog`).
6. **Verification**:
   - Run `go test ./...` and ensure all Go tests pass.
   - Verify that all existing `data-testid` attributes are strictly preserved.
   - Write your complete handoff report to `/home/ksp/ksp-camera-auto/.agents/worker_camera_m1/handoff.md`.
   - Send a message to parent when completed.
