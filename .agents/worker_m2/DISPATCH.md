# Task Assignment: Milestone 2 — MCP Server Integration & Documentation

## 2026-08-24T13:34:00Z

Working Directory: `/home/ksp/ksp-camera-auto/.agents/worker_m2`

Read:
- `/home/ksp/ksp-camera-auto/ORIGINAL_REQUEST.md`
- `/home/ksp/ksp-camera-auto/PROJECT.md`
- `/home/ksp/ksp-camera-auto/.agents/explorer_mcp_core/handoff.md`
- `/home/ksp/ksp-camera-auto/.agents/explorer_deploy_infra/handoff.md`
- `/home/ksp/ksp-camera-auto/.agents/worker_m1/handoff.md`
- `/home/ksp/ksp-camera-auto/.agents/worker_m2/DISPATCH.md`

Tasks for Milestone 2:
1. Integrate RedBida tools into `internal/mcp/server.go`:
   - Update `NewServer(cfg *config.Config, inv *config.Inventory, shinobiClient *shinobi.Client, redbidaService ...*redbida.Service) *Server`.
   - Instantiate/wire `redbida.Service` and call `registerRedbidaTools(registry, cfg, rSvc)`.
   - Update `internal/server/server.go` and `cmd/kspcam/main.go` so they pass their initialized `redbida.Service` to `mcp.NewServer`.
   - Update `internal/mcp/server_test.go` to test that `tools/list` returns all 31 tools and that JSON-RPC calls dispatch properly.
2. Update documentation:
   - `docs/help/mcp-server.md`: Add RedBida & Onboarding category (6 tools) and update tool count (31 tools).
   - `docs/help/redbida.md`: Add section detailing the 6 MCP tools and cover upload-logo/logo.png routes.
   - `docs/CODEBASE-KNOWLEDGE.md`: Update Section 7 MCP Surface and tools list.
   - `GEMINI.md` and `AGENTS.md`: Update package description (31+ tools), add new tools to the MCP tools table with arguments/types, and update diagrams.
   - Run `/home/ksp/go-sdk/bin/go run ./tools/docgen -check` to verify doc coverage.
3. Verify test suite:
   - Run `/home/ksp/go-sdk/bin/go test -v ./internal/mcp/...`
   - Run `/home/ksp/go-sdk/bin/go test ./...`
   - Run `/home/ksp/go-sdk/bin/go build ./cmd/kspcam`

Write completion report to `/home/ksp/ksp-camera-auto/.agents/worker_m2/handoff.md`.
