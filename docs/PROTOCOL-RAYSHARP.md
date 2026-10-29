# Đặc Tả Giao Thức Raysharp NVR & AI IP Camera (`PROTOCOL-RAYSHARP.md`)

> **Tài liệu kỹ thuật chính thức (Architecture & Protocol Reference)** cho việc tích hợp dòng Đầu ghi hình AI NVR (`N10256N` / `N10256-16HDD` / `RS-SM-2426`) và Camera IP AI (`VC31.35.1.3.6` / `6M019A`) của hãng **Raysharp Technology** (Zhuhai / Zhongshan RaySharp Security Technology Co., Ltd. — ODM cho Advantech, Lorex, Swann, GW Security) vào hệ thống `ksp-camera-auto` (`kspcam`) và `inut-bida-hub`.
>
> **Nguồn bằng chứng (Empirical Evidence):**
> - Phân tích dịch ngược gói ứng dụng Web SPA bằng công cụ **REA MCP (`analyze_javascript_application`, Evidence ID `ev_60b94692be9e3f05b589042100492e5c43ce8cc44b60144f5c8318a0ff822d68`)** trên 16 bundle JavaScript thực tế thu thập từ thiết bị.
> - Kiểm chứng trực tiếp (Live Read-Only Verification) trên phần cứng thực tế tại node `77.88.207.70` (`inut-207-70`):
>   - **Đầu ghi AI NVR 256 kênh**: `192.168.2.101` (`lan2` DHCP) / `192.168.1.234` (`lan1` Static), Model `N10256N` (`N10256-16HDD`), Hardware `2M023`, Firmware `VC1.3.0-260212`, HTTP API `V1.026`, MAC `38:24:F1:15:D0:48` / `38:24:F1:15:D0:16`, P2P ID `9EHTWW5RGGNJL76Z111A`.
>   - **Camera IP AI 3K ($2880 \times 1620$)**: `192.168.2.100`, Model `IP Camera`, Hardware `6M019A`, Firmware `VC31.35.1.3.6-260302`, HTTP API `V1.0`, MAC `38:24:F1:16:41:75`, P2P ID `4H3GSGDS6M7HPYPT111A`.

---

## 1. Tổng Quan Kiến Trúc & Cổng Giao Tiếp (Port & Dual-NIC Architecture)

### 1.1 Nhận Diện Phần Cứng Qua OUI MAC & HTTP Fingerprint
- **Dải MAC OUI**: `38:24:F1` (Raysharp Technology).
- **HTTP Headers đặc trưng**:
  - Mọi yêu cầu `GET /API/*` hoặc `HEAD /` không đúng phương thức sẽ trả về `405 Method Not Allowed` kèm `Content-Type: application/json` và header bảo mật `Content-Security-Policy` đặc trưng của Raysharp Web 5.0 UI.
  - Điểm cuối không cần xác thực `POST /API/Login/Range` luôn trả về `200 OK` với `"result": "success"`, `"http_api_version": "V1.0"` (hoặc `"V1.026"`) và `"site_version": "N10256N"` / `"IP Camera VC31..."`.

### 1.2 Ma Trận Cổng Dịch Vụ (Service Port Matrix)

| Cổng | Giao Thức | Thiết Bị Mở | Chức Năng Kỹ Thuật |
|---|---|---|---|
| `80/tcp` | HTTP JSON API | NVR & IPC | Cổng điều khiển chính cho toàn bộ REST API `/API/<Module>/<SubModule>/<Action>` và giao diện Web SPA. |
| `443/tcp` | HTTPS JSON API | NVR & IPC | Cổng API bảo mật TLS tương đương cổng 80. |
| `554/tcp` | RTSP / RTP | NVR & IPC | Truyền luồng video/audio thời gian thực (`Mainstream`, `Substream`, `Mobilestream`) và RTSP Playback. |
| `19102/tcp` | Raysharp Private Media | NVR (`N10256N`) | Cổng nhị phân nội bộ phục vụ truyền media/control của CMS (`VMS Pro`) và app di động (`RxCamView`). |
| `47423/tcp` | Raysharp Private IPC | IPC (`6M019A`) | Cổng giao thức `Private` nội bộ để đầu ghi NVR kết nối và quản lý camera IP con. |
| `65002/tcp` | Virtual Host Proxy | NVR (`N10256N`) | Cổng proxy HTTP/TCP từ NVR xuyên thẳng vào Camera IP con gắn ở kênh `CH2` (`forward_port: 65002`, `CH<N>` tương ứng `65000 + N`). |

### 1.3 Kiến Trúc 2 Card Mạng Độc Lập Trên Đầu Ghi (`lan1` & `lan2`)
Đầu ghi `N10256N` trang bị 2 giao diện Ethernet tách biệt (cấu hình tại `/API/NetworkConfig/NetBase/Get`):
- **`lan1` (Mạng quản trị / WAN nội bộ)**: Mặc định đặt IP tĩnh (ví dụ `192.168.1.234/24`, MAC `38:24:F1:15:D0:16`).
- **`lan2` (Mạng Camera IP / Switch nội bộ)**: Có thể chạy DHCP hoặc Static trên subnet riêng (ví dụ `192.168.2.101/24`, MAC `38:24:F1:15:D0:48`) để gom lưu lượng băng thông cao từ hàng chục/trăm camera AI mà không làm nghẽn mạng LAN chính.
- *Lưu ý định dạng IP của Raysharp*: Các trường IPv4 trong `/API/NetworkConfig/NetBase/Get` và `/API/ChannelConfig/IPChannel/Get` đôi khi được pad đủ 3 chữ số mỗi octet (ví dụ `"192.168.001.234"`, `"255.255.255.000"`). Khi parse trong Go bắt buộc phải chuẩn hóa bỏ số `0` đứng đầu mỗi octet (`normalizeIPv4`).

---

## 2. Cơ Chế Xác Thực 3 Thành Phần (HTTP Digest + Session Cookie + CSRF Token)

Khác với Hikvision ISAPI (dùng Digest trên từng request đơn lẻ) hay Dahua DVRIP (dùng TCP session ID), Raysharp HTTP API V1.0 yêu cầu kết hợp **3 thành phần bảo mật**:

