---
id: scheduler
title: "Lập lịch hẹn giờ camera"
section: admin
order: 35
keywords: [hẹn giờ, scheduler, lịch trình, tự động, day night, màu sắc, reboot, khởi động lại, camera, tác vụ, full color]
ui: "#cameras"
covers: ["/api/scheduler/jobs", "/api/scheduler/jobs/update", "/api/scheduler/jobs/delete", "/api/scheduler/jobs/toggle", "/api/scheduler/jobs/run", "/api/scheduler/history"]
related: [hinh-anh-mau-sac, kho-camera, bao-tri-thiet-bi]
---
## Mục đích

Lập lịch hẹn giờ tự động thực thi các tác vụ định kỳ cho camera kspcam,
đặc biệt là tự động chuyển chế độ màu hình ảnh Ngày/Đêm (Full Color ban ngày,
Auto ban đêm) cho các camera Dahua/KBVision, hoặc khởi động lại (Reboot)
thiết bị định kỳ theo giờ đã định.

Cấu hình lịch trình được lưu trữ bền vững vào tệp `schedules.json` và tự động
nạp lại khi máy chủ khởi động.

## Mặc định sẵn có

Nếu hệ thống chưa có lịch trình nào, bộ lập lịch sẽ tự động khởi tạo 2 tác vụ:
- **07:00 hàng ngày**: Chuyển Camera 2 & 3 sang chế độ **Full Color** (Luôn màu sắc).
- **17:00 hàng ngày**: Chuyển Camera 2 & 3 sang chế độ **Auto** (Tự động chuyển ngày/đêm).

## Cách dùng

1. Mở trang [Kho camera](#cameras), chọn tab **⏰ Hẹn giờ** trên thanh tác vụ.
2. Danh sách các lịch trình hiển thị với các thông tin:
   - Tên lịch trình và loại tác vụ (Chỉnh màu Ngày/Đêm hoặc Khởi động lại).
   - Thời gian kích hoạt (HH:MM) và các ngày trong tuần.
   - Danh sách camera mục tiêu.
   - Trạng thái kích hoạt (Đang bật / Đã tắt) và thời điểm chạy kế tiếp.
3. Các nút thao tác trên từng dòng:
   - **Bật/Tắt** (Toggle): nhanh chóng bật hoặc tắt lịch trình.
   - **Chạy ngay** (Run Now): kích hoạt thực thi tác vụ ngay lập tức để kiểm tra.
   - **Sửa** (Edit): mở hộp thoại chỉnh sửa thông số lịch trình.
   - **Xóa** (Delete): gỡ bỏ lịch trình khỏi hệ thống.
4. Bấm **Thêm lịch trình** để tạo tác vụ mới:
   - Đặt tên gợi nhớ cho tác vụ.
   - Chọn giờ chạy (HH:MM) và các ngày trong tuần (mặc định tất cả các ngày).
   - Chọn loại hành động:
     - **Chỉnh màu Ngày/Đêm**: chọn chế độ màu (Full Color = 0, Tự động = 1, Đen trắng = 2) và kênh.
     - **Khởi động lại**: tự động gửi lệnh reboot đến camera.
   - Tích chọn các camera cần áp dụng tác vụ.
   - Bấm **Lưu lịch trình**.
5. Theo dõi mục **Nhật ký thực thi** (History) bên dưới để xem kết quả các lần chạy gần nhất: thời gian, trạng thái thành công/thất bại và nội dung phản hồi từ thiết bị.

## Lưu ý

- Tính năng chỉnh màu Ngày/Đêm (`DayNightColor`) hỗ trợ tối ưu trên các dòng camera Dahua / KBVision / Imou.
- Tác vụ được thực thi ở tiến trình nền máy chủ, không phụ thuộc vào việc trình duyệt web có đang mở hay không.
- Nhật ký thực thi lưu trữ tối đa 50 lần chạy gần nhất dưới dạng vòng xoay (circular log).
