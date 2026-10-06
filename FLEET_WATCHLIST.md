# FLEET WATCHLIST & HARDENING INVENTORY
> **Tài liệu theo dõi trạng thái tối ưu hóa và danh mục thiết bị Edge Fleet (ksp-camera-auto)**  
> Cập nhật tự động: Mỗi node khi hoàn tất tối ưu hóa đều được ghi nhận chi tiết về thông số phần cứng, số lượng camera, cấu hình PM2 Memory Guard, ZRAM, Watchdog, Storage Guard và trạng thái vận hành thực tế.

---

## 1. Bảng Tổng Hợp Watchlist Các Node Đã Tối Ưu

| Node ID | Địa Chỉ IP VPN | Tên Quán / Sân Thể Thao | Shinobi Group | Số Cam ($N$) | Kiến Trúc CPU | ZRAM Swap | PM2 Memory Guard (cam / inut / mqtt) | Smart Watchdog (60s) | Storage Guard (15p) | Sysrq Gỡ Sạch | Kspcam (:2028) | Trạng Thái Vận Hành |
|---|---|---|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|---|
| **`inut_204_157`** | `77.88.204.157` | Kely Pickleball | `kely` | 12 cam | ARM64 | **3.5 GiB** (lz4, uncap) | **220M / 280M / 120M** (`/dev/null`) | ✅ Active | ✅ Active (`kely`) | ✅ Gỡ sạch | ✅ Active | **Uptime >3 ngày**, RAM trống 713M–950M, 24 FFmpeg ghi 12 sân liên tục, 0 OOM |
| **`inut_205_72`** | `77.88.205.72` | Master Billiards Club | `master2` | 10 cam | ARM64 | **1.9 GiB** (lz4, uncap) | **220M / 280M / 120M** (`/dev/null`) | ✅ Active | ✅ Active (`master2`) | ✅ Gỡ sạch | ✅ Active | **Uptime >19 ngày**, RAM trống 962M–1.1G, 20 FFmpeg ghi 10 bàn 3K liên tục |
| **`inut_205_87`** | `77.88.205.87` | Heypick Pickleball | `heyPick` | 7 cam | ARM64 | **1.9 GiB** (lz4, uncap) | **180M / 280M / 120M** (`/dev/null`) | ✅ Active | ✅ Active (`heyPick`) | ✅ Gỡ sạch | ✅ Active | Triệt tiêu chu kỳ reboot 1–2 ngày/lần, RAM trống >900M, 14 FFmpeg ghi 7 sân liên tục |
| **`inut_204_159`** | `77.88.204.159` | Axis Billiards | `axis` | 9 cam | ARM64 (S912 8-core) | **1.8 GiB** (lz4, uncap) | **200M / 280M / 120M** (`/dev/null`) | ✅ Active | ✅ Active (`axis`) | ✅ Gỡ sạch | ✅ Active | Triệt tiêu lỗi CPU=8 sysrq bus lockup gây treo cứng phải rút nguồn, 18 FFmpeg ghi 9 bàn 3K liên tục |
| **`inut_207_12`** | `77.88.207.12` | Poseidon Billiards | `hk` | 17 cam | ARMv7l (rk322x) | ZRAM active | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | Độc lập | Fix camera16 T31N PTS crash-loop (chuyển H.264 Main 2048k tcp), đủ 34 FFmpeg ghi 17 bàn liên tục |
| **`inut_205_56`** | `77.88.205.56` | King Kong Thái Nguyên | `mp4` | 9 cam | ARM64 | **1.9 GiB** (lz4, uncap) | PM2 fork mode | ✅ Active | ✅ Active (`mp4`) | ✅ Gỡ sạch | ✅ Active | Chuẩn hóa 9 cam về 2880x1620 @ 25fps 1536k VBR GOP 50 H.265 AAC, giảm 215M RAM FFmpeg, 18 FFmpeg ghi đều |
| **`inut_205_53`** | `77.88.205.53` | The Best Billiards Club | `ultrapool1` | 12 cam | ARM64 | **1.9 GiB** (lz4, uncap) | PM2 fork mode | ✅ Active | ✅ Active (`ultrapool1`) | ✅ Gỡ sạch | ✅ Active | Triệt tiêu CPU audio transcoding pcm_mulaw sang AAC (`acodec=no`), wallclock=0, RAM trống tăng từ 41M lên >900M |
| **`inut_205_62`** | `77.88.205.62` | Billiards 2000 Club | `2000` | 8 cam | ARM64 | **1.9 GiB** (lz4, uncap) | PM2 fork mode | ✅ Active | ✅ Active (`2000`) | ✅ Gỡ sạch | ✅ Active | Hồi sinh 8 camera zombie (từ 0 lên 18 FFmpeg), cứu nguy đĩa 80%, ghi hình ổn định |
| **`inut_205_6`** | `77.88.205.6` | TheLight Pool Arena | `thelightpoolarena` | 10 cam | ARM64 | **3.8 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active (`thelightpoolarena`) | ✅ Gỡ sạch | ✅ Active | 20 FFmpeg ghi 10 bàn liên tục, RAM trống 970M, Swap 3.8G |
| **`inut_205_10`** | `77.88.205.10` | CLB Billiard The King | `xinchao` | 8 cam | ARM64 | **1.9 GiB** (lz4, uncap) | PM2 fork mode | ✅ Active | ✅ Active (`xinchao`) | ✅ Gỡ sạch | ✅ Active | Giải tỏa nguy cơ tràn đĩa 84% (còn 9GB), 16 FFmpeg ghi 8 bàn liên tục |
| **`inut_205_67`** | `77.88.205.67` | BinK Billiards Club | `billiardclub` | 11 cam | ARM64 | **1.9 GiB** (lz4, uncap) | PM2 fork mode | ✅ Active | ✅ Active (`billiardclub`) | ✅ Gỡ sạch | ✅ Active | Cấu hình camera11 mới (1920x1080 2048k H.264 AAC), giữ nguyên 10 cam 4MP H.265, đủ 22 FFmpeg |
| **`inut_205_69`** | `77.88.205.69` | PHE Vietnam | `phe` | 6 cam | ARM64 | **1.9 GiB** (lz4, uncap) | PM2 fork mode | ✅ Active | ✅ Active (`phe`) | ✅ Gỡ sạch | ✅ Active | Triệt tiêu lỗi rớt online/offline liên tục (xóa flash swapfile nghẽn I/O, gỡ 5 node sysrq), ping phẳng 31ms, 12 FFmpeg ghi đều |
| **`inut_205_85`** | `77.88.205.85` | Lion King Nam Định CS2 | `lionkingnd` | 6 cam | ARM64 | **1.9 GiB** (lz4, uncap) | PM2 fork mode | ✅ Active | ✅ Active (`lionkingnd`) | ✅ Gỡ sạch | ✅ Active | Hồi sinh 6 camera chết 8 tiếng (0 FFmpeg sau reboot), Smart Watchdog tự restart nếu 0 FFmpeg, 12 FFmpeg ghi đều |
| **`inut_204_148`** | `77.88.204.148` | Ruby Billiard Club | `na` | 7 cam | ARM64 | **1.9 GiB** (lz4, uncap) | PM2 fork mode | ✅ Active | ✅ Active (`na`) | ✅ Gỡ sạch | ✅ Active | Đổi IP camera07 sang `192.168.1.65` thành công, gỡ 6 node sysrq, 12 FFmpeg ghi ổn định |
| **`inut_205_77`** | `77.88.205.77` | ZONE+ Billiards | `toktok3` | 12 cam | ARM64 | **1.9 GiB** (lz4, uncap) | PM2 fork mode | ✅ Active | ✅ Active (`toktok3`) | ✅ Gỡ sạch | ✅ Active | Gỡ 4 node sysrq, ZRAM 1.9G, Smart Watchdog, Storage guard, 24 FFmpeg ghi 12 bàn liên tục |
| **`inut_205_122`** | `77.88.205.122` | Magic Arena Đà Nẵng | `magic3` | 16 cam | ARM64 | **2.8 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Chuẩn hóa 64 kênh NVR về 2560x1440 2048kbps VBR SmartCodec true, 32 FFmpeg ghi mượt |
| **`inut_205_123`** | `77.88.205.123` | Magic Arena Đà Nẵng | `magic2` | 12 cam | ARM64 | **2.8 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Chuẩn hóa NVR 2560x1440 2048kbps VBR SmartCodec true, 24 FFmpeg ghi mượt |
| **`inut_205_124`** | `77.88.205.124` | Magic Arena Đà Nẵng | `magic1` | 16 cam | ARM64 | **2.8 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Chuẩn hóa NVR 2560x1440 2048kbps VBR SmartCodec true, 32 FFmpeg ghi mượt |
| **`inut_205_31`** | `77.88.205.31` | Khét Bida / Khét Coffee | `khet` | Multi | ARM64 | **1.9 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Tắt Wi-Fi ẩn camera, giải quyết xung đột ARP, chuẩn hóa GOP 50, faststart + HTTP 206 range |
| **`inut_205_37`** | `77.88.205.37` | GenZ Billiards | `genz` | Multi | ARM64 | **1.9 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Dọn sạch 45GB file mồ côi & file hỏng 0-byte, cài đặt Storage Guard cron, retention 3 ngày |
| **`inut_205_75`** | `77.88.205.75` | 49 Ninh Hòa | `ninhhoa` | 10 cam | ARM64 | **1.9 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Khắc phục OOM do queue leak `wall_clock_timestamp_ignore: 1`, bulk apply 2MP 25fps 768k CBR AAC |
| **`inut_204_146`** | `77.88.204.146` | Sniper Billiards | `sniper` | 14 cam | ARM64 | **3.8 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Chuẩn hóa kênh NVR 5-18 về 1080p 25fps 1280k CBR AAC GOP 50, gỡ sysrq, ZRAM 3.8G |
| **`inut_205_73`** | `77.88.205.73` | Pika Sun Pickleball | `pikasun` | Multi | ARM64 | **1.9 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Anti-A Guardian 2MP 768kbps SmartCodec false AAC, ZRAM 1.9G, bảo vệ luồng |
| **`inut_200_76`** | `77.88.200.76` | SlyBilliards Club | `sly` | Multi | ARMv7l (32-bit) | **1.5 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Triển khai kspcam ARMv7 32-bit tĩnh, Anti-A Guardian 2MP 768k AAC, ZRAM 1.5G |
| **`inut_203_250`** | `77.88.203.250` | KKING Vân Đình | `kking` | Multi | ARMv7l (32-bit) | **1.5 GiB** (lz4) | PM2 fork mode | ✅ Active | ✅ Active | ✅ Gỡ sạch | ✅ Active | Triển khai kspcam ARMv7, tắt CPU AAC transcoding (`acodec=no`), `wall_clock=0`, ZRAM 1.5G |
| **`inut_207_139`** | `77.88.207.139` | Hikvision Edge Gateway | - | 2 cam | ARM64 | ZRAM active | PM2 fork mode | - | - | - | ✅ Active | Quét phát hiện 2 camera Hikvision (`192.168.0.101` & `.250`), tạo đường hầm FRPC TCP qua MQTT tới `inut.io.vn:5006/5007` |

