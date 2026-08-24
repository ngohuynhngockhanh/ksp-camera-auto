# Báo Cáo Chuyển Giao Khảo Sát Codebase `/#cameras` (Handoff Report)

## 1. Observation (Quan Sát Trực Tiếp)
- **Tệp tin Frontend & Routing**:
  - `web/static/index.html` (dòng 119–472): Định nghĩa view `#view-cameras`, task tabs `#camera-task-tabs` (với 4 tabs `list`, `bulk`, `nvr`, `results`), không gian làm việc chi tiết `#camera-detail` (dòng 136–247) chia thành 2 cột: cột trái preview + live MJPEG `#cd-live` và cột phải 7 tabs `#ce-tabs` (`osd`, `picture`, `video`, `audio`, `network`, `ptz`, `maint`).
  - `web/static/app.js` (dòng 409–466): Hàm `renderCameras()` render danh sách camera vào `#cam-tbody` dưới dạng bảng HTML `<tr>` duy nhất, chưa có nút chuyển đổi Grid/Card View.
  - `web/static/app.js` (dòng 1402–1430): `BULK_SETTINGS` định nghĩa 7 thiết lập bulk (codec, res, smart, gop, bitrate, osd, audio) và thanh tóm tắt `#bulk-summary`. Chưa có nút 1-click preset "Áp dụng Chuẩn Bida (Golden Template)" và chưa có cảnh báo an toàn khi chọn FPS > 25 trên 4K.
  - `web/static/app.js` (dòng 1771–1779) & `web/static/ui-core.js` (dòng 141–203): `livePreview` khởi tạo luồng MJPEG `/api/live` tự ngắt sau 5 phút hoặc khi tab bị ẩn 30s.
  - `web/static/app.js` (dòng 606–670): Quản lý danh sách NVR, trạng thái sức khỏe `/api/nvr/health`, watchdog `/api/nvr/watchdog`, và quét kênh con `/api/nvr/scan` (dòng 543–571).
- **Tệp tin Backend & Cấu Trúc Dữ Liệu**:
  - `internal/server/server.go` (dòng 128–160): Đăng ký toàn bộ 30+ endpoints REST API cho camera, probe, apply, snapshot, live, ptz, network, wifi, storage, autoreboot, device-time, nvr health & scan.
  - `internal/server/api.go` (dòng 103–141): Cấu trúc `deviceView` phản ánh đầy đủ thông tin camera/NVR (`id`, `name`, `host`, `port`, `vendor`, `username`, `password`, `serialNumber`, `nvrId`, `nvrChannel`, `noStorage`, `isNvr`, `nvrWatchdog`, `nvrSyncTimeFromHost`).
  - `internal/camera/camera.go` (dòng 44–81, 140–260): Định nghĩa `Profile`, `StreamInfo`, `FPSCapability` và các capability interfaces: `FPSSettings`, `PictureSettings`, `NetworkSettings`, `Rebooter`, `StorageManager`, `DeviceIdentity`.
  - `internal/bulk/bulk.go` (dòng 16–86): Điều phối thực thi tuần tự `Apply` qua SSE stream `Event`.
  - `internal/nvrhealth/health.go` (dòng 11–100): Phân loại sức khỏe NVR (`healthy`, `repairing`, `warning`, `critical`), tính toán `CoveredDuration` và độ lệch giờ/NTP.
- **Tài liệu chuẩn & Kỹ năng**:
  - `.agents/skills/camera-naming/SKILL.md`: Quy định Golden Template cho 8 camera: Remux Stream copy 0% CPU, Codec H.264/H.265, GOP 50/100, 5 phút/segment, Audio AAC probe & conversion.

---

## 2. Logic Chain (Chuỗi Lập Luận)
1. Từ quan sát DOM `index.html` và hàm `renderCameras()` trong `app.js`, hệ thống hiện chỉ hỗ trợ hiển thị bảng truyền thống (`#cam-table`). Để đạt tiêu chí R1 (Modern Glassmorphism & Ergonomic UX), cần bổ sung công tắc chuyển đổi View Switcher (Table / Card Grid) hiển thị thumbnail snapshot tự động tải từ `/api/snapshot` và badge hãng.
2. Từ quan sát `actions-cell` trong bảng camera và toolbar, hiện các thao tác nhanh (Live, Snapshot, PTZ, Reboot, NTP) bị giấu trong menu con `⋯`. Việc đưa Quick Actions Toolbar 1-Click trực tiếp lên hàng/card sẽ cải thiện đáng kể tốc độ vận hành thực địa.
3. Từ quan sát `BULK_SETTINGS` trong `app.js` và `SKILL.md`, người dùng hiện phải tích từng checkbox thủ công khi cấu hình camera mới. Việc tích hợp nút 1-click **"⚡ Áp dụng Chuẩn Bida (Golden Template)"** và cảnh báo vượt ngưỡng an toàn phần cứng (FPS > 25 trên 4K) sẽ tự động hóa hoàn toàn quy trình onboarding camera.
4. Từ quan sát các test suite Playwright trong `tests/ui/` (`cameras.spec.js`, `detail.spec.js`, `bulk.spec.js`, `nvr.spec.js`), toàn bộ các test case định danh DOM qua `data-testid`. Bất kỳ cải tiến giao diện nào cũng bắt buộc phải bảo tồn 100% các selector `data-testid` hiện hữu để đảm bảo test luôn pass.

---

## 3. Caveats (Lưu Ý & Vùng Chưa Khảo Sát)
- Giao diện `/#redbida` và quy chuẩn 20-tab INI thuộc phạm vi của task R2/M3 (đã được khảo sát bởi agent song song).
- Mã nguồn backend Go đã ổn định 100%, không yêu cầu thêm mới endpoint backend mà chỉ tận dụng tối đa 30+ endpoint REST API sẵn có.
- "No caveats." về mặt luồng dữ liệu và tương thích DOM `data-testid`.

---

## 4. Conclusion (Kết Luận Khảo Sát)
Giao diện `/#cameras` hiện tại sở hữu nền tảng kiến trúc rất vững chắc: tách bạch SPA thuần Vanilla JS, hỗ trợ luồng SSE realtime, snapshot caching singleflight, và 7 tab Camera Detail độc lập. Kế hoạch đại tu R1 có thể tiến hành thuận lợi và liền mạch bằng cách:
1. Thêm View Switcher: Chế độ Card Grid Glassmorphism với snapshot thumbnail và Quick Actions Toolbar 1-Click.
2. Nâng cấp Smart Bulk Wizard: Nút 1-Click Golden Template Bida và bộ cảnh báo an toàn Safety Limits.
3. Hoàn thiện trực quan hóa NVR Health & Timeline coverage.
4. Giữ nguyên 100% `data-testid` để bảo toàn Playwright E2E test suites.

---

## 5. Verification Method (Phương Pháp Xác Minh)
1. **Kiểm tra Go Backend & Test Suite:**
   ```bash
   go test ./...
   ```
2. **Kiểm tra UI Playwright Test Suite:**
   ```bash
   npx playwright test
   ```
3. **Kiểm tra Tệp Báo Cáo:**
   - Đọc báo cáo chi tiết: `/home/ksp/ksp-camera-auto/.agents/explorer_survey_cameras/analysis.md`
   - Đọc báo cáo chuyển giao: `/home/ksp/ksp-camera-auto/.agents/explorer_survey_cameras/handoff.md`
