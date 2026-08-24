# MCP Core Architecture Exploration Report

> **Target Directory**: `/home/ksp/ksp-camera-auto/internal/mcp/`  
> **Investigation Date**: 2026-08-24  
> **Status**: Completed  

---

## 1. Observation

Direct code examination of `internal/mcp/`, `internal/server/`, `internal/redbida/`, and `cmd/kspcam/` revealed the following concrete architectural components:

### 1.1 MCP Core Structs, Types & Wire Protocol (`internal/mcp/types.go`)

- **JSON-RPC 2.0 & Protocol Constants (`types.go:11-23`)**:
  - `JSONRPCVersion = "2.0"`
  - `ProtocolVersion = "2024-11-05"`, `ProtocolVersionOld = "2024-10-07"`
  - Standard error codes:
    - `CodeParseError = -32700`
    - `CodeInvalidRequest = -32600`
    - `CodeMethodNotFound = -32601`
    - `CodeInvalidParams = -32602`
    - `CodeInternalError = -32603`
    - Application codes: `CodeUnauthorized = -32001`, `CodeDeviceUnreachable = -32002`
- **Request / Response Models (`types.go:25-54`)**:
  - `JSONRPCRequest`: `{ "jsonrpc": "2.0", "id": any, "method": string, "params": json.RawMessage }`
  - `JSONRPCResponse`: `{ "jsonrpc": "2.0", "id": any, "result": any, "error": *JSONRPCError }`
  - `JSONRPCNotification`: `{ "jsonrpc": "2.0", "method": string, "params": json.RawMessage }`
  - `JSONRPCError`: `{ "code": int, "message": string, "data": any }`
- **MCP Tool Definition & Result Models (`types.go:104-191`)**:
  - `ToolInputSchema`: `{ "type": "object", "properties": map[string]any, "required": []string, "description": string }`
  - `Tool`: `{ "name": string, "description": string, "inputSchema": ToolInputSchema }`
  - `ContentItem`: `{ "type": "text"|"image", "text": string, "data": string (base64), "mimeType": string }`
  - `ToolResult`: `{ "content": []ContentItem, "isError": bool }`
  - Helper functions:
    - `NewTextResult(text string) ToolResult`: Single text item with `isError: false`.
    - `NewJSONResult(v any) (ToolResult, error)`: Pretty-prints JSON via `json.MarshalIndent(v, "", "  ")` into text content item.
    - `NewErrorResult(errMsg string) ToolResult`: Sets `isError: true` with error description in text content item.
    - `NewImageResult(mimeType, base64Data string) ToolResult`: Base64 encoded payload with MIME type (e.g. `image/jpeg`).

### 1.2 Tool Registry & Execution Dispatcher (`internal/mcp/registry.go`)

- **Registry Struct (`registry.go:19-30`)**:
  ```go
  type ToolHandler func(ctx context.Context, args json.RawMessage) (ToolResult, error)

  type registeredTool struct {
      tool    Tool
      handler ToolHandler
  }

  type Registry struct {
      mu    sync.RWMutex
      tools map[string]registeredTool
  }
  ```
- **Registration & Execution Methods**:
  - `Register(tool Tool, handler ToolHandler)` (`registry.go:33-40`): Stores tool and handler under `tools[tool.Name]` with write lock.
  - `Get(name string) (Tool, ToolHandler, bool)` (`registry.go:43-51`): Thread-safe read lookup.
  - `List() []Tool` (`registry.go:54-69`): Returns tools sorted deterministically in alphabetical order by name (`sort.Strings(names)`).
  - `Call(ctx context.Context, name string, args json.RawMessage) (ToolResult, error)` (`registry.go:72-82`): Looks up tool handler and executes it under context. Returns `NewErrorResult("unknown tool ...")` if name not found.

### 1.3 Server Core & JSON-RPC Lifecycle (`internal/mcp/server.go`)

- **Server Struct (`server.go:16-23`)**:
  ```go
  type Server struct {
      cfg      *config.Config
      inv      *config.Inventory
      shinobi  *shinobi.Client
      registry *Registry
      mu       sync.RWMutex
      sessions map[string]*Session
  }
  ```
