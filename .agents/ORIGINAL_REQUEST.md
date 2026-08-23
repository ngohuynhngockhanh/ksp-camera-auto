# Original User Request

## 2026-08-23T16:28:39Z

Tích hợp quản lý toàn diện Shinobi NVR vào `ksp-camera-auto` (qua Shinobi REST API), tự động kiểm tra/khởi tạo tài khoản & API Key Shinobi thông qua Ansible playbook `make ksp-bida` (không mã hóa cứng mật khẩu trong Go), và phát triển bộ công cụ MCP Server (Model Context Protocol) hoàn chỉnh nhúng trực tiếp trong binary `kspcam` để phục vụ trợ lý AI.

Working directory: /home/ksp/ksp-camera-auto
Integrity mode: development

## Requirements

### R1. Ansible Automated Shinobi Provisioning (`playbook/roles/app_ksp_bida`)
Nâng cấp role Ansible `app_ksp_bida` trên máy chủ `172.16.5.180` để tự động hóa hoàn toàn việc cấp phát Shinobi:
- Kiểm tra tài khoản người dùng `ngohuynhngockhanh@gmail.com` / `smarthome12345` qua endpoint `http://127.0.0.1:8080/?json=true`.
- Nếu chưa có tài khoản, tự động đăng nhập Super Admin `http://127.0.0.1:8080/super/?json=true` (`ngohuynhngockhanh@gmail.com` / `KSPHondaCity51F79713@`), tạo tài khoản người dùng và sinh Group Key (`mid` / `ke`).
- Gọi API Shinobi tạo API Key mới với ràng buộc IP `127.0.0.1` và cấp full toàn bộ quyền hạn (`all`).
- Ghi các thông số kết nối vào `/opt/ksp-cam/config.yaml` (`shinobi.api_url`, `shinobi.api_key`, `shinobi.group_key`).

### R2. Shinobi Go Client & Full Management Engine (`internal/shinobi`)
Xây dựng module Go client thuần kết nối tới Shinobi REST API:
- Quản lý Monitors đầy đủ: Xem danh sách (`ListMonitors`), Thêm mới monitor (`AddMonitor` với luồng RTSP, codec, audio), Chỉnh sửa (`EditMonitor`), Xóa (`DeleteMonitor`), Bật/Tắt/Khởi động lại stream (`ChangeMonitorState`).
- Cơ chế Đồng bộ 2 chiều (Bi-directional Sync): Đồng bộ danh sách camera từ `cameras.yaml` sang Shinobi monitors và ngược lại.
- Bổ sung các REST API tương ứng trong `internal/server` (`/api/shinobi/status`, `/api/shinobi/monitors`, `/api/shinobi/sync`, `/api/shinobi/videos`) và tích hợp vào giao diện Web nhúng.

### R3. Embedded MCP Server (Model Context Protocol) in `kspcam`
Nhúng trực tiếp MCP Server chuẩn JSON-RPC 2.0 vào binary `kspcam` (hỗ trợ cả Stdio mode qua flag `kspcam --mcp` và HTTP/SSE endpoint `/mcp` trên cổng `:2028`):
- **Camera Inventory Tools**: `kspcam_list_cameras`, `kspcam_upsert_camera`, `kspcam_delete_camera`, `kspcam_probe_camera`.
- **Camera Config & Bulk Tools**: `kspcam_apply_profile`, `kspcam_set_channel_name`, `kspcam_set_osd`, `kspcam_reboot_camera`, `kspcam_change_password`.
- **Discovery & Diagnosis Tools**: `kspcam_scan_lan`, `kspcam_try_password`, `kspcam_wifi_scan`, `kspcam_get_network`, `kspcam_get_nvr_health`, `kspcam_get_recordings`, `kspcam_get_snapshot`.
- **Shinobi Management Tools**: `shinobi_list_monitors`, `shinobi_add_monitor`, `shinobi_edit_monitor`, `shinobi_delete_monitor`, `shinobi_sync_inventory`, `shinobi_change_monitor_state`, `shinobi_get_videos`.
- Hỗ trợ cơ chế bảo mật xác thực API Key cho SSE MCP transport.

### R4. Test Suite, Documentation & Validation
- Viết Unit Tests đầy đủ cho `internal/shinobi` và `internal/mcp`.
- Cập nhật tài liệu `GEMINI.md`, `AGENTS.md` và sinh bài viết trợ giúp trong `docs/help/` (`make docs`).
- Kiểm thử thực địa bằng cách chạy Ansible deploy lên `inut_204_63` và xác nhận kết nối Shinobi API + MCP Server hoạt động hoàn hảo.

## Acceptance Criteria

### Ansible Provisioning
- [ ] Role `app_ksp_bida` trên `172.16.5.180` kiểm tra và tạo thành công user `ngohuynhngockhanh@gmail.com` / `smarthome12345` và API Key Shinobi 127.0.0.1 full quyền.
- [ ] `/opt/ksp-cam/config.yaml` trên box đích chứa đầy đủ `shinobi` section với `api_url`, `api_key`, `group_key`.
- [ ] Tuyệt đối không hardcode mật khẩu Super Admin bên trong mã nguồn Go.

### Shinobi Management Engine
- [ ] Module `internal/shinobi` thực hiện được CRUD Monitor trên Shinobi qua REST API với API Key đã cấp.
- [ ] Tính năng sync 2 chiều giữa `cameras.yaml` và Shinobi monitors hoạt động chính xác.

### MCP Server
- [ ] `kspcam --mcp` khởi động MCP server giao tiếp qua Stdio, phản hồi đúng danh sách `tools/list` và thực thi thành công `tools/call`.
- [ ] Endpoint SSE `/mcp` trên server `:2028` cho phép client MCP từ xa kết nối và gọi công cụ an toàn qua API Key.

### Build & Integrity
- [ ] Build tĩnh đa kiến trúc (`make build-all`) thành công không lỗi.
- [ ] Tất cả Go unit tests (`go test ./...`) và `make docs-check` đều pass 100%.

## Follow-up — 2026-08-23T16:33:47Z

LƯU Ý QUAN TRỌNG TỪ USER VỀ CƠ CHẾ ĐỒNG BỘ (SYNC):
User yêu cầu: KHÔNG tự động chạy sync ngầm liên tục 2 chiều giữa ksp-cam và Shinobi. Thay vào đó, mỗi chiều đồng bộ phải có NÚT BẤM RIÊNG BIỆT (manual trigger):
1. Nút "Đồng bộ từ KSP-Cam sang Shinobi" (Export / Push cameras.yaml -> Shinobi monitors).
2. Nút "Đồng bộ từ Shinobi về KSP-Cam" (Import / Pull Shinobi monitors -> cameras.yaml).
Tạo các REST API endpoint tương ứng cho từng chiều (ví dụ: POST /api/shinobi/sync-to-shinobi và POST /api/shinobi/sync-from-shinobi) cùng các công cụ MCP tương ứng (`shinobi_sync_to_shinobi`, `shinobi_sync_from_shinobi`), kèm nút bấm rõ ràng trên Web UI.
