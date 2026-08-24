# Comprehensive Infrastructure, Build & Deployment Survey Report

## Executive Summary
This report provides an exhaustive investigation of the testing infrastructure, multi-architecture build system, edge deployment tooling, target nodes (`inut_204_164` and `inut_204_163`), and repository hygiene for `ksp-camera-auto`. All test suites (Go unit tests and Playwright UI tests) pass 100%, cross-compilation produces pure static binaries for all required CPU architectures, deployment pipelines are fully operational via Ansible over reverse SSH tunnels, and both target edge nodes are actively running `kspcam` on port `:2028` with verified public HTTP frp endpoints.

---

## 1. Testing Infrastructure

### 1.1 Go Unit & Integration Tests (`go test ./...`)
- **Toolchain**: Go `go1.26.5 linux/amd64` (located at `/home/ksp/go-sdk/bin/go`).
- **Test Inventory**: 56 test files (`*_test.go`) spanning 16 packages across backend subsystems:
  - `internal/bulk`: Bulk configuration orchestrator & credential test suite (`credtest_test.go`).
  - `internal/camera`: Camera interface capabilities & FPS limit checks (`fps_test.go`, `port_fallback_test.go`).
  - `internal/config`: YAML configuration, AES-256-GCM encryption at rest, inventory deletion (`config_test.go`, `crypto_test.go`, `inventory_delete_test.go`).
  - `internal/dahua`: DVRIP binary framing (32-byte header), 2-step challenge/response MD5 login, multi-frame JSON-RPC assembly, OSD live, network, time sync, storage (`dvrip_test.go`, `dial_test.go`, `encode_test.go`, `multiframe_test.go`, `davdownload_test.go`, etc.).
  - `internal/discovery`: ONVIF (UDP 3702), Dahua DVRIP broadcast (UDP 37810), Hikvision SADP (UDP 37020), and Nmap port discovery.
  - `internal/hik` & `internal/isapi`: Hikvision ISAPI RFC 2617 HTTP Digest authentication, XML mutation engine, media search, playback, remote device proxy (`digest_test.go`, `isapi_test.go`, `search_test.go`, `storage_test.go`, `playback_test.go`).
  - `internal/mcp`: Embedded Model Context Protocol (MCP) server, stdio & SSE JSON-RPC 2.0 transports, 31 registered tool handlers, adversarial fuzzing test suites.
  - `internal/nvrhealth`: Health evaluation algorithm, timeline gap classification, clock drift, storage growth rate.
  - `internal/redbida`: MQTT client (`127.0.0.1:12369`), catalog schema extraction, 20-tab INI visual parser, risk classification, read-back verification, preset generators, adversarial payload tests.
  - `internal/server`: HTTP routing, session store, login rate limiter (`loginLimiter`), singleflight snapshot cache (`snapshotCache`), SSE event streams, NVR watchdog loop.
  - `internal/shinobi`: Shinobi NVR REST API client, CRUD monitor, 2-way sync engine.
  - `internal/tiandy`: Dual-plane Tiandy adapter (RTSP media plane + ISAPI session config plane). Live hardware tests gracefully skip when hardware environment variables are absent.
  - `web`: Embedded static asset loading (`embed_test.go`).
- **Execution & Status**:
  - Command: `go test -cover ./...`
  - Result: **100% PASS** across all test suites.
  - Coverage: Ranges up to 83.2% for `internal/redbida`, 81.9% for `internal/nvrhealth`, 72.7% for `internal/shinobi`, 71.2% for `internal/importer`, 65.7% for `internal/config`, 100% for `web`.

### 1.2 Playwright UI End-to-End Tests (`tests/ui/`)
- **Toolchain**: Node.js `v24.18.1`, `@playwright/test` `^1.55.0`.
- **Harness Architecture**:
  - Test runner: `npx playwright test` configured in `playwright.config.js`.
  - Integrated local server: `webServer` automatically launches Python static server on `http://127.0.0.1:4173` serving `web/static/`.
  - Multi-device matrix: Tested on `desktop` (Desktop Chrome) and `mobile` (iPhone 13 viewport emulation).
  - Mock Fixture engine (`tests/ui/fixtures.js`): Comprehensive API mocking covering all `/api/*` endpoints (JSON REST, singleflight snapshots via 1x1 GIF, SSE ndjson event streams for bulk apply/password change, RedBida catalog, NVR health/scan).