```mermaid
sequenceDiagram
    autonumber
    participant Client as kspcam (internal/raysharp)
    participant Device as Raysharp NVR / AI IPC (:80)

    Note over Client,Device: Bước 0 (Tùy chọn): Thăm dò thông tin trước đăng nhập
    Client->>Device: POST /API/Login/Range {"version":"1.0","data":{}}
    Device-->>Client: 200 OK {"result":"success","data":{"device_type":"IPC","site_version":"..."}}

    Note over Client,Device: Bước 1: Yêu cầu Challenge Digest
    Client->>Device: POST /API/Web/Login {"data":{"remote_terminal_info":"WEB,chrome"}}
    Device-->>Client: 401 Unauthorized (WWW-Authenticate: Digest realm="device", nonce="...", qop="auth")

    Note over Client,Device: Bước 2: Gửi Digest Response + Nhận Session Cookie & X-csrftoken
    Client->>Device: POST /API/Web/Login (Authorization: Digest ... + {"data":{"remote_terminal_info":"WEB,chrome"}})
    Device-->>Client: 200 OK (Set-Cookie: session_80=<hex>; X-csrftoken: <hex>)

    Note over Client,Device: Bước 3: Gọi các API cấu hình & AI với Cookie + X-csrftoken
    Client->>Device: POST /API/StreamConfig/MainStream/Get (Cookie: session_80=... & X-csrftoken: ...)
    Device-->>Client: 200 OK {"result":"success","data":{"channel_info":{...}}}

    Note over Client,Device: Bước 4: Đóng phiên an toàn (Tránh tràn bảng session thiết bị)
    Client->>Device: POST /API/Web/Logout (Cookie: session_80=... & X-csrftoken: ...)
    Device-->>Client: 200 OK {"version":"1.0","result":"success"}
```

### 2.1 Công Thức Tính Toán HTTP Digest (`module 485` trong `core.js`)
1. Khi gửi `POST /API/Web/Login` không có header `Authorization`, thiết bị trả về HTTP `401` kèm:
   ```http
   WWW-Authenticate: Digest realm="device",nonce="fda214a42b5b3f4bc63b544897117e8f0780127b593987ad70267e096e96260b",qop="auth",userhash="true"
   ```
2. Hàm băm $H$ là `MD5` theo mặc định, hoặc `SHA-256` nếu trong `WWW-Authenticate` có `algorithm="SHA-256"`:
   $$\text{HA1} = \text{hex}\big(H(\text{username} + \text{":"} + \text{realm} + \text{":"} + \text{password})\big)$$
   $$\text{HA2} = \text{hex}\big(H(\text{"POST:/API/Web/Login"})\big)$$
   $$\text{response} = \text{hex}\big(H(\text{HA1} + \text{":"} + \text{nonce} + \text{":"} + \text{nc} + \text{":"} + \text{cnonce} + \text{":"} + \text{qop} + \text{":"} + \text{HA2})\big)$$
   với `nc = "00000001"` (8 chữ số hex) và `cnonce` là chuỗi 16 ký tự hex ngẫu nhiên.

### 2.2 Các Quy Tắc Sống Còn Khi Gọi API (Hard-won Gotchas)
1. **Bắt buộc gửi `remote_terminal_info` trong body lúc Login**:
   - Payload của `POST /API/Web/Login` bắt buộc phải là:
     ```json
     {"data": {"remote_terminal_info": "WEB,chrome"}}
     ```
2. **Bắt buộc lưu cả `Set-Cookie` (`session_80` / `session`) và header `X-csrftoken`**:
   - Phản hồi `200 OK` của `/API/Web/Login` trả về token CSRF trong **HTTP Response Header `X-csrftoken`** (không nằm trong body JSON trên firmware mới). Nếu header `X-csrftoken` có dấu phẩy `,`, chỉ lấy phần tử đầu tiên `strings.TrimSpace(strings.Split(hdr, ",")[0])`.
   - Nếu các request tiếp theo thiếu `Cookie` hoặc thiếu `X-csrftoken`, thiết bị sẽ trả về HTTP `400 Bad Request`:
     ```json
     {"version": "1.0", "error_code": "one_IE"}
     ```
3. **Nén Gzip Payload (`accepted_compress_encoding`)**:
   - Mặc định thiết bị chấp nhận JSON thuần (`Content-Type: application/json; charset=utf-8`) cho mọi request. Trên NVR 256 kênh, `/API/Login/DeviceInfo/Get` khai báo `"accepted_compress_encoding": ["gzip"]`, cho phép client gửi body nén gzip nếu payload quá lớn, nhưng JSON không nén vẫn hoạt động 100%.
4. **Chia nhỏ Request theo Batch (`support_partial_request`)**:
   - Với NVR 256 kênh, một số endpoint nặng như `/API/IPCMaintaint/IPCDisk/Get` quy định `quota: 16, total: 256` (gửi danh sách `channel_list` tối đa 16 kênh mỗi lần).
5. **Đổi mật khẩu dùng Ephemeral Key (`Login/TransKey/Get` & `Maintenance/TransKey/Get`)**:
   - Các thao tác đổi mật khẩu tài khoản (`/API/SystemConfig/User/Set`) hoặc xác thực bậc 2 (`base_secondary_authentication`) sử dụng khóa phiên PBKDF2-SHA256 (`base_salt`) + ECDH/X25519 (`base_x_public` trong `sdk_se_wasm.wasm`) lấy từ `/API/Maintenance/TransKey/Get`.

---

## 3. Danh Mục Toàn Bộ Điểm Cuối API (`/API/*` Endpoint Reference)

Mọi endpoint dưới đây đều nhận `POST /API/<Path>/<Action>` (trong đó `<Action>` là `Range`, `Get`, `Set`, `Default`, `Search`, `Check`, `Control`) với cấu trúc:
- **Request**: `{"version": "1.0", "data": <Payload>}`
- **Response**: `{"result": "success", "data": <ResponseData>}` (hoặc `{"result": "failed", "error_code": "...", "reason": "..."}`).

### 3.1 Nhóm Xác Thực, Thông Tin Hệ Thống & Kênh (`Login/*`, `SystemInfo/*`)

