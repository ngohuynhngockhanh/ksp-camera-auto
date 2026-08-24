# Progress Tracker — Milestone 4 Camera & Shinobi Provisioning

Last visited: 2026-08-24T16:45:30+07:00

## Status: COMPLETED

### Task Checklist
- [x] 1. Check connectivity to target host `inut_204_163` via controller `root@172.16.5.180`.
- [x] 2. Probe Dahua/KBVision NVR or cameras with Serial Number `AK0C842PAZ39A81` and test authentication (`admin:a12345678`).
- [x] 3. Identify active video channels (channels 1-5) and stream parameters.
- [x] 4. Update `/opt/ksp-cam/cameras.yaml` on target host with inventory entries for Camera01-05 and Dahua-NVR.
- [x] 5. Apply Golden Template Configuration strictly (mode record, hls, vcodec copy, record_vcodec copy, cust_record "-tag:v hvc1", empty cust_input/cust_stream, watchdog_reset 1, signal_check 10).
- [x] 6. Sync Shinobi NVR monitors (mid `camera01` to `camera05`, GroupKey `P6zP1kVhht`).
- [x] 7. Verify Shinobi monitor status via `http://127.0.0.1:2028/api/shinobi/monitors` and Shinobi API `http://127.0.0.1:8080`.
- [x] 8. Verify HLS streaming, recording to `/media/usb1`, snapshots, and no ffmpeg crashes.
- [x] 9. Generate handoff report and notify parent.
