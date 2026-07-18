---
id: doi-mat-khau-thiet-bi
title: "Đổi mật khẩu camera hàng loạt"
section: bulk
order: 20
keywords: [đổi mật khẩu, mật khẩu, pass, đổi pass, tài khoản, quên mật khẩu, reset mật khẩu, mật khẩu mới, khóa ngoài, bảo mật, hàng loạt, dahua, hikvision]
ui: "#cameras"
covers: ["/api/password"]
related: [chinh-hang-loat, kho-camera, loi-thuong-gap]
---
## Mục đích

Đổi **tài khoản/mật khẩu đăng nhập của chính camera** cho nhiều cam đã chọn
cùng lúc, ngay trong tab Kho camera (thẻ đỏ **Đổi mật khẩu camera**). Sau khi
đổi thành công, công cụ **tự cập nhật mật khẩu mới vào kho** để vẫn kết nối
được (không tự khóa mình ra ngoài).

Đây là thao tác **rủi ro cao** — hãy đọc kỹ phần Lưu ý trước khi bấm.

## Cách dùng

1. Mở tab [Kho camera](#cameras), **tick chọn** các camera cần đổi mật khẩu.
   Cam đã chọn hiện thành chip ở các thẻ bên dưới.
2. Kéo xuống thẻ đỏ **Đổi mật khẩu camera (cho các cam đã chọn)**.
3. Nhập **Tài khoản mới** (thường giữ `admin`) và **Mật khẩu mới**.
4. Bấm **Đổi mật khẩu**.
5. Theo dõi **nhật ký trực tiếp**: công cụ chạy **tuần tự từng cam một**,
   báo từng cam thành công hay lỗi. Cam nào báo *"OK — đã cập nhật kho"*
   nghĩa là đã đổi xong và mật khẩu mới đã lưu vào kho.

## Lưu ý

> **Lưu ý:** đây là thao tác không thể hoàn tác từ xa. Nếu bạn nhập **sai** hoặc
> gõ nhầm mật khẩu mới, mật khẩu sai đó sẽ được ghi vào **tất cả** cam đã chọn.
> Muốn sửa lại, bạn phải biết mật khẩu vừa đặt để đăng nhập; nếu quên thì chỉ
> còn cách **reset cứng** (nhấn nút reset trên thiết bị) tại chỗ. Hãy kiểm tra
> mật khẩu mới thật kỹ trước khi bấm.

> **Lưu ý:** các hệ thống khác đang dùng **mật khẩu cũ** (đầu ghi NVR, Shinobi,
> app xem hình, luồng RTSP đã lưu…) sẽ **mất kết nối** ngay sau khi đổi. Sau
> khi đổi mật khẩu camera, bạn phải cập nhật lại mật khẩu ở mọi nơi đang gọi
> tới cam đó.

- Mỗi hãng có **yêu cầu độ mạnh mật khẩu** riêng. Nhiều model Hikvision từ chối
  mật khẩu yếu — thường cần **8 ký tự trở lên**, có cả chữ và số. Nếu thiết bị
  từ chối, lỗi sẽ hiện ngay trong nhật ký; hãy đặt mật khẩu mạnh hơn rồi thử lại.
- Với **Dahua/KBVision**, công cụ đổi mật khẩu của **chính tài khoản đang đăng
  nhập**; ô **Tài khoản mới** chỉ có tác dụng đổi tên tài khoản trên Hikvision.
- Trên firmware Dahua đời mới, mật khẩu được gửi dưới dạng **băm (hash)** đúng
  chuẩn thiết bị; bản cũ hơn dùng dạng thô. Công cụ tự thử cả hai nên cả hai
  đời máy đều đổi được — bạn không phải làm gì thêm.
- Nên đổi **thử trên 1 cam** trước, xác nhận đăng nhập lại được, rồi mới làm
  hàng loạt.

## Sự cố thường gặp

- Đổi mật khẩu báo lỗi **từ chối** (rejected/weak) → mật khẩu chưa đủ mạnh theo
  yêu cầu của hãng; đặt mật khẩu dài hơn, thêm chữ và số rồi thử lại.
- Sau khi đổi, cam **không kết nối được nữa** → kho đã tự lưu mật khẩu mới,
  nhưng nếu bạn đổi tiếp bằng công cụ khác thì kho sẽ lệch; bấm **Dò** để kiểm
  tra, cần thì sửa lại mật khẩu trong [Kho camera](#help/kho-camera).
- Một cam báo **"không có trong kho"** → cam đó chưa được lưu; thêm lại vào
  [Kho camera](#cameras) trước khi đổi.
- Đầu ghi nhiều kênh **phản hồi chậm/timeout** → xem thêm bài
  [Lỗi thường gặp](#help/loi-thuong-gap) để tăng thời gian chờ.
