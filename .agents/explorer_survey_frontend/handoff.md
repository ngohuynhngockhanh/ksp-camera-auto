# Báo Cáo Khảo Sát & Đặc Tả Kỹ Thuật Frontend UI & Glassmorphism Spec (RedBida)

**Người thực hiện**: Explorer 2 (Frontend UI & Glassmorphism Spec)  
**Ngày thực hiện**: 2026-08-24  
**Trạng thái**: Hoàn tất khảo sát & thiết kế đặc tả chi tiết (Ready for Implementation)

---

## 1. Observation (Quan Sát Codebase Hiện Tại)

Qua khảo sát chi tiết toàn bộ các file mã nguồn liên quan đến frontend và RedBida:

### 1.1 `web/static/redbida.js` (268 dòng)
- **Quản lý State (`redbidaState`)** (dòng 3–11):
  ```javascript
  const redbidaState = {
    metas: [],
    values: new Map(),
    drafts: new Map(),
    results: new Map(),
    loaded: false,
    sourceWarning: '',
    busy: false,
  };
  ```
- **Hệ thống lọc & tìm kiếm** (dòng 61–71):
  Hỗ trợ lọc theo `redbida-search` (tìm `meta.key`, `meta.label`, `meta.group`), `redbida-group` (theo nhóm) và `redbida-dirty-only` (chỉ các key có trong `drafts`).
- **Render Editor theo `valueType`** (dòng 73–88):
  - `boolean`: Render `<select>` (true/false).
  - `json`: Render `<textarea>` (parse JSON khi input, hiển thị lỗi realtime nếu sai cú pháp).
  - `image`: Render text input + `<input type="file" accept="image/png,image/jpeg,image/webp">` + `<img class="redbida-logo-preview">`. Giới hạn dung lượng file tối đa **512 KiB** (dòng 161).
  - `number`: Render `<input type="number">`.
  - `string`: Render `<input type="text">`.
  - Không `editable`: Render `<code class="redbida-protected-value">`.
- **Render Table Rows** (dòng 90–123):
  Render các dòng `<tr>` có attribute `data-red-row="${meta.key}"`, class `redbida-dirty` khi key đã bị thay đổi, badge risk `.redbida-risk-${meta.risk}`, cột trạng thái `.redbida-row-status` và cột giá trị hiện tại `.redbida-current`.
- **Giao tiếp API**:
  - `GET /api/redbida/catalog`: Lấy danh sách metadata các key.
  - `POST /api/redbida/refresh`: Yêu cầu backend đọc giá trị thực tế qua MQTT broker (`{"keys": [...]}`).
  - `POST /api/redbida/apply`: Gửi batch thay đổi (`{"changes": {...}, "confirmed": bool}`) với cơ chế xác nhận trước nếu có key thuộc nhóm `confirm-required`.
  - `GET /api/redbida/time-status`: Đọc giờ host, trạng thái NTP trust.
- **Vòng đời View (`redbidaOnShow`)** (dòng 251–258):
  Được gọi khi người dùng chuyển hash sang `#redbida`. Tự động nạp catalog, refresh giá trị và cập nhật time-status nếu chưa tải lần đầu.

### 1.2 `web/static/index.html` (Phần `#view-redbida`, dòng 543–577)
- **DOM Structure hiện tại**:
  - Tiêu đề `<div class="page-heading">` với 2 nút `#redbida-refresh` ("Đọc lại từ OTA", `data-testid="redbida-refresh"`) và `#redbida-apply` ("Submit thay đổi", `data-testid="redbida-apply"`).
  - Grid thống kê 4 thẻ `.redbida-status-grid`: `#redbida-node-status` (Node-RED), `#redbida-key-count` (Số key đã nạp), `#redbida-time-status` (Giờ host), `#redbida-ntp-status` (Trạng thái NTP).
  - Thanh công cụ `.card.redbida-toolbar`: Ô tìm kiếm `#redbida-search`, dropdown `#redbida-group`, checkbox `#redbida-dirty-only`, nút `#redbida-time-refresh`.
  - Hộp thông báo `#redbida-msg`.
  - Bảng cấu hình `.reflow.redbida-table#redbida-table` với `<tbody>` `#redbida-tbody`.
  - Dòng chú thích `.muted` ở cuối trang.

