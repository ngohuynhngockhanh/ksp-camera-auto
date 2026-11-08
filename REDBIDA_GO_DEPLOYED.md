# REDBIDA_GO_DEPLOYED.md — Danh Sách Node Đã Triển Khai `redbida-go` (`:2023`)

> **Mục đích:** Theo dõi tập trung toàn bộ các Edge Box đã chuyển đổi tiến trình PM2 `inut` (`:2023`) từ Node-RED (`inut_3rd_project`) sang **`redbida-go`** thuần Go (`CGO_ENABLED=0`, MQTT `12369` `mhw`, `hls.js`, hỗ trợ `#!/0..#!/19` `C01..C20` + `#!/20..23` + `#!/24..#!/30` `C21..C27`).

---

## 1. Danh Sách Các Node Đã Triển Khai (`Production Fleet`)

- **`inut_206_14` (`77.88.206.14` / `6.14` — King Billiards - Bình Nguyên)**:
  - **Ngày lên Full Go:** `2026-11-07`
  - **Kiến trúc:** `linux/armv7` (`armv7l`), chạy song song `go-shinobi-ksp` (`:8080`, 1 cam) + `redbida-go` (`:2023`) + `kspcam` (`:2028`) + `ota-mqtt` (`:12369`).
  - **RAM tiết kiệm:** Tiến trình PM2 `inut` giảm từ `~175 MB` (Node-RED) xuống **`~14 MB`** (`redbida-go`).
  - **Trạng thái:** `ONLINE` — hoạt động đầy đủ `/#!/0..30`, `/api/v1/:id`, `/inut-billiard-live-cong-ty-inut-jsc/:id`, `/hls/...`, `/videos/...`, MQTT `12369` (`/data/alarms` chuẩn 1:1).

- **`inut_206_10` (`77.88.206.10` / `6.10` — 1992 Đại Dương Club)**:
  - **Ngày lên Full Go:** `2026-11-07`
  - **Kiến trúc:** `linux/armv7` (`armv7l`), chạy song song `go-shinobi-ksp` (`:8080`, 1 cam) + `redbida-go` (`:2023`) + `kspcam` (`:2028`) + `ota-mqtt` (`:12369`).
  - **RAM tiết kiệm:** Tiến trình PM2 `inut` giảm từ `148.4 MB` (Node-RED) xuống **`~11 MB`** (`redbida-go`).
  - **Trạng thái:** `ONLINE` — hoạt động đầy đủ `/#!/0..30`, `/api/v1/:id`, `/inut-billiard-live-cong-ty-inut-jsc/:id`, `/hls/...`, `/videos/...`, MQTT `12369` (`/data/alarms` chuẩn 1:1).

- **`inut_205_98` (`77.88.205.98` / `5.98` — Empire Gaming & Billiards – Bắc Ninh)**:
  - **Ngày lên Full Go:** `2026-11-07`
  - **Kiến trúc:** `linux/arm64` (`aarch64`), chạy song song `go-shinobi-ksp` (`:8080`, 10 camera 3K HEVC) + `redbida-go` (`:2023`) + `kspcam` (`:2028`) + `inut-bida-agent` + `ota-mqtt` (`:12369`).
  - **RAM tiết kiệm:** Tiến trình PM2 `inut` giảm từ `~225.5 MB` (Node-RED) xuống **`~14 MB`** (`redbida-go`).
  - **Trạng thái:** `ONLINE` — hoạt động đầy đủ `/#!/0..30`, `/api/v1/:id`, `/inut-billiard-live-cong-ty-inut-jsc/:id`, `/hls/...`, `/videos/...`, MQTT `12369` (`/data/alarms` chuẩn 1:1).

- **`inut_205_67` (`77.88.205.67` / `5.67` — BinK Billiards Club - Quy Nhơn)**:
  - **Ngày lên Full Go:** `2026-11-08`
  - **Kiến trúc:** `linux/arm64` (`aarch64`), chạy đồng bộ `go-shinobi-ksp` (`:8080`, 11 camera NVR, RAM `~16 MB`) + `redbida-go` (`:2023`, RAM `~13 MB`) + `ksp-go2rtc` (`:1984`, CORS `origin: "*"`) + `kspcam` (`:2028`) + `inut-bida-agent` + `ota-mqtt` (`:12369`).
  - **Trạng thái:** `ONLINE` — đầy đủ CORS (`Access-Control-Allow-Origin: *`) trên cả `:2023`, `:8080`, `:1984` cho màn hình bảng điểm Android `rk3566_r` (`Cameralive CodePush v41`), i18n (`vi`/`en`/`cz`), và CheckVAR 1 Cam & 2 Cam.

- **`inut_204_251` (`77.88.204.251` / `4.251` — TG Golden Billiards Club)**:
  - **Ngày lên Full Go:** `2026-11-08`
  - **Kiến trúc:** `linux/arm64` (`aarch64`), chạy đồng bộ `go-shinobi-ksp` (`:8080`, 10 camera, RAM `~18.7 MB`) + `redbida-go` (`:2023`, RAM `~14.2 MB`, giảm từ `183.6 MB` Node-RED) + `ksp-go2rtc` (`:1984`, RAM `~15.6 MB`, CORS `origin: "*"`) + `ota-mqtt` (`:12369`). Tổng RAM hệ thống giảm từ `737 MB` xuống **`464 MB`** (giải phóng `273 MB` RAM + `311 MB` Swap).
  - **Trạng thái:** `ONLINE` — đầy đủ CORS (`Access-Control-Allow-Origin: *`), `/#!/0..30`, `/api/v1/:id`, `/inut-billiard-live-cong-ty-inut-jsc/:id`, `/root/go2rtc.yaml` 10 streams, và MQTT `12369` (`mhw` unwrap `/private/i_sets/ack`).

