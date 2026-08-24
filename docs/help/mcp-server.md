---
id: mcp-server
title: "Máy chủ MCP (Model Context Protocol)"
section: admin
order: 25
keywords: [mcp, model context protocol, jsonrpc, ai, stdio, sse, api-key, trợ lý ảo, llm, tools, automation]
ui: ""
covers: ["/mcp", "/mcp/", "/mcp/messages"]
related: [dong-lenh, cau-hinh-yaml, shinobi-nvr, redbida]
---
## Mục đích

Máy chủ **Model Context Protocol (MCP)** nhúng trực tiếp trong `kspcam` cung cấp bộ 31 công cụ chuẩn hóa (qua JSON-RPC 2.0) giúp các trợ lý AI (như Antigravity, Claude Desktop, Cursor, Hermes, ChatGPT) trực tiếp dò tìm, cấu hình, chẩn đoán hệ thống camera IP đa hãng (Dahua/KBVision, Hikvision, Tiandy), điều khiển Shinobi NVR và quản lý toàn diện cấu hình RedBida / Bida Onboarding (Node-RED / OTA MQTT).

## Giao thức & Phương thức kết nối

1. **Chế độ dòng lệnh Stdio (`kspcam --mcp`)**:
   - Chạy trực tiếp qua tiến trình cục bộ, giao tiếp qua `stdin` / `stdout` (newline-delimited JSON).
   - Tự động chuyển toàn bộ log sang `stderr` để tránh làm sai lệch khung gói tin JSON-RPC.

2. **Chế độ HTTP / SSE (`/mcp` trên cổng `:2028`)**:
   - **Stream sự kiện SSE (`GET /mcp`)**: Mở kênh Server-Sent Events nhận thông báo và kết quả tác vụ.
   - **Gửi thông điệp (`POST /mcp/messages?sessionId=...`)**: Gửi các lệnh gọi JSON-RPC (`tools/list`, `tools/call`).
   - **Gọi trực tiếp không trạng thái (`POST /mcp`)**: Gửi yêu cầu JSON-RPC trực tiếp và nhận phản hồi JSON ngay lập tức.

## Bảo mật & Xác thực

- Hỗ trợ bảo mật bằng API Key được cấu hình trong `config.yaml` (`mcp.api_key`).
- Client từ xa truyền API Key qua header `X-MCP-Key: <key>`, `Authorization: Bearer <key>`, hoặc tham số URL `?key=<key>`.
- Mặc định cho phép kết nối nội bộ Loopback (`127.0.0.1`, `::1`, `localhost`) mà không cần API Key (`mcp.allow_unauthenticated_loopback: true`).

## Danh mục công cụ (Tool Catalog)

- **Quản lý kho Camera**: `kspcam_list_cameras`, `kspcam_upsert_camera`, `kspcam_delete_camera`, `kspcam_probe_camera`.
- **Cấu hình & Thực thi tuần tự**: `kspcam_apply_profile`, `kspcam_set_channel_name`, `kspcam_set_osd`, `kspcam_reboot_camera`, `kspcam_change_password`.
- **Dò tìm & Chẩn đoán**: `kspcam_scan_lan`, `kspcam_try_password`, `kspcam_wifi_scan`, `kspcam_get_network`, `kspcam_get_nvr_health`, `kspcam_get_recordings`, `kspcam_get_snapshot`.
- **Quản lý Shinobi NVR**: `shinobi_list_monitors`, `shinobi_add_monitor`, `shinobi_edit_monitor`, `shinobi_delete_monitor`, `shinobi_sync_to_shinobi`, `shinobi_sync_from_shinobi`, `shinobi_sync_inventory`, `shinobi_change_monitor_state`, `shinobi_get_videos`.
- **RedBida & Onboarding**: `redbida_list_catalog`, `redbida_get_keys`, `redbida_set_keys`, `redbida_apply_onboarding_preset`, `redbida_trigger_go2rtc`, `redbida_get_time_status`.
