# Original User Request

## Initial Request — 2026-08-24T18:53:11+07:00

Nâng cấp toàn diện giao diện `#redbida` trong ứng dụng Web KSP-Cam (`:2028`): thiết kế hiện đại, sang trọng, trực quan; tích hợp Trung tâm Tri thức & Quy trình Onboarding chuẩn (Knowledge & Onboarding Workflow Hub) trực tiếp trên giao diện để quản trị viên dễ dàng theo dõi, kiểm soát toàn bộ thông số chuẩn, đối chiếu trạng thái MQTT `/private/i_sets` và cập nhật quy trình trong tương lai.

Working directory: /home/ksp/ksp-camera-auto
Integrity mode: development

## Requirements

### R1. Thiết kế Giao diện RedBida Sang trọng & Hiện đại (`web/static/` & `#view-redbida`)
- Nâng cấp layout `#view-redbida` với phong cách thiết kế UI cao cấp (Modern Dark/Light Glassmorphism, Responsive Grid, thẻ thống kê trực quan, typography chuẩn chỉn).
- Tích hợp **Trung tâm Tri thức Chuẩn (System Knowledge Hub)** hiển thị trực quan 4 trụ cột Onboarding:
  1. **Branding & Giao diện Quán**: `ui_title`, `ui_bg` (có visual preview gradient live), `logo_header`, `logo_header_text`, `logo_livestream`, `ui_scoreboard`, `ui_tabs_links` (20 tab INI preview/generator), `custom_hashtags` (mẫu chuẩn `#<UITitle> #BILLIARDSlive #INUTlive #highlightsports`).
  2. **Video Streaming & Go2RTC Engine**: `camera_count`, `toolbar_show_count`, `video_config` (`range=72`), `hls_using_go2rtc` (`true`), nút 1-click kích hoạt `button_generate_go2rtc_stream`.
  3. **Shinobi NVR Authentication & Group Sync**: `shinobi_camera_id` (Group Key), `shinobi_group_key`, `shinobi_token` (API key 0.0.0.0 Streams/Videos), `shinobi_monitor_token` (API key 0.0.0.0 Monitors), các thông số Golden Template (`cutoff: 5`, `copy` remux).
  4. **Hệ thống & An ninh**: `frpc_config` (quản lý proxy qua Redbida), `ggcode` (`G-SFSDZPR95Z`), NTP / Time sync, RAM Watchdogs.
- Bổ sung bộ công cụ **Preset / One-Click Onboarding Generator**: Nhập Tên Quán (`ui_title`), Số Camera (`camera_count`) và Group Key Shinobi -> Tự động sinh đầy đủ toàn bộ bộ tham số chuẩn (`ui_tabs_links`, `custom_hashtags`, `ui_bg`, v.v.) và cho phép 1-click Apply qua MQTT `/private/i_sets`.

### R2. Đảm bảo Chuẩn Giao tiếp MQTT & Catalog Backend (`internal/redbida`, `internal/server`)
- Đảm bảo toàn bộ thao tác ghi/đọc từ Web UI tuân thủ cấu trúc gói tin chuẩn `ota-mqtt`:
  - Gửi `/private/i_sets`: `{"info": {"<key>": "<val>", ...}}`.
  - Gửi `/private/i_gets`: `{"info": ["<key1>", "<key2>", ...]}`.
- Bổ sung định nghĩa meta, nhãn hiển thị, gợi ý quy chuẩn và phân nhóm logic cho toàn bộ các key mới (`logo_header`, `logo_header_text`, `ui_bg`, `camera_count`, `toolbar_show_count`, `hls_using_go2rtc`, `button_generate_go2rtc_stream`, v.v.) trong `internal/redbida/catalog.go`.
- Hỗ trợ Live Preview trực quan cho `ui_bg` (render background CSS) và `logo_header` / `logo_livestream` (render hình ảnh preview thực tế).

### R3. Kiểm thử & Đóng gói Toàn diện (Testing & Verification)
- Viết / cập nhật Unit Test đầy đủ cho `internal/redbida` và `internal/server` (đảm bảo 100% pass).
- Kiểm tra tính tương thích frontend (`redbida.js`, `index.html`, `style.css`), đảm bảo không có lỗi console JavaScript.
- Kiểm thử thực tế trên target node qua Ansible / curl / API endpoint `/api/redbida/refresh` và `/api/redbida/apply`.
- Đồng bộ và commit git toàn bộ thay đổi.

## Acceptance Criteria

### Giao diện & Trải nghiệm Người dùng (UI/UX)
- [ ] Giao diện `#view-redbida` hiển thị trực quan, phân chia rõ ràng các khối Tri thức Onboarding, Thẻ thống kê trạng thái, Bảng chỉnh sửa Key và Khối Preset Onboarding 1-click.
- [ ] Visual preview hoạt động mượt mà cho `ui_bg` CSS gradient và `logo_header` / `logo_livestream`.
- [ ] Bảng cấu hình hỗ trợ lọc theo nhóm, tìm kiếm realtime, lọc "Chỉ thay đổi", hiển thị badge Risk / Secret / Editable rõ ràng.

### Backend & MQTT Protocol Integrity
- [ ] `internal/redbida/catalog.go` khai báo đầy đủ metadata cho toàn bộ danh mục key tiêu chuẩn của hệ sinh thái Bida.
- [ ] Mọi lệnh apply và refresh từ Web UI giao tiếp chính xác qua MQTT broker `127.0.0.1:12369` theo đúng cấu trúc `{"info": ...}` và xác thực đọc lại (read-back verification).

### Build & Test Suite
- [ ] Tất cả Go unit tests (`go test ./...`) pass 100%.
- [ ] Build tĩnh binary `make build-all` thành công không có lỗi.
- [ ] Binary mới được deploy và kiểm thử hoạt động trơn tru trên môi trường thực tế.
