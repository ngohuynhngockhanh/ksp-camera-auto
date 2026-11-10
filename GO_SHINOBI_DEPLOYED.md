# GO_SHINOBI_DEPLOYED.md — Danh Sách Node Đã Triển Khai iNut NVR (Go-Shinobi-KSP)

> **Mục đích:** Theo dõi tập trung toàn bộ các Edge Box đã chuyển đổi (Onboarding) từ Shinobi cũ (Node.js `camera.js` + MariaDB/MySQL `ccio`) sang **iNut NVR (`Go-Shinobi-KSP`)** thuần Go (`CGO_ENABLED=0`, SQLite WAL, Zero-Transcoding, bảo vệ mount `/media/usb1`).
> **Repo mã nguồn:** `/home/ksp/go-shinobi-ksp` (`git@github.com:inut-team/go-shinobi-ksp.git`)

---

## 1. Danh Sách Node Đã Triển Khai (Deployed Nodes Ledger)

### 1.1 `inut_206_14` — `77.88.206.14` (King Billiards Club)
- **Tên quán / Điểm lắp đặt:** King Billiards Club
- **Domain C-Store / Store:** `http://c-store-kingbn.video.io.vn/` • `http://store-kingbn.video.io.vn/`
- **Kiến trúc phần cứng:** `armv7l` (`Linux 5.15.93-sunxi`, 1 GB RAM, thẻ nhớ F2FS `/media/usb1`)
- **Binary triển khai:** `/opt/go-shinobi-ksp/go-shinobi-ksp` (`dist/go-shinobi-ksp-linux-armv7`, PM2 process `camera`)
- **Database SQLite WAL:** `/opt/go-shinobi-ksp/shinobi.sqlite` (MariaDB/MySQL đã dừng & disable, giải phóng ~120 MB RAM)
- **Group Key (`ke` / `shinobi_camera_id`):** `kingbn`
- **Số lượng Camera Monitors:** `1` (`camera01` — `192.168.1.64:554/Streaming/Channels/101`, chế độ `record`)
- **Bảo toàn API Keys & Video Cũ:**
  - Giữ nguyên 100% `shinobi_token`, `shinobi_monitor_token` trong `/root/ota-mqtt/change_ok/` và toàn bộ API keys trong bảng `API`.
  - Bảo toàn 100% video `.mp4` trên `/media/usb1/kingbn/camera01/` (hiển thị chuẩn múi giờ Box `GMT+7` và đầy đủ `actionUrl` trên cổng `:2023`).
- **Trạng thái kiểm chứng:** `ONLINE` • RAM `go-shinobi-ksp` ~13 MB • Live HLS + MP4 Segment (`moov_size=262144:movflags=+faststart`) + Watchdog & Storage Guard tương thích kép.

---

### 1.2 `inut_206_10` — `77.88.206.10` (1992 Đại Dương Club)
- **Tên quán / Điểm lắp đặt:** 1992 Đại Dương Club (`ui_title=1992 Đại Dương Club`)
- **Kiến trúc phần cứng:** `armv7l` (`Linux 5.15.93-sunxi`, 1 GB RAM, thẻ nhớ F2FS `/media/usb1` 59 GB)
- **Binary triển khai:** `/opt/go-shinobi-ksp/go-shinobi-ksp` (`dist/go-shinobi-ksp-linux-armv7`, PM2 process `camera`)
- **Database SQLite WAL:** `/opt/go-shinobi-ksp/shinobi.sqlite` (Bản sao lưu gốc tại `/opt/go-shinobi-ksp/backup-legacy/ccio_backup.sql`, MariaDB/MySQL đã dừng & disable)
- **Group Key (`ke` / `shinobi_camera_id`):** `kingbn2`
- **Số lượng Camera Monitors:** `1` (`camera01` — `192.168.1.182:554/cam/realmonitor?channel=1&subtype=0`, chế độ `record`)
- **Bảo toàn API Keys & Video Cũ:**
  - Giữ nguyên 100% 5 API keys từ MySQL `ccio.API` (bao gồm `shinobi_token=<REDACTED_SHINOBI_TOKEN>` và `shinobi_monitor_token=<REDACTED_MONITOR_TOKEN>`).
  - Chốt clip đang ghi (`CheckVAR date -s`) trước khi chuyển đổi và đồng bộ nguyên vẹn toàn bộ **546+ file video `.mp4`** trên `/media/usb1/kingbn2/camera01/` (0 clip bị mất).
- **Trạng thái kiểm chứng:** `ONLINE` • Đã qua kiểm thử `make go-shinobi inut_206_10` và `make backup-shinobi inut_206_10`.

---

