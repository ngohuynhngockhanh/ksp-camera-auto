# Handoff Report: Khảo Sát & Thiết Kế Kiến Trúc R2 (`/#redbida` Overhaul)

> **Agent:** Explorer (`explorer_survey_redbida`)  
> **Target Recipient:** Parent Orchestrator (`d0a95b30-795a-486d-a88c-9c086b9f99b0`)  
> **Thời gian:** 2026-08-24T14:43:00Z  
> **Trạng thái:** Hoàn tất khảo sát & lập bản thiết kế kiến trúc toàn diện cho R2.

---

## 1. Observation (Quan Sát Trực Tiếp Từ Codebase)

1. **Frontend HTML (`web/static/index.html:544-767`)**:
   - Giao diện `#view-redbida` hiện có 4 khối chính: Status Grid (6 metric cards), Preset Card (5 trường nhập + 6 swatch màu cũ), Knowledge Hub (4 trụ cột), Toolbar & Table (`#redbida-table`).
   - Thiếu widget **Golden Standard Inspector** để kiểm tra và tính % Chuẩn Bida.
   - Thiếu **Visual 20-Tab INI Editor** (`ui_tabs_links` hiện hiển thị `<textarea>` 4 dòng đơn giản).
   - Bộ swatch màu hiện chỉ có 6 mẫu với tên cũ, chưa đủ **8 mẫu Gradient Bida Đẳng Cấp** theo yêu cầu R2.

2. **Frontend JS Logic (`web/static/redbida.js:1-598`)**:
   - `redbidaState` lưu trữ `metas`, `values`, `drafts`, `results`, `loaded`, `sourceWarning`, `busy`.
   - Hàm `removeVietnameseTones()` dùng `normalize('NFD')` để khử dấu tiếng Việt.
   - Hàm `redbidaGeneratePreset()` sinh 15 tham số và render diff card, tuy nhiên chưa có cơ chế audit realtime từng key độc lập và nút auto-fix từng key.
   - Khi người dùng gõ tên quán, chưa có bộ sinh hashtag tự động cập nhật realtime trên input event.

3. **Backend Service & Routes (`internal/redbida/`, `internal/server/api_redbida.go`, `internal/mcp/tools_redbida.go`)**:
   - `internal/redbida/catalog.go`: Phân loại 6 nhóm (`Security / Credentials`, `Network / MQTT`, `Branding / Logo`, `Livestream`, `UI / Display`, `Schedule / Maintenance`), 4 mức Risk (`editable`, `confirm-required`, `read-only-protected`, `unknown`), và các kiểu dữ liệu (`string`, `number`, `boolean`, `json`, `image`).
   - `internal/redbida/service.go`: Thực thi `Refresh()` qua MQTT `/private/i_gets` và `Apply()` qua `/private/i_sets` với xác nhận đọc lại 3 lần.
   - `internal/server/api_redbida.go`: Cung cấp 4 API REST `/api/redbida/catalog`, `/api/redbida/refresh`, `/api/redbida/apply`, `/api/redbida/time-status`.
   - `internal/mcp/tools_redbida.go`: Đã cài đặt chuẩn hóa 15 tham số Golden Template qua công cụ `redbida_apply_onboarding_preset`.

4. **Kiểm thử hiện hành**:
   - 100% Go unit tests (`PATH=/home/ksp/.goroot/bin:$PATH go test ./...`) PASS.
   - 100% Playwright tests cho RedBida (`npx playwright test tests/ui/redbida.spec.js tests/ui/redbida_m3_challenger.spec.js`) PASS (22/22 tests).

---

## 2. Logic Chain (Chuỗi Lập Luận Kỹ Thuật)

1. **Khả năng tương thích backend**: Vì toàn bộ API Go backend và MQTT broker đã hoàn thiện, ổn định và có kiểm thử toàn diện, **việc thực hiện R2 KHÔNG đòi hỏi sửa đổi giao thức gói tin hay Go backend**, mà tập trung 100% vào cải tiến lớp giao diện (`index.html`, `redbida.js`, `style.css`).
2. **Audit Engine độc lập**: Client-side có thể tự động đối chiếu mảng `redbidaState.values` với bộ 15 quy tắc Golden Standard. Từ đó:
   - Tính toán chính xác $	ext{Score} = (	ext{Passed} / 15) 	imes 100\%$.
   - Hiển thị thanh tiến độ đổi màu (Xanh 100%, Vàng 70-99%, Đỏ <70%).
   - Cung cấp nút "⚡ Sửa nhanh 1-Click" cho từng key hoặc Auto-Fix All cho toàn bộ key bị lệch chuẩn.