- **Initialization (`server.go:26-45`)**:
  - `NewServer(cfg *config.Config, inv *config.Inventory, shinobiClient *shinobi.Client) *Server`
  - Registers 4 existing categories:
    1. `registerCameraInventoryTools(registry, cfg, inv)`
    2. `registerCameraConfigTools(registry, cfg, inv)`
    3. `registerDiscoveryDiagnosisTools(registry, cfg, inv)`
    4. `registerShinobiTools(registry, cfg, inv, shinobiClient)`
- **Request Processing (`server.go:54-127`)**:
  - `ProcessRequest(ctx, req)` routes methods:
    - `"initialize"`: Returns `InitializeResult{ProtocolVersion: "2024-11-05", Capabilities: {Tools: {ListChanged: false}}, ServerInfo: {Name: "kspcam", Version: "1.0.0"}}`.
    - `"notifications/initialized"`: Returns `isNotification = true` (suppresses outgoing response frame).
    - `"ping"`: Returns empty JSON map `{}`.
    - `"tools/list"`: Calls `s.registry.List()`, returns `ToolsListResult{Tools: [...]}`.
    - `"tools/call"`: Unmarshals `ToolCallParams{Name, Arguments}`. If unmarshal fails, returns `-32602` (`CodeInvalidParams`). Dispatches to `s.registry.Call(ctx, params.Name, params.Arguments)`. If handler returns non-empty `ToolResult`, returns `resp.Result = toolResult`. If handler returns `err != nil` with empty content, returns `-32603` (`CodeInternalError`).
    - Default: Returns `-32601` (`CodeMethodNotFound`).
- **Raw Message Processing (`server.go:130-153`)**:
  - `ProcessMessage(ctx, msg)`: Decodes raw JSON bytes into `JSONRPCRequest`. Returns parse error `-32700` if invalid. Encodes response back to JSON bytes.

### 1.4 Transport Channels

#### A. Stdio Mode (`internal/mcp/stdio.go` & `cmd/kspcam/main.go`)
- Executed when `kspcam` is invoked with `--mcp` flag: `kspcam --mcp --config /opt/ksp-cam/config.yaml`.
- Redirects logger: `log.SetOutput(os.Stderr)` (`stdio.go:15`) to keep `stdout` 100% clean for JSON-RPC messages.
- Uses `bufio.Scanner` with an 8MB buffer (`stdio.go:23-24`) reading from `os.Stdin`.
- Thread-safe output writer protected by `sync.Mutex` (`writeMu`) writing `\n`-delimited JSON bytes to `os.Stdout`.
- Listens on `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` (`cmd/kspcam/main.go:62`) for graceful termination.

#### B. HTTP & SSE Mode (`internal/mcp/sse.go` & `internal/server/server.go`)
- Mounted on `:2028` via `s.mux.Handle("/mcp", mcpHandler)` and `s.mux.Handle("/mcp/messages", mcpHandler)`.
- Endpoints:
  1. `GET /mcp`: Initiates Server-Sent Events stream (`text/event-stream`). Generates a 16-byte hex session ID, registers `Session{ID, outgoing: make(chan []byte, 64), created: time.Now()}`, and sends initial handshake event:
     ```http
     event: endpoint
     data: /mcp/messages?sessionId=<sessionId>
     ```
  2. `POST /mcp/messages?sessionId=<sessionId>`: Receives JSON-RPC request for a specific SSE stream. Dispatches via `ProcessMessage`, pushes serialized response to `sess.outgoing` channel, and immediately returns HTTP `202 Accepted` (`{"status":"accepted"}`). The SSE loop reads `sess.outgoing` and pushes `event: message\ndata: <json>\n\n`.
  3. `POST /mcp`: Direct stateless JSON-RPC request. Executes `ProcessMessage` directly and returns HTTP `200 OK` with JSON response.
  4. `OPTIONS /mcp`: CORS preflight handling (`Access-Control-Allow-Origin: *`, headers: `Content-Type, Authorization, X-MCP-Key, X-Session-ID`).
- Authentication (`sse.go:66-110`):
  - If `cfg.AllowUnauthenticatedLoopback == true`, checks client IP (`clientIP(r)`). If loopback (`127.0.0.1`, `::1`, `localhost`), permits unauthenticated access.
  - If API key configured in `cfg.APIKey`: validates via `subtle.ConstantTimeCompare` against:
    - Header `X-MCP-Key: <key>`
    - Header `Authorization: Bearer <key>`
    - Query parameter `?key=<key>` or `?apiKey=<key>`

