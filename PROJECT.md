# Project: ksp-camera-auto UI/UX Overhaul & Dual-Track Deployment (Cameras & RedBida)

## Architecture
- **Web Layer (`web/static/`)**: Single-page application embedded via `go:embed static` into the Go static binary. Modern Glassmorphism styling (`--glass-*`, `backdrop-filter: blur(16px)`), responsive CSS Grid/Flexbox, dynamic DOM rendering across `/#cameras`, `/#redbida`, `/#discovery`, `/#shinobi`, and `/#help`.
- **API & Server Layer (`internal/server/`)**: Go HTTP server exposing 35+ REST endpoints (`/api/cameras`, `/api/probe`, `/api/apply`, `/api/snapshot`, `/api/live`, `/api/ptz`, `/api/storage`, `/api/recordings`, `/api/playback`, `/api/channel-info`, `/api/osd`, `/api/picture`, `/api/network`, `/api/wifi`, `/api/device-time`, `/api/autoreboot`, `/api/nvr/*`, `/api/redbida/*`, `/api/shinobi/*`) protected by session auth, rate limiting, and singleflight snapshot caching.
- **Core Protocol & Camera Adapters (`internal/camera/`, `internal/dahua/`, `internal/isapi/`, `internal/hik/`, `internal/tiandy/`)**: Unified camera capability interfaces with sequential bulk orchestration, hardware safety limits, and 3-state read-back verification.
- **RedBida & MQTT Layer (`internal/redbida/`, `internal/mcp/`)**: 15-key Golden Standard metadata catalog, MQTT broker integration (`127.0.0.1:12369`), and 31+ tool embedded Model Context Protocol (MCP) server.

## Feature Inventory
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F1 | View Switcher: Table & Grid Cards View | Modern Glassmorphism Grid Card view with auto snapshot thumbnails, vendor badges, status indicators, resolution/FPS tags alongside Table view | M1 | ORIGINAL_REQUEST §R1 |
| F2 | Quick Actions Toolbar | 1-Click action buttons on each camera row/card (Instant Live Stream, Snapshot modal, PTZ quick nudge, Reboot, NTP sync) | M1 | ORIGINAL_REQUEST §R1 |
| F3 | Camera Detail 7-Tab Workspace | Left column live MJPEG + snapshot preview with fullscreen/auto-refresh; Right column 7 tabs (OSD/Channel, Color sliders Lite/Full, Video encoder with FPS caps, Audio encoder with AAC conversion, Network & Wi-Fi scanning with RSSI %, PTZ 8-direction pan-tilt, Storage & Maintenance/Auto-Reboot) | M1 | ORIGINAL_REQUEST §R1 |
| F4 | Smart Bulk Wizard & Golden Template | 1-Click preset "Áp dụng Chuẩn Bida (Golden Template)", hardware safety limits clamping alerts (e.g., FPS > 25 on 4K), sequential execution progress | M1 | ORIGINAL_REQUEST §R1 |
| F5 | NVR Diagnostics & Sub-channel Scanning | Visual timeline gap analysis, automated sub-camera scanning & mapping from NVR, watchdog status | M1 | ORIGINAL_REQUEST §R1 |
| F6 | Golden Standard Inspector & 1-Click Auto-Fix | Automatic audit of 15 node configuration keys, % Chuẩn Bida progress bar, 1-Click per-key Auto-Fix and Auto-Fix All | M2 | ORIGINAL_REQUEST §R2 |
| F7 | Curated 8 CSS Gradient Palette & Live Canvas Preview | 8 luxury gradient presets (Royal Deep Blue Glow, Midnight Emerald Cyber, Cyberpunk Neon, Golden Velvet, Obsidian Carbon, Crimson Elegance, Sapphire Blue, Ruby Luxury), custom color picker, realtime canvas preview | M2 | ORIGINAL_REQUEST §R2 |
| F8 | Visual 20-Tab INI Editor `[C01]`..`[C20]` | 20-tab matrix grid editor, per-table label/URL editing, quick copy stream URL, 1-click sync venue name to `vid_play_label`, raw INI toggle | M2 | ORIGINAL_REQUEST §R2 |
| F9 | Smart Hashtag Generator | Dynamic Unicode normalization (NFC/NFD) stripping Vietnamese diacritics on typing venue name to produce clean hashtags | M2 | ORIGINAL_REQUEST §R2 |
| F10 | Enhanced Key Management Table | Group filtering, fast search, Risk Badges, inline image checkerboard preview, and gradient swatch preview | M2 | ORIGINAL_REQUEST §R2 |
| F11 | Deep Compatibility & Go Unit Tests | Verify 100% Go unit tests (`go test ./...`) across all packages without regressions | M3 | ORIGINAL_REQUEST §R3 |
| F12 | Playwright UI Automated Test Suite | Comprehensive Playwright test suite covering Cameras Grid/Table, Quick Actions, Detail 7-tabs, RedBida Inspector, 8 Gradients, 20-tab INI editor, and Hashtag generator | M3 | ORIGINAL_REQUEST §R3 |
| F13 | Multi-Arch Static Binary Build | Static compilation (`CGO_ENABLED=0`) for `linux/amd64`, `linux/arm64`, and `linux/armv7` via `make build-all` | M3 | ORIGINAL_REQUEST §R3 |
| F14 | Edge Node Deployment & Git Main Push | Remote deployment to `inut_204_164` and `inut_204_163` via Ansible/SSH/SCP, live healthz verification, git commit & push to `main` | M3 | ORIGINAL_REQUEST §R3 |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Full Overhaul of `/#cameras` | `web/static/index.html`, `web/static/app.js`, `web/static/ui-core.js`, `web/static/style.css` (Grid/Table, Quick Actions, Detail 7 Tabs, Smart Bulk Golden Template, NVR Diagnostics) | None | DONE |
| M2 | Full Overhaul of `/#redbida` | `web/static/index.html`, `web/static/redbida.js`, `web/static/style.css` (Inspector 15 keys, 8 Gradient Palette, 20-Tab INI Editor, Smart Hashtags, Key Management) | M1 | DONE |
| M3 | Comprehensive Testing, Multi-Arch Build, Edge Node Deployment & Git Push | `tests/ui/`, Playwright specs, Go unit tests, `Makefile`, Ansible deployment, target nodes `inut_204_164` / `inut_204_163`, Git push | M1, M2 | DONE |

