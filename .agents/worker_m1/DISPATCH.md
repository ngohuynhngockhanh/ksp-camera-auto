## 2026-08-24T11:57:39Z

You are Worker M1 for Milestone 1 (Backend Catalog & Metadata Refinements).
Your working directory is: /home/ksp/ksp-camera-auto/.agents/worker_m1/
Authoritative user request: /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md
Master project specification: /home/ksp/ksp-camera-auto/.agents/PROJECT.md
Backend Survey Report: /home/ksp/ksp-camera-auto/.agents/explorer_survey_backend/handoff.md
Knowledge Survey Report: /home/ksp/ksp-camera-auto/.agents/explorer_survey_knowledge/handoff.md

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Scope & File Ownership:
You EXCLUSIVELY own:
- `internal/redbida/catalog.go`
- `internal/redbida/redbida_test.go`
- `internal/server/api_redbida_test.go`

Required Tasks:
1. Read /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md, the survey reports, and current files.
2. In `internal/redbida/catalog.go`:
   - Remove `toolbar_show_count` from `runtimeKeyRe` (line 13).
   - Add `toolbar_show_count` to `editableKeySet`, `numberKeySet`, and `numericRules` (`[0, 4096]`, `integer: true`).
   - Remove `ui_tabs_links` and `custom_hashtags` from `jsonKeySet` (line 94) so they default to `TypeString` (allowing multiline INI format for `ui_tabs_links` and string hashtags without JSON parse rejection).
   - Add `shinobi_group_key` to `fallbackKeys` list.
   - Refine `metaForKey` grouping logic to classify keys into intuitive domain groups:
     * `"Branding / Logo"`: `logo_header`, `logo_header_text`, `logo_livestream`, `logo_cat_cam`, `company_name`, `banner_top`, `custom_hashtags`, `app_*`.
     * `"Livestream"`: `camera_count`, `toolbar_show_count`, `video_config`, `hls_using_go2rtc`, `hls_using_go2rtc_livestream`, `hls_using_go2rtc_tiktok`, `button_generate_go2rtc_stream`, `default_delay_*`, `fps_default`, `livestream_default_bitrate`, `place_livestream`.
     * `"UI / Display"`: `ui_title`, `ui_bg`, `ui_scoreboard`, `ui_tabs_links`, `ui_css_custom`, `ui_title_color`, `ui_download_text`, `ui_fb`, `ui_zalo`, `ui_tiktok`, `ui_google`, `ui_phone`, `language`, `show_toolbar`, `large_monitor`, `help_link`, `url_live_help`, `default_tiso_*`, `shop_id`, `realtime_shop_id`.
     * `"Schedule / Maintenance"`: `stop_camera_*`, `button_reboot`, `button_restart_shinobi`, `max_free_ram_*`, `max_shared_ram_*`, `db_check_*`, `watch_uptime_process`.
     * `"Security / Credentials"`: `sensitiveKeyRe` matches (including `shinobi_*`, `ggcode`, `frpc_config`).
3. In `internal/redbida/redbida_test.go` and `internal/server/api_redbida_test.go`:
   - Add unit tests verifying `toolbar_show_count` (editable number), `custom_hashtags` (string type and Branding group), `ui_tabs_links` (string type and UI / Display group accepting multiline INI), `shinobi_group_key` (fallback presence), and new grouping classifications.
   - Run tests: `/home/ksp/go-sdk/bin/go test -v ./internal/redbida/... -cover` and `/home/ksp/go-sdk/bin/go test -v ./internal/server/...` and `/home/ksp/go-sdk/bin/go test ./...`. Ensure 100% pass.
4. Write completion report to `/home/ksp/ksp-camera-auto/.agents/worker_m1/handoff.md` including exact commands run, test outputs, and diff summary.
5. Send completion message back to parent.
