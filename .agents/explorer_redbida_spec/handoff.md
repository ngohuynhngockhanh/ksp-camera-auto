# RedBida MQTT & Onboarding Specification Exploration Report

**Explorer Archetype**: `teamwork_preview_explorer` (`explorer`, `synthesizer`)  
**Target Module**: `internal/redbida/`, `internal/mcp/`, `web/static/redbida.js`, `plan-sync-redbida/`  
**Timestamp**: 2026-08-24T20:25:00+07:00  
**Handoff Type**: Hard Handoff (Task Complete)

---

## 1. Observation

Direct observations from codebase inspection, specification documents, unit tests, and runtime evidence:

### 1.1 MQTT Broker & Topic Specifications
- **Local Broker Address**: `127.0.0.1:12369` (configured in `internal/config/config.go:184-195` as default `RedbidaConfig`).
- **Key Directory**: `/root/ota-mqtt/change_ok/` on edge nodes (`inut_204_164`, `inut_204_163`, `inut_204_63`).
- **Node-RED Surface**: Runs on `0.0.0.0:2023`, active project `ok2`. Survey-only / read-only surface; configuration writes are sent exclusively via MQTT (`plan-sync-redbida/00-discovery-evidence.md:5-17`).
- **MQTT Topics**:
  * Read Request Topic: `/private/i_gets`
  * Read Acknowledgement Topic: `/private/i_gets/ack`
  * Write Request Topic: `/private/i_sets`
  * Write Acknowledgement Topic: `/private/i_sets/ack`
- **Pure Go MQTT Client Library**: `github.com/eclipse/paho.mqtt.golang` (already integrated in `internal/redbida/mqtt.go:10`, pure Go, 100% `CGO_ENABLED=0` static binary compatible).

### 1.2 MQTT Request & Acknowledgement Wire Payloads
- **Read Request (`/private/i_gets`)** (`internal/redbida/mqtt.go:154`, `plan-sync-redbida/02-sync-contract.md:8-10`):
  ```json
  {
    "info": ["logo_header", "show_toolbar", "ui_title"]
  }
  ```
- **Read Acknowledgement (`/private/i_gets/ack`)** (`internal/redbida/mqtt.go:156`, `plan-sync-redbida/02-sync-contract.md:14-16`):
  ```json
  {
    "info": {
      "logo_header": "https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png",
      "show_toolbar": true,
      "ui_title": "CX King Luxury"
    }
  }
  ```
- **Write Request (`/private/i_sets`)** (`internal/redbida/mqtt.go:182`, `plan-sync-redbida/02-sync-contract.md:20-22`):
  ```json
  {
    "info": {
      "ui_title": "CX King Luxury",
      "camera_count": 8
    }
  }
  ```
- **Write Acknowledgement (`/private/i_sets/ack`)** (`internal/redbida/mqtt.go:184-188`, `plan-sync-redbida/02-sync-contract.md:26-28`):
  ```json
  {
    "info": {
      "ui_title": {
        "oldValue": "Old Name",
        "newValue": "CX King Luxury"
      },
      "camera_count": {
        "oldValue": 6,
        "newValue": 8
      }
    }
  }
  ```

### 1.3 Concurrency, Timeout, Matching & Read-Back Verification
- **Shared Ack Topic Serialization**: The legacy `/private/+/ack` topics do not provide request/correlation IDs. `MQTTBroker` serializes all requests with `b.mu.Lock()` (`internal/redbida/mqtt.go:86`).
- **Retained Message Filter**: Message handler skips retained messages (`if msg.Retained() { return }`, `mqtt.go:95-97`).
- **Response Matching**: Verifies the ACK contains all requested keys (`containsExactRaw`, `containsAllWriteAck`) and matching values (`valuesEqual`). Malformed or unrelated messages from other flows are ignored (`mqtt.go:121-127`).
- **Ack Timeout Handling**: Returns `*AckTimeoutError`. When write ACK times out, `service.Apply` does NOT blindly retry; instead, it executes read-back verification via `s.readBack(ctx, validated)` (`service.go:133-157`).
- **Read-Back Verification**: Reads back keys up to 3 times with exponential backoff (`100ms`, `200ms`) and compares read-back values against requested values (`service.go:222-272`).

