# Đặc Tả Giao Thức TVT Digital / Secuvision (`internal/tvt` — NVMS-9000 XML, ONVIF Profile S/G & RTSP Replay)

Tài liệu này mô tả chi tiết giao thức truyền thông của dòng đầu ghi NVR & Camera IP **Shenzhen TVT Digital Technology** (`NVMS-9000`, `TD-33xx`, `TD-31xx`) cùng các thương hiệu OEM (**Secuvision Australia** `NVR-0804B1-4P`, `NVR-0816B2-16P`; **Speco Technologies N9K**), đã được xác minh trực tiếp trên phần cứng thực tế tại `inut_204_61` (`Celestial Billiards`).

---

## 1. Nhận Diện Thiết Bị & Đường Dẫn RTSP Chuẩn

- **Mã Vendor trong `kspcam`**: `tvt` (`config.VendorTVT`), cổng điều khiển HTTP mặc định: **`80`**, cổng RTSP mặc định: **`554`**.
- **Nhận diện không cần xác thực (`Pre-login Probe`)**:
  - `POST http://<host>:80/reqLogin` với body:
    ```xml
    <?xml version="1.0" encoding="utf-8" ?>
    <request version="1.0" systemType="NVMS-9000" clientType="WEB"><token>null</token></request>
    ```
  - Thiết bị TVT trả về HTTP `200` với `<response version="1.0" systemType="NVMS-9000"><status>success</status><content><nonce>...</nonce><sessionId>{...}</sessionId></content></response>`.
- **Đường dẫn RTSP Trực tiếp (Live Stream, kênh `1-based`)**:
  - Luồng chính (Main): `rtsp://<user>:<pass>@<host>:554/chID=<1..N>&streamType=main&linkType=tcp`
  - Luồng phụ (Sub): `rtsp://<user>:<pass>@<host>:554/chID=<1..N>&streamType=sub&linkType=tcp`
  - *Lưu ý Importer*: Chuỗi `chID=2&streamType=main&linkType=tcp` nằm trực tiếp trong `URL.Path` (không có dấu `?`), nên hàm `tvt.ParseChannelFromRTSP` dùng biểu thức chính quy `(?i)(?:^|[/?&])chid=(\d+)` để bóc tách kênh chính xác.
- **Đường dẫn RTSP Phát lại (Replay / Hybrid Clip Export)**:
  - `rtsp://<user>:<pass>@<host>:554/chID=<1..N>&date=YYYY-MM-DD&time=HH:MM:SS&timelen=<durSec+5>&streamType=main&linkType=tcp`
  - `date` và `time` tính theo **giờ địa phương của đầu ghi NVR** (được `tvt.Client.GetNVRTimezoneLocation` xác định tự động bằng hiệu số `LocalDateTime - UTCDateTime` từ ONVIF `GetSystemDateAndTime`).
  - Tham số `timelen` luôn được cộng dư `+5` giây so với cờ `-t <durSec>` của FFmpeg, kết hợp cờ `-stimeout 8000000` (8 giây) trên demuxer RTSP để tránh treo kết nối TCP khi kết thúc đoạn ghi hình theo chuyển động (Motion Recording).

---

## 2. Kiến Trúc Điều Khiển Kép (`Dual-Plane Control`)

### 2.1 Tầng 1 — Giao Thức Web XML Nguyên Bản `NVMS-9000` (`internal/tvt/xml_plane.go`)
1. **Xác thực 2 bước**:
   - Bước 1 (`POST /reqLogin`): Nhận `nonce`, `sessionId` (cắt bỏ dấu `{}`), và `token`.
   - Bước 2 (`POST /doLogin`):
     - Tính `md5Pw = hex_lower(MD5(password))`
     - Tính `sha512Pw = hex_lower(SHA512(md5Pw + "#" + nonce))`
     - Gửi kèm header `Cookie: sessionId=<sessionId>` và XML `<content><userName><![CDATA[...]]></userName><password><![CDATA[<sha512Pw>]]></password></content>`.
     - Giải mã `sessionKey` (nếu có) bằng `AES-128-ECB` (`ZeroPadding`) với khóa `md5Pw` (32 byte ASCII UTF-8 hoặc 16 byte).
   - Tự động đăng nhập lại khi gặp mã lỗi hết hạn phiên `536871011` (`IDCS_LOGIN_OVERTIME`), và giải phóng slot phiên bằng `POST /doLogout` khi `Close()`.
2. **Các Endpoint XML chính**:
   - Thông tin phần cứng & Serial: `POST /queryBasicCfg`
   - Danh sách kênh camera IP: `POST /queryDevList`
   - Cấu hình luồng chính / phụ: `POST /queryNodeEncodeInfo`, `POST /editNodeEncodeInfo`, `POST /queryNetworkNodeEncodeInfo`, `POST /editNetworkNodeEncodeInfo`
   - Tên kênh & OSD: `POST /queryIPChlORChlOSD`, `POST /editDev`, `POST /editChlWaterMark`
   - Ổ cứng lưu trữ: `POST /queryDiskInfo`
   - Chụp ảnh nhanh: `POST /snapChlPicture`
   - Tra cứu lịch sử ghi hình: `POST /queryChlRecLog`

### 2.2 Tầng 2 — Chuẩn Mở `ONVIF Profile S + Profile G` (`internal/tvt/onvif_plane.go`)
Khi tài khoản cấu hình trên Box là tài khoản phân quyền riêng cho luồng media/ONVIF (như tài khoản `rtsp` trả mã `536870948` trên `/doLogin`), `tvt.Client` tự động chuyển sang giao thức SOAP `WS-UsernameToken PasswordDigest`:
- `/onvif/device_service`: `GetDeviceInformation` (Model, Firmware, SerialNumber), `GetSystemDateAndTime` (tự động đồng bộ múi giờ NVR), `SystemReboot`.
- `/onvif/Media2` & `/onvif/Media1`: `GetVideoEncoderConfigurations` (đọc chính xác `H265` vs `H264`, `2880x1620`, `GovLength`, `BitrateLimit`, `FrameRateLimit`) và `GetSnapshotUri`.
- `/onvif/Recording` & `/onvif/SearchRecording` (**ONVIF Profile G**):
  - `GetRecordings` (`Record_<ch>_0`) để liệt kê toàn bộ kênh ghi hình trên NVR.
  - `GetRecordingInformation` kiểm tra tức thì khoảng lưu trữ `[EarliestRecording, LatestRecording]` trong `~25ms` (phục vụ truy vấn Hybrid 72h với độ phức tạp `O(1)`).
  - `FindEvents` + `GetEventSearchResults` lọc theo chủ đề `tns1:RecordingHistory/Recording/State` (`IsRecording=true/false`) để trả về từng phân đoạn ghi hình chính xác tới từng giây cho thanh timeline ngắn hạn (`<= 6h`), tự động gọi `EndSearch` chống rò rỉ token tìm kiếm trên NVR.
