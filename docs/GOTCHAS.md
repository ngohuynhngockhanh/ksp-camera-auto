# Gotchas & vendor quirks (hard-won, all verified live unless noted)

## Dahua / DVRIP
- **Config port is DVRIP, not DHIP.** Sending a DHIP frame to 37777 gets no
  response. Use the `\xa0`/`\xb0` login + `\xf6` JSON framing.
- **Multi-frame responses.** Large NVR configs span several `\xf6` frames; read
  until `header[16:20]` (total) bytes are gathered. Symptom if not:
  `decode response: unexpected end of JSON input`.
- **`header[16:20]` is a length ONLY for `\xf6` frames.** On `\xb0` login frames
  it's not — applying a "too large" check there broke login with
  `frame too large: total=237140541`. Guard the total/reassembly to `header[0]==0xf6`.
- **Resolution is Width/Height ints + `CustomResolutionName`**, not a `resolution`
  string. Unsupported values → explicit error (code 268959743).
- **Codec change is silently ignored if unsupported** (returns ok). Always read back.
- **Smart Codec is the `SmartEncode` config**, per-channel, not part of `Encode`.

## Hikvision / ISAPI
- **PUT the FULL StreamingChannel doc.** A trimmed re-marshalled struct →
  `statusCode 4 "Invalid XML Content"`. Use raw-XML mutation of the GET'd doc.
- **Smart Codec: use the INLINE `<SmartCodec><enabled>` element.** Many
  cameras/NVR channels REJECT `PUT .../channels/<id>/smartCodec` with
  `statusCode 4 "Invalid Operation" / invalidOperation`. Verified on the
  192.168.1.210–218 fleet behind inut_204_142. Scope the `<enabled>` replace to
  inside the `<SmartCodec>` block (Video/Audio also have `<enabled>`). Fall back
  to the sub-resource only when there's no inline element.
- **`maxFrameRate` is fps×100.** NVR pass-through channels report `0` = "theo nguồn".
- **Channel ids on an NVR**: `GET /ISAPI/Streaming/channels` returns all
  (`101,201,…`) in one call — probe the whole fleet cheaply.
- **Port 8000 can't be black-box cloned** (encrypted handshake). Use ISAPI:80 on
  the LAN, or the cgo `hiksdk` build.
- **`replaceXMLTag` silently no-ops on an ABSENT tag** → a setter can "succeed"
  without changing anything. GOP/bitrate use `mutateStreamChannelStrict`, which
  errors if any target tag is missing from the device doc. New setters that edit
  raw XML must do the same.
- **Bitrate = AVERAGE when Smart Codec is ON.** `SetBitrate` reads the final
  smart-codec state first (so in `Apply` smart codec runs BEFORE bitrate), then
  writes `<vbrAverageCap>`/`<vbrUpperCap>` accordingly.
- **`videoQualityControlType` casing varies** (`VBR` vs `vbr`) — write the mode
  in the casing the device currently serves, or it may reject it.
- **`keyFrameInterval` unit varies by firmware** (ms on most, frames on some);
  prefer `<GovLength>` (always frames) when the doc has it.

## Channel name / OSD / snapshot
- **Dahua channel name** (`ChannelTitle[Channel].Name`) — confirmed against the
  official spec (§4.7) AND **live-verified** via `/api/channel-info` against a
  real deployed camera (returned `"Bàn S1"`, a real operator-set label).
- **Dahua 4-line custom OSD** (`VideoWidget[Channel].CustomTitle[index].Text`)
  — the `.Text` field wasn't in the locally shipped v1.40 PDF (only
  position/color/`EncodeBlend` were), but is **live-confirmed**: a real device
  returned 4 real `CustomTitle` slots with actual venue-name text in slots 0–1
  and empty strings in 2–3. `GetOSDLines`/`SetOSDLines`'s read path is proven;
  `SetOSDLines`'s WRITE path (and the `EncodeBlend` toggle it also writes) is
  still unverified — it requires a `Text` key to already exist (which it does
  on real hardware per the above) before attempting a write, so a firmware
  that genuinely lacks the key still fails loudly via `dahua.ErrOSDUnsupported`
  instead of silently no-op'ing.
- **Dahua snapshot** (`GET http://<host>:80/cgi-bin/snapshot.cgi?channel=<n>`,
  0-based, Digest auth, confirmed against spec §4.1.3) — **live-tested and
  confirmed UNREACHABLE from this environment**: the project's live Dahua test
  camera is NAT'd with only the DVRIP port (37777→54273) forwarded; hitting
  `/api/snapshot` against it correctly surfaces as a clean 502, not a hang or
  crash. Verify on-LAN (port 80 reachable) before relying on this in
  production. No sub-stream selector exists — the endpoint always returns
  whatever encoder the device's snapshot pipeline uses; the `stream` parameter
  is accepted at the API level but ignored for Dahua.
