# Camera Protocols & Discovery Mechanisms: Deep Technical Investigation Report

## Executive Summary
This report presents a comprehensive, code-verified investigation of all camera protocols (Dahua DVRIP, Hikvision ISAPI, Hikvision HCNetSDK Cgo) and network discovery mechanisms (ONVIF WS-Discovery, Dahua DHDiscover, Hikvision SADP, Nmap TCP scan) within the `ksp-camera-auto` codebase, alongside the system's concurrency and safety architecture.

---

## 1. Observation

### 1.1 Dahua / KBVision DVRIP Protocol (Port 37777 / 8888)
**Files:** `internal/dahua/dhip.go`, `internal/dahua/hash.go`, `internal/dahua/encode.go`, `internal/dahua/maintain.go`, `internal/dahua/user.go`, `internal/dahua/name.go`, `internal/dahua/network.go`, `internal/dahua/snapshot_dvrip.go`, `internal/dahua/davdownload.go`, `internal/dahua/timeconfig.go`.

#### A. Binary Framing & Header Specification (32-Byte Header)
All communication over the DVRIP TCP session uses a fixed 32-byte header followed by variable payload bytes (`dhip.go:22` `const headerLen = 32`).

Header layout and byte offsets:
- `[0:4]` (Big-Endian `uint32`): Frame Type / Magic marker
  - `0xa0010000`: Realm / challenge request frame (`dhip.go:116`)
  - `0xa0050000`: Login request frame (`dhip.go:134`)
  - `0xb0000000` / `0xb0...`: Login response frame (`dhip.go:140-150`)
  - `0xf6000000`: JSON-RPC request / response frame (`dhip.go:228`)
  - `0xf4000000`: Parameter protocol frame (`davdownload.go:235`)
  - `0x11000000` / `0x11...`: Snapshot request frame (`snapshot_dvrip.go:152`)
  - `0xbc...`: Snapshot JPEG data frame (`snapshot_dvrip.go:55`)
  - `0xbb...`: Media data payload frame for `.dav` (DHAV) download (`davdownload.go:293`)
  - `0xa1...`: Passive subchannel keepalive poke frame (`davdownload.go:185`)
- `[4:8]` (Little-Endian `uint32`): Chunk length of THIS frame (`dhip.go:268`).
- `[8:12]` (Little-Endian `uint32`): Request ID in JSON-RPC frames (`dhip.go:230`), or ErrorCode in `\xb0` login response (`dhip.go:145`).
- `[16:20]` (Little-Endian `uint32`): Total payload length across multi-frame JSON (`\xf6`) responses (`dhip.go:285`), or `SessionID` in `\xb0` login response (`dhip.go:149`).
- `[24:28]` (Little-Endian `uint32`): `SessionID` in JSON-RPC (`\xf6`) frames (`dhip.go:232`).
- `[24:32]` (Big-Endian `uint64`): Fixed protocol magic in login frames:
  - `0x050201010000a1aa` for Realm request (`dhip.go:117`)
  - `0x050200080000a1aa` for Login request (`dhip.go:136`)
- `[28:32]`: Fixed byte `0x0a` at offset 28 in snapshot request `0x11` (`snapshot_dvrip.go:154`).

