---
name: go-shinobi-compatibility
description: >-
  Chuẩn tương thích 1:1 giữa iNut NVR (Go-Shinobi-KSP :8080) với RedBida Node-RED (:2023), kspcam (:2028), Shinobi Node.js cũ và Ansible (172.16.5.180): API contract, múi giờ GMT+7 SQLite, CheckVAR flush, bảo vệ mount /media/usb1, Import/Export JSON và Onboarding không mất clip.
---

# Go-Shinobi-KSP (`iNut NVR`) — Chuẩn Tương Thích 1:1 RedBida `:2023`, `kspcam` `:2028` & Onboarding

Kỹ năng này quy định bộ hợp đồng API, xử lý múi giờ SQLite, bảo vệ mount `/media/usb1`, tương thích Import/Export JSON và quy trình Onboarding từ Shinobi Node.js cũ sang **iNut NVR (`Go-Shinobi-KSP` `:8080`)**.

---

## 1. Bảng Hợp Đồng API Shinobi Chuẩn 1:1 (`:8080` <-> RedBida `:2023` + `kspcam` `:2028` + Ansible)

Mọi triển khai `go-shinobi-ksp` bắt buộc giữ nguyên chữ ký đường dẫn và cấu trúc JSON của các endpoint sau:

| Endpoint (`:8080`) | Người Gọi Chính | Yêu Cầu Tương Thích Bắt Buộc |
|---|---|---|
| `GET,POST /?json=true` | `kspcam`, Ansible `backup-shinobi` | Nhận `mail`, `pass`, `function=dash` (JSON hoặc form); trả về `ok: true`, `auth_token`, `$user.auth_token`, `$user.ke`, `$user.uid`. |
| `GET,POST /super/?json=true` & `/super/:token/accounts/registerAdmin` | Ansible `app_ksp_bida` | Xác thực Super Admin và tự động khởi tạo GroupKey (`ke`) + User. |
| `GET /:apiKey/monitor/:ke` & `/:apiKey/monitor/:ke/:mid` | RedBida `:2023`, `kspcam`, Ansible | Trả về mảng JSON `[...]` các Monitor; `details` là chuỗi JSON-encoded chứa đầy đủ `auto_host`, `muser`, `mpass`, `input_map_choices`, `cust_record`, `cutoff`, `wall_clock_timestamp_ignore`. |
| `POST /:apiKey/configureMonitor/:ke/:mid` | `kspcam`, Shinobi Import | Hỗ trợ cả `application/x-www-form-urlencoded` (`data=<JSON>`) và `application/json`; tự động parse full URL trong `details.auto_host` và khởi động lại worker `ffmpeg`. |
| `GET /:apiKey/configureMonitor/:ke/:mid/delete` & `/:apiKey/monitor/:ke/:mid/delete` | `kspcam`, Shinobi UI | Xóa Monitor khỏi SQLite, dừng worker `ffmpeg`, hỗ trợ `?deleteFiles=true`. |
| `GET /:apiKey/monitor/:ke/:mid/:state` | `kspcam`, RedBida `:2023` | Chuyển trạng thái tức thời: `record` (Recording), `start` (Watch-Only), `idle` (Idle), `stop` (Disabled), `restart`. |
| `GET /:apiKey/hls/:ke/:mid/s.m3u8` & `/:seg.ts` (cùng alias `/hls/:ke/:mid/...`) | RedBida `:2023`, Web UI | Phục vụ luồng HLS trực tiếp từ RAM `/dev/shm/streams/<ke>/<mid>/` với `Cache-Control: no-store`. |
| `GET /:apiKey/videos/:ke` & `GET /:apiKey/videos/:ke/:mid` | RedBida `:2023`, `kspcam` | Hỗ trợ `?start=<ISO>&end=<ISO>&limit=N`; trả về `{total, limit, skip, videos: [...]}` với đầy đủ `actionUrl`, `href`, `links`, và giờ địa phương `+07:00`. |
| `GET /:apiKey/videos/:ke/:mid/:filename` | RedBida `:2023`, Mobile/iOS | Phục vụ file `.mp4` qua `http.ServeContent` (hỗ trợ HTTP `206 Partial Content` Range request cho iOS Safari) và `?download=1` (`Content-Disposition: attachment`). |
| `GET,POST /:apiKey/videos/:ke/:mid/:filename/delete` & `.../status/:status` | RedBida `:2023`, Shinobi UI | Xóa file `.mp4` vật lý + bản ghi SQLite hoặc cập nhật trạng thái đã đọc (`1`/`2`). |
| `GET /:apiKey/api/:ke/list`, `POST .../add`, `POST .../delete` | Ansible `app_ksp_bida` | Quản lý bảng `API` (`code`, `ke`, `uid`, `ip`, `details`), bảo toàn 3-tier token (`shinobi_token`, `shinobi_monitor_token`, `admin_key 127.0.0.1`). |

---

## 2. Ba Quy Tắc Sống Còn Để RedBida `:2023` Hiển Thị & Cắt Clip Đúng 100%

1. **Bắt buộc có `actionUrl` và `links` trong từng phần tử `videos[]`**:
   - Trên cổng `:2023`, hàm Node-RED `prepare to list` chạy lệnh:
     `video.inut = video.actionUrl.replace("/" + msg.shinobi_token, "");`
   - Nếu thiếu `actionUrl`, hàm này văng ngoại lệ `TypeError` và làm danh sách video tải về trên `:2023` trắng trơn.
