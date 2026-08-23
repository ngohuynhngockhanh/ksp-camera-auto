# Investigation Report: Build Systems, Test Suites, Tooling, Sample Scripts & Test Harness Strategy

## 1. Observation

### 1.1 Build Systems & Toolchain Configuration

#### Top-level Files & Dependencies
- **`Makefile`** (`/home/ksp/ksp-camera-auto/Makefile`, lines 1–66):
  - Binary name: `kspcam` (`BINARY := kspcam`, `PKG := ./cmd/kspcam`, `DIST := dist`).
  - Version injection: `VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)`, injected via `LDFLAGS := -s -w -X main.version=$(VERSION)`.
  - Default Cgo state: `export CGO_ENABLED=0` (line 8) enforces pure-Go static binaries by default.
  - Standard targets: `all`, `build`, `run`, `test`, `vet`, `fmt`, `tidy`, `clean`.
  - Cross-compilation targets (`build-all`, lines 34–43):
    - `build-amd64`: `GOOS=linux GOARCH=amd64 $(GO) build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-amd64 $(PKG)`
    - `build-arm32`: `GOOS=linux GOARCH=arm GOARM=7 $(GO) build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-armv7 $(PKG)`
    - `build-arm64`: `GOOS=linux GOARCH=arm64 $(GO) build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-arm64 $(PKG)`
  - Cgo / Hikvision SDK target (`build-hiksdk`, lines 48–53):
    - Validates `HIKSDK` environment variable.
    - Compilation flags:
      ```makefile
      CGO_ENABLED=1 \
      CGO_CPPFLAGS="-I$(HIKSDK)/incEn" \
      CGO_LDFLAGS="-L$(HIKSDK)/lib -lhcnetsdk -Wl,-rpath,$(HIKSDK)/lib" \
      $(GO) build -tags hiksdk -ldflags '$(LDFLAGS)' -o $(BINARY)-hiksdk $(PKG)
      ```
    - Runtime requirement: `KSPCAM_HIKSDK_PATH=$(HIKSDK)/lib` (or dynamic linker search path) pointing to `libhcnetsdk.so` and `HCNetSDKCom/`.
  - Documentation generation targets (lines 57–61):
    - `docs`: `$(GO) run ./tools/docgen` compiles `docs/help/*.md` into `web/static/help/help-index.json`.
    - `docs-check`: `$(GO) run ./tools/docgen -check` fails if any HTTP API route or UI nav tab is uncovered.

- **`go.mod`** (`/home/ksp/ksp-camera-auto/go.mod`, lines 1–8):
  - Module: `github.com/ngohuynhngockhanh/ksp-camera-auto`
  - Go version: `go 1.25.0`
  - Direct dependencies:
    - `gopkg.in/yaml.v3 v3.0.1` (used in `internal/config` and `tools/docgen`)
    - `golang.org/x/crypto v0.54.0` (used for bcrypt and AES-GCM primitives)
  - Zero heavy external frameworks (no heavy web framework like Gin/Echo, no third-party ORM, no external SDK packages in default build).

- **`package.json`** (`/home/ksp/ksp-camera-auto/package.json`, lines 1–11):
  - Package: `ksp-camera-auto-ui-tests`
  - Scripts: `"test:ui": "playwright test"`
  - Dev dependencies: `@playwright/test: ^1.55.0`

- **Host Go Toolchain Path**:
  - Found Go binary at `/home/ksp/go-sdk/bin/go` (version: `go1.26.5 linux/amd64`).

---

### 1.2 Cgo & HCNetSDK Dynamic Library Architecture

- **`internal/hiksdk/sdk.go`** (`//go:build hiksdk`, lines 1–147):
  - Exposes `Open(host string, port int, user, pass string) (*Session, error)` and `(s *Session) Transport() isapi.Transport`.
  - Calls `ensureInit()` via `sync.Once`, passing `os.Getenv("KSPCAM_HIKSDK_PATH")` to `C.hik_init(clib)`.
  - Maps synchronous `NET_DVR_STDXMLConfig` calls to `isapi.Transport.Do(ctx, method, path, body)`.
