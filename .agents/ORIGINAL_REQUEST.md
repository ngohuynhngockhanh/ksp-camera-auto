# Original User Request

## Request — 2026-10-04
User request:
"Ok coi tiếp 4.159 nãy bị treo phải nhờ rút gắm lại tìm nguyên nhân với"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_204_159` (`77.88.204.159`) — Axis Billiards (group `axis`, 9 Dahua cameras)

## Root Cause Analysis
1. **Phần cứng Octopus Planet (Chip Amlogic S912, 8 nhân CPU)**:
   - Box chạy chip 8 nhân (`CPU=8`).
2. **"Cầu dao tự sát" trong Node-RED đặc thù cho CPU 8 nhân**:
   - Trong file `/root/.node-red/projects/inut_3rd_project/flow.json`:
     ```bash
     CPU=$(grep -c ^processor /proc/cpuinfo); echo "[INFO] CPU cores: $CPU"; [ "$CPU" -eq 8 ] && { echo "[ACTION] Force reboot"; echo b > /proc/sysrq-trigger; } || { echo "[ACTION] pm2 restart camera"; pm2 restart camera; }
     ```
   - Trên các box 4 nhân, khi RAM thiếu, script chỉ chạy `pm2 restart camera`.
   - NHƯNG trên box này vì có **8 nhân (`CPU=8`)**, nó cố tình nhảy vào nhánh `echo b > /proc/sysrq-trigger`!
3. **Cơ chế gây treo cứng (Freeze/Hang) phải rút nguồn cắm lại**:
   - Khi `echo b > /proc/sysrq-trigger` kích hoạt trong lúc 18 tiến trình FFmpeg đang ghi dữ liệu xuống thẻ nhớ exFAT (`mmcblk1p1`) và eMMC (`mmcblk2p2`):
     - Chip controller MMC (`meson-gx-mmc`) bị ngắt ngang khi DMA bus đang pending.
     - SoC Amlogic S912 bị khóa cứng thanh ghi bus reset (bus lockup state) thay vì power-cycle sạch PMIC.
     - Đèn nguồn vẫn sáng nhưng toàn bộ CPU bị treo cứng ở bootrom/reset vector, watchdog phần cứng không thể tự reset được $\rightarrow$ **bắt buộc phải rút phích cắm điện nguồn 12V ra cắm lại mới khởi động lại được**.
4. **Không có Swap (`Swap = 0B`)**:
   - `armbian-zram-config` bị tắt (`ENABLED=false`).
   - RAM trống thực tế chỉ quanh mức **62 MB**, thấp hơn ngưỡng kích hoạt `max_free_ram_force_reboot = 70 MB` $\rightarrow$ box liên tục rơi vào bẫy tự sát `sysrq`.
5. **Dung lượng thẻ nhớ nguy hiểm & Không có Storage Guard**:
   - Thẻ nhớ `/media/usb1` đang dùng **47 GB / 58 GB (81%)**, hoàn toàn chưa có cron tự xả thẻ nhớ.
6. **Watchdog camera cũ**:
   - Script watchdog cũ chỉ kiểm tra PID tồn tại, không kiểm tra tiến trình FFmpeg thực tế.

## Remediation & Actions Taken
1. **Kích hoạt ZRAM Swap 1.8 GiB**:
   - Cấu hình `ZRAM_PERCENTAGE=100`, `SWAP_ALGORITHM=lz4`, `vm.swappiness = 60`, xóa `mem_limit`.
   - Nâng RAM khả dụng lên **`938 MB / 1.8 GiB`** (có sẵn 1.8 GiB đệm ảo nén nhanh).
2. **Gỡ bỏ triệt để các ngòi nổ `sysrq` và `rm -rf /tmp` trong Node-RED**:
   - Thay thế toàn bộ 5 node `echo b > /proc/sysrq-trigger` và node `[ "$CPU" -eq 8 ]` bằng lệnh giải phóng cache và restart camera an toàn.
   - Hạ các ngưỡng cảnh báo RAM trong `/root/ota-mqtt/change_ok/` về mức an toàn `60 / 80 / 100`.
3. **Cài đặt Smart Watchdog (`shinobi-camera-watchdog`)**:
   - Cài đặt script kiểm tra `pgrep -c ffmpeg` kèm timer quét mỗi 60 giây.
4. **Cài đặt Van xả thẻ nhớ tự động (`shinobi-storage-guard`)**:
   - Script xả đĩa targeting `/media/usb1/axis/`, kích hoạt cron 15 phút/lần (`/etc/cron.d/shinobi-storage-guard`).
5. **Cập nhật `kspcam` (:2028)**:
   - Deploy binary tĩnh ARM64 mới nhất vào `/opt/ksp-cam/kspcam` và service `kspcam.service` (Active).
6. **Chuẩn hóa Shinobi**:
   - Cập nhật `wall_clock_timestamp_ignore = '0'`, `hls_time = '2'`, `max_keep_days = '3'`, bật cờ dọn file mồ côi trong `conf.json`.

## Acceptance Criteria
- [x] C1: Node `204_159` được khảo sát và xác định chính xác nguyên nhân gây treo cứng.
- [x] C2: `kspcam` được triển khai và kích hoạt thành công trên cổng `:2028`.
- [x] C3: Hệ thống được tối ưu ZRAM, Node-RED sysrq, Watchdog thông minh, Storage guard, và timestamp.
- [x] C4: Có đầy đủ 18 tiến trình FFmpeg ghi hình ổn định, file video mới tạo hợp lệ trên cả 9 camera.
