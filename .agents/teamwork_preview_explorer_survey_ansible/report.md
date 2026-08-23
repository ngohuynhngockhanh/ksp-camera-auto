# Investigation Report: Ansible Automated Shinobi Provisioning (Requirement R1)

## 1. Executive Summary

This investigation explores the end-to-end automation of Shinobi NVR provisioning inside the Ansible role `app_ksp_bida` on controller `172.16.5.180`. The objective is to automatically verify and provision Shinobi user accounts, generate group keys and IP-restricted (127.0.0.1) API keys with full capabilities, write connection details to `/opt/ksp-cam/config.yaml`, and map the configuration cleanly into Go (`internal/config/config.go`) without hardcoding any credentials in the Go binary.

All findings have been validated live on the controller (`172.16.5.180`) and the active target fixture node (`inut_204_63` / `77.88.204.63`).

---

## 2. Infrastructure, Environment & Access Map

### 2.1 Ansible Controller & Workspace
- **Host**: `root@172.16.5.180` (Ubuntu 22.04 LTS, `ansible [core 2.17.14]`, Python 3.10.12).
- **Ansible Root**: `/build/armbian-build/ansible/`
- **Makefile Invocation**:
  ```makefile
  ksp-bida:
  	@if [ -z "$(word 2,$(MAKECMDGOALS))" ]; then echo "Usage: make ksp-bida <host[,host2,...]>"; exit 1; fi
  	@echo ">>> ksp-camera-auto (kspcam :2028 + frp) -> $(word 2,$(MAKECMDGOALS))"
  	@ansible-playbook -i inventories/linux playbook/ksp-bida.yml --forks 10 -e "target=$(word 2,$(MAKECMDGOALS))"
  ```
- **Playbook**: `/build/armbian-build/ansible/playbook/ksp-bida.yml`
- **Role Location**: `/build/armbian-build/ansible/playbook/roles/app_ksp_bida/`
  - `tasks/main.yml`
  - `vars/main.yml`
  - `files/kspcam` & `files/kspcam-armhf`
  - `files/frpc_add.py`
- **Global Variables**: `/build/armbian-build/ansible/playbook/vars/global_vars.yml`
- **Custom Modules**: `/build/armbian-build/ansible/library/json_patch.py`

### 2.2 Target Node Inspection (`inut_204_63` / `77.88.204.63`)
- **OS**: Linux ARM64 (Rockchip 64-bit kernel 6.1).
- **PM2 Processes**:
  - `camera` (pid 1166): Shinobi NVR instance running `/home/Shinobi/camera.js` on port `8080`.
  - `index`, `inut`, `ota-mqtt`.
- **Shinobi Directory**: `/home/Shinobi/`
  - `super.json`: `[{"mail": "ngohuynhngockhanh@gmail.com", "pass": "2a0bf9d867579d319e031c7225fd4d07"}]` (MD5 of `KSPHondaCity51F79713@`).
  - `conf.json`: Port 8080, MySQL `ccio`, videos dir `/media/usb1`.
- **Target App Directory**: `/opt/ksp-cam/`
  - `kspcam` (binary)
  - `config.yaml`
  - `cameras.yaml`
  - `.kspcam.key` (AES-256-GCM inventory encryption key)
  - `shinobi_monitors.json`
- **Group Key Record**: `/root/ota-mqtt/change_ok/shinobi_camera_id` (contains Group Key e.g. `pymid463`).

---

## 3. Shinobi REST API Protocol Specifications

Shinobi provides three distinct authentication layers:
1. **Regular User Dashboard Authentication** (`s.auth` / `Users` DB table)
2. **Super Administrator Authentication** (`s.superAuth` / `super.json` file)
3. **API Key Authentication** (`s.api` / `API` DB table)

### 3.1 Regular User Login & Verification Flow
- **Endpoint**: `POST http://127.0.0.1:8080/?json=true`
- **Request Headers**: `Content-Type: application/json`
- **Request Body**:
  ```json
  {
    "function": "dash",
    "mail": "ngohuynhngockhanh@gmail.com",
    "pass": "smarthome12345",
    "machineID": "ksp-bida-{{ inventory_hostname }}"
  }
  ```
- **Response Structure**:
  ```json
  {
    "ok": true,
    "$user": {
      "auth_token": "ba300f476e60b6bbe772aef941b59fb0",
      "ke": "pymid463",
      "uid": "VsKWex7YFj",
      "mail": "ngohuynhngockhanh@gmail.com"
    },
    "timezone": "Asia/Saigon"
  }
  ```
