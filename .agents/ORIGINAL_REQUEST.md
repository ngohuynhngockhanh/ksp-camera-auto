# Original User Request

## Request — 2026-10-06
User request:
"Ok qua 5.72 caid vậy luoon check xem ok ko"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_205_72` (`77.88.205.72`) — Master Billiards Club (group `master2`, 10 Dahua cameras)

## Requirements & Implementation Results

### R1. Khảo sát hiện trạng & điều tra sức khỏe hệ thống trên `inut_205_72` [HOÀN THÀNH]
- **Kiến trúc phần cứng**: Allwinner H313 (`aarch64`), kernel `6.6.44-current-sunxi64`.
- **Uptime**: Uptime 19 ngày liên tục (`up 19 days, 20:24`).
- **RAM & ZRAM ban đầu**:
  - RAM vật lý 1.9 GiB (Used: 833 MiB, Avail: 1.1 GiB).
  - Swap: 1.9 GiB ZRAM (`MEM_LIMIT_PERCENTAGE=100`, nhưng `swappiness` đang để mức thấp 60, chưa có `vfs_cache_pressure = 100`).
- **PM2**:
  - `camera` (pid 3255609), `inut` (pid 3253755), `ota-mqtt` (pid 3239619).
  - Chưa được gán giới hạn `--max-memory-restart`.
  - Tiến trình `camera` đang ghi log ra file đĩa `/root/.pm2/logs/camera-out.log` và `camera-error.log` (chưa chuyển sang `/dev/null` như `ota-mqtt`).
- **Thẻ nhớ & Watchdog**:
  - Thẻ nhớ 118 GB: Dùng 70 GB (60%), còn trống 48 GB.
  - Đã có Smart Watchdog `shinobi-camera-watchdog.timer` và Storage Guard `shinobi-storage-guard` cron.
  - Active `flow.json` Node-RED đã sạch hoàn toàn lệnh `sysrq` (chỉ còn trong các file backup cũ).

### R2. Đóng gói & Triển khai cấu hình tối ưu toàn diện như 4-157 [HOÀN THÀNH]
1. **ZRAM Swap 100% & Tối ưu Kernel VM**:
   - `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100` (đã lưu vào `/etc/sysctl.conf`).
   - Xóa bỏ `mem_limit`: `echo 0 > /sys/block/zram0/mem_limit`.
2. **PM2 Memory Guard theo số lượng camera (N=10)**:
   - `camera`: `--max-memory-restart 220M --output /dev/null --error /dev/null`
   - `inut`: `--max-memory-restart 280M --output /dev/null --error /dev/null`
   - `ota-mqtt`: `--max-memory-restart 120M --output /dev/null --error /dev/null`
   - Đã lưu cấu hình vĩnh viễn vào `/root/.pm2/dump.pm2` (`pm2 save`).
   - Xóa sạch các file log cũ trong `/root/.pm2/logs/`, bảo vệ đĩa không bị mòn hay đầy dung lượng.
3. **Smart Watchdog & Storage Guard**:
   - `shinobi-camera-watchdog.timer` hoạt động quét mỗi 60 giây.
   - `shinobi-storage-guard` cron hoạt động mỗi 15 phút (bảo vệ `/media/usb1/master2/`).

### R3. Nghiệm thu & Kết quả đo đạc thực tế [HOÀN THÀNH]
- **RAM khả dụng**: Đạt **`962 MiB – 1.1 GiB`** (rất thông thoáng).
- **ZRAM Swap**: Sẵn sàng **`1.9 GiB`** (đang nén lưu 194 MB dữ liệu chỉ tốn 37 MB RAM thực).
- **Tiến trình ghi hình**: Toàn bộ **20/20 tiến trình FFmpeg** (10 stream + 10 record) đang chạy mượt mà.
- **File video mới**: Tất cả 10 camera đều đang sinh file video mới liên tục trong `/media/usb1/master2/` (`21:05:xx.mp4`, 3K 2880x1620 @ 30fps H.265, AAC).
- **PM2 Log Paths**: Cả 3 tiến trình (`camera`, `inut`, `ota-mqtt`) đều xả log vào `/dev/null` chuẩn xác.

## Acceptance Criteria
- [x] C1: Node `205_72` được khảo sát và ghi nhận hiện trạng chi tiết.
- [x] C2: ZRAM 100% không giới hạn và PM2 memory guard được cấu hình chuẩn xác.
- [x] C3: Node-RED sysrq bị loại bỏ, Smart Watchdog và Storage guard cron được kích hoạt.
- [x] C4: Tất cả các camera ghi hình ổn định, FFmpeg chạy mượt mà, RAM khả dụng thông thoáng.
