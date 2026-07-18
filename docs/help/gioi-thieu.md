---
id: gioi-thieu
title: "Giới thiệu & quy trình làm việc"
section: start
order: 10
keywords: [giới thiệu, bắt đầu, quy trình, hướng dẫn, tổng quan, kspcam, camera, hàng loạt, dahua, kbvision, hikvision]
ui: "#dashboard"
covers: []
related: [quet-lan, kho-camera, chinh-hang-loat]
---
## Mục đích

kspcam là công cụ **chỉnh cấu hình hàng loạt** cho camera IP **Hikvision** và
**Dahua/KBVision**: đổi độ phân giải, codec, Smart Codec, bitrate, âm thanh,
mật khẩu… cho nhiều camera cùng lúc, ngay trên trình duyệt.

Mọi thay đổi được áp dụng **tuần tự từng cam một** cho an toàn, có **log trực
tiếp** từng bước và **đọc lại để xác nhận** thay đổi thực sự có hiệu lực.

## Cách dùng

Quy trình chuẩn gồm 4 bước, theo đúng thứ tự các tab bên trái:

1. [Quét mạng](#scan) (tùy chọn) — tìm camera trong LAN bằng ONVIF / Dahua /
   Hikvision SADP, hoặc quét subnet bằng nmap, rồi bấm **Thêm vào kho**.
   Xem chi tiết ở bài [Quét LAN tìm camera](#help/quet-lan).
2. [Kho camera](#cameras) — thêm camera thủ công (IP, cổng, hãng, tài khoản)
   nếu không quét được. Bấm **Dò** để đọc cấu hình luồng hiện tại của từng cam.
3. **Chỉnh hàng loạt** (trong tab Kho camera) — tick chọn camera, chọn luồng
   và thông số muốn đổi, bấm **Áp dụng**. Xem bài
   [Chỉnh thông số hàng loạt](#help/chinh-hang-loat).
4. Theo dõi **log trực tiếp** và bảng **Kết quả** để biết cam nào thành công,
   cam nào lỗi.

## Lưu ý

- Cổng cấu hình mặc định: Dahua/KBVision **37777**, Hikvision **80** (ISAPI).
  Đây là cổng giao thức riêng của hãng, không phải cổng RTSP xem hình.
- Quét UDP chỉ thấy camera **cùng mạng LAN**. Camera qua NAT thì thêm thủ công
  bằng IP:cổng đã NAT.

## Sự cố thường gặp

- Không đăng nhập được giao diện → xem bài [Đăng nhập & phiên làm việc](#help/dang-nhap).
- Thêm cam rồi nhưng **Dò** báo lỗi → kiểm tra lại cổng (37777/80), tài khoản
  và mật khẩu của chính camera đó.
