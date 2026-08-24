# Plan — RedBida MCP Suite Expansion & Deployment

## Objectives
Deliver the complete RedBida & Onboarding MCP tools suite in `ksp-camera-auto`, integrate it into the server, achieve 100% test coverage, compile multi-arch binaries, deploy to live nodes (`inut_204_164`, `inut_204_163`), verify live MCP calls, and push git commit.

## Detailed Steps

### Step 0: Survey & Architecture Discovery
- Explorer 1 (`survey_mcp_core`): Survey `internal/mcp/` architecture, tools registration mechanism, tool handlers, error handling, session/stdio/SSE handling, existing tests.
- Explorer 2 (`survey_redbida_mqtt`): Survey MQTT topics, port 12369 or localhost config, `internal/` conventions, all 15 parameters format specifications for onboarding (tab format, hashtag normalization, background url, etc.), time sync NTP details.
- Explorer 3 (`survey_infra_deploy`): Survey `Makefile`, build targets (`make build-all`), remote nodes access / SSH / SCP / MCP testing endpoints (`inut_204_164`, `inut_204_163`), docs structure (`docs/`, `GEMINI.md`).

### Step 1: Synthesize into PROJECT.md
- Merge survey findings into `/home/ksp/ksp-camera-auto/PROJECT.md` with:
  - Architecture & Code Layout
  - Complete Feature Inventory (F1 to F8)
  - Milestone Definitions (M1, M2, M3)
  - Interface Contracts & Parameter Formats

### Step 2: Milestone 1 — RedBida & Onboarding MCP Tools Implementation
- Sub-orchestrator / Worker creates `internal/mcp/tools_redbida.go` containing:
  - `redbida_list_catalog`: Returns full metadata catalog, functional groups, risk level, data types.
  - `redbida_get_keys`: Queries MQTT `/private/i_gets` with `{"info": [...]}` and receives response with timeout/error handling.
  - `redbida_set_keys`: Publishes to `/private/i_sets` with `{"info": {...}}` and performs read-back verification.
  - `redbida_apply_onboarding_preset`: 1-Click calculation and application of 15 standard parameters (`ui_title`, `ui_bg`, `custom_hashtags`, `ui_tabs_links` 20 tab INI, `camera_count`, `toolbar_show_count`, `hls_using_go2rtc`, `button_generate_go2rtc_stream`, `logo_header`, `logo_header_text`, `shinobi_camera_id`, `shinobi_group_key`, `video_config`, `ui_scoreboard`, `ggcode`).
  - `redbida_trigger_go2rtc`: Publishes `button_generate_go2rtc_stream: "true"` to trigger Node-RED flow.
  - `redbida_get_time_status`: Checks system time & NTP sync status.
- Iteration loop: Explorer -> Worker -> Reviewers (2) -> Challengers (2) -> Auditor.

### Step 3: Milestone 2 — Server Integration, Documentation & Dual Transports
- Integrate new tools in `internal/mcp/server.go` `registerTools()`.
- Validate Stdio mode and HTTP/SSE mode on port :2028.
- Update documentation in `docs/` and `GEMINI.md` detailing new tools.
- Iteration loop: Explorer -> Worker -> Reviewers (2) -> Challengers (2) -> Auditor.

### Step 4: Milestone 3 — Comprehensive Testing, Multi-Arch Build & Remote Deployment
- Implement unit tests in `internal/mcp/tools_redbida_test.go` and `internal/mcp/server_test.go` (100% pass).
- Verify JSON-RPC 2.0 protocol compliance (`initialize`, `tools/list`, `tools/call`).
- Run `make build-all` to produce `linux/amd64`, `linux/arm64`, `linux/armv7` binaries.
- Deploy and live-verify on nodes `inut_204_164` and `inut_204_163`.
- Git commit & push.
- Review & Final Forensic Integrity Audit.

### Step 5: Final Review & Synthesis
- Final report to user/caller.