- **`internal/hiksdk/stub.go`** (`//go:build !hiksdk`, lines 1–7):
  - Provides an empty package stub for default `CGO_ENABLED=0` builds.
- **`internal/hiksdk/shim.h`** & **`shim.cpp`** (`//go:build hiksdk`, 35 & 60 lines):
  - Plain C interface wrapping HCNetSDK C++ API:
    - `hik_init(const char *libdir)`: Calls `NET_DVR_Init()`, `NET_DVR_SetConnectTime(5000, 1)`, `NET_DVR_SetReconnect(10000, 1)`, `NET_DVR_SetSDKInitCfg(NET_SDK_INIT_CFG_SDK_PATH, &p)`.
    - `hik_login(ip, port, user, pass)`: Calls `NET_DVR_Login_V30`.
    - `hik_stdxml(uid, url, body, blen, out, outcap, outlen, status, statuscap)`: Executes `NET_DVR_STDXMLConfig` with `NET_DVR_XML_CONFIG_INPUT` and `NET_DVR_XML_CONFIG_OUTPUT`.
    - `hik_logout(uid)` / `hik_cleanup()`: Calls `NET_DVR_Logout` and `NET_DVR_Cleanup`.
- **`internal/camera/hik_http.go`** (`//go:build !hiksdk`) vs **`hik_sdk.go`** (`//go:build hiksdk`):
  - `!hiksdk`: `openHikClient` dials HTTP Digest ISAPI on port 80 via `hik.Dial(d.Host, d.Port, false, d.Username, d.Password, timeout)`.
  - `hiksdk`: `openHikClient` logs into port 8000 via `hiksdk.Open` and wraps it in `hik.NewWithClient(isapi.NewWithTransport(sess.Transport()), sess.Close)`.

---

### 1.3 Go Test Suite & Coverage Inventory

The repository contains **39 Go test files** across 11 internal packages:

| Package | Test Files | Tested Functionality | Statement Coverage |
|---|---|---|---|
| `internal/bulk` | `credtest_test.go` | Target normalization, alias mapping (LC/Lechange), sequential execution order, event stream dispatch | **45.8%** |
| `internal/camera` | `fps_test.go`, `port_fallback_test.go` | Safe FPS capability fallback, Dahua KBVision 37777->8888 port fallback logic | **1.3%** |
| `internal/config` | `crypto_test.go`, `inventory_delete_test.go` | AES-GCM encrypt/decrypt roundtrip, passthrough of legacy plaintext, bulk inventory deletion | **48.5%** |
| `internal/dahua` | `dial_test.go`, `dvrip_test.go`, `encode_test.go`, `identity_test.go`, `live_test.go`, `multiframe_test.go`, `name_test.go`, `network_test.go`, `osd_live_test.go`, `picture_test.go`, `record_test.go`, `storage_test.go`, `timeconfig_test.go`, `user_test.go`, `davdownload_test.go` | DVRIP login hash generation (gen1 Sofia hash, gen2 MD5 challenge), realm parsing, JSON-RPC frame mapping, error code strings, static IP validation, serial parsing | **19.0%** |
| `internal/discovery` | `discovery_test.go` | ONVIF probe XML generation with UUID v4, SADP probe XML, probe match XML parser, nmap stdout parser, safe scan target sanitizer | **46.8%** |
| `internal/hik` | `mediafind_test.go`, `playback_test.go`, `remotedevice_test.go`, `storage_test.go` | Remote device mapping, playback range formatting, media finding | **30.9%** |
| `internal/importer` | `shinobi_test.go` | Shinobi monitor JSON parser (string vs object `details`, vendor detection from RTSP path) | **71.2%** |
| `internal/isapi` | `digest_test.go`, `inputproxy_test.go`, `isapi_test.go`, `network_test.go`, `search_test.go`, `storage_test.go` | `fakeISAPIServer` with HTTP Digest auth (RFC 2617), XML mutation (`replaceXMLTagInNthBlock`), smartCodec get/set, GOP/bitrate XML adjustments, channel ID calculation | **64.2%** |
| `internal/nvrhealth` | `health_test.go` | NVR watchdog calculations, segment closing cooldown, disk growth stall detection | **81.9%** |
| `internal/server` | `cameras_delete_test.go`, `cameras_upsert_test.go`, `nvr_health_test.go`, `server_test.go`, `snapshot_cache_test.go` | `loginLimiter` IP lockout & expiry, camera upsert/merge logic, bulk camera deletion endpoint, NVR health check handler | **10.2%** |
| `internal/tiandy` | `tiandy_test.go` | Tiandy ISAPI CGI session auth, playback URL generation | **9.6%** |

