# Khảo Sát & Phân Tích Toàn Diện Codebase `/#cameras` (Camera Management & Bulk Configuration)

> **Agent:** Explorer (`explorer_survey_cameras`)  
> **Target Requirement:** R1: Nâng cấp toàn diện giao diện `/#cameras` (Kho camera, Chỉnh hàng loạt, Camera Detail Workspace, Smart Bulk Wizard, NVR Diagnostics).  
> **Thời gian khảo sát:** 2026-08-24.  
> **Codebase:** `ksp-camera-auto` (Backend Go thuần `CGO_ENABLED=0`, Frontend Vanilla SPA qua `go:embed static`).

---

## 1. Tổng Quan Kiến Trúc Frontend & Bản Đồ Tệp Tin

### 1.1 Danh mục tệp tin giao diện (`web/static/`)
| Tệp Tin | Kích thước | Vai trò kiến trúc |
|---|---|---|
| `web/static/index.html` | 69 KB (1142 dòng) | Cấu trúc DOM cốt lõi: chứa toàn bộ markup SPA của `#view-cameras`, `#camera-task-tabs`, `#camera-detail`, `#nvr-link-dialog`, `#gallery-dialog`, `#lightbox-dialog`, `#camera-form-dialog`. |
| `web/static/app.js` | 170 KB (3431 dòng) | Trình điều khiển trung tâm (Controller): quản lý Hash routing, Camera Inventory, lọc/tìm kiếm, render bảng/thẻ, singleflight snapshot loading, live MJPEG session, lazy loading 7 tab Camera Detail, PTZ controller, diffing Picture settings, video FPS capability, bulk SSE execution, NVR link & health watchdog. |
| `web/static/ui-core.js` | 8.3 KB (222 dòng) | UI Primitives: `escapeHtml`, `timeoutSec()`, `setBusy()`, `showToast()`, `showConfirm()`, `openDialog()`, `closeDialog()`, `progressBar()`, `livePreview()`, `api()` fetch client. |
| `web/static/style.css` | 56 KB (1384 dòng) | Hệ thống Design Tokens (Dark/Light theme, Glassmorphism tokens `--glass-*`), layout grid/flex, responsive reflow tables, sticky camera detail layout, PTZ pad, summary bar, danger zone. |
| `web/static/review.js` | 31 KB | Quản lý trang Xem lại (Playback Timeline, Vis-Timeline, Fast MP4/MKV export progress, QR HMAC token generator). |
| `web/static/redbida.js` | 26 KB | Quản lý trang RedBida / OTA-MQTT (20-tab INI, Gradient Palette, Knowledge Hub). |
| `web/static/help.js` | 11 KB | Quản lý trợ giúp, tìm kiếm bài viết và trợ lý hỏi đáp nhanh. |
| `web/static/qrcode.min.js` | 20 KB | Thư viện tạo mã QR SVG/Canvas cho Serial Number và Playback Token. |
| `web/static/vis-timeline-graph2d.min.js` | 569 KB | Thư viện biểu đồ timeline trực quan cho playback và phân tích khoảng trống ghi hình. |

---

## 2. Khảo Sát Chi Tiết Các Thành Phần Giao Diện `/#cameras` Hiện Tại

### 2.1 Cấu trúc Task Tabs & Routing
Giao diện `/#cameras` được điều phối bởi router dạng hash `#cameras/<task>`:
- `#cameras/list`: Danh sách kho camera (tìm kiếm, lọc hãng, dò tên kênh, chọn bulk, thêm/sửa/xóa).
- `#cameras/bulk`: Bảng điều khiển chỉnh hàng loạt (chọn stream, kênh, codec, res, smart codec, GOP, bitrate, OSD, audio, password).
- `#cameras/nvr`: Quản lý đầu ghi NVR (liên kết camera không thẻ nhớ, báo cáo sức khỏe timeline, bật/tắt watchdog, sync NTP).
- `#cameras/results`: Bảng tiến độ và nhật ký thực thi SSE theo thời gian thực.
- `#cameras/cam/<encodedId>/<tab>`: Không gian làm việc chi tiết (Camera Detail Workspace), hỗ trợ 7 tab: `osd`, `picture`, `video`, `audio`, `network`, `ptz`, `maint`.

