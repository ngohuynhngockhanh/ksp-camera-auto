# Báo Cáo Khảo Sát & Đặc Tả Thiết Kế: Shinobi Go Client & Full Management Engine (R2)

> **Dự án:** `ksp-camera-auto` (`kspcam`)  
> **Người thực hiện:** `teamwork_preview_explorer` (Survey Shinobi Engine)  
> **Mục tiêu:** Khảo sát chi tiết hiện trạng codebase, thiết kế module pure-Go `internal/shinobi`, cơ chế đồng bộ 2 chiều (Bi-directional Sync), các REST API endpoints trong `internal/server/`, và giao diện Web UI nhúng phục vụ quản lý Shinobi NVR.

---

## 1. Tóm tắt điều hành & Mục tiêu thiết kế

Hệ thống `ksp-camera-auto` hiện tại đã có khả năng nhập thủ công danh sách camera từ file JSON export của Shinobi (`internal/importer/shinobi.go`), nhưng chưa có khả năng giao tiếp trực tiếp qua REST API với Shinobi NVR.

Yêu cầu **R2** đặt ra mục tiêu:
1. Xây dựng thư viện client thuần Go (`internal/shinobi`) tương tác trực tiếp với Shinobi REST API bằng API Key & Group Key được cấp phát từ Ansible (R1).
2. Hỗ trợ đầy đủ vòng đời quản lý Monitors: Danh sách (`ListMonitors`), Chi tiết (`GetMonitor`), Thêm mới (`AddMonitor`), Chỉnh sửa (`EditMonitor`), Xóa (`DeleteMonitor`), Điều khiển trạng thái luồng (`ChangeMonitorState`: start, stop, restart, record, idle), và Tra cứu video đã ghi hình (`GetVideos`).
3. Phát triển **Bộ điều phối đồng bộ hai chiều (Bi-directional Sync Engine)**:
   - **Inventory → Shinobi**: Tự động sinh monitor trên Shinobi từ danh sách `cameras.yaml`, tự cấu hình đường dẫn RTSP chuẩn hóa theo từng hãng (Dahua, Hikvision, Tiandy), gán codec `copy` để tối ưu 0% CPU transcoding trên Edge Gateway/Box.
   - **Shinobi → Inventory**: Tự động nạp các camera mới được thêm trên Shinobi vào kho `cameras.yaml` với cơ chế bóc tách IP, port, vendor, credentials và NVR channel.
   - **Both (Hai chiều)**: Tự động đối soát và đồng bộ hai chiều trong một thao tác.
4. Tích hợp các REST API trong `internal/server/` (`/api/shinobi/status`, `/api/shinobi/monitors`, `/api/shinobi/sync`, `/api/shinobi/videos`) và giao diện Web SPA nhúng.

---

## 2. Phân tích hiện trạng Codebase

### 2.1 Package `internal/importer` (`shinobi.go`)
- **Cấu trúc dữ liệu hiện tại:**
  ```go
  type shinobiMonitor struct {
      Name    string          `json:"name"`
      Mid     string          `json:"mid"`
      Host    string          `json:"host"`
      Details json.RawMessage `json:"details"`
  }
  type shinobiDetails struct {
      AutoHost string `json:"auto_host"` // rtsp://user:pass@host:port/path
      Muser    string `json:"muser"`
      Mpass    string `json:"mpass"`
  }
  ```
- **Kỹ thuật xử lý chuỗi JSON lồng (Stringified JSON in Details):**
  Shinobi live API trả về trường `details` dưới dạng **JSON string đã bị escape** (`"details": "{\"auto_host\":\"...\"}"`), trong khi file export thủ công có thể là JSON object lồng nhau. Hàm `parseDetails` trong `importer/shinobi.go` đã xử lý mượt mà cả hai trường hợp này:
  ```go
  func parseDetails(raw json.RawMessage) shinobiDetails {
      var d shinobiDetails
      raw = json.RawMessage(strings.TrimSpace(string(raw)))
      if len(raw) == 0 { return d }
      if raw[0] == '"' { // stringified JSON
          var s string
          if json.Unmarshal(raw, &s) == nil {
              _ = json.Unmarshal([]byte(s), &d)
          }
          return d
      }
      _ = json.Unmarshal(raw, &d)
      return d
  }
  ```
