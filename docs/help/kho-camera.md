---
id: kho-camera
title: "Kho camera: thêm, sửa, xóa"
section: cameras
order: 10
keywords: [kho camera, thêm cam, thêm camera, sửa camera, xóa camera, đổi tên, host, IP, cổng, port, dahua, kbvision, hikvision, tài khoản, mật khẩu, mã hóa, danh sách camera]
ui: "#cameras"
covers: ["/api/cameras", "/api/cameras/delete"]
related: [gioi-thieu, doc-cau-hinh, anh-chup, chinh-hang-loat]
---
## Mục đích

Tab **Kho camera** là danh sách toàn bộ camera bạn quản lý trong kspcam: tên,
địa chỉ, hãng, tài khoản đăng nhập. Đây là nơi bắt đầu cho mọi thao tác khác —
dò cấu hình, xem hình, chỉnh hàng loạt, đổi mật khẩu đều thao tác trên các
camera có trong kho này.

## Cách dùng

1. Mở tab [Kho camera](#cameras). Thẻ **Thêm / sửa camera** ở trên cùng.
2. Nhập thông tin:
   - **Tên** — nhãn tùy bạn đặt, chỉ hiển thị trong kspcam (không ghi xuống
     camera).
   - **Host / IP** — bắt buộc.
   - **Cổng** — cổng giao thức cấu hình của hãng (không phải cổng RTSP xem
     hình). Bỏ trống thì dùng cổng mặc định theo hãng.
   - **Hãng** — chọn **Dahua / KBVision** hoặc **Hikvision**.
   - **Tài khoản**, **Mật khẩu** — thông tin đăng nhập của chính camera đó.
3. Bấm **Thêm / Lưu camera** để lưu vào kho.
4. Danh sách bên dưới hiện mỗi camera một dòng, có các nút:
   - Icon bút chì cạnh **Tên** — sửa nhanh tên trong kho (không đổi gì trên
     thiết bị), gõ tên mới rồi Enter để lưu, Esc để hủy.
   - **Dò** — đọc cấu hình luồng hiện tại. Xem bài
     [Đọc cấu hình (Dò)](#help/doc-cau-hinh).
   - **Xem hình** — chụp ảnh nhanh từ camera. Xem bài
     [Ảnh chụp nhanh](#help/anh-chup).
   - **Tất cả kênh** — với đầu ghi, mở lưới ảnh mọi kênh cùng lúc.
   - **Sửa** — nạp lại thông tin của camera vào form phía trên để chỉnh sửa.
   - **Xóa** — xóa camera khỏi kho, có hộp thoại xác nhận.
5. Để sửa một camera đã có: bấm **Sửa**, form phía trên tự điền sẵn dữ liệu,
   chỉnh xong bấm lại **Thêm / Lưu camera**.
6. Tick chọn ô ở đầu mỗi dòng (hoặc ô **Chọn tất cả** trên tiêu đề bảng) để
   dùng camera đó cho **Chỉnh hàng loạt** hoặc **Đổi mật khẩu**.

## Lưu ý

- **Đổi Host hoặc Cổng khi sửa sẽ tạo một mục mới** trong kho (vì kspcam dùng
  `host:cổng` làm mã định danh), chứ không ghi đè mục cũ — nhớ xóa mục cũ nếu
  không cần nữa.
- Để trống ô **Mật khẩu** khi sửa sẽ **giữ nguyên mật khẩu cũ đã lưu** — tiện
  khi chỉ muốn đổi tên hoặc tài khoản mà không gõ lại mật khẩu.
- Dữ liệu kho được lưu trong file `cameras.yaml` trên máy chủ; mật khẩu lưu
  **mã hóa AES-GCM** trên file, chỉ được giải mã khi hiển thị cho phiên đã
  đăng nhập.
- Xóa camera khỏi kho **không đổi gì trên chính camera** — chỉ gỡ khỏi danh
  sách quản lý của kspcam.

## Sự cố thường gặp

- Thêm camera xong bấm **Dò** báo lỗi → kiểm tra lại **Cổng** (Dahua/KBVision
  mặc định 37777, Hikvision mặc định 80) và **Tài khoản/Mật khẩu** đúng của
  chính camera đó.
- Không nhớ mật khẩu camera → dùng [Thử mật khẩu hàng loạt](#help/thu-mat-khau)
  ở tab Quét mạng trước khi thêm vào kho.
- Sửa xong không thấy thay đổi → kiểm tra có bấm nhầm **Sửa** ở dòng khác rồi
  lưu đè hay không; danh sách luôn tải lại (`loadCameras`) sau mỗi lần lưu nên
  F5 lại trang nếu nghi ngờ giao diện chưa cập nhật.