- **Hikvision channel name — CORRECTED, was wrong initially.**
  `/ISAPI/Streaming/channels/{id}`'s `<channelName>` field is **NOT** the
  operator-assigned name — on the live NVR it just held an internal id-like
  default (`"101"`, `"102"`...). The real name (live-confirmed: `"BAN 1"`,
  `"BAN 2"`, ... on an NVR proxying 9 remote IP cameras) lives at
  `/ISAPI/ContentMgmt/InputProxy/channels/{ch}` → `<name>`, `ch` = native
  channel number. `/ISAPI/ContentMgmt/InputProxy/channels` (no id) lists every
  channel's name in one GET — `ProbeAll` uses this instead of N per-channel
  calls. Devices that AREN'T an NVR proxying remote IP cameras (standalone IP
  camera, or an NVR with local/analog inputs) don't expose InputProxy at all —
  `GetChannelName`/`SetChannelName` fall back to
  `/ISAPI/System/Video/inputs/channels/{ch}` → `<name>` for those, but that
  fallback path is **not live-verified** (every Hikvision device reachable in
  this project turned out to be InputProxy-style; requesting
  `System/Video/inputs/channels/1` on it returned `statusCode 4 "Invalid
  Operation"`, confirming it doesn't apply there — but never tested where it
  DOES apply).
- **Hikvision snapshot** (`GET /ISAPI/Streaming/channels/{id}/picture`) —
  **live-verified**: returned a genuine 704×576 JPEG (26 KB) from a real NVR
  channel. Confirmed by this repo's own
  `docs-sdk/hikvision/hikvision-best-practices-README.md` too.
- **Hikvision OSD overlay** (`/ISAPI/System/Video/inputs/channels/{id}/overlays`,
  `TextOverlayList/TextOverlay/{id,enabled,displayText}`) — the field names
  were an educated guess from Hikvision's general ISAPI convention (no
  reference doc for this endpoint ships in this repo) and are now
  **live-verified for the READ path**: a real NVR channel returned genuine
  overlay text (`"ATV BILLIARDS ARENA"`, an address line). Write path
  (`SetOverlayText`) not live-tested. `GetOverlayText`/`SetOverlayText` return
  `isapi.ErrOverlayUnsupported` if a device's document has no
  `TextOverlayList`/`<displayText>`, so older firmware without this resource
  fails loudly instead of returning empty/wrong data.
- Both vendors' snapshot/OSD/name calls only support ISAPI-over-HTTP (port 80)
  and DVRIP/HTTP CGI (port 37777+80) — the `hiksdk` (SDK, port 8000) transport
  does not implement snapshot capture; see `internal/hiksdk` if that's ever
  needed (would require a new C shim wrapping
  `NET_DVR_CaptureJPEGPicture_NEW`).
- **None of the WRITE paths** (`SetChannelName`, `SetOSDLines`,
  `SetOverlayText`) have been exercised against a live device in this
  session — only reads, deliberately, to avoid mutating the production-seeded
  camera fleet without explicit operator sign-off. Confirm a write on one
  camera before relying on it at scale.

