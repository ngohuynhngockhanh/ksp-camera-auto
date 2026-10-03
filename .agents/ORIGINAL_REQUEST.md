# Original User Request

## Request — 2026-10-03
User request:
"5.67 mình mới cài thêm camera11 vo shinobi bạn sửa thanh 2mp 2048kbps aac h264 giúp mình nhé, 10 cam trước đó để yên"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_205_67` (`77.88.205.67`) — BinK Billiards Club (group `billiardclub`)
- Target Camera: `camera11` (`192.168.10.21:37777`)
- Strict Constraint: Giữ nguyên 100% cấu hình của 10 camera trước đó (`camera01` .. `camera10`).

## Requirements & Implementation Results

### R1. Khảo sát & cấu hình thông số Camera11 theo yêu cầu [HOÀN THÀNH]
- **Thông số yêu cầu**:
  - Độ phân giải: **2MP (1920x1080)**
  - Tốc độ bit: **2048 kbps VBR**
  - Chuẩn nén Video: **H.264**
  - Chuẩn nén Âm thanh: **AAC**
  - Tốc độ khung hình / GOP: **25 fps / GOP 50**
- **Thực thi trên camera**:
  - Thêm `192.168.10.21:37777` vào `/opt/ksp-cam/cameras.yaml`.
  - Tắt `anti_a` trong `config.yaml` để tránh Anti-A tự động khóa H.265.
  - Áp dụng cấu hình qua `kspcam_apply_profile` và xác nhận đọc lại (Read-back verification) thành công:
    - Codec: H.264 OK
    - Resolution: 1920x1080 OK
    - Bitrate: 2048 Kbps VBR OK
    - FPS / GOP: 25 fps / GOP 50 OK
    - SmartCodec: False OK
    - Audio: AAC OK

### R2. Cập nhật cấu hình Shinobi Monitor cho `camera11` [HOÀN THÀNH]
- Cập nhật monitor `camera11` trong cơ sở dữ liệu Shinobi:
  - `vcodec`: `"copy"`
  - `acodec`: `"copy"`, `stream_acodec`: `"copy"`, `record_acodec`: `"aac"`
  - `cust_record`: `""` (bỏ cờ `-tag:v hvc1` vì đây là luồng H.264)
  - `wall_clock_timestamp_ignore`: `"0"`
  - `hls_time`: `"2"`

### R3. Bảo tồn nguyên vẹn 10 camera trước đó (`camera01` .. `camera10`) [HOÀN THÀNH]
- Đã probe và kiểm tra 10 camera trước:
  - Cả 10 camera giữ nguyên 100% cấu hình: **2560x1440 (4MP) @ 25fps, H.265, 1536 kbps VBR, GOP 50, SmartCodec: True, Audio: AAC**.

### R4. Nghiệm thu ghi hình thực tế [HOÀN THÀNH]
- `camera11` đang ghi hình file `.mp4` đều đặn trong `/media/usb1/billiardclub/camera11/`.
- Dùng `ffprobe` kiểm tra trực tiếp file ghi hình hoàn chỉnh:
  - Video stream: `codec_name=h264`, `width=1920`, `height=1080`, `bit_rate=1052415`
  - Audio stream: `codec_name=aac`, `bit_rate=17353`
  - Phát mượt mà, đúng chuẩn container MP4.

## Acceptance Criteria
- [x] C1: Camera11 được cấu hình đúng 2MP (1920x1080), 2048kbps, H.264, AAC.
- [x] C2: 10 camera trước đó được giữ nguyên vẹn 100%.
- [x] C3: Cấu hình Shinobi của camera11 được cập nhật đúng chuẩn H.264 copy.
- [x] C4: File video MP4 mới của camera11 sinh đều đặn và probe hợp lệ.
