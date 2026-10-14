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

### 1.1 Quy Trình Onboarding Chuẩn & Phòng Chống Lỗi (Golden Onboarding SOP & Pitfall Prevention)

Khi triển khai onboarding quán mới (đặc biệt các quán nhiều camera / đa box), BẮT BUỘC tuân thủ 5 nguyên tắc sau để không lặp lại sự cố:
1. **Bắt buộc có `input_map_choices` trong Monitor details**:
   - Khi tạo Monitor JSON qua Shinobi API (`configureMonitor`), trường `details.input_map_choices` **BẮT BUỘC** phải có:
     `{"stream": [], "snap": [], "record": [], "record_timelapse": [], "detector": [], "detector_object": [], "detector_sip_buffer": []}` và `input_maps: ""`.
   - *Hậu quả nếu thiếu:* Shinobi `builders.js:398` sẽ văng lỗi `TypeError: Cannot read properties of undefined (reading 'stream')`, làm crash-loop `camera.js` và khiến toàn bộ FFmpeg = 0.
2. **Kích thước độ phân giải ghi hình (`width` x `height`) chuẩn theo camera**:
   - Luôn cập nhật `width` và `height` trong bảng `Monitors` theo đúng độ phân giải thực tế của luồng camera/NVR ($1920 \times 1080$ cho 2MP/1080p, $2880 \times 1620$ cho 3K, $2560 \times 1440$ cho 4MP).
   - Trong `details`: gán `stream_scale_x: ""` và `stream_scale_y: ""` để passthrough video gốc không bị bóp méo, gán `detector_scale_x`/`detector_scale_y` bằng độ phân giải stream. Tuyệt đối không để sót mặc định `640x480`.
3. **Cài đặt Dung lượng tối đa tài khoản Shinobi (`Users.details.size`)**:
   - Shinobi mặc định chỉ cấp hạn mức **10,000 MB (10 GB)** nếu user details thiếu trường `size`. Nếu không cấu hình, khi video đạt 10 GB Shinobi sẽ báo đầy ổ đĩa ảo và ngừng ghi hình dù thẻ nhớ còn trống hàng chục GB.
   - Bắt buộc cập nhật `Users.details` trường `size` tương ứng với dung lượng thẻ nhớ thực tế:
     - Thẻ 64 GB $\rightarrow$ `size: "50000"` (50 GB), `days: "3"`, `size_video_percent: "90"`
     - Thẻ 128 GB $\rightarrow$ `size: "100000"` (100 GB), `days: "3"`, `size_video_percent: "90"`
     - Thẻ 256 GB $\rightarrow$ `size: "210000"` (210 GB), `days: "3"`, `size_video_percent: "90"`
4. **Cấu hình FRP Subdomain & IP ảo cho hệ thống Multi-Box cùng 1 quán**:
   - Box 1: Subdomain `store-<tênquán>1`, `c-store-<tênquán>1`
   - Box 2: Subdomain `store-<tênquán>2`, `c-store-<tênquán>2`
   - Gán IP ảo chính là IP card `eth0` hiện tại, kèm chuyển tiếp iptables port 80 -> 2023.
5. **Phân bổ kênh NVR tuần tự**:
   - Chia đều số kênh camera bàn giữa các box (ví dụ quán 26 bàn: Box 1 kênh 1..13, Box 2 kênh 14..26).
   - Audio codec nếu là `pcm_alaw`/`pcm_mulaw` phải đặt `acodec: "no"` để chống ngốn CPU phần mềm.
6. **Kỷ luật lưu trữ: CẤM TỰ Ý FORMAT KHI CHƯA HỎI KỸ**:
   - Tuyệt đối KHÔNG tự ý format thẻ nhớ/ổ đĩa.
   - Khi người dùng bảo format, BẮT BUỘC phải hỏi lại xác nhận kỹ càng: nêu rõ IP thiết bị, tên ổ đĩa (`/dev/mmcblk0p1` thẻ nhớ vs `/dev/mmcblk2` eMMC boot), dung lượng, và cảnh báo mất dữ liệu video. Chỉ thực hiện khi người dùng xác nhận lần hai.
