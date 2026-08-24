# Task Assignment: Milestone 1 — RedBida & Onboarding MCP Tools Implementation

Target file: `/home/ksp/ksp-camera-auto/internal/mcp/tools_redbida.go`

Read:
- `/home/ksp/ksp-camera-auto/ORIGINAL_REQUEST.md`
- `/home/ksp/ksp-camera-auto/PROJECT.md`
- `/home/ksp/ksp-camera-auto/.agents/explorer_mcp_core/handoff.md`
- `/home/ksp/ksp-camera-auto/.agents/explorer_redbida_spec/handoff.md`

Implement:
1. `redbida_list_catalog`: Returns catalog keys with group, risk classification (editable, confirm-required, protected), type, and description.
2. `redbida_get_keys`: Queries MQTT `/private/i_gets` with `{"info": [...]}` and masks secrets.
3. `redbida_set_keys`: Publishes to `/private/i_sets` with `{"info": {...}}` and enforces read-back verification.
4. `redbida_apply_onboarding_preset`: 1-Click calculation and application of 15 standard parameters (`ui_title`, `ui_bg` without semicolon, `custom_hashtags` normalized without diacritics, `ui_tabs_links` 20-tab INI `[C01]`-`[C20]`, `camera_count`, `toolbar_show_count`, `hls_using_go2rtc`, `button_generate_go2rtc_stream`, `logo_header`, `logo_header_text`, `shinobi_camera_id`, `shinobi_group_key`, `video_config`, `ui_scoreboard`, `ggcode`).
5. `redbida_trigger_go2rtc`: Publishes `button_generate_go2rtc_stream: "true"` to trigger Node-RED flow.
6. `redbida_get_time_status`: Checks system time & NTP sync status via timedatectl.
7. Helper function `registerRedbidaTools(r *Registry, cfg *config.Config, redbidaSvc *redbida.Service)`.

Run build/tests via Go at `/home/ksp/go-sdk/bin/go` and report in `/home/ksp/ksp-camera-auto/.agents/worker_m1/handoff.md`.
