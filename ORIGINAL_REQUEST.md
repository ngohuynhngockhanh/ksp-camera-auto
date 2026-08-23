# Original User Request

## 2026-08-23T15:56:02Z

Khám phá toàn diện codebase `ksp-camera-auto`, phân tích chi tiết kiến trúc Go, các giao thức camera (Dahua DVRIP, Hikvision ISAPI / HCNetSDK), frontend web nhúng và hệ thống kiểm thử, sau đó tổng hợp thành file tài liệu/context chuẩn `GEMINI.md` đóng vai trò là "Bộ não thứ hai" (Second Brain) và Test/Development Harness cho việc lập trình và tham vấn AI.

Working directory: /home/ksp/ksp-camera-auto
Integrity mode: development

## Requirements

### R1. Deep Architecture & Domain Analysis
Phân tích toàn diện luồng dữ liệu và cấu trúc dự án `ksp-camera-auto`:
- Luồng từ Web UI / REST API (`internal/server`, `web/`) xuống tầng điều phối tuần tự (`internal/bulk`).
- Lớp trừu tượng Camera (`internal/camera`) và cơ chế đọc/áp dụng cấu hình (Probe / Apply / Read-back verification).
- Module khám phá thiết bị mạng (`internal/discovery` với ONVIF, Dahua UDP broadcast, Hikvision SADP, nmap).
- Hệ thống quản lý cấu hình & lưu trữ kho camera (`internal/config`, `cameras.yaml`).

### R2. Protocol & Communication Specifications
Tài liệu hóa chi tiết đặc tả kỹ thuật và cơ chế giao tiếp của từng loại giao thức:
- **Dahua / KBVision DVRIP (cổng 37777)**: Cấu trúc khung nhị phân (binary framing), cơ chế băm xác thực 2 bước (two-step MD5 hash login), JSON-RPC configManager (`Encode get/set`), cơ chế keep-alive và xử lý timeout.
- **Hikvision ISAPI (cổng 80 - LAN)**: HTTP Digest Authentication, cấu trúc XML StreamingChannel (độ phân giải, bitrate, codec H.264/H.265, Smart Codec H.265+, Audio AAC).
- **Hikvision HCNetSDK (cổng 8000 - Cgo/NAT)**: Cơ chế wrapper Cgo (`internal/hiksdk`), nạp dynamic library, truyền XML qua `NET_DVR_STDXMLConfig`.
- Cơ chế an toàn (Safety & Concurrency): Nguyên tắc áp dụng tuần tự (sequential execution) từng camera để tránh quá tải hoặc làm treo thiết bị.

### R3. Development, Build & Test/Eval Harness
Tài liệu hóa chi tiết môi trường phát triển và kiểm thử:
- Quy trình build tĩnh (`CGO_ENABLED=0`) đa kiến trúc (AMD64, ARMv7, ARM64) và build Cgo (`make build-hiksdk`).
- Hướng dẫn chạy và cấu hình các bộ test hiện có (Go unit tests, Playwright E2E UI tests, script kiểm tra mẫu `chk_samples.js`, `chk_vnmap.js`).
- Hướng dẫn thiết lập test harness / mock camera simulator để lập trình viên và AI agent có thể kiểm thử logic cấu hình mà không cần thiết bị phần cứng thật.

### R4. Comprehensive `GEMINI.md` Generation
Tổng hợp và cấu trúc toàn bộ nội dung phân tích vào file `GEMINI.md` đặt tại root thư mục dự án (`/home/ksp/ksp-camera-auto/GEMINI.md`).
- Trình bày mạch lạc, sử dụng bảng biểu, Mermaid diagram mô tả kiến trúc và sequence flow.
- Cung cấp checklist quy ước code (Go conventions, error handling, backward compatibility).
- Cung cấp hướng dẫn nhanh (Quickstart) dành cho AI Agent khi nhận task mới trong repo.

## Acceptance Criteria

### Documentation Completeness & Accuracy
- [ ] File `GEMINI.md` được tạo thành công tại `/home/ksp/ksp-camera-auto/GEMINI.md` với đầy đủ nội dung, không có placeholder hoặc đánh dấu TODO/TBD.
- [ ] Tài liệu phân tích chính xác từng package trong `internal/` (`dahua`, `isapi`, `hik`, `camera`, `bulk`, `discovery`, `config`, `server`) tương ứng với code thực tế.
- [ ] Có ít nhất 2 sơ đồ Mermaid trực quan: (1) Sơ đồ tổng thể kiến trúc hệ thống (Architecture Diagram) và (2) Sơ đồ luồng xử lý cấu hình tuần tự (Sequence Diagram của Probe -> Apply -> Read-back).

### Protocol & Technical Depth
- [ ] Tài liệu mô tả rõ ràng cấu trúc gói tin / payload của cả 2 nhánh giao thức chính: Dahua DVRIP và Hikvision ISAPI/SDK.
- [ ] Liệt kê đầy đủ các tham số cấu hình camera hỗ trợ: Độ phân giải, Codec, Profile, Smart Codec/H.265+, Audio Codec.

### Test & Harness Guidance
- [ ] Có phần hướng dẫn cụ thể về Test Harness: cách chạy unit test, e2e test với Playwright, và chiến lược mock giao thức để kiểm thử an toàn.
- [ ] Lệnh build, run và test trong tài liệu được đối chiếu chính xác với `Makefile` và `package.json`.
