# Deployment — `make ksp-bida` (Ansible)

Deploys `kspcam` onto Armbian/IoT boxes and exposes its `:2028` UI through frp.
Lives on the ansible server `root@172.16.5.180:/build/armbian-build/ansible`,
mirroring `make inut-simple`.

```bash
make ksp-bida <host[,host2,...]>     # e.g. make ksp-bida inut_204_105,inut_204_142
```

## What it does (role `app_ksp_bida`)

1. **Arch-match** the binary: `-armhf` for 32-bit ARM (`ansible_architecture ~ '^armv'`),
   else arm64. Binaries are prebuilt (pure-Go, CGO off) into the role's `files/`.
2. Copy `kspcam` → `/opt/ksp-cam/kspcam`; write `config.yaml` (login
   `admin/smarthome12345`); install + start the `kspcam.service` systemd unit
   (`--addr 0.0.0.0:2028`, `KSPCAM_KEY_FILE=/opt/ksp-cam/.kspcam.key`).
3. **Auto-seed from Shinobi** (best-effort; skipped if Shinobi/creds absent):
   log into the box's Shinobi (`127.0.0.1:8080`), fetch monitors via the API,
   save to `/opt/ksp-cam/shinobi_monitors.json`, and run
   `kspcam --import-shinobi` to populate the inventory. Mirrors `backup-shinobi`.
4. **frp**: edit `/root/ota-mqtt/change_ok/frpc_config` — add `[common]`
   (`server_addr=video.io.vn`, `server_port=7002`, token) ONLY if missing, and
   add `[ksp-cam-<host>]` (`type=http`, `local_port=2028`, `subdomain=ksp-cam-<host>`).
   The host uses **dashes** (`inut_204_105` → `ksp-cam-inut-204-105`) since frp/DNS
   subdomains disallow `_`. Restart frpc.

## Shinobi seed credentials

The seed reads `shinobi_mail` / `shinobi_pass` (the same account `backup-shinobi`
uses). They are NOT hardcoded in the role; put them in a server-side vars file
(e.g. copy from `backup-shinobi.yml` into the role's `vars/main.yml`, or
`playbook/vars/global_vars.yml`). Empty → the seed block is skipped and the deploy
still succeeds. The Shinobi API returns each monitor's `details` as a JSON-encoded
STRING (not an object) — the importer handles both.

## Building the binaries

The role's `files/kspcam` (arm64) and `files/kspcam-armhf` (armv7) are the
**pure-Go default** build (Hikvision on an ARM box uses ISAPI:80, not the SDK).
Rebuild + copy from the dev machine:

```bash
make build-arm64 build-arm32
scp dist/kspcam-linux-arm64  root@172.16.5.180:.../roles/app_ksp_bida/files/kspcam
scp dist/kspcam-linux-armv7  root@172.16.5.180:.../roles/app_ksp_bida/files/kspcam-armhf
```

## Verify a box

```bash
ansible -i inventories/linux <host> -m shell -a \
  'systemctl is-active kspcam; ss -ltn | grep :2028; \
   curl -s -o/dev/null -w "%{http_code}" -d username=admin -d password=smarthome12345 http://127.0.0.1:2028/login'
# frpc: grep -A3 ksp-cam /root/ota-mqtt/change_ok/frpc_config
```

Reachable externally at the frp `ksp-cam-<host>` subdomain (must terminate TLS).
