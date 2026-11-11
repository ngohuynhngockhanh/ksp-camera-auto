---
id: tvt-nvms
title: "Cấu hình Đầu ghi & Camera TVT / Secuvision (NVMS-9000 & ONVIF Profile S/G)"
section: admin
order: 37
keywords: [tvt, secuvision, speco, nvms, nvms-9000, onvif, profile-g, chid, nvr-0816b2, nvr]
ui: ""
covers: []
related: [mcp-server, kho-camera, nvr-bridge, shinobi-nvr]
---
## Mục đích

Hỗ trợ quản trị và trích xuất ghi hình Hybrid `0% CPU` cho dòng đầu ghi NVR và Camera IP của **Shenzhen TVT Digital Technology (`vendor: tvt`)**, bao gồm các dòng OEM quốc tế như **Secuvision Australia** (`NVR-0804B1-4P`, `NVR-0808B2-8P`, `NVR-0816B2-16P`) và **Speco Technologies N9K** bằng kiến trúc đa tầng thuần Go (`CGO_ENABLED=0`):

1. **Tầng HTTP XML `NVMS-9000` trên cổng `80` (`/reqLogin` + `/doLogin`)**:
   - Xác thực 2 bước thách thức `MD5` + `SHA-512` (`hex(SHA512(hex(MD5(password)) + "#" + nonce))`) kèm `Cookie: sessionId=<guid>` và giải mã `sessionKey` `AES-128-ECB`.
   - Quản trị đầy đủ cấu hình mã hóa luồng chính/phụ (`queryNodeEncodeInfo` / `editNodeEncodeInfo`, `queryNetworkNodeEncodeInfo` / `editNetworkNodeEncodeInfo`), tên kênh & OSD (`editDev`, `editChlWaterMark`), ổ cứng (`queryDiskInfo`), ảnh chụp nhanh (`snapChlPicture`) và truy vấn lịch sử ghi hình (`queryChlRecLog`).
2. **Tầng Chuẩn Mở `ONVIF Profile S + Profile G` (`/onvif/Media2`, `/onvif/Recording`, `/onvif/SearchRecording`)**:
   - Tự động chuyển mạch liền mạch sang `WS-UsernameToken PasswordDigest` khi tài khoản cấu hình trên Box là tài khoản chuyên dụng cho luồng RTSP/ONVIF (ví dụ tài khoản `rtsp` không có quyền đăng nhập Web Admin).
   - Đọc chuẩn xác thông số `H.265` / `H.264`, độ phân giải `2880x1620`, `GovLength` và bitrate từ `/onvif/Media2`, tự động tính độ lệch múi giờ thực tế của đầu ghi qua `GetSystemDateAndTime`, và truy vấn lịch sử ghi hình theo sự kiện `RecordingHistory/Recording/State` (`FindEvents`) hoặc lát cắt nhanh `O(1)` cho cửa sổ Hybrid 72 giờ (`GetRecordingInformation`).
3. **Tầng RTSP Live & Replay `0% CPU` trên cổng `554`**:
   - Luồng trực tiếp: `rtsp://<user>:<pass>@<host>:554/chID=<1..N>&streamType=main|sub&linkType=tcp`.
   - Luồng phát lại / trích xuất Hybrid: `rtsp://<user>:<pass>@<host>:554/chID=<1..N>&date=YYYY-MM-DD&time=HH:MM:SS&timelen=<sec+5>&streamType=main&linkType=tcp`, kết hợp chia mảnh song song `mediaexport.FastMP4Range` (`-c:v copy -an -tag:v hvc1 -movflags +faststart`) trong `/dev/shm/kspcam-clips`.
