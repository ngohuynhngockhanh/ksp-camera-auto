# RedBida Backend Architecture & Catalog Spec Survey Report

## 1. Observation

Direct observations from examining the codebase, protocol implementations, catalog definitions, and test suites:

### 1.1 Architecture & Codebase Map
- **`internal/redbida/types.go`**:
  - Line 10–28: Defines domain risk types (`RiskEditable`, `RiskConfirm`, `RiskProtected`, `RiskUnknown`) and value types (`TypeString`, `TypeNumber`, `TypeBoolean`, `TypeJSON`, `TypeImage`, `TypeUnknown`).
  - Line 30–39: `KeyMeta` structure containing `Key`, `Label`, `Group`, `Description`, `Risk`, `ValueType`, `Editable`, `Secret`.
  - Line 41–46: `KeyValue` structure with `Key`, `Meta`, `Value`, `Exists`.
  - Line 53–64: `ChangeResult` with `Key`, `Meta`, `OldValue`, `NewValue`, `Changed`, `Acknowledged`, `ReadBack`, `Verified`, `Applied`, `Error`.
  - Line 66–69: `Broker` interface with `Read(ctx context.Context, keys []string) (map[string]any, error)` and `Write(ctx context.Context, changes map[string]any) (map[string]WriteAck, error)`.
  - Line 89–91: `isLogoKey(key)` checks `logo_header`, `logo_livestream`, `logo_cat_cam` for `TypeImage`.
  - Line 93–98: `redact(meta, value)` masks sensitive values with `"********"` when `meta.Secret == true`.

- **`internal/redbida/catalog.go`**:
  - Line 11: `sensitiveKeyRe` matches `(?i)(password|token|secret|username|mqtt_|shinobi_|sapo_url|md5_node_id|node_info|blacklist_keys|hidden_keys|config_no_use|frpc_config|gortc_default_config|apiRecentInput_string|ggcode|api_key|access_key|private_key|credential|cookie|s3-storage)`.
  - Line 12: `protectedKeyRe` matches `(?i)(^|_)(ip|route|gateway|dns|frpc|broker|port|virtual_ip|static_|wifi_|valid_license|inut_id)`.
  - Line 13: `runtimeKeyRe` matches `(?i)(^api(_model)?_count$|^download_count$|^packed_count$|^view_count$|^toolbar_show_count$|^node_config_|^test_button$)`.
  - Line 18–50: `fallbackKeys` defines 112 static known keys observed from node survey (`inut_204_63`).
  - Line 53–67: `editableKeySet` contains standard operator editable keys (`ui_title`, `ui_bg`, `logo_header`, `logo_header_text`, `logo_livestream`, `ui_scoreboard`, `ui_tabs_links`, `custom_hashtags`, `camera_count`, `video_config`, `hls_using_go2rtc`, `show_toolbar`, etc.).
  - Line 68–77: `confirmEditableKeySet` contains keys requiring confirmation before submission (`button_generate_go2rtc_stream`, `button_reboot`, `button_restart_shinobi`, `max_free_ram_*`, `stop_camera_*`, etc.).
  - Line 78–87: `booleanKeySet` contains boolean fields.
  - Line 88–93: `numberKeySet` contains numeric fields (`camera_count`, `fps_default`, `livestream_default_bitrate`, `max_free_ram_*`, etc.).
  - Line 94: `jsonKeySet` contains `custom_hashtags` and `ui_tabs_links`.
  - Line 227–272: `metaForKey(key, label, description)` determines `Group`, `Risk`, `ValueType`, `Editable`, and `Secret`.
    - Group classification rules:
      - `sensitiveKeyRe` -> `"Security / Credentials"`
      - `protectedKeyRe` -> `"Network / MQTT"`
      - `strings.HasPrefix(key, "logo_") || key == "company_name" || key == "banner_top" || strings.HasPrefix(key, "app_")` -> `"Branding / Logo"`
      - `strings.Contains(key, "livestream") || strings.HasPrefix(key, "hls_") || key == "place_livestream" || key == "fps_default" || strings.HasPrefix(key, "default_delay_") || key == "disable_cut_realtime"` -> `"Livestream"`
      - `strings.HasPrefix(key, "ui_") || key == "language" || key == "show_toolbar" || key == "large_monitor" || key == "help_link" || key == "url_live_help"` -> `"UI / Display"`
      - `strings.HasPrefix(key, "stop_camera_") || strings.Contains(key, "reboot") || strings.Contains(key, "watch_uptime") || strings.HasPrefix(key, "db_check_") || strings.HasPrefix(key, "max_free_ram_")` -> `"Schedule / Maintenance"`
      - Fallback for all other keys -> `"Advanced / Unknown"`

