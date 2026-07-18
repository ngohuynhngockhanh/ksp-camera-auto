---
id: doc-cau-hinh
title: "Đọc cấu hình (Dò)"
section: cameras
order: 20
keywords: [dò, đọc cấu hình, kiểm tra cấu hình, thông tin luồng, độ phân giải, codec, fps, âm thanh, smart codec, GOP, bitrate, dò các cam đã chọn, dò hàng loạt, xác nhận cấu hình]
ui: "#cameras"
covers: ["/api/probe"]
related: [kho-camera, chinh-hang-loat, anh-chup]
---
## Mục đích

**Dò** kết nối trực tiếp tới camera và đọc lại cấu hình luồng thật đang chạy
trên thiết bị: độ phân giải, codec, fps, âm thanh, Smart Codec, GOP, bitrate…
Dùng để biết cam đang cấu hình ra sao trước khi đổi, và để xác nhận sau khi
[Chỉnh hàng loạt](#help/chinh-hang-loat) đã áp dụng thật sự có hiệu lực.

## Cách dùng

1. Dò từng camera: ở tab [Kho camera](#cameras), bấm nút **Dò** trên dòng
   camera muốn kiểm tra. Cột **Thông tin luồng** của dòng đó chuyển sang
   "đang dò..." rồi hiện kết quả.
2. Dò nhiều camera cùng lúc: tick chọn các camera trong danh sách, kéo xuống
   thẻ **Chỉnh hàng loạt**, bấm **Dò các cam đã chọn**. Các camera được dò
   **tuần tự từng cam một** (không song song) — theo dõi tiến trình ở thanh
   trạng thái và ô thông báo phía trên nút.
3. Kết quả hiện ngay trong cột **Thông tin luồng** của từng dòng, gồm mỗi
   luồng một dòng: kênh, tên luồng (main/sub1/sub2), độ rộng x cao, codec,
   fps, trạng thái âm thanh, Smart Codec bật/tắt, GOP, bitrate, và các dòng
   OSD hiện có.

## Lưu ý

- Kết quả dò được **giữ tạm trong trình duyệt** (không lưu lại trên máy chủ)
  — chuyển tab hoặc bấm **Xem hình**/**Tất cả kênh** vẫn dùng lại được kết
  quả dò gần nhất mà không cần dò lại.
- Với đầu ghi nhiều kênh, kết quả dò gộp cả các kênh đã cấu hình sẵn trên
  thiết bị — không phụ thuộc vào ô **Kênh** ở form Chỉnh hàng loạt.
- Camera phản hồi chậm (đầu ghi nhiều kênh) nên tăng **Timeout mỗi cam** ở
  thẻ Chỉnh hàng loạt trước khi dò hàng loạt.

## Sự cố thường gặp

- Bấm **Dò** báo lỗi ngay → sai **Cổng** cấu hình (Dahua/KBVision 37777,
  Hikvision 80) hoặc sai **Tài khoản/Mật khẩu** đã lưu trong
  [Kho camera](#help/kho-camera).
- Dò hàng loạt bị dừng giữa chừng ở một cam → cam đó không phản hồi kịp
  trong thời gian chờ; tăng **Timeout mỗi cam** rồi dò lại riêng cam đó.
- Kết quả hiện thiếu một số luồng (chỉ có main, không có sub) → một số model
  không trả về sub-stream khi chưa từng cấu hình; bấm **Chỉnh hàng loạt** với
  luồng đó một lần để camera tạo ra cấu hình cho luồng.
