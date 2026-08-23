## 2026-08-23T16:48:48Z

You are teamwork_preview_worker implementing Milestone M2: Shinobi Go Client & Full Management Engine (Requirement R2).
Your working directory is: /home/ksp/ksp-camera-auto/.agents/worker_m2/
Read /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md and /home/ksp/ksp-camera-auto/PROJECT.md before doing anything.
Also read /home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_shinobi/report.md for complete details on Shinobi API parameters and JSON schemas.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A reviewer/auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

CRITICAL USER CONSTRAINT:
User requires NO automatic background sync loop between ksp-cam and Shinobi. Every sync action must be manually triggered via separate buttons and dedicated API endpoints:
1. "Đồng bộ từ KSP-Cam sang Shinobi" -> `POST /api/shinobi/sync-to-shinobi` (Push / Export `cameras.yaml` to Shinobi monitors)
2. "Đồng bộ từ Shinobi về KSP-Cam" -> `POST /api/shinobi/sync-from-shinobi` (Pull / Import Shinobi monitors to `cameras.yaml`)

Scope & Implementation Details:
1. `internal/shinobi/`:
   - Data models: `Monitor`, `MonitorConfig`, `MonitorDetails`, `Video`, `SyncReport`.
   - `Client`: `NewClient(apiURL, apiKey, groupKey string) *Client` (with HTTP timeout, context support).
   - CRUD Operations:
     - `ListMonitors(ctx context.Context) ([]Monitor, error)` (`GET /:apiKey/monitor/:groupKey`)
     - `GetMonitor(ctx context.Context, mid string) (*Monitor, error)` (`GET /:apiKey/monitor/:groupKey/:mid`)
     - `AddMonitor(ctx context.Context, mon MonitorConfig) error` (`POST /:apiKey/configureMonitor/:groupKey/:mid`)
     - `EditMonitor(ctx context.Context, mid string, mon MonitorConfig) error` (`POST /:apiKey/configureMonitor/:groupKey/:mid`)
     - `DeleteMonitor(ctx context.Context, mid string) error` (`GET /:apiKey/monitor/:groupKey/:mid/delete`)
     - `ChangeMonitorState(ctx context.Context, mid, state string) error` (`GET /:apiKey/monitor/:groupKey/:mid/:state`) - supports `start`, `stop`, `record`, `idle`, `restart`.
     - `GetVideos(ctx context.Context, mid string, limit int) ([]Video, error)` (`GET /:apiKey/videos/:groupKey/:mid`)
     - `Status(ctx context.Context) (*ShinobiStatus, error)`
   - Manual Sync Engine:
     - `SyncToShinobi(ctx context.Context, inv *config.Inventory) (*SyncReport, error)`: Converts cameras from inventory into Shinobi monitors with RTSP URL (`rtsp://user:pass@host:port/path`), `vcodec: "copy"`, sub-stream if available, audio enabled.
     - `SyncFromShinobi(ctx context.Context, inv *config.Inventory) (*SyncReport, error)`: Converts monitors from Shinobi into inventory cameras (using vendor heuristic from path or type).
   - Unit tests in `internal/shinobi/client_test.go` using `httptest.Server`.

2. `internal/server/`:
   - Register Shinobi REST endpoints in `internal/server/server.go`:
     - `GET /api/shinobi/status`: Return connection status, monitor count, group key, api url.
     - `GET /api/shinobi/monitors`: Return list of monitors.
     - `POST /api/shinobi/monitors`: Add, update, delete, or change monitor state.
     - `POST /api/shinobi/sync-to-shinobi`: Trigger push sync (cameras.yaml -> Shinobi).
     - `POST /api/shinobi/sync-from-shinobi`: Trigger pull sync (Shinobi -> cameras.yaml).
     - `GET /api/shinobi/videos`: Return video recordings.
   - Initialize Shinobi client in `Server` struct using `cfg.Shinobi`.

3. `web/static/`:
   - Add Shinobi navigation tab in `app.js` and `index.html`.
   - Render Shinobi monitor cards, status pills, stream state toggles (start/stop/record).
   - Add two distinct manual trigger sync buttons:
     - Button 1: "Đồng bộ từ KSP-Cam sang Shinobi" (calls `POST /api/shinobi/sync-to-shinobi`)
     - Button 2: "Đồng bộ từ Shinobi về KSP-Cam" (calls `POST /api/shinobi/sync-from-shinobi`)
   - Toast/Alert notifications displaying sync report results.

4. Run tests:
   - Run `go test -v ./internal/shinobi/...` and `go test ./...`.
   - Run `go vet ./...` and `make check` to ensure clean build.