### 1.3 `inut_205_98` — `77.88.205.98` (Empire Gaming & Billiards – Bắc Ninh)
- **Tên quán / Điểm lắp đặt:** Empire Gaming & Billiards – Bắc Ninh (`ui_title=Empire Gaming & Billiards – Bắc Ninh`)
- **Domain C-Store / Store:** `http://c-store-empire.video.io.vn/` • `http://store-empire.video.io.vn/` • `http://ksp-cam-inut-205-98.video.io.vn/`
- **Kiến trúc phần cứng:** `aarch64` (`Linux 6.6.44-current-sunxi64`, 2 GB RAM, thẻ nhớ F2FS `/media/usb1` 59 GB)
- **Binary triển khai:** `/opt/go-shinobi-ksp/go-shinobi-ksp` (`go-shinobi-ksp-linux-arm64`, PM2 process `camera`)
- **Database SQLite WAL:** `/opt/go-shinobi-ksp/shinobi.sqlite` (Bản sao lưu gốc tại `/opt/go-shinobi-ksp/backup-legacy/ccio_backup.sql`, MariaDB/MySQL đã dừng & disable)
- **Group Key (`ke` / `shinobi_camera_id`):** `khanhha`
- **Số lượng Camera Monitors:** `10` (`camera01`..`camera10` — `192.168.3.x:554/cam/realmonitor?channel=1&subtype=0`, HEVC 2880x1620 @ 30fps, chế độ `record`)
- **Bảo toàn API Keys & Video Cũ:**
  - Giữ nguyên 100% 5 API keys từ MySQL `ccio.API` (`<REDACTED_ADMIN_KEY>` cho `127.0.0.1`, `<REDACTED_SHINOBI_TOKEN>` cho `shinobi_token`, `<REDACTED_MONITOR_TOKEN>` cho `shinobi_monitor_token`).
  - Đồng bộ toàn bộ **4,854+ file video `.mp4`** trên `/media/usb1/khanhha/camera01..camera10/` và chuẩn hóa Golden Template (`2880x1620`, `-tag:v hvc1 -segment_format_options moov_size=262144:movflags=+faststart`).
- **Trạng thái kiểm chứng:** `ONLINE` • 10/10 luồng FFmpeg ghi hình & HLS mượt mà • RAM `go-shinobi-ksp` ~23 MB.

---

### 1.4 `inut_205_67` — `77.88.205.67` (BinK Billiards Club)
- **Tên quán / Điểm lắp đặt:** BinK Billiards Club (`ui_title=BinK Billiards Club`)
- **Kiến trúc phần cứng:** `aarch64` (`Linux 6.6.44-current-sunxi64`, 2 GB RAM, thẻ nhớ F2FS `/media/usb1` 58 GB)
- **Binary triển khai:** `/opt/go-shinobi-ksp/go-shinobi-ksp` (`go-shinobi-ksp-linux-arm64`, PM2 process `camera`)
- **Database SQLite WAL:** `/opt/go-shinobi-ksp/shinobi.sqlite` (MariaDB/MySQL đã dừng & disable, tổng RAM hệ thống giảm `-560 MB` từ `1154 MB` xuống `594 MB`)
- **Group Key (`ke` / `shinobi_camera_id`):** `billiardclub`
- **Số lượng Camera Monitors:** `11` (`camera01`..`camera11` — NVR `192.168.10.10:554/cam/realmonitor?channel=1..11&subtype=0`, chế độ `record`)
- **Bảo toàn API Keys & Video Cũ:**
  - Giữ nguyên 100% 5 API keys từ MySQL `ccio.API` (`<REDACTED_SHINOBI_TOKEN>`, `<REDACTED_MONITOR_TOKEN>`) và toàn bộ **667+ file video `.mp4`** trên `/media/usb1/billiardclub/camera01..camera11/`.
- **Trạng thái kiểm chứng:** `ONLINE` • 11/11 luồng FFmpeg ghi hình 5 phút/clip (`moov_size=262144:movflags=+faststart`) • RAM `go-shinobi-ksp` ~18 MB.


### 1.5 `inut_204_251` — `77.88.204.251` (TG Golden Billiards Club)
- **Tên quán / Điểm lắp đặt:** TG Golden Billiards Club (`ui_title=TG Golden Billiards Club`)
- **Kiến trúc phần cứng:** `aarch64` (`Linux 5.15.190-ophub`, 2 GB RAM, thẻ nhớ F2FS `/media/usb1` 118 GB)
- **Binary triển khai:** `/opt/go-shinobi-ksp/go-shinobi-ksp` (`go-shinobi-ksp-linux-arm64`, PM2 process `camera`)
- **Database SQLite WAL:** `/opt/go-shinobi-ksp/shinobi.sqlite` (MariaDB/MySQL đã dừng & disable, tổng RAM hệ thống giảm từ `737 MB` xuống `464 MB` và Swap giảm từ `391 MB` xuống `80 MB`)
- **Group Key (`ke` / `shinobi_camera_id`):** `s912`
- **Số lượng Camera Monitors:** `10` (`camera01`..`camera10` — `192.168.2.239..249:554/cam/realmonitor?channel=1&subtype=0`, chế độ `record`)
- **Bảo toàn API Keys & Video Cũ:**
  - Giữ nguyên 100% API keys từ MySQL `ccio.API` (`<REDACTED_SHINOBI_TOKEN>`, `<REDACTED_MONITOR_TOKEN>`) và toàn bộ 82 GB file video `.mp4` trên `/media/usb1/s912/camera01..camera10/`.
