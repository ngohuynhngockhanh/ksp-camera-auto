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
| **Redbida `shinobi_camera_id`** | Giá trị là **Shinobi Group ID / Group Key** | `AWU8wJMd2l`, `P6zP1kVhht` | Mã nhóm Group ID của Shinobi NVR. |
| **Redbida `shinobi_group_key`** | Chuỗi 10 ký tự GroupKey Shinobi | `AWU8wJMd2l`, `P6zP1kVhht` | Lưu mã nhóm người dùng Shinobi. |
| **Redbida `shinobi_token`** | API Key IP `0.0.0.0` (Quyền: View Streams, View Videos, Snapshots) | `zd5DARMBYbos4CqoMlvDIafwBP6IR0` | Dùng cho khách hàng xem luồng trực tiếp và xem lại/tải video. |
| **Redbida `shinobi_monitor_token`** | API Key IP `0.0.0.0` (Quyền: Get Monitors, View Streams, View Videos) | `2Ow8jOi8MEwUfBByYruwgGapk2wHVL` | Dùng để lấy danh sách monitors và cấu hình luồng camera. |
| **Redbida `ui_tabs_links`** | Chuẩn INI 20 section `[C01]` .. `[C20]` (4 dòng/section) | Xem chi tiết Section 4 bên dưới | Dòng 3 `vid_play_label` = `<ui_title>` (Tên quán). |
| **Redbida `video_config`** | Chuỗi cấu hình video playback | `range=72` | Giới hạn tra cứu lịch sử highlight/video 72 giờ. |
| **Redbida `custom_hashtags`** | Chuỗi hashtag định dạng chuẩn | `#<UITitleNoSpaces> #BILLIARDSlive #INUTlive #highlightsports` | Tên quán không dấu không cách + 3 hashtag chuẩn. |
| **Redbida `ui_bg`** | CSS background gradient data | `radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, ... )` | Màu gradient CSS riêng biệt, sang trọng cho từng quán (**KHÔNG có dấu chấm phẩy `;` ở cuối**). |
| **Redbida `camera_count`** | Số lượng camera Shinobi thực tế | `5`, `8`, `10` | Bắt buộc bằng số lượng camera active trên Shinobi. |
| **Redbida `toolbar_show_count`** | Số nút camera hiển thị trên thanh công cụ | `5`, `8`, `10` | Luôn đặt cùng giá trị với `camera_count`. |
| **Redbida `hls_using_go2rtc`** | Bật phân phối HLS qua Go2RTC | `true` | Tối ưu hóa độ trễ stream và giảm tải CPU. |
| **Redbida `button_generate_go2rtc_stream`** | Nút trigger sinh cấu hình Go2RTC | `true` | Gửi qua `/private/i_sets` để Node-RED 2023 tự sinh `/root/go2rtc.yaml`. |

---

## 2. Quy tắc kế thừa Golden Template từ `Camera01`

`Camera01` (`mid: camera01`) được chỉ định là **chuẩn mẫu (Golden Standard)** cho mọi camera tiếp theo:

### A. Stream & Codec Engine (Remux Copy - 0% CPU Transcoding)
- `mode`: `"record"`
- `stream_type`: `"hls"`
- `stream_vcodec`: `"copy"`
- `vcodec`: `"copy"`
- `record_vcodec`: `"copy"`
- `cutoff`: `"5"` (**BẮT BUỘC: 5 phút / segment file**, `segment_time 300`)
- `rtsp_transport`: `"tcp"`
- `preset_stream`: `"ultrafast"`
- `hls_time`: `"2"`
- `hls_list_size`: `"2"`

### B. Quy tắc xử lý Âm thanh (Audio Codec & Probe Workflow):
- **Quy trình bắt buộc khi thêm/cài đặt Camera vào hệ thống**:
  1. **Bước 1 (Probe Audio)**: Thăm dò luồng/cấu hình âm thanh của camera (`audioEnable`, `audioCodec`, hoặc qua `ffprobe`).
  2. **Bước 2 (Chuyển đổi sang AAC)**: Nếu phát hiện âm thanh đang ở codec khác AAC (`pcm_alaw`, `pcm_mulaw`, `G.711A`, `G.711U`, `PCM`...), hệ thống **BẮT BUỘC** phải thử gửi lệnh cấu hình chuyển encoder âm thanh của camera về `AAC` (`Audio.Compression=AAC` / `SetAudioAAC: true`).
  3. **Bước 3 (Đọc lại & Phân nhánh cấu hình Shinobi)**:
     - **Nếu chuyển thành công sang AAC** (hoặc camera vốn đã có AAC):
       - `acodec`: `"copy"`
       - `stream_acodec`: `"copy"`
       - `record_acodec`: `"aac"`
     - **Nếu KHÔNG sửa được về AAC** (firmware/phần cứng camera không hỗ trợ encoder AAC, read-back vẫn giữ non-AAC hoặc không có mic):
       - `acodec`: `"no"`
       - `stream_acodec`: `"no"`
       - `record_acodec`: `"no"` (Tắt toàn bộ copy âm thanh trên Shinobi để chống lỗi luồng / giật lag).

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

