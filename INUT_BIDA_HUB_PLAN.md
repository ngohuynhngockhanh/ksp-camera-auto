# KẾ HOẠCH KIẾN TRÚC TOÀN DIỆN & LỘ TRÌNH XÂY DỰNG `inut-bida-hub` TRÊN `root@77.88.208.17`
> **Tài liệu Thiết kế Kiến trúc Chuẩn (Master Architecture & Execution Tree)**  
> **Đối tượng đánh giá gốc**: `/home/ksp/ksp-camera-auto` (`kspcam` — cổng mặc định `:2028`)  
> **Đối tượng tham chiếu hạ tầng (Read-Only)**: `inut-point-hub` (`:2088`), `inut-solar-hub` (`:2089`), `inut-p2p-hub` (`:31797/:31798`) trên máy chủ trung tâm `root@77.88.208.17` (`inut01`)  
> **Mục tiêu**: Đánh giá toàn diện UI/UX và mức độ hoàn thiện của `kspcam`, chỉ rõ 16 điểm nghẽn kiến trúc & giao diện khiến `kspcam` chưa đạt mục tiêu làm Central Fleet Hub, và lập **Cây Phương Án Toàn Diện (Master Tree)** để triển khai `inut-bida-hub` bằng Go + SQLite WAL + FRPC tại `root@77.88.208.17:2090`.

---

## PHẦN 1: ĐÁNH GIÁ TOÀN DIỆN UI/UX & MỨC ĐỘ HOÀN THIỆN CỦA `ksp-camera-auto` (`kspcam`)

### 1.1. Tổng quan & Bảng chấm điểm 15 Module/Tab hiện hữu

`ksp-camera-auto` (`cmd/kspcam/main.go:1-165`, cấu hình port mặc định `:2028` tại `internal/config/config.go:171`) được thiết kế ban đầu như một **tiện ích Edge cục bộ (On-Box Camera & RedBida Utility)** chạy trực tiếp bên trong từng Box Bida/Pickleball (Allwinner H313/H616, Rockchip RK3528/RK3328, Amlogic S912).

| Tiêu chí đánh giá | Điểm số | Mức độ hoàn thiện | Kết luận tổng quát |
| :--- | :---: | :---: | :--- |
| **Vai trò Công cụ Cạnh Đơn lẻ (Single-Node Edge Tool trên 1 Box)** | **8.4 / 10** | **84% (Cao)** | Rất mạnh về điều khiển camera nội bộ (Dahua DVRIP, Hikvision ISAPI, Tiandy RTSP/ONVIF), tua/cắt video 72h `vis-timeline` xuất QR MP4, giám sát lưu lượng `AF_PACKET` `iftop` theo luồng, Anti-A Guardian giữ chuẩn H.265, và giao diện Glassmorphism RedBida OTA-MQTT 1-Click. |
| **Vai trò Trung tâm Điều hành Đội Box (`inut-bida-hub` tại `77.88.208.17`)** | **1.8 / 10** | **18% (Chưa đạt)** | Kiến trúc khóa cứng giả định **1 tiến trình = 1 Box**. Phụ thuộc vào 1 file `cameras.yaml` duy nhất, 1 Shinobi `127.0.0.1:8080`, 1 MQTT broker `127.0.0.1:12369`, 1 thư mục `/root/ota-mqtt/change_ok` nội bộ và card mạng L2 cục bộ. Hoàn toàn chưa có quản lý đội thiết bị (Fleet Matrix 30+ box), chưa có Reverse Proxy `/proxy/{nodeId}/`, chưa có cổng khách hàng `/portal/{token}` bền vững, và mù thông số Golden Hardening 8 bước. |

#### Bảng chấm điểm chi tiết 15 Module / Tab giao diện của `kspcam`

| # | Module / Tab (`data-view`) | File Nguồn & Dòng Code Chính Xác | DOM Selectors & Hàm JS Chính | API Routes & Go Handlers | Điểm Edge (1 Box) | Điểm Fleet Hub (`inut-bida-hub`) |
| :- | :--- | :--- | :--- | :--- | :---: | :---: |
| **0** | **Login & Session Gate** (`login.html`) | `web/static/login.html:1-118`<br>`internal/server/server.go:130, 211-251, 355-434` | `form.card[action="/login"]`, `#username`, `#password`, `#theme-toggle`, `#err` | `GET/POST /login` (`handleLogin`), `GET /logout` (`handleLogout`), cookie `kspcam_session` | **8.5 / 10** | **2.5 / 10** |
| **1** | **App Shell, Topbar & Responsive Nav** | `web/static/index.html:18-48, 1384-1391`<br>`web/static/app.js:6-36, 179-310`<br>`web/static/ui-core.js:1-196`<br>`web/static/style.css:154-205, 922-960` | `#sidebar`, `#sidebar-nav`, `.topbar`, `#settings-toggle`, `#g-timeout`, `#bottomnav`, `#drawer`, `#toast-container` | `GET /api/config` (`handleConfig` tại `internal/server/api.go:2257-2266`) | **8.5 / 10** | **3.0 / 10** |
| **2** | **Dashboard (`Tổng quan`)** (`#dashboard`) | `web/static/index.html:49-65`<br>`web/static/app.js:312-331`<br>`web/static/style.css:1032-1040` | `#view-dashboard`, `#stat-total`, `#stat-dahua`, `#stat-hik`, `#stat-lastrun`, `#dash-recent` | Tổng hợp phía client từ mảng `cameras` (`GET /api/cameras` tại `app.js:122`) | **5.5 / 10** | **1.0 / 10** |
| **3** | **Quét mạng LAN (`#scan`)** | `web/static/index.html:68-107`<br>`web/static/app.js:3179-3416`<br>`internal/server/api_scan.go:19-145` | `#view-scan`, `#scan-lan-btn`, `#scan-subnet`, `#scan-nmap-btn`, `#scan-table`, `#scan-try-btn`, `#scan-try-progress` | `POST /api/scan` (`handleScan`), `POST /api/scan/try-password` (`handleTryPassword` SSE) | **8.8 / 10** | **2.0 / 10** |
| **4a** | **Kho Camera — Danh sách (`#cameras/list`)** | `web/static/index.html:110-124, 267-318, 1453-1504`<br>`web/static/app.js:335-566, 774-960`<br>`web/static/style.css:1804-1997` | `#view-cameras`, `#cam-table`, `#cam-tbody`, `#cam-grid`, `.cam-card`, `#camera-search`, `#camera-vendor-filter`, `#camera-form-dialog` | `GET/POST /api/cameras` (`handleCameras`), `POST /api/cameras/delete`, `POST /api/cameras/delete-bulk`, `POST /api/probe`, `POST /api/channel-names` | **9.0 / 10** | **2.5 / 10** |
| **4b** | **Chi tiết Camera & Quick PTZ (`#cameras/cam/<id>/<tab>`)** | `web/static/index.html:126-247, 1506-1561`<br>`web/static/app.js:1079-1519, 1891-2960`<br>`web/static/style.css:1696-1725, 2029-2055` | `#camera-detail`, `#cd-live`, `#ce-tabs`, `#ce-panel-osd`, `#ce-panel-picture`, `#ce-panel-video`, `#ce-panel-audio`, `#ce-panel-network`, `#ce-panel-ptz`, `#ce-panel-maint`, `#quick-ptz-dialog` | `GET /api/live`, `GET /api/snapshot`, `GET /api/channel-info`, `POST /api/channel-name`, `POST /api/osd`, `GET/POST /api/picture`, `GET/POST /api/network`, `GET/POST /api/wifi`, `POST /api/wifi-scan`, `POST /api/ptz`, `POST /api/reboot`, `GET/POST /api/storage`, `GET/POST /api/device-time`, `GET/POST /api/autoreboot` | **9.2 / 10** | **2.5 / 10** |
| **4c** | **Cấu hình Hàng loạt & Đổi Mật khẩu (`#cameras/bulk`)** | `web/static/index.html:320-480`<br>`web/static/app.js:1580-1755, 2962-3177`<br>`internal/server/api.go:335-558, 1742-1792` | `[data-camera-panel="bulk"]`, `#bulk-golden-template-btn`, `#bulk-camera-picker`, `#bulk-summary-chips`, `#bulk-safety-alert`, `#apply-btn`, `#pw-btn`, `#pw-restore-btn` | `POST /api/apply` (`handleApply` SSE), `POST /api/password` (`handlePassword` SSE), `GET /api/password/backup`, `POST /api/password/restore` | **8.5 / 10** | **2.0 / 10** |
| **4d** | **Đầu ghi NVR Dự phòng & Watchdog (`#cameras/nvr`)** | `web/static/index.html:249-265, 1395-1425`<br>`web/static/app.js:583-772`<br>`internal/server/api.go:832-1044`<br>`internal/server/nvr_health.go:19-456` | `[data-camera-panel="nvr"]`, `#nvr-list-table`, `#nvr-open-btn`, `#nvr-link-dialog`, `[data-nvr-watchdog]`, `[data-nvr-sync]` | `POST /api/nvr/scan`, `POST /api/nvr/link`, `GET /api/nvr/channels`, `GET /api/nvr/health`, `POST /api/nvr/health/check`, `POST /api/nvr/watchdog` | **8.6 / 10** | **2.0 / 10** |
| **4e** | **Anti-A Guardian (`#cameras/anti-a`)** | `web/static/index.html:496-590`<br>`web/static/app.js:3769-3938`<br>`internal/server/api_anti_a.go:8-82`<br>`internal/server/anti_a.go:15-526` | `#camera-panel-anti-a`, `#anti-a-status-badge`, `#anti-a-enabled-toggle`, `#anti-a-interval-input`, `#anti-a-mode-select`, `#anti-a-trigger-btn`, `#anti-a-logs-tbody` | `GET/POST /api/anti-a` (`handleAntiA`), `POST /api/anti-a/trigger` (`handleAntiATrigger`) | **8.5 / 10** | **2.0 / 10** |
| **4f** | **Lưu lượng Mạng `iftop` (`#cameras/traffic`)** | `web/static/index.html:593-678`<br>`web/static/app.js:3961-4226`<br>`internal/server/api_traffic.go:12-123`<br>`internal/traffic/traffic.go:14-432`<br>`internal/traffic/sniffer_linux.go:16-143` | `#camera-panel-traffic`, `#traffic-toggle-btn`, `#traffic-iface-select`, `#traffic-search-input`, `#traffic-only-cams`, `#traffic-tbody` | `GET /api/network/traffic/interfaces`, `GET /api/network/traffic/snapshot`, `GET /api/network/traffic/stream` (SSE 1s) | **8.8 / 10** | **1.5 / 10** |
| **4g** | **Kết quả Tác vụ Hàng loạt (`#cameras/results`)** | `web/static/index.html:482-494`<br>`web/static/app.js:46-119, 2971-2995`<br>`web/static/style.css:1544-1565` | `[data-camera-panel="results"]`, `#apply-progress`, `#apply-log`, `#result-table`, `#result-tbody` | Nhận luồng SSE từ `POST /api/apply` và `POST /api/password` (`streamPost` tại `app.js:46-119`) | **7.5 / 10** | **1.0 / 10** |
| **5** | **Xem lại Video Timeline (`#review`)** | `web/static/index.html:681-747`<br>`web/static/review.js:1-583`<br>`internal/server/api.go:1966-2255`<br>`web/static/style.css:1625-1694` | `#view-review`, `#rv-cam`, `#rv-video`, `#rv-timeline` (`vis.Timeline`), `#rv-split5-btn`, `#rv-download`, `#rv-download-dav`, `#rv-qr`, `#rv-qr-modal` | `GET /api/recordings`, `GET /api/playback` (`format=fastmp4`, `format=native`), `GET /api/export-progress`, `GET /api/playback-token` | **9.0 / 10** | **2.5 / 10** |
| **6** | **RedBida / OTA-MQTT Console (`#redbida`)** | `web/static/index.html:750-1143`<br>`web/static/redbida.js:1-1636`<br>`internal/server/api_redbida.go:10-106`<br>`internal/redbida/catalog.go:1-297`<br>`internal/redbida/service.go:1-447`<br>`web/static/style.css:207-920` | `#view-redbida`, `#redbida-inspector-panel`, `#redbida-preset-panel`, `#redbida-20tab-panel`, `#redbida-knowledge-hub`, `#redbida-table`, `#redbida-floating-bar` | `GET /api/redbida/catalog`, `POST /api/redbida/refresh`, `POST /api/redbida/apply`, `GET /api/redbida/time-status`, `POST /api/redbida/upload-logo` | **9.2 / 10** | **2.0 / 10** |
| **7** | **Quản lý Shinobi NVR & Import (`#shinobi`, `#import`)** | `web/static/index.html:1146-1356`<br>`web/static/app.js:3418-3724`<br>`internal/server/api_shinobi.go:14-215`<br>`internal/shinobi/sync.go:1-378`<br>`internal/shinobi/client.go:1-412` | `#view-shinobi`, `#shinobi-stat-badge`, `#shinobi-sync-to-btn`, `#shinobi-sync-from-btn`, `#shinobi-table`, `#shinobi-edit-modal`, `#shinobi-videos-modal`, `#view-import` | `GET /api/shinobi/status`, `GET/POST /api/shinobi/monitors`, `POST /api/shinobi/sync-to-shinobi`, `POST /api/shinobi/sync-from-shinobi`, `GET /api/shinobi/videos`, `POST /api/import` | **7.2 / 10** | **1.5 / 10** |
| **8** | **Trợ giúp Offline (`#help`)** | `web/static/index.html:1359-1380`<br>`web/static/help.js:1-251`<br>`web/static/help/help-index.json` | `#view-help`, `#help-search`, `#help-results`, `#help-article`, `#chat-log`, `#chat-form` | Tải tĩnh `GET /help/help-index.json` (`loadDb()` tại `help.js:34-54`) | **8.0 / 10** | **3.0 / 10** |

