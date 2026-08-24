# Dispatch for Explorer Survey Backend

- Target: Investigate Go backend for Redbida in internal/redbida, internal/server, internal/config, and related test files.
- Deliverable: handoff.md in this directory.

## 2026-08-24T11:54:13Z
Mission:
Thoroughly explore the backend implementation related to RedBida:
1. Read /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md completely.
2. Investigate `internal/redbida/` (`catalog.go`, `client.go`, `types.go`, `redbida_test.go`, etc.) and `internal/server/` (`redbida_handlers.go`, `server.go`, `routes.go`, etc.).
3. Analyze MQTT structure (`ota-mqtt`), topic `/private/i_sets`, `/private/i_gets`, key definitions, groups, read-back verification, and validation rules.
4. Identify missing keys and metadata required by R1 and R2 (e.g., `logo_header`, `logo_header_text`, `ui_bg`, `camera_count`, `toolbar_show_count`, `hls_using_go2rtc`, `button_generate_go2rtc_stream`, `ggcode`, etc.).
5. Check existing test coverage for `internal/redbida` and `internal/server`.
6. Write a comprehensive survey report to `/home/ksp/ksp-camera-auto/.agents/explorer_survey_backend/handoff.md` and send a completion message with summary back to parent.
