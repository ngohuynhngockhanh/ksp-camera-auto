# Milestone 1: Backend Catalog & Metadata Refinements — Handoff Report

## 1. Observation

Direct observations from codebase inspection and execution:

### 1.1 Files Modified & Created
- **`internal/redbida/catalog.go`**:
  - Line 13: Removed `toolbar_show_count` from `runtimeKeyRe` regex (`(?i)(^api(_model)?_count$|^download_count$|^packed_count$|^view_count$|^node_config_|^test_button$)`).
  - Lines 37–45: Added `shinobi_group_key` to `fallbackKeys` between `shinobi_camera_id` and `shinobi_monitor_token`.
  - Line 62: Added `toolbar_show_count` to `editableKeySet`.
  - Line 89: Added `toolbar_show_count` to `numberKeySet`.
  - Line 91: Removed `custom_hashtags` and `ui_tabs_links` from `jsonKeySet` (`var jsonKeySet = keySet("")`), allowing them to default to `TypeString`.
  - Lines 93–95: Added `init()` registering `numericRules["toolbar_show_count"] = numericRule{min: 0, max: 4096, integer: true}`.
  - Lines 238–253: Refined `metaForKey` grouping logic to classify keys into 5 core domain groups:
    * `"Branding / Logo"`: `strings.HasPrefix(key, "logo_") || strings.HasPrefix(key, "disable_logo_") || key == "company_name" || key == "banner_top" || key == "custom_hashtags" || strings.HasPrefix(key, "app_")`.
    * `"Livestream"`: `key == "camera_count" || key == "toolbar_show_count" || key == "video_config" || key == "button_generate_go2rtc_stream" || strings.Contains(key, "livestream") || strings.HasPrefix(key, "hls_") || key == "place_livestream" || key == "fps_default" || strings.HasPrefix(key, "default_delay_") || key == "disable_cut_realtime"`.
    * `"UI / Display"`: `strings.HasPrefix(key, "ui_") || key == "language" || key == "show_toolbar" || key == "large_monitor" || key == "help_link" || key == "url_live_help" || strings.HasPrefix(key, "default_tiso_") || key == "shop_id" || key == "realtime_shop_id"`.
    * `"Schedule / Maintenance"`: `strings.HasPrefix(key, "stop_camera_") || strings.Contains(key, "reboot") || strings.Contains(key, "watch_uptime") || strings.HasPrefix(key, "db_check_") || strings.HasPrefix(key, "max_free_ram_") || strings.HasPrefix(key, "max_shared_ram_") || key == "button_restart_shinobi"`.
    * `"Security / Credentials"`: `sensitiveKeyRe` matches (including `shinobi_*`, `ggcode`, `frpc_config`).

- **`internal/redbida/redbida_test.go`**:
  - Added unit test suite covering:
    * `TestCatalogToolbarShowCountMetadataAndValidation`: validates `RiskEditable`, `TypeNumber`, `Group == "Livestream"`, valid integers in `[0, 4096]`, invalid bounds (`-1`, `4097`), non-integer (`8.5`), non-number (`"8"`).
    * `TestCatalogStringKeysAcceptTextAndMultiline`: validates `custom_hashtags` (`TypeString`, `Group == "Branding / Logo"`) and `ui_tabs_links` (`TypeString`, `Group == "UI / Display"` accepting 20-section multiline INI format).
    * `TestCatalogShinobiGroupKeyFallbackAndClassification`: validates presence in fallback catalog, `RiskProtected`, `Secret == true`, `Group == "Security / Credentials"`.
    * `TestCatalogDomainGroupingClassifications`: verifies 38+ keys across all 5 domain groups.
    * `TestCatalogListOrderingAndFallbackCompleteness`: validates sort order by Group then Key, and catalog length >= 50.

- **`internal/server/api_redbida_test.go`**:
  - Added `TestRedbidaCatalogHandlerMetadataAndDomainGroups`: tests `/api/redbida/catalog` HTTP response contains updated metadata for `toolbar_show_count`, `custom_hashtags`, `ui_tabs_links`, `shinobi_group_key`, and domain groups.
  - Added `TestRedbidaApplyBatchPresetChanges`: tests `/api/redbida/apply` successfully processes batch changes containing `toolbar_show_count`, `custom_hashtags`, `ui_tabs_links`, `ui_title`, and `camera_count`.