- **Execution Result**: All 39 test files pass synchronously in `< 0.2s` (`go test ./...`).
- **Docgen Check**: `make docs-check` executes `tools/docgen -check` and checks that every HTTP route and UI hash tab has a corresponding Markdown help article or is listed in `docs/help/coverage-ignore.txt`. Currently flags pre-existing uncovered routes: `/api/nvr/health`, `/api/nvr/health/check`, `/api/nvr/watchdog`.

---

### 1.4 Playwright E2E UI Test Suite

- **Configuration (`playwright.config.js`)**:
  - `testDir: './tests/ui'`
  - `fullyParallel: true`
  - `baseURL: 'http://127.0.0.1:4173'`
  - `webServer`: Launches `python3 -m http.server 4173 --directory web/static` (pure static file server, Go backend is not invoked during UI tests).
  - Projects: `desktop` (Desktop Chrome) and `mobile` (iPhone 13 viewport with Chromium engine).
- **Test Fixtures (`tests/ui/fixtures.js`)**:
  - `mockApi(page, overrides)`: Intercepts all `**/api/**` requests via `page.route()`.
  - Default mock responses (`DEFAULTS`): `/api/config`, `/api/cameras`, `/api/probe`, `/api/fps-capability`, `/api/channel-info`, `/api/picture`, `/api/network`, `/api/wifi`, `/api/storage`, `/api/autoreboot`, `/api/device-time`, `/api/ptz`, `/api/nvr/*`, etc.
  - Streaming endpoints: `/api/apply` and `/api/password` mock Server-Sent Events (SSE) using `ndjson()` returning `data: {...}\n\n` event frames (`device_start`, `step`, `device_done`, `done`).
  - Binary endpoints: `/api/snapshot` and `/api/live` return a 1x1 transparent GIF (`PIXEL`); `/api/playback` returns empty video buffer.
  - Unmocked route guard: Returns HTTP 501 (`fixture thiếu mock cho ${path}`) to prevent silent test failures.
  - App synchronization: `openApp(page, { hash, overrides })` waits for `window.__kspReady === true`.
- **Test Files**:
  - `tests/ui/cameras.spec.js` (search, filtering, desktop sorting, row opening, bulk deletion, add/edit modal, inline rename, Dahua port persistence and QR code rendering).
  - `tests/ui/bulk.spec.js` (summary bar live updates, parameter toggles, tab selection persistence, SSE progress streaming to results tab, password change disclosure).
  - `tests/ui/detail.spec.js` (channel name & OSD editing, picture adjustment sliders, network IP/Wi-Fi configuration, storage info/format, auto-reboot).
  - `tests/ui/scan.spec.js` (discovery table rendering, credential testing, normalization of OEM aliases, adding discovered devices to inventory).
  - `tests/ui/nvr.spec.js` (NVR health dashboard, channel status badges, storage growth indicators, watchdog trigger, channel linking).
  - `tests/ui/review.spec.js` (playback timeline rendering via Vis Timeline, calendar navigation, video player token generation, export progress).
  - `tests/ui/nav.spec.js` & `mobile.spec.js` (navigation bar, mobile hamburger menu, table reflow without horizontal overflow).
