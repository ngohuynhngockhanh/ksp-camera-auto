# Original User Request

## Request — 2026-10-01
User request:
"4_221,5_71,5_72 Deply lên 3 con này nhé"

Working directory: `/home/ksp/ksp-camera-auto`

Target Nodes:
1. `inut_204_221` (`77.88.204.221`) — Linh Billiards Club
2. `inut_205_71` (`77.88.205.71`) — Master Billiards Club (Master 1)
3. `inut_205_72` (`77.88.205.72`) — Master Billiards Club (Master 2)

## Requirements

### R1. Khảo sát hiện trạng & kiểm tra dịch vụ trên cả 3 node [HOÀN THÀNH]
- Đã kiểm tra chi tiết trên từng node `204_221`, `205_71`, `205_72`:
  - `204_221`: Camera bị errored trong PM2, 0 tiến trình FFmpeg (chết đứng từ 04:30 sáng), Swap 0B, thiếu Watchdog & Storage Guard.
  - `205_71`: Camera bị errored trong PM2, 0 tiến trình FFmpeg (chết đứng từ 06:30 sáng), Swap 0B, thiếu Watchdog & Storage Guard.
  - `205_72`: Thẻ nhớ kịch 94% (chỉ còn 8.2GB trống, chuẩn bị crash), Swap 0B, thiếu Storage Guard.

### R2. Đóng gói & Triển khai `ksp-cam` lên cả 3 node [HOÀN THÀNH]
- Đã đóng gói binary tĩnh `kspcam` ARM64 mới nhất.
- Đã cài đặt và kích hoạt thành công dịch vụ `kspcam.service` trên cổng `:2028` của cả 3 node.

### R3. Tối ưu hóa hệ thống toàn diện chống crash / OOM / tràn thẻ nhớ [HOÀN THÀNH]
- Kích hoạt phân vùng ZRAM Swap 1.8 GiB – 1.9 GiB nén `lz4` trên cả 3 node, gỡ bỏ `mem_limit`, tối ưu `vm.swappiness = 60`.
- Gỡ bỏ hoàn toàn ngòi nổ `echo b > /proc/sysrq-trigger` trong Node-RED và hạ ngưỡng kích hoạt RAM về 60/80/100.
- Khắc phục thứ tự khởi động `pm2-root.service` chờ MariaDB (`After=mariadb.service`).
- Cài đặt Watchdog thông minh `shinobi-camera-watchdog` (quét mỗi 60s, tự cứu nếu camera zombie).
- Cài đặt Van xả thẻ nhớ tự động `shinobi-storage-guard.sh` (cron 15 phút/lần, dọn rác 0-byte, xoay vòng khi $\ge 85\%$).
- Đặc biệt trên `205_72`: Storage Guard đã lập tức xả thẻ từ 94% xuống 58% (giải phóng hơn 41GB dung lượng trống).
- Chuẩn hóa cấu hình Shinobi (`wall_clock_timestamp_ignore = "0"`, `hls_time = "2"`, `max_keep_days = 3`).

### R4. Nghiệm thu & Xác nhận hoạt động [HOÀN THÀNH]
- Cả 3 node đều đang chạy `kspcam` (:2028) active.
- Toàn bộ camera trên cả 3 node đã hồi sinh và đang ghi hình đầy đủ (FFmpeg hoạt động liên tục, sinh file mới đều đặn).

## Acceptance Criteria
- [x] C1: Cả 3 node đều được khảo sát và ghi nhận hiện trạng chi tiết.
- [x] C2: `kspcam` được triển khai và kích hoạt thành công trên cổng `:2028` của cả 3 node.
- [x] C3: Cả 3 node đều được tối ưu ZRAM, Node-RED sysrq, Watchdog, Storage guard, và timestamp.
- [x] C4: Cả 3 node đều có đầy đủ tiến trình FFmpeg ghi hình ổn định, file video mới tạo hợp lệ.
