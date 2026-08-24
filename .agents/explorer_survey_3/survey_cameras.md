# Field Camera Network & Shinobi Golden Template Survey Report

> **Target System**: `inut_204_163` (`77.88.204.163` / local LAN IP: `192.168.1.21/24`)  
> **Investigation Date**: 2026-08-24T16:20:00+07:00  
> **Survey Specialist**: `teamwork_preview_explorer` (Survey Specialist 3)  
> **Authoritative Standards**: `/home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md`  

---

## 1. Executive Summary

This report documents the live network reachability, device identification, credential & lockout status, audio capabilities, and Shinobi NVR monitor configuration state for the 8-camera deployment on target system `inut_204_163`.

### Key Findings
1. **Network Topology & Camera Reachability**:
   - The requested IP range `192.168.1.190` to `192.168.1.197` is currently **unreachable / inactive** (0 hosts responding to ARP, ICMP, TCP, or UDP probes).
   - The physical camera fleet was successfully discovered live on the local subnet at **`192.168.1.111` through `192.168.1.118`** with ports `80` (HTTP), `554` (RTSP), and `37777` (DVRIP TCP) open and active on all 8 units.
   - An 8-channel Dahua NVR (`DHI-NVR1108HS-S3/H`) is active at `192.168.1.150`, and an 8-channel Dahua XVR (`DH-XVR5108HS-I3`) is active at `192.168.1.3`.
2. **Device Hardware & Audio Capability**:
   - All 8 cameras are genuine **Dahua DH-IPC-HDW1230T2-A** (2MP Eyeball Dome IP Cameras) running firmware `2.860.100Z000.0.R`.
   - The `-A` model suffix denotes hardware with a **Built-in Microphone (Audio Support)**. The cameras support both AAC and G.711A audio encoding.
3. **Authentication & Lockout State**:
   - The standard credential configured across the project is `admin:Admin123456`.
   - Probing revealed Dahua error code `0x0104` (`account locked`), indicating that recent repeated login attempts have triggered Dahua's temporary anti-brute-force account lockout mechanism.
4. **Shinobi NVR State & Golden Template Gap**:
   - Shinobi NVR is running on `127.0.0.1:8080` (PM2 process `camera`, PID 13092) with MariaDB database `ccio` and admin user `ngohuynhngockhanh@gmail.com` (Group Key `ke: P6zP1kVhht`).
   - The Shinobi `Monitors` table currently has **0 monitors configured**.
   - All 8 monitors need to be provisioned following the **Golden Template** (0% CPU remux copy, HLS streaming, `-tag:v hvc1`, audio AAC copy/record, empty cust flags).

---

## 2. Camera Network Reachability & Inventory

### 2.1 Live Network Discovery Table (Physical Devices)

| IP Address | Device Class | Model / Vendor | MAC Address | Serial Number (SN) | Open Ports | Audio Hardware |
|---|---|---|---|---|---|---|
| `192.168.1.111` | IPC Camera | Dahua `DH-IPC-HDW1230T2-A` | `e0:2e:fe:14:12:d6` | `BD0AD8APAGAEA84` | `80`, `554`, `37777` | Built-in Mic (`-A`) |
| `192.168.1.112` | IPC Camera | Dahua `DH-IPC-HDW1230T2-A` | `e0:2e:fe:14:13:62` | `BD0AD8APAGDA525` | `80`, `554`, `37777` | Built-in Mic (`-A`) |
| `192.168.1.113` | IPC Camera | Dahua `DH-IPC-HDW1230T2-A` | `e0:2e:fe:14:13:76` | `BD0AD8APAG93A4D` | `80`, `554`, `37777` | Built-in Mic (`-A`) |
| `192.168.1.114` | IPC Camera | Dahua `DH-IPC-HDW1230T2-A` | `e0:2e:fe:14:13:5b` | `BD0AD8APAG7A521` | `80`, `554`, `37777` | Built-in Mic (`-A`) |
| `192.168.1.115` | IPC Camera | Dahua `DH-IPC-HDW1230T2-A` | `e0:2e:fe:14:13:8b` | `BD0AD8APAGBFC69` | `80`, `554`, `37777` | Built-in Mic (`-A`) |
| `192.168.1.116` | IPC Camera | Dahua `DH-IPC-HDW1230T2-A` | `e0:2e:fe:14:13:7e` | `BD0AD8APAG649F4` | `80`, `554`, `37777` | Built-in Mic (`-A`) |
| `192.168.1.117` | IPC Camera | Dahua `DH-IPC-HDW1230T2-A` | `e0:2e:fe:24:6c:ae` | `BD0D8CFPAGF6DFD` | `80`, `554`, `37777` | Built-in Mic (`-A`) |
| `192.168.1.118` | IPC Camera | Dahua `DH-IPC-HDW1230T2-A` | `e0:2e:fe:24:6b:bc` | `BD0D8CFPAGC40AB` | `80`, `554`, `37777` | Built-in Mic (`-A`) |
| `192.168.1.150` | NVR (8-Ch) | Dahua `DHI-NVR1108HS-S3/H` | `f8:ce:07:f0:1f:b2` | `AK0E6EEPAZ67F3B` | `80`, `554`, `37777` | 8-Ch NVR Host |
| `192.168.1.3` | XVR (8-Ch) | Dahua `DH-XVR5108HS-I3` | `f4:b1:c2:53:72:7d` | `9D094B8PAZ60D23` | `80`, `554`, `37777` | 8-Ch XVR Host |

