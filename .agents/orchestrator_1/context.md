# Context & System Specifications

## Target Device Specification
- Host / IP: `inut_204_163` (`77.88.204.163`)
- Access: SSH root access or local network bridges
- Services on target:
  - Shinobi NVR: `http://127.0.0.1:8080` (or `http://77.88.204.163:8080`)
  - Node-RED: port `2023`
  - MQTT Broker: `127.0.0.1:12369`
  - Key catalog path: `/root/ota-mqtt/change_ok`
  - KSP-Cam Web & MCP Service: port `2028` (`/opt/ksp-cam`)

## Local Workspace
- Path: `/home/ksp/ksp-camera-auto`
- Codebase: Pure Go `kspcam`, `cmd/kspcam`, `internal/...` (redbida, shinobi, mcp, camera, bulk, isapi, dahua, etc.)
- Skills: `/home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md`

## Integration Architecture
1. **KSP-Cam <-> Shinobi NVR**:
   - REST API on port 8080 with APIKey and GroupKey.
   - Monitors created with Golden Template configuration (`camera01`..`camera08`).
2. **KSP-Cam <-> Node-RED (redbida)**:
   - MQTT topics: `/private/i_gets`, `/private/i_sets` on broker `127.0.0.1:12369`.
   - Key catalog: JSON files in `/root/ota-mqtt/change_ok`.
   - Web API endpoints: `/api/redbida/catalog`, `/api/redbida/refresh`.
3. **KSP-Cam <-> Cameras**:
   - IP range: `192.168.1.190` - `192.168.1.197`.
   - Protocol: Dahua/KBVision DVRIP (TCP 37777) / ISAPI / RTSP 554.
   - Golden Template: remux copy, `-tag:v hvc1`, empty input/stream flags, audio AAC copy/no.
