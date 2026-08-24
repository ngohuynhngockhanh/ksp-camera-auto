# Original User Request

## 2026-08-24T09:07:07Z

Triển khai và hoàn thiện cấu hình `ksp-camera-auto` (`kspcam`) trên thiết bị đích `inut_204_163` (`77.88.204.163`), tích hợp đồng bộ thông suốt với Node-RED (cổng :2023) qua module `redbida` (MQTT/Key files) và Shinobi NVR (:8080) để phục vụ bàn giao cho khách hàng.

Working directory: /home/ksp/ksp-camera-auto
Integrity mode: development

## Requirements

### R1. Tự động hóa Triển khai lên Thiết bị Khách hàng (`inut_204_163`)
- Triển khai binary `kspcam` mới nhất kèm cấu hình `/opt/ksp-cam/config.yaml` hỗ trợ đầy đủ: Shinobi NVR (:8080), MCP Server nhúng (:2028), và Redbida/Node-RED integration (:12369 / :2023).
- Cấp phát và đồng bộ tài khoản/API key Shinobi tự động trên `inut_204_163` (không hardcode trong binary).

### R2. Tích hợp Thông suốt giữa KSP-Cam và Node-RED (:2023 / Redbida)
- Kích hoạt và cấu hình module `redbida` trên `kspcam` kết nối tới broker MQTT cục bộ `127.0.0.1:12369` (topics `/private/i_gets`, `/private/i_sets`) và thư mục key catalog `/root/ota-mqtt/change_ok`.
- Đảm bảo giao diện Web KSP-Cam và Node-RED (port 2023) đọc/ghi thông số cấu hình dự án của khách hàng chính xác, nhất quán.

### R3. Thăm dò và Thiết lập Camera / Shinobi trên `inut_204_163`
- Quét và probe danh sách camera tại hiện trường, áp dụng chuẩn Golden Template (Audio AAC copy / no, `-tag:v hvc1`, empty input/stream flags).
- Kiểm tra tính ổn định của luồng video, chức năng ghi hình và sức khỏe NVR.

## Acceptance Criteria

### Deployment & Service Verification
- [ ] Dịch vụ `kspcam.service` hoạt động ổn định trên `inut_204_163:2028` với trạng thái `active (running)`.
- [ ] API endpoints `/api/redbida/catalog`, `/api/redbida/refresh`, `/api/shinobi/status` trả về dữ liệu hợp lệ không có lỗi 500.

### Node-RED & MQTT Bridge
- [ ] Kết nối MQTT broker `127.0.0.1:12369` thông suốt giữa `kspcam` và Node-RED.
- [ ] Đọc và ghi key cấu hình qua KSP-Cam Web UI / API phản ánh chính xác vào Node-RED project.

## Follow-up — 2026-08-24T09:21:39Z

[BỔ SUNG YÊU CẦU QUAN TRỌNG TỪ NGƯỜI DÙNG CHO INUT_204_163]

Sau khi hoàn tất cài đặt, người dùng yêu cầu thực hiện ngay các bước sau trên `inut_204_163`:
1. Sử dụng MCP Server của kspcam trên `inut_204_163` (nếu thiếu tool nào thì tự động code thêm tool vào internal/mcp).
2. Quét probe mạng tìm camera/NVR Dahua/KBVision có Serial Number: `AK0C842PAZ39A81` với mật khẩu: `a12345678`.
3. Mục tiêu: Cài đặt và cấu hình 5 camera từ `Camera01` -> `Camera05` (chuẩn hóa mid `camera01` -> `camera05`, kế thừa chuẩn Golden Template: H.265 `-tag:v hvc1`, stream/input flag trống, audio copy/no).
4. Tên quán: Thiết lập tên quán là "CX King Luxury" (cập nhật qua redbida / file change_ok / Node-RED).
5. IP ảo: Kiểm tra ping qua MCP xem dải IP có trống IP .254 không thì add IP ảo .254 vào interface; nếu không được thì thử .253.
6. Quyền API Key & Token Shinobi:
   - Tạo / cấu hình Shinobi API Key với IP restriction `0.0.0.0` (quyền xem và tải clip).
   - Cài đặt `shinobi_monitor_token` (cũng cho phép `0.0.0.0`) và lưu vào `/root/ota-mqtt/change_ok/shinobi_monitor_token` để đồng bộ hoàn toàn cho khách hàng.

## Follow-up — 2026-08-24T09:46:28Z

[ĐIỀU CHỈNH KHẨN CẤP THIẾT BỊ ĐÍCH CHO QUÁN CX KING]

Người dùng vừa xác nhận thiết bị đích chuẩn của quán "CX King Luxury" là `inut_204_164` (IP: `77.88.204.164`) chứ không phải `inut_204_163`.

Hãy chuyển ngay toàn bộ quy trình sang `inut_204_164`:
1. Triển khai kspcam lên `inut_204_164` (`77.88.204.164`).
2. Cấu hình Shinobi API key & token `0.0.0.0`, lưu `shinobi_monitor_token` vào `/root/ota-mqtt/change_ok/shinobi_monitor_token`.
3. Cấu hình tên quán "CX King Luxury" qua Redbida / Node-RED (:2023) / change_ok.
4. Kiểm tra ping IP ảo trên LAN của `inut_204_164` (thử .254, nếu không được thử .253) và gán IP ảo vào interface.
5. Dò tìm NVR SN `AK0C842PAZ39A81` (pass: `a12345678`), cấu hình 5 camera `Camera01` -> `Camera05` theo Golden Template.
