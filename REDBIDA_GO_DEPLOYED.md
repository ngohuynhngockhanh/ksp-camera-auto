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
  - **Ngày lên Full Go:** `2026-11-07`
  - **Kiến trúc:** `linux/arm64` (`aarch64`), chạy song song `go-shinobi-ksp` (`:8080`, 11 camera NVR) + `redbida-go` (`:2023`) + `kspcam` (`:2028`) + `inut-bida-agent` + `ota-mqtt` (`:12369`).
  - **RAM & Swap tiết kiệm:** Tổng RAM hệ thống giảm **`-560 MB`** (`1154 MB -> 594 MB`), Swap giảm **`-636 MB`** (`832 MB -> 196 MB`); PM2 `inut` giảm từ `192.3 MB` xuống **`~27 MB`** (`redbida-go`).
  - **Trạng thái:** `ONLINE` — hoạt động đầy đủ `/#!/0..30`, `/api/v1/:id`, `/inut-billiard-live-cong-ty-inut-jsc/:id`, `/hls/...`, `/videos/...`, MQTT `12369` (`/data/alarms` chuẩn 1:1).

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