- **Trạng thái kiểm chứng:** `ONLINE` • Luồng FFmpeg ghi hình 5 phút/clip (`moov_size=262144:movflags=+faststart`) • RAM `go-shinobi-ksp` ~18.7 MB • Load average `0.95`.

### 1.6 Lô 9 Node `armv7l` (`1 GB RAM`) Lên Full Go Ngày `2026-11-08`
- **`inut_201_102` (`77.88.201.102` — QL Billiards Club):** `ke=qlbilliards` • `7` cam (`192.168.1.234..240`) • RAM giảm từ `480 MB` xuống **`251 MB`** • `7/7` Recording & `7/7` snapshots `:2028` OK.
- **`inut_202_208` (`77.88.202.208` — CLB Bida Win):** `ke=win11` • `6` cam (`192.168.1.240..245`) • RAM giảm từ `538 MB` xuống **`223 MB`** • `6/6` Recording & `6/6` snapshots `:2028` OK.
- **`inut_202_214` (`77.88.202.214` — Billiard Club Chấn Hoàng):** `ke=chanhoang` • `7` cam (`192.168.2.241..247`) • RAM giảm từ `476 MB` xuống **`253 MB`** • `7/7` Recording & `7/7` snapshots `:2028` OK.
- **`inut_202_213` (`77.88.202.213` — Núi Thành Billiards Center):** `ke=nuithanh` • `4` cam (`192.168.1.240..243`, 2 cam online) • RAM giảm từ `411 MB` xuống **`206 MB`** • `2/2` cam online snapshots `:2028` OK.
- **`inut_202_226` (`77.88.202.226` — Queen Billiards Club):** `ke=giaphong` • `5` cam (`192.168.1.240..244`, 1 cam online) • RAM giảm từ `409 MB` xuống **`199 MB`** • `1/1` cam online snapshot `:2028` OK.
- **`inut_203_80` (`77.88.203.80` — Monkey Billiards Club):** `ke=monkey` • `8` cam (`192.168.1.202..210`) • RAM giảm từ `484 MB` xuống **`325 MB`** • `8/8` Recording & `8/8` snapshots `:2028` OK.
- **`inut_203_90` (`77.88.203.90` — AMIGOS Billiards):** `ke=amigos` • `7` cam NVR Hikvision (`192.168.100.189` ch `301..1001`) • RAM giảm từ `664 MB` xuống **`253 MB`** • `7/7` Recording & `7/7` snapshots `:2028` OK.
- **`inut_200_76` (`77.88.200.76` — SlyBilliards Club):** `ke=sly` • `7` cam (`192.168.1.241..247`, đã chạy `fsck.f2fs -a` phục hồi phân vùng `/media/usb1` không mất dữ liệu) • RAM giảm từ `493 MB` xuống **`257 MB`** • `6/7` Recording & `5/5` cam online snapshots `:2028` OK.
- **`inut_200_79` (`77.88.200.79` — 247 Billiards & Coffee Cần Thơ):** `ke=golfbg` • `6` cam (`192.168.1.201..206`) • RAM giảm từ `694 MB` xuống **`225 MB`** • `6/6` Recording & `6/6` snapshots `:2028` OK.