### 1.5 Catalog of Existing 25 MCP Tools

| STT | Tool Name | Source File | Description |
|---|---|---|---|
| 1 | `kspcam_list_cameras` | `tools_camera.go:16` | Lists inventory cameras filtered by vendor or NVR flag with password masked. |
| 2 | `kspcam_upsert_camera` | `tools_camera.go:77` | Adds/updates camera in `cameras.yaml` with AES-256-GCM encryption. |
| 3 | `kspcam_delete_camera` | `tools_camera.go:243` | Deletes a camera by ID from inventory. |
| 4 | `kspcam_probe_camera` | `tools_camera.go:283` | Live hardware probe for stream profiles, FPS, codec, serial number. |
| 5 | `kspcam_apply_profile` | `tools_config.go:17` | Sequential bulk profile configuration across multiple cameras. |
| 6 | `kspcam_set_channel_name` | `tools_config.go:104` | Changes hardware channel title on camera. |
| 7 | `kspcam_set_osd` | `tools_config.go:178` | Configures 4 lines of OSD text overlay on video. |
| 8 | `kspcam_reboot_camera` | `tools_config.go:258` | Sends remote hardware reboot command to camera/NVR. |
| 9 | `kspcam_change_password` | `tools_config.go:326` | Updates hardware admin password and syncs encrypted `cameras.yaml`. |
| 10 | `kspcam_scan_lan` | `tools_discovery.go:20` | Discovers IP cameras via ONVIF (3702), Dahua (37810), Hik SADP (37020), or Nmap. |
| 11 | `kspcam_try_password` | `tools_discovery.go:103` | Sequentially tests credentials against discovered targets. |
| 12 | `kspcam_wifi_scan` | `tools_discovery.go:202` | Triggers over-the-air Wi-Fi scan on wireless cameras. |
| 13 | `kspcam_get_network` | `tools_discovery.go:267` | Reads network interface config (IP, mask, gateway, DNS, DHCP). |
| 14 | `kspcam_get_nvr_health` | `tools_discovery.go:332` | Reads NVR recording health, storage disks, uptime, and clock drift. |
| 15 | `kspcam_get_recordings` | `tools_discovery.go:441` | Queries video recording segments for date/time range. |
| 16 | `kspcam_get_snapshot` | `tools_discovery.go:530` | Returns live JPEG snapshot as base64 image data. |
| 17 | `shinobi_list_monitors` | `tools_shinobi.go:24` | Lists Shinobi NVR monitors, stream URLs, and recording modes. |
| 18 | `shinobi_add_monitor` | `tools_shinobi.go:49` | Adds new monitor stream to Shinobi NVR. |
| 19 | `shinobi_edit_monitor` | `tools_shinobi.go:209` | Modifies existing Shinobi monitor settings. |
| 20 | `shinobi_delete_monitor` | `tools_shinobi.go:372` | Deletes monitor from Shinobi NVR. |
| 21 | `shinobi_sync_to_shinobi` | `tools_shinobi.go:419` | Pushes `cameras.yaml` inventory to Shinobi NVR. |
| 22 | `shinobi_sync_from_shinobi` | `tools_shinobi.go:445` | Pulls Shinobi monitors into `cameras.yaml`. |
| 23 | `shinobi_sync_inventory` | `tools_shinobi.go:471` | Bidirectional synchronization between inventory and Shinobi. |
| 24 | `shinobi_change_monitor_state`| `tools_shinobi.go:533` | Changes active execution state (`record`, `start`, `stop`, `restart`). |
| 25 | `shinobi_get_videos` | `tools_shinobi.go:581` | Queries recorded video clips from Shinobi NVR. |

### 1.6 Existing RedBida Subsystem Architecture (`internal/redbida/`)

- **MQTT Client (`internal/redbida/mqtt.go`)**:
  - Connects to broker at `127.0.0.1:12369` (or `cfg.Redbida.BrokerHost` / `BrokerPort`).
  - Read topic: `/private/i_gets` with payload `{"info": ["key1", "key2", ...]}` -> receives on `/private/i_gets/ack` with payload `{"info": {"key1": <val>, ...}}`.
  - Write topic: `/private/i_sets` with payload `{"info": {"key1": <val>, ...}}` -> receives on `/private/i_sets/ack` with payload `{"info": {"key1": {"oldValue": <v1>, "newValue": <v2>}}}`.
  - Matches exact keys and rejects retained messages (`msg.Retained() == true`).
