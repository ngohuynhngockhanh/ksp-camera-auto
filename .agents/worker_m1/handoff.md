# Handoff Report: Milestone M1 — Ansible Automated Shinobi Provisioning & Config (Requirement R1)

## 1. Observation

### 1.1 Go Configuration Changes
- **File**: `/home/ksp/ksp-camera-auto/internal/config/config.go`
  - Added `ShinobiConfig` struct (lines 62-67):
    ```go
    type ShinobiConfig struct {
        APIURL   string `yaml:"api_url"`
        APIKey   string `yaml:"api_key"`
        GroupKey string `yaml:"group_key"`
    }
    ```
  - Added `MCPConfig` struct (lines 69-74):
    ```go
    type MCPConfig struct {
        Enabled                      bool   `yaml:"enabled"`
        APIKey                       string `yaml:"api_key"`
        AllowUnauthenticatedLoopback bool   `yaml:"allow_unauthenticated_loopback"`
    }
    ```
  - Added `Shinobi ShinobiConfig` and `MCP MCPConfig` fields to `Config` struct (lines 76-83).
  - Updated `Default()` (lines 85-115) to default `Shinobi.APIURL` to `"http://127.0.0.1:8080"`, `MCP.Enabled` to `true`, `MCP.AllowUnauthenticatedLoopback` to `true`.
  - Updated `applyDefaults()` (lines 137-183) to backfill `Shinobi.APIURL`.
- **File**: `/home/ksp/ksp-camera-auto/internal/config/config_test.go`
  - Created unit tests covering default config, missing file fallback, full YAML unmarshalling (Shinobi + MCP), and syntax error handling.
- **File**: `/home/ksp/ksp-camera-auto/config.example.yaml`
  - Added documented sample configuration blocks for `shinobi` and `mcp`.

### 1.2 Ansible Automation Changes
- **Controller Host**: `root@172.16.5.180`
- **Role Location**: `/build/armbian-build/ansible/playbook/roles/app_ksp_bida/`
- **Variables**: `vars/main.yml`
  ```yaml
  shinobi_api_url: "http://127.0.0.1:8080"
  shinobi_mail: "ngohuynhngockhanh@gmail.com"
  shinobi_pass: "smarthome12345"
  shinobi_super_mail: "ngohuynhngockhanh@gmail.com"
  shinobi_super_pass: "KSPHondaCity51F79713@"
  shinobi_super_token: "ksp_super_token_kspbida_auto"
  ```
- **Provisioning Tasks**: `tasks/shinobi_provision.yml`
  1. Service liveness probe on port `8080`.
  2. Regular user probe `POST http://127.0.0.1:8080/?json=true` (`ngohuynhngockhanh@gmail.com` / `smarthome12345`).
  3. Super Admin registration fallback (`POST /super/?json=true`, patch `/home/Shinobi/super.json`, `POST /super/<token>/accounts/registerAdmin`).
  4. Session token and Group Key extraction (`$user.auth_token`, `$user.ke`).
  5. API Key provisioning via `POST /:auth/api/:ke/add` with IP `127.0.0.1` and full capabilities (`auth_socket`, `get_monitors`, `control_monitors`, `get_logs`, `watch_stream`, `watch_snapshot`, `watch_videos`, `delete_videos`).
  6. Legacy key sync to `/root/ota-mqtt/change_ok/shinobi_camera_id`.
- **Main Deployment Tasks**: `tasks/main.yml`
  - Includes `shinobi_provision.yml`.
  - Dynamically writes `/opt/ksp-cam/config.yaml` containing `shinobi` and `mcp` sections.
  - Auto-seeds monitors using the provisioned API Key and restarts `kspcam.service`.

### 1.3 Live Target Execution Result (`inut_204_63`)
- Command: `ansible-playbook -i /build/armbian-build/ansible/inventories/linux /build/armbian-build/ansible/playbook/ksp-bida.yml -e 'target=inut_204_63'`
- Result verbatim:
  ```
  TASK [app_ksp_bida : Report Shinobi provisioning status] ***********************
  ok: [inut_204_63] => {
      "msg": "Shinobi provisioned: GroupKey=pymid463, APIKey=kiwUyr... (length 30)"
  }
  ...
  TASK [app_ksp_bida : Seed result] **********************************************
  ok: [inut_204_63] => {
      "msg": "2026/08/23 23:46:45 imported 10 cameras (skipped 0) into /opt/ksp-cam/cameras.yaml"
  }
  ...
  PLAY RECAP *********************************************************************
  inut_204_63 : ok=26 changed=5 unreachable=0 failed=0 skipped=7 rescued=0 ignored=0
  ```