- **Nhận diện hãng (Vendor Detection) & Tách kênh NVR (Channel Extraction):**
  - Đường dẫn chứa `/cam/realmonitor` → `config.VendorDahua` (cổng cấu hình 37777).
  - Đường dẫn chứa `/Streaming/Channels` hoặc `/streaming/channels` → `config.VendorHikvision` (cổng ISAPI 80).
  - Trích xuất kênh NVR từ tham số query `?channel=N` (Dahua), đường dẫn `/Streaming/Channels/<id>` (`id/100` cho Hikvision), hoặc `/channel/stream` (Tiandy).

### 2.2 Package `internal/config`
- `config.Config`: Cần mở rộng thêm struct `Shinobi`:
  ```go
  type Shinobi struct {
      APIURL   string `yaml:"api_url"`
      APIKey   string `yaml:"api_key"`
      GroupKey string `yaml:"group_key"`
  }
  ```
- `config.Inventory`: Đã hỗ trợ thread-safe RWMutex (`sync.RWMutex`), các phương thức `List()`, `Get(id)`, `FindByHost(host, port)`, `Upsert(d)`, `Delete(id)`, `DeleteMany(ids)`. Mật khẩu được mã hóa tự động AES-256-GCM khi lưu vào đĩa.

### 2.3 Package `internal/server`
- Hệ thống routing dùng `http.ServeMux` với helper bảo vệ xác thực `requireAuth` và giới hạn kích thước payload `limitBody(8<<20, ...)`.
- Helper chuẩn hóa JSON: `writeJSON(w, code, v)`, `writeErr(w, code, msg)`.
- Phân quyền: `admin` (toàn quyền) và `viewer` (chỉ xem lại & snapshot). Các API Shinobi phải yêu cầu quyền `admin`.

### 2.4 Giao diện Web `web/static/`
- SPA viết bằng vanilla JavaScript module (`app.js`, `ui-core.js`, `review.js`).
- Navigation sidebar được khởi tạo từ mảng `NAV_ITEMS`.
- Có sẵn `api(path, opts)` helper tự động gán `Content-Type: application/json` và điều hướng khi hết hạn phiên đăng nhập (`401`).

---

## 3. Đặc tả thiết kế module `internal/shinobi`

### 3.1 Cấu trúc thư mục đề xuất
```
internal/shinobi/
├── client.go           # REST API Client: HTTP transport, query builders, error handling
├── monitor.go          # Monitor data structures, parser, serialization
├── sync.go             # Bi-directional sync engine (Inventory <-> Shinobi)
├── video.go            # Video recording metadata & queries
└── shinobi_test.go     # Full mock-based unit test suite
```

### 3.2 Đặc tả Data Models (`monitor.go` & `video.go`)

