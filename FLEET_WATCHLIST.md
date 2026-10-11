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
6. **Storage Guard 15p (Tiêu chuẩn mới 90%)**: Script `/usr/local/bin/shinobi-storage-guard.sh` targeting thư mục video của quán, dọn rác 0-byte và xoay vòng xả đĩa khi $\ge 90\%$ về mức $\le 80\%$ (do Shinobi đã tự động chừa lại 10% dung lượng an toàn).
7. **Shinobi Zero-Transcode & Timestamp Fix**: `vcodec=copy`, `acodec=copy` (hoặc `no`), `wall_clock_timestamp_ignore=0` (hoặc passthrough), `hls_time=2`, `max_keep_days=3`, kích hoạt `deleteOld`/`deleteOrphans` trong `conf.json`.

---

## 2. Danh Mục Các Node Đã Áp Dụng Chuẩn PM2 Memory Guard & Tối Ưu Toàn Diện

- [x] **Node `inut_204_141`** (789 Billiards — `77.88.204.141`, group `mia`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB, Thẻ nhớ 58 GB
  - LAN Local: `192.168.1.10/24` (DHCP từ router Huawei `HG8041X6-12` `192.168.1.1`)
  - Virtual IP mới: **`192.168.1.250/24`** (`end0:1`) -> Tránh triệt để xung đột IP với camera Imou (`30:24:50:f8:57:af`) đang chiếm `192.168.1.254`
  - Khắc phục lỗi khách quét QR:
    - Chuyển toàn bộ virtual IP sang `192.168.1.250` đồng bộ qua Redbida / `change_ok` (`eth0_virtual_ip`, `end0_virtual_ip`, `wifi_ip`, `qr_ip`).
    - Cấu hình IP alias chuẩn `/24` (`192.168.1.250/24 brd 192.168.1.255 dev end0 label end0:1`) và `arp_ignore=0`, `arp_announce=0`.
    - Chuyển tiếp cổng 80 sang cổng 2023 (`PREROUTING` & `OUTPUT`). Khách vào `http://192.168.1.250` hoặc `:2023` đều mở trang ngay lập tức.
  - Cấu hình PM2 Memory Guard (N=7): `camera`: 180M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/mia`), Sysrq gỡ sạch 100%
  - Trạng thái: RAM khả dụng 1.0 GiB, 14/14 FFmpeg ghi 7 bàn liên tục, file mới sinh đều đặn

- [x] **Node `inut_204_150`** (99 Billiards - CS11 - 1 — `77.88.204.150`, group `99cs111`)
  - Phần cứng: Rockchip RK3528 (ARM64), RAM 1.8 GiB, Thẻ nhớ 118 GB
  - LAN Local: `172.10.17.252` (Cùng LAN với `inut_204_152` và `inut_206_60`), MAC `02:04:69:ff:04:50`
  - Số lượng camera: 12 camera (`camera01` .. `camera13`, all `record`)
  - Fix VPN: Tái lập phiên Tinc VPN kết nối máy chủ `77.88.192.2`, node đã online thông suốt
  - Cấu hình PM2 Memory Guard: `camera`: 220M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 2.6 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/99cs111`), Gỡ 6 node sysrq
  - Trạng thái: 24/25 FFmpeg ghi 12 bàn liên tục, RAM khả dụng >730M, file mới sinh đều đặn

- [x] **Node `inut_204_152`** (99 Billiards - CS11 - 2 — `77.88.204.152`, group `99cs112`)
  - Phần cứng: Rockchip RK3528 (ARM64), RAM 1.8 GiB
  - LAN Local: `172.10.16.109/23` (Trạm cầu nối quét ARP LAN thành công)
  - Số lượng camera: 12 camera
  - Cấu hình PM2 Memory Guard: `camera`: 180M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.8 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p, Sysrq gỡ sạch
  - Trạng thái: RAM khả dụng 1.4 GB, 12/12 FFmpeg ghi hình ổn định

