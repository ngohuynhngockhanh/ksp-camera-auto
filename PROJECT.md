# Project: ksp-camera-auto RedBida & Onboarding MCP Tools Suite

## Architecture

`ksp-camera-auto` features an embedded Model Context Protocol (MCP) server that provides AI assistants with standardized tools over dual transports: Stdio JSON-RPC 2.0 (CLI `--mcp`) and HTTP/SSE (`:2028/mcp`).

### Subsystem Interaction & Data Flow

```
[AI Assistant / Client]
       │
       ├── Stdio (JSON-RPC 2.0 via CLI --mcp)
       └── HTTP / SSE (:2028/mcp, with Loopback bypass / API Key auth)
               │
               ▼
   [internal/mcp: Server & Registry]
               │
       ┌───────┼───────────────────────────┬──────────────────────────┐
       │       │                           │                          │
       ▼       ▼                           ▼                          ▼
 [Camera Tools] [Discovery Tools]    [Shinobi Tools]       [RedBida & Onboarding Tools]
 (kspcam_*)     (kspcam_scan_*)      (shinobi_*)           (redbida_*)
                                                                      │
                                                     ┌────────────────┴────────────────┐
                                                     ▼                                 ▼
                                            [MQTT Broker :12369]             [System Time & NTP]
                                            /private/i_gets & /private/i_sets (timedatectl)
```

## Feature Inventory

| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F1 | `redbida_list_catalog` | Lists all configuration keys in the RedBida / OTA-MQTT catalog with functional groups, risk classifications (editable, confirm-required, protected), data types, and availability. | M1 | ORIGINAL_REQUEST §R1.1 |
| F2 | `redbida_get_keys` | Reads live key values from local `ota-mqtt` broker (`127.0.0.1:12369`) via topic `/private/i_gets` with `{"info": [...]}` and automatic secret masking (`********`). | M1 | ORIGINAL_REQUEST §R1.2 |
| F3 | `redbida_set_keys` | Writes key-value changes to `ota-mqtt` via `/private/i_sets` with `{"info": {...}}` and enforces mandatory read-back verification (up to 3 attempts with exponential backoff). | M1 | ORIGINAL_REQUEST §R1.3 |
| F4 | `redbida_apply_onboarding_preset` | 1-Click Bida Onboarding tool: synthesizes and applies the 15 standard parameters (`ui_title`, `ui_bg` without semicolon, `custom_hashtags` normalized without diacritics, `ui_tabs_links` 20-tab INI, `camera_count`, `toolbar_show_count`, `hls_using_go2rtc`, `button_generate_go2rtc_stream`, `logo_header`, `logo_header_text`, `shinobi_camera_id`, `shinobi_group_key`, `video_config`, `ui_scoreboard`, `ggcode`) with read-back verification. | M1 | ORIGINAL_REQUEST §R1.4 |
| F5 | `redbida_trigger_go2rtc` | Triggers Node-RED (:2023) to generate `/root/go2rtc.yaml` stream configuration by publishing `button_generate_go2rtc_stream: "true"` over `/private/i_sets`. | M1 | ORIGINAL_REQUEST §R1.5 |
| F6 | `redbida_get_time_status` | Checks host system time (RFC 3339) and NTP synchronization status via `timedatectl`. | M1 | ORIGINAL_REQUEST §R1.6 |
| F7 | MCP Server Registration & Dual Transports | Registers all 6 `redbida_*` tools in `internal/mcp/server.go`, wires `redbida.Service` into `NewServer`, ensures smooth operation for both Stdio (`kspcam --mcp`) and HTTP/SSE (`:2028/mcp`) transports. | M2 | ORIGINAL_REQUEST §R2 |
| F8 | Documentation Updates | Updates technical documentation in `docs/help/mcp-server.md`, `docs/help/redbida.md`, `docs/CODEBASE-KNOWLEDGE.md`, and `GEMINI.md` / `AGENTS.md` to detail all 31 MCP tools. | M2 | ORIGINAL_REQUEST §R2 |
| F9 | Comprehensive Unit & JSON-RPC Tests | Implements 100% passing unit tests in `internal/mcp/tools_redbida_test.go` and `internal/mcp/server_test.go` verifying JSON-RPC 2.0 compliance (`initialize`, `tools/list`, `tools/call`). | M3 | ORIGINAL_REQUEST §R3 |
| F10 | Multi-Arch Compilation | Builds static binaries (`CGO_ENABLED=0`) for `linux/amd64`, `linux/arm64`, and `linux/armv7` via `make build-all`. | M3 | ORIGINAL_REQUEST §R3 |
| F11 | Remote Deployment & Live Verification | Deploys ARM64 binary to live edge nodes `inut_204_164` and `inut_204_163` via jump host `root@172.16.5.180`, restarts services, and verifies live MCP tool calls over HTTP/SSE. | M3 | ORIGINAL_REQUEST §R3 |
| F12 | Git Commit & Push | Commits and pushes all code, test, and documentation changes to the remote git repository. | M3 | ORIGINAL_REQUEST §R3 |

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | RedBida & Onboarding MCP Tools Suite | Implement `internal/mcp/tools_redbida.go` with F1-F6: `redbida_list_catalog`, `redbida_get_keys`, `redbida_set_keys`, `redbida_apply_onboarding_preset` (all 15 parameters, 20-tab INI, accent stripping, trailing semicolon removal), `redbida_trigger_go2rtc`, and `redbida_get_time_status`. | none | DONE |
| M2 | MCP Server Integration & Documentation | Integrate new tools in `internal/mcp/server.go` (F7), wire `redbida.Service` into `NewServer`, verify Stdio/SSE modes, and update documentation in `docs/` and `GEMINI.md`/`AGENTS.md` (F8). | M1 | DONE |
| M3 | Testing, Multi-Arch Build, Remote Deployment & Live Verification | Implement unit tests (F9), execute `make build-all` (F10), deploy to `inut_204_164` and `inut_204_163` (F11), execute live MCP tests, and push git commit (F12). | M2 | IN_PROGRESS |

