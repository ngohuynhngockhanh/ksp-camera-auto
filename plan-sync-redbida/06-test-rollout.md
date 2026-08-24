# Test and Rollout

Validation:

- Go unit tests for catalog, redaction, validation and ack mapping;
- server handler tests for catalog/refresh/apply;
- Playwright tests for grouping, protected keys, logo upload and submit;
- full `go test ./...`, `go vet ./...`, JavaScript syntax and Playwright suite.

Deployment target is the clean customer box `inut_204_163`, using the existing
`make ksp-bida` Ansible path. `inut_204_63` remains read-only reference evidence.
Post-deploy checks must verify kspcam health/version, Node-RED port 2023 remains
active, MQTT refresh/apply/read-back works, and Node-RED flow hashes do not
change.

## Completed acceptance on `inut_204_163`

- Release: `3e58415-redbida-ui-gate-20260824` (static ARM64).
- Binary SHA-256: `cfe0c016039f3f61f90d1341fc29aca9d2627f8f43303a11ed2c2a5c7df8c2c2`.
- Ansible result: `failed=0`; `kspcam.service` active/enabled, `NRestarts=0`.
- Local services: `2028` healthy, Node-RED `2023` HTTP 200, OTA broker `12369`
  listening.
- RedBida refresh: `130` catalog keys, source available, NTP synchronized.
- Live apply: `logo_header` acknowledged, read-back verified and persisted;
  existing key count increased from 9 to 10.
- Node-RED PID `12338` and active-project flow hash for
  `/root/.node-red/projects/ok2/flow.json`,
  `f8d59f17d8d903e9033b897b7df5e4e07eace0c9eb713fcea4025b6688fd16c8`,
  remained unchanged.
- OTA PID `14057` and source hash
  `7893d9c72290fd6c4cf82f2c0144d55a02bb313313e2e7013d1fca8402cf5930`
  remained unchanged.
- Public HTTP health returns 200 at
  `ksp-cam-inut-204-163.video.io.vn/healthz`; central HTTPS routing currently
  returns 404 and remains a separate ingress/TLS follow-up.

The Shinobi provisioning template on the controller hit its existing rescue
path, but the standalone `kspcam` deployment and RedBida acceptance completed
successfully. `inut_204_63` was not modified.