- [x] **Node `inut_206_60`** (99 Billiards - CS11 - 3 — `77.88.206.60`, group `vip`)
  - Phần cứng: Rockchip RK3528 (ARM64), LAN Local: `172.10.16.122/23`, MAC `02:06:06:06:06:60`
  - Hoạt động ổn định cùng phân đoạn LAN với `204_150` và `204_152`

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
  - Trạng thái: Uptime >4 ngày, RAM khả dụng 887M, 22/22 FFmpeg ghi 11 bàn liên tục

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
  - Trạng thái: Hồi sinh toàn bộ 6 camera (sau 8 tiếng đứng hình), RAM khả dụng 1.1 GB, 12/12 FFmpeg ghi đều

- [x] **Node `inut_205_56`** (King Kong Thái Nguyên — `77.88.205.56`, group `mp4`)
  - Phần cứng: Allwinner H313 (ARM64), RAM 1.9 GiB
  - Số lượng camera: 9 camera Dahua (chuẩn hóa 25fps 1536k VBR GOP 50 H.265 AAC)
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

- [x] **Node `inut_207_12`** (Poseidon Billiards — `77.88.207.12`, group `hk`)
  - Phần cứng: ARMv7l (rk322x), 17 camera Dahua
  - Cấu hình fix: Khắc phục camera16 chip T31N PTS crash-loop (chuyển H.264 Main 2048k, `rtsp_transport=tcp`)
  - Trạng thái: 34/34 FFmpeg ghi 17 bàn liên tục, RAM khả dụng 1.0 GiB

- [x] **Các node NVR Magic Arena Đà Nẵng** (`inut_205_122`, `inut_205_123`, `inut_205_124`)
  - Chuẩn hóa toàn bộ 64 kênh NVR về 2560x1440 2048k VBR SmartCodec true
  - ZRAM 2.8 GiB, 88 tiến trình FFmpeg ghi hình mượt mà trên cả 3 box

- [x] **Node `inut_202_213`** (Núi Thành Billiards Center — `77.88.202.213`, group `nuithanh`)
  - Phần cứng: Cortex-A7 sunxi (ARMv7l), RAM 1.0 GiB, Thẻ nhớ 30 GB (F2FS)
  - LAN Local: `192.168.1.254/24` (eth0)
  - Số lượng camera: 4 camera Dahua (`192.168.1.240` - `243`), trong đó Cam 3, 4 online 3K H.265 SmartCodec
  - Nguyên nhân hay treo trước đó:
    - `vm.swappiness = 0` và ZRAM chỉ 500MB khiến kernel không dùng swap, thiếu RAM dẫn đến đơ máy
    - Node-RED `ok2` flow có bẫy `load average > 12` gọi `echo b > /proc/sysrq-trigger` gây sập nguồn tức thì
    - `netwatch.service` ping router `192.168.1.1` lỗi 2 lần là kích hoạt `echo b > /proc/sysrq-trigger` (reboot lặp vào 22h-23h hàng đêm)
    - Thẻ nhớ `/media/usb1` đầy 81% do không có Storage Guard và Shinobi thiếu `deleteOld`
  - Cấu hình đã xử lý:
    - Vô hiệu hóa toàn bộ `sysrq`: gỡ bẫy Node-RED, trung hòa netwatch, `sysctl kernel.sysrq = 0`
    - Cấu hình ZRAM Swap 1.0 GiB unconstrained, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`
    - PM2 Memory Guard: `camera: 160M` | `inut: 240M` | `ota-mqtt: 100M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p theo chuẩn mới 90% (`/media/usb1/nuithanh`), cấu hình `max_keep_days=3`
  - Trạng thái: RAM khả dụng >520 MB, ZRAM 1.0 GB đệm an toàn, các luồng đang ghi hình ổn định

