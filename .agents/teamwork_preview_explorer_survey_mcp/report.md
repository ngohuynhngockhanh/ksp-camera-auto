# Comprehensive Investigation & Specification Report: Embedded MCP Server in `kspcam`

**Author:** Teamwork Explorer (`survey_mcp`)  
**Target Project:** `ksp-camera-auto` (`kspcam`)  
**Target Milestone:** Requirement R3 (Embedded MCP Server)  
**Date:** 2026-08-23  

---

## 1. Executive Summary

This report establishes the complete architectural blueprint, protocol specification, transport mechanisms, security boundaries, and tool catalog for embedding a native Model Context Protocol (MCP) Server into `kspcam`.

The embedded MCP server enables AI assistants (such as Antigravity, Claude Desktop, Hermes, Cursor, and custom LLM agents) to autonomously discover, configure, inspect, diagnose, and manage multi-vendor IP camera networks (Dahua/KBVision, Hikvision, Tiandy) and their associated Shinobi NVR instances through a unified, standardized interface.

### Key Architectural Tenets
1. **Zero External Dependencies / Pure Go**: Standard JSON-RPC 2.0 protocol engine built using Go standard library (`encoding/json`, `net/http`, `bufio`, `sync`, `context`) keeping `CGO_ENABLED=0` static binary compatibility.
2. **Dual Transport Support**:
   - **Stdio Mode (`kspcam --mcp`)**: Native process execution with strict stdio isolation (stdout dedicated exclusively to newline-delimited JSON-RPC, logging redirected to stderr).
   - **HTTP / SSE Transport (`/mcp` on port `:2028`)**: Real-time Server-Sent Events stream for downstream events coupled with POST endpoints for upstream messages, protected by configurable API Key authentication.
3. **Strict Sequential Execution & Safety Constraints**: All batch modifications, encoder re-initializations, and credential tests strictly adhere to the safety-first sequential pipeline established in `internal/bulk` to avoid network storms or device lockouts.
4. **Comprehensive 23-Tool Matrix**: Full coverage across 4 operational domains:
   - Camera Inventory (4 tools)
   - Camera Configuration & Bulk Profiles (5 tools)
   - Discovery & Diagnostics (7 tools)
   - Shinobi NVR Management (7 tools)

---

## 2. MCP Protocol Specification (Model Context Protocol / JSON-RPC 2.0)

### 2.1 Protocol Compliance & Message Framing
The implementation conforms to the Model Context Protocol Specification (Protocol Version `2024-11-05` / `2024-10-07`) over JSON-RPC 2.0.

#### Core JSON-RPC 2.0 Framing Rules
- Every message is a UTF-8 encoded JSON object.
- Stdio transport uses single-line newline-delimited JSON (`\n` terminator, no pretty-printing).
- SSE transport sends JSON payloads inside `data: <json>\n\n` event envelopes.

