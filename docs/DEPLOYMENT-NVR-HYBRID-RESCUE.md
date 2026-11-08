# DEPLOYMENT-NVR-HYBRID-RESCUE.md — Sổ Tay Quy Trình Triển Khai Thực Địa: Không Thẻ Nhớ (`Diskless`), Lai Mở Rộng (`Hybrid SD+NVR`) & Cứu Hộ Box Hỏng (`Zero-Idle Box Rescue`)

> **Mục đích tài liệu:** Quy trình chuẩn từng bước (`SOP / Field Runbook`) dành cho kỹ thuật viên vận hành và AI Agent khi triển khai bộ 3 gói thuần Go (`kspcam :2028` + `go-shinobi-ksp :8080` + `redbida-go :2023` + `ksp-go2rtc :1984`) từ máy chủ điều phối `root@172.16.5.180`. Mọi lệnh, kịch bản kiểm tra và quy trình hoàn trả (`Failback`) đều được đúc kết trực tiếp từ các ca triển khai thực chiến trên `inut_202_222`, `inut_205_123`, `inut_205_124` và `inut_206_14`.

---

## 1. Bảng Chọn Chế Độ Vận Hành Theo Tình Huống Thực Địa

| Tình Huống Tại Quán / CLB | Chế Độ Vận Hành | Luồng Xem Live | Luồng Lưu & Xem Lại Clip | Tải CPU Nền |
|---|---|---|---|---|
| **Box mới không gắn thẻ nhớ** (quán có sẵn đầu ghi NVR hoặc Cam cắm thẻ nhớ riêng) | **Chế độ 1: Diskless NVR-Direct** | `go-shinobi-ksp` HLS hoặc `go2rtc` | `100%` kéo từ NVR / Cam-SD vào RAM `/dev/shm` (`0` ghi thẻ Box) | Siêu nhẹ (`load ~0.5`) |
| **Box có gắn thẻ nhớ nhưng gánh nhiều cam (`8–16` cam 2K)** khiến thẻ chỉ lưu được `6–18 giờ` | **Chế độ 2: Hybrid SD + NVR `72h`** | `go-shinobi-ksp` HLS / `go2rtc` | Các giờ gần nhất đọc thẳng từ thẻ nhớ `/media/usb1` (`0` băng thông NVR); các ngày cũ hơn tự nối khít từ ổ cứng NVR lên đủ `72 giờ` | Không tăng thêm tải nền |
| **1 Box trong CLB nhiều Box bị hỏng phần cứng** (`5.122` chết, cần `5.123` đứng ra gánh ngay `16` bàn) | **Chế độ 3: Zero-Idle Box Rescue** | **Chỉ mở `go2rtc` khi có khách bấm xem** (`mode="idle"`, `0` tiến trình `ffmpeg` nền) | `100%` kéo từ NVR vào RAM `/dev/shm` khi khách bấm xem/cắt (`0` ghi đè thẻ nhớ Box cứu hộ) | **`0.0%` khi không có khách xem** |

---

## 2. Kỷ Luật An Toàn Bắt Buộc Trước Mọi Ca Triển Khai (`Golden Rules`)

1. **Bảo toàn `100%` nhận diện thương hiệu của quán**: Tuyệt đối không ghi đè `ui_title`, `ui_bg`, `logo_header`, `logo_header_text`, `logo_livestream`, `custom_hashtags` hoặc mật khẩu trong `/root/ota-mqtt/change_ok/`.
2. **An toàn thẻ nhớ & chống treo khởi động (`Zero External Storage in /etc/fstab`)**:
   - Tuyệt đối không format thẻ nhớ khi chưa có xác nhận 2 bước.
   - Không bao giờ đưa `/dev/mmcblk0p1` hoặc `/dev/mmcblk1p1` vào `/etc/fstab`.
   - Khóa vĩnh viễn `netwatch.service` (`stop` + `disable` + `mask`) và đặt `kernel.sysrq = 0`.
3. **Bất biến `0% CPU Encode`**: Toàn bộ đường ống rút và cắt clip chỉ dùng `-c:v copy` + `-c:a copy` (hoặc `-an` nếu âm thanh G.711 không tương thích MP4) kèm `-tag:v hvc1 -movflags +faststart`.
4. **Cấu hình `config.yaml` không trùng khóa**: Thư viện `gopkg.in/yaml.v3` từ chối khởi động nếu file YAML bị lặp tên khóa (ví dụ lặp `kspcam_config_path:`). Luôn kiểm tra `pm2 list` đạt `online` sau khi sửa file cấu hình.
5. **Lưu ý lệnh PM2**: Luôn khởi động lại từng dịch vụ trên từng dòng riêng (`pm2 restart camera` rồi mới `pm2 restart inut`), không truyền 2 tên app chung một lệnh `pm2 restart`.

