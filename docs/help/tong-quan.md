---
id: tong-quan
title: "Tổng quan (Dashboard)"
section: start
order: 20
keywords: [tổng quan, dashboard, trang chủ, thống kê, số lượng camera, kết quả áp dụng gần nhất, hoạt động gần đây, quét mạng, quản lý camera, dahua, hikvision, kbvision]
ui: "#dashboard"
covers: []
related: [gioi-thieu, giao-dien, chinh-hang-loat]
---
## Mục đích

Trang **Tổng quan** là màn hình đầu tiên sau khi đăng nhập. Nó cho bạn cái
nhìn nhanh về kho camera hiện có và kết quả lần áp dụng cấu hình/đổi mật khẩu
gần nhất, kèm hai lối tắt vào các thao tác hay dùng.

## Cách dùng

1. Xem 4 thẻ thống kê ở đầu trang:
   - **Tổng camera** — tổng số camera đang có trong kho.
   - **Dahua / KBVision** — số camera thuộc hai hãng này.
   - **Hikvision** — số camera Hikvision.
   - **Kết quả áp dụng gần nhất** — số camera **thành công / lỗi** của lần
     **Áp dụng** hoặc **Đổi mật khẩu** gần nhất trong phiên làm việc hiện tại;
     hiện dấu `–` nếu chưa thao tác gì.
2. Bấm **Quét mạng** hoặc **Quản lý camera** trong khối lối tắt để chuyển
   thẳng sang tab tương ứng.
3. Xem khối **Hoạt động gần đây** để biết chi tiết lần chạy gần nhất: loại
   thao tác (áp dụng cấu hình / đổi mật khẩu), giờ chạy, tổng số camera, số
   thành công và số lỗi.

## Lưu ý

- Các số liệu ở **Kết quả áp dụng gần nhất** và **Hoạt động gần đây** chỉ tồn
  tại trong **phiên làm việc hiện tại** (bộ nhớ trình duyệt) — **tải lại
  trang hoặc đăng nhập lại sẽ mất**, không phải lỗi.
- Thẻ **Tổng camera** đếm theo kho camera đã lưu, không phụ thuộc kết quả
  quét mạng gần nhất.

## Sự cố thường gặp

- Các thẻ thống kê hiện `–` dù đã thêm camera → chưa tải xong danh sách, mở
  tab [Kho camera](#cameras) rồi quay lại Tổng quan.
- Không thấy **Hoạt động gần đây** dù vừa áp dụng xong → kiểm tra lại, mục
  này chỉ cập nhật sau khi thao tác **Áp dụng** ở
  [Chỉnh hàng loạt](#help/chinh-hang-loat) hoặc đổi mật khẩu chạy xong hẳn
  (không tính khi đang chạy dở).