Khi chuyển sang chế độ Detail (`#cameras/cam/...`), hàm `renderCameraTask()` sẽ ẩn `#camera-task-tabs`, ẩn tiêu đề trang và cho `#camera-detail` chiếm trọn không gian làm việc.

---

### 2.2 Chế độ hiển thị: Grid vs Table Views
- **Hiện trạng:**
  - Hiện tại chỉ có một cấu trúc bảng duy nhất (`#cam-table` với tbody `#cam-tbody`).
  - Trên màn hình Desktop: bảng hiển thị các cột (Checkbox, Tên, Host, Cổng, Hãng, SN/QR, Tài khoản, Mật khẩu, Thông tin luồng, Thao tác).
  - Trên màn hình Mobile (`max-width: 767px`): bảng áp dụng class `.reflow` biến từng thẻ `<tr>` thành dạng khối thẻ dọc xếp chồng, ẩn `thead`.
  - **Khoảng trống cần nâng cấp (Gap Analysis):** Chưa có nút chuyển đổi linh hoạt giữa **Chế độ Danh sách (Table View)** và **Chế độ Lưới thẻ (Grid Cards View)** với thumbnail snapshot trực quan tải tự động, badge hãng màu sắc nổi bật, thông số Stream/FPS/Codec dạng thẻ hiện đại và badge trạng thái online/offline.

---

### 2.3 Cơ chế nạp Snapshot & Quản lý Luồng Live
- **Tải Snapshot:**
  - Snapshot được phục vụ qua endpoint `GET /api/snapshot?id=<id>&channel=<ch>&stream=<st>&timeoutSeconds=<sec>`.
  - Backend sử dụng `snapCache` (`internal/server/snapshot_cache.go`) với cơ chế Single-flight + TTL 4 giây để chống nghẽn CPU/RAM khi nhiều client cùng yêu cầu ảnh.
  - Trên Frontend: Trong modal Xem hình (`#gallery-dialog`), hàm `loadGalleryTilesBatched()` nạp theo lô (`GALLERY_BATCH_SIZE = 4`) để không làm tràn socket camera/NVR. Khi xem ảnh lớn (`#lightbox-dialog`), URL blob object được tái sử dụng để tránh fetch lại.
- **Xem Live Stream MJPEG:**
  - Endpoint `GET /api/live?id=<id>&channel=<ch>&fps=6` phục vụ luồng `multipart/x-mixed-replace`.
  - Hàm `livePreview(els, source)` trong `ui-core.js` quản lý phiên live stream:
    - Phiên xem tự động giới hạn 5 phút (`SESSION_MS = 5 * 60 * 1000`) để bảo vệ tài nguyên camera.
    - Cung cấp nút "+5 phút" (`#cd-live-extend`) để gia hạn phiên.
    - Tự động ngắt stream sau 30 giây khi tab trình duyệt bị ẩn (`document.hidden`).
    - Khác với phiên bản cũ, phiên stream sống xuyên suốt khi người dùng chuyển đổi qua lại giữa 7 tab trong Camera Detail.

---

### 2.4 Thanh thao tác nhanh (Quick Actions Toolbar)
- **Hiện trạng:**
  - Trong bảng camera, cột `actions-cell` chỉ có 1 nút "Xem hình" (`data-action="view"`) và 1 nút menu xổ xuống `⋯` (`details.row-menu` chứa: Cấu hình chi tiết, Dò cấu hình, Xem tất cả kênh, Sửa thông tin kho, Xóa khỏi kho).
  - Thanh công cụ trên đầu (`.camera-toolbar`) chỉ có: Ô tìm kiếm (`#camera-search`), Bộ lọc hãng (`#camera-vendor-filter`), Nút "Dò tên kênh (tất cả)" (`#probe-names-btn`), Nút "Xóa các cam đã chọn" (`#bulk-delete-cameras-btn`).
  - **Khoảng trống cần nâng cấp (Gap Analysis):** Cần tích hợp thanh công cụ Quick Actions 1-Click trực tiếp trên từng hàng/thẻ camera để thao tác siêu tốc:
    1. **Live Stream tức thì** (Mở popup hoặc live view mini)
    2. **Chụp Snapshot** (Lấy ảnh mới nhất tải về hoặc preview)
    3. **Điều khiển PTZ nhanh**
    4. **Khởi động lại thiết bị (Reboot)** (Có popup xác nhận an toàn)
    5. **Đồng bộ giờ NTP** (Đồng bộ ngay với máy chủ INUT/Host)

