# Knowledge Hub & Onboarding Flow Specification (RedBida Onboarding Hub)

## 1. Observation

### 1.1 Direct Source Code & Documentation Evidence
1. **Authoritative Requirements (`/home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md`)**:
   - Lines 12-19: Demands upgraded `#view-redbida` layout with modern glassmorphism UI, a visual **Knowledge Hub** covering 4 Onboarding Pillars, live preview for `ui_bg` (gradient) and logos, and a **Preset / One-Click Onboarding Generator** mapping `ui_title`, `camera_count`, `shinobi_group_key` to a complete key-value batch via `/private/i_sets`.
   - Lines 21-27: Demands MQTT `/private/i_sets` and `/private/i_gets` protocol integrity (`{"info": ...}`), catalog metadata updates in `internal/redbida/catalog.go`.
2. **Skill Standard (`/home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md`)**:
   - Lines 13-36: Defines standard naming conventions: `CameraXX` (Title case), `cameraXX` (lowercase `mid`), `shinobi_camera_id` / `shinobi_group_key` (10-char alphanumeric), `shinobi_token` / `shinobi_monitor_token` (IP `0.0.0.0` tokens), `video_config` (`range=72`), `custom_hashtags` (`#<UITitleNoSpaces> #BILLIARDSlive #INUTlive #highlightsports`), `ui_bg` (CSS gradient without trailing semicolon), `camera_count` & `toolbar_show_count` (exact match integers), `hls_using_go2rtc` (`true`), `logo_header` (`https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png`), `logo_header_text` (`Billiard Live - Tải clip bàn bida và livestream`), `button_generate_go2rtc_stream` (`true`).
   - Lines 39-78: Golden Template from `Camera01`: `mode: "record"`, `stream_type: "hls"`, `cutoff: "5"` (5-minute chunks, `segment_time 300`), remux `copy` (`vcodec: copy`, `stream_vcodec: copy`, `record_vcodec: copy`), `rtsp_transport: "tcp"`, `preset_stream: "ultrafast"`, `hls_time: "2"`, `hls_list_size: "2"`, `cust_input: ""`, `cust_stream: ""`, `cust_record: "-tag:v hvc1"`, audio fallback to `AAC` / `no`.
   - Lines 95-118: Section 4 defines INI format for `ui_tabs_links` with exactly 20 sections `[C01]` to `[C20]`, 4 lines per section, line 3 `vid_play_label = <ui_title>`.
3. **Current Catalog State (`internal/redbida/catalog.go`)**:
   - Line 13: `runtimeKeyRe` currently contains `toolbar_show_count` (`(?i)(...|^toolbar_show_count$... )`). This unintentionally forces `toolbar_show_count` to be `RiskProtected` (read-only), preventing operators from configuring it.
   - Line 94: `var jsonKeySet = keySet("custom_hashtags ui_tabs_links")`. Treating `ui_tabs_links` (multi-line INI text) and `custom_hashtags` (plain string) as `TypeJSON` causes `validateValue` in `service.go:333-338` to reject plain string values.
   - Lines 231-247: `metaForKey` does not group `custom_hashtags`, `logo_header_text`, `camera_count`, or `toolbar_show_count` into their intuitive domain groups.
4. **Current UI Implementation (`web/static/redbida.js` & `index.html`)**:
   - `web/static/index.html:544-577`: Currently renders a flat tabular view with only 4 basic stat cards, a simple search box, and a raw table. It completely lacks the 4 Onboarding Pillar knowledge cards, visual CSS gradient preview box, and 1-Click Onboarding Generator wizard.

---

## 2. Logic Chain

### 2.1 The 4 Core Onboarding Pillars: Deep Domain Specification

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                 REDBIDA ONBOARDING HUB                                 │
└───────┬───────────────────────┬────────────────────────┬───────────────────────┬───────┘
        │                       │                        │                       │
        ▼                       ▼                        ▼                       ▼
 ┌──────────────┐        ┌──────────────┐        ┌──────────────┐        ┌──────────────┐
 │   PILLAR 1   │        │   PILLAR 2   │        │   PILLAR 3   │        │   PILLAR 4   │
 │   Branding   │        │  Streaming   │        │   Shinobi    │        │   System &   │
 │  & Giao Diện │        │  & Go2RTC    │        │  NVR & Sync  │        │   Security   │
 └──────────────┘        └──────────────┘        └──────────────┘        └──────────────┘