- **Catalog & Safety Engine (`internal/redbida/catalog.go`)**:
  - Scans keys from directory `cfg.Redbida.KeyDir` (default `/root/ota-mqtt/change_ok`).
  - Classifies keys into 4 risk tiers: `RiskEditable`, `RiskConfirm`, `RiskProtected`, `RiskUnknown`.
  - Classifies data types: `TypeString`, `TypeNumber`, `TypeBoolean`, `TypeJSON`, `TypeImage`.
  - Masks sensitive secrets (`Secret == true`) returning `"********"`.
- **Service & Verification Engine (`internal/redbida/service.go`)**:
  - `Refresh(ctx, keys)`: Normalizes and fetches keys from broker, cross-checking with catalog.
  - `Apply(ctx, changes, confirmed)`:
    1. Validates key name, editability, and risk tier (confirm-required keys require `confirmed == true`).
    2. Validates type and numeric range boundaries (`numericRules`).
    3. Publishes to MQTT `/private/i_sets` and waits for `/private/i_sets/ack`.
    4. Performs mandatory **read-back verification** (`readBack`) via `/private/i_gets` (up to 3 attempts) to guarantee physical persistence.
    5. Returns `[]ChangeResult{Key, Meta, OldValue, NewValue, Changed, Acknowledged, ReadBack, Verified, Applied, Error}`.
- **Time Status & NTP Sync (`internal/server/api_redbida.go:86-102` & `internal/server/nvr_health.go:437-450`)**:
  - `hostTimeTrusted(ctx)` executes `timedatectl show -p NTPSynchronized --value` with 2-second timeout.
  - Returns `hostTime`, `hostTimeRFC3339`, `ntpSynchronized: bool`, `driftThresholdSeconds: 60`, `policy`.

---

## 2. Logic Chain

1. **Tool Dispatch Consistency**:
   - `internal/mcp/server.go` serves as the single point of dispatch for both Stdio and HTTP/SSE JSON-RPC transports.
   - All tool registration functions (`registerCameraInventoryTools`, `registerCameraConfigTools`, `registerDiscoveryDiagnosisTools`, `registerShinobiTools`) adhere to a clean functional registration pattern `registerXxxTools(r *Registry, cfg *config.Config, ...)` that registers `Tool` schemas and `ToolHandler` callbacks into `r *Registry`.
   - By creating `internal/mcp/tools_redbida.go` with `func registerRedbidaTools(r *Registry, cfg *config.Config, redbidaSvc *redbida.Service)`, the new RedBida tools will seamlessly integrate into the existing registry lifecycle.

2. **Zero-Breaking-Change Dependency Injection**:
   - In `internal/server/server.go:66-75`, `s.redbida` is initialized when `cfg.Redbida.Enabled`.
   - In `cmd/kspcam/main.go:61`, `mcpServer` is instantiated.
   - If `mcp.NewServer` uses an optional variadic argument for `redbidaSvc ...*redbida.Service` (or automatically instantiates `redbida.NewService` when `cfg.Redbida.Enabled` and `redbidaSvc` is not passed), all existing unit tests and callers remain 100% source- and binary-compatible without modifying prior test setup helpers.

3. **Parity with Frontend Onboarding Preset Generator**:
   - `web/static/redbida.js` (`lines 285-351`) and `.agents/skills/camera-naming/SKILL.md` define the exact algorithm for 1-Click Onboarding:
     1. `ui_bg`: Strip trailing semicolon (CSS gradients must not end in `;`).
     2. `custom_hashtags`: Remove Vietnamese accents, strip non-alphanumeric characters, and prepend shop tag with `#<CleanTitle> #BILLIARDSlive #INUTlive #highlightsports`.
     3. `ui_tabs_links`: Generate exactly 20 sections `[C01]` to `[C20]` with `vid_play_label = <ui_title>`.
     4. Set `camera_count` and `toolbar_show_count` to the same integer.
     5. Set standard video parameters: `video_config: "range=72"`, `hls_using_go2rtc: true`, `hls_using_go2rtc_livestream: true`, `hls_using_go2rtc_tiktok: true`, `ui_scoreboard: true`.
     6. Set standard branding: `logo_header: "https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png"`, `logo_header_text: "Billiard Live - Tải clip bàn bida và livestream"`.
     7. Trigger Go2RTC generation: `button_generate_go2rtc_stream: true`.
   - Implementing this logic in Go inside `redbida_apply_onboarding_preset` gives AI agents direct 1-click execution capabilities via MCP.

