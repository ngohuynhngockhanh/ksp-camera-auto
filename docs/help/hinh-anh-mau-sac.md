---
id: hinh-anh-mau-sac
title: "Chỉnh màu hình ảnh (Dahua/KBVision)"
section: dahua
order: 10
keywords: [chỉnh màu, màu, độ sáng, sáng, tối, độ tương phản, bão hòa, sắc độ, ngược sáng, chống chói, WDR, BLC, HLC, cân bằng trắng, ngày đêm, xoay ảnh, lật ảnh, phơi sáng]
ui: "#cameras"
covers: ["/api/picture"]
related: [anh-chup, ten-kenh-osd, ptz]
---
## Mục đích

Chỉnh **màu và hình ảnh** cho từng kênh camera: độ sáng, tương phản, bão hòa,
sắc độ, cân bằng trắng, chế độ ngày/đêm và các mức **chống ngược sáng**
(BLC/HLC/WDR). Có **ảnh xem trực tiếp** ngay trong hộp thoại để bạn thấy kết
quả trước và sau khi chỉnh.

> **Lưu ý:** đây là tính năng **chỉ dành cho Dahua/KBVision**. Camera Hikvision
> sẽ báo lỗi `camera này không hỗ trợ tính năng này (chỉ Dahua/KBVision)` và
> không hiện tab **Chỉnh màu**.

## Cách dùng

1. Mở tab [Kho camera](#cameras), bấm **Xem hình** (hoặc **Tất cả kênh**) ở
   camera cần chỉnh để mở khung xem.
2. Ở kênh muốn chỉnh, bấm **Sửa tên & OSD** để mở hộp thoại.
3. Chuyển sang tab **Chỉnh màu** (chỉ hiện với camera Dahua/KBVision).
4. Chọn chế độ chỉnh:
   - **Cơ bản** — các thông số hay dùng nhất, gồm: **WhiteBalance** (cân bằng
     trắng), **Flip** (lật ảnh), **Xoay ảnh**, **DayNightColor** (Luôn màu / Tự
     chuyển theo độ sáng / Luôn đen trắng), **ExposureMode** (chế độ phơi sáng)
     và các thanh kéo chống ngược sáng: **Bù ngược sáng (BLC)**,
     **Chống chói (HLC)**, **Dải tương phản rộng (WDR)**.
   - **Đầy đủ** — mở mục **Màu sắc** để chỉnh **Brightness** (độ sáng),
     **Contrast** (tương phản), **Saturation** (bão hòa), **Hue** (sắc độ);
     cùng các mục **Ảnh chung**, **Ban đêm (NightOptions)** và
     **Ban ngày phụ (NormalOptions)**.
5. Kéo các thanh trượt hoặc đổi giá trị. Với BLC/HLC/WDR, kéo về **0** là tắt,
   kéo lên cao là tăng mức.
6. Bấm **Lưu chỉnh màu**.
7. Bấm **Tải lại ảnh** ở khung xem bên trái để chụp lại và kiểm tra kết quả.

## Lưu ý

- Các thanh chống ngược sáng (BLC/HLC/WDR) chỉ hiện khi **camera thực sự có**
  thông số đó. Máy nào không hỗ trợ thì thanh tương ứng sẽ không xuất hiện.
- Công cụ chỉ gửi đi những trường **bạn thực sự thay đổi**; trường không đụng
  tới thì giữ nguyên trên thiết bị.
- Trường bị camera báo **không hỗ trợ** sẽ bị khoá (không chỉnh được) — đây là
  giới hạn của chính model camera, không phải lỗi công cụ.
- Chế độ **Đầy đủ** đọc/ghi trực tiếp mọi thông số Dahua trả về, nhãn giữ theo
  tên gốc của hãng (Brightness, Contrast...). Nếu không chắc, dùng **Cơ bản**.

## Sự cố thường gặp

- Không thấy tab **Chỉnh màu** → camera đang là Hikvision, hoặc trong kho khai
  báo sai hãng. Kiểm tra lại cột **Hãng** ở [Kho camera](#cameras).
- Đã lưu nhưng ảnh **không đổi** → bấm **Tải lại ảnh** để chụp mới (ảnh cũ có
  thể còn trong bộ nhớ đệm); một số model cần vài giây để áp màu mới.
- Kéo BLC/HLC/WDR mà hình **không sáng hơn** → cảnh quá ngược sáng vượt khả
  năng bù; thử tăng thêm WDR hoặc chỉnh **Brightness** trong chế độ Đầy đủ.
- Báo lỗi tải hoặc lưu → kiểm tra cổng cấu hình (Dahua 37777) và tài khoản;
  thử [Đọc cấu hình (Dò)](#help/doc-cau-hinh) trước.
