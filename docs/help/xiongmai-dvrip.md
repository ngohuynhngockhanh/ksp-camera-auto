---
id: xiongmai-dvrip
title: "Cấu hình Camera & Đầu ghi Xiongmai / XMEye / iCSee (Sofia 34567 & CGI 80)"
section: admin
order: 36
keywords: [xiongmai, xmeye, icsee, vitacam, jftech, sofia, dvrip, 34567, cgi, hvr, nvr]
ui: ""
covers: []
related: [mcp-server, kho-camera, chinh-hang-loat, shinobi-nvr]
---
## Mục đích

Hỗ trợ quản trị toàn diện dòng Camera IP (Vitacam, iCSee, XMEye, JFTech) và Đầu ghi DVR/HVR/NVR của hãng **Hangzhou Xiongmai Technology (`vendor: xiongmai`)** bằng kiến trúc hướng đối tượng thuần Go (`CGO_ENABLED=0`), tự động đàm phán song song cả hai giao thức:

1. **Giao thức Nhị phân Sofia DVRIP trên cổng TCP `34567` (`SofiaTCPTransport`)**:
   - Đóng gói khung 20-byte Little-Endian (`0xFF 0x01`), xác thực băm `MD5_8`, tự động ghép mảnh đa gói (`TotalPacket` / `CurPacket`) cho các dòng Camera IP độc lập (như Vitacam, iCSee 8MB Flash không mở CGI cổng 80).
2. **Giao thức HTTP CGI + WebSocket trên cổng `80` (`CGITransport`)**:
   - Xác thực khóa công khai **RSA-1024 PKCS#1 v1.5** kết hợp phiên mã hóa **`AES-128-CBC` ZeroPadding (`DTAK`)** cho các đầu ghi HVR/NVR Xiongmai đời mới yêu cầu bắt tay bảo mật (`Ret = 205` trên cổng `34567`).
   - Hỗ trợ xuyên kênh kỹ thuật số **`bypass@AVEnc.Encode.[nDig]`** cho camera IP gắn sau đầu ghi HVR/NVR.

## Các tính năng hỗ trợ

- **Tự động dò tìm & Nhận diện (`Discovery` & `Shinobi Sync`)**:
  - Quét broadcast UDP `34569` (`IPSearchReq` MsgID `1530`), quét Nmap cổng `34567`, thăm dò `GetPreLoginInfo` trên cổng `80` và tự động nhận diện đường dẫn RTSP `/user=..._channel=1_stream=0.sdp?real_stream`.
- **Đọc thông số & Chỉnh hàng loạt có Read-back (`Probe` & `Bulk Apply`)**:
  - Chuyển đổi tự động bảng mã độ phân giải Xiongmai (`4K`, `5M`, `4M`, `3M`, `1080P`, `720P`, `D1`...) và quy đổi chuẩn xác `GOP` theo giây (`1..12s`) sang số khung hình (`frames`).
- **Tên kênh, 4 dòng OSD, Mạng Hex-IP, NTP, Ổ cứng & Quét kênh NVR**:
  - Đọc/ghi `ChannelTitle`, `fVideo.OSDInfo`, `NetWork.NetCommon` (chuyển đổi IP Hex Little-Endian `0x8A4DA8C0` $\leftrightarrow$ `192.168.77.138`), `NetWork.RemoteDeviceV3` và `StorageInfo`.
- **Xem lại & Cắt clip Hybrid `0% CPU` (`OPFileQuery` + `OPPlayBack` / `HttpPlayBack`)**:
  - Truy vấn phân trang 64 bản ghi qua `OPFileQuery` và rút luồng H.264/H.265 gốc qua WebSocket nhị phân hoặc Sofia `1420/1424`, đóng gói `+faststart` trực tiếp trong `/dev/shm/kspcam-clips`.
