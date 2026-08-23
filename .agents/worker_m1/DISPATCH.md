## 2026-08-23T16:35:56Z
You are teamwork_preview_worker implementing Milestone M1: Ansible Automated Shinobi Provisioning & Config (Requirement R1).
Your working directory is: /home/ksp/ksp-camera-auto/.agents/worker_m1/
Read /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md and /home/ksp/ksp-camera-auto/PROJECT.md before doing anything. Also read /home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_ansible/report.md for exact findings, API payloads, and Ansible role layout.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A reviewer/auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Scope & Tasks:
1. Update Go configuration in `/home/ksp/ksp-camera-auto/internal/config/config.go` and `config.example.yaml`:
   - Add `ShinobiConfig` struct: `APIURL string yaml:"api_url"`, `APIKey string yaml:"api_key"`, `GroupKey string yaml:"group_key"`.
   - Add `MCPConfig` struct: `Enabled bool yaml:"enabled"`, `APIKey string yaml:"api_key"`, `AllowUnauthenticatedLoopback bool yaml:"allow_unauthenticated_loopback"`.
   - Ensure zero hardcoded passwords in Go code.
   - Run `go test -v ./internal/config/...` to verify parsing and defaults.
2. Upgrade Ansible role `app_ksp_bida` on `172.16.5.180` (`/build/armbian-build/ansible/playbook/roles/app_ksp_bida/`):
   - In role vars/defaults: define Shinobi credentials and API defaults (`shinobi_mail`, `shinobi_pass`, `shinobi_super_mail`, `shinobi_super_pass`).
   - Implement automated Shinobi provisioning tasks (`tasks/shinobi_provision.yml` included in `tasks/main.yml`):
     a) Check if Shinobi service is active on target (port 8080 or PM2).
     b) Probe user login `POST http://127.0.0.1:8080/?json=true` (`ngohuynhngockhanh@gmail.com` / `smarthome12345`).
     c) If user does not exist, authenticate as Super Admin `POST http://127.0.0.1:8080/super/?json=true` (`ngohuynhngockhanh@gmail.com` / `KSPHondaCity51F79713@`), invoke register admin API to create the user account and retrieve `ke` (Group Key).
     d) Using user session token (`auth_token`), check existing API keys or create new API key with IP restriction `127.0.0.1` and all permissions (`auth_socket,get_monitors,control_monitors,get_logs,watch_stream,watch_snapshot,watch_videos,delete_videos`).
     e) Update `/opt/ksp-cam/config.yaml` on target with `shinobi` section (`api_url`, `api_key`, `group_key`) and `mcp` section.
3. Test the Ansible role changes by executing dry-run / syntax check or running against `inut_204_63` if safe, and verify `go test ./...` passes.

Produce a handoff report at `/home/ksp/ksp-camera-auto/.agents/worker_m1/handoff.md` and notify parent when complete via send_message.