- **Key Extraction**:
  - `auth_token`: `response.$user.auth_token` (used for temporary session authentication).
  - `group_key`: `response.$user.ke` (Group Key identifier for monitors/cameras).
  - `user_id`: `response.$user.uid`.

### 3.2 Super Administrator Login & User Provisioning Flow
When a user does not exist or login returns `ok: false`:
1. **Super Admin Credentials**:
   - `mail`: `ngohuynhngockhanh@gmail.com`
   - `pass`: `KSPHondaCity51F79713@` (stored as MD5 `2a0bf9d867579d319e031c7225fd4d07` in `/home/Shinobi/super.json`).
2. **Super Admin API Authentication**:
   Shinobi reads `/home/Shinobi/super.json` on each super API call. If a super token is present in the `tokens` array/object of `super.json`:
   ```json
   [
     {
       "mail": "ngohuynhngockhanh@gmail.com",
       "pass": "2a0bf9d867579d319e031c7225fd4d07",
       "tokens": ["ksp_super_token_kspbida_auto"]
     }
   ]
   ```
   Ansible uses `json_patch` (built-in custom library) to ensure `tokens` contains the automation token.
3. **Admin User Registration Endpoint**:
   - **URL**: `POST http://127.0.0.1:8080/super/ksp_super_token_kspbida_auto/accounts/registerAdmin`
   - **Payload**:
     ```json
     {
       "mail": "ngohuynhngockhanh@gmail.com",
       "pass": "smarthome12345",
       "password_again": "smarthome12345",
       "ke": "",
       "details": {
         "allmonitors": "1"
       }
     }
     ```
   - **Response**: `{ "ok": true, "user": { "ke": "<generated_ke>", "uid": "<generated_uid>", "mail": "..." } }`.
4. **Post-Registration**: Re-authenticate via regular user login (`POST /?json=true`) to obtain session token and group key.

### 3.3 Shinobi API Key Creation & Permission Specification
Once user `auth_token` and `group_key` are available:
- **Listing Existing Keys**:
  - `GET http://127.0.0.1:8080/{{ auth_token }}/api/{{ group_key }}/list`
  - Returns: `{ "ok": true, "uid": "...", "ke": "...", "keys": [ { "code": "...", "ip": "127.0.0.1", "details": {...} } ] }`.
- **Creating a Full-Access Key with 127.0.0.1 IP Binding**:
  - `POST http://127.0.0.1:8080/{{ auth_token }}/api/{{ group_key }}/add`
  - **Payload**:
    ```json
    {
      "ip": "127.0.0.1",
      "details": {
        "auth_socket": "1",
        "get_monitors": "1",
        "control_monitors": "1",
        "get_logs": "1",
        "watch_stream": "1",
        "watch_snapshot": "1",
        "watch_videos": "1",
        "delete_videos": "1"
      }
    }
    ```
  - **Permission Breakdown** (from `/home/Shinobi/libs/monitor.js:770` `s.checkPermission`):
    - `auth_socket: "1"` — allows WebSocket and streaming socket authentication.
    - `get_monitors: "1"` — allows querying monitor lists and probe info.
    - `control_monitors: "1"` — allows start/stop/restart stream operations and monitor CRUD (`addOrEditMonitor`, `deleteMonitor`).
    - `get_logs: "1"` — allows reading Shinobi system and detector logs.
    - `watch_stream: "1"` — allows viewing live streams (FLV, HLS, MJPEG).
    - `watch_snapshot: "1"` — allows capturing JPEG snapshots.
    - `watch_videos: "1"` — allows querying recorded video lists and playback.
    - `delete_videos: "1"` — allows deleting recorded video files.
    - When all permissions equal `"1"`, `isRestrictedApiKey` evaluates to `false` (full unrestricted administrative capability).
  - **Response**:
    ```json
    {
      "ok": true,
      "api": {
        "ke": "pymid463",
        "uid": "VsKWex7YFj",
        "code": "30CharacterAlphanumericString",
        "ip": "127.0.0.1",
        "details": { ... },
        "time": "2026-08-23 23:33:00"
      }
    }
    ```

---

## 4. Ansible Role Implementation Plan (`app_ksp_bida`)

