# Comprehensive Survey Report: Target Host `inut_204_163` (`77.88.204.163`)

**Target Identifier**: `inut_204_163`  
**Primary IP (VPN / Overlay)**: `77.88.204.163`  
**LAN IP (Local Subnet)**: `192.168.1.21/24`  
**Survey Timestamp**: 2026-08-24T09:14:30Z  
**Surveyor**: Teamwork Explorer (Survey Specialist 2)

---

## 1. Remote Access & Deployment Route

### 1.1 SSH Connectivity Topology
- **Direct SSH from Developer Workstation**: SSH connection requires key authentication; direct SSH without key is rejected (`Permission denied (publickey)`).
- **Control Jump Route (Ansible Controller)**:
  - **Controller Host**: `root@172.16.5.180` (Ubuntu 22.04 LTS x86_64, Ansible core 2.17.14).
  - **Controller SSH Access**: Passwordless root access via workstation SSH key.
  - **Target SSH Access**: Controller holds authorized key `/root/.ssh/id_rsa` which has immediate, passwordless `root@77.88.204.163` access.
  - **Direct Command Execution Pattern**:
    ```bash
    ssh root@172.16.5.180 "ssh root@77.88.204.163 '<command>'"
    ```
- **Automated Deployment Pipeline**:
  - Located on controller at `/build/armbian-build/ansible/`.
  - Inventory entry: `/build/armbian-build/ansible/inventories/linux/hosts:363` (`inut_204_163 ansible_host=77.88.204.163`).
  - Role path: `/build/armbian-build/ansible/playbook/roles/app_ksp_bida/`.
  - Playbook invocation:
    ```bash
    make ksp-bida inut_204_163
    # or directly:
    ansible-playbook -i inventories/linux playbook/ksp-bida.yml -e "target=inut_204_163"
    ```
- **External Web Access (FRP)**:
  - FRP client runs on `inut_204_163` connected to `video.io.vn:7002`.
  - Subdomain routing: `http://ksp-cam-inut-204-163.video.io.vn/` proxies port `:2028`.

---

## 2. Host System & Hardware Architecture

| Parameter | Observed Value | Verification Detail |
|---|---|---|
| **Hostname** | `inut-204-163` | `uname -a` |
| **OS Distribution** | `Armbian OS 25.11.0 bullseye` (Debian 11 bullseye) | `/etc/os-release` |
| **Linux Kernel** | `6.1.155-ophub` #1 SMP aarch64 | `uname -r` |
| **SoC / Architecture** | Amlogic S912 (meson-gxm) / ARM64 (`aarch64`) | `lscpu`, `/etc/armbian-release` |
| **CPU Cores** | 8x ARM Cortex-A53 @ 1.512 GHz (64-bit Little-Endian) | `lscpu` (8 online cores) |
| **System Memory (RAM)** | 1.8 GiB total (601 MiB used, 1.1 GiB free/available) | `free -h` |
| **Swap Space** | 0 Bytes (disabled) | `free -h` |
| **Root Filesystem** | `/dev/mmcblk2p2` (eMMC): 6.6 GiB total, 3.7 GiB used (56%), 3.0 GiB available | `df -h /` |
| **Media Recording Storage** | `/dev/mmcblk1p1` (SD/USB): 58 GiB total, 384 KiB used (1%), 58 GiB available mounted at `/media/usb1` | `df -h /media/usb1` |
| **Systemd Status** | `running` (State: healthy, all default targets loaded) | `systemctl is-system-running` |
| **Network Interfaces** | `eth0`: `192.168.1.21/24`<br>`tun1`: `77.88.204.163/32`<br>`lo`: `127.0.0.1/8` | `ip -4 addr` |

---

## 3. Services Survey & Diagnostics

### 3.1 Shinobi NVR Service (Port :8080)
- **Process & Supervisor**: Managed via PM2 (Process ID: 4, name: `camera`, PID: 13092), running `/home/Shinobi/camera.js` under Node.js v20.19.6.
- **Port State**: Listening on TCP `0.0.0.0:8080`.
- **Database Engine**: MariaDB 10.5.29 running on `127.0.0.1:3306` (database `ccio`).
- **Super Administrator Credentials**:
  - Email: `ngohuynhngockhanh@gmail.com`
  - Pass Hash: `2a0bf9d867579d319e031c7225fd4d07` (MD5 of `KSPHondaCity51F79713@`) in `/home/Shinobi/super.json`.
  - Super Token: `ksp_super_token_kspbida_auto` configured in `super.json:tokens`.
