# FLEET WATCHLIST & HARDENING INVENTORY
> **Tài liệu theo dõi trạng thái tối ưu hóa và danh mục thiết bị Edge Fleet (ksp-camera-auto)**  
> Cập nhật tự động: Mỗi node khi hoàn tất tối ưu hóa đều được ghi nhận chi tiết về thông số phần cứng, số lượng camera, cấu hình PM2 Memory Guard, ZRAM, Watchdog, Storage Guard và trạng thái vận hành thực tế.

---

## 1. Tiêu Chuẩn Vàng Khi Tối Ưu Hóa (Golden Hardening Checklist)

Mọi node trong danh sách dưới đây đều tuân thủ nghiêm ngặt 8 nguyên tắc cốt lõi:
 1. **ZRAM Swap 100% Unconstrained**: `ZRAM_PERCENTAGE=100`, `MEM_LIMIT_PERCENTAGE=100`, `echo 0 > /sys/block/zram0/mem_limit`, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100` -> Triệt tiêu hoàn toàn lỗi `-ENOMEM` và OOM killer.
 2. **PM2 Memory Guard**: Giới hạn heap chủ động cho Node.js theo công thức số camera: `camera: (120 + N*8)M`, `inut: 280M`, `ota-mqtt: 120M`, tự tái sinh êm sau 1–2s mà không làm gián đoạn hệ thống.
 3. **Bảo toàn Log `/dev/null`**: Cả `pm_out_log_path` và `pm_err_log_path` đều trỏ vào `/dev/null` để chống mòn thẻ nhớ và chống đầy dung lượng `/var/log` / tmpfs.
 4. **Triệt tiêu ngòi nổ tự sát Node-RED**: Loại bỏ hoàn toàn các lệnh `echo b > /proc/sysrq-trigger`, `rm -rf /tmp`, và nhánh `[ "$CPU" -eq 8 ]`. Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
 5. **Smart Watchdog 60s**: Script `/usr/local/bin/shinobi-camera-watchdog.sh` kiểm tra số lượng tiến trình FFmpeg thực tế (`pgrep -c ffmpeg`). Nếu camera online $> 90\text{s}$ mà có 0 FFmpeg thì tự phục hồi ngay.
 6. **Storage Guard 15p (Tiêu chuẩn mới 90%)**: Script `/usr/local/bin/shinobi-storage-guard.sh` targeting thư mục video của quán, dọn rác 0-byte và xoay vòng xả đĩa khi $\ge 90\%$ về mức $\le 80\%$ (do Shinobi đã tự động chừa lại 10% dung lượng an toàn).
 7. **Shinobi Zero-Transcode & Timestamp Fix**: `vcodec=copy`, `acodec=copy` (hoặc `no`), `wall_clock_timestamp_ignore=0` (hoặc passthrough), `hls_time=2`, `max_keep_days=3`, kích hoạt `deleteOld`/`deleteOrphans` trong `conf.json`.
8. **Automount An Toàn & Bỏ Qua Lỗi Thẻ Nhớ (`errors=continue`) - Tuyệt Đối Không Dùng `/etc/fstab`**: CẤM đưa `/dev/mmcblk0p1` vào `/etc/fstab` để chống treo boot khi mất thẻ. Bắt buộc quản lý qua udev automount bằng 3 file profile `/etc/media-automount.d/{f2fs,ext4,auto}` với `AUTOMOUNT_OPTS="errors=continue,noatime,users"` và remount nóng trực tiếp `mount -o remount,noatime,errors=continue /media/usb1` mà **KHÔNG CẦN FORMAT THẺ**, bảo toàn trọn vẹn 100% video ghi hình.
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
7. **Quy trình Automount fix `errors=continue` (CẤM dùng `/etc/fstab`)**:
   - Tuyệt đối **KHÔNG** đưa `/dev/mmcblk0p1` vào `/etc/fstab`. Nếu đưa vào fstab mà lỡ không có thẻ nhớ hoặc thẻ hỏng phân vùng, `systemd` sẽ block boot và văng vào Emergency Mode (treo máy hoàn toàn, mất mạng/SSH, brick từ xa).
   - **Quy trình chuẩn hóa udev automount**:
     1. Tạo 3 file trong `/etc/media-automount.d/`: `f2fs`, `ext4`, `auto` chứa `AUTOMOUNT_OPTS="errors=continue,noatime,users"`.
     2. Dọn sạch `/dev/mmcblk0p1` khỏi `/etc/fstab` (chỉ giữ `/` và `/tmp`).
     3. Chạy lệnh remount nóng: `mount -o remount,noatime,errors=continue /media/usb1`.
     4. *Tuyệt đối KHÔNG cần format lại thẻ nhớ*, toàn bộ video clip ghi hình hiện tại được giữ nguyên vẹn 100%.
8. **Chống Lỗi Lưu Clip (`moov atom not found`) Khi Có Camera Bị Rút Điện / Offline**:
   - **Nguyên nhân**:
     1. Mặc định Shinobi bật `doSnapshot = true`. Khi có camera bị rút điện/offline, mỗi 15s `monitor.js` lại tạo một `Worker` (`snapshot.js` + `ffmpeg -s 500x500`) trước khi ping. Do hàm `processKill(snapProcess)` kiểm tra `proc.pid` (vốn là `undefined` trên Node.js `Worker` thread), các `Worker` này không bị kill sau 5s mà treo suốt 130s, làm phình RAM `camera.js` vượt ngưỡng PM2 (`180M`) và khiến PM2 kill `camera.js` mỗi 90–110s.
     2. Khi `camera.js` bị kill giữa chừng (trước khi hết chu kỳ cắt file 120s), tiến trình `ffmpeg` của các camera đang online bị ngắt đột ngột. Do cờ `-movflags +frag_keyframe...` đặt ở cấp ngoài bị muxer `-f segment` bỏ qua (và bị `-movflags +faststart` ghi đè), file `.mp4` đang ghi dở không có `moov` atom (`moov atom not found`). Khi khởi động lại, Shinobi lại quét và chèn các file hỏng này vào bảng `Videos`.
   - **Giải pháp chuẩn bắt buộc**:
     1. Đặt `"doSnapshot": false` trong `/home/Shinobi/conf.json` và vá `snapProcess.terminate()` trong `/home/Shinobi/libs/monitor.js`.
     2. Đặt `cust_record`: `"-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof"` để truyền thẳng `movflags` vào từng segment `.mp4` $\rightarrow$ Header `moov` được ghi ngay từ byte `0x20`, dù mất điện hay kill `-9` giữa chừng thì clip vẫn xem và tải được 100%.
     3. Đặt ngưỡng PM2 `camera` tối thiểu `280M - 300M` trên kiến trúc 64-bit (`aarch64`) để tránh PM2 kill nhầm khi nhiều camera offline.
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
  - Khắc phục triệt để lỗi lưu clip hỏng khi có 5/7 camera rút điện offline:
    - Đặt `"doSnapshot": false` trong `/home/Shinobi/conf.json` & vá `snapProcess.terminate()` trong `libs/monitor.js` để chặn rò rỉ Worker thread khi camera offline.
    - Cập nhật `cust_record`: `"-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof"` cho toàn bộ 7 camera $\rightarrow$ File `.mp4` có `moov` atom ngay từ giây đầu tiên (`iso5` fragmented MP4), đọc/tải mượt kể cả khi đang ghi.
    - Dọn sạch 1,447 file `.mp4` bị cụt `moov` atom trước đó trong `/media/usb1/mia/` và bảng `Videos`, giữ nguyên 1,357 file `.mp4` hợp lệ.
  - Cấu hình PM2 Memory Guard (N=7): `camera`: 300M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`)
  - Cấu hình Kernel & Swap: ZRAM 1.9 GiB (`lz4`, uncap `mem_limit=0`, `swappiness=80`, `vfs_cache_pressure=100`)
  - Cơ chế bảo vệ: Smart Watchdog 60s, Storage Guard 15p (`/media/usb1/mia`), Sysrq gỡ sạch 100%
  - Trạng thái: RAM `camera` ổn định ~144 MB (0 restart), các camera đang bật điện (`camera03`, `camera06`) ghi hình liên tục và xuất/lưu clip hợp lệ 100%

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
  - Phần cứng: Allwinner H313 (ARM64, kernel 6.6), RAM 1.9 GiB, Thẻ nhớ 60 GB F2FS (`/dev/mmcblk0p1`)
  - Số lượng camera: 12 camera Dahua 3K ($2880 \times 1620$ @ 30fps, H.265) (`camera01`..`camera11`, `camera13`)
  - Nguyên nhân lỗi lưu clip & tự khởi động lại `camera.js` liên tục trước đó:
    1. Watchdog cũ gọi `timeout 15s pm2 pid camera` (khởi chạy Node.js V8 CLI tốn >15s khi CPU H313 gánh 12 luồng 3K), bị timeout trả về rỗng nên tưởng nhầm `camera` chết và tự chạy `pm2 restart camera` hơn 30 lần/ngày, cắt ngang toàn bộ 12 file `.mp4` đang ghi.
    2. `cust_record` chỉ có `-tag:v hvc1` (thiếu `-segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof`), khiến mỗi lần bị restart giữa chu kỳ 5 phút đều sinh ra 12 file `.mp4` cụt `moov` atom (`00000000 mdat`).
    3. `camera13` bị lỗi cú pháp JSON trong `Monitors.details` (`detector_filters`), `/home/Shinobi/conf.json` thiếu `"doSnapshot": false`, `monitor.js` chưa vá `snapProcess.terminate()`, `kernel.sysrq = 1`, thiếu `/etc/media-automount.d/{f2fs,ext4}` (`errors=continue,noatime,users`).
  - Các bước đã xử lý triệt để (Không format thẻ nhớ, bảo toàn 675 file `.mp4` hợp lệ):
    1. Thay thế Watchdog bằng bản tra bảng tiến trình kernel trực tiếp (`pgrep -f '/home/Shinobi/camera\.js'`, tốn `0.001s`, 0% CPU, không bao giờ timeout giả).
    2. Đặt `"doSnapshot": false` trong `conf.json`, vá `snapProcess.terminate()` trong `libs/monitor.js`, sửa JSON `camera13` và cập nhật `width=2880, height=1620` + `cust_record: "-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof"` cho toàn bộ 12 camera.
    3. Dọn sạch **311 file `.mp4` bị cụt `moov` atom (`00000000 mdat`)** và các dòng DB lỗi tương ứng trong `Videos`, giữ nguyên **675 file `.mp4` hợp lệ**.
    4. Cấu hình PM2 Memory Guard: `camera`: 300M | `inut`: 280M | `ota-mqtt`: 120M (log `/dev/null`), Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/toktok3`), Automount `errors=continue,noatime,users`, khóa `kernel.sysrq = 0`, thêm `iptables` redirect `80 -> 2023`.
  - Trạng thái: Đủ 24/24 tiến trình FFmpeg ghi 12 bàn liên tục, file `.mp4` đang ghi có `moov` atom ngay tại byte `0x20` (`iso5`), RAM `camera.js` ~146 MB (0 restart).

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

- [x] **Node `inut_205_123`** (Magic Arena Đà Nẵng - Box 2 — `77.88.205.123`, group `magic2`)
  - Phần cứng: ARMv8 (aarch64), RAM 1.9 GiB, Thẻ nhớ 58 GB F2FS
  - Số lượng camera: 12 camera (`camera01` .. `camera12`)
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% video cũ):
    - Cấu hình automount profile `/etc/media-automount.d/{f2fs,ext4,auto}` với cờ `errors=continue,noatime,users` (an toàn, không ghi vào `/etc/fstab`)
    - Remount nóng trực tiếp: `mount -o remount,noatime,errors=continue /media/usb1`
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/magic2`)
    - PM2 Memory Guard: `camera: 220M` | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Đủ 24/24 tiến trình FFmpeg (12 stream + 12 record), video mới sinh liên tục, bảo toàn trọn vẹn 38 GB video cũ

