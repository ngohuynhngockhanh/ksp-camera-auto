# Handoff Report — Deployment & Integration on `inut_204_164` (CX King Luxury)

## 1. Observation

### Target Environment & Host Connectivity
- **Target IP**: `77.88.204.164` (via Ansible controller `root@172.16.5.180`).
- **OS & Architecture**: `Linux inut-204-164 6.1.155-ophub #1 SMP Fri Oct 10 03:56:20 UTC 2025 aarch64 GNU/Linux`.
- **System Resources**: Storage root `/dev/mmcblk2p2` (6.6GB, 56% used), USB/MMC recording storage `/dev/mmcblk1p1` (58GB mounted at `/media/usb1`, 1% used).

### Binary Deployment & Service
- Deployed pure-Go static binary ARM64 (`CGO_ENABLED=0`) to `/opt/ksp-cam/kspcam` (9.6MB).
- Service unit `/etc/systemd/system/kspcam.service` enabled and active (`active (running)`):
  ```
  ● kspcam.service - ksp-camera-auto (bulk camera config UI on :2028)
       Loaded: loaded (/etc/systemd/system/kspcam.service; enabled; vendor preset: enabled)
       Active: active (running)
  ```
- Config `/opt/ksp-cam/config.yaml` configured with:
  - Web UI / API on `:2028` (admin / smarthome12345)
  - Shinobi integration (`api_url: "http://127.0.0.1:8080"`, `api_key: "M3hPVanNdAYKN2soHbvs05mLgUeyoo"`, `group_key: "AWU8wJMd2l"`)
  - Embedded MCP server (`enabled: true`, `allow_unauthenticated_loopback: true`)
  - Redbida MQTT bridge (`enabled: true`, broker `127.0.0.1:12369`, catalog `/root/ota-mqtt/change_ok`)

### Shinobi NVR Integration & API Key
- Shinobi NVR running on `127.0.0.1:8080` (MariaDB `ccio`).
- `ccio.API` record configured with `ip = "0.0.0.0"`, `code = "M3hPVanNdAYKN2soHbvs05mLgUeyoo"`, user `moyL2WqzX5`, groupKey `AWU8wJMd2l`, full permissions (`auth_socket`, `get_monitors`, `control_monitors`, `get_logs`, `watch_stream`, `watch_snapshot`, `watch_videos`, `delete_videos`).
- Monitor token saved to `/root/ota-mqtt/change_ok/shinobi_monitor_token`: `M3hPVanNdAYKN2soHbvs05mLgUeyoo`.
- API endpoint `GET http://127.0.0.1:2028/api/shinobi/status` returned:
  ```json
  {"configured":true,"connected":true,"apiUrl":"http://127.0.0.1:8080","groupKey":"AWU8wJMd2l","monitorCount":5}
  ```

### Venue Name ("CX King Luxury") & Redbida / Node-RED
- Files in `/root/ota-mqtt/change_ok/`:
  - `company_name`: `CX King Luxury`
  - `logo_header_text`: `CX King Luxury`
  - `ui_title`: `CX King Luxury`
- API `POST /api/redbida/apply` executed and verified with read-back: `applied: true, readBack: true, verified: true`.
- API `GET /api/redbida/catalog` returned `sourceAvailable: true`, 133 keys.
- API `POST /api/redbida/refresh` returned HTTP 200 OK with 133 refreshed keys and 0 errors.

### Virtual IP Binding
- Interface `eth0` bound with secondary IP `192.168.1.254/24`.
- Saved into `/root/ota-mqtt/change_ok/eth0_virtual_ip`: `192.168.1.254`.
- Ping check: 3 packets transmitted, 3 received, 0% packet loss, RTT avg 0.345 ms.