- **`internal/redbida/mqtt.go`**:
  - Line 40–63: Default connection parameters (`127.0.0.1:12369`, read topic `/private/i_gets`, read ack topic `/private/i_gets/ack`, write topic `/private/i_sets`, write ack topic `/private/i_sets/ack`, timeout 10s).
  - Line 65–81: `connect()` uses Paho MQTT client with `CleanSession: true`, `AutoReconnect: false`. Client ID: `kspcam-redbida-<nanos>`.
  - Line 85–138: `request()` subscribes to ack topic, publishes request payload with `QoS: 0, Retain: false`, filters out retained messages, and iterates decoding incoming ack packets.
  - Line 152–171: `Read()` publishes `{"info": ["<key1>", "<key2>", ...]}` and decodes `{"info": {"<key1>": <val1>, ...}}` via `containsExactRaw`.
  - Line 173–211: `Write()` publishes `{"info": {"<key1>": <val1>, ...}}` and decodes `{"info": {"<key1>": {"oldValue": <old>, "newValue": <new>}, ...}}` via `containsAllWriteAck` and `valuesEqual(ack.NewValue, expected)`.

- **`internal/redbida/service.go`**:
  - Line 38–80: `Refresh()` reads keys from catalog/broker, redacts secret values, and records dynamic observations in catalog.
  - Line 82–202: `Apply()` validates keys, checks `meta.Editable`, checks `meta.Risk == RiskConfirm` vs `confirmed` flag, validates values via `validateValue()`, performs `broker.Write()`, and performs mandatory 3-attempt `readBack()` verification. If write ack times out (`AckTimeoutError`), `Apply()` executes `readBack()` to check if disk update succeeded.
  - Line 300–345: `validateValue()` validates:
    - String length <= 2MB
    - Image: Data URL (PNG/JPEG/WebP <= 512KB with `http.DetectContentType` validation) or HTTP(S) URL or local path.
    - Boolean: strictly `bool`.
    - Number: numeric with per-key bounds in `numericRules`.
    - JSON: strictly `map[string]any` or `[]any`.
  - Line 353–367: `numericRules` specifies min/max/integer bounds (`camera_count`: [0, 4096] integer, `fps_default`: [1, 120], `livestream_default_bitrate`: [64, 100000], `max_free_ram_*`: [0, 1099511627776]).

- **`internal/server/api_redbida.go`**:
  - Line 10–26: `GET /api/redbida/catalog` -> `{keys, sourceAvailable, sourceError}` (Viewer/Admin).
  - Line 28–57: `POST /api/redbida/refresh` -> `{values, refreshedAt}` (Viewer/Admin).
  - Line 59–84: `POST /api/redbida/apply` -> `{results, appliedAt}` (Admin only).
  - Line 86–102: `GET /api/redbida/time-status` -> `{hostTime, hostTimeRFC3339, ntpSynchronized, driftThresholdSeconds, policy, nodeRedReadOnly}`.
  - Line 104–114: `redbidaTimeout()` calculates 3x multiplier (default 30s up to 120s) for batch writes.

- **`internal/config/config.go`**:
  - Line 106–144: `RedbidaConfig` with YAML aliases (`broker` -> host:port, `catalog_dir` -> `key_dir`).
  - Default: `Enabled: false`, `BrokerHost: "127.0.0.1"`, `BrokerPort: 12369`, `KeyDir: "/root/ota-mqtt/change_ok"`.

### 1.2 Test Execution Results
- `go test -v ./internal/redbida/... -cover`: All 18 tests passed. Statement coverage: `81.7%`.
- `go test -v ./internal/server -run "Redbida"`: All 5 tests passed (`TestRedbidaHandlersRefreshAndApply`, `TestRedbidaCatalogHandlerIncludesDiscoveredMetadata`, `TestRedbidaCatalogReportsUnavailableSourceOnFirstRequest`, `TestConfigReportsRedbidaEnabled`, `TestRedbidaRoutesEnforceViewerAuthorization`).
- `go test ./...`: All unit test packages across the workspace passed 100%.