- **`inut_201_102` (`77.88.201.102` — QL Billiards Club, `ke=qlbilliards`, 7 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM: `251M/999M` (giảm từ `480M`), `go-shinobi-ksp` `~9.8 MB`, `redbida-go` `~10.7 MB`, `ksp-go2rtc` `~10.1 MB`, `kspcam` `:2028` (`7/7` snapshots OK), `inut-bida-agent` `active`.

- **`inut_202_208` (`77.88.202.208` — CLB Bida Win, `ke=win11`, 6 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM: `223M/999M` (giảm từ `538M`), `go-shinobi-ksp` `~9.2 MB`, `redbida-go` `~9.1 MB`, `ksp-go2rtc` `~8.3 MB`, `kspcam` `:2028` (`6/6` snapshots OK), `inut-bida-agent` `active`.

- **`inut_202_214` (`77.88.202.214` — Billiard Club Chấn Hoàng, `ke=chanhoang`, 7 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM: `253M/999M` (giảm từ `476M`), `go-shinobi-ksp` `~12.2 MB`, `redbida-go` `~9.5 MB`, `ksp-go2rtc` `~7.6 MB`, `kspcam` `:2028` (`7/7` snapshots OK), `inut-bida-agent` `active`.

- **`inut_202_213` (`77.88.202.213` — Núi Thành Billiards Center, `ke=nuithanh`, 4 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM: `206M/999M` (giảm từ `411M`), `go-shinobi-ksp` `~13.0 MB`, `redbida-go` `~11.9 MB`, `ksp-go2rtc` `~13.5 MB`, `kspcam` `:2028` (4 cam synced, 2 online snapshots OK), `inut-bida-agent` `active`.

- **`inut_202_226` (`77.88.202.226` — Queen Billiards Club, `ke=giaphong`, 5 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM: `199M/999M` (giảm từ `409M`), `go-shinobi-ksp` `~14.4 MB`, `redbida-go` `~11.1 MB`, `ksp-go2rtc` `~12.5 MB`, `kspcam` `:2028` (5 cam synced, 1 online snapshot OK), `inut-bida-agent` `active`.

- **`inut_203_80` (`77.88.203.80` — Monkey Billiards Club, `ke=monkey`, 8 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM: `325M/999M` (giảm từ `484M`), `go-shinobi-ksp` `~12.3 MB`, `redbida-go` `~11.0 MB`, `ksp-go2rtc` `~13.9 MB`, `kspcam` `:2028` (`8/8` snapshots OK), `inut-bida-agent` `active`.

- **`inut_203_90` (`77.88.203.90` — AMIGOS Billiards, `ke=amigos`, 7 cam NVR Hikvision)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM: `253M/999M` (giảm từ `664M`), `go-shinobi-ksp` `~16.0 MB` (`7/7` Recording), `redbida-go` `~10.9 MB`, `ksp-go2rtc` `~11.7 MB`, `kspcam` `:2028` (`7/7` snapshots OK), `inut-bida-agent` `active`.

- **`inut_200_76` (`77.88.200.76` — SlyBilliards Club, `ke=sly`, 7 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM: `257M/999M` (giảm từ `493M`), `go-shinobi-ksp` `~16.1 MB` (`fsck.f2fs -a` phục hồi thẻ nhớ không mất clip, `6/7` cam online Recording), `redbida-go` `~11.5 MB`, `ksp-go2rtc` `~14.5 MB`, `kspcam` `:2028` (`5/5` cam online snapshots OK), `inut-bida-agent` `active`.

- **`inut_200_79` (`77.88.200.79` — 247 Billiards & Coffee Cần Thơ, `ke=golfbg`, 6 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM: `225M/999M` (giảm từ `694M`), `go-shinobi-ksp` `~16.0 MB` (`6/6` Recording), `redbida-go` `~11.8 MB`, `ksp-go2rtc` `~13.4 MB`, `kspcam` `:2028` (`6/6` snapshots OK), `inut-bida-agent` `active`.

- **`inut_204_163` (`77.88.204.163` / `4.163` — SD Billiards Club - CS2, `ke=P6zP1kVhht`, 8 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `934M` xuống **`452M/1794M`** (giải phóng `482 MB`), `go-shinobi-ksp` `~17.7 MB`, `redbida-go` `~27.4 MB`, `ksp-go2rtc` `~15.2 MB`, `kspcam` `:2028` (`8/8` snapshots OK), `inut-bida-agent` `active`.

- **`inut_205_123` (`77.88.205.123` / `5.123` — Magic Arena Đà Nẵng, `ke=magic2`, 12 cam gốc + **Cứu hộ 16 cam của `5.122`** = 28 cam)**:
  - **Ngày lên Full Go + Hybrid SD↔NVR + Box Rescue (`192.168.5.122` & `store-magic-1.video.io.vn`):** `2026-11-08` • `linux/arm64` • RAM **`463M/1923M`** • Chạy `12` cam gốc (`camera01..12`, `897` clip Hybrid `72h`) **ĐỒNG THỜI gánh cứu hộ toàn bộ `16` cam của `5.122` (`camera13..28`, kênh NVR `3, 4, 17..30`, `900` clip `72h`)** qua VIP `192.168.5.122/24` trên `lan0` + tên miền `store-magic-1.video.io.vn` với **`0` tiến trình `ffmpeg` nền** (Live on-demand qua `go2rtc`, Replay/Cut `8.23s` trực tiếp từ đầu ghi Hikvision `192.168.1.50:82`).

- **`inut_205_124` (`77.88.205.124` / `5.124` — Magic Arena Đà Nẵng, `ke=magic1`, 16 cam — `Hybrid SD + NVR 72h`)**:
  - **Ngày lên Full Go + Hybrid SD↔NVR Bridge:** `2026-11-08` • `linux/arm64` • RAM **`523M/1923M`** • Kết hợp thẻ nhớ 128GB (`~240–254` clip gần nhất `~17.2 giờ`) + đầu ghi Hikvision 64 kênh `192.168.1.50:82` (kênh `43..58`, `+675–683` clip quá khứ) thành **`917–935` clip/bàn liên mạch trọn 72 giờ (`4.2x` dung lượng thẻ nhớ)** • Mở clip 45 giờ trước `17.24s` / tua Slow-Mo `20.78ms`, cắt clip vắt qua điểm nối NVR↔SD trong `7.44s`, tên file tải chuẩn `inut_cameraXX_...`.

- **`inut_204_36` (`77.88.204.36` / `4.36` — LEO Billiard, `ke=bida2392`, 17 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1243M` xuống **`533M/1980M`** (giải phóng `710 MB`), `go-shinobi-ksp` `~20.5 MB`, `redbida-go` `~27.8 MB`, `ksp-go2rtc` `~15.7 MB`, `kspcam` `:2028` (`17/17` snapshots OK), `inut-bida-agent` `active`.

- **`inut_205_112` (`77.88.205.112` / `5.112` — LEO Billiard, `ke=t9`, 13 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1362M` xuống **`766M/1923M`** (giải phóng `596 MB`), `go-shinobi-ksp` `~18.0 MB`, `redbida-go` `~27.0 MB`, `ksp-go2rtc` `~16.2 MB`, `kspcam` `:2028` (`13/13` snapshots OK), `inut-bida-agent` `active`.

- **`inut_205_29` (`77.88.205.29` / `5.29` — CLB Bida 49 Ninh Hoà, `ke=banchobienhoanha`, 12 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1149M` xuống **`592M/1923M`** (giải phóng `557 MB`), `go-shinobi-ksp` `~25.9 MB`, `redbida-go` `~28.5 MB`, `ksp-go2rtc` `~15.8 MB`, `kspcam` `:2028` (`12/12` snapshots OK), `inut-bida-agent` `active`.

- **`inut_205_75` (`77.88.205.75` / `5.75` — 49 Billiards Carom Club Ninh Hòa, `ke=49`, 10 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `872M` xuống **`562M/1923M`** (giải phóng `310 MB`), `go-shinobi-ksp` `~20.6 MB`, `redbida-go` `~12.6 MB`, `ksp-go2rtc` `~16.1 MB`, `kspcam` `:2028` (`10/10` snapshots OK), `inut-bida-agent` `active`.

- **`inut_205_85` (`77.88.205.85` / `5.85` — Lion King Billiards Club Nam Định - CS2, `ke=lionkingnd`, 6 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `809M` xuống **`503M/1923M`** (giải phóng `306 MB`), `go-shinobi-ksp` `~18.6 MB`, `redbida-go` `~26.6 MB`, `ksp-go2rtc` `~16.2 MB`, `kspcam` `:2028` (`6/6` snapshots OK), `inut-bida-agent` `active`.

- **`inut_202_173` (`77.88.202.173` — Thế Trâm Pickleball, `ke=dmdeabhancca`, 1 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM giảm từ `561M` xuống **`225M/999M`** (giải phóng `336 MB`), `go-shinobi-ksp` `~17.0 MB` (`1/1` Recording), `redbida-go` `~11.7 MB`, `kspcam` `:2028` (`1/1` snapshots OK), `inut-bida-agent` `active`.
- **`inut_202_210` (`77.88.202.210` — Lang Biang Pickleball Club, `ke=INUTJSCSO1VIETNAMNHEMAYCONVO`, 2 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM giảm từ `579M` xuống **`280M/999M`** (giải phóng `299 MB`), `go-shinobi-ksp` `~14.5 MB` (`2/2` Recording), `redbida-go` `~10.8 MB`, `kspcam` `:2028` (`2/2` snapshots OK), `inut-bida-agent` `active`.
- **`inut_202_218` (`77.88.202.218` — PFA T-Tennis & Pickleball, `ke=pfareal`, 2 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM giảm từ `537M` xuống **`206M/999M`** (giải phóng `331 MB`), `go-shinobi-ksp` `~10.4 MB` (`2/2` Recording), `redbida-go` `~5.5 MB`, `kspcam` `:2028` (`1/1` cam online snapshot OK), `inut-bida-agent` `active`.
- **`inut_202_219` (`77.88.202.219` — CLB Pickleball 26gram - CN2, `ke=vnvarISdog`, 3 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM giảm từ `574M` xuống **`242M/999M`** (giải phóng `332 MB`), `go-shinobi-ksp` `~16.9 MB` (`3/3` Recording), `redbida-go` `~11.8 MB`, `kspcam` `:2028` (`3/3` snapshots OK), `inut-bida-agent` `active`.
- **`inut_202_222` (`77.88.202.222` — Fox Billiards & Coffee Hội An, `ke=chiubia`, 4 cam — Node Đầu Tiên Chạy `Zero-CPU NVR-Direct Bridge` Không Thẻ Nhớ)**:
  - **Ngày lên Full Go + NVR Fallback Bridge:** `2026-11-08` • `linux/armv7` (`1 GB RAM`, không gắn thẻ nhớ `/media/usb1`) • Liên kết đầu ghi Dahua `DHI-NVR1108HS-S3/H-VN` (`192.168.1.243:37777` kênh `1, 2, 6, 8`) • Đầy đủ `866+` clip 5 phút 2K (`2560x1440` `hvc1` + `aac`), CheckVAR `1.03s`, mở clip 5 phút `13.45s` / tua Slow-Mo `13.01ms` từ `/dev/shm`, cắt clip 90s vắt 2 khung trong `6.02s` (`0% CPU encode`, `loadavg = 0.58`).
- **`inut_203_81` (`77.88.203.81` — Dinker Pickleball Club, `ke=dinker`, 6 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/armv7` • RAM giảm từ `643M` xuống **`261M/999M`** (giải phóng `382 MB`), `go-shinobi-ksp` `~9.3 MB` (`6/6` Recording), `redbida-go` `~6.6 MB`, `kspcam` `:2028` (`5/5` cam online snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_22` (`77.88.204.22` — VSP Pickleball, `ke=lAxKsdIBux`, 14 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1372M` xuống **`713M/3974M`** (giải phóng `659 MB`), `go-shinobi-ksp` `~19.9 MB` (`14/14` Recording), `redbida-go` `~13.5 MB`, `kspcam` `:2028` (`13/13` cam online snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_107` (`77.88.204.107` — Trang Hoàng Pickleball, `ke=cam4107`, 16 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1161M` xuống **`547M/1980M`** (giải phóng `614 MB`), `go-shinobi-ksp` `~18.7 MB` (`16/16` Recording), `redbida-go` `~13.3 MB`, `kspcam` `:2028` (`16/16` snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_135` (`77.88.204.135` — HTP Pickleball, `ke=b389`, 6 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `955M` xuống **`539M/1923M`** (giải phóng `416 MB`), `go-shinobi-ksp` `~18.7 MB` (`6/6` Recording), `redbida-go` `~13.9 MB`, `kspcam` `:2028` (`6/6` snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_157` (`77.88.204.157` — Kely Pickleball, `ke=kely`, 13 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `937M` xuống **`512M/1794M`** (giải phóng `425 MB`), `go-shinobi-ksp` `~20.2 MB` (`12/13` Recording), `redbida-go` `~29.5 MB`, `kspcam` `:2028` (`12/12` cam online snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_158` (`77.88.204.158` — Sân Pickleball BRG, `ke=brg`, 12 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1081M` xuống **`538M/1794M`** (giải phóng `543 MB`), `go-shinobi-ksp` `~17.5 MB` (`12/12` Recording), `redbida-go` `~13.2 MB`, `kspcam` `:2028` (`12/12` snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_160` (`77.88.204.160` — QA Pickleball, `ke=qa`, 3 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `822M` xuống **`408M/1794M`** (giải phóng `414 MB`), `go-shinobi-ksp` `~17.9 MB` (`3/3` Recording), `redbida-go` `~26.5 MB`, `kspcam` `:2028` (`3/3` snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_165` (`77.88.204.165` — Nguyễn Huệ Pickleball, `ke=KT8zqe2yGq`, 4 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `840M` xuống **`429M/1794M`** (giải phóng `411 MB`), `go-shinobi-ksp` `~17.4 MB` (`4/4` Recording), `redbida-go` `~28.2 MB`, `kspcam` `:2028` (`4/4` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_8` (`77.88.205.8` — PICKO 247, `ke=picko247`, 6 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `794M` xuống **`371M/1923M`** (giải phóng `423 MB`), `go-shinobi-ksp` `~19.3 MB` (`6/6` Recording), `redbida-go` `~29.0 MB`, `kspcam` `:2028` (`6/6` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_12` (`77.88.205.12` — Hoàng Tuấn Pickleball, `ke=hoangtuan`, 5 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1099M` xuống **`641M/1923M`** (giải phóng `458 MB`), `go-shinobi-ksp` `~18.4 MB` (`5/5` Recording), `redbida-go` `~28.0 MB`, `kspcam` `:2028` (`5/5` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_27` (`77.88.205.27` — MatchUp Sports Complex, `ke=thucc`, 6 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1047M` xuống **`520M/1923M`** (giải phóng `527 MB`), `go-shinobi-ksp` `~18.6 MB` (`6/6` Recording), `redbida-go` `~28.0 MB`, `kspcam` `:2028` (`6/6` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_5` (`77.88.205.5` / `5.5` — Tok Tok Billiards Center, `ke=toktok`, 18 cam, có `banner_top` popup)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1201M` xuống **`484M/1923M`** (giải phóng `717 MB`), `go-shinobi-ksp` `~22.3 MB` (`18/18` Recording), `redbida-go` `~12.9 MB` (đầy đủ `banner_top` popup + click sang Facebook + upload ảnh banner trong `#!/22`), `ksp-go2rtc` `~14.9 MB`, `kspcam` `:2028` (`18/18` snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_97` (`77.88.204.97` / `4.97` — Tok Tok II Billiards Center, `ke=toktok2`, 16 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1218M` xuống **`546M/1980M`** (giải phóng `672 MB`), `go-shinobi-ksp` `~20.7 MB` (`16/16` Recording), `redbida-go` `~29.3 MB`, `ksp-go2rtc` `~11.7 MB`, `kspcam` `:2028` (`16/16` snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_146` (`77.88.204.146` / `4.146` — Sniper Billiards Club, `ke=sniper`, 14 cam qua NVR `192.168.1.253`)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1460M` xuống **`912M/3919M`** (giải phóng `548 MB`), `go-shinobi-ksp` `~17.7 MB` (`14/14` Recording, bảo toàn `1,410` clip `.mp4`), `redbida-go` `~13.3 MB`, `ksp-go2rtc` `~15.0 MB`, `kspcam` `:2028` (`1/1` NVR snapshot OK), `inut-bida-agent` `active`.
- **`inut_205_47` (`77.88.205.47` / `5.47` — Aileex Billiards Club Yên Bái, `ke=alieex`, 13 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `983M` xuống **`510M/1923M`** (giải phóng `473 MB`), `go-shinobi-ksp` `~18.3 MB` (`13/13` Recording), `redbida-go` `~14.0 MB`, `ksp-go2rtc` `~15.6 MB`, `kspcam` `:2028` (`13/13` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_6` (`77.88.205.6` — TheLight Pool Arena, `ke=thelightpoolarena`, 10 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `703M` xuống **`388M/1923M`** (giải phóng `315 MB`), `go-shinobi-ksp` `~20.5 MB` (`10/10` Recording), `redbida-go` `~12.5 MB`, `ksp-go2rtc` `~16.0 MB`, `kspcam` `:2028` (`10/10` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_7` (`77.88.205.7` — LoGi Billiards, `ke=logi`, 10 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1004M` xuống **`391M/1923M`** (giải phóng `613 MB`), `go-shinobi-ksp` `~19.0 MB` (`10/10` Recording), `redbida-go` `~12.5 MB`, `ksp-go2rtc` `~15.6 MB`, `kspcam` `:2028` (`10/10` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_10` (`77.88.205.10` — CLB Billiard The King, `ke=xinchao`, 8 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `729M` xuống **`388M/1923M`** (giải phóng `341 MB`), `go-shinobi-ksp` `~20.0 MB` (`8/8` Recording), `redbida-go` `~27.4 MB`, `ksp-go2rtc` `~16.0 MB`, `kspcam` `:2028` (`8/8` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_14` (`77.88.205.14` — ATD Billiards Club, `ke=h2`, 12 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1222M` xuống **`513M/1923M`** (giải phóng `709 MB`), `go-shinobi-ksp` `~19.7 MB` (`7/7` active Recording, `1` Stopped, `4` cam tắt điện LAN), `redbida-go` `~27.9 MB`, `ksp-go2rtc` `~15.9 MB`, `kspcam` `:2028` (`8/8` cam online snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_17` (`77.88.205.17` — Bi-a AnhEm, `ke=anhem`, 8 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `951M` xuống **`471M/1923M`** (giải phóng `480 MB`), `go-shinobi-ksp` `~22.0 MB` (`8/8` Recording), `redbida-go` `~27.2 MB`, `ksp-go2rtc` `~15.8 MB`, `kspcam` `:2028` (`8/8` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_18` (`77.88.205.18` — MAY Billiards, `ke=b52cs32`, 8 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `729M` xuống **`398M/1923M`** (giải phóng `331 MB`), `go-shinobi-ksp` `~26.9 MB` (`7/7` cam online Recording, `1` cam `.242` tắt điện LAN), `redbida-go` `~12.4 MB`, `ksp-go2rtc` `~16.0 MB`, `kspcam` `:2028` (`7/7` cam online snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_19` (`77.88.205.19` — Billiards Quang Phúc, `ke=qp`, 10 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `933M` xuống **`417M/1923M`** (giải phóng `516 MB`), `go-shinobi-ksp` `~18.7 MB` (`6/6` active Recording, `4` Stopped), `redbida-go` `~29.4 MB`, `ksp-go2rtc` `~15.8 MB`, `kspcam` `:2028` (`6/6` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_43` (`77.88.205.43` — Konoha Premium, `ke=konoha`, 13 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1284M` xuống **`700M/1923M`** (giải phóng `584 MB`), `go-shinobi-ksp` `~17.5 MB` (`12/12` active Recording, `1` Stopped), `redbida-go` `~27.9 MB`, `ksp-go2rtc` `~15.8 MB`, `kspcam` `:2028` (`13/13` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_45` (`77.88.205.45` — Bida Hoàng Phát 6, `ke=thua`, 9 cam `DH-H2AE`)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1271M` xuống **`600M/1923M`** (giải phóng `671 MB`), dọn ổ `/media/usb1` từ `97%` về `84%`, giải phóng zombie RTSP session trên `DH-H2AE`, `go-shinobi-ksp` `~19.6 MB` (`9/9` Recording), `redbida-go` `~29.0 MB`, `ksp-go2rtc` `~16.1 MB`, `kspcam` `:2028` (`9/9` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_51` (`77.88.205.51` — Dragon Billiards Club, `ke=thin`, 10 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1197M` xuống **`535M/1923M`** (giải phóng `662 MB`), `go-shinobi-ksp` `~20.5 MB` (`10/10` Recording), `redbida-go` `~27.1 MB`, `ksp-go2rtc` `~15.9 MB`, `kspcam` `:2028` (`10/10` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_53` (`77.88.205.53` — The Best Billiards Club, `ke=ultrapool1`, 12 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `848M` xuống **`474M/1923M`** (giải phóng `374 MB`), `go-shinobi-ksp` `~17.7 MB` (`12/12` Recording), `redbida-go` `~14.4 MB`, `ksp-go2rtc` `~16.0 MB`, `kspcam` `:2028` (`11/12` snapshots OK — `1` cam CareCam RTSP-only), `inut-bida-agent` `active`.
- **`inut_205_55` (`77.88.205.55` — Tuyết Linh Billiards Club, `ke=tuyetlinh`, 11 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `763M` xuống **`421M/1923M`** (giải phóng `342 MB`), `go-shinobi-ksp` `~19.5 MB` (`11/11` Recording), `redbida-go` `~27.3 MB`, `ksp-go2rtc` `~15.8 MB`, `kspcam` `:2028` (`11/11` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_56` (`77.88.205.56` — King Kong Thái Nguyên, `ke=mp4`, 9 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `779M` xuống **`392M/1923M`** (giải phóng `387 MB`), `go-shinobi-ksp` `~19.9 MB` (`9/9` Recording), `redbida-go` `~29.6 MB`, `ksp-go2rtc` `~15.8 MB`, `kspcam` `:2028` (`9/9` snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_82` (`77.88.205.82` — Hoàng Tom Billiards, `ke=mia`, 16 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `1205M` xuống **`632M/1923M`** (giải phóng `573 MB`), `go-shinobi-ksp` `~20.5 MB` (`16/16` Recording), `redbida-go` `~30.9 MB`, `ksp-go2rtc` `~16.1 MB`, `kspcam` `:2028` (`30/30` kênh snapshots OK), `inut-bida-agent` `active`.
- **`inut_205_121` (`77.88.205.121` — Black Coffee Billiards, `ke=loicafe`, 6 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `935M` xuống **`391M/1923M`** (giải phóng `544 MB`), `go-shinobi-ksp` `~18.9 MB` (`6/6` Recording), `redbida-go` `~29.3 MB`, `ksp-go2rtc` `~15.8 MB`, `kspcam` `:2028` (`6/6` snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_144` (`77.88.204.144` / `4.144` — Lio Kig Billiads Club, `ke=replaceemmc`, 8 cam qua NVR `192.168.1.200`)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `962M` xuống **`502M/1923M`** (giải phóng `460 MB`), `go-shinobi-ksp` `~20.1 MB` (`8/8` Recording, bảo toàn `1,239` clip `.mp4`), `redbida-go` `~28.6 MB`, `ksp-go2rtc` `~15.6 MB`, `kspcam` `:2028` (`8/8` snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_34` (`77.88.204.34` / `4.34` — Billiards Guu Pool Arena, `ke=bida2391`, 16 cam)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` (RK35xx kernel `6.1.63`, thẻ `239 GB` F2FS `noatime,users`) • RAM giảm mạnh từ `1579M` xuống **`674M/3974M`** (giải phóng **`905 MB`** RAM), `go-shinobi-ksp` `~19.9 MB` (`16/16` Recording, bảo toàn `3,278` clip `.mp4`), `redbida-go` `~27.7 MB`, `ksp-go2rtc` `~15.4 MB`, `kspcam` `:2028` (`16/16` snapshots OK), `inut-bida-agent` `active`.
- **`inut_204_133` (`77.88.204.133` / `4.133` — Zone+ Thái Nguyên, `ke=zoneplus1`, 10 cam Imou `IPC-A32EP-L-3`)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm từ `838M` (Swap `512M`) xuống **`580M/1923M` (Swap `59M`)** (giải phóng `258 MB` RAM + `453 MB` Swap), `go-shinobi-ksp` `~19.8 MB` (`10/10` Recording), `redbida-go` `~29.2 MB`, `ksp-go2rtc` `~15.8 MB` (`200 OK`), `kspcam` `:2028` (`10/10` snapshots OK + cầu nối Hybrid `[Cam-SD]` bật sẵn `10/10` bàn; hiện khe thẻ microSD trên 10 cam đang để trống), `inut-bida-agent` `active`.
- **`inut_204_137` (`77.88.204.137` / `4.137` — Zone+ Thái Nguyên, `ke=zoneplus2`, 9 cam Imou `IPC-A32EP-L-3`)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` • RAM giảm mạnh từ `1129M` xuống **`547M/1923M`** (giải phóng **`582 MB`** RAM), `go-shinobi-ksp` `~19.8 MB` (`9/9` Recording), `redbida-go` `~14.1 MB`, `ksp-go2rtc` `~15.9 MB` (`200 OK`), `kspcam` `:2028` (`9/9` snapshots OK + cầu nối Hybrid `[Cam-SD]` bật sẵn `9/9` bàn; hiện khe thẻ microSD trên 9 cam đang để trống), `inut-bida-agent` `active`.
- **`inut_205_57` (`77.88.205.57` / `5.57` — Zone+ Thái Nguyên, `ke=moi`, 12 cam Imou `IPC-A32E-L`)**:
  - **Ngày lên Full Go:** `2026-11-08` • `linux/arm64` (đã gỡ bỏ & khóa `netwatch.service` + `kernel.sysrq=0`) • RAM giảm từ `898M` (Swap `587M`) xuống **`480M/1923M` (Swap `79M`)** (giải phóng **`418 MB`** RAM + **`508 MB`** Swap), `go-shinobi-ksp` `~17.8 MB` (`12/12` Recording), `redbida-go` `~27.1 MB`, `ksp-go2rtc` `~16.1 MB` (`200 OK`), `kspcam` `:2028` (`12/12` snapshots OK + cầu nối Hybrid `[Cam-SD]` bật sẵn `12/12` bàn; hiện khe thẻ microSD trên 12 cam đang để trống), `inut-bida-agent` `active`.
- **`inut_205_77` (`77.88.205.77` / `5.77` — ZONE+ Billiards Center, `ke=toktok3`, 12 cam — **Node Đầu Tiên Chạy Thực Chiến `Hybrid Box-SD + [Cam-SD] 72h`**)**:
  - **Ngày lên Full Go + Hybrid Box-SD↔Cam-SD:** `2026-11-08` • `linux/arm64` • Load giảm mạnh từ `7.57` xuống **`2.58`**, Swap giảm từ `452M` xuống **`98M`**, `max_storage_percent: 85` (`target: 80`) • Kết hợp thẻ nhớ Box 60GB (`~69` clip gần nhất `~4.8 giờ`, `0` tải lên cam) + thẻ nhớ 64GB gắn trong từng mắt cam (`+627..1,095` clip quá khứ) thành **`697–1,165` clip/bàn liên mạch trọn `72 giờ` (`14x` dung lượng thẻ Box)** • Cắt clip `70s` từ `2 ngày trước` trên `[Cam-SD]` qua `StreamTSFast` (`Rate-Control: no`) xong trong **`32.0s`** (`15.2 MB`, `moov=True`), tua Slow-Mo **`25.94 ms` (`HTTP 206`)**, `12/12` Recording & `11/11` snapshots `:2028` OK.
---

## 2. Lệnh Ansible Trên `172.16.5.180` (`/build/armbian-build/ansible`)

```bash
# Triển khai / cập nhật Full Go (go-shinobi-ksp :8080 + redbida-go :2023):
make go-shinobi inut_205_67
make redbida-go inut_205_67

# Rollback tức thì về Node-RED (:2023) hoặc Shinobi Node.js (:8080):
make redbida-rollback inut_205_67
make go-shinobi-rollback inut_205_67
```

---

## 3. Ghi Chú & Checklist Chủ Động Kiểm Thử Từng Tính Năng (`redbida-go` & `go-shinobi-ksp`)

1. **Cấu trúc MQTT `/data/alarms` & Hiển thị trên `vnmap.inut.vn/device/86`**:
   - **Chuẩn gói tin `/data/alarms`**:
     - `usb`: **Bắt buộc là mảng JSON `[]`** (`[]string`), tuyệt đối không gửi `{}` hoặc `null` vì trang cũ `vnmap.inut.vn/device/86` (`Playing.js`) gọi `_usbRecords.filter(u => u !== 'ESP')` (nếu `usb` là `{}` sẽ văng `TypeError: f.filter is not a function`).
     - `p2p`: Object ánh xạ từng proxy trong `/tmp/frpc.ini` sang `{"status": "start proxy success", "datetime": "YYYY/MM/DD HH:MM:SS"}` khi `len(frpc_config) > 32`.
     - `info`: Mọi giá trị là chuỗi không có `\n` thừa ở cuối, có đầy đủ tiền tố nhãn (`"Camera uptime: <etime>"`, `"P2P uptime: <etime>"`, `"Uptime go2rtc_linux: <etime>"`, `"API req: <model>/<api>"`, `"View: <packed>/<download>/<view>"`, `"<iface>  (U↑): <tx> Mbps  (D↓): <rx> Mbps"`, và danh sách thiết bị kết nối `"<id>|<model>|<manufacturer>|DD/MM/YYYY HH:MM:SS"` múi giờ `GMT+7`).
   - **Cách test chủ động**: Truy cập `https://vnmap.inut.vn/device/86`, gõ mã node (`5.98`, `5.67`, `6.14`, `6.10`) vào ô `Nhập tên hoặc mã (MQTT) của cụm loa`, bấm `TÌM KIẾM` $\to$ xác nhận `0 pageerror`, hiển thị nút `[P2P]`, `[SỐ USB: 0]` và đầy đủ các dòng `- API req`, `- View`, `- P2P uptime`, `- Camera uptime`, tốc độ mạng và danh sách iPhone/Android.
2. **Thanh Toolbar & Menu Góc Trái (`?show_toolbar_open=1`)**:
   - **Mặc định (`/#!/0`)**: Ẩn hoàn toàn thanh `<md-toolbar id="mainToolbar">` (`display: none`) để giao diện 3 khối trên trang chủ gọn gàng, rõ ràng.
   - **Khi có `?show_toolbar_open=1` (`/#!/0?show_toolbar_open=1`)**: Hiển thị `#mainToolbar`; bấm nút `☰` chỉ liệt kê danh sách các bàn camera (`C01 - Bàn 1`, `C02 - Bàn 2`...), đã bỏ các mục hệ thống `20..23` khỏi menu góc và giữ nguyên `?show_toolbar_open=1` khi chuyển bàn.
3. **Chế Độ Phóng To, Tua Chậm (`0.05x..8x`) & Zoom/Pan iOS Safari (`#overlay`)**:
   - **Giao diện gọn & trong suốt**: Cụm nút điều khiển `.overlay-controls` và `.progress-container` dùng nền bán trong suốt `rgba(15, 23, 42, 0.38)` (`backdrop-filter: blur(4px)`), không che khuất bàn bida.
   - **Slow-Motion (`0.05x..8x`)**: Clamp `playbackRate` phần cứng về `[0.0625, 16]` (chống văng `DOMException` trên Chromium/WebKit khi bấm `0.05x`), tắt `preservesPitch`, tự động mute khi `< 0.5x` hoặc `> 2x`.
   - **Zoom/Pan iOS Safari (`#zoomStage`)**: Áp dụng `translate3d(...) scale(...)` lên thẻ bọc `#zoomStage` thay vì `<video>`, tự động neo lại `panStartX/Y` khi nhấc 1 ngón tay sau khi pinch 2 ngón (`0.0px` jump), khóa giới hạn kéo (`clampPan`), hỗ trợ hàng nút Zoom 1 ngón (`-`, `Zoom 1x`, `1.5x`, `2x`, `3x`, `4x`, `+`) và chạm đúp (`1x <-> 2.5x`).
   - **Phục vụ trực tiếp HTTP `206 Partial Content` (`/videos/:ke/:mid/:file`)**: Đọc trực tiếp từ `/media/usb1` qua `http.ServeContent` (`Accept-Ranges: bytes`), giúp các nút `-1s`, `-3s`, `-5s`, `+1s`, `+3s`, `+5s`, `+10s` phản hồi tức thì.
4. **Trang Cắt Clip (`/inut-billiard-live-cong-ty-inut-jsc/:id`) & Pro Player Highlight QR (`?from_app=1`)**:
   - **Các nút lùi thời gian (`5 phút trước` .. `4 giờ trước`) + nút `Mới nhất`**: Mỗi lần bấm nút `5 phút trước`..`4 giờ trước`, cửa sổ chọn `[Xanh, Vàng]` lùi tiếp về trước đúng bấy nhiêu phút tính từ mốc đang chọn (cộng dồn liên tiếp); bấm `Mới nhất` nhảy ngay về đoạn 3 phút mới nhất ở mép phải.
   - **Chế độ Tablet (`?from_app=1`)**: Sau khi cắt xong, bung thẻ `PRO PLAYER HIGHLIGHT` kèm ảnh cầu thủ `/pro-player-qr.png`, hiển thị rõ khung giờ `HH:MM:SS -> HH:MM:SS` (`MM:SS`) và mã QR viền vàng cho khách quét tải về điện thoại.
5. **Ghi Hình Liên Tục 5 Phút/Clip (`300s`) Trên `go-shinobi-ksp` (`:8080`)**:
   - **Bắt buộc**: Chạy `/usr/bin/ffmpeg` với `-tag:v hvc1 -segment_format_options moov_size=262144:movflags=+faststart -f segment -segment_atclocktime 1 -reset_timestamps 1 -strftime 1 -segment_list pipe:1 -segment_list_type flat -segment_time 300`.
   - **Không tự cắt ngắn clip**: `Engine.Stop()` và `shinobi-camera-watchdog.sh` tuyệt đối không chạy `date -s` khi dừng/khởi động lại (`date -s` chỉ chạy khi người dùng bấm `CheckVAR` `/new-check-var`).
6. **Khóa Bảo Vệ Ansible (`make bida-update` & `make shinobi-update` trên `172.16.5.180`)**:
   - Các playbook `app_bida_update`, `bida-prism-health.yml`, và `app_shinobi_update` tự động kiểm tra `/opt/redbida-go/redbida-go` và `/opt/go-shinobi-ksp/go-shinobi-ksp`; nếu đã lên Go sẽ tự động bỏ qua (`meta: end_host`, `changed=0`).
