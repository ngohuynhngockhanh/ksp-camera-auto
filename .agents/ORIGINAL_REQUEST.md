# Original User Request

## Request — 2026-09-29
User request:
"Okay nhé cái đó viết tải liệu thôi viết xong deploy ksp-cam lên 5.74,5.97,5.21,5.22 và 76 ạ xem thử nó lỗi sao ko ghi hình á"

Working directory: `/home/ksp/ksp-camera-auto`

Target Nodes:
1. `inut_205_74` (`77.88.205.74`)
2. `inut_205_97` (`77.88.205.97`)
3. `inut_205_21` (`77.88.205.21`)
4. `inut_205_22` (`77.88.205.22`)
5. `inut_205_76` (`77.88.205.76`)

## Requirements

### R1. Viết tài liệu đánh giá kỹ thuật NVR Advantech / Raysharp / TVT & Lộ trình Kspcam
- Soạn thảo tài liệu kỹ thuật toàn diện `docs/nvr_evaluation_advantech_raysharp.md`:
  - Giải mã chi tiết nguồn gốc thiết bị từ User Manual (`RS-SM-2426 NVR 5.0UI` - Raysharp Technology OEM cho các dự án chính phủ/công nghiệp như Advantech).
  - Đối chiếu với NVR thực địa TVT Digital NVMS-9000 tại `4.61`.
  - Phân tích chi tiết các cổng mạng (80, 554, 9000), giao thức RTSP, ONVIF Profile S/G/T, XML RESTful, và CGI.
  - Lộ trình thiết kế kiến trúc adapter cho `ksp-cam` (Go interfaces, zero cgo).

### R2. Khảo sát hiện trạng & Chẩn đoán lỗi không ghi hình trên 5 node
- Kiểm tra chi tiết trên từng node: `205_74`, `205_97`, `205_21`, `205_22`, `205_76`:
  - Thông số hệ thống: CPU, RAM, Swap/ZRAM, nhiệt độ, uptime, lịch sử reboot.
  - Dung lượng và trạng thái thẻ nhớ `/media/usb1` (đầy ổ 100%, lỗi I/O, hay rác mồ côi).
  - Trạng thái Shinobi (`Monitors`, `conf.json`), số tiến trình FFmpeg đang chạy.
  - Phân tích nguyên nhân chính xác tại sao camera không ghi hình trên từng box.

### R3. Triển khai `ksp-cam` trên cả 5 node
- Đóng gói binary tĩnh `kspcam` ARM64.
- Cài đặt cấu hình `/opt/ksp-cam/config.yaml`, `/opt/ksp-cam/cameras.yaml` và systemd unit `kspcam.service` (cổng `:2028`).
- Kích hoạt dịch vụ và đảm bảo Web UI / API `:2028` hoạt động ổn định.

### R4. Xử lý triệt để nguyên nhân lỗi không ghi hình & tối ưu toàn diện trên cả 5 node
- Dọn dẹp thẻ nhớ và thiết lập van xả an toàn `shinobi-storage-guard.sh` (chạy cron 15p/lần).
- Cập nhật thời hạn lưu trữ database `max_keep_days = 2 - 3`.
- Bật ZRAM Swap (1.9 GB) và tối ưu `vm.swappiness = 60` chống tràn RAM.
- Gỡ bỏ ngòi nổ `sysrq-trigger` trong Node-RED (nếu có).
- Fix thứ tự khởi động `pm2-root.service` chờ MariaDB và nâng cấp watchdog FFmpeg.
- Xác nhận các camera đã ghi hình trở lại bình thường trên cả 5 box.

## Acceptance Criteria
- [ ] C1: Tài liệu `docs/nvr_evaluation_advantech_raysharp.md` được tạo đầy đủ, chuyên sâu và chuẩn xác.
- [ ] C2: Báo cáo nguyên nhân gốc rễ cụ thể vì sao camera không ghi hình trên từng node trong số 5 node.
- [ ] C3: `kspcam` được triển khai và chạy nền ổn định trên cổng `:2028` của cả 5 node.
- [ ] C4: Cả 5 node đều ghi hình ổn định, FFmpeg hoạt động đầy đủ, có file video mới tạo với kích thước chuẩn (> 0 Byte).