| Endpoint | Tham Số `data` | Dữ Liệu Trả Về Chính |
|---|---|---|
| `/API/Login/Range` | `{}` (Không cần login) | `device_type` (`"IPC"` hoặc NVR), `site_version`, `http_api_version`, `default_lang`, `password` min/max len. |
| `/API/Web/Login` | `{"remote_terminal_info":"WEB,chrome"}` | Header `Set-Cookie: session_80=...`, `X-csrftoken: ...`, body `device_type` (`"N10256-16HDD"`). |
| `/API/Web/Logout` | `{}` | Giải phóng session hiện tại. |
| `/API/Login/Heartbeat` | `{}` | Giữ sống phiên đăng nhập (`keep-alive`). |
| `/API/Login/DeviceInfo/Get` | `{"support_new_schedule":true}` | `encode_type_ability` (`["H.264","H.265","H.264+","H.265+"]`), `intelligent_chn_arr`, `channel_num`. |
| `/API/Login/ChannelInfo/Get` | `{}` | Mảng `channel_param.items[]`: `channel` (`"CH1".."CH256"`), `connect_status` (`"Online"`, `"Offline"`, `"NotConfigured"`), `channel_name`, `ability[]`, `intelligent_ability[]` (`["FD","PVD","LPD","LCD","SOD","CrossCount","HeatMap","CrowdDensity","QD","Intrusion","RegionEntrance","RegionExiting","RSD"]`). |
| `/API/SystemInfo/Base/Get` | `{}` | `device_name`, `device_type`, `hardware_version`, `software_version`, `ie_client_version`, `mac_address`, `p2p_id` (dùng làm Serial Number định danh phần cứng), `hdd_volume`. |
| `/API/ChannelConfig/IPChannel/Get` | `{}` (Trên NVR) | Danh sách toàn bộ kênh IP con trên NVR: `channel_info["CH2"]` gồm `state` (`"Online"`), `ip_address` (`"192.168.2.100"`), `port` (`80`), `protocol` (`"Private"` / `"Onvif"`), `username`, `mac_address`, `software_version`, `forward_port` (`65002`), `channel_name`. |

### 3.2 Nhóm Cấu Hình Luồng Video, Âm Thanh, OSD & Hình Ảnh (`StreamConfig/*`, `ChannelConfig/*`, `DeviceConfig/*`)

#### A. Luồng Chính & Luồng Phụ (`/API/StreamConfig/MainStream/{Range,Get,Set}` & `SubStream`, `MobileStream`)
- **Đọc hiện trạng (`Get`)**:
  ```json
  {
    "result": "success",
    "data": {
      "channel_info": {
        "CH1": {
          "resolution": "2880 x 1620",
          "fps": 25,
          "bitrate": 3072,
          "bitrate_mode": "Predefined",
          "custom_bitrate": 3072,
          "video_encode_type": "H.265",
          "video_encode_level": "MainProfile",
          "bitrate_control": "CBR",
          "video_quality": "Highest",
          "audio": true,
          "i_frame_interval": 50
        }
      }
    }
  }
  ```
- **Quy tắc Ghi (`Set`) & Đối chiếu Đọc lại (Tri-State Verification)**:
  - **Độ phân giải (`resolution`)**: Định dạng chuỗi có khoảng trắng `"<Width> x <Height>"` (ví dụ `"2880 x 1620"`, `"2560 x 1440"`, `"2304 x 1296"`, `"1920 x 1080"`, `"1280 x 720"`).
  - **Chuẩn nén & SmartCodec (`video_encode_type`)**: Raysharp gộp chung chuẩn nén cơ sở và SmartCodec vào một trường duy nhất:
    - Khi tắt SmartCodec: `"H.265"` hoặc `"H.264"`.
    - Khi bật SmartCodec: `"H.265+"` hoặc `"H.264+"`.
  - **Tốc độ khung hình (`fps`)**: Số nguyên (`1..25` hoặc `1..30`). Tra cứu dải FPS hợp lệ theo từng độ phân giải tại `/API/StreamConfig/MainStream/Range` (`streaminfo[].fps.ranges`).
  - **Bitrate (`bitrate`, `custom_bitrate`, `bitrate_mode`, `bitrate_control`)**:
    - `bitrate_control`: `"CBR"` hoặc `"VBR"`.
    - Nếu giá trị Kbps nằm trong danh sách chuẩn (`[256, 320, 384, 448, 512, 640, 768, 896, 1024, 1280, 1536, 1792, 2048, 3072, 4096, 5120, 6144, 8192]`), đặt `bitrate_mode: "Predefined"`, `bitrate: <val>`, `custom_bitrate: <val>`.
    - Nếu là giá trị tùy chỉnh ngoài danh sách, đặt `bitrate_mode: "UserDefined"`, `custom_bitrate: <val>`, `bitrate: <val>`.
  - **Khoảng cách I-Frame (`i_frame_interval`)**: Số nguyên tính bằng frame (`1..100`, thường bằng $2 \times \text{FPS}$).

#### B. Cấu Hình Âm Thanh (`/API/DeviceConfig/Audio/{Range,Get,Set}`)
- Trả về `channel_info["CHx"]`: `{"audio_type": "G711A", "in_volume": 9, "out_volume": 9}`.
- **Gotcha phần cứng**: Cả NVR `N10256N` và IPC `6M019A` chỉ hỗ trợ `["G711A", "G711U"]` (`pcm_alaw` / `pcm_mulaw`), không hỗ trợ `AAC` phần cứng. Khi `Profile.SetAudioAAC` được gọi trong `kspcam`, nếu `Range` không có `"AAC"`, driver sẽ giữ nguyên `audio: true` trên luồng và trả về cảnh báo Clamped/Degraded rõ ràng, đồng thời `shinobi.BuildMonitorConfig` cho `VendorRaysharp` sẽ đặt `acodec: "no"`, `stream_acodec: "no"`, `record_acodec: "no"` để tuân thủ tuyệt đối nguyên tắc Zero-Transcoding (0% CPU).

#### C. Tên Kênh & Hiển Thị OSD (`/API/ChannelConfig/OSD/{Range,Get,Set}`)
- Cấu trúc `channel_info["CHx"]`:
  ```json
  {
    "status": "Online",
    "name": {"show": true, "text": "Ban 01", "pos": {"x": 290, "y": 0}},
    "alarm": {"show": false, "text": "Alarm Pos", "pos": {"x": 0, "y": 50}},
    "datetime": {"show": true, "date_format": "YYYY-MM-DD", "time_format": 24, "pos": {"x": 390, "y": 0}},
    "refresh_rate": "50Hz",
    "privacy_protection": false
  }
  ```
- Hỗ trợ đọc/ghi tên kênh hiển thị (`name.text`, `name.show`) và dòng chữ phụ (`alarm.text`, `alarm.show`).

