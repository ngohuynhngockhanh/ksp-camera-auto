# Đặc Tả Giao Thức Xiongmai (`internal/xiongmai` — Sofia TCP 34567 & HTTP CGI/WS 80)

Tài liệu đặc tả kiến trúc hướng đối tượng thuần Go (`CGO_ENABLED=0`) cho hệ sinh thái Camera IP và Đầu ghi DVR/HVR/NVR của **Hangzhou Xiongmai Technology (XM / XMEye / iCSee / Vitacam / JFTech / Sofia)** trong `kspcam`.

---

## 1. Kiến Trúc Hướng Đối Tượng Đa Tầng Giao Vận (`Transport` Strategy Pattern)

Thực tế triển khai tại hiện trường (ví dụ node `inut_208_4` — Nhà Yến Anh Hùng Gò Vấp) tồn tại song song 2 thế hệ firmware Xiongmai:
- **Camera IP độc lập 8MB Flash (Vitacam / iCSee / XMEye)**: Chỉ mở giao thức nhị phân **Sofia DVRIP trên TCP `34567`** (`MD5_8`), cổng HTTP `80` tắt hoặc chỉ chứa bootstrap rỗng.
- **Đầu ghi HVR/NVR đời mới (bật bảo mật RSA)**: Từ chối đăng nhập `MD5_8` trần trên cổng `34567` với mã **`Ret = 205` (`ERR_RSA_REQUIRED`)**, đồng thời cung cấp đầy đủ REST CGI mã hóa **RSA-1024 + `AES-128-CBC` ZeroPadding** và WebSocket nhị phân trên cổng `80`.

Package `internal/xiongmai` tách bạch hoàn toàn tầng giao vận thông qua interface `xiongmai.Transport`:

```mermaid
classDiagram
    class Transport {
        <<interface>>
        +Name() string
        +Login(ctx) (*LoginInfo, error)
        +GetInfo(ctx, name) (map, error)
        +GetAbility(ctx, name) (map, error)
        +GetConfig(ctx, name) (map, error)
        +SetConfig(ctx, name, payload) (map, error)
        +OpDev(ctx, name, param) (map, error)
        +Snapshot(ctx, ch) ([]byte, error)
        +DownloadRecordFile(ctx, w, ch, file, start, end) error
        +Close() error
    }
    class SofiaTCPTransport {
        +20-byte Little-Endian Header (0xFF 0x01)
        +MD5_8 Login (MsgID 1000)
        +Multi-packet Reassembly
        +OPPlayBack Claim/Data (1422/1424)
    }
    class CGITransport {
        +RSA-1024 PKCS#1 v1.5 GetSalt/Login
        +AES-128-CBC ZeroPadding (DTAK)
        +Large Plain-JSON Fallback
        +RFC 6455 WebSocket HttpPlayBack
    }
    class Client {
        -transport Transport
        -sysInfo *SystemInfoData
        +ProbeAll(ctx) ([]StreamInfo, error)
        +SetCodec/SetResolution/SetFPS/SetGOP/SetBitrate()
        +GetRemoteDevices(ctx) ([]RemoteChannel, error)
        +FindRecordings/ExportMP4Faststart()
    }
    class xiongmaiCamera {
        +Implements camera.Camera + 12 Capability Interfaces (incl. NVRHealthConfig)
    }
    Transport <|.. SofiaTCPTransport
    Transport <|.. CGITransport
    Client *-- Transport : Auto-Negotiates 34567 <-> 80
    xiongmaiCamera *-- Client
```

### Quy tắc Đàm phán Tự động (`Client.Transport`)
1. Khi cổng cấu hình là `34567` (mặc định `xiongmai_port: 34567`): Thử `SofiaTCPTransport(:34567)` trước. Nếu thiết bị trả `Ret == 205` (`ErrRSAHandshakeRequired`) hoặc không mở `34567`, tự động chuyển sang `CGITransport(:80)`.
2. Khi cổng cấu hình là `80` (hoặc cổng HTTP tùy biến): Thử `CGITransport` trước; nếu timeout / `404` (như trên các mắt cam lẻ), tự động chuyển sang `SofiaTCPTransport(:34567)`.
3. **Chống khóa tài khoản (`Ret = 206`)**: Nếu tầng thứ nhất trả về `ErrAuthFailed` (`Ret = 106 / 203` — sai mật khẩu), `Client` dừng ngay lập tức, tuyệt đối không thử lại trên tầng thứ hai để tránh tốn gấp đôi số lần thử sai.

---

## 2. Đặc Tả Mật Mã & Quy Đổi Tham Số Đặc Thù

