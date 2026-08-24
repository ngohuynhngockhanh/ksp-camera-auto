# Forensic Audit Report: Milestone 1 (Backend Catalog & Metadata Refinements)

## 1. Observation

Direct forensic observations from codebase inspection, diff analysis, and test execution:

### 1.1 Scope & Modified Files
- **`internal/redbida/catalog.go`**:
  - Line 13: `toolbar_show_count` was removed from `runtimeKeyRe` regex (`(?i)(^api(_model)?_count$|^download_count$|^packed_count$|^view_count$|^node_config_|^test_button$)`).
  - Lines 37–45: `shinobi_group_key` added to `fallbackKeys`.
  - Line 65: `toolbar_show_count` added to `editableKeySet`.
  - Line 94: `toolbar_show_count` added to `numberKeySet`.
  - Line 96: `jsonKeySet` emptied (`var jsonKeySet = keySet("")`), allowing `custom_hashtags` and `ui_tabs_links` to default to `TypeString`.
  - Lines 98–100: Registered numeric rule in `init()`: `numericRules["toolbar_show_count"] = numericRule{min: 0, max: 4096, integer: true}`.
  - Lines 238–253: Updated `metaForKey` grouping logic to map keys to:
    * `"Branding / Logo"`: `logo_*`, `disable_logo_*`, `company_name`, `banner_top`, `custom_hashtags`, `app_*`.
    * `"Livestream"`: `camera_count`, `toolbar_show_count`, `video_config`, `button_generate_go2rtc_stream`, `*livestream*`, `hls_*`, `place_livestream`, `fps_default`, `default_delay_*`, `disable_cut_realtime`.
    * `"UI / Display"`: `ui_*`, `language`, `show_toolbar`, `large_monitor`, `help_link`, `url_live_help`, `default_tiso_*`, `shop_id`, `realtime_shop_id`.
    * `"Schedule / Maintenance"`: `stop_camera_*`, `*reboot*`, `*watch_uptime*`, `db_check_*`, `max_free_ram_*`, `max_shared_ram_*`, `button_restart_shinobi`.
    * `"Security / Credentials"`: `sensitiveKeyRe` matches (including `shinobi_*`, `ggcode`, `frpc_config`).

- **`internal/redbida/redbida_test.go`**:
  - New test file with 203 lines adding 5 comprehensive tests:
    1. `TestCatalogToolbarShowCountMetadataAndValidation`: verifies `RiskEditable`, `TypeNumber`, `Group == "Livestream"`, valid integers in `[0, 4096]`, invalid bounds (`-1`, `4097`), non-integer (`8.5`), non-number (`"8"`).
    2. `TestCatalogStringKeysAcceptTextAndMultiline`: verifies `custom_hashtags` (`TypeString`, `Group == "Branding / Logo"`) and `ui_tabs_links` (`TypeString`, `Group == "UI / Display"` with multiline 20-tab INI input).
    3. `TestCatalogShinobiGroupKeyFallbackAndClassification`: verifies fallback catalog presence, `RiskProtected`, `Secret: true`, `Group == "Security / Credentials"`.
    4. `TestCatalogDomainGroupingClassifications`: verifies 38 keys across all 5 domain categories.
    5. `TestCatalogListOrderingAndFallbackCompleteness`: validates sort order by Group then Key, and catalog length >= 50.

- **`internal/server/api_redbida_test.go`**:
  - Added `TestRedbidaCatalogHandlerMetadataAndDomainGroups`: validates HTTP GET `/api/redbida/catalog` response payload.
  - Added `TestRedbidaApplyBatchPresetChanges`: validates HTTP POST `/api/redbida/apply` batch mutation across 5 preset keys.