---

## 3. Quy Trình Triển Khai Chế Độ 1 & Chế Độ 2 (`Diskless NVR-Direct` & `Hybrid SD + NVR 72h`)

### Bước 1: Triển khai bộ 3 gói Go qua Ansible trên `172.16.5.180`
```bash
ssh root@172.16.5.180
cd /build/armbian-build/ansible

# 1. Cài đặt / nâng cấp kspcam (:2028) có sẵn khối bridge /dev/shm:
make ksp-cam <ten_node>

# 2. Cài đặt / nâng cấp go-shinobi-ksp (:8080) bật sẵn nvr_direct_fallback: true:
make go-shinobi <ten_node>

# 3. Cài đặt / nâng cấp redbida-go (:2023) nối sẵn cut_spool_dir /dev/shm/redbida-cuts:
make redbida-go <ten_node>
```

### Bước 2: Xác minh cổng điều khiển thật của đầu ghi NVR trong `/opt/ksp-cam/cameras.yaml`
> **Kinh nghiệm thực địa sống còn:** Khi import camera từ đường dẫn RTSP của Shinobi (ví dụ `:554` hoặc `:1554`), trường `port:` trong `cameras.yaml` mặc định điền cổng `80`. Tuy nhiên nhiều CLB đổi cổng HTTP ISAPI của đầu ghi Hikvision sang cổng khác (như `:82` tại Magic Arena Đà Nẵng) hoặc dùng đầu ghi Dahua cổng `:37777` / `:8888`.

- **Nếu dùng đầu ghi Hikvision (kênh gắn trực tiếp `192.168.1.50:<port>-c31..c42`)**:
  1. Kiểm tra cổng HTTP ISAPI thật đang mở trên IP đầu ghi (thường là `80`, `81`, `82`, `8080`).
  2. Cập nhật `port: <cong_http_that>` trong `/opt/ksp-cam/cameras.yaml` và chạy `systemctl restart kspcam.service`. (`kspcam` sẽ tự động dò cổng RTSP tùy chỉnh như `1554` qua `/ISAPI/Security/adminAccesses` và rút clip tốc độ cao qua `ISAPI DownloadTimeRange`).
- **Nếu dùng Camera IP lẻ ghi vào đầu ghi Dahua / Hikvision riêng biệt (như `inut_202_222`)**:
  1. Mở giao diện Web `kspcam (:2028)` -> mục **Liên kết NVR** -> bấm **Quét NVR** (nhập IP đầu ghi, cổng `37777` hoặc `80`, tài khoản đầu ghi).
  2. Lưu liên kết từng `Camera01..CameraN` vào đúng kênh trên đầu ghi (`nvrId` + `nvrChannel`).

### Bước 3: Kiểm tra thông suốt Chế độ Diskless / Hybrid
```bash
# Kiểm tra bảng ánh xạ cầu nối trên Box:
curl -s -H "X-Kspcam-Key: $(cat /root/ota-mqtt/change_ok/shinobi_token)" \
  http://127.0.0.1:2028/api/v1/bridge/status | python3 -m json.tool

# Kiểm tra tổng số clip (thẻ nhớ + NVR nối khít 72 giờ) trên cổng :2023:
curl -s "http://127.0.0.1:2023/api/dashboard/state?tab=0" | python3 -c '
import sys, json
vids = json.load(sys.stdin)["activeCamera"]["videos"]
sd = [v for v in vids if not v["video"]["filename"].startswith("v_nvr_")]
nv = [v for v in vids if v["video"]["filename"].startswith("v_nvr_")]
print(f"Tổng={len(vids)} clip | Thẻ nhớ={len(sd)} | Nối dài từ NVR={len(nv)}")
'
```

---

## 4. Quy Trình Cứu Hộ Box Hỏng 1-Click (`Chế Độ 3: Zero-Idle Box Rescue SOP`)

Áp dụng khi một CLB có từ 2 Box trở lên cùng nối chung mạng màn hình tỉ số (`lan0`, ví dụ `192.168.5.0/24`) và mạng camera/đầu ghi (`eth0`, ví dụ `192.168.1.0/24`). Giả sử **Box A (`inut_205_122` — IP `192.168.5.122`, tên miền `store-magic-1.video.io.vn`)** bị hỏng phần cứng, ta dùng **Box B (`inut_205_123` — IP `192.168.5.123`)** đứng ra cứu hộ.