#### D. Tinh Chỉnh Hình Ảnh & Màu Sắc (`/API/ChannelConfig/Color/{Range,Get,Set}` & `/API/ChannelConfig/ImageControl/{Range,Get,Set}`)
- Quản lý Brightness, Contrast, Saturation, Sharpness, Hue, Day/Night IR-Cut, Backlight/WDR, Flip/Mirror, Defog.

#### E. Chụp Ảnh Snapshot Tức Thời (`/API/Snapshot/{Range,Get}`)
- Gửi `POST /API/Snapshot/Get`:
  ```json
  {
    "version": "1.0",
    "data": {
      "channel": "CH1",
      "snapshot_resolution": "720 x 480",
      "reset_session_timeout": false
    }
  }
  ```
- Trả về `{"result": "success", "data": {"img_format": "image/jpeg", "img_data": "<base64_jpeg>"}}`.

### 3.3 Nhóm Mạng, Thời Gian, Lưu Trữ, Bảo Trì & Xem Lại (`NetworkConfig/*`, `SystemConfig/*`, `StorageConfig/*`, `Maintenance/*`, `Playback/*`)

| Chức Năng | Endpoint Raysharp | Chi Tiết Kỹ Thuật |
|---|---|---|
| **Đọc/Ghi Mạng & IP** | `/API/NetworkConfig/NetBase/{Get,Set}` | Trên IPC: object `wan` (`dhcp`, `ip_address`, `subnet_mask`, `gateway`, `dns1`, `dns2`) + mảng `port`. Trên NVR: `lan1`, `lan2`, `dns1`, `dns2`. |
| **Quét Wi-Fi** | `/API/NetworkConfig/ScanWlan/Get` | Quét danh sách Wi-Fi AP (trên các mẫu IPC hỗ trợ Wi-Fi). |
| **Đọc/Ghi Giờ & NTP** | `/API/SystemConfig/DateTime/{Get,Set}` & `/API/SystemConfig/NTP/{Get,Set}` | Quản lý `date`, `time`, `time_zone`, `ntp_enable`, `server`. |
| **Tự Động Reboot** | `/API/Maintenance/AutoReboot/{Get,Set}` | `auto_reboot` (`bool`), `maintenance_interval` (`"7 Day"`), `maintenance_time` (`"00:00-02:00"`). |
| **Khởi Động Lại Ngay** | `/API/Maintenance/DeviceReboot/Set` (hoặc `/API/Maintenance/DeviceReboot`) | Khởi động lại thiết bị từ xa. |
| **Quản Lý Ổ Đĩa / HDD** | `/API/StorageConfig/Disk/{Get,Set}` | Đọc `disk_info[]`, `hdd_format_type`, `over_write`. |
| **Tra Cứu Ghi Hình** | `/API/Playback/SearchMonth/Get` & `/API/Playback/SearchRecord/Search` | Tìm kiếm lịch sử ghi hình theo tháng và danh sách đoạn video trong ngày (`channel: ["CH2"]`, `start_date`, `start_time`, `end_date`, `end_time`, `stream_mode: "Mainstream"`). |
| **Điều Khiển PTZ** | `/API/PreviewChannel/PTZ/Control` | Điều khiển xoay/zoom PTZ (`channel`, `cmd`, `speed`, `state`). |

---

## 4. Kiến Trúc AI Toàn Diện & Cơ Chế Lưu Trữ / Đẩy Sự Kiện AI

### 4.1 Cấu Hình Thuật Toán AI Tại Biên (`/API/Event/*`)
1. **Tổng quan trạng thái AI tất cả các kênh (`POST /API/Event/ChnSmart/Get`)**:
   - Trả về danh sách nhóm thuật toán AI trên từng kênh (`Target Detection`, `Vehicle Intelligence`, `Behavior Analysis`, `Statistics Analysis`, `Normal Event`, `Thermal`) kèm trạng thái `state: "On" | "Off"` và danh sách xung đột phần cứng `mutual_ability`.
2. **Đọc/Ghi Tham Số Thuật Toán AI (`POST /API/Event/SettingConfig/{Range,Get,Set}`)**:
   - Request payload: `{"channel": "CH1", "ability": "<AbilityName>"}`
   - Các giá trị `<AbilityName>` chuẩn:
     - `"FaceDetection"` (`FD`): `switch`, `sensitivity`, `snap_mode` (`"OptimalMode"`), `snap_num`, `face_angle`, `roll_range`, `pitch_range`, `yaw_range`, `min_pixel`, `max_pixel`, `face_enhance`, `rule_info`.
     - `"HumanVehicle"` (`PVD`): `switch`, `sensitivity`, `detection_mode` (`"MotionMode"`), `detection_type` (`["Pedestrian", "Motor Vehicle", "Non-motor Vehicle"]`), `vehicle_attribute`, `pedestrian_attribute`, `rule_info`.
     - `"LicensePlate"` (`LPD`): `switch`, `detection_type` (`"EU_Plate"`), `select_mode` (`"entrance_exit"`), `snap_interval`, `lpd_enhance`, `capture_vehicle_image`, `rule_info`.
     - `"LineCrossing"` (`LCD`): `switch`, `sensitivity`, `target_validity`, `detection_type`, `rule_info` (`rule_line: {x1,y1,x2,y2}`, `rule_type: "A->B" | "B->A" | "A<->B"`).
     - `"Intrusion"`: `switch`, `sensitivity`, `time_threshold`, `target_validity`, `detection_type`, `rule_info` (`rule_rect: {x1..x8, y1..y8}`).
     - `"EnterRegion"`, `"ExitRegion"`, `"ObjectDetection"` (`SOD`), `"CrossCounting"` (`CC`), `"HeatMap"`, `"CrowdDensity"` (`CD`), `"QueueLength"` (`QD`), `"RareSound"` (`RSD`), `"MotionDetection"`, `"VideoTampering"`, `"SoundDetection"`.
3. **Đọc/Ghi Liên Kết Báo Động AI (`POST /API/Event/AlarmLinkage/{Range,Get,Set}`)**:
   - Request payload: `{"channel": "CH1", "ability": "<AbilityName>"}`
   - Điều khiển: `record_enable`, `post_recording`, `http_listening`, `ftp_picture_upload`, `send_email`, `deterrence` (đèn chớp Floodlight), `siren` (còi hú), `voice_prompts`.
