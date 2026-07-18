---
id: quet-lan
title: "Quét LAN tìm camera"
section: scan
order: 10
keywords: [quét, quét mạng, scan, tìm cam, dò mạng, dò camera, onvif, sadp, dahua discover, nmap, subnet, cidr, broadcast, lan, thêm vào kho]
ui: "#scan"
covers: ["/api/scan"]
related: [gioi-thieu, thu-mat-khau, kho-camera]
---
## Mục đích

Tự động tìm camera đang có trong mạng LAN thay vì phải gõ tay từng IP. Tab
**Quét mạng** dùng 3 giao thức UDP chạy song song — **ONVIF WS-Discovery**,
**Dahua DHDiscover**, **Hikvision SADP** — và có thêm lựa chọn quét bằng
**nmap** cho subnet đã route được nhưng không cùng LAN.

## Cách dùng

1. Mở tab [Quét mạng](#scan).
2. Bấm **Quét LAN (ONVIF/Dahua/Hik)** — quét đồng thời cả 3 giao thức, mất
   vài giây.
3. Xem bảng kết quả với các cột: **IP**, **Cổng**, **Hãng**, **Model**,
   **MAC**, **Nguồn** (phương thức tìm ra dòng này: onvif / dahua /
   hikvision-sadp / nmap), **Trạng thái**.
4. Nếu camera ở subnet khác (route được nhưng không cùng LAN với máy chạy
   kspcam): nhập **Subnet (CIDR)**, ví dụ `192.168.1.0/24`, rồi bấm **Quét
   nmap**. Cách này chỉ dò cổng mở (80, 8000, 37777, 37778, 8888) chứ không
   xác thực bằng đúng giao thức của hãng, nên cột Hãng/Model có thể để trống.
5. Ở dòng muốn thêm, bấm **Thêm vào kho** — công cụ điền sẵn IP, Cổng, Hãng,
   Tên vào form **Thêm/sửa camera** ở tab [Kho camera](#cameras); bạn tự nhập
   **Tài khoản**/**Mật khẩu** rồi bấm **Thêm/Lưu camera**.
6. Có thể tick chọn nhiều dòng (checkbox đầu bảng, hoặc **Chọn tất cả**) để
   dùng cho [Thử mật khẩu hàng loạt](#help/thu-mat-khau).

## Lưu ý

- Quét LAN dùng UDP broadcast/multicast nên chỉ thấy thiết bị **cùng mạng
  LAN/VLAN** với máy chạy kspcam — không xuyên NAT, không qua router. Camera
  ở xa (đã NAT) phải thêm thủ công ở Kho camera, hoặc dùng nmap nếu subnet đó
  reachable qua route.
- Quét nmap cần lệnh `nmap` có sẵn trong PATH của **máy chủ chạy kspcam**
  (không phải máy đang mở trình duyệt).
- Subnet cho nmap chỉ chấp nhận CIDR, một IP, hoặc khoảng đơn giản (ví dụ
  `192.168.1.0/24`, `10.0.0.5`, `192.168.1.1-254`) — không nhận cú pháp khác.

## Sự cố thường gặp

- Quét LAN không ra kết quả nào → camera có thể tắt tính năng discovery, hoặc
  máy chạy kspcam không cùng VLAN với camera; thử **Quét nmap** hoặc thêm cam
  thủ công ở [Kho camera](#help/kho-camera).
- Quét nmap báo lỗi không tìm thấy nmap → cần cài `nmap` trên máy chủ chạy
  kspcam.
- Bấm **Quét nmap** không chạy, báo sai định dạng → subnet phải đúng dạng
  CIDR, ví dụ `192.168.1.0/24`.
- Thêm vào kho xong nhưng bấm **Dò** báo lỗi tài khoản/mật khẩu → quét chỉ
  tìm ra thiết bị chứ không biết tài khoản/mật khẩu; nhập đúng thông tin đăng
  nhập của camera đó, hoặc thử [Thử mật khẩu hàng loạt](#help/thu-mat-khau)
  trước khi thêm.
