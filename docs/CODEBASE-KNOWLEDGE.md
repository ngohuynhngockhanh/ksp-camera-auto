# KSP Camera Auto - Codebase Knowledge (verified 2026-08-24)

Tai lieu nay la ban do hien tai cua repo, duoc doi chieu tu source code va cac
test/fixture dang co. Khi tai lieu cu mau thuan voi code, uu tien code thuc te.
Chi tiet protocol DaHua/Hikvision va gotcha live van nam trong
`GEMINI.md`, `docs/PROTOCOL-DAHUA.md`, `docs/PROTOCOL-HIKVISION.md` va
`docs/GOTCHAS.md`.

## 1. What This Repo Is

`ksp-camera-auto` (`kspcam`) la mot binary Go phuc vu web UI va API de:

- quan ly inventory camera/NVR;
- probe va cau hinh hang loat camera theo tung thiet bi, tuan tu;
- xem snapshot/live, recording search va playback/export;
- discovery trong LAN va import monitor tu Shinobi;
- quan ly monitor Shinobi bang REST;
- expose 24 MCP tools qua Stdio va HTTP/SSE.

Binary mac dinh duoc build voi `CGO_ENABLED=0`, web assets duoc nhung qua
`web/embed.go`. Build `hiksdk` la ngoai le, dung cgo va HCNetSDK cho Hikvision
chi mo cong 8000.

## 2. Runtime Map

```text
cmd/kspcam/main.go
  -> config.Load(config.yaml)
  -> config.LoadInventory(cameras.yaml)
  -> server.New(cfg, inventory)
       -> embedded web/static
       -> internal/mcp Server
       -> optional Shinobi client
       -> session store, login limiter, snapshot cache, NVR watchdog
  -> net/http on cfg.server.addr (default :2028)
```

CLI modes:

- `--config <path>`: choose YAML config;
- `--addr <addr>`: override listener;
- `--version`: print build version;
- `--hash-password <pw>`: print bcrypt hash;
- `--import-shinobi <file>` with `--import-hik-port`/`--import-dahua-port`:
  import monitors into the inventory and exit;
- `--mcp`: run MCP JSON-RPC on stdin/stdout; logs stay on stderr.

The repository also contains empty directories `cmd/nvrdiag/` and
`internal/localrecorder/`; they are mentioned by older architecture documents
but currently have no Go source and are not part of `go test ./...`.

## 3. Package Ownership

| Package | Responsibility | Main seam |
|---|---|---|
| `internal/config` | YAML defaults, inventory persistence, AES-GCM password encryption | `Config`, `Inventory`, `Device` |
| `internal/camera` | Vendor-neutral control surface and capability interfaces | `Camera`, `Profile`, `Open` |
| `internal/bulk` | Sequential bulk execution and credential testing | `Apply`, progress events |
| `internal/server` | HTTP routes, auth, SSE, playback, NVR health/watchdog | `Server.Handler()` |
| `internal/dahua` | DVRIP login/framing, configManager, media and device controls | pure Go TCP/HTTP/RTSP |
| `internal/isapi` | Hikvision-compatible XML API and Digest transport | `Transport`, `Client` |
| `internal/hik` | Adapter over `isapi` | HTTP or SDK-backed client |
| `internal/hiksdk` | Optional HCNetSDK cgo transport | build tag `hiksdk` |
| `internal/tiandy` | Tiandy RTSP media + session-auth ISAPI config plane | pure Go |
| `internal/discovery` | ONVIF, Dahua DHDiscover, Hik SADP, nmap scan | `Scan`, `ScanSubnet` |
| `internal/importer` | Shinobi monitor JSON to inventory devices | `ParseShinobi` |
| `internal/shinobi` | Shinobi REST CRUD and manual two-way sync | `Client`, `SyncReport` |
| `internal/mcp` | JSON-RPC 2.0 / MCP registry, Stdio, HTTP/SSE | `Server`, `Registry` |
| `internal/nvrhealth` | Recording/storage/time health classification | report/status model |
| `internal/mediaexport` | Parallel RTSP chunk fetch + ffmpeg MP4/MKV remux | `FastMP4Range`, `FastNativeRange` |
| `web/static` | SPA HTML/CSS/JS, help bundle, timeline and QR assets | embedded by `web/embed.go` |
| `tools/docgen` | Build/check `web/static/help/help-index.json` | `make docs`, `make docs-check` |

## 4. Data and Persistence

Top-level YAML config (`config.example.yaml`):

- `server`: address, admin credentials or `password_hash`, viewer credentials,
  failed-login threshold and lockout minutes;
- `cameras_file`: inventory path, default `cameras.yaml`;
- `defaults`: vendor ports, credentials, operation timeout, bulk new password,
  review-hour limit;