### 1.4 Detailed Specification for All 15 Onboarding Parameters
From `ORIGINAL_REQUEST.md:17`, `.agents/skills/camera-naming/SKILL.md:22-35`, and `web/static/redbida.js:285-333`:

| STT | Parameter Key | Data Type | Catalog Group | Risk Level | Validation & Format Specification | Default / Golden Template Example |
|---|---|---|---|---|---|---|
| 1 | `ui_title` | `string` | `UI / Display` | `editable` | Non-empty string. Venue/Club name. | `"CX King Luxury"` or `"SD Billiards Club - CS2"` |
| 2 | `ui_bg` | `string` | `UI / Display` | `editable` | CSS linear/radial background gradient. **STRICT RULE: NO TRAILING SEMICOLON `;`** (Must be stripped: `.replace(/;\s*$/, '').trim()`). | `"radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )"` |
| 3 | `custom_hashtags` | `string` | `Branding / Logo` | `editable` | Normalized hashtag string: Venue name stripped of Vietnamese tones/accents (`removeVietnameseTones`) + non-alphanumeric chars removed, followed by 3 standard tags. Format: `#<CleanTitle> #BILLIARDSlive #INUTlive #highlightsports`. | `#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports` |
| 4 | `ui_tabs_links` | `string` (multiline INI) | `UI / Display` | `editable` | Exactly 20 INI sections `[C01]` to `[C20]` (2-digit zero padded). Each section has 4 lines: `stream_label=Video Trực tiếp\nvid_list_label=Danh sách highlight\nvid_play_label=<ui_title>\nlist_refresh_label=Cập nhật highlight`. Double newline `\n\n` between sections. | See Section 1.5 below |
| 5 | `camera_count` | `number` (int) | `Livestream` | `editable` | Integer in `[0, 4096]`. Total active Shinobi camera/table count. | `8` |
| 6 | `toolbar_show_count` | `number` (int) | `Livestream` | `editable` | Integer in `[0, 4096]`. Number of buttons on livestream toolbar. Always equals `camera_count`. | `8` |
| 7 | `hls_using_go2rtc` | `boolean` | `Livestream` | `editable` | `true` or `false`. Enables low-latency Go2RTC stream distribution. | `true` |
| 8 | `button_generate_go2rtc_stream` | `boolean` / `string` | `Livestream` | `confirm-required` | Gated maintenance trigger. Sends `true` to MQTT to trigger Node-RED :2023 flow to generate `/root/go2rtc.yaml`. | `true` |
| 9 | `logo_header` | `image` (`string`) | `Branding / Logo` | `editable` | URL (`http/https`) or Base64 data URL (`image/png`, `image/jpeg`, `image/webp` max 512 KiB). Fixed standard logo URL. | `"https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png"` |
| 10 | `logo_header_text` | `string` | `Branding / Logo` | `editable` | Slogan string displayed next to header logo. | `"Billiard Live - Tải clip bàn bida và livestream"` |
| 11 | `shinobi_camera_id` | `string` | `Security / Credentials` | `read-only-protected` (Secret) | Shinobi Group ID / Group Key identifying club instance. Masked as `********` in standard views. | `"AWU8wJMd2l"` or `"CX_KING_LUXURY"` |
| 12 | `shinobi_group_key` | `string` | `Security / Credentials` | `read-only-protected` (Secret) | Shinobi Group Key (ke). Masked as `********` in standard views. | `"AWU8wJMd2l"` or `"CX_KING_LUXURY"` |
| 13 | `video_config` | `string` | `Livestream` | `editable` | Video query range string. Standard 72-hour playback window. | `"range=72"` |
| 14 | `ui_scoreboard` | `boolean` | `UI / Display` | `editable` | `true` or `false`. Electronic scoreboard overlay toggle. | `true` |
| 15 | `ggcode` | `string` | `Security / Credentials` | `read-only-protected` (Secret) | Google Analytics measurement / tracking code. | `"G-SFSDZPR95Z"` |