- Target `/opt/ksp-cam/config.yaml` content verified live:
  ```yaml
  server:
    addr: ":2028"
    username: "admin"
    password: "smarthome12345"
  cameras_file: "/opt/ksp-cam/cameras.yaml"
  defaults:
    hikvision_port: 8000
    dahua_port: 37777
    username: "admin"
    password: "smarthome12345"
    timeout_seconds: 30
    new_password: "smarthome12345"
  shinobi:
    api_url: "http://127.0.0.1:8080"
    api_key: "kiwUyrh1oSSGe1uB4s9kcdWlDJgbAY"
    group_key: "pymid463"
  mcp:
    enabled: true
    allow_unauthenticated_loopback: true
  ```
- Systemd service `kspcam.service` active and running (`PID 17317`).

---

## 2. Logic Chain

1. **Requirement Mapping**: Requirement R1 dictates automated Shinobi user check and API Key generation via Ansible, persisting connection settings in `/opt/ksp-cam/config.yaml`, and mapping these cleanly into Go struct `ShinobiConfig` without embedding plain-text credentials in the binary.
2. **Go Architecture**: By defining `ShinobiConfig` (`APIURL`, `APIKey`, `GroupKey`) and `MCPConfig` (`Enabled`, `APIKey`, `AllowUnauthenticatedLoopback`) in `internal/config/config.go`, Go runtime components in future milestones (M2 Shinobi Client & M3 MCP Server) receive strongly typed, validated settings while the Go binary remains completely decoupled from plain-text administrative passwords.
3. **Ansible Idempotence**: The `shinobi_provision.yml` workflow tests whether the user is already authenticated. If valid, it skips account creation; if missing, it executes Super Admin registration. It then checks for an existing `127.0.0.1` full-permission key before issuing an add request, ensuring idempotent re-runs do not duplicate API keys.
4. **Validation**: All Go tests (`go test -v ./internal/config/...` and `go test ./...`) pass 100%. The Ansible playbook was executed against remote node `inut_204_63`, successfully provisioning the API key, rendering `config.yaml`, importing 10 cameras, and running `kspcam.service`.

---

## 3. Caveats

- **Network Scope**: The generated Shinobi API key is restricted to IP `127.0.0.1`. Remote clients accessing Shinobi directly must route through local loopback or use the `kspcam` reverse proxy/API layer.
- **Shinobi Super Admin Password**: Super Admin password is stored only in Ansible variables on controller `172.16.5.180` (`vars/main.yml`) and is never baked into the compiled Go binary.

---

## 4. Conclusion

Milestone M1 (Requirement R1) is completely implemented, tested, and validated:
- `internal/config/config.go` provides `ShinobiConfig` and `MCPConfig` with unit tests passing.
- `config.example.yaml` document sample configurations.
- Ansible role `app_ksp_bida` on `172.16.5.180` automates Shinobi account verification, super admin registration fallback, IP-restricted API key generation, and `/opt/ksp-cam/config.yaml` writing.
- Live deployment to `inut_204_63` confirmed 100% success.

---

## 5. Verification Method

### 5.1 Run Go Unit Tests
```bash
export PATH=/home/ksp/go-sdk/bin:$PATH
cd /home/ksp/ksp-camera-auto
go test -v ./internal/config/...
go test ./...
go run ./tools/docgen -check
```

### 5.2 Verify Ansible Role & Remote Deployment
```bash
# Syntax Check
ssh root@172.16.5.180 "ansible-playbook -i /build/armbian-build/ansible/inventories/linux /build/armbian-build/ansible/playbook/ksp-bida.yml --syntax-check -e 'target=inut_204_63'"

# Live Deployment Check on inut_204_63
ssh root@172.16.5.180 "ansible-playbook -i /build/armbian-build/ansible/inventories/linux /build/armbian-build/ansible/playbook/ksp-bida.yml -e 'target=inut_204_63'"

# Verify Remote Target config.yaml & service
ssh root@172.16.5.180 "ansible -i /build/armbian-build/ansible/inventories/linux inut_204_63 -m shell -a 'cat /opt/ksp-cam/config.yaml; systemctl status kspcam --no-pager'"
```