#### B. Multi-Frame Reassembly
In `dhip.go:284-308`, when `header[0] == 0xf6`, `header[16:20]` defines `total` expected bytes. When a response exceeds one frame (e.g. an NVR's full `Encode` table), the client continues reading subsequent 32-byte headers and chunk payloads until `len(payload) >= total`.
*Critical constraint:* `header[16:20]` is ONLY checked for length when `header[0] == 0xf6`. Applying a length sanity check to `\xb0` login frames causes false rejections because `[16:20]` holds `SessionID` (`docs/GOTCHAS.md:9-11`).

#### C. Two-Step MD5 Challenge/Response Algorithm
Implemented in `internal/dahua/hash.go` and `internal/dahua/dhip.go:113-151`:
1. **Step 1 (Realm Request):**
   - Client sends 32-byte packet (`0xa0010000` + 20 zero bytes + `0x050201010000a1aa`).
   - Device replies with `\xb0` frame containing body: `Realm:Login to <REALM>\r\nRandom:<RANDOM>\r\n\r\n`.
   - `parseRealm` extracts `realm` (e.g., `"Login to 18038F6DBFE666A3"`) and `random` (`dhip.go:155-179`).
2. **Step 2 (Hash Computation):**
   - `gen1Hash(password)` (`hash.go:18-34`): Dahua "Sofia" 8-char compressor:
     ```go
     sum := md5.Sum([]byte(password)) // 16 bytes
     out := make([]byte, 8)
     for j := 0; j < 8; j++ {
         v := (int(sum[2*j]) + int(sum[2*j+1])) % 62
         if v < 10 { v += 48 } // '0'-'9'
         else if v < 36 { v += 55 } // 'A'-'Z'
         else { v += 61 } // 'a'-'z'
         out[j] = byte(v)
     }
     ```
   - `gen2Hash(user, pass, realm, random)` (`hash.go:41-44`):
     $$\text{gen2} = \text{UPPER}\Big(\text{MD5}\big(\text{user} + \text{":"} + \text{random} + \text{":"} + \text{UPPER}(\text{MD5}(\text{user} + \text{":"} + \text{realm} + \text{":"} + \text{pass}))\big)\Big)$$
   - `dvripLoginHash`:
     $$\text{loginHash} = \text{user} + \text{"\&\&"} + \text{gen2} + \text{UPPER}\big(\text{MD5}(\text{gen1})\big)$$
3. **Step 3 (Login Transmission & Response):**
   - Client sends header `0xa0050000` + `LE32(len(loginHash))` + 16 zero bytes + `0x050200080000a1aa` followed by `loginHash` payload.
   - Server returns `\xb0` frame: ErrorCode in `header[8:12]` (`0x0008` = Success; `0x0100` = auth fail; `0x0101` = user invalid; `0x0104` = account locked; `0x0111` = uninitialized; `0x0303` = user already logged in). `SessionID` is extracted from `header[16:20]`.

#### D. JSON-RPC `configManager` Structure & Mutation
Over `\xf6` frames, all calls use envelope `{"method":..., "params":..., "id":..., "session":...}`:
- **Array-shaped tables (`encode.go:45-72`):** `configManager.getConfig` / `setConfig` with `name`:
  - `Encode`: Channel-indexed array `table[ch].MainFormat[0]` (main stream), `ExtraFormat[0]` (sub1), `ExtraFormat[1]` (sub2).
    - Resolution: `Video.Width`, `Video.Height`, `Video.CustomResolutionName` (`"1920x1080"`).
    - Codec: `Video.Compression` (`"H.264"`, `"H.264H"`, `"H.264B"`, `"H.265"`, `"MJPG"`), `Video.Profile` (`"Main"`, `"High"`, `"Baseline"`). Unsupported codec is silently ignored by firmware, requiring read-back verification.
    - Framerate: `Video.FPS`.
    - I-frame Interval: `Video.GOP` (in frames).
    - Bitrate: `Video.BitRate` (Kbps), `Video.BitRateControl` (`"VBR"` / `"CBR"`).
    - Audio AAC: `MainFormat[0].Audio.Compression = "AAC"`, `AudioEnable = true`.
  - `SmartEncode`: Per-channel `table[ch].Enable` (bool).
  - `ChannelTitle`: `table[ch].Name` (`name.go:8-34`).
  - `VideoWidget`: `table[ch].CustomTitle[0..3].{Text, EncodeBlend}` for 4-line OSD (`name.go:46-134`). Main (slot 0) and Sub (slot 1) must be written with identical text or firmware silently drops write (`camera.go:701-724`).
  - `VideoColor` & `VideoInOptions`: Picture color & Day/Night exposure parameters (`picture.go:1-188`).
- **Object-shaped tables (`network.go:24-52`):** `getObjectTable` / `setObjectTable`:
  - `Network`: Keyed by interface (`"eth0"`), `DhcpEnable`, `IPAddress`, `SubnetMask`, `DefaultGateway`, `DnsServers` (`network.go:54-159`).
  - `WLan`: Keyed by interface (`"eth2"`), `SSID`, `Keys[0]`, `Encryption` (`network.go:161-213`).
  - `AutoMaintain`: `AutoRebootEnable`, `AutoRebootDay`, `AutoRebootHour`, `AutoRebootMinute` (`maintain.go:21-62`).
- **RPC Methods:**
  - `userManager.modifyPassword`: Hashed `UPPER(MD5(user:realm:newPass))` with fallback to plaintext (`user.go:21-69`).
  - `magicBox.getSerialNo`: Hardware serial number (`identity.go:12-25`).
  - `magicBox.reboot`: Immediate reboot (`maintain.go:68-79`).
  - `netApp.scanWLanDevices`: In-band Wi-Fi AP scan over DVRIP (`network.go:228-267`).

#### E. Keep-Alive, Timeouts, and Reconnection
- **Keep-Alive:** During `.dav` download over passive subchannel, camera buffers and stalls unless poked; `davdownload.go:180-198` sends a 32-byte keepalive frame with `header[0] = 0xa1` every 2 seconds.
- **Timeouts:** Socket-level read/write deadlines `conn.SetReadDeadline(time.Now().Add(timeout))` (`dhip.go:248, 264`).
- **Port Fallback:** `camera.Open` detects `dahua.ErrDialUnreachable` (TCP connection refused/timeout) and automatically retries on port `8888` for KBVision OEMs (`camera.go:340-356`).

---

### 1.2 Hikvision ISAPI Protocol (Port 80 HTTP / HTTPS)
**Files:** `internal/isapi/digest.go`, `internal/isapi/isapi.go`, `internal/isapi/network.go`, `internal/hik/hik.go`.

#### A. HTTP Digest Authentication Implementation (`digest.go`)
Pure Go RFC 2617 implementation supporting MD5 with `qop="auth"`:
- `ParseChallenge` (`digest.go:31-63`): Parses `WWW-Authenticate: Digest realm="...", nonce="...", qop="auth", opaque="..."`.
- Nonce Counter (`nc`): 8-hex-digit counter (`%08x`), incremented per request per host (`digest.go:220-229`).
- Client Nonce (`cnonce`): 16-hex-character crypto random string (`digest.go:118-125`).
- Authorization Header Computation (`digest.go:103-116`):
  $$HA1 = \text{MD5}(\text{username} + \text{":"} + \text{realm} + \text{":"} + \text{password})$$
  $$HA2 = \text{MD5}(\text{method} + \text{":"} + \text{uri})$$
  $$\text{Response} = \text{MD5}(HA1 + \text{":"} + \text{nonce} + \text{":"} + nc + \text{":"} + \text{cnonce} + \text{":"} + \text{"auth"} + \text{":"} + HA2)$$
- `DigestTransport` (`digest.go:127-233`): Caches challenges per host, sends preemptive Authorization headers to prevent redundant 401 roundtrips, and transparently refreshes on nonce expiry.
- Body Rewind: Clones request via `GetBody()` (`digest.go:238-248`) so requests can be retried after a 401 challenge.
- Unbounded Client (`streamClient`, `isapi.go:221-224`): Provides timeout-unbound transport for large video downloads and dense recording searches without truncating streams.

#### B. XML Endpoints & Payloads (`isapi.go`)
- **Compound Channel ID:** `channelID = channelNo * 100 + streamType + 1` (`101` = ch 1 main, `102` = ch 1 sub, `201` = ch 2 main).
- **Core Endpoints:**
  - `GET /ISAPI/Streaming/channels`: Enumerates all channels & streams across an NVR (`isapi.go:975`).
  - `GET /ISAPI/Streaming/channels/{id}`: Detailed channel XML (`isapi.go:241`).
  - `PUT /ISAPI/Streaming/channels/{id}`: State mutation (`isapi.go:256`).
  - `GET /ISAPI/Streaming/channels/{id}/capabilities`: Max framerate / codec capabilities (`isapi.go:584`).
  - `GET /ISAPI/Streaming/channels/{id}/picture`: Raw JPEG snapshot binary (`isapi.go:693`).
  - `GET /ISAPI/ContentMgmt/InputProxy/channels` & `.../{ch}`: Operator channel names on NVR proxying IP cameras (`isapi.go:703, 710, 777`).
  - `GET /ISAPI/System/Video/inputs/channels/{ch}`: Local input channel name fallback (`isapi.go:718`).
  - `GET/PUT /ISAPI/System/Video/inputs/channels/{ch}/overlays`: OSD free-text overlays (`isapi.go:829`).
  - `GET/PUT /ISAPI/System/Network/interfaces` & `.../{ifaceID}/ipAddress`: Static IP/DHCP (`network.go:70, 110`).
  - `PUT /ISAPI/Security/users/{id}`: Password change for user id 1 (`isapi.go:950`).
  - `PUT /ISAPI/System/reboot`: Device restart (`network.go:216`).

#### C. XML Mutation Strategy (GET-Modify-PUT)
Real Hikvision firmware rejects re-marshalled partial Go structs with `statusCode 4 "Invalid XML Content"`. `isapi.go:320-337` performs direct byte-level XML mutation (`replaceXMLTag`, `replaceXMLTagInBlock`, `replaceXMLTagInNthBlock`):
- **Resolution:** `videoResolutionWidth`, `videoResolutionHeight`.
- **Framerate:** `maxFrameRate` (stored as $\text{fps} \times 100$, e.g. `2500` = 25 fps; `0` indicates pass-through).
- **Codec:** `videoCodecType` (`"H.264"`, `"H.265"`, `"MJPEG"`).
- **Smart Codec (H.264+ / H.265+):**
  1. *Inline (Preferred):* Mutates `<Video><SmartCodec><enabled>true</enabled></SmartCodec>` (`isapi.go:636-645`).
  2. *Standalone (Fallback):* `PUT /ISAPI/Streaming/channels/{id}/smartCodec` with `<SmartCodec><enabled>true</enabled></SmartCodec>`.
  - Base codec must be set to `H.265` before enabling Smart Codec (`isapi.go:622-626`).
- **I-frame Interval (GOP):** `<GovLength>` in frames (`isapi.go:444`). If absent, `<keyFrameInterval>` (in ms: $\text{gop} \times 1000 / \text{fps}$ or frames).
- **Bitrate:** `<videoQualityControlType>` (`"VBR"` / `"CBR"`, matching device casing), `<constantBitRate>`, `<vbrUpperCap>`, `<vbrAverageCap>` (written when Smart Codec is enabled).
- **Audio AAC:** `<Audio><enabled>true</enabled><audioCompressionType>AAC</audioCompressionType></Audio>`.

#### D. Response Status Parsing
- `<ResponseStatus><statusCode>1</statusCode><statusString>OK</statusString></ResponseStatus>` (`isapi.go:124-135`).
- Static IP changes: `statusCode 7` or `subStatusCode "rebootRequired"` is handled by automatically triggering a device reboot (`network.go:194-200`).

---

### 1.3 Hikvision HCNetSDK (Port 8000 Cgo / NAT)
**Files:** `internal/hiksdk/sdk.go`, `internal/hiksdk/shim.h`, `internal/hiksdk/shim.cpp`, `internal/hiksdk/stub.go`, `internal/camera/hik_sdk.go`.

#### A. Architecture & Rationale
Port 8000 binary protocol uses encrypted handshakes (libcrypto/ssl bundled in vendor SDK) that cannot be black-box cloned. Pure Go handles LAN port 80; Cgo wrapper `internal/hiksdk` handles port 8000 (`docs/PROTOCOL-HIKVISION.md:62-83`).

#### B. Dynamic Library Loading & Cgo Directives
- Build tag: `//go:build hiksdk`. Non-hiksdk builds compile `stub.go`.
- Cgo compilation flags (`sdk.go:18-22`):
  ```go
  /*
  #cgo LDFLAGS: -lstdc++
  #include "shim.h"
  #include <stdlib.h>
  */
  import "C"
  ```
- Make build flags:
  `CGO_CPPFLAGS="-I<sdk>/incEn" CGO_LDFLAGS="-L<sdk>/lib -lhcnetsdk -Wl,-rpath,<sdk>/lib"`
- Runtime initialization (`sdk.go:39-48`): Reads `KSPCAM_HIKSDK_PATH` from environment and calls `hik_init(clib)` protected by `sync.Once`.

#### C. C++ Shim Implementation (`shim.cpp`)
- `hik_init(const char *libdir)`: Calls `NET_DVR_Init()`, `NET_DVR_SetConnectTime(5000, 1)`, `NET_DVR_SetReconnect(10000, 1)`, and configures plugin path `NET_SDK_INIT_CFG_SDK_PATH` via `NET_DVR_SetSDKInitCfg`.
- `hik_login(ip, port, user, pass)`: Executes `NET_DVR_Login_V30` (compatible with V40 devices), returning `LONG uid` (`shim.cpp:22-28`).
- `hik_stdxml(uid, url, body, blen, out, outcap, outlen, status, statuscap)` (`shim.cpp:30-57`):
  - Populates `NET_DVR_XML_CONFIG_INPUT` with `lpRequestUrl = "GET /ISAPI/Streaming/channels/101"`, `dwRecvTimeOut = 5000`, `lpInBuffer = body`.
  - Populates `NET_DVR_XML_CONFIG_OUTPUT` with `lpOutBuffer = out` (1 MB) and `lpStatusBuffer = status` (8 KB).
  - Calls `NET_DVR_STDXMLConfig(uid, &in, &o)`.
- `hik_logout(uid)`: `NET_DVR_Logout(uid)`.
- `hik_cleanup()`: `NET_DVR_Cleanup()`.

#### D. Pluggable Transport Seam & Thread Safety
`internal/isapi.Transport` interface (`isapi.go:29-31`):
```go
type Transport interface {
    Do(ctx context.Context, method, path string, body []byte) ([]byte, error)
}
```
- `Session` struct (`sdk.go:52-55`) holds `sync.Mutex` and `uid`.
- `transport.Do` (`sdk.go:97-112`) delegates to `s.stdxml`:
  - `GET`: Returns `out` buffer (resource XML).
  - `PUT` / `POST` / `DELETE`: Returns `status` buffer (`ResponseStatus` XML).
- 100% of ISAPI get/set/probe/mutate logic in `internal/isapi` is reused verbatim over port 8000.

---

### 1.4 Discovery Mechanisms (`internal/discovery/`)
**Files:** `internal/discovery/discovery.go`, `internal/discovery/onvif.go`, `internal/discovery/dahua.go`, `internal/discovery/sadp.go`, `internal/discovery/nmap.go`.

| Protocol | Transport | Target Address | Port | Scope | Extracted Metadata |
|---|---|---|---|---|---|
| **ONVIF WS-Discovery** | UDP Multicast | `239.255.255.250` | `3702` | L2 Broadcast / VLAN | IP (`XAddrs`), Port (`37777` for Dahua), Vendor, Model, Name |
| **Dahua DHDiscover** | UDP Broadcast & Multicast | `255.255.255.255` & `239.255.255.251` | `37810` | L2 Broadcast / VLAN | Source IP, Vendor (`"dahua"`), Model/DeviceType, MAC |
| **Hikvision SADP** | UDP Multicast | `239.255.255.250` | `37020` | L2 Broadcast / VLAN | IPv4Address, Vendor (`"hikvision"`), DeviceType/Model, MAC, SoftwareVersion |
| **Nmap Subnet Scan** | TCP Connect (`-sT`) | Target Subnet/CIDR | `80,8000,37777,37778,8888` | L3 Routed Subnets | Host IP, Open Port, Inferred Vendor |

#### Implementation Details
1. **ONVIF WS-Discovery (`onvif.go`):**
   - Sends SOAP Probe envelope to `239.255.255.250:3702` with `d:Types` set to `dn:NetworkVideoTransmitter` and random UUID `MessageID`.
   - Streaming XML token decoder (`parseONVIFProbeMatch`) extracts `XAddrs` and `Scopes`.
   - Heuristics: `vendorFromText` identifies `hikvision`, `dahua`, `kbvision`, `lechange`, `tiandy`, `lc`, and Dahua IPC model families (`ipc-h*`, `ipc-c*`, etc.).
2. **Dahua DHDiscover (`dahua.go`):**
   - DHIP header (32 bytes: `0x20000000`, `"DHIP"`, length at `[16:20]` and `[24:28]`) + JSON `{"method":"DHDiscover.search","params":{"mac":"","uni":1}}`.
   - Sets `SO_BROADCAST` socket option (`setBroadcast` via syscall) to send to `255.255.255.255:37810` and multicasts to `239.255.255.251:37810`.
   - Extracts MAC address via regex `^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$` and model from JSON keys (`devicetype`, `type`, `detail`).
3. **Hikvision SADP (`sadp.go`):**
   - Sends XML probe `<?xml version="1.0" encoding="utf-8"?><Probe><Uuid>UUID</Uuid><Types>inquiry</Types></Probe>` to `239.255.255.250:37020`.
   - Decodes `<ProbeMatch>`: `DeviceType`, `DeviceDescription`, `MAC`, `IPv4Address`, `SoftwareVersion`.
4. **Nmap Subnet Scan (`nmap.go`):**
   - Safety validation: `safeScanTargetRe` strictly enforces valid IPv4/CIDR/ranges to prevent command/argument injection.
   - Command: `nmap -Pn -sT -p 80,8000,37777,37778,8888 --open -- <cidr>`.
   - Port mapping: 8000 $\to$ `"hikvision"`, 37777/37778/8888 $\to$ `"dahua"`, 80 $\to$ ambiguous (`""`).
5. **Scan Coordinator (`discovery.go`):**
   - Executes `scanONVIF`, `scanDahua`, `scanSADP` concurrently via `sync.WaitGroup`.
   - Merges and dedupes results by IP using `score()` (preference for higher metadata completeness), sorted numerically by IP.

---

### 1.5 Concurrency & Safety Rules
**Files:** `internal/bulk/bulk.go`, `internal/camera/camera.go`, `internal/server/snapshot_cache.go`.

#### A. Per-Camera Sequential Execution
- In `internal/bulk/bulk.go:49-86`, `Apply` iterates through `req.DeviceIDs` strictly one device at a time:
  - Changing video encoding parameters (resolution, codec, GOP, bitrate) restarts the camera's hardware DSP encoder and drops the live RTSP stream.
  - Embedded camera web servers/CPUs have minimal resources; concurrent requests can cause socket exhaustion, watchdog reboots, or dropped frames on continuous recordings.
  - `dahua.Client` is explicitly single-threaded (`dhip.go:32-33`).
  - Sequential execution ensures failures are isolated and live progress (`Event` SSE stream) provides an accurate per-camera audit trail.

#### B. Read-Back Verification Protocol (Probe $\to$ Apply $\to$ Read-Back)
For every configuration step, `camera.Apply` executes a tri-state verification:
1. **Exact Match:** Device returns requested value $\to$ `OK: true`.
2. **Clamped Match:** Device accepted change but clamped to supported hardware step (e.g. bitrate 4096 clamped to 4000) $\to$ `OK: true`, logs actual clamped value.
3. **Unchanged Failure:** Device returned success RPC but ignored unsupported setting $\to$ `OK: false`, fails loudly with diagnostic message.

#### C. Global Concurrency Limits (Semaphores)
- `playbackSem = make(chan struct{}, 4)` (`dahua/playback.go:16`, `hik/playback.go:29`, `tiandy/playback.go:17`): Caps concurrent RTSP playback streams to 4.
- `liveSem = make(chan struct{}, 3)` (`dahua/snapshot_dvrip.go:75`): Caps concurrent DVRIP MJPEG live streams to 3.
- `ffmpegSem = make(chan struct{}, cap)` (`dahua/snapshot.go:26`): Caps concurrent ffmpeg snapshot processes.
- `buildSem = make(chan struct{}, 1)` (`mediaexport/fastmp4.go:96`): Serializes fast MP4 packaging jobs.
- `snapshot_cache` (`server/snapshot_cache.go:25`): Per-camera singleflight mutex serializing simultaneous snapshot requests for the same camera.

---

## 2. Logic Chain

1. **Protocol Separation by Vendor & Port:**
   - Observations of `internal/dahua/dhip.go` and `docs/PROTOCOL-DAHUA.md` establish that Dahua port 37777/8888 uses DVRIP framing (32-byte header with `\xa0`/`\xb0`/`\xf6` opcodes), not DHIP HTTP.
   - Observations of `internal/isapi/` establish that Hikvision port 80 uses HTTP Digest authentication over XML REST endpoints.
   - Observations of `internal/hiksdk/` establish that Hikvision port 8000 requires HCNetSDK Cgo bindings (`NET_DVR_STDXMLConfig`) due to proprietary encrypted handshakes.
2. **Authentication Flow Integrity:**
   - In Dahua, Sofia 8-character password compression followed by double-MD5 challenge (`user:random:UPPER(MD5(user:realm:pass))`) generates the exact wire credential required by DVRIP.
   - In Hikvision ISAPI, RFC 2617 Digest calculation with nonce counting (`nc`) and client nonces (`cnonce`) satisfies Hikvision security constraints.
3. **Transport Seam Reusability:**
   - Abstracting `isapi.Transport` allows the same XML manipulation engine (`isapi.Client`) to serve Hikvision ISAPI (port 80 HTTP), Hikvision HCNetSDK (port 8000 Cgo), and Tiandy (port 8081 CGI session) with zero code duplication.
4. **Safety & Concurrency Invariance:**
   - Read-back verification is required because Dahua and Hikvision firmware silently accept unsupported codecs without applying them.
   - Per-camera serialization in `bulk.Apply` and global semaphores protect embedded camera hardware and low-power host gateways from resource starvation.

---

## 3. Caveats

1. **Multi-Frame Response Guard:** `header[16:20]` in DVRIP is only a length field for `0xf6` frames. Checking it on `0xb0` login frames will falsely reject valid sessions.
2. **Smart Codec Bitrate Precedence:** When Smart Codec (H.264+/H.265+) is enabled on Hikvision, the device treats configured bitrate as average (`<vbrAverageCap>`), not peak (`<vbrUpperCap>`). Smart Codec must be applied before Bitrate in the execution sequence.
3. **Hikvision XML Schema Sensitivity:** Re-marshalling partial structs fails with `statusCode 4 "Invalid XML Content"`. Raw XML mutation must be used.
4. **Dahua OSD Line Mirroring:** On Dahua firmware, CustomTitle slots 0 (Main) and 1 (Sub) must be written with identical text or the device silently drops the update.
5. **Discovery Boundary:** UDP multicast/broadcast (ONVIF, Dahua, SADP) cannot cross L3 subnets or NAT routers; routed subnets require Nmap TCP scanning.

---

## 4. Conclusion

The `ksp-camera-auto` codebase implements a robust, modular, and resilient camera management engine:
- **Dahua DVRIP:** Fully implemented in pure Go with binary framing, Sofia MD5 authentication, multi-frame reassembly, and JSON-RPC configManager.
- **Hikvision ISAPI & HCNetSDK:** Pluggable transport architecture sharing a pure-Go raw-XML mutation engine across both HTTP Digest (port 80) and HCNetSDK Cgo (port 8000).
- **Network Discovery:** 4-tier discovery subsystem combining concurrent L2 UDP probes (ONVIF, DHDiscover, SADP) with L3 Nmap TCP scanning.
- **Concurrency & Safety:** Strict per-camera sequential execution with tri-state read-back verification and global resource semaphores.

---

## 5. Verification Method

### 5.1 Automated Unit Tests
Run the project unit test suite:
```bash
go test -v ./internal/dahua/... ./internal/isapi/... ./internal/discovery/... ./internal/camera/... ./internal/bulk/...
```

### 5.2 Specific Protocol Test Cases
1. **Dahua DVRIP Multi-Frame & MD5 Tests:**
   - `internal/dahua/hash_test.go` / `identity_test.go` / `multiframe_test.go`
   - `go test -v ./internal/dahua/ -run "TestLogin|TestMultiFrame|TestEncode"`
2. **Hikvision Digest & Raw XML Mutation Tests:**
   - `internal/isapi/digest_test.go` / `isapi_test.go`
   - `go test -v ./internal/isapi/ -run "TestDigest|TestMutate|TestStreamingChannel"`
3. **Discovery UDP & Nmap Tests:**
   - `internal/discovery/discovery_test.go`
   - `go test -v ./internal/discovery/ -run "TestParse|TestMerge|TestSafeScan"`

### 5.3 Cgo Build Verification
Verify HCNetSDK compilation (requires SDK headers/libs in `HIKSDK` path):
```bash
CGO_ENABLED=1 go test -tags hiksdk -v ./internal/hiksdk/...
```