---

## 2. Tiêu Chuẩn Vàng Khi Tối Ưu Hóa (Golden Hardening Checklist)

Mọi node trong danh sách trên đều tuân thủ nghiêm ngặt 7 nguyên tắc cốt lõi:
1. **ZRAM Swap 100% Unconstrained**: `ZRAM_PERCENTAGE=100`, `MEM_LIMIT_PERCENTAGE=100`, `echo 0 > /sys/block/zram0/mem_limit`, `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100` $\rightarrow$ Triệt tiêu lỗi `-ENOMEM` và OOM killer.
2. **PM2 Memory Guard**: Giới hạn heap chủ động cho Node.js (`camera: (120 + N*8)M`, `inut: 280M`, `ota-mqtt: 120M`), tự tái sinh êm sau 1–2s mà không làm gián đoạn hệ thống.
3. **Bảo toàn Log `/dev/null`**: Cả `pm_out_log_path` và `pm_err_log_path` đều trỏ vào `/dev/null` để chống mòn thẻ nhớ và chống đầy dung lượng `/var/log` / tmpfs.
4. **Triệt tiêu ngòi nổ tự sát Node-RED**: Loại bỏ hoàn toàn các lệnh `echo b > /proc/sysrq-trigger`, `rm -rf /tmp`, và nhánh `[ "$CPU" -eq 8 ]`. Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
5. **Smart Watchdog 60s**: Script `/usr/local/bin/shinobi-camera-watchdog.sh` kiểm tra số lượng tiến trình FFmpeg thực tế (`pgrep -c ffmpeg`). Nếu camera online $> 90\text{s}$ mà có 0 FFmpeg thì tự phục hồi ngay.
6. **Storage Guard 15p**: Script `/usr/local/bin/shinobi-storage-guard.sh` targeting thư mục video của quán, dọn rác 0-byte và xoay vòng xả đĩa khi $\ge 85\%$ về mức $\le 75\%$.
7. **Shinobi Zero-Transcode & Timestamp Fix**: `vcodec=copy`, `acodec=copy` (hoặc `no`), `wall_clock_timestamp_ignore=0` (hoặc passthrough), `hls_time=2`, `max_keep_days=3`, kích hoạt `deleteOld`/`deleteOrphans` trong `conf.json`.