1. **Hàm băm `MD5_8`**:
   - Với `digest = MD5(password)` (16 bytes), tính `r[i] = (digest[2*i] + digest[2*i+1]) % 62` với `i = 0..7` và ánh xạ vào `0-9` (`0..9`), `A-Z` (`10..35`), `a-z` (`36..61`).
2. **Đăng nhập CGI RSA + Phiên `AES-128-CBC` ZeroPadding**:
   - `Sign = RSA_PKCS1v15_Hex(OEMHeader + Salt + hex_md5_lower(password))`
   - `DTAK = RSA_PKCS1v15_Hex(aesKey16)` (IV gồm 16 byte `0x00`, đệm byte `0x00` cuối chuỗi).
3. **Định dạng `MainFormat` / `ExtraFormat` kép & Kênh IP `bypass@`**:
   - Tự động đọc/ghi cả định dạng đối tượng `{...}` (HVR/CGI) lẫn mảng 1 phần tử `[{...}]` (IPC Sofia).
   - Trên đầu ghi HVR/NVR khi `ch >= VideoInChannel`, tự động định tuyến sang `bypass@AVEnc.Encode.[ch - VideoInChannel]`.
4. **Quy đổi `GOP` (Giây $\leftrightarrow$ Frames) & IP Hex Little-Endian**:
   - Xiongmai lưu `Video.GOP` theo **giây (`1..12s`)** thay vì số frame: `kspcam` tự động nhân `FPS` khi `Probe` và chia `round(gopFrames / fps)` khi `Apply`.
   - Địa chỉ IP trong `NetWork.NetCommon` lưu dạng Hex Little-Endian (`"0x8A4DA8C0"` $\leftrightarrow$ `"192.168.77.138"`).
5. **Đường dẫn RTSP Chuẩn & Zero-CPU Playback Bridge**:
   - RTSP: `rtsp://<user>:<pass>@<host>:554/user=<user>_password=<pass>_channel=<1..N>_stream=<0|1>.sdp?real_stream`
   - **Zero-CPU Hybrid Playback Bridge (`XMStreamExtractor`)**: Tách vỏ khung media Xiongmai xuyên biên gói WebSocket / Sofia TCP `1424` (khung I-Frame `00 00 01 FC/FE` 16 byte với `uint32` LE length tại `[12:16]`; khung P-Frame `00 00 01 FD` 8 byte với **`uint32` LE length tại `[4:8]`**; khung Audio/Info `00 00 01 FA/F9` 8 byte với `uint16` LE length tại `[6:8]`), tính toán phân số tốc độ khung hình thực `ObservedFPSRational()` (`<tổng số frame>/<khoảng giây thực>` giữa các mốc giờ nén 32-bit tại `[8:12]` của I-Frame — tự động khớp `1.000x` thời gian thực cho cả cam Vitacam `6 fps`, Imou `20 fps`, Hikvision `25 fps` hay cam đêm VFR), ngắt kết nối sớm ngay khi chạm `end` và đóng gói MP4 `+faststart` với `-c:v copy -an` (`0% CPU` transcode).
6. **Tương thích Full NVR & Shinobi Hybrid 1:1 như Dahua / Hikvision**:
   - **`GetRemoteDevices` (`/api/nvr/scan` & `/api/nvr/link`)**: Giải mã `SingleConnId` dạng chuỗi Hex (`"0x00000004"` $\to$ `Decoder[3]`), tự động bỏ qua các khe `NoConfig` (`"0x00000000"` hoặc `NetWork.ChnStatus == "NoConfig"`).
   - **`Probe` kênh RTSP-Proxy (`NetWork.ChnStatus`)**: Khi đầu ghi IP NVR (`VideoInChannel == 0`) thu luồng RTSP thuần không qua bộ mã hóa (`AVEnc.Encode` trả `Ret = 607`), tự động fallback đọc `NetWork.ChnStatus` (`CurRes: "1080P/D1"`).
   - **`NVRHealthConfig` (`/api/nvr/health` & Watchdog)**: Đọc `SystemInfo.DeviceRunTime`, bảng lịch `Record` (`ConfigRecord` / `ManualRecord` / `ClosedRecord` + mặt nạ `Mask` 7 ngày) và `StorageInfo` (`State: "Success"`).
   - **`Snapshot` WebSocket `HttpRealplay`**: Khi đầu ghi tắt `/webcapture.jpg`, tự động kéo 1 IDR keyframe qua `/websocket-bin/Type=HttpRealplay` ngay trên cổng HTTP `80` (hoạt động xuyên NAT không cần mở cổng RTSP `554`).