- **Main User Account & Authentication**:
  - Email: `ngohuynhngockhanh@gmail.com`
  - Password: `smarthome12345` (SHA-256 in DB: `83bdc93367d1a29dac1b422a4a654e87f0a366d950e260e70405b0118c99444c`).
  - User ID (`uid`): `7Tge1rS47M`
  - Group Key (`ke`): `P6zP1kVhht`
  - Live Auth Token: `7dd744d1221627c95352edd889af096d` (temporary session).
- **Dedicated REST API Key**:
  - Code: `YAN3BDMg4mAS4VaFqJ13S0RSIh92wy`
  - IP Restriction: `127.0.0.1`
  - Capabilities: Full unrestricted administrative rights (`auth_socket`, `get_monitors`, `control_monitors`, `get_logs`, `watch_stream`, `watch_snapshot`, `watch_videos`, `delete_videos`).
- **Storage Configuration**:
  - Primary Videos Dir: `/media/usb1` (58 GiB free).
  - Additional Storage: `/home/Shinobi/videos2/`, `/tmp/khanh/`, `/root/cache/`.
- **Current Monitor Count**: 0 monitors currently configured in Shinobi. Ready for automatic ingestion from `kspcam`.

### 3.2 Node-RED Service (Port :2023)
- **Process & Supervisor**: Managed via PM2 (Process ID: 2, name: `inut`, PID: 12338), running Node-RED v4.1.2 under Node.js v20.19.6.
- **Port State**: Listening on TCP `0.0.0.0:2023`.
- **Root Directory**: `/root/.node-red/`.
- **Active Project**: `ok2` configured in `/root/.node-red/.config.projects.json`.
- **Active Flow File**: `/root/.node-red/projects/ok2/flow.json` (size: 411,627 bytes).
- **HTTP Liveness**:
  - `GET /`: `200 OK` (Node-RED dashboard / UI).
  - `GET /admin`: `301 Moved` to `/admin/` (Node-RED Editor).

### 3.3 MQTT Broker (Port :12369)
- **Process & Supervisor**: Managed via PM2 (Process ID: 3, name: `ota-mqtt`, PID: 14057), running `/root/ota-mqtt/index.js`.
- **Broker Engine**: Embedded `aedes` v0.49.0 MQTT broker on Node.js.
- **Port State**: Listening on TCP `0.0.0.0:12369` (plus UDP `37021`).
- **Liveness & Responsiveness**: TCP connection probe to `127.0.0.1:12369` succeeds immediately (socket connect result: `0`).
- **MQTT Topics Handled**:
  - Read Topic: `/private/i_gets`
  - Read Ack Topic: `/private/i_gets/ack`
  - Write Topic: `/private/i_sets`
  - Write Ack Topic: `/private/i_sets/ack`

### 3.4 Redbida Key Catalog Directory (`/root/ota-mqtt/change_ok`)
- **Directory**: `/root/ota-mqtt/change_ok` exists with `0755` permissions.
- **Existing Files & Values**:
  | File Name | Size | Content Summary / Value |
  |---|---|---|
  | `apiRecentInput_string` | 2 B | `[]` |
  | `db_check_range` | 3 B | `365` |
  | `db_check_size_lm` | 4 B | `2166` |
  | `frpc_config` | 202 B | `[common] server_addr = video.io.vn; server_port = 7002; token = 2J6R3...` |
  | `logo_header` | 80 B | `https://vnmap-backend.inut.vn/uploads/logo_fbf4fa6d73ea89dff0b1_1_d9e2a7cf00.png` |
  | `mqtt_broker` | 29 B | `inut.mqtt.mysystemservice.com` |
  | `mqtt_password` | 32 B | `eRVCNGjJHw1VDVZu9DclJPLWjwCu46ho` |
  | `mqtt_port` | 4 B | `2883` |
  | `mqtt_username` | 11 B | `cast204.163` |
  | `node_info` | 9,329 B | JSON UI settings definitions for Redbida |
  | `view_count` | 1 B | `1` |