2. **Cắt bỏ hậu tố `"Z"` giả khi đọc `DATETIME` từ `modernc.org/sqlite`**:
   - Driver `modernc.org/sqlite` khi `Scan` cột `DATETIME` (`"2006-01-02 15:04:05"`) vào biến `string` sẽ tự gắn hậu tố `"Z"` (`"2006-11-07T16:05:01Z"`).
   - Trong `parseSQLTime(raw)`: bắt buộc `raw = strings.TrimSuffix(strings.TrimSpace(raw), "Z")` rồi mới `time.ParseInLocation(..., time.Local)` để thời gian clip trả về đúng múi giờ của Box (`+07:00`), không bị lệch +7 tiếng.
3. **Parse `?start=` chuẩn `RFC3339Nano` và đổi sang `.In(time.Local)`**:
   - `:2023` truyền `?start=2026-11-07T07:00:00.000Z` (UTC có `.000Z`). Phải parse bằng `time.RFC3339Nano` và đổi `.In(time.Local)` trước khi lọc trong SQLite.

---

## 3. Bảo Vệ Mount `/media/usb1` & Chốt Clip Đang Ghi (`CheckVAR` Flush)

1. **Kiểm tra mount `/media/usb1` (`IsVideosDirMounted`) trước khi ghi hình**:
   - Trước khi khởi chạy `ffmpeg` ở chế độ `record`, bắt buộc đối chiếu `/proc/mounts` và `st_dev` của `/media/usb1` so với `/`.
   - Nếu `/media/usb1` **chưa được mount**:
     - Hạ toàn bộ camera `record` xuống chạy chế độ **Chỉ Xem (`Watch-Only` — chỉ xuất HLS `/dev/shm/streams`, tuyệt đối không tạo/ghi file `.mp4` làm đầy bộ nhớ eMMC gốc `/`)**.
     - Đặt trạng thái `Watching (Unmounted /media/usb1!)` và hiển thị cảnh báo đỏ `#mountAlarmBanner` trên Web UI.
     - Tự động kiểm tra lại mỗi 10 giây (`reconcileMountState`) để bật lại `Recording` ngay khi `/media/usb1` được mount.
2. **Chốt clip đang ghi (`FlushCheckVarSegments`) trước khi Stop / Restart / Onboarding**:
   - Thực thi `date -s "$(date '+%Y-%m-%d %H:%M:%S')"` (giống subflow `/new-check-var` trên `:2023`) kết hợp gửi `SIGINT` khi dừng `ffmpeg` để `ffmpeg -f segment -segment_atclocktime 1` đóng ngay lập tức file `.mp4` đang ghi với đầy đủ `moov` atom ở đầu file (`moov_size=262144:movflags=+faststart`) và index vào SQLite (0 clip lỗi/mất).

---

## 4. Tương Thích Import/Export JSON 1:1 & Onboarding (`make go-shinobi <target>`)

1. **Định dạng Export/Import Monitor JSON**:
   - Khi xuất JSON (`GET /api/v1/admin/monitors/export` hoặc `--export-monitors`), bắt buộc xuất **mảng JSON trần `[ { "mid": "...", "ke": "...", "details": "{...}" } ]`**.
   - Tuyệt đối không bọc trong `{"monitors": [...]}` khi xuất vì hàm `importMonitor()` của Shinobi Node.js cũ nhận diện `parsedData.monitors` là định dạng ZoneMinder.
   - Khi nhập JSON (`POST /api/v1/admin/monitors/import` hoặc `--import-monitors`), hỗ trợ cả mảng `[...]`, object đơn `{"mid":...}`, object bọc `{"monitors":[...]}` và các object nối tiếp `{...}\n{...}`.
2. **Quy trình Onboarding chuẩn trên máy chủ Ansible (`172.16.5.180`)**:
   - Chạy `make go-shinobi <target>` (`playbook/go-shinobi-onboard.yml`):
     1) Chốt clip đang ghi (`date -s`), sao lưu `mysqldump ccio > /opt/go-shinobi-ksp/backup-legacy/ccio_backup.sql`.
     2) Chạy `/opt/go-shinobi-ksp/go-shinobi-ksp --config /opt/go-shinobi-ksp/config.yaml --migrate-legacy` để chuyển 100% `Users`, `API` keys (`shinobi_token`, `shinobi_monitor_token`), `Monitors`, và `Videos` sang SQLite.
     3) Chuyển PM2 `camera` sang `go-shinobi-ksp`, tắt/disable `mariadb`/`mysql`, cài đặt `shinobi-camera-watchdog.sh` + `shinobi-storage-guard.sh` tương thích kép.
     4) Kiểm tra `make backup-shinobi <target>` và cập nhật node vào file `GO_SHINOBI_DEPLOYED.md`.
3. **Quy tắc SSH Trực Tiếp Vào Edge Node (Không Nhảy Qua `172.16.5.180`)**:
   - Khi cần SSH/SCP tới bất kỳ Edge Node nào (`77.88.200.* .. 77.88.207.*`), kết nối **trực tiếp 1-hop** bằng khóa `~/.ssh/id_rsa_macbook` (public key `~/.ssh/id_rsa_macbook.pub`, đã cấu hình tự động trong `~/.ssh/config`): `ssh root@77.88.<subnet>.<host>`. Tuyệt đối không chạy SSH lồng qua `172.16.5.180`.