```go
package shinobi

import (
	"encoding/json"
	"time"
)

// MonitorConfig biểu diễn cấu hình đầy đủ của một monitor trên Shinobi.
type MonitorConfig struct {
	Mid      string         `json:"mid"`
	Ke       string         `json:"ke,omitempty"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`     // "h264", "mjpeg", "flv", "mp4"
	Mode     string         `json:"mode"`     // "start", "stop", "record", "idle"
	Host     string         `json:"host"`     // Camera IP / hostname
	Port     string         `json:"port"`     // RTSP port (thường là "554")
	Protocol string         `json:"protocol"` // "rtsp", "http", "https"
	Path     string         `json:"path"`     // RTSP path, ví dụ "/Streaming/Channels/101"
	Ext      string         `json:"ext"`      // "mp4", "webm"
	FPS      string         `json:"fps,omitempty"`
	Width    string         `json:"width,omitempty"`
	Height   string         `json:"height,omitempty"`
	Details  MonitorDetails `json:"details"`
}

// MonitorDetails chứa các thông số kỹ thuật chi tiết của luồng video/audio Shinobi.
type MonitorDetails struct {
	AutoHost           string `json:"auto_host"`            // rtsp://user:pass@host:554/path
	Muser              string `json:"muser"`                // RTSP Username
	Mpass              string `json:"mpass"`                // RTSP Password
	Port               string `json:"port"`                 // "554"
	Protocol           string `json:"protocol"`             // "rtsp"
	StreamType         string `json:"stream_type"`          // "mp4", "h264", "flv"
	StreamFlvType      string `json:"stream_flv_type"`     // "ws", "http"
	StreamMjpegClients string `json:"stream_mjpeg_clients"` // "1"
	Vcodec             string `json:"vcodec"`               // "copy" (Stream copy - 0% CPU Transcoding)
	Acodec             string `json:"acodec"`               // "copy", "aac", "no"
	RecordVcodec       string `json:"record_vcodec"`        // "copy"
	RecordAcodec       string `json:"record_acodec"`        // "aac"
	Detector           string `json:"detector"`             // "0"
	CustomRTSP         string `json:"custom_rtsp,omitempty"`
}

// MonitorView là cấu hình monitor trả về từ Shinobi API (Details có thể là string hoặc object).
type MonitorView struct {
	Mid      string          `json:"mid"`
	Ke       string          `json:"ke"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Mode     string          `json:"mode"`
	Host     string          `json:"host"`
	Port     string          `json:"port"`
	Protocol string          `json:"protocol"`
	Path     string          `json:"path"`
	Ext      string          `json:"ext"`
	FPS      string          `json:"fps"`
	Width    string          `json:"width"`
	Height   string          `json:"height"`
	Details  json.RawMessage `json:"details"`
}

// ParseDetails giải mã details ra struct chuẩn.
func (mv *MonitorView) ParseDetails() MonitorDetails {
	var d MonitorDetails
	raw := mv.Details
	if len(raw) == 0 {
		return d
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			_ = json.Unmarshal([]byte(s), &d)
		}
		return d
	}
	_ = json.Unmarshal(raw, &d)
	return d
}

// VideoFile biểu diễn bản ghi video được Shinobi lưu trữ.
type VideoFile struct {
	Mid      string    `json:"mid"`
	Ke       string    `json:"ke"`
	Time     time.Time `json:"time"`
	End      time.Time `json:"end"`
	Ext      string    `json:"ext"`
	Size     int64     `json:"size"`
	Href     string    `json:"href"`
	Filename string    `json:"filename"`
	Status   int       `json:"status"`
}
```

### 3.3 Đặc tả REST Client API (`client.go`)

```go
package shinobi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	apiURL   string
	apiKey   string
	groupKey string
	http     *http.Client
}

