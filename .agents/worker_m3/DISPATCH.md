## 2026-08-23T16:55:17Z
You are teamwork_preview_worker implementing Milestone M3: Embedded MCP Server in kspcam (Requirement R3).
Your working directory is: /home/ksp/ksp-camera-auto/.agents/worker_m3/
Read /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md and /home/ksp/ksp-camera-auto/PROJECT.md before doing anything.
Also read /home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_mcp/report.md for exact JSON schemas, protocol structs, and tool handler specifications.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A reviewer/auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

CRITICAL USER CONSTRAINT:
Sync tools must provide distinct manual triggers:
- `shinobi_sync_to_shinobi` (Push / Export cameras.yaml -> Shinobi monitors)
- `shinobi_sync_from_shinobi` (Pull / Import Shinobi monitors -> cameras.yaml)

Scope & Implementation Details:
1. Create `internal/mcp/`:
   - Protocol types: JSON-RPC 2.0 request, response, error, notification, tool definitions (`Tool`, `ToolInputSchema`, `ToolResult`, `TextContent`).
   - Server engine (`server.go`):
     - Methods: `initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`.
     - Standard tool registry containing all 23 tools across the 4 groups:
       1. Camera Inventory: `kspcam_list_cameras`, `kspcam_upsert_camera`, `kspcam_delete_camera`, `kspcam_probe_camera`
       2. Camera Config & Bulk: `kspcam_apply_profile`, `kspcam_set_channel_name`, `kspcam_set_osd`, `kspcam_reboot_camera`, `kspcam_change_password`
       3. Discovery & Diagnosis: `kspcam_scan_lan`, `kspcam_try_password`, `kspcam_wifi_scan`, `kspcam_get_network`, `kspcam_get_nvr_health`, `kspcam_get_recordings`, `kspcam_get_snapshot`
       4. Shinobi Management: `shinobi_list_monitors`, `shinobi_add_monitor`, `shinobi_edit_monitor`, `shinobi_delete_monitor`, `shinobi_sync_to_shinobi`, `shinobi_sync_from_shinobi`, `shinobi_change_monitor_state`, `shinobi_get_videos`
   - Transports:
     - Stdio transport (`stdio.go`): `RunStdio(ctx context.Context) error`. Ensures `log.SetOutput(os.Stderr)` so `os.Stdout` only receives valid JSON-RPC frames.
     - SSE transport (`sse.go`): `Handler() http.Handler`. Handles `GET /mcp` (SSE stream with `event: endpoint`), `POST /mcp/messages` with session mapping, and API Key authorization (`X-MCP-Key` header or `?key=` query param, respecting `AllowUnauthenticatedLoopback`).
   - Tool execution handlers (`tools.go`, `tools_camera.go`, `tools_shinobi.go`, `tools_discovery.go`): connecting tool calls to `config.Inventory`, `bulk.Apply`, `camera.Open`, `shinobi.Client`, `discovery.Scan`, etc.
   - Comprehensive unit tests in `internal/mcp/server_test.go` and `internal/mcp/tools_test.go` testing Stdio request/response, SSE endpoints, authentication, and all tool handlers.

2. Update `cmd/kspcam/main.go`:
   - Add `--mcp` flag: `mcpFlag := flag.Bool("mcp", false, "Start MCP (Model Context Protocol) server over Stdio")`.
   - If `--mcp` is set:
     - Redirect standard logger to `os.Stderr`.
     - Load config and inventory.
     - Initialize `shinobi.Client` (if configured in config.yaml).
     - Start `mcp.NewServer(cfg, inv, shinobiClient).RunStdio(ctx)`.
     - Exit cleanly when context terminates.

3. Update `internal/server/server.go`:
   - Initialize `mcp.Server` in `NewServer()`.
   - Register HTTP handlers for `/mcp` and `/mcp/messages`.
   - Add tests for `/mcp` and `/mcp/messages` routes.

4. Run tests and verification:
   - Run `go test -v ./internal/mcp/...` and `go test ./...`.
   - Run `go vet ./...`, `make check`, and `make build-all`.

Write your handoff report to `/home/ksp/ksp-camera-auto/.agents/worker_m3/handoff.md` and notify parent when complete via send_message.