- **Note on Group Key Record**: `/root/ota-mqtt/change_ok/shinobi_camera_id` is ready to be set to `P6zP1kVhht`.

### 3.5 `kspcam` Service & Status (`kspcam.service`)
- **Service Name**: `kspcam.service` (Systemd unit `/etc/systemd/system/kspcam.service`).
- **State**: `active (running)`, PID 27041, uptime healthy, 0 unexpected restarts.
- **Binary**: `/opt/ksp-cam/kspcam` (ARM64 static binary, version `3e58415-redbida-hardened-20260824`).
- **Listening Port**: TCP `0.0.0.0:2028`.
- **Verified Endpoints**:
  - `GET /healthz` -> `ok` (HTTP 200)
  - `POST /login` -> HTTP 200 with session cookie (`admin:smarthome12345`)
  - `GET /api/config` -> `{"maxReviewHours":72,"redbidaEnabled":true,"role":"admin"}`
  - `GET /api/redbida/catalog` -> Returns 130 configuration keys from Node-RED project!
  - `GET /api/shinobi/status` -> Currently returns `{"configured":false}` because the `shinobi:` section in `/opt/ksp-cam/config.yaml` is pending insertion of the generated API key.

---

## 4. LAN Network & Discovered Cameras Survey

`inut_204_163` is directly attached to the customer LAN `192.168.1.0/24` via `eth0` (`192.168.1.21`).  
Network probing confirmed **8 active Dahua/KBVision IP cameras** responding on the LAN:

| Camera Name | IP Address | MAC Address | Vendor / OUI | Open Ports | Protocol |
|---|---|---|---|---|---|
| Camera 01 | `192.168.1.111` | `e0:2e:fe:14:12:d6` | Dahua / KBVision | `80`, `554`, `37777` | DVRIP / RTSP / HTTP |
| Camera 02 | `192.168.1.112` | `e0:2e:fe:14:13:62` | Dahua / KBVision | `80`, `554`, `37777` | DVRIP / RTSP / HTTP |
| Camera 03 | `192.168.1.113` | `e0:2e:fe:14:13:76` | Dahua / KBVision | `80`, `554`, `37777` | DVRIP / RTSP / HTTP |
| Camera 04 | `192.168.1.114` | `e0:2e:fe:14:13:5b` | Dahua / KBVision | `80`, `554`, `37777` | DVRIP / RTSP / HTTP |
| Camera 05 | `192.168.1.115` | `e0:2e:fe:14:13:8b` | Dahua / KBVision | `80`, `554`, `37777` | DVRIP / RTSP / HTTP |
| Camera 06 | `192.168.1.116` | `e0:2e:fe:14:13:7e` | Dahua / KBVision | `80`, `554`, `37777` | DVRIP / RTSP / HTTP |
| Camera 07 | `192.168.1.117` | `e0:2e:fe:24:6c:ae` | Dahua / KBVision | `80`, `554`, `37777` | DVRIP / RTSP / HTTP |
| Camera 08 | `192.168.1.118` | `e0:2e:fe:24:6b:bc` | Dahua / KBVision | `80`, `554`, `37777` | DVRIP / RTSP / HTTP |

---

## 5. Synthesis & Downstream Action Items

1. **Shinobi Configuration Block for `/opt/ksp-cam/config.yaml`**:
   ```yaml
   shinobi:
     api_url: "http://127.0.0.1:8080"
     api_key: "YAN3BDMg4mAS4VaFqJ13S0RSIh92wy"
     group_key: "P6zP1kVhht"
   ```
2. **Key Catalog Persistence**:
   - Write `P6zP1kVhht` into `/root/ota-mqtt/change_ok/shinobi_camera_id`.
3. **Camera Ingestion & Golden Template**:
   - Populate `/opt/ksp-cam/cameras.yaml` with the 8 Dahua cameras (`192.168.1.111` - `192.168.1.118`) with default credentials `admin:smarthome12345`.
   - Apply Golden Template standards (audio AAC copy / no, `-tag:v hvc1`, clean stream flags) and sync to Shinobi monitors.
