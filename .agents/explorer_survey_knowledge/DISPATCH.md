## 2026-08-24T11:54:13Z
You are Explorer 3 (Knowledge Hub & Onboarding Flow Spec).
Your working directory is: /home/ksp/ksp-camera-auto/.agents/explorer_survey_knowledge/
Authoritative user request: /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md
Codebase guidelines: /home/ksp/ksp-camera-auto/AGENTS.md, /home/ksp/ksp-camera-auto/GEMINI.md, and skills in `.agents/skills/` (like camera-naming).

Your Mission:
Thoroughly explore the domain knowledge, rules, and formulas for the RedBida Onboarding Hub:
1. Read /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md completely.
2. Investigate the 4 core Onboarding Pillars:
   - Pillar 1 (Branding & Giao diện Quán): `ui_title`, `ui_bg`, `logo_header`, `logo_header_text`, `logo_livestream`, `ui_scoreboard`, `ui_tabs_links` (20-tab INI format specs), `custom_hashtags` (`#<UITitle> #BILLIARDSlive #INUTlive #highlightsports`).
   - Pillar 2 (Video Streaming & Go2RTC Engine): `camera_count`, `toolbar_show_count`, `video_config` (`range=72`), `hls_using_go2rtc` (`true`), `button_generate_go2rtc_stream`.
   - Pillar 3 (Shinobi NVR Authentication & Group Sync): `shinobi_camera_id`, `shinobi_group_key`, `shinobi_token`, `shinobi_monitor_token`, Golden Template parameters (`cutoff: 5`, `copy` remux).
   - Pillar 4 (Hệ thống & An ninh): `frpc_config`, `ggcode` (`G-SFSDZPR95Z`), NTP/Time sync, RAM Watchdogs.
3. Define the precise algorithm and formula for the Preset / One-Click Onboarding Generator:
   - Inputs: Tên Quán (`ui_title`), Số Camera (`camera_count`), Shinobi Group Key (`shinobi_group_key` / `shinobi_camera_id`).
   - Output Key-Value map to be applied via `/private/i_sets`.
4. Investigate documentation, help files, and existing code in the repository to ensure all standard constants and formulas match real-world field specifications.
5. Write a comprehensive survey report to `/home/ksp/ksp-camera-auto/.agents/explorer_survey_knowledge/handoff.md` and send a completion message with summary back to parent.