### 1.2 Independent Test Execution Outputs
- **Command**: `/home/ksp/go-sdk/bin/go test -count=1 -v ./internal/redbida/... -cover`
  ```
  === RUN   TestMQTTReadIgnoresRetainedAndUnrelatedAcknowledgements
  --- PASS: TestMQTTReadIgnoresRetainedAndUnrelatedAcknowledgements (0.00s)
  === RUN   TestMQTTWriteWaitsForMatchingAcknowledgement
  --- PASS: TestMQTTWriteWaitsForMatchingAcknowledgement (0.00s)
  === RUN   TestMQTTAcknowledgementTimeoutIsTyped
  --- PASS: TestMQTTAcknowledgementTimeoutIsTyped (0.02s)
  === RUN   TestMQTTDefaultsAndDecodeErrors
  --- PASS: TestMQTTDefaultsAndDecodeErrors (0.00s)
  === RUN   TestCatalogToolbarShowCountMetadataAndValidation
  --- PASS: TestCatalogToolbarShowCountMetadataAndValidation (0.00s)
  === RUN   TestCatalogStringKeysAcceptTextAndMultiline
  --- PASS: TestCatalogStringKeysAcceptTextAndMultiline (0.00s)
  === RUN   TestCatalogShinobiGroupKeyFallbackAndClassification
  --- PASS: TestCatalogShinobiGroupKeyFallbackAndClassification (0.01s)
  === RUN   TestCatalogDomainGroupingClassifications
  --- PASS: TestCatalogDomainGroupingClassifications (0.00s)
  === RUN   TestCatalogListOrderingAndFallbackCompleteness
  --- PASS: TestCatalogListOrderingAndFallbackCompleteness (0.00s)
  === RUN   TestCatalogDiscoversKeysAndClassifiesRisk
  --- PASS: TestCatalogDiscoversKeysAndClassifiesRisk (0.01s)
  === RUN   TestRefreshRedactsSecretsAndPreservesTypes
  --- PASS: TestRefreshRedactsSecretsAndPreservesTypes (0.01s)
  === RUN   TestApplyRejectsProtectedUnknownAndUnconfirmedKeys
  --- PASS: TestApplyRejectsProtectedUnknownAndUnconfirmedKeys (0.06s)
  === RUN   TestApplyAllowsConfirmedChangeAndRejectsOversizedImage
  --- PASS: TestApplyAllowsConfirmedChangeAndRejectsOversizedImage (0.04s)
  === RUN   TestApplyFailsClosedWhenReadBackDoesNotMatch
  --- PASS: TestApplyFailsClosedWhenReadBackDoesNotMatch (0.32s)
  === RUN   TestNumericValidationUsesPerKeyRanges
  --- PASS: TestNumericValidationUsesPerKeyRanges (0.00s)
  === RUN   TestNumberValueAcceptsGoNumericTypes
  --- PASS: TestNumberValueAcceptsGoNumericTypes (0.00s)
  === RUN   TestMaintenanceMemoryThresholdRequiresConfirmation
  --- PASS: TestMaintenanceMemoryThresholdRequiresConfirmation (0.00s)
  === RUN   TestCatalogFallsBackToKnownKeysWhenDirectoryUnavailable
  --- PASS: TestCatalogFallsBackToKnownKeysWhenDirectoryUnavailable (0.01s)
  === RUN   TestImageValidationRejectsUnsupportedDataURL
  --- PASS: TestImageValidationRejectsUnsupportedDataURL (0.00s)
  === RUN   TestCatalogDoesNotGrantWriteAccessFromANameHeuristic
  --- PASS: TestCatalogDoesNotGrantWriteAccessFromANameHeuristic (0.01s)
  === RUN   TestApplyFailsClosedWhenCatalogSourceUnavailable
  --- PASS: TestApplyFailsClosedWhenCatalogSourceUnavailable (0.00s)
  === RUN   TestStringValidationRejectsStructuredValues
  --- PASS: TestStringValidationRejectsStructuredValues (0.00s)
  === RUN   TestApplyRecoversFromAckTimeoutUsingReadBack
  --- PASS: TestApplyRecoversFromAckTimeoutUsingReadBack (0.01s)
  PASS
  coverage: 82.0% of statements
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/redbida	0.508s	coverage: 82.0% of statements
  ```

- **Command**: `/home/ksp/go-sdk/bin/go test -count=1 ./...`
  ```
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/bulk	0.011s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera	0.013s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config	0.017s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua	0.025s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/discovery	0.007s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/hik	0.034s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/importer	0.043s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi	0.068s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/mcp	0.124s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/nvrhealth	0.007s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/redbida	0.445s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/server	0.235s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi	0.022s
  ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/tiandy	0.006s
  ```

