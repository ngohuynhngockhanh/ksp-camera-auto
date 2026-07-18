---
id: dong-lenh
title: "Dòng lệnh & build"
section: admin
order: 20
keywords: [dòng lệnh, cli, flag, tham số, build, deploy, triển khai, cổng, addr, phiên bản, version, hash mật khẩu, import shinobi, make, static binary, arm]
ui: ""
covers: []
related: [nhap-shinobi, cau-hinh-yaml, dong-lenh]
---
## Mục đích

kspcam là một **file binary tĩnh** duy nhất (thuần Go, không cgo), chạy bằng
dòng lệnh. Bài này liệt kê các tham số dòng lệnh của `kspcam` và các lệnh
`make` để build/triển khai — dành cho người cài đặt/vận hành, không cần biết
lập trình.

## Cách dùng

Chạy chương trình:

```bash
./kspcam --config config.yaml --addr 0.0.0.0:2028
```

Các tham số dòng lệnh:

| Tham số | Mặc định | Ý nghĩa |
|---|---|---|
| `--config` | `config.yaml` | Đường dẫn file cấu hình. |
| `--addr` | (theo config) | Ghi đè địa chỉ:cổng lắng nghe, ví dụ `:2028` hoặc `0.0.0.0:2028`. |
| `--version` | — | In phiên bản rồi thoát. |
| `--hash-password` | — | In ra hash bcrypt của mật khẩu đăng nhập web truyền vào, dùng để dán vào `config.yaml`, rồi thoát. |
| `--import-shinobi` | — | Đường dẫn file JSON monitor Shinobi; nhập thẳng vào file camera rồi thoát (không chạy web). |
| `--import-hik-port` | `80` | Cổng cấu hình gán cho camera Hikvision khi dùng `--import-shinobi`. |
| `--import-dahua-port` | `37777` | Cổng cấu hình gán cho camera Dahua/KBVision khi dùng `--import-shinobi`. |

Ví dụ tạo hash mật khẩu mới:

```bash
./kspcam --hash-password "matkhaumoi"
```

Ví dụ nhập camera từ Shinobi mà không cần mở giao diện web (xem thêm bài
[Nhập từ Shinobi](#help/nhap-shinobi)):

```bash
./kspcam --import-shinobi monitors.json --import-hik-port 80 --import-dahua-port 37777
```

Các lệnh `make` để build (chạy trong thư mục mã nguồn):

| Lệnh | Kết quả |
|---|---|
| `make build` | Build ra file `kspcam` cho kiến trúc máy hiện tại. |
| `make build-all` | Build tĩnh cho cả 3 kiến trúc vào thư mục `dist/`: `kspcam-linux-amd64`, `kspcam-linux-armv7` (ARM 32-bit), `kspcam-linux-arm64`. |
| `make docs` | Sinh lại gói bài trợ giúp (`web/static/help/help-index.json`) từ các file trong `docs/help/`. |
| `make docs-check` | Kiểm tra bài trợ giúp có phủ đủ route/tab hiện có không, dùng trước khi phát hành. |

## Lưu ý

- Cổng lắng nghe mặc định là **`:2028`**; đổi bằng `--addr` hoặc trong
  `config.yaml`.
- File build từ `make build`/`make build-all` là binary tĩnh, không phụ thuộc
  runtime — copy sang thiết bị đích và chạy trực tiếp, không cần cài thêm gì.
- `--import-shinobi` chỉ ghi vào file camera rồi thoát, **không khởi động**
  giao diện web trong lần chạy đó.

## Sự cố thường gặp

- Chạy `./kspcam` báo lỗi đọc `config.yaml` → kiểm tra đường dẫn truyền vào
  `--config`, hoặc tạo file cấu hình từ `config.example.yaml`.
- Đổi mật khẩu đăng nhập web nhưng đăng nhập không được → phải dùng đúng hash
  in ra từ `--hash-password`, dán vào đúng trường mật khẩu trong
  `config.yaml`, sai định dạng sẽ không đăng nhập được.
- `make docs-check` báo thiếu bài trợ giúp → route/tab mới chưa có file
  `docs/help/*.md` tương ứng, cần viết thêm trước khi phát hành.
