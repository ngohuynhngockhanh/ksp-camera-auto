# Gotchas & vendor quirks (hard-won, all verified live)

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

## Security (see the audit)
- `nmap` subnet must be strictly validated (IPv4/CIDR/range) + passed after `--`
  (argument-injection → root arbitrary file write otherwise).
- Login rate-limited (5 fails → 1 min lock); `/api/*` bodies capped (8 MiB);
  session cookie `Secure` behind HTTPS.
- **Shared default `admin/smarthome12345`** on an internet-exposed service is an
  accepted operator tradeoff — front it with TLS-terminating frp.
- Never commit vendor SDKs, real camera credentials, or `cameras.yaml`
  (all gitignored). Several pushes were correctly blocked for leaked creds.