---

### 2.5 Không gian làm việc Camera Detail Workspace (7 Tab Chuyên Nghiệp)
Kiến trúc layout 2 cột hiện đại:
- **Cột trái (Sticky Preview Column - 340px):**
  - Khung ảnh Snapshot preview (`#ce-preview-img-wrap`) với spinner và xử lý lỗi chi tiết.
  - Luồng Live Stream MJPEG (`#cd-live`).
  - Thanh điều khiển preview: Nút "▶ Xem trực tiếp" (`#cd-live-start`), "+5 phút" (`#cd-live-extend`), "Dừng" (`#cd-live-stop`), "Tải lại ảnh" (`#ce-preview-reload`).
  - Bộ chọn kênh camera con / NVR (`#detail-channel`).
- **Cột phải (7 Tab Điều Khiển):**
  1. **Tab 1: Tên & OSD (`#ce-panel-osd`)**:
     - Đổi tên hiển thị trên camera (`#ce-name`).
     - 4 dòng OSD (`#ce-osd-fields`) với ô nhập văn bản và checkbox "Hiện trên hình".
     - Hỗ trợ biến `{name}` để tự động điền tên camera.
     - Endpoint: `GET /api/channel-info`, `POST /api/channel-name`, `POST /api/osd`.
  2. **Tab 2: Chỉnh màu (`#ce-panel-picture`)**:
     - Chế độ **Cơ bản (Lite)**: White Balance, Flip, Rotate90 (với bộ nút xoay Stepper `<` `>`), DayNightColor, ExposureMode, cùng thanh trượt cường độ Bù ngược sáng (BLC), Chống chói (HLC), Dải tương phản rộng (WDR) với output realtime.
     - Chế độ **Đầy đủ (Full)**: Phân nhóm collapsible (Màu sắc, Ảnh chung, Ban đêm NightOptions, Ban ngày phụ NormalOptions).
     - Hệ thống khóa trường tự động dựa trên phản hồi năng lực `GetVideoInputCaps` từ camera (tránh set tham số phần cứng không hỗ trợ).
     - Endpoint: `GET /api/picture`, `POST /api/picture`.
  3. **Tab 3: Video Encoder (`#ce-panel-video`)**:
     - Hiển thị từng Stream (Chính main, Phụ 1 sub, Phụ 2 sub2).
     - Các trường: Rộng, Cao, Codec (`H.265`, `H.264`, `H.264H`, `H.264B`, `MJPG`), FPS (tự động kiểm tra trần năng lực qua `POST /api/fps-capability`), Bitrate (Kbps), Kiểu CBR/VBR, GOP (I-frame), Smart Codec (H.264+/H.265+).
     - Endpoint: `POST /api/fps-capability`, `POST /api/apply`.
  4. **Tab 4: Âm thanh (`#ce-panel-audio`)**:
     - Bảng hiển thị thông số Stream, Audio Codec (`AAC`, `G.711A`, `G.711U`, `PCM`...) và trạng thái bật/tắt.
     - Nút 1-click **"Bật audio AAC (stream chính)"** phục vụ chuẩn phát trực tuyến Web/iPhone/Android.
     - Endpoint: `POST /api/apply` với `setAudioAAC: true`.
  5. **Tab 5: Mạng & Wi-Fi (`#ce-panel-network`)**:
     - Đọc/ghi cấu hình Card mạng: Interface, DHCP vs Static IP, Subnet Mask, Gateway, DNS 1 & 2, hiển thị MAC và MTU.
     - Cấu hình Wi-Fi: SSID, Mật khẩu mới, Nút **"Quét Wi-Fi"** hiển thị danh sách AP kèm chất lượng sóng (`linkQuality %`).
     - Checkbox xác nhận rủi ro mất kết nối bắt buộc trước khi bấm Lưu.
     - Endpoint: `GET /api/network`, `POST /api/network`, `GET /api/wifi`, `POST /api/wifi`, `POST /api/wifi-scan`.
  6. **Tab 6: Bàn xoay PTZ (`#ce-panel-ptz`)**:
     - Bàn phím điều khiển 8 hướng (↖ ↑ ↗ ← ● → ↙ ↓ ↘) hỗ trợ thao tác giữ chuột/chạm (`pointerdown`/`pointerup`).
     - Thanh trượt tốc độ xoay (1–8).
     - Các nút chức năng: Zoom + (Tele), Zoom − (Wide), Focus Far (Nét xa), Focus Near (Nét gần).
     - Endpoint: `POST /api/ptz`.
  7. **Tab 7: Bảo trì (`#ce-panel-maint`)**:
     - Nút khởi động lại camera (`#maint-reboot-btn` qua `POST /api/reboot`).
     - Trạng thái Thẻ nhớ / Ổ cứng NVR (`#maint-storage` qua `GET /api/storage`): dung lượng tổng, đã dùng, số phân vùng, trạng thái Healthy/Error, nút Format ổ đĩa (`POST /api/storage`).
     - Tình trạng ghi hình NVR (`#maint-record-health` qua `GET /api/nvr/health`).
     - Ngày giờ & NTP (`#maint-device-time` qua `GET /api/device-time`, `POST /api/device-time`): tính toán độ lệch giờ (drift) so với host, nút lấy giờ trình duyệt, bật đồng bộ NTP.
     - Lịch tự khởi động lại định kỳ (`#maint-autoreboot` qua `GET /api/autoreboot`, `POST /api/autoreboot`).
     - Nút chuyển nhanh sang trang Xem lại timeline video.