*Additional related keys staged by preset*: `company_name: <ui_title>`, `hls_using_go2rtc_livestream: true`, `hls_using_go2rtc_tiktok: true`.

### 1.5 Exact 20-Section INI Template for `ui_tabs_links`
```ini
[C01]
stream_label=Video Trực tiếp
vid_list_label=Danh sách highlight
vid_play_label=CX King Luxury
list_refresh_label=Cập nhật highlight

[C02]
stream_label=Video Trực tiếp
vid_list_label=Danh sách highlight
vid_play_label=CX King Luxury
list_refresh_label=Cập nhật highlight

... (C03 to C19) ...

[C20]
stream_label=Video Trực tiếp
vid_list_label=Danh sách highlight
vid_play_label=CX King Luxury
list_refresh_label=Cập nhật highlight
```

### 1.6 Go2RTC Trigger & Time Status Mechanisms
- **Go2RTC Trigger Mechanism (`redbida_trigger_go2rtc`)**:
  * Triggered by writing `{"button_generate_go2rtc_stream": true}` (or `"true"`) to `/private/i_sets`.
  * Node-RED flow (port 2023) receives message, fetches all Shinobi monitors, compiles RTSP streams into `/root/go2rtc.yaml`, and reloads `go2rtc` daemon managed by PM2 (`SKILL.md:142-162`).
  * Read-back and ack verification confirm Node-RED received and acknowledged the trigger.
- **Time Status Mechanism (`redbida_get_time_status`)**:
  * Evaluates host system time and queries NTP synchronization state via `timedatectl show -p NTPSynchronized --value` (`internal/server/nvr_health.go:437-450`, `internal/server/api_redbida.go:86-102`).
  * Returns:
    ```json
    {
      "hostTime": "2026-08-24 20:25:00",
      "hostTimeRFC3339": "2026-08-24T20:25:00+07:00",
      "ntpSynchronized": true,
      "driftThresholdSeconds": 60,
      "policy": "sync only when host NTP is trusted and measured drift exceeds 60 seconds",
      "nodeRedReadOnly": true
    }
    ```

### 1.7 Catalog Domain Groups, Risk Levels & Type Rules
Defined in `internal/redbida/catalog.go:238-278` & `types.go:10-28`:
1. **Groups**:
   - `Branding / Logo`
   - `Livestream`
   - `UI / Display`
   - `Schedule / Maintenance`
   - `Security / Credentials`
   - `Network / MQTT`
   - `Advanced / Unknown`
2. **Risk Levels**:
   - `editable` (`RiskEditable`): Standard parameters safe for live updates.
   - `confirm-required` (`RiskConfirm`): Actions requiring explicit confirmation flag (`confirmed: true`).
   - `read-only-protected` (`RiskProtected`): Sensitive credentials, network configs, or internal counters (cannot be updated via standard set; secrets redacted as `********`).
   - `unknown` (`RiskUnknown`): Unrecognized keys; rejected on write.
3. **Data Types**: `string`, `number`, `boolean`, `image`, `json`, `unknown`.

---

## 2. Logic Chain

1. **Evidence 1.1 & 1.2 -> MQTT Protocol Design**:
   - The Go backend communicates with `ota-mqtt` exclusively via TCP MQTT on `127.0.0.1:12369`.
   - Read payloads MUST wrap string key arrays in `{"info": [...]}`.
   - Write payloads MUST wrap key-value maps in `{"info": {...}}`.
   - Read acks return `{"info": {"key": value}}`.
   - Write acks return `{"info": {"key": {"oldValue": ..., "newValue": ...}}}`.