4. **Lịch Trình Hoạt Động AI (`POST /API/Event/Plan/{Range,Get,Set}`)**:
   - Request payload: `{"channel": "CH1", "ability": "<AbilityName>"}`.

### 4.2 Cơ Sở Dữ Liệu Khuôn Mặt, Biển Số & Tìm Kiếm Đối Tượng AI (`/API/AI/*`)
Đầu ghi NVR Raysharp lưu trữ toàn bộ lịch sử ảnh chụp AI (Face, Pedestrian, Vehicle, License Plate) kèm vector đặc trưng (feature embedding) trên ổ cứng và cung cấp các API:
- **Nhóm Khuôn Mặt (`FDGroup/*`, `Faces/*`, `AddedFaces/*`)**:
  - `POST /API/AI/FDGroup/GetId` & `/API/AI/FDGroup/Get` & `/API/AI/FDGroup/Add` & `/API/AI/FDGroup/Modify` & `/API/AI/FDGroup/Remove`
  - `POST /API/AI/Faces/Add` & `/API/AI/Faces/Modify` & `/API/AI/Faces/Remove`
  - `POST /API/AI/AddedFaces/Search` & `/API/AI/AddedFaces/GetByIndex` & `/API/AI/AddedFaces/GetById`
- **Nhóm Biển Số Xe (`PlateGroup/*`, `Plates/*`, `AddedPlates/*`)**:
  - `POST /API/AI/PlateGroup/GetId` & `/API/AI/PlateGroup/Get` & `/API/AI/PlateGroup/Add` & `/API/AI/PlateGroup/Modify` & `/API/AI/PlateGroup/Remove`
  - `POST /API/AI/Plates/Add` & `/API/AI/Plates/Modify` & `/API/AI/Plates/Remove` & `/API/AI/AddedPlates/GetById`
- **Tìm Kiếm Ảnh Chụp AI Theo Thời Gian (`SnapedFaces/*`, `SnapedObjects/*`)**:
  - `POST /API/AI/SnapedFaces/Search` $\rightarrow$ `/API/AI/SnapedFaces/GetByIndex` $\rightarrow$ `/API/AI/SnapedFaces/GetById`
  - `POST /API/AI/SnapedObjects/Search` (hoặc `SearchPlate`) $\rightarrow$ `/API/AI/SnapedObjects/GetByIndex` $\rightarrow$ `/API/AI/SnapedObjects/GetById`
- **Thống Kê AI (`FaceStatistics/*`, `ObjectStatistics/*`, `CCStatistics/*`, `HeatMapStatistics/*`)**:
  - `POST /API/AI/FaceStatistics/Search` & `/Get`, `POST /API/AI/ObjectStatistics/Get`.

### 4.3 Luồng Sự Kiện Thời Gian Thực Kép (Dual-Mode Real-time Event Ingestion)
1. **Chế độ 1 — Long-Polling `/API/Event/Check` (Không cần mở cổng hay cấu hình trên cam/NVR)**:
   - Gửi `POST /API/Event/Check` với:
     ```json
     {
       "version": "1.0",
       "data": {
         "plus_eventchk": "eventAiPushPic",
         "ext_data": {
           "subscribe_type": [{"event": ["all"]}, {"aipushpic": ["all"]}]
         },
         "reader_id": 3,
         "sequence": 438,
         "lap_number": 0
       }
     }
     ```
   - Lần gọi đầu tiên không truyền `reader_id`/`sequence`/`lap_number`; từ lần thứ 2 trở đi truyền lại bộ 3 con trỏ ring-buffer nhận được từ phản hồi trước đó.
   - Thiết bị trả về:
     - `alarm_list[]`: Trạng thái cảnh báo hệ thống (`system_alarm` như `NoDisk`, `DiskError`) và từng kênh (`channel_alarm`: `motion_alarm`, `videoloss`, `alarm_state: [{"int_subtype": "pd_vd" | "fd" | "lcd" | "intrusion" | "lpd"}]`).
     - `ai_snap_picture`: Mảng `FaceInfo[]`, `SnapedObjInfo[]`, `PlateInfo[]` chứa ảnh chụp đối tượng AI tức thời.
2. **Chế độ 2 — Webhook Push Chủ Động từ NVR / IPC về `kspcam`**:
   - NVR (`POST /API/AlarmConfig/EventPush/Set`) và IPC (`POST /API/Event/Notification/Set`) có thể được cấu hình đẩy HTTP POST trực tiếp vào endpoint `/api/events/webhook/raysharp` của `kspcam`.

---

## 5. Đường Dẫn RTSP & Gotchas Khi Kéo Luồng

1. **Đường dẫn RTSP trên Đầu ghi NVR (`192.168.2.101`)**:
   - Luồng chính (MainStream): `rtsp://<user>:<pass>@<host>:554/rtsp/streaming?channel=<1..256>&subtype=0`
   - Luồng phụ (SubStream): `rtsp://<user>:<pass>@<host>:554/rtsp/streaming?channel=<1..256>&subtype=1`
   - Luồng thứ 3 (MobileStream): `rtsp://<user>:<pass>@<host>:554/rtsp/streaming?channel=<1..256>&subtype=2`
2. **Đường dẫn RTSP trên Camera IP lẻ (`192.168.2.100`) — Gotcha `453 Not Enough Bandwidth`**:
   - Mặc dù `/API/Preview/StreamUrl/Get` trên IPC trả về `channel=0&subtype=0`, khi IPC đang kết nối với NVR thì gọi `channel=0` sẽ bị từ chối với lỗi RTSP **`453 Not Enough Bandwidth`**.
   - Bắt buộc dùng **`channel=1`** (1-based index):
     `rtsp://<user>:<pass>@<host>:554/rtsp/streaming?channel=1&subtype=0`
     thì cả NVR và IPC đơn lẻ đều phát luồng H.265 mượt mà 100%.

---

## 6. Ma Trận Chạy Song Song Trên NPU (`mutual_ability`) & Đánh Giá Khả Thi 8 Tính Năng AI

Qua thực nghiệm bắn trực tiếp các tổ hợp `POST /API/Event/SettingConfig/Set` lên NPU của Camera `192.168.2.100` (`6M019A`) và đối chiếu `/API/Event/ChnSmart/Get`:

### 6.1 Các Nhóm Chạy Song Song Trên NPU Camera (`6M019A`)