### Bước 1: Kiểm tra trước khi kích hoạt (`Pre-Flight Check` trên `172.16.5.180`)
1. **Xác định file backup Shinobi của Box hỏng trên `172.16.5.180`**:
   ```bash
   ls -lh /build/ota/backup_shinobi/<box_hong>/monitors_*.json
   cat /build/ota/backup_nha_yen/<box_hong>/ota-mqtt/change_ok/frpc_config
   ```
2. **Đảm bảo IP ảo của Box hỏng trên `lan0` không còn xung đột ARP** (rút dây mạng khỏi Box hỏng hoặc xác nhận cổng mạng Box hỏng đã tắt):
   ```bash
   ssh root@<ip_vpn_box_cuu_ho> "ping -I lan0 -c 2 -W 1 192.168.5.122 || echo 'IP 192.168.5.122 TRỐNG — SẴN SÀNG CHIẾM VIP'"
   ```
3. **Đảm bảo `/opt/ksp-cam/config.yaml` trên Box cứu hộ dùng khóa Admin nội bộ `127.0.0.1` (`control_monitors: "1"`)** cho mục `shinobi.api_key` để `kspcam` có quyền tạo các monitor `mode: "idle"` trên `go-shinobi-ksp`.

### Bước 2: Chạy lệnh Kích Hoạt Cứu Hộ 1-Click từ máy chủ `172.16.5.180`
Chạy trực tiếp kịch bản chuẩn dưới đây trên `172.16.5.180` (chỉ cần thay biến ở 6 dòng đầu):

```bash
DEAD_NODE="inut_205_122"
RESCUE_HOST="77.88.205.123"
DEAD_VIP="192.168.5.122/24"
DEAD_SUBDOMAIN="store-magic-1"
NVR_HTTP_PORT=82
BACKUP_JSON="/build/ota/backup_shinobi/${DEAD_NODE}/monitors_magic3_${DEAD_NODE}.json"

python3 - "$DEAD_NODE" "$RESCUE_HOST" "$DEAD_VIP" "$DEAD_SUBDOMAIN" "$NVR_HTTP_PORT" "$BACKUP_JSON" << 'PY'
import sys, json, subprocess

dead_node, rescue_host, dead_vip, dead_sub, nvr_port, backup_path = sys.argv[1:]
mons_raw = open(backup_path, encoding="utf-8").read()

remote_py = f"""
import json, urllib.request, subprocess, time
mcp_req = {{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {{
        "name": "kspcam_rescue_box",
        "arguments": {{
            "action": "activate",
            "peerName": {json.dumps(dead_node)},
            "iface": "lan0",
            "vip": {json.dumps(dead_vip)},
            "frpcSubdomain": {json.dumps(dead_sub)},
            "publicSiteUrl": {json.dumps("http://" + dead_sub + ".video.io.vn")},
            "nvrHttpPort": {int(nvr_port)},
            "monitorsJson": {json.dumps(mons_raw)}
        }}
    }}
}}
req = urllib.request.Request(
    "http://127.0.0.1:2028/mcp",
    data=json.dumps(mcp_req).encode(),
    headers={{"Content-Type": "application/json"}}
)
print(urllib.request.urlopen(req, timeout=35).read().decode())
subprocess.run(["pm2", "restart", "inut"], check=True)
"""

subprocess.run(
    ["ssh", "-o", "StrictHostKeyChecking=no", f"root@{rescue_host}", "python3", "-"],
    input=remote_py, text=True, check=True
)
PY
```

### Bước 3: Checklist 6 Điểm Nghiệm Thu Sau Khi Kích Hoạt Cứu Hộ
Chạy kiểm tra nhanh trên Box cứu hộ để xác nhận đạt tuyệt đối cả 6 tiêu chí:
- [ ] **1. Đã nhận IP ảo trên `lan0`**: `ip -4 addr show dev lan0` hiển thị cả IP chính (`192.168.5.123/24`) và IP cứu hộ (`192.168.5.122/24`).
- [ ] **2. `0` tiến trình `ffmpeg` chạy nền cho các cam cứu hộ**: `ps -ef | grep '/usr/bin/ffmpeg' | grep -v grep | wc -l` bằng **đúng số cam gốc** của Box cứu hộ (ví dụ `12` tiến trình, tuyệt đối không sinh thêm `16` tiến trình nền).
- [ ] **3. Dịch kênh tự động chuẩn `1:1` trên IP ảo**:
  - `curl -s "http://192.168.5.122:2023/api/v1/0"` trả về `"camera_id": "camera13"`, `"support_live_password": false`.
  - `curl -s "http://192.168.5.122:2023/api/dashboard/state?tab=0"` trả về đúng số bàn của Box hỏng (`16` tab) kèm `860–900+` clip `v_nvr_*` (`72 giờ`).
