# Local Codebase & Modules Survey Report (ksp-camera-auto)

**Survey Specialist:** Survey Specialist 1: Local Codebase & Modules  
**Timestamp:** 2026-08-24T16:11:00+07:00  
**Workspace:** `/home/ksp/ksp-camera-auto`  
**Go SDK Path:** `/home/ksp/go-sdk/bin/go` (Go 1.25.0)

---

## 1. Executive Summary

This report maps the architecture, package structures, API contracts, configuration schemas, build targets, and deployment mechanics for `ksp-camera-auto` (`kspcam`), focusing on:
1. **`redbida` integration**: The local key/value settings console bridging `kspcam` and `ota-mqtt` over MQTT (`127.0.0.1:12369`) and filesystem catalog (`/root/ota-mqtt/change_ok`), while treating Node-RED (`:2023`) as read-only.
2. **`shinobi` integration**: Pure Go REST client for Shinobi NVR (`:8080`), API/group key authentication, monitor CRUD, state management, two-way sync, and Golden Template inheritance.
3. **Build & deployment specifications**: Multi-arch pure Go static compilation (`CGO_ENABLED=0`), CLI flags, systemd unit definitions (`kspcam.service`), and AES-256-GCM configuration schemas.

---

## 2. Redbida / OTA-MQTT Integration Deep Dive

### 2.1 Architecture & Component Mapping

```
internal/redbida/
├── types.go           # Data models: KeyMeta, KeyValue, Risk, ValueType, Broker interface
├── catalog.go         # Key classification regex, fallback key list (130 keys), file catalog scanner
├── mqtt.go            # Eclipse Paho MQTT client, request/ack correlations, AckTimeoutError handling
└── service.go         # High-level orchestrator: normalization, validation, write + 3-attempt read-back verification
```

Associated server & UI files:
- `internal/server/api_redbida.go`: HTTP handler endpoints (`/api/redbida/catalog`, `/api/redbida/refresh`, `/api/redbida/apply`, `/api/redbida/time-status`).
- `internal/server/server.go`: Conditional route mounting and permission gating (`viewerAllowed`).
- `web/static/redbida.js`: Web frontend rendering categorized tabs, inputs, logo uploads, and read-back status.

### 2.2 MQTT Broker Connection & Topic Protocol

- **Broker Connection**:
  - Host: `127.0.0.1` (configurable via `redbida.broker_host`)
  - Port: `12369` (configurable via `redbida.broker_port`)
  - Client ID: `kspcam-redbida-<timestamp_nanos>`
  - CleanSession: `true`, AutoReconnect: `false`, ConnectTimeout: `10s` (configurable)
- **Topics & Payloads**:
  | Action | Publish Topic | Subscribe/Ack Topic | Request Payload | Ack Response Payload |
  |---|---|---|---|---|
  | **Read** | `/private/i_gets` | `/private/i_gets/ack` | `{"info": ["key1", "key2"]}` | `{"info": {"key1": rawVal1, "key2": rawVal2}}` |
  | **Write** | `/private/i_sets` | `/private/i_sets/ack` | `{"info": {"key1": newVal1}}` | `{"info": {"key1": {"oldValue": oldVal, "newValue": newVal}}}` |
- **Acknowledgement & Read-Back Resilience (`mqtt.go:29-38, 120-137`, `service.go:131-201`)**:
  - Because `/private/i_gets/ack` and `/private/i_sets/ack` are shared legacy topics without correlation IDs, `MQTTBroker` ignores retained messages and filters incoming packets to find matching keys.
  - If a write publish succeeds but the ack topic times out, an `AckTimeoutError` is returned. Instead of failing blindly, `Service.Apply` executes **read-back verification** over `/private/i_gets` to determine if the write took effect.
  - Even upon a successful write ack, `Service.Apply` performs up to 3 read-back attempts (with 100ms and 200ms backoff) to verify the stored value matches the requested value.

### 2.3 Key Catalog & Risk Classification (`catalog.go`)