2. **Evidence 1.3 -> Concurrency & Verification Guarantees**:
   - Because the legacy MQTT topics do not feature unique transaction or correlation IDs, operations must be serialized with a mutex.
   - Read-back verification is essential because camera/NVR/Node-RED firmware may drop or fail to persist writes even if an initial socket write succeeded.
   - Handling `AckTimeoutError` via read-back instead of immediate retry prevents double-rebooting hardware or spamming the bus.

3. **Evidence 1.4 & 1.5 -> Onboarding 1-Click Preset Synthesis**:
   - All 15 parameters can be deterministically calculated from basic inputs (`uiTitle`, `cameraCount`, `shinobiGroupId`, `uiBg`, `ggcode`).
   - `removeVietnameseTones` ensures hashtags are compliant across all social platforms and search indexing engines.
   - Trailing semicolon removal on `ui_bg` prevents broken CSS syntax in frontend DOM injections.
   - Generating 20 sections `[C01]`-`[C20]` satisfies legacy frontend template indexing requirements.

4. **Evidence 1.6 & 1.7 -> MCP Toolset Architecture**:
   - The 6 required RedBida MCP tools (`redbida_list_catalog`, `redbida_get_keys`, `redbida_set_keys`, `redbida_apply_onboarding_preset`, `redbida_trigger_go2rtc`, `redbida_get_time_status`) map cleanly to the underlying `redbida.Service` and `redbida.Catalog` methods.
   - All MCP tools should be registered in `internal/mcp/tools_redbida.go` and exposed via both Stdio JSON-RPC and HTTP/SSE `:2028/mcp`.

---

## 3. Caveats

- **Key Directory Inspection**: Directory `/root/ota-mqtt/change_ok` is inspected when available on edge nodes; if running on a development machine without `/root/ota-mqtt/change_ok`, `Catalog` automatically falls back to `fallbackKeys` in `catalog.go:18-51`.
- **Protected Keys in Preset**: In standard `service.Apply`, keys with `RiskProtected` (like `shinobi_camera_id`, `shinobi_group_key`, `ggcode`) are guarded against direct client manipulation. The onboarding preset tool handler in MCP should have elevated administrative execution or direct broker write for initial provisioning.
- **Node-RED Read-Only Constraint**: All operations interact strictly via MQTT topics, never modifying Node-RED flows directly or restarting Node-RED.

---

## 4. Conclusion

The specification for RedBida, local MQTT communication, and all 15 Onboarding parameters is 100% defined, verified against codebase sources, and ready for immediate implementation in `internal/mcp/tools_redbida.go`:

1. **MQTT Wire Protocol**: Pure Go `paho.mqtt.golang`, broker `127.0.0.1:12369`, `/private/i_gets` (`{"info": [...]}`) and `/private/i_sets` (`{"info": {...}}`).
2. **Read-Back Verification**: 3-attempt exponential backoff read-back with deep JSON equality check.
3. **15 Onboarding Parameters**: Full validation rules, sanitization (no semicolon on `ui_bg`, accent-free `custom_hashtags`, 20-section INI `ui_tabs_links`), and golden template defaults.
4. **Go2RTC & Time Status**: Trigger via `button_generate_go2rtc_stream: "true"`, time status via `timedatectl`.
5. **Catalog Metadata**: 7 domain groups, 4 risk tiers, 5 data types.

---

## 5. Verification Method

To independently verify the facts and tests described in this exploration:

```bash
# 1. Run all unit tests for redbida package
/home/ksp/go-sdk/bin/go test -v ./internal/redbida/... -cover

# 2. Run all unit tests for server package (including redbida API endpoints)
/home/ksp/go-sdk/bin/go test -v ./internal/server/...

# 3. Run all unit tests for MCP server package
/home/ksp/go-sdk/bin/go test -v ./internal/mcp/...

# 4. Run entire project test suite
/home/ksp/go-sdk/bin/go test ./...

# 5. Verify static compilation for main binary
/home/ksp/go-sdk/bin/go build ./cmd/kspcam
```
