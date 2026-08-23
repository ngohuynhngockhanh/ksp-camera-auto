---
name: camera-naming
description: >-
  Quy tắc chuẩn hóa đặt tên Camera, Monitor ID, Device ID và kế thừa cấu hình chuẩn mẫu (Golden Template) từ Camera01 cho ksp-camera-auto và Shinobi NVR.
---

# Camera & Shinobi Monitor Naming & Template Standard

Kỹ năng này hướng dẫn quy tắc đặt tên, quản lý định danh và kế thừa cấu hình chuẩn từ `Camera01` sang các camera trong hệ thống `ksp-camera-auto` và **Shinobi NVR**.

---

## 1. Quy tắc đặt tên và định danh (Naming Conventions)

| Đối tượng | Quy ước định dạng | Ví dụ chuẩn | Lưu ý |
|---|---|---|---|
| **Tên hiển thị Camera (Name)** | `CameraXX` (Title case, đệm 2 số) | `Camera01`, `Camera02`, `Camera08` | Không có dấu cách, chữ `C` viết hoa. |
| **Shinobi Monitor ID (`mid`)** | `cameraXX` (Lowercase, đệm 2 số) | `camera01`, `camera02`, `camera08` | Chữ thường toàn bộ, định danh duy nhất trên Shinobi NVR. |
| **KSP-Cam Inventory ID** | `<ip_address>:<port>` | `192.168.1.195:37777` | Cổng cấu hình DVRIP (`37777`), ISAPI (`80`), hoặc SDK (`8000`). |
| **RTSP Main-stream Path** | `rtsp://<user>:<pass>@<ip>:554/cam/realmonitor?channel=1&subtype=0` | `rtsp://admin:Admin123456@192.168.1.195:554/cam/realmonitor?channel=1&subtype=0` | Transport bắt buộc `tcp`, codec remux `copy`. |
| **RTSP Sub-stream Path** | `rtsp://<user>:<pass>@<ip>:554/cam/realmonitor?channel=1&subtype=1` | `rtsp://admin:Admin123456@192.168.1.195:554/cam/realmonitor?channel=1&subtype=1` | Dùng cho preview / thumbnail hoặc low bandwidth. |
| **Shinobi Notes Metadata** | JSON format: `{"sn": "...", "safety_code": "..."}` | `{"sn": "33443ACPSFC97C2", "safety_code": "L2D6643F"}` | Lưu trữ Serial Number và Safety Code gốc của hãng. |

---

## 2. Quy tắc kế thừa Golden Template từ `Camera01`

`Camera01` (`mid: camera01`) được chỉ định là **chuẩn mẫu (Golden Standard)** cho mọi camera tiếp theo:

### A. Stream & Codec Engine (Remux Copy - 0% CPU Transcoding)
- `mode`: `"record"`
- `stream_type`: `"hls"`
- `stream_vcodec`: `"copy"`
- `vcodec`: `"copy"`
- `record_vcodec`: `"copy"`
- `rtsp_transport`: `"tcp"`
- `preset_stream`: `"ultrafast"`
- `hls_time`: `"2"`
- `hls_list_size`: `"2"`

### B. Quy tắc xử lý Âm thanh (Audio Codec Rule):
- **Nếu camera CÓ âm thanh AAC** (`audioEnable: true` và `audioCodec == "AAC"`):
  - `acodec`: `"copy"`
  - `stream_acodec`: `"copy"`
  - `record_acodec`: `"aac"`
- **Nếu camera KHÔNG có âm thanh AAC** (hoặc tắt mic/audio):
  - `acodec`: `"no"`
  - `stream_acodec`: `"no"`
  - `record_acodec`: `"no"`

### C. Quy tắc Flags (FFmpeg Flags Standard):
- `cust_input`: `""` (**BẮT BUỘC ĐỂ TRỐNG** — không chèn cờ `-fflags nobuffer...` hay `-flags low_delay...`).
- `cust_stream`: `""` (**BẮT BUỘC ĐỂ TRỐNG** — không chèn cờ `-hls_flags program_date_time...`).
- `cust_record`: `"-tag:v hvc1"` (**BẮT BUỘC** cho chuẩn H.265 để container MP4 gắn đúng fourcc `hvc1` phát mượt trên Apple/Web).

### D. Watchdog & Giám sát:
- `watchdog_reset`: `"1"`
- `signal_check`: `"10"`

---

## 3. Danh sách 8 Camera Chuẩn hóa Mẫu

| STT | Monitor ID (`mid`) | Tên (`Name`) | Địa chỉ IP | Serial Number (`SN`) | Safety Code | Mật khẩu |
|---|---|---|---|---|---|---|
| 1 | `camera01` | `Camera01` | `192.168.1.195` | `33443ACPSFC97C2` | `L2D6643F` | `Admin123456` |
| 2 | `camera02` | `Camera02` | `192.168.1.191` | `33443ACPSFF371A` | `L2A9991E` | `Admin123456` |
| 3 | `camera03` | `Camera03` | `192.168.1.192` | `33443ACPSFAA7D5` | `L251423F` | `Admin123456` |
| 4 | `camera04` | `Camera04` | `192.168.1.194` | `33443ACPSF5D901` | `L2AAA219` | `Admin123456` |
| 5 | `camera05` | `Camera05` | `192.168.1.190` | `0731FACPSF85A97` | `L28BF007` | `Admin123456` |
| 6 | `camera06` | `Camera06` | `192.168.1.197` | `0731FACPSF4A471` | `L250833C` | `Admin123456` |
| 7 | `camera07` | `Camera07` | `192.168.1.196` | `33443ACPSF294DB` | `L22F39D1` | `Admin123456` |
| 8 | `camera08` | `Camera08` | `192.168.1.193` | `33443ACPSF01EB0` | `L21F07E0` | `Admin123456` |
