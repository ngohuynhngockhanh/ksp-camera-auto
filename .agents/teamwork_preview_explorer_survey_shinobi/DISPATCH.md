## 2026-08-23T16:30:00Z

You are teamwork_preview_explorer surveying Shinobi Go Client & Full Management Engine (Requirement R2).
Your working directory is: /home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_shinobi/
Read /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md before doing anything.

Mission:
Investigate the design and implementation of `internal/shinobi` and its server integration:
1. Examine existing codebase:
   - `internal/importer/` (how Shinobi JSON format is currently parsed, fields like mid, name, type, host, port, path, protocol, etc.)
   - `internal/config/` (Inventory, Camera structure, cameras.yaml schema)
   - `internal/server/` (existing REST API route patterns, session auth, JSON helpers)
   - `web/static/` (UI layout, how views and tabs are structured)
2. Detail the Shinobi REST API client requirements:
   - Client struct initialization with API URL, API Key, Group Key.
   - Monitors CRUD:
     - `ListMonitors(ctx)`: `GET /:apiKey/monitor/:groupKey`
     - `AddMonitor(ctx, mon MonitorConfig)`: `POST /:apiKey/configureMonitor/:groupKey/:monitorId` (or Shinobi standard save endpoint)
     - `EditMonitor(ctx, mon MonitorConfig)`
     - `DeleteMonitor(ctx, monitorId)`: `GET /:apiKey/monitor/:groupKey/:monitorId/delete`
     - `ChangeMonitorState(ctx, monitorId, state)`: `GET /:apiKey/monitor/:groupKey/:monitorId/:state` (start/stop/restart)
     - `GetVideos(ctx, monitorId, limit)`: `GET /:apiKey/videos/:groupKey/:monitorId`
   - Bi-directional sync engine:
     - Sync from `cameras.yaml` (Inventory) to Shinobi (auto-create/update monitors with RTSP URL, credentials, sub-streams).
     - Sync from Shinobi to `cameras.yaml` (auto-import monitors not in inventory).
3. Detail the HTTP Server REST endpoints to add in `internal/server/`:
   - `GET /api/shinobi/status`
   - `GET /api/shinobi/monitors`
   - `POST /api/shinobi/monitors` (add/edit/delete/state)
   - `POST /api/shinobi/sync`
   - `GET /api/shinobi/videos`
4. Detail Web UI components needed for Shinobi management.

Produce a comprehensive investigation report at `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_shinobi/report.md` and write a handoff summary at `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_shinobi/handoff.md`.
Use send_message to notify parent when complete.