3. **Visual 20-Tab INI Matrix**: Việc bóc tách chuỗi INI `ui_tabs_links` thành cấu trúc dữ liệu 20 bàn (`C01` đến `C20`) cho phép người dùng click chọn từng bàn trên ma trận trực quan, chỉnh sửa tên bàn hoặc nhãn luồng, và 1-click đồng bộ tên quán cho toàn bộ 20 bàn mà không lo lỗi cú pháp INI.
4. **Bộ 8 Gradient Palette**: 8 bảng màu được chuẩn hóa cụ thể (Royal Deep Blue Glow, Midnight Emerald Cyber, Cyberpunk Neon, Golden Velvet, Obsidian Carbon, Crimson Elegance, Sapphire Blue, Ruby Luxury) kết hợp ô chọn màu tùy biến và Live Canvas Preview tạo ra trải nghiệm thị giác chuyên nghiệp, đẳng cấp cho hệ thống Bida.

---

## 3. Caveats (Lưu Ý & Rủi Ro Tiềm Ẩn)

1. **Bảo toàn tính tương thích với Playwright UI Tests**:
   - Các selector cốt lõi như `[data-testid="redbida-refresh"]`, `[data-testid="redbida-apply"]`, `[data-red-row="..."]`, `[data-red-key="..."]`, `#redbida-preset-panel`, `#redbida-preset-diff`, `#redbida-preset-gen-btn`, `#redbida-preset-reset-btn`, `#redbida-knowledge-hub` bắt buộc phải được giữ nguyên ID và attribute để các bài test hiện hành tiếp tục pass 100%.
2. **Quy tắc dấu chấm phẩy `;` trong `ui_bg`**:
   - Tránh tuyệt đối chèn dấu `;` vào cuối chuỗi `ui_bg` khi sinh preset hoặc auto-fix, vì CSS inline style của Node-RED / App bida sẽ bị xung đột.
3. **Xử lý chuỗi `ui_tabs_links`**:
   - Khi chuyển đổi qua lại giữa chế độ Visual Form và Raw Textarea, phải đảm bảo parser INI chịu lỗi tốt, không làm mất các section tùy biến nếu số lượng khác 20.

---

## 4. Conclusion (Kết Luận & Đề Xuất Hành Động)

1. Phân hệ `/#redbida` đã có nền tảng hạ tầng vững chắc, sẵn sàng cho bước nâng cấp giao diện toàn diện R2.
2. Bản thiết kế kiến trúc chi tiết tại `.agents/explorer_survey_redbida/analysis.md` đã định nghĩa đầy đủ cấu trúc DOM, CSS Glassmorphism, 8 mẫu Gradient, logic Inspector 15 keys, 1-Click Auto-Fix, 20-Tab INI Editor và Smart Hashtag Generator.
3. Đề xuất chuyển giao kế hoạch cho Agent Coder / Frontend Specialist để tiến hành triển khai theo lộ trình 4 bước: DOM -> Logic JS -> Styling CSS -> E2E Testing.

---

## 5. Verification Method (Phương Pháp Kiểm Tra & Độc Lập Xác Minh)

1. **Kiểm tra Backend Go Unit Tests**:
   ```bash
   PATH=/home/ksp/.goroot/bin:$PATH go test ./...
   ```
2. **Kiểm tra Playwright UI Tests**:
   ```bash
   npx playwright test tests/ui/redbida.spec.js tests/ui/redbida_m3_challenger.spec.js
   ```
3. **Kiểm tra file báo cáo**:
   - Báo cáo chi tiết: `/home/ksp/ksp-camera-auto/.agents/explorer_survey_redbida/analysis.md`
   - Báo cáo bàn giao: `/home/ksp/ksp-camera-auto/.agents/explorer_survey_redbida/handoff.md`
