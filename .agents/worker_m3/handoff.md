# Handoff Report: Milestone M3 — Embedded MCP Server in `kspcam`

**Agent**: teamwork_preview_worker (`worker_m3`)  
**Milestone**: M3 (Requirement R3: Embedded MCP Server)  
**Date**: 2026-08-23  

---

## 1. Observation

1. **Protocol & Specification**:
   - The MCP specification (`2024-11-05` / `2024-10-07`) over JSON-RPC 2.0 requires support for `initialize`, `notifications/initialized`, `ping`, `tools/list`, and `tools/call`.
   - Dual transport requirements:
     - Stdio transport (`kspcam --mcp`): Newline-delimited JSON-RPC over stdin/stdout with `log.SetOutput(os.Stderr)` preventing stream corruption.
     - HTTP/SSE transport (`/mcp` and `/mcp/messages` on web server port `:2028`): Streaming downstream events over SSE (`event: endpoint`, `event: message`) and ingesting JSON-RPC calls over POST, protected by API Key authentication and loopback allowances.

2. **Implemented Package Structure (`internal/mcp/`)**:
   - `internal/mcp/types.go`: JSON-RPC 2.0 requests, responses, errors, notifications, MCP handshake types, Tool definitions, and ToolResult constructors (`NewTextResult`, `NewJSONResult`, `NewErrorResult`, `NewImageResult`).
   - `internal/mcp/registry.go`: Thread-safe registry mapping tool names to schemas and handler functions (`ToolHandler`).
   - `internal/mcp/server.go`: Server engine handling `initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`, and session state.
   - `internal/mcp/stdio.go`: Stdio runner with stdin scanner (up to 8MB buffer) and stdout synchronization.
   - `internal/mcp/sse.go`: HTTP handler for `GET /mcp` (SSE stream), `POST /mcp/messages` (session messages), and `POST /mcp` (stateless direct JSON-RPC) with API key checking (`X-MCP-Key`, `Authorization: Bearer`, or `?key=`).
   - `internal/mcp/tools_camera.go`: Camera inventory tools (`kspcam_list_cameras`, `kspcam_upsert_camera`, `kspcam_delete_camera`, `kspcam_probe_camera`).
   - `internal/mcp/tools_config.go`: Camera config tools (`kspcam_apply_profile`, `kspcam_set_channel_name`, `kspcam_set_osd`, `kspcam_reboot_camera`, `kspcam_change_password`).
   - `internal/mcp/tools_discovery.go`: Discovery & Diagnosis tools (`kspcam_scan_lan`, `kspcam_try_password`, `kspcam_wifi_scan`, `kspcam_get_network`, `kspcam_get_nvr_health`, `kspcam_get_recordings`, `kspcam_get_snapshot`).
   - `internal/mcp/tools_shinobi.go`: Shinobi management tools (`shinobi_list_monitors`, `shinobi_add_monitor`, `shinobi_edit_monitor`, `shinobi_delete_monitor`, `shinobi_sync_to_shinobi`, `shinobi_sync_from_shinobi`, `shinobi_sync_inventory`, `shinobi_change_monitor_state`, `shinobi_get_videos`).

3. **System Integration**:
   - `cmd/kspcam/main.go`: Added `--mcp` CLI flag to run the Stdio server.
   - `internal/server/server.go`: Initialized `mcp.Server` in `New()` and registered `/mcp`, `/mcp/`, and `/mcp/messages` HTTP routes.
   - `docs/help/mcp-server.md`: Created help article covering MCP server features, routes, and authentication.
   - `PROJECT.md`: Updated Milestone M3 status to `DONE`.

---

## 2. Logic Chain

1. **Safety & Zero-Dependency**:
   - The MCP server is implemented purely using standard Go library primitives (`encoding/json`, `net/http`, `sync`, `bufio`, `crypto/subtle`, `crypto/rand`).
   - Maintains full static binary build compatibility (`CGO_ENABLED=0`) across `amd64`, `armv7`, and `arm64`.

2. **Sequential Hardware Safety**:
   - Bulk operations (`kspcam_apply_profile`) and credential testing (`kspcam_try_password`) directly utilize `internal/bulk.Apply` and `internal/bulk.TryPasswords`, guaranteeing sequential execution without overloading device encoders or switch buffers.

3. **Manual Trigger Sync Compliance**:
   - Implemented distinct manual sync tools:
     - `shinobi_sync_to_shinobi`: Push / Export `cameras.yaml` -> Shinobi monitors.
     - `shinobi_sync_from_shinobi`: Pull / Import Shinobi monitors -> `cameras.yaml`.
     - `shinobi_sync_inventory`: Optional bidirectional helper with explicit `direction` parameter.

4. **Security & Transport**:
   - The SSE transport supports `X-MCP-Key` header, `Authorization: Bearer <key>`, and URL query parameter `?key=`.
   - `AllowUnauthenticatedLoopback: true` allows local automation (e.g., 127.0.0.1 AI agents) to connect seamlessly while rejecting unauthenticated remote connections.

---

## 3. Caveats

- For tools requiring physical camera hardware (`kspcam_probe_camera`, `kspcam_wifi_scan`, `kspcam_get_snapshot`, etc.), device connectivity depends on network reachability and valid credentials in `cameras.yaml`. If unreachable, standard error responses (`isError: true` or JSON-RPC error codes) are returned.
- Shinobi management tools require Shinobi to be configured in `config.yaml` (`shinobi.api_url`, `shinobi.api_key`, `shinobi.group_key`).

---

## 4. Conclusion

Milestone M3 (Requirement R3) is completely implemented and verified:
- Pure Go Model Context Protocol (MCP) server engine embedded into `kspcam`.
- Stdio transport (`kspcam --mcp`) tested and functional with strict logging isolation.
- HTTP / SSE transport (`/mcp` and `/mcp/messages`) fully integrated and tested.
- 24 standardized MCP tools implemented and unit-tested across all 4 operational domains.
- Multi-architecture static builds (`make build-all`), linter (`make vet`), docs validation (`make docs-check`), and all project unit tests pass 100%.

---

## 5. Verification Method

To independently verify the implementation:

1. **Run MCP unit tests**:
   ```bash
   go test -v ./internal/mcp/...
   ```

2. **Run all project tests**:
   ```bash
   go test -count=1 ./...
   ```

3. **Run code validation and docs checks**:
   ```bash
   make vet
   make docs-check
   ```

4. **Build all static binaries**:
   ```bash
   make build-all
   ```

5. **Test Stdio MCP mode live**:
   ```bash
   echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ./dist/kspcam-linux-amd64 --mcp
   echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"kspcam_list_cameras","arguments":{}}}' | ./dist/kspcam-linux-amd64 --mcp
   ```
