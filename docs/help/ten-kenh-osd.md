---
id: ten-kenh-osd
title: "Tên kênh & OSD"
section: cameras
order: 40
keywords: [tên kênh, đổi tên kênh, OSD, chữ trên hình, overlay, dòng chữ, hiện chữ, sửa tên, chỉnh OSD, tên camera trên hình, watermark, xem trước]
ui: "#cameras"
covers: ["/api/channel-info", "/api/channel-name", "/api/channel-names", "/api/osd"]
related: [anh-chup, kho-camera, doc-cau-hinh, hinh-anh-mau-sac]
---
## Mục đích

Đổi **tên kênh** hiển thị ngay trên hình (chữ overlay do chính camera vẽ lên
luồng video) và các **dòng chữ OSD** khác (ngày giờ, địa điểm…), có xem trước
ảnh thật ngay trong hộp thoại. Đây là tên ghi **trên chính camera**, khác với
**Tên** trong [Kho camera](#help/kho-camera) (chỉ là nhãn quản lý nội bộ của
kspcam).

## Cách dùng

1. Mở hộp thoại xem hình cho camera cần sửa: bấm **Xem hình**, **Tất cả
   kênh** hoặc **Xem hình hàng loạt** ở tab [Kho camera](#cameras). Xem chi
   tiết ở bài [Ảnh chụp nhanh](#help/anh-chup).
2. Trên ô hình của đúng kênh cần sửa, bấm **Sửa tên & OSD**. Hộp thoại
   **Sửa tên & OSD** mở ra, bên trái là ảnh xem trước, bên phải là form (tab
   **Tên & OSD** được chọn sẵn).
3. Đợi vài giây để kspcam đọc cấu hình hiện tại từ camera, form sẽ tự điền:
   - **Tên trên camera (kênh)** — tên overlay hiện tại.
   - Các ô **Dòng OSD 1, 2, 3…** — nội dung từng dòng chữ, mỗi dòng có ô tick
     **Hiện** để bật/tắt hiển thị dòng đó trên hình mà không cần xóa nội
     dung.
4. Sửa tên và/hoặc nội dung các dòng OSD, tick/bỏ tick **Hiện** theo ý muốn.
5. Bấm **Lưu tên & OSD**. kspcam ghi tên xuống camera trước, sau đó ghi các
   dòng OSD; thông báo cho biết đã áp dụng bao nhiêu dòng trên tổng số dòng
   đã nhập.
6. Bấm **Tải lại ảnh** (dưới ảnh xem trước) để chụp lại và kiểm tra chữ đã
   hiện đúng trên hình chưa.

## Lưu ý

- Nếu camera **không hỗ trợ chỉnh OSD qua API**, phần Dòng OSD sẽ để trống
  kèm ghi chú — bạn vẫn đổi được **Tên trên camera** bình thường.
- Số dòng OSD ghi xuống được **phụ thuộc số khe overlay** camera hỗ trợ; nếu
  bạn nhập nhiều dòng hơn khe camera có, thông báo sau khi lưu sẽ cho biết
  chỉ áp dụng được một phần (ví dụ "áp dụng 2/4 dòng").
- Với camera Dahua/KBVision, hộp thoại còn có thêm tab **Chỉnh màu** và
  **PTZ** — xem bài [Hình ảnh & màu sắc](#help/hinh-anh-mau-sac) và
  [PTZ](#help/ptz).
- Đổi tên/OSD ghi thẳng xuống camera, có hiệu lực ngay trên luồng đang xem —
  không cần khởi động lại thiết bị.

## Sự cố thường gặp

- Mở hộp thoại báo "Lỗi tải..." → camera mất kết nối giữa lúc mở hình và mở
  form sửa; đóng hộp thoại, thử lại **Dò** ở [Kho camera](#help/kho-camera)
  rồi mở lại.
- Lưu xong nhưng ảnh xem trước không thấy đổi → bấm **Tải lại ảnh**, ảnh xem
  trước có thể đang hiện bản chụp cũ trong bộ nhớ đệm.
- Nhập dòng OSD nhưng camera không hiện chữ → kiểm tra lại ô tick **Hiện**
  của đúng dòng đó có đang bật không.