---

### 1.2. Điểm mạnh cốt lõi cần kế thừa (Reuse) từ `ksp-camera-auto`

1. **Stack giao thức Camera thuần Go (`internal/camera/`, `internal/dahua/`, `internal/isapi/`, `internal/tiandy/`, `internal/discovery/`)**:
   - Đã xử lý trọn vẹn DVRIP port `37777` (Dahua), ISAPI HTTP Digest (Hikvision), ONVIF/RTSP (Tiandy), điều khiển OSD, VideoColor, VideoInOptions (kèm khóa an toàn `GetVideoInputCaps`), PTZ 8 hướng, định dạng thẻ SD/HDD và đồng bộ NTP. Cần giữ nguyên trên Edge (`kspcam` chạy tại từng Box `:2028`) và điều phối từ `inut-bida-hub` qua đường hầm.
2. **Giao diện Glassmorphism RedBida + 17-Rule Golden Standard Inspector + 20-Tab INI Editor (`web/static/redbida.js:196-369, 886-1255`, `web/static/style.css:207-920`)**:
   - Thiết kế thẻ kính mờ (`--glass-bg-card`, `backdrop-filter: blur(16px)`), bộ kiểm tra 17 quy tắc chuẩn Bida tự động sửa lỗi 1-Click (`redbidaAutoFixAll`), trình tạo Preset 8 dải màu kèm khung xem trước trực quan (`#redbida-preset-bg-preview`), và trình sửa 20 tab `[C01]..[C20]` là tài sản UI/UX cực kỳ giá trị cần port thẳng lên phân hệ RedBida Studio của `inut-bida-hub`.
3. **Timeline Video Review 72h + Cắt MP4 song song + Mã QR (`web/static/review.js:1-583`, `internal/server/api.go:1966-2255`)**:
   - Thanh thời gian `vis-timeline` với 3 con trỏ kéo thả (`playhead`, `cutStart`, `cutEnd`), chia nhanh 5 phút (`buildSplit5`), tải nhanh `&format=fastmp4` ghép nối song song các đoạn RTSP và tạo mã QR tải về điện thoại rất mượt cho chủ quán và cơ thủ.
4. **Anti-A Guardian & NVR Health Watchdog (`internal/server/anti_a.go:15-526`, `internal/server/nvr_health.go:19-456`)**:
   - Cơ chế tự động tuần tra chống phần mềm đối thủ (Arena) ép camera về H.264 và watchdog tự động kích lại kênh đầu ghi NVR bị kẹt cần được giữ ở Edge và đẩy báo cáo tập trung về `inut-bida-hub`.

---

### 1.3. Phân tích sâu 16 Điểm Nghẽn UI/UX & Kiến Trúc khiến `kspcam` chưa đạt mục tiêu `inut-bida-hub`

#### Nhóm A: Khóa cứng kiến trúc Đơn-Node (Single-Node Edge Coupling)

- **Điểm nghẽn #1 — Xung đột khóa chính `host:port` trong `cameras.yaml` đơn lẻ khi quản lý nhiều Box**:
  - **Dẫn chứng**: `internal/config/config.go:154-180` trỏ `CamerasFile` vào 1 file `"cameras.yaml"`. `internal/config/inventory.go:15-33, 36, 69` và `internal/server/api.go:220-226, 267-269` (`handleCamerasUpsert`) lấy `d.ID = fmt.Sprintf("%s:%d", d.Host, d.Port)` (ví dụ `192.168.1.10:37777`) làm khóa chính duy nhất.
  - **Hệ quả**: Hầu hết các CLB Bida trong hệ thống (`FLEET_WATCHLIST.md:57, 198, 275, 429`) đều dùng dải mạng LAN `192.168.1.0/24`. Trên hơn 30 Box (`inut_204_141`, `inut_205_72`, `inut_204_161`...), hàng chục camera cùng mang IP `192.168.1.10:37777` hoặc đầu ghi `192.168.1.240:37777`. Việc thiếu trường định danh `box_id` / `venue_id` khiến `kspcam` ghi đè dữ liệu lẫn nhau và máy chủ `77.88.208.17` cũng không thể quay số trực tiếp tới IP LAN RFC1918 của từng quán nếu không đi qua đại lý cạnh (Edge Agent).
- **Điểm nghẽn #2 — Khóa cứng 1 thực thể Shinobi NVR cục bộ (`127.0.0.1:8080`) và 1 Group Key duy nhất**:
  - **Dẫn chứng**: `internal/config/config.go:63-68, 182-184` cấu hình cứng `APIURL: "http://127.0.0.1:8080"`, 1 `APIKey` và 1 `GroupKey`. `internal/server/server.go:66-68` khởi tạo đúng 1 `s.shinobi` cố định; `web/static/index.html:1158-1180` & `web/static/app.js:3470-3530` chỉ hiển thị 1 Group Key.
  - **Hệ quả**: Mỗi Box Bida chạy một tiến trình Shinobi + MariaDB riêng với Group Key riêng (`mia`, `99cs111`, `kely`, `master2`, `axis`, `ux0pAUigSn`... theo `FLEET_WATCHLIST.md:55-643`). `kspcam` không thể kết nối hay chuyển đổi giữa hàng chục Shinobi NVR của toàn đội.
- **Điểm nghẽn #3 — Khóa cứng MQTT Broker cục bộ (`127.0.0.1:12369`) và đọc trực tiếp thư mục `/root/ota-mqtt/change_ok` trên máy chủ**:
  - **Dẫn chứng**: `internal/config/config.go:99-110, 189-199` đặt cứng `BrokerHost: "127.0.0.1"`, `BrokerPort: 12369`, `KeyDir: "/root/ota-mqtt/change_ok"`. `internal/redbida/catalog.go:92-122` gọi `os.ReadDir(c.keyDir)` trên ổ đĩa cục bộ. Thậm chí `web/static/index.html:783-786` và `web/static/redbida.js:520-523` gán cứng chuỗi `'127.0.0.1:12369'` trên giao diện.
  - **Hệ quả**: Trên máy chủ trung tâm `root@77.88.208.17`, thư mục `/root/ota-mqtt/change_ok` của 30+ Box từ xa không tồn tại cục bộ (`internal/redbida/service.go:47-50` báo lỗi `catalog source unavailable`).
- **Điểm nghẽn #4 — Sniffer `AF_PACKET` (`iftop`) và Quét mạng UDP Broadcast chỉ bắt gói tin trên card mạng nội bộ**:
  - **Dẫn chứng**: `internal/traffic/sniffer_linux.go:20-34` mở raw socket `syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(ethPIP))` trên card mạng của chính máy đang chạy; `internal/server/api_scan.go:49-60` phát gói tin broadcast L2/L3 trên card mạng nội bộ.
  - **Hệ quả**: Nếu chạy tại `77.88.208.17`, tính năng `iftop` và `Quét mạng` sẽ quét card mạng của VPS `77.88.208.17` chứ không thể bắt lưu lượng cổng `eth0`/`end0` tại quán (nơi xảy ra lỗi nghẽn 100Mbps làm rớt 440.000 gói tin như tại `inut_207_44` trong `FLEET_WATCHLIST.md:575-583`).

#### Nhóm B: Thiếu hoàn toàn Kiến trúc Quản lý Đội Thiết Bị (Fleet) & Đa Thuê Bao (Multi-Tenant)

- **Điểm nghẽn #5 — Không có Bảng điều khiển Đội Box (Fleet Matrix) và Không hỗ trợ Gom nhóm CLB Nhiều Box**:
  - **Dẫn chứng**: `web/static/app.js:23-32` (`NAV_ITEMS`) chỉ có các mục của 1 Box; `#dashboard` (`web/static/index.html:49-65`, `app.js:312-331`) chỉ có 4 thẻ đếm số camera tĩnh và biến RAM `lastRun` (`app.js:126`) bị xóa trắng mỗi khi bấm `F5`.
  - **Hệ quả**: Không thể quản lý 30+ Box trải dài trên các dải `77.88.202.x–207.x`, đặc biệt là các **CLB Bida quy mô lớn chạy nhiều Box song song** (như *99 Billiards CS11* gồm 3 Box `204.150`, `204.152`, `206.60`; *Magic Arena Đà Nẵng* gồm 3 Box `205.122`, `205.123`, `205.124`; *Top Billiards Thái Nguyên* gồm 2 Box `204.38`, `204.83` — `FLEET_WATCHLIST.md:115-190, 275-315, 460-520`).
- **Điểm nghẽn #6 — Thiếu Reverse Proxy Đường hầm (`/fleet/proxy/{boxId}/...`) và Bộ chuyển ngữ cảnh Box trên Topbar**:
  - **Dẫn chứng**: `internal/server/server.go:128-207` chỉ đăng ký route phẳng `/api/*`; `web/static/ui-core.js:183-196` (`api(path, opts)`) gọi thẳng `/api/...` không có tiền tố trạm.
  - **Hệ quả**: Kỹ thuật viên không thể bấm 1 nút từ Hub để mở trực tiếp giao diện `kspcam (:2028)`, `Shinobi (:8080)`, hoặc `Node-RED (:2023)` của từng Box từ xa như cách `inut-point-hub` và `inut-solar-hub` đang làm.
- **Điểm nghẽn #7 — Không có Cổng Chủ Quán Đa Thuê Bao (`/portal/{token}`) và Lỗi mất hiệu lực mã QR khi khởi động lại**:
  - **Dẫn chứng**: `internal/server/server.go:211-251, 405-427` chỉ hỗ trợ 1 tài khoản `admin` và 1 tài khoản `viewer` toàn cục trong `config.yaml`. Hơn nữa, tính năng xuất QR video (`web/static/review.js:567-582`, `internal/server/api.go:2245-2254`) ký HMAC bằng khóa ngẫu nhiên trên RAM `s.dlKey` sinh bởi `rand.Read(s.dlKey)` lúc khởi động (`internal/server/server.go:63, 79-81`).
  - **Hệ quả**: Mỗi lần khởi động lại dịch vụ, **toàn bộ mã QR đã phát cho khách hàng đều bị hỏng chữ ký**. Chủ CLB Bida cũng không có đường link riêng (`/portal/{token}`) chỉ hiển thị đúng các bàn/camera thuộc quán của họ.
