# Adversarial Challenge Report: Milestone 2 — RedBida UI Overhaul (`/#redbida`)

> **Agent:** Empirical Challenger 1 M2 (`challenger_m2_1`)  
> **Target Recipient:** Orchestrator Parent (`d0a95b30-795a-486d-a88c-9c086b9f99b0`)  
> **Thời gian:** 2026-08-24T15:31:30Z  
> **Explicit Verdict:** `REQUEST_CHANGES` (Yêu cầu chỉnh sửa 3 lỗi logic đã xác minh thực nghiệm)

---

## 1. Observation (Quan Sát Trực Tiếp)

### A. Kiểm thử nền tảng (Baseline Tests)
- `PATH=/home/ksp/.goroot/bin:$PATH go test ./...`: 100% PASS (exit code 0).
- `npx playwright test tests/ui/redbida_m2_overhaul.spec.js`: 5/5 tests PASS (100%).

### B. Quan sát qua Adversarial Stress Testing (`tests/ui/redbida_m2_adversarial.spec.js`)

1. **Lỗi 1 (High): `ui_bg` Auto-Fix không xóa được nhiều dấu chấm phẩy liên tiếp (`;;;`) và không sửa được giá trị không phải gradient (như `#ffffff`, `red`)**
   - **Vị trí**: `web/static/redbida.js:234-239`
   - **Đoạn mã**:
     ```javascript
     fix: (cur) => {
       if (typeof cur === 'string' && cur.trim()) {
         return cur.replace(/;\s*$/, '').trim();
       }
       return 'linear-gradient(135deg, #0b192c 0%, #1e3e62 50%, #000000 100%)';
     }
     ```
   - **Thực nghiệm chứng minh**:
     ```bash
     node -e "
     const cur1 = 'linear-gradient(135deg, #0b192c, #000);;;  ';
     const fixed1 = cur1.replace(/;\s*$/, '').trim();
     console.log('fixed1:', fixed1, 'check:', fixed1.includes('gradient') && !/;\s*$/.test(fixed1.trim()));

     const cur2 = '#ffffff';
     const fixed2 = cur2.replace(/;\s*$/, '').trim();
     console.log('fixed2:', fixed2, 'check:', fixed2.includes('gradient') && !/;\s*$/.test(fixed2.trim()));
     "
     ```
   - **Kết quả trả về**:
     ```
     fixed1: linear-gradient(135deg, #0b192c, #000);; check: false
     fixed2: #ffffff check: false
     ```
   - **Hậu quả**: Khi người dùng nhấn "⚡ Sửa nhanh" trên `ui_bg` hoặc "⚡ 1-Click Sửa Tất Cả", giá trị trả về vẫn chứa dấu `;` hoặc vẫn là mã màu không hợp lệ, khiến hàm `check` tiếp tục trả về `false` (Lệch chuẩn), không thể đạt 100% Golden Standard.

2. **Lỗi 2 (Medium): `custom_hashtags` Golden Standard Check bỏ sót nguyên âm tiếng Việt viết hoa có dấu**
   - **Vị trí**: `web/static/redbida.js:248`
   - **Đoạn mã**:
     ```javascript
     const hasVN = /[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđĐ]/.test(val);
     ```
   - **Thực nghiệm chứng minh**:
     ```bash
     node -e "
     const val = '#QUÁNBIDA #BILLIARDSlive #INUTlive #highlightsports';
     const hasVN = /[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđĐ]/.test(val);
     console.log('val:', val, 'hasVN:', hasVN, 'check passed:', !hasVN && val.includes('#BILLIARDSlive') && val.includes('#INUTlive') && val.includes('#highlightsports'));
     "
     ```
   - **Kết quả trả về**:
     ```
     val: #QUÁNBIDA #BILLIARDSlive #INUTlive #highlightsports hasVN: false check passed: true
     ```
   - **Hậu quả**: Regex thiếu các ký tự nguyên âm viết hoa (`ÀÁẢÃẠ...`) và không có cờ `/i`. Chuỗi hashtag còn dấu tiếng Việt in hoa (như `#QUÁNBIDA`) bị đánh giá sai là "✅ Đạt chuẩn".

3. **Lỗi 3 (Medium): `company_name` Check chấp nhận tên công ty rác khi `ui_title` chưa có hoặc rỗng**
   - **Vị trí**: `web/static/redbida.js:222`
   - **Đoạn mã**:
     ```javascript
     check: (val) => {
       const title = getEffectiveValue('ui_title');
       return typeof val === 'string' && val.trim().length > 0 && (!title || val === title);
     }
     ```
   - **Thực nghiệm chứng minh**: Khi `ui_title` là `""` (chưa cấu hình hoặc rỗng), điều kiện `!title` trở thành `true`, khiến bất kỳ chuỗi `company_name` nào (ví dụ `"Wrong Company"`) cũng được coi là "✅ Đạt chuẩn" dù thực tế lệch hoàn toàn với tên quán.

