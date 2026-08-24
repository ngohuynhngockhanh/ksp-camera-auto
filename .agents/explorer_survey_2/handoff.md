# Handoff Report: Target Host `inut_204_163` Environment Survey

## 1. Observation

- **SSH & Remote Access Route**:
  - Direct SSH from workstation: `ssh root@77.88.204.163` failed with `Permission denied (publickey,password,keyboard-interactive)`.
  - Ansible controller `ssh root@172.16.5.180` connected successfully with `Linux armbianbuild 5.15.0-171-generic x86_64`.
  - Controller to target: `ssh root@172.16.5.180 "ssh root@77.88.204.163 'uname -a'"` returned:
    ```
    Linux inut-204-163 6.1.155-ophub #1 SMP Fri Oct 10 03:56:20 UTC 2025 aarch64 GNU/Linux
    ```
  - Inventory entry in `/build/armbian-build/ansible/inventories/linux/hosts:363`: `inut_204_163 ansible_host=77.88.204.163`.

- **Target System Environment**:
  - OS: `Armbian OS 25.11.0 bullseye` (Debian 11 bullseye, Linux 6.1.155-ophub aarch64).
  - CPU: 8 cores Cortex-A53 @ 1.512 GHz (Amlogic S912 SoC).
  - RAM: 1.8 GiB total, 601 MiB used, 1.1 GiB available, 0B swap.
  - Disk: `/dev/mmcblk2p2` (6.6G total, 3.7G used, 3.0G available at `/`), `/dev/mmcblk1p1` (58G total, 384K used, 58G available at `/media/usb1`).
  - Network: `eth0` (`192.168.1.21/24`), `tun1` (`77.88.204.163/32`), `lo` (`127.0.0.1/8`).
  - Systemd status: `systemctl is-system-running` returned `running`.

- **Shinobi NVR Service on :8080**:
  - PM2 process ID 4 (`camera`, PID 13092) running `/home/Shinobi/camera.js` listening on TCP `0.0.0.0:8080`.
  - MariaDB 10.5.29 on `127.0.0.1:3306` database `ccio`.
  - Super admin: `ngohuynhngockhanh@gmail.com` with hash `2a0bf9d867579d319e031c7225fd4d07` in `/home/Shinobi/super.json`, token `ksp_super_token_kspbida_auto`.
  - Main user: `ngohuynhngockhanh@gmail.com` / `smarthome12345` (UID: `7Tge1rS47M`, GroupKey: `P6zP1kVhht`).
  - Dedicated API Key: `YAN3BDMg4mAS4VaFqJ13S0RSIh92wy` (IP `127.0.0.1`, full administrative permissions).
  - Monitor count in DB: 0. Storage: `/media/usb1` (58 GB free).

- **Node-RED on :2023**:
  - PM2 process ID 2 (`inut`, PID 12338) running `node-red` v4.1.2 listening on TCP `0.0.0.0:2023`.
  - Directory: `/root/.node-red/`. Active project: `ok2` in `.config.projects.json`.
  - Flow file: `/root/.node-red/projects/ok2/flow.json` (411,627 bytes).
  - HTTP `GET http://127.0.0.1:2023/` returns `200 OK`.

- **MQTT Broker on :12369**:
  - PM2 process ID 3 (`ota-mqtt`, PID 14057) running `node /root/ota-mqtt/index.js` using embedded `aedes` v0.49.0 listening on TCP `0.0.0.0:12369`.
  - Socket connect to `127.0.0.1:12369` returned code `0` (connected).
  - Topics: `/private/i_gets`, `/private/i_gets/ack`, `/private/i_sets`, `/private/i_sets/ack`.

- **Redbida Key Catalog Directory `/root/ota-mqtt/change_ok`**:
  - Exists with 11 configuration files: `apiRecentInput_string`, `db_check_range`, `db_check_size_lm`, `frpc_config`, `logo_header`, `mqtt_broker`, `mqtt_password`, `mqtt_port`, `mqtt_username`, `node_info`, `view_count`.