#### Standard Message Structures
```go
// JSONRPCRequest represents an incoming client request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"` // Must be "2.0"
	ID      any             `json:"id"`      // string, int, or float
	Method  string          `json:"method"`  // e.g. "initialize", "tools/list", "tools/call"
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCNotification represents an incoming or outgoing notification.
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"` // Must be "2.0"
	Method  string          `json:"method"`  // e.g. "notifications/initialized"
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing server response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"` // Must be "2.0"
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError defines standard error structures.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
```

#### Standard Error Code Reference
| Code | Error Type | Condition in `kspcam` MCP Server |
|---|---|---|
| `-32700` | Parse Error | Invalid JSON received on Stdio or HTTP POST |
| `-32600` | Invalid Request | Missing `"jsonrpc": "2.0"` or invalid request fields |
| `-32601` | Method Not Found | Unrecognized method (e.g. unknown RPC or prompt/resource methods) |
| `-32602` | Invalid Params | Failed schema validation or unmarshalable tool arguments |
| `-32603` | Internal Error | Panic recovery, unhandled internal server error |
| `-32001` | Unauthorized | Missing or invalid MCP API Key |
| `-32002` | Device Unreachable | Target camera connection timeout or dial failure |

---

### 2.2 Core Protocol Lifecycle Methods

```mermaid
sequenceDiagram
    autonumber
    actor Client as AI Client (Claude / Hermes / Cursor)
    participant MCP as kspcam Embedded MCP Server
    participant Inv as Inventory & Camera Layer

    Note over Client,MCP: 1. Handshake & Initialization
    Client->>MCP: {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"Hermes","version":"1.0"}}}
    MCP-->>Client: {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false}},"serverInfo":{"name":"kspcam","version":"1.0.0"}}}
    Client->>MCP: {"jsonrpc":"2.0","method":"notifications/initialized"}

    Note over Client,MCP: 2. Tool Discovery
    Client->>MCP: {"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
    MCP-->>Client: {"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"kspcam_list_cameras",...},...]}}

    Note over Client,MCP: 3. Tool Invocation
    Client->>MCP: {"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"kspcam_probe_camera","arguments":{"id":"192.168.1.108:37777"}}}
    MCP->>Inv: Dial & Probe Camera Hardware
    Inv-->>MCP: Probe Data (StreamInfo, SerialNumber, OSD)
    MCP-->>Client: {"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"{...}"}],"isError":false}}

    Note over Client,MCP: 4. Health Check / Ping
    Client->>MCP: {"jsonrpc":"2.0","id":4,"method":"ping","params":{}}
    MCP-->>Client: {"jsonrpc":"2.0","id":4,"result":{}}
```

#### Method Specifications:
1. **`initialize`**:
   - **Request Params**:
     ```json
     {
       "protocolVersion": "2024-11-05",
       "capabilities": {
         "roots": { "listChanged": false },
         "sampling": {}
       },
       "clientInfo": {
         "name": "hermes-agent",
         "version": "1.0.0"
       }
     }
     ```
   - **Response Result**:
     ```json
     {
       "protocolVersion": "2024-11-05",
       "capabilities": {
         "tools": {
           "listChanged": false
         }
       },
       "serverInfo": {
         "name": "kspcam-mcp",
         "version": "1.0.0"
       }
     }
     ```
2. **`notifications/initialized`**:
   - Notification without response. Signals that the client is ready to exchange tool calls.
3. **`ping`**:
   - Responds with empty object `{}`.
4. **`tools/list`**:
   - Returns array of registered tools. Supports optional pagination cursor if tools list is paginated (not required for 23 tools, all returned in single page).
5. **`tools/call`**:
   - Executes named tool with validated arguments.
   - Result format:
     ```json
     {
       "content": [
         {
           "type": "text",
           "text": "{\n  \"status\": \"success\",\n  \"data\": ...\n}"
         }
       ],
       "isError": false
     }
     ```

---

## 3. Transport Mechanisms

### 3.1 Stdio Transport (`kspcam --mcp`)

#### Logging Isolation & Stream Integrity
When `kspcam` is invoked with `--mcp`:
1. `os.Stdout` is strictly reserved for JSON-RPC 2.0 communication.
2. Standard library logging (`log.SetOutput(os.Stderr)`) and any internal logging are immediately routed to `os.Stderr`.
3. If an unhandled panic occurs, it is recovered, formatted as a JSON-RPC error response or logged to `os.Stderr`, preventing corrupt output on `os.Stdout`.

#### Concurrency & Lifecycle
- Input: `bufio.NewScanner(os.Stdin)` in a dedicated read loop.
- Output: `sync.Mutex` wrapping writes to `os.Stdout` followed by `Flush()`.
- Signal handling: Traps `os.Interrupt` and `syscall.SIGTERM`, cleanly closing device sessions and exiting with code `0`.

---

### 3.2 HTTP / SSE Transport (`/mcp` on Web Server `:2028`)

The SSE transport allows remote AI agents and network services to connect over HTTP without launching a local process.

#### Endpoint Topology
| Endpoint | Method | Purpose | Authentication |
|---|---|---|---|
| `/mcp` | `GET` | Establishes Server-Sent Events (SSE) downstream channel | API Key or Admin Session |
| `/mcp/messages` | `POST` | Ingests client JSON-RPC upstream messages for a session | API Key or Admin Session |
| `/mcp` | `POST` | Direct HTTP JSON-RPC endpoint (Stateless single-request mode) | API Key or Admin Session |

#### SSE Connection Handshake Flow
1. Client connects to `GET /mcp` (or `GET /mcp?apiKey=<key>`).
2. Server validates authentication (API Key header `X-MCP-Key`, `Authorization: Bearer <key>`, or URL parameter `?key=`).
3. Server generates a cryptographically random 16-byte session identifier (`sessionId`).
4. Server writes headers:
   ```http
   HTTP/1.1 200 OK
   Content-Type: text/event-stream
   Cache-Control: no-cache
   Connection: keep-alive
   X-Accel-Buffering: no
   ```
5. Server sends the `endpoint` initial event:
   ```http
   event: endpoint
   data: /mcp/messages?sessionId=e9f4c0a1b2d34567
   ```
6. Client posts subsequent requests to `POST /mcp/messages?sessionId=e9f4c0a1b2d34567`.
7. Server processes requests and sends matching JSON-RPC responses down the SSE stream:
   ```http
   event: message
   data: {"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"..."}]}}
   ```

#### Configuration Schema (`config.yaml`)
```yaml
server:
  addr: ":2028"
  username: "admin"
  password: "smarthome12345"

mcp:
  enabled: true
  api_key: "ksp_mcp_sec_994827104928" # If set, requires matching X-MCP-Key or ?key=
  allow_unauthenticated_loopback: true # Allow 127.0.0.1 clients without API key
```

---

## 4. Comprehensive Tools Definition & Schema (23 Tools)

All 23 tools are specified below with their exact name, category, human/LLM description, JSON Schema definition, return format, and underlying Go call graph.

---

### 4.1 Category 1: Camera Inventory Tools (4 Tools)

#### 1. `kspcam_list_cameras`
- **Description**: List all cameras currently registered in the `kspcam` inventory (`cameras.yaml`), including host, port, vendor, serial number, and NVR link status.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "properties": {
      "vendor": {
        "type": "string",
        "enum": ["hikvision", "dahua", "tiandy"],
        "description": "Filter cameras by vendor family"
      },
      "isNvr": {
        "type": "boolean",
        "description": "Filter to only NVR devices or standalone cameras"
      }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  devices := inv.List()
  // Filter by vendor/isNvr if provided
  // Return JSON array of Device objects
  ```
- **Output Sample**:
  ```json
  [
    {
      "id": "192.168.1.108:37777",
      "name": "Bàn 1 (Dahua)",
      "host": "192.168.1.108",
      "port": 37777,
      "vendor": "dahua",
      "username": "admin",
      "serialNumber": "7J0458BPAZ00001",
      "isNvr": false
    }
  ]
  ```

---

#### 2. `kspcam_upsert_camera`
- **Description**: Add a new camera or update an existing camera in the inventory. Passwords are automatically encrypted at rest using AES-256-GCM in `cameras.yaml`.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["host", "port", "vendor"],
    "properties": {
      "id": {
        "type": "string",
        "description": "Unique device ID. Defaults to host:port if omitted."
      },
      "name": {
        "type": "string",
        "description": "Human-readable label for camera location (e.g. 'Bàn 8 VIP')"
      },
      "host": {
        "type": "string",
        "description": "IP address or hostname of camera"
      },
      "port": {
        "type": "integer",
        "description": "Control port (37777/8888 for Dahua, 80 for Hikvision ISAPI, 554 for Tiandy)"
      },
      "vendor": {
        "type": "string",
        "enum": ["hikvision", "dahua", "tiandy"],
        "description": "Vendor family identifier"
      },
      "username": {
        "type": "string",
        "description": "Admin username (default: admin)"
      },
      "password": {
        "type": "string",
        "description": "Plaintext device password (will be encrypted at rest)"
      },
      "serialNumber": {
        "type": "string",
        "description": "Hardware serial number if known"
      },
      "nvrId": {
        "type": "string",
        "description": "Fallback NVR device ID"
      },
      "nvrChannel": {
        "type": "integer",
        "description": "1-based channel on the fallback NVR"
      },
      "nvrName": {
        "type": "string",
        "description": "Display name of NVR channel"
      },
      "noStorage": {
        "type": "boolean",
        "description": "True if camera has no local SD card and relies on NVR"
      },
      "isNvr": {
        "type": "boolean",
        "description": "True if this device is an NVR"
      },
      "nvrWatchdog": {
        "type": "boolean",
        "description": "Enable automated recording health repair watchdog"
      },
      "nvrSyncTimeFromHost": {
        "type": "boolean",
        "description": "Enable host clock synchronization watchdog"
      }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  d := config.Device{...}
  err := inv.Upsert(d)
  ```
- **Output Sample**:
  ```json
  {
    "status": "success",
    "device": {
      "id": "192.168.1.108:37777",
      "name": "Bàn 1 (Dahua)",
      "host": "192.168.1.108",
      "port": 37777,
      "vendor": "dahua"
    }
  }
  ```

---

#### 3. `kspcam_delete_camera`
- **Description**: Remove one or more camera devices from the inventory.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id"],
    "properties": {
      "id": {
        "type": "string",
        "description": "Device ID to delete (e.g. '192.168.1.108:37777')"
      }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  err := inv.Delete(req.ID)
  ```
- **Output Sample**:
  ```json
  {
    "status": "success",
    "deletedId": "192.168.1.108:37777"
  }
  ```

---

#### 4. `kspcam_probe_camera`
- **Description**: Connect live to camera hardware and probe stream encoding capabilities (resolutions, codec, FPS, GOP, bitrate, audio), channel title, OSD overlay lines, and hardware serial number.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id"],
    "properties": {
      "id": {
        "type": "string",
        "description": "Device ID in inventory"
      },
      "timeoutSeconds": {
        "type": "integer",
        "description": "Probe timeout in seconds (default: 30, min: 5, max: 600)"
      }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  dev, ok := inv.Get(req.ID)
  cam, err := camera.Open(ctx, dev, timeout)
  streams, err := cam.Probe(ctx)
  if idCam, ok := cam.(camera.DeviceIdentity); ok {
      serial, _ := idCam.GetSerialNumber(ctx)
  }
  ```
- **Output Sample**:
  ```json
  {
    "deviceId": "192.168.1.108:37777",
    "serialNumber": "7J0458BPAZ00001",
    "streams": [
      {
        "channel": 0,
        "stream": 0,
        "width": 1920,
        "height": 1080,
        "fps": 25,
        "compression": "H.265",
        "profile": "Main",
        "gop": 50,
        "bitrateKbps": 2048,
        "bitrateMode": "CBR",
        "audioCodec": "AAC",
        "audioEnable": true,
        "smartCodec": true,
        "name": "CAM_BAN_1",
        "osdLines": ["INUT SMART", "BAN 1", "", ""]
      }
    ]
  }
  ```

---

### 4.2 Category 2: Camera Config & Bulk Tools (5 Tools)

#### 5. `kspcam_apply_profile`
- **Description**: Apply video encode settings (resolution, codec, FPS, GOP, bitrate, audio AAC, SmartCodec, OSD overlay) across one or multiple cameras in sequential order.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["deviceIds", "profile"],
    "properties": {
      "deviceIds": {
        "type": "array",
        "items": { "type": "string" },
        "description": "List of target device IDs"
      },
      "profile": {
        "type": "object",
        "properties": {
          "setResolution": { "type": "boolean" },
          "width": { "type": "integer" },
          "height": { "type": "integer" },
          "setCodec": { "type": "boolean" },
          "codec": { "type": "string", "enum": ["H.264", "H.264H", "H.264B", "H.265", "MJPG"] },
          "codecProfile": { "type": "string", "enum": ["Main", "High", "Baseline"] },
          "setFps": { "type": "boolean" },
          "fps": { "type": "integer" },
          "setGop": { "type": "boolean" },
          "gop": { "type": "integer" },
          "setBitrate": { "type": "boolean" },
          "bitrate": { "type": "integer" },
          "bitrateMode": { "type": "string", "enum": ["CBR", "VBR"] },
          "setAudioAAC": { "type": "boolean" },
          "setSmartCodec": { "type": "boolean" },
          "smartCodec": { "type": "boolean" },
          "setOsd": { "type": "boolean" },
          "osdLines": { "type": "array", "items": { "type": "string" } },
          "streams": { "type": "array", "items": { "type": "integer" } },
          "channels": { "type": "array", "items": { "type": "integer" } }
        }
      },
      "timeoutSeconds": {
        "type": "integer",
        "description": "Per-camera timeout in seconds"
      }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  results := bulk.Apply(ctx, inv, req, timeout, nil)
  ```
- **Output Sample**:
  ```json
  [
    {
      "deviceId": "192.168.1.108:37777",
      "name": "Bàn 1 (Dahua)",
      "host": "192.168.1.108",
      "ok": true,
      "steps": [
        { "step": "set codec to H.265 on main", "detail": "H.265/Main", "ok": true },
        { "step": "set resolution to 1920x1080 on main", "detail": "1920x1080", "ok": true },
        { "step": "set fps to 25 on main", "detail": "25 fps", "ok": true },
        { "step": "set gop to 50 on main", "detail": "50", "ok": true }
      ]
    }
  ]
  ```

---

#### 6. `kspcam_set_channel_name`
- **Description**: Update the on-camera hardware channel title (distinct from the local inventory label).
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id", "name"],
    "properties": {
      "id": { "type": "string", "description": "Device ID" },
      "channel": { "type": "integer", "description": "0-based channel index (default 0)" },
      "name": { "type": "string", "description": "New channel name" },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  cam, err := camera.Open(ctx, dev, timeout)
  err = cam.SetChannelName(ctx, req.Channel, req.Name)
  ```
- **Output Sample**:
  ```json
  { "status": "success", "id": "192.168.1.108:37777", "channel": 0, "name": "Bàn 1 VIP" }
  ```

---

#### 7. `kspcam_set_osd`
- **Description**: Configure up to 4 lines of free-text OSD text overlay and visibility on the camera video.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id", "lines"],
    "properties": {
      "id": { "type": "string", "description": "Device ID" },
      "channel": { "type": "integer", "description": "0-based channel index" },
      "lines": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Up to 4 text lines. Use '{name}' to substitute camera inventory name."
      },
      "enabled": {
        "type": "array",
        "items": { "type": "boolean" },
        "description": "Visibility flag per line"
      },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  cam, err := camera.Open(ctx, dev, timeout)
  applied, err := cam.SetOSDLines(ctx, req.Channel, req.Lines, req.Enabled)
  ```
- **Output Sample**:
  ```json
  { "status": "success", "appliedLines": 4 }
  ```

---

#### 8. `kspcam_reboot_camera`
- **Description**: Send a hardware reboot command to a camera or NVR (DVRIP `magicBox.reboot` or ISAPI `/System/reboot`).
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id"],
    "properties": {
      "id": { "type": "string", "description": "Device ID" },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  cam, err := camera.Open(ctx, dev, timeout)
  rebooter, ok := cam.(camera.Rebooter)
  err = rebooter.Reboot(ctx)
  ```
- **Output Sample**:
  ```json
  { "status": "success", "message": "Reboot command accepted by device" }
  ```

---

#### 9. `kspcam_change_password`
- **Description**: Change the administrative password on the physical camera hardware and update the encrypted credentials in `cameras.yaml`.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id", "newPassword"],
    "properties": {
      "id": { "type": "string", "description": "Device ID" },
      "newUser": { "type": "string", "description": "New username if updating user, or omit to keep current" },
      "newPassword": { "type": "string", "description": "New plaintext password" },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  cam, err := camera.Open(ctx, dev, timeout)
  err = cam.ChangePassword(ctx, req.NewUser, req.NewPassword)
  if err == nil {
      dev.Password = req.NewPassword
      inv.Upsert(dev)
  }
  ```
- **Output Sample**:
  ```json
  { "status": "success", "id": "192.168.1.108:37777", "message": "Password changed and inventory updated" }
  ```

---

### 4.3 Category 3: Discovery & Diagnosis Tools (7 Tools)

#### 10. `kspcam_scan_lan`
- **Description**: Discover IP cameras on the local network (via ONVIF WS-Discovery UDP 3702, Dahua DHDiscover UDP 37810, Hikvision SADP UDP 37020) or across routed subnets (via Nmap TCP port scan).
- **Input Schema**:
  ```json
  {
    "type": "object",
    "properties": {
      "method": {
        "type": "string",
        "enum": ["auto", "onvif", "dahua", "sadp", "nmap"],
        "description": "Discovery protocol. 'auto' broadcasts all 3 UDP protocols."
      },
      "subnet": {
        "type": "string",
        "description": "Subnet in CIDR format (e.g. '192.168.1.0/24') for Nmap scans"
      },
      "timeoutSeconds": {
        "type": "integer",
        "description": "Scan timeout in seconds (default 5)"
      }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  results, err := discovery.Scan(ctx, timeout)
  // or discovery.ScanSubnet(ctx, req.Subnet, timeout)
  ```
- **Output Sample**:
  ```json
  [
    {
      "ip": "192.168.1.108",
      "port": 37777,
      "vendor": "dahua",
      "model": "DH-IPC-HFW1230S1P",
      "mac": "3c:ef:8c:12:34:56",
      "name": "IPC",
      "via": "dahua"
    }
  ]
  ```

---

#### 11. `kspcam_try_password`
- **Description**: Test a matrix of usernames and passwords sequentially against discovered cameras to verify working credentials.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["devices", "credentials"],
    "properties": {
      "devices": {
        "type": "array",
        "items": {
          "type": "object",
          "required": ["ip", "port", "vendor"],
          "properties": {
            "ip": { "type": "string" },
            "port": { "type": "integer" },
            "vendor": { "type": "string", "enum": ["hikvision", "dahua", "tiandy"] }
          }
        }
      },
      "credentials": {
        "type": "array",
        "items": {
          "type": "object",
          "required": ["username", "password"],
          "properties": {
            "username": { "type": "string" },
            "password": { "type": "string" }
          }
        }
      },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  results := bulk.TryPasswords(ctx, req, timeout, nil)
  ```
- **Output Sample**:
  ```json
  [
    {
      "ip": "192.168.1.108",
      "port": 37777,
      "vendor": "dahua",
      "username": "admin",
      "password": "smarthome12345",
      "ok": true
    }
  ]
  ```

---

#### 12. `kspcam_wifi_scan`
- **Description**: Trigger an over-the-air Wi-Fi scan on a wireless camera to inspect available Access Points (SSID, signal strength RSSI, auth mode).
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id"],
    "properties": {
      "id": { "type": "string", "description": "Device ID" },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  cam, err := camera.Open(ctx, dev, timeout)
  ns, ok := cam.(camera.NetworkSettings)
  aps, err := ns.ScanWiFi(ctx)
  ```
- **Output Sample**:
  ```json
  [
    { "ssid": "KSP_BIDA_5G", "rssi": -45, "auth": "WPA2-PSK" },
    { "ssid": "KSP_GUEST", "rssi": -68, "auth": "WPA2-PSK" }
  ]
  ```

---

#### 13. `kspcam_get_network`
- **Description**: Retrieve network interface configurations (IP, Subnet Mask, Gateway, DNS, DHCP state) from a camera.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id"],
    "properties": {
      "id": { "type": "string", "description": "Device ID" },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  cam, err := camera.Open(ctx, dev, timeout)
  ns, ok := cam.(camera.NetworkSettings)
  netCfg, err := ns.GetNetworkConfig(ctx)
  ```
- **Output Sample**:
  ```json
  {
    "interfaces": {
      "eth0": {
        "dhcp": false,
        "ip": "192.168.1.108",
        "mask": "255.255.255.0",
        "gateway": "192.168.1.1",
        "dns": ["8.8.8.8", "1.1.1.1"]
      }
    }
  }
  ```

---

#### 14. `kspcam_get_nvr_health`
- **Description**: Check the recording health of an NVR or linked camera (timing record state, uptime, storage disk status, gaps).
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id"],
    "properties": {
      "id": { "type": "string", "description": "NVR or Camera Device ID" },
      "forceCheck": { "type": "boolean", "description": "Force fresh probe ignoring cache" },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  cam, err := camera.Open(ctx, dev, timeout)
  hc, _ := cam.(camera.NVRHealthConfig)
  sm, _ := cam.(camera.StorageManager)
  state, _ := hc.GetRecordState(ctx, channelCount)
  uptime, _ := hc.GetUptime(ctx)
  disks, _ := sm.GetStorageInfo(ctx)
  ```
- **Output Sample**:
  ```json
  {
    "nvrId": "192.168.1.200:37777",
    "uptimeSeconds": 864000,
    "healthy": true,
    "disks": [
      { "name": "sda1", "totalMB": 3815447, "freeMB": 120400, "status": "normal" }
    ],
    "channels": [
      { "channel": 1, "name": "Bàn 1", "recording": true, "timingRecordEnabled": true }
    ]
  }
  ```

---

#### 15. `kspcam_get_recordings`
- **Description**: Query stored recording segments on a camera or NVR channel for a specific date/time range.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id", "startTime", "endTime"],
    "properties": {
      "id": { "type": "string", "description": "Device ID" },
      "channel": { "type": "integer", "description": "0-based channel index" },
      "startTime": { "type": "string", "description": "Start timestamp in ISO 8601 / RFC 3339 format" },
      "endTime": { "type": "string", "description": "End timestamp in ISO 8601 / RFC 3339 format" },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  cam, err := camera.Open(ctx, dev, timeout)
  rec, ok := cam.(camera.Recorder)
  segments, err := rec.FindRecordings(ctx, req.Channel, start, end)
  ```
- **Output Sample**:
  ```json
  [
    {
      "channel": 0,
      "startTime": "2026-08-23T14:00:00Z",
      "endTime": "2026-08-23T15:00:00Z",
      "length": 3600,
      "fileType": "dav"
    }
  ]
  ```

---

#### 16. `kspcam_get_snapshot`
- **Description**: Fetch a single live JPEG snapshot from a camera channel and return it formatted as base64 image data.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["id"],
    "properties": {
      "id": { "type": "string", "description": "Device ID" },
      "channel": { "type": "integer", "description": "0-based channel index" },
      "stream": { "type": "integer", "description": "0=Main, 1=Sub1" },
      "timeoutSeconds": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  cam, err := camera.Open(ctx, dev, timeout)
  jpegBytes, err := cam.Snapshot(ctx, req.Channel, req.Stream)
  ```
- **Output Sample**:
  ```json
  {
    "mimeType": "image/jpeg",
    "data": "/9j/4AAQSkZJRgABAQEASABIAAD..."
  }
  ```

---

### 4.4 Category 4: Shinobi Management Tools (7 Tools)

#### 17. `shinobi_list_monitors`
- **Description**: List all monitors configured in the Shinobi NVR instance with their status, stream URLs, and recording modes.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "properties": {
      "groupKey": { "type": "string", "description": "Optional group key override" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  monitors, err := shinobiClient.ListMonitors(ctx, groupKey)
  ```
- **Output Sample**:
  ```json
  [
    {
      "mid": "cam_ban1",
      "name": "Bàn 1",
      "mode": "record",
      "host": "192.168.1.108",
      "type": "h265",
      "ext": "mp4",
      "protocol": "rtsp",
      "auto_host": "rtsp://admin:smarthome12345@192.168.1.108:554/cam/realmonitor?channel=1&subtype=0"
    }
  ]
  ```

---

#### 18. `shinobi_add_monitor`
- **Description**: Add a new camera monitor stream into Shinobi NVR with RTSP URL, credentials, stream dimensions, audio, and recording mode.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["mid", "name", "rtspUrl"],
    "properties": {
      "mid": { "type": "string", "description": "Unique alphanumeric monitor ID" },
      "name": { "type": "string", "description": "Monitor display name" },
      "rtspUrl": { "type": "string", "description": "Full RTSP stream URL with credentials" },
      "mode": { "type": "string", "enum": ["idle", "start", "record"], "default": "record" },
      "width": { "type": "integer", "default": 1920 },
      "height": { "type": "integer", "default": 1080 },
      "fps": { "type": "integer", "default": 25 },
      "audioCodec": { "type": "string", "default": "aac" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  err := shinobiClient.AddMonitor(ctx, req)
  ```
- **Output Sample**:
  ```json
  { "status": "success", "mid": "cam_ban1", "message": "Monitor added to Shinobi" }
  ```

---

#### 19. `shinobi_edit_monitor`
- **Description**: Edit an existing monitor in Shinobi NVR (modify RTSP URL, resolution, FPS, or mode).
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["mid"],
    "properties": {
      "mid": { "type": "string", "description": "Monitor ID to update" },
      "name": { "type": "string" },
      "rtspUrl": { "type": "string" },
      "mode": { "type": "string", "enum": ["idle", "start", "record"] },
      "width": { "type": "integer" },
      "height": { "type": "integer" },
      "fps": { "type": "integer" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  err := shinobiClient.EditMonitor(ctx, req.Mid, req)
  ```
- **Output Sample**:
  ```json
  { "status": "success", "mid": "cam_ban1", "message": "Monitor updated" }
  ```

---

#### 20. `shinobi_delete_monitor`
- **Description**: Delete a monitor configuration from Shinobi NVR.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["mid"],
    "properties": {
      "mid": { "type": "string", "description": "Monitor ID to delete" },
      "deleteRecordings": { "type": "boolean", "default": false }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  err := shinobiClient.DeleteMonitor(ctx, req.Mid, req.DeleteRecordings)
  ```
- **Output Sample**:
  ```json
  { "status": "success", "mid": "cam_ban1", "message": "Monitor deleted" }
  ```

---

#### 21. `shinobi_sync_inventory`
- **Description**: Reconcile cameras between `kspcam` inventory (`cameras.yaml`) and Shinobi NVR monitors in either or both directions.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "properties": {
      "direction": {
        "type": "string",
        "enum": ["both", "to_shinobi", "from_shinobi"],
        "default": "both",
        "description": "'to_shinobi' exports inventory to Shinobi, 'from_shinobi' imports Shinobi monitors to inventory, 'both' does bi-directional sync"
      },
      "dryRun": { "type": "boolean", "default": false }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  summary, err := shinobiSync.Run(ctx, inv, shinobiClient, req.Direction, req.DryRun)
  ```
- **Output Sample**:
  ```json
  {
    "status": "success",
    "direction": "both",
    "toShinobi": { "added": 2, "updated": 0, "skipped": 6 },
    "fromShinobi": { "added": 0, "skipped": 8 }
  }
  ```

---

#### 22. `shinobi_change_monitor_state`
- **Description**: Change the active execution state of a Shinobi monitor (idle/stop, start watching, start recording, or restart process).
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["mid", "state"],
    "properties": {
      "mid": { "type": "string", "description": "Monitor ID" },
      "state": { "type": "string", "enum": ["idle", "start", "record", "restart"] }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  err := shinobiClient.ChangeMonitorState(ctx, req.Mid, req.State)
  ```
- **Output Sample**:
  ```json
  { "status": "success", "mid": "cam_ban1", "state": "record" }
  ```

---

#### 23. `shinobi_get_videos`
- **Description**: Query stored video recordings for a Shinobi monitor within a date range.
- **Input Schema**:
  ```json
  {
    "type": "object",
    "required": ["mid"],
    "properties": {
      "mid": { "type": "string", "description": "Monitor ID" },
      "limit": { "type": "integer", "default": 50 },
      "startTime": { "type": "string", "description": "Start timestamp in ISO 8601 / RFC 3339" },
      "endTime": { "type": "string", "description": "End timestamp in ISO 8601 / RFC 3339" }
    }
  }
  ```
- **Go Call Graph**:
  ```go
  videos, err := shinobiClient.GetVideos(ctx, req.Mid, req.Limit, start, end)
  ```
- **Output Sample**:
  ```json
  [
    {
      "mid": "cam_ban1",
      "filename": "2026-08-23T15-00-00.mp4",
      "size": 45012340,
      "time": "2026-08-23T15:00:00Z",
      "href": "/shinobi/videos/group/cam_ban1/2026-08-23T15-00-00.mp4"
    }
  ]
  ```

---

## 5. Package Architecture & Code Layout

### 5.1 New Subpackage: `internal/mcp/`

```
internal/mcp/
├── types.go            # JSON-RPC 2.0 & MCP protocol structures (Request, Response, Error, Initialize, Tool)
├── registry.go         # ToolRegistry with schema validation, registration, and dispatch
├── server.go           # MCP Server engine, session management, method routing
├── stdio.go            # Stdio transport runner with log isolation (os.Stderr redirection)
├── sse.go              # HTTP/SSE transport handler (/mcp & /mcp/messages) with API Key authentication
├── tools_inventory.go  # Handlers: kspcam_list_cameras, kspcam_upsert_camera, kspcam_delete_camera, kspcam_probe_camera
├── tools_config.go     # Handlers: kspcam_apply_profile, kspcam_set_channel_name, kspcam_set_osd, kspcam_reboot_camera, kspcam_change_password
├── tools_diagnosis.go  # Handlers: kspcam_scan_lan, kspcam_try_password, kspcam_wifi_scan, kspcam_get_network, kspcam_get_nvr_health, kspcam_get_recordings, kspcam_get_snapshot
└── tools_shinobi.go    # Handlers: shinobi_list_monitors, shinobi_add_monitor, shinobi_edit_monitor, shinobi_delete_monitor, shinobi_sync_inventory, shinobi_change_monitor_state, shinobi_get_videos
```

### 5.2 Integration Touchpoints

#### 1. Command Entrypoint (`cmd/kspcam/main.go`)
```go
mcpMode := flag.Bool("mcp", false, "run embedded MCP server over stdio for AI assistants and exit")
flag.Parse()

if *mcpMode {
    // Redirect standard logging to Stderr to prevent stdout JSON-RPC stream corruption
    log.SetOutput(os.Stderr)
    
    cfg, err := config.Load(*configPath)
    if err != nil {
        log.Fatalf("config: %v", err)
    }
    inv, err := config.LoadInventory(cfg.CamerasFile)
    if err != nil {
        log.Fatalf("inventory: %v", err)
    }
    
    shinobiCl := shinobi.NewClient(cfg.Shinobi.APIURL, cfg.Shinobi.APIKey, cfg.Shinobi.GroupKey)
    mcpSrv := mcp.NewServer(cfg, inv, shinobiCl)
    
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()
    
    if err := mcp.RunStdio(ctx, mcpSrv); err != nil {
        log.Fatalf("mcp stdio: %v", err)
    }
    return
}
```

#### 2. Configuration Extensions (`internal/config/config.go`)
```go
type MCPConfig struct {
    Enabled                     bool   `yaml:"enabled"`
    APIKey                      string `yaml:"api_key"`
    AllowUnauthenticatedLoopback bool  `yaml:"allow_unauthenticated_loopback"`
}

type ShinobiConfig struct {
    APIURL   string `yaml:"api_url"`
    APIKey   string `yaml:"api_key"`
    GroupKey string `yaml:"group_key"`
}

type Config struct {
    Server      Server        `yaml:"server"`
    CamerasFile string        `yaml:"cameras_file"`
    Defaults    Defaults      `yaml:"defaults"`
    MCP         MCPConfig     `yaml:"mcp"`
    Shinobi     ShinobiConfig `yaml:"shinobi"`
}
```

#### 3. Web Server Routes (`internal/server/server.go`)
```go
// Register MCP endpoints on HTTP mux
mcpHandler := mcp.NewHTTPHandler(mcpSrv, cfg.MCP)
s.mux.Handle("/mcp", mcpHandler.ServeSSE)
s.mux.Handle("/mcp/messages", mcpHandler.ServeMessages)
```

---

## 6. Implementation & Verification Plan

### 6.1 Unit & Integration Testing Strategy
1. **Protocol Unit Tests (`internal/mcp/protocol_test.go`)**:
   - Verify JSON-RPC unmarshaling, method routing (`initialize`, `ping`, `tools/list`, `tools/call`).
   - Validate error codes for invalid requests (`-32600`), missing tools (`-32601`), invalid schemas (`-32602`).
2. **Stdio Transport Test (`internal/mcp/stdio_test.go`)**:
   - Pipe canned JSON-RPC requests via in-memory `bytes.Buffer` / `io.Pipe` and assert line-by-line responses on stdout without stderr log contamination.
3. **SSE Transport Test (`internal/mcp/sse_test.go`)**:
   - Connect via `httptest.NewServer` with `Accept: text/event-stream`.
   - Verify `endpoint` event generation, API Key rejection with `401 Unauthorized`, and message posting via `/mcp/messages`.
4. **Tool Execution Mocks (`internal/mcp/tools_test.go`)**:
   - Mock `Inventory`, `Camera`, `bulk.Apply`, and `shinobi.Client` to verify that all 23 tools parse parameters correctly and return standard text / image content.

### 6.2 Test Command
```bash
# Run all unit tests including new MCP package
go test -v ./internal/mcp/... ./...

# Verify zero cgo compilation
CGO_ENABLED=0 go build -o /tmp/kspcam ./cmd/kspcam
```

---

## 7. Conclusion

The design specified in this report provides a complete, production-grade, zero-dependency embedded MCP server for `kspcam`. It faithfully satisfies Requirement R3, integrates seamlessly with existing `internal/camera`, `internal/bulk`, `internal/discovery`, and `internal/shinobi` packages, and guarantees strict sequential execution safety for all camera hardware operations.