- **Execution Result**: 91 passed, 11 skipped (by design for mobile/desktop conditions) in ~25s (`npm run test:ui`).

---

### 1.5 Sample Scripts & Diagnostic Tools

- **`chk_samples.js` & `chk_vnmap.js`**:
  - Standalone Node.js automation scripts utilizing `playwright` directly (`chromium.launch()`).
  - Target external portals (`https://apphub.a.inut.vn/citizen`, `https://apphub.a.vnmap3d.io.vn/citizen`).
  - Automated assertions:
    - Login submission (`#login input[name=phone]`, `#login input[name=password]`).
    - Waiting for sample question selector (`.sample`).
    - Extracting text content of sample chips.
    - Measuring element bounding boxes (`getBoundingClientRect()`, height, `fontSize`).
    - Simulating click on first question chip and asserting generated `.bubble` and removal of `.welcome`.
    - Mobile layout verification (`viewport: { width: 390, height: 844 }`): checks `document.documentElement.scrollWidth > window.innerWidth` to detect horizontal overflow.
    - Screenshot generation to `/tmp/*.png`.
- **`tools/hik-oracle/` (`README.md`, `oracle.cpp`)**:
  - Standalone C++ test utility linking directly against HCNetSDK (`-I<sdk>/incEn -L<sdk>/lib -lhcnetsdk`).
  - Used for reverse-engineering and proving port 8000 ISAPI XML tunneling via `NET_DVR_STDXMLConfig`.
- **`tools/docgen/` (`main.go`, `check.go`, `markdown.go`)**:
  - Tool to compile and validate markdown documentation in `docs/help/*.md` against all routes in `internal/server/server.go` and UI tabs in `web/static/app.js`.

---

## 2. Logic Chain

1. **Static Binary Architecture vs Cgo Separation**:
   - `Makefile` defaults to `CGO_ENABLED=0` with `LDFLAGS="-s -w -X main.version=..."` across all target architectures (`linux/amd64`, `linux/armv7`, `linux/arm64`). This produces zero-dependency static binaries that run cleanly across diverse embedded Linux / Armbian distributions without glibc/musl compatibility issues.
   - The proprietary Hikvision port 8000 SDK requires Cgo and dynamic linking against `libhcnetsdk.so`. This is cleanly isolated behind Go build tags (`//go:build hiksdk` vs `//go:build !hiksdk`) and an empty `stub.go`. The abstraction boundary is `isapi.Transport`, ensuring `internal/isapi` XML parsing logic is 100% shared regardless of whether the underlying transport is plain HTTP Digest or Cgo SDK.

2. **Decoupled Frontend Test Architecture**:
   - Playwright UI tests in `tests/ui/` do not spin up the Go backend. Instead, they spin up a lightweight Python HTTP static server on port 4173 serving `web/static/` and intercept all `/api/*` routes via Playwright's `page.route()`.
   - This design allows exhaustive UI testing (component states, SSE streaming, error banners, dialogs, responsive layouts) without needing any real cameras, mock backend daemons, or network permissions.

3. **Analysis of Current Backend Test Coverage Gaps**:
   - In Go unit tests, `internal/isapi` achieves 64.2% and `internal/nvrhealth` achieves 81.9% coverage because they use in-process mock HTTP servers (`fakeISAPIServer`).
   - Conversely, `internal/camera` (1.3%) and `internal/dahua` (19.0%) have lower statement coverage because `dahua.Client` communicates over a custom binary TCP socket protocol (DVRIP on port 37777). Existing tests only verify pure helpers (`hash.go`, `identity.go`, `encode.go`) and one basic 1-request fail server in `dial_test.go`.
   - To achieve high coverage and enable safe local/CI testing for AI agents, a **Mock Dahua DVRIP TCP Server** and **Mock Hikvision ISAPI Server** must be provided as a standard test harness.