- **Test Specs Inventory**:
  1. `tests/ui/bulk.spec.js`: Bulk configuration wizard, device streaming progress, validation.
  2. `tests/ui/cameras.spec.js`: Camera inventory table/grid views, filtering, badges, delete operations.
  3. `tests/ui/detail.spec.js`: Camera detail workspace (7 tabs: OSD, Color, Video/Audio, Network/Wi-Fi, PTZ, Maintenance).
  4. `tests/ui/mobile.spec.js`: Mobile responsive layouts, collapsible sidebars, touch controls.
  5. `tests/ui/nav.spec.js`: SPA routing, hash changes, header status badges, help drawer.
  6. `tests/ui/nvr.spec.js`: NVR health reporting, timeline diagnostic cards, child camera mapping.
  7. `tests/ui/redbida.spec.js`: RedBida knowledge hub, risk badge classification, read-only protection, NTP status.
  8. `tests/ui/redbida_m3_challenger.spec.js`: 1-Click preset generator, diff card, instant apply, 20-tab INI editor, gradient palette.
  9. `tests/ui/review.spec.js`: Video playback viewer, speed selector, QR HMAC playback token generation.
  10. `tests/ui/scan.spec.js`: Network discovery results, manufacturer normalization, password testing.
- **Execution & Status**:
  - Command: `npx playwright test`
  - Result: **113 passed, 11 skipped, 0 failed** (100% pass rate).

---

## 2. Multi-Architecture Build System

### 2.1 Makefile Structure & Build Flags
- **Compiler Flags**:
  - `CGO_ENABLED=0` exported by default to produce 100% statically linked binaries without glibc or dynamic C runtime dependencies.
  - `LDFLAGS := -s -w -X main.version=$(VERSION)`: Strips debug symbols (`-s -w`) to produce compact binaries (~10 MB).
- **Cross-Compilation Targets**:
  - `make build-amd64`: `GOOS=linux GOARCH=amd64` -> `dist/kspcam-linux-amd64` (10.88 MB, x86-64 ELF static).
  - `make build-arm32`: `GOOS=linux GOARCH=arm GOARM=7` -> `dist/kspcam-linux-armv7` (10.49 MB, ARM 32-bit EABI5 ELF static).
  - `make build-arm64`: `GOOS=linux GOARCH=arm64` -> `dist/kspcam-linux-arm64` (10.16 MB, ARM aarch64 ELF static).
  - `make build-all`: Compiles all 3 architectures in one target.
- **Documentation Verification Target**:
  - `make docs-check`: Runs `go run ./tools/docgen -check`.
  - Verified: **25 help articles** (`docs/help/*.md`) successfully indexed into `web/static/help/help-index.json`.

---

## 3. Deployment Tooling, Infrastructure & Target Edge Nodes

### 3.1 Deployment Architecture & Pipeline
- **Ansible Controller Server**: `root@172.16.5.180` (accessible via tap interface `tap_hpc` `172.32.0.69`).
- **Repository Location on Server**: `/build/armbian-build/ansible`.
- **Ansible Role**: `app_ksp_bida` located at `playbook/roles/app_ksp_bida/`.
- **Deployment Playbook**: `playbook/ksp-bida.yml` invoked via `make ksp-bida <hosts>` (e.g. `make ksp-bida inut_204_164,inut_204_163`).

### 3.2 Role `app_ksp_bida` Workflow
1. **Architecture Matching**:
   - Detects `ansible_architecture`. Selects `kspcam-armhf` for 32-bit ARM or `kspcam` for 64-bit ARM (`aarch64`).
   - Copies binary to `/opt/ksp-cam/kspcam` (`mode: 0755`).
2. **Automated Shinobi Provisioning**:
   - Executes `tasks/shinobi_provision.yml` to ensure local Shinobi NVR instance credentials and API keys are initialized.
3. **Configuration Deployment**:
   - Deploys `/opt/ksp-cam/config.yaml` (`mode: 0640`) configuring web server on `:2028`, default credentials, Shinobi API keys, MCP loopback allowlist, and RedBida MQTT settings (`127.0.0.1:12369`).
