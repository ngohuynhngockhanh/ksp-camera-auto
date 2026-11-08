---
id: box-rescue
title: Cứu Hộ Box Hỏng & Chạy Gánh Không Tải Nền (Box Rescue Mode)
section: admin
order: 13
keywords:
  - cứu hộ box
  - box rescue
  - virtual ip lan0
  - failover
  - go2rtc on-demand
  - chạy gánh
covers:
  - /api/rescue/status
  - /api/rescue/activate
  - /api/rescue/deactivate
related:
  - nvr-bridge
  - shinobi-nvr
---

Khi một CLB Bida hoặc Sân Thể Thao lắp nhiều Edge Box song song (ví dụ Box 1 quản lý 16 bàn ở IP `192.168.5.122`, Box 2 quản lý 12 bàn ở IP `192.168.5.123`) mà một Box gặp sự cố phần cứng, tính năng **Cứu Hộ Box (Box Rescue Mode)** cho phép Box khỏe mạnh bên cạnh đứng ra gánh toàn bộ số bàn của Box hỏng mà **không gây tốn CPU hay dung lượng thẻ nhớ khi rảnh (`0% Idle Load`)**.

## Nguyên lý vận hành Cứu Hộ Box (`Zero-Idle Failover`)

1. **Chiếm quyền IP ảo (`lan0_virtual_ip`) & Tên miền QR (`FRPC Subdomain`)**:
   - Box cứu hộ tự động gắn thêm địa chỉ IP ảo của Box hỏng lên cổng mạng nội bộ (ví dụ `192.168.5.122/24` trên `lan0`) kèm gói tin Gratuitous ARP để toàn bộ màn hình tỉ số tại bàn (`Cameralive`) và điện thoại khách chuyển sang Box cứu hộ ngay lập tức.
   - Tự động mở kèm subdomain FRPC của Box hỏng (ví dụ `store-magic-1.video.io.vn`) để toàn bộ mã QR dán sẵn trên bàn vẫn quét xem được qua 4G.
2. **Tự động dịch kênh theo Host (`Camera Slot Offset`)**:
   - Khi khách hoặc màn hình tỉ số truy cập vào IP/tên miền của Box đang chạy bình thường (`192.168.5.123`): hệ thống giữ nguyên các bàn gốc `camera01..camera12` (`#!/0..11`).
   - Khi khách hoặc màn hình tỉ số truy cập vào IP/tên miền của Box được cứu hộ (`192.168.5.122` hoặc `store-magic-1.video.io.vn`): `redbida-go (:2023)` tự động cộng độ lệch `cameraOffset = +12`, ánh xạ `#!/0..15` và `/api/v1/0..15` sang `camera13..camera28`!
3. **Xem trực tiếp hoàn toàn On-Demand qua `go2rtc` (`0%` CPU khi không có người xem)**:
   - Các kênh cứu hộ `camera13..camera28` được khởi tạo ở chế độ `mode: "idle"` trên `go-shinobi-ksp (:8080)` — tuyệt đối **không chạy tiến trình `ffmpeg` nền** và **không ghi đè vào thẻ nhớ `/media/usb1`** của Box cứu hộ.
   - Chỉ khi khách hoặc màn hình tỉ số mở xem trực tiếp bàn đó, `go2rtc` mới kết nối luồng RTSP và tự động ngắt ngay khi khách đóng trang.
4. **Xem lại 72 giờ, Slow-Mo `0.05x–8x` & Cắt Clip trực tiếp từ Đầu ghi NVR (`0% CPU Encode`)**:
   - Lịch sử clip, CheckVAR, tua chậm và cắt video của các bàn cứu hộ được kéo trực tiếp theo yêu cầu từ ổ cứng đầu ghi NVR vào kho đệm RAM `/dev/shm/kspcam-clips` với `100% stream copy`.

## Các API Quản lý Cứu Hộ Box trên `kspcam (:2028)`

- `GET /api/rescue/status`: Kiểm tra danh sách các Box đang được cứu hộ, dải kênh gánh và trạng thái IP ảo.
- `POST /api/rescue/activate`: Kích hoạt cứu hộ 1-Click từ file JSON backup monitor Shinobi của Box hỏng (`peerName`, `iface`, `vip`, `frpcSubdomain`, `nvrHttpPort`, `monitorsJson`).
- `POST /api/rescue/deactivate`: Gỡ bỏ IP ảo và dọn sạch các kênh cứu hộ ngay khi Box hỏng đã được sửa chữa/thay mới.
