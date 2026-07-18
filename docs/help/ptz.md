---
id: ptz
title: "Điều khiển PTZ (Dahua/KBVision)"
section: dahua
order: 40
keywords: [PTZ, xoay, quay, quét, xoay camera, lên xuống, trái phải, zoom, phóng to, thu nhỏ, nét, lấy nét, focus, tốc độ xoay, điều khiển hướng]
ui: "#cameras"
covers: ["/api/ptz"]
related: [hinh-anh-mau-sac, anh-chup, ten-kenh-osd]
---
## Mục đích

Điều khiển **PTZ** cho camera Dahua/KBVision có cơ cấu quay: **xoay 8 hướng**,
**zoom** (phóng to/thu nhỏ) và chỉnh **nét** (focus). Có **ảnh xem trực tiếp**
để bạn kiểm tra camera đã quay tới đúng góc chưa.

> **Lưu ý:** đây là tính năng **chỉ dành cho Dahua/KBVision**. Camera Hikvision
> sẽ báo lỗi `camera này không hỗ trợ tính năng này (chỉ Dahua/KBVision)` và
> không hiện tab **PTZ**. Camera cố định (không có cơ cấu quay) sẽ không nhúc
> nhích dù bấm nút.

## Cách dùng

1. Mở tab [Kho camera](#cameras), bấm **Xem hình** (hoặc **Tất cả kênh**) ở
   camera cần điều khiển.
2. Ở kênh đó bấm **Sửa tên & OSD** để mở hộp thoại, rồi chuyển sang tab **PTZ**
   (chỉ hiện với camera Dahua/KBVision).
3. Dùng **bàn phím 8 hướng**: **giữ** nút hướng (↑ ↓ ← → và 4 góc chéo) để
   camera quay, **thả tay** ra là dừng.
4. Kéo thanh **Tốc độ (1–8)** để chỉnh nhanh/chậm khi quay.
5. Zoom: bấm **Zoom + (gần)** để phóng to, **Zoom − (xa)** để thu nhỏ.
6. Lấy nét: bấm **Nét xa** hoặc **Nét gần** nếu ảnh bị mờ.
7. Bấm **Tải lại ảnh** ở khung xem bên trái để kiểm tra góc/độ nét mới.

## Lưu ý

- Lệnh PTZ chạy qua **kết nối cấu hình DVRIP** đang dùng, và **tự chuyển sang
  HTTP CGI** khi cần — nên thường vẫn quay được kể cả khi không mở riêng cổng
  80. Nếu quay không được, thử mở cổng HTTP (80) của camera.
- Nút giữ-để-quay dùng cơ chế **giữ thì chạy, thả thì dừng**. Nếu lỡ nhả chuột
  ra ngoài vùng nút mà camera vẫn quay, bấm lại rồi thả đúng trên nút để gửi
  lệnh dừng; đóng hộp thoại cũng tự gửi lệnh dừng.
- Đặt **tốc độ thấp** (1–3) khi cần canh góc chính xác, tốc độ cao khi quét
  nhanh sang vùng khác.

## Sự cố thường gặp

- **Bấm nút mà camera không quay** → camera là loại cố định (không có PTZ),
  hoặc báo lỗi PTZ: kiểm tra camera có mở cổng 80 và có cơ cấu quay không.
- **Không thấy tab PTZ** → camera là Hikvision, hoặc khai báo sai hãng trong
  [Kho camera](#cameras).
- **Quay được nhưng không dừng** → thả tay đúng trên nút để gửi lệnh dừng, hoặc
  đóng hộp thoại (thao tác đóng sẽ tự dừng).
- **Zoom/nét không ăn** → một số camera speed dome cần vài giây để phản hồi;
  bấm lại và bấm **Tải lại ảnh** để xem kết quả.