1. **Nhóm 4 Tính Năng Nền (Luôn Bật Song Song Tự Do — Không Bao Giờ Xung Đột)**:
   - `HeatMap` (Bản đồ nhiệt mật độ)
   - `RareSound` (Phát hiện âm thanh bất thường)
   - `SoundDetection` (Cảnh báo ngưỡng âm thanh)
   - `VideoTampering` (Che khuất / Xịt sơn / Đổi góc camera)

2. **Combo NPU Mạnh Nhất — 8 Tính Năng Chạy Đồng Thời Trên 1 Camera**:
   - Nhóm 4 thuật toán Khu vực cấm & Hành vi chạy song song cùng nhau:
     - `LineCrossing` (`LCD` — Vượt rào ảo `A->B`, `B->A`, `A<->B`)
     - `Intrusion` (Xâm nhập khu vực cấm có ngưỡng thời gian `time_threshold`)
     - `EnterRegion` (Đi vào vùng cấm)
     - `ExitRegion` (Đi ra khỏi vùng)
   - Kết hợp 4 tính năng nền: `HeatMap` + `RareSound` + `SoundDetection` + `VideoTampering`.
   - Cả 4 thuật toán hành vi đều hỗ trợ `detection_type: ["Pedestrian", "Motor Vehicle", "Non-motorized Vehicle"]` kèm trích xuất thuộc tính `pedestrian_attribute: true` và `vehicle_attribute: true`.

3. **Các Chế Độ NPU Độc Quyền (Chạy 1 Chế Độ Chính + 4 Tính Năng Nền = 5 Tính Năng Đồng Thời)**:
   - **Chế độ Khuôn mặt**: `['FaceDetection', 'HeatMap', 'RareSound', 'SoundDetection', 'VideoTampering']` (+ NVR chạy `face_recognition: V0.6.0.0.2-release`).
   - **Chế độ Biển số xe**: `['LicensePlate', 'HeatMap', 'RareSound', 'SoundDetection', 'VideoTampering']` (kèm `capture_vehicle_image: true`, `vehicle_attribute: true`).
   - **Chế độ Đếm lượt Người/Xe**: `['CrossCounting', 'HeatMap', 'RareSound', 'SoundDetection', 'VideoTampering']`.
   - **Chế độ Mật độ đám đông**: `['CrowdDensity', 'HeatMap', 'RareSound', 'SoundDetection', 'VideoTampering']`.
   - **Chế độ Độ dài hàng đợi / Tắc nghẽn**: `['QueueLength', 'HeatMap', 'RareSound', 'SoundDetection', 'VideoTampering']`.
   - **Chế độ Phát hiện Người & Xe toàn khung**: `['HumanVehicle', 'HeatMap', 'RareSound', 'SoundDetection', 'VideoTampering']`.
   - **Chế độ Đồ vật bỏ quên / Mất**: `['ObjectDetection', 'HeatMap', 'RareSound', 'SoundDetection', 'VideoTampering']`.

### 6.2 Bảng Đánh Giá Mức Độ Khả Thi Giữa Firmware Raysharp & Camera Hub (`inut-bida-hub`)

| STT | Tính Năng Yêu Cầu | Mức Độ Khả Thi | Tầng Firmware (Camera `6M019A` + NVR `N10256N`) | Tầng Phần Mềm (`kspcam` + `inut-bida-hub`) |
|---|---|:---:|---|---|
| 1 | **Nhận diện khuôn mặt, phân tích thuộc tính** | **90%** | Cam cắt mặt (`FaceDetection`); NVR chạy `face_recognition: V0.6.0.0.2-release`, lọc `Gender`, `fAttrAge`, `GlassesType`, `Expression`, `MouthMask` (`/API/AI/SnapedFaces/Search`). *Yêu cầu gắn HDD vào NVR.* | Thu thập sự kiện `face_detection` qua `Event/Check` + MQTT, lưu trữ và hiển thị thuộc tính tập trung trên Hub. |
| 2 | **Giới hạn khu vực cấm xâm nhập** | **100%** | Cam/NVR chạy song song cả 4 luật `Intrusion`, `EnterRegion`, `ExitRegion`, `LineCrossing` với 4 vùng đa giác 8 đỉnh (`rule_rect`), lọc `Pedestrian` / `Motor Vehicle`. | Giao diện/API cấu hình vùng đa giác (`POST /api/ai/setting`) và đẩy cảnh báo tức thời. |
| 3 | **Tìm kiếm & truy vết xe biển đen, đối tượng truy nã** | **95%** | NVR quản lý danh sách đen khuôn mặt (`FDGroup`, `Policy=1`) và biển số (`PlateGroup`, `SnapedObjects/SearchPlate` có dung sai ký tự `MaxErrorCharCnt`). | Hub gom sự kiện `face_detection` & `license_plate` từ nhiều đầu ghi NVR để truy vết liên trạm (Cross-NVR Tracking) theo dòng thời gian. |
| 4 | **Báo cáo thống kê người, phương tiện** | **100%** | Cam đếm `CrossCounting` (`Cross In`/`Out`); NVR xuất báo cáo Ngày/Tuần/Tháng/Năm qua `AI/CCStatistics/Get`, `AI/ObjectStatistics/Get`, `AI/FaceStatistics/Get`. | Hub tổng hợp số liệu từ NVR và luồng MQTT để vẽ biểu đồ toàn hệ thống. |
| 5 | **Cảnh báo tắc nghẽn, kẹt xe, tụ tập đám đông** | **85%** | Cam có sẵn `CrowdDensity` (ngưỡng `max_detection_num: 50`) và `QueueLength` (ngưỡng số lượng `max_detection_num: 10` + thời gian ứ đọng `max_pro_time: 60s`). | Kết hợp cảnh báo `CrowdDensity` / `QueueLength` / `Intrusion` từ cam với thống kê mật độ phương tiện trên Hub. |
| 6 | **Bản đồ nhiệt hiển thị mật độ (HeatMap)** | **95%** | `HeatMap` chạy nền không xung đột NPU; NVR trả về ảnh nền + lưới mật độ nhiệt (`AI/HeatMapStatistics/Get`, `heat_map_height: 36`). | Hub lấy dữ liệu ma trận nhiệt từ NVR và render lớp phủ heatmap. |
| 7 | **Nhận diện biển số phương tiện (LPR / ANPR)** | **85% -> 99%** | Cam/NVR có sẵn `LicensePlate` (`EU_Plate`, `US_Plate`, `capture_vehicle_image: true`). Biển 1 dòng đọc tốt; biển 2 dòng VN đôi khi bị cắt dòng trên `EU_Plate`. | Khi cam đẩy ảnh crop xe/biển số (`PlateImg` / `ObjectImg`) qua MQTT về Hub, Hub có thể chạy OCR biển số VN 2 dòng hậu kiểm đạt 99%. |
| 8 | **Mở rộng, nâng cấp tính năng phần mềm trong khả năng phần cứng** | **100%** | Hỗ trợ nâng cấp firmware IPC hàng loạt qua NVR (`/API/IPCMaintaint/IPCUpgrade/*`) và chuyển đổi nóng preset NPU trong < 200ms. | Kiến trúc `kspcam` (`internal/raysharp`, `internal/eventbus`, `/api/raysharp/raw`) + `inut-bida-hub` cho phép bổ sung thuật toán/báo cáo mới tại Hub mà không bị giới hạn bởi firmware đầu ghi. |