---

## 2. Logic Chain (Chuỗi Lập Luận Kỹ Thuật)

1. Từ **Quan sát 1**: Regex `replace(/;\s*$/, '')` chỉ xóa duy nhất 1 ký tự `;` ở cuối. Nếu chuỗi có từ 2 dấu `;` trở lên hoặc có ký tự màu hex thuần (`#fff`), hàm `fix` giữ nguyên chuỗi lỗi. Sau khi `fix` xong, hàm `check` đối chiếu lại thấy chuỗi vẫn vi phạm tiêu chí gradient hợp lệ và không có dấu chấm phẩy, dẫn tới việc nút "⚡ Sửa nhanh" không sửa triệt để được lỗi.
2. Từ **Quan sát 2**: Tiêu chuẩn R2 và Skill `camera-naming` yêu cầu `custom_hashtags` phải sạch dấu tiếng Việt chuẩn hóa. Việc biểu thức chính quy chỉ liệt kê ký tự thường khiến các chuỗi in hoa có dấu lọt qua bộ kiểm duyệt Golden Standard.
3. Từ **Quan sát 3**: `company_name` theo quy chuẩn bắt buộc phải đồng nhất với `ui_title`. Việc cho phép `!title` làm điều kiện thoát (escape hatch) tạo ra lỗ hổng false-positive trong bảng Checklist Inspector.
4. **Các thành phần hoạt động xuất sắc (Robustness Confirmed)**:
   - Bộ 8 Palette Gradient (`REDBIDA_GRADIENT_PALETTE`): 100% không có trailing semicolon `;`, chuyển đổi class `.active` chuẩn xác, Live Canvas Preview phản hồi mượt mà.
   - Smart Hashtag Generator (`generateSmartHashtags`): Xử lý hoàn hảo mọi chuỗi tiếng Việt phức tạp (NFC/NFD, compound diacritics, ký tự đặc biệt, rỗng/undefined).
   - Visual 20-Tab INI Editor: Parse và serialize 2 chiều 20 section `[C01]`..`[C20]` ổn định, đồng bộ tên quán 1-click chính xác.

---

## 3. Caveats (Lưu Ý & Giới Hạn)

- Không có caveat nào đối với môi trường backend Go (Go tests 100% pass).
- Các lỗi phát hiện thuần túy nằm ở logic kiểm tra và auto-fix của client-side JS (`web/static/redbida.js`).

---

## 4. Conclusion & Actionable Recommendations (Kết Luận & Yêu Cầu Chỉnh Sửa)

### Explicit Verdict: `REQUEST_CHANGES`

Để Milestone 2 đạt độ hoàn hảo tuyệt đối, yêu cầu Implementation Worker M2 thực hiện 3 chỉnh sửa sau trong `web/static/redbida.js`:

1. **Sửa `ui_bg.fix`** (Line 234-239):
   ```javascript
   // THAY THẾ:
   fix: (cur) => {
     if (typeof cur === 'string' && cur.includes('gradient')) {
       return cur.replace(/[;\s]+$/, '').trim();
     }
     return 'linear-gradient(135deg, #0b192c 0%, #1e3e62 50%, #000000 100%)';
   },
   ```

2. **Sửa `custom_hashtags.check`** (Line 248):
   ```javascript
   // THAY THẾ:
   const hasVN = /[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]/i.test(val);
   ```

3. **Sửa `company_name.check`** (Line 222):
   ```javascript
   // THAY THẾ:
   check: (val) => {
     const title = getEffectiveValue('ui_title');
     return typeof val === 'string' && val.trim().length > 0 && typeof title === 'string' && title.trim().length > 0 && val === title;
   },
   ```

---

## 5. Verification Method (Phương Pháp Kiểm Tra Lại)

Sau khi Worker sửa xong, chạy các lệnh kiểm tra sau để nghiệm thu:

1. **Chạy Adversarial Stress Suite**:
   ```bash
   npx playwright test tests/ui/redbida_m2_adversarial.spec.js
   ```
   *(Kỳ vọng: 4/4 passed 100%)*

2. **Chạy Toàn Bộ Test Suite RedBida**:
   ```bash
   npx playwright test tests/ui/redbida_m2_overhaul.spec.js tests/ui/redbida_m3_challenger.spec.js tests/ui/redbida.spec.js tests/ui/redbida_m2_adversarial.spec.js
   ```
   *(Kỳ vọng: 20/20 passed 100%)*

3. **Chạy Go Unit Tests**:
   ```bash
   PATH=/home/ksp/.goroot/bin:$PATH go test ./...
   ```
   *(Kỳ vọng: 100% PASS)*