```

#### Pillar 1: Branding & Giao diện Quán (Branding & Shop Experience)
| Key Name | Type | Risk | Standard / Default Value | Purpose & Visual Behavior |
|---|---|---|---|---|
| `ui_title` | `string` | `editable` | Ví dụ: `CX King Luxury`, `SD Billiards Club - CS2` | Tiêu đề chính hiển thị trên Web App khách hàng và trang tải video. |
| `ui_bg` | `string` | `editable` | `radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )` | Chuỗi CSS gradient. **BẮT BUỘC KHÔNG CÓ DẤU `;` Ở CUỐI**. Render live preview gradient trực quan trên UI. |
| `logo_header` | `image` (url/data) | `editable` | `https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png` | Ảnh logo thương hiệu trên thanh header. Live preview hiển thị ảnh thật. |
| `logo_header_text` | `string` | `editable` | `Billiard Live - Tải clip bàn bida và livestream` | Câu slogan/tiêu đề phụ chuẩn hóa hiển thị bên cạnh logo header. |
| `logo_livestream` | `image` (url/data) | `editable` | URL ảnh watermark hoặc data URL | Logo đè lên góc video livestream / snapshot bàn bida. |
| `ui_scoreboard` | `boolean` | `editable` | `true` | Bật/tắt widget bảng điểm / scoreboard số bàn trực tiếp. |
| `ui_tabs_links` | `string` (multiline INI) | `editable` | 20 sections `[C01]`..`[C20]` (xem spec 2.2) | Cấu hình nhãn 20 tab bàn bida, `vid_play_label` tự động gán bằng `ui_title`. |
| `custom_hashtags` | `string` | `editable` | `#<UITitleNoSpaces> #BILLIARDSlive #INUTlive #highlightsports` | Chuỗi hashtags chuẩn hóa phục vụ SEO và chia sẻ video mạng xã hội. |

#### Pillar 2: Video Streaming & Go2RTC Engine (Low-Latency Video Pipeline)
| Key Name | Type | Risk | Standard / Default Value | Purpose & Operational Logic |
|---|---|---|---|---|
| `camera_count` | `number` (int) | `editable` | `5`, `8`, `10`, `12`, `16`, `20` | Số lượng camera Shinobi active thực tế trong quán. Giới hạn `[1..20]`. |
| `toolbar_show_count` | `number` (int) | `editable` | Bằng với `camera_count` | Số lượng nút camera hiển thị trên thanh công cụ UI. Luôn đồng bộ 1:1 với `camera_count`. |
| `video_config` | `string` | `editable` | `range=72` | Giới hạn tra cứu lịch sử highlight / video clip trong 72 giờ qua. |
| `hls_using_go2rtc` | `boolean` | `editable` | `true` | Kích hoạt phân phối luồng HLS qua Go2RTC để tối ưu hóa độ trễ sub-second và giảm tải CPU. |
| `hls_using_go2rtc_livestream` | `boolean` | `editable` | `true` | Phân phối luồng livestream HLS qua Go2RTC. |
| `button_generate_go2rtc_stream` | `boolean` | `confirm-required` | `true` | Gửi qua `/private/i_sets` để Node-RED 2023 tự sinh `/root/go2rtc.yaml` từ danh sách camera Shinobi. |

#### Pillar 3: Shinobi NVR Authentication & Group Sync (Recording & Template Engine)
| Key Name / Parameter | Type | Risk | Standard / Default Value | Purpose & Architectural Rules |
|---|---|---|---|---|
| `shinobi_camera_id` | `string` | `protected` / `secret` | `AWU8wJMd2l` (chuỗi 10 ký tự) | Shinobi Group ID / Group Key quản lý NVR. |
| `shinobi_group_key` | `string` | `protected` / `secret` | `AWU8wJMd2l` | Đồng bộ cùng giá trị với `shinobi_camera_id`. |
| `shinobi_token` | `string` | `protected` / `secret` | 30 ký tự alphanumeric | API Key IP `0.0.0.0` có quyền `View Streams, View Videos, Snapshots` (dùng cho khách hàng). |
| `shinobi_monitor_token` | `string` | `protected` / `secret` | 30 ký tự alphanumeric | API Key IP `0.0.0.0` có quyền `Get Monitors, View Streams, View Videos` (dùng quản trị). |
| **Golden Template: `cutoff`** | `string` / `number` | Config Template | `"5"` (**BẮT BUỘC: 5 phút / segment**) | Cắt nhỏ file ghi hình đúng 300 giây/đoạn để phục vụ tua nhanh và trích xuất highlight tức thì. |
| **Golden Template: Codec** | `string` | Config Template | `vcodec: "copy"`, `stream_vcodec: "copy"`, `record_vcodec: "copy"` | Passthrough 0% CPU transcoding, giữ nguyên chất lượng gốc từ Camera IP. |
| **Golden Template: Format & Flags** | `string` | Config Template | `ext: "mp4"`, `cust_record: "-tag:v hvc1"`, `cust_input: ""`, `cust_stream: ""` | Chuẩn H.265 MP4 playback tối ưu trên Safari/iOS/Android/Chrome. |
| **Golden Template: Audio Policy** | Workflow | Device Setup | Thử set `Audio.Compression=AAC`. Nếu OK: `record_acodec: "aac"`, `acodec: "copy"`. Nếu fail: `acodec: "no"`, `stream_acodec: "no"`, `record_acodec: "no"`. | Tự động thích ứng để tránh vỡ luồng hoặc crash FFmpeg khi camera không có mic. |

