# Original User Request

## Request — 2026-10-01
User request:
"Ok 4_148 nữa"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_204_148` (`77.88.204.148`) — RUBY BILLIARD CLUB (group `na`)

## Requirements

### R1. Khảo sát hiện trạng & kiểm tra dịch vụ trên `inut_204_148` [HOÀN THÀNH]
- Đã kiểm tra chi tiết trên node `204_148`:
  - Box H313 (ARM64), 1.9GB RAM, Swap = 0B.
  - Từng reboot lúc 07:16 sáng nay do cạn RAM và kích hoạt 6 node sysrq trong Node-RED.
  - Thẻ nhớ 69% (40GB/58GB) không có Storage Guard cron.
  - Watchdog cũ chỉ kiểm tra PID mà không kiểm tra số lượng tiến trình FFmpeg thực tế.

### R2. Đóng gói & Triển khai `ksp-cam` lên `inut_204_148` [HOÀN THÀNH]
- Đã cập nhật binary tĩnh `kspcam` ARM64 mới nhất.
- Dịch vụ `kspcam.service` trên cổng `:2028` đang active và quản lý 7 camera Hikvision.

### R3. Tối ưu hóa hệ thống toàn diện chống crash / OOM / tràn thẻ nhớ [HOÀN THÀNH]
- Kích hoạt phân vùng ZRAM Swap 1.9 GiB nén `lz4` trên 4 core CPU, gỡ bỏ `mem_limit`, tối ưu `vm.swappiness = 60`.
- Gỡ bỏ hoàn toàn 6 node `echo b > /proc/sysrq-trigger` trong Node-RED và hạ ngưỡng cảnh báo RAM về 60/80/100.
- Khắc phục thứ tự khởi động `pm2-root.service` chờ MariaDB (`After=mariadb.service`).
- Nâng cấp Watchdog thông minh `shinobi-camera-watchdog` (quét mỗi 60s, kiểm tra tiến trình FFmpeg thực tế).
- Cài đặt Van xả thẻ nhớ tự động `shinobi-storage-guard.sh` (cron 15 phút/lần, dọn rác 0-byte, xoay vòng khi $\ge 85\%$).
- Chuẩn hóa cấu hình Shinobi (`wall_clock_timestamp_ignore = "0"`, `hls_time = "2"`, `max_keep_days = 3`).

### R4. Nghiệm thu & Xác nhận hoạt động [HOÀN THÀNH]
- `kspcam` (:2028) active.
- Toàn bộ camera đang ghi hình đầy đủ (12–13 tiến trình FFmpeg hoạt động liên tục, sinh file mới đều đặn).

## Acceptance Criteria
- [x] C1: Node `204_148` được khảo sát và ghi nhận hiện trạng chi tiết.
- [x] C2: `kspcam` được triển khai và kích hoạt thành công trên cổng `:2028`.
- [x] C3: Hệ thống được tối ưu ZRAM, Node-RED sysrq, Watchdog, Storage guard, và timestamp.
- [x] C4: Có đầy đủ tiến trình FFmpeg ghi hình ổn định, file video mới tạo hợp lệ.