- [x] **Node `inut_205_124`** (Magic Arena Đà Nẵng - Box 1 — `77.88.205.124`, group `magic1`)
  - Phần cứng: ARMv8 (aarch64), RAM 1.9 GiB, Thẻ nhớ 128 GB (108 GB F2FS)
  - Số lượng camera: 16 camera (`camera01` .. `camera16`)
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% video cũ):
    - Cấu hình automount profile `/etc/media-automount.d/{f2fs,ext4,auto}` với cờ `errors=continue,noatime,users` (an toàn, không ghi vào `/etc/fstab`)
    - Remount nóng trực tiếp: `mount -o remount,noatime,errors=continue /media/usb1`
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/magic1`)
    - PM2 Memory Guard: `camera: 250M` | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Đủ 32/32 tiến trình FFmpeg (16 stream + 16 record), video mới sinh liên tục, bảo toàn trọn vẹn 71 GB video cũ

- [x] **Node `inut_204_150`** (99 Billiards - CS11 - 1 — `77.88.204.150`, group `99cs111`)
  - Phần cứng: Allwinner H6/H616 (ARM64, kernel 6.1), RAM 1.8 GiB, Thẻ nhớ 117 GB F2FS (`/dev/mmcblk1p1`)
  - Số lượng camera: 11 camera (`camera01` .. `camera13`)
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% video cũ):
    - Cấu hình automount profile `/etc/media-automount.d/{f2fs,ext4,auto}` với cờ `noatime,users` (an toàn, không ghi fstab)
    - Kích hoạt ZRAM Swap unconstrained, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/99cs111`) + Smart Watchdog 60s
    - Shinobi `deleteOld=true`, `deleteOrphans=true`
    - PM2 Memory Guard: `camera: 210M` | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Shinobi camera process online, bảo toàn trọn vẹn dữ liệu video trên thẻ 117 GB F2FS

- [x] **Node `inut_204_152`** (99 Billiards - CS11 - 2 — `77.88.204.152`, group `99cs112`)
  - Phần cứng: Allwinner H6/H616 (ARM64, kernel 6.1), RAM 1.8 GiB, Thẻ nhớ 116 GB F2FS (`/dev/mmcblk1p1`)
  - Số lượng camera: 12 camera (`camera01` .. `camera13`)
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% video cũ):
    - Cấu hình automount profile `/etc/media-automount.d/{f2fs,ext4,auto}` với cờ `noatime,users` (an toàn, không ghi fstab)
    - Kích hoạt ZRAM Swap unconstrained, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/99cs112`) + Smart Watchdog 60s
    - Shinobi `deleteOld=true`, `deleteOrphans=true`
    - PM2 Memory Guard: `camera: 220M` | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Shinobi camera process online, bảo toàn trọn vẹn dữ liệu video trên thẻ 116 GB F2FS

- [x] **Node `inut_206_60`** (99 Billiards - CS11 - 3 — `77.88.206.60`, group `vip`)
  - Phần cứng: Rockchip RK3568/RK3588 (ARM64, kernel 6.1), RAM 3.8 GiB, Ổ cứng ngoài 233 GB ext4 (`/dev/sda1`)
  - Số lượng camera: 19 camera bàn VIP
  - Cấu hình đã xử lý (Không format ổ cứng, bảo toàn 100% video cũ):
    - Cấu hình automount profile `/etc/media-automount.d/{f2fs,ext4,auto}` với cờ `errors=continue,noatime,users` (an toàn, không ghi fstab)
    - Remount nóng trực tiếp `errors=continue` trên ext4
    - Kích hoạt ZRAM Swap unconstrained, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/vip`) + Smart Watchdog 60s
    - Shinobi `deleteOld=true`, `deleteOrphans=true`
    - PM2 Memory Guard: `camera: 350M` (19 cameras) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Đủ ~70-95 tiến trình FFmpeg và go2rtc, video mới sinh liên tục, bảo toàn trọn vẹn ổ cứng 233 GB