## Snapshot: RTSP+ffmpeg beat CGI and NetSDK (live-verified 2026-07-16)
- **Dahua NetSDK's `CLIENT_SnapPicture`/`CLIENT_SnapPictureEx`/`CLIENT_SnapPictureToFile`
  are vehicle-DVR-only** — confirmed straight from the real header
  (`dhnetsdk.h`, sourced from `github.com/hysios/dhnetsdk`, a third-party
  Linux SDK mirror): they sit under the `车载设备接口` ("vehicle-mounted
  device interface") section. Calling them against a standard IPC
  (`NET_IPC_SERIAL`) fails immediately client-side with `NET_ILLEGAL_PARAM`
  regardless of parameters — confirmed via packet capture (no snap-specific
  bytes ever hit the wire; only the standard post-login capability probe
  did). The only NetSDK path for a standard camera is
  `CLIENT_CapturePictureEx`, which requires an active `CLIENT_RealPlayEx`
  live stream and decodes client-side (H.264/H.265) — a much bigger lift
  than a single RPC call, and not pursued.
- **`snapshot.cgi` (`GetSnapshotCGI`) is unreliable on modern firmware**:
  live-tested against 9 real Dahua cameras on `inut_205_50`'s LAN and got a
  bare `Error\r\nBad Request!\r\n` from every one, auth notwithstanding.
- **RTSP + ffmpeg (`GetSnapshotRTSP`) works and is now the default**:
  `dahua.GetSnapshot` tries RTSP first (`rtsp://user:pass@host:554/cam/
  realmonitor?channel=<n+1>&subtype=1`, sub-stream for a cheap decode,
  `-skip_frame nokey -frames:v 1` to stop at the first keyframe rather than
  transcode), falling back to CGI on failure. Live-verified end-to-end
  through the deployed `/api/snapshot` on `inut_205_50` against a real
  camera (21KB JPEG, correct OSD timestamp/venue text). Requires `ffmpeg` on
  PATH on the deploy target — already present on the Armbian `inut_*` boxes
  (Shinobi depends on it), not yet verified as a hard requirement to add to
  the ansible role.
- The original NAT'd `dahua-ip` test camera (45.251.114.38:54273) still
  can't be reached this way: only its DVRIP port is forwarded, not
  HTTP:80 or RTSP:554 — a port-forwarding gap on that specific device, not
  a code issue. Both new paths and the old CGI path all need one of those
  two ports opened for that particular camera.

## Picture/color tuning + network config (Dahua/KBVision, unverified)
- **`VideoColor`/`VideoInOptions`** (`internal/dahua/picture.go`) — same
  caveat as OSD: `.Text`-style content fields and the day/night sub-profiles
  are sourced from the spec PDF, **not live-verified**. `GetPicture`/
  `SetPicture` deliberately return/accept raw `map[string]any` instead of a
  hand-typed struct (~90 fields across VideoColor + VideoInOptions +
  NightOptions + NormalOptions, varying by model per `GetVideoInputCaps`) —
  see the package doc comment. Only unit-tested against synthetic maps
  (`picture_test.go`); no write attempted against a live device.
- **`Network`/`WLan`** (`internal/dahua/network.go`) are **object-shaped**
  (keyed by interface name), the first tables in this codebase that aren't a
  per-channel array — hence the separate `getObjectTable`/`setObjectTable`.
  `SetStaticIP` validates every address (`net.ParseIP`) before sending
  anything, specifically because a bad IP/mask/gateway on a camera's only
  interface can make it unreachable. **Deliberately not live-tested** (read
  or write) in this session — the project's live Dahua test camera is
  NAT'd with only the DVRIP port forwarded, and mutating its network config
  was explicitly out of scope for automated testing; only unit tests
  (`network_test.go`) cover the validation/parsing logic.
- **Wi-Fi AP scan** (`ScanWiFi`, CGI `wlan.cgi?action=scanWlanDevices`) hits
  the same port-80-may-be-unreachable situation as `snapshot.cgi` — expect a
  clean error, not a hang, when only the DVRIP port is NAT'd/forwarded.
- **KBVision 8888 fallback** (`camera.Open`, `dahua.ErrDialUnreachable`) is
  classified purely on whether the initial `net.DialTimeout` to the
  configured port succeeded — a login/credential failure on 37777 is never
  reclassified as "try 8888 instead". Covered by `dial_test.go` against a
  closed port and a fake login-failure server; no real KBVision unit was
  available to verify the fallback end-to-end.

## Importer / Shinobi
- **`details` is a JSON-encoded STRING** from the live API (an exported dump has
  it as an object). Handle both. Symptom: `cannot unmarshal string into … details`.
- **Vendor from RTSP path**: `/Streaming/Channels/` → Hikvision; `/cam/realmonitor`
  → Dahua. RTSP port (554) ≠ the config port (Hik 80 on-LAN, Dahua 37777).

## Environment / ops
- **Sandbox DNS.** On the ksp dev box the sandboxed server process can't resolve
  some bare hostnames (`video.io.vn`) though subdomains resolve; use IPs there.
  Real deploys resolve DNS fine.
- **Encryption key persistence.** Camera passwords are AES-encrypted with
  `~/.kspcam.key` (or `KSPCAM_KEY_FILE`). If the key is lost, stored passwords
  become unreadable — the deploy pins it at `/opt/ksp-cam/.kspcam.key`.
- **Timeout.** Slow multi-channel NVRs need a higher per-device timeout; it's
  settable from the UI (5–600s, default 30).
- **Changing resolution/codec restarts the encoder** → RTSP viewers drop briefly.
  Changing GOP/bitrate carries the same caveat.
- **Devices clamp GOP/bitrate to their supported range.** Apply-verify is
  tri-state: exact match = OK; changed-but-different = OK (report the clamped
  value); unchanged = fail loudly.

## Security (see the audit)
- `nmap` subnet must be strictly validated (IPv4/CIDR/range) + passed after `--`
  (argument-injection → root arbitrary file write otherwise).
- Login rate-limited (5 fails → 1 min lock); `/api/*` bodies capped (8 MiB);
  session cookie `Secure` behind HTTPS.
- **Shared default `admin/smarthome12345`** on an internet-exposed service is an
  accepted operator tradeoff — front it with TLS-terminating frp.
- Never commit vendor SDKs, real camera credentials, or `cameras.yaml`
  (all gitignored). Several pushes were correctly blocked for leaked creds.
