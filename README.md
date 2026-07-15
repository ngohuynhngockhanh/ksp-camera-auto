# ksp-camera-auto

Công cụ **chỉnh cấu hình hàng loạt** cho camera IP **Hikvision** và **Dahua/KBVision**,
điều khiển qua **giao diện web**. Một file binary tĩnh, nhẹ, thuần Go — không cgo, không SDK
đóng, dễ deploy và build ra ARM.

Chỉnh được cho nhiều cam cùng lúc:

- **Độ phân giải** (main / sub stream)
- **Codec & profile**: H.264 / H.264 High / H.264 Baseline / H.265 / MJPEG
- **Smart Codec / H.265+** (bật / tắt)
- **Audio → AAC**

Áp dụng **tuần tự từng cam một cho an toàn**, kèm **log quá trình trực tiếp** (thấy từng bước
của từng cam ngay khi chạy), và **đọc lại (read-back) để xác nhận** mỗi thay đổi thực sự có hiệu lực.

## Chạy nhanh

```bash
# Build (máy hiện tại)
make build

# Chạy — mở http://<ip>:2028, đăng nhập admin / inut12345
./kspcam --addr 0.0.0.0:2028
```

Tài khoản/mật khẩu giao diện và cổng lấy từ `config.yaml` (xem `config.example.yaml`); mặc định
là `:2028` và `admin` / `inut12345`.

## Build đa kiến trúc

Tất cả đều là binary tĩnh (`CGO_ENABLED=0`), một file, không phụ thuộc runtime:

```bash
make build-all      # dist/kspcam-linux-amd64, -linux-armv7 (arm32), -linux-arm64
```

Copy file tương ứng lên thiết bị và chạy — không cần cài gì thêm.

## Cách dùng (giao diện web)

1. **Đăng nhập** `admin` / `inut12345`.
2. **Quét mạng** (tùy chọn) — tìm cam trong LAN bằng ONVIF / Dahua / Hikvision SADP, hoặc quét
   một subnet bằng `nmap`. Bấm "Thêm vào kho" để nạp nhanh.
   > Lưu ý: quét UDP chỉ thấy cam **cùng mạng LAN** (không qua NAT). Qua NAT thì thêm cam thủ công
   > bằng IP:cổng, hoặc dùng nmap để quét subnet reachable.
3. **Kho camera** — thêm cam (IP, cổng, hãng, tài khoản). Bấm **Dò** để xem cấu hình luồng hiện tại.
4. **Chỉnh hàng loạt** — tick chọn cam, chọn luồng (Main/Sub1/Sub2) + kênh, bật những thứ muốn đổi
   (độ phân giải / codec / smart codec / AAC), bấm **Áp dụng**.
5. **Log** — theo dõi tiến trình từng cam, từng bước, trực tiếp.

## Kết nối tới camera (quan trọng)

Các cổng "cấu hình" của camera là **giao thức nhị phân độc quyền**, không phải HTTP:

| Hãng | Cổng mặc định | Giao thức | Trạng thái |
|---|---|---|---|
| **Dahua / KBVision** | 37777 (hoặc 8888) | **DVRIP** (nhị phân + JSON-RPC) | ✅ thuần Go, đã test cam thật |
| **Hikvision** (qua LAN) | 80 | **ISAPI / HTTP** | ✅ thuần Go (`internal/isapi`) |
| **Hikvision** (qua 8000) | 8000 | HCNetSDK (cgo, build `hiksdk`) | ✅ đã test NVR thật qua NAT |

- **Dahua/KBVision**: clone giao thức **DVRIP** thuần Go (đăng nhập băm 2 bước + `configManager`).
  Chỉ cần NAT cổng 37777. Đã kiểm thử trên cam thật.
- **Hikvision qua LAN (cổng 80)**: điều khiển qua **ISAPI/HTTP** thuần Go — build mặc định, chạy
  ARM dễ. Dùng khi deploy trên máy cùng mạng với cam/NVR.
- **Hikvision qua cổng 8000 (NAT)**: cổng 8000 là giao thức đóng, mã hoá — **không** clone hộp-đen
  được. Giải pháp: build tùy chọn **`hiksdk`** (cgo) nhúng HCNetSDK, mang **đúng ISAPI XML** đó đi
  qua `NET_DVR_STDXMLConfig`. Đã test đọc/ghi cấu hình trên đầu ghi **DS-7108NI-Q1/M** thật qua NAT.

### Build bản Hikvision-8000 (`hiksdk`)

Cần bộ **HCNetSDK Linux** (tải từ nguồn chính gốc Hikvision; KHÔNG commit vào repo):

```bash
make build-hiksdk HIKSDK=/path/to/HCNetSDK      # thư mục có incEn/ và lib/
KSPCAM_HIKSDK_PATH=<HIKSDK>/lib ./kspcam-hiksdk --addr 0.0.0.0:2028
```

Bản này dùng cgo nên nặng hơn và cần SDK theo kiến trúc (amd64 sẵn; ARM cần SDK ARM). Bản mặc định
(`make build`) vẫn thuần Go, không cgo — Dahua + Hikvision-qua-LAN.

## Kiến trúc

```
cmd/kspcam            Điểm vào: nạp config, chạy web server :2028
internal/config       Config YAML + kho camera (cameras.yaml, sửa từ UI)
internal/dahua        Client DVRIP thuần Go (37777): framing, login, Encode get/set
internal/isapi        Hikvision ISAPI: HTTP Digest + XML StreamingChannel (get/set)
internal/hik          Adapter Hikvision (ISAPI qua HTTP)
internal/camera       Lớp Camera chung (Probe / Apply) + factory theo hãng
internal/bulk         Điều phối áp dụng TUẦN TỰ + phát sự kiện tiến trình
internal/discovery    Quét mạng: ONVIF (3702), Dahua (37810), Hik SADP (37020), nmap
internal/server       Web server, đăng nhập session, API JSON + stream log
web/                  Giao diện nhúng (go:embed)
```

## Bảo mật

- Mật khẩu camera **chỉ nằm ở runtime** (`cameras.yaml`, đã gitignore) — không bao giờ commit,
  không bao giờ lộ ra API JSON.
- Đăng nhập giao diện so sánh hằng-thời-gian; session bằng cookie HttpOnly.

## Kiểm thử

```bash
go test ./...          # unit + mock (không cần phần cứng)
```

Test chạm cam thật được bật bằng biến môi trường (không hardcode thông tin đăng nhập), ví dụ:

```bash
KSPCAM_LIVE_DAHUA=host:port KSPCAM_LIVE_DAHUA_AUTH=user:pass \
  go test ./internal/dahua -run TestLive -v
```

## Ghi chú vận hành

- Đổi độ phân giải/codec sẽ **khởi động lại encoder** → RTSP đang xem sẽ rớt và kết nối lại.
  Nên làm trong khung giờ bảo trì.
- Camera **từ chối độ phân giải không hỗ trợ** (báo lỗi rõ ràng) — chọn giá trị model đó hỗ trợ.
- Đổi codec: có model **âm thầm bỏ qua** giá trị không hỗ trợ → công cụ đọc lại để báo đúng kết quả.