---

## 2. Logic Chain

1. **Protocol Compliance & Data Integrity**:
   - The user request requires that all interactions with Redbida use MQTT topics `/private/i_sets` (for writes: `{"info": {"<key>": "<val>", ...}}`) and `/private/i_gets` (for reads: `{"info": ["<key1>", "<key2>", ...]}`).
   - Observation from `internal/redbida/mqtt.go` (lines 154, 182) confirms that `MQTTBroker` strictly adheres to this schema.
   - Observation from `internal/redbida/service.go` (lines 135–200, 222–272) confirms that every write is verified through a mandatory 3-phase read-back (`/private/i_gets`) with retry backoff, ensuring complete data consistency and zero blind writes.

2. **Analysis of 4 Onboarding Pillars vs Existing Catalog**:
   - **Pillar 1: Branding & Giao diện Quán**:
     - `ui_title`, `ui_bg`, `logo_header`, `logo_header_text`, `logo_livestream`, `ui_scoreboard`, `ui_tabs_links`: Fully registered in `fallbackKeys`, `editableKeySet`, and correctly typed.
     - `custom_hashtags`: Registered in `fallbackKeys`, `editableKeySet`, and `jsonKeySet`, but currently classified in group `"Advanced / Unknown"`. It should be grouped into `"Branding / Logo"` or `"UI / Display"`.
   - **Pillar 2: Video Streaming & Go2RTC Engine**:
     - `hls_using_go2rtc`: Registered in `fallbackKeys`, `editableKeySet`, `booleanKeySet`, group `"Livestream"`.
     - `button_generate_go2rtc_stream`: Registered in `fallbackKeys`, `confirmEditableKeySet`, but currently group `"Advanced / Unknown"`. It should be classified under `"Livestream"` or `"Schedule / Maintenance"`.
     - `camera_count`: Registered in `fallbackKeys`, `editableKeySet`, `numberKeySet` with bounds [0, 4096], but group `"Advanced / Unknown"`. It should be classified under `"Livestream"` or `"System / Hardware"`.
     - `video_config`: Registered in `fallbackKeys`, `editableKeySet`, `TypeString` (e.g. `range=72`), but group `"Advanced / Unknown"`. It should be grouped under `"Livestream"`.
     - `show_toolbar`: Registered in `fallbackKeys`, `editableKeySet`, `booleanKeySet`, group `"UI / Display"`.
     - `toolbar_show_count`: Matched by `runtimeKeyRe` -> `RiskProtected`, `editable: false`. This is a read-only counter from Node-RED runtime.
   - **Pillar 3: Shinobi NVR Authentication & Group Sync**:
     - `shinobi_camera_id`, `shinobi_token`, `shinobi_monitor_token`: Registered in `fallbackKeys`, matched by `sensitiveKeyRe` -> `RiskProtected`, `secret: true`, `editable: false`, group `"Security / Credentials"`.
     - `shinobi_group_key`: Matched by `sensitiveKeyRe` (`shinobi_` prefix) -> `RiskProtected`, `secret: true`, `editable: false`, group `"Security / Credentials"`. However, it is not explicitly listed in the `fallbackKeys` string list. Adding it explicitly prevents missing-metadata edge cases when disk directory `/root/ota-mqtt/change_ok` is unavailable.
   - **Pillar 4: Hệ thống & An ninh**:
     - `frpc_config`, `ggcode`: Registered in `fallbackKeys`, matched by `sensitiveKeyRe` -> `RiskProtected`, `secret: true`, `editable: false`, group `"Security / Credentials"`.
     - `inut_id`: Registered in `fallbackKeys`, matched by `protectedKeyRe` -> `RiskProtected`, `secret: false`, `editable: false`, group `"Network / MQTT"`.
     - `max_free_ram_*` / Watchdog thresholds: Registered in `fallbackKeys`, `confirmEditableKeySet`, `numberKeySet`, `numericRules` (0 to 1 TB), group `"Schedule / Maintenance"`.
     - Time sync: `GET /api/redbida/time-status` provides host NTP validation and drift policy.