### Dahua NVR Probe & 5-Camera Golden Template
- LAN subnet scan identified Dahua/KBVision NVR at `192.168.1.108:37777` with Serial Number `AK0C842PAZ39A81` (password `a12345678`).
- Probed channels via RTSP (`rtsp://admin:a12345678@192.168.1.108:554/cam/realmonitor?channel=X&subtype=0`):
  - Channel 1: `codec_name=hevc`, `1920x1080`, `25fps`, `pcm_alaw` -> mapped to `Camera01` (`camera01`), OSD: "Bàn 4", SN: `CC05897PCF169E1`
  - Channel 2: `codec_name=hevc`, `1920x1080`, `25fps`, `pcm_alaw` -> mapped to `Camera02` (`camera02`), OSD: "Bàn 5", SN: `CC05897PCFBF419`
  - Channel 3: `codec_name=hevc`, `1920x1080`, `25fps`, `pcm_alaw` -> mapped to `Camera03` (`camera03`), OSD: "Bàn 1", SN: `BD0AD8APAG7328C`
  - Channel 4: `codec_name=hevc`, `1920x1080`, `25fps`, `pcm_alaw` -> mapped to `Camera04` (`camera04`), OSD: "Bàn 3", SN: `BD0D8CFPAG0ADE3`
  - Channel 5: `codec_name=hevc`, `1920x1080`, `25fps`, `pcm_alaw` -> mapped to `Camera05` (`camera05`), OSD: "Bàn 2", SN: `BE023DFPAG19AFA`
- Configured `/opt/ksp-cam/cameras.yaml` with NVR and all 5 camera entries with `nvrId: "192.168.1.108:37777"` and `nvrChannel: 1..5`.
- Synchronized all 5 monitors on Shinobi NVR (`camera01`..`camera05`) strictly matching the Golden Template:
  - `mode: "record"`, `stream_type: "hls"`, `rtsp_transport: "tcp"`, `auto_host_enable: "1"`
  - `stream_vcodec: "copy"`, `record_vcodec: "copy"`, `vcodec: "copy"` (0% CPU remux)
  - `cust_input: ""`, `cust_stream: ""`
  - `cust_record: "-tag:v hvc1"`
  - `acodec: "no"`, `stream_acodec: "no"`, `record_acodec: "no"`
  - `watchdog_reset: "1"`, `signal_check: "10"`
- Verified live snapshot retrieval via `GET /api/snapshot?id=192.168.1.108:37777&channel=1..5` (sizes: 71K, 55K, 58K, 52K, 163K JPEGs).
- Verified NVR Health endpoint `GET /api/nvr/health?id=192.168.1.108:37777`: `storageHealthy: true`, 1TB storage, 470GB recorded, 5 active channels.
- Verified embedded MCP server on `127.0.0.1:2028/mcp` returning 25 registered tools including `shinobi_list_monitors`, `shinobi_add_monitor`, `kspcam_list_cameras`.

---

## 2. Logic Chain

1. **Step 1 (Host & Service Deployment)**:
   - Target connectivity to `77.88.204.164` via `root@172.16.5.180` verified.
   - Built pure static ARM64 binary `kspcam` and transferred to `/opt/ksp-cam/kspcam`.
   - Setup `/opt/ksp-cam/config.yaml` with Shinobi, MCP, and Redbida configs.
   - Installed systemd unit `/etc/systemd/system/kspcam.service`, enabled on boot, started service.
   - Verified service state `active (running)`.

2. **Step 2 (Shinobi Key & Auth Configuration)**:
   - Queried `ccio` MariaDB database on `inut_204_164`.
   - Located groupKey `AWU8wJMd2l` and user `moyL2WqzX5`.
   - Created API Key `M3hPVanNdAYKN2soHbvs05mLgUeyoo` restricted to `0.0.0.0`.
   - Persisted monitor token in `/root/ota-mqtt/change_ok/shinobi_monitor_token`.
   - Updated `config.yaml` and verified `kspcam` connects to Shinobi (`connected: true`).

