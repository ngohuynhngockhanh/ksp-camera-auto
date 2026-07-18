---
id: thu-mat-khau
title: "Thử mật khẩu hàng loạt"
section: scan
order: 20
keywords: [thử mật khẩu, mật khẩu, pass, dò mật khẩu, mật khẩu hàng loạt, đăng nhập, khóa tài khoản, lockout, quét mạng, tài khoản, kiểm tra đăng nhập, admin]
ui: "#scan"
covers: ["/api/scan/try-password"]
related: [quet-lan, kho-camera, doi-mat-khau-thiet-bi]
---
## Mục đích

Sau khi [quét LAN](#help/quet-lan) ra danh sách thiết bị chưa có trong kho và
chưa rõ mật khẩu, dùng thẻ **Thử mật khẩu hàng loạt** để thử nhanh một tài
khoản/mật khẩu ứng viên trên nhiều thiết bị đã tick cùng lúc, thay vì đăng
nhập thử tay từng cái. Đây chỉ là **kiểm tra đăng nhập** — không đổi bất cứ gì
trên camera và không tự thêm cam vào kho.

## Cách dùng

1. Ở tab [Quét mạng](#scan), quét LAN trước để có danh sách thiết bị (xem bài
   [Quét LAN tìm camera](#help/quet-lan)).
2. Tick chọn các dòng muốn thử trong bảng kết quả (checkbox đầu bảng hoặc
   **Chọn tất cả**).
3. Nhập **Tài khoản** (mặc định `admin`) và **Mật khẩu thử**.
4. Bấm **Thử mật khẩu hàng loạt**.
5. Theo dõi tiến trình: từng thiết bị được thử **tuần tự** (không song
   song), cột **Trạng thái** của mỗi dòng cập nhật ngay — nhãn xanh **OK**
   nếu đăng nhập được, nhãn đỏ **Lỗi** (trỏ chuột vào để xem chi tiết) nếu
   sai hoặc không kết nối được.
6. Chạy xong, dòng thông báo hiện tổng số đăng nhập thành công trên tổng số
   đã thử.
7. Với thiết bị **OK**, bấm **Thêm vào kho** ở bảng phía trên rồi tự nhập lại
   đúng **Tài khoản**/**Mật khẩu** vừa thử thành công vào form — công cụ
   không tự điền lại phần này khi thêm vào kho.

## Lưu ý

- Chạy tuần tự từng thiết bị một, không chạy song song — chọn càng nhiều
  thiết bị thì càng lâu.
- Chỉ kiểm tra đăng nhập, không sửa cấu hình hay tự thêm cam vào kho.
- Thiết bị quét được nhưng chưa xác định được **Hãng** (Dahua/Hikvision) sẽ
  bị bỏ qua khi thử.

> **Lưu ý:** thử sai mật khẩu nhiều lần liên tiếp có thể khiến camera (đặc
> biệt Dahua/KBVision) **tự khóa tài khoản** một thời gian theo cơ chế chống
> dò mật khẩu của hãng. Chỉ thử với số mật khẩu hợp lý, tránh dò nhiều mật
> khẩu khác nhau liên tục trên cùng một cam.

## Sự cố thường gặp

- Toàn bộ dòng báo **Lỗi** → kiểm tra lại Tài khoản/Mật khẩu, hoặc camera đã
  bị khóa tạm do thử sai nhiều lần trước đó (đợi vài phút rồi thử lại).
- Nút **Thử mật khẩu hàng loạt** bị mờ không bấm được → chưa tick chọn thiết
  bị nào trong bảng kết quả.
- Dòng báo "không xác định được hãng, bỏ qua" → thiết bị này quét được IP
  nhưng chưa rõ hãng; thêm thủ công ở [Kho camera](#help/kho-camera), chọn
  đúng hãng, rồi thử lại từ đó.
