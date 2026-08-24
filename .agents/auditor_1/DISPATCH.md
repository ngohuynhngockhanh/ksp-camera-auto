# Auditor 1 Dispatch

## 2026-08-24T09:46:04Z

Perform independent forensic integrity verification on `inut_204_163` and the local workspace:
1. Static analysis & Source code integrity:
   - Check that no test results or fake responses are hardcoded.
   - Verify `kspcam` binary on target is authentic compilation of the Go codebase.
2. Runtime tracing & Live system verification on target `inut_204_163`:
   - Inspect live systemd service `kspcam.service`, configuration `/opt/ksp-cam/config.yaml`, `/opt/ksp-cam/cameras.yaml`.
   - Inspect MariaDB `ccio.API` and `ccio.Monitors` tables on Shinobi NVR.
   - Inspect files in `/root/ota-mqtt/change_ok/` (`company_name`, `logo_header_text`, `ui_title`, `eth0_virtual_ip`, `shinobi_monitor_token`).
   - Inspect virtual IP binding on `eth0`.
   - Inspect live Dahua NVR at `192.168.1.150:37777` (SN `AK0C842PAZ39A81`).
   - Ensure all acceptance criteria are genuinely met without facades, mocks, or shortcuts.

Produce a comprehensive Forensic Audit Report and explicit verdict (CLEAN or INTEGRITY VIOLATION) in `/home/ksp/ksp-camera-auto/.agents/auditor_1/handoff.md`. Send message to parent orchestrator when complete.