4. **Systemd Service Setup**:
   - Installs `/etc/systemd/system/kspcam.service`:
     ```ini
     [Unit]
     Description=ksp-camera-auto (bulk camera config UI on :2028)
     After=network-online.target
     Wants=network-online.target
     [Service]
     WorkingDirectory=/opt/ksp-cam
     Environment=KSPCAM_KEY_FILE=/opt/ksp-cam/.kspcam.key
     ExecStart=/opt/ksp-cam/kspcam --addr 0.0.0.0:2028 --config /opt/ksp-cam/config.yaml
     Restart=always
     RestartSec=5
     [Install]
     WantedBy=multi-user.target
     ```
   - Reloads systemd daemon and restarts `kspcam.service`.
5. **Inventory Auto-Seeding**:
   - Imports Shinobi monitor JSON into camera inventory via `/opt/ksp-cam/kspcam --import-shinobi`.
6. **frp Reverse Proxy Configuration**:
   - Modifies `/root/ota-mqtt/change_ok/frpc_config` via helper `frpc_add.py` to register HTTP proxy subdomain `ksp-cam-<hostname>` pointing to `local_port = 2028`.
   - Restarts `frpc` daemon.

### 3.3 Target Edge Nodes Live Status

| Node Hostname | IP / Tunnel Endpoint | Architecture | Service Status | Listening Port | Public frp URL | HTTP Health Check |
|---|---|---|---|---|---|---|
| `inut_204_164` | `video.io.vn:45529` | `aarch64` (`arm64`) | `kspcam.service` ACTIVE (Running) | `0.0.0.0:2028` | `http://ksp-cam-inut-204-164.video.io.vn` | `HTTP 200 OK` (`/healthz`) |
| `inut_204_163` | `video.io.vn:45528` | `aarch64` (`arm64`) | `kspcam.service` ACTIVE (Running) | `0.0.0.0:2028` | `http://ksp-cam-inut-204-163.video.io.vn` | `HTTP 200 OK` (`/healthz`) |

### 3.4 Deployment Procedure for Release
To build and deploy new binary updates to edge nodes:
```bash
# 1. Build all static binaries
make build-all

# 2. Upload compiled binaries to Ansible build server
scp dist/kspcam-linux-arm64 root@172.16.5.180:/build/armbian-build/ansible/playbook/roles/app_ksp_bida/files/kspcam
scp dist/kspcam-linux-armv7 root@172.16.5.180:/build/armbian-build/ansible/playbook/roles/app_ksp_bida/files/kspcam-armhf

# 3. Trigger deployment to target boxes
ssh root@172.16.5.180 "cd /build/armbian-build/ansible && make ksp-bida inut_204_164,inut_204_163"
```

---

## 4. Git Status & Repository Hygiene

- **Branch**: `main` (synchronized with `origin/main` at commit `50ccb56`).
- **Tracked Codebase**: 100% clean. No uncommitted modifications to Go source code, JS/CSS web assets, documentation, or Makefile.
- **Git Ignore Policy**: `.gitignore` strictly excludes temporary binary artifacts (`dist/`, `bin/`, `kspcam`), secrets (`cameras.yaml`, `config.yaml`), logs, `node_modules/`, Playwright reports (`test-results/`, `playwright-report/`), and vendor SDKs.
- **Agent Metadata Boundary**: All agent planning, briefing, and reports are strictly confined within `.agents/` adhering to project directory conventions.

---

## 5. Matrix Summary

| Component | Target / Standard | Current Status | Verification Command |
|---|---|---|---|
| Go Unit Tests | 100% Pass across all 16 packages | **PASS** (0 failures) | `go test ./...` |
| Playwright UI Tests | 100% Pass across 10 spec files | **PASS** (113 passed, 11 skipped) | `npx playwright test` |
| Static Compilation | Pure Go, `CGO_ENABLED=0`, stripped | **PASS** (`amd64`, `arm64`, `armv7`) | `make build-all && file dist/*` |
| Doc Coverage | 25 help articles mapped to routes | **PASS** (100% coverage) | `make docs-check` |
| Edge Node `inut_204_164` | `kspcam.service` active + frp 200 OK | **ONLINE** (`0.0.0.0:2028`) | `curl http://ksp-cam-inut-204-164.video.io.vn/healthz` |
| Edge Node `inut_204_163` | `kspcam.service` active + frp 200 OK | **ONLINE** (`0.0.0.0:2028`) | `curl http://ksp-cam-inut-204-163.video.io.vn/healthz` |
| Repository Hygiene | Clean tree, no stray artifacts | **CLEAN** (only `.agents/` metadata) | `git status` |
