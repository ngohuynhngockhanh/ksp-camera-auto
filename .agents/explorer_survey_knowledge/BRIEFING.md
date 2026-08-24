# BRIEFING — 2026-08-24T18:56:30+07:00

## Mission
Thoroughly explore the domain knowledge, rules, formulas, and architecture for the RedBida Onboarding Hub (4 Pillars, 1-Click Onboarding Generator, INI Tab specs, Shinobi/Go2RTC parameters, etc.).

## 🔒 My Identity
- Archetype: Teamwork explorer
- Roles: Explorer 3 (Knowledge Hub & Onboarding Flow Spec)
- Working directory: /home/ksp/ksp-camera-auto/.agents/explorer_survey_knowledge
- Original parent: 2459fd81-eea0-41c3-8a5b-e354b9c9f098
- Milestone: Onboarding Knowledge Hub & 1-Click Generator Specification

## 🔒 Key Constraints
- Read-only investigation — do NOT implement production code
- Adhere to Teamwork protocol and 5-component handoff report
- Deliver comprehensive domain formulas and key-value mapping specs for RedBida Onboarding

## Current Parent
- Conversation ID: 2459fd81-eea0-41c3-8a5b-e354b9c9f098
- Updated: 2026-08-24T18:56:30+07:00

## Investigation State
- **Explored paths**:
  - `/home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md`
  - `/home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md`
  - `/home/ksp/ksp-camera-auto/internal/redbida/catalog.go`, `types.go`, `service.go`, `mqtt.go`
  - `/home/ksp/ksp-camera-auto/internal/shinobi/sync.go`, `types.go`, `client.go`
  - `/home/ksp/ksp-camera-auto/web/static/redbida.js`, `index.html`, `style.css`
  - `/home/ksp/ksp-camera-auto/tests/ui/redbida.spec.js`
  - `/home/ksp/ksp-camera-auto/docs/help/redbida.md`, `docs/testing/redbida-ota-sync.tdd.md`
- **Key findings**:
  - Fully mapped all 4 Onboarding Pillars with exact key names, types, risks, and standard values.
  - Specified exact mathematical & text algorithms for hashtag generation (Vietnamese tone strip + alphanumeric sanitization) and 20-tab INI formatting (`[C01]`..`[C20]`).
  - Identified critical catalog issues: `toolbar_show_count` trapped in `runtimeKeyRe`, `ui_tabs_links` and `custom_hashtags` miscategorized as `TypeJSON` instead of `TypeString`, and missing preset gradients.
  - Formulated the exact 1-Click Onboarding Generator schema & MQTT payload for `/private/i_sets`.
- **Unexplored areas**: None; all 4 pillars and domain formulas thoroughly verified.

## Key Decisions Made
- Documented complete formulas, presets, and architecture in `handoff.md`.

## Artifact Index
- `/home/ksp/ksp-camera-auto/.agents/explorer_survey_knowledge/handoff.md` — Complete Knowledge Survey & Onboarding Flow Spec report
