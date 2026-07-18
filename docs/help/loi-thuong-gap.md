---
id: loi-thuong-gap
title: "Lỗi thường gặp & cách xử lý"
section: admin
order: 30
keywords: [lỗi, sự cố, không kết nối, timeout, không áp dụng, không đổi, mờ, đen, snapshot chậm, mật khẩu, cổng sai, khắc phục, xử lý lỗi, config]
ui: ""
covers: []
related: [chinh-hang-loat, doc-cau-hinh, anh-chup, doi-mat-khau-thiet-bi]
---
## Mục đích

Bài tổng hợp các lỗi hay gặp khi dùng kspcam và cách xử lý, gom theo nhóm. Khi
gặp sự cố, tìm đúng nhóm bên dưới theo *hiện tượng → nguyên nhân → cách xử lý*.

## Sự cố thường gặp

### Kết nối

- **Cam không kết nối / Dò báo lỗi** → sai cổng cấu hình. Nhớ dùng đúng **cổng
  giao thức** của hãng, không phải cổng RTSP: Dahua/KBVision **37777** (đôi khi
  8888), Hikvision **80** (qua LAN) hoặc **8000** (SDK), RTSP xem hình là **554**.
  Kiểm tra lại cổng, tài khoản và mật khẩu của chính cam đó.
- **Quét mạng không thấy cam** → quét UDP (ONVIF/Dahua/Hik SADP) chỉ thấy cam
  **cùng mạng LAN**, không thấy cam qua NAT. Cam qua NAT thì thêm thủ công bằng
  `IP:cổng` đã NAT, hoặc quét subnet reachable bằng nmap.
- **Đầu ghi NVR nhiều kênh phản hồi chậm / timeout** → tăng **Timeout mỗi cam**
  (60–120 giây, tối đa 600) và thu hẹp khoảng kênh; đặt mặc định trong
  `defaults.timeout_seconds` của [config.yaml](#help/cau-hinh-yaml).
- **Hikvision qua cổng 8000 không điều khiển được** → cổng 8000 là giao thức
  đóng, mã hóa; bản mặc định không nói chuyện được với nó. Dùng **ISAPI cổng 80**
  khi ở cùng LAN, hoặc build bản đặc biệt **`hiksdk`** (nhúng HCNetSDK).

### Áp dụng cấu hình

- **Camera từ chối độ phân giải** → model đó không hỗ trợ giá trị vừa chọn; bấm
  **Dò** xem giá trị hiện tại rồi chọn preset gần nhất mà cam hỗ trợ.
- **Đổi codec báo thành công nhưng giá trị không đổi** → một số model **âm thầm
  bỏ qua** codec không hỗ trợ. Công cụ luôn **đọc lại** sau khi ghi, nên log
  đọc-lại sẽ báo đúng giá trị thật trên máy — tin vào giá trị đọc lại đó.
- **Đổi GOP/bitrate ra giá trị khác mình nhập** → thiết bị **kẹp (clamp)** về
  khoảng nó hỗ trợ. Kết quả vẫn báo OK và hiện giá trị thật đã ghi; nếu tag đích
  không có trên thiết bị thì công cụ báo lỗi rõ, không âm thầm bỏ qua.
- **Đang xem thì hình rớt vài giây khi áp dụng** → đổi độ phân giải/codec (và cả
  GOP/bitrate) làm camera **khởi động lại encoder**, luồng RTSP rớt rồi tự kết
  nối lại. Nên thao tác trong khung giờ bảo trì.

### Hình ảnh / Snapshot

- **Ảnh chụp (snapshot) chậm hoặc đen trên máy yếu** → công cụ chụp ảnh qua
  RTSP + ffmpeg; máy cấu hình thấp (RAM ít) xử lý chậm. Công cụ đã **giới hạn số
  luồng ffmpeg chạy song song** và **cache ảnh ngắn hạn** để đỡ tải — hãy chờ
  hoặc chụp ít cam một lúc.
- **Ảnh chụp báo lỗi 502 / không lấy được ảnh** → cam đó chỉ NAT cổng cấu hình
  (37777), chưa mở cổng **80** hoặc **554**. Snapshot cần một trong hai cổng này
  thông; mở cổng cho cam đó hoặc thử lúc ở cùng LAN.
- Xem thêm bài [Ảnh chụp nhanh](#help/anh-chup).

### Mật khẩu

- **Đổi mật khẩu báo từ chối (weak/rejected)** → mật khẩu chưa đủ mạnh theo yêu
  cầu hãng (nhiều model Hikvision cần 8 ký tự trở lên, có chữ và số); đặt mật
  khẩu mạnh hơn rồi thử lại. Lỗi của thiết bị hiện ngay trong nhật ký.
- **Đổi mật khẩu xong hệ thống khác mất kết nối** → NVR, Shinobi, app xem hình
  đang dùng **mật khẩu cũ** sẽ rớt; cập nhật lại mật khẩu ở mọi nơi gọi tới cam.
- **Đổi mật khẩu Dahua nhưng tên tài khoản không đổi** → với Dahua/KBVision công
  cụ chỉ đổi **mật khẩu** của tài khoản đang đăng nhập; ô Tài khoản mới chỉ đổi
  tên trên Hikvision. Chi tiết ở bài [Đổi mật khẩu camera](#help/doi-mat-khau-thiet-bi).
- **Mở lại kho thấy mật khẩu camera không đọc được** → mất khóa mã hóa
  `~/.kspcam.key`; khôi phục đúng khóa cũ, xem [config.yaml](#help/cau-hinh-yaml).
