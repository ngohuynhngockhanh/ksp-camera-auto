# Original User Request

## Request — 2026-10-01
User request:
"Ok 5_77 nữa"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_205_77` (`77.88.205.77`) — ZONE+ Billiards Center (group `toktok3`)

## Requirements

### R1. Khảo sát hiện trạng & kiểm tra dịch vụ trên `inut_205_77` [HOÀN THÀNH]
- Đã kiểm tra chi tiết trên node `205_77`:
  - Box H313 (ARM64), 1.9GB RAM, Swap = 0B.
  - Vừa bị reboot lúc 15:17 chiều nay do cạn RAM và kích hoạt 4 node sysrq trong Node-RED.
  - Thẻ nhớ 76% (46GB/60GB) không có Storage Guard.
  - Không có Watchdog timer tự hồi sinh camera.

### R2. Đóng gói & Triển khai `ksp-cam` lên `inut_205_77` [HOÀN THÀNH]
- Đã triển khai binary tĩnh `kspcam` ARM64 mới nhất.
- Đã kích hoạt thành công dịch vụ `kspcam.service` trên cổng `:2028`.

### R3. Tối ưu hóa hệ thống toàn diện chống crash / OOM / tràn thẻ nhớ [HOÀN THÀNH]
- Kích hoạt phân vùng ZRAM Swap 1.9 GiB nén `lz4` trên 4 core CPU, gỡ bỏ `mem_limit`, tối ưu `vm.swappiness = 60`.
- Gỡ bỏ hoàn toàn 4 node `echo b > /proc/sysrq-trigger` trong Node-RED và hạ ngưỡng cảnh báo RAM về 60/80/100.
- Khắc phục thứ tự khởi động `pm2-root.service` chờ MariaDB (`After=mariadb.service`).
- Cài đặt Watchdog thông minh `shinobi-camera-watchdog` (quét mỗi 60s, tự cứu nếu camera zombie).
- Cài đặt Van xả thẻ nhớ tự động `shinobi-storage-guard.sh` (cron 15 phút/lần, dọn rác 0-byte, xoay vòng khi $\ge 85\%$).
- Chuẩn hóa cấu hình Shinobi (`wall_clock_timestamp_ignore = "0"`, `hls_time = "2"`, `max_keep_days = 3`).

### R4. Nghiệm thu & Xác nhận hoạt động [HOÀN THÀNH]
- `kspcam` (:2028) active.
- Toàn bộ camera đang ghi hình đầy đủ (15–24 tiến trình FFmpeg hoạt động liên tục, sinh file mới đều đặn).

## Acceptance Criteria
- [x] C1: Node `205_77` được khảo sát và ghi nhận hiện trạng chi tiết.
- [x] C2: `kspcam` được triển khai và kích hoạt thành công trên cổng `:2028`.
- [x] C3: Hệ thống được tối ưu ZRAM, Node-RED sysrq, Watchdog, Storage guard, và timestamp.
- [x] C4: Có đầy đủ tiến trình FFmpeg ghi hình ổn định, file video mới tạo hợp lệ.
