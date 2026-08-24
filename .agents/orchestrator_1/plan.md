# Master Execution Plan: ksp-camera-auto Deployment & Integration on inut_204_163

## Objective
Triển khai hoàn chỉnh `ksp-camera-auto` lên thiết bị `inut_204_163` (77.88.204.163), tích hợp với Node-RED (:2023) qua module `redbida` (MQTT :12369 / change_ok catalog) và Shinobi NVR (:8080), probe và cấu hình 8 camera với Golden Template, kiểm tra sức khỏe và bàn giao.

## Milestones Breakdown

### Milestone 1: Comprehensive Survey & Discovery
- **Scope**:
  - Local codebase survey: redbida module implementation, config schema, Shinobi client, build scripts.
  - Target system survey (`inut_204_163` / 77.88.204.163): SSH access, OS architecture (amd64/arm), existing services (Node-RED on 2023, MQTT broker on 12369, Shinobi NVR on 8080, `/root/ota-mqtt/change_ok`, systemd services).
  - Network & Camera environment survey on target LAN (192.168.1.x, camera IPs, credentials, ONVIF/DVRIP/ISAPI access).
- **Deliverables**: Survey synthesis report with full environment status and dependency roadmap.

### Milestone 2: Build & Target Deployment
- **Scope**:
  - Compile static binary `kspcam` for target architecture.
  - Prepare target configuration (`/opt/ksp-cam/config.yaml` or relevant path) with encrypted storage, Shinobi credentials, redbida settings, MCP server.
  - Setup and verify `kspcam.service` on target `inut_204_163:2028`.
- **Exit Gate**: `kspcam.service` active and running on `inut_204_163`, web UI accessible on port 2028, healthz 200 OK.

### Milestone 3: Redbida & Node-RED (:2023) Integration
- **Scope**:
  - Verify MQTT broker `127.0.0.1:12369` on target with `/private/i_gets` and `/private/i_sets`.
  - Verify and configure key catalog `/root/ota-mqtt/change_ok`.
  - Validate endpoints `/api/redbida/catalog` and `/api/redbida/refresh`.
  - Verify bi-directional sync/update between KSP-Cam Web UI / API and Node-RED project.
- **Exit Gate**: Endpoints return valid data with no 500 errors, MQTT test round-trip passes, Node-RED integration verified.

### Milestone 4: Camera Setup, Golden Template & Shinobi Monitor Sync
- **Scope**:
  - Probe target cameras (192.168.1.190-197 or detected cameras).
  - Apply Camera Naming convention (`Camera01`..`Camera08`, `mid: camera01..camera08`).
  - Apply Golden Template settings: remux copy, audio AAC copy/no, `-tag:v hvc1`, empty cust_input/cust_stream, watchdog flags.
  - Synchronize monitors to Shinobi NVR (:8080) and verify stream states (`start`/`record`).
  - Test video playback and recording health.
- **Exit Gate**: 8 cameras configured, Shinobi monitors active, streams and recordings healthy.

### Milestone 5: E2E Verification & Forensic Integrity Audit
- **Scope**:
  - Comprehensive verification against all Acceptance Criteria in ORIGINAL_REQUEST.md.
  - Challenger stress test & edge case verification.
  - Forensic integrity audit (`teamwork_preview_auditor`).
  - Generate final handover report for human/sentinel review.
- **Exit Gate**: All acceptance criteria satisfied, Reviewers APPROVE, Auditor CLEAN.
