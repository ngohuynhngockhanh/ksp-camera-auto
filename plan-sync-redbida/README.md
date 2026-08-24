# RedBida / OTA-MQTT Sync

This folder records the implementation contract for the RedBida settings
console. Node-RED on port `2023` is survey-only. Runtime writes go through the
local `ota-mqtt` MQTT bridge on `127.0.0.1:12369`.

Implementation entrypoints:

- Backend: `internal/redbida/`, `internal/server/api_redbida.go`
- Config: `internal/config/config.go`
- UI: `web/static/redbida.js`, `web/static/index.html`
- Tests: `internal/redbida/*_test.go`, `internal/server/api_redbida_test.go`,
  `tests/ui/redbida.spec.js`
- Acceptance: `remote_acceptance.py`, launched on a deployed box with
  `run_acceptance_from_config.sh` so credentials do not appear in command logs.
