---
id: mang-ip-tinh
title: "Đổi IP tĩnh / DHCP (Dahua/KBVision)"
section: dahua
order: 20
keywords: [mạng, IP tĩnh, đổi IP, IP, DHCP, gateway, cổng vào, DNS, subnet, subnet mask, địa chỉ IP, interface, đổi địa chỉ, cấu hình mạng]
ui: "#cameras"
covers: ["/api/network"]
related: [kho-camera, wifi, loi-thuong-gap]
---
## Mục đích

Đọc và đổi **cấu hình mạng của chính camera**: bật/tắt **DHCP**, đặt **IP
tĩnh**, **subnet mask**, **gateway** và **DNS**. Dùng khi cần gán IP cố định
cho camera để dễ quản lý và xem hình ổn định.

> **Lưu ý:** đây là tính năng **chỉ dành cho Dahua/KBVision**. Camera Hikvision
> sẽ báo lỗi `camera này không hỗ trợ tính năng này (chỉ Dahua/KBVision)` và
> thẻ **Mạng** sẽ không mở.

## Cách dùng

1. Mở tab [Kho camera](#cameras), bấm nút **sửa** (hình bút chì) ở camera
   Dahua/KBVision cần đổi. Thẻ **Mạng** sẽ tự hiện bên dưới form.
2. Chờ công cụ đọc xong cấu hình hiện tại (interface, IP, gateway, DNS).
3. Nếu thiết bị có nhiều **Interface**, chọn đúng cổng mạng đang dùng.
4. Chọn cách cấp IP:
   - Tick **Dùng DHCP (tự động lấy IP)** để camera tự xin IP từ router.
   - Bỏ tick DHCP để nhập tay: **Địa chỉ IP**, **Subnet mask**, **Gateway**,
     **DNS 1**, **DNS 2**.
5. Tick ô xác nhận **"Tôi hiểu đổi IP/gateway sai có thể khiến camera mất kết
   nối, phải vào tận nơi để sửa lại."** — chỉ khi tick ô này thì nút lưu mới
   bật lên.
6. Bấm **Lưu cấu hình mạng**.

## Lưu ý

> **Lưu ý:** đổi IP/gateway/subnet **SAI có thể khiến camera biến mất khỏi
> mạng** — không xem hình, không cấu hình được từ xa nữa. Trước khi lưu, hãy
> chắc chắn:
> - IP mới nằm **cùng lớp mạng** với router (ví dụ router `192.168.1.1` thì IP
>   camera nên là `192.168.1.x`).
> - **Gateway** đúng bằng IP của router.
> - IP mới **không trùng** với thiết bị khác đang chạy.

- Sau khi đổi IP thành công, camera **đổi địa chỉ ngay**. Bạn phải vào
  [Kho camera](#cameras), **sửa lại host/cổng** của mục này thành IP mới thì
  mới kết nối lại được (đổi host sẽ tạo mục mới trong kho).
- Thẻ Mạng chỉ đọc/ghi được qua **cổng cấu hình DVRIP** đang dùng để kết nối
  camera. Với camera qua NAT chỉ mở cổng DVRIP thì vẫn đổi được IP, nhưng đổi
  xong địa chỉ NAT có thể phải khai báo lại.

## Sự cố thường gặp

- **Đổi IP xong mất kết nối camera** → dò lại IP mới của camera bằng
  [Quét LAN tìm camera](#help/quet-lan) (khi bạn đang cùng mạng LAN), hoặc dùng
  công cụ ConfigTool của hãng để tìm/đặt lại IP. Nếu vẫn không thấy, **reset
  cứng** camera về mặc định rồi cấu hình lại.
- **Không mở được thẻ Mạng** → camera là Hikvision, hoặc khai báo sai hãng
  trong kho.
- **Lưu báo lỗi** → kiểm tra cổng cấu hình (Dahua 37777) và tài khoản; camera
  đời cũ có thể không cho ghi mạng qua tài khoản thường (cần tài khoản admin).
- **DHCP bật rồi mà không biết IP mới** → dùng [Quét LAN](#help/quet-lan) hoặc
  xem danh sách thiết bị (DHCP client) trên router.