## Interface Contracts

### 1. `redbida_list_catalog`
- **Arguments**: `{ "group"?: string, "editableOnly"?: boolean }`
- **Output**: Array of catalog items with `key`, `group`, `risk`, `type`, `description`, `secret`, `source`.

### 2. `redbida_get_keys`
- **Arguments**: `{ "keys"?: string[], "all"?: boolean }`
- **Output**: Array of `{ key, value, risk, type, secret, verified }`. Secrets masked as `"********"`.

### 3. `redbida_set_keys`
- **Arguments**: `{ "changes": { [key: string]: any }, "confirmed"?: boolean }`
- **Output**: Array of `ChangeResult{ key, meta, oldValue, newValue, changed, acknowledged, readBack, verified, applied, error }`.

### 4. `redbida_apply_onboarding_preset`
- **Arguments**:
  ```json
  {
    "title": "string (required)",
    "cameraCount": "integer (required, 1-20)",
    "bg": "string (optional CSS gradient)",
    "groupKey": "string (optional Shinobi group key)",
    "shinobiToken": "string (optional)",
    "shinobiMonitorToken": "string (optional)",
    "ggcode": "string (optional Google Analytics)",
    "customHashtags": "string (optional override)",
    "dryRun": "boolean (optional)",
    "confirmed": "boolean (optional, default true)"
  }
  ```
- **Synthesized 15 Golden Template Parameters**:
  1. `ui_title`: Quán title (e.g. `"CX King Luxury"`)
  2. `company_name`: Same as `ui_title`
  3. `ui_bg`: CSS gradient background with trailing semicolon stripped (e.g. `"radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )"`)
  4. `custom_hashtags`: Normalized no diacritics + `#<CleanTitle> #BILLIARDSlive #INUTlive #highlightsports`
  5. `ui_tabs_links`: Exactly 20 sections `[C01]` to `[C20]` INI format with `stream_label=Video Trực tiếp\nvid_list_label=Danh sách highlight\nvid_play_label=<ui_title>\nlist_refresh_label=Cập nhật highlight`
  6. `camera_count`: Integer matching `cameraCount`
  7. `toolbar_show_count`: Integer matching `cameraCount`
  8. `hls_using_go2rtc`: `true`
  9. `button_generate_go2rtc_stream`: `true`
  10. `logo_header`: `"https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png"`
  11. `logo_header_text`: `"Billiard Live - Tải clip bàn bida và livestream"`
  12. `shinobi_camera_id`: Primary camera identifier (e.g. `"C01"` or provided `groupKey`)
  13. `shinobi_group_key`: Shinobi group key string (e.g. `"AWU8wJMd2l"`)
  14. `video_config`: `"range=72"`
  15. `ui_scoreboard`: `true`
  16. `ggcode`: Google Analytics measurement code (e.g. `"G-SFSDZPR95Z"`)

### 5. `redbida_trigger_go2rtc`
- **Arguments**: `{}`
- **Output**: `{ "ok": true, "message": "Go2RTC generation triggered via MQTT button_generate_go2rtc_stream" }`

### 6. `redbida_get_time_status`
- **Arguments**: `{}`
- **Output**: `{ "hostTime": string, "hostTimeRFC3339": string, "ntpSynchronized": boolean, "driftThresholdSeconds": 60, "policy": string }`

## Code Layout

- `internal/mcp/types.go` — JSON-RPC 2.0 models & Tool schemas
- `internal/mcp/registry.go` — Tool registry & invocation dispatcher
- `internal/mcp/server.go` — Server lifecycle & tool category registrations
- `internal/mcp/tools_redbida.go` — RedBida & Onboarding MCP tools implementation (NEW)
- `internal/mcp/tools_redbida_test.go` — Unit tests for RedBida tools (NEW)
- `internal/mcp/server_test.go` — JSON-RPC 2.0 integration & tool registration tests
- `internal/redbida/` — Pure Go MQTT client, Catalog, Service, and verification logic
- `docs/help/mcp-server.md` — Help documentation for MCP server
- `docs/help/redbida.md` — Help documentation for RedBida
- `docs/CODEBASE-KNOWLEDGE.md` — Architecture & tools inventory
- `GEMINI.md` / `AGENTS.md` — Second brain & tools table