### 1.7 Lô 8 Node `aarch64` (`2 GB RAM`) Lên Full Go Ngày `2026-11-08`
- **`inut_204_163` (`77.88.204.163` — SD Billiards Club - CS2):** `ke=P6zP1kVhht` • `8` cam • RAM giảm từ `934 MB` xuống **`452 MB`** • `8/8` Recording & `8/8` snapshots `:2028` OK.
- **`inut_205_123` (`77.88.205.123` — Magic Arena Đà Nẵng):** `ke=magic2` • `12` cam NVR `:1554` • RAM giảm từ `921 MB` xuống **`436 MB`** • `12/12` Recording & `12/12` snapshots `:2028` OK.
- **`inut_205_124` (`77.88.205.124` — Magic Arena Đà Nẵng):** `ke=magic1` • `16` cam NVR `:1554` • RAM giảm từ `1143 MB` xuống **`500 MB`** • `16/16` Recording & `16/16` snapshots `:2028` OK.
- **`inut_204_36` (`77.88.204.36` — LEO Billiard):** `ke=bida2392` • `17` cam NVR • RAM giảm từ `1243 MB` xuống **`533 MB`** • `17/17` Recording & `17/17` snapshots `:2028` OK.
- **`inut_205_112` (`77.88.205.112` — LEO Billiard):** `ke=t9` • `13` cam NVR • RAM giảm từ `1362 MB` xuống **`766 MB`** • `13/13` Recording & `13/13` snapshots `:2028` OK.
- **`inut_205_29` (`77.88.205.29` — CLB Bida 49 Ninh Hoà):** `ke=banchobienhoanha` • `12` cam • RAM giảm từ `1149 MB` xuống **`592 MB`** • `12/12` Recording & `12/12` snapshots `:2028` OK.
- **`inut_205_75` (`77.88.205.75` — 49 Billiards Carom Club Ninh Hòa):** `ke=49` • `10` cam • RAM giảm từ `872 MB` xuống **`562 MB`** • `10/10` Recording & `10/10` snapshots `:2028` OK.
- **`inut_205_85` (`77.88.205.85` — Lion King Billiards Club Nam Định - CS2):** `ke=lionkingnd` • `6` cam • RAM giảm từ `809 MB` xuống **`503 MB`** • `6/6` Recording & `6/6` snapshots `:2028` OK.

### 1.8 Lô 16 Node (`202_173..205_27`) Lên Full Go Ngày `2026-11-08`
- **`inut_202_173` (`77.88.202.173` — Thế Trâm Pickleball):** `ke=dmdeabhancca` • `1` cam • RAM giảm từ `561 MB` xuống **`225 MB`** • `1/1` Recording & `1/1` snapshot `:2028` OK.
- **`inut_202_210` (`77.88.202.210` — Lang Biang Pickleball Club):** `ke=INUTJSCSO1VIETNAMNHEMAYCONVO` • `2` cam • RAM giảm từ `579 MB` xuống **`280 MB`** • `2/2` Recording & `2/2` snapshots `:2028` OK.
- **`inut_202_218` (`77.88.202.218` — PFA T-Tennis & Pickleball):** `ke=pfareal` • `2` cam • RAM giảm từ `537 MB` xuống **`206 MB`** • `2/2` Recording & `1/1` cam online snapshot `:2028` OK.
- **`inut_202_219` (`77.88.202.219` — CLB Pickleball 26gram - CN2):** `ke=vnvarISdog` • `3` cam • RAM giảm từ `574 MB` xuống **`242 MB`** • `3/3` Recording & `3/3` snapshots `:2028` OK.
- **`inut_202_222` (`77.88.202.222` — Fox Billiards & Coffee Hội An):** `ke=chiubia` • `4` cam • RAM giảm từ `610 MB` xuống **`205 MB`** • `4/4` snapshots `:2028` OK (chưa gắn thẻ nhớ ngoài `/media/usb1`, Storage Guard tự bảo vệ eMMC).
- **`inut_203_81` (`77.88.203.81` — Dinker Pickleball Club):** `ke=dinker` • `6` cam • RAM giảm từ `643 MB` xuống **`261 MB`** • `6/6` Recording & `5/5` cam online snapshots `:2028` OK.
- **`inut_204_22` (`77.88.204.22` — VSP Pickleball):** `ke=lAxKsdIBux` • `14` cam • RAM giảm từ `1372 MB` xuống **`713 MB`** • `14/14` Recording & `13/13` cam online snapshots `:2028` OK.
- **`inut_204_107` (`77.88.204.107` — Trang Hoàng Pickleball):** `ke=cam4107` • `16` cam • RAM giảm từ `1161 MB` xuống **`547 MB`** • `16/16` Recording & `16/16` snapshots `:2028` OK.
- **`inut_204_135` (`77.88.204.135` — HTP Pickleball):** `ke=b389` • `6` cam • RAM giảm từ `955 MB` xuống **`539 MB`** • `6/6` Recording & `6/6` snapshots `:2028` OK.
- **`inut_204_157` (`77.88.204.157` — Kely Pickleball):** `ke=kely` • `13` cam • RAM giảm từ `937 MB` xuống **`512 MB`** • `12/13` Recording & `12/12` cam online snapshots `:2028` OK.
- **`inut_204_158` (`77.88.204.158` — Sân Pickleball BRG):** `ke=brg` • `12` cam • RAM giảm từ `1081 MB` xuống **`538 MB`** • `12/12` Recording & `12/12` snapshots `:2028` OK.
- **`inut_204_160` (`77.88.204.160` — QA Pickleball):** `ke=qa` • `3` cam • RAM giảm từ `822 MB` xuống **`408 MB`** • `3/3` Recording & `3/3` snapshots `:2028` OK.
- **`inut_204_165` (`77.88.204.165` — Nguyễn Huệ Pickleball):** `ke=KT8zqe2yGq` • `4` cam • RAM giảm từ `840 MB` xuống **`429 MB`** • `4/4` Recording & `4/4` snapshots `:2028` OK.
- **`inut_205_8` (`77.88.205.8` — PICKO 247):** `ke=picko247` • `6` cam • RAM giảm từ `794 MB` xuống **`371 MB`** • `6/6` Recording & `6/6` snapshots `:2028` OK.
- **`inut_205_12` (`77.88.205.12` — Hoàng Tuấn Pickleball):** `ke=hoangtuan` • `5` cam • RAM giảm từ `1099 MB` xuống **`641 MB`** • `5/5` Recording & `5/5` snapshots `:2028` OK.
- **`inut_205_27` (`77.88.205.27` — MatchUp Sports Complex):** `ke=thucc` • `6` cam • RAM giảm từ `1047 MB` xuống **`520 MB`** • `6/6` Recording & `6/6` snapshots `:2028` OK.
- **`inut_205_5` (`77.88.205.5` — Tok Tok Billiards Center):** `ke=toktok` • `18` cam • RAM giảm từ `1201 MB` xuống **`484 MB`** • `18/18` Recording & `18/18` snapshots `:2028` OK.
- **`inut_204_97` (`77.88.204.97` — Tok Tok II Billiards Center):** `ke=toktok2` • `16` cam • RAM giảm từ `1218 MB` xuống **`546 MB`** • `16/16` Recording & `16/16` snapshots `:2028` OK.
- **`inut_204_146` (`77.88.204.146` — Sniper Billiards Club):** `ke=sniper` • `14` cam • RAM giảm từ `1460 MB` xuống **`912 MB`** • `14/14` Recording & `1/1` NVR snapshot `:2028` OK.
- **`inut_205_47` (`77.88.205.47` — Aileex Billiards Club Yên Bái):** `ke=alieex` • `13` cam • RAM giảm từ `983 MB` xuống **`510 MB`** • `13/13` Recording & `13/13` snapshots `:2028` OK.