---

### 2.6 Bộ điều phối Chỉnh hàng loạt (Smart Bulk Wizard)
- **Hiện trạng:**
  - Chọn camera qua chip và danh sách checkbox (`#bulk-camera-picker`).
  - Thanh tóm tắt **"Sẽ đổi"** (`#bulk-summary`) tự động sinh chip tổng kết những thiết lập đã bật (Codec, Res, Smart, GOP, Bitrate, OSD, Audio). Bấm vào chip sẽ tự động cuộn mượt đến thẻ cài đặt tương ứng.
  - Chọn luồng áp dụng: Main (0), Sub1 (1), Sub2 (2).
  - Chọn kênh: Hỗ trợ cú pháp chuỗi linh hoạt (ví dụ: `1`, `1-8`, `1,3,5`).
  - Thực thi tuần tự (Sequential Execution) qua SSE stream `/api/apply` hiển thị live progress bar và log chi tiết.
  - Đổi mật khẩu hàng loạt trong vùng Danger Zone (`details.danger-zone`) và tự động cập nhật lại kho `cameras.yaml`.
- **Khoảng trống cần nâng cấp (Gap Analysis):**
  1. Chưa có nút 1-click **"Áp dụng Chuẩn Bida (Golden Template)"** để lập tức nạp cấu hình tối ưu nhất cho toàn bộ hệ thống Bida (H.264/H.265 Baseline, GOP 50/100, 5 phút/segment, AAC Audio remux, 0% CPU transcoding).
  2. Chưa có hệ thống **Phát hiện Giới hạn An toàn & Cảnh báo Clamping (Safety Limits Inspector)** trên UI (ví dụ: cảnh báo màu vàng khi người dùng chọn FPS > 25 trên độ phân giải 4K, hoặc bitrate quá cao gây nghẽn băng thông switch).

---

### 2.7 Chẩn đoán NVR & Quét Kênh Con Tự Động
- **Quét & Liên kết Đầu ghi (`#nvr-link-dialog`):**
  - Nhập IP, cổng, tài khoản, mật khẩu, hãng đầu ghi (Hikvision/Dahua/Tiandy).
  - Nút **"Quét đầu ghi"** (`POST /api/nvr/scan`): Tự động phát hiện danh sách camera IP con đang kết nối vào NVR, kiểm tra xem camera con có thẻ nhớ cục bộ hay không, tự động map gợi ý với camera trong kho.
  - Nút **"Lưu liên kết"** (`POST /api/nvr/link`): Lưu metadata `nvrId`, `nvrChannel`, `noStorage` vào `cameras.yaml` để các camera không có thẻ nhớ sẽ tự động lấy video xem lại/tải clip từ đầu ghi.
