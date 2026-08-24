---
id: redbida
title: "Quản lý Cấu hình RedBida (Node-RED / OTA MQTT)"
section: admin
order: 16
keywords: [redbida, bida, nodered, node-red, mqtt, ota, change_ok, catalog, key, broker]
ui: "#redbida"
covers: ["/api/redbida/catalog", "/api/redbida/refresh", "/api/redbida/apply", "/api/redbida/time-status"]
related: [cau-hinh-yaml, mcp-server]
---
## Mục đích

Cung cấp giao diện quản lý và tra cứu tập trung các tham số cấu hình hệ thống **RedBida** (Node-RED và cầu nối OTA MQTT). Giao tiếp an toàn qua giao thức MQTT nội bộ, bảo vệ các khóa nhạy cảm và xác nhận đọc lại (read-back verification) khi áp dụng cấu hình.

## Tính năng chính

1. **Tra cứu danh mục khóa (Catalog)**:
   - Quét danh mục khóa cấu hình từ thư mục `/root/ota-mqtt/change_ok` và dự án Node-RED.
   - Phân nhóm khóa rõ ràng (Giao diện / UI, Hệ thống / System, Bảo mật / Security, Phần cứng / Hardware).

2. **Phân loại rủi ro & An toàn**:
   - **Editable**: Các khóa có thể chỉnh sửa trực tiếp (giao diện, tiêu đề, hiển thị...).
   - **Confirm Required**: Các thao tác cần xác nhận (khởi động lại, lệnh bảo trì).
   - **Protected**: Các khóa chỉ đọc hoặc bảo vệ nhạy cảm (thông tin mạng, mật khẩu gốc).

3. **Cập nhật & Xác nhận đọc lại**:
   - Gửi yêu cầu cập nhật qua topic MQTT riêng tư `/private/i_sets`.
   - Tự động đọc lại qua `/private/i_gets` để đảm bảo giá trị đã được áp dụng thành công.

## Cách sử dụng

1. Vào tab [RedBida](#redbida).
2. Tra cứu hoặc tìm kiếm tham số cấu hình cần chỉnh sửa.
3. Thay đổi giá trị và nhấn **Lưu cấu hình**.