7. **Tự động giải mã `GetRandomUser` (`admin` $\to$ `admin1`) & Tương thích `uc-httpd 1.0.0`**:
   - Khi cấu hình `username: "admin"`, `CGITransport` tự động gọi `/cgi-bin/resetpwd.cgi` (`GetDevInfo` + `GetRandomUser`) để ánh xạ sang tên tài khoản quản trị thực tế (`admin1`) và đồng bộ `EffectiveUsername()` cho cả RTSP/Snapshot.
   - Đối với URL `http://`, `doXiongmaiHTTP` gửi trực tiếp qua `doRawHTTP10` nguyên tử (1 request = 1 response), xử lý triệt để lỗi dấu cách đầu dòng `" Content-type: text/plain\r\n"` của `uc-httpd 1.0.0` mà không bao giờ gửi lặp 2 lần các lệnh CGI có trạng thái (`GetSalt`, `Login`, `SetConfig`, `websocket.cgi`).
---

## 3. Hồ Sơ Thiết Bị Thực Tế Tại Node `inut_208_4` (Nhà Yến Anh Hùng Gò Vấp)

### 3.1 Thông Tin Đầu Ghi & Tài Khoản Quản Trị (`192.168.77.138`)
- **Model / Phần cứng:** `NBD80S16S-KL` (`16` kênh IP `DigChannel: 16`, `VideoInChannel: 0`)
- **Firmware:** `V4.03.R11.C6380233.12201.140000.0000000` (Build `2023-04-14 13:11:09`)
- **SerialNo:** `c1beba4793bccbdapnx9` (Khóa AES `GetRandomUser`: `a4793b1beba493bc`)
- **MAC:** `00:12:43:aa:de:9e` · **Cổng:** HTTP `80`, Sofia TCP `34567`, RTSP `554`
- **Ổ cứng (`StorageInfo`):** `0x000E8E0D` ($\approx 953.8\text{ GB}$ / $1\text{ TB}$), đang ghi hình bình thường (`Status: 0`).
- **Danh sách tài khoản quản trị (Group `admin`, đủ 55 quyền + đồng bộ `System.ExUserMap` cho RTSP `:554`):**

| Vai trò | Username | Password | Ghi chú |
|---|---|---|---|
| **Admin chính (`RandomUser`)** | `admin1` | `<REDACTED>` | Tự động điền trên Web UI & tự động ánh xạ khi `kspcam` cấu hình `username: admin` |
| **Admin dự phòng (Backup)** | `admin2` | `<REDACTED>` | Tài khoản dự phòng đồng bộ 55 quyền + `System.ExUserMap` cho RTSP `:554` |
| **Admin nội bộ / HDMI** | `pymid` | `<REDACTED>` | Tài khoản `AutoLogin` trên màn hình HDMI cục bộ |

### 3.2 Bảng Kênh Camera IP Con Trên Đầu Ghi `192.168.77.138` (`NetWork.RemoteDeviceV3`)

| Kênh | Tên | Trạng thái | Độ phân giải | IP Camera con | Cổng / Giao thức | Monitor Shinobi (`mid`) |
|---|---|---|---|---|---|---|
| **CH01** | `D01` | `Connected` | `1080P/D1` | `192.168.77.71` | `554` / `RTSP` | `camera01` |
| **CH02** | `D02` | `Offline` | `Unknown` | `192.168.77.218` | `554` / `RTSP` | `camera02` |
| **CH03** | `D03` | `Connected` | `1080P/704x576` | `192.168.77.34` | `554` / `RTSP` | `camera03` |
| **CH04** | `D04` | `Connected` | `1080P/720x576` | `192.168.77.146` | `554` / `RTSP` | `camera04` |
| **CH05** | `D05` | `Connected` | `1080P/D1` | `192.168.77.178` | `554` / `RTSP` | `camera05` |
| **CH06** | `D06` | `Connected` | `1080P/VGA` | `192.168.77.160` | `554` / `RTSP` | `camera06` |
| **CH07** | `D07` | `Connected` | `1080P/704x576` | `192.168.77.65` | `554` / `RTSP` | `camera07` |
| **CH08** | `D08` | `Connected` | `1080P/QVGA` | `192.168.77.205` | `554` / `RTSP` | `camera08` |
| **CH09** | `D09` | `Connected` | `1080P/VGA` | `192.168.77.84` | `554` / `RTSP` | `camera09` |
| **CH10** | `D10` | `Connected` | `1080P/VGA` | `192.168.77.147` | `554` / `RTSP` | `camera10` |
| **CH11** | `D11` | `Offline` | `Unknown` | `192.168.77.98` | `554` / `RTSP` | *(Chưa gán)* |
| **CH12–16** | `D12–16` | `NoConfig` | — | — | — | *(Chưa gán kênh)* |
