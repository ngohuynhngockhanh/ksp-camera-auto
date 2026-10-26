---
id: raysharp-ai-events
title: "Cấu hình Camera AI Raysharp & Sự kiện MQTT / Hub"
section: admin
order: 35
keywords: [raysharp, advantech, ai, face detection, human vehicle, license plate, mqtt, eventbus, webhook, longpoll, nvr, n10256n]
ui: ""
covers: ["/api/ai/capabilities", "/api/ai/setting", "/api/ai/linkage", "/api/ai/event-push", "/api/raysharp/raw", "/api/events/status", "/api/events/config", "/api/events/list", "/api/events/stream", "/api/events/poll", "/api/events/webhook/raysharp"]
related: [mcp-server, kho-camera, chinh-hang-loat, shinobi-nvr]
---
## Mục đích

Hỗ trợ quản trị toàn diện dòng **Đầu ghi AI NVR (`N10256N` / `RS-SM-2426`)** và **Camera IP AI (`VC31` / `6M019A`)** của hãng **Raysharp Technology** (Advantech OEM), đồng thời mở rộng khả năng cấu hình thuật toán AI tại biên và xuất bản luồng sự kiện AI / Cảnh báo thời gian thực qua **MQTT Event Bus** và **`inut-bida-hub`**.

## Các tính năng chính

1. **Quản lý cấu hình chuẩn `kspcam` cho vendor `raysharp`**:
   - Tự động nhận diện qua cổng HTTP 80 (`/API/Login/Range`) và đường dẫn RTSP `/rtsp/streaming?channel=<1..256>&subtype=<0|1>`.
   - Hỗ trợ đầy đủ: Đọc thông số (`Probe`), Chỉnh hàng loạt (`Apply` Codec H.264/H.265/H.264+/H.265+, Độ phân giải, FPS, GOP, Bitrate), Tên kênh & OSD, Chụp ảnh Snapshot, Quét kênh IP trên NVR, Mạng IP tĩnh, Đồng bộ giờ NTP, Quản lý ổ đĩa và Xem lại ghi hình.

2. **Cấu hình Thuật toán AI & Liên kết Báo động (`/api/ai/*`)**:
   - **Xem danh sách khả năng AI (`GET /api/ai/capabilities`)**: Kiểm tra trạng thái Bật/Tắt (`On`/`Off`) và ràng buộc loại trừ (`mutual_ability`) của 13 thuật toán AI (`FaceDetection`, `HumanVehicle`, `LicensePlate`, `LineCrossing`, `Intrusion`, `CrossCounting`, `HeatMap`, `CrowdDensity`, `QueueLength`...).
   - **Đọc/Ghi tham số luật AI (`GET/POST /api/ai/setting`)**: Điều chỉnh công tắc bật/tắt (`switch`), độ nhạy (`sensitivity`), đối tượng phát hiện (`Pedestrian`, `Motor Vehicle`), vùng đa giác 8 đỉnh (`rule_rect`) và vạch kẻ (`rule_line`).
   - **Đọc/Ghi liên kết báo động (`GET/POST /api/ai/linkage`)**: Bật/tắt ghi hình sự kiện, đèn chớp Floodlight, còi hú Siren, đẩy HTTP.
   - **Gọi trực tiếp 200+ API gốc (`POST /api/raysharp/raw`)**: Truy cập trực tiếp các API quản lý thư viện khuôn mặt (`AI/FDGroup/*`, `AI/AddedFaces/*`), biển số xe (`AI/PlateGroup/*`), và tìm kiếm ảnh chụp AI (`AI/SnapedFaces/Search`, `AI/SnapedObjects/Search`).

3. **Hệ thống Sự kiện AI & MQTT Event Bus (`/api/events/*`)**:
   - **Thu thập sự kiện chế độ kép (Dual-Mode)**: Kết hợp Long-Polling `/API/Event/Check` (`plus_eventchk: "eventAiPushPic"`) và cổng nhận Webhook (`POST /api/events/webhook/raysharp`).
   - **Xuất bản MQTT & Hub**: Tự động bắn sự kiện JSON kèm ảnh chụp Base64 (tùy chọn) lên MQTT Broker (`kspcam/events/<box_id>/<device_id>/ch<N>/<event_type>`) và đẩy trực tiếp về `inut-bida-hub` (`/api/agent/events`).