- [x] **Node `inut_204_222`** (Hiếu Billiards Arena — `77.88.204.222`, group `job`)
  - Phần cứng: Amlogic S912 8-core Cortex-A53 (aarch64), RAM 1.8 GiB, Thẻ nhớ 58 GB
  - Số lượng camera: 6 camera Dahua 3K (`192.168.1.237` - `242`), 2880x1620 25fps H.265 SmartCodec AAC
  - Nguyên nhân hay treo trước đó:
    - ZRAM bị tắt hoàn toàn (`ENABLED=false`, Swap = 0B, `swappiness = 0`), khiến 6 camera 3K ngốn sạch RAM gây nghẽn
    - Bẫy sysrq: Node-RED có bẫy `load average > 12`, `/usr/local/bin/force_reboot.sh`, và `netwatch.service` đều gọi `echo b > /proc/sysrq-trigger`
    - Trên chip Amlogic S912, lệnh sysrq làm đơ cứng bus lưu trữ `meson-gx-mmc` khi đang ghi nhiều luồng video, khiến box bị treo cứng phải rút nguồn
    - `change_ok` đặt ngưỡng restart quá cao (`max_free_ram_restart_camera: 300`) kết hợp lịch restart 03:30, 15:00, 23:00 làm `camera` restart liên tục 63 lần
    - Thẻ nhớ `/media/usb1` phình lên 71% (41 GB) do không có Storage Guard và thiếu `deleteOld`
  - Cấu hình đã xử lý:
    - Kích hoạt ZRAM Swap 1.8 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`
    - Khóa triệt để sysrq ở kernel (`sysctl kernel.sysrq = 0`), gỡ bỏ toàn bộ lệnh sysrq trong Node-RED, force_reboot và netwatch
    - PM2 Memory Guard: `camera: 180M` | `inut: 280M` | `ota-mqtt: 120M` với log `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% (`/media/usb1/job`), cấu hình Shinobi `deleteOld=true`, `max_keep_days=3`, `wallclock=0`
  - Trạng thái: Đủ 12/12 tiến trình FFmpeg ghi 6 bàn 3K liên tục, ZRAM 1.8 GiB đệm mượt, hết hoàn toàn nguy cơ treo bus

