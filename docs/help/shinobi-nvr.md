---
id: shinobi-nvr
title: "Quản lý Shinobi NVR"
section: import
order: 15
keywords: [shinobi, nvr, đồng bộ, sync, monitor, api, ghi hình, start, stop, record, video, clips, push, pull]
ui: "#shinobi"
covers: ["/api/shinobi/status", "/api/shinobi/monitors", "/api/shinobi/sync-to-shinobi", "/api/shinobi/sync-from-shinobi", "/api/shinobi/videos"]
related: [nhap-shinobi, kho-camera, xem-lai]
---
## Mục đích

Tích hợp quản lý tập trung và tương tác trực tiếp với **Shinobi NVR** qua REST API (sử dụng API Key và Group Key được cấp phát tự động). Cho phép xem danh sách monitors, điều khiển trạng thái luồng video, tra cứu danh sách video đã ghi hình và thực hiện đồng bộ hai chiều thủ công giữa kho `cameras.yaml` và Shinobi.

## Tính năng chính

1. **Trạng thái kết nối REST API**:
   - Hiển thị tình trạng kết nối tới máy chủ Shinobi, địa chỉ API URL, Group Key (`ke`) và số lượng monitor hiện có.
   - Nút mở nhanh giao diện Shinobi Web Dashboard.

2. **Đồng bộ thủ công hai chiều (Manual Trigger Sync)**:
   - **Đồng bộ từ KSP-Cam sang Shinobi (`POST /api/shinobi/sync-to-shinobi`)**: Tự động chuyển toàn bộ danh sách camera từ `cameras.yaml` thành các monitors trên Shinobi với URL RTSP chuẩn hóa theo từng hãng và codec `copy` (0% CPU transcoding trên Edge Gateway).
   - **Đồng bộ từ Shinobi về KSP-Cam (`POST /api/shinobi/sync-from-shinobi`)**: Tự động nạp danh sách monitors trên Shinobi vào kho `cameras.yaml`, tự động bóc tách IP, cổng cấu hình, tài khoản và kênh NVR.

3. **Quản lý & Điều khiển Monitor**:
   - Thêm monitor mới hoặc chỉnh sửa cấu hình monitor hiện có.
   - Hỗ trợ chọn nhanh từ camera có sẵn trong kho để tự điền IP, tài khoản, mật khẩu và RTSP path.
   - Điều khiển trạng thái tức thì: **Ghi hình (record)**, **Xem trực tiếp (start)**, **Tắt (stop)**.
   - Xem danh sách và tải trực tiếp các đoạn video clip (`.mp4`) đã ghi hình trên Shinobi.

## Cách sử dụng

1. Vào tab [Shinobi NVR](#shinobi).
2. Kiểm tra thẻ **Trạng thái kết nối**: nếu hiện **Đã kết nối ●** (màu xanh), hệ thống đã sẵn sàng giao tiếp với Shinobi.
3. Để xuất toàn bộ camera sang Shinobi, bấm nút **Đồng bộ từ KSP-Cam sang Shinobi**.
4. Để kéo các monitor mới từ Shinobi về kho, bấm nút **Đồng bộ từ Shinobi về KSP-Cam**.
5. Trên bảng danh sách monitor, bấm **Ghi** để bắt đầu lưu trữ video, **Xem** để mở luồng trực tiếp, hoặc **Video** để xem các bản ghi đã lưu.
