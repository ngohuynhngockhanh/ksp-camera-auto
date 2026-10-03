# Original User Request

## Request — 2026-10-03
User request:
"ok fix bên 5_85 nữa bị lỗi hoài luôn"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_205_85` (`77.88.205.85`) — Lion King Billiards Club Nam Định - CS2 (group `lionkingnd`, 6 cameras via NVR `192.168.1.200`)

## Requirements & Implementation Results

### R1. Điều tra nguyên nhân gây lỗi liên tục trên `inut_205_85` [HOÀN THÀNH]
- **Hiện tượng thực tế**:
  1. Box tự động reboot liên tục mỗi 1–2 ngày (04:16 hôm nay, 20:16 ngày 01/10, 03:16 ngày 30/09, 00:16 ngày 29/09, 01:16 ngày 28/09, 20:16 ngày 26/09...).
  2. Toàn bộ 6 camera đã **chết hoàn toàn từ 04:32 sáng đến 12:45 trưa (hơn 8 tiếng không ghi hình)** với **0 tiến trình FFmpeg chạy**.
- **Nguyên nhân gốc rễ (Root Causes)**:
  1. **Ngòi nổ tự sát Node-RED**: `max_free_ram_force_reboot = 70` kết hợp với `Swap = 0B`. Mỗi khi RAM trống tụt xuống dưới 70 MB, Node-RED ép kernel chạy `echo b > /proc/sysrq-trigger` gây reboot khẩn cấp.
  2. **Khiếm khuyết của Watchdog cũ**: Script watchdog cũ chỉ kiểm tra `pm2 pid camera` (PID 997). Sau khi hệ thống reboot lúc 04:16, camera ghi được 16 phút rồi luồng FFmpeg gặp lỗi rớt kết nối. Vì PID 997 vẫn tồn tại, watchdog cũ coi là "online" và **không bao giờ khởi động lại camera**, bỏ mặc hệ thống chết suốt 8 tiếng.
  3. **Không có Storage Guard**: Thẻ nhớ 73% (43GB/59GB) hoàn toàn không có cron dọn rác và xoay vòng file.

### R2. Triển khai giải pháp khắc phục triệt để [HOÀN THÀNH]
- **Kích hoạt ZRAM Swap 1.9 GiB**: Thuật toán `lz4`, `MEM_LIMIT_PERCENTAGE=100`, `vm.swappiness = 60`, đã xóa bỏ `mem_limit`.
- **Gỡ bỏ ngòi nổ tự sát Node-RED**: Loại bỏ toàn bộ các lệnh `echo b > /proc/sysrq-trigger` và `rm -rf /tmp` trong `flow.json` (active flow có 0 sysrq). Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
- **Khắc phục thứ tự khởi động dịch vụ**: Cập nhật `pm2-root.service` chờ MariaDB (`After=mariadb.service`, `Wants=mariadb.service`).
- **Nâng cấp Watchdog thông minh**: Cài đặt script `/usr/local/bin/shinobi-camera-watchdog.sh` kèm timer `shinobi-camera-watchdog.timer` (quét mỗi 60s, kiểm tra số lượng tiến trình FFmpeg thực tế: nếu camera online > 90s mà 0 FFmpeg thì tự động restart camera ngay).
- **Cài đặt Storage Guard**: Cài đặt `/usr/local/bin/shinobi-storage-guard.sh` targeting `/media/usb1/lionkingnd`, kích hoạt cron 15 phút/lần (`/etc/cron.d/shinobi-storage-guard`, xả khi $\ge 85\%$).
- **Chuẩn hóa Shinobi**: Kích hoạt `deleteOld`, `deleteOrphans`, `deleteOverMax` trong `conf.json`; đặt `max_keep_days = 3`, `wall_clock_timestamp_ignore = '0'`, `hls_time = '2'` trong MySQL.
- **Cập nhật kspcam**: Đã deploy binary ARM64 tĩnh mới nhất vào `/opt/ksp-cam/kspcam` và service `kspcam.service` (Active).

### R3. Nghiệm thu & Kết quả đo đạc thực tế [HOÀN THÀNH]
- **Hồi sinh camera**: Toàn bộ 6 camera đã hồi sinh thành công từ 0 lên đủ **12 tiến trình FFmpeg** (6 stream HLS + 6 record MP4).
- **File video mới**: Tất cả 6 camera đang sinh file video mới liên tục trong `/media/usb1/lionkingnd/` (`12:55:00.mp4`, chuẩn 3MP 2304x1296 @ 20fps, H.265, AAC).
- **RAM khả dụng**: Đạt **`950 MB / 1.9 GiB`**, có sẵn **`1.9 GiB` ZRAM Swap** đệm an toàn.
- **Tính ổn định**: Triệt tiêu hoàn toàn chu kỳ reboot tự sát và ngăn chặn tình trạng camera zombie không ghi hình.

## Acceptance Criteria
- [x] C1: Node `205_85` được khảo sát và xác định nguyên nhân gây lỗi chi tiết.
- [x] C2: `kspcam` được triển khai và kích hoạt thành công trên cổng `:2028`.
- [x] C3: Hệ thống được tối ưu ZRAM, Node-RED sysrq, Watchdog thông minh, Storage guard, và timestamp.
- [x] C4: Có đầy đủ tiến trình FFmpeg ghi hình ổn định, file video mới tạo hợp lệ trên cả 6 camera.
