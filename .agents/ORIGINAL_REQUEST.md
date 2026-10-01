# Original User Request

## Request — 2026-10-01
User request:
"Camera07 bạn đổi ip thành 1.65 thả xem đổi bên camera chính luôn cảm ơn"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_204_148` (`77.88.204.148`) — RUBY BILLIARD CLUB (group `na`)
- Target Camera: `Camera07` (Hikvision IP Camera `DS-2CD1143G2-LIU`, MAC `04:ee:cd:7b:e2:aa`)

## Requirements

### R1. Đổi địa chỉ IP của Camera07 trực tiếp trên phần cứng camera từ 192.168.1.64 sang 192.168.1.65 [HOÀN THÀNH]
- Gọi lệnh ISAPI `PUT /ISAPI/System/Network/interfaces/1/ipAddress` cấu hình IP tĩnh `192.168.1.65` (subnet `255.255.255.0`, gateway `192.168.1.1`).
- Thực hiện reboot camera để áp dụng IP mới (`/ISAPI/System/reboot`).
- Sau 26 giây, camera đã phản hồi ping thành công trên IP mới `192.168.1.65`.
- Kiểm tra ISAPI `deviceInfo` và `Network/interfaces/1/ipAddress` xác nhận chính xác model `DS-2CD1143G2-LIU`, MAC `04:ee:cd:7b:e2:aa` đang chạy trên `192.168.1.65`.

### R2. Cập nhật cấu hình Shinobi NVR cho Monitor `camera07` [HOÀN THÀNH]
- Cập nhật host sang `192.168.1.65`.
- Sửa lại đường dẫn RTSP từ `/Streaming/channels/1001` (kênh NVR sai) thành `/Streaming/channels/101` (luồng chính của camera độc lập).
- Chuyển chế độ hoạt động từ `stop` sang `record`.
- Đã kiểm tra ffprobe xác nhận luồng H.265 và âm thanh AAC hoạt động ổn định.

### R3. Cập nhật cấu hình `kspcam` (`/opt/ksp-cam/cameras.yaml`) [HOÀN THÀNH]
- Cập nhật thiết bị Camera07 sang `id: 192.168.1.65:80`, `host: 192.168.1.65`.
- Đã probe qua MCP xác nhận thông số live: 2560x1440 (2K), 20fps, H.265, AAC, GOP 50, Bitrate 3072 Kbps.

### R4. Nghiệm thu ghi hình thực tế [HOÀN THÀNH]
- Toàn bộ 7/7 camera (`camera01` đến `camera07`) đang chạy 14 tiến trình FFmpeg liên tục.
- Thư mục `/media/usb1/na/camera07/` đang sinh các file `.mp4` đều đặn.

## Acceptance Criteria
- [x] C1: Đổi IP trên phần cứng camera chính thành 192.168.1.65 thành công.
- [x] C2: Monitor camera07 trong Shinobi được trỏ đúng IP 1.65 và luồng 101.
- [x] C3: cameras.yaml và kspcam được cập nhật và probe thành công.
- [x] C4: Camera07 bắt đầu ghi hình ổn định cùng toàn bộ 6 camera còn lại.
