# Progress — Reviewer 2 (Camera & Shinobi Golden Template Reviewer)

- **Status**: IN_PROGRESS
- **Last visited**: 2026-08-24T16:46:40+07:00
- **Target Host**: `inut_204_163` (`77.88.204.163` via `root@172.16.5.180`)

## Checklist & Status
- [x] Initial dispatch received and parsed
- [x] BRIEFING initialized
- [ ] 1. Verify Dahua NVR at `192.168.1.150:37777`, SN `AK0C842PAZ39A81`, pass `a12345678`
- [ ] 2. Verify 5 cameras (`Camera01` - `Camera05`, `mid: camera01` - `camera05`) in `/opt/ksp-cam/cameras.yaml` & `GET /api/cameras`
- [ ] 3. Verify Golden Template compliance in Shinobi NVR (`http://127.0.0.1:8080`, GroupKey `P6zP1kVhht`):
  - `mode: record`, `stream_type: hls`
  - `stream_vcodec: copy`, `record_vcodec: copy`, `vcodec: copy`
  - `cust_record: "-tag:v hvc1"`
  - `cust_input: ""`, `cust_stream: ""`
  - `acodec: "no"`, `stream_acodec: "no"`, `record_acodec: "no"`
  - `watchdog_reset: "1"`, `signal_check: "10"`
- [ ] 4. Verify Shinobi status endpoint: `GET http://127.0.0.1:2028/api/shinobi/status` (`configured: true`, `connected: true`, `monitorCount: 5`)
- [ ] 5. Verify `/media/usb1` storage directory structure for recordings
- [ ] 6. Adversarial Review & Integrity Checks (stress tests, edge cases, failure modes)
- [ ] 7. Generate final `handoff.md` and send completion message to parent