#### Pillar 4: Hệ thống & An ninh (System Integrity & Remote Access)
| Key Name | Type | Risk | Standard / Default Value | Purpose & Safety Rules |
|---|---|---|---|---|
| `frpc_config` | `string` (INI) | `protected` / `secret` | Nội dung cấu hình proxy FRP | Khai báo proxy/subdomain an toàn qua MQTT. Không sửa file `/tmp/frpc.ini` thủ công. |
| `ggcode` | `string` | `protected` / `secret` | `G-SFSDZPR95Z` | Google Analytics Tracking ID đo lường lưu lượng xem video toàn hệ thống. |
| `NTP / Time Sync` | Metric | Monitoring | `driftThresholdSeconds: 60`, `ntpSynchronized: true` | Đảm bảo đồng hồ camera, host Linux và NVR khớp chính xác từng giây để gắn nhãn highlight. |
| `RAM Watchdogs` | Numbers | `confirm-required` | `max_free_ram_restart_camera`, `max_free_ram_force_reboot`, `max_shared_ram_camera` | Bộ giám sát tự phục hồi khi bộ nhớ RAM khả dụng giảm xuống dưới ngưỡng an toàn. |

---

### 2.2 Thuật toán Chi tiết: Preset / One-Click Onboarding Generator

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ONE-CLICK ONBOARDING GENERATOR FLOW                      │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                1. Inputs: ui_title, camera_count, shinobi_group_key
                                       │
                                       ▼
       ┌───────────────────────────────────────────────────────────────┐
       │ A. Hashtag Sanitization: removeVietnameseTones + stripSpecial │
       │    Result: #<CleanTitle> #BILLIARDSlive #INUTlive #highlight..│
       ├───────────────────────────────────────────────────────────────┤
       │ B. 20-Tab INI Generator: [C01]..[C20] with vid_play_label     │
       ├───────────────────────────────────────────────────────────────┤
       │ C. Theme Preset Selector: Deep Blue Cosmos / Royal / Emerald  │
       ├───────────────────────────────────────────────────────────────┤
       │ D. Standard Keys Binding: logo_header, slogans, go2rtc, etc.  │
       └───────────────────────────────┬───────────────────────────────┘
                                       │
                                       ▼
            2. Build Complete Key-Value Map ({changes: {...}})
                                       │
                                       ▼
            3. POST /api/redbida/apply (confirmed: true)
                                       │
                                       ▼
            4. MQTT /private/i_sets -> /private/i_sets/ack
                                       │
                                       ▼
            5. Read-back Verification (/private/i_gets -> ack)
                                       │
                                       ▼
            6. UI Success Feedback & Live Preview Update