- **Điểm nghẽn #8 — Khóa cứng `frpc_config` & `*_virtual_ip` khiến kỹ thuật viên không thể sửa tên miền `store-*` / `c-store-*` và Virtual IP trên UI**:
  - **Dẫn chứng**: `internal/redbida/catalog.go:12-13, 196` đánh dấu `frpc_config`, `eth0_virtual_ip`, `end0_virtual_ip`, `lan0_virtual_ip`, `wlan0_virtual_ip`, `eth0_static_route` là `read-only-protected` hoặc `secret`.
  - **Hệ quả**: Trong thực tế vận hành (`FLEET_WATCHLIST.md:30-34, 62-66, 391-445, 550-565`), việc sửa nhầm tên miền trong `/root/ota-mqtt/change_ok/frpc_config` (ví dụ `inut_205_73` Pika Sun Pickleball bị trùng tên miền `store-lionking-2`) và xử lý xung đột IP ảo `.254` (`eth0:1`/`end0:1` + `iptables 80->2023` tại `inut_204_141`) diễn ra thường xuyên nhưng giao diện lại khóa không cho sửa và không kiểm tra trạng thái HTTP 200 của `store-<shop>.video.io.vn` / `c-store-<shop>.video.io.vn`.

#### Nhóm C: Mù Thông Số Chuẩn Hóa 8 Bước (Golden Hardening SOP) & Sức Khỏe Hệ Thống

- **Điểm nghẽn #9 — Hoàn toàn mù thông số 8 bước Golden Hardening (`FLEET_WATCHLIST.md:8-17`)**:
  - **Dẫn chứng**: Đối chiếu toàn bộ `internal/server/server.go:128-207` và `web/static/index.html:1-1572`, `kspcam` có **0 API và 0 giao diện** kiểm tra hay khắc phục 8 tiêu chuẩn sống còn của Box Bida:
    1. **ZRAM Swap 100% không giới hạn**: Kiểm tra `/sys/block/zram0/mem_limit == 0`, thuật toán `lz4`/`zstd`, `vm.swappiness == 80`, và phát hiện `/swapfile` gây hỏng thẻ nhớ eMMC (`FLEET_WATCHLIST.md:9, 250`).
    2. **PM2 Memory Guard & `/dev/null` Logs**: Kiểm tra giới hạn RAM cho `camera` (`280M` hoặc `(120+N*8)M`), `inut` (`280M`), `ota-mqtt` (`120M`) và hướng log ra `/dev/null` (`FLEET_WATCHLIST.md:10-11`).
    3. **Vô hiệu hóa "Bom tự sát" `sysrq-trigger`**: Kiểm tra `sysctl kernel.sysrq == 0` và quét gỡ lệnh `echo b > /proc/sysrq-trigger` hoặc `rm -rf /tmp` trong Node-RED `flow.json` (`ok2`), `/usr/local/bin/force_reboot.sh`, `netwatch.service` (`FLEET_WATCHLIST.md:12, 204-236`).
    4. **Smart Watchdog 60s (`shinobi-camera-watchdog.timer`)**: Giám sát tiến trình `ffmpeg` thực tế (`FLEET_WATCHLIST.md:13`).
    5. **Storage Guard 15m (`shinobi-storage-guard.timer`, ngưỡng 90% -> 80%)**: Chống tràn 100% ổ đĩa `/media/usb1` (`FLEET_WATCHLIST.md:14, 344-386`).
    6. **Udev Automount `errors=continue,noatime` & Chống bẫy `/etc/fstab`**: Phát hiện dòng `/dev/mmcblk0p1` trong `/etc/fstab` gây treo máy vào Emergency Mode khi thẻ SD lỗi (`FLEET_WATCHLIST.md:16, 41-46`).
- **Điểm nghẽn #10 — Giao diện Shinobi mù số lượng tiến trình `pgrep -c ffmpeg`, rò rỉ Worker `doSnapshot`, lỗi `moov atom` và treo MariaDB `liburing`**:
  - **Dẫn chứng**: `internal/server/api_shinobi.go:14-41` và `web/static/app.js:3532-3572` chỉ đọc trường `m.mode` (`record`, `start`, `stop`) từ bảng MySQL `Monitors`.
  - **Hệ quả**: Giao diện báo "Đang ghi hình (`record`)" nhưng thực tế trên Box: (a) `pgrep -c ffmpeg` bằng `0` do `camera.js` sập; (b) 28+ tiến trình zombie `snapshot.js` ngốn sạch RAM vì `doSnapshot: true` trong `/home/Shinobi/conf.json` và thiếu `snapProcess.terminate()` trong `/home/Shinobi/libs/monitor.js` (`FLEET_WATCHLIST.md:48-51, 449-453`); (c) File `.mp4` sinh ra bị hỏng `moov atom not found`; hoặc (d) MariaDB 10.6 treo cứng do thiếu `innodb_use_native_aio = 0` trên chip 32-bit ARMv7l (`inut_202_226`, `FLEET_WATCHLIST.md:266-273`).
- **Điểm nghẽn #11 — Hàm đồng bộ Shinobi (`internal/shinobi/sync.go` & `web/static/app.js`) vi phạm nghiêm trọng SOP Chuẩn Hóa (`FLEET_WATCHLIST.md:18-53`)**:
  - **Dẫn chứng**: `internal/shinobi/sync.go:80-112` (`BuildMonitorConfig`) và `web/static/app.js:3685-3709` tạo cấu hình Monitor bị **thiếu**:
    1. Thiếu `details.input_map_choices: {"stream": [], "snap": [], "record": [], "record_timelapse": [], "detector": [], "detector_object": [], "detector_sip_buffer": []}` và `input_maps: ""` — nguyên nhân trực tiếp làm Shinobi `builders.js:398` văng lỗi `TypeError: Cannot read properties of undefined (reading 'stream')` đánh sập toàn bộ FFmpeg (`FLEET_WATCHLIST.md:20-23`)!
    2. Thiếu `width`, `height` tường minh ở cấp cao nhất và `stream_scale_x: ""`, `stream_scale_y: ""` (`FLEET_WATCHLIST.md:24-26`).
    3. Thiếu cờ chống hỏng file MP4 khi mất điện: `sync.go:106` chỉ đặt `CustRecord: "-tag:v hvc1"` thay vì `"-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof"` (`FLEET_WATCHLIST.md:52`), còn `app.js:3695-3708` bỏ trống hoàn toàn `cust_record` và `wall_clock_timestamp_ignore: "0"`.
    4. Thiếu kiểm tra codec âm thanh `pcm_alaw`/`pcm_mulaw` để gán `acodec: "no"` (`sync.go:101` gán cứng `Acodec: "copy"` gây treo FFmpeg trên camera Dahua đời cũ — `FLEET_WATCHLIST.md:36, 143`).
    5. Không cập nhật hạn mức dung lượng `Users.details.size` (`50000` MB cho thẻ 64GB, `100000` MB cho 128GB, `210000` MB cho 256GB), khiến Shinobi bị khóa ở mức mặc định 10GB (`FLEET_WATCHLIST.md:27-31`).
- **Điểm nghẽn #12 — Không có Telemetry phần cứng, nhiệt độ SoC, phân vùng thẻ nhớ F2FS/ext4 và độ trễ VPN của chính Box**:
  - **Dẫn chứng**: `internal/server/api.go:1834-1884` (`handleStorage`) chỉ đọc ổ cứng bên trong camera/NVR qua DVRIP, hoàn toàn không đọc nhiệt độ CPU (`/sys/class/thermal/thermal_zone0/temp`), RAM/ZRAM, dung lượng eMMC `/dev/mmcblk2p1`, thẻ nhớ video `/media/usb1` (`f2fs`/`ext4`), `/tmp` hay độ trễ VPN `tun0` của Box.

#### Nhóm D: Mâu thuẫn Logic Giao diện, Quản lý Trạng thái Tạm thời & Lỗi UX Vận hành

- **Điểm nghẽn #13 — Mâu thuẫn trực tiếp giữa nút "⚡ Áp dụng Chuẩn Bida (Golden Template)" (`H.264 / 2048 Kbps`) và Anti-A Guardian (`H.265 / <=1280 Kbps`)**:
  - **Dẫn chứng**: Trong `web/static/index.html:325-329` và `web/static/app.js:1645-1692`, nút `#bulk-golden-template-btn` áp đặt `p-codec-value = 'H.264'`, `1920x1080`, `Bitrate 2048 CBR`. Ngược lại, `internal/server/anti_a.go:229-240` (`isStreamCompliant`) và `internal/config/config.go:205` lại bắt buộc `H.265 (HEVC)` + `Smart Codec` + `Bitrate <= 1280 Kbps`!
  - **Hệ quả**: Khi kỹ thuật viên bấm nút "Áp dụng Chuẩn Bida", chính `Anti-A Guardian` trên Box đó sẽ coi cấu hình vừa áp là "vi phạm do phần mềm đối thủ Arena" (`anti_a.go:286`) và tự động ghi đè ngược về `H.265`! Hơn nữa, mức `2048 Kbps` cho 13 camera từng làm nghẽn cổng LAN 100Mbps tại `inut_207_44` (`FLEET_WATCHLIST.md:575-583`).
- **Điểm nghẽn #14 — Trạng thái lưu tạm trên RAM (`probeCache`, `antiAGuardian.logs`, `nvrWatchdog.states`), mất sạch khi tải lại trang hoặc khởi động lại**:
  - **Dẫn chứng**: `web/static/app.js:122-126` lưu `probeCache` trên biến JS toàn cục (bấm `F5` là mất hết thông số độ phân giải/codec đã dò của camera, bắt phải bấm `#probe-selected-btn` dò lại rất lâu). Ở backend, `internal/server/anti_a.go:65-77` và `internal/server/nvr_health.go:50-56` chỉ lưu lịch sử trên RAM.
- **Điểm nghẽn #15 — Form "1-Click Onboarding Preset" (`redbida.js`) bỏ quên ô nhập `shinobi_group_key` và `ggcode`**:
  - **Dẫn chứng**: `web/static/index.html:831-838` hiển thị 2 ô nhập `#redbida-preset-groupkey` (`shinobi_group_key`) và `#redbida-preset-ggcode` (`ggcode`), nhưng hàm `redbidaGeneratePreset()` tại `web/static/redbida.js:886-951` hoàn toàn không đọc 2 ô này! Nghiêm trọng hơn, `internal/redbida/catalog.go:12, 41-55` khóa cả 2 khóa này là `Editable: false` (`Risk: "secret"`), nên nếu gửi lên cũng bị `internal/redbida/service.go:108-109` từ chối.
- **Điểm nghẽn #16 — Lỗi UX hiển thị thông báo khóa IP trên `login.html`, lỗi nút Copy RTSP URL và nghẽn bảng 115 dòng trên Mobile**:
  - **Dẫn chứng**:
    1. `internal/server/server.go:365-368` chuyển hướng `/login?err=locked` khi IP bị khóa 30 phút, nhưng `web/static/login.html:100-106` chỉ kiểm tra `.get('err')` rồi luôn in câu `"Sai tài khoản hoặc mật khẩu."` khiến người dùng không biết mình bị khóa IP.
    2. `web/static/redbida.js:1200-1212` (`#redbida-tab-copy-url-btn`) tạo chuỗi `rtsp://${location.hostname || '127.0.0.1'}:554/...` lấy hostname của trình duyệt web thay vì IP LAN thực của camera/đầu ghi.
    3. `web/static/style.css:1054-1068` (`table.reflow`) kết hợp `web/static/redbida.js:763-805` biến ~115 khóa OTA-MQTT thành danh sách dọc hơn 700 dòng DOM không phân trang trên màn hình điện thoại.