## Interface Contracts
### Web UI ↔ REST API (`internal/server`)
- `GET /api/cameras` -> `[]deviceView`
- `POST /api/probe` with `{id, timeoutSeconds}` -> `probeView`
- `POST /api/fps-capability` with `{id, channel, stream, width, height, codec}` -> `{fpsList: []int}`
- `POST /api/apply` with `bulk.Request` -> `text/event-stream` (`bulk.Event`)
- `GET /api/snapshot?id=...&channel=...` -> `image/jpeg`
- `GET /api/live?id=...&channel=...&fps=...` -> `multipart/x-mixed-replace`
- `POST /api/ptz` with `{id, channel, code, speed, start}` -> `{ok: true}`
- `POST /api/reboot` with `{id}` -> `{ok: true}`
- `POST /api/device-time` with `{id, time}` -> `{ok: true}`
- `POST /api/nvr/scan` with `{id, ...}` -> `[]nvrScanRow`
- `GET /api/nvr/health?id=...` -> `nvrHealthReport`
- `POST /api/redbida/catalog` -> `{ "keys": [KeyMeta...], "sourceAvailable": bool }`
- `POST /api/redbida/refresh` with `{ "keys": ["k1", ...] }` -> `{ "values": { ... }, "refreshedAt": "RFC3339" }`
- `POST /api/redbida/apply` with `{ "changes": { "k1": "v1" }, "confirmed": bool }` -> `{ "results": { ... }, "appliedAt": "RFC3339" }`

## Code Layout
- `web/static/index.html`: Main SPA layout containing `#view-cameras`, `#camera-detail`, `#view-redbida`, etc.
- `web/static/app.js`: Camera inventory rendering, grid/table view switcher, quick actions, camera detail 7 tabs, bulk wizard, NVR management.
- `web/static/ui-core.js`: UI helper utilities, modal managers, live preview controllers, notification toasts.
- `web/static/redbida.js`: RedBida SPA logic, 15-key Inspector, 8-Gradient Palette, 20-Tab INI Editor, Smart Hashtags generator, Key management table.
- `web/static/style.css`: Glassmorphism design tokens (`--glass-*`), responsive grid layouts, card styles, glowing badges, animations.
- `internal/server/`: HTTP server, REST endpoints, session auth, rate limiting.
- `internal/camera/`: Camera interface, capabilities, and device abstraction.
- `internal/redbida/`: Key catalog, validation rules, MQTT broker client.
- `tests/ui/`: Playwright E2E UI test suites.
