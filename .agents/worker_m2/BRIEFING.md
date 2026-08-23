# BRIEFING — 2026-08-23T16:54:40Z

## Mission
Implement Milestone M2: Shinobi Go Client & Full Management Engine (Requirement R2) with manual push/pull sync endpoints and web UI tab.

## 🔒 My Identity
- Archetype: teamwork_preview_worker
- Roles: implementer, qa, specialist
- Working directory: /home/ksp/ksp-camera-auto/.agents/worker_m2/
- Original parent: 9f1b4f13-68d2-4737-bba3-0b3ede306ce1
- Milestone: M2: Shinobi Go Client & Full Management Engine

## 🔒 Key Constraints
- Pure Go implementation (CGO_ENABLED=0 compatible)
- NO automatic background sync loop between ksp-cam and Shinobi
- Manual sync triggers:
  1. Push / Export: `POST /api/shinobi/sync-to-shinobi` (cameras.yaml -> Shinobi monitors)
  2. Pull / Import: `POST /api/shinobi/sync-from-shinobi` (Shinobi monitors -> cameras.yaml)
- Web UI has distinct buttons for both actions with clear user feedback
- DO NOT CHEAT. Genuine implementations only.

## Current Parent
- Conversation ID: 9f1b4f13-68d2-4737-bba3-0b3ede306ce1
- Updated: 2026-08-23T16:54:40Z

## Task Summary
- **What to build**:
  1. `internal/shinobi/`:
     - Models: `Monitor`, `MonitorConfig`, `MonitorDetails`, `Video`, `SyncReport`, `ShinobiStatus`
     - Client: `NewClient(apiURL, apiKey, groupKey string) *Client`
     - Methods: `ListMonitors`, `GetMonitor`, `AddMonitor`, `EditMonitor`, `DeleteMonitor`, `ChangeMonitorState`, `GetVideos`, `Status`
     - Manual Sync Engine: `SyncToShinobi`, `SyncFromShinobi`, `DeviceToMid`, `BuildMonitorConfig`
     - Unit tests in `internal/shinobi/client_test.go` with mock server
  2. `internal/server/`:
     - Registered routes: `/api/shinobi/status`, `/api/shinobi/monitors`, `/api/shinobi/sync-to-shinobi`, `/api/shinobi/sync-from-shinobi`, `/api/shinobi/videos`
     - Server struct uses `cfg.Shinobi` to initialize `*shinobi.Client`
     - Tests in `internal/server/api_shinobi_test.go`
  3. `web/static/`:
     - Shinobi management tab in `index.html` & `app.js`
     - Monitor cards, state toggle buttons, modal for Add/Edit/View details
     - Manual sync buttons: "Đồng bộ từ KSP-Cam sang Shinobi" & "Đồng bộ từ Shinobi về KSP-Cam"
     - Help documentation in `docs/help/shinobi-nvr.md` and indexed in `web/static/help/help-index.json`
- **Success criteria**:
  - `go test ./...` and `go test -v ./internal/shinobi/...` pass 100%
  - `go vet ./...` clean
  - `make docs-check` pass 100%
  - `make build-all` creates `kspcam-linux-amd64`, `kspcam-linux-armv7`, `kspcam-linux-arm64`

## Change Tracker
- **Files created**:
  - `internal/shinobi/types.go`
  - `internal/shinobi/client.go`
  - `internal/shinobi/sync.go`
  - `internal/shinobi/client_test.go`
  - `internal/server/api_shinobi.go`
  - `internal/server/api_shinobi_test.go`
  - `docs/help/shinobi-nvr.md`
- **Files modified**:
  - `internal/server/server.go`
  - `web/static/index.html`
  - `web/static/app.js`
  - `web/static/style.css`
  - `web/static/help/help-index.json`
- **Build status**: Pass 100%
- **Pending issues**: None

## Quality Status
- **Build/test result**: Pass (all packages pass)
- **Lint status**: Clean
- **Tests added/modified**: `internal/shinobi/client_test.go`, `internal/server/api_shinobi_test.go`

## Loaded Skills
- None
