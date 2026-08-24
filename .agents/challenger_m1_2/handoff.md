# Empirical Adversarial Challenge Report: Milestone 1 (RedBida & Onboarding MCP Tools Suite)

## 1. Observation

Direct empirical observations from source inspection and execution of the adversarial test harness using `/home/ksp/go-sdk/bin/go test -v -race`:

1. **Target Implementation**:
   - File: `/home/ksp/ksp-camera-auto/internal/mcp/tools_redbida.go` (484 lines).
   - Tools Registered:
     - `redbida_list_catalog` (lines 25-76)
     - `redbida_get_keys` (lines 78-127)
     - `redbida_set_keys` (lines 129-176)
     - `redbida_apply_onboarding_preset` (lines 178-334)
     - `redbida_trigger_go2rtc` (lines 336-362)
     - `redbida_get_time_status` (lines 364-385)
   - Helper Utilities:
     - `removeVietnameseTones` (lines 387-434): Pure Go NFC/NFD accent stripping.
     - `sanitizeCleanTitle` (lines 436-446): Strips diacritics and non-alphanumerics for hashtags.
     - `generate20TabINITabs` (lines 448-455): Generates exactly 20 INI sections `[C01]` to `[C20]` with `vid_play_label=<title>`.
     - `sanitizeCSSGradient` (lines 457-467): Strips trailing semicolons and whitespace from CSS gradient.
     - `queryNTPSynchronized` (lines 469-483): Queries `timedatectl` with 2-second context timeout.

2. **Empirical Adversarial Test Execution**:
   - Test File: `/home/ksp/ksp-camera-auto/internal/mcp/tools_redbida_adversarial_test.go`
   - Command: `/home/ksp/go-sdk/bin/go test -v -race -run="TestAdversarial|TestRedbida|TestRemoveVietnamese|TestSanitize|TestGenerate" ./internal/mcp/...`
   - Results:
     ```
     === RUN   TestAdversarial_BrokerTimeout_ReadAndWrite
     --- PASS: TestAdversarial_BrokerTimeout_ReadAndWrite (0.12s)
     === RUN   TestAdversarial_BrokerAckTimeout_RecoveryAndFailure
     --- PASS: TestAdversarial_BrokerAckTimeout_RecoveryAndFailure (1.16s)
     === RUN   TestAdversarial_PartialAcks_And_CorruptedReadBack
     --- PASS: TestAdversarial_PartialAcks_And_CorruptedReadBack (0.61s)
     === RUN   TestAdversarial_ConfirmationEnforcement_And_ProtectedKeys
     --- PASS: TestAdversarial_ConfirmationEnforcement_And_ProtectedKeys (0.28s)
     === RUN   TestAdversarial_OnboardingPreset_ExtremeInputs
     --- PASS: TestAdversarial_OnboardingPreset_ExtremeInputs (0.00s)
     === RUN   TestAdversarial_ConcurrencyStress
     --- PASS: TestAdversarial_ConcurrencyStress (8.50s)
     === RUN   TestAdversarial_JSONRPC20_Integration
     --- PASS: TestAdversarial_JSONRPC20_Integration (0.00s)
     === RUN   TestRedbidaTools_ListCatalog
     --- PASS: TestRedbidaTools_ListCatalog (0.12s)
     === RUN   TestRedbidaTools_GetKeys
     --- PASS: TestRedbidaTools_GetKeys (4.29s)
     === RUN   TestRedbidaTools_SetKeys
     --- PASS: TestRedbidaTools_SetKeys (0.14s)
     === RUN   TestRedbidaTools_ApplyOnboardingPreset_DryRun
     --- PASS: TestRedbidaTools_ApplyOnboardingPreset_DryRun (0.00s)
     === RUN   TestRedbidaTools_ApplyOnboardingPreset_Live
     --- PASS: TestRedbidaTools_ApplyOnboardingPreset_Live (0.56s)
     === RUN   TestRedbidaTools_ApplyOnboardingPreset_Validations
     --- PASS: TestRedbidaTools_ApplyOnboardingPreset_Validations (0.00s)
     === RUN   TestRedbidaTools_TriggerGo2RTC
     --- PASS: TestRedbidaTools_TriggerGo2RTC (0.10s)
     === RUN   TestRedbidaTools_GetTimeStatus
     --- PASS: TestRedbidaTools_GetTimeStatus (0.01s)
     === RUN   TestRedbidaTools_DisabledServiceGracefulHandling
     --- PASS: TestRedbidaTools_DisabledServiceGracefulHandling (0.01s)
     PASS
     ok  	github.com/ngohuynhngockhanh/ksp-camera-auto/internal/mcp	16.961s
     ```

---

## 2. Logic Chain

1. **Broker Failure & Timeout Handling**:
   - *Observation*: In `TestAdversarial_BrokerTimeout_ReadAndWrite`, broker `Read` and `Write` failures returning `context.DeadlineExceeded` and network errors were tested.
   - *Inference*: Both `redbida_get_keys` and `redbida_set_keys` correctly capture the broker error and return structured `ToolResult` with `IsError=true` without leaking uninitialized state or panicking.
   - *Observation*: In `TestAdversarial_BrokerAckTimeout_RecoveryAndFailure`, when MQTT write ACK timed out (`redbida.AckTimeoutError`):
     - If the physical broker received the update, `readBack` confirmed matching state, recovering gracefully (`Applied: true`, `Verified: true`, `ReadBack: true`).
     - If the broker state did not update (stale state) or `readBack` failed, the operation failed closed (`Applied: false`, `Verified: false`, error clearly reported).
   - *Inference*: The read-back verification state machine in `Service.Apply` successfully guards against false positives during transient MQTT packet loss.

