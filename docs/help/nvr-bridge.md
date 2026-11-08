---
id: nvr-bridge
title: Cầu nối NVR / Thẻ nhớ Camera cho Shinobi-Go & RedBida-Go
section: admin
order: 12
keywords:
  - nvr bridge
  - x-kspcam-key
  - v_nvr
  - không thẻ nhớ
  - /dev/shm/kspcam-clips
  - faststart
covers:
  - /api/v1/bridge/status
  - /api/v1/bridge/recordings
  - /api/v1/bridge/clip.mp4
  - /api/v1/bridge/flush-cache
related:
  - xem-lai
  - shinobi-nvr
---

Khi Edge Box không gắn thẻ nhớ `/media/usb1` hoặc thẻ nhớ gặp sự cố vật lý, cầu nối nội bộ **NVR / Cam-SD Bridge** trên `kspcam (:2028)` cho phép `go-shinobi-ksp (:8080)` và `redbida-go (:2023)` đọc danh sách ghi hình, xem tua chậm Slow-Mo `0.05x–8x` và cắt/tải clip MP4 chuẩn `+faststart` trực tiếp từ ổ cứng đầu ghi NVR (Dahua, Hikvision, Tiandy, Raysharp) hoặc thẻ nhớ lắp trong camera IP.

## Nguyên tắc vận hành an toàn (0% CPU Encode)

- **Tuyệt đối không encode bằng CPU**: Toàn bộ quá trình đóng gói sang MP4 sử dụng `100% stream copy` (`-c:v copy`, `-c:a copy` hoặc `-an` nếu luồng âm thanh G.711 không tương thích container MP4), kèm thẻ `-tag:v hvc1` và `-movflags +faststart` để đặt `moov` atom ngay đầu file, tương thích hoàn toàn với iPhone Safari và ứng dụng Ảnh (Photos) trên iOS.
- **Kho đệm RAM `/dev/shm/kspcam-clips`**: Clip được trích xuất qua mạng dây LAN với tốc độ cao vào bộ nhớ RAM `/dev/shm`, tự động giới hạn dung lượng LRU (`120 MB` trên box RAM 1 GB, `240 MB` trên box RAM 2–4 GB) và tự dọn sau `10 phút`.
- **Bảo vệ phần cứng đầu ghi & thẻ nhớ camera**: Giới hạn tối đa `2` luồng trích xuất đồng thời trên mỗi địa chỉ IP đầu ghi NVR (`max_per_nvr: 2`), và khóa cứng `1` luồng tuần tự đối với camera IP lẻ chạy thẻ nhớ riêng.

## Cấu hình xác thực `X-Kspcam-Key`

Cấu hình trong `/opt/ksp-cam/config.yaml`:

```yaml
bridge:
  enabled: true
  api_key: ""
  spool_dir: "/dev/shm/kspcam-clips"
  max_spool_mb: 0
  clip_ttl_seconds: 600
  max_per_nvr: 2
```

Các dịch vụ nội bộ trên cùng thiết bị (`127.0.0.1`) truyền khóa qua header `X-Kspcam-Key` khi gọi nhóm endpoint `/api/v1/bridge/status`, `/api/v1/bridge/recordings`, `/api/v1/bridge/clip.mp4` và `/api/v1/bridge/flush-cache`.
