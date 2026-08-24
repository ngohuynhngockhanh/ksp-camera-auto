## 2026-08-24T09:26:38Z
You are teamwork_preview_worker (Camera & Shinobi Provisioning Worker for Milestone 4).
Working directory: /home/ksp/ksp-camera-auto/.agents/worker_camera_m4
Authoritative request: /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md
Skill documentation: /home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Target Host: `inut_204_163` (`77.88.204.163` via Ansible controller `root@172.16.5.180`)

Objectives:
1. Probe network for Dahua/KBVision NVR or cameras with Serial Number: `AK0C842PAZ39A81` and password `a12345678`:
   - Check `192.168.1.150` (or `192.168.1.3` / `192.168.1.111-118` / DHDiscover / DVRIP port 37777).
   - Test authentication with `admin:a12345678` (or `admin:Admin123456` / `admin:smarthome12345`).
   - Identify active video channels (channels 1 to 5).
2. Setup and Configure 5 Cameras (`Camera01` through `Camera05`):
   - Monitor IDs: `camera01`, `camera02`, `camera03`, `camera04`, `camera05`.
   - Names: `Camera01`, `Camera02`, `Camera03`, `Camera04`, `Camera05`.
   - Update `/opt/ksp-cam/cameras.yaml` on target host with inventory entries.
3. Apply Golden Template Configuration strictly:
   - `mode`: `"record"`
   - `stream_type`: `"hls"`
   - `stream_vcodec`: `"copy"`
   - `vcodec`: `"copy"`
   - `record_vcodec`: `"copy"`
   - `rtsp_transport`: `"tcp"`
   - `preset_stream`: `"ultrafast"`
   - `hls_time`: `"2"`
   - `hls_list_size`: `"2"`
   - `cust_input`: `""` (MUST BE EMPTY)
   - `cust_stream`: `""` (MUST BE EMPTY)
   - `cust_record`: `"-tag:v hvc1"` (MANDATORY for H.265)
   - Audio: If audio supported, `acodec: "copy"`, `stream_acodec: "copy"`, `record_acodec: "aac"`. If disabled, `"no"`.
   - `watchdog_reset`: `"1"`, `signal_check`: `"10"`
4. Shinobi NVR Synchronization:
   - Sync the 5 monitors into Shinobi NVR (`http://127.0.0.1:8080`, GroupKey `P6zP1kVhht`) via Shinobi REST API or `kspcam` `/api/shinobi/sync-to-shinobi` / `/api/shinobi/monitors`.
   - Verify `GET http://127.0.0.1:2028/api/shinobi/monitors` returns the 5 monitors (`camera01` to `camera05`).
5. Video Stream & Recording Verification:
   - Verify all 5 monitors are active (`mode: record`).
   - Check that video streams generate HLS segments without ffmpeg crashes.
   - Verify recordings save to `/media/usb1`.
   - Test snapshot / live stream response for each camera.

Write your detailed report and `handoff.md` in `/home/ksp/ksp-camera-auto/.agents/worker_camera_m4/handoff.md`. Send message to parent orchestrator when complete.