func New(apiURL, apiKey, groupKey string, timeout time.Duration) (*Client, error) {
	apiURL = strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if apiURL == "" {
		return nil, fmt.Errorf("shinobi api url is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("shinobi api key is required")
	}
	if groupKey == "" {
		return nil, fmt.Errorf("shinobi group key is required")
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		apiURL:   apiURL,
		apiKey:   apiKey,
		groupKey: groupKey,
		http:     &http.Client{Timeout: timeout},
	}, nil
}
```

#### Chi tiết các phương thức Client:

1. **`ListMonitors(ctx context.Context) ([]MonitorView, error)`**
   - **HTTP:** `GET {apiURL}/{apiKey}/monitor/{groupKey}`
   - **Xử lý phản hồi:** Shinobi có thể trả về mảng trực tiếp `[]MonitorView` hoặc `{"monitors": [...]}` hoặc `{"ok": false, "msg": "..."}`. Client tự unwrapping và kiểm tra lỗi.

2. **`GetMonitor(ctx context.Context, mid string) (*MonitorView, error)`**
   - **HTTP:** `GET {apiURL}/{apiKey}/monitor/{groupKey}/{mid}`
   - Trả về cấu hình của monitor tương ứng.

3. **`AddMonitor(ctx context.Context, mon MonitorConfig) error` & `EditMonitor(ctx context.Context, mon MonitorConfig) error`**
   - **HTTP:** `POST {apiURL}/{apiKey}/configureMonitor/{groupKey}/{mid}`
   - **Payload serialization:**
     Shinobi server nhận payload qua form field `data=<json_string>` hoặc JSON body `{ "data": <MonitorConfig> }`.
     Để đảm bảo tương thích 100% với tất cả phiên bản Shinobi Node.js:
     ```go
     monJSON, err := json.Marshal(mon)
     formData := url.Values{}
     formData.Set("data", string(monJSON))
     req, err := http.NewRequestWithContext(ctx, http.MethodPost,
         fmt.Sprintf("%s/%s/configureMonitor/%s/%s", c.apiURL, c.apiKey, c.groupKey, mon.Mid),
         strings.NewReader(formData.Encode()))
     req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
     ```
   - Kiểm tra mã phản hồi `200 OK` và nội dung JSON trả về (`ok: true` hoặc `msg: "Monitor Saved"`).

4. **`DeleteMonitor(ctx context.Context, mid string) error`**
   - **HTTP:** `GET {apiURL}/{apiKey}/monitor/{groupKey}/{mid}/delete` (hoặc `POST`/`DELETE`).
   - Kiểm tra kết quả phản hồi `ok: true`.

5. **`ChangeMonitorState(ctx context.Context, mid, state string) error`**
   - **HTTP:** `GET {apiURL}/{apiKey}/monitor/{groupKey}/{mid}/{state}`
   - Các giá trị hợp lệ của `state`: `"start"`, `"stop"`, `"record"`, `"idle"`, `"restart"`.

6. **`GetVideos(ctx context.Context, mid string, limit int) ([]VideoFile, error)`**
   - **HTTP:** `GET {apiURL}/{apiKey}/videos/{groupKey}/{mid}?limit={limit}`
   - Giải mã danh sách video clips đã lưu.

7. **`GetStatus(ctx context.Context) (StatusInfo, error)`**
   - Gọi thử `ListMonitors` với context timeout ngắn (3-5s). Nếu thành công, trả về trạng thái kết nối `connected = true`, số lượng monitor hiện có, API URL và Group Key.

---

## 4. Đặc tả cơ chế Đồng bộ 2 chiều (Bi-directional Sync Engine)

Module `sync.go` đảm nhận việc ánh xạ và đồng bộ giữa `config.Inventory` và `Shinobi Monitors`.

### 4.1 Quy tắc Ánh xạ từ Camera Inventory sang Shinobi Monitor
1. **Monitor ID (`mid`)**:
   - Định dạng chuẩn hóa không dấu, không ký tự đặc biệt:
     - Camera đơn: `cam_<host_underscores>_<port>` (Ví dụ: `cam_192_168_1_10_80`)
     - Kênh NVR: `cam_<host_underscores>_c<nvrChannel>` (Ví dụ: `cam_192_168_1_200_c5`)
2. **Xây dựng URL RTSP theo Vendor (`AutoHost` & `Path`):**
   - **Dahua / KBVision:**
     - Channel $N$ (mặc định $N=1$):
     - Main Stream: `rtsp://<user>:<pass>@<host>:554/cam/realmonitor?channel=<N>&subtype=0`
     - Sub Stream: `rtsp://<user>:<pass>@<host>:554/cam/realmonitor?channel=<N>&subtype=1`
     - Path: `/cam/realmonitor?channel=<N>&subtype=0`
   - **Hikvision:**
     - Channel $N$ (mặc định $N=1$):
     - Main Stream: `rtsp://<user>:<pass>@<host>:554/Streaming/Channels/<N*100+1>` (VD: `/Streaming/Channels/101`, `/Streaming/Channels/201`)
     - Sub Stream: `rtsp://<user>:<pass>@<host>:554/Streaming/Channels/<N*100+2>` (VD: `/Streaming/Channels/102`, `/Streaming/Channels/202`)
     - Path: `/Streaming/Channels/<N*100+1>`
   - **Tiandy:**
     - Main Stream: `rtsp://<user>:<pass>@<host>:554/cam/realmonitor?channel=<N>&subtype=0` hoặc `/<N>/1`
3. **Cấu hình Video Codec Tối ưu hóa Phần cứng:**
   - `Vcodec`: `"copy"`
   - `RecordVcodec`: `"copy"`
   - `Acodec`: `"copy"` (hoặc `"aac"`)
   - `RecordAcodec`: `"aac"`
   - `StreamType`: `"mp4"` (hoặc `"h264"`)
   - **Lợi ích:** FFmpeg trên Shinobi chỉ thực hiện remux container sang MP4 mà không giải mã/nén lại luồng H.264/H.265, giúp CPU usage trên Gateway/Box duy trì ở mức < 5% ngay cả khi chạy 16-32 kênh camera liên tục.

### 4.2 Thuật toán Đồng bộ (Sync Algorithms)

#### A. `SyncToShinobi(ctx, inv, client) -> SyncReport`
1. Lấy danh sách thiết bị từ `inv.List()`.
2. Lấy danh sách monitor hiện có từ Shinobi `client.ListMonitors(ctx)`.
3. Xây dựng map `existingByMid` và `existingByHostPath`.
4. Duyệt từng thiết bị trong Inventory:
   - Sinh cấu hình `MonitorConfig` chuẩn.
   - Nếu monitor chưa tồn tại trên Shinobi → gọi `client.AddMonitor(ctx, mon)` (`Created++`).
   - Nếu monitor đã tồn tại: so sánh URL RTSP, credentials, tên hiển thị. Nếu có thay đổi → gọi `client.EditMonitor(ctx, mon)` (`Updated++`). Nếu không đổi → `Unchanged++`.
5. Trả về báo cáo chi tiết: `{ created, updated, unchanged, errors }`.

#### B. `SyncFromShinobi(ctx, inv, client, hikPort, dahuaPort) -> ImportReport`
1. Lấy danh sách monitors từ Shinobi `client.ListMonitors(ctx)`.
2. Chuyển đổi danh sách monitors sang JSON raw và đưa qua `importer.ParseShinobi(data, hikPort, dahuaPort)`.
3. Duyệt từng camera thu được:
   - Kiểm tra xem camera đã có trong `Inventory` chưa (`inv.Get(id)` hoặc `inv.FindByHost(host, port)`).
   - Nếu chưa có: `inv.Upsert(d)` (`Added++`).
   - Nếu đã có: giữ nguyên các trường NVR-fallback (`NVRID`, `NVRChannel`, `NVRWatchdog`, `NVRSyncTimeFromHost`) và cập nhật tài khoản/mật khẩu mới nếu cần.
4. Trả về báo cáo chi tiết: `{ added, skipped, errors }`.

#### C. `SyncBoth(ctx, inv, client, hikPort, dahuaPort) -> CombinedReport`
- Thực hiện `SyncFromShinobi` trước (nạp camera mới từ Shinobi vào kho).
- Thực hiện `SyncToShinobi` sau (cập nhật lại các camera chưa có trên Shinobi).
- Tổng hợp kết quả báo cáo thống nhất.

---

## 5. Đặc tả HTTP Server REST Endpoints (`internal/server/`)

### 5.1 Cập nhật cấu hình Server & Dependency Injection
Trong `internal/server/server.go`:
```go
type Server struct {
    cfg      config.Config
    inv      *config.Inventory
    shinobi  *shinobi.Client // khởi tạo từ cfg.Shinobi nếu có cấu hình
    ...
}
```

### 5.2 Danh sách Tuyến API Shinobi (REST API Matrix)

| Tuyến Đường (Route) | Phương Thức | Quyền Hạn | Request Payload | Phản Hồi (Response) | Mục Đích |
|---|---|---|---|---|---|
| `/api/shinobi/status` | `GET` | Admin | None | `{configured, connected, apiUrl, groupKey, monitorCount, error}` | Kiểm tra trạng thái kết nối & số monitor |
| `/api/shinobi/monitors` | `GET` | Admin | None | `[]MonitorSummary` | Lấy danh sách toàn bộ monitor từ Shinobi |
| `/api/shinobi/monitors` | `POST` | Admin | `monitorActionReq` | `{ok: true, message: string}` | Thêm, sửa, xóa, hoặc đổi trạng thái monitor |
| `/api/shinobi/sync` | `POST` | Admin | `{direction: "to" \| "from" \| "both"}` | `SyncResultJSON` | Kích hoạt đồng bộ giữa kho và Shinobi |
| `/api/shinobi/videos` | `GET` | Admin | Query: `mid`, `limit` | `[]VideoFile` | Lấy danh sách video bản ghi của 1 monitor |

### 5.3 Chi tiết Handler Implementation

#### 1. `GET /api/shinobi/status`
- Kiểm tra `s.cfg.Shinobi.APIURL` và `s.cfg.Shinobi.APIKey`.
- Nếu chưa cấu hình: trả về `{ configured: false, connected: false }`.
- Nếu đã cấu hình: gọi `client.ListMonitors(ctx)` với timeout 5s.
  - Thành công: `{ configured: true, connected: true, apiUrl: "...", groupKey: "...", monitorCount: N }`.
  - Thất bại: `{ configured: true, connected: false, error: err.Error() }`.

#### 2. `GET /api/shinobi/monitors`
- Gọi `client.ListMonitors(ctx)`.
- Chuyển đổi thành danh sách tóm tắt kèm thông tin parse chi tiết (RTSP URL, user, mode, status).

#### 3. `POST /api/shinobi/monitors`
- Body:
  ```json
  {
    "action": "add" | "edit" | "delete" | "state",
    "monitorId": "cam_01",
    "state": "start" | "stop" | "record" | "restart",
    "monitor": { ... }
  }
  ```
- Định tuyến theo `action`:
  - `action == "add"`: gọi `client.AddMonitor(ctx, req.Monitor)`.
  - `action == "edit"`: gọi `client.EditMonitor(ctx, req.Monitor)`.
  - `action == "delete"`: gọi `client.DeleteMonitor(ctx, req.MonitorID)`.
  - `action == "state"`: gọi `client.ChangeMonitorState(ctx, req.MonitorID, req.State)`.

#### 4. `POST /api/shinobi/sync`
- Body: `{ "direction": "to_shinobi" | "from_shinobi" | "both" }`.
- Gọi hàm tương ứng trong `internal/shinobi/sync.go` và trả về số lượng bản ghi đã tạo/cập nhật/nạp vào kho.

#### 5. `GET /api/shinobi/videos`
- Query params: `mid` (bắt buộc), `limit` (mặc định 50).
- Gọi `client.GetVideos(ctx, mid, limit)` và trả về mảng `[]VideoFile`.

---

## 6. Đặc tả Giao diện Web UI (`web/static/`)

### 6.1 Bố cục Điều hướng (Navigation & View Structure)
- Bổ sung mục **"Shinobi NVR"** vào menu bên trái và drawer trên mobile:
  ```js
  { hash: 'shinobi', label: 'Shinobi NVR', short: 'Shinobi', icon: ICONS.video, bottom: false }
  ```
- Nâng cấp tab `view-import` hiện tại hoặc tạo view chuyên biệt `view-shinobi` gồm 4 phân khu chính:

```
+-------------------------------------------------------------------------------+
| Shinobi NVR Management                                                        |
+-------------------------------------------------------------------------------+
| [ Card 1: Trạng Thái Kết Nối & Thông Tin Hệ Thống ]                           |
|  - Trạng thái: [ Đã kết nối ● ] (Xanh) / [ Mất kết nối ● ] (Đỏ)               |
|  - API URL: http://127.0.0.1:8080   Group Key: kspbida   Monitors: 12        |
|  - Nút: [ Kiểm tra kết nối ]  [ Mở Shinobi Web Dashboard ↗ ]                 |
+-------------------------------------------------------------------------------+
| [ Card 2: Đồng Bộ Dữ Liệu Hai Chiều (Bi-directional Sync) ]                   |
|  - [ ⬆ Đồng bộ Kho → Shinobi ] (Tự tạo/cập nhật monitors trên Shinobi)        |
|  - [ ⬇ Đồng bộ Shinobi → Kho ] (Nạp camera mới từ Shinobi vào kho)            |
|  - [ ⇄ Đồng bộ 2 Chiều (Both) ]                                               |
|  - Tiến trình & Thông báo kết quả đồng bộ trực quan (Toast / Log)             |
+-------------------------------------------------------------------------------+
| [ Card 3: Danh Sách & Quản Lý Monitor Trên Shinobi ]                          |
|  - Nút: [ + Thêm Monitor Mới ]   [ 🔄 Tải Lại ]                              |
|  - Bảng dữ liệu Monitor:                                                      |
|    | ID | Tên | Host / IP | Chế độ | Codec | Trạng thái | Thao tác          |
|    | cam_10 | Cam Bàn 1 | 192.168.1.10 | Record | copy | Chạy | [Start][Stop][Rec][Sửa][Xóa][Video] |
+-------------------------------------------------------------------------------+
| [ Card 4: Nhập Thủ Công Từ File JSON (Legacy Import Fallback) ]               |
|  - Chọn file JSON hoặc dán text JSON để nhập ngoại tuyến khi cần              |
+-------------------------------------------------------------------------------+
```

### 6.2 Các Dialog / Modal Tương Tác:
1. **Modal Thêm/Sửa Monitor (`#shinobi-modal`):**
   - Điền nhanh từ danh sách camera trong kho (dropdown chọn camera có sẵn để tự điền IP, user, pass, RTSP path).
   - Thiết lập chế độ: Xem luồng (Watch/Start), Ghi hình (Record), Tắt (Stop).
   - Thiết lập Codec: `copy` (mặc định tối ưu), `h264`.
2. **Modal Danh Sách Video Ghi Hình (`#shinobi-videos-modal`):**
   - Xem danh sách các đoạn video MP4 đã lưu của camera trên Shinobi.
   - Nút Xem trực tiếp (HTML5 Video Player) hoặc Tải file về máy.

---

## 7. Khung Kiểm Thử & Mock Server Blueprint (`internal/shinobi/shinobi_test.go`)

Để đảm bảo tỷ lệ kiểm thử đạt 100% không phụ thuộc vào server thật trong lúc build/CI, chúng ta xây dựng `MockShinobiServer` dùng `httptest.Server`.

```go
package shinobi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
)

type MockShinobiServer struct {
	Server   *httptest.Server
	mu       sync.Mutex
	APIKey   string
	GroupKey string
	Monitors map[string]shinobi.MonitorConfig
	States   map[string]string
	Videos   map[string][]shinobi.VideoFile
}

func NewMockShinobiServer(apiKey, groupKey string) *MockShinobiServer {
	m := &MockShinobiServer{
		APIKey:   apiKey,
		GroupKey: groupKey,
		Monitors: make(map[string]shinobi.MonitorConfig),
		States:   make(map[string]string),
		Videos:   make(map[string][]shinobi.VideoFile),
	}
	mux := http.NewServeMux()

	// Handler /:apiKey/monitor/:groupKey
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 2 || parts[0] != m.APIKey {
			http.Error(w, `{"ok":false,"msg":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// Route: GET /:apiKey/monitor/:groupKey
		if len(parts) == 3 && parts[1] == "monitor" && parts[2] == m.GroupKey && r.Method == http.MethodGet {
			list := make([]map[string]any, 0)
			for _, mon := range m.Monitors {
				detJSON, _ := json.Marshal(mon.Details)
				list = append(list, map[string]any{
					"mid":     mon.Mid,
					"ke":      m.GroupKey,
					"name":    mon.Name,
					"type":    mon.Type,
					"mode":    m.States[mon.Mid],
					"host":    mon.Host,
					"port":    mon.Port,
					"details": string(detJSON), // Test stringified JSON
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
			return
		}

		// Route: POST /:apiKey/configureMonitor/:groupKey/:mid
		if len(parts) == 4 && parts[1] == "configureMonitor" && parts[2] == m.GroupKey && r.Method == http.MethodPost {
			mid := parts[3]
			_ = r.ParseForm()
			dataStr := r.FormValue("data")
			var cfg shinobi.MonitorConfig
			if err := json.Unmarshal([]byte(dataStr), &cfg); err != nil {
				http.Error(w, `{"ok":false,"msg":"bad json"}`, http.StatusBadRequest)
				return
			}
			cfg.Mid = mid
			m.Monitors[mid] = cfg
			if m.States[mid] == "" {
				m.States[mid] = cfg.Mode
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"msg":"Monitor Saved"}`))
			return
		}

		// Route: GET /:apiKey/monitor/:groupKey/:mid/delete
		if len(parts) == 5 && parts[1] == "monitor" && parts[2] == m.GroupKey && parts[4] == "delete" {
			mid := parts[3]
			delete(m.Monitors, mid)
			delete(m.States, mid)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"msg":"Monitor Deleted"}`))
			return
		}

		// Route: GET /:apiKey/monitor/:groupKey/:mid/:state
		if len(parts) == 5 && parts[1] == "monitor" && parts[2] == m.GroupKey {
			mid := parts[3]
			state := parts[4]
			m.States[mid] = state
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}

		// Route: GET /:apiKey/videos/:groupKey/:mid
		if len(parts) == 4 && parts[1] == "videos" && parts[2] == m.GroupKey {
			mid := parts[3]
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(m.Videos[mid])
			return
		}

		http.NotFound(w, r)
	})

	m.Server = httptest.NewServer(mux)
	return m
}