---

## PHẦN 2: ĐỐI CHIẾU KIẾN TRÚC THAM CHIẾU TỪ `root@77.88.208.17` (`inut-solar-hub`, `inut-point-hub`, `inut-p2p-hub`)

### 2.1. Bản đồ Hạ tầng, Port & Dịch vụ thực tế trên `root@77.88.208.17` (`inut01`)

Khảo sát Read-Only trực tiếp trên máy chủ trung tâm `root@77.88.208.17` (Ubuntu 24.04.4 LTS `x86_64`, Kernel `6.8.0-124-generic`, Public IP `103.226.248.95` NAT vào `ens192: 192.168.200.153/24`) cho thấy quy hoạch chuẩn xác của hệ sinh thái Central Hub hiện tại:

#### A. Bảng phân bổ Cổng (Port Allocation) & Dịch vụ Hub trên `77.88.208.17`

| Cổng (Port) | Giao thức | Địa chỉ Bind | Binary / Tiến trình | Systemd Service Unit | Tên miền Public / Vai trò |
| :--- | :---: | :---: | :--- | :--- | :--- |
| `80` / `443` | TCP | `0.0.0.0` | `nginx` | `nginx.service` | Giải mã SSL Let's Encrypt cho `*.a.inut.io.vn`, `*.a.vnmap.ai` -> proxy vào `127.0.0.1:8098` / `:8090` |
| `656` | TCP/UDP | `0.0.0.0` | `tincd -n inut -D` | `tinc@inut.service` | Tinc VPN mesh daemon (`tun1`: `77.88.208.17/32`) kết nối `inut_vpn_01` (`77.88.192.1`) & `inut_vpn_02` (`77.88.192.2`) |
| **`2088`** | **TCP** | **`*`** | `/usr/local/bin/inut-point-hub` | **`inut-point-hub.service`** | **iNut Point Central Fleet Hub (`https://inut-point.p2p.inut.io.vn`)** |
| **`2089`** | **TCP** | **`*`** | `/usr/local/bin/inut-solar-hub` | **`inut-solar-hub.service`** | **iNut Solar Central Fleet Hub (`https://inut-solar.p2p.inut.io.vn`)** |
| **`2090`** *(Đề xuất)* | **TCP** | **`*` / `127.0.0.1`** | `/usr/local/bin/inut-bida-hub` *(ĐANG TRỐNG)* | **`inut-bida-hub.service`** | **Cổng kế tiếp liền kề dải `2088`/`2089` dành riêng cho `inut-bida-hub` (`https://inut-bida.p2p.inut.io.vn` & `https://inut-bida.a.inut.io.vn`)** |
| `7000` / `7001` | TCP/UDP | `*` | `/usr/local/bin/inut-frps` | `inut-frps.service` | FRP Server riêng cho STCP/SUDP (`b.inut.io.vn:7000`) |
| **`7008` / `7009`** | **TCP** | **`*`** | `/root/inut-a-frp/bin/frps` | **`frps-inut-a-frp.service`** | **FRP Server nội bộ cho `*.a.inut.io.vn` (`bind_port=7008`, `vhost_http_port=8098`, `dashboard_port=7508`)** |
| **`7201`** | **TCP** | `127.0.0.1` | `/root/inut-a-frp/bin/fp-multiuser` | `fp-multiuser-inut-a-frp.service` | Plugin xác thực đa người dùng cho `frps-inut-a-frp` (`/root/inut-a-frp/conf/tokens.txt`) |
| **`31797` / `31798`** | **TCP** | **`*`** | `/usr/local/bin/inut-p2p-hub` | **`inut-p2p-hub.service`** | **iNut P2P Hub Web UI/API (`:31797`) & Device Long-Polling (`:31798`) (`https://inut-p2p.a.inut.io.vn`)** |

#### B. Mẫu cấu hình `systemd` và `frpc` nguyên bản đang chạy trên `77.88.208.17`

- **Systemd Service chuẩn (`inut-point-hub.service` / `inut-solar-hub.service`)**:
  ```ini
  [Unit]
  Description=iNut Point Central Fleet Hub
  After=network.target network-online.target
  Wants=network-online.target

  [Service]
  Type=simple
  WorkingDirectory=/opt/inut-point-hub
  ExecStart=/usr/local/bin/inut-point-hub -port 2088 -db /opt/inut-point-hub/data/hub.db
  Restart=always
  RestartSec=5s
  LimitNOFILE=65536

  [Install]
  WantedBy=multi-user.target
  ```
- **Cấu hình FRPC chuẩn xuất ra `p2p.inut.io.vn` (`/etc/frp/frpc-inut-point.ini` & `frpc-inut-solar.ini`)**:
  ```ini
  [common]
  server_addr = p2p.inut.io.vn
  server_port = 7008
  user = ksp_pdf_signer
  meta_token = b55ea6043210c1a0864735fe06aca46b

  [inut-point]
  type = http
  local_ip = 127.0.0.1
  local_port = 2088
  subdomain = inut-point
  ```
- **Cấu hình FRPC chuẩn xuất ra `a.inut.io.vn` (`/etc/frp/frpc-inut-p2p.ini` xác thực qua `/root/inut-a-frp/conf/tokens.txt`)**:
  ```ini
  [common]
  server_addr = a.inut.io.vn
  server_port = 7008
  user = inut-p2p
  meta_token = 6b91c84f23e745a90d8c1e2f3456789a

  [inut-p2p-hub]
  type = http
  local_ip = 127.0.0.1
  local_port = 31797
  subdomain = inut-p2p
  ```

#### C. Phát hiện Quan trọng về Định tuyến Tinc VPN (`tun1`) trên `77.88.208.17`

- Trên `77.88.208.17`, file `/etc/tinc/inut/tinc-up` hiện chỉ cấu hình:
  ```sh
  ifconfig $INTERFACE 77.88.208.17 netmask 255.255.255.255
  ip route add 77.88.192.0/24 dev $INTERFACE
  ```
- Do chỉ có route `77.88.192.0/24 dev tun1` (hướng về 2 node trung chuyển `inut_vpn_01` `77.88.192.1` và `inut_vpn_02` `77.88.192.2`), khi `77.88.208.17` chủ động mở kết nối trực tiếp tới các dải Box `77.88.202.x–207.x`, gói tin bị đẩy nhầm ra gateway mặc định `192.168.200.1 dev ens192` và rớt 100%.
- **Giải pháp Kiến trúc Kết nối Kép (Dual-Transport) bắt buộc cho `inut-bida-hub`**:
  1. **Luồng chính (Outbound WebSocket + Yamux Reverse Tunnel `/fleet/tunnel`)**: Giống hệt `inut-point-hub` và `inut-solar-hub`, Edge Agent (`kspcam` trên từng Box) chủ động gọi ngược từ trong ra (`wss://inut-bida.p2p.inut.io.vn/fleet/tunnel` hoặc `ws://77.88.208.17:2090/fleet/tunnel` qua NAT `77.88.192.2`). Cách này xuyên qua mọi lớp NAT/4G/CGNAT mà không cần mở port trên Box.
  2. **Luồng phụ (Direct VPN `77.88.x.y` Poller & Proxy cho các Box `kspcam` thế hệ cũ)**: Bổ sung route `ip route add 77.88.0.0/16 via 77.88.192.2 dev tun1` (hoặc SSH/SOCKS5 relay qua `77.88.192.2`) để `inut-bida-hub` lập tức thăm dò và điều khiển được ngay toàn bộ 30+ Box Bida đang chạy `kspcam (:2028)` và `Shinobi (:8080)` trước khi nâng cấp binary trên Box.

---

### 2.2. Đúc kết tinh hoa từ 3 dự án Hub mẫu (`inut-point-hub`, `inut-solar-hub`, `inut-p2p-hub`)