- **Catalog Source**: Scans files in `keyDir` (default `/root/ota-mqtt/change_ok`). 0-byte files represent empty string values (`Empty(key) == true`).
- **Regex Patterns**:
  - `sensitiveKeyRe`: matches passwords, tokens, API keys, credentials, secret keys -> Group: `"Security / Credentials"`, `Secret: true`, `Risk: RiskProtected`, redacted as `"********"`.
  - `protectedKeyRe`: matches IPs, routes, gateways, DNS, broker, virtual IPs, licenses -> Group: `"Network / MQTT"`, `Risk: RiskProtected` (`editable: false`).
  - `runtimeKeyRe`: counters, test buttons -> `Risk: RiskProtected` (`editable: false`).
  - `validKeyRe`: `^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`.
- **Key Categories & Risk Levels**:
  - `RiskEditable`: Freely editable via UI/API (e.g. `company_name`, `banner_top`, `logo_header`, `language`, `fps_default`, `video_config`).
  - `RiskConfirm`: Requires `confirmed: true` flag in request (e.g. `button_reboot`, `button_restart_shinobi`, `disable_reboot_camera_at_4am`, `stop_camera_*`).
  - `RiskProtected`: Read-only protected (passwords, network routes, tokens).
- **Validation Rules (`service.go:300-441`)**:
  - `TypeImage`: Allowed for `logo_header`, `logo_livestream`, `logo_cat_cam`. Accepts absolute paths (`/path`), HTTP(S) URLs, or base64 Data URLs (`data:image/png;base64,...`, `image/jpeg`, `image/webp`) capped at 512 KB with MIME type verification via `http.DetectContentType`.
  - `TypeNumber`: Enforces explicit bounds (e.g. `camera_count` [0..4096], `fps_default` [1..120], `livestream_default_bitrate` [64..100000]).
  - `TypeBoolean`: Boolean or parseable string boolean.
  - `TypeJSON`: Valid JSON map or array (`custom_hashtags`, `ui_tabs_links`).

### 2.4 REST API Endpoints (`internal/server/api_redbida.go`)

| Endpoint | Method | Role | Request Payload | Response Payload | Description |
|---|---|---|---|---|---|
| `/api/redbida/catalog` | `GET` | Viewer / Admin | None | `{"keys": []KeyMeta, "sourceAvailable": bool, "sourceError": string}` | Returns full key catalog and discovery state |
| `/api/redbida/refresh` | `POST` | Admin | `{"keys": ["key1", ...]}` (empty = all) | `{"values": []KeyValue, "refreshedAt": string}` | Queries MQTT broker for current live key values |
| `/api/redbida/apply` | `POST` | Admin | `{"changes": {"key": "val"}, "confirmed": bool}` | `{"results": []ChangeResult, "appliedAt": string}` | Validates, writes to MQTT, and executes read-back verification |
| `/api/redbida/time-status` | `GET` | Viewer / Admin | None | `{"hostTime": "...", "ntpSynchronized": bool, "nodeRedReadOnly": true, ...}` | Checks host clock and NTP synchronization status |

*Note on Routing:* If `redbida.enabled: false` in `config.yaml`, `/api/redbida/*` routes return `404 Not Found` ("redbida integration is disabled").

---

## 3. Shinobi NVR Integration Deep Dive

### 3.1 Architecture & Component Mapping

```
internal/shinobi/
├── types.go           # MonitorConfig, MonitorDetails, Monitor (with FlexibleString & ParseDetails), Video, SyncReport, ShinobiStatus
├── client.go          # HTTP REST API client: ListMonitors, GetMonitor, AddMonitor, EditMonitor, DeleteMonitor, ChangeMonitorState, GetVideos, Status
└── sync.go            # Bi-directional sync engine: BuildMonitorConfig, SyncToShinobi, SyncFromShinobi, DeviceToMid, vendor path detection
```

Associated server & MCP files:
- `internal/server/api_shinobi.go`: HTTP REST endpoints (`/api/shinobi/status`, `/api/shinobi/monitors`, `/api/shinobi/sync-to-shinobi`, `/api/shinobi/sync-from-shinobi`, `/api/shinobi/videos`).
- `internal/mcp/tools_shinobi.go`: 8 registered MCP tools for AI assistants.

### 3.2 Authentication & API Endpoints (`client.go`)