func (m *MockShinobiServer) Close() {
	m.Server.Close()
}
```

---

## 8. Kế hoạch triển khai & Phân công nhiệm vụ (Implementation Roadmap)

1. **Giai đoạn 1: Triển khai lõi Client & Sync (`internal/shinobi/`)**
   - Hiện thực hóa `MonitorConfig`, `MonitorView`, `MonitorDetails`, `VideoFile`.
   - Hiện thực hóa `Client` và tất cả các hàm CRUD + State + Videos + Status.
   - Hiện thực hóa `SyncToShinobi`, `SyncFromShinobi`, `SyncBoth` trong `sync.go`.
   - Viết toàn bộ test case kiểm thử đạt 100% pass với `httptest.Server`.

2. **Giai đoạn 2: Tích hợp Server & REST API (`internal/config` & `internal/server`)**
   - Thêm `Shinobi` struct vào `internal/config/config.go`.
   - Bổ sung các routes `/api/shinobi/status`, `/api/shinobi/monitors`, `/api/shinobi/sync`, `/api/shinobi/videos` vào `internal/server/server.go` và `api_shinobi.go`.
   - Viết server tests kiểm tra xác thực và phản hồi JSON.

3. **Giai đoạn 3: Tích hợp Giao diện Web SPA (`web/static/`)**
   - Thêm mục `Shinobi NVR` vào menu điều hướng.
   - Xây dựng giao diện xem trạng thái, danh sách monitors, nút đồng bộ 2 chiều, modal thêm/sửa monitor, modal xem video clips.

4. **Giai đoạn 4: Tài liệu & Kiểm định toàn diện**
   - Cập nhật tài liệu `GEMINI.md`, `AGENTS.md`.
   - Bổ sung tài liệu trợ giúp `docs/help/shinobi-nvr.md` và chạy `make docs`.
   - Chạy kiểm thử đa kiến trúc `make build-all` và `go test ./...`.

---
