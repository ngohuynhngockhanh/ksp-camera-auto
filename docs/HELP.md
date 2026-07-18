# Hệ thống Trợ giúp trong app (/help)

Tab **Trợ giúp** trong web UI: tìm kiếm (gõ không dấu vẫn khớp) + chatbox gợi ý
tính năng. Toàn bộ nội dung sinh từ markdown, nhúng vào binary — không gọi API
ngoài, không LLM, chạy offline.

## Sơ đồ

```
docs/help/*.md  ──(make docs)──►  web/static/help/help-index.json  ──go:embed──►  binary
     │                                        │
  STYLE.md (quy chuẩn viết)            web/static/help.js (search + chatbox + render)
  _template.md (khung bài mới)         tab "Trợ giúp" trong index.html
  coverage-ignore.txt (bỏ qua drift)
```

- **`tools/docgen/`** — generator Go thuần (chỉ dùng yaml.v3 sẵn có). Render
  Markdown → HTML ngay lúc gen (escape hết, cấm HTML thô), fail sớm khi sai
  cú pháp. `main.go` (đọc bài + validate + ghi JSON), `markdown.go` (subset
  markdown), `check.go` (drift check).
- **`web/static/help.js`** — toàn bộ logic UI trợ giúp, tách riêng khỏi
  `app.js` (app.js chỉ thêm mục nav + fix router cho deep-link `#help/<id>`).
- **`web/static/help/help-index.json`** — file sinh ra, **commit vào repo**
  (build không phụ thuộc docgen).

## Lệnh

```bash
make docs        # sinh lại bundle từ docs/help/*.md
make docs-check  # fail nếu có route/tab chưa được bài nào phủ (dùng trong CI/trước commit)
```

Máy nào `go` không nằm trong PATH thì thêm `GO=/path/to/go`, ví dụ trên máy
dev hiện tại: `make docs GO=/home/ksp/.goroot/bin/go`.

## Thêm / sửa bài trợ giúp

1. Copy `docs/help/_template.md` → `docs/help/<id-moi>.md` (id = tên file,
   kebab-case không dấu).
2. Viết theo `docs/help/STYLE.md` — quan trọng nhất là **frontmatter**:
   - `keywords:` 8–15 từ khóa (quyết định search + chatbox tìm ra bài).
   - `ui:` hash tab chứa tính năng (nút "Mở tính năng").
   - `covers:` các route API bài này bao phủ (đầu vào của drift check).
3. `make docs` — mọi lỗi (id trùng, link `#help/…` gãy, markdown sai,
   `related` trỏ bài không tồn tại) đều fail tại đây, không hỏng âm thầm
   trên trình duyệt.
4. Commit cả file `.md` lẫn `help-index.json` mới sinh.

Khi thêm **route API mới** vào `internal/server/server.go` hoặc **tab mới**
vào `NAV_ITEMS` (app.js): `make docs-check` sẽ fail cho tới khi có bài nhận
phủ nó (qua `covers:`/`ui:`) hoặc thêm nó vào `docs/help/coverage-ignore.txt`
(comment bằng `//`).

## Viết nội dung bằng agent (cách đã dùng để tạo 20 bài đầu)

Giao mỗi cụm bài cho một agent (Opus cho phần nặng protocol/rủi ro, Sonnet cho
phần còn lại), chạy song song, mỗi agent sở hữu file riêng. Prompt cần đưa:

- Đường dẫn `STYLE.md` + 1–2 bài mẫu làm chuẩn chất lượng.
- Frontmatter cố định từng bài (id/section/order/ui/covers) — agent tự viết
  title/keywords/body.
- Danh sách file nguồn phải đọc (index.html, handler trong `api.go`,
  `docs/GOTCHAS.md`…) và yêu cầu **đối chiếu nhãn nút thật** trước khi viết.
- Danh sách toàn bộ id hợp lệ để `related:`/link `#help/…` không gãy.
- Cấm chạy generator/build (chạy tập trung một lần sau khi đủ bài).

## Ghi chú kỹ thuật

- Search/chatbox chuẩn hóa tiếng Việt phía JS: NFD bỏ dấu + replace riêng
  `đ→d` (NFD không tách được `đ`); lọc stopword ("tôi", "muốn"…); khớp theo
  từ (token 2 ký tự phải trùng nguyên từ) + cộng điểm lớn khi cả cụm keyword
  xuất hiện trong câu hỏi.
- Bài viết render bằng `innerHTML` nhưng an toàn vì HTML do docgen sinh và
  escape toàn bộ text; markdown chứa HTML thô sẽ bị chặn từ lúc gen.
- QA tự động: script Playwright mẫu nằm trong phiên làm việc trước (đăng
  nhập → search có dấu/không dấu → chat → deep-link → mobile 390px); có thể
  tái tạo nhanh từ mô tả này khi cần.