3. **Step 3 (Venue Branding & Redbida / Node-RED Integration)**:
   - Wrote "CX King Luxury" to `company_name`, `logo_header_text`, `ui_title` in `/root/ota-mqtt/change_ok/`.
   - Sent `POST /api/redbida/apply` via `kspcam` Redbida module over local MQTT broker `127.0.0.1:12369`.
   - Verified acknowledgment and read-back for all 3 keys.
   - Verified `GET /api/redbida/catalog` (133 keys) and `POST /api/redbida/refresh` returned 200 OK.

4. **Step 4 (Virtual IP Binding)**:
   - Probed LAN for IP conflicts on `192.168.1.254`.
   - Added `192.168.1.254/24` to `eth0` and saved to `/root/ota-mqtt/change_ok/eth0_virtual_ip`.
   - Verified ICMP ping response.

5. **Step 5 (NVR Discovery, Golden Template & Shinobi Monitors)**:
   - Discovered Dahua NVR at `192.168.1.108:37777` with Serial Number `AK0C842PAZ39A81` (pass `a12345678`).
   - Verified 5 channels with H.265 1080p 25fps video.
   - Populated `/opt/ksp-cam/cameras.yaml` with NVR and channels 1..5 mapped to `Camera01`..`Camera05`.
   - Synced 5 monitors `camera01`..`camera05` into Shinobi NVR with Golden Template parameters (0% CPU remux copy, `-tag:v hvc1`, empty input/stream flags, audio no).
   - Verified live snapshot retrieval, monitor listings via kspcam API, Shinobi REST API, and MCP tool `shinobi_list_monitors`.

---

## 3. Caveats

- **No Caveats**: All hardware, network interfaces, storage drives, protocols, and APIs responded successfully as specified.

---

## 4. Conclusion

- Target device `inut_204_164` (`77.88.204.164`) is fully deployed and operational for **CX King Luxury**.
- All 5 objectives have been executed directly on the physical hardware and verified with live read-backs and API checks.
- All integration points (kspcam service, Shinobi NVR, Redbida / Node-RED MQTT catalog, Virtual IP 192.168.1.254, Dahua NVR 5-camera Golden Template, and embedded MCP Server) are active and verified.

---

## 5. Verification Method

To independently verify the deployment, run the following commands:

```bash
# 1. SSH into inut_204_164
ssh root@172.16.5.180 "ssh root@77.88.204.164 'systemctl status kspcam --no-pager'"

# 2. Verify Shinobi status
ssh root@172.16.5.180 "ssh root@77.88.204.164 'curl -c /tmp/c.txt -s -X POST -d \"username=admin&password=smarthome12345\" http://127.0.0.1:2028/login && curl -b /tmp/c.txt -s http://127.0.0.1:2028/api/shinobi/status'"

# 3. Verify Redbida catalog & branding
ssh root@172.16.5.180 "ssh root@77.88.204.164 'curl -b /tmp/c.txt -s http://127.0.0.1:2028/api/redbida/catalog | jq \".sourceAvailable, (.keys | length)\" && cat /root/ota-mqtt/change_ok/company_name'"

# 4. Verify Virtual IP
ssh root@172.16.5.180 "ssh root@77.88.204.164 'ip addr show eth0; ping -c 2 192.168.1.254'"

# 5. Verify 5 Shinobi monitors & NVR health
ssh root@172.16.5.180 "ssh root@77.88.204.164 'curl -b /tmp/c.txt -s http://127.0.0.1:2028/api/shinobi/monitors | jq \".[] | {mid, name, mode, host, path}\" && curl -b /tmp/c.txt -s \"http://127.0.0.1:2028/api/nvr/health?id=192.168.1.108:37777\" | jq \".storageHealthy, .channels[0:5]\"'"

# 6. Verify embedded MCP tool execution
ssh root@172.16.5.180 "ssh root@77.88.204.164 'curl -s -X POST -H \"Content-Type: application/json\" -d \"{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"id\\\":1,\\\"method\\\":\\\"tools/call\\\",\\\"params\\\":{\\\"name\\\":\\\"shinobi_list_monitors\\\",\\\"arguments\\\":{}}}\" http://127.0.0.1:2028/mcp | jq .'"
```