2. **Partial Write Failure & Corrupted Read-Back**:
   - *Observation*: In `TestAdversarial_PartialAcks_And_CorruptedReadBack`, a batch write of 3 keys was tested where the broker acked key 1, omitted ack for key 2, and returned stale/corrupted data during readback for key 3.
   - *Inference*: Key 1 succeeded (`Applied: true`, `Verified: true`), Key 2 failed with `"missing acknowledgement"`, and Key 3 failed with `"read-back mismatch"`. The tool isolates per-key outcomes accurately in the returned `ChangeResult` array.

3. **Confirmation & Risk Policy Enforcement**:
   - *Observation*: In `TestAdversarial_ConfirmationEnforcement_And_ProtectedKeys`, setting `RiskConfirm` keys (`max_free_ram_restart_camera`, `restart_camera_now`) with `confirmed: false` or omitted was immediately rejected with `"confirmation is required"`, and zero write calls were dispatched to the broker. Setting with `confirmed: true` succeeded. Setting read-only / protected keys (`frpc_config`) was rejected with `"key is read-only"`.
   - *Inference*: Security boundaries and confirmation policies are enforced at the service level before any broker transmission occurs.

4. **1-Click Onboarding Preset Synthesis**:
   - *Observation*: In `TestAdversarial_OnboardingPreset_ExtremeInputs`, complex inputs were tested:
     - Vietnamese venue title `"  CLB Bida Sài Gòn Đệ Nhất - CS3 & CS4 (Phú Nhuận) #2026 !  "` synthesized `#CLBBidaSaiGonDeNhatCS3CS4PhuNhuan2026 #BILLIARDSlive #INUTlive #highlightsports`.
     - Pure emoji title `"✨⭐🎉🚀"` fell back cleanly to `#BILLIARDSlive #INUTlive #highlightsports`.
     - CSS gradient trailing semicolons (`"linear-gradient(...); ; ; ; \t\n"`) were cleanly stripped.
     - 20-tab INI configuration contained exactly sections `[C01]` through `[C20]` with interpolated `vid_play_label`.
     - `cameraCount` boundaries (-10, 0, 21, 100) were rejected with `"cameraCount must be between 1 and 20"`.
     - DryRun mode synthesized all 15 parameters without writing to broker.
   - *Inference*: Onboarding preset synthesis conforms 100% to the Golden Template and RedBida naming skill specifications.

5. **Concurrency & Thread Safety**:
   - *Observation*: In `TestAdversarial_ConcurrencyStress`, 50 concurrent goroutines executing 500 mixed operations across all 6 RedBida tools ran under `-race` for 8.5 seconds with zero data races, panics, or deadlocks.
   - *Inference*: The implementation is concurrency-safe.

6. **Nil Service Resilience**:
   - *Observation*: `TestRedbidaTools_DisabledServiceGracefulHandling` verified that when `redbidaSvc == nil`, all tools return clear disabled messages without nil-pointer panics, while `redbida_get_time_status` continues functioning independently.

---

## 3. Caveats

1. **Live MQTT Broker**:
   - Tests were executed against comprehensive in-memory and mock broker implementations (`mockRedbidaBroker`, `flexibleMockBroker`). Live edge testing against actual edge nodes (`inut_204_164`, `inut_204_163`) and Node-RED :2023 will occur in Milestone 3.
2. **Note on Existing `server_test.go`**:
   - In `internal/mcp/server_test.go:291` (`TestServer_SSETransport`), `httptest.ResponseRecorder` buffer read in the test occurs concurrently with `ServeHTTP` writing in a goroutine. This is a pre-existing test fixture nuance in `server_test.go` (M2 scope) and does not affect the RedBida tools in `tools_redbida.go`.

---

## 4. Conclusion

**Verdict: APPROVE**

Milestone 1 (`internal/mcp/tools_redbida.go`) satisfies all requirements of §R1 and Milestone 1 of `PROJECT.md`:
- All 6 tools (`redbida_list_catalog`, `redbida_get_keys`, `redbida_set_keys`, `redbida_apply_onboarding_preset`, `redbida_trigger_go2rtc`, `redbida_get_time_status`) are correctly implemented and registered.
- Robust error handling for broker timeouts, partial writes, disconnects, and unconfirmed modifications.
- High concurrency tested with 50 workers under Go race detector with 0 data races.
- 100% pass rate on all unit and adversarial stress tests.

---

## 5. Verification Method

To independently reproduce and verify these empirical results:

```bash
# Run all RedBida unit and adversarial stress tests with race detection:
/home/ksp/go-sdk/bin/go test -v -race -run="TestAdversarial|TestRedbida|TestRemoveVietnamese|TestSanitize|TestGenerate" ./internal/mcp/...

# Run RedBida catalog & service tests:
/home/ksp/go-sdk/bin/go test -v -race ./internal/redbida/...
```