### 1.2 Verification Commands and Outputs
1. `/home/ksp/go-sdk/bin/go test -v ./internal/redbida/... -cover`:
   - All 23 tests PASSED. Coverage: `82.0%` of statements.
2. `/home/ksp/go-sdk/bin/go test -v ./internal/server/...`:
   - All tests in `internal/server` PASSED (including all 7 Redbida tests).
3. `/home/ksp/go-sdk/bin/go test ./...`:
   - 100% PASS across all workspace packages.
4. Static build verification (`/home/ksp/go-sdk/bin/go build ./cmd/kspcam`):
   - Zero compilation warnings or errors.

---

## 2. Logic Chain

1. **`toolbar_show_count` Editable Fix**:
   - `toolbar_show_count` was previously matched by `runtimeKeyRe`, marking it `RiskProtected` / `editable: false`.
   - By removing it from `runtimeKeyRe` and adding it to `editableKeySet` and `numberKeySet`, it is classified as `RiskEditable` with `ValueType: TypeNumber`.
   - Adding `numericRules["toolbar_show_count"] = numericRule{min: 0, max: 4096, integer: true}` guarantees input validation strictly checks for integers in `[0, 4096]`.

2. **String Types for `custom_hashtags` and `ui_tabs_links`**:
   - `custom_hashtags` and `ui_tabs_links` were previously in `jsonKeySet`, causing `validateValue` in `service.go` to require a JSON map or slice, rejecting valid plain strings and multiline INI strings.
   - Removing them from `jsonKeySet` causes them to resolve to `TypeString`. `validateValue` accepts strings up to 2MB, enabling full 20-section INI configurations for `ui_tabs_links` and hashtag strings without errors.

3. **`shinobi_group_key` Fallback Presence**:
   - Adding `shinobi_group_key` to `fallbackKeys` ensures that when the physical key directory (`/root/ota-mqtt/change_ok`) is temporarily unavailable, `catalog.List()` and `catalog.Meta("shinobi_group_key")` return valid metadata (`RiskProtected`, `Secret: true`, `Group: "Security / Credentials"`).

4. **Refined Group Mapping**:
   - `metaForKey` now evaluates specific prefixes and explicit key names to assign keys into intuitive business groups (`Branding / Logo`, `Livestream`, `UI / Display`, `Schedule / Maintenance`, `Security / Credentials`) instead of dumping standard keys into `"Advanced / Unknown"`.

---

## 3. Caveats

- **Protected Keys Immutable via `/api/redbida/apply`**: Keys matched by `sensitiveKeyRe` (such as `shinobi_group_key`, `shinobi_camera_id`, `ggcode`, `frpc_config`) remain `RiskProtected` (`editable: false`) by design for security. They cannot be modified via `/api/redbida/apply` directly without administrative override.
- **Node-RED Read-Only Constraint**: All configuration mutations happen exclusively via MQTT `/private/i_sets` with read-back verification via `/private/i_gets`.

---

## 4. Conclusion

All requirements for Milestone 1 (Backend Catalog & Metadata Refinements) are fully implemented and verified:
- `toolbar_show_count` is now an editable integer number (`[0, 4096]`).
- `custom_hashtags` and `ui_tabs_links` accept plain and multiline strings without JSON rejection.
- `shinobi_group_key` is present in fallback keys and correctly classified.
- All keys are categorized into clean, intuitive domain groups.
- Full test coverage added with 100% pass across all unit tests.

---

## 5. Verification Method

To independently verify this milestone:

```bash
# 1. Run unit tests for redbida package with coverage
/home/ksp/go-sdk/bin/go test -v ./internal/redbida/... -cover

# 2. Run unit tests for server package
/home/ksp/go-sdk/bin/go test -v ./internal/server/...

# 3. Run all unit tests across the entire repository
/home/ksp/go-sdk/bin/go test ./...

# 4. Verify static compilation
/home/ksp/go-sdk/bin/go build ./cmd/kspcam
```
