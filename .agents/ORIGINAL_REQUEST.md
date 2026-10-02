# Original User Request

## Request — 2026-10-02
User request:
"5_53 luôn nhé"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_205_53` (`77.88.205.53`) — The Best Billiards Club (group `ultrapool1`)
- Cameras: 12 IP cameras (`camera01` to `camera12`)

## Requirements & Implementation Results

### R1. Khảo sát hiện trạng & lỗ hổng hệ thống trên `inut_205_53` [HOÀN THÀNH]
- Đã khảo sát chi tiết:
  - Box Allwinner H313 (ARM64), 1.9 GB RAM vật lý, gánh 12 camera.
  - Swap: 0 Byte (`ENABLED=false`), Free RAM chỉ còn vỏn vẹn **41 MB**.
  - `wall_clock_timestamp_ignore: "1"` (FFmpeg thiếu cờ `-use_wallclock_as_timestamps 1` làm rò rỉ bộ nhớ queue).
  - Shinobi đang bật audio transcoding CPU (`acodec: "default"`, mux sang AAC trong khi camera gửi `pcm_mulaw`), ngốn CPU và RAM.
  - Node-RED có ngòi nổ `sysrq-trigger` khi RAM trống dưới 70 MB.
  - Chưa cài đặt `kspcam` (:2028) và chưa có Van xả thẻ nhớ tự động (Storage Guard).

### R2. Đóng gói & Triển khai `ksp-cam` (:2028) [HOÀN THÀNH]
- Đã triển khai binary ARM64 tĩnh mới nhất vào `/opt/ksp-cam/kspcam`.
- Tạo admin API key chuyên biệt cho IP `127.0.0.1` trong bảng `ccio.API`.
- Cấu hình `/opt/ksp-cam/config.yaml` và kích hoạt dịch vụ `kspcam.service` (Active).

### R3. Tối ưu hóa hệ thống toàn diện chống crash / OOM / tràn thẻ nhớ [HOÀN THÀNH]
- **Kích hoạt ZRAM Swap 1.9 GiB**: Nén `lz4`, `MEM_LIMIT_PERCENTAGE=100`, `vm.swappiness = 60`, đã xóa bỏ `mem_limit`.
- **Gỡ bỏ ngòi nổ tự sát Node-RED**: Loại bỏ toàn bộ các lệnh `echo b > /proc/sysrq-trigger` và `rm -rf /tmp` trong `flow.json`. Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
- **Khắc phục thứ tự khởi động dịch vụ**: Cập nhật `pm2-root.service` chờ MariaDB (`After=mariadb.service`, `Wants=mariadb.service`).
- **Nâng cấp Watchdog thông minh**: Cài đặt `/usr/local/bin/shinobi-camera-watchdog.sh` kèm timer `shinobi-camera-watchdog.timer` (quét mỗi 60s, kiểm tra số lượng tiến trình FFmpeg thực tế).
- **Cài đặt Storage Guard**: Cài đặt `/usr/local/bin/shinobi-storage-guard.sh` targeting `/media/usb1/ultrapool1`, kích hoạt cron 15 phút/lần (`/etc/cron.d/shinobi-storage-guard`).
- **Chuẩn hóa Shinobi & triệt tiêu CPU audio transcoding**:
  - `wall_clock_timestamp_ignore = "0"` $\rightarrow$ chèn `-use_wallclock_as_timestamps 1` ngăn chặn trôi timestamp và phình RAM.
  - `acodec = "no"`, `stream_acodec = "no"`, `record_acodec = "no"` $\rightarrow$ chèn cờ `-an`, loại bỏ hoàn toàn bộ chuyển đổi âm thanh `pcm_mulaw` sang AAC tốn CPU.
  - `hls_time = "2"`, `max_keep_days = 3`, bật các cờ dọn file mồ côi trong `conf.json`.

### R4. Nghiệm thu & Kết quả đo đạc thực tế [HOÀN THÀNH]
- **RAM khả dụng**: Tăng từ **41 MB** lên **882 MB – 1.1 GiB** (thông thoáng gấp 25 lần).
- **ZRAM Swap**: 1.9 GiB bảo vệ hệ thống tuyệt đối.
- **Tiến trình ghi hình**: Toàn bộ 11 camera đang online (`camera02` đến `camera12`) đều ghi hình mượt mà, file mới sinh đều đặn (`camera01` bị mất nguồn/mất kết nối vật lý tại cổng mạng LAN).

## Acceptance Criteria
- [x] C1: Node `205_53` được khảo sát và ghi nhận hiện trạng chi tiết.
- [x] C2: `kspcam` được triển khai và kích hoạt thành công trên cổng `:2028`.
- [x] C3: Hệ thống được tối ưu ZRAM, Node-RED sysrq, Watchdog, Storage guard, và timestamp.
- [x] C4: Có đầy đủ tiến trình FFmpeg ghi hình ổn định, file video mới tạo hợp lệ.
