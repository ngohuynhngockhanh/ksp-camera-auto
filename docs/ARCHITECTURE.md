# Architecture

`ksp-camera-auto` (`kspcam`) is a single static Go binary that serves a web UI
(default `:2028`) for bulk-configuring **Hikvision** and **Dahua/KBVision** IP
cameras and NVRs. Login default `admin` / `smarthome12345`.

## Package layout

```
cmd/kspcam            Entrypoint, flags (--config, --addr, --hash-password,
                      --import-shinobi, --version), graceful shutdown.
internal/config       YAML config + file-backed camera Inventory. AES-GCM
                      encryption of stored camera passwords (crypto.go).
internal/dahua        Pure-Go DVRIP client (TCP, port 37777 family): framing,
                      login, configManager get/set (Encode, SmartEncode).
internal/isapi        Hikvision ISAPI: HTTP Digest transport + XML get/set.
                      Pluggable Transport interface (HTTP default; SDK for 8000).
internal/hik          Thin adapter over isapi.Client.
internal/hiksdk       OPTIONAL cgo backend (build tag `hiksdk`): HCNetSDK on
                      port 8000, carrying ISAPI XML via NET_DVR_STDXMLConfig.
internal/camera       Vendor-agnostic Camera interface + factory (Open).
internal/bulk         Sequential orchestrator, streams progress Events (SSE).
internal/importer     Parse Shinobi monitor JSON -> devices (RTSP + vendor).
internal/discovery    LAN scan: ONVIF/Dahua/Hik UDP + nmap subnet.
internal/server       Web server: session login, JSON API, SSE, static UI.
web/                  Embedded UI (go:embed).
```

## Camera abstraction

`camera.Camera` is the seam every vendor implements:

```go
type Camera interface {
    Probe(ctx) ([]StreamInfo, error)               // read all channels/streams
    Apply(ctx, Profile, emit func(StepResult)) []StepResult
    ChangePassword(ctx, newUser, newPass string) error
    Close() error
}
```

`camera.Open(device)` dials the right client by `device.Vendor`. `Profile`
carries which settings to change (resolution, codec, smart codec, AAC, I-frame
interval/GOP, bitrate+mode) and the
target `Channels`/`Streams`. Adapters iterate channel × stream, emit one
`StepResult` per action (with GET read-back verification), and never fail the
whole batch on one error.

## Transports per vendor

| Vendor | Reachable via | Transport | Build |
|---|---|---|---|
| Dahua / KBVision | port 37777 (NAT or LAN) | DVRIP (pure Go) | default |
| Hikvision (on LAN) | port 80 | ISAPI / HTTP Digest (pure Go) | default |
| Hikvision (NAT'd 8000 only) | port 8000 | HCNetSDK via cgo | `-tags hiksdk` |

The default binary is **CGO-free** and cross-builds to amd64/arm32/arm64. The
`hiksdk` build is cgo (needs the HCNetSDK per arch) and is only for reaching
Hikvision when *only* the proprietary 8000 port is exposed — on a LAN, plain
ISAPI on port 80 is used instead (same XML, no SDK).

## Request flow (bulk apply)

1. UI `POST /api/apply` (or `/api/password`) — JSON `{deviceIds, profile, timeoutSeconds}`.
2. `bulk.Apply` processes devices **one at a time** (safety), emitting Events.
3. Server streams each Event as `data: <json>\n\n` (SSE); the UI renders a live log.
4. Each adapter does GET → mutate → PUT, then GET read-back to confirm.

## Security posture

See [GOTCHAS.md](GOTCHAS.md) and the audit summary in `SECURITY.md`. Key controls:
per-IP login rate-limit, request-body cap, `Secure` cookie behind HTTPS, strict
nmap-target validation, AES-encrypted stored passwords. The shared default login
(`admin/smarthome12345`) is an accepted operator tradeoff for fleet convenience.