4. **Error Handling Alignment**:
   - If `redbidaSvc == nil` (e.g. RedBida disabled in `config.yaml`), the tool handler gracefully returns `NewErrorResult("redbida integration is disabled or not configured in config.yaml")` rather than causing runtime nil panics.
   - For `redbida_set_keys` and `redbida_apply_onboarding_preset`, the handler passes `confirmed: true` if confirmed is requested, triggering full read-back verification and returning structured `ChangeResult` lists.

---

## 3. Caveats

1. **Stdio vs HTTP/SSE Isolation**:
   - In Stdio mode, all diagnostic logs must go strictly to `stderr` (`log.SetOutput(os.Stderr)`). `tools_redbida.go` must not write raw output to `os.Stdout` (always return values through `ToolResult`).
2. **Vietnamese Diacritic Transformation in Pure Go**:
   - To keep static cross-compilation lightweight (`CGO_ENABLED=0`) and avoid adding heavy external Unicode dependencies to `go.mod`, `removeVietnameseTones` should be implemented via a comprehensive rune transformation map covering all combined and decomposed Vietnamese vowels (`à-ỹ`, `đ/Đ`).
3. **MQTT Ack Timeout vs Read-back Verification**:
   - If `ota-mqtt` takes longer than `timeout_seconds` to reply with an ack on `/private/i_sets/ack`, `redbida.Service` gracefully falls back to direct read-back over `/private/i_gets`. The MCP tool handler should reflect this partial/verified status clearly in the `ToolResult` JSON.
4. **Unauthenticated Loopback Security**:
   - `allow_unauthenticated_loopback: true` allows local AI agents (running on `127.0.0.1`) to invoke RedBida tools without managing API keys, but remote access via external interfaces will still enforce `X-MCP-Key` / `Bearer` authentication.

---

## 4. Conclusion & Architecture Blueprint for `internal/mcp/tools_redbida.go`

### 4.1 Specification for 6 RedBida MCP Tools

