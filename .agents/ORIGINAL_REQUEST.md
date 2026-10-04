# Original User Request

## Request — 2026-10-04
User request:
"Kiểm tra 7.12 chạy ok ko em do có camera16 ko biết sao ko chạy xem giúp a"

Working directory: `/home/ksp/ksp-camera-auto`

Target Node:
- `inut_207_12` (`77.88.207.12`) — Poseidon Billiards (group `hk`, 17 Dahua cameras `camera01` .. `camera17`)
- Target Camera: `camera16` (Dahua `DH-IPC-HDW1230C-A-V7` on `192.168.1.216:554`)

## Root Cause Analysis
1. **Model & Chipset**:
   - `camera16` là dòng camera Dahua đời mới chạy chip **Ingenic T31N** (`DH-IPC-HDW1230C-A-V7`), khác biệt so với các camera đời trước (`HDW1230C-A`).
2. **Cấu hình sai `rtsp_transport`**:
   - Trong Shinobi database `Monitors.details`, `camera16` bị cấu hình `rtsp_transport: "no"`.
   - Điều này khiến FFmpeg kết nối RTSP bằng giao thức **UDP** thay vì **TCP**.
3. **Lỗi đứt gãy luồng HLS / mpegts**:
   - Firmware chip T31N khi xuất luồng H.265 không đính kèm PTS trên các NALU khởi tạo (VPS/SPS/PPS). Khi chạy kèm cờ `-use_wallclock_as_timestamps 1`, container `mpegts` của HLS báo lỗi nghiêm trọng `[mpegts] first pts value must be set` và crash ngay lập tức.
   - Hậu quả: FFmpeg chết liên tục sau mỗi 18 giây, tạo ra hàng loạt file rác 294 bytes và camera16 hoàn toàn không thể xem live hay ghi hình.

## Remediation & Actions Taken
1. **Cấu hình lại camera phần cứng**:
   - Chuyển `table.Encode[0].MainFormat[0].Video.Compression = H.264`, `Profile = Main`, `BitRate = 2048` trên camera `192.168.1.216`. Chuẩn H.264 trên chip T31N cung cấp đầy đủ PTS và tương thích 100% với FFmpeg.
2. **Cập nhật Shinobi Monitor `camera16`**:
   - `rtsp_transport = "tcp"` (buộc FFmpeg kéo RTSP qua TCP).
   - `wall_clock_timestamp_ignore = "1"` (không gán cờ wallclock gây lệch PTS).
   - `cust_record = ""` (xóa bỏ cờ `-tag:v hvc1` vì video là H.264).
   - `vcodec = "copy"`, `acodec = "no"`.
3. **Dọn dẹp & Khởi động lại**:
   - Xóa bỏ toàn bộ các file rác 294 bytes cũ trong `/media/usb1/hk/camera16/`.
   - Khởi động lại tiến trình PM2 `camera`.

## Acceptance Verification
- [x] FFmpeg cho `camera16` đã chạy ổn định liên tục, không bị crash-loop.
- [x] File ghi hình MP4 của `camera16` sinh đều đặn và tăng trưởng bình thường (`12 MB` cho đoạn hiện tại).
- [x] Luồng live HLS (`/dev/shm/streams/hk/camera16/s.m3u8`) sinh đầy đủ các segment `.ts` và `icon.jpg`.
- [x] Toàn bộ 17/17 camera trên node `inut_207_12` đều có 34 tiến trình FFmpeg hoạt động đầy đủ.
