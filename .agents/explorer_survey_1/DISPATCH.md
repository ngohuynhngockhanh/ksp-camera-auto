## 2026-08-24T09:08:26Z

You are teamwork_preview_explorer (Survey Specialist 1: Local Codebase & Modules).
Working directory: /home/ksp/ksp-camera-auto/.agents/explorer_survey_1
Authoritative request: /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md

Objective:
Investigate the local codebase in /home/ksp/ksp-camera-auto to map all components relevant to:
1. `redbida` integration: Look into how redbida is structured, package files, MQTT broker connection details (127.0.0.1:12369, /private/i_gets, /private/i_sets), key catalog (/root/ota-mqtt/change_ok), REST APIs (/api/redbida/catalog, /api/redbida/refresh, /api/redbida/set), and how it coordinates with config.
2. `shinobi` integration: Look into `internal/shinobi`, API key/group key handling, monitor CRUD, sync routes, and config schema.
3. Build and deployment requirements: Look at Makefile, go.mod, cmd/kspcam, flags, systemd service definitions, configuration schema in `internal/config`.

Scope boundaries:
- Read-only exploration. DO NOT modify source code files.
- Produce a structured exploration report in `/home/ksp/ksp-camera-auto/.agents/explorer_survey_1/survey_codebase.md` and write your `handoff.md`.

Completion criteria:
- Complete report on redbida, shinobi, config, and build options with exact source code references. Send message to orchestrator when done.
