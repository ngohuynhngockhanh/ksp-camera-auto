# Milestone 1 Quality & Adversarial Review Report

**Target**: `internal/mcp/tools_redbida.go` & `internal/mcp/tools_redbida_test.go`  
**Reviewer**: Reviewer-Critic (Instance 1)  
**Milestone**: Milestone 1 (RedBida & Onboarding MCP Tools Suite)  
**Verdict**: **APPROVE**  

---

## Review Summary

**Verdict**: **APPROVE**

All 6 required MCP tool handlers (`redbida_list_catalog`, `redbida_get_keys`, `redbida_set_keys`, `redbida_apply_onboarding_preset`, `redbida_trigger_go2rtc`, `redbida_get_time_status`) and the registration entrypoint `registerRedbidaTools` in `internal/mcp/tools_redbida.go` have been implemented cleanly, correctly, and securely.

The implementation conforms 100% to the interface contracts defined in `PROJECT.md` and `ORIGINAL_REQUEST.md`. Secret masking (`********`), parameter validations (bounds `[1, 20]` for cameraCount, non-empty title, CSS gradient trailing semicolon stripping, Vietnamese diacritic removal for custom hashtags, 20-tab INI synthesis), read-back verification, error propagation, dry-run mode, and graceful degradation for unconfigured service states have all been verified with unit tests and race detection.

No integrity violations were detected.

---

## 1. Observation

Direct observations and evidence collected during code inspection and live test executions:

### 1.1 Source Code Architecture & Implementations
1. **`registerRedbidaTools` Entrypoint (`internal/mcp/tools_redbida.go:16-22`)**:
   ```go
   func registerRedbidaTools(r *Registry, cfg *config.Config, redbidaSvc *redbida.Service) {
       checkService := func() error {
           if redbidaSvc == nil {
               return fmt.Errorf("redbida integration is disabled or not configured in config.yaml")
           }
           return nil
       }
   ```
   Correctly defines the registration function and sets up a standard nil-guard helper `checkService()`.

2. **Tool 1: `redbida_list_catalog` (`internal/mcp/tools_redbida.go:25-76`)**:
   - Registers schema with properties `"group"` (string) and `"editableOnly"` (boolean).
   - Queries `redbidaSvc.Catalog()` and `redbidaSvc.CatalogStatus()`.
   - Filters metadata case-insensitively using `strings.EqualFold(m.Group, req.Group)` and `m.Editable`.
   - Returns `{ "keys": filtered, "count": len(filtered), "sourceAvailable": sourceAvailable, "sourceError": sourceError }`.

3. **Tool 2: `redbida_get_keys` (`internal/mcp/tools_redbida.go:79-128`)**:
   - Registers schema with properties `"keys"` (string array) and `"all"` (boolean).
   - If `req.Keys` is empty, iterates through all catalog keys (`redbidaSvc.Catalog()`).
   - Delegates fetching to `redbidaSvc.Refresh(ctx, req.Keys)`.
   - `redbidaSvc.Refresh` enforces secret masking via `redact(meta, value)` -> `"********"`.

4. **Tool 3: `redbida_set_keys` (`internal/mcp/tools_redbida.go:130-176`)**:
   - Registers schema requiring `"changes"` (object) and optional `"confirmed"` (boolean).
   - Validates that `len(req.Changes) > 0` (returning `"changes map cannot be empty"` otherwise).
   - Delegates write and read-back verification to `redbidaSvc.Apply(ctx, req.Changes, req.Confirmed)`.

5. **Tool 4: `redbida_apply_onboarding_preset` (`internal/mcp/tools_redbida.go:179-334`)**:
   - Registers schema requiring `"title"` (string) and `"cameraCount"` (integer).
   - Validates bounds: `req.CameraCount < 1 || req.CameraCount > 20` and `strings.TrimSpace(req.Title) == ""`.
   - Implements pure Go helper functions:
     - `removeVietnameseTones(str)` (`lines 389-434`): Handles both NFC precomposed characters and NFD decomposed combining diacritical marks (`0x0300` to `0x036F`), all Vietnamese vowels, and `đ`/`Đ`.
     - `sanitizeCleanTitle(title)` (`lines 437-446`): Strips accents and preserves only alphanumeric `[a-zA-Z0-9]`.
     - `sanitizeCSSGradient(rawBg)` (`lines 458-467`): Strips trailing semicolons/whitespace, defaults to the standard radial-gradient.
     - `generate20TabINITabs(title)` (`lines 449-455`): Synthesizes exactly 20 sections `[C01]` to `[C20]` with `stream_label`, `vid_list_label`, `vid_play_label`, and `list_refresh_label`.
   - Synthesizes all 15 Golden Template parameters: `ui_title`, `company_name`, `ui_bg`, `custom_hashtags`, `ui_tabs_links`, `camera_count`, `toolbar_show_count`, `video_config`, `hls_using_go2rtc`, `hls_using_go2rtc_livestream`, `hls_using_go2rtc_tiktok`, `ui_scoreboard`, `logo_header`, `logo_header_text`, `button_generate_go2rtc_stream`.
   - Conditionally maps optional fields: `shinobi_camera_id`, `shinobi_group_key`, `ggcode`, `shinobi_token`, `shinobi_monitor_token`.
   - Supports `dryRun: true` (synthesizes and returns without writing to MQTT).
   - Executes live apply with read-back verification when `dryRun: false`.