- [x] **Node `inut_205_24`** (Zone 9 Billiards - 2 — `77.88.205.24`, group `kao`)
  - Phần cứng: Allwinner H616 (ARM64, kernel 6.6), RAM 1.9 GiB, Thẻ nhớ 119 GB F2FS (`/dev/mmcblk0p1`)
  - Số lượng camera: 11 camera (`camera01` .. `camera11`)
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% video cũ):
    - Cấu hình automount profile `/etc/media-automount.d/{f2fs,ext4,auto}` với cờ `errors=continue,noatime,users` (an toàn, không ghi fstab)
    - Remount nóng trực tiếp: `mount -o remount,noatime,errors=continue /media/usb1` (xác nhận `errors=continue` active)
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/kao`) + Smart Watchdog 60s
    - Shinobi `deleteOld=true`, `deleteOrphans=true`
    - PM2 Memory Guard: `camera: 210M` | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Đủ 22/22 tiến trình FFmpeg (11 stream + 11 record), video mới sinh liên tục, bảo toàn trọn vẹn 101 GB video cũ

- [x] **Node `inut_204_220`** (Khánh Hà Billiards — `77.88.204.220`, group `khanhha2`)
  - Phần cứng: Allwinner / Rockchip (ARM64, kernel 6.1), RAM 1.8 GiB, Thẻ nhớ 115 GB ext4 (`/dev/mmcblk1p1`)
  - Số lượng camera: 10 camera (`camera01` .. `camera10`)
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% video cũ):
    - Cấu hình automount profile `/etc/media-automount.d/{ext4,auto,f2fs}` với cờ `errors=continue,noatime,users` (an toàn, không ghi fstab)
    - Remount nóng trực tiếp: `mount -o remount,noatime,errors=continue /media/usb1`
    - Kích hoạt ZRAM Swap unconstrained, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/khanhha2`) + Smart Watchdog 60s
    - Shinobi `deleteOld=true`, `deleteOrphans=true`
    - PM2 Memory Guard: `camera: 210M` | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Đủ 20/20 tiến trình FFmpeg (10 stream + 10 record), video mới sinh liên tục, bảo toàn trọn vẹn 90 GB video cũ

- [x] **Node `inut_207_13`** (TOP ONE / Axis Hải Phòng — `77.88.207.13`, group `hk`)
  - Phần cứng: Cortex-A55 / Rockchip (ARM64, kernel 6.1), RAM 3.2 GiB, Thẻ nhớ 117 GB F2FS (`/dev/mmcblk1p1`)
  - Số lượng camera: 16 camera (`camera01` .. `camera16`)
  - Các điểm nghẽn trước đó:
    - Chưa có Storage Guard: Thẻ nhớ phình lên 97% (112 GB / 117 GB) khiến F2FS bị lỗi I/O và treo tiến trình
    - Chưa có Swap: ZRAM bị tắt (`ENABLED=false`, Swap = 0B)
    - Bẫy sysrq: `kernel.sysrq = 1`
    - PM2 chưa có Memory Guard và log chưa trỏ `/dev/null`
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn video):
    - Cấu hình automount profile `/etc/media-automount.d/{f2fs,ext4,auto}` với cờ `noatime,users` (an toàn, không ghi fstab)
    - Remount nóng `noatime`, chạy `fsck.f2fs` sửa lỗi SIT table thành công
    - Cài đặt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/hk`), chạy xả đĩa tức thời giải phóng từ 97% xuống 39% (trống 72 GB)
    - Kích hoạt ZRAM Swap 2.4 GiB unconstrained (`zstd`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Kích hoạt Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)
    - Shinobi `deleteOld=true`, `deleteOrphans=true`, user limit 100,000 MB
    - PM2 Memory Guard: `camera: 250M` (16 camera) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Đủ 25-32 tiến trình FFmpeg (16 stream + 16 record), video mới sinh liên tục, RAM khả dụng 2.0 GiB, đĩa trống 72 GB

- [x] **Node `inut_205_73`** (Pika Sun Pickleball — `77.88.205.73`, group `arenaTuoIL`)
  - Phần cứng: Allwinner H616 (ARM64, kernel 6.6), RAM 1.9 GiB, Thẻ nhớ 58 GB F2FS (`/dev/mmcblk0p1`)
  - Số lượng camera: 6 camera Dahua (`camera01` .. `camera06`)
  - FRP Subdomains: Store: **`http://store-pika.video.io.vn`** | C-Store: **`http://c-store-pika.video.io.vn`** (đã đổi từ `lionking-2` sang `pika`)
  - Các điểm nghẽn trước đó:
    - Cấu hình frpc bị gán nhầm subdomain `store-lionking-2` và `c-store-lionking-2`
    - Bảng `Monitors` trong MySQL bị lỗi cú pháp escape JSON tại cột `details` (`camera04`, `camera05`, `camera06`), khiến Shinobi không khởi động được luồng camera (FFmpeg = 0 từ ngày 12/10)
    - Chưa có PM2 Memory Guard, chưa có Smart Watchdog, chưa có Storage Guard
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% video cũ):
    - Sửa toàn bộ cấu hình frpc trong `/root/ota-mqtt/change_ok/frpc_config` và `/tmp/frpc.ini`: `c-store-lionking-2` $\rightarrow$ `c-store-pika`, `store-lionking-2` $\rightarrow$ `store-pika`, proxy `pika_plugin_http_proxy`
    - Kiểm tra xác nhận `store-pika.video.io.vn` và `c-store-pika.video.io.vn` trả về `200 OK`
    - Chạy script sửa chữa triệt để cú pháp JSON cột `details` cho toàn bộ các camera trong MySQL, nạp `input_map_choices` đầy đủ
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained (`lz4`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Cài đặt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/arenaTuoIL`) + Smart Watchdog 60s
    - Cài đặt iptables redirect port 80 -> 2023
    - PM2 Memory Guard: `camera: 170M` (6 camera) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Đủ 11 tiến trình FFmpeg và go2rtc, cả 6 camera đã phục hồi và đang ghi hình video mới liên tục, RAM khả dụng 1.3 GiB

- [x] **Node `inut_207_44`** (MIAPool Arena — `77.88.207.44`, group `mia`)
  - Phần cứng: Rockchip (armv7l, kernel 6.6), RAM 1.9 GiB, Thẻ nhớ 58 GB F2FS (`/dev/mmcblk0p1`), Cổng mạng 100Mb/s Fast Ethernet (`end0`)
  - Số lượng camera: 13 camera Dahua 3K/2K/1080p (`camera01` .. `camera13`)
  - Các điểm nghẽn trước đó:
    - Băng thông mạng LAN bị quá tải nặng: 13 camera đều chạy bitrate 2048 Kbps @ 30fps dồn về cổng 100M, sinh ra lưu lượng ~27 – 35 Mbps và gây rớt hơn 440,000 gói tin (`dropped packets`)
    - Kéo đôi (Double Streaming): 4 camera (`camera07`, `camera08`, `camera12`, `camera13`) bị kéo đồng thời 2 luồng 3K (vừa về đầu ghi NVR `192.168.1.88` vừa về Shinobi) làm quá tải DSP và mạng camera
    - Cột `details` trong MySQL của các camera bị lỗi cú pháp JSON (`detector_filters`)
    - Chưa có PM2 Memory Guard, chưa có ZRAM unconstrained, chưa có Storage Guard 15p, bẫy `kernel.sysrq = 1`
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% video cũ):
    - Điều chỉnh chuẩn toàn bộ 13 camera về **30 FPS & Bitrate 1536 Kbps CBR** có xác nhận đọc lại (Read-Back Verification 100%) $\rightarrow$ Giảm hơn **60% tải mạng LAN** (băng thông nhận hạ từ 35 Mbps xuống chỉ còn **~10 Mbps**)
    - Chuyển hướng 4 camera kéo qua đầu ghi NVR `192.168.1.88`:
      - `camera07` (`192.168.1.228`) $\rightarrow$ NVR Channel 5
      - `camera08` (`192.168.1.62`)  $\rightarrow$ NVR Channel 6
      - `camera12` (`192.168.1.253`) $\rightarrow$ NVR Channel 2
      - `camera13` (`192.168.1.204`) $\rightarrow$ NVR Channel 4
    - Cập nhật cả Shinobi MySQL `Monitors` và `/root/go2rtc.yaml`
    - Sửa toàn bộ lỗi cú pháp JSON trong `details` MySQL, cập nhật độ phân giải chuẩn và `input_map_choices`
    - Kích hoạt ZRAM Swap 1.9 GiB unconstrained, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Cài đặt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/mia`) + Smart Watchdog 60s
    - Cài đặt automount profile `/etc/media-automount.d/` (`noatime,users`, không dùng `/etc/fstab`)
    - Cài đặt iptables redirect port 80 -> 2023
    - PM2 Memory Guard: `camera: 230M` (13 cameras) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Đủ tiến trình FFmpeg ghi hình liên tục cho cả 13 camera, băng thông mạng LAN cực kỳ êm (~10 Mbps, giảm 60%), RAM khả dụng 1.1 GiB, hết hoàn toàn hiện tượng nghẽn rớt gói