- **Base URL & Credentials**: Configured via `shinobi.api_url`, `shinobi.api_key`, `shinobi.group_key`.
- **Shinobi REST Wire Format**:
  - `ListMonitors`: `GET /:apiKey/monitor/:groupKey`
  - `GetMonitor`: `GET /:apiKey/monitor/:groupKey/:mid`
  - `AddMonitor` / `EditMonitor`: `POST /:apiKey/configureMonitor/:groupKey/:mid` with `Content-Type: application/x-www-form-urlencoded` and body `data=<url_encoded_json>`
  - `DeleteMonitor`: `GET /:apiKey/configureMonitor/:groupKey/:mid/delete`
  - `ChangeMonitorState`: `GET /:apiKey/monitor/:groupKey/:mid/:state` (valid states: `start`, `stop`, `record`, `idle`, `restart`)
  - `GetVideos`: `GET /:apiKey/videos/:groupKey/:mid?limit=:limit`
- **Gotcha Handling in Unmarshalling (`types.go:52-117`)**:
  - Live Shinobi instances often return `details` as an escaped JSON string (`"details":"{...}"`) instead of an object. `Monitor.ParseDetails()` supports both escaped JSON string and direct JSON object unmarshalling.
  - Number/String polymorphic fields (`port`, `fps`, `width`, `height`) are handled safely via `FlexibleString`.

### 3.3 Golden Template & Bi-directional Sync (`sync.go`)

- **Deterministic Monitor ID (`DeviceToMid`)**:
  - Pattern: `cam_<sanitized_host>_c<nvr_channel>` for NVR channels, or `cam_<sanitized_host>_<port>` for direct cameras.
- **Golden Template Defaults (`sync.go:74-105`)**:
  - `type`: `"h264"`, `mode`: `"record"`, `ext`: `"mp4"`, `protocol`: `"rtsp"`, `port`: `"554"`
  - `stream_type`: `"hls"`, `stream_flv_type`: `"ws"`, `stream_vcodec`: `"copy"`, `stream_acodec`: `"copy"`
  - `vcodec`: `"copy"`, `acodec`: `"copy"`, `record_vcodec`: `"copy"`, `record_acodec`: `"aac"`
  - `cust_input`: `""` (empty)
  - `cust_stream`: `""` (empty)
  - `cust_record`: `"-tag:v hvc1"` (required for H.265 MP4 Apple/browser playback)
- **Sync Operations**:
  - `SyncToShinobi` (`POST /api/shinobi/sync-to-shinobi`): Iterates over `inventory.List()`, builds `MonitorConfig`, calls `AddMonitor` or `EditMonitor` if properties changed, reports `created`, `updated`, `unchanged`, `errors`.
  - `SyncFromShinobi` (`POST /api/shinobi/sync-from-shinobi`): Queries all Shinobi monitors, extracts RTSP credentials and stream paths, detects vendor (`/cam/realmonitor` -> Dahua, `/Streaming/Channels` -> Hikvision), parses NVR channel numbers, and inserts/updates `cameras.yaml` while preserving existing NVR links.

---

## 4. Build, Flags, Configuration Schema & Deployment

### 4.1 Build System & Toolchain (`Makefile`, `go.mod`)

- **Go Version**: `go 1.25.0`
- **Build Flag**: `CGO_ENABLED=0` (pure Go static binary, no runtime C dependencies).
- **LDFLAGS**: `-s -w -X main.version=$(VERSION)`
- **Cross-Compilation Targets**:
  | Target Command | Environment Variables | Output Binary Path | Target Architecture |
  |---|---|---|---|
  | `make build-amd64` | `GOOS=linux GOARCH=amd64` | `dist/kspcam-linux-amd64` | x86_64 Gateway / Server |
  | `make build-arm32` | `GOOS=linux GOARCH=arm GOARM=7` | `dist/kspcam-linux-armv7` | 32-bit ARM (armv7 / armhf) |
  | `make build-arm64` | `GOOS=linux GOARCH=arm64` | `dist/kspcam-linux-arm64` | 64-bit ARM (AArch64 / Orange Pi / RK3588) |
  | `make build-hiksdk`| `CGO_ENABLED=1 -tags hiksdk` | `kspcam-hiksdk` | Optional Cgo build for Hikvision Port 8000 SDK |

