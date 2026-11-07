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
