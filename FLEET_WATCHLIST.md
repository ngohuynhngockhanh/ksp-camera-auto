# FLEET WATCHLIST & HARDENING INVENTORY
> **Tài liệu theo dõi trạng thái tối ưu hóa và danh mục thiết bị Edge Fleet (ksp-camera-auto)**  
> Cập nhật tự động: Mỗi node khi hoàn tất tối ưu hóa đều được ghi nhận chi tiết về thông số phần cứng, số lượng camera, cấu hình PM2 Memory Guard, ZRAM, Watchdog, Storage Guard và trạng thái vận hành thực tế.

---

## 1. Tiêu Chuẩn Vàng Khi Tối Ưu Hóa (Golden Hardening Checklist)

Mọi node trong danh sách dưới đây đều tuân thủ nghiêm ngặt 7 nguyên tắc cốt lõi:
1. **ZRAM Swap 100% Unconstrained**: `ZRAM_PERCENTAGE=100`, `MEM_LIMIT_PERCENTAGE=100`, `echo 0 > /sys/block/zram0/mem_limit`, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100` -> Triệt tiêu hoàn toàn lỗi `-ENOMEM` và OOM killer.
2. **PM2 Memory Guard**: Giới hạn heap chủ động cho Node.js theo công thức số camera: `camera: (120 + N*8)M`, `inut: 280M`, `ota-mqtt: 120M`, tự tái sinh êm sau 1–2s mà không làm gián đoạn hệ thống.
3. **Bảo toàn Log `/dev/null`**: Cả `pm_out_log_path` và `pm_err_log_path` đều trỏ vào `/dev/null` để chống mòn thẻ nhớ và chống đầy dung lượng `/var/log` / tmpfs.
4. **Triệt tiêu ngòi nổ tự sát Node-RED**: Loại bỏ hoàn toàn các lệnh `echo b > /proc/sysrq-trigger`, `rm -rf /tmp`, và nhánh `[ "$CPU" -eq 8 ]`. Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
5. **Smart Watchdog 60s**: Script `/usr/local/bin/shinobi-camera-watchdog.sh` kiểm tra số lượng tiến trình FFmpeg thực tế (`pgrep -c ffmpeg`). Nếu camera online $> 90\text{s}$ mà có 0 FFmpeg thì tự phục hồi ngay.
6. **Storage Guard 15p**: Script `/usr/local/bin/shinobi-storage-guard.sh` targeting thư mục video của quán, dọn rác 0-byte và xoay vòng xả đĩa khi $\ge 85\%$ về mức $\le 75\%$.
7. **Shinobi Zero-Transcode & Timestamp Fix**: `vcodec=copy`, `acodec=copy` (hoặc `no`), `wall_clock_timestamp_ignore=0` (hoặc passthrough), `hls_time=2`, `max_keep_days=3`, kích hoạt `deleteOld`/`deleteOrphans` trong `conf.json`.

---

## 2. Danh Mục Các Node Đã Áp Dụng Chuẩn PM2 Memory Guard & Tối Ưu Toàn Diện

- [x] **Node `inut_204_157`** (Kely Pickleball — `77.88.204.157`, group `kely`)
  - Phần cứng: ARM64 (Octopus Planet), RAM 1.8 GiB
  - Số lượng camera: 12 camera (12 sân Pickleball)
  - Cấu hình PM2 Memory Guard: `camera`: 220M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 3.5 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/kely`), Sysrq gỡ sạch 100%
  - Trạng thái: Uptime >3 ngày, RAM khả dụng 713M–950M, 24/24 FFmpeg ghi 12 sân liên tục, 0 OOM

- [x] **Node `inut_205_72`** (Master Billiards Club — `77.88.205.72`, group `master2`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 10 camera Dahua 3K (`2880x1620 @ 30fps`, H.265, AAC)
  - Cấu hình PM2 Memory Guard: `camera`: 220M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/master2`), Sysrq gỡ sạch 100%
  - Trạng thái: Uptime >19 ngày, RAM khả dụng 962M–1.1G, 20/20 FFmpeg ghi 10 bàn liên tục

- [x] **Node `inut_205_87`** (Heypick Pickleball 292 Kim Giang — `77.88.205.87`, group `heyPick`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 7 camera Dahua
  - Cấu hình PM2 Memory Guard: `camera`: 180M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/heyPick`), Sysrq gỡ sạch 100%
  - Trạng thái: Triệt tiêu chu kỳ reboot 1–2 ngày/lần, RAM khả dụng >850M, 14/14 FFmpeg ghi 7 sân liên tục