```go
package mcp

// 1. redbida_list_catalog
// Name: "redbida_list_catalog"
// Description: "List all configuration keys in the RedBida / OTA-MQTT catalog with their metadata, functional group, risk classification (editable, confirm-required, protected), data type, and storage source availability."
// InputSchema: {
//   "type": "object",
//   "properties": {
//     "group": {"type": "string", "description": "Filter by group (e.g. 'UI / Display', 'Livestream', 'Branding / Logo', 'Schedule / Maintenance', 'Security / Credentials', 'Network / MQTT')"},
//     "editableOnly": {"type": "boolean", "description": "Only return keys that can be edited"}
//   }
// }

// 2. redbida_get_keys
// Name: "redbida_get_keys"
// Description: "Read the live values of one or more configuration keys from the local ota-mqtt broker (via /private/i_gets). Sensitive credentials are automatically masked."
// InputSchema: {
//   "type": "object",
//   "properties": {
//     "keys": {"type": "array", "items": {"type": "string"}, "description": "List of key names to fetch"},
//     "all": {"type": "boolean", "description": "If true and keys is empty, fetch all available keys from the catalog"}
//   }
// }

// 3. redbida_set_keys
// Name: "redbida_set_keys"
// Description: "Write one or more key-value pairs to the local ota-mqtt broker (via /private/i_sets) with mandatory read-back verification. High-risk maintenance keys require confirmed=true."
// InputSchema: {
//   "type": "object",
//   "required": ["changes"],
//   "properties": {
//     "changes": {"type": "object", "description": "Key-value map of configuration changes"},
//     "confirmed": {"type": "boolean", "description": "Must be true to apply confirm-required maintenance or restart keys"}
//   }
// }

// 4. redbida_apply_onboarding_preset
// Name: "redbida_apply_onboarding_preset"
// Description: "1-Click Bida Onboarding Tool: Automatically synthesizes and applies the 15 standard golden template parameters (title, company_name, sanitized ui_bg gradient, diacritic-free custom_hashtags, 20-tab INI ui_tabs_links, camera_count, toolbar_show_count, video_config, go2rtc flags, scoreboard, logos, and go2rtc trigger flag) with full read-back verification."
// InputSchema: {
//   "type": "object",
//   "required": ["title", "cameraCount"],
//   "properties": {
//     "title": {"type": "string", "description": "Shop name / venue label (e.g. 'CX King Luxury')"},
//     "cameraCount": {"type": "integer", "description": "Number of active cameras / monitors (e.g. 8)"},
//     "bg": {"type": "string", "description": "CSS gradient background string (trailing semicolons will be automatically stripped)"},
//     "groupKey": {"type": "string", "description": "Shinobi group key / shinobi_camera_id"},
//     "shinobiToken": {"type": "string", "description": "Shinobi view token (API key with view stream/video rights)"},
//     "shinobiMonitorToken": {"type": "string", "description": "Shinobi monitor token (API key with get monitor rights)"},
//     "ggcode": {"type": "string", "description": "Google Analytics measurement ID (e.g. 'G-SFSDZPR95Z')"},
//     "customHashtags": {"type": "string", "description": "Custom hashtags override. If omitted, generated as #<CleanTitle> #BILLIARDSlive #INUTlive #highlightsports"},
//     "dryRun": {"type": "boolean", "description": "If true, returns the synthesized 15 keys without writing to MQTT broker"},
//     "confirmed": {"type": "boolean", "description": "Whether to auto-confirm maintenance keys (default: true)"}
//   }
// }

// 5. redbida_trigger_go2rtc
// Name: "redbida_trigger_go2rtc"
// Description: "Trigger Node-RED :2023 to generate /root/go2rtc.yaml stream configurations by publishing button_generate_go2rtc_stream: 'true' over MQTT /private/i_sets."
// InputSchema: {
//   "type": "object"
// }

// 6. redbida_get_time_status
// Name: "redbida_get_time_status"
// Description: "Check host system clock, RFC 3339 timestamp, and NTP synchronization status via timedatectl to ensure accurate video playback timelines."
// InputSchema: {
//   "type": "object"
// }
```

### 4.2 Blueprint for `removeVietnameseTones` & Onboarding Preset Generator in Go

```go
func removeVietnameseTones(str string) string {
	if str == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(str))
	for _, r := range str {
		switch r {
		case 'à', 'á', 'ả', 'ã', 'ạ', 'ă', 'ằ', 'ắ', 'ẳ', 'ẵ', 'ặ', 'â', 'ầ', 'ấ', 'ẩ', 'ẫ', 'ậ':
			b.WriteRune('a')
		case 'À', 'Á', 'Ả', 'Ã', 'Ạ', 'Ă', 'Ằ', 'Ắ', 'Ẳ', 'Ẵ', 'Ặ', 'Â', 'Ầ', 'Ấ', 'Ẩ', 'Ẫ', 'Ậ':
			b.WriteRune('A')
		case 'è', 'é', 'ẻ', 'ẽ', 'ẹ', 'ê', 'ề', 'ế', 'ể', 'ễ', 'ệ':
			b.WriteRune('e')
		case 'È', 'É', 'Ẻ', 'Ẽ', 'Ẹ', 'Ê', 'Ề', 'Ế', 'Ể', 'Ễ', 'Ệ':
			b.WriteRune('E')
		case 'ì', 'í', 'ỉ', 'ĩ', 'ị':
			b.WriteRune('i')
		case 'Ì', 'Í', 'Ỉ', 'Ĩ', 'Ị':
			b.WriteRune('I')
		case 'ò', 'ó', 'ỏ', 'õ', 'ọ', 'ô', 'ồ', 'ố', 'ổ', 'ỗ', 'ộ', 'ơ', 'ờ', 'ớ', 'ở', 'ỡ', 'ợ':
			b.WriteRune('o')
		case 'Ò', 'Ó', 'Ỏ', 'Õ', 'Ọ', 'Ô', 'Ồ', 'Ố', 'Ổ', 'Ỗ', 'Ộ', 'Ơ', 'Ờ', 'Ớ', 'Ở', 'Ỡ', 'Ợ':
			b.WriteRune('O')
		case 'ù', 'ú', 'ủ', 'ũ', 'ụ', 'ư', 'ừ', 'ứ', 'ử', 'ữ', 'ự':
			b.WriteRune('u')
		case 'Ù', 'Ú', 'Ủ', 'Ũ', 'Ụ', 'Ư', 'Ừ', 'Ứ', 'Ử', 'Ữ', 'Ự':
			b.WriteRune('U')
		case 'ỳ', 'ý', 'ỷ', 'ỹ', 'ỵ':
			b.WriteRune('y')
		case 'Ỳ', 'Ý', 'Ỷ', 'Ỹ', 'Ỵ':
			b.WriteRune('Y')
		case 'đ':
			b.WriteRune('d')
		case 'Đ':
			b.WriteRune('D')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeCleanTitle(title string) string {
	noTones := removeVietnameseTones(title)
	var b strings.Builder
	for _, r := range noTones {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func generate20TabINITabs(title string) string {
	var sections []string
	for i := 1; i <= 20; i++ {
		sections = append(sections, fmt.Sprintf("[C%02d]\nstream_label=Video Trực tiếp\nvid_list_label=Danh sách highlight\nvid_play_label=%s\nlist_refresh_label=Cập nhật highlight", i, title))
	}
	return strings.Join(sections, "\n\n")
}
```

