---
id: cau-hinh-yaml
title: "Tệp cấu hình config.yaml"
section: admin
order: 10
keywords: [config, cấu hình, config.yaml, yaml, cổng, port, đăng nhập, mật khẩu, password_hash, bcrypt, khóa, mã hóa, timeout, defaults, admin]
ui: ""
covers: []
related: [dong-lenh, dang-nhap, loi-thuong-gap]
---
## Mục đích

`config.yaml` là tệp cấu hình chạy máy chủ kspcam: địa chỉ lắng nghe, tài khoản
đăng nhập giao diện, cổng mặc định theo hãng, tài khoản camera mặc định và thời
gian chờ. Bài này dành cho **người cài đặt máy chủ** — mọi trường đều có giá trị
mặc định an toàn, thiếu tệp thì công cụ vẫn chạy được.

Chép từ `config.example.yaml` thành `config.yaml` rồi sửa cho phù hợp.

## Cách dùng

Ví dụ một tệp `config.yaml` đầy đủ (thay chỗ `...` bằng thông tin của bạn,
**đừng dùng mật khẩu mẫu**):

```yaml
server:
  # Địa chỉ web UI lắng nghe.
  addr: ":2028"
  # Tài khoản đăng nhập giao diện.
  username: "admin"
  password: "<mat-khau-cua-ban>"
  # Tùy chọn: dùng bcrypt hash thay cho password ở trên
  # (sinh bằng: kspcam --hash-password <mat-khau>). Để trống thì dùng password.
  password_hash: ""
  # Chặn dò mật khẩu: khóa IP sau số lần đăng nhập sai liên tiếp,
  # trong số phút nhất định.
  login_max_attempts: 5
  login_lockout_minutes: 30

# Nơi lưu kho camera (chỉnh sửa được từ giao diện web).
cameras_file: "cameras.yaml"

# Cổng cấu hình mặc định theo hãng, dùng khi camera không ghi rõ cổng.
defaults:
  hikvision_port: 8000      # cổng riêng/SDK của Hikvision
  dahua_port: 37777         # cổng DVRIP của Dahua/KBVision (đôi khi 8888)
  # Tài khoản camera mặc định khi entry để trống.
  username: "admin"
  password: "<mat-khau-cam-mac-dinh>"
  # Thời gian chờ mỗi thao tác (giây); tăng lên nếu đầu ghi nhiều kênh chậm.
  # Giao diện web có thể ghi đè theo từng lần chạy.
  timeout_seconds: 30
  # Mật khẩu mới mặc định khi đổi mật khẩu camera hàng loạt.
  new_password: "<mat-khau-moi-mac-dinh>"
```

Các nhóm trường chính:

| Trường | Ý nghĩa |
|---|---|
| `server.addr` | Địa chỉ/cổng web UI lắng nghe (mặc định `:2028`). |
| `server.username` / `server.password` | Tài khoản đăng nhập giao diện. |
| `server.password_hash` | Bcrypt hash; nếu đặt sẽ được kiểm tra **thay cho** `password`. |
| `server.login_max_attempts` | Số lần sai liên tiếp thì khóa IP (mặc định 5). |
| `server.login_lockout_minutes` | Số phút khóa IP (mặc định 30). |
| `cameras_file` | Đường dẫn kho camera. |
| `defaults.hikvision_port` / `dahua_port` | Cổng cấu hình mặc định theo hãng. |
| `defaults.username` / `password` | Tài khoản camera mặc định. |
| `defaults.timeout_seconds` | Thời gian chờ mỗi thiết bị (giây). |
| `defaults.new_password` | Mật khẩu mới mặc định khi đổi mật khẩu hàng loạt. |

## Lưu ý

> **Lưu ý:** mật khẩu camera trong `cameras.yaml` được **mã hóa AES-256-GCM**
> bằng khóa lưu ở `~/.kspcam.key` (tự sinh lần đầu, quyền 0600). Nếu **mất khóa
> này, toàn bộ mật khẩu đã lưu sẽ không đọc lại được**. Bản deploy ghim khóa tại
> `/opt/ksp-cam/.kspcam.key` — hãy sao lưu khóa cùng với `cameras.yaml`.

- Có thể ghi đè khóa mã hóa bằng biến môi trường: `KSPCAM_KEY` (chuỗi base64
  32 byte, hoặc chuỗi bất kỳ sẽ được băm thành 32 byte) hoặc `KSPCAM_KEY_FILE`
  (đường dẫn tới tệp khóa). `KSPCAM_KEY` được ưu tiên trước.
- Nên dùng `password_hash` thay cho `password` khi dịch vụ mở ra Internet, để
  mật khẩu giao diện không nằm dạng thô trong tệp. Sinh hash bằng
  `kspcam --hash-password <mật-khẩu>` rồi dán vào `password_hash`.
- **Không commit** `config.yaml` hay `cameras.yaml` chứa mật khẩu thật lên git.
- Cổng cấu hình (37777 / 80 / 8000) **khác** cổng RTSP xem hình (554) — đừng
  nhầm hai loại cổng này.

## Sự cố thường gặp

- Sửa `config.yaml` xong nhưng không có tác dụng → phải **khởi động lại** tiến
  trình kspcam để nạp lại cấu hình.
- Mở lại kho thấy mật khẩu camera **không giải mã được** → khóa `~/.kspcam.key`
  đã bị thay/mất; khôi phục đúng khóa cũ hoặc nhập lại mật khẩu camera.
- Bị **khóa đăng nhập** giao diện sau nhiều lần sai → chờ hết
  `login_lockout_minutes`, hoặc xem bài [Đăng nhập](#help/dang-nhap).
