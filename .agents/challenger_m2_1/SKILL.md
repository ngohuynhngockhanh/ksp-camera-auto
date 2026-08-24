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
| **Redbida `logo_header`** | URL logo header chuẩn | `https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png` | URL ảnh logo header cố định cho hệ thống Bida. |
| **Redbida `logo_header_text`** | Chuỗi tiêu đề phụ / header text | `Billiard Live - Tải clip bàn bida và livestream` | Luôn cố định câu slogan chuẩn này cho toàn bộ hệ thống Bida. |
| **Redbida `button_generate_go2rtc_stream`** | Nút trigger sinh cấu hình Go2RTC | `true` | Gửi qua `/private/i_sets` để Node-RED 2023 tự sinh `/root/go2rtc.yaml`. |