### 4.1 Variables Configuration
In `playbook/roles/app_ksp_bida/vars/main.yml`:
```yaml
---
shinobi_api_url: "http://127.0.0.1:8080"
shinobi_mail: "ngohuynhngockhanh@gmail.com"
shinobi_pass: "smarthome12345"
shinobi_super_mail: "ngohuynhngockhanh@gmail.com"
shinobi_super_pass: "KSPHondaCity51F79713@"
shinobi_super_token: "ksp_super_token_kspbida_auto"
```

### 4.2 Automated Provisioning Tasks (`tasks/main.yml`)
The tasks follow a fail-safe, idempotent workflow:
1. **Liveness Probe**: Check if Shinobi service is running on `:8080`.
2. **User Login Probe**: Attempt `POST /?json=true`.
3. **Super Admin Provisioning (Conditional)**: If user login fails:
   - Patch `/home/Shinobi/super.json` to insert `tokens: ["ksp_super_token_kspbida_auto"]`.
   - Call `POST /super/<token>/accounts/registerAdmin`.
   - Re-login as regular user.
4. **Group Key & Auth Token Resolution**: Capture `ksp_shinobi_token` and `ksp_shinobi_gk`.
5. **API Key Provisioning**:
   - Query `GET /<token>/api/<gk>/list`.
   - Search for existing `127.0.0.1` key with full access.
   - If absent, call `POST /<token>/api/<gk>/add` with `ip: "127.0.0.1"` and all permissions.
   - Set fact `ksp_shinobi_api_key`.
6. **Persist Group Key File**: Update `/root/ota-mqtt/change_ok/shinobi_camera_id`.
7. **Deploy Configuration**: Write `/opt/ksp-cam/config.yaml` containing the `shinobi` block.
8. **Auto-Seed Monitors**: Query Shinobi API with `ksp_shinobi_api_key` directly and run `kspcam --import-shinobi`.
9. **Restart Service**: Restart `kspcam.service` via systemd.

### 4.3 Target `/opt/ksp-cam/config.yaml` Output
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
  api_key: "30CharacterAlphanumericKey"
  group_key: "pymid463"
```

---

## 5. Go Struct Definition & Security Architecture

### 5.1 Go Struct in `internal/config/config.go`
```go
// Shinobi holds connection parameters for the Shinobi NVR REST API.
type Shinobi struct {
	APIURL   string `yaml:"api_url"`   // Base URL e.g. "http://127.0.0.1:8080"
	APIKey   string `yaml:"api_key"`   // 30-character API key
	GroupKey string `yaml:"group_key"` // Shinobi Group Key (ke)
}

type Config struct {
	Server      Server   `yaml:"server"`
	CamerasFile string   `yaml:"cameras_file"`
	Defaults    Defaults `yaml:"defaults"`
	Shinobi     Shinobi  `yaml:"shinobi"`
}
```

### 5.2 Zero Hardcoding Security Guarantee
- **No Password in Go Source**: The Go client (`internal/shinobi`) communicates solely via the REST API using `APIKey` and `GroupKey`. It never accepts or contains Super Admin passwords or user plain-text credentials.
- **Ansible Separation**: Passwords reside exclusively in Ansible variable files on the controller.
- **Local Isolation**: The generated API key is restricted to IP `127.0.0.1`, preventing unauthorized remote access even if the API key were intercepted.

---

## 6. Synthesis & Verification Summary

| Item | Status | Evidence / Verification Method |
|---|---|---|
| SSH access to controller `172.16.5.180` | Confirmed | `ssh root@172.16.5.180` succeeded |
| Ansible role & playbook discovered | Confirmed | `/build/armbian-build/ansible/playbook/roles/app_ksp_bida/` |
| Shinobi user login API verified | Confirmed | `POST /?json=true` returned `auth_token` and `ke: pymid463` on `inut_204_63` |
| Super Admin auth flow verified | Confirmed | `POST /super/?json=true` with md5 hash in `/home/Shinobi/super.json` |
| API Key creation & permissions verified | Confirmed | `POST /:auth/api/:ke/add` returned 30-char code with IP `127.0.0.1` |
| Go configuration struct designed | Confirmed | `Shinobi` struct with `api_url`, `api_key`, `group_key` in `internal/config/` |
| Zero hardcoding in Go verified | Confirmed | Only connection endpoints & API key stored in `config.yaml` |