---

## 7. Hồ Sơ Dịch Ngược Phần Mềm Máy Tính `VMS Pro64_VANTECH(VCMS)_V2.15.1-260422_01.exe` (Lưu Trữ Tham Chiếu)

### 7.1 Nguồn Tải & Định Danh Gói Cài Đặt (Link Lưu Trữ Chính Thức)
- **Link tải Viettel MyBox (Gốc)**: `https://mybox.vietteltelecom.vn/drive/s/xmbVkOivvJDl84wBK2QZV3YVzmGpfC?zarsrc=31&utm_source=zalo&utm_medium=zalo&utm_campaign=zalo`
- **API tải trực tiếp (Direct Download Endpoint)**:
  `https://mybox.vietteltelecom.vn/api/v1/file-entries/download/NjA1MzUzNDJ8cA?shareable_link=xmbVkOivvJDl84wBK2QZV3YVzmGpfC&download=true`
- **Tên file**: `VMS Pro64_VANTECH(VCMS)_V2.15.1-260422_01.exe`
- **Dung lượng**: `170,805,959 bytes` (`~162.9 MiB`)
- **SHA-256**: `b385da0322ee8d1142fe351e5ee7d27a3cea6bf00b79a01d101e7cf71a9a8c2d`
- **Định dạng đóng gói**: Inno Setup `5.6.0 (Unicode)` (`company_name=VANTECH`, `version=V2.15.1-260422` trong `app/default.ini`).
- **Bằng chứng phân tích REA MCP**:
  - `inspect_pe_resources` trên bộ cài gốc: `ev_2fbf75dde052c28c843aa83fc755433ab0ef1ef2d959f6cbbe22d0571d778866`
  - `inspect_pe_resources` trên `app/VCMS.exe` (`30,278,656 bytes`, SHA-256 `f035957b91d39e4dac4e65842c6b103e9a75aea985a875b451681c3ca0222dc2`): `ev_1df061f7deb3f5b5f9990eb5f8b9d5ad941dc9b3df4fc983eb30b238ed2bc49e`
  - `inspect_pe_resources` trên `app/RSNet64.dll` (`4,360,704 bytes`, SHA-256 `6e38a738ea926eb3815eefd391cfa409bd1745d3cf1cf966ad3ae15bd4cf21a5`): `ev_fdee99600d04e363f8acd06ae30059043dcad10cc23d7824d31a2e845294d55c`
  - `inspect_pe_resources` trên `app/RSPlay64.dll` (`4,760,064 bytes`, SHA-256 `c40c87bf05895a01753cad97ea05f3cfbd9ed9e204f41ff4402d350c8776bd72`): `ev_1cb90f15e0fa79f75e4cb63387cb3339200b4d9b42d1ddd69c4b15b85f0571e4`
  - `inspect_artifact` & `extract_artifact` trên `app/import_template_enu.zip`: `ev_7be1f71731044a78bca6e8da3439bc899251ac789fde5ce906bb07514c552d28`, `ev_41419f57b03c2ca56d363f90946bf157a90faf08d52d78d2e7c3b69656d86524`
  - `analyze_javascript_application` trên tập module JS giải nén từ Qt Resource `.rdata` của `VCMS.exe`: `ev_6066c39e8b4adaa507be3bfae39d6e35d8bfb0c950b8905927561b6d5b2e3b2c`

### 7.2 Cấu Trúc Thành Phần Cốt Lõi Trong `VCMS.exe`
1. **Kiến trúc Hybrid C++ Qt5 + Embedded WebEngine**:
   - `VCMS.exe` kết hợp giao diện Qt5 Widgets (`1,503` C++ RTTI classes: `RSFaceID`, `RSRepeatVisitors`, `RSAttendanceCalculationObj`, `rsDataAnalysisHeatMap`, `rsAIScene`...) với `Qt5WebEngineCore.dll` + `Qt5WebChannel.dll` nhúng `283` tài nguyên Web (`qrc:/www/res/...`) để cấu hình thiết bị từ xa.
   - Lưu trữ cục bộ bằng **3 cơ sở dữ liệu SQLite (`app/sqldatabase/`)** có hỗ trợ mã hóa `sqlitecipher.dll`:
     - `base.db`: Quản lý thiết bị (`TDevice`, `TDevForCMSLogin`), nhóm kênh (`TGroupChannel`), bản đồ điện tử (`TEmap`, `TEmapObject`), chấm công (`TAttendance*`) và nhân sự/vector khuôn mặt (`TPersonnelManagement` lưu cột `FEATURE` và `FTVERION`).
     - `Local.db`: Quản lý lịch ghi hình cục bộ và chỉ mục file video (`TLocalRecordFileInfo`, `TLocalAlarmRecordFileInfo`).
     - `Alarm.db`: Quản lý kịch bản liên kết báo động (`TAlarmConfig`, `TChannelAlarmConfig` với cột `IntelligentBit`) và nhật ký (`TAlarmLog`).