### 4.2 Binary CLI Flags (`cmd/kspcam/main.go`)

| Flag | Default | Description |
|---|---|---|
| `--config <path>` | `config.yaml` | Path to runtime YAML configuration file |
| `--addr <addr>` | `""` (uses config `:2028`) | Override web server listen address (e.g. `0.0.0.0:2028`) |
| `--version` | `false` | Print build version and exit |
| `--hash-password <pw>` | `""` | Generate bcrypt hash for web password and exit |
| `--import-shinobi <file>` | `""` | Import monitors JSON file into inventory and exit |
| `--import-hik-port <port>` | `80` | Default config port for imported Hikvision cameras |
| `--import-dahua-port <port>`| `37777` | Default config port for imported Dahua cameras |
| `--mcp` | `false` | Start embedded MCP server over Stdio for AI agents |

### 4.3 Configuration Schema (`internal/config/config.go`)

```yaml
# /opt/ksp-cam/config.yaml

server:
  addr: ":2028"
  username: "admin"
  password: "smarthome12345"
  password_hash: ""                   # Optional bcrypt hash
  viewer_username: "viewer"
  viewer_password: "inut12345"
  login_max_attempts: 5
  login_lockout_minutes: 30

cameras_file: "cameras.yaml"

defaults:
  hikvision_port: 8000
  dahua_port: 37777
  tiandy_port: 554
  username: "admin"
  password: "smarthome12345"
  timeout_seconds: 30
  new_password: "smarthome12345"
  max_review_hours: 72

shinobi:
  api_url: "http://127.0.0.1:8080"
  api_key: ""                         # 30-character API key
  group_key: ""                       # Shinobi Group Key (ke)

mcp:
  enabled: true
  api_key: ""
  allow_unauthenticated_loopback: true

redbida:
  enabled: true                       # Set to true for Redbida/Node-RED integration
  broker_host: "127.0.0.1"
  broker_port: 12369
  read_topic: "/private/i_gets"
  read_ack_topic: "/private/i_gets/ack"
  write_topic: "/private/i_sets"
  write_ack_topic: "/private/i_sets/ack"
  key_dir: "/root/ota-mqtt/change_ok"
  timeout_seconds: 10
  max_batch_keys: 200
```

### 4.4 Password Encryption at Rest (`crypto.go`)

- Encrypted values in `cameras.yaml` are prefixed with `enc:<base64>`.
- AES-256-GCM encryption key is loaded from:
  1. `KSPCAM_KEY` environment variable (32 bytes base64 or SHA-256 hashed string).
  2. `KSPCAM_KEY_FILE` environment variable (e.g. `/opt/ksp-cam/.kspcam.key`).
  3. Default `~/.kspcam.key` (generated with `0600` file permissions).

### 4.5 Systemd Service Specification (`kspcam.service`)

File location: `/etc/systemd/system/kspcam.service`

```ini
[Unit]
Description=ksp-camera-auto (bulk camera config UI on :2028)
After=network-online.target
Wants=network-online.target

[Service]
WorkingDirectory=/opt/ksp-cam
Environment=KSPCAM_KEY_FILE=/opt/ksp-cam/.kspcam.key
ExecStart=/opt/ksp-cam/kspcam --addr 0.0.0.0:2028 --config /opt/ksp-cam/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

---

## 5. Verification Command References

All test suites can be validated with the standard project commands:

```bash
# Go unit & integration tests
/home/ksp/go-sdk/bin/go test ./...
/home/ksp/go-sdk/bin/go vet ./...

# Package specific test with coverage
/home/ksp/go-sdk/bin/go test ./internal/redbida/... -cover
/home/ksp/go-sdk/bin/go test ./internal/shinobi/... -cover
/home/ksp/go-sdk/bin/go test ./internal/config/... -cover
/home/ksp/go-sdk/bin/go test ./internal/server/... -cover

# Build binary
/home/ksp/go-sdk/bin/go build -ldflags "-s -w" -o kspcam ./cmd/kspcam
```