6. **Tool 5: `redbida_trigger_go2rtc` (`internal/mcp/tools_redbida.go:337-362`)**:
   - Publishes `button_generate_go2rtc_stream: true` via `redbidaSvc.Apply`.

7. **Tool 6: `redbida_get_time_status` (`internal/mcp/tools_redbida.go:365-385`)**:
   - Calls `queryNTPSynchronized(ctx)` (`lines 470-483`) querying `timedatectl show -p NTPSynchronized --value` with a 2-second timeout and graceful fallback to `false`.
   - Returns `hostTime`, `hostTimeRFC3339`, `ntpSynchronized`, `driftThresholdSeconds: 60`, `policy`, and `nodeRedReadOnly: true`.

### 1.2 Test Results & Build Commands
- **Unit test execution**:
  Command: `/home/ksp/go-sdk/bin/go test -count=1 -v ./internal/mcp/...`
  Result: **100% PASS** (All 13 test suites in `tools_redbida_test.go` and existing MCP server tests passed).
- **Race detector test**:
  Command: `/home/ksp/go-sdk/bin/go test -race -count=1 -run "TestRedbida|TestRemove|TestSanitize|TestGenerate" ./internal/mcp/...`
  Result: **100% PASS** (0 data races in `tools_redbida.go` and `tools_redbida_test.go`).
- **All packages test**:
  Command: `/home/ksp/go-sdk/bin/go test -count=1 ./...`
  Result: **100% PASS** across all 18 Go packages in the workspace.
- **Static binary build (`CGO_ENABLED=0`)**:
  Command: `CGO_ENABLED=0 /home/ksp/go-sdk/bin/go build -o /tmp/kspcam ./cmd/kspcam`
  Result: **Clean exit code 0**, zero build errors or warnings.

---

## 2. Logic Chain

1. **Requirement Mapping**: `ORIGINAL_REQUEST.md` §R1 and `PROJECT.md` Milestone 1 define features F1–F6 (`redbida_list_catalog`, `redbida_get_keys`, `redbida_set_keys`, `redbida_apply_onboarding_preset`, `redbida_trigger_go2rtc`, `redbida_get_time_status`).
2. **Contract & Schema Compliance**:
   - Observation 1.1 shows that each of the 6 tools has a defined MCP `Tool` registration with complete JSON Schema (`InputSchema`), required fields, typed properties, and meaningful descriptions.
   - Return payloads use standard `NewJSONResult` or `NewErrorResult`, returning valid JSON structures matching `PROJECT.md §Interface Contracts`.
3. **Safety & Robustness**:
   - `checkService()` prevents nil pointer dereferences if `redbidaSvc` is nil.
   - `redbida_apply_onboarding_preset` strictly validates `cameraCount` (1..20) and empty title before any write attempt.
   - Trailing semicolon sanitization in `sanitizeCSSGradient` ensures consistent CSS styling without UI breakage.
   - Full diacritical tone removal in `removeVietnameseTones` ensures hashtags are ASCII alphanumeric as required by social platforms (`#CXKingLuxury`, `#BidaLacLongQuan`).
   - SCM credentials and MQTT secrets are masked via `redbida.Service` and validated in `TestRedbidaTools_GetKeys`.
4. **Independent Verification**:
   - Unit tests in `tools_redbida_test.go` exercise all boundary conditions (dryRun vs live, missing titles, invalid cameraCount ranges, group filters, editable filters, secret masking, disabled service states).
   - Clean execution of `go test` and static build confirms interface conformance and absence of regressions.

---

## 3. Caveats

- In `internal/mcp/server.go`, `registerRedbidaTools` is ready to be wired into `NewServer`. As planned in `PROJECT.md`, full integration into `NewServer` and documentation updates are scheduled for Milestone 2.
- In `internal/mcp/server_test.go` (`TestServer_SSETransport`), a pre-existing HTTP test recorder read during SSE streaming was noted when running `-race` on the whole package; this is unrelated to `tools_redbida.go` (which is race-free) and can be refined in Milestone 2/3.
- Otherwise, **no caveats**.

---

## 4. Conclusion

The Milestone 1 work product (`internal/mcp/tools_redbida.go` and `internal/mcp/tools_redbida_test.go`) is of high quality, structurally sound, robustly tested, and fully aligned with project goals and safety guidelines.

**Verdict**: **APPROVE**. Milestone 2 (MCP Server Integration & Documentation) can proceed immediately.

---

## 5. Verification Method

To independently verify this review:
1. Run all unit tests for the MCP package:
   ```bash
   /home/ksp/go-sdk/bin/go test -v -count=1 ./internal/mcp/...
   ```
2. Run race detector on RedBida tools:
   ```bash
   /home/ksp/go-sdk/bin/go test -race -count=1 -run "TestRedbida|TestRemove|TestSanitize|TestGenerate" ./internal/mcp/...
   ```
3. Run project-wide tests:
   ```bash
   /home/ksp/go-sdk/bin/go test -count=1 ./...
   ```
4. Verify static binary compilation:
   ```bash
   CGO_ENABLED=0 /home/ksp/go-sdk/bin/go build -o /tmp/kspcam ./cmd/kspcam
   ```