- **`kspcam` Service & Status**:
  - Systemd unit `/etc/systemd/system/kspcam.service` is `active (running)` (PID 27041, static ARM64 binary `3e58415-redbida-hardened-20260824`).
  - `GET /healthz` -> `ok` (200 OK).
  - `POST /login` -> 200 OK (`admin:smarthome12345`).
  - `GET /api/redbida/catalog` -> Returns 130 keys.
  - `/opt/ksp-cam/config.yaml` has `redbida` & `mcp` enabled; `shinobi` section needs API key and group key populated.

- **LAN Cameras**:
  - 8 Dahua/KBVision cameras online on `192.168.1.111` through `192.168.1.118` (all with open ports 80, 554, 37777).

---

## 2. Logic Chain

1. **Remote Execution Path**: Direct SSH access from the workstation is restricted by public key auth, but `ssh root@172.16.5.180` has passwordless key access to `root@77.88.204.163`. All operations and Ansible playbooks run through this controller.
2. **Binary Compatibility**: Target architecture is `aarch64` (`arm64`), requiring `GOARCH=arm64` static binaries.
3. **Shinobi Integration**: Shinobi is fully operational on port `8080` with MariaDB. An IP-bound API key (`YAN3BDMg4mAS4VaFqJ13S0RSIh92wy`) and Group Key (`P6zP1kVhht`) were verified and are ready to be placed into `/opt/ksp-cam/config.yaml`.
4. **Node-RED & MQTT Bridge**: Node-RED (port 2023, active project `ok2`) and `ota-mqtt` broker (port 12369) are running normally. `kspcam`'s Redbida module successfully loaded all 130 catalog keys via `GET /api/redbida/catalog`.
5. **Camera Provisioning**: 8 Dahua cameras (`192.168.1.111` - `118`) are active on the local subnet and ready for Golden Template configuration and Shinobi sync.

---

## 3. Caveats

- Direct SSH requires proxying through `172.16.5.180`.
- External HTTPS access via `https://ksp-cam-inut-204-163.video.io.vn/` is currently returning 404 at the central ingress until TLS certificate routing is updated; plain HTTP `http://ksp-cam-inut-204-163.video.io.vn/` operates as expected.
- No destructive changes were performed during this survey.

---

## 4. Conclusion

Target host `inut_204_163` (`77.88.204.163`) is in a healthy, fully-functional state with all prerequisites met for completing the deployment and integration of `ksp-camera-auto`:
- Architecture: ARM64 / Debian 11 / Kernel 6.1.
- Shinobi NVR (:8080) active with Group Key `P6zP1kVhht` and API Key `YAN3BDMg4mAS4VaFqJ13S0RSIh92wy`.
- Node-RED (:2023) active with project `ok2`.
- MQTT Broker (:12369) active and responsive.
- Redbida catalog (/root/ota-mqtt/change_ok) populated with 11 keys and 130 Node-RED project parameters.
- 8 Dahua cameras (`192.168.1.111` - `192.168.1.118`) reachable and ready for ingestion.

---

## 5. Verification Method

To independently reproduce and verify all findings:
```bash
# 1. Check target OS and architecture
ssh root@172.16.5.180 "ssh root@77.88.204.163 'uname -a; lscpu | grep Architecture'"

# 2. Check running services
ssh root@172.16.5.180 "ssh root@77.88.204.163 'pm2 list; systemctl status kspcam.service --no-pager'"

# 3. Verify Shinobi API Key & Monitors
ssh root@172.16.5.180 "ssh root@77.88.204.163 'curl -s http://127.0.0.1:8080/YAN3BDMg4mAS4VaFqJ13S0RSIh92wy/monitor/P6zP1kVhht'"

# 4. Verify Node-RED & MQTT broker
ssh root@172.16.5.180 "ssh root@77.88.204.163 'curl -I http://127.0.0.1:2023/ && nc -z -v 127.0.0.1 12369'"

# 5. Verify kspcam health and redbida catalog
ssh root@172.16.5.180 "ssh root@77.88.204.163 'curl -s http://127.0.0.1:2028/healthz'"
```