- `shinobi`: `api_url`, `api_key`, `group_key`;
- `mcp`: `enabled`, `api_key`, `allow_unauthenticated_loopback`.

`config.Inventory` is guarded by `sync.RWMutex`. `Upsert`, `Delete` and
`DeleteMany` write through a temporary file and rename it into place. Device
passwords are decrypted in memory and saved as `enc:<base64>` AES-256-GCM.
Key source precedence: `KSPCAM_KEY`, then `KSPCAM_KEY_FILE`, then
`~/.kspcam.key` (0600, auto-created).

Important `config.Device` fields include `id`, `host`, `port`, `vendor`,
credentials, serial number, NVR fallback/link fields, `isNvr`, and watchdog/time
sync flags. IDs default to `host:port`; multi-channel Shinobi imports may use
`host:port-cN`.

## 5. Camera and Vendor Planes

### Common control flow

`camera.Open` selects the adapter by `config.Vendor`. `Camera.Probe` reads
current streams. `Camera.Apply` executes only enabled fields in `camera.Profile`
and emits one `StepResult` per action. Vendor writes are followed by read-back;
callers can distinguish exact, degraded/clamped, and failed/unchanged outcomes.

Capability interfaces keep risky or vendor-specific operations out of the base
interface: FPS, serial, picture, network/Wi-Fi, reboot, storage, remote-device
listing, auto-reboot, device time, NVR health, recording, and PTZ.

### Dahua / KBVision

- DVRIP over TCP 37777, with automatic retry on 8888 and persisted fallback;
- 32-byte binary framing, two-step challenge/login, `configManager` JSON-RPC;
- encode, smart codec, AAC, GOP/FPS/bitrate, OSD, channel name, picture,
  network/Wi-Fi, storage, reboot, PTZ, recordings and playback;
- snapshot path prefers RTSP+ffmpeg in current implementation; HTTP CGI is a
  fallback/legacy path and may be unreachable behind NAT.

### Hikvision

- default adapter uses ISAPI HTTP(S) with RFC 2617 Digest;
- XML GET/modify/PUT is shared by encode, smart codec, AAC, GOP/bitrate,
  network, channel names, overlays, storage, reboot and playback/search;
- channel IDs follow `channel*100 + stream + 1` (`101`, `102`, `201`, ...);
- `-tags hiksdk` swaps the transport to HCNetSDK `NET_DVR_STDXMLConfig` over
  port 8000 while reusing the same XML logic.

### Tiandy

- RTSP `:554` is the media/playback plane;
- ISAPI-compatible config is on `:8081` with Tiandy CGI session auth, wrapped as
  an `isapi.Transport`;
- playback uses shared media export; native `.dav` and some Wi-Fi operations
  remain unsupported.

## 6. HTTP Surface

Public: `/login`, `/logout`, `/healthz`.

Authenticated API families:

- inventory/import/probe: `/api/cameras*`, `/api/import`, `/api/probe`,
  `/api/fps-capability`;
- bulk/config: `/api/apply`, `/api/password`, `/api/channel-*`, `/api/osd`,
  `/api/picture`, `/api/network`, `/api/wifi*`, `/api/device-time`,
  `/api/autoreboot`, `/api/ptz`, `/api/reboot`, `/api/storage`;
- discovery: `/api/scan`, `/api/scan/try-password`;
- media: `/api/snapshot`, `/api/live`, `/api/recordings`, `/api/playback`,
  `/api/playback-token`, `/api/export-progress`;
- NVR: `/api/nvr/scan`, `/api/nvr/link`, `/api/nvr/channels`,
  `/api/nvr/health`, `/api/nvr/health/check`, `/api/nvr/watchdog`;
- Shinobi: `/api/shinobi/status`, `/api/shinobi/monitors`, sync-to/from routes,
  and `/api/shinobi/videos`;
- embedded MCP: `GET/POST /mcp`, `POST /mcp/messages`.

Sessions use the `kspcam_session` HttpOnly cookie with a 12-hour in-memory TTL.
Viewer accounts are read-only for config, camera list, recordings, live,
snapshot and playback-token. Login failures are limited per client IP (default
5 failures, 30-minute lockout). Request bodies are capped at 8 MiB on API
handlers.

Playback links can be signed with a short-lived HMAC token, allowing a QR-scanned
phone to fetch `/api/playback` without an admin session. The HMAC key is random
per server process, so tokens do not survive restart.

## 7. MCP Surface

Protocol version: `2024-11-05`, JSON-RPC `2.0`.

Transports:

- `kspcam --mcp`: newline-delimited JSON-RPC on stdin/stdout;
- HTTP stateless `POST /mcp`;
- HTTP SSE `GET /mcp` plus `POST /mcp/messages?sessionId=...`.

The registry currently exposes 24 tools:

