# Project: ksp-camera-auto — Shinobi NVR Management & Embedded MCP Server

Current source-verified architecture snapshot: [docs/CODEBASE-KNOWLEDGE.md](docs/CODEBASE-KNOWLEDGE.md).

## Architecture
`ksp-camera-auto` (`kspcam`) is a Go-based camera automation, bulk configuration, Shinobi NVR management, and embedded AI MCP tool server.
- **Entrypoint**: `cmd/kspcam/main.go` (supports web server on `:2028`, `--mcp` Stdio mode, `--config`, `--version`)
- **Configuration & Storage**: `internal/config/` (AES-GCM encrypted YAML store, Shinobi config, MCP config)
- **Web & API Layer**: `internal/server/` (Cookie auth, REST API, SSE streaming, snapshot cache, Shinobi management REST routes `/api/shinobi/*`, SSE MCP endpoint `/mcp`)
- **Shinobi Management**: `internal/shinobi/` (Pure Go client for Shinobi NVR REST API, monitors CRUD, stream states, videos, manual trigger 2-way sync engine)
- **Embedded MCP Server**: `internal/mcp/` (JSON-RPC 2.0 protocol engine, Stdio transport, HTTP/SSE transport with API key auth, 24 tool definitions across Inventory, Config/Bulk, Discovery/Diagnosis, and Shinobi domains)
- **Core Orchestration**: `internal/bulk/` (Sequential task execution, credential testing)
- **Camera Abstraction Layer**: `internal/camera/` (Unified `Camera` interface, capability type assertions, Probe -> Apply -> Read-Back verification)
- **Protocol Implementations**: `internal/dahua/`, `internal/isapi/`, `internal/hik/`, `internal/hiksdk/`, `internal/tiandy/`
- **Discovery**: `internal/discovery/` (ONVIF 3702, Dahua 37810, SADP 37020, Nmap TCP scan)
- **Embedded UI**: `web/static/` (Embedded HTML/JS/CSS single-file binary distribution via `go:embed static`, Shinobi management tab with manual sync buttons)
- **Ansible Automation**: `/build/armbian-build/ansible/playbook/roles/app_ksp_bida/` on controller `172.16.5.180` (automated Shinobi user check, super admin fallback, IP-restricted API key generation, and config writing)

## Feature Inventory
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| 1 | R1.1 Ansible Role Upgrade (`app_ksp_bida`) | Automated Shinobi user check, super admin registration, 127.0.0.1 full-capability API Key generation | M1 | ORIGINAL_REQUEST.md §R1 |
| 2 | R1.2 Config Automation & Go Structs | Write `shinobi` section into `/opt/ksp-cam/config.yaml`, Go struct in `internal/config/config.go` with zero hardcoded passwords | M1 | ORIGINAL_REQUEST.md §R1 |
| 3 | R2.1 Pure Go Shinobi REST Client | `internal/shinobi` package with `ListMonitors`, `AddMonitor`, `EditMonitor`, `DeleteMonitor`, `ChangeMonitorState`, `GetVideos` | M2 | ORIGINAL_REQUEST.md §R2 |
| 4 | R2.2 Manual Trigger 2-Way Sync Engine | Push `cameras.yaml` -> Shinobi monitors & Pull Shinobi monitors -> `cameras.yaml` (No automatic background loop) | M2 | ORIGINAL_REQUEST.md §R2, §Follow-up |
| 5 | R2.3 Shinobi Server REST Endpoints | `/api/shinobi/status`, `/api/shinobi/monitors`, `/api/shinobi/sync-to-shinobi`, `/api/shinobi/sync-from-shinobi`, `/api/shinobi/videos` | M2 | ORIGINAL_REQUEST.md §R2, §Follow-up |
| 6 | R2.4 Embedded Web UI Shinobi Management | New Shinobi tab in Web UI with monitor status cards, stream toggle, manual push/pull sync buttons | M2 | ORIGINAL_REQUEST.md §R2, §Follow-up |
| 7 | R3.1 MCP Protocol Engine & Transports | JSON-RPC 2.0 MCP `2024-11-05` server with Stdio (`kspcam --mcp`) and HTTP/SSE (`/mcp` on `:2028`) with API key security | M3 | ORIGINAL_REQUEST.md §R3 |
| 8 | R3.2 Camera Inventory MCP Tools | `kspcam_list_cameras`, `kspcam_upsert_camera`, `kspcam_delete_camera`, `kspcam_probe_camera` | M3 | ORIGINAL_REQUEST.md §R3 |
| 9 | R3.3 Camera Config & Bulk MCP Tools | `kspcam_apply_profile`, `kspcam_set_channel_name`, `kspcam_set_osd`, `kspcam_reboot_camera`, `kspcam_change_password` | M3 | ORIGINAL_REQUEST.md §R3 |
| 10 | R3.4 Discovery & Diagnosis MCP Tools | `kspcam_scan_lan`, `kspcam_try_password`, `kspcam_wifi_scan`, `kspcam_get_network`, `kspcam_get_nvr_health`, `kspcam_get_recordings`, `kspcam_get_snapshot` | M3 | ORIGINAL_REQUEST.md §R3 |
| 11 | R3.5 Shinobi Management MCP Tools | `shinobi_list_monitors`, `shinobi_add_monitor`, `shinobi_edit_monitor`, `shinobi_delete_monitor`, `shinobi_sync_to_shinobi`, `shinobi_sync_from_shinobi`, `shinobi_change_monitor_state`, `shinobi_get_videos` | M3 | ORIGINAL_REQUEST.md §R3, §Follow-up |
| 12 | R4.1 Comprehensive Unit Tests | Unit tests for `internal/shinobi` (mock server) and `internal/mcp` (stdio & SSE transports, tool execution) | M4 | ORIGINAL_REQUEST.md §R4 |
| 13 | R4.2 Documentation & Help Articles | Update `GEMINI.md`, `AGENTS.md`, and generate help documentation via `make docs` | M4 | ORIGINAL_REQUEST.md §R4 |
| 14 | R4.3 Static Multi-Arch Build & Quality Gates | `make build-all` (amd64, armv7, arm64), `go test ./...`, `make docs-check` pass 100% | M4 | ORIGINAL_REQUEST.md §R4 |
| 15 | R4.4 Live Remote Deployment Validation | Deploy to `inut_204_63` via Ansible `make ksp-bida` and verify live Shinobi API + MCP Server operations | M4 | ORIGINAL_REQUEST.md §R4 |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | M1: Ansible Automated Shinobi Provisioning & Config | `app_ksp_bida` on `172.16.5.180`, `internal/config/` (Shinobi & MCP structs) | none | DONE |
| 2 | M2: Shinobi Go Client & Full Management Engine | `internal/shinobi/`, `internal/server/` Shinobi routes, Web UI Shinobi tab & manual sync buttons | M1 | DONE |
| 3 | M3: Embedded MCP Server in `kspcam` | `internal/mcp/`, Stdio & SSE transports, 23 tool definitions, `cmd/kspcam/main.go`, `internal/server/` integration | M1, M2 | DONE |
| 4 | M4: Tests, Documentation, Multi-Arch Build & Remote Validation | `internal/shinobi/*_test.go`, `internal/mcp/*_test.go`, `GEMINI.md`, `AGENTS.md`, `make docs`, `make build-all`, `inut_204_63` deploy test | M1, M2, M3 | DONE |

