# Camera & Shinobi Monitor Naming & Template Standard (Local Copy)

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