### 1.3 `web/static/style.css` & Hệ thống Design Tokens
- **CSS Variables hiện tại** (dòng 3–62):
  - Dark Mode mặc định (`:root`, `:root[data-theme="dark"]`): `--bg: #0f172a`, `--surface: #1e293b`, `--surface-2: #334155`, `--border: #334155`, `--text: #f1f5f9`, `--text-2: #cbd5e1`, `--muted: #94a3b8`, `--accent: #38bdf8`, `--accent-strong: #0ea5e9`, `--on-accent: #0c1524`, `--success: #34d399`, `--warning: #fbbf24`, `--danger: #f87171`, `--info: #7dd3fc`.
  - Light Mode (`:root[data-theme="light"]`, `@media (prefers-color-scheme: light)`): `--bg: #f1f5f9`, `--surface: #ffffff`, `--surface-2: #f8fafc`, `--border: #e2e8f0`, `--text: #0f172a`, `--text-2: #334155`, `--muted: #64748b`, `--accent: #0284c7`, `--accent-strong: #0369a1`, `--on-accent: #ffffff`.
- **CSS cho Redbida hiện tại** (dòng 171–190):
  Chỉ có 19 dòng CSS đơn giản cho grid, toolbar, editor và badge màu risk. Chưa có hiệu ứng Glassmorphism, chưa có live preview gradient, chưa có preview checkerboard cho logo, chưa có tab preview cho 20 tab INI.

### 1.4 `web/static/app.js` & `web/static/ui-core.js`
- Quản lý chuyển tab: `NAV_ITEMS` có mục `{ hash: 'redbida', label: 'RedBida / OTA', short: 'RedBida', icon: ICONS.settings, bottom: false }`.
- Gating quyền: Kích hoạt hiển thị mục menu khi `cfg?.redbidaEnabled === true` (dòng 3372).
- Các hàm tiện ích có sẵn trong `ui-core.js`:
  - `api(path, opts)`: Xử lý HTTP request JSON tự động kèm auth check.
  - `showToast(msg, type)`: Hiển thị toast thông báo góc màn hình.
  - `showConfirm(title, msg, opts)`: Hộp thoại xác nhận bất đồng bộ.
  - `setBusy(btn, busy, label)`: Hiệu ứng spinner và disable nút.
  - `escapeHtml(s)` / `cssEscape(s)`: Xử lý an toàn chuỗi HTML/CSS.

### 1.5 `tests/ui/redbida.spec.js` (Ràng buộc kiểm thử Playwright)
Tất cả các selectors sau **BẮT BUỘC** phải được giữ nguyên vẹn để không làm vỡ bất kỳ test Playwright nào:
1. `page.getByRole('heading', { name: 'RedBida / OTA-MQTT' })`
2. `#redbida-key-count`
3. `[data-red-row="<key>"]`
4. `[data-red-key="<key>"]`
5. `[data-red-file="<key>"]`
6. `#redbida-search`
7. `#redbida-group`
8. `#redbida-dirty-only`
9. `#redbida-tbody`
10. `.redbida-logo-preview`
11. `[data-testid="redbida-refresh"]` / `#redbida-refresh`
12. `[data-testid="redbida-apply"]` / `#redbida-apply`
13. `.redbida-dirty`
14. `.redbida-row-status`
15. `.redbida-current`
16. `#redbida-ntp-status`
17. `#redbida-node-status`
18. `#redbida-time-status`
19. `#redbida-msg`

---

## 2. Logic Chain (Lập Luận & Kiến Trúc Giải Pháp Frontend)