3. **Gaps in Key Grouping & Catalog Metadata**:
   - In `internal/redbida/catalog.go:232-246`, `metaForKey` leaves several high-value keys in `"Advanced / Unknown"`:
     - `custom_hashtags` -> Should map to `"Branding / Logo"` (or `"UI / Display"`).
     - `camera_count`, `video_config`, `button_generate_go2rtc_stream` -> Should map to `"Livestream"`.
     - `button_restart_shinobi` -> Should map to `"Schedule / Maintenance"`.
     - `default_tiso_1_color`, `default_tiso_2_color`, `default_tiso_3_color`, `default_tiso_4_color`, `default_tiso_type` -> Should map to `"UI / Display"`.
     - `shop_id`, `realtime_shop_id` -> Should map to `"UI / Display"` or `"Branding / Logo"`.

---

## 3. Caveats

- **Node-RED Read-Only Constraint**: Node-RED port `:2023` and flow files (`/root/.node-red/projects/ok2/flow.json`) must remain strictly untouched and treated as a read-only survey surface. All mutations are performed via the local MQTT broker (`127.0.0.1:12369`) communicating with `ota-mqtt`.
- **Protected Keys**: Secret keys (`shinobi_token`, `shinobi_monitor_token`, `frpc_config`, `ggcode`) and network configuration keys (`eth0_*`, `lan0_*`, `wlan0_*`) are intentionally protected as `RiskProtected` / `editable: false` by design to prevent accidental lockout.
- **Image Validation Constraints**: Image values for `logo_header`, `logo_livestream`, and `logo_cat_cam` are restricted to Data URLs (PNG, JPEG, WebP up to 512 KiB) or valid HTTP(S) URLs / local absolute paths. SVG data URLs are rejected by security policy.

---

## 4. Conclusion

1. **Backend & Protocol Status**:
   - The Go backend implementation for Redbida in `internal/redbida` and `internal/server` is robust, well-architected, and fully tested (`81.7%` test coverage).
   - The MQTT messaging layer (`internal/redbida/mqtt.go`) correctly implements the `ota-mqtt` protocol specifications for `/private/i_sets` and `/private/i_gets`.
   - The read-back verification engine (`internal/redbida/service.go`) guarantees strong consistency and safely handles timeout recovery.

2. **Catalog Metadata Recommendations**:
   - In `internal/redbida/catalog.go`:
     - Add `shinobi_group_key` to `fallbackKeys`.
     - Enhance `metaForKey` grouping logic to classify `custom_hashtags`, `camera_count`, `video_config`, `button_generate_go2rtc_stream`, `button_restart_shinobi`, `default_tiso_*`, and `shop_id` into their intuitive groups (`Branding / Logo`, `Livestream`, `UI / Display`, `Schedule / Maintenance`) rather than defaulting to `"Advanced / Unknown"`.

3. **Readiness for Frontend & Onboarding Hub (R1 & R2)**:
   - The backend REST endpoints (`/api/redbida/catalog`, `/api/redbida/refresh`, `/api/redbida/apply`, `/api/redbida/time-status`) provide all necessary primitives to support the modern Glassmorphism UI, Onboarding Knowledge Hub, Preset Generator, and Live Image/CSS Gradient Previews.

---

## 5. Verification Method

To verify the findings and ensure no regressions:

1. **Run Unit Tests**:
   ```bash
   /home/ksp/go-sdk/bin/go test -v ./internal/redbida/... -cover
   /home/ksp/go-sdk/bin/go test -v ./internal/server -run "Redbida"
   /home/ksp/go-sdk/bin/go test ./...
   ```
2. **Inspect Catalog Classification**:
   - Verify `metaForKey("custom_hashtags", "", "")`, `metaForKey("camera_count", "", "")`, `metaForKey("video_config", "", "")`, etc., in `internal/redbida/catalog.go`.
3. **Inspect MQTT Topics & Payloads**:
   - Verify `NewMQTTBroker` defaults in `internal/redbida/mqtt.go` lines 41–63.
   - Verify `Read()` payload format `{"info": keys}` in line 154 and `Write()` payload format `{"info": changes}` in line 182.
4. **Inspect Read-back Verification**:
   - Verify `s.readBack(ctx, acknowledged)` in `internal/redbida/service.go` lines 178–200 and lines 222–272.
