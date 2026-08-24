# Handoff Report: Deployment, Build Infrastructure & MCP RedBida Investigation

## 1. Observation

### 1.1 Build Infrastructure & Multi-Arch Toolchain
- **`Makefile` Analysis** (`/home/ksp/ksp-camera-auto/Makefile`):
  - Line 8: `export CGO_ENABLED=0` enforces static linking for all standard builds.
  - Line 5: `LDFLAGS := -s -w -X main.version=$(VERSION)` injects version and strips binary symbols.
  - Lines 34-43 define multi-arch cross-compilation targets:
    - `build-amd64`: `GOOS=linux GOARCH=amd64 $(GO) build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-amd64 $(PKG)`
    - `build-arm32`: `GOOS=linux GOARCH=arm GOARM=7 $(GO) build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-armv7 $(PKG)`
    - `build-arm64`: `GOOS=linux GOARCH=arm64 $(GO) build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-arm64 $(PKG)`
    - `build-all`: Chains `build-amd64 build-arm32 build-arm64`.
  - Go Toolchain Location: Go is installed at `/home/ksp/go-sdk/bin/go` (version `go1.26.5 linux/amd64`). The default shell PATH must include `/home/ksp/go-sdk/bin`.
  - Verified Compilation: Running `PATH=/home/ksp/go-sdk/bin:$PATH make build-all` successfully produced all three static binaries in `dist/`:
    - `dist/kspcam-linux-amd64`: ELF 64-bit LSB executable, x86-64, statically linked, stripped (11 MB).
    - `dist/kspcam-linux-arm64`: ELF 64-bit LSB executable, ARM aarch64, statically linked, stripped (9.7 MB).
    - `dist/kspcam-linux-armv7`: ELF 32-bit LSB executable, ARM EABI5, statically linked, stripped (9.9 MB).

### 1.2 Remote Deployment Nodes Exploration
- **Network Topology & Jump Host**:
  - Direct SSH to `77.88.204.164` or `77.88.204.163` prompts for password authentication.
  - Jump Host / Bastion: `root@172.16.5.180` (Ansible server) has authorized SSH key access to both target nodes.
  - Access Path: `ssh root@172.16.5.180 "ssh root@77.88.204.164 '<command>'"` and `ssh root@172.16.5.180 "ssh root@77.88.204.163 '<command>'"` execute with zero friction.
- **Node 1: `inut_204_164` (`77.88.204.164`)**:
  - OS / Arch: Linux 6.1.155-ophub, `aarch64` (ARM64).
  - Systemd Service: `/etc/systemd/system/kspcam.service` (Active: `active (running)`).
    - `WorkingDirectory=/opt/ksp-cam`
    - `Environment=KSPCAM_KEY_FILE=/opt/ksp-cam/.kspcam.key`
    - `ExecStart=/opt/ksp-cam/kspcam --addr 0.0.0.0:2028 --config /opt/ksp-cam/config.yaml`
  - Active Listening Ports:
    - `:2028` (TCP): `kspcam` Web UI & MCP server
    - `:12369` (TCP): `ota-mqtt` broker (`/root/ota-mqtt/`)
    - `:8080` (TCP): `Shinobi NVR`
    - `:1984` (TCP): `go2rtc`
    - `:2023` (TCP): `Node-RED`
  - Live MCP RPC Test:
    - `POST /mcp` with `{"jsonrpc":"2.0","id":1,"method":"initialize",...}` returned `{"protocolVersion":"2024-11-05","serverInfo":{"name":"kspcam","version":"1.0.0"}}`.
    - `POST /mcp` with `{"method":"tools/list"}` returned 25 existing tools (16 `kspcam_*`, 9 `shinobi_*`, 0 `redbida_*`).
    - `POST /mcp` with `{"method":"tools/call","params":{"name":"redbida_list_catalog"}}` returned `{"isError":true,"content":[{"text":"unknown tool \"redbida_list_catalog\""}]}`.
- **Node 2: `inut_204_163` (`77.88.204.163`)**:
  - OS / Arch: Linux 6.1.155-ophub, `aarch64` (ARM64).
  - Systemd Service: `/etc/systemd/system/kspcam.service` (Active: `active (running)`).
  - Active Ports: Identical stack (`:2028` kspcam, `:12369` ota-mqtt, `:8080` Shinobi, `:1984` go2rtc, `:2023` Node-RED).
  - Live MCP RPC Test: Responded identically to node 164.

### 1.3 Documentation & Knowledge Base Gap Analysis
- **`docs/help/mcp-server.md`**:
  - Mentions "23+ công cụ chuẩn hóa" and lists 4 categories: Camera Inventory, Config & Bulk, Discovery & Diagnosis, Shinobi Management.
  - Missing: 5th category "RedBida & Onboarding" with the 6 new tools (`redbida_list_catalog`, `redbida_get_keys`, `redbida_set_keys`, `redbida_apply_onboarding_preset`, `redbida_trigger_go2rtc`, `redbida_get_time_status`).
