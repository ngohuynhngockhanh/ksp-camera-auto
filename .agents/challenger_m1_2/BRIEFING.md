# BRIEFING — 2026-08-24T20:31:00+07:00

## Mission
Adversarial empirical testing & stress-testing for Milestone 1 (RedBida & Onboarding MCP Tools Suite): Verify `internal/mcp/tools_redbida.go` under high concurrency, mock broker failures (timeout, ack failure, partial write failure, nil service), read-back verification, confirmation enforcement, and 1-Click Onboarding synthesis. Deliver concrete verdict (APPROVE / REQUEST_CHANGES).

## 🔒 My Identity
- Archetype: challenger / empirical-challenger
- Roles: critic, specialist
- Working directory: /home/ksp/ksp-camera-auto/.agents/challenger_m1_2
- Original parent: 6a8fb107-278e-456d-910f-dfb3bd7838d2
- Milestone: M1 (RedBida & Onboarding MCP Tools Suite)
- Instance: 2 of 2

## 🔒 Key Constraints
- Review-only — do NOT modify implementation code.
- Layout Compliance — .agents/ holds only metadata; tests executed directly on target or co-located Go test files.
- Empirical verification — all claims must be backed by executed tests, race detector, boundary tests, and memory/concurrency harnesses.

## Current Parent
- Conversation ID: 6a8fb107-278e-456d-910f-dfb3bd7838d2
- Updated: 2026-08-24T20:31:00+07:00

## Review Scope
- **Files to review**:
  - `internal/mcp/tools_redbida.go`
  - `internal/mcp/tools_redbida_test.go`
  - `internal/mcp/registry.go`
  - `internal/mcp/server.go`
  - `internal/redbida/service.go`
  - `internal/redbida/catalog.go`
  - `internal/redbida/mqtt.go`
- **Interface contracts**: PROJECT.md, ORIGINAL_REQUEST.md
- **Review criteria**: concurrency safety, race detection, mock broker failure recovery & fail-closed behaviors, read-back verification, risk/confirmation enforcement, boundary conditions, UTF-8/Vietnamese diacritics stripping, 20-tab INI format.

## Attack Surface
- **Hypotheses to test**:
  1. Concurrency / race conditions under parallel tool invocations across all 6 `redbida_*` tools with race detector (`-race`).
  2. Mock broker error handling: Read timeout (`context.DeadlineExceeded`), Write timeout, network disconnect.
  3. Broker ACK timeout recovery: successful read-back recovers gracefully vs failed read-back fails closed.
  4. Partial ACK handling & corrupted/stale read-back detection.
  5. Confirmation enforcement: `RiskConfirm` keys rejected when `confirmed: false` or omitted, accepted when `confirmed: true`.
  6. Protected / read-only keys rejected without calling broker.
  7. Nil/disabled service handling for all 6 tools without panic.
  8. 1-Click Onboarding preset synthesis: 15 parameters, diacritic-free hashtags, semicolon-stripped CSS, 20-tab INI, cameraCount boundaries (0, 21, 1, 20).
- **Vulnerabilities found**: [TBD]
- **Untested angles**: [TBD]

## Loaded Skills
- **Source**: `/home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md`
- **Local copy**: Loaded directly from workspace
- **Core methodology**: Camera, Monitor ID, Device ID naming standards, Golden Template from Camera01, RedBida 20-tab INI specification, custom_hashtags formatting, MQTT /private/i_sets protocol.

## Key Decisions Made
- [2026-08-24] Initializing empirical stress harness for `internal/mcp/tools_redbida.go`.

## Artifact Index
- `/home/ksp/ksp-camera-auto/.agents/challenger_m1_2/DISPATCH.md` — Inbound dispatches
- `/home/ksp/ksp-camera-auto/.agents/challenger_m1_2/BRIEFING.md` — Situational awareness
- `/home/ksp/ksp-camera-auto/.agents/challenger_m1_2/progress.md` — Liveness & progress tracking
- `/home/ksp/ksp-camera-auto/.agents/challenger_m1_2/handoff.md` — Final verdict and empirical challenge report
