# Project: RedBida Modern Glassmorphism UI & Knowledge/Onboarding Hub Upgrade

## Architecture
- **Web Layer (`web/static/`)**: Single-page application embedded via `go:embed static` into the Go static binary. Uses Vanilla ES6+ JavaScript, CSS3 with modern Dark/Light Glassmorphism design tokens (`--glass-*`, `backdrop-filter: blur(16px)`), responsive CSS Grid, and dynamic DOM rendering.
- **API & Server Layer (`internal/server/`)**: Go HTTP server exposing REST endpoints (`/api/redbida/catalog`, `/api/redbida/refresh`, `/api/redbida/apply`, `/api/redbida/time-status`) protected with role-based auth (Viewer for read/refresh, Admin for apply/mutations).
- **Core Engine & Catalog (`internal/redbida/`)**: Key metadata catalog, type validation (`TypeString`, `TypeNumber`, `TypeBoolean`, `TypeImage`), risk gating (`RiskEditable`, `RiskConfirm`, `RiskProtected`), and service orchestrator with mandatory 3-phase read-back verification.
- **Protocol & Broker Layer (`internal/redbida/mqtt.go`)**: MQTT client communicating with local `ota-mqtt` broker (`127.0.0.1:12369`) via `/private/i_sets` (writes: `{"info": {"<key>": "<val>"}}`), `/private/i_gets` (reads: `{"info": ["<key1>", ...]}`), and corresponding ack topics.

## Feature Inventory
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F1 | Catalog Metadata & Classification Fixes | Update `catalog.go` (`toolbar_show_count` editable, `ui_tabs_links`/`custom_hashtags` string type, `shinobi_group_key` fallback, logical group mappings for 10+ keys) | M1 | ORIGINAL_REQUEST §R2 |
| F2 | Backend Go Unit Tests | Comprehensive test suite for updated catalog metadata, validation rules, and server routes | M1 | ORIGINAL_REQUEST §R3 |
| F3 | Glassmorphism Design Tokens & CSS Styles | Modern Dark/Light glassmorphism system, responsive grid layouts, status cards, glowing accents in `style.css` | M2 | ORIGINAL_REQUEST §R1 |
| F4 | DOM Layout for 4-Pillar Hub & Preset Panel | Enhanced HTML structure in `#view-redbida` (`index.html`) with Hero bar, 4 Knowledge Pillar cards, Preset Generator section, and preserved test selectors | M2 | ORIGINAL_REQUEST §R1 |
| F5 | 4-Pillar Knowledge Hub Component | Interactive cards in `redbida.js` displaying domain knowledge, key specs, and instant filter-by-pillar buttons | M3 | ORIGINAL_REQUEST §R1 |
| F6 | 1-Click Onboarding Preset Generator | Auto-calculates 13+ standard parameters, 20-tab INI `ui_tabs_links`, clean hashtags, visual diff preview, and 1-click batch submit | M3 | ORIGINAL_REQUEST §R1 |
| F7 | Visual Live Previews & Swatches | Realtime CSS gradient preview for `ui_bg` with 6 preset swatches, checkerboard logo preview for `logo_header`/`logo_livestream`, and 20-tab player simulator | M3 | ORIGINAL_REQUEST §R1, §R2 |
| F8 | Playwright UI & E2E Automated Testing | 100% passing Playwright test suite (`tests/ui/redbida.spec.js`), zero JavaScript console errors, all test selectors preserved | M4 | ORIGINAL_REQUEST §R3 |
| F9 | Static Binary Build & Deployment Check | `go test ./...` 100% pass, `make build-all` static compilation success, API verification on target port `:2028` | M4 | ORIGINAL_REQUEST §R3 |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Backend Catalog & Metadata Refinements | `internal/redbida/catalog.go`, `internal/redbida/redbida_test.go`, `internal/server/api_redbida_test.go` | None | DONE |
| M2 | Frontend Glassmorphism Design & DOM Structure | `web/static/style.css`, `web/static/index.html` | M1 | DONE |
| M3 | Knowledge Hub, Preset Generator & Live Previews | `web/static/redbida.js` | M1, M2 | DONE |
| M4 | Comprehensive Testing, Static Build & Verification | Playwright tests, Go test suite, static build check | M1, M2, M3 | DONE |

## Interface Contracts
### Web UI ↔ REST API (`internal/server`)
- `GET /api/redbida/catalog` -> `{ "keys": [KeyMeta...], "sourceAvailable": bool, "sourceError": string }`
- `POST /api/redbida/refresh` with `{ "keys": ["k1", "k2"] }` -> `{ "values": { "k1": { "key": "k1", "value": "val", "meta": KeyMeta, "exists": bool } }, "refreshedAt": "RFC3339" }`
- `POST /api/redbida/apply` with `{ "changes": { "k1": "v1" }, "confirmed": bool }` -> `{ "results": { "k1": ChangeResult }, "appliedAt": "RFC3339" }`
- `GET /api/redbida/time-status` -> `{ "hostTime": int64, "hostTimeRFC3339": string, "ntpSynchronized": bool, "driftThresholdSeconds": int, "policy": string, "nodeRedReadOnly": bool }`

### Go Backend ↔ MQTT Broker (`internal/redbida/mqtt.go`)
- Write Topic `/private/i_sets`: `{"info": {"<key>": "<val>", ...}}`
- Write Ack Topic `/private/i_sets/ack`: `{"info": {"<key>": {"oldValue": <old>, "newValue": <new>}, ...}}`
- Read Topic `/private/i_gets`: `{"info": ["<key1>", "<key2>", ...]}`
- Read Ack Topic `/private/i_gets/ack`: `{"info": {"<key1>": <val1>, "<key2>": <val2>, ...}}`

## Code Layout
- `internal/redbida/catalog.go`: Metadata definitions, key classification, risk/value types, grouping logic.
- `internal/redbida/service.go`: Value validation, read-back verification engine, change result synthesis.
- `internal/redbida/mqtt.go`: MQTT client for `ota-mqtt` on `127.0.0.1:12369`.
- `internal/server/api_redbida.go`: HTTP handler endpoints for RedBida.
- `web/static/index.html`: Main SPA HTML including `#view-redbida`.
- `web/static/style.css`: Application styles, glassmorphism tokens, responsive layouts.
- `web/static/redbida.js`: Frontend state machine, Knowledge Hub logic, Preset Generator, Live Previews, DOM rendering.
- `tests/ui/redbida.spec.js`: Playwright E2E UI test suite.