- [x] **Node `inut_204_161`** (NTV Billiards — `77.88.204.161`, group `ntv`)
  - Phần cứng: Amlogic S912 8-core Cortex-A53 (aarch64), RAM 1.8 GiB, Thẻ nhớ 117 GB
  - Số lượng camera: 12 camera Dahua 3K (`192.168.1.211` - `222`), 2880x1620 25fps H.265 SmartCodec AAC
  - Nguyên nhân hay treo trước đó:
    - Dùng `/swapfile` trên eMMC flash kèm `vm.swappiness = 0` (swap không bao giờ hoạt động), ZRAM bị tắt (`ENABLED=false`) khiến 12 camera 3K ngốn cạn RAM (chỉ còn ~480MB trống)
    - Bẫy sysrq: Node-RED có bẫy `load average > 12` gọi `echo b > /proc/sysrq-trigger`, trong khi `/etc/sysctl.d/` bật `kernel.sysrq = 1`
    - Trên chip Amlogic S912, lệnh sysrq làm đơ cứng bus lưu trữ `meson-gx-mmc` khi đang ghi đồng thời 24 luồng video
    - `change_ok` đặt ngưỡng restart RAM quá cao (`max_free_ram_restart_camera: 250`, `force_restart: 150`)
    - Thẻ nhớ `/media/usb1` phình lên 74% (86 GB / 117 GB) do không có Storage Guard và Shinobi thiếu `deleteOld`
  - Cấu hình đã xử lý:
    - Xóa bỏ `/swapfile` eMMC, giải phóng 1GB flash; kích hoạt ZRAM Swap 1.8 GiB unconstrained (`lz4`)
    - Cấu hình `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `sysctl kernel.sysrq = 0`
    - Gỡ bỏ hoàn toàn lệnh sysrq trong flow Node-RED `ok2`
    - PM2 Memory Guard: `camera: 220M` (12 cameras) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% (`/media/usb1/ntv`), cấu hình Shinobi `deleteOld=true`, `max_keep_days=3`, `wallclock=0`
  - Trạng thái: Đủ 24/24 tiến trình FFmpeg ghi 12 bàn 3K liên tục, RAM khả dụng tăng lên >690 MB, ZRAM 1.8 GiB đệm an toàn, triệt tiêu nguy cơ treo bus

- [x] **Node `inut_202_226`** (Queen Billiards Club — `77.88.202.226`, group `giaphong`)
  - Phần cứng: Cortex-A7 sunxi (ARMv7l), RAM 1.0 GiB, Thẻ nhớ 59 GB
  - LAN Local: `192.168.1.254/24` (eth0)
  - Số lượng camera: 5 camera Dahua (`192.168.1.240` - `244`), 1920x1080 25fps H.265 SmartCodec AAC (Cam 1-4 online)
  - Nguyên nhân chết luồng / treo máy trước đó:
    - MariaDB 10.6 bị deadlock trong vòng lặp crash recovery với `liburing` trên nền kernel sunxi ARMv7l 32-bit (PID 838 ngốn 65% CPU liên tục suốt 1 ngày 2 giờ, socket `/run/mysqld/mysqld.sock` không thể mở)
    - Shinobi không kết nối được MySQL nên không thể nạp danh sách monitor, dẫn đến FFmpeg = 0, camera đứng hình hoàn toàn từ đêm 09/10
    - `vm.swappiness = 0` và ZRAM chỉ 500MB khiến kernel không dùng swap, cạn kiệt RAM
    - Node-RED `ok2` flow có bẫy `load average > 12` gọi `echo b > /proc/sysrq-trigger` gây sập nguồn đột ngột, làm hỏng InnoDB log
    - Thẻ nhớ chưa có Storage Guard và Shinobi thiếu `deleteOld`
  - Cấu hình đã xử lý:
    - Khắc phục triệt để lỗi MariaDB: cấu hình `/etc/mysql/mariadb.conf.d/99-innodb-aio.cnf` (`innodb_use_native_aio = 0`), MariaDB khởi động mượt mà trong 1 giây
    - Kích hoạt ZRAM Swap 1.0 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`
    - Khóa triệt để sysrq ở kernel (`sysctl kernel.sysrq = 0`), gỡ bỏ bẫy sysrq trong Node-RED
    - PM2 Memory Guard: `camera: 160M` | `inut: 240M` | `ota-mqtt: 100M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% (`/media/usb1/giaphong`), cấu hình Shinobi `deleteOld=true`, `max_keep_days=3`, `wallclock=0`
  - Trạng thái: Hồi sinh toàn bộ hệ thống ghi hình, đủ 8 tiến trình FFmpeg ghi 4 bàn liên tục, RAM khả dụng >650 MB, ZRAM 1.0 GB đệm an toàn

- [x] **Node `inut_205_120`** (Viễn Đông Pickleball — `77.88.205.120`, group `tanminh`)
  - Phần cứng: Allwinner H616/H618 4-core Cortex-A53 (aarch64), RAM 1.9 GiB, Thẻ nhớ 59 GB
  - LAN Local: `192.168.20.204/24` (DHCP), Virtual IP: `192.168.20.252/32` (eth0)
  - Số lượng camera: 4 camera Dahua 3K (`192.168.20.205`, `.206`, `.136`, `.137`), 2880x1620 30fps H.265 SmartCodec AAC
  - Nguyên nhân và kết quả kiểm tra tính năng tải video:
    - Khách tải clip đang trong quá trình ghi (chưa đóng container MP4 thiếu `moov atom`) sẽ báo lỗi; các file đã ghi xong hoàn tất tải mượt mà qua API `/cut-video-stream`
    - Trước đó ZRAM bị tắt (`ENABLED=false`, Swap = 0B, `swappiness = 0`), khiến RAM thực bị ép xuống mức nguy hiểm khi nhiều khách tải video
    - Node-RED `ok2` flow có bẫy `load average > 12` gọi `echo b > /proc/sysrq-trigger`, tiềm ẩn nguy cơ sập nguồn đột ngột
    - Thẻ nhớ `/media/usb1` phình lên 74% (43 GB / 59 GB) do không có Storage Guard và Shinobi thiếu `deleteOld`
  - Cấu hình đã xử lý:
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`
    - Khóa triệt để sysrq ở kernel (`sysctl kernel.sysrq = 0`), gỡ bỏ bẫy sysrq trong Node-RED
    - PM2 Memory Guard: `camera: 180M` | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% (`/media/usb1/tanminh`), cấu hình Shinobi `deleteOld=true`, `max_keep_days=3`, `wallclock=0`
  - Trạng thái: 8/8 tiến trình FFmpeg ghi 4 sân Pickleball 3K liên tục, tải clip ổn định, RAM khả dụng >820 MB, ZRAM 1.9 GiB đệm mượt