7. **Automount an toàn: CẤM đưa thẻ nhớ ngoài vào `/etc/fstab`**:
   - Tuyệt đối **KHÔNG** đưa `/dev/mmcblk0p1` vào `/etc/fstab`. Nếu ghi vào fstab mà lỡ không có thẻ nhớ hoặc thẻ nhớ hỏng phân vùng, `systemd` sẽ block boot và văng vào Emergency Mode (treo máy hoàn toàn, mất mạng/SSH, brick từ xa).
   - **Chuẩn an toàn**: Giữ `/etc/fstab` sạch (chỉ chứa `/` và `/tmp`). Sử dụng cơ chế udev automount kết hợp cấu hình `/etc/media-automount.d/{f2fs,ext4,auto}` với cờ `AUTOMOUNT_OPTS="errors=continue,noatime,users"`.
   - *Hiệu quả:* Có thẻ nhớ thì tự mount vào `/media/usb1`, không có thẻ nhớ box vẫn boot vào Linux bình thường, mạng và SSH luôn online 100%.
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
  - FRP Subdomains: Store: **`http://store-toptn1.video.io.vn`** | C-Store: **`http://c-store-toptn1.video.io.vn`**
  - Số lượng camera: 13 camera Dahua (`Camera01` .. `Camera13` tương ứng Bàn 01 đến Bàn 13 trên đầu ghi Dahua `192.168.1.73` channels 1..13)
  - Cấu hình Onboarding & Golden Hardening:
    - Cấu hình Shinobi REST API: 13 camera 1080p ($1920 \times 1080$) remux `vcodec=copy`, `acodec=no`, `cust_record="-tag:v hvc1"`, `wall_clock_timestamp_ignore="0"`, `cutoff=5`
    - Cấu hình Dung lượng tối đa: Shinobi User `size: 100000` MB (100 GB / thẻ 128 GB), `days: 3`, `size_video_percent: 90`
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
  - FRP Subdomains: Store: **`http://store-toptn2.video.io.vn`** | C-Store: **`http://c-store-toptn2.video.io.vn`**
  - Số lượng camera: 13 camera Dahua (`Camera01` .. `Camera13` tương ứng Bàn 14 đến Bàn 26 trên đầu ghi Dahua `192.168.1.73` channels 14..26)
  - Cấu hình Onboarding & Golden Hardening:
    - Cấu hình Shinobi REST API: 13 camera 1080p ($1920 \times 1080$) remux `vcodec=copy`, `acodec=no`, `cust_record="-tag:v hvc1"`, `wall_clock_timestamp_ignore="0"`, `cutoff=5`
    - Cấu hình Dung lượng tối đa: Shinobi User `size: 50000` MB (50 GB / thẻ 64 GB), `days: 3`, `size_video_percent: 90`
    - Cấu hình Branding: `ui_title`: "Top Billiards Thái Nguyên", `company_name`: "Billiard Live", logo header & hashtag chuẩn Bida
    - Cấu hình IP ảo: gán `192.168.1.239` đồng bộ vào `change_ok` (`eth0_virtual_ip`, `end0_virtual_ip`, `wifi_ip`, `qr_ip`), chuyển tiếp iptables port 80 -> 2023
    - Kích hoạt ZRAM Swap 3.8 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, `kernel.sysrq = 0`
    - PM2 Memory Guard: `camera: 230M` (13 cameras) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/YZT7lO2CzM`), cấu hình Shinobi `deleteOld=true`, `max_keep_days=3`
  - Trạng thái: Đủ 26/26 tiến trình FFmpeg (13 stream + 13 record), video mới sinh đều đặn, RAM khả dụng >2.6 GB, ZRAM 3.8 GiB đệm mượt

- [x] **Node `inut_204_165`** (Nguyễn Huệ Pickleball — `77.88.204.165`, group `KT8zqe2yGq`)
  - Phần cứng: Amlogic S912 8-core Cortex-A53 (aarch64), RAM 1.8 GiB, Thẻ nhớ 58 GB
  - Mạng LAN & IP: `192.168.1.10/24` (eth0 DHCP), IP ảo: **`192.168.1.254/24`** (`eth0:1`)
  - FRP Subdomains: Store: **`http://store-nguyenhue.video.io.vn`** | C-Store: **`http://c-store-nguyenhue.video.io.vn`**
  - Số lượng camera: 4 camera Dahua 4MP (`192.168.1.201` .. `204`), 2688x1664 20fps H.265 AAC
  - Các điểm nghẽn trước đó:
    - ZRAM bị tắt hoàn toàn (`ENABLED=false`, Swap = 0B, `swappiness = 0`), nguy cơ cạn RAM khi stream/tải clip
    - Bẫy sysrq: Node-RED `flow.json` cũ có bẫy sysrq, kernel bật `kernel.sysrq = 1`
    - Watchdog cũ chỉ kiểm tra PID `camera.js`, không kiểm tra FFmpeg processes
    - Chưa có Storage Guard 15p, Shinobi thiếu `deleteOld: true` và `deleteOrphans: true`
    - Bảng `Monitors` có `width` và `height` là NULL, `Users.details` thiếu trường `size` hạn mức thẻ nhớ
    - Chưa cấu hình redirect port 80 -> 2023 trên iptables
  - Cấu hình đã xử lý:
    - Kích hoạt ZRAM Swap 1.8 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Sanitized triệt để lệnh sysrq trong các flow Node-RED
    - Cấu hình IP ảo `192.168.1.254/24` (`eth0:1`), chuyển tiếp iptables port 80 -> 2023 (`PREROUTING` & `OUTPUT`)
    - PM2 Memory Guard: `camera: 180M` | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Nâng cấp Smart Watchdog 60s kiểm tra `pgrep -c ffmpeg` (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/KT8zqe2yGq`), dọn rác và folder rỗng
    - Cấu hình Shinobi `deleteOld=true`, `deleteOrphans=true`, `max_keep_days=3`, `wallclock=0`
    - Cập nhật độ phân giải chuẩn $2688 \times 1664$ (4MP) và user size `50000` MB (50 GB / thẻ 64 GB)
  - Trạng thái: Đủ 8/8 tiến trình FFmpeg ghi 4 sân Pickleball 4MP liên tục, RAM khả dụng >990 MB, ZRAM 1.8 GiB đệm an toàn

- [x] **Node `inut_207_18`** (Câu Lạc Bộ Trí Nguyễn Café — `77.88.207.18`, group `tringuyen`)
  - Phần cứng: Cortex-A7 sunxi/rockchip (ARMv7l), RAM 1.9 GiB, Thẻ nhớ 58 GB F2FS
  - Mạng LAN & IP: `192.168.1.5/24` (end0 DHCP), IP ảo: **`192.168.1.252/24`** (`end0:1`)
  - FRP Subdomains: Store: **`http://store-tringuyen.video.io.vn`** | C-Store: **`http://c-store-tringuyen.video.io.vn`**
  - Số lượng camera: 10 camera Dahua 3K (`192.168.1.190` - `199`), 2880x1620 30fps H.265 SmartCodec AAC
  - Các điểm nghẽn trước đó:
    - Rò rỉ tiến trình FFmpeg nghiêm trọng: 28 tiến trình snapshot zombie tích tụ từ tháng 8/tháng 9 treo vĩnh viễn (tổng 48 FFmpeg), ngốn cạn RAM thực (chỉ còn 294 MiB trống)
    - `camera.js` uptime 2 tháng không giới hạn heap, phình to lên tới 336.3 MB
    - ZRAM bị tắt hoàn toàn (`ENABLED=false`, Swap = 0B, `swappiness = 0`)
    - Thẻ nhớ `/media/usb1` phình lên 85% (49 GB / 58 GB) do chưa có Storage Guard và thiếu `deleteOld`
    - Bảng `Monitors` lưu kích thước cũ `640x480` và thiếu `max_keep_days`, `Users.details` thiếu `days: 3`
  - Cấu hình đã xử lý:
    - Tiêu diệt toàn bộ 28 tiến trình snapshot zombie, phục hồi chuẩn xác 20 tiến trình FFmpeg (10 stream + 10 record)
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - PM2 Memory Guard: `camera: 210M` (10 cameras 3K) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Cấu hình IP ảo `192.168.1.252/24` (`end0:1`), cố định trong `/etc/network/interfaces`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/tringuyen`)
    - Cấu hình Shinobi `deleteOld=true`, `deleteOrphans=true`, `max_keep_days=3`, `wallclock=0`
    - Cập nhật độ phân giải chuẩn $2880 \times 1620$ (3K) và user size `50000` MB (50 GB / thẻ 64 GB)
  - Trạng thái: Thẻ nhớ 58 GB `/dev/mmcblk0p1` đã format F2FS an toàn (cách ly ổ eMMC `/dev/mmcblk2p1`), mount bền vững qua `/etc/fstab`, 20/20 tiến trình FFmpeg ghi 10 bàn 3K liên tục, video mới sinh đều đặn, RAM khả dụng >1.1 GB, ZRAM 1.9 GiB đệm mượt

- [x] **Node `inut_205_57`** (Zone+ Thái Nguyên — `77.88.205.57`, group `moi`)
  - Phần cứng: Allwinner H616/H618 4-core Cortex-A53 (aarch64), RAM 1.9 GiB, Thẻ nhớ 128 GB F2FS (mới thay và format)
  - Mạng LAN & IP: `192.168.1.240/24` (eth0 Static), IP ảo: **`192.168.1.240`** (`eth0:1`)
  - FRP Subdomains: Store: **`http://store-zoneplus.video.io.vn`** | C-Store: **`http://c-store-zoneplus.video.io.vn`**
  - Số lượng camera: 12 camera Dahua 2K (`192.168.1.221` .. `232`), 2304x1296 25fps H.264 AAC
  - Các điểm nghẽn trước đó:
    - ZRAM bị bóp nghẽn bởi `ZRAM_PERCENTAGE=50` và `vm.swappiness = 0` (swap không hoạt động dù 12 camera ngốn RAM)
    - Bẫy sysrq: Node-RED `flow.json` có rule kiểm tra tải gọi `echo b > /proc/sysrq-trigger`, `netwatch.service` ping gateway lỗi là gọi sysrq, kernel bật `kernel.sysrq = 176`
    - Chưa có Storage Guard 15p, Shinobi thiếu `deleteOld: true` và `deleteOrphans: true`
    - Bảng `Monitors` lưu kích thước cũ `640x480` và `wall_clock_timestamp_ignore="1"`, `Users.details` thiếu `days: 3`
    - Chưa có iptables redirect port 80 -> 2023
  - Cấu hình đã xử lý:
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Sanitized triệt để lệnh sysrq trong flow Node-RED và netwatch_loop.sh
    - Cấu hình IP ảo `192.168.1.240/24` (`eth0:1`), cài đặt iptables redirect port 80 -> 2023 (`PREROUTING` & `OUTPUT`)
    - PM2 Memory Guard: `camera: 220M` (12 cameras 2K) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/moi`)
    - Cấu hình Shinobi `deleteOld=true`, `deleteOrphans=true`, `max_keep_days=3`, `wallclock=0`
    - Cập nhật độ phân giải chuẩn $2304 \times 1296$ (2K) và user size `100000` MB (100 GB / thẻ 128 GB F2FS mới)
  - Trạng thái: Đủ 24/24 tiến trình FFmpeg (12 stream + 12 record), video mới sinh đều đặn, RAM khả dụng >920 MB, ZRAM 1.9 GiB đệm an toàn

- [x] **Node `inut_205_31`** (Khét Billiards Club — `77.88.205.31`, group `khet`)
  - Phần cứng: Cortex-A53 / ARMv8 (aarch64), RAM 1.9 GiB, Thẻ nhớ 58 GB
  - Mạng LAN & IP: `192.168.1.254/24` (end0 Static, giữ nguyên theo yêu cầu)
  - Số lượng camera: 6 camera Dahua/Imou 1080p (`camera01` .. `camera06`)
  - Các điểm nghẽn trước đó:
    - Kéo luồng kép (double-streaming): Shinobi kéo trực tiếp từ camera con `192.168.1.152`, `192.168.1.153`, `192.168.1.209` (Imou IPC-F22 Wi-Fi) trong khi đầu ghi NVR `192.168.1.212` cũng đang kéo $\rightarrow$ Quá tải DSP/Wi-Fi gây giật lag camera khi cắm box
    - Chưa áp dụng bộ Golden Hardening (ZRAM unconstrained, PM2 Memory Guard, Smart Watchdog, Storage Guard 90%)
  - Cấu hình đã xử lý:
    - Chuyển hướng toàn bộ 6 camera kéo luồng qua đầu ghi NVR `192.168.1.212`:
      - `camera01` $\rightarrow$ NVR Channel 2 (`192.168.1.152`, Pool 5+4)
      - `camera02` $\rightarrow$ NVR Channel 5
      - `camera03` $\rightarrow$ NVR Channel 4
      - `camera04` $\rightarrow$ NVR Channel 6
      - `camera05` $\rightarrow$ NVR Channel 7 (`192.168.1.153`, Pool 9+8)
      - `camera06` $\rightarrow$ NVR Channel 8 (`192.168.1.209`, Pool 11+10)
    - Giữ nguyên 100% cấu hình mạng và IP tĩnh `192.168.1.254` của box theo đúng chỉ thị
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - PM2 Memory Guard: `camera: 170M` | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Kích hoạt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/khet`)
    - Cấu hình Shinobi `deleteOld=true`, `deleteOrphans=true`, `max_keep_days=3`, `wallclock=0`, user limit `50000` MB
  - Trạng thái: Đủ 12/12 tiến trình FFmpeg (6 stream + 6 record), video mới sinh liên tục, luồng camera mượt mà qua NVR, không còn hiện tượng giật lag
