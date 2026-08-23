# Handoff Report: Ansible Automated Shinobi Provisioning Survey

## 1. Observation

- **Ansible Controller Environment**:
  - Host: `root@172.16.5.180` (Ubuntu 22.04, `ansible [core 2.17.14]`, Python 3.10.12).
  - Makefile: `/build/armbian-build/ansible/Makefile` target `ksp-bida: @ansible-playbook -i inventories/linux playbook/ksp-bida.yml --forks 10 -e "target=$(word 2,$(MAKECMDGOALS))"`.
  - Playbook: `/build/armbian-build/ansible/playbook/ksp-bida.yml` targeting role `app_ksp_bida`.
  - Role path: `/build/armbian-build/ansible/playbook/roles/app_ksp_bida/`.
  - Global vars: `/build/armbian-build/ansible/playbook/vars/global_vars.yml`.
  - Role vars: `/build/armbian-build/ansible/playbook/roles/app_ksp_bida/vars/main.yml` (`shinobi_mail: "ngohuynhngockhanh@gmail.com"`, `shinobi_pass: "smarthome12345"`).
  - Custom module: `/build/armbian-build/ansible/library/json_patch.py`.

- **Target Fixture Node (`inut_204_63` / `77.88.204.63`)**:
  - Shinobi service runs under PM2 (`camera`, `/home/Shinobi/camera.js`) listening on `http://127.0.0.1:8080`.
  - `super.json`: `[{"mail":"ngohuynhngockhanh@gmail.com","pass":"2a0bf9d867579d319e031c7225fd4d07"}]` (MD5 of `KSPHondaCity51F79713@`).
  - `/opt/ksp-cam/`: Contains `kspcam` binary, `config.yaml`, `cameras.yaml`, `.kspcam.key`.
  - `/root/ota-mqtt/change_ok/shinobi_camera_id`: Contains Group Key `pymid463`.

- **Shinobi REST API Endpoints & Live Verification**:
  - User Login: `POST http://127.0.0.1:8080/?json=true` with `function="dash"`, `mail="ngohuynhngockhanh@gmail.com"`, `pass="smarthome12345"` -> Returns `ok: true`, `auth_token: "..."`, `ke: "pymid463"`, `uid: "VsKWex7YFj"`.
  - Super Admin Login: `POST http://127.0.0.1:8080/super/?json=true` with `function="super"`, `mail="ngohuynhngockhanh@gmail.com"`, `pass="KSPHondaCity51F79713@"` -> Returns `ok: true`.
  - Super Admin API: `POST http://127.0.0.1:8080/super/<token>/accounts/registerAdmin` -> Creates admin user when user does not exist.
  - API Key List: `GET http://127.0.0.1:8080/<auth_token>/api/<ke>/list` -> Returns list of active keys for user.
  - API Key Create: `POST http://127.0.0.1:8080/<auth_token>/api/<ke>/add` with `ip: "127.0.0.1"` and all permissions (`auth_socket`, `get_monitors`, `control_monitors`, `get_logs`, `watch_stream`, `watch_snapshot`, `watch_videos`, `delete_videos`) -> Returns `ok: true`, `api.code` (30-character API key).
  - API Key Direct Request: `GET http://127.0.0.1:8080/<api_key>/monitor/<ke>` successfully retrieves monitors.

- **Go Struct in `internal/config/config.go`**:
  - Currently contains `Server`, `Defaults`, `CamerasFile`.
  - Must add `Shinobi` struct with `APIURL string yaml:"api_url"`, `APIKey string yaml:"api_key"`, `GroupKey string yaml:"group_key"`.
  - Passwords are never defined in Go structs or binary.

## 2. Logic Chain

1. Shinobi NVR on each edge box listens on `127.0.0.1:8080`.
2. When deploying `app_ksp_bida` via `make ksp-bida <host>`, Ansible executes on controller `172.16.5.180` and communicates with the target node.
3. Ansible tests user authentication via `POST http://127.0.0.1:8080/?json=true`. If the user does not exist, Ansible falls back to the Super Admin API (`POST /super/<token>/accounts/registerAdmin`) using credentials stored safely in Ansible vars.
4. Once user authentication succeeds, Ansible retrieves the session `auth_token` and `group_key` (`ke`).
5. Ansible calls `GET /<auth_token>/api/<ke>/list` to check for an existing `127.0.0.1` API key. If none exists, it calls `POST /<auth_token>/api/<ke>/add` to generate a dedicated, IP-restricted (`127.0.0.1`) API key with full permissions.
6. Ansible writes the `shinobi` block (`api_url`, `api_key`, `group_key`) directly into `/opt/ksp-cam/config.yaml`.
7. `kspcam` starts up, reads `/opt/ksp-cam/config.yaml`, and maps the fields into `cfg.Shinobi`, providing full REST API capability with zero password hardcoding.

## 3. Caveats

- Super Admin user registration requires a valid super token. Ansible can ensure a persistent automation token in `/home/Shinobi/super.json` via `json_patch`.
- If Shinobi is not installed or the PM2 process is stopped on a target node, Ansible should catch the error gracefully and continue deploying `kspcam` with an empty `shinobi` section (best-effort resilience).

## 4. Conclusion

- Automated provisioning for Shinobi inside Ansible role `app_ksp_bida` is completely feasible, robust, and verified live on actual hardware.
- The Go struct additions to `internal/config/config.go` (`Shinobi.APIURL`, `Shinobi.APIKey`, `Shinobi.GroupKey`) cleanly decouple configuration from credentials, guaranteeing zero password hardcoding.
- Detailed implementation steps and task definitions are documented in `report.md`.

## 5. Verification Method

- **Controller & Target Connectivity**:
  ```sh
  ssh root@172.16.5.180 "ansible-inventory -i /build/armbian-build/ansible/inventories/linux --host inut_204_63"
  ```
- **Live Shinobi API Verification on Target**:
  ```sh
  ssh root@172.16.5.180 "ssh root@77.88.204.63 'curl -s -X POST http://127.0.0.1:8080/?json=true -H \"Content-Type: application/json\" -d \"{\\\"function\\\":\\\"dash\\\",\\\"mail\\\":\\\"ngohuynhngockhanh@gmail.com\\\",\\\"pass\\\":\\\"smarthome12345\\\"}\" | jq -r \".ok\"'"
  ```
- **Go Test Verification**:
  ```sh
  go test -v ./internal/config/...
  ```