---

## 4. Đặc tả Chuẩn INI `ui_tabs_links` & Redbida Keys

File `/root/ota-mqtt/change_ok/ui_tabs_links` bắt buộc định dạng INI với đúng 20 section từ `[C01]` đến `[C20]` (2 chữ số đệm 0). Mỗi section gồm 4 dòng:
```ini
[C01]
stream_label=Video Trực tiếp
vid_list_label=Danh sách highlight
vid_play_label=<ui_title>
list_refresh_label=Cập nhật highlight

[C02]
stream_label=Video Trực tiếp
vid_list_label=Danh sách highlight
vid_play_label=<ui_title>
list_refresh_label=Cập nhật highlight
...
[C20]
stream_label=Video Trực tiếp
vid_list_label=Danh sách highlight
vid_play_label=<ui_title>
list_refresh_label=Cập nhật highlight
```
*Lưu ý: Dòng 3 `vid_play_label` bắt buộc bằng giá trị của `<ui_title>` (Tên quán, ví dụ `CX King Luxury` hoặc `SD Billiards Club - CS2`).*

---

## 5. NAT Port Fallback khi VPN đứt
- `inut_204_164` (CX King Luxury): `video.io.vn:45529`
- `inut_204_163` (SD Billiards Club - CS2): `video.io.vn:45528`
- Sử dụng qua Ansible inventory: `inut_204_164 ansible_host=video.io.vn ansible_port=45529` / `inut_204_163 ansible_host=video.io.vn ansible_port=45528`.

---

## 6. Quy tắc Giao tiếp & Đồng bộ Cấu hình Redbida (MQTT-Only)
- **Tuyệt đối KHÔNG ghi đè trực tiếp file vào `/root/ota-mqtt/change_ok/`**.
- Mọi thao tác cấu hình/đồng bộ thông số quán và Redbida **BẮT BUỘC** phải:
  1. Kết nối tới MQTT Broker cục bộ `127.0.0.1:12369`.
  2. Đẩy gói tin cập nhật JSON `{"key": "<key_name>", "value": "<val>"}` lên topic `/private/i_sets`.
  3. Lắng nghe phản hồi xác nhận từ topic `/private/i_sets/ack`.
  4. Đọc lại (read-back) qua topic `/private/i_gets` và đối chiếu kết quả trả về trên `/private/i_gets/ack`.
  5. `ota-mqtt` và Node-RED sẽ tự động nhận diện thay đổi, lưu trữ nhất quán và đồng bộ hai chiều với cloud.
- **Quản lý FRP / Subdomain qua key `frpc_config`**:
  - Tuyệt đối **KHÔNG** thao tác trực tiếp vào `/tmp/frpc.ini` hoặc tự restart process frpc thủ công.
  - Khi cần bổ sung proxy/subdomain, chỉ cần cập nhật nội dung vào key **`frpc_config`** của Redbida qua MQTT `/private/i_sets`. Hệ thống sẽ tự động xử lý và duy trì kết nối frpc an toàn.

---

## 7. Cài đặt & Quản lý Tiến trình Go2RTC qua PM2

Kiểm tra kiến trúc CPU thiết bị (`uname -m`):
- **64-bit (`aarch64` / `arm64`)**:
  ```bash
  cd ~
  wget https://inut-vip-pro-dl.a.inut.vn/go2rtc_linux_arm64 -O go2rtc_linux_arm
  chmod +x go2rtc_linux_arm
  pm2 start ./go2rtc_linux_arm -e /dev/null -o /dev/null
  pm2 save
  ```
- **32-bit (`armv7l` / `armv7`)**:
  ```bash
  cd ~
  wget https://inut-vip-pro-dl.a.inut.vn/go2rtc_linux_arm -O go2rtc_linux_arm
  chmod +x go2rtc_linux_arm
  pm2 start ./go2rtc_linux_arm -e /dev/null -o /dev/null
  pm2 save
  ```
*Sau khi cài đặt xong, đẩy key `button_generate_go2rtc_stream: "true"` qua MQTT `/private/i_sets` để Node-RED :2023 sinh `/root/go2rtc.yaml`, sau đó gọi `pm2 restart go2rtc_linux_arm`.*