- [x] **Node `inut_205_55`** (Tuyết Linh Billiards Club — `77.88.205.55`, group `tuyetlinh`)
  - Phần cứng: Allwinner H616/H618 4-core Cortex-A53 (aarch64), RAM 1.9 GiB, Thẻ nhớ 60 GB
  - Số lượng camera: 11 camera Dahua (`192.168.1.171`, `.181` - `.186`, `.191`, `.194`, `.195`, `.199`), H.265 AAC
  - Nguyên nhân và các điểm nghẽn trước đó:
    - ZRAM bị bóp nghẽn bởi `MEM_LIMIT_PERCENTAGE=50`, swap đã dùng tới 439 MB sắp chạm trần nén, có nguy cơ gây lỗi kernel `-ENOMEM` và kích hoạt OOM killer
    - Kernel bật cờ `kernel.sysrq = 176`, `vm.swappiness = 60` chưa tối ưu cho flash wear
    - Node-RED `ok2` flow có bẫy `load average > 12` gọi `echo b > /proc/sysrq-trigger`, tiềm ẩn nguy cơ sập nguồn đột ngột khi quán đông khách
    - Storage Guard cũ đặt ngưỡng 85% $\rightarrow$ 75%, chưa áp dụng chuẩn mới 90% $\rightarrow$ 80% (làm xóa bớt ngày lưu trữ của quán)
  - Cấu hình đã xử lý:
    - Mở trần hoàn toàn ZRAM Swap: `MEM_LIMIT_PERCENTAGE=100`, unconstrained `mem_limit=0`
    - Cấu hình `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `sysctl kernel.sysrq = 0`
    - Gỡ bỏ hoàn toàn lệnh sysrq trong flow Node-RED `ok2`, nâng ngưỡng kiểm tra tải an toàn
    - PM2 Memory Guard: `camera: 210M` (11 cameras) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Nâng cấp Storage Guard 15p lên chuẩn mới 90% $\rightarrow$ 80% (`/media/usb1/tuyetlinh`)
  - Trạng thái: Đủ 22-23 tiến trình FFmpeg ghi 11 bàn liên tục, RAM khả dụng >960 MB, ZRAM 1.9 GiB đệm mượt mà không lo `-ENOMEM`

- [x] **Node `inut_204_53`** (City Billiards Club TN - CS3 — `77.88.204.53`, group `4953`)
  - Phần cứng: Rockchip RK3528/RK3328 4-core Cortex-A53 (aarch64), RAM 3.9 GiB, Thẻ nhớ 239 GB (F2FS)
  - Số lượng camera: 16 camera Dahua 3K (`192.168.1.151` - `168`), 2880x1620 30fps H.265 SmartCodec AAC
  - Nguyên nhân và các điểm nghẽn trước đó:
    - Thẻ nhớ `/media/usb1` bị **tràn 100% dung lượng đĩa (239 GB / 239 GB, Available = 0B)**, khiến FFmpeg không thể ghi video, số tiến trình FFmpeg bị rớt từ 32 xuống chỉ còn 8-9 luồng
    - Chưa có Storage Guard tự động dọn rác và Shinobi thiếu cấu hình tự xóa clip cũ (`deleteOld=false`)
    - ZRAM Swap bị tắt hoàn toàn (`ENABLED=false`, Swap = 0B, `swappiness = 0`), chưa có cơ chế đệm swap cho 16 luồng 3K
    - Chưa có PM2 Memory Guard, Node.js heap phình to không giới hạn
    - Chưa có Smart Watchdog kiểm tra phục hồi tiến trình FFmpeg
  - Cấu hình đã xử lý:
    - Giải cứu khẩn cấp dung lượng đĩa: xóa bỏ video cũ ngày 10/10 và rạng sáng 11/10, giải phóng ngay **122 GB dung lượng trống** (đưa đĩa về 50%)
    - Kích hoạt ZRAM Swap 3.8 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`
    - Khóa triệt để sysrq ở kernel (`sysctl kernel.sysrq = 0`), gỡ bỏ bẫy sysrq trong Node-RED
    - PM2 Memory Guard: `camera: 260M` (16 cameras) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/4953`), cấu hình Shinobi `deleteOld=true`, `max_keep_days=3`, `wallclock=0`
  - Trạng thái: Hồi sinh toàn bộ 16 camera, đủ 23-32 tiến trình FFmpeg ghi hình liên tục, thẻ nhớ còn trống 122 GB (50%), RAM khả dụng >2.7 GB, Swap ZRAM 3.8 GiB đệm mượt

- [x] **Node `inut_205_60`** (Bata Billiards Club — `77.88.205.60`, group `bata`)
  - Phần cứng: Allwinner H616/H618 4-core Cortex-A53 (aarch64), RAM 1.9 GiB, Thẻ nhớ 58 GB
  - Số lượng camera: 11 camera Hikvision (`192.168.1.202`, `.204`, `.206` - `.212`, `.214`, `.216`), 1920x1080 25fps H.265 SmartCodec VBR 1024kbps
  - Nguyên nhân và các điểm nghẽn trước đó:
    - Thẻ nhớ `/media/usb1` bị **tràn 100% dung lượng đĩa (58 GB / 58 GB, Available = 0B)** từ ngày 08/10, khiến FFmpeg không thể ghi tiếp, số tiến trình FFmpeg bị rớt từ 22 xuống chỉ còn 3 luồng
    - Chưa có Storage Guard tự động dọn rác và Shinobi thiếu cấu hình tự xóa clip cũ (`deleteOld: false`)
    - ZRAM Swap bị tắt hoàn toàn (`ENABLED=false`, Swap = 0B, `swappiness = 0`), khiến RAM thực bị ép xuống mức nguy hiểm
    - Bẫy sysrq: `netwatch.service` ping router `192.168.1.1` thất bại 2 lần là gọi `echo b > /proc/sysrq-trigger`, trong khi kernel đang bật `kernel.sysrq = 1`
    - Chưa có PM2 Memory Guard, Node.js heap không bị giới hạn
  - Cấu hình đã xử lý:
    - Giải cứu khẩn cấp dung lượng đĩa: xóa bỏ video cũ ngày 07/10, giải phóng ngay **39 GB dung lượng trống** (đưa đĩa về 33% rồi sau đó 4%)
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`
    - Khóa triệt để sysrq ở kernel (`sysctl kernel.sysrq = 0`), trung hòa lệnh sysrq trong `netwatch_loop.sh`
    - PM2 Memory Guard: `camera: 210M` (11 cameras) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/bata`), cấu hình Shinobi `deleteOld=true`, `max_keep_days=3`, `wallclock=0`
  - Trạng thái: Hồi sinh toàn bộ 11 camera Hikvision, đủ 22-23 tiến trình FFmpeg ghi hình liên tục, thẻ nhớ còn trống >50 GB, RAM khả dụng >850 MB, Swap ZRAM 1.9 GiB đệm mượt

- [x] **Node `inut_204_38`** (Top Billiards Thái Nguyên - Box 1 — `77.88.204.38`, group `ux0pAUigSn`)
  - Phần cứng: Rockchip RK3528/RK3328 4-core Cortex-A53 (aarch64), RAM 3.9 GiB, Thẻ nhớ 115 GB
  - Mạng LAN & IP: `192.168.1.133/24` (eth0 DHCP), IP ảo: **`192.168.1.133`** (`eth0:1`)
  - Số lượng camera: 13 camera Dahua (`Camera01` .. `Camera13` tương ứng Bàn 01 đến Bàn 13 trên đầu ghi Dahua `192.168.1.73` channels 1..13)
  - Cấu hình Onboarding & Golden Hardening:
    - Cấu hình Shinobi REST API: 13 camera remux `vcodec=copy`, `acodec=no`, `cust_record="-tag:v hvc1"`, `wall_clock_timestamp_ignore="0"`, `cutoff=5`
    - Cấu hình Branding: `ui_title`: "Top Billiards Thái Nguyên", `company_name`: "Billiard Live", logo header & hashtag chuẩn Bida
    - Cấu hình IP ảo: gán `192.168.1.133` đồng bộ vào `change_ok` (`eth0_virtual_ip`, `end0_virtual_ip`, `wifi_ip`, `qr_ip`), chuyển tiếp iptables port 80 -> 2023
    - Kích hoạt ZRAM Swap 3.8 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, `kernel.sysrq = 0`
    - PM2 Memory Guard: `camera: 230M` (13 cameras) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/ux0pAUigSn`), cấu hình Shinobi `deleteOld=true`, `max_keep_days=3`
  - Trạng thái: Đủ 26/26 tiến trình FFmpeg (13 stream + 13 record), video mới sinh đều đặn, RAM khả dụng >2.6 GB, ZRAM 3.8 GiB đệm mượt

