# Worker 164 Execution Dispatch

## 2026-08-24T09:47:22Z

You are teamwork_preview_worker (Lead Deployment & Integration Worker for inut_204_164 / CX King Luxury).
Working directory: /home/ksp/ksp-camera-auto/.agents/worker_164_execution
Authoritative request: /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md
Skill documentation: /home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

CRITICAL INSTRUCTION:
The customer confirmed the real target device for "CX King Luxury" is `inut_204_164` (IP: `77.88.204.164` via Ansible controller `root@172.16.5.180`).
Execute all tasks directly on `inut_204_164` (`77.88.204.164`).

Step-by-step Objectives:
1. Target Inspection & Binary Deployment on `inut_204_164`:
   - Verify connection: `ssh root@172.16.5.180 "ssh root@77.88.204.164 'uname -a'"`
   - Deploy latest static ARM64 `kspcam` binary to `/opt/ksp-cam/kspcam` on `inut_204_164`.
   - Setup `/opt/ksp-cam/config.yaml` on `inut_204_164` (enable Shinobi, Redbida 127.0.0.1:12369 / change_ok, MCP :2028).
   - Setup and start `kspcam.service` on `inut_204_164`.

2. Shinobi API Key & Token (0.0.0.0 IP restriction):
   - Check Shinobi NVR on `inut_204_164` (:8080, MariaDB `ccio`), find user & groupKey.
   - Create or update API Key in `ccio.API` with `ip = "0.0.0.0"` and permissions (view streams, get videos).
   - Save `shinobi_monitor_token` (also allowing `0.0.0.0`) into `/root/ota-mqtt/change_ok/shinobi_monitor_token`.
   - Update `/opt/ksp-cam/config.yaml` with the Shinobi API Key & groupKey, restart `kspcam.service`, verify `GET http://127.0.0.1:2028/api/shinobi/status` returns `connected: true`.

3. Venue Name ("CX King Luxury") & Node-RED / Redbida:
   - Set venue name to "CX King Luxury" in `/root/ota-mqtt/change_ok/` (`company_name`, `logo_header_text`, `ui_title`).
   - Publish to MQTT broker `127.0.0.1:12369` (topic `/private/i_sets`) or via `POST /api/redbida/apply`.
   - Verify `GET /api/redbida/catalog` (130 keys) and `POST /api/redbida/refresh` respond with 200 OK without 500 errors.

4. Virtual IP Binding:
   - Ping check on LAN of `inut_204_164` for `192.168.1.254`.
   - If free, add virtual IP `192.168.1.254/24` to interface `eth0`. If not free, try `.253`.
   - Save to `/root/ota-mqtt/change_ok/eth0_virtual_ip` and verify ping response.

5. Dahua/KBVision NVR Probe & 5-Camera Golden Template:
   - Probe the LAN subnet of `inut_204_164` to find the Dahua/KBVision NVR with Serial Number `AK0C842PAZ39A81` (pass: `a12345678`).
   - Identify 5 channels and configure in `/opt/ksp-cam/cameras.yaml` (`Camera01`..`Camera05`, `mid: camera01`..`camera05`).
   - Apply Golden Template strictly:
     - `mode: "record"`, `stream_type: "hls"`
     - `stream_vcodec: "copy"`, `record_vcodec: "copy"`, `vcodec: "copy"` (0% CPU remux)
     - `cust_input: ""` (empty), `cust_stream: ""` (empty)
     - `cust_record: "-tag:v hvc1"`
     - Audio: `acodec: "no"`, `stream_acodec: "no"`, `record_acodec: "no"` (or copy/aac if mic enabled)
     - `watchdog_reset: "1"`, `signal_check: "10"`
   - Sync 5 monitors to Shinobi NVR (`http://127.0.0.1:8080`, GroupKey).
   - Verify `GET /api/shinobi/monitors` returns all 5 monitors.
   - Verify HLS streams, snapshots, and disk storage recording on target.
