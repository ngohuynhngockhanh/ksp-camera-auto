# Original User Request

## Request — 2026-10-06
User request:
"4.141 online vpn nhưng khách vô wifi quét ko đc router có pass ko tác động router chỉ xem rồi coi inut có lỗi gì ko tranh chủ tối ưu hoá luôn. Pass roiter gateway á HWTCD2DC31B6"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_204_141` (`77.88.204.141`)
- Router Gateway Password: `HWTCD2DC31B6` (READ-ONLY: KHÔNG TÁC ĐỘNG ROUTER, CHỈ XEM NẾU CẦN)

## Requirements

### R1. Khảo sát hiện trạng mạng & tìm nguyên nhân khách vào Wi-Fi không quét/truy cập được
- Kiểm tra các card mạng trên `inut_204_141` (`ip a`, `ip route`, `ip neigh`).
- Kiểm tra địa chỉ IP LAN và IP Virtual Alias (`eth0_virtual_ip` / `192.168.1.254` hoặc tương đương).
- Kiểm tra mạng Wi-Fi và gateway router: gateway IP, DHCP range, subnet của khách kết nối Wi-Fi.
- Kiểm tra router gateway (kiểm tra Web UI / port / status với pass `HWTCD2DC31B6` nếu cần, tuyệt đối không sửa đổi router).
- Kiểm tra dịch vụ phục vụ khách trên box: Node-RED (:2023), Shinobi (:8080), go2rtc (:1984), mDNS / Avahi, iptables / firewall rules.
- Xác định vì sao khách kết nối Wi-Fi không quét/mở được trang clip/live (khác subnet, AP Client Isolation, IP alias sai dải, hoặc iptables chặn).

### R2. Tối ưu hóa hệ thống toàn diện trên `inut_204_141`
- Mở rộng phân vùng ZRAM Swap (100% RAM vật lý) với thuật toán `lz4`, gỡ bỏ `mem_limit` (`echo 0 > mem_limit`), tối ưu `vm.swappiness = 80`, `vm.vfs_cache_pressure = 100`.
- PM2 Memory Guard: Giới hạn heap chủ động cho Node.js (`camera: (120 + N*8)M`, `inut: 280M`, `ota-mqtt: 120M`), xả log vào `/dev/null`.
- Triệt tiêu ngòi nổ tự sát Node-RED: Loại bỏ hoàn toàn các lệnh `echo b > /proc/sysrq-trigger`, `rm -rf /tmp`, và nhánh `[ "$CPU" -eq 8 ]`. Hạ ngưỡng cảnh báo RAM về `60 / 80 / 100`.
- Cấu hình PM2 service ordering: Cập nhật `pm2-root.service` chờ MariaDB (`After=mariadb.service`, `Wants=mariadb.service`).
- Cài đặt Smart Watchdog (`shinobi-camera-watchdog`) quét mỗi 60 giây.
- Cài đặt Storage Guard (`shinobi-storage-guard`) cron 15 phút/lần bảo vệ thẻ nhớ `/media/usb1/<group_key>`.
- Chuẩn hóa Shinobi: `wall_clock_timestamp_ignore = '0'`, `hls_time = '2'`, `max_keep_days = '3'`, bật cờ dọn file mồ côi trong `conf.json`.
- Cập nhật binary tĩnh `kspcam` ARM64 mới nhất.

### R3. Khắc phục lỗi truy cập mạng của khách & Nghiệm thu
- Xử lý cấu hình trên iNut (chỉnh IP alias, iptables, listening interfaces hoặc hướng dẫn khách/router nếu do AP isolation).
- Nghiệm thu tính ổn định của hệ thống, RAM khả dụng, tiến trình FFmpeg và khả năng truy cập.

## Acceptance Criteria
- [ ] C1: Khảo sát hiện trạng và xác định chính xác lý do khách vào Wi-Fi không quét được.
- [ ] C2: Triển khai tối ưu hóa toàn diện (ZRAM, PM2 guard, sysrq, watchdog, storage guard).
- [ ] C3: Đảm bảo dịch vụ web/portal trên iNut lắng nghe và định tuyến chính xác.
- [ ] C4: Báo cáo kết quả đầy đủ bằng định dạng checklist (không dùng bảng).
