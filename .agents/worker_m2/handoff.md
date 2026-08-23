# Handoff Report: Milestone M2 — Shinobi Go Client & Full Management Engine (Requirement R2)

## 1. Observation
- Created pure Go package `internal/shinobi/`:
  - `types.go`: Data models `MonitorConfig`, `MonitorDetails`, `Monitor` (with custom stringified/nested JSON `ParseDetails()`), `Video`, `SyncReport`, `ShinobiStatus`.
  - `client.go`: `NewClient(apiURL, apiKey, groupKey string) *Client`, `ListMonitors`, `GetMonitor`, `AddMonitor`, `EditMonitor`, `DeleteMonitor`, `ChangeMonitorState` (supports `start`, `stop`, `record`, `idle`, `restart`), `GetVideos`, `Status`.
  - `sync.go`: Manual trigger bi-directional sync engine:
    - `SyncToShinobi(ctx, inv)`: Converts inventory devices to Shinobi monitors with RTSP URLs per vendor (Dahua `/cam/realmonitor`, Hikvision `/Streaming/Channels`, Tiandy), sets `vcodec: "copy"` (0% CPU transcoding), and tracks `{Created, Updated, Unchanged, Errors}`.
    - `SyncFromShinobi(ctx, inv)`: Pulls Shinobi monitors, parses RTSP URLs, heuristics for vendor detection and NVR channels, preserves existing NVR fallback fields, and imports new devices into `config.Inventory`.
  - `client_test.go`: Comprehensive unit tests verifying CRUD operations, monitor states, error handling, video retrieval, and bi-directional sync with `httptest.Server`.
- Updated `internal/server/`:
  - `server.go`: Initialized `*shinobi.Client` in `Server` struct using `cfg.Shinobi`.
  - Registered authenticated REST API routes:
    - `GET /api/shinobi/status`
    - `GET /api/shinobi/monitors`
    - `POST /api/shinobi/monitors` (action dispatcher for add, edit, delete, state)
    - `POST /api/shinobi/sync-to-shinobi` (Push sync trigger)
    - `POST /api/shinobi/sync-from-shinobi` (Pull sync trigger)
    - `GET /api/shinobi/videos`
  - `api_shinobi.go`: Handlers for all Shinobi endpoints with error reporting.
  - `api_shinobi_test.go`: Unit tests for server routes, authentication, push/pull sync endpoints, and state changes.
- Updated `web/static/`:
  - `index.html`: Added `#view-shinobi` section with API status cards, monitor table, Add/Edit modal, and Video recordings viewer modal. Added two distinct manual sync trigger buttons:
    - Button 1: "Đồng bộ từ KSP-Cam sang Shinobi" (`#shinobi-sync-to-btn` -> `POST /api/shinobi/sync-to-shinobi`)
    - Button 2: "Đồng bộ từ Shinobi về KSP-Cam" (`#shinobi-sync-from-btn` -> `POST /api/shinobi/sync-from-shinobi`)
  - `app.js`: Added `ICONS.video`, added `shinobi` to `NAV_ITEMS`, route handling in `setRoute()`, render and event handling logic (`renderShinobiView`, `loadShinobiStatus`, `loadShinobiMonitors`, `shinobiSetState`, `shinobiDeleteMonitor`, sync trigger button handlers with `showToast`).
  - `style.css`: Added badge variant classes for status pills.
  - `docs/help/shinobi-nvr.md`: Added documentation for Shinobi NVR management covering all routes and `#shinobi` UI hash.
  - `web/static/help/help-index.json`: Rebuilt via `make docs` covering all 23 help articles.

## 2. Logic Chain
1. User requirement specifically mandated **NO automatic background sync loop** between ksp-cam and Shinobi to avoid unintentional overwrites and resource spikes on constrained devices.
2. We implemented dedicated, separate manual trigger endpoints (`POST /api/shinobi/sync-to-shinobi` and `POST /api/shinobi/sync-from-shinobi`) and matching Web UI buttons.
3. For Shinobi monitors created from inventory, `vcodec: "copy"` and `record_vcodec: "copy"` ensure that Shinobi/FFmpeg performs container remuxing without CPU-intensive video re-encoding, preserving edge box performance.
4. Channel heuristic and multi-monitor detection in `sync.go` preserves NVR channel assignments (`-cN`) and respects existing device configurations.

## 3. Caveats
- Shinobi live API returns the `details` field as an escaped JSON string (`"details": "{\"auto_host\":\"...\"}"`), whereas file export might be a JSON object. Both are handled seamlessly by `Monitor.ParseDetails()`.
- If Shinobi is not configured in `config.yaml` (`api_url` or `api_key` empty), `/api/shinobi/status` returns `{configured: false, connected: false}` without failing, and the Web UI displays an informative status pill.

## 4. Conclusion
Milestone M2 (Requirement R2) is fully implemented, thoroughly tested, and verified against all acceptance criteria. All unit tests pass 100%, `go vet` is clean, `make docs-check` passes 100%, and multi-arch static binaries compile without warnings.

## 5. Verification Method
- Run Go unit tests:
  ```bash
  export PATH=/home/ksp/.goroot/bin:$PATH
  go test -v ./internal/shinobi/...
  go test -v ./internal/server/...
  go test ./...
  ```
- Run linter and docs check:
  ```bash
  export PATH=/home/ksp/.goroot/bin:$PATH
  go vet ./...
  make docs-check
  ```
- Run static multi-arch builds:
  ```bash
  export PATH=/home/ksp/.goroot/bin:$PATH
  make build-all
  ```