- [x] **Node `inut_204_159`** (Axis Billiards — `77.88.204.159`, group `axis`)
  - Phần cứng: Amlogic S912 8-core (ARM64), RAM 1.8 GiB
  - Số lượng camera: 9 camera Dahua 3K (`2880x1620 @ 25fps`, H.265, AAC)
  - Cấu hình PM2 Memory Guard: `camera`: 200M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.8 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/axis`), Gỡ sạch bẫy `CPU=8` sysrq
  - Trạng thái: Triệt tiêu lỗi khóa bus (bus lockup) gây treo cứng phải rút nguồn, 18/18 FFmpeg ghi 9 bàn liên tục

- [x] **Node `inut_205_67`** (BinK Billiards Club — `77.88.205.67`, group `billiardclub`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 11 camera (10 cam 4MP H.265 + 1 cam 2MP H.264 AAC)
  - Cấu hình PM2 Memory Guard: `camera`: 220M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/billiardclub`), Sysrq gỡ sạch 100%
  - Trạng thái: Uptime >4 ngày, RAM khả dụng 887M–1.0G, 22/22 FFmpeg ghi 11 bàn liên tục

- [x] **Node `inut_205_69`** (PHE Vietnam — `77.88.205.69`, group `phe`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 6 camera
  - Cấu hình PM2 Memory Guard: `camera`: 170M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Xóa bỏ swapfile eMMC gây nghẽn I/O, Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/phe`), Sysrq gỡ sạch
  - Trạng thái: Triệt tiêu tình trạng rớt online/offline, ping phẳng 31ms, RAM khả dụng 1.07 GB, 12/12 FFmpeg ghi đều

- [x] **Node `inut_205_85`** (Lion King Nam Định CS2 — `77.88.205.85`, group `lionkingnd`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 6 camera Dahua NVR (`2304x1296 @ 20fps`, H.265, AAC)
  - Cấu hình PM2 Memory Guard: `camera`: 170M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s (tự restart nếu 0 FFmpeg), Storage Guard 15p (`/media/usb1/lionkingnd`), Sysrq gỡ sạch
  - Trạng thái: Hồi sinh toàn bộ 6 camera (sau 8 tiếng bị đứng hình), RAM khả dụng 1.1 GB, 12/12 FFmpeg ghi đều

- [x] **Node `inut_205_56`** (King Kong Thái Nguyên — `77.88.205.56`, group `mp4`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 9 camera Dahua chuẩn hóa 25fps 1536k VBR GOP 50 H.265 AAC
  - Cấu hình PM2 Memory Guard: `camera`: 200M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/mp4`), Gỡ 5 node sysrq và `rm -rf /tmp`
  - Trạng thái: RAM khả dụng 1.05 GB, 18/18 FFmpeg ghi 9 bàn liên tục

- [x] **Node `inut_205_53`** (The Best Billiards Club — `77.88.205.53`, group `ultrapool1`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 12 camera
  - Cấu hình PM2 Memory Guard: `camera`: 220M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Tắt CPU audio transcoding (`acodec=no`), wallclock=0, Smart Watchdog 60s, Storage Guard 15p
  - Trạng thái: RAM khả dụng 981 MB (tăng từ 41MB), 23 FFmpeg ghi hình liên tục

- [x] **Node `inut_205_62`** (Billiards 2000 Club — `77.88.205.62`, group `2000`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 8 camera Dahua
  - Cấu hình PM2 Memory Guard: `camera`: 185M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/2000`), Sysrq gỡ sạch
  - Trạng thái: Hồi sinh 8 camera zombie (từ 0 lên 16 FFmpeg), RAM khả dụng 1.06 GB, ghi hình ổn định

- [x] **Node `inut_205_6`** (TheLight Pool Arena — `77.88.205.6`, group `thelightpoolarena`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 10 camera
  - Cấu hình PM2 Memory Guard: `camera`: 200M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 3.8 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/thelightpoolarena`), Sysrq gỡ sạch
  - Trạng thái: RAM khả dụng 969 MB, Swap 3.8 GiB, 20/20 FFmpeg ghi 10 bàn liên tục

- [x] **Node `inut_205_10`** (CLB Billiard The King — `77.88.205.10`, group `xinchao`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 8 camera Dahua
  - Cấu hình PM2 Memory Guard: `camera`: 185M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Giải tỏa nguy cơ tràn đĩa 84%, Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/xinchao`), Sysrq gỡ sạch
  - Trạng thái: RAM khả dụng 1.0 GB, 16/16 FFmpeg ghi 8 bàn liên tục

- [x] **Node `inut_205_77`** (ZONE+ Billiards Center — `77.88.205.77`, group `toktok3`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 12 camera Dahua
  - Cấu hình PM2 Memory Guard: `camera`: 220M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/toktok3`), Sysrq gỡ sạch
  - Trạng thái: RAM khả dụng 1.17 GB, 24/24 FFmpeg ghi 12 bàn liên tục

- [x] **Node `inut_204_148`** (Ruby Billiard Club — `77.88.204.148`, group `na`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 7 camera Hikvision (Camera07 IP `192.168.1.65`)
  - Cấu hình PM2 Memory Guard: `camera`: 180M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/na`), Sysrq gỡ sạch
  - Trạng thái: RAM khả dụng 1.12 GB, 12/12 FFmpeg ghi hình ổn định
