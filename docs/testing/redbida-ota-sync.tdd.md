# RedBida / OTA-MQTT Sync TDD Evidence

## RED

- Catalog tests defined the reviewed allowlist, fail-closed handling for unknown and protected keys, secret masking, and discovery from `/root/ota-mqtt/change_ok`.
- MQTT tests defined exact-key acknowledgement matching, retained-message rejection, timeout behavior, and read-back verification before a write can be reported as applied.
- Service and handler tests defined type validation, batch limits, confirmation for dangerous keys, partial results, role authorization, and unavailable-source behavior.
- Playwright tests defined grouped editing, protected fields, real image upload, stale JSON draft rejection, compact data-URL rendering, partial apply handling, and duplicate-submit prevention on desktop and mobile.

## GREEN

- `internal/redbida` implements the catalog, validation, MQTT transport, acknowledgement filtering, and mandatory read-back verification.
- `internal/server/api_redbida.go` exposes catalog, refresh, apply, and time-status endpoints only when RedBida is explicitly enabled.
- `web/static/redbida.js` provides the grouped operator form without calling, editing, or restarting Node-RED.
- Runtime writes use the local OTA broker at `127.0.0.1:12369`; Node-RED port `2023` remains survey-only.

## Verification

- `/home/ksp/go-sdk/bin/go test ./...`: pass.
- `/home/ksp/go-sdk/bin/go vet ./...`: pass.
- `/home/ksp/go-sdk/bin/go test ./internal/redbida -cover`: pass, `80.4%` statement coverage.
- `node --check web/static/redbida.js` and `node --check web/static/app.js`: pass.
- `npx --no-install playwright test tests/ui/redbida.spec.js --workers=1`: `14 passed` across desktop and mobile.
- Full Playwright regression after the final UI gate: `105 passed`, `11 skipped`, `116 total`.
- `npm audit --omit=dev`: `0 vulnerabilities`.
- `git diff --check`: pass.

## Live Acceptance: `inut_204_163`

- Release `3e58415-redbida-ui-gate-20260824`, static ARM64 SHA-256 `cfe0c016039f3f61f90d1341fc29aca9d2627f8f43303a11ed2c2a5c7df8c2c2`.
- Ansible `make ksp-bida inut_204_163`: `failed=0`; `kspcam.service` active/enabled with zero restarts.
- Local health endpoint and port `2028` are healthy; Node-RED `2023` returns HTTP 200; OTA MQTT `12369` is listening.
- Catalog reports `130` keys with source available and NTP synchronized.
- Applying `logo_header` returned `acknowledged=true`, `verified=true`, `applied=true`, and the following refresh reported the key as existing.
- Node-RED PID `12338` and active-project flow hash `f8d59f17d8d903e9033b897b7df5e4e07eace0c9eb713fcea4025b6688fd16c8` for `/root/.node-red/projects/ok2/flow.json` remained unchanged across the final rollout.
- OTA PID `14057` and source hash `7893d9c72290fd6c4cf82f2c0144d55a02bb313313e2e7013d1fca8402cf5930` remained unchanged.
- Public HTTP health check at `ksp-cam-inut-204-163.video.io.vn/healthz` returns 200. HTTPS currently returns 404 at the central ingress and requires separate TLS routing before credentials should be used over the public endpoint.

The repeatable authenticated probe is `plan-sync-redbida/remote_acceptance.py`; it deliberately avoids printing protected values.