- [ ] **4. Xem Live On-Demand qua `go2rtc`**: `curl -sI "http://192.168.5.122:2023/inutjsc/camera13.m3u8"` trả về `HTTP/1.1 200 OK` (`application/vnd.apple.mpegurl`).
- [ ] **5. Tên miền QR `4G` của Box hỏng sống lại**: `curl -s "http://store-magic-1.video.io.vn/api/dashboard/state?tab=0"` trả về `HTTP 200` đầy đủ danh sách clip.
- [ ] **6. Các bàn gốc của Box cứu hộ không bị ảnh hưởng**: `curl -s "http://192.168.5.123:2023/api/dashboard/state?tab=0"` vẫn giữ nguyên `"cameraId": "camera01"` và đúng số tab gốc (`12` tab).

---

## 5. Quy Trình Hoàn Trả Khi Lắp Lại Box Mới (`Failback / Deactivate SOP`)

> **Cảnh báo quan trọng:** Trước khi cắm dây mạng `lan0` cho Box mới sửa xong (`192.168.5.122`), **BẮT BUỘC** chạy lệnh gỡ cứu hộ trên Box cứu hộ trước để nhả IP ảo `192.168.5.122` và nhả subdomain FRPC `store-magic-1`, tránh đụng độ trùng IP!

```bash
# Chạy trên Box cứu hộ (ví dụ 77.88.205.123) để nhả sạch IP ảo + subdomain + kênh cứu hộ:
curl -s http://127.0.0.1:2028/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "kspcam_rescue_box",
      "arguments": {
        "action": "deactivate",
        "peerName": "inut_205_122"
      }
    }
  }' && pm2 restart inut
```

Sau khi chạy lệnh trên:
1. Kiểm tra `ip -4 addr show dev lan0` trên Box cứu hộ đã mất IP `192.168.5.122/24`.
2. Cắm nguồn và dây mạng cho Box `5.122` đã sửa/thay mới và chạy bộ lệnh chuẩn `make ksp-cam` + `make go-shinobi` + `make redbida-go` cho Box mới.

---

## 6. Bảng Chẩn Đoán Nhanh Sự Cố Thực Địa (`Field Troubleshooting`)

| Triệu Chứng Gặp Phải | Nguyên Nhân Gốc | Cách Xử Lý Dứt Điểm Trong 10 Giây |
|---|---|---|
| Kích hoạt cứu hộ báo `status 401: Not Authorized` | `shinobi.api_key` trong `/opt/ksp-cam/config.yaml` đang điền `shinobi_token` (chỉ có quyền xem, `control_monitors: "0"`) | Đổi `shinobi.api_key` sang khóa Admin `127.0.0.1` (`control_monitors: "1"`) trong bảng `API` của `/opt/go-shinobi-ksp/shinobi.sqlite`, rồi `systemctl restart kspcam.service`. |
| Kênh cứu hộ lên tab đúng nhưng danh sách `clips = 0` | Cổng HTTP của đầu ghi trong `cameras.yaml` đang để mặc định `80` trong khi đầu ghi dùng cổng tùy chỉnh (như `82`) | Truyền đúng `"nvrHttpPort": 82` khi gọi `kspcam_rescue_box` (hoặc sửa `port: 82` trong `cameras.yaml` + `systemctl restart kspcam.service`). |
| Mở `/inutjsc/camera13.m3u8` báo `404 Not Found` | `/root/go2rtc.yaml` chưa biên dịch thêm `camera13..camera28` | Chạy `pm2 restart inut` (khi có `rescue_box_json`, `redbida-go` tự động biên dịch lại `/root/go2rtc.yaml` gồm cả các monitor `idle` và khởi động lại `go2rtc_linux_arm`). |
| `pm2 list` báo tiến trình `camera` hoặc `inut` bị `errored` sau khi sửa `config.yaml` | File `config.yaml` bị dán trùng 2 dòng cùng tên khóa (lỗi cú pháp strict của `yaml.v3`) | Kiểm tra `cat -n /opt/go-shinobi-ksp/config.yaml`, xóa dòng khóa bị trùng rồi chạy `pm2 restart camera`. |