- **Static Build**: `/home/ksp/go-sdk/bin/go build ./cmd/kspcam` exited with returncode 0 (clean compilation).

---

## 2. Logic Chain

1. **Integrity Mode & Standards**:
   - `ORIGINAL_REQUEST.md` specifies `Integrity mode: development`. Under development mode, prohibited patterns are hardcoded test results, facade implementations, and fabricated verification artifacts.
2. **Analysis for Prohibited Patterns**:
   - **Pattern 1: Hardcoded Test Results**: Checked all modified and new test functions. All test cases perform genuine computation, invoke `metaForKey`, `validateValue`, `catalog.List()`, `catalog.Meta()`, `s.handleRedbidaCatalog()`, and `s.handleRedbidaApply()`. No hardcoded dummy bypasses or string-matching shortcuts were detected.
   - **Pattern 2: Facade Implementations**: Checked `catalog.go`. All metadata mappings, regular expressions, set memberships, and numeric validation rules are fully wired and functional.
   - **Pattern 3: Fabricated Verification Outputs**: All tests were executed live and independently with `-count=1` (bypassing cache), producing genuine passing results and 82.0% statement coverage.
   - **Pattern 4: Self-certifying Tests**: Tests check behavior against the domain contract defined in `ORIGINAL_REQUEST.md` and `PROJECT.md`.
3. **Specification Conformance**:
   - `toolbar_show_count` is editable, numeric (`[0, 4096]`), integer-enforced, and grouped under `Livestream`.
   - `custom_hashtags` and `ui_tabs_links` accept plain and multiline strings up to 2MB without JSON errors.
   - `shinobi_group_key` is present in fallback keys, classified as `RiskProtected`, `Secret: true`, and grouped under `Security / Credentials`.
   - Domain groupings cleanly map 38+ representative keys into the 5 core business groups.

---

## 3. Caveats

- **Scope Boundary**: This audit is scoped exclusively to Milestone 1 deliverables (`internal/redbida/catalog.go`, `internal/redbida/redbida_test.go`, and `internal/server/api_redbida_test.go`). Frontend UI components (`web/static/`) and Playwright E2E suites will be implemented and audited in subsequent milestones (M2, M3, M4).
- **Physical MQTT Broker Dependencies**: Tests use mock broker structures (`redbidaTestBroker`) to test service logic in isolation, which is standard and authentic for Go backend unit testing. Live broker integration is verified on node runtime.

---

## 4. Conclusion

## Forensic Audit Report

**Work Product**: `internal/redbida/catalog.go`, `internal/redbida/redbida_test.go`, `internal/server/api_redbida_test.go` (Milestone 1 Deliverables)
**Profile**: General Project (Development Mode)
**Verdict**: CLEAN

### Phase Results
- **Hardcoded Output Detection**: PASS — No hardcoded test bypasses or fabricated returns.
- **Facade Detection**: PASS — Real logic implementation for catalog metadata, regexes, and numeric validation rules.
- **Behavioral & Test Execution**: PASS — 100% test pass across all packages (`go test -count=1 ./...`), `internal/redbida` coverage at 82.0%.
- **Static Binary Compilation**: PASS — Clean build of `./cmd/kspcam`.
- **Specification Compliance**: PASS — `toolbar_show_count`, `custom_hashtags`, `ui_tabs_links`, `shinobi_group_key`, and 5 domain categories fully match `ORIGINAL_REQUEST.md` requirements.

---

## 5. Verification Method

To independently reproduce the forensic verification:

```bash
# 1. Run redbida unit tests fresh with coverage
/home/ksp/go-sdk/bin/go test -count=1 -v ./internal/redbida/... -cover

# 2. Run server unit tests
/home/ksp/go-sdk/bin/go test -count=1 -v -run "Redbida" ./internal/server/...

# 3. Run full workspace test suite
/home/ksp/go-sdk/bin/go test -count=1 ./...

# 4. Compile static binary
/home/ksp/go-sdk/bin/go build ./cmd/kspcam
```
