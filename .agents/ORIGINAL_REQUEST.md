# Original User Request

## Request — 2026-10-01
User request:
"Thế smart codec 2048 đi"

Working directory: `/home/ksp/ksp-camera-auto`

Target Nodes:
- `inut_205_122` (`77.88.205.122`) — Magic Arena Đà Nẵng
- `inut_205_123` (`77.88.205.123`) — Magic Arena Đà Nẵng
- `inut_205_124` (`77.88.205.124`) — Magic Arena Đà Nẵng
- Central NVR: Hikvision `DS-7764NI-M4` at `192.168.1.50:82`

## Requirements

### R1. Cấu hình SmartCodec 2048 Kbps trên toàn bộ 64 kênh NVR [HOÀN THÀNH]
- Áp dụng cấu hình chuẩn hóa trên 64 kênh chính (`101` .. `6401`):
  - Độ phân giải: `2560x1440` (4MP)
  - Chuẩn nén: `H.265`
  - Bitrate: `2048 Kbps VBR` (`vbrUpperCap = 2048`, `vbrLowerCap = 32`)
  - SmartCodec: `true` (H.265+)
- Đọc lại đối chiếu (Read-back verification) đạt 100% (64/64 kênh).

### R2. Nghiệm thu hoạt động của các luồng ghi hình [HOÀN THÀNH]
- Toàn bộ các node `122`, `123`, `124` tiếp tục ghi hình luồng 4MP 2048 Kbps ổn định với 88 tiến trình FFmpeg hoạt động liên tục.

## Acceptance Criteria
- [x] C1: Toàn bộ 64 kênh trên NVR được cập nhật về 2560x1440, H.265, 2048 VBR, SmartCodec true.
- [x] C2: Xác nhận đọc lại 64/64 kênh khớp chính xác.
- [x] C3: Hệ thống ghi hình trên 3 node hoạt động ổn định không gián đoạn.
