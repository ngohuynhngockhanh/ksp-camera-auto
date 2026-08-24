# Original User Request

## Initial Request — 2026-08-24T14:37:39Z

Nâng cấp và tái cấu trúc toàn diện 2 giao diện trung tâm `/#cameras` (Quản lý kho camera & cấu hình hàng loạt) và `/#redbida` (Trung tâm Tri thức & Onboarding Bida) trong `kspcam` (`:2028`): đạt đỉnh cao thẩm mỹ (Modern Glassmorphism, Micro-interactions), cực kỳ thân thiện (Ergonomic UX, trực quan, thao tác nhanh) và thông minh vượt trội (Tự động kiểm tra chuẩn Golden Template, Preset 1-Click, Live Visual Previews, Smart Diagnostics, Bộ chọn bảng màu Gradient).

Working directory: /home/ksp/ksp-camera-auto
Integrity mode: development

## Requirements

### R1. Nâng Cấp Toàn Diện Giao Diện `/#cameras` Đẹp, Thân Thiện & Khôn Hơn
- **Chế độ hiển thị linh hoạt (Grid / Table View)**: Cho phép chuyển đổi giữa dạng Danh sách (Table) và dạng Thẻ trực quan (Grid Cards) với ảnh Snapshot thumbnail tự động tải, badge hãng (Dahua/Hikvision/Tiandy/ONVIF), độ phân giải, FPS, Codec và trạng thái kết nối.
- **Thao tác nhanh 1-Click (Quick Actions Toolbar)**: Xem Live Stream tức thì, Chụp Snapshot, Điều khiển PTZ nhanh, Khởi động lại thiết bị (Reboot), Đồng bộ giờ NTP ngay từ danh sách.
- **Không gian làm việc Camera Detail chuyên nghiệp**:
  - Cột trái: Live Stream MJPEG độ trễ thấp + ảnh Snapshot với nút phóng to toàn màn hình (Fullscreen) và nút gia hạn luồng thông minh.
  - Cột phải: 7 Tab điều khiển mượt mà (OSD/Tên kênh, Chỉnh màu Lite/Full với thanh trượt realtime, Video/Audio encoder, Mạng & quét Wi-Fi với cột sóng tín hiệu, Bàn xoay PTZ hỗ trợ phím tắt bàn phím, Bảo trì & Lịch tự động khởi động).
- **Bộ điều phối Chỉnh hàng loạt Thông minh (Smart Bulk Wizard)**:
  - Tự động cảnh báo nếu cấu hình vượt quá giới hạn an toàn của thiết bị (ví dụ: FPS > 25 trên 4K).
  - Tích hợp sẵn nút 1-click **"Áp dụng Chuẩn Bida (Golden Template)"** (Audio copy/remux, H.264/H.265 baseline, GOP chuẩn).
- **Chẩn đoán NVR & Quét Kênh Con Tự động**: Báo cáo sức khỏe timeline NVR trực quan, tự động quét và map danh sách camera con từ NVR.

### R2. Nâng Cấp Toàn Diện Giao Diện `/#redbida` Đẹp, Sang Trọng & Khôn Hơn
- **Trợ lý Kiểm tra Chuẩn Tự động (Golden Standard Inspector & Auto-Fix)**:
  - Tự động quét và đối chiếu toàn bộ 15 key cấu hình trên node với quy chuẩn Golden Template.
  - Hiển thị thanh tiến độ chuẩn hóa (% Chuẩn Bida) và nút **"Sửa nhanh 1-Click"** cho bất kỳ thông số nào lệch chuẩn (như `ui_bg` có dấu `;`, `custom_hashtags` còn dấu tiếng Việt, `camera_count` không khớp).
- **Bộ sưu tập Bảng màu Gradient Bida Đẳng cấp (Curated CSS Gradient Palette)**:
  - Tích hợp sẵn 8 mẫu màu Gradient siêu đẹp (Royal Deep Blue Glow, Midnight Emerald Cyber, Cyberpunk Neon, Golden Velvet, Obsidian Carbon, Crimson Elegance, Sapphire Blue, Ruby Luxury) kèm ô chọn màu tùy biến với Live Preview Canvas ngay lập tức.
- **Trình soạn thảo Tab INI Trực quan (Visual 20-Tab INI Editor)**:
  - Hiển thị ma trận 20 tab `[C01]` .. `[C20]` dạng lưới trực quan thay vì ô text thô sơ, cho phép chỉnh sửa từng bàn, sao chép URL nhanh, tự động đồng bộ tên quán vào dòng `vid_play_label`.
- **Trình sinh Hashtag Tự động Thông minh**: Tự động loại bỏ dấu tiếng Việt chuẩn hóa Unicode (NFC/NFD) ngay khi người dùng gõ tên quán.
- **Bảng Quản trị Key Cao Cấp**: Tìm kiếm tức thì, lọc theo nhóm, phân loại Risk Badge rõ ràng, xem trước hình ảnh logo và màu nền trực tiếp trên từng dòng.

### R3. Kiểm Thử Khắt Khe, Đóng Gói Binary Đa Kiến Trúc & Triển Khai Thực Địa
- Khảo sát sâu toàn bộ luồng DOM, sự kiện JS (`app.js`, `redbida.js`, `ui-core.js`, `style.css`), bảo toàn 100% tính tương thích với backend Go và MQTT protocol.
- Đảm bảo 100% Go Unit Tests (`go test ./...`) và Playwright UI Tests vượt qua không có lỗi.
- Đóng gói static binary đa kiến trúc (`linux/amd64`, `linux/arm64`, `linux/armv7`).
- Triển khai và kiểm thử thực tế trên target node `inut_204_164` và `inut_204_163`.
- Commit và push git toàn bộ thay đổi lên nhánh `main`.

## Acceptance Criteria

### Tính Thẩm Mỹ & Trải Nghiệm (UI/UX)
- [ ] Giao diện `/#cameras` và `/#redbida` khoác lên diện mạo Modern Glassmorphism đồng nhất, bóng bẩy, hiển thị mượt mà trên cả desktop và mobile.
- [ ] Tính năng Grid/Table view trên `/#cameras` hoạt động mượt mà với snapshot thumbnail và quick actions.
- [ ] Bảng màu Gradient 8 mẫu và Live Preview Canvas trên `/#redbida` phản hồi tức thì.
- [ ] Bộ Inspector tự động đối chiếu Golden Standard tính toán % chuẩn xác và hỗ trợ sửa nhanh 1-click.

### Độ Ổn Định & Giao Thức Backend
- [ ] Toàn bộ API `/api/cameras`, `/api/probe`, `/api/apply`, `/api/redbida/*` và MQTT broker `127.0.0.1:12369` hoạt động ổn định, không lỗi console JS.
- [ ] MCP Server (31 công cụ) tiếp tục hoạt động hoàn hảo cả Stdio và SSE mode.

### Build & Triển Khai
- [ ] Tất cả Go test suites pass 100%.
- [ ] `make build-all` biên dịch thành công.
- [ ] Đã deploy lên các box thực tế và dịch vụ `kspcam` hoạt động ổn định.