### 1.9 Lô 15 Node (`205_6..205_121`) Lên Full Go Ngày `2026-11-08`
- **`inut_205_6` (`77.88.205.6` — TheLight Pool Arena):** `ke=thelightpoolarena` • `10` cam • RAM giảm từ `703 MB` xuống **`388 MB`** • `10/10` Recording & `10/10` snapshots `:2028` OK.
- **`inut_205_7` (`77.88.205.7` — LoGi Billiards):** `ke=logi` • `10` cam • RAM giảm từ `1004 MB` xuống **`391 MB`** • `10/10` Recording & `10/10` snapshots `:2028` OK.
- **`inut_205_10` (`77.88.205.10` — CLB Billiard The King):** `ke=xinchao` • `8` cam • RAM giảm từ `729 MB` xuống **`388 MB`** • `8/8` Recording & `8/8` snapshots `:2028` OK.
- **`inut_205_14` (`77.88.205.14` — ATD Billiards Club):** `ke=h2` • `12` cam (`8` online) • RAM giảm từ `1222 MB` xuống **`513 MB`** • `7/7` active Recording & `8/8` online snapshots `:2028` OK.
- **`inut_205_17` (`77.88.205.17` — Bi-a AnhEm):** `ke=anhem` • `8` cam • RAM giảm từ `951 MB` xuống **`471 MB`** • `8/8` Recording & `8/8` snapshots `:2028` OK.
- **`inut_205_18` (`77.88.205.18` — MAY Billiards):** `ke=b52cs32` • `8` cam (`7` online) • RAM giảm từ `729 MB` xuống **`398 MB`** • `7/7` online Recording & `7/7` online snapshots `:2028` OK.
- **`inut_205_19` (`77.88.205.19` — Billiards Quang Phúc):** `ke=qp` • `10` cam (`6` active, `4` Stopped) • RAM giảm từ `933 MB` xuống **`417 MB`** • `6/6` active Recording & `6/6` snapshots `:2028` OK.
- **`inut_205_43` (`77.88.205.43` — Konoha Premium):** `ke=konoha` • `13` cam (`12` active, `1` Stopped) • RAM giảm từ `1284 MB` xuống **`700 MB`** • `12/12` active Recording & `13/13` snapshots `:2028` OK.
- **`inut_205_45` (`77.88.205.45` — Bida Hoàng Phát 6):** `ke=thua` • `9` cam • RAM giảm từ `1271 MB` xuống **`600 MB`** • `9/9` Recording & `9/9` snapshots `:2028` OK.
- **`inut_205_51` (`77.88.205.51` — Dragon Billiards Club):** `ke=thin` • `10` cam • RAM giảm từ `1197 MB` xuống **`535 MB`** • `10/10` Recording & `10/10` snapshots `:2028` OK.
- **`inut_205_53` (`77.88.205.53` — The Best Billiards Club):** `ke=ultrapool1` • `12` cam • RAM giảm từ `848 MB` xuống **`474 MB`** • `12/12` Recording & `11/12` snapshots `:2028` OK.
- **`inut_205_55` (`77.88.205.55` — Tuyết Linh Billiards Club):** `ke=tuyetlinh` • `11` cam • RAM giảm từ `763 MB` xuống **`421 MB`** • `11/11` Recording & `11/11` snapshots `:2028` OK.
- **`inut_205_56` (`77.88.205.56` — King Kong Thái Nguyên):** `ke=mp4` • `9` cam • RAM giảm từ `779 MB` xuống **`392 MB`** • `9/9` Recording & `9/9` snapshots `:2028` OK.
- **`inut_205_82` (`77.88.205.82` — Hoàng Tom Billiards):** `ke=mia` • `16` cam • RAM giảm từ `1205 MB` xuống **`632 MB`** • `16/16` Recording & `30/30` snapshots `:2028` OK.
- **`inut_205_121` (`77.88.205.121` — Black Coffee Billiards):** `ke=loicafe` • `6` cam • RAM giảm từ `935 MB` xuống **`391 MB`** • `6/6` Recording & `6/6` snapshots `:2028` OK.
- **`inut_204_144` (`77.88.204.144` — Lio Kig Billiads Club):** `ke=replaceemmc` • `8` cam • RAM giảm từ `962 MB` xuống **`502 MB`** • `8/8` Recording & `8/8` snapshots `:2028` OK.
- **`inut_204_34` (`77.88.204.34` — Billiards Guu Pool Arena):** `ke=bida2391` • `16` cam • RAM giảm từ `1579 MB` xuống **`674 MB`** • `16/16` Recording & `16/16` snapshots `:2028` OK.
- **`inut_204_200` (`77.88.204.200` / `4.200` — F - Zone Billiards):** `ke=timbo` • `17` cam (`16` active online, `1` offline) • RAM giảm từ `2118 MB` xuống **`1400 MB`** (Free RAM: `2.5 GB`, MariaDB tắt & mask) • `16/16` active continuous 5-min MP4 Recording OK • NVR Dahua `192.168.1.199:37777` 8CH (`live:inut123@`) đã quét & phân tích: kênh 1-5 là camera an ninh quầy/khuôn viên, kênh 6 `192.168.1.211` không có lịch ghi NVR, toàn bộ 17 bàn bida ghi hình độc lập 100% trên thẻ nhớ Box-SD (`/media/usb1`, f2fs).
- **`inut_201_237` (`77.88.201.237` / `1.237` — Bida Dũng Huê):** `ke=kao` • `6` cam • `linux/armv7` • RAM giảm từ `783 MB` xuống **`370 MB`** (giải phóng **`413 MB`**, RAM khả dụng tăng vọt lên **`580 MB / 999 MB`**, MariaDB tắt & mask) • `camera01..04` ghi hình liên tục 5 phút/clip trên thẻ Box-SD (`/media/usb1`, f2fs `30 GB`, Storage Guard `85%`) • `camera10` (`192.168.1.236`) có thẻ nhớ 32GB onboard (`/dev/mmc0`, `/mnt/sd`) kích hoạt thành công chế độ **Cam-SD Hybrid Bridge** (`v_nvr_*` clips), phát trực tiếp HTTP 206 và cắt clip `/cut-video-stream` tức thì với 0% CPU encode.
- **`inut_204_2` (`77.88.204.2` / `4.2` — Dragon Billiards Club):** `ke=Fgbghldvgk` • `11` cam (`camera01..10` 10 bàn bida + `camera11` offline) • `linux/arm64` (`6.1.63-current-rockchip64`, RAM `4 GB`) • RAM giảm từ `1,325 MB` xuống **`645 MB/3,974 MB`** (giải phóng **`680 MB`**, RAM khả dụng tăng lên **`3,151 MB`**, MariaDB tắt & mask) • `5,313` video clips và `11` monitors migrate thành công sang SQLite WAL, 0 clip mất (`mp4_before=5321 mp4_after=5342`) • Kích hoạt thành công chế độ **Hybrid SD ↔ NVR Xiongmai** (`NBD80N32RA-KL-V3`, `192.168.1.93:80`, ổ `2 TB`, Kênh `1..10` `D02..D11`), mở rộng lịch sử từ vài ngày thẻ Box `239 GB` lên **trọn vẹn 18+ ngày (hơn 430 giờ, từ `2026-10-23`)**; tua Slow-Mo **`36.8 ms` (`HTTP 206 Partial Content`)**, cắt clip `/cut-video-stream` tức thì với 0% CPU encode.
- **`inut_205_81` (`77.88.205.81` / `5.81` — Galaxy Billards Trị An):** `ke=dn` • `8` cam • `linux/arm64` (`6.6.44-current-sunxi64`, RAM `2 GB`) • RAM giảm từ `875 MB` xuống **`480 MB/1,923 MB`** (giải phóng **`395 MB`**, RAM khả dụng tăng lên **`1,442 MB`**, MariaDB tắt & mask) • `578` video clips và `8` monitors migrate sang SQLite WAL, 0 clip mất • `8/8` camera Dahua online, `camera01..06` (`admin:Abcd@1234`), `camera07..08` (`admin:admin@123`, 3K `2880x1620` 30fps) • Thẻ nhớ `/media/usb1` `59 GB` f2fs (`noatime,errors=continue`, Storage Guard `85%`) • Tua Slow-Mo **`6.79 ms` (`HTTP 206 Partial Content`)**, cắt clip `/cut-video-stream` tức thì `200 OK` video/mp4.
- **`inut_202_211` (`77.88.202.211` / `2.211` — 99 Billiards CS2 - Box 1):** `ke=uiOooJI3Lu` • `5` cam • `linux/armv7` (RAM `1 GB`) • RAM giảm từ `582 MB` xuống **`254 MB/999 MB`** (giải phóng **`328 MB`**, RAM khả dụng tăng lên **`680 MB`**, MariaDB tắt & mask) • Thẻ nhớ Box `/dev/mmcblk0p1` `30 GB` f2fs automount chuẩn hóa `noatime,errors=continue` • Kích hoạt thành công chế độ **Hybrid SD ↔ NVR Hikvision** (`DS-7632NXI-K2`, `192.168.1.200:80`, user `live:inut161@`), liên kết `5/5` bàn (`camera01` ch 24, `camera02` ch 1, `camera03` ch 4, `camera04` ch 16, `camera05` ch 22) vào NVR, tự động tổng hợp các clip ảo 5 phút `v_nvr_*` cho các ngày trước; tua Slow-Mo tức thì qua `HTTP 206` và cắt clip 0% CPU encode.
- **`inut_204_67` (`77.88.204.67` / `4.67` — 99 Billiards CS2 - Box 2):** `ke=u5Fk1fbkkZ` • `14` cam • `linux/arm64` (`6.1.63-current-rockchip64`, RAM `4 GB`) • RAM giảm từ `1,129 MB` xuống **`430 MB/3,974 MB`** (giải phóng **`699 MB`**, RAM khả dụng tăng lên **`3,200 MB`**, MariaDB tắt & mask) • Thẻ nhớ Box `/dev/mmcblk0p1` `120 GB` f2fs automount chuẩn hóa `noatime,errors=continue` • Kích hoạt thành công chế độ **Hybrid SD ↔ NVR Hikvision** (`DS-7632NXI-K2`, `192.168.1.200:80`, user `live:inut161@`), liên kết toàn bộ `14/14` bàn (`camera01..camera14` tương ứng kênh `9, 8, 17, 19, 11, 10, 12, 14, 15, 5, 2, 6, 23, 13`) vào NVR, tự động tổng hợp `v_nvr_*` clips 5 phút trên thanh timeline :2023; tua Slow-Mo tức thì qua `HTTP 206` và cắt clip 0% CPU encode.
- **`inut_205_39` (`77.88.205.39` / `5.39` — Mitkute Billiards):** `ke=tony21` • `6` cam (`Camera01..06`, Dahua DH-H5AE `192.168.100.201..206:37777`) • `linux/arm64` (`6.6.44-current-sunxi64`, RAM `2 GB`) • RAM giảm từ `771 MB` xuống **`479 MB/1,923 MB`** (giải phóng **`292 MB`**, RAM khả dụng tăng từ `1,152 MB` lên **`1,444 MB`**, MariaDB & MySQL tắt và masked) • `6` monitors và 4 API keys migrate sạch sang SQLite WAL, 0 clip mất (`mp4_before=0 mp4_after=0`) • `go-shinobi-ksp` ~15.8 MB, `redbida-go` ~14.2 MB, `kspcam` `:2028` ~2.7 MB, `inut-bida-agent` active ~2.6 MB • Đã thực hiện Golden Hardening: chuẩn hóa media-automount `usb${device_num:-1}`, cấu hình `/etc/media-automount.d/{f2fs,ext4,auto}`, fstab sạch, gỡ & mask `netwatch.service`, khóa `kernel.sysrq=0` • Đánh giá Hybrid: Quét LAN 192.168.100.0/24 không có NVR/Cam-SD cho dàn bàn (DH-H5AE `cardReader=false`), giữ nguyên kiến trúc Box-SD thuần; thẻ nhớ Box `mmcblk0` gặp lỗi phần cứng bad sector I/O (-5) được hệ thống bảo vệ chuyển sang Watch-Only HLS an toàn không ghi đè eMMC root.
- **`inut_204_18` (`77.88.204.18` / `4.18` — The King Luxury Billiard - 1):** `ke=lux1` • `14` cam (`Camera01..14`, ONVIF TR38Y22/51 `192.168.1.151..181:554`) • `linux/arm64` (`6.1.63-current-rockchip64`, RAM `4 GB`) • RAM giảm từ `1,181 MB` xuống **`715 MB/3,974 MB`** (giải phóng **`466 MB`**, RAM khả dụng tăng từ `2,561 MB` lên **`3,065 MB`**, MariaDB & MySQL tắt và masked) • `8,107` video clips và `15` monitors migrate sạch sang SQLite WAL, 0 clip mất (hiện tại `>8,175` clips) • Thẻ nhớ Box `/media/usb1` `60 GB` f2fs: giải phóng `45.3 GB` file descriptor rò rỉ của Node.js cũ (hạ từ `100% full` xuống `76%..80%`, Storage Guard `85%/80%` bảo vệ) • Đã thực hiện Golden Hardening: netwatch disabled & masked, sysrq=0, media-automount `usb${device_num:-1}`, `/etc/media-automount.d/{f2fs,ext4,auto}` (`noatime,users`), service override `Type=oneshot` • Đánh giá Hybrid: Quét mạng LAN 192.168.1.0/24 và 192.168.20.0/24 không có NVR/Cam-SD cho dàn bàn bida -> Giữ kiến trúc Box-SD thuần • Đã kiểm chứng `/healthz` trên `:8080`, `:2023`, `:2028` và tua Slow-Mo **`HTTP 206 Partial Content`** mượt mà (+faststart moov atom tại byte 0x1C).
- **`inut_203_251` (`77.88.203.251` / `3.251` — Ken Billiards Club):** `ke=ken` • `6` cam (`Camera01..06`, Dahua `192.168.5.246..251:37777`) • `linux/armv7` (`5.15.80-sunxi`, RAM `1 GB`) • RAM giảm từ `673 MB` xuống **`317 MB/999 MB`** (giải phóng **`356 MB`**, RAM khả dụng tăng từ `267 MB` lên **`631 MB`**, MariaDB & MySQL tắt và masked) • `986` video clips, 6 monitors, 1 user và 9 API keys migrate sạch sang SQLite WAL, 0 clip mất (`mp4_before=986 mp4_after=986`) • `go-shinobi-ksp` ~13.8 MB, `redbida-go` ~11.0 MB, `kspcam` `:2028` ~1.3 MB, `inut-bida-agent` active (Yamux) • Đã thực hiện Golden Hardening: chuẩn hóa media-automount `usb${device_num:-1}`, cấu hình `/etc/media-automount.d/{f2fs,ext4,auto}`, fstab sạch, gỡ & mask `netwatch.service`, khóa `kernel.sysrq = 0`, thẻ nhớ `/media/usb1` 60GB f2fs (Storage Guard `85%/80%` bảo vệ) • Đánh giá Hybrid: Quét mạng LAN 192.168.5.0/24 không có NVR/Cam-SD cho dàn bàn -> Giữ kiến trúc Box-SD thuần • Kiểm chứng đầy đủ `/healthz` trên `:8080`, `:2023`, `:2028`, phản hồi tua Slow-Mo **HTTP 206 Partial Content** chuẩn xác.
---

## 2. Quy Trình Chuẩn Onboarding Từ Shinobi Cũ Lên Go-Shinobi-KSP (Zero Clip Loss)

### 2.1 Lệnh Ansible 1-Click Trên Máy Chủ Build (`172.16.5.180`)
```bash
cd /build/armbian-build/ansible

# 1. Onboarding tự động (Chốt clip đang ghi + Backup MySQL ccio + Migrate Users/API/Monitors/Videos + Chuyển PM2 camera + Tắt MySQL):
make go-shinobi inut_206_10

# 2. Backup cấu hình Monitors ra file JSON chuẩn 1:1 Shinobi:
make backup-shinobi inut_206_10

# 3. Cập nhật binary Go-Shinobi-KSP mới nhất (tự động nhận diện Go-Shinobi vs Shinobi cũ):
make shinobi-update inut_206_10

# 4. Cài đặt / cập nhật Watchdog & Storage Guard tương thích kép:
make auto-fix-shinobi inut_206_10
```