```text
kspcam_list_cameras             kspcam_upsert_camera
kspcam_delete_camera            kspcam_probe_camera
kspcam_apply_profile             kspcam_set_channel_name
kspcam_set_osd                   kspcam_reboot_camera
kspcam_change_password           kspcam_scan_lan
kspcam_try_password              kspcam_wifi_scan
kspcam_get_network               kspcam_get_nvr_health
kspcam_get_recordings            kspcam_get_snapshot
shinobi_list_monitors            shinobi_add_monitor
shinobi_edit_monitor             shinobi_delete_monitor
shinobi_sync_to_shinobi          shinobi_sync_from_shinobi
shinobi_sync_inventory           shinobi_change_monitor_state
shinobi_get_videos
```

HTTP MCP auth accepts loopback bypass when configured, `X-MCP-Key`, Bearer
token, or `key`/`apiKey` query parameters. `MCPConfig.Enabled` is present in the
configuration model; route registration is currently performed by
`server.New` regardless, so treat the flag as a compatibility/configuration
surface until an explicit enforcement check is added.

## 8. Shinobi Sync Rules

`internal/shinobi` builds deterministic monitor IDs from host/port or NVR
channel. `SyncToShinobi` creates or updates monitors only when relevant fields
changed. `SyncFromShinobi` parses `AutoHost`, detects vendor from RTSP path,
preserves existing NVR links/watchdog flags, and groups multi-channel hosts.
Sync is manual; there is no background inventory-to-Shinobi reconciliation loop.

## 9. Discovery and Media Constraints

ONVIF, Dahua DHDiscover and Hik SADP require the same L2 broadcast domain.
`ScanSubnet`/nmap is the routed-subnet option and only infers open camera-like
ports. Discovery errors are best-effort and do not discard results from other
methods.

`mediaexport` uses ffmpeg, temporary files and bounded parallel RTSP chunks.
Whole-file exports are serialized process-wide to protect small deploy boxes;
chunk concurrency is separately bounded. MP4 may transcode Tiandy G.711 audio
to AAC; native MKV preserves the original audio/video bitstreams.

## 10. Tests, Builds, and Verification

Make targets:

```text
make build          # static kspcam, CGO_ENABLED=0
make test           # go test ./...
make vet            # go vet ./...
make fmt            # go fmt ./...
make build-all     # linux amd64, armv7, arm64
make build-hiksdk  # optional cgo HCNetSDK build
make docs           # regenerate help-index.json
make docs-check     # verify help coverage
```

Current source inventory contains 45 Go test files and 8 Playwright specs.
Playwright serves `web/static` with Python and mocks every `/api/*` call in
`tests/ui/fixtures.js`; it does not boot the Go backend.

Verification note for this snapshot: the current shell does not have the `go`
binary installed, so Go tests/build/vet could not be executed here. Node-based
fixtures and committed artifacts remain inspectable, but a Go-capable CI or
developer machine must run the Make targets before release.

## 11. Change Navigation

| Task | Start here |
|---|---|
| Add/modify HTTP endpoint | `internal/server/server.go`, then handler in `internal/server/api.go` or `api_shinobi.go` |
| Change bulk behavior | `internal/bulk/bulk.go`, `internal/camera/camera.go` |
| Add vendor operation | vendor client, then capability interface/adapter, then server route or MCP tool |
| Change Dahua wire behavior | `internal/dahua/dhip.go`, related tests and `docs/PROTOCOL-DAHUA.md` |
| Change Hikvision XML | `internal/isapi`, `internal/hik`, related fake-server tests |
| Change inventory/config | `internal/config/config.go`, `inventory.go`, `crypto.go` |
| Add MCP tool | `internal/mcp/tools_*.go`, then `internal/mcp/*_test.go` |
| Change Shinobi mapping/sync | `internal/shinobi/sync.go`, `client.go`, tests |
| Change UI | `web/static/app.js`, `review.js`, `ui-core.js`, matching Playwright spec/fixture |
| Change help content | `docs/help/*.md`, run `make docs` and `make docs-check` |

## 12. Known Documentation Drift / Follow-up

- `GEMINI.md`, `AGENTS.md` and parts of `docs/ARCHITECTURE.md` still describe
  the empty `cmd/nvrdiag` and `internal/localrecorder` directories as active
  implementations.
- Older documents call Hikvision port 8000 the default, while the pure-Go
  HTTP adapter requires the configured ISAPI port (normally 80); 8000 is only
  available in the optional `hiksdk` build.
- `MCPConfig.Enabled` is not currently enforced by route construction.
- The repository has extensive unit/fake-server coverage, but live write paths
  for risky device operations should still be validated on one maintenance
  camera before fleet rollout; see `docs/GOTCHAS.md`.