## Interface Contracts
### `internal/config/config.go`
- `ShinobiConfig`: `APIURL string (yaml:"api_url")`, `APIKey string (yaml:"api_key")`, `GroupKey string (yaml:"group_key")`
- `MCPConfig`: `Enabled bool (yaml:"enabled")`, `APIKey string (yaml:"api_key")`, `AllowUnauthenticatedLoopback bool (yaml:"allow_unauthenticated_loopback")`

### `internal/shinobi/client.go`
- `Client`: `NewClient(apiURL, apiKey, groupKey string) *Client`
- `ListMonitors(ctx context.Context) ([]Monitor, error)`
- `AddMonitor(ctx context.Context, mon MonitorConfig) error`
- `EditMonitor(ctx context.Context, mid string, mon MonitorConfig) error`
- `DeleteMonitor(ctx context.Context, mid string) error`
- `ChangeMonitorState(ctx context.Context, mid, state string) error`
- `GetVideos(ctx context.Context, mid string, limit int) ([]Video, error)`
- `SyncToShinobi(ctx context.Context, inv *config.Inventory) (*SyncReport, error)`
- `SyncFromShinobi(ctx context.Context, inv *config.Inventory) (*SyncReport, error)`

### `internal/mcp/server.go`
- `Server`: `NewServer(cfg *config.Config, inv *config.Inventory, shinobiClient *shinobi.Client) *Server`
- `RunStdio(ctx context.Context) error`
- `HTTPHandler() http.Handler`

## Code Layout
- `cmd/kspcam/main.go`: Flags (`--mcp`), Stdio mode initiation, config loader
- `internal/config/config.go`: `ShinobiConfig`, `MCPConfig` structs and defaults
- `internal/shinobi/`: Go Shinobi REST client, sync logic, data models
- `internal/shinobi/client_test.go`: Shinobi mock test suite
- `internal/mcp/`: Server, JSON-RPC 2.0 engine, stdio/sse transports, tool handlers
- `internal/mcp/server_test.go`: MCP server and tools test suite
- `internal/server/`: HTTP route handlers for `/api/shinobi/*` and `/mcp`
- `web/static/`: Embedded Web UI Shinobi management tab and manual sync buttons
- `GEMINI.md`, `AGENTS.md`, `docs/help/`: Architecture, protocol, and help articles