Từ các yêu cầu trong `ORIGINAL_REQUEST.md` và hiện trạng codebase:

```
[Hiện trạng UI cơ bản] 
        + 
[Yêu cầu R1: Dark/Light Glassmorphism, 4 Pillars, 1-Click Onboarding Preset, Live Previews] 
        + 
[Bảo toàn 100% Test Selectors & API Endpoints]
        ↓
{Thiết kế Kiến trúc Component Đa Tầng Mới cho #view-redbida}
```

### 2.1 Hệ thống Thiết kế Dark/Light Glassmorphism Spec
Tạo các biến CSS riêng biệt trong `style.css` kế thừa hoàn hảo bảng màu gốc của KSP-Cam:

```css
/* Glassmorphism Tokens */
:root {
  --glass-bg: rgba(30, 41, 59, 0.68);
  --glass-bg-subtle: rgba(15, 23, 42, 0.45);
  --glass-bg-card: rgba(30, 41, 59, 0.55);
  --glass-border: rgba(255, 255, 255, 0.12);
  --glass-border-subtle: rgba(255, 255, 255, 0.07);
  --glass-blur: blur(16px) saturate(180%);
  --glass-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.28);
  --glass-shadow-sm: 0 4px 16px 0 rgba(0, 0, 0, 0.18);
  --glass-glow-accent: 0 0 20px rgba(56, 189, 248, 0.25);
  --glass-glow-success: 0 0 20px rgba(52, 211, 153, 0.22);
}

:root[data-theme="light"], @media (prefers-color-scheme: light) {
  :root {
    --glass-bg: rgba(255, 255, 255, 0.78);
    --glass-bg-subtle: rgba(241, 245, 249, 0.65);
    --glass-bg-card: rgba(255, 255, 255, 0.70);
    --glass-border: rgba(0, 0, 0, 0.09);
    --glass-border-subtle: rgba(0, 0, 0, 0.05);
    --glass-blur: blur(16px) saturate(180%);
    --glass-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.07);
    --glass-shadow-sm: 0 4px 16px 0 rgba(0, 0, 0, 0.04);
  }
}
:root[data-theme="dark"] {
  --glass-bg: rgba(30, 41, 59, 0.68);
  --glass-bg-subtle: rgba(15, 23, 42, 0.45);
  --glass-bg-card: rgba(30, 41, 59, 0.55);
  --glass-border: rgba(255, 255, 255, 0.12);
  --glass-border-subtle: rgba(255, 255, 255, 0.07);
  --glass-blur: blur(16px) saturate(180%);
  --glass-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.28);
}
```

### 2.2 Kiến trúc DOM Component `#view-redbida`

Toàn bộ view được chia thành 5 khối chức năng trực quan:

```
#view-redbida
├── 1. Header & Quick Actions (.page-heading)
│   ├── Title: "RedBida / OTA-MQTT" + Subtitle
│   └── Actions: [🔄 Đọc lại từ OTA], [⚡ Áp dụng thay đổi], [⚡ Preset Onboarding 1-Click], [📚 Tri thức Onboarding]
│
├── 2. Glass Metric Cards Grid (.redbida-status-grid)
│   ├── Card 1: Node-RED 2023 (Chỉ khảo sát / Read-Only)
│   ├── Card 2: Tổng Key Đã Nạp (#redbida-key-count)
│   ├── Card 3: Giờ Máy Chủ Host (#redbida-time-status)
│   ├── Card 4: Trạng thái NTP (#redbida-ntp-status)
│   ├── Card 5 (Mới): Kênh Giao Tiếp MQTT (Broker 127.0.0.1:12369 / Read-Back Verifier)
│   └── Card 6 (Mới): Thay Đổi Chưa Ghi (Pending Drafts Badge)
│
├── 3. Preset / 1-Click Onboarding Generator (#redbida-preset-panel) (Collapsible / Featured Card)
│   ├── Form Input:
│   │   ├── Tên Quán Bida (`ui_title`) — ví dụ: "CX King Luxury"
│   │   ├── Số Camera / Bàn Bida (`camera_count`) — ví dụ: 8
│   │   ├── Group Key Shinobi (`shinobi_camera_id` / `shinobi_group_key`) — ví dụ: "CX_KING_LUXURY"
│   │   ├── Bộ chọn Theme Background (`ui_bg`) — Chọn Gradient Preset hoặc nhập CSS Custom
│   │   └── Mã Google Analytics (`ggcode`) — Mặc định: "G-SFSDZPR95Z"
│   ├── Bộ Chọn Nhanh Gradient Swatches (Royal Blue, Navy Cyber, Emerald Green, Luxury Gold, Crimson Velvet)
│   ├── Action: [⚡ Sinh Bộ Cấu Hình Chuẩn (Generate Preset)]
│   └── Visual Diff Card: Hiển thị bảng so sánh toàn bộ các key sẽ được nạp vào Draft + [🚀 Áp Dụng Ngay Lên OTA-MQTT]
│
├── 4. Trung Tâm Tri Thức Chuẩn 4 Trụ Cột (#redbida-knowledge-hub) (.redbida-pillars-grid)
│   ├── Pillar 1: Branding & Giao Diện Quán (Palette Icon)
│   │   ├── Highlight keys: `ui_title`, `ui_bg`, `logo_header`, `logo_header_text`, `logo_livestream`, `ui_scoreboard`, `ui_tabs_links`, `custom_hashtags`
│   │   └── Live Preview Box: Visual CSS Gradient Swatch + Logo Image Preview + INI Tab Simulator
│   │
│   ├── Pillar 2: Video Streaming & Go2RTC Engine (Video Icon)
│   │   ├── Highlight keys: `camera_count`, `toolbar_show_count`, `video_config` (range=72), `hls_using_go2rtc` (true), `hls_using_go2rtc_livestream`, `hls_using_go2rtc_tiktok`
│   │   └── Quick Action: [1-Click Kích Hoạt Tạo Lại Stream Go2RTC (`button_generate_go2rtc_stream`)]
│   │
│   ├── Pillar 3: Shinobi NVR Auth & Group Sync (Shield/Key Icon)
│   │   ├── Highlight keys: `shinobi_camera_id` (Group Key), `shinobi_group_key`, `shinobi_token` (API key 0.0.0.0 Streams), `shinobi_monitor_token` (API key 0.0.0.0 Monitors)
│   │   └── Golden Template Reference: cutoff=5, copy remux (0% CPU)
│   │
│   └── Pillar 4: Hệ Thống, An Ninh & Tự Phục Hồi (Server/Watchdog Icon)
│       ├── Highlight keys: `frpc_config` (FRP Cloud Tunnel), `ggcode` (Google Analytics), NTP Time Sync, RAM Watchdogs (`max_free_ram_restart_camera`, `max_free_ram_force_reboot`)
│       └── Auto-Reboot Schedules (`stop_camera_*`, `enable_hardware_reboot_camera_at_4am`)
│
├── 5. Thanh Điều Khiển, Tìm Kiếm & Lọc Nâng Cao (.card.redbida-toolbar)
│   ├── Tìm kiếm realtime: `#redbida-search` (tìm key, nhãn, giá trị, nhóm)
│   ├── Lọc theo nhóm: `#redbida-group`
│   ├── Lọc theo Risk: Dropdown ("Tất cả", "Editable", "Cần xác nhận", "Chỉ đọc / Mật")
│   ├── Checkbox: `#redbida-dirty-only` ("Chỉ thay đổi")
│   └── Nút cập nhật giờ: `#redbida-time-refresh`
│
├── 6. Hộp Thông Báo Phản Hồi (#redbida-msg) (Glass Alert Box với Icon & Animation)
│
└── 7. Bảng Cấu Hình Tham Số Thông Minh (#redbida-table) (.reflow.redbida-table)
    ├── Cột: Nhóm (Group Pill)
    ├── Cột: Key & Tên Nhãn (Code badge + Vi label)
    ├── Cột: Risk Level (Glass Badges: Editable / Confirm / Protected / Secret)
    ├── Cột: Giá Trị Hiện Tại (Smart truncated code + data URL badge)
    ├── Cột: Giá Trị Mới & Bộ Chỉnh Sửa Trực Quan:
    │   ├── Boolean: Modern Switch / Toggle
    │   ├── Number: Number Stepper Input
    │   ├── Image: Input URL / Base64 + Upload File (<=512KiB) + Live Image Thumbnail
    │   ├── CSS Gradient (`ui_bg`): CSS text input + Live Gradient Swatch Preview
    │   ├── JSON / INI: Syntax Highlighted / Formatted Textarea + Validated Indicator
    │   └── String: Input Text có gợi ý autocomplete
    └── Cột: Trạng Thái (Status Indicator: Đã đọc / Đã sửa / Đã xác minh / Lỗi)
```

---

## 3. Chi Tiết Thiết Kế Các Tính Năng Đặc Thù

### 3.1 Bộ Tạo Preset 1-Click Onboarding Generator (`redbidaGeneratePreset`)

Khi quản trị viên nhập:
1. **Tên Quán (`ui_title`)**: Ví dụ `"SD Billiards Club - CS2"`
2. **Số Camera (`camera_count`)**: Ví dụ `8`
3. **Shinobi Group Key (`shinobi_group_key`)**: Ví dụ `"SD_BILLIARDS_CS2"`
4. **Theme Gradient Preset (`ui_bg`)**: Chọn preset ví dụ `linear-gradient(135deg, #1e3c72 0%, #2a5298 100%)`
5. **Mã GGCode (`ggcode`)**: `"G-SFSDZPR95Z"`

Thuật toán tự động sinh tập hợp các key chuẩn:
```javascript
function redbidaGeneratePreset({ uiTitle, cameraCount, shinobiGroupKey, uiBg, ggcode }) {
  const count = parseInt(cameraCount, 10) || 8;
  const title = (uiTitle || 'Billiard Club').trim();
  const cleanTag = title.replace(/[^a-zA-Z0-9]/g, '');
  const groupKey = (shinobiGroupKey || cleanTag.toUpperCase()).trim();
  const bg = uiBg || 'linear-gradient(135deg, #1e3c72 0%, #2a5298 100%)';
  const ga = ggcode || 'G-SFSDZPR95Z';

  // 1. Sinh INI 20-Tab cho ui_tabs_links theo chuẩn SKILL.md
  let iniTabs = '';
  for (let i = 1; i <= 20; i++) {
    const pad = String(i).padStart(2, '0');
    iniTabs += `[C${pad}]\nstream_label=Video Trực tiếp\nvid_list_label=Danh sách highlight\nvid_play_label=${title}\nlist_refresh_label=Cập nhật highlight\n\n`;
  }
  iniTabs = iniTabs.trim();

  // 2. Sinh hashtags chuẩn
  const hashtags = [`#${cleanTag}`, '#BILLIARDSlive', '#INUTlive', '#highlightsports'];

  // 3. Tập hợp danh mục thay đổi
  const generatedChanges = {
    ui_title: title,
    ui_bg: bg,
    custom_hashtags: hashtags,
    ui_tabs_links: iniTabs,
    camera_count: count,
    toolbar_show_count: count,
    video_config: 'range=72',
    hls_using_go2rtc: true,
    hls_using_go2rtc_livestream: true,
    hls_using_go2rtc_tiktok: true,
    ui_scoreboard: true,
    shinobi_camera_id: groupKey,
    shinobi_group_key: groupKey,
    ggcode: ga,
  };

  // 4. Đưa vào redbidaState.drafts
  Object.entries(generatedChanges).forEach(([key, val]) => {
    redbidaState.drafts.set(key, val);
  });

  // 5. Cập nhật giao diện và hiển thị bản tóm tắt Diff
  redbidaRender();
  redbidaShowPresetDiff(generatedChanges);
}
```

### 3.2 Visual Live Previews (Trực Quan Hóa Tức Thời)

1. **`ui_bg` Live CSS Preview**:
   - Khung preview hiển thị trực tiếp background CSS đang soạn thảo.
   - Thư viện 6 Gradient Presets phổ biến cho quán Bida:
     - *Royal Sapphire*: `linear-gradient(135deg, #1e3c72 0%, #2a5298 100%)`
     - *Cyber Emerald*: `linear-gradient(135deg, #093028 0%, #237a57 100%)`
     - *Midnight Obsidian*: `linear-gradient(135deg, #0f2027 0%, #203a43 50%, #2c5364 100%)`
     - *Luxury Crimson*: `linear-gradient(135deg, #4b1248 0%, #f0c27b 100%)`
     - *Deep Amethyst*: `linear-gradient(135deg, #20002c 0%, #cbb4d4 100%)`
     - *Dark Titanium*: `linear-gradient(135deg, #1f1c2c 0%, #928dab 100%)`
   - Bấm vào bất kỳ preset nào sẽ tự động điền vào `ui_bg` và cập nhật preview tức thời.

2. **`logo_header` & `logo_livestream` Image Preview**:
   - Khung preview hỗ trợ 2 chế độ nền (Dark Checkerboard & Light Checkerboard) để kiểm tra độ trong suốt của file PNG/WebP.
   - Hiển thị kích thước ảnh, định dạng và dung lượng (KB).
   - Nút tải lên file trực tiếp (kèm validation <= 512 KiB) hoặc dán link URL.

3. **`ui_tabs_links` Interactive Player Simulator**:
   - Trình mô phỏng thanh tab 20 bàn (`C01`..`C20`) trực tiếp trên Web UI.
   - Cho phép quản trị viên bấm thử qua lại giữa các bàn để kiểm tra tên hiển thị (`vid_play_label`) và nhãn hiển thị trước khi submit lên MQTT.

---

## 4. Caveats & Ràng Buộc Kỹ Thuật (Constraints)

1. **Bảo tồn Tuyệt đối Selectors cho Test Automation**:
   - `#redbida-refresh`, `#redbida-apply`, `#redbida-search`, `#redbida-group`, `#redbida-dirty-only`, `#redbida-key-count`, `#redbida-node-status`, `#redbida-time-status`, `#redbida-ntp-status`, `#redbida-tbody`, `[data-red-row]`, `[data-red-key]`, `[data-red-file]`, `.redbida-dirty`, `.redbida-row-status`, `.redbida-current`, `.redbida-logo-preview` phải tồn tại trong DOM và phản hồi đúng sự kiện như ban đầu.
2. **Xác thực Đọc lại (Read-back Verification)**:
   - Thao tác ghi qua `/api/redbida/apply` không chỉ gửi `/private/i_sets` mà còn đối chiếu phản hồi `/private/i_gets` từ broker `127.0.0.1:12369`. Trạng thái dòng chỉ chuyển sang "Đã xác minh" khi `applied: true` và `verified: true`.
3. **Độ tương thích Trình duyệt (No-build requirement)**:
   - Mã nguồn tuân thủ nguyên tắc thuần Vanilla JS (ES6+), không dùng thư viện framework ngoài hay build pipeline (Webpack/Vite/Babel). Hoạt động hoàn hảo qua `go:embed static`.
4. **An toàn Mật khẩu & Dữ liệu nhạy cảm**:
   - Các key chứa `password`, `token`, `secret`, `mqtt_` được bảo vệ tự động bằng thuộc tính `secret: true`, hiển thị `********` và không cho phép chỉnh sửa trực tiếp từ giao diện để tránh rò rỉ credential hệ thống.

---

## 5. Conclusion (Kế Hoạch Triển Khai Chi Tiết)

### Danh sách các file cần chỉnh sửa trong Phase Thực Thi:

| STT | File Mục Tiêu | Nội Dung Cải Tiến |
|---|---|---|
| 1 | `web/static/style.css` | Bổ sung Glassmorphism design tokens (`--glass-*`), styles cho 4-pillar grid, preset generator, live preview swatch, checkerboard logo preview, interactive tab simulator, glass badges và responsive media queries. |
| 2 | `web/static/index.html` | Cập nhật toàn diện cấu trúc `#view-redbida` bổ sung Hero Quick-Action Bar, Preset 1-Click Generator Form, 4-Pillar Knowledge Hub, Live Preview Cards và Bảng cấu hình nâng cao. |
| 3 | `web/static/redbida.js` | Nâng cấp hàm `redbidaRender()`, bổ sung `redbidaGeneratePreset()`, `redbidaSelectBgPreset()`, `redbidaUpdateLivePreviews()`, `redbidaFilterByPillar()`, bộ xử lý kiểm tra định dạng INI/JSON và visual diff feedback. |

---

## 6. Verification Method (Phương Pháp Kiểm Thử Toàn Diện)

### 6.1 Kiểm thử Tự động (Automated Playwright Test Suite)
Chạy bộ kiểm thử giao diện Playwright để đảm bảo 100% test cases RedBida pass:
```bash
npx playwright test tests/ui/redbida.spec.js
```
Các kịch bản kiểm tra:
1. `RedBida console groups keys and keeps protected values read-only` -> PASS
2. `RedBida uses value metadata returned by refresh` -> PASS
3. `RedBida search filters and logo upload submits through apply API` -> PASS
4. `RedBida keeps failed drafts and clears verified keys after a partial apply` -> PASS
5. `RedBida invalid JSON cannot submit a stale valid draft` -> PASS
6. `RedBida renders data URL values as a compact descriptor` -> PASS
7. `RedBida navigation stays hidden when the integration is disabled` -> PASS
8. `RedBida does not present rejected input as the current value` -> PASS
9. `RedBida suppresses duplicate submit clicks while apply is in flight` -> PASS

### 6.2 Kiểm thử Đơn vị Backend (Go Unit Tests)
```bash
go test -v ./internal/redbida ./internal/server
```

### 6.3 Kiểm thử Trực quan Thủ công (Manual Visual Inspection Checklist)
1. **Kiểm tra Dark/Light Theme**:
   - Chuyển đổi giữa chế độ Sáng (`[data-theme="light"]`) và Tối (`[data-theme="dark"]`).
   - Đảm bảo hiệu ứng Glassmorphism (độ mờ viền, nền bán trong suốt, text contrast) hiển thị sắc nét và dễ đọc trên cả 2 theme.
2. **Kiểm tra 1-Click Onboarding Generator**:
   - Nhập Tên quán `"Bida Hoàng Gia"`, 12 bàn, Group key `"HOANG_GIA_01"`, chọn theme Gradient `"Cyber Emerald"`.
   - Nhấn "⚡ Sinh Bộ Cấu Hình Chuẩn" -> Kiểm tra `drafts` được điền tự động 13+ tham số (`ui_title`, `ui_bg`, `custom_hashtags`, `ui_tabs_links`, `camera_count`, v.v.).
3. **Kiểm tra Visual Live Preview**:
   - Đổi mã CSS Gradient `ui_bg` -> Khung xem trước đổi màu gradient theo thời gian thực.
   - Chọn file logo PNG trong suốt -> Khung xem trước hiển thị logo trên nền checkerboard dark/light.
4. **Kiểm tra Đáp ứng Màn hình (Mobile & Tablet Responsive)**:
   - Co giãn cửa sổ từ Mobile (375px), Tablet (768px) đến Desktop (1440px).
   - Đảm bảo các grid chuyển cột mượt mà, bảng reflow responsive không bị tràn khung ngang.
