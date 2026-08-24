## 2026-08-24T12:22:54Z

You are Worker M3 for Milestone 3 (Knowledge Hub, Preset Generator & Live Previews).
Your working directory is: /home/ksp/ksp-camera-auto/.agents/worker_m3/
Authoritative user request: /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md
Master project specification: /home/ksp/ksp-camera-auto/.agents/PROJECT.md
Frontend Survey Report: /home/ksp/ksp-camera-auto/.agents/explorer_survey_frontend/handoff.md
Knowledge Survey Report: /home/ksp/ksp-camera-auto/.agents/explorer_survey_knowledge/handoff.md
Worker M2 Handoff: /home/ksp/ksp-camera-auto/.agents/worker_m2/handoff.md

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Scope & File Ownership:
You EXCLUSIVELY own:
- `web/static/redbida.js`

Required Tasks:
1. Read /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md, the survey reports, and current `web/static/redbida.js`.
2. In `web/static/redbida.js`:
   - Implement **Preset / 1-Click Onboarding Generator** (`redbidaGeneratePreset`):
     * Read form inputs: `#redbida-preset-title`, `#redbida-preset-count`, `#redbida-preset-groupkey`, `#redbida-preset-bg`, `#redbida-preset-ggcode`.
     * Clean & sanitize hashtags using `removeVietnameseTones` + strip non-alphanumerics -> `#<CleanTitle> #BILLIARDSlive #INUTlive #highlightsports`.
     * Generate 20-tab INI `ui_tabs_links`: `[C01]` to `[C20]` with `vid_play_label = <ui_title>`.
     * Populate standard parameters:
       - `ui_title`: title
       - `company_name`: title
       - `ui_bg`: bg (cleaned of trailing semicolons)
       - `custom_hashtags`: hashtag string
       - `ui_tabs_links`: iniTabs
       - `camera_count`: count
       - `toolbar_show_count`: count
       - `video_config`: 'range=72'
       - `hls_using_go2rtc`: true
       - `hls_using_go2rtc_livestream`: true
       - `hls_using_go2rtc_tiktok`: true
       - `ui_scoreboard`: true
       - `logo_header`: 'https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png'
       - `logo_header_text`: 'Billiard Live - Tải clip bàn bida và livestream'
       - `button_generate_go2rtc_stream`: true
     * Set values into `redbidaState.drafts` and render visual diff preview card in `#redbida-preset-diff`.
   - Implement **Gradient Preset Swatches & Live Previews**:
     * Wire `.redbida-swatch` click events to set `#redbida-preset-bg` and update live preview `#redbida-preset-bg-preview`.
     * In table rows: for key `ui_bg`, render a live gradient swatch preview underneath the input.
   - Implement **Logo Live Preview with Checkerboard**:
     * Wire file input for `logo_header` / `logo_livestream` with max 512 KiB validation, base64 conversion, and live checkerboard thumbnail preview.
   - Implement **4-Pillar Filter Buttons & Quick Actions**:
     * Wire `.redbida-pillar-btn[data-filter-group]` to update `#redbida-group` dropdown and trigger table filtering.
     * Wire 1-Click Go2RTC stream generation quick action (`button_generate_go2rtc_stream`).
     * Wire Collapsible toggles `#redbida-toggle-preset` and `#redbida-toggle-hub`.
   - Ensure `#redbida-broker-status` and `#redbida-draft-count` update dynamically during state changes.
   - **CRITICAL**: Strictly preserve all existing functionality, event handlers, draft staging, read-back verification feedback, and 100% of Playwright test selectors.
3. Test and verify JavaScript syntax (`node --check web/static/redbida.js`), run Playwright tests (`npx playwright test tests/ui/redbida.spec.js`), and run Go unit tests (`go test ./...`). Ensure 100% pass with zero console errors.
4. Write completion report to `/home/ksp/ksp-camera-auto/.agents/worker_m3/handoff.md`.
5. Send completion message back to parent.