### 2.2 Range 192.168.1.190 – 192.168.1.197 Check

| Target IP | Ping ICMP | TCP 80 (HTTP) | TCP 554 (RTSP) | TCP 37777 (DVRIP) | UDP 37810 (DHDiscover) | Status |
|---|---|---|---|---|---|---|
| `192.168.1.190` | 100% Loss | Closed / Filtered | Closed / Filtered | Closed / Filtered | No Reply | Unreachable |
| `192.168.1.191` | 100% Loss | Closed / Filtered | Closed / Filtered | Closed / Filtered | No Reply | Unreachable |
| `192.168.1.192` | 100% Loss | Closed / Filtered | Closed / Filtered | Closed / Filtered | No Reply | Unreachable |
| `192.168.1.193` | 100% Loss | Closed / Filtered | Closed / Filtered | Closed / Filtered | No Reply | Unreachable |
| `192.168.1.194` | 100% Loss | Closed / Filtered | Closed / Filtered | Closed / Filtered | No Reply | Unreachable |
| `192.168.1.195` | 100% Loss | Closed / Filtered | Closed / Filtered | Closed / Filtered | No Reply | Unreachable |
| `192.168.1.196` | 100% Loss | Closed / Filtered | Closed / Filtered | Closed / Filtered | No Reply | Unreachable |
| `192.168.1.197` | 100% Loss | Closed / Filtered | Closed / Filtered | Closed / Filtered | No Reply | Unreachable |

*Note: The camera inventory in SKILL.md documents static target IPs `192.168.1.190-197` (or previous deployment mapping). On the current live network, the 8 cameras are active on `192.168.1.111-118`.*

---

## 3. Shinobi NVR & Golden Template Compliance Analysis

### 3.1 Current Shinobi NVR State
- **URL**: `http://127.0.0.1:8080`
- **Database**: MariaDB `ccio`
- **Admin User**: `ngohuynhngockhanh@gmail.com`
- **Group Key (`ke`)**: `P6zP1kVhht`
- **Configured Monitors**: **0** (Table `Monitors` is empty).

### 3.2 Golden Template Specification vs Actual Gap

| Parameter | Golden Template Requirement | Purpose / Rationale | Current Actual State |
|---|---|---|---|
| **Monitor ID (`mid`)** | `camera01` .. `camera08` | Lowercase standard identifier | None (0 monitors) |
| **Monitor Name** | `Camera01` .. `Camera08` | TitleCase standard display name | None |
| **`mode`** | `"record"` | Continuous recording mode | None |
| **`stream_type`** | `"hls"` | HLS Web/Mobile low-latency stream | None |
| **`vcodec`** | `"copy"` | 0% CPU transcoding remux | None |
| **`stream_vcodec`** | `"copy"` | Stream passthrough | None |
| **`record_vcodec`** | `"copy"` | Direct video stream disk copy | None |
| **`rtsp_transport`** | `"tcp"` | Packet loss prevention over TCP | None |
| **`preset_stream`** | `"ultrafast"` | Minimal HLS segmentation delay | None |
| **`hls_time`** | `"2"` | 2-second HLS fragment length | None |
| **`hls_list_size`** | `"2"` | 2-segment playlist buffer | None |
| **`acodec`** | `"copy"` (AAC) / `"no"` (No Audio) | Audio stream passthrough | None |
| **`stream_acodec`** | `"copy"` (AAC) / `"no"` (No Audio) | Web audio playback | None |
| **`record_acodec`** | `"aac"` (AAC) / `"no"` (No Audio) | MP4 audio standard container | None |
| **`cust_input`** | `""` (**Must be empty**) | Prevent FFmpeg input flag conflicts | None |
| **`cust_stream`** | `""` (**Must be empty**) | Prevent HLS stream timestamp drift | None |
| **`cust_record`** | `"-tag:v hvc1"` (**Mandatory**) | Fix H.265 MP4 container tag for Apple/Web | None |
| **`watchdog_reset`** | `"1"` | Self-healing stream reconnect | None |
| **`signal_check`** | `"10"` | 10s stream health watchdog interval | None |
| **`details.notes`** | `{"sn": "...", "safety_code": "..."}` | Camera hardware serial & safety code | None |

