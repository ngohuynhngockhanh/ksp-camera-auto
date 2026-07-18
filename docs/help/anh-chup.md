---
id: anh-chup
title: "Ảnh chụp nhanh"
section: cameras
order: 30
keywords: [ảnh chụp, chụp hình, xem hình, xem hình hàng loạt, snapshot, tất cả kênh, lightbox, phóng to, tải lại ảnh, kiểm tra hình ảnh, gallery, đầu ghi]
ui: "#cameras"
covers: ["/api/snapshot"]
related: [kho-camera, doc-cau-hinh, ten-kenh-osd]
---
## Mục đích

Chụp một khung hình JPEG trực tiếp từ camera để kiểm tra nhanh: cam có sống
không, hình có mờ/lệch màu không, OSD hiện đúng chưa — không cần mở phần mềm
xem camera riêng.

## Cách dùng

1. Xem hình một camera: ở tab [Kho camera](#cameras), bấm **Xem hình** trên
   dòng camera. Hộp thoại **Xem hình** mở ra, chụp theo đúng **Kênh** và
   **Luồng áp dụng** đang chọn ở thẻ Chỉnh hàng loạt (mặc định kênh 1, luồng
   Main).
2. Xem tất cả kênh (đầu ghi nhiều kênh): bấm **Tất cả kênh** trên dòng
   camera — kspcam tự dò các kênh thiết bị đang có rồi mở lưới ảnh main
   stream của từng kênh.
3. Xem hàng loạt nhiều camera: tick chọn các camera trong danh sách, kéo
   xuống thẻ Chỉnh hàng loạt, bấm **Xem hình hàng loạt**. Ảnh được tải theo
   từng đợt nhỏ (không tải cùng lúc tất cả) để không làm quá tải camera/đầu
   ghi yếu.
4. Trong lưới ảnh: mỗi ô có nút tải lại (icon vòng tròn) để chụp ảnh mới, và
   nút **Sửa tên & OSD** để mở nhanh hộp chỉnh
   [Tên kênh & OSD](#help/ten-kenh-osd) cho đúng kênh đó.
5. Bấm vào một ảnh để phóng to (lightbox); trong lightbox có **Tải lại** để
   chụp ảnh mới và **Đóng** để quay về lưới.
6. Ô nào lỗi sẽ hiện thẳng thông báo lỗi từ camera thay vì icon vỡ hình — bấm
   vào ô lỗi để thử chụp lại.

## Lưu ý

- Ảnh chụp qua RTSP bằng ffmpeg trên máy chủ kspcam, không phải chụp trực
  tiếp trên trình duyệt — nên **có thể mất vài giây** mới hiện, đặc biệt khi
  máy chủ cấu hình yếu hoặc đang có nhiều ảnh được chụp cùng lúc (kspcam giới
  hạn số tiến trình chụp chạy song song để tránh treo máy chủ).
- Ảnh vừa chụp được **giữ tạm vài giây** trên máy chủ — mở lại cùng một kênh
  ngay sau đó có thể trả về đúng ảnh cũ thay vì chụp mới; dùng nút **tải lại**
  nếu cần ảnh thật mới nhất.
- Đóng hộp thoại Xem hình / lightbox sẽ giải phóng ảnh đã tải trong trình
  duyệt — mở lại sẽ chụp lại từ đầu.

## Sự cố thường gặp

- Ô ảnh báo lỗi kết nối/timeout → camera không phản hồi kịp; bấm vào ô để thử
  lại, hoặc tăng **Timeout mỗi cam** ở thẻ Chỉnh hàng loạt nếu cam/đầu ghi
  vốn chậm.
- Ảnh không đổi dù đã đổi cấu hình → dùng nút **tải lại** (ảnh có thể đang
  được phục vụ từ bộ nhớ đệm vài giây gần nhất).
- **Tất cả kênh** báo "Không tìm thấy kênh nào" → chưa dò lần nào; bấm
  [Dò](#help/doc-cau-hinh) cho camera đó trước rồi thử lại.
- Nhiều ô cùng báo lỗi khi xem hàng loạt số lượng lớn → máy chủ đang giới hạn
  số ảnh chụp cùng lúc để không treo; đợi các ô khác tải xong rồi bấm lại các
  ô lỗi.
