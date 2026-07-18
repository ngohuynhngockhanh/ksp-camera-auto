---
id: chinh-hang-loat
title: "Chỉnh thông số hàng loạt"
section: bulk
order: 10
keywords: [chỉnh hàng loạt, áp dụng, độ phân giải, codec, H.264, H.265, MJPEG, smart codec, H.265+, GOP, I-frame, bitrate, CBR, VBR, âm thanh, AAC, luồng, main, sub, kênh, đầu ghi]
ui: "#cameras"
covers: ["/api/apply"]
related: [doc-cau-hinh, anh-chup, doi-mat-khau-thiet-bi]
---
## Mục đích

Đổi các thông số luồng video cho **nhiều camera cùng lúc**: codec, độ phân
giải, Smart Codec, GOP, bitrate, âm thanh AAC. Đây là tính năng trung tâm của
kspcam — thay vì vào từng cam chỉnh tay, bạn chọn danh sách cam và áp một cấu
hình chung.

## Cách dùng

1. Mở tab [Kho camera](#cameras), **tick chọn** các camera muốn chỉnh trong
   danh sách (cam đã chọn hiện thành chip ở thẻ **Chỉnh hàng loạt**).
2. Chọn **Luồng áp dụng**: Main / Sub1 / Sub2 (chọn được nhiều luồng).
3. Nhập **Kênh** — với camera thường để `1`; với đầu ghi nhiều kênh nhập
   khoảng, ví dụ `1-8` hoặc `1,3,5`.
4. Bật những thông số muốn đổi (không bật thì giữ nguyên trên thiết bị):
   - **Đổi codec**: H.265, H.264 (Main/High/Baseline), MJPEG.
   - **Đổi độ phân giải**: chọn preset 4K → 640x360, hoặc **Tuỳ chỉnh** nhập
     Rộng x Cao.
   - **Đổi Smart Codec (H.264+/H.265+)**: Bật/Tắt.
   - **Đổi khoảng I-frame (GOP)**: số khung hình giữa 2 I-frame.
   - **Đổi bitrate**: Kbps + chế độ CBR/VBR (để "Giữ nguyên chế độ" nếu chỉ
     đổi số). Cam Hikvision đang bật Smart Codec sẽ hiểu đây là bitrate
     trung bình.
   - **Bật âm thanh AAC**.
5. Chỉnh **Timeout mỗi cam** nếu có đầu ghi phản hồi chậm (mặc định 30 giây).
6. Bấm **Dò các cam đã chọn** để xem cấu hình hiện tại trước khi đổi
   (khuyến nghị), rồi bấm **Áp dụng**.
7. Theo dõi **log trực tiếp**: từng cam, từng bước, thành công/lỗi hiện ngay.
   Kết quả tổng hợp nằm ở bảng **Kết quả** cuối trang.

## Lưu ý

> **Lưu ý:** đổi độ phân giải/codec làm camera **khởi động lại encoder** —
> luồng RTSP đang xem sẽ rớt vài giây rồi tự kết nối lại. Nên thao tác trong
> khung giờ bảo trì.

- Các cam được áp dụng **tuần tự từng cam một** (không chạy song song) để an
  toàn; danh sách dài sẽ mất thời gian tương ứng.
- Sau khi ghi, công cụ **đọc lại cấu hình** từ thiết bị để xác nhận — kết quả
  báo "thành công" nghĩa là thiết bị thực sự đã nhận giá trị mới.

## Sự cố thường gặp

- Camera **từ chối độ phân giải** → model đó không hỗ trợ giá trị vừa chọn;
  bấm **Dò** xem giá trị hiện tại và chọn preset gần nhất mà cam hỗ trợ.
- Đổi codec báo thành công nhưng giá trị **không đổi** → một số model âm thầm
  bỏ qua codec không hỗ trợ; log đọc-lại sẽ báo đúng giá trị thật trên máy.
- Đầu ghi nhiều kênh bị **timeout** → tăng **Timeout mỗi cam** (60–120 giây)
  và thu hẹp khoảng kênh.
- Cam không phản hồi → kiểm tra cổng cấu hình (Dahua 37777, Hikvision 80) và
  tài khoản; thử [Đọc cấu hình (Dò)](#help/doc-cau-hinh) trước.
