# Original User Request

## 2026-08-24T13:18:08Z

Mở rộng toàn diện bộ công cụ MCP Server (Model Context Protocol) nhúng trong `ksp-camera-auto` (`kspcam`), bổ sung toàn bộ bộ công cụ RedBida / Bida Onboarding (`redbida_*`), hoàn thiện 100% khả năng tự động hóa và tích hợp cấu hình MCP server để trợ lý AI có thể gọi mọi tool nhanh chóng và chính xác.

Working directory: /home/ksp/ksp-camera-auto
Integrity mode: development

## Requirements

### R1. Xây dựng Bộ Công cụ MCP RedBida & Onboarding Toàn diện (`internal/mcp/tools_redbida.go`)
Bổ sung đầy đủ các công cụ MCP chuyên biệt cho RedBida, giao tiếp chuẩn xác qua MQTT `/private/i_sets` và `/private/i_gets`:
1. `redbida_list_catalog`: Liệt kê danh mục toàn bộ metadata, nhóm chức năng, mức độ rủi ro (risk level), kiểu dữ liệu (json, string, boolean, number) của các key cấu hình Bida.
2. `redbida_get_keys`: Đọc giá trị của một hoặc nhiều key từ `ota-mqtt` broker cục bộ qua topic `/private/i_gets` (payload `{"info": [...]}`).
3. `redbida_set_keys`: Ghi giá trị một hoặc nhiều key tới `ota-mqtt` broker qua topic `/private/i_sets` (payload `{"info": {...}}`) kèm cơ chế xác nhận đọc lại (read-back verification).
4. `redbida_apply_onboarding_preset`: Bộ công cụ Onboarding 1-Click: Tự động tính toán và áp dụng đồng bộ 15 tham số tiêu chuẩn (`ui_title`, `ui_bg` không chấm phẩy, `custom_hashtags` chuẩn hóa không dấu, `ui_tabs_links` 20 tab INI, `camera_count`, `toolbar_show_count`, `hls_using_go2rtc`, `button_generate_go2rtc_stream`, `logo_header`, `logo_header_text`, `shinobi_camera_id`, `shinobi_group_key`, `video_config`, `ui_scoreboard`, `ggcode`).
5. `redbida_trigger_go2rtc`: Kích hoạt gửi cờ `button_generate_go2rtc_stream: "true"` để Node-RED :2023 sinh `/root/go2rtc.yaml`.
6. `redbida_get_time_status`: Kiểm tra trạng thái đồng bộ giờ hệ thống và NTP.

### R2. Tích hợp & Đăng ký MCP Server Hoàn chỉnh (`internal/mcp/`, `cmd/kspcam/`, `docs/`)
- Đăng ký toàn bộ các tool RedBida mới vào `Registry` của `Server` trong `internal/mcp/server.go`.
- Đảm bảo hoạt động thông suốt cả 2 phương thức kết nối:
  - **Stdio Mode**: Chạy trực tiếp qua lệnh CLI `kspcam --mcp --config /opt/ksp-cam/config.yaml`.
  - **HTTP / SSE Mode**: Kết nối tới endpoint `http://127.0.0.1:2028/mcp` kèm cơ chế xác thực API key và loopback không cần mật khẩu.
- Cập nhật tài liệu kỹ thuật `docs/` và `GEMINI.md` liệt kê đầy đủ danh mục tất cả công cụ MCP hỗ trợ.

### R3. Kiểm thử Toàn diện & Đóng gói Multi-Arch (Testing & Verification)
- Viết Unit Tests đầy đủ cho `internal/mcp/tools_redbida.go` và `internal/mcp/server_test.go` (100% pass).
- Kiểm tra tính tương thích JSON-RPC 2.0 (initialize, tools/list, tools/call) cho toàn bộ các tool mới.
- Biên dịch đa kiến trúc `make build-all` (`amd64`, `arm64`, `armv7`).
- Triển khai binary mới lên các node thực tế (`inut_204_164`, `inut_204_163`) và kiểm thử gọi tool qua endpoint `/mcp`.
- Commit và push git toàn bộ thay đổi.

## Acceptance Criteria

### MCP Tool Coverage & Protocol Correctness
- [ ] Lệnh `tools/list` trên MCP Server trả về đầy đủ toàn bộ danh mục công cụ (Camera Inventory, Camera Config, Discovery, Shinobi NVR, RedBida & Onboarding).
- [ ] Các tool `redbida_get_keys` và `redbida_set_keys` giao tiếp chính xác qua MQTT broker `127.0.0.1:12369` theo đúng cấu trúc `{"info": ...}` và nhận diện đúng các trường hợp lỗi/timeout.
- [ ] Tool `redbida_apply_onboarding_preset` sinh chính xác 100% định dạng `ui_tabs_links`, `custom_hashtags` không dấu, `ui_bg` không dấu chấm phẩy, `camera_count` và các token Shinobi.

### Test Suite & Deployment
- [ ] Tất cả Go unit tests (`go test ./...`) pass 100%.
- [ ] Build tĩnh binary `make build-all` thành công.
- [ ] Binary mới được deploy lên `inut_204_164` và `inut_204_163`, kiểm thử gọi MCP tools thành công.
