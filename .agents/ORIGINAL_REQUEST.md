# Original User Request

## Request — 2026-10-04
User request:
"4-157 vẫn bị hết ram ó"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_204_157` (`77.88.204.157`) — Kely Pickleball (group `kely`, 12 recorded cameras `camera01` .. `camera13`, `camera03` stop)

## Root Cause Analysis
1. **Phân bổ tải bộ nhớ cực cao (12 Camera trên 1.8GB RAM)**:
   - Box có 1.8 GiB RAM vật lý nhưng gánh tới **12 luồng camera** (24 tiến trình FFmpeg + 12 tiến trình `singleCamera.js` Node.js).
   - Tổng bộ nhớ ẩn danh cơ sở (Anonymous RSS) của toàn bộ tiến trình:
     - 12 tiến trình FFmpeg: ~520 MB
     - 12 tiến trình `singleCamera.js`: ~600 MB
     - Node-RED (`inut`): ~250 MB
     - Shinobi master (`camera.js`): ~150 MB (ballooned to 327 MB)
     - PM2, ota-mqtt, mariadb: ~200 MB
     - **Tổng base memory: ~1,725 MB / 1,837 MB** (chiếm 95% RAM vật lý).
2. **Khuyết tật ZRAM `MEM_LIMIT_PERCENTAGE=50` (Nguyên nhân cốt lõi gây OOM)**:
   - Trong `/etc/default/armbian-zram-config`, `MEM_LIMIT_PERCENTAGE` bị để giá trị mặc định là **`50`**.
   - Hậu quả: Khi ZRAM lưu được ~110 MB dữ liệu nén, driver ZRAM chạm trần và trả về mã lỗi **`-ENOMEM` (Swap write error)** cho kernel.
   - Khi kernel không thể hoán đổi thêm trang nhớ ẩn danh vào Swap, cơ chế page reclaim thất bại hoàn toàn. Ngay lập tức kernel gọi OOM killer (lúc 15:38 hôm nay):
     `Out of memory: Killed process 2157 (node /home/Shin) total-vm:52544412kB, anon-rss:326860kB`.
3. **Không có ngưỡng giới hạn heap cho Node.js**:
   - V8 mặc định cho phép heap phình lên 300MB–500MB trước khi thu gom rác. Chỉ cần `camera.js` hoặc `node-red` nở thêm 100MB là toàn bộ hệ thống bị cạn RAM.
4. **Thiếu vắng Watchdog timer và Storage Guard**:
   - Box hoàn toàn chưa có `shinobi-camera-watchdog.timer` và chưa có `shinobi-storage-guard`.

## Remediation & Actions Taken
1. **Mở khóa ZRAM Swap lên 100% & Gỡ bỏ `mem_limit`**:
   - Cấu hình `ZRAM_PERCENTAGE=100`, `MEM_LIMIT_PERCENTAGE=100`, `SWAP_ALGORITHM=lz4`.
   - Ghi `0` vào `/sys/block/zram0/mem_limit` để xóa bỏ hoàn toàn giới hạn dung lượng swap.
   - Tăng `vm.swappiness = 80` và `vm.vfs_cache_pressure = 100` để kernel chủ động đẩy các trang nhớ tĩnh của Node.js sang ZRAM.
   - Mở rộng tổng Swap lên **`3.5 GiB`**.
2. **Thiết lập giới hạn bộ nhớ PM2 chủ động (`--max-memory-restart`)**:
   - `camera`: giới hạn 220MB (ngăn chặn tình trạng balloon lên 327MB gây OOM).
   - `inut` (node-red): giới hạn 280MB.
   - `ota-mqtt`: giới hạn 120MB.
3. **Cài đặt Smart Watchdog (`shinobi-camera-watchdog`)**:
   - Kích hoạt timer `shinobi-camera-watchdog.timer` quét mỗi 60s, kiểm tra số lượng tiến trình FFmpeg thực tế (`pgrep -c ffmpeg`).
4. **Cài đặt Storage Guard (`shinobi-storage-guard`)**:
   - Kích hoạt cron 15 phút/lần bảo vệ thẻ nhớ `/media/usb1/kely/` (xả khi $\ge 85\%$).
5. **Chuẩn hóa Shinobi**:
   - Cập nhật `wall_clock_timestamp_ignore = '0'`, `hls_time = '2'`, `max_keep_days = '3'`, bật cờ dọn file mồ côi trong `conf.json`.

## Acceptance Verification
- [x] RAM khả dụng tăng từ mức báo động lên **`950 MiB`** (tăng thêm gần 300MB RAM khả dụng).
- [x] Swap ZRAM hoạt động trơn tru với dung lượng mở rộng **`3.5 GiB`** (không còn bị chặn ở 50%).
- [x] Toàn bộ 12 camera ghi hình đang chạy đủ **24 tiến trình FFmpeg** (12 stream HLS + 12 record MP4).
- [x] File ghi hình MP4 sinh đều đặn trên cả 12 sân pickleball trong `/media/usb1/kely/`.
- [x] Smart watchdog và Storage guard cron đã kích hoạt và hoạt động tự động.