- **Giám sát Sức khỏe Ghi hình & Watchdog Tự Chữa Lỗi:**
  - Tab Đầu ghi (`#cameras/nvr`) hiển thị bảng NVR kèm trạng thái sức khỏe (Tốt `healthy`, Đang sửa `repairing`, Cảnh báo `warning`, Lỗi ghi hình `critical`).
  - Checkbox **"Tự sửa ghi hình"** (`nvrWatchdog` qua `POST /api/nvr/watchdog`): Bật tiến trình nền watchdog 15s tự động phát hiện kênh bị tắt lịch hoặc sai chế độ để bật lại ghi hình 24/7.
  - Checkbox **"Lấy giờ từ INUT"** (`nvrSyncTimeFromHost`): Tự động đồng bộ đồng hồ NVR với máy chủ NTP/Host để chống sai lệch mốc thời gian trên timeline.
  - Nút **"Kiểm tra ngay"** (`POST /api/nvr/health/check`): Ép chạy kiểm tra sức khỏe tức thì.

---

## 3. Ma Trận API Backend & Cấu Trúc Dữ Liệu Chi Tiết

### 3.1 Bảng Tổng Hợp Endpoint Backend Dùng Cho `/#cameras`

| Phương Thức | Tuyến Đường (Route) | Payload Yêu Cầu | Định Dạng Phản Hồi | Mục Đích Kỹ Thuật |
|---|---|---|---|---|
| `GET` | `/api/cameras` | None | `[]deviceView` | Lấy danh sách toàn bộ camera & NVR trong kho |
| `POST` | `/api/cameras` | `cameraUpsertReq` | `deviceView` | Thêm mới hoặc cập nhật thông tin camera vào kho |
| `POST` | `/api/cameras/delete` | `{id: string, timeoutSeconds?: int}` | `{ok: true}` | Xóa 1 camera khỏi kho inventory |
| `POST` | `/api/cameras/delete-bulk` | `{ids: []string}` | `{ok: bool, deleted: int, skipped: int}` | Xóa hàng loạt camera đã chọn |
| `POST` | `/api/probe` | `{id: string, timeoutSeconds?: int}` | `[]camera.StreamInfo` hoặc `{streams, serialNumber, port}` | Thăm dò live các thông số video/audio/serial |
| `POST` | `/api/fps-capability` | `{id, channel, stream, width, height, codec, timeoutSeconds}` | `camera.FPSCapability` (`{currentFps, maxFps, source}`) | Đọc trần FPS an toàn cho cấu hình độ phân giải |
| `POST` | `/api/apply` | `bulk.Request` (`{deviceIds, profile, timeoutSeconds}`) | SSE Stream (`bulk.Event`) | Áp dụng cấu hình hàng loạt tuần tự |
| `POST` | `/api/password` | `{deviceIds, newUsername, newPassword, timeoutSeconds}` | SSE Stream (`bulk.Event`) | Đổi mật khẩu hàng loạt & cập nhật kho |
| `GET` | `/api/snapshot` | Query: `id, channel, stream, timeoutSeconds, _r` | Binary `image/jpeg` | Lấy ảnh snapshot (qua Singleflight TTL cache) |
| `GET` | `/api/live` | Query: `id, channel, fps, _r` | `multipart/x-mixed-replace` | Xem luồng trực tiếp MJPEG |
| `POST` | `/api/ptz` | `{id, channel, code, speed, start, timeoutSeconds}` | `{ok: true}` | Điều khiển xoay/quét/zoom/focus PTZ |
| `POST` | `/api/reboot` | `{id, timeoutSeconds}` | `{ok: bool, note: string}` | Khởi động lại thiết bị từ xa |
| `GET` | `/api/storage` | Query: `id, timeoutSeconds` | `[]dahua.StorageDevice` | Đọc dung lượng, phân vùng và sức khỏe thẻ/ổ cứng |
| `POST` | `/api/storage` | `{id, name, timeoutSeconds}` | `{ok: bool, note: string}` | Format định dạng phân vùng lưu trữ |
| `GET` | `/api/channel-info` | Query: `id, channel, timeoutSeconds` | `{name, osdLines, osdEnabled, osdSupported}` | Lấy tên kênh và 4 dòng text OSD |
| `POST` | `/api/channel-name` | `{id, channel, name, timeoutSeconds}` | `{ok: true}` | Đổi tên kênh trên camera |
| `POST` | `/api/osd` | `{id, channel, lines, enabled, timeoutSeconds}` | `{ok: bool, appliedLines: int}` | Ghi 4 dòng OSD lên hình |
| `POST` | `/api/channel-names`| `{ids: []string, timeoutSeconds}` | `{count: int, ok: true}` | Dò và cập nhật tên kênh toàn bộ camera |
| `GET` | `/api/picture` | Query: `id, channel, timeoutSeconds` | `{color, options, caps, capsError}` | Đọc màu sắc và thông số hình ảnh chi tiết |
| `POST` | `/api/picture` | `{id, channel, color, options, timeoutSeconds}` | `{color, options, ok: true}` | Ghi đè các thông số màu sắc đã thay đổi |
| `GET` | `/api/network` | Query: `id, timeoutSeconds` | `dahua.NetworkConfig` | Đọc cấu hình card mạng Ethernet |
| `POST` | `/api/network` | `{id, interface, dhcpEnable, ipAddress, subnetMask, gateway, dns, timeoutSeconds}` | `{ok: bool, note: string}` | Ghi cấu hình IP tĩnh cho camera |
| `GET` | `/api/wifi` | Query: `id, timeoutSeconds` | `map[string]map[string]any` | Đọc cấu hình kết nối Wi-Fi |
| `POST` | `/api/wifi` | `{id, interface, ssid, password, timeoutSeconds}` | `{ok: true}` | Ghi kết nối mạng Wi-Fi |
| `POST` | `/api/wifi-scan` | `{id, timeoutSeconds}` | `{devices: []dahua.WiFiAP}` | Yêu cầu camera quét các mạng Wi-Fi lân cận |
| `GET` | `/api/device-time` | Query: `id, timeoutSeconds` | `dahua.TimeConfig` | Đọc ngày giờ, timezone và máy chủ NTP |
| `POST` | `/api/device-time` | `dahua.TimeConfig` + `{id, timeoutSeconds}` | `dahua.TimeConfig` | Ghi ngày giờ và máy chủ NTP |
| `GET` | `/api/autoreboot` | Query: `id, timeoutSeconds` | `dahua.AutoRebootSchedule` | Đọc lịch tự khởi động lại định kỳ |
| `POST` | `/api/autoreboot` | `dahua.AutoRebootSchedule` + `{id, timeoutSeconds}` | `{ok: true}` | Lưu lịch tự khởi động lại định kỳ |
| `POST` | `/api/nvr/scan` | `{host, port, username, password, vendor, timeoutSeconds}` | `{rows: []nvrScanRow}` | Quét các kênh camera con trên NVR |
| `POST` | `/api/nvr/link` | `{nvr, mappings: []nvrMapping}` | `{ok: bool, linked: int}` | Lưu liên kết NVR fallback cho camera |
| `GET` | `/api/nvr/channels`| Query: `id, timeoutSeconds` | `{channels: []nvrChannelInfo}` | Lấy danh sách kênh NVR thực tế |
| `GET` | `/api/nvr/health` | Query: `id` | `nvrHealthReport` | Đọc báo cáo sức khỏe ghi hình NVR |
| `POST` | `/api/nvr/health/check`| `{id}` | `nvrHealthReport` | Ép kiểm tra sức khỏe NVR tức thì |
| `POST` | `/api/nvr/watchdog`| `{id, enabled, syncTimeFromHost}` | `{ok: true}` | Cấu hình watchdog tự sửa ghi hình |

