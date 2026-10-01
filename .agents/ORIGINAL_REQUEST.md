# Original User Request

## Request — 2026-10-01
User request:
"Ok stop camera07 ghi hình đi"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_204_148` (`77.88.204.148`) — RUBY BILLIARD CLUB (group `na`)
- Target Camera: `Camera07` (IP `192.168.1.65`)

## Requirements

### R1. Dừng ghi hình Camera07 [HOÀN THÀNH]
- Gọi Shinobi API chuyển trạng thái `camera07` sang `stop`:
  `GET /xsXHWnSbLiOPnPSjlLF71gvA45rk2S/monitor/na/camera07/stop` -> Trả về `{"ok": true, "msg": "Monitor mode changed : stop"}`.
- Cập nhật cơ sở dữ liệu MySQL `Monitors` (`mode = 'stop'`).
- Tiến trình FFmpeg của `camera07` đã dừng hoàn toàn (tổng số tiến trình FFmpeg giảm từ 14 xuống 12).
- Các camera từ `camera01` đến `camera06` vẫn tiếp tục ghi hình bình thường không bị gián đoạn.

## Acceptance Criteria
- [x] C1: Dừng ghi hình và luồng của Camera07 thành công.
- [x] C2: Số tiến trình FFmpeg của camera07 được giải phóng hoàn toàn.
- [x] C3: 6 camera còn lại (camera01 đến camera06) tiếp tục ghi hình ổn định.