---

## 3. Caveats

1. **Hardware-Bound Operations**:
   - Certain camera operations cannot be verified on mock sockets without simulating hardware feedback:
     - Wi-Fi Radio Scan (`dahua.ScanWiFiRPC` / `netApp.scanWLanDevices`): Physical RF scan timing and signal quality.
     - Physical PTZ mechanical movement: Motor stepping and limit stops.
     - Video encoder hardware restarts: Temporary RTSP stream drop when changing resolution/codec on real DSP chips.
2. **`docs-check` Known Coverage Gap**:
   - `make docs-check` currently reports missing documentation coverage for 3 NVR health endpoints: `/api/nvr/health`, `/api/nvr/health/check`, and `/api/nvr/watchdog`. These must be added to `docs/help/*.md` or `docs/help/coverage-ignore.txt` when updating documentation.
3. **Host Toolchain Environment**:
   - In the subagent shell environment, standard `/usr/bin/go` is not symlinked. The active Go toolchain resides at `/home/ksp/go-sdk/bin/go` (Go 1.26.5). All build and test scripts should export `PATH="/home/ksp/go-sdk/bin:$PATH"`.
4. **Live Device Safety Constraint**:
   - Never run unverified bulk write tests against production cameras listed in `cameras.yaml`. All automated tests and AI agent verifications must use mock simulators or loopback test servers.

---

## 4. Conclusion & Strategic Blueprints

### 4.1 Mock Camera Simulator & Test Harness Strategy

To enable comprehensive local testing without physical cameras, developers and AI agents can deploy lightweight in-process mock servers.

```
                    ┌────────────────────────────────────────────────────────┐
                    │                   Test Harness Space                   │
                    │                                                        │
                    │   ┌──────────────────────┐   ┌─────────────────────┐   │
                    │   │  Mock DVRIP Server   │   │  Mock ISAPI Server  │   │
                    │   │  (TCP :37777 / :0)   │   │ (httptest.Server:0) │   │
                    │   └──────────┬───────────┘   └──────────┬──────────┘   │
                    └──────────────┼──────────────────────────┼──────────────┘
                                   │                          │
                        DVRIP TCP  │               HTTP ISAPI │
                        Framing    │               Digest XML │
                                   ▼                          ▼
                    ┌────────────────────────────────────────────────────────┐
                    │                      kspcam Core                       │
                    │                                                        │
                    │   ┌──────────────────────┐   ┌─────────────────────┐   │
                    │   │   internal/dahua     │   │   internal/isapi    │   │
                    │   └──────────┬───────────┘   └──────────┬──────────┘   │
                    │              │                          │              │
                    │              └────────────┬─────────────┘              │
                    │                           ▼                            │
                    │               ┌───────────────────────┐                │
                    │               │    internal/camera    │                │
                    │               └───────────┬───────────┘                │
                    │                           ▼                            │
                    │               ┌───────────────────────┐                │
                    │               │     internal/bulk     │                │
                    │               └───────────┬───────────┘                │
                    │                           ▼                            │
                    │               ┌───────────────────────┐                │
                    │               │    internal/server    │                │
                    │               └───────────────────────┘                │
                    └────────────────────────────────────────────────────────┘
```

#### A. Mock Dahua DVRIP TCP Server Blueprint
- **Protocol Framing**:
  - Header: 32 bytes fixed length.
  - Header structure:
    - Bytes `[0:4]`: Frame Magic / Type (`0xa0010000` = Realm Req, `0xa0050000` = Login Req, `0xb0...` = Login Resp, `0xf6000000` = JSON-RPC).
    - Bytes `[4:8]`: Chunk length (Little-Endian `uint32`).
    - Bytes `[8:12]`: Sequence ID / Error code (`0x0008` = Login success, `0x0100` = Auth fail).
    - Bytes `[16:20]`: Total length for multi-frame JSON payload (or Session ID).
    - Bytes `[24:28]`: Session ID (`uint32`).