- **`docs/help/redbida.md`**:
  - Lacks cross-link to the new `redbida_*` MCP tools.
- **`docs/CODEBASE-KNOWLEDGE.md`**:
  - Section 7 "MCP Surface" lists 24 tools. Needs update to list 31 tools including RedBida suite.
- **`GEMINI.md` / `AGENTS.md`**:
  - Section 2.1 Package Layout mentions `internal/mcp` (24+ tools -> 30+ tools).
  - Section 3.8 C Table "Bảng Danh mục 24+ Công cụ MCP Chuẩn hóa": Needs new rows for the 6 `redbida_*` tools with parameter specs.
  - Architecture diagram: Add RedBida MQTT & Onboarding MCP invocation flow.
- **`tools/docgen -check`**:
  - Currently flags 3 routes needing documentation coverage: `/logo.png`, `/api/upload-logo`, `/api/redbida/upload-logo`. These can be added to `docs/help/coverage-ignore.txt` or mapped in `docs/help/redbida.md`.

---

## 2. Logic Chain

1. **Multi-Arch Binary Selection**:
   - Both target nodes (`inut_204_164` and `inut_204_163`) report `aarch64` (`ARM64`).
   - Therefore, the deployment artifact for these nodes is exclusively `dist/kspcam-linux-arm64`.
   - `dist/kspcam-linux-armv7` and `dist/kspcam-linux-amd64` remain fully supported and validated via `make build-all`.

2. **Deployment Mechanics**:
   - Dev machine connects to jump host `root@172.16.5.180` via SSH/SCP.
   - Jump host connects directly to target nodes (`77.88.204.164` and `77.88.204.163`).
   - Staging through `/tmp/` on `172.16.5.180` allows instantaneous distribution:
     ```bash
     scp dist/kspcam-linux-arm64 root@172.16.5.180:/tmp/kspcam-linux-arm64
     ssh root@172.16.5.180 "scp /tmp/kspcam-linux-arm64 root@77.88.204.164:/opt/ksp-cam/kspcam && ssh root@77.88.204.164 'systemctl restart kspcam.service'"
     ssh root@172.16.5.180 "scp /tmp/kspcam-linux-arm64 root@77.88.204.163:/opt/ksp-cam/kspcam && ssh root@77.88.204.163 'systemctl restart kspcam.service'"
     ```

3. **MCP RedBida Architecture & Service Integration**:
   - `internal/redbida` already provides `Catalog`, `Service`, `MQTTBroker`, `Refresh`, and `Apply`.
   - `internal/mcp` requires a new file `tools_redbida.go` defining the 6 MCP tools.
   - `internal/mcp/server.go` needs `redbida *redbida.Service` wired into `NewServer` (or registered via `registerRedbidaTools`).
   - `cmd/kspcam/main.go` and `internal/server/server.go` pass the instantiated `redbida.Service` into `mcp.NewServer`.

4. **1-Click Onboarding Preset Calculation (15 Parameters)**:
   - Matches the domain logic established in `web/static/redbida.js`:
     1. `ui_title`: Quán title (e.g. "CX King Luxury")
     2. `company_name`: Same as title
     3. `ui_bg`: Sanitized CSS gradient string (trailing semicolons stripped)
     4. `custom_hashtags`: Normalized non-diacritic hashtag string
     5. `ui_tabs_links`: 20-section INI configuration `[C01]`..`[C20]` with `vid_play_label = <ui_title>`
     6. `camera_count`: Integer (1-20)
     7. `toolbar_show_count`: Integer matching camera_count
     8. `hls_using_go2rtc`: Boolean `true` (or string `"true"`)
     9. `button_generate_go2rtc_stream`: Boolean `true`
     10. `logo_header`: Default `"https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png"`
     11. `logo_header_text`: Default `"Billiard Live - Tải clip bàn bida và livestream"`
     12. `shinobi_camera_id`: Primary camera identifier (`"C01"`)
     13. `shinobi_group_key`: Shinobi group key string
     14. `video_config`: Default `"range=72"`
     15. `ui_scoreboard`: Boolean `true`
     16. `ggcode`: Google Analytics code (e.g. `"G-SFSDZPR95Z"`)

---

## 3. Caveats

1. **PATH Requirement for Go CLI**:
   - Go is located in `/home/ksp/go-sdk/bin/go`. Scripts and terminal commands must prefix `PATH=/home/ksp/go-sdk/bin:$PATH` to avoid `go: command not found`.
2. **Jump Host Authentication**:
   - Direct SSH from the local environment to `77.88.204.164` or `77.88.204.163` requires password auth; all automated deployments must route through `root@172.16.5.180`.