- [x] **Node `inut_204_167`** (OnePiece — `77.88.204.167`, group `3ejN83HASs`)
  - Phần cứng: Rockchip / Ophub (ARM64, kernel 6.1), RAM 1.8 GiB, Thẻ nhớ 58 GB F2FS (`/dev/mmcblk1p1`)
  - Số lượng camera: 6 camera Dahua 2K ($2304 \times 1296$, H.265) (`camera01` .. `camera06`)
  - Các lỗ hổng trước khi xử lý:
    - Máy vừa tự sập/reboot (uptime mới 43 phút) do chưa có ZRAM Swap (`Swap = 0B`), chưa có PM2 Memory Guard, bẫy `kernel.sysrq = 1`
    - Thẻ nhớ đã dùng 71% (42 GB / 58 GB) nhưng chưa có Storage Guard 15p và `Users.details.days` đang bị rỗng `""`
    - Cả 6 camera trong MySQL bị để trống `width = NULL, height = NULL` và chưa có cờ `-segment_format_options` chống lỗi cụt `moov` atom
    - Chưa có Smart Watchdog 60s, chưa có iptables redirect port 80 $\rightarrow$ 2023
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% 42 GB video cũ):
    - Kích hoạt ZRAM Swap 1.8 GiB unconstrained (`zstd`), `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`, khóa cứng `kernel.sysrq = 0`
    - Cài đặt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/3ejN83HASs`) + Smart Watchdog 60s
    - Cài đặt automount profile `/etc/media-automount.d/` (`noatime,users`, không dùng `/etc/fstab`)
    - Đặt `"doSnapshot": false` trong `/home/Shinobi/conf.json` & vá `snapProcess.terminate()` trong `libs/monitor.js`
    - Cập nhật `width = 2304, height = 1296` và `cust_record`: `"-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof"` cho toàn bộ 6 camera
    - Dọn 6 file `.mp4` bị cụt `moov` atom do lần sập nguồn trước đó, giữ nguyên 820 file `.mp4` hợp lệ
    - Cài đặt iptables redirect port 80 -> 2023
    - PM2 Memory Guard: `camera: 280M` (6 cameras aarch64) | `inut: 280M` | `ota-mqtt: 120M` với log trỏ `/dev/null`
  - Trạng thái: Đủ 12/12 tiến trình FFmpeg (6 stream + 6 record), file `.mp4` mới ghi chuẩn `iso5` fragmented (`moov` ngay tại byte `0x20`), RAM khả dụng 972 MiB + 1.8 GiB ZRAM

- [x] **Node `inut_204_155`** (Bảo Anh Billiards Club — `77.88.204.155`, group `baoanh2`)
  - Phần cứng: Rockchip / Ophub (ARM64, kernel 6.1.155-ophub), RAM 1.8 GiB, Thẻ nhớ 59 GB F2FS (`/dev/mmcblk1p1`)
  - Số lượng camera: 8 camera Dahua (`camera01` .. `camera08`, `192.168.1.190` .. `192.168.1.197`)
  - Các lỗ hổng trước khi xử lý:
    - Dịch vụ `/lib/systemd/system/media-automount@.service` để `Type=forking` khiến systemd timeout sau 90s và tự động `umount` thẻ nhớ `/media/usb1`, làm cả 8 camera chỉ phát HLS mà không ghi file `.mp4`
    - Cột `details` trong MySQL của cả 8 camera bị lỗi JSON `detector_filters` và thiếu `-segment_format_options`
    - Chưa có Storage Guard, Smart Watchdog, PM2 Memory Guard, chưa cài `inut-bida-agent`
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% 41 GB video cũ):
    - Tạo drop-in `/etc/systemd/system/media-automount@.service.d/override.conf` (`Type=oneshot`, `RemainAfterExit=yes`) và cấu hình `/etc/media-automount.d/` (`noatime,users`), giữ sạch `/etc/fstab`
    - Sửa toàn bộ JSON `Monitors.details`, bật Fragmented MP4 (`-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof`), đặt `"doSnapshot": false`, `"deleteOrphans": false`
    - Cài đặt Storage Guard 15p chuẩn 90% $\rightarrow$ 80% (`/media/usb1/baoanh2`) + Smart Watchdog 60s + PM2 Memory Guard (`camera: 200M`) + `inut-bida-agent.service` (Yamux)
  - Trạng thái: Đủ 16/16 tiến trình FFmpeg, cả 8 camera ghi hình `.mp4` mới liên tục trên `/media/usb1/baoanh2`, Yamux ONLINE (`rtt_ms: 64 ms`)

- [x] **Node `inut_204_48`** (Job Billiards — `77.88.204.48`, group `job`)
  - Phần cứng: Rockchip RK322x (armv7l, kernel 4.4.194-rk322x), RAM 1.9 GiB, Thẻ nhớ 58 GB **ext4** (`/dev/mmcblk0p1`)
  - Số lượng camera: 13 camera Hikvision 1080p H.265 (`camera01` .. `camera13`, `192.168.1.101` .. `192.168.1.113`)
  - Các lỗ hổng trước khi xử lý:
    - Thẻ nhớ `ext4` báo đầy **100% (`0 B` trống)** mặc dù `du -sh /media/usb1/job` chỉ có `41 GB`: nguyên nhân do lỗi rò rỉ File Descriptor trong `/home/Shinobi/libs/videos.js` (`file.close()` không tồn tại trên `ReadStream` của Node.js cũ, khiến tiến trình `camera.js` giữ mở **1,227 file `.mp4 (deleted)`** chiếm cứng **~16 GB** dung lượng đĩa và tạo ra **5,467 file `.mp4` rỗng `0 byte`**)
    - Chỉ còn `2` tiến trình FFmpeg chạy, 12/13 bàn ngừng ghi hình từ 16:47 ngày 21/10
    - `Swap = 0B` (chưa kích hoạt `/dev/zram0`), `media-automount@.service` để `Type=forking`, cả 13 monitor trong MySQL bị lỗi JSON `detector_filters`
  - Cấu hình đã xử lý (Giữ nguyên chuẩn **ext4**, không format thẻ nhớ, bảo toàn 100% 41 GB video hợp lệ):
    - Vá trực tiếp `/home/Shinobi/libs/videos.js` đổi `file.close()` $\rightarrow$ `file.destroy()` để đóng dứt điểm File Descriptor khi stream HTTP kết thúc, khởi động lại `camera.js` giải phóng ngay **16 GB** dung lượng bị ngâm (thẻ nhớ hạ từ **100% xuống 72%**, trống **15 GB**)
    - Dọn sạch 5,467 file `.mp4` `0 byte` do lúc đầy đĩa tạo ra, giữ nguyên 4,312 file `.mp4` (`41 GB`) hợp lệ
    - Giữ nguyên định dạng **ext4** theo đúng đặc thù kernel `4.4.194-rk322x`, cấu hình `/etc/media-automount.d/ext4` (`errors=continue,noatime,users`), remount nóng `/media/usb1` với `errors=continue,noatime`
    - Kích hoạt ZRAM Swap `1.9 GiB` (`lz4`) qua dịch vụ `inut-zram.service`, khóa `kernel.sysrq = 0`, `vm.swappiness = 80`
    - Sửa lỗi JSON `Monitors.details` cho cả 13 camera, bật Fragmented MP4, áp cấu hình H.265 / SmartCodec / 30 FPS / 1536 Kbps cho toàn bộ 13 camera Hikvision
    - Cài đặt Storage Guard 15p (`90% -> 80%`), Smart Watchdog 60s, PM2 Memory Guard (`camera: 250M`), và `inut-bida-agent.service` (Yamux)
  - Trạng thái: Đủ **26/26 tiến trình FFmpeg**, cả 13/13 bàn đang ghi hình `.mp4` mới liên tục trên `/media/usb1/job`, 0 file `(deleted)` bị rò rỉ, Yamux ONLINE (`rtt_ms: 70 ms`)

- [x] **Node `inut_205_50` & `inut_205_54`** (Skyline Pool Club Box 1 & Box 2 — `77.88.205.50` group `sun` & `77.88.205.54` group `ultrapool2`)
  - Phần cứng: Allwinner H616 (aarch64, kernel 6.6.44-current-sunxi64), RAM 1.9 GiB; `205.50` dùng thẻ 120 GB F2FS (`83 GB / 69%`), `205.54` dùng thẻ 58 GB F2FS (`44 GB / 76%`)
  - Số lượng camera & Đầu ghi NVR:
    - `inut_205_50` (Box 1): 9 camera Dahua 2K (`192.168.1.227` .. `192.168.1.238`) + Đầu ghi Dahua NVR `192.168.1.246` (`DHI-NVR1108HS-S3/H-VN`, SN `AD08EEAPCF17F3A`, user `live`/`inut123@`)
    - `inut_205_54` (Box 2): 9 camera Dahua 2K (`192.168.1.203` .. `192.168.1.214`) + Đầu ghi Dahua NVR `192.168.1.200` (`DH-NVR2108HS-4KS2`, SN `8H06BQ9PBC03612`, user `live`/`inut123@`)
  - Các lỗ hổng trước khi xử lý:
    - Mất kết nối OpenVPN `tun1` từ máy chủ `172.16.5.180` (phải đi qua FRPS HTTP Proxy `50206` và `50205`), chưa cài `inut-bida-agent` (Yamux)
    - Đầu ghi Dahua NVR `192.168.1.246` và `192.168.1.200` bị treo kết nối đám mây Dahua P2P (`Easy4ip / T2UServer`); khi đăng nhập DVRIP cổng `37777` vào đầu ghi Dahua NVR trả mã `0x080800` thay vì `0x0008`
    - Cả 2 box đều đang `Swap = 0B` (chưa kích hoạt ZRAM), chưa có PM2 Memory Guard, chưa có Storage Guard 15p, chưa có Smart Watchdog 60s, lỗi JSON `detector_filters` trong MySQL và thiếu `-segment_format_options`
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% 83 GB + 44 GB video cũ):
    - Mở rộng `internal/dahua/dhip.go` chấp nhận mã `errCode[1] == 0x08` (`0x080800`) của đầu ghi Dahua NVR, thêm cả 2 đầu ghi (`192.168.1.246:37777` và `192.168.1.200:37777`) vào `cameras.yaml` và thực thi lệnh `magicBox.reboot` khởi động lại cả 2 đầu ghi NVR thành công $\rightarrow$ Cả 2 NVR đã boot lại và kích hoạt lại `T2UServer` (`www.easy4ipcloud.com`)
    - Kích hoạt ZRAM Swap `1.9 GiB` (`lz4`) qua dịch vụ `inut-zram.service`, khóa `kernel.sysrq = 0`, `vm.swappiness = 80`
    - Vá `videos.js` (`file.destroy()`) và `monitor.js` (`snapProcess.terminate()`), sửa lỗi JSON `Monitors.details` và bật Fragmented MP4 cho toàn bộ 18 camera trên 2 box
    - Cài đặt Storage Guard 15p (`90% -> 80%`), Smart Watchdog 60s, PM2 Memory Guard (`camera: 220M`), và `inut-bida-agent.service` (Yamux)
  - Trạng thái: Cả 2 box đạt **18/18 tiến trình FFmpeg** (`9/9` bàn mỗi box đang ghi hình `.mp4` mới liên tục), cả 2 đầu ghi Dahua NVR đã khởi động lại sạch sẽ, Yamux ONLINE (`rtt_ms: 33 ms` & `35 ms`)

- [x] **Node `inut_204_135`** (HTP Pickleball — `77.88.204.135`, group `b389`)
  - Phần cứng: Allwinner H616 (aarch64, kernel 6.6.44-current-sunxi64), RAM 1.9 GiB, Thẻ nhớ 58 GB F2FS (`/dev/mmcblk0p1`)
  - Số lượng camera: 6 camera Hikvision 2K ($2560 \times 1440$, H.265) (`camera01` .. `camera06`, `192.168.1.120` .. `192.168.1.125`)
  - Các vấn đề trước khi xử lý:
    - Toàn bộ 6 monitor trong Shinobi (`camera01..06`) đang bị tắt tiếng (`acodec: "no"`, `stream_acodec: "no"`), khiến FFmpeg sinh cờ `-an` làm mất hoàn toàn âm thanh khi xem live và tải clip; riêng `camera01` (`192.168.1.120` — `SAN 2B`) còn bị tắt `<Audio><enabled>false</enabled>` ngay trên encoder của camera
    - Cấu hình giới hạn tải/xem lại clip đang để `video_config: range=72` thay vì **2.5 giờ (`range=2.5`)**
    - Chưa có Golden Hardening (`kernel.sysrq = 1`, `vm.swappiness = 0`, ZRAM mới 961 MiB, PM2 chưa giới hạn RAM khiến `camera` restart 236 lần, chưa có Storage Guard 15p, chưa có Smart Watchdog 60s, chưa cài `inut-bida-agent`)
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% 39 GB video):
    - Bật `<Audio><enabled>true</enabled>` + `<audioCompressionType>AAC</audioCompressionType>` trên cả 6 camera Hikvision (`192.168.1.120..125`, đồng thời nâng cấp `isapi.Client.SetAudioAAC` luôn bật `enabled=true`), và mở `acodec: "copy"`, `stream_acodec: "copy"`, `record_acodec: "aac"` trên cả 6 monitor Shinobi $\rightarrow$ Kiểm tra `ffprobe` trên cả 6 file `.mp4` mới đều có đủ 2 luồng `hevc,video` + `aac,audio,16000Hz`
    - Chỉnh thời gian xem & tải clip về đúng **2.5 giờ (`150 phút`)**: cập nhật `/root/ota-mqtt/change_ok/video_config` = `range=2.5` (giữ `db_check_range = 730` cho thẻ nhớ) và khởi động lại `ota-mqtt` + `inut` (Node-RED)
    - Triển khai đầy đủ bộ Golden Hardening (ZRAM `1.9 GiB`, `sysrq = 0`, `swappiness = 80`, `videos.js` `file.destroy()`, `monitor.js` `snapProcess.terminate()`, Fragmented MP4, Storage Guard 15p `90% -> 80%`, Smart Watchdog 60s, PM2 Memory Guard `camera: 220M`, `inut-bida-agent.service` Yamux)
  - Trạng thái: Đủ **12/12 tiến trình FFmpeg**, cả 6/6 sân đang ghi hình `.mp4` mới có âm thanh **AAC 16kHz** rõ nét, giới hạn tải clip **2.5 giờ (`range=2.5`)**, Yamux ONLINE (`rtt_ms: 59 ms`)

- [x] **Node `inut_205_75`** (49 Billiards Carom Club Ninh Hòa — `77.88.205.75`, group `49`)
  - Phần cứng: Allwinner H616 (aarch64, kernel 6.6.44-current-sunxi64), RAM 1.9 GiB, Thẻ nhớ 118 GB **exFAT** (`/dev/mmcblk0p1`)
  - Số lượng camera: 10 camera Dahua 1080p H.265 (`camera01` .. `camera10`, `192.168.1.251` .. `253`, `192.168.1.230` .. `236`)
  - Các lỗ hổng trước khi xử lý:
    - Máy vừa tự khởi động lại lúc 08:16 sáng (uptime 22 phút) do chưa khóa `kernel.sysrq = 0`, PM2 chưa giới hạn RAM (`max_memory_restart: null`), và thẻ `exFAT` bị báo `Volume was not properly unmounted` kèm `errors=remount-ro`
    - Toàn bộ 10 monitor (`camera01..10`) trong MySQL đều bị lỗi cú pháp JSON ở trường `detector_filters` và chỉ có `cust_record: "-tag:v hvc1"` (thiếu `-segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof`), `"deleteOrphans": true`
    - Cả 10 camera Dahua đang tắt `SmartCodec: false`, bitrate 1280 kbps; `shinobi-storage-guard.sh` đang đặt ngưỡng cũ `85% -> 75%` không xóa bản ghi MySQL `Videos`; chưa cài `inut-bida-agent` (Yamux)
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% 69 GB video):
    - Cấu hình `/etc/media-automount.d/{f2fs,ext4,exfat,auto}` với `errors=continue,noatime,users` + drop-in `Type=oneshot` cho `media-automount@.service`, giữ sạch `/etc/fstab`
    - Khóa `kernel.sysrq = 0`, `vm.swappiness = 80`, vá `videos.js` (`file.destroy()`) và `monitor.js` (`snapProcess.terminate()`), đặt `"doSnapshot": false`, `"deleteOrphans": false`
    - Sửa lỗi JSON `Monitors.details` và bật Fragmented MP4 cho toàn bộ 10 camera; đồng thời áp cấu hình **H.265 + SmartCodec + 25 FPS + 1536 Kbps CBR + Audio AAC** cho cả 10 camera Dahua
    - Chuẩn hóa Storage Guard 15p (`90% -> 80%`) có đồng bộ MySQL `Videos`, Smart Watchdog 60s, PM2 Memory Guard (`camera: 230M`), và cài đặt `inut-bida-agent.service` (Yamux)
  - Trạng thái: Đủ **20/20 tiến trình FFmpeg**, cả 10/10 bàn đang ghi hình `.mp4` mới liên tục trên `/media/usb1/49` (`69 GB / 118 GB`, `59%`), Yamux ONLINE (`rtt_ms: 52 ms`)

- [x] **Node `inut_204_2`** (Dragon Billiards Club — `77.88.204.2`, group `Fgbghldvgk`)
  - Phần cứng: Rockchip RK3318 (aarch64, kernel 6.1.63-current-rockchip64), RAM 3.9 GiB, Thẻ nhớ 239 GB **btrfs** (`/dev/mmcblk0p1`)
  - Số lượng camera: 11 kênh kéo qua Đầu ghi NVR `192.168.1.93` (cổng RTSP 554 & VMS 34567, H.264 1080p) (`camera01` .. `camera11` tương ứng Kênh 2..11 và Kênh 1)
  - Các lỗ hổng nghiêm trọng trước khi xử lý:
    - **Box bị sập RAM và tự reboot mỗi 12 giờ** (`last -x` ghi nhận reboot liên tục ngày 22/10, 23/10, 24/10 và 09:47 sáng 25/10): trong `/root/crash_2026_10_24_000006.log`, tiến trình `camera.js` phình lên **25.7% RAM (~1.05 GiB)** làm RAM khả dụng tụt xuống dưới 250 MB và kích hoạt lệnh reboot của Node-RED
    - **Nguyên nhân rò rỉ RAM**: Kênh 1 trên đầu ghi `192.168.1.93` (`camera11`) đang không có tín hiệu camera từ 23/10, kết hợp với `"doSnapshot"` chưa tắt khiến `monitor.js` đẻ ra hàng loạt tiến trình `ffmpeg -s 500x500 ... /dev/shm/streams/Fgbghldvgk/camera11/*.jpg` và Worker `snapshot.js` bị treo liên tục
    - `Swap = 0B` (chưa kích hoạt ZRAM), `kernel.sysrq = 176`, `vm.swappiness = 0`, thẻ nhớ 239 GB đã đầy **85% (`201 GB`)** nhưng **chưa có Storage Guard 15p**
    - Toàn bộ 11 monitor trong MySQL đều bị lỗi cú pháp JSON ở trường `detector_filters` và `cust_record: ""` (trống), khiến mọi file `.mp4` đang ghi dở đều bị lỗi `moov atom not found` mỗi khi box tự reboot
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% 201 GB video):
    - Đặt `"doSnapshot": false` trong `/home/Shinobi/conf.json`, `snap: "0"` trong `Monitors.details`, và vá `snapProcess.terminate()` trong `/home/Shinobi/libs/monitor.js` + `file.destroy()` trong `/home/Shinobi/libs/videos.js` $\rightarrow$ Chặn đứng hoàn toàn rò rỉ Worker từ kênh `camera11` offline, RAM của `camera.js` giữ ổn định ở ~140 MB (RAM trống khả dụng vọt lên **2.8 GiB / 3.9 GiB**)
    - Kích hoạt **ZRAM Swap `2.4 GiB`** qua dịch vụ `inut-zram.service`, khóa `kernel.sysrq = 0`, `vm.swappiness = 80`
    - Sửa sạch lỗi JSON `Monitors.details` và bật Fragmented MP4 (`-segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof`, chuẩn H.264) cho cả 11 monitor $\rightarrow$ Xác nhận bằng `ffprobe` các file `.mp4` mới vừa ghi được vài giây đã đọc được `moov` atom (`h264,1920,1080`) ngay lập tức
    - Cài đặt **Storage Guard 15p (`90% -> 80%`)** bảo vệ thẻ 239 GB (đang ở 84%), **Smart Watchdog 60s**, **PM2 Memory Guard (`camera: 280M`)**, dịch vụ `kspcam.service` và `inut-bida-agent.service` (Yamux)
  - Trạng thái: Đủ **22 tiến trình FFmpeg** (`10/10` kênh có camera trên NVR đang ghi hình `.mp4` chuẩn Fragmented MP4 liên tục), RAM khả dụng **2.8 GiB + 2.4 GiB ZRAM**, chấm dứt hoàn toàn vòng lặp tự reboot mỗi 12 giờ, Yamux ONLINE (`rtt_ms: 70 ms`)

- [x] **Node `inut_205_52`, `inut_205_59`, `inut_205_110`, `inut_204_133`**
  - **`inut_205_52` (Gen Z Billiards 375 Vũ Tông Phan — `77.88.205.52`, group `hn7`, 12 camera Dahua + NVR `192.168.1.99`)**:
    - Lỗi trước đó: Tiến trình `camera` trong PM2 bị sập sang trạng thái `errored` (`FFMPEG_COUNT=0`), `Swap = 0B`, chưa có Watchdog, cả 12 monitor bị lỗi JSON `detector_filters`.
    - Đã xử lý: Bật ZRAM 1.9 GiB, sửa JSON 12 monitor + bật Fragmented MP4, cài đặt Smart Watchdog 60s + Storage Guard 15p + PM2 Guard `260M` + `inut-bida-agent`. Đã hồi sinh đủ **24/24 tiến trình FFmpeg** (`12/12` bàn ghi `.mp4` mới liên tục trên `/media/usb1/hn7`, thẻ 60 GB ở mức `68%`).
  - **`inut_205_59` (Tuấn King Billiards Club — `77.88.205.59`, group `tuancreep`, 10 camera Dahua)**:
    - Lỗi trước đó: Tiến trình `camera` trong PM2 bị `errored` (`FFMPEG_COUNT=0`), ZRAM mới 961 MiB, chưa có Watchdog, 10 monitor lỗi JSON `detector_filters`.
    - Đã xử lý: Nâng ZRAM 1.9 GiB, sửa JSON 10 monitor + bật Fragmented MP4, cài đặt Smart Watchdog 60s + Storage Guard 15p + PM2 Guard `230M` + `inut-bida-agent`. Đã hồi sinh đủ **20/20 tiến trình FFmpeg** (`10/10` bàn ghi `.mp4` mới liên tục trên `/media/usb1/tuancreep`).
  - **`inut_205_110` (99 Billiards - CS9 - 2 — `77.88.205.110`, group `9992`, 15 camera Hikvision)**:
    - Lỗi trước đó: Tiến trình `camera` trong PM2 bị `errored` (`FFMPEG_COUNT=0`), `Swap = 0B`, cấu hình `frpc_config` bị gõ nhầm `subdomain = c-store-99-cs96-2` (thay vì `c-store-99-cs9-2`), 15 monitor lỗi JSON `detector_filters`.
    - Đã xử lý: Sửa lại subdomain FRPC `c-store-99-cs9-2`, bật ZRAM 1.9 GiB, sửa JSON 15 monitor + bật Fragmented MP4, cài đặt Smart Watchdog 60s + Storage Guard 15p + PM2 Guard `260M` + `inut-bida-agent`. Đã hồi sinh đủ **30/30 tiến trình FFmpeg** (`15/15` bàn ghi `.mp4` mới liên tục trên `/media/usb1/9992`, thẻ 118 GB ở mức `70%`).
  - **`inut_204_133` (Zone+ Thái Nguyên 1 — `77.88.204.133`, group `zoneplus1`, 10 camera)**:
    - Chẩn đoán phần cứng: Thẻ nhớ MicroSD trên khe `mmc0` (`4020000.mmc`) đã chết chip điều khiển flash vật lý (`F2FS-fs: Issue discard failed, ret: -5`, `f2fs_commit_super fails to record err:-5`, mất hoàn toàn block device `/dev/mmcblk0` kể cả sau khi rebind `sunxi-mmc` và reboot).
    - Đã xử lý: Áp dụng đầy đủ bộ Golden Hardening (ZRAM 1.9 GiB, `nodiscard` trong `/etc/media-automount.d/f2fs`, sửa JSON 10 monitor, Smart Watchdog 60s, PM2 Guard `230M`, `inut-bida-agent`). Hiện tại **20/20 tiến trình FFmpeg** đang chạy phát Live HLS bình thường cho cả 10 bàn (`store-zoneplus1.video.io.vn`); cần thay thẻ nhớ vật lý mới tại quán để ghi hình `.mp4` trở lại.

- [x] **Node `inut_200_81`** (Billiards B51 — `77.88.200.81`, group `fecredit`)
  - Phần cứng: Allwinner Sunxi (armv7l, kernel 5.15.93-sunxi), RAM **999 MiB (1 GB)**, Thẻ nhớ 60 GB F2FS (`/dev/mmcblk0p1`)
  - Số lượng camera: 5 camera Dahua `DH-C5A` (`camera01` .. `camera05`)
  - Các lỗ hổng trước khi xử lý:
    - `camera04` (`192.168.1.165`) bị cấu hình `acodec: "aac"` thay vì `"copy"`, khiến tiến trình `ffmpeg` phải transcode âm thanh bằng CPU (`-c:a aac`) ngốn **19% CPU** liên tục mặc dù bản thân camera đã phát sẵn âm thanh chuẩn `AAC`
    - Box chỉ có 1 GB RAM nhưng ZRAM mới để `499 MiB`, chưa có PM2 Memory Guard, chưa có Storage Guard 15p (thẻ đã dùng 77%), chưa có Smart Watchdog 60s, cả 5 monitor bị lỗi JSON `detector_filters` và thiếu `-segment_format_options`
    - `camera01` (`192.168.1.240`) mất kết nối LAN khi kiểm tra (chưa xác định nguyên nhân nguồn hay mạng); `camera05` (IP cũ `192.168.1.9`, Wi-Fi `wlan0`) trả `Connection reset by peer` trên RTSP `554`, dù DVRIP `37777` vẫn chụp ảnh được. Chưa có bằng chứng đầu ghi khác chiếm slot hay firmware cố ý khóa RTSP.
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% 42 GB video):
    - Sửa toàn bộ 5 monitor (`camera01..05`) về `acodec: "copy"`, `stream_acodec: "copy"`, `vcodec: "copy"` (Zero-transcoding) và bật Fragmented MP4 $\rightarrow$ Giảm ngay tải CPU của `camera04` từ 19% xuống ~3%
    - Áp cấu hình chuẩn **H.265 + SmartCodec + 1536 Kbps CBR + GOP 50 + Audio AAC** cho các camera online (`192.168.1.165`, `167`, `169`, `9`)
    - Nâng ZRAM Swap từ `499 MiB` lên **`960 MiB` (`lz4`)**, khóa `kernel.sysrq = 0`, `vm.swappiness = 80`, cấu hình `/etc/media-automount.d/` (`noatime,users` cho kernel 5.15)
    - Cài đặt Storage Guard 15p (`90% -> 80%`), Smart Watchdog 60s, PM2 Memory Guard (`camera: 180M`, `inut: 200M`, `ota-mqtt: 100M`), và `inut-bida-agent.service` (Yamux)
  - Cập nhật 25/10/2026, camera05 (SN `AD0786FPAG01146`):
    - Giữ IP tĩnh `wlan0 = 192.168.1.168/24`, gateway `192.168.1.1`; inventory, Shinobi `camera05` và go2rtc đã chuyển sang IP này.
    - Người dùng đã xác nhận nguy cơ brick/mất cấu hình do gói có ghi bootloader. Gói nguyên bản từ Drive: 12,718,567 byte; SHA-256 `2566ff2bd5ab9d02beca583b506c3fba32a16eca31372a1f694d5fb132e79da6`; HWID `IPC-C5AP` khớp chính xác. ZIP CRC kiểm tra đạt; chữ ký số chưa được xác minh độc lập.
    - Kiểm chứng truyền đủ gói bằng `upgrader.appendData` qua DVRIP rồi `upgrader.cancel` thành công trước lần nạp thật. Lần nạp thật `upgrader.execute` được chấp nhận, báo tiến độ kernel/rootfs/bootloader đến 100%. Đọc lại sau boot xác nhận **`2.860.0000000.3.R`, BuildDate `2024-04-02`**, thay cho `.26.R` ngày `2026-05-30`; IP Wi-Fi và thông tin đăng nhập còn nguyên. Chỉ camera05 được nạp firmware.
    - Backup trên box: `/root/camera05-monitor-before-firmware-20261025.sql`, `/root/c5a-before-downgrade-20261025-152802.json` (mode 0600).
    - Đã khôi phục RTSP `Enable=true, Port=554`, bật lại monitor `camera05` chế độ `record`. **RTSP vẫn trả `Invalid data found when processing input`; camera05 chưa tạo MP4 mới.** DVRIP snapshot sau downgrade trả JPEG 24,400 byte. Hạ firmware thành công nhưng chưa khắc phục RTSP; không kết luận nguyên nhân là phiên bản firmware.
  - Cập nhật 25/10/2026 (Test tính năng Nạp / Hạ cấp Firmware qua Web UI mới trên `inut_200_81`):
    - Triển khai bản build `kspcam-linux-armv7` mới lên box 81 (`:2028`), mở trực tiếp giao diện Web UI, upload gói `firmware.pkg` (`2.860.0000000.3.R` — `2024-04-02`, `12,420 KB`, 9 phân vùng) và bấm **Kiểm tra tương thích (Preflight)**:
      - Hệ thống nhận diện đúng **4/5 camera khớp chính xác mã phần cứng `IPC-C5AP:01:02:00:100:68:00:01:10:01:01:04:4B0:00:02:00:00:00:01:00:00:40`** (`Camera02`, `Camera03`, `Camera04`, `Camera05`), riêng `Camera01` (`192.168.1.240`) đang tắt nguồn tại quán nên báo `unreachable`.
    - Chạy **Nạp Firmware Chính Thức** qua Web UI tuần tự cho 3 camera (`Camera04`, `Camera02`, `Camera03`):
      - Tự động sao lưu đầy đủ cấu hình trước khi nạp vào `/opt/ksp-cam/backups/fw_backup_192.168.1.{165,167,169}_37777_*.json`.
      - **`Camera02` (`192.168.1.167`, SN `AD0786FPAG01044`)**: Hạ cấp thành công từ `2.860.0000000.24.R` $\rightarrow$ **`2.860.0000000.3.R`**, tự khởi động lại sau 85 giây, kết nối lại Wi-Fi `192.168.1.167`, luồng RTSP `554` (`hevc 2880x1620` + `aac`) hoạt động mượt mà và đang ghi hình `.mp4` mới liên tục (`/media/usb1/fecredit/camera02/2026-10-25T17-05-00.mp4`).
      - **`Camera04` (`192.168.1.165`, SN `AD0786FPAG01104`, từ `2.860.100Z00C.0.R`)** và **`Camera03` (`192.168.1.169`, SN `AD0786FPAG00816`, từ `2.860.0000000.24.R`)**: Đã truyền 100% gói và ghi Flash (`kernel.img` $\rightarrow$ `romfs-x.squashfs.img` $\rightarrow$ `dhboot.bin.img` $\rightarrow$ `100%`) rồi reboot, nhưng sau khi reboot chưa tự bắt lại Wi-Fi `Bida Viet Huy` (do gói có cờ `//IPC_RestoreDefault` hoặc treo kết nối WPA sau khi hạ từ nhánh `100Z00C` / `.24.R`); cần rút nguồn cắm lại hoặc cắm dây LAN tại quán để nhận lại mạng.

- [x] **Node `inut_204_251`** (TG Golden Billiards Club — `77.88.204.251`, group `s912`)
  - Phần cứng: Amlogic S912 (aarch64, kernel 5.15.190-ophub), RAM **1.8 GiB**, Thẻ nhớ 118 GB F2FS (`/dev/mmcblk1p1`)
  - Số lượng camera: 10 camera Dahua (`camera01` .. `camera10` trên dải `192.168.2.0/24`)
  - Các lỗ hổng trước khi xử lý:
    - `camera07` bị trỏ nhầm vào IP cũ `192.168.2.88` (offline, 0 file ghi hình từ tháng 8), trong khi camera thật của bàn **Libre 6 - Libre 12** (`d4:43:0e:a6:10:6b`, SN `AD0A156PCFA3EAF`) đang chạy ở **`192.168.2.247`** (và đã được khai báo trong `config_no_use` của quán)
    - `camera10` (`192.168.2.239` — TV Pool, SN `9L090BDRAG3E4F5`) dùng firmware chỉ hỗ trợ âm thanh `G.711A` (không hỗ trợ `AAC`) nhưng monitor lại cấu hình `acodec: "aac"` kèm `-fflags +genpts -vsync 2 -rtbufsize 64M`, khiến `ffmpeg` phải transcode âm thanh bằng CPU ngốn **14.5% CPU** liên tục; đồng thời video đang để `2048 Kbps VBR` và tắt `SmartCodec`
    - `/opt/ksp-cam/cameras.yaml` lưu sai mật khẩu mặc định (`smarthome12345` thay vì mật khẩu thực tế `inut1234`) và thiếu cấu hình `shinobi` trong `/opt/ksp-cam/config.yaml`
    - `camera_count` bị để `8` trong khi `toolbar_show_count = 10`; `kernel.sysrq = 176`, `vm.swappiness = 0`, ZRAM mới 900 MB (`lzo-rle`), chưa có PM2 Memory Guard (`camera` đã restart 13 lần), chưa có Storage Guard 15p và chưa có Smart Watchdog 60s
  - Cấu hình đã xử lý (Không format thẻ nhớ, bảo toàn 100% 83 GB video):
    - Cập nhật `camera07` trong Shinobi `Monitors`, `/root/go2rtc.yaml` và `/opt/ksp-cam/cameras.yaml` từ `192.168.2.88` sang **`192.168.2.247`** $\rightarrow$ Hồi sinh ngay bàn `camera07` (Libre 6 - Libre 12) ghi hình `.mp4` mới liên tục!
    - Chuyển toàn bộ 9/9 camera online về chuẩn **H.265 + SmartCodec + 1536 Kbps CBR + GOP 50 + Audio AAC**; riêng `camera10` (`192.168.2.239`, chỉ hỗ trợ G.711A) đã bật `H.265 + SmartCodec + 1536 Kbps CBR + GOP 50` và đặt `acodec: "no"`, `stream_acodec: "no"` trên Shinobi $\rightarrow$ Hạ tải CPU của `camera10` từ **14.5% xuống ~5.1%**
    - Sửa JSON cả 10 monitor (`detector_filters: "{}"`, `snap: "0"`, `input_map_choices`, Fragmented MP4 `-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof`), đặt `doSnapshot: false` và vá `snapProcess.terminate()` trong `libs/monitor.js`
    - Đồng bộ `camera_count = 10`, `toolbar_show_count = 10`, cập nhật hạn mức `Users.details.size = "100000"` (100 GB / 3 ngày)
    - Nâng ZRAM Swap từ `900 MiB` lên **`1.9 GiB` (`lz4`)**, khóa `kernel.sysrq = 0`, `vm.swappiness = 80`, cấu hình `/etc/media-automount.d/` (`noatime,users,nodiscard`), cài đặt Storage Guard 15p (`90% -> 80%`), Smart Watchdog 60s, PM2 Memory Guard (`camera: 230M`, `inut: 280M`, `ota-mqtt: 120M`), và `inut-bida-agent.service`
  - Trạng thái: **18 tiến trình (`9/10` bàn: `camera01`, `02`, `04`, `05`, `06`, `07`, `08`, `09`, `10`)** đang chạy mượt và ghi hình `.mp4` mới liên tục vào `/media/usb1/s912/` (`71%`, còn trống 35 GB); riêng `camera03` (`192.168.2.243`) đang tắt nguồn/mất kết nối tại quán
