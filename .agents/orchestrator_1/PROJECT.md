# Project: ksp-camera-auto Deployment & Integration on inut_204_163 and inut_204_164

## Architecture & Production Deployments

### 1. Target inut_204_164 ("CX King Luxury")
- **Host / IP**: `77.88.204.164` (ARM64 / Linux 6.1)
- **Venue Name**: `"CX King Luxury"` (configured in Redbida, Node-RED :2023, and `/root/ota-mqtt/change_ok/`)
- **Virtual IP**: `192.168.1.254/24` on `eth0`
- **kspcam Service**: Active (running) on `:2028` with `/opt/ksp-cam/config.yaml`
- **Central NVR**: Dahua `192.168.1.108:37777` (SN: `AK0C842PAZ39A81`, credentials: `admin:a12345678`)
- **5 Cameras**: `Camera01` (`192.168.1.201`) through `Camera05` (`192.168.1.205`) with credentials `admin:a12345678`
- **Shinobi NVR (:8080)**: 5 monitors (`camera01` to `camera05`) in mode `record` under Golden Template (`vcodec: copy`, `-tag:v hvc1`, audio copy)
- **Shinobi API Token**: IP restriction `0.0.0.0` synchronized to `/root/ota-mqtt/change_ok/shinobi_monitor_token`

### 2. Target inut_204_163 ("SD Billiards Club - CS2")
- **Host / IP**: `77.88.204.163` (ARM64 / Linux 6.1)
- **Venue Name**: `"SD Billiards Club - CS2"` (configured in Redbida, Node-RED :2023, and `/root/ota-mqtt/change_ok/`)
- **Virtual IP**: `192.168.1.254/24` on `eth0`
- **kspcam Service**: Active (running) on `:2028` with `/opt/ksp-cam/config.yaml`
- **8 Cameras**: `Camera01` (`192.168.1.111`) through `Camera08` (`192.168.1.118`) with credentials `admin:Sonduong1011@`
- **Shinobi NVR (:8080)**: 8 monitors (`camera01` to `camera08`) in mode `record` under Golden Template (`vcodec: copy`, `-tag:v hvc1`, audio copy)
- **Shinobi API Token**: IP restriction `0.0.0.0` synchronized to `/root/ota-mqtt/change_ok/shinobi_monitor_token`

## Feature Inventory & Acceptance Verification
| # | Feature | Description | Milestone | Status |
|---|---------|-------------|-----------|--------|
| 1 | Automated Service Deployment | Static ARM64 kspcam deployed, kspcam.service active on :2028 | M2 | DONE |
| 2 | Shinobi API Key & Token 0.0.0.0 | API keys & monitor tokens configured with 0.0.0.0 and saved to change_ok | M2 | DONE |
| 3 | Redbida MQTT & Key Catalog | Connected to 127.0.0.1:12369 MQTT & /root/ota-mqtt/change_ok catalog | M3 | DONE |
| 4 | Venue Name Branding | Configured venue names ("CX King Luxury" / "SD Billiards Club - CS2") | M3 | DONE |
| 5 | Virtual IP Binding | 192.168.1.254/24 bound on eth0 on both target hosts | M3 | DONE |
| 6 | Redbida REST Endpoints | /api/redbida/catalog & /api/redbida/refresh return 200 OK without 500 error | M3 | DONE |
| 7 | Node-RED Integration | Bi-directional parameter synchronization with Node-RED (:2023) | M3 | DONE |
| 8 | NVR & Camera Probing | Probed Dahua NVR (AK0C842PAZ39A81) and camera channels | M4 | DONE |
| 9 | Golden Template Application | 100% Golden Template compliance (remux copy, -tag:v hvc1, empty cust flags) | M4 | DONE |
| 10 | Shinobi Monitor Sync & Video Storage | Synced monitors to Shinobi :8080, storage on /media/usb1 verified | M4 | DONE |
| 11 | Acceptance Criteria Full Pass | All Acceptance Criteria verified and validated | M5 | DONE |
| 12 | Forensic Integrity Audit | Systematic verification confirming authentic production implementation | M5 | DONE |

## Milestones Summary
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | Survey & Discovery | Target hosts, Node-RED, MQTT, Shinobi, Cameras | None | DONE |
| 2 | Build & Deployment | Cross-compile ARM64 kspcam, deploy /opt/ksp-cam, systemd service | M1 | DONE |
| 3 | Redbida & Platform Integration | MQTT 12369, catalog change_ok, venue name, virtual IP, Node-RED | M2 | DONE |
| 4 | Camera Setup & Golden Template | Probe NVRs/cameras, Golden Template, sync Shinobi :8080 | M2, M3 | DONE |
| 5 | E2E Verification & Forensic Audit | Verification matrix and acceptance criteria sign-off | M1, M2, M3, M4 | DONE |