---

### 3.2 Các Cấu Trúc Dữ Liệu Go Cốt Lõi

```go
// 1. Dữ liệu Camera Inventory (internal/server/api.go)
type deviceView struct {
    ID                  string        `json:"id"`
    Name                string        `json:"name"`
    Host                string        `json:"host"`
    Port                int           `json:"port"`
    Vendor              config.Vendor `json:"vendor"`
    Username            string        `json:"username"`
    Password            string        `json:"password"`
    SerialNumber        string        `json:"serialNumber,omitempty"`
    NVRID               string        `json:"nvrId,omitempty"`
    NVRChannel          int           `json:"nvrChannel,omitempty"`
    NVRName             string        `json:"nvrName,omitempty"`
    ChannelName         string        `json:"channelName,omitempty"`
    NoStorage           bool          `json:"noStorage,omitempty"`
    IsNVR               bool          `json:"isNvr,omitempty"`
    NVRWatchdog         bool          `json:"nvrWatchdog,omitempty"`
    NVRSyncTimeFromHost bool          `json:"nvrSyncTimeFromHost,omitempty"`
}

// 2. Cấu hình Profile Áp dụng Hàng loạt (internal/camera/camera.go)
type Profile struct {
    SetResolution bool     `json:"setResolution"`
    Width         int      `json:"width"`
    Height        int      `json:"height"`
    SetCodec      bool     `json:"setCodec"`
    Codec         string   `json:"codec"`        // H.265, H.264, H.264H, H.264B, MJPG
    CodecProfile  string   `json:"codecProfile"`
    SetSmartCodec bool     `json:"setSmartCodec"`
    SmartCodec    bool     `json:"smartCodec"`
    SetAudioAAC   bool     `json:"setAudioAAC"`
    SetOSD        bool     `json:"setOsd"`
    OSDLines      []string `json:"osdLines"`
    SetGOP        bool     `json:"setGop"`
    GOP           int      `json:"gop"`
    SetFPS        bool     `json:"setFps"`
    FPS           int      `json:"fps"`
    SetBitrate    bool     `json:"setBitrate"`
    Bitrate       int      `json:"bitrate"`
    BitrateMode   string   `json:"bitrateMode"`  // CBR, VBR
    Streams       []int    `json:"streams"`      // [0]=Main, [1]=Sub1, [2]=Sub2
    Channel       int      `json:"channel"`
    Channels      []int    `json:"channels"`
}

// 3. Thông tin Luồng Video Live Probe (internal/camera/camera.go)
type StreamInfo struct {
    Channel     int      `json:"channel"`
    Stream      int      `json:"stream"`
    Width       int      `json:"width"`
    Height      int      `json:"height"`
    FPS         int      `json:"fps"`
    Compression string   `json:"compression"`
    Profile     string   `json:"profile"`
    AudioCodec  string   `json:"audioCodec"`
    AudioEnable bool     `json:"audioEnable"`
    SmartCodec  bool     `json:"smartCodec"`
    GOP         int      `json:"gop"`
    BitrateKbps int      `json:"bitrateKbps"`
    BitrateMode string   `json:"bitrateMode"`
    Name        string   `json:"name,omitempty"`
    OSDLines    []string `json:"osdLines,omitempty"`
}

// 4. Báo cáo Sức khỏe NVR (internal/server/nvr_health.go)
type nvrHealthReport struct {
    ID                 string              `json:"id"`
    Status             nvrhealth.Status    `json:"status"` // healthy, repairing, warning, critical, unknown
    Reasons            []nvrhealth.Reason  `json:"reasons"`
    Reachable          bool                `json:"reachable"`
    WatchdogEnabled    bool                `json:"watchdogEnabled"`
    SyncTimeFromHost   bool                `json:"syncTimeFromHost"`
    HostTime           string              `json:"hostTime"`
    HostTimeTrusted    bool                `json:"hostTimeTrusted"`
    NVRTime            string              `json:"nvrTime,omitempty"`
    ClockDriftSeconds  int64               `json:"clockDriftSeconds"`
    UptimeMinutes      int64               `json:"uptimeMinutes,omitempty"`
    StorageHealthy     bool                `json:"storageHealthy"`
    StorageTotalBytes  int64               `json:"storageTotalBytes"`
    StorageUsedBytes   int64               `json:"storageUsedBytes"`
    StorageGrowing     bool                `json:"storageGrowing"`
    Channels           []nvrhealth.Channel `json:"channels"`
    LastCheck          string              `json:"lastCheck"`
    NextCheck          string              `json:"nextCheck,omitempty"`
}
```