---

## 4. Proposed 8-Monitor Provisioning Matrix

Mapping the 8 live discovered cameras to the Shinobi Golden Template standard:

| STT | Monitor ID (`mid`) | Name | Live IP | Protocol URL (`auto_host`) | Serial Number | Model | Audio Codec Config |
|---|---|---|---|---|---|---|---|
| 1 | `camera01` | `Camera01` | `192.168.1.111` | `rtsp://admin:Admin123456@192.168.1.111:554/cam/realmonitor?channel=1&subtype=0` | `BD0AD8APAGAEA84` | `DH-IPC-HDW1230T2-A` | `acodec: copy`, `record_acodec: aac` |
| 2 | `camera02` | `Camera02` | `192.168.1.112` | `rtsp://admin:Admin123456@192.168.1.112:554/cam/realmonitor?channel=1&subtype=0` | `BD0AD8APAGDA525` | `DH-IPC-HDW1230T2-A` | `acodec: copy`, `record_acodec: aac` |
| 3 | `camera03` | `Camera03` | `192.168.1.113` | `rtsp://admin:Admin123456@192.168.1.113:554/cam/realmonitor?channel=1&subtype=0` | `BD0AD8APAG93A4D` | `DH-IPC-HDW1230T2-A` | `acodec: copy`, `record_acodec: aac` |
| 4 | `camera04` | `Camera04` | `192.168.1.114` | `rtsp://admin:Admin123456@192.168.1.114:554/cam/realmonitor?channel=1&subtype=0` | `BD0AD8APAG7A521` | `DH-IPC-HDW1230T2-A` | `acodec: copy`, `record_acodec: aac` |
| 5 | `camera05` | `Camera05` | `192.168.1.115` | `rtsp://admin:Admin123456@192.168.1.115:554/cam/realmonitor?channel=1&subtype=0` | `BD0AD8APAGBFC69` | `DH-IPC-HDW1230T2-A` | `acodec: copy`, `record_acodec: aac` |
| 6 | `camera06` | `Camera06` | `192.168.1.116` | `rtsp://admin:Admin123456@192.168.1.116:554/cam/realmonitor?channel=1&subtype=0` | `BD0AD8APAG649F4` | `DH-IPC-HDW1230T2-A` | `acodec: copy`, `record_acodec: aac` |
| 7 | `camera07` | `Camera07` | `192.168.1.117` | `rtsp://admin:Admin123456@192.168.1.117:554/cam/realmonitor?channel=1&subtype=0` | `BD0D8CFPAGF6DFD` | `DH-IPC-HDW1230T2-A` | `acodec: copy`, `record_acodec: aac` |
| 8 | `camera08` | `Camera08` | `192.168.1.118` | `rtsp://admin:Admin123456@192.168.1.118:554/cam/realmonitor?channel=1&subtype=0` | `BD0D8CFPAGC40AB` | `DH-IPC-HDW1230T2-A` | `acodec: copy`, `record_acodec: aac` |

---

## 5. Implementation Recommendations for Worker Agents

1. **Camera Network Configuration**:
   - If static IP binding `192.168.1.190-197` is required by customer network plan, use `kspcam` network API / Dahua `configManager.setConfig` to assign static IPs from current `192.168.1.111-118` to `192.168.1.190-197`.
2. **Account Lockout Recovery**:
   - Allow the camera security lockout window (5-15 minutes) to expire, or reboot cameras if needed before executing bulk configuration changes.
3. **Shinobi Provisioning**:
   - Provision Shinobi monitors using `internal/shinobi` client or `cameras.yaml` -> `SyncToShinobi` engine adhering strictly to Golden Template parameters.
