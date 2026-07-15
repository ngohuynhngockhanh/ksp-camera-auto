# Dahua / KBVision — DVRIP protocol (port 37777)

KBVision is a Dahua OEM: identical protocols. The forwarded "config port"
(37777 family) speaks **DVRIP**, NOT DHIP. (DHIP — magic `0x2000000044484950` —
only runs on the HTTP/HTTPS ports and TCP/5000.) Reverse-engineered from mcw0's
DahuaConsole; reference sources live in `docs-sdk/dahua/` (gitignored).

## Framing (32-byte header)

All frames: a 32-byte header + payload.

- **Login (`\xa0` request / `\xb0` response)** — binary handshake below.
- **JSON (`\xf6`)** — after login, JSON-RPC. Header:
  - `[0:4]` = `0xf6000000` (big-endian marker)
  - `[4:8]` = **chunk length** of THIS frame (LE)
  - `[8:12]` = request id (LE)
  - `[16:20]` = **total** JSON length (LE) — see multi-frame below
  - `[24:28]` = session id (LE)

> **Multi-frame (critical):** a large JSON response (e.g. an NVR's whole `Encode`
> config) is split across SEVERAL `\xf6` frames, each with its own 32-byte
> header. `header[16:20]` on the first frame is the TOTAL payload size; keep
> reading (header + chunk) frames and concatenating until you have `total` bytes.
> `header[16:20]` is a length ONLY for `\xf6` frames — on `\xb0` login frames it
> is not a length, so never apply the "too large" check there. (Both were real
> bugs — see GOTCHAS.)

## Login (two-step MD5 challenge)

1. Send realm request: header `0xa0010000` + 20×`0x00` + `0x050201010000a1aa`, no payload.
2. Device replies (`\xb0`) with text `Realm:Login to <REALM>\r\nRandom:<RANDOM>\r\n`.
   - `realm` = the full `Login to <hex>` string; `random` = the Random value.
3. Compute the login hash:
   - `gen1` = Dahua "sofia" 8-char hash of the password (MD5 → compressor).
   - `gen2` = `UPPER(MD5(user:random:UPPER(MD5(user:realm:pass))))`.
   - `loginHash = user + "&&" + gen2 + UPPER(MD5(gen1))`.
4. Send login: header `0xa0050000` + `LE32(len(hash))` + 16×`0x00` +
   `0x050200080000a1aa`, payload = the hash string.
5. Response header: `ErrorCode` at `[8:12]` (`0x0008` = OK), `SessionID` at `[16:20]`.

Then all calls are JSON-RPC `{"method","params","id","session"}` over `\xf6` frames.

## Config get/set (`configManager`)

- `configManager.getConfig {name:"Encode"}` → `params.table` (array, one per channel).
- `configManager.setConfig {name:"Encode", table:[…]}` — send back the full table.
- Channel N: `table[N].MainFormat[0]` (main), `ExtraFormat[0]/[1]` (sub1/sub2).

### Field mapping (verified on live cams/NVR)
- **Resolution**: `Video.Width` / `Video.Height` (integers) + `CustomResolutionName`
  (`"WxH"`). NOT a `resolution` string. Device **errors** on unsupported values
  (code 268959743) — offer only supported resolutions.
- **Codec**: `Video.Compression` = `H.264` / `H.264H` (High) / `H.264B` (Baseline)
  / `H.265` / `MJPG`; `Video.Profile` = `Main`/`High`/`Baseline`. Unsupported
  codec is **silently ignored** (no error) → always read back to verify.
- **Smart Codec / H.265+**: SEPARATE config `SmartEncode.table[ch].Enable` (bool),
  per-channel.
- **I-frame interval (GOP)**: `Video.GOP` (integer, **frames**; verified live —
  e.g. 50/60).
- **Bitrate**: `Video.BitRate` (integer, **Kbps**) + `Video.BitRateControl` =
  `VBR`/`CBR`. Same `Encode` table as resolution/codec, so it round-trips through
  the same `getTable`/`setTable`. The device may **clamp** to a supported step —
  read back and report the accepted value rather than hard-failing.
- **Audio AAC**: `MainFormat[0].Audio.Compression = "AAC"` + `AudioEnable = true`.
- **Password change**: `userManager.modifyPassword {name, pwd, pwdOld}` (pwdOld =
  the current login credential; hash form varies by firmware).

Capabilities RPCs are unreliable across firmware — probe `getConfig`/read-back
rather than assuming a fixed value set.