| Thành phần Kiến trúc | Nguồn Tham Chiếu (File:Line) | Tinh hoa kỹ thuật đúc kết cho `inut-bida-hub` |
| :--- | :--- | :--- |
| **1. Đóng gói 1 Binary Tĩnh + `embed.FS` Tách Biệt** | `inut-point-hub/cmd/inut-point-hub/main.go:22-96`<br>`inut-point-hub/web/embed.go:1-6` | Dùng `//go:embed templates/* static/*` tách bạch file `.html`, `.css`, `.js` (tránh nhồi 5.800 dòng HTML vào chuỗi Go như `inut-solar-monitor/internal/hub/dashboard.go`). Biên dịch tĩnh `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. |
| **2. Cơ sở dữ liệu SQLite WAL + Migration SHA-256 + Backup Vault** | `inut-point-hub/internal/storage/sqlite.go:27-827`<br>`inut-point-hub/internal/hub/backup_vault.go:53-245` | Dùng `modernc.org/sqlite` (`SetMaxOpenConns(1)`, `journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=5000`, `foreign_keys=ON`), kiểm tra checksum SHA-256 bảng `schema_migrations`, worker dọn dẹp `incremental_vacuum` mỗi 30 phút và lưu trữ 3 bản sao lưu cấu hình (`cameras.yaml`, `change_ok`, `Monitors.json`) của từng Box. |
| **3. Đường hầm Ngược Yamux + Đo RTT & Telemetry chủ động** | `inut-point-hub/internal/hub/tunnel.go:19-249`<br>`inut-solar-monitor/internal/hub/tunnel.go:36-226` | Nâng cấp WebSocket (`github.com/coder/websocket`, giới hạn đọc 32MB) lên `yamux.Client` (`KeepAliveInterval=15s`). Cứ mỗi `15s`, Hub vừa gọi `sess.Ping()` đo độ trễ `rtt_ms`, vừa mở luồng ảo lấy `/api/telemetry/live` để giữ kết nối không bị FRP/Nginx ngắt và đẩy SSE tức thì. |
| **4. Phê duyệt Thiết bị Mới bằng Vân tay (`Enrollment Queue`)** | `inut-point-hub/internal/hub/server.go:39-42, 268-360, 978-1126` | Khi Box mới kết nối kèm `X-Box-Secret`, nếu chưa được duyệt sẽ vào hàng đợi `box_enrollments` kèm mã vân tay 12 ký tự hex `sha256(secret)[:12]` (`HTTP 403 ENROLLMENT_PENDING`) để Admin bấm duyệt 1-Click trên UI và gán vào CLB/Đại lý. |
| **5. Streaming Reverse Proxy (`FlushInterval: -1`) + Tiêm `<base>` & Monkey-Patch JS** | `inut-solar-monitor/internal/hub/tunnel.go:229-592`<br>`inut-point-hub/internal/hub/proxy.go:44-172` | Kết hợp chữ ký bảo mật `SignStaffRequestV2` (`X-Inut-Staff-Sig`) của `inut-point-hub` với `FlushInterval: -1` và bộ viết lại đường dẫn `injectStationPrefix` (`<base href="/fleet/proxy/{boxId}/">`, monkey-patch `window.fetch`, `XMLHttpRequest`, `WebSocket`, `EventSource`) của `inut-solar-hub`. Nhờ đó giao diện gốc của `kspcam (:2028)`, `Shinobi (:8080)`, `Node-RED (:2023)` cùng luồng video MJPEG/MP4 và SSE chạy mượt mà dưới đường dẫn con của Hub! |
| **6. Cô lập URL 4 Tầng & Bộ Bàn Giao 1-Click (Handover Kit)** | `inut-point-hub/internal/hub/portal.go:15-306`<br>`inut-point-hub/web/static/fleet.js:1765-2182`<br>`inut-p2p/internal/hub/server.go:880-955` | Tách biệt: (1) Hub Admin (`/`), (2) Proxy Kỹ thuật (`/fleet/proxy/{boxId}/*`), (3) Cổng Chủ CLB Bida (`/portal/{portal_token}/*` dùng cookie cô lập `Path=/portal/{token}`), (4) Link/QR tải video khách hàng (`/clip/{clip_token}`) ký bằng khóa bền vững trong SQLite. Kèm nút **"📋 Sao Chép Gói Bàn Giao (Zalo/Telegram)"** và tạo mã QR SVG thuần JS không cần CDN. |
| **7. Phân quyền Cây Đại lý (Agency RBAC) + Argon2id + 2FA TOTP** | `inut-point-hub/internal/hub/auth_handlers.go:50-673`<br>`inut-p2p/internal/hub/auth.go:34-215` | 4 vai trò (`superadmin`, `engineer`, `operator`, `viewer`), phân cấp đại lý đệ quy (`WITH RECURSIVE`), mật khẩu băm `Argon2id`, xác thực 2 lớp `TOTP`, và bảng `security_lockouts` lưu SQLite kèm thông báo thời gian khóa rõ ràng trên trang đăng nhập. |
| **8. Hệ thống Thiết kế SCADA Công nghiệp (`Cyber-Industrial UI`)** | `inut-p2p/frontend/src/index.css:1-139`<br>`inut-point-hub/web/templates/fleet.html:74-188` | Nền kép Sáng/Tối (`#eef5f4` / `#071a24`), đèn LED 3 màu phần cứng (Xanh/Vàng/Đỏ), huy hiệu `LIVE SSE` tự động chuyển sang `POLLING 15s` khi rớt mạng, dải 6 thẻ KPI chữ số `tabular-nums`, biểu đồ xung nhịp `Sparkline SVG`, hộp thoại lệnh nhanh `Ctrl+K` (`CommandPalette`), và ngăn kéo chi tiết `SideDrawer`. |

---

## PHẦN 3: CÂY KIẾN TRÚC TOÀN DIỆN (MASTER ARCHITECTURE TREE) CỦA `inut-bida-hub`

### 3.1. Cây Thư Mục Mã Nguồn Dự Án (`Directory & Codebase Tree`)

```text
ksp-camera-auto/ (hoặc repo độc lập /home/ksp/inut-bida-hub)
├── cmd/
│   ├── kspcam/main.go                    # Binary Edge chạy tại từng Box (:2028) + tích hợp Yamux Tunnel Client
│   └── inut-bida-hub/main.go             # Binary Central Hub chạy tại root@77.88.208.17 (:2090)
├── internal/
│   ├── fleet/                            # [DÙNG CHUNG EDGE <-> HUB] Giao thức đường hầm & Heartbeat
│   │   ├── tunnel_client.go              # Edge撥 (dial) wss://inut-bida.p2p.inut.io.vn/fleet/tunnel + Yamux Server
│   │   ├── heartbeat.go                  # Gói tin Telemetry 30s (CPU, Temp, RAM, ZRAM, SD/F2FS, FFmpeg, Cameras, Golden 8 bước)
│   │   └── sign_v2.go                    # Ký & xác thực HMAC-SHA256 SignStaffRequestV2 (X-Inut-Staff-Sig)
│   ├── hub/                              # [CENTRAL HUB :2090] Lõi điều phối đội Box Bida/Pickleball
│   │   ├── server.go                     # HTTP ServeMux, Middleware (TraversalGuard, CSP, CSRFGuard), Graceful Shutdown
│   │   ├── tunnel_manager.go             # Quản lý phiên yamux.Client, vòng lặp Ping RTT 15s + DialSite(ctx, boxID)
│   │   ├── vpn_poller.go                 # Luồng phụ: Thăm dò trực tiếp 77.88.202.x–207.x (:2028/:8080/:2023) cho Box cũ
│   │   ├── proxy.go                      # Streaming ReverseProxy (FlushInterval: -1) + injectBoxPrefix (<base>, fetch/WS/SSE)
│   │   ├── portal.go                     # Cổng Chủ Quán cô lập (/portal/{portal_token}/*) + Audited Staff Bypass 30m
│   │   ├── clip_qr.go                    # Cổng tải/xem clip QR khách hàng (/clip/{clip_token}) ký bằng khóa bền vững SQLite
│   │   ├── auth.go                       # Argon2id, JWT/Session Cookie (inut_bida_session), TOTP 2FA, SQLite Lockouts
│   │   ├── agencies.go                   # Phân quyền 4 vai trò (superadmin, engineer, operator, viewer) & Cây Đại lý đệ quy
│   │   ├── events.go                     # Broker SSE thời gian thực (/fleet/api/events)
│   │   └── backup_vault.go               # Kho lưu trữ 3 thế hệ (cameras.yaml, change_ok.tar.gz, shinobi_monitors.json)
│   ├── hardening/                        # [ĐẶC THÙ BIDA] Bộ kiểm toán & Sửa lỗi 1-Click 8 Bước Golden Hardening
│   │   ├── inspector_8step.go            # Kiểm tra ZRAM, PM2 Guard, sysrq=0, Watchdog 60s, Storage Guard 15m, udev fstab, Shinobi
│   │   ├── shinobi_golden.go             # Chuẩn hóa Monitor (input_map_choices, frag_keyframe+empty_moov, Users.details.size)
│   │   ├── redbida_fleet.go              # Mở khóa có kiểm soát frpc_config, *_virtual_ip, shinobi_group_key, ggcode + HTTP 200 check
│   │   └── remediator.go                 # Thực thi vá lỗi từ xa qua Yamux API hoặc SSH Batch
│   └── storage/
│       ├── sqlite.go                     # modernc.org/sqlite WAL, busy_timeout=5000, incremental_vacuum(30m), retention(1h)
│       └── migrations.go                 # 13 bảng dữ liệu chuẩn hóa có kiểm tra SHA-256 checksum
├── web/
│   ├── embed.go                          # //go:embed templates/* static/*
│   ├── templates/
│   │   ├── hub_login.html                # Màn hình đăng nhập Cyber-Industrial + hiển thị đếm ngược khi bị khóa IP
│   │   ├── hub_cockpit.html              # Giao diện điều hành trung tâm 6 phân hệ (SCADA Dual-Canvas #eef5f4 / #071a24)
│   │   ├── club_portal.html              # Giao diện Cổng Chủ CLB Bida (/portal/{token}) xem trực tiếp, tua lại & tải QR
│   │   └── box_offline.html              # Thẻ báo mất kết nối tự động thử lại sau 5 giây
│   └── static/
│       ├── hub.css                       # Biến màu SCADA + Glassmorphism RedBida + Đèn LED 3 màu + tabular-nums
│       ├── hub-core.js                   # Hàm h() hyperscript chống XSS, kết nối LIVE SSE + tự động fallback POLLING 15s
│       ├── view-fleet-matrix.js          # Phân hệ 1: Bảng điều khiển 30+ Box & Gom nhóm CLB nhiều Box + Sparkline RTT
│       ├── view-golden-hardening.js      # Phân hệ 2: Ma trận Chuẩn hóa 8 bước & Sửa lỗi 1-Click + Dọn file MP4 lỗi moov
│       ├── view-cameras-nvr.js           # Phân hệ 3: Kho Camera & NVR toàn đội (Chuẩn H.265 / 1536 Kbps đồng bộ Anti-A)
│       ├── view-redbida-frpc.js          # Phân hệ 4: RedBida Studio, Sửa tên miền store-*/c-store-* & Virtual IP eth0:1
│       ├── view-proxy-review.js          # Phân hệ 5: Nhúng Reverse Proxy (/fleet/proxy/{boxId}/) & Xem lại Timeline 72h
│       └── view-portals-handover.js      # Phân hệ 6: Quản lý Đại lý, Cổng Chủ Quán, QR SVG thuần JS & Gói Bàn Giao Zalo
└── deploy/
    ├── inut-bida-hub.service             # Systemd service cho cổng :2090 trên root@77.88.208.17
    ├── frpc-inut-bida.service            # Systemd service cho tiến trình frpc-inut-bida
    ├── frpc-inut-bida.ini                # Cấu hình xuất subdomain inut-bida.p2p.inut.io.vn & inut-bida.a.inut.io.vn
    └── litestream.yml                    # Sao lưu liên tục WAL của /opt/inut-bida-hub/data/hub.db
```

---

### 3.2. Cây Kiến Trúc Luồng Kết Nối Kép (`Dual-Transport: Yamux Tunnel + VPN 77.88.x.y Poller`)

```text
[Trình duyệt Admin / Kỹ thuật / Chủ CLB Bida]
       │  HTTPS (TLS 1.3)
       ▼
[inut-bida.p2p.inut.io.vn]  hoặc  [inut-bida.a.inut.io.vn (Nginx :443 -> frps :8098)]
       │  FRP Tunnel (127.0.0.1:2090)
       ▼