---

## 4. Đặc Tả Golden Template Bida Cho Smart Bulk Wizard

Đối chiếu trực tiếp từ tài liệu kỹ năng `camera-naming` (`.agents/skills/camera-naming/SKILL.md`):

1. **Quy chuẩn đặt tên & định danh:**
   - Tên hiển thị Camera: `CameraXX` (Title case đệm 2 số, ví dụ `Camera01`, `Camera08`).
   - Monitor ID Shinobi: `cameraXX` (Lowercase đệm 2 số, ví dụ `camera01`, `camera08`).
   - Inventory ID: `<ip_address>:<port>` (ví dụ `192.168.1.195:37777`).
2. **Quy chuẩn Video Codec & Stream Engine:**
   - Remux Stream Mode: `copy` (0% CPU Transcoding).
   - Video Codec: `H.264` (Main) hoặc `H.265` Baseline.
   - GOP (Khoảng cách I-frame): Cố định 50 hoặc 100 frames.
   - Segment Cutoff: Đúng 5 phút (`cutoff: 5`, `segment_time 300`).
   - Bitrate: CBR 2048 Kbps – 4096 Kbps (FHD 1080p).
3. **Quy trình Probe & Chuẩn hóa Âm thanh (Audio Workflow):**
   - Bước 1: Probe âm thanh camera (`audioEnable`, `audioCodec`).
   - Bước 2: Nếu phát hiện âm thanh không phải AAC (`pcm_alaw`, `pcm_mulaw`, `G.711A`, `G.711U`...), bắt buộc tự động gửi lệnh chuyển encoder của camera về `AAC` (`setAudioAAC: true`).
   - Bước 3: Đọc lại (read-back). Nếu thành công -> cấu hình Shinobi `acodec: "copy"`, `record_acodec: "aac"`. Nếu không hỗ trợ -> tắt âm thanh trên Shinobi (`acodec: "no"`, `record_acodec: "no"`) để chống giật lag luồng media.

