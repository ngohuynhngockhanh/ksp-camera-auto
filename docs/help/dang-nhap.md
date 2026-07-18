---
id: dang-nhap
title: "Đăng nhập & phiên làm việc"
section: start
order: 40
keywords: [đăng nhập, đăng xuất, mật khẩu web, tài khoản admin, phiên làm việc, session, hết phiên, khóa tài khoản, sai mật khẩu, quá số lần, config.yaml, bcrypt, đổi mật khẩu đăng nhập]
ui: ""
covers: ["/login", "/logout"]
related: [gioi-thieu, giao-dien, cau-hinh-yaml]
---
## Mục đích

Trang **Đăng nhập** bảo vệ toàn bộ kspcam bằng một tài khoản quản trị duy
nhất (khác với tài khoản đăng nhập của từng camera). Bài này giải thích tài
khoản này lấy từ đâu, phiên làm việc kéo dài bao lâu, và điều gì xảy ra khi
đăng nhập sai nhiều lần.

## Cách dùng

1. Truy cập kspcam, nếu chưa đăng nhập bạn sẽ được đưa tới `/login`.
2. Nhập **Tài khoản** và **Mật khẩu** của web kspcam, bấm **Đăng nhập**.
3. Đăng nhập thành công sẽ vào thẳng tab **Tổng quan**.
4. Để thoát, bấm **Đăng xuất** ở thanh trên (desktop) hoặc trong menu
   (điện thoại) — xem bài [Điều hướng giao diện](#help/giao-dien).

**Tài khoản/mật khẩu lấy từ đâu:** đây là tài khoản trong `config.yaml`
(mục `server.username` / `server.password`), không liên quan tài khoản của
từng camera. Muốn đổi, sửa `config.yaml` rồi khởi động lại kspcam.

> **Lưu ý:** thay vì lưu mật khẩu dạng chữ thường, bạn có thể chạy
> `kspcam --hash-password <mật khẩu>` để lấy chuỗi băm **bcrypt**, dán vào
> `server.password_hash` trong `config.yaml` (bỏ trống `server.password`).

## Lưu ý

- Phiên đăng nhập (session) có hiệu lực khoảng **12 giờ**; sau đó phải đăng
  nhập lại.
- Đăng nhập sai liên tiếp quá số lần cho phép (mặc định **5 lần**) sẽ tạm
  **khóa đăng nhập từ IP đó** trong một khoảng thời gian (mặc định **30
  phút**), sau đó tự mở khóa. Hai giá trị này chỉnh được ở
  `server.login_max_attempts` và `server.login_lockout_minutes` trong
  `config.yaml`.
- Trang đăng nhập không phân biệt rõ "sai mật khẩu" hay "đang bị khóa" —
  cả hai đều hiện chung thông báo **"Sai tài khoản hoặc mật khẩu."**

## Sự cố thường gặp

- Đăng nhập đúng tài khoản/mật khẩu vẫn báo lỗi, thử vài lần vẫn không được →
  có thể IP đang bị khóa tạm do trước đó đăng nhập sai nhiều lần; đợi hết thời
  gian khóa rồi thử lại, hoặc kiểm tra lại đúng tài khoản trong `config.yaml`.
- Quên mật khẩu web → sửa trực tiếp `server.password` (hoặc
  `server.password_hash`) trong `config.yaml` trên máy chủ rồi khởi động lại
  kspcam.
- Đang thao tác giữa chừng thì bị đá về trang đăng nhập → phiên làm việc đã
  hết hạn (khoảng 12 giờ) hoặc kspcam vừa khởi động lại; đăng nhập lại và làm
  tiếp.