- **State Machine Implementation**:
  ```go
  type MockDVRIPServer struct {
      ln       net.Listener
      addr     string
      user     string
      pass     string
      realm    string
      random   string
      channels map[int]map[string]any // mock Encode / VideoWidget state
  }

  func StartMockDVRIP(t *testing.T, user, pass string) *MockDVRIPServer {
      ln, err := net.Listen("tcp", "127.0.0.1:0")
      if err != nil { t.Fatalf("listen: %v", err) }
      s := &MockDVRIPServer{
          ln:     ln,
          addr:   ln.Addr().String(),
          user:   user,
          pass:   pass,
          realm:  "Login to TESTDEVICE1234",
          random: "166042717d",
          channels: map[int]map[string]any{
              0: {"Width": float64(1920), "Height": float64(1080), "FPS": float64(25), "Compression": "H.264"},
          },
      }
      go s.serve()
      return s
  }
  ```
- **Supported RPC Handlers**:
  1. `configManager.getConfig?name=Encode`: Returns JSON `{"result": true, "params": {"table": [{"MainFormat": [...], "ExtraFormat": [...]}]}}`.
  2. `configManager.setConfig?name=Encode`: Merges modified parameters and returns `{"result": true}`.
  3. `magicBox.getSerialNo`: Returns `{"result": {"serialNumber": "8K01234PAZ56789"}}`.
  4. `magicBox.reboot`: Returns `{"result": true}`.
  5. `userManager.modifyPassword`: Verifies old hash and updates mock credential.
  6. Multi-frame fragmentation: Splitting responses over 1024 bytes into multiple `\xf6` packets to exercise multi-frame reassembly in `readFrame()`.

#### B. Mock Hikvision ISAPI HTTP Server Blueprint
- Built using standard `net/http/httptest.Server`.
- Implements RFC 2617 HTTP Digest authentication on all endpoints:
  - First request returns `401 Unauthorized` with `WWW-Authenticate: Digest realm="IP Camera", nonce="...", qop="auth"`.
  - Subsequent requests with `Authorization: Digest ...` validated via `checkAuth()`.
- Endpoints:
  - `GET/PUT /ISAPI/Streaming/channels/{id}`: Returns / updates `<StreamingChannel>` XML document.
  - `GET/PUT /ISAPI/Streaming/channels/{id}/smartCodec`: Returns / updates `<SmartCodec>` sub-resource.
  - `GET /ISAPI/Streaming/channels/{id}/capabilities`: Returns `<StreamingChannel>` with `<maxFrameRate opt="500,2000,2500" max="3000">`.
  - `GET /ISAPI/ContentMgmt/InputProxy/channels`: Returns NVR channel list.
  - `GET/PUT /ISAPI/System/Video/inputs/channels/{id}/overlays`: Returns / updates `<TextOverlayList>` XML.
  - `PUT /ISAPI/System/reboot`: Simulates reboot command.

#### C. Mock SDK Stub
- Pure-Go: Use `isapi.NewWithTransport(mockTransport)` where `mockTransport` implements:
  ```go
  type mockTransport struct {
      handlers map[string]func(body []byte) ([]byte, error)
  }
  func (m *mockTransport) Do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
      key := method + " " + path
      if h, ok := m.handlers[key]; ok {
          return h(body)
      }
      return []byte("<ResponseStatus><statusCode>1</statusCode></ResponseStatus>"), nil
  }
  ```

---

### 4.2 Go Coding Conventions & Repository Rules

