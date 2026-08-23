## 2026-08-23T16:30:00Z

Investigate how to automate Shinobi provisioning in Ansible role `app_ksp_bida` on `172.16.5.180`:
1. Search the workspace and environment for Ansible playbooks, roles (`app_ksp_bida`), `Makefile` (`make ksp-bida`), deploy scripts, and server access.
2. Investigate the Shinobi Super Admin and User API flows:
   - Check existing user: `POST http://127.0.0.1:8080/?json=true` with mail=`ngohuynhngockhanh@gmail.com`, pass=`smarthome12345`.
   - Super Admin login & user creation: `POST http://127.0.0.1:8080/super/?json=true` with mail=`ngohuynhngockhanh@gmail.com`, pass=`KSPHondaCity51F79713@`. Extract session / call add user endpoint to create user and get Group Key (`ke` / `mid`).
   - Create API Key: endpoint `http://127.0.0.1:8080/super/api/add` or user API key endpoint with IP restriction `127.0.0.1` and permissions=`all`.
   - Write config parameters to `/opt/ksp-cam/config.yaml` (`shinobi.api_url`, `shinobi.api_key`, `shinobi.group_key`).
3. Check `internal/config/` to see how Shinobi config struct should be defined in Go (`shinobi.api_url`, `shinobi.api_key`, `shinobi.group_key`), ensuring zero hardcoding of passwords in Go source.

Produce a comprehensive investigation report at `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_ansible/report.md` and write a handoff summary at `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_ansible/handoff.md`.
Use send_message to notify parent when complete.