┌───────────────────────────────────────────────────────────────────────────────────┐
│ MÁY CHỦ TRUNG TÂM: root@77.88.208.17 — inut-bida-hub (:2090)                      │
│                                                                                   │
│  ┌─────────────────────┐   ┌────────────────────────┐   ┌──────────────────────┐  │
│  │ 4-Tier Auth & RBAC  │   │ SQLite WAL (hub.db)    │   │ SSE Real-Time Broker │  │
│  │ - Hub Session / 2FA │   │ - 13 Bảng chuẩn hóa    │   │ - /fleet/api/events  │  │
│  │ - /portal/{token}   │   │ - Khóa QR bền vững     │   │ - Đẩy trạng thái 1s  │  │
│  │ - /clip/{token}     │   │ - 3-Gen Backup Vault   │   └──────────────────────┘  │
│  └─────────┬───────────┘   └───────────┬────────────┘                             │
│            ▼                           ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────────────────┐  │
│  │ Bộ Định Tuyến Hợp Nhất: DialBox(ctx, boxID, targetPort)                     │  │
│  ├──────────────────────────────────────┬──────────────────────────────────────┤  │
│  │ LUỒNG A (Ưu tiên #1 - Xuyên NAT/4G)  │ LUỒNG B (Dự phòng - Tương thích cũ)  │  │
│  │ Yamux Reverse Tunnel (/fleet/tunnel) │ VPN Direct Poller (77.88.202.x–207.x)│  │
│  │ - Box gọi wss://.../fleet/tunnel     │ - Qua route 77.88.0.0/16 via 192.2   │  │
│  │ - Mở kênh ảo tới 127.0.0.1:2028/8080 │ - Gọi thẳng HTTP <vpn_ip>:2028/:8080 │  │
│  │ - Ping RTT mỗi 15s + Lấy Telemetry   │ - Dành cho Box chưa nâng cấp binary  │  │
│  └──────────────────┬───────────────────┴──────────────────┬───────────────────┘  │
└─────────────────────┼──────────────────────────────────────┼──────────────────────┘
                      │                                      │
                      ▼                                      ▼
      [Box Bida Thế Hệ Mới (Yamux)]          [Box Bida Hiện Hữu (30+ Box trên VPN)]
      - kspcam (:2028) + Shinobi (:8080)     - inut_204_141, inut_205_72, inut_207_44...
      - Node-RED (:2023) + MQTT (:12369)     - kspcam (:2028) + Shinobi (:8080)
```

---

### 3.3. Cây Lược Đồ Cơ Sở Dữ Liệu SQLite WAL (`Database Schema Tree - /opt/inut-bida-hub/data/hub.db`)

```text
/opt/inut-bida-hub/data/hub.db (PRAGMA journal_mode=WAL; busy_timeout=5000; foreign_keys=ON)
├── 1. Nhóm Phân Quyền, Đại Lý & Bảo Mật (RBAC & Security)
│   ├── agencies                 (id, name, parent_id, created_at) -> Hỗ trợ đệ quy Chuỗi / Khu vực / Đại lý
│   ├── users                    (id, username, password_hash [Argon2id], role [superadmin|engineer|operator|viewer], agency_id, totp_secret, totp_enabled, token_valid_after)
│   ├── sessions                 (token_hash, user_id, ip, user_agent, expires_at, created_at)
│   ├── security_lockouts        (ip_or_key PRIMARY KEY, failed_count, locked_until, updated_at) -> Lưu bền vững, trả mã lỗi rõ ràng cho login.html
│   └── admin_audit              (id, ts, actor, role, action, target_venue_id, target_box_id, detail_json, prev_hash, entry_hash) -> Chuỗi băm SHA-256 chống sửa xóa
│
├── 2. Nhóm Quản Lý CLB Bida (Venues) & Đội Box (Multi-Box per Venue)
│   ├── venues                   (id, name, agency_id, address, portal_token [32-hex], owner_username, owner_password_hash, clip_signing_key [32-byte bền vững], created_at)
│   │                            *Giải quyết Điểm nghẽn #5 & #7: 1 CLB (như 99 Billiards, Magic Arena, Top Billiards) gắn được N Box; khóa ký QR clip không bao giờ mất khi restart!*
│   ├── boxes                    (id [vd: inut_204_141], venue_id, name, vpn_ip, lan_subnet, virtual_ip, secret_hash, status, tunnel_active, rtt_ms, soc_model,
│   │                             shinobi_api_key, shinobi_group_key, store_domain, cstore_domain, golden_score, last_seen)
│   ├── box_enrollments          (id, box_id, fingerprint [sha256[:12]], candidate_secret, remote_ip, soc_model, first_seen, status)
│   └── box_backups              (id, box_id, generation [1..3], sha256, size_bytes, payload_gzip BLOB, created_at) -> Lưu 3 bản backup cameras.yaml + change_ok + Monitors
│
├── 3. Nhóm Kho Camera, Đầu Ghi NVR & Bộ Nhớ Đệm Đã Dò (Giải quyết Điểm nghẽn #1, #13, #14)
│   ├── cameras                  (id [box_id + ":" + host + ":" + port], box_id, venue_id, table_no, name, vendor, host, port, rtsp_port, username, password_enc,
│   │                             channel, no_storage, nvr_id, nvr_channel, online, codec, width, height, fps, bitrate_kbps, audio_codec, osd_title, last_probed_at)
│   │                            *Khóa chính kép (box_id, host, port) chấm dứt hoàn toàn xung đột IP 192.168.1.x và lưu probeCache bền vững!*
│   └── nvr_channels             (box_id, nvr_host, channel_no, camera_id, hdd_status, recording_ok, time_drift_sec, watchdog_kick_count, last_checked_at)
│
└── 4. Nhóm Telemetry Hệ Thống, Chuẩn Hóa 8 Bước & Cổng Khách Hàng (Giải quyết Điểm nghẽn #9, #10, #12)
    ├── box_telemetry            (id, box_id, ts, cpu_pct, temp_c, load_1m, ram_pct, zram_used_mb, zram_limit_bytes, emmc_pct, sd_mount_pct, sd_fs_type,
    │                             sd_mount_opts, ffmpeg_count, snapshot_worker_count, shinobi_monitors_total, shinobi_monitors_recording, store_http_code, cstore_http_code)
    ├── golden_audits            (box_id PRIMARY KEY, audited_at, step1_zram_ok, step2_pm2_ok, step3_sysrq_ok, step4_watchdog_ok, step5_storage_guard_ok,
    │                             step6_shinobi_ok, step7_udev_fstab_ok, step8_frpc_vip_ok, corrupted_mp4_count, details_json)
    └── portal_sessions          (token_hash, venue_id, via ['owner'|'hub:<user>'], ip, expires_at, created_at)
```

---

### 3.4. Cây Giao Diện UI/UX & Component (`UI/UX Screen & Component Tree` — 6 Phân hệ + Cổng Chủ Quán)

```text
inut-bida-hub UI/UX (SCADA Dual-Canvas: Light #eef5f4 / Dark #071a24 + RedBida Glassmorphism)
├── [Khung Điều Khiển Chung - Sticky Topbar & Global Hotkeys]
│   ├── Cụm Đèn LED Phần Cứng 3 Màu (Xanh: Toàn đội chuẩn | Vàng: Có cảnh báo đĩa/FFmpeg | Đỏ: Có Box mất kết nối)
│   ├── Huy hiệu [🟢 LIVE SSE] (Tự động chuyển sang [🟡 POLLING 15s] khi rớt mạng)
│   ├── Bộ Chuyển Ngữ Cảnh Nhanh (Chọn CLB -> Chọn Box hoặc Gõ Ctrl+K mở CommandPalette tìm nhanh Box/Bàn/IP)
│   └── Dải 6 Thẻ KPI Toàn Đội (tabular-nums):
│       1. Tổng CLB & Box (Online / Offline)          4. Chuẩn Hóa 8 Bước (100% Đạt / Cần vá lỗi)
│       2. Tổng Camera & Luồng FFmpeg (Thực chạy/Kỳ vọng) 5. Tên Miền Store / C-Store (HTTP 200 / Lỗi FRPC)
│       3. Sức Khỏe Thẻ Nhớ Video & NVR (Tốt / Đầy >85%)  6. Anti-A & Đường Hầm (Yamux RTT ms + Sparkline)
│
├── PHÂN HỆ 1: Ma Trận Điều Hành Đội Box (`#fleet-matrix` — Thay thế hoàn toàn `#dashboard` cũ)
│   ├── Thanh lọc nhanh: [Tất cả] [🟢 Trực tuyến] [🔴 Mất kết nối] [⚠️ Lỗi FFmpeg/Zombie] [🛡️ Chưa chuẩn Golden] [🏢 CLB nhiều Box]
│   ├── Chế độ xem Gom nhóm theo CLB (hỗ trợ 99 Billiards 3 Box, Magic Arena 3 Box, Top Billiards 2 Box)
│   └── Bảng Mật Độ Cao (Desktop) / Thẻ Chạm 44px (Mobile): Hiển thị SoC, Nhiệt độ, RAM/ZRAM, % Thẻ SD (`errors=continue`), Số `ffmpeg` thực tế vs `2*N`, RTT Sparkline, và 3 nút mở nhanh Proxy (`kspcam`, `Shinobi`, `Node-RED`).
│
├── PHÂN HỆ 2: Trung Tâm Chuẩn Hóa 8 Bước Golden Hardening (`#golden-center` — Mới hoàn toàn)
│   ├── Bảng Ma Trận 8 Tiêu Chuẩn x 30+ Box (Đèn Xanh/Đỏ từng bước từ Bước 1 ZRAM đến Bước 8 FRPC/VIP)
│   ├── Ngăn Kéo Chi Tiết (`SideDrawer`): So sánh thông số thực tế trên Box với chuẩn SOP (`FLEET_WATCHLIST.md:8-53`)
│   └── Nút Hành Động 1-Click: [🛠️ Vá Chuẩn Golden Bước N] [🧹 Quét & Xóa MP4 Lỗi moov atom] [🔄 Đồng Bộ Chuẩn Shinobi Monitors + Quota]
│
├── PHÂN HỆ 3: Kho Camera, Đầu Ghi NVR & Anti-A Toàn Tuyến (`#fleet-cameras`)
│   ├── Bộ lọc theo CLB / Box / Hãng (Dahua, Hikvision, Tiandy) / Trạng thái ghi hình — Dữ liệu lấy từ SQLite `cameras` (không mất khi bấm F5!)
│   ├── Không gian Chi tiết Camera 2 Cột (Giữ nguyên MJPEG Live + OSD/Picture/Video/Audio/PTZ/Maint từ `kspcam`)
│   ├── Cấu hình Hàng loạt Chuẩn Hóa Mới: Nút [⚡ Áp dụng Chuẩn Bida (Golden H.265)] đặt `H.265 + Smart Codec + 1536 Kbps CBR + GOP 50 + AAC` (Đồng nhất 100% với Anti-A Guardian, giải quyết triệt để Điểm nghẽn #13!)
│   └── Bảng Tổng Hợp NVR Watchdog & Nhật Ký Vi Phạm Anti-A lưu bền vững trong SQLite.
│
├── PHÂN HỆ 4: RedBida OTA-MQTT, Tên Miền FRPC & Virtual IP Studio (`#fleet-redbida`)
│   ├── Kế thừa 17-Rule Golden Inspector + 8 Gradient Preset + 20-Tab INI Editor (`[C01]..[C20]`) từ `redbida.js`
│   ├── [ĐÃ SỬA LỖI #15]: Form 1-Click Preset đọc và áp dụng đầy đủ `shinobi_group_key` và `ggcode`
│   ├── [ĐÃ SỬA LỖI #8]: Hộp thoại Mở Khóa An Toàn (Có xác nhận) cho phép sửa `frpc_config` (`store-*`, `c-store-*`), `eth0_virtual_ip`/`end0_virtual_ip`, kiểm tra xung đột ARP `.254` và rule `iptables 80 -> 2023`
│   └── [ĐÃ SỬA LỖI #16]: Nút Copy RTSP URL lấy đúng IP LAN thực của camera (`cam.host`) và bảng ~115 khóa có phân trang/lọc mượt trên điện thoại.
│
├── PHÂN HỆ 5: Xem Lại Video Tập Trung & Cổng Reverse Proxy Trực Tiếp (`#fleet-review-proxy`)
│   ├── Trình Tua & Cắt Video `vis-timeline` 72h cho bất kỳ Bàn/Camera của bất kỳ CLB nào + Xuất QR MP4 bền vững (`/clip/{clip_token}`)
│   └── Khung Nhúng/Mở Tab Mới Reverse Proxy (`/fleet/proxy/{boxId}/{kspcam|shinobi|nodered}/`) tự động đăng nhập (Auto-Auth Injection).
│
└── PHÂN HỆ 6: Quản Lý Đại Lý, Cổng Chủ Quán (`/portal/{token}`) & Gói Bàn Giao (`#fleet-portals`)
    ├── Cây Phân Quyền Đại Lý (`superadmin`, `engineer`, `operator`, `viewer`) & Hàng Đợi Phê Duyệt Box Mới (`Enrollment Fingerprint`)
    ├── Hộp Thoại Gói Bàn Giao 1-Click (`Handover Kit Modal`): Sinh mã QR SVG thuần JS, nút [Vào thử Cổng Chủ Quán 30p (Audited Bypass)], và nút [📋 Sao Chép Gói Bàn Giao Zalo/Telegram]
    └── Giao Diện Riêng Cho Chủ CLB Bida (`/portal/{portal_token}`): Chỉ hiển thị đúng các bàn Bida của quán đó (Xem trực tiếp, Tua lại 72h, Cắt video tải về điện thoại, Trạng thái đầu ghi).
```

---

### 3.5. Cây Định Tuyến API, SSE & Reverse Proxy (`API & Proxy Route Tree`)

```text
HTTP ServeMux của inut-bida-hub (:2090)
├── 1. Tuyến Kết Nối Từ Box Bida (Edge-to-Hub Uplink — Xác thực bằng X-Box-Secret)
│   ├── GET  /fleet/tunnel                                 # Nâng cấp WebSocket -> Yamux Reverse Tunnel + Đo RTT 15s
│   ├── POST /fleet/api/heartbeat                          # Nhận Telemetry 30s, danh sách Camera, Golden 8 bước & Enrollment
│   └── POST /fleet/api/boxes/{boxId}/backups              # Nhận gói sao lưu 3 thế hệ (cameras.yaml, change_ok, Monitors)
│
├── 2. Tuyến Quản Trị Đội Thiết Bị & Chuẩn Hóa 8 Bước (Hub Admin/Engineer/Operator API)
│   ├── GET  /fleet/api/events                             # Luồng SSE thời gian thực (box.online, box.heartbeat, golden.alert)
│   ├── GET  /fleet/api/summary                            # Số liệu 6 thẻ KPI toàn đội + Danh sách CLB & Box theo quyền Đại lý
│   ├── POST /fleet/api/venues                             # Tạo/cập nhật CLB Bida (gắn nhiều Box vào 1 CLB)
│   ├── POST /fleet/api/enrollments/{id}/approve           # Duyệt vân tay Box mới 1-Click và gán vào CLB
│   ├── GET  /fleet/api/boxes/{boxId}/golden-audit         # Kết quả kiểm tra chi tiết 8 bước Golden Hardening
│   ├── POST /fleet/api/boxes/{boxId}/golden-remediate     # Vá lỗi 1-Click (ZRAM, PM2, sysrq=0, Watchdog, Shinobi, fstab, FRPC)
│   ├── POST /fleet/api/boxes/{boxId}/shinobi-golden-sync  # Đồng bộ Monitor chuẩn (input_map_choices, frag_keyframe, quota)
│   └── ALL  /fleet/api/boxes/{boxId}/edge/*               # Chuyển tiếp lệnh JSON (cameras, nvr, anti-a, redbida, scan, traffic)
│
├── 3. Tuyến Streaming Reverse Proxy Kỹ Thuật (Yêu cầu phiên đăng nhập Hub + Quyền Đại lý)
│   ├── ALL  /fleet/proxy/{boxId}/kspcam/*                 # Proxy vào kspcam (:2028) kèm SignStaffRequestV2 + injectBoxPrefix
│   ├── ALL  /fleet/proxy/{boxId}/shinobi/*                # Proxy vào Shinobi (:8080) xem luồng HLS/MP4/Monitors
│   └── ALL  /fleet/proxy/{boxId}/nodered/*                # Proxy vào Node-RED (:2023) kiểm tra flow RedBida
│
└── 4. Tuyến Cô Lập Dành Cho Chủ CLB Bida & Khách Hàng Quét QR (Multi-Tenant Public/Tokenized)
    ├── GET  /portal/{portal_token}                        # Trang giao diện riêng của Chủ CLB Bida (cookie inut_portal_<token>)
    ├── POST /portal/{portal_token}/api/login              # Đăng nhập Chủ Quán (hoặc nhận phiên Audited Bypass 30 phút từ Hub)
    ├── GET  /portal/{portal_token}/api/tables             # Danh sách bàn Bida, ảnh chụp mới nhất & trạng thái ghi hình của quán
    ├── GET  /portal/{portal_token}/api/recordings         # Lịch sử video 72h của từng bàn trong quán
    ├── GET  /portal/{portal_token}/api/playback           # Phát/cắt nhanh MP4 (&format=fastmp4) qua đường hầm về trình duyệt
    └── GET  /clip/{clip_token}                            # Link tải/xem đoạn video khách hàng quét mã QR (ký bằng clip_signing_key bền vững)
```

---

### 3.6. Cây Triển Khai Production trên `root@77.88.208.17` (`:2090`, Systemd & FRPC)

```text
root@77.88.208.17 (inut01)
├── /usr/local/bin/inut-bida-hub                           # Binary tĩnh (CGO_ENABLED=0 GOOS=linux GOARCH=amd64, chmod 0755)
├── /opt/inut-bida-hub/
│   ├── data/hub.db                                        # SQLite WAL chính (+ hub.db-wal, hub.db-shm)
│   ├── backups/                                           # Kho lưu trữ sao lưu cấu hình các Box
│   └── reports/                                           # Báo cáo kiểm toán định kỳ toàn đội
├── /etc/systemd/system/inut-bida-hub.service              # Chạy cổng :2090 (liền kề :2088 inut-point-hub và :2089 inut-solar-hub)
├── /etc/systemd/system/frpc-inut-bida.service             # Dịch vụ đẩy cổng :2090 ra tên miền Public HTTPS
└── /etc/frp/frpc-inut-bida.ini                            # Cấu hình FRPC (Hỗ trợ song song p2p.inut.io.vn & a.inut.io.vn)
```

#### Nội dung chuẩn `/etc/systemd/system/inut-bida-hub.service`
```ini
[Unit]
Description=iNut Bida Central Fleet Management Hub (Port 2090)
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/inut-bida-hub
ExecStart=/usr/local/bin/inut-bida-hub -port 2090 -db /opt/inut-bida-hub/data/hub.db
Restart=always
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

#### Nội dung chuẩn `/etc/frp/frpc-inut-bida.ini` (Đồng bộ 100% với `frpc-inut-point.ini` & `frpc-inut-solar.ini`)
```ini
[common]
server_addr = p2p.inut.io.vn
server_port = 7008
user = ksp_pdf_signer
meta_token = b55ea6043210c1a0864735fe06aca46b

[inut-bida]
type = http
local_ip = 127.0.0.1
local_port = 2090
subdomain = inut-bida
```
*(Tùy chọn mở rộng: Có thể thêm 1 instance trỏ vào `server_addr = a.inut.io.vn`, `server_port = 7008`, `user = inut-p2p`, `meta_token = 6b91c84f23e745a90d8c1e2f3456789a`, `subdomain = inut-bida` để chạy đồng thời trên cả `https://inut-bida.p2p.inut.io.vn` và `https://inut-bida.a.inut.io.vn` mà không cần khởi động lại bất kỳ dịch vụ `frps` nào đang chạy).*

---

## PHẦN 4: CÂY LỘ TRÌNH TRIỂN KHAI TỪNG BƯỚC (STEP-BY-STEP EXECUTION TREE — PHASE 1 ĐẾN PHASE 6)

```text
LỘ TRÌNH XÂY DỰNG & TRIỂN KHAI inut-bida-hub (6 GIAI ĐOẠN TUẦN TỰ)
│
├─► PHASE 1: Khởi tạo Khung Binary Go, Cơ Sở Dữ Liệu SQLite WAL & Phân Quyền Đại Lý (RBAC)
│   ├── Bước 1.1: Tạo entrypoint `cmd/inut-bida-hub/main.go` (-port 2090, -db /opt/inut-bida-hub/data/hub.db) và `web/embed.go` (`//go:embed templates/* static/*`).
│   ├── Bước 1.2: Cài đặt `internal/storage/sqlite.go` (`modernc.org/sqlite`, `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`, `incremental_vacuum` 30m, `retention` 1h) và `migrations.go` tạo 13 bảng dữ liệu có kiểm tra SHA-256 checksum.
│   ├── Bước 1.3: Xây dựng `internal/hub/auth.go` & `agencies.go`: Mật khẩu Argon2id, xác thực TOTP 2FA, phân cấp 4 vai trò (`superadmin`, `engineer`, `operator`, `viewer`), truy vấn đệ quy CTE cho Cây Đại lý, và bảng `security_lockouts` lưu SQLite.
│   ├── Bước 1.4: Sửa triệt để lỗi UX đăng nhập (Điểm nghẽn #16.1): Khi IP vượt ngưỡng thử sai, trả về thời gian khóa còn lại và hiển thị cảnh báo đếm ngược rõ ràng trên `hub_login.html`.
│   └── [Tiêu chí nghiệm thu Phase 1]: `go test ./internal/storage/... ./internal/hub/...` PASS 100%; đăng nhập, tạo Đại lý/CLB và khóa brute-force hoạt động bền vững qua các lần restart.
│
├─► PHASE 2: Kết Nối Kép (Yamux Reverse Tunnel + VPN `77.88.x.y` Poller) & Streaming Reverse Proxy
│   ├── Bước 2.1: Xây dựng `internal/fleet/tunnel_client.go` (nhúng vào `kspcam`) và `internal/hub/tunnel_manager.go` (trên Hub): Nâng cấp WebSocket `/fleet/tunnel` lên `yamux`, duy trì vòng lặp `sess.Ping()` đo `rtt_ms` mỗi 15s và đẩy sự kiện qua `internal/hub/events.go` (SSE).
│   ├── Bước 2.2: Xây dựng Hàng đợi Phê duyệt Vân tay (`box_enrollments`): Box mới kết nối trả về `403 ENROLLMENT_PENDING` kèm mã vân tay 12 ký tự hex `sha256(secret)[:12]` để Admin duyệt 1-Click trên giao diện.
│   ├── Bước 2.3: Xây dựng `internal/hub/vpn_poller.go` & hàm hợp nhất `DialBox(ctx, boxID, targetPort)`: Ưu tiên đi qua luồng ảo Yamux; nếu Box chưa có Yamux thì tự động gọi qua mạng VPN `77.88.202.x–207.x` (sau khi cấu hình route `77.88.0.0/16 via 77.88.192.2 dev tun1` trên `77.88.208.17`), nạp danh sách 30+ Box từ `FLEET_WATCHLIST.md`.
│   ├── Bước 2.4: Xây dựng `internal/hub/proxy.go`: Streaming Reverse Proxy (`FlushInterval: -1`) kèm chữ ký `SignStaffRequestV2` (`X-Inut-Staff-Sig`), xóa cookie của Hub và tiêm `<base href="/fleet/proxy/{boxId}/{app}/">` + script monkey-patch `fetch`/`XHR`/`WebSocket`/`EventSource` để mở mượt `kspcam (:2028)`, `Shinobi (:8080)`, `Node-RED (:2023)`.
│   └── [Tiêu chí nghiệm thu Phase 2]: Mở trực tiếp giao diện `kspcam` và xem luồng MJPEG/MP4 của bất kỳ Box nào qua `/fleet/proxy/{boxId}/kspcam/` không bị lỗi đường dẫn tĩnh 404 hay đứt luồng SSE.
│
├─► PHASE 3: Giao Diện SCADA Fleet Matrix, Dải 6 Thẻ KPI & Gom Nhóm CLB Nhiều Box
│   ├── Bước 3.1: Thiết kế `web/static/hub.css` và `web/templates/hub_cockpit.html` theo chuẩn SCADA Dual-Canvas (`#eef5f4` Light / `#071a24` Dark), cụm đèn LED 3 màu, huy hiệu `LIVE SSE` tự chuyển sang `POLLING 15s` khi mất mạng, và hộp thoại lệnh nhanh `Ctrl+K` (`CommandPalette`).
│   ├── Bước 3.2: Xây dựng `view-fleet-matrix.js` (Giải quyết Điểm nghẽn #5 & #12): Hiển thị Dải 6 Thẻ KPI chữ số `tabular-nums`, gom nhóm nhiều Box vào 1 CLB (như *99 Billiards CS11* 3 Box, *Magic Arena* 3 Box, *Top Billiards* 2 Box), vẽ biểu đồ xung nhịp `Sparkline SVG` cho RTT và hiển thị nhiệt độ SoC, RAM/ZRAM, % thẻ SD (`f2fs`/`ext4`), số tiến trình `ffmpeg` thực tế so với `2*N` camera.
│   └── [Tiêu chí nghiệm thu Phase 3]: Giao diện hiển thị tức thì 30+ Box không cần bấm F5; trên màn hình `<768px` tự động chuyển sang dạng thẻ chạm `44px` mượt mà.
│
├─► PHASE 4: Trung Tâm Chuẩn Hóa 8 Bước (Golden Hardening), Sửa Lỗi Shinobi Sync & RedBida/FRPC Studio
│   ├── Bước 4.1: Xây dựng `internal/hardening/inspector_8step.go` & `remediator.go` + giao diện `view-golden-hardening.js` (Giải quyết Điểm nghẽn #9 & #10): Kiểm toán và vá lỗi 1-Click cho 8 tiêu chuẩn:
│   │     (1) ZRAM 100% (`mem_limit=0`, `swappiness=80`, xóa `/swapfile` trên eMMC);
│   │     (2) PM2 Memory Guard (`camera`, `inut`, `ota-mqtt`) & hướng log ra `/dev/null`;
│   │     (3) Vô hiệu hóa `kernel.sysrq=0` & gỡ lệnh `echo b > /proc/sysrq-trigger` trong Node-RED/netwatch;
│   │     (4) Smart Watchdog 60s (`shinobi-camera-watchdog.timer` đếm `pgrep -c ffmpeg`);
│   │     (5) Storage Guard 15m (`shinobi-storage-guard.timer` cắt 90% -> 80%);
│   │     (6) Tắt `doSnapshot: false` trong Shinobi `conf.json`, vá `snapProcess.terminate()` chống rò rỉ Worker, kiểm tra `innodb_use_native_aio=0` cho ARMv7l, và quét dọn file `.mp4` hỏng `moov atom`;
│   │     (7) Kiểm tra udev automount `errors=continue,noatime` và gỡ dòng `/dev/mmcblk0p1` khỏi `/etc/fstab`;
│   │     (8) Kiểm tra HTTP 200 của `store-<shop>.video.io.vn` & `c-store-<shop>.video.io.vn` và kiểm tra xung đột ARP của Virtual IP (`eth0:1`/`end0:1` + `iptables 80->2023`).
│   ├── Bước 4.2: Sửa lỗi đồng bộ Shinobi trong `internal/shinobi/sync.go` & `internal/hardening/shinobi_golden.go` (Giải quyết Điểm nghẽn #11): Bổ sung bắt buộc `details.input_map_choices`, `input_maps: ""`, `width`/`height` tường minh, `cust_record: "-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof"`, tự động đặt `acodec: "no"` khi nguồn âm thanh là `pcm_alaw`/`pcm_mulaw`, và tự động nâng hạn mức `Users.details.size` (`50000`/`100000`/`210000` MB theo dung lượng thẻ SD).
│   ├── Bước 4.3: Chuẩn hóa Kho Camera Toàn Đội (`view-cameras-nvr.js` — Giải quyết Điểm nghẽn #1, #13, #14): Lưu `cameras` theo khóa kép `(box_id, host, port)` và giữ `probeCache` bền vững trong SQLite; sửa nút **⚡ Áp dụng Chuẩn Bida (Golden Template)** thành `H.265 + Smart Codec + 1536 Kbps CBR + GOP 50 + AAC` đồng nhất 100% với `Anti-A Guardian`.
│   ├── Bước 4.4: Nâng cấp RedBida & FRPC Studio (`view-redbida-frpc.js` — Giải quyết Điểm nghẽn #3, #8, #15, #16): Cho phép đọc/ghi `/root/ota-mqtt/change_ok` từ xa của từng Box; sửa `redbidaGeneratePreset()` để nạp đầy đủ `shinobi_group_key` và `ggcode`; thêm chế độ Mở Khóa Có Xác Nhận cho `frpc_config` & `*_virtual_ip`; sửa nút Copy RTSP URL lấy đúng `cam.host` LAN IP và thêm phân trang cho bảng ~115 khóa trên Mobile.
│   └── [Tiêu chí nghiệm thu Phase 4]: Đồng bộ Shinobi từ Hub không bao giờ làm sập `builders.js:398`; áp dụng Golden Template không bị Anti-A báo vi phạm; sửa tên miền `store-*` và `shinobi_group_key` hoạt động 1-Click.
│
├─► PHASE 5: Cổng Chủ Quán Đa Thuê Bao (`/portal/{token}`), QR Clip Bền Vững & Gói Bàn Giao 1-Click
│   ├── Bước 5.1: Xây dựng `internal/hub/portal.go` & `web/templates/club_portal.html` (Giải quyết Điểm nghẽn #7): Mỗi CLB Bida có một đường dẫn `/portal/{portal_token}` (32-hex) sử dụng cookie cô lập `Path=/portal/{portal_token}`. Chủ quán đăng nhập chỉ thấy đúng các bàn Bida của quán mình, xem ảnh/luồng trực tiếp, tua lại timeline 72h và cắt video.
│   ├── Bước 5.2: Xây dựng `internal/hub/clip_qr.go`: Ký mã QR tải video (`/clip/{clip_token}`) bằng khóa `venues.clip_signing_key` lưu trong SQLite thay vì biến RAM `s.dlKey`, đảm bảo mã QR đã in/gửi cho khách vẫn tải được bình thường sau khi khởi động lại server.
│   ├── Bước 5.3: Xây dựng Hộp thoại Gói Bàn Giao (`view-portals-handover.js`): Tích hợp bộ sinh mã QR SVG thuần JS (không phụ thuộc CDN), nút **Vào thử Cổng Chủ Quán 30 phút (`Audited Staff Bypass`)**, và nút **📋 Sao Chép Toàn Bộ Gói Bàn Giao (Zalo/Telegram)** giống `inut-point-hub`.
│   └── [Tiêu chí nghiệm thu Phase 5]: Truy cập `/portal/{token}` không thể đọc dữ liệu của CLB khác; khởi động lại `inut-bida-hub` không làm hỏng bất kỳ link QR `/clip/{clip_token}` nào.
│
└─► PHASE 6: Đóng Gói Binary Tĩnh, Triển Khai Zero-Downtime trên `root@77.88.208.17:2090` & Kiểm Chứng Toàn Tuyến
    ├── Bước 6.1: Biên dịch tĩnh `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/inut-bida-hub ./cmd/inut-bida-hub` và chạy kiểm thử `go vet ./... && go test -race ./...`.
    ├── Bước 6.2: Tạo thư mục `/opt/inut-bida-hub/{data,backups,reports}`, tải binary lên `/usr/local/bin/inut-bida-hub.new` và tráo nguyên tử `mv -f /usr/local/bin/inut-bida-hub.new /usr/local/bin/inut-bida-hub` trên `root@77.88.208.17`.
    ├── Bước 6.3: Cài đặt và kích hoạt `inut-bida-hub.service` (cổng `:2090`) cùng `frpc-inut-bida.service` (`/etc/frp/frpc-inut-bida.ini` xuất ra `https://inut-bida.p2p.inut.io.vn` và/hoặc `https://inut-bida.a.inut.io.vn`).
    └── Bước 6.4: Kiểm chứng độc lập (Zero-Regression Verification):
          1. `ss -tulpn | grep -E ':2088|:2089|:2090|:31797'` -> Xác nhận `:2090` đang LISTEN và các dịch vụ hiện hữu (`inut-point-hub` `:2088`, `inut-solar-hub` `:2089`, `inut-p2p-hub` `:31797`) hoạt động nguyên vẹn 100%.
          2. `curl -fsSL http://127.0.0.1:2090/api/health` & `curl -fsSL https://inut-bida.p2p.inut.io.vn/api/health` -> Trả về HTTP 200 `{"status":"ok","service":"inut-bida-hub"}`.
          3. Ghi nhận trạng thái triển khai chuẩn xác vào sổ cái `DEPLOYED_DEVICES.json`.
```

---

## PHỤ LỤC: BẢNG ĐỐI CHIẾU KHẮC PHỤC 16 ĐIỂM NGHẼN (`kspcam` $\rightarrow$ `inut-bida-hub`)

| Mã | Điểm nghẽn trên `kspcam` hiện tại | Giải pháp triệt để trong `inut-bida-hub` (`:2090`) | Phân hệ / File Đích |
| :-: | :--- | :--- | :--- |
| **#1** | Xung đột khóa `host:port` trong 1 file `cameras.yaml` | Lưu bảng SQLite `cameras` với khóa chính kép `(box_id, host, port)` gắn với `venue_id` | `internal/storage/migrations.go` |
| **#2** | Khóa cứng 1 Shinobi `127.0.0.1:8080` & 1 Group Key | Lưu `shinobi_api_key` & `shinobi_group_key` theo từng Box trong bảng `boxes` | `internal/hardening/shinobi_golden.go` |
| **#3** | Khóa cứng MQTT `127.0.0.1:12369` & `/root/ota-mqtt/change_ok` | Điều phối đọc/ghi catalog OTA-MQTT của từng Box qua đường hầm `/fleet/api/boxes/{boxId}/edge/redbida` | `internal/hardening/redbida_fleet.go` |
| **#4** | `AF_PACKET` `iftop` & UDP Scan chỉ chạy trên card mạng máy chủ | Giữ sniffer & LAN scan tại Edge `kspcam`, truyền luồng SSE kết quả qua đường hầm Yamux về Hub | `internal/hub/proxy.go` |
| **#5** | Không có bảng điều khiển 30+ Box & CLB nhiều Box | Phân hệ 1 `#fleet-matrix` gom nhóm nhiều Box theo CLB (`venues` 1-N `boxes`) + Dải 6 thẻ KPI | `web/static/view-fleet-matrix.js` |
| **#6** | Thiếu Reverse Proxy vào từng Box & bộ chọn Box | Streaming Reverse Proxy `/fleet/proxy/{boxId}/{kspcam\|shinobi\|nodered}/` + `Ctrl+K` | `internal/hub/proxy.go` |
| **#7** | Thiếu Cổng Chủ Quán & mất hiệu lực QR khi restart server | Cổng `/portal/{portal_token}` cô lập + ký `/clip/{clip_token}` bằng `venues.clip_signing_key` trong SQLite | `internal/hub/portal.go`, `clip_qr.go` |
| **#8** | Khóa cứng `frpc_config` & `*_virtual_ip` không cho sửa trên UI | Chế độ Mở Khóa Có Xác Nhận + kiểm tra tự động HTTP 200 `store-*`/`c-store-*` & ARP `.254` | `web/static/view-redbida-frpc.js` |
| **#9** | Mù hoàn toàn 8 bước chuẩn hóa Golden Hardening | Phân hệ 2 `#golden-center` kiểm toán tự động & vá lỗi 1-Click cả 8 bước SOP | `internal/hardening/inspector_8step.go` |
| **#10** | Mù số lượng `pgrep -c ffmpeg`, rò rỉ `doSnapshot`, lỗi `moov` | Đếm `ffmpeg_count` & `snapshot_worker_count` thực tế trong Heartbeat + công cụ dọn MP4 lỗi `moov` | `internal/fleet/heartbeat.go` |
| **#11** | `sync.go` thiếu `input_map_choices`, `empty_moov`, quota 10GB | Bổ sung đủ `input_map_choices`, `frag_keyframe+empty_moov`, guard `pcm_alaw` và nâng `Users.details.size` | `internal/hardening/shinobi_golden.go` |
| **#12** | Không có Telemetry nhiệt độ SoC, ZRAM, thẻ nhớ F2FS/ext4 | Thu thập đầy đủ vào bảng `box_telemetry` mỗi 30s và vẽ biểu đồ Sparkline trên Fleet Matrix | `internal/storage/migrations.go` |
| **#13** | Mâu thuẫn Golden Template (`H.264/2048k`) vs Anti-A (`H.265`) | Đồng nhất nút Chuẩn Bida thành `H.265 + Smart Codec + 1536 Kbps CBR + GOP 50 + AAC` | `web/static/view-cameras-nvr.js` |
| **#14** | Mất `probeCache` khi bấm F5, mất log Anti-A/NVR khi restart | Lưu bền vững thông số camera đã dò và lịch sử cảnh báo vào SQLite `hub.db` + đẩy SSE | `internal/storage/sqlite.go` |
| **#15** | `redbidaGeneratePreset` bỏ sót `shinobi_group_key` & `ggcode` | Đọc đủ 2 ô input trên form và cho phép cập nhật an toàn trong `redbida.Service.Apply` | `web/static/view-redbida-frpc.js` |
| **#16** | Lỗi `login.html` `?err=locked`, lỗi Copy RTSP URL & bảng 115 dòng | Hiển thị rõ thời gian khóa IP, copy đúng `rtsp://<cam.host>:554/...` và phân trang bảng trên Mobile | `web/templates/hub_login.html`, `view-redbida-frpc.js` |
