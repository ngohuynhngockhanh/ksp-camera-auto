# Original User Request

## Request — 2026-10-06
User request:
"5.87 luôn nhé"
Follow-up:
"Add những con đã tối ưu vô watchlist để khi nào tôi hỏi là biết đường trả lời"

Working directory: `/home/ksp/ksp-camera-auto`

Target Nodes:
- `inut_205_87` (`77.88.205.87`) — Heypick - Pickleball 292 Kim Giang (group `heyPick`, 7 cameras)
- Master Watchlist Creation: `FLEET_WATCHLIST.md`

## Requirements & Implementation Results

### R1. Tối ưu hóa toàn diện trên node `inut_205_87` [HOÀN THÀNH]
- **Kích hoạt ZRAM Swap 1.9 GiB**: Thuật toán `lz4`, `MEM_LIMIT_PERCENTAGE=100`, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, đã xóa bỏ `mem_limit`.
- **Gỡ bỏ ngòi nổ tự sát Node-RED**: Loại bỏ toàn bộ các lệnh `echo b > /proc/sysrq-trigger` và `rm -rf /tmp` trong `flow.json` (active flow có 0 sysrq). Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
- **Cấu hình PM2 Memory Guard (N=7)**:
  - `camera`: `--max-memory-restart 180M`
  - `inut`: `--max-memory-restart 280M`
  - `ota-mqtt`: `--max-memory-restart 120M`
  - Đảm bảo xả log vào `/dev/null` 100% để chống mòn thẻ nhớ và đầy đĩa.
  - Lưu vĩnh viễn vào `/root/.pm2/dump.pm2`.
- **Khắc phục thứ tự khởi động dịch vụ**: Cập nhật `pm2-root.service` chờ MariaDB (`After=mariadb.service`, `Wants=mariadb.service`).
- **Nâng cấp Smart Watchdog**: Cài đặt `/usr/local/bin/shinobi-camera-watchdog.sh` kèm timer `shinobi-camera-watchdog.timer` (quét mỗi 60s, kiểm tra `pgrep -c ffmpeg`).
- **Cài đặt Storage Guard**: Cài đặt `/usr/local/bin/shinobi-storage-guard.sh` targeting `/media/usb1/heyPick`, kích hoạt cron 15 phút/lần (`/etc/cron.d/shinobi-storage-guard`, xả khi $\ge 85\%$).
- **Chuẩn hóa Shinobi**: Kích hoạt `deleteOld`, `deleteOrphans`, `deleteOverMax` trong `conf.json`; đặt `max_keep_days = 3`, `wall_clock_timestamp_ignore = '0'`, `hls_time = '2'` trong MySQL.
- **Cập nhật kspcam**: Deploy binary ARM64 tĩnh mới nhất vào `/opt/ksp-cam/kspcam` và service `kspcam.service` (Active).

### R2. Thiết lập Master Watchlist (`FLEET_WATCHLIST.md`) [HOÀN THÀNH]
- Tạo bảng theo dõi chi tiết 26 node edge đã được khảo sát và tối ưu hóa trong toàn bộ hệ thống:
  - Node ID, IP VPN, Tên quán / sân, Group key, Số camera ($N$), Kiến trúc CPU, ZRAM Swap, PM2 Memory Guard, Smart Watchdog, Storage Guard, Sysrq status, kspcam, và Trạng thái vận hành thực tế.
  - Định rõ 7 Tiêu Chuẩn Vàng Khi Tối Ưu Hóa (Golden Hardening Checklist).

## Acceptance Criteria
- [x] C1: Node `205_87` được khảo sát và tối ưu hóa toàn diện.
- [x] C2: ZRAM 100% không giới hạn và PM2 memory guard được cấu hình chuẩn xác trên 205_87.
- [x] C3: Node-RED sysrq bị loại bỏ, Smart Watchdog và Storage guard cron được kích hoạt.
- [x] C4: Tất cả 7 camera ghi hình ổn định (14 FFmpeg processes), RAM khả dụng thông thoáng.
- [x] C5: Master Watchlist `FLEET_WATCHLIST.md` được tạo đầy đủ để tra cứu tức thì bất kỳ lúc nào.
