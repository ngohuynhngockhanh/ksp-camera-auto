# DISPATCH — 2026-08-24T16:26:27+07:00

Target Host: `inut_204_163` (`77.88.204.163` via Ansible controller `root@172.16.5.180`)

Objectives:
1. Venue Name Configuration ("CX King Luxury"):
   - Set the venue name to "CX King Luxury".
   - Update `/root/ota-mqtt/change_ok/logo_header` (or whatever file in `/root/ota-mqtt/change_ok` corresponds to the venue title/logo header).
   - Test publishing update via MQTT broker `127.0.0.1:12369` on topic `/private/i_sets` or via `POST /api/redbida/apply` from `kspcam`.
   - Verify that the value is written to disk and read back accurately.
2. Virtual IP Binding:
   - Check if IP `192.168.1.254` is in use on LAN (ping `192.168.1.254`).
   - If free, add virtual IP `192.168.1.254/24` to `eth0` (`ip addr add 192.168.1.254/24 dev eth0` or `ifconfig eth0:0 192.168.1.254 netmask 255.255.255.0 up`).
   - If `.254` is already in use, check `.253` and bind `.253`.
   - Verify `ip addr show dev eth0` shows the virtual IP.
3. Shinobi API Key & Monitor Token (0.0.0.0 IP restriction):
   - In Shinobi (database `ccio` / API), create/configure an API Key for user `ngohuynhngockhanh@gmail.com` (GroupKey `P6zP1kVhht`) with IP restriction `0.0.0.0` and permissions for viewing streams and downloading/getting videos.
   - Configure `shinobi_monitor_token` (also with `0.0.0.0` allowance) and write it into `/root/ota-mqtt/change_ok/shinobi_monitor_token`.
   - Update `/opt/ksp-cam/config.yaml` with the configured API key if needed, restart `kspcam.service`, and verify `GET /api/shinobi/status`.
4. Node-RED & Redbida Integration Testing:
   - Verify `GET /api/redbida/catalog` and `POST /api/redbida/refresh` respond with 200 OK and no 500 errors.
   - Verify that keys in `/root/ota-mqtt/change_ok` and project parameters in Node-RED project `ok2` (:2023) are synchronized.
