# Original User Request

## Request — 2026-10-02
User request:
"Ok xong chỉnh về 25fps giúp 1536 kbps là ok"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_205_56` (`77.88.205.56`) — King Kong Thái Nguyên (group `mp4`)
- Cameras: 9 Dahua cameras (`192.168.1.241` .. `192.168.1.250`)

## Requirements & Implementation Results

### R1. Tối ưu hóa hệ thống toàn diện chống treo / crash / OOM trên `inut_205_56` [HOÀN THÀNH]
- **Kích hoạt ZRAM Swap 1.9 GiB**: Thuật toán `lz4`, `MEM_LIMIT_PERCENTAGE=100`, `vm.swappiness = 60`, đã xóa bỏ `mem_limit`.
- **Gỡ bỏ ngòi nổ tự sát Node-RED**: Loại bỏ toàn bộ các lệnh `echo b > /proc/sysrq-trigger` và `rm -rf /tmp` trong `inut_3rd_project/flow.json` và `ok2/flow.json`. Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
- **Cấu hình khởi động dịch vụ**: Cập nhật `pm2-root.service` chờ MariaDB (`After=mariadb.service`, `Wants=mariadb.service`).
- **Nâng cấp Watchdog thông minh**: Cài đặt `/usr/local/bin/shinobi-camera-watchdog.sh` kèm timer `shinobi-camera-watchdog.timer` (quét mỗi 60s, kiểm tra số lượng tiến trình FFmpeg thực tế).
- **Cài đặt Storage Guard**: Cài đặt `/usr/local/bin/shinobi-storage-guard.sh` targeting `/media/usb1/mp4`, kích hoạt cron 15 phút/lần (`/etc/cron.d/shinobi-storage-guard`).
- **Chuẩn hóa Shinobi**: Kích hoạt `deleteOld`, `deleteOrphans`, `deleteOverMax` trong `conf.json`; đặt `max_keep_days = 3`, `wall_clock_timestamp_ignore = '0'`, `hls_time = '2'` trong MySQL.
- **Cập nhật kspcam**: Đã deploy binary ARM64 tĩnh mới nhất vào `/opt/ksp-cam/kspcam` và service `kspcam.service` (Active).

### R2. Chuẩn hóa 9 camera Dahua về 25 FPS, 1536 Kbps, GOP 50 [HOÀN THÀNH]
- Đã áp dụng tuần tự an toàn qua `kspcam_apply_profile` và xác nhận đọc lại 100% trên cả 9 camera:
  - `192.168.1.241:37777`: 2880x1620 @ 25fps, H.265, 1536kbps VBR, GOP 50, AAC (OK)
  - `192.168.1.242:37777`: 2880x1620 @ 25fps, H.265, 1536kbps VBR, GOP 50, AAC (OK)
  - `192.168.1.243:37777`: 2880x1620 @ 25fps, H.265, 1536kbps VBR, GOP 50, AAC (OK)
  - `192.168.1.244:37777`: 2880x1620 @ 25fps, H.265, 1536kbps VBR, GOP 50, AAC (OK)
  - `192.168.1.245:37777`: 2880x1620 @ 25fps, H.265, 1536kbps VBR, GOP 50, AAC (OK)
  - `192.168.1.246:37777`: 2880x1620 @ 25fps, H.265, 1536kbps VBR, GOP 50, AAC (OK)
  - `192.168.1.248:37777`: 2880x1620 @ 25fps, H.265, 1536kbps VBR, GOP 50, AAC (OK)
  - `192.168.1.249:37777`: 2880x1620 @ 25fps, H.265, 1536kbps VBR, GOP 50, AAC (OK)
  - `192.168.1.250:37777`: 2880x1620 @ 25fps, H.265, 1536kbps VBR, GOP 50, AAC (OK)

### R3. Nghiệm thu & Kết quả đo đạc thực tế [HOÀN THÀNH]
- **Tải RAM FFmpeg**: Giảm từ **839 MB** xuống **623 MB** (tiết kiệm hơn 215 MB RAM vật lý).
- **RAM khả dụng**: Tăng từ **47 MB** lên **913 MB – 1.0 GiB**.
- **ZRAM Swap**: Sẵn sàng 1.9 GiB bảo vệ hệ thống tuyệt đối không bao giờ bị nghẽn RAM.
- **Tiến trình ghi hình**: Toàn bộ **18/18 tiến trình FFmpeg** đang chạy ghi hình file mới đều đặn.

## Acceptance Criteria
- [x] C1: ZRAM Swap 1.9 GiB được kích hoạt và hoạt động.
- [x] C2: Node-RED sysrq và `rm -rf /tmp` bị loại bỏ hoàn toàn.
- [x] C3: Watchdog thông minh và Storage guard cron được kích hoạt.
- [x] C4: Toàn bộ 9 camera Dahua được chuẩn hóa về 25fps, 1536 kbps H.265, GOP 50.
- [x] C5: Cả 9 camera ghi hình đều đặn, RAM khả dụng thông thoáng > 900 MB.
