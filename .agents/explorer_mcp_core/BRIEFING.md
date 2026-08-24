# BRIEFING — 2026-08-24T20:25:00Z

## Mission
Thoroughly investigate internal/mcp architecture, tool registration, JSON-RPC 2.0 dispatching, test harness, and provide blueprints for RedBida tools integration.

## 🔒 My Identity
- Archetype: explorer
- Roles: investigator, synthesizer
- Working directory: /home/ksp/ksp-camera-auto/.agents/explorer_mcp_core
- Original parent: 6a8fb107-278e-456d-910f-dfb3bd7838d2
- Milestone: MCP Core Architecture Exploration

## 🔒 Key Constraints
- Read-only investigation — do NOT implement
- Base findings on verifiable code evidence (file paths, line numbers, exact types, function signatures)
- Write full report to .agents/explorer_mcp_core/handoff.md

## Current Parent
- Conversation ID: 6a8fb107-278e-456d-910f-dfb3bd7838d2
- Updated: 2026-08-24T20:25:00Z

## Investigation State
- **Explored paths**:
  - `internal/mcp/` (`types.go`, `registry.go`, `server.go`, `stdio.go`, `sse.go`, `tools_camera.go`, `tools_config.go`, `tools_discovery.go`, `tools_shinobi.go`, `server_test.go`, `tools_test.go`)
  - `internal/server/` (`server.go`, `api_redbida.go`, `mcp_test.go`, `nvr_health.go`)
  - `internal/redbida/` (`types.go`, `catalog.go`, `service.go`, `mqtt.go`)
  - `cmd/kspcam/main.go`
  - `.agents/skills/camera-naming/SKILL.md`
  - `web/static/redbida.js`
- **Key findings**:
  - Existing MCP Server has 25 tools across 4 categories (camera inventory, camera config, discovery & diagnosis, shinobi NVR).
  - JSON-RPC 2.0 layer handles `initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`.
  - Stdio mode handles line-delimited JSON over stdin/stdout, redirecting logger to stderr.
  - HTTP/SSE mode supports `/mcp` (GET SSE stream and POST stateless) and `/mcp/messages` (POST with session ID).
  - RedBida service in `internal/redbida` communicates via MQTT `/private/i_gets` and `/private/i_sets` with mandatory read-back verification.
  - Ready blueprint for 6 new tools in `internal/mcp/tools_redbida.go` expanding catalog to 31 tools.
- **Unexplored areas**: None. Exploration complete.

## Key Decisions Made
- Use variadic `redbidaSvc ...*redbida.Service` in `mcp.NewServer` or automatic initialization when `cfg.Redbida.Enabled` for 100% backward compatibility with existing tests and CLI invocation.
- Recommend pure Go diacritic removal table in `tools_redbida.go` matching `web/static/redbida.js` `removeVietnameseTones`.

## Artifact Index
- handoff.md — Comprehensive 5-component exploration and architecture report
- progress.md — Liveness and progress heartbeat
- BRIEFING.md — Persistent context index
