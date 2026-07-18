---
id: nhap-shinobi
title: "Nhập từ Shinobi"
section: import
order: 10
keywords: [nhập shinobi, shinobi, import, nhập camera, monitor, json, rtsp, cli, dòng lệnh, đầu ghi, chuyển hệ thống, nhập hàng loạt, kho camera]
ui: "#import"
covers: ["/api/import"]
related: [kho-camera, quet-lan, dong-lenh]
---
## Mục đích

Nhập nhanh nhiều camera cùng lúc từ file cấu hình **monitor JSON của
Shinobi** (NVR mã nguồn mở), thay vì gõ tay từng cam. Công cụ tự bóc IP và
tài khoản/mật khẩu, tự đoán **Hãng** từ đường dẫn RTSP: `/Streaming/Channels`
→ Hikvision, `/cam/realmonitor` → Dahua/KBVision.

## Cách dùng

1. Mở tab [Nhập từ Shinobi](#import).
2. Kiểm tra **Cổng Hik** (mặc định `80`) và **Cổng Dahua** (mặc định `37777`)
   — đây là cổng cấu hình sẽ gán cho camera vừa nhập, khác với cổng RTSP xem
   hình.
3. Nạp dữ liệu bằng một trong hai cách:
   - Bấm **Chọn file JSON** và chọn file export monitor của Shinobi, hoặc
   - Dán trực tiếp nội dung JSON vào ô **JSON Shinobi**.
4. Bấm **Nhập vào kho**. Thông báo hiện số camera đã nhập thành công, và số
   bị bỏ qua (thiếu host, không đọc được URL).
5. Vào tab [Kho camera](#cameras) để kiểm tra lại danh sách vừa nhập — nếu
   công cụ đoán sai Hãng hoặc tài khoản/mật khẩu chưa đúng, sửa lại thủ công
   ở đó.

Ngoài giao diện web, có thể nhập bằng **dòng lệnh** (hữu ích khi chạy trên
máy chủ không mở trình duyệt):

```bash
./kspcam --import-shinobi monitors.json \
  --import-hik-port 80 --import-dahua-port 37777
```

Lệnh này đọc file JSON, nạp thẳng vào file cấu hình camera rồi thoát (không
chạy web server). Xem thêm các cờ dòng lệnh khác ở bài
[Dòng lệnh & build](#help/dong-lenh).

## Lưu ý

- Công cụ đoán **Hãng** dựa vào đường dẫn RTSP trong JSON; nếu không nhận ra
  dạng đường dẫn nào, mặc định gán là Hikvision — kiểm tra lại sau khi nhập.
- Tài khoản/mật khẩu lấy từ URL RTSP (`rtsp://user:pass@...`) hoặc từ trường
  `muser`/`mpass` trong JSON của Shinobi.
- Cổng Hik/Dahua nhập ở đây là **cổng cấu hình** (ISAPI/DVRIP), không phải
  cổng RTSP `554` thường thấy trong URL.

## Sự cố thường gặp

- Thông báo "bỏ qua ... (thiếu host)" → monitor đó trong JSON không có IP/host
  hợp lệ (URL RTSP trống hoặc sai định dạng), Shinobi bỏ qua camera này.
- Nhập báo lỗi JSON không hợp lệ → JSON phải là một mảng monitor, hoặc object
  có khóa `monitors`/`data` chứa mảng đó; kiểm tra lại file export từ Shinobi.
- Nhập xong nhưng camera không **Dò** được → hãng hoặc cổng cấu hình bị đoán
  sai; sửa lại ở [Kho camera](#help/kho-camera) rồi bấm **Dò** lại.