3. **MQTT Broker Dependency for Live Tests**:
   - Unit tests in `internal/mcp/` must use mock brokers (already supported in `internal/redbida/redbida_test.go`) so that `go test ./...` runs deterministically offline without requiring a live MQTT broker on port 12369.
   - Real MQTT verification is performed during the remote deployment phase on `inut_204_164` and `inut_204_163`.

---

## 4. Conclusion

- The build system is healthy, pure Go (`CGO_ENABLED=0`), and cross-compiles cleanly to `amd64`, `arm64`, and `armv7`.
- Target deployment nodes `inut_204_164` and `inut_204_163` are accessible, healthy, running `aarch64` Linux with systemd `kspcam.service`, and have live `ota-mqtt` (:12369), `go2rtc` (:1984), `Node-RED` (:2023), and `Shinobi` (:8080) running.
- Documentation needs targeted updates in `docs/help/mcp-server.md`, `docs/help/redbida.md`, `docs/CODEBASE-KNOWLEDGE.md`, and `GEMINI.md` / `AGENTS.md`.
- All prerequisite infrastructure, network routes, build pipelines, and target runtime nodes are verified and ready for Milestone R1/R2/R3 execution.

---

## 5. Verification Method

### 5.1 Local Build & Unit Test Verification
```bash
# 1. Run all unit tests
PATH=/home/ksp/go-sdk/bin:$PATH go test -v -count=1 ./internal/mcp/...
PATH=/home/ksp/go-sdk/bin:$PATH go test ./...

# 2. Build multi-arch binaries
PATH=/home/ksp/go-sdk/bin:$PATH make build-all

# 3. Verify generated binaries
file dist/kspcam-linux-amd64 dist/kspcam-linux-arm64 dist/kspcam-linux-armv7
```

### 5.2 Documentation Drift Verification
```bash
# Verify all web routes and help articles align
PATH=/home/ksp/go-sdk/bin:$PATH go run ./tools/docgen -check
```

### 5.3 Remote Deployment & Live MCP Invocation Verification
```bash
# 1. Deploy arm64 binary to node 164 & 163 via jump host
scp dist/kspcam-linux-arm64 root@172.16.5.180:/tmp/kspcam-linux-arm64

ssh root@172.16.5.180 "scp /tmp/kspcam-linux-arm64 root@77.88.204.164:/opt/ksp-cam/kspcam && \
  ssh root@77.88.204.164 'chmod +x /opt/ksp-cam/kspcam && systemctl restart kspcam.service && systemctl is-active kspcam'"

ssh root@172.16.5.180 "scp /tmp/kspcam-linux-arm64 root@77.88.204.163:/opt/ksp-cam/kspcam && \
  ssh root@77.88.204.163 'chmod +x /opt/ksp-cam/kspcam && systemctl restart kspcam.service && systemctl is-active kspcam'"

# 2. Test JSON-RPC tools/list on node 164 (must return 31 tools including redbida_*)
ssh root@172.16.5.180 "ssh root@77.88.204.164 'curl -s -X POST http://127.0.0.1:2028/mcp \
  -H \"Content-Type: application/json\" \
  -d \"{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"id\\\":1,\\\"method\\\":\\\"tools/list\\\"}\"'"

# 3. Test calling redbida_list_catalog on node 164
ssh root@172.16.5.180 "ssh root@77.88.204.164 'curl -s -X POST http://127.0.0.1:2028/mcp \
  -H \"Content-Type: application/json\" \
  -d \"{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"id\\\":2,\\\"method\\\":\\\"tools/call\\\",\\\"params\\\":{\\\"name\\\":\\\"redbida_list_catalog\\\",\\\"arguments\\\":{}}}\"'"

# 4. Test calling redbida_get_keys on node 164
ssh root@172.16.5.180 "ssh root@77.88.204.164 'curl -s -X POST http://127.0.0.1:2028/mcp \
  -H \"Content-Type: application/json\" \
  -d \"{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"id\\\":3,\\\"method\\\":\\\"tools/call\\\",\\\"params\\\":{\\\"name\\\":\\\"redbida_get_keys\\\",\\\"arguments\\\":{\\\"keys\\\":[\\\"logo_header_text\\\",\\\"ui_title\\\",\\\"camera_count\\\"]}}}\"'"

# 5. Test calling redbida_get_time_status on node 164
ssh root@172.16.5.180 "ssh root@77.88.204.164 'curl -s -X POST http://127.0.0.1:2028/mcp \
  -H \"Content-Type: application/json\" \
  -d \"{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"id\\\":4,\\\"method\\\":\\\"tools/call\\\",\\\"params\\\":{\\\"name\\\":\\\"redbida_get_time_status\\\",\\\"arguments\\\":{}}}\"'"
```
