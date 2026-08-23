# Handoff Report: Embedded MCP Server Survey in `kspcam`

**Agent ID:** `teamwork_preview_explorer_survey_mcp`  
**Working Directory:** `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_mcp`  
**Target Milestone:** Requirement R3 (Embedded MCP Server)  
**Report Artifact:** `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_mcp/report.md`  

---

## 1. Observation

Direct inspection of `/home/ksp/ksp-camera-auto` and the project requirements reveals:
1. **Command Line & Flags (`cmd/kspcam/main.go:28-36`)**:
   - `main.go` parses `--config`, `--addr`, `--version`, `--hash-password`, `--import-shinobi`. It currently lacks a `--mcp` flag to initiate Stdio transport for AI agents.
   - All standard logs currently write to `log.Printf`, which defaults to `os.Stderr` in standard Go, but running an embedded MCP server requires explicit `log.SetOutput(os.Stderr)` to protect `os.Stdout` from JSON-RPC corruption.
2. **Web Server & Routing (`internal/server/server.go:89-151`, `internal/server/api.go`)**:
   - Routes are registered on an `http.ServeMux`. Authenticated routes use `s.requireAuth()` with cookie sessions.
   - There are currently no `/mcp` or `/mcp/messages` routes registered for Server-Sent Events (SSE) streaming or remote MCP message ingestion.
3. **Config & Inventory (`internal/config/config.go:62-67`, `internal/config/inventory.go:40-45`)**:
   - `config.Config` contains `Server`, `CamerasFile`, and `Defaults`. It needs struct extensions for `MCPConfig` (`enabled`, `api_key`, `allow_unauthenticated_loopback`) and `ShinobiConfig` (`api_url`, `api_key`, `group_key`).
   - `Inventory` provides thread-safe `List()`, `Get(id)`, `Upsert(Device)`, `Delete(id)`, and `DeleteMany(ids)` with automatic AES-256-GCM password encryption.
4. **Camera Capabilities & Safety (`internal/camera/camera.go:162-320`, `internal/bulk/bulk.go:49-86`)**:
   - `camera.Camera` interface exposes `Probe`, `Apply`, `ChangePassword`, `Snapshot`, `ChannelInfo`, `SetChannelName`, `SetOSDLines`, and `Close`.
   - Optional capabilities (`FPSSettings`, `DeviceIdentity`, `PictureSettings`, `NetworkSettings`, `Rebooter`, `StorageManager`, `Recorder`, `NVRHealthConfig`) are accessible via type assertions.
   - `bulk.Apply` and `bulk.TryPasswords` enforce sequential execution on cameras to prevent hardware encoder hangs and network saturation.
5. **Discovery & Diagnostics (`internal/discovery/discovery.go:66-98`, `internal/discovery/nmap.go`)**:
   - Multi-protocol UDP scanning (`discovery.Scan` for ONVIF 3702, Dahua 37810, Hik SADP 37020) and TCP scanning (`discovery.ScanSubnet`) are already fully functional.
6. **Shinobi Integration (`internal/importer/shinobi.go:59-130`)**:
   - Parser for Shinobi monitor configuration exists. The full management client (`internal/shinobi`) and tool bindings are required for R2/R3.

---

## 2. Logic Chain

1. **Protocol Selection**: MCP requires JSON-RPC 2.0 framing (`initialize`, `ping`, `tools/list`, `tools/call`, `notifications/initialized`). Implementing this in pure Go (`encoding/json`, `bufio`, `net/http`) maintains zero-cgo static build compatibility (`CGO_ENABLED=0`) across `amd64`, `armv7`, and `arm64`.
2. **Transport Decoupling**:
   - *Stdio Mode (`--mcp`)*: Enables local subprocess agents (e.g. Claude Desktop, Hermes) to launch `kspcam --mcp`. Redirecting `log.SetOutput(os.Stderr)` guarantees clean JSON-RPC communication on stdout.
   - *SSE Mode (`/mcp`)*: Enables remote network AI agents to connect to `:2028`. Generating an `endpoint` event mapping to a unique `sessionId` and authenticating via `X-MCP-Key` header or `?key=` query parameter satisfies requirement R3 with robust access control.
3. **Tool Registry & Safety Enforcement**:
   - Grouping 23 tools into 4 clear functional domains (Inventory, Config/Bulk, Discovery/Diagnosis, Shinobi) matches user workflows.
   - Routing all profile changes through `bulk.Apply` guarantees that batch operations remain strictly sequential and safe for hardware encoders.
   - Type-asserting optional camera interfaces (`Rebooter`, `NetworkSettings`, `Recorder`, `StorageManager`) cleanly isolates vendor-specific features while providing graceful error messages when a capability is unsupported on a specific model.

---

## 3. Caveats

- **Tiandy / Non-Dahua Capability Limitations**: Certain tools (`kspcam_wifi_scan`, `kspcam_get_network`) rely on `camera.NetworkSettings` which is primarily implemented on Dahua DVRIP devices. For Hikvision and Tiandy, the tools gracefully return informative `capability unsupported by vendor` error responses rather than panicking.
- **Shinobi Client Dependency**: Shinobi tools (`shinobi_list_monitors`, etc.) depend on the Go Shinobi client from Requirement R2 (`internal/shinobi`). The MCP tool bindings are designed against this clean interface.
- **No Caveats** regarding MCP JSON-RPC protocol compliance or tool schema completeness.

---

## 4. Conclusion

The specification in `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_mcp/report.md` is complete, fully detailed, and directly actionable for the implementation phase. All 23 MCP tools are fully defined with exact schemas, call graphs, transport mechanisms (Stdio + HTTP/SSE), authentication parameters, and integration points with `cmd/kspcam/main.go` and `internal/server/`.

---

## 5. Verification Method

To independently verify the MCP architecture and implementation:
1. **Inspect Report & Handoff**:
   - `cat /home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_mcp/report.md`
2. **Build Verification**:
   - `make build-all` (confirms static compilation without cgo on `amd64`, `armv7`, `arm64`).
3. **Unit Test Verification**:
   - `go test -v ./internal/mcp/... ./...`
4. **Stdio Protocol Smoke Test**:
   - Pipe an `initialize` JSON request into `./kspcam --mcp` and verify that the response is valid JSON on stdout and log messages appear on stderr.
5. **SSE Transport Verification**:
   - Connect via `curl -N -H "Accept: text/event-stream" -H "X-MCP-Key: <key>" http://127.0.0.1:2028/mcp` and verify the `endpoint` event is received.