```

#### Step A: Thuật toán Chuẩn hóa Hashtag (`custom_hashtags`)
```javascript
function generateCustomHashtags(uiTitle) {
  if (!uiTitle || typeof uiTitle !== 'string') {
    return '#BILLIARDSlive #INUTlive #highlightsports';
  }
  // 1. Chuyển tiếng Việt có dấu thành không dấu
  let str = uiTitle
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[đĐ]/g, m => m === 'đ' ? 'd' : 'D');

  // 2. Loại bỏ toàn bộ khoảng trắng, dấu câu và ký tự đặc biệt (chỉ giữ chữ cái và số)
  str = str.replace(/[^a-zA-Z0-9]/g, '');

  if (!str) {
    return '#BILLIARDSlive #INUTlive #highlightsports';
  }
  return `#${str} #BILLIARDSlive #INUTlive #highlightsports`;
}
```
- **Ví dụ kiểm thử (Test Cases)**:
  - `"CX King Luxury"` $\rightarrow$ `"#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports"`
  - `"SD Billiards Club - CS2"` $\rightarrow$ `"#SDBilliardsClubCS2 #BILLIARDSlive #INUTlive #highlightsports"`
  - `"Bida 3 Băng Hoàng Gia"` $\rightarrow$ `"#Bida3BangHoangGia #BILLIARDSlive #INUTlive #highlightsports"`
  - `"Billiard & Cafe 24/7 (Quận 1)"` $\rightarrow$ `"#BilliardCafe247Quan1 #BILLIARDSlive #INUTlive #highlightsports"`

#### Step B: Thuật toán Sinh cấu hình 20 Tab Bàn Bida (`ui_tabs_links`)
File `/root/ota-mqtt/change_ok/ui_tabs_links` bắt buộc định dạng INI với đúng 20 section từ `[C01]` đến `[C20]`.
```javascript
function generateUITabsLinks(uiTitle) {
  const title = (uiTitle || 'Billiard Club').trim();
  const sections = [];
  for (let i = 1; i <= 20; i++) {
    const pad = String(i).padStart(2, '0');
    sections.push(
      `[C${pad}]\n` +
      `stream_label=Video Trực tiếp\n` +
      `vid_list_label=Danh sách highlight\n` +
      `vid_play_label=${title}\n` +
      `list_refresh_label=Cập nhật highlight`
    );
  }
  return sections.join('\n\n');
}
```

#### Step C: Danh mục Mẫu Giao diện Nền (Gradient Background Presets)
1. **Preset 1: Deep Blue Midnight (Chuẩn Mặc định)**:
   `radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )`
2. **Preset 2: Dark Royal Amethyst**:
   `linear-gradient(135deg, #180b2a 0%, #0c0414 100%)`
3. **Preset 3: Emerald Cyber Green**:
   `linear-gradient(135deg, #05261b 0%, #01130d 100%)`
4. **Preset 4: Obsidian Slate Charcoal**:
   `linear-gradient(135deg, #181b20 0%, #0d0f12 100%)`
5. **Preset 5: Crimson Luxury Velvet**:
   `linear-gradient(135deg, #2b0b14 0%, #120307 100%)`

*(Lưu ý: Mọi chuỗi gradient tuyệt đối không có dấu chấm phẩy `;` ở cuối để tương thích hoàn hảo với cú pháp CSS `background: <value>` của web app).*

#### Step D: Tổng hợp Bản đồ Key-Value Đầy đủ (Complete Output Payload)
Khi kích hoạt One-Click Generator, bộ tham số hoàn chỉnh sau sẽ được chuẩn bị:
```json
{
  "ui_title": "CX King Luxury",
  "company_name": "CX King Luxury",
  "camera_count": 8,
  "toolbar_show_count": 8,
  "ui_scoreboard": true,
  "hls_using_go2rtc": true,
  "hls_using_go2rtc_livestream": true,
  "video_config": "range=72",
  "logo_header": "https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png",
  "logo_header_text": "Billiard Live - Tải clip bàn bida và livestream",
  "custom_hashtags": "#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports",
  "ui_tabs_links": "[C01]\nstream_label=Video Trực tiếp\nvid_list_label=Danh sách highlight\nvid_play_label=CX King Luxury\nlist_refresh_label=Cập nhật highlight\n\n[C02]...",
  "ui_bg": "radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )",
  "button_generate_go2rtc_stream": true
}
```

---

## 3. Caveats & Nuances

1. **Phân loại Key trong `internal/redbida/catalog.go`**:
   - `toolbar_show_count`: Phải được gỡ bỏ khỏi `runtimeKeyRe` (dòng 13) và thêm vào `editableKeySet`, `numberKeySet`, và `numericRules` (`min: 0, max: 4096, integer: true`).
   - `ui_tabs_links` & `custom_hashtags`: Phải gỡ bỏ khỏi `jsonKeySet` (dòng 94) vì chúng là dữ liệu text nhiều dòng / chuỗi ký tự thường. Khi gỡ khỏi `jsonKeySet`, chúng sẽ được xử lý là `TypeString`, cho phép nhập liệu INI text và hashtag thoải mái mà không bị chặn bởi `validateValue`.
   - `metaForKey` Grouping: Cần ánh xạ nhóm chuẩn:
     - `Branding / Logo`: `logo_header`, `logo_header_text`, `logo_livestream`, `logo_cat_cam`, `company_name`, `banner_top`, `custom_hashtags`, `app_*`.
     - `Video & Streaming`: `camera_count`, `toolbar_show_count`, `video_config`, `hls_using_go2rtc`, `hls_using_go2rtc_livestream`, `hls_using_go2rtc_tiktok`, `button_generate_go2rtc_stream`, `default_delay_*`, `fps_default`, `livestream_default_bitrate`, `place_livestream`.
     - `UI / Display`: `ui_title`, `ui_bg`, `ui_scoreboard`, `ui_tabs_links`, `ui_css_custom`, `ui_title_color`, `ui_download_text`, `ui_fb`, `ui_zalo`, `ui_tiktok`, `ui_google`, `ui_phone`, `language`, `show_toolbar`, `large_monitor`, `help_link`, `url_live_help`.
     - `Schedule & Maintenance`: `stop_camera_*`, `button_reboot`, `button_restart_shinobi`, `max_free_ram_*`, `max_shared_ram_*`, `db_check_*`, `watch_uptime_process`.
     - `Security / Credentials`: `shinobi_*`, `mqtt_*`, `*password*`, `*token*`, `*secret*`, `ggcode`, `frpc_config`.
2. **Khóa Bảo mật vs. Onboarding (`shinobi_group_key`, `shinobi_camera_id`, `ggcode`)**:
   - Trong hệ thống hiện tại, các khóa chứa tiền tố `shinobi_` hoặc `ggcode` được nhận diện bởi `sensitiveKeyRe` nên được gán `RiskProtected` và `Secret: true` (trả về `"********"` khi refresh).
   - Nếu Onboarding Generator chỉ áp dụng các khóa giao diện (`ui_title`, `camera_count`, `toolbar_show_count`, `ui_tabs_links`, `custom_hashtags`, `ui_bg`, `logo_header`, `logo_header_text`, `video_config`, `hls_using_go2rtc`, `button_generate_go2rtc_stream`), các khóa này 100% thuộc `RiskEditable` hoặc `RiskConfirm` và apply qua MQTT thành công ngay lập tức.
   - Trường hợp muốn hỗ trợ cập nhật `shinobi_group_key` từ form Onboarding, cần lưu ý backend `Service.Apply` hiện tại chặn ghi vào các khóa `RiskProtected`. Do đó, Shinobi group sync nên được xử lý qua tab Shinobi chuyên dụng hoặc gán riêng quyền nếu kiến trúc cho phép.
3. **Giới hạn Độ dài & Kích thước**:
   - `ui_bg` và `ui_tabs_links`: Chuỗi text dài (với `ui_tabs_links` ~ 2000 ký tự). Cơ chế `validateValue` cho phép chuỗi string tối đa `2 MB` nên hoàn toàn đáp ứng tốt.
   - Logo Upload: Giới hạn tối đa 512 KiB cho ảnh data URL base64 (PNG, JPEG, WebP). Khuyến khích sử dụng URL CDN cố định (`https://vnmap-backend.inut.vn/...`) để gói tin MQTT gọn nhẹ.

---

## 4. Conclusion

Bản đặc tả tri thức này cung cấp toàn bộ nền tảng quy tắc, công thức toán học/chuỗi, cấu trúc bảng dữ liệu, và luồng tương tác cho **Trung tâm Tri thức & Bộ công cụ Onboarding RedBida**:
1. **4 Trụ Cột Tri Thức** được định nghĩa rành mạch với từng Key, Loại dữ liệu, Mức độ rủi ro, Giá trị chuẩn và Hành vi UI.
2. **Thuật toán Hashtag & INI Generator** được chuẩn hóa chính xác, đảm bảo 100% tương thích với thực tế triển khai trên các quán Bida thực địa.
3. **Các điểm nghẽn trong `catalog.go`** (`runtimeKeyRe` chặn `toolbar_show_count`, `jsonKeySet` ép kiểu sai cho `ui_tabs_links`/`custom_hashtags`) đã được chỉ rõ với giải pháp drop-in.

---

## 5. Verification Method

### 5.1 Kiểm tra Đơn vị (Unit Tests)
```bash
# Kiểm tra toàn bộ package internal/redbida và internal/server
/home/ksp/go-sdk/bin/go test ./internal/redbida -v
/home/ksp/go-sdk/bin/go test ./internal/server -v -run "TestRedbida"
```

### 5.2 Kiểm tra Frontend & Syntax
```bash
node --check web/static/redbida.js
node --check web/static/app.js
```

### 5.3 Kiểm tra E2E Playwright
```bash
npx playwright test tests/ui/redbida.spec.js --workers=1
```

### 5.4 Kiểm tra Thực tế MQTT Broker
```bash
# Kiểm tra đọc danh mục key qua REST API
curl -s -b /tmp/ksp_cookie http://127.0.0.1:2028/api/redbida/catalog | jq .

# Thử nghiệm gửi refresh và kiểm tra metadata
curl -s -X POST -b /tmp/ksp_cookie http://127.0.0.1:2028/api/redbida/refresh -d '{"keys":["ui_title","camera_count","toolbar_show_count","ui_tabs_links","custom_hashtags","ui_bg"]}' | jq .
```