---

## 5. Kiến Trúc Kế Thừa & Khuyến Nghị Nâng Cấp Cho Giai Đoạn Tiếp Theo

1. **Bổ sung Grid View (Card Grid) bên cạnh Table View:**
   - Thêm nút chuyển đổi View Switcher (Icon Lưới ⊞ / Icon Bảng ☰) được lưu vào `localStorage`.
   - Card View hiển thị snapshot thumbnail trực tiếp (tự động tải qua singleflight snapshot cache), badge thương hiệu màu sắc, thông số luồng gọn gàng và bộ nút Quick Actions.
2. **Tích hợp Quick Actions Toolbar:**
   - Thêm các nút tắt 1-click trực tiếp trên hàng và card: Xem Live Stream tức thì, Chụp Snapshot tải nhanh, Điều khiển PTZ, Reboot thiết bị và Đồng bộ giờ NTP tức thì.
3. **Nâng cấp Smart Bulk Wizard với Golden Template 1-Click & Safety Limiter:**
   - Tích hợp nút **"⚡ Áp dụng Chuẩn Bida (Golden Template)"** tự động điền sẵn các thông số chuẩn (H.264/H.265, GOP 50, Bitrate 2048 CBR, Bật Audio AAC).
   - Thêm bộ kiểm tra an toàn realtime (Safety Inspector): Hiển thị cảnh báo màu vàng khi người dùng cấu hình FPS > 25 trên 4K (3840x2160) hoặc bitrate > 8192 Kbps.
4. **Bảo tồn 100% Khả Năng Tương Thích & Test Suite:**
   - Giữ nguyên toàn bộ các thuộc tính `data-testid` (`camera-row`, `camera-search`, `camera-vendor-filter`, `bulk-delete-cameras`, `task-tab-*`, `detail-tab-*`, `detail-channel-name`, `detail-save-*`, `nvr-list`, `nvr-status`, `nvr-watchdog`, `nvr-sync-time`, `bulk-summary`, `bulk-password`, `bulk-apply`, `result-list`, v.v.) để đảm bảo toàn bộ Playwright UI tests trong `tests/ui/` tiếp tục vượt qua 100%.

