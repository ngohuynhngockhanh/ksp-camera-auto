# Dispatch for Explorer Survey 3

## 2026-08-24T09:08:26Z

User request:
You are teamwork_preview_explorer (Survey Specialist 3: Camera Network & Shinobi Golden Template).
Working directory: /home/ksp/ksp-camera-auto/.agents/explorer_survey_3
Authoritative request: /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md
Skill documentation: /home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md

Objective:
Investigate the field camera network and Shinobi monitors on the target system:
1. Camera reachability from target host or local network: check IP range `192.168.1.190` to `192.168.1.197` (ports 37777, 80, 554).
2. For each camera: verify model/vendor (Dahua/KBVision, Hikvision, etc.), serial number, credentials (e.g. Admin123456), audio capability (AAC vs none).
3. Review Shinobi NVR existing monitor configuration on target host (:8080) vs the 8 camera specifications in `/home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md`.
4. Check Golden Template requirements: `vcodec: copy`, `stream_vcodec: copy`, `record_vcodec: copy`, `cust_record: "-tag:v hvc1"`, empty `cust_input` & `cust_stream`, audio AAC rule.

Scope boundaries:
- Exploration and probing only. DO NOT apply permanent disruptive changes.
- Produce a structured report in `/home/ksp/ksp-camera-auto/.agents/explorer_survey_3/survey_cameras.md` and write your `handoff.md`.

Completion criteria:
- Complete inventory of the 8 cameras, their live reachability, audio codec capabilities, current vs expected Shinobi monitor configs according to Golden Template. Send message to orchestrator when done.
