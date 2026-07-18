---
id: wifi
title: "Cấu hình Wi-Fi (Dahua/KBVision)"
section: dahua
order: 30
keywords: [wifi, wi-fi, wf, sóng, mạng không dây, SSID, tên wifi, mật khẩu wifi, pass wifi, quét wifi, dò wifi, kết nối wifi, camera không dây]
ui: "#cameras"
covers: ["/api/wifi", "/api/wifi-scan"]
related: [mang-ip-tinh, kho-camera, loi-thuong-gap]
---
## Mục đích

Đọc và đổi cấu hình **Wi-Fi** cho camera Dahua/KBVision có ăng-ten không dây:
xem **SSID** đang kết nối, **quét** các mạng Wi-Fi xung quanh, rồi đặt **SSID +
mật khẩu** mới cho camera.

> **Lưu ý:** đây là tính năng **chỉ dành cho Dahua/KBVision**. Camera Hikvision
> sẽ báo lỗi `camera này không hỗ trợ tính năng này (chỉ Dahua/KBVision)`.

## Cách dùng

1. Mở tab [Kho camera](#cameras), bấm nút **sửa** (hình bút chì) ở camera
   Dahua/KBVision. Thẻ **Mạng** hiện ra, mục **Wi-Fi** nằm bên dưới phần IP.
2. Xem **SSID** camera đang dùng ở ô đầu.
3. Bấm **Quét Wi-Fi** để dò các mạng xung quanh. Mỗi mạng hiện dạng nút bấm
   kèm **% chất lượng sóng**; bấm vào một mạng để tự điền SSID.
4. Nhập **Mật khẩu Wi-Fi** (để trống = giữ nguyên mật khẩu cũ).
5. Tick ô xác nhận **"Tôi hiểu đổi Wi-Fi sai có thể khiến camera mất kết nối."**
   — chỉ khi tick thì nút lưu mới bật.
6. Bấm **Lưu Wi-Fi** và xác nhận ở hộp thoại hiện lên.

## Lưu ý

> **Lưu ý:** đổi Wi-Fi sai (sai mật khẩu, chọn nhầm mạng) sẽ khiến camera
> **rớt mạng và biến mất**. Nếu camera đang chỉ kết nối bằng Wi-Fi, hãy chắc
> chắn SSID và mật khẩu đúng trước khi lưu. Khi rớt, thường phải cắm dây LAN
> hoặc reset camera để cấu hình lại.

- **Quét Wi-Fi cần cổng HTTP (80) của camera** đang mở và tới được — đây là
  thao tác chạy qua cổng khác với cổng cấu hình DVRIP. Với camera chỉ NAT cổng
  DVRIP, nút **Quét Wi-Fi** có thể báo lỗi dù việc đọc/ghi SSID vẫn chạy.
- Sau khi đổi Wi-Fi, camera **chuyển sang mạng mới** và **địa chỉ IP có thể
  đổi**. Bạn cần dò lại IP mới rồi cập nhật host trong [Kho camera](#cameras)
  (xem thêm [Đổi IP tĩnh / DHCP](#help/mang-ip-tinh)).
- Nếu camera không có ăng-ten Wi-Fi, mục Wi-Fi sẽ báo "Thiết bị không có/không
  đọc được cấu hình Wi-Fi".

## Sự cố thường gặp

- **Quét Wi-Fi báo lỗi** → cổng HTTP (80) của camera không tới được (thường do
  chỉ NAT cổng DVRIP). Hãy gõ tay **SSID** thay vì quét.
- **Không thấy mục Wi-Fi** → camera không có sóng Wi-Fi, hoặc là Hikvision,
  hoặc khai báo sai hãng trong kho.
- **Lưu Wi-Fi xong mất camera** → cắm dây LAN vào camera để lấy lại kết nối,
  hoặc [Quét LAN](#help/quet-lan) tìm IP mới; nếu vẫn mất thì reset camera.
- **Quét thấy mạng nhưng nối không được** → kiểm tra lại mật khẩu, và bảo đảm
  router phát băng tần camera hỗ trợ (nhiều camera chỉ nhận **2.4GHz**).
