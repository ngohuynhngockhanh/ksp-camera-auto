# BRIEFING — 2026-08-24T20:28:40+07:00

## Mission
Milestone 1: Implement RedBida & Onboarding MCP Tools Suite in internal/mcp/tools_redbida.go.

## 🔒 My Identity
- Archetype: worker
- Roles: implementer, qa, specialist
- Working directory: /home/ksp/ksp-camera-auto/.agents/worker_m1
- Original parent: 6a8fb107-278e-456d-910f-dfb3bd7838d2
- Milestone: M1 (RedBida & Onboarding MCP Tools Suite)

## 🔒 Key Constraints
- Target file: `internal/mcp/tools_redbida.go`
- Genuine implementation only, no hardcoded cheating.
- 100% test pass with full coverage for new changes.
- Pure Go with CGO_ENABLED=0 compatibility.

## Current Parent
- Conversation ID: 6a8fb107-278e-456d-910f-dfb3bd7838d2
- Updated: 2026-08-24T20:28:40+07:00

## Task Summary
- **What to build**: `internal/mcp/tools_redbida.go` implementing 6 MCP tools:
  1. `redbida_list_catalog`
  2. `redbida_get_keys`
  3. `redbida_set_keys`
  4. `redbida_apply_onboarding_preset`
  5. `redbida_trigger_go2rtc`
  6. `redbida_get_time_status`
  and helper `registerRedbidaTools(r *Registry, cfg *config.Config, redbidaSvc *redbida.Service)`.
- **Success criteria**: All tools functional, pure Go removeVietnameseTones, 20-tab INI, trailing semicolon stripping on CSS gradients, full read-back verification, unit tests passing.
- **Interface contracts**: `PROJECT.md` § Interface Contracts.

## Loaded Skills
- **Source**: `/home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md`
- **Core methodology**: Camera/monitor naming standard, Golden Template inheritance from Camera01, 20-section INI tabs `[C01]`-`[C20]`, CSS gradient background without trailing semicolon, and hashtag normalization.

## Key Decisions Made
- Implemented `removeVietnameseTones` in pure Go supporting both NFC (precomposed) and NFD (decomposed) combining diacritical marks (`U+0300`-`U+036F`).
- Implemented `sanitizeCleanTitle` stripping all punctuation/whitespace for `#<CleanTitle> #BILLIARDSlive #INUTlive #highlightsports`.
- Implemented `sanitizeCSSGradient` stripping trailing semicolon `;` to ensure valid CSS DOM injection.
- Implemented `generate20TabINITabs` producing exactly 20 sections `[C01]`-`[C20]` with `vid_play_label = <ui_title>`.
- Included `button_generate_go2rtc_stream` and `button_restart_shinobi` in `booleanKeySet` in `catalog.go`.
- Added complete unit test suite in `internal/mcp/tools_redbida_test.go` covering dry-run, live execution, validations, catalog filtering, secret masking, and disabled service fallback.

## Artifact Index
- `internal/mcp/tools_redbida.go` — RedBida & Onboarding MCP tools implementation
- `internal/mcp/tools_redbida_test.go` — Unit tests for RedBida MCP tools
- `internal/redbida/catalog.go` — Catalog booleanKeySet refinement

## Change Tracker
- **Files modified**:
  - `internal/mcp/tools_redbida.go`: Created new MCP tools file.
  - `internal/mcp/tools_redbida_test.go`: Created comprehensive test suite.
  - `internal/redbida/catalog.go`: Added `button_generate_go2rtc_stream` and `button_restart_shinobi` to `booleanKeySet`.
- **Build status**: PASS (`go test ./...` 100% pass)
- **Pending issues**: None

## Quality Status
- **Build/test result**: PASS (all unit tests pass across all packages)
- **Lint status**: Clean (`go vet` passes with 0 issues)
- **Tests added/modified**: 13 test functions in `internal/mcp/tools_redbida_test.go`
