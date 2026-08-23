---
id: bao-tri-thiet-bi
title: "Thiết bị & bảo trì camera"
section: cameras
order: 40
keywords: [thiết bị, bảo trì, mạng, khởi động lại, reboot, thẻ nhớ, lưu trữ, format, tự khởi động]
ui: "#cameras"
covers: ["/api/reboot", "/api/storage", "/api/autoreboot", "/api/device-time", "/api/nvr/health", "/api/nvr/health/check", "/api/nvr/watchdog"]
related: [kho-camera, mang-ip-tinh, wifi, xem-lai]
---
## Mục đích

Mọi thao tác cấp thiết bị — mạng, lưu trữ, đồng hồ, tự khởi động — nằm trong
tab **Bảo trì** của trang chi tiết camera. Việc liên kết đầu ghi ở lại tab
**Đầu ghi** vì nó thao tác trên nhiều camera cùng lúc.

## Cách dùng

1. Mở **Camera → Danh sách**, bấm vào hàng của thiết bị cần xem.
2. Chọn tab **Bảo trì** (mạng nằm ở tab **Mạng** ngay cạnh). Tên và địa chỉ
   thiết bị đang mở luôn hiện ở đầu trang, nên không còn nguy cơ thao tác nhầm
   camera như khi phải chọn từ một danh sách xổ xuống.
3. Với đầu ghi, có lối tắt: **Camera → Đầu ghi → HDD & ghi hình**.
4. Kiểm tra kỹ thiết bị đang mở trước khi đổi IP, khởi động lại hoặc format
   thẻ nhớ. Các thao tác nguy hiểm luôn yêu cầu xác nhận.

Tab **Bảo trì** hiển thị, theo thứ tự: nút khởi động lại, dung lượng và tình
trạng thẻ nhớ/ổ cứng (kèm nút format), tình trạng ghi hình với đầu ghi, **Ngày
giờ & NTP** của thiết bị, và lịch **Tự khởi động lại**.

## Lưu ý

- Đồng hồ thiết bị lệch làm mốc thời gian bản ghi sai và tìm theo khoảng giờ ra
  kết quả rỗng; bật NTP hoặc bấm đồng bộ theo giờ trình duyệt.
- Đổi IP sai có thể làm camera mất kết nối và phải sửa trực tiếp tại hiện trường.
- Format thẻ nhớ xóa toàn bộ bản ghi trên thiết bị và không thể hoàn tác.
- Sau khi reboot, camera thường mất kết nối khoảng 30–60 giây.