1. **Dependency Discipline**:
   - Standard library first. Do NOT add external frameworks (Gin, Fiber, GORM).
   - The only allowed dependencies are `gopkg.in/yaml.v3` (YAML parsing) and `golang.org/x/crypto` (cryptography).
2. **Error Handling & Sentinel Checks**:
   - Always wrap errors with context: `fmt.Errorf("dahua dial %s: %w", addr, err)`.
   - Use sentinel errors for actionable branching (`errors.Is(err, dahua.ErrDialUnreachable)`, `errors.Is(err, dahua.ErrOSDUnsupported)`).
   - Provide safe fallbacks where appropriate (e.g. `safeFPSCapability` returning minimum 20 fps if capability query fails).
3. **Probe -> Apply -> Read-Back Pattern**:
   - Hardware writes must never assume success based on status return alone (some cameras silently ignore unsupported settings). Always execute a read-back check to confirm state.
4. **Concurrency & Thread Safety**:
   - Bulk operations (`internal/bulk`) MUST be executed sequentially per device.
   - Network sessions (`dahua.Client`, `hiksdk.Session`) must be protected with `sync.Mutex`.
   - All network connections must enforce deadlines / timeout contexts.
5. **Security Rules**:
   - Passwords in `cameras.yaml` must be AES-GCM encrypted (`enc:...`) using `internal/config/crypto.go`.
   - Never commit camera passwords, API keys, or proprietary SDK zip/shared libraries.
   - Strict sanitization on command execution (e.g. `isSafeScanTarget` for nmap).

---

### 4.3 AI Agent Quickstart Workflow

When an AI agent is assigned a task in `ksp-camera-auto`:

```
Step 1: Environment Setup
  export PATH="/home/ksp/go-sdk/bin:$PATH"
  go version  # Ensure Go 1.25+ is accessible

Step 2: Verification Baseline
  go test ./...         # Run full Go test suite (must pass in <0.5s)
  npm run test:ui       # Run Playwright UI tests (102 tests)
  make docs-check       # Verify help documentation coverage

Step 3: Implementing Changes
  - Backend logic: internal/<vendor>/ or internal/camera/ or internal/server/
  - Follow Go conventions: explicit error wrapping (%w), sentinel errors, zero new dependencies.
  - If adding an HTTP route or UI tab: update docs/help/*.md or docs/help/coverage-ignore.txt.

Step 4: Writing Tests (TDD)
  - Unit tests: Create *_test.go alongside modified package.
  - For protocol/network logic: Use loopback net.Listen or httptest.Server (see mock blueprints).
  - For UI changes: Update tests/ui/*.spec.js and mock endpoints in tests/ui/fixtures.js.

Step 5: Pre-Commit Quality Gate
  go fmt ./...
  go vet ./...
  go test -cover ./...
  npm run test:ui
  make build-all        # Verify cross-compilation for amd64, armv7, arm64
```

---

## 5. Verification Method

### 5.1 Verification Commands

Run the following commands to independently verify the findings in this report:

```bash
# 1. Ensure Go toolchain is on PATH
export PATH="/home/ksp/go-sdk/bin:$PATH"

# 2. Verify Go unit test suite
go test -v ./...

# 3. Verify statement coverage per package
go test -cover ./...

# 4. Verify cross-compilation targets
make build-all
ls -lh dist/

# 5. Verify Playwright E2E UI tests
npm run test:ui

# 6. Verify documentation generator check
make docs-check
```

### 5.2 Verification Output Summary
- `go test ./...` exits with code 0.
- `make build-all` successfully generates:
  - `dist/kspcam-linux-amd64` (~9.5MB)
  - `dist/kspcam-linux-armv7` (~8.9MB)
  - `dist/kspcam-linux-arm64` (~9.2MB)
- `npm run test:ui` executes 102 tests (91 passed, 11 skipped) with exit code 0.
- `make docs-check` identifies uncovered routes (`/api/nvr/health`, `/api/nvr/health/check`, `/api/nvr/watchdog`).