### 4.3 Integration into `internal/mcp/server.go`

Modify `NewServer` signature and body:
```go
func NewServer(cfg *config.Config, inv *config.Inventory, shinobiClient *shinobi.Client, redbidaService ...*redbida.Service) *Server {
    if cfg == nil {
        defaultCfg := config.Default()
        cfg = &defaultCfg
    }

    var rSvc *redbida.Service
    if len(redbidaService) > 0 && redbidaService[0] != nil {
        rSvc = redbidaService[0]
    } else if cfg.Redbida.Enabled {
        broker := redbida.NewMQTTBroker(redbida.MQTTOptions{
            Host: cfg.Redbida.BrokerHost, Port: cfg.Redbida.BrokerPort,
            ReadTopic: cfg.Redbida.ReadTopic, ReadAckTopic: cfg.Redbida.ReadAckTopic,
            WriteTopic: cfg.Redbida.WriteTopic, WriteAckTopic: cfg.Redbida.WriteAckTopic,
            Timeout: time.Duration(cfg.Redbida.TimeoutSeconds) * time.Second,
        })
        rSvc = redbida.NewService(broker, redbida.NewCatalog(cfg.Redbida.KeyDir), cfg.Redbida.MaxBatchKeys)
    }

    registry := NewRegistry()
    registerCameraInventoryTools(registry, cfg, inv)
    registerCameraConfigTools(registry, cfg, inv)
    registerDiscoveryDiagnosisTools(registry, cfg, inv)
    registerShinobiTools(registry, cfg, inv, shinobiClient)
    registerRedbidaTools(registry, cfg, rSvc)

    return &Server{
        cfg:      cfg,
        inv:      inv,
        shinobi:  shinobiClient,
        redbida:  rSvc,
        registry: registry,
        sessions: make(map[string]*Session),
    }
}
```

---

## 5. Verification Method

To independently verify the MCP architecture and subsequent RedBida tool integration:

1. **Unit Test Suite Execution**:
   ```bash
   /home/ksp/go-sdk/bin/go test -v ./internal/mcp
   /home/ksp/go-sdk/bin/go test -v ./internal/redbida
   /home/ksp/go-sdk/bin/go test ./...
   ```
2. **Tool List Verification (`tools/list`)**:
   Verify that `tools/list` returns all 31 registered tools (25 core + 6 RedBida tools).
3. **Stdio Protocol Smoke Test**:
   ```bash
   echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./bin/kspcam --mcp --config config.yaml
   ```
4. **HTTP / SSE Direct Request Smoke Test**:
   ```bash
   curl -s -X POST http://127.0.0.1:2028/mcp \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"redbida_get_time_status","arguments":{}}}'
   ```
5. **Onboarding Preset Hermetic Unit Test**:
   Verify generated `ui_tabs_links` spans `[C01]` to `[C20]`, `ui_bg` has no trailing semicolon, and `custom_hashtags` strips all Vietnamese accents.