2. **4 Họ SDK Đa Hãng Đi Kèm**:
   - **Raysharp SDK (Chính)**: `RSNet64.dll` (143 exports: giao thức nhị phân port `9000` + HTTP/HTTPS JSON `/API/*` trên nền `boost::beast`), `RSPlay64.dll` (153 exports: giải mã H.264/H.265/MJPEG phần cứng DXGI/D3D9/D2D1 + HiSilicon decoder `CHisi264Decoder`/`CHisi265Decoder`), `RSOnvifs64.dll` (32 exports), `RSIPsound64.dll` (5 exports đàm thoại 2 chiều & phát file âm thanh xuống loa IP), `RSStream.dll` (`live555`).
   - **Dahua SDK**: `dhnetsdk.dll`, `dhconfigsdk.dll`, `dhplay.dll`, `avnetsdk.dll`, `StreamConvertor.dll`, `StreamSvr.dll`, `IvsDrawer.dll`.
   - **Hikvision SDK**: `HCNetSDK.dll`, `HCCore.dll`, `PlayCtrl.dll`, `Sadp.dll`, `HCNetSDKCom/*`.
   - **8 Họ P2P Cloud (`app/p2p/*`)**: `RSP2PLibrary.dll` (Raysharp), `IOTCAPIs.dll` / `AVAPIs.dll` / `P2PTunnelAPIs.dll` (TUTK), `GlnkAgentSDK.dll` (Goolink), `PPCS_API.dll` / `PPPP_API.dll` (CS2), `libt2u.dll` (Vantech RSVV/VV), `NpcMpiMonTsp.dll` (HSD).

### 7.3 Các Phát Hiện Kỹ Thuật & Thuật Toán AI Có Thể Tái Sử Dụng
1. **Thư viện Ghép Ảnh Toàn Cảnh Độc Lập `app/rsstitcher.dll` (`1.19 MB`)**:
   - Xây dựng trên nền **OpenPano + CImg + FLANN** (không phụ thuộc OpenCV nặng):
     - Phát hiện đặc trưng **DoG + SIFT Descriptor** (`pano::SIFTDetector`, `dog.cc`, `orientation.cc`).
     - Khớp đặc trưng bằng **FLANN KD-Tree** + ước lượng ma trận **Homography RANSAC** (`homography.cc`, `transform_estimate.cc`).
     - Chiếu hình trụ **Cylindrical Warping** (`pano::CylinderStitcher::build_warp`) và trộn đa băng tần **Multi-Band Blending** (`pano::MultiBandBlender::WeightedPixel`).
   - Chỉ export 3 hàm C++ đơn giản (`stitching_create`, `stitching_proc`, `stitching_release`), nhận mảng đường dẫn ảnh đầu vào và xuất ảnh Panorama ghép hoàn chỉnh.
2. **Bộ Tham Số Thị Giác Máy Tính JDVCA (`MsgParamJDVCA* = 724..726`)**:
   - Cho phép tinh chỉnh trực tiếp các siêu tham số thuật toán trên camera từ xa:
     - **JDVCA Intrusion Detection (`725`, `Intrusion_Detection.js`)**: `imgResizeFactor`, `minPersonConf`, `skipFrameNum`, `bgsUpdataFactor`, `BBSInRoiThr`, `fgInRoiThr`, `fgPesBBThr`, `minOverlapRatio`, `pesRectRatio`, và đặc biệt là **`footRectRatioFrom` / `footRectRatioTo`** (xét vị trí hộp giới hạn bàn chân trong đa giác ROI để chống báo giả do góc nghiêng camera).
     - **JDVCA Red Mantle / Occlusion & Lost Object (`726`, `Red_Mantle_Detection.js`)**: Tinh chỉnh 2 tốc độ học nền `bgsUpdateSecondsSlow` & `bgsUpdateSecondsFast`, `fDetectRatioFixed`, `fDetectRatioLosted`, `fDetectRatioOcclusion`, `vanishKeepSeconds`, `toFixKeepSeconds`.
     - **JDVCA Meter Recognition (`724`, `Type_Meter_Recognition.js`)**: Đọc giá trị đồng hồ kim tự động qua ROI `(left, top, width, height)` và thông số góc `begangle`, `anglescale`, `initv`, `valuescale`.
3. **Quy Tắc Khóa GOP Khi Bật SmartCodec (`H.264+` / `H.265+`) Trên Firmware Raysharp/Vantech**:
   - Trong `main_stream_set.js` (dòng `764`, `844`, `976`), khi `VideoEncType == 3` (`H.264+`) hoặc `VideoEncType == 4` (`H.265+`), firmware **tự động ẩn và khóa cứng `IFrameInterval` (GOP)** (`$("#IPC_form_Iratio_div").css('display', 'none')`).
   - Do đó, giống như quy tắc trên Dahua/Imou/KBVision, **bắt buộc phải tắt SmartCodec (`VideoEncType = 1` / `"H.265"`)** trên mọi camera Raysharp/Vantech để có thể khóa `i_frame_interval = fps * 2` (`2.0s`), giúp phân đoạn HLS `.ts` luôn đạt chuẩn `2.000s`.
4. **Giải Mã Phiên Bản Model NPU (`AI_Model.js` — `paramPageJson.AI_Model = "FaceDatabaseConfig"`)**:
   - Phiên bản thuật toán NPU 32-bit `version` trả về từ `/API/AI/Model/Get` được giải mã theo công thức:
     $$\text{Company} = (\text{ver} \gg 27) \& \text{0x1F}, \quad \text{Chip} = (\text{ver} \gg 21) \& \text{0x3F}, \quad \text{Func} = (\text{ver} \gg 16) \& \text{0x1F}, \quad \text{Alg} = (\text{ver} \gg 10) \& \text{0x3F}$$
     $$\text{Release} = \lfloor (\text{ver} \& \text{0x3FF}) / 2 \rfloor + 1, \quad \text{Beta} = (\text{ver} \& 1)$$
     (Ví dụ: `V0.6.0.0.2-release`).
5. **Đường Dẫn RTSP Chuẩn Cho Dòng Firmware Cũ & Mới (`RTSP_Set.js`)**:
   - **Firmware HTTP API V1.0 mới (`6M019A` / `N10256N`)**: `rtsp://<user>:<pass>@<IP>:554/rtsp/streaming?channel=1&subtype=0` (Main) / `subtype=1` (Sub).
   - **Firmware Raysharp/Vantech truyền thống (`DEV_IPC` / `DEV_NVR`)**: `rtsp://<user>:<pass>@<IP>:554/ch01/0` (Main), `/ch01/1` (Sub), `/ch01/2` (Mobile) hoặc `/ip01/0` (trên Hybrid DVR).
