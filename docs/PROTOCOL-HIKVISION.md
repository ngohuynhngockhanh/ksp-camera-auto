# Hikvision — ISAPI (port 80) and HCNetSDK (port 8000)

Two transports carry the **same ISAPI XML**:
- **ISAPI over HTTP Digest (port 80)** — pure Go, used on the camera LAN.
- **HCNetSDK on port 8000** — cgo (`-tags hiksdk`), for devices reachable only
  on the proprietary port. The SDK tunnels ISAPI via `NET_DVR_STDXMLConfig`.

## ISAPI basics

- HTTP **Digest** auth (MD5, qop=auth); Go stdlib has no client → `internal/isapi/digest.go`.
- Channel id = `channelNo*100 + streamType`: `101` main, `102` sub. On an NVR,
  cameras appear as `101, 201, …, 801`. Read them all in ONE call:
  `GET /ISAPI/Streaming/channels` → `StreamingChannelList`.
- State-changing PUT returns `<ResponseStatus><statusCode>1` on success.

### GET-modify-PUT must preserve the FULL document
Real cameras/NVRs reject a trimmed re-marshalled `StreamingChannel` with
`statusCode 4 "Invalid XML Content"`. So the setters do **raw-XML mutation**: GET
the full doc, replace only the target tag values, PUT the whole doc back.

### Field mapping
- **Resolution**: `videoResolutionWidth` / `videoResolutionHeight`; `maxFrameRate`
  is **fps×100** (2500 = 25fps). NVR pass-through channels report `maxFrameRate=0`
  ("theo nguồn").
- **Codec**: `videoCodecType` = `H.264` / `H.265` / `MJPEG`.
- **Smart Codec (H.264+/H.265+)**: TWO conventions — support both:
  1. **Inline** `<Video><SmartCodec><enabled>true</enabled></SmartCodec>` in the
     StreamingChannel doc. **Preferred** — many cameras/NVR channels REJECT the
     sub-resource with `Invalid Operation`.
  2. Standalone `PUT /ISAPI/Streaming/channels/<id>/smartCodec`. Fallback only.
  H.265+ requires the base codec to be H.265 first.
- **I-frame interval (GOP)**: `<Video><GovLength>` in **frames** (verified live).
  Some firmware exposes `<keyFrameInterval>` instead — often in **ms**, so
  convert `frames→ms = gop*1000/fps` (fps = `maxFrameRate`/100); `gopEdits`
  prefers `GovLength` when both exist.
- **Bitrate**: `<Video>` children — `<videoQualityControlType>` = `VBR`/`CBR`
  (case-sensitive; some firmware serves lowercase `vbr`/`cbr` and rejects the
  wrong case, so mode writes match the device's current casing). VBR cap =
  `<vbrUpperCap>` (Kbps), floor `<vbrLowerCap>`; CBR target = `<constantBitRate>`.
  **When Smart Codec (H.264+/H.265+) is ON the device treats the configured
  bitrate as AVERAGE** — write `<vbrAverageCap>` if present, else fall back to
  `<vbrUpperCap>`/`<constantBitRate>`. Read-back (`bitrateFromVideo`) mirrors the
  same precedence so verify doesn't false-fail.
- **Audio AAC**: `<Audio><audioCompressionType>AAC</audioCompressionType>`.
- **Password change**: `PUT /ISAPI/Security/users/<id>` (id 1 = admin) with
  `<User><id>1</id><userName>…</userName><password>…</password></User>`.
- **Channel name** (⚠ write path not live-verified): `<channelName>` inside the
  StreamingChannel doc — already a struct field (`ChannelName`), just wasn't
  read/written for this purpose before. Same `mutateStreamChannelStrict`
  pattern as GOP/bitrate.
- **Snapshot**: `GET /ISAPI/Streaming/channels/<id>/picture` → raw JPEG bytes,
  no XML envelope. Confirmed by this repo's own
  `docs-sdk/hikvision/hikvision-best-practices-README.md`; not tested live in
  this codebase (the live test camera here is SDK-8000-only).
- **OSD text overlay** (⚠ entirely unverified, see `docs/GOTCHAS.md`):
  `GET/PUT /ISAPI/System/Video/inputs/channels/<id>/overlays` (note: `<id>` here
  is the native channel number, NOT the compound streaming-channel id) →
  `<VideoOverlay><TextOverlayList><TextOverlay><id>/<enabled>/<displayText></TextOverlay>…`.
  Standard Hikvision ISAPI convention from general domain knowledge, not
  confirmed against a live device or any reference doc in this repo.

## HCNetSDK on 8000 (the `hiksdk` build)

The 8000 protocol is closed and **cannot be black-box cloned**: it is silent to
probes and its handshake is encrypted (the SDK bundles libcrypto/ssl). Verified
live against a DS-7108NI-Q1/M NVR.

**Solution:** optional cgo backend `internal/hiksdk` (build tag `hiksdk`):
- A C++ shim exposes plain-C `hik_login`/`hik_stdxml` over the C++ `HCNetSDK.h`.
- `NET_DVR_Login_V30` on 8000 → `NET_DVR_STDXMLConfig` carries ISAPI XML.
- `internal/isapi` was refactored to a pluggable `Transport`; the SDK session
  implements it, so ALL the ISAPI get/set logic is reused unchanged.
- GET → the SDK out-buffer; PUT → the SDK status-buffer (the ResponseStatus).

Build/run:
```bash
make build-hiksdk HIKSDK=/path/to/HCNetSDK   # dir with incEn/ and lib/
KSPCAM_HIKSDK_PATH=<HIKSDK>/lib ./kspcam-hiksdk --addr 0.0.0.0:2028
```
The SDK is downloaded from Hikvision's official site (Akamai bot-protected → must
be fetched via a browser, not curl) into `docs-sdk/` (gitignored, never committed).
Dev oracle: `tools/hik-oracle/` (links the SDK to study/prove the 8000 path).
