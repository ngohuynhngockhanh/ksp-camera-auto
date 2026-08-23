# Orchestration Plan: Shinobi Integration & Embedded MCP Server

## Objectives
Integrate comprehensive Shinobi NVR management, Ansible automated provisioning (`playbook/roles/app_ksp_bida`), and embedded MCP Server in `kspcam` with full testing, documentation, and verification.

## Phases & Milestones

### Phase 0: Survey & Discovery (M0)
- **Explorer 1 (Ansible & Config)**: Investigate `playbook/roles/app_ksp_bida` on `172.16.5.180`, Shinobi API endpoints (`/?json=true`, `/super/?json=true`), group key creation, API key generation with IP 127.0.0.1 and all permissions, `/opt/ksp-cam/config.yaml` schema and `internal/config/config.go`.
- **Explorer 2 (Shinobi Go Client & REST API)**: Investigate Shinobi REST API (monitors CRUD, states, RTSP/codecs), existing `internal/importer` Shinobi parsing, bi-directional sync design with `cameras.yaml`/Inventory, and REST endpoints in `internal/server/`.
- **Explorer 3 (Embedded MCP Server)**: Investigate MCP JSON-RPC 2.0 specification, Stdio transport (`kspcam --mcp`), HTTP/SSE endpoint `/mcp` on port 2028 with API key security, and complete tool schemas for all 4 tool groups (Inventory, Bulk/Config, Discovery/Diag, Shinobi).

### Phase 1: Milestone M1 — Ansible Automated Shinobi Provisioning (R1)
- Upgrade `app_ksp_bida` role on `172.16.5.180` to automate user check, super admin login, user & group key creation, API key creation (IP 127.0.0.1, all perms), and writing `shinobi` section into `/opt/ksp-cam/config.yaml`.
- Ensure zero hardcoded Super Admin password in Go code.

### Phase 2: Milestone M2 — Shinobi Go Client & Full Management Engine (R2)
- Build pure Go `internal/shinobi` client (Monitors CRUD, stream control, bi-directional sync).
- Add REST endpoints in `internal/server` (`/api/shinobi/status`, `/api/shinobi/monitors`, `/api/shinobi/sync`, `/api/shinobi/videos`) and web UI integration.

### Phase 3: Milestone M3 — Embedded MCP Server (R3)
- Implement `internal/mcp` supporting Stdio mode (`kspcam --mcp`) and HTTP/SSE mode (`/mcp` on `:2028` with API key auth).
- Implement all required MCP tools (Camera Inventory, Config & Bulk, Discovery & Diag, Shinobi Management).

### Phase 4: Milestone M4 — Tests, Docs, Multi-Arch Build & Remote Validation (R4)
- Comprehensive unit tests in `internal/shinobi` and `internal/mcp`.
- Update `GEMINI.md`, `AGENTS.md`, and run `make docs`.
- Run `make build-all`, `go test ./...`, `make docs-check`.
- Deploy to `inut_204_63` and validate live Shinobi API + MCP Server operations.

### Phase 5: Review, Gate & Final Notification
- Gate checks and completion reporting to Sentinel.