- [x] **Node `inut_204_83`** (Top Billiards Thái Nguyên - Box 2 — `77.88.204.83`, group `YZT7lO2CzM`)
  - Phần cứng: Rockchip RK3528/RK3328 4-core Cortex-A53 (aarch64), RAM 3.9 GiB, Thẻ nhớ 58 GB
  - Mạng LAN & IP: `192.168.1.239/24` (eth0 DHCP), IP ảo: **`192.168.1.239`** (`eth0:1`)
  - Số lượng camera: 13 camera Dahua (`Camera01` .. `Camera13` tương ứng Bàn 14 đến Bàn 26 trên đầu ghi Dahua `192.168.1.73` channels 14..26)
  - Cấu hình Onboarding & Golden Hardening:
    - Cấu hình Shinobi REST API: 13 camera remux `vcodec=copy`, `acodec=no`, `cust_record="-tag:v hvc1"`, `wall_clock_timestamp_ignore="0"`, `cutoff=5`
    - Cấu hình Branding: `ui_title`: "Top Billiards Thái Nguyên", `company_name`: "Billiard Live", logo header & hashtag chuẩn Bida
    - Cấu hình IP ảo: gán `192.168.1.239` đồng bộ vào `change_ok` (`eth0_virtual_ip`, `end0_virtual_ip`, `wifi_ip`, `qr_ip`), chuyển tiếp iptables port 80 -> 2023
    - Kích hoạt ZRAM Swap 3.8 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, `kernel.sysrq = 0`
    - PM2 Memory Guard: `camera: 230M` (13 cameras) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/YZT7lO2CzM`), cấu hình Shinobi `deleteOld=true`, `max_keep_days=3`
  - Trạng thái: Đủ 26/26 tiến trình FFmpeg (13 stream + 13 record), video mới sinh đều đặn, RAM khả dụng >2.6 GB, ZRAM 3.8 GiB đệm mượt
