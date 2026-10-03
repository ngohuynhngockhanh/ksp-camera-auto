# Original User Request

## Request — 2026-10-03
User request:
"chỉ review ko tác động, review 5_67 đang vận hanh thế nào, các cam bao nhiêu fps thông số ra sao ?"
Follow-up request:
"ok làm đ i"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_205_67` (`77.88.205.67`) — BinK Billiards Club (group `billiardclub`, 10 Dahua cameras)

## Requirements & Implementation Results

### R1. Tối ưu hóa hệ thống toàn diện chống crash / OOM / treo trên `inut_205_67` [HOÀN THÀNH]
- **Kích hoạt ZRAM Swap 1.9 GiB**: Thuật toán `lz4`, `MEM_LIMIT_PERCENTAGE=100`, `vm.swappiness = 60`, đã xóa bỏ `mem_limit`.
- **Gỡ bỏ ngòi nổ tự sát Node-RED**: Loại bỏ toàn bộ các lệnh `echo b > /proc/sysrq-trigger` và `rm -rf /tmp` trong `flow.json` (active flow có 0 sysrq). Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
- **Cấu hình khởi động dịch vụ**: Cập nhật `pm2-root.service` chờ MariaDB (`After=mariadb.service`, `Wants=mariadb.service`).
- **Nâng cấp Watchdog thông minh**: Cài đặt `/usr/local/bin/shinobi-camera-watchdog.sh` kèm timer `shinobi-camera-watchdog.timer` (quét mỗi 60s, kiểm tra số lượng tiến trình FFmpeg thực tế).
- **Cài đặt Storage Guard**: Cài đặt `/usr/local/bin/shinobi-storage-guard.sh` targeting `/media/usb1/billiardclub`, kích hoạt cron 15 phút/lần (`/etc/cron.d/shinobi-storage-guard`, xả khi $\ge 85\%$).
- **Chuẩn hóa Shinobi**: Kích hoạt `deleteOld`, `deleteOrphans`, `deleteOverMax` trong `conf.json`; đặt `max_keep_days = 3`, `wall_clock_timestamp_ignore = '0'`, `hls_time = '2'` trong MySQL.
- **Cập nhật kspcam**: Đã deploy binary ARM64 tĩnh mới nhất vào `/opt/ksp-cam/kspcam` và service `kspcam.service` (Active).

### R2. Đồng bộ chuẩn hóa Camera01 về cấu hình chung với 9 camera còn lại [HOÀN THÀNH]
- Đã áp dụng tuần tự qua `kspcam_apply_profile` và xác nhận đọc lại:
  - `Camera01` (`192.168.10.11:37777`): **2560x1440 @ 25fps, H.265, 1536 kbps VBR, GOP 50, SmartCodec true, AAC**.
  - Toàn bộ 10/10 camera hiện tại đạt chuẩn đồng bộ 100%.

### R3. Nghiệm thu & Kết quả đo đạc thực tế [HOÀN THÀNH]
- **RAM khả dụng**: Đạt **`851 MB – 933 MB`**, có sẵn **`1.9 GiB` ZRAM Swap** đệm an toàn.
- **Tiến trình FFmpeg**: Đầy đủ **20/20 tiến trình FFmpeg** (10 stream + 10 record) đang chạy ổn định.
- **File video mới**: Tất cả 10 camera đều sinh file video mới liên tục trong `/media/usb1/billiardclub/` (`12:29:xx.mp4`).

## Acceptance Criteria
- [x] C1: ZRAM Swap 1.9 GiB được kích hoạt và hoạt động.
- [x] C2: Node-RED sysrq và `rm -rf /tmp` bị loại bỏ hoàn toàn khỏi active flow.
- [x] C3: Watchdog thông minh và Storage guard cron được kích hoạt.
- [x] C4: Toàn bộ 10 camera Dahua đồng bộ chuẩn 25fps, 1536 kbps VBR, H.265, GOP 50.
- [x] C5: Cả 10 camera ghi hình đều đặn, RAM khả dụng thông thoáng.
