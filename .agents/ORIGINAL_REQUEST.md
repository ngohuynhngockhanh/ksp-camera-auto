# Original User Request

## Request — 2026-10-02
User request:
"5_62, 5_6 nữa nha"
"5_10 nữa"

Working directory: `/home/ksp/ksp-camera-auto`

Target Nodes:
- `inut_205_62` (`77.88.205.62`) — Billiards 2000 Club (group `2000`, 8 cameras)
- `inut_205_6` (`77.88.205.6`) — TheLight Pool Arena (group `thelightpoolarena`, 10 cameras)
- `inut_205_10` (`77.88.205.10`) — CLB Billiard The King (group `xinchao`, 8 cameras)

## Requirements & Implementation Results

### R1. Khảo sát hiện trạng & phát hiện sự cố trên 3 node [HOÀN THÀNH]
- **`inut_205_62`**:
  - Box vừa bị reboot, **Swap = 0 Byte**.
  - **Sự cố nghiêm trọng**: Camera process trong PM2 báo online nhưng có **0 tiến trình FFmpeg** chạy (toàn bộ 8 camera bị zombie, không ghi hình).
  - Không có Storage Guard, thẻ nhớ 80% (48GB/60GB).
  - Có 3 node `echo b > /proc/sysrq-trigger` trong Node-RED.
- **`inut_205_6`**:
  - Đã có ZRAM nhưng chưa có Watchdog timer tự hồi sinh camera.
  - Có 3 node `sysrq-trigger` trong Node-RED.
- **`inut_205_10`**:
  - Uptime 17 ngày, thẻ nhớ **84%** (47GB/59GB, chỉ còn 9.3GB).
  - **Swap = 0 Byte**, không có Storage Guard, không có Watchdog timer.
  - Có 3 node `sysrq-trigger` trong Node-RED.

### R2. Đóng gói & Triển khai `kspcam` (:2028) [HOÀN THÀNH]
- Cập nhật binary tĩnh `kspcam` ARM64 mới nhất trên cả 3 node và đảm bảo `kspcam.service` hoạt động liên tục.

### R3. Tối ưu hóa hệ thống toàn diện chống crash / OOM / tràn thẻ nhớ [HOÀN THÀNH]
- **ZRAM Swap**: Kích hoạt phân vùng ZRAM Swap 1.9 GiB (hoặc 3.8 GiB) nén `lz4`, `MEM_LIMIT_PERCENTAGE=100`, `vm.swappiness = 60`, xóa `mem_limit`.
- **Gỡ bỏ ngòi nổ tự sát Node-RED**: Loại bỏ toàn bộ các lệnh `echo b > /proc/sysrq-trigger` và `rm -rf /tmp` trong `flow.json`. Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
- **Khắc phục thứ tự khởi động dịch vụ**: Cập nhật `pm2-root.service` chờ MariaDB (`After=mariadb.service`, `Wants=mariadb.service`).
- **Nâng cấp Watchdog thông minh**: Cài đặt `/usr/local/bin/shinobi-camera-watchdog.sh` kèm timer `shinobi-camera-watchdog.timer` trên cả 3 node (quét mỗi 60s, kiểm tra số lượng tiến trình FFmpeg thực tế).
- **Cài đặt Storage Guard**: Cài đặt `/usr/local/bin/shinobi-storage-guard.sh` targeting thư mục video của từng quán, kích hoạt cron 15 phút/lần (`/etc/cron.d/shinobi-storage-guard`).
- **Chuẩn hóa Shinobi**: `wall_clock_timestamp_ignore = "0"`, `hls_time = "2"`, `max_keep_days = 3`, bật cờ dọn file mồ côi trong `conf.json`.

### R4. Nghiệm thu & Kết quả đo đạc thực tế [HOÀN THÀNH]
- **`inut_205_62`**: Hồi sinh thành công 8/8 camera (từ 0 lên 18 tiến trình FFmpeg), sinh file mới `12:30:00.mp4` trên toàn bộ 8 camera, ZRAM 1.9GB active, RAM khả dụng 859 MB.
- **`inut_205_6`**: 10/10 camera ghi hình đều đặn (20 FFmpeg processes), file mới `12:30:00.mp4`, ZRAM 3.8GB active, RAM khả dụng 970 MB.
- **`inut_205_10`**: 8/8 camera ghi hình đều đặn (16 FFmpeg processes), file mới `12:30:00.mp4`, ZRAM 1.9GB active, RAM khả dụng 824 MB, giải tỏa nguy cơ tràn đĩa 84%.

## Acceptance Criteria
- [x] C1: Cả 3 node `205_62`, `205_6`, `205_10` được khảo sát và ghi nhận hiện trạng chi tiết.
- [x] C2: `kspcam` được triển khai và kích hoạt thành công trên cổng `:2028`.
- [x] C3: Hệ thống được tối ưu ZRAM, Node-RED sysrq, Watchdog, Storage guard, và timestamp.
- [x] C4: Có đầy đủ tiến trình FFmpeg ghi hình ổn định, file video mới tạo hợp lệ trên cả 3 node.
