# Quy chuẩn viết bài trợ giúp (docs/help)

Bài trợ giúp là nội dung **tiếng Việt cho người vận hành camera** — không phải
docs kỹ thuật. Người đọc là kỹ thuật viên lắp đặt/quản lý camera, không đọc code.

Sinh bundle: `make docs` (ghi `web/static/help/help-index.json`, commit file này).
Kiểm phủ tính năng: `make docs-check` (fail nếu route/tab mới chưa có bài).

## Frontmatter (bắt buộc)

```yaml
---
id: chinh-hang-loat            # = tên file (kebab-case, không dấu)
title: "Chỉnh thông số hàng loạt"
section: bulk                  # start|scan|cameras|bulk|dahua|import|admin
order: 10                      # thứ tự trong section (10, 20, 30…)
keywords: [codec, H.265, độ phân giải, bitrate]   # 8–15 từ khóa
ui: "#cameras"                 # tab chứa tính năng ("" nếu không có)
covers: ["/api/apply"]         # route API mà bài này bao phủ
related: [doc-cau-hinh]        # id các bài liên quan (phải tồn tại)
---
```

## Từ khóa (quan trọng — quyết định search & chatbox)

- Viết **có dấu đầy đủ** (hệ thống tự khớp khi người dùng gõ không dấu).
- Bao gồm cả **từ thông tục / từ đồng nghĩa** người dùng hay gõ: "pass",
  "mật khẩu", "reset", "đổi tên", "wifi", "màn hình", "mờ", "giật"…
- Nghĩ theo hướng: *người dùng sẽ gõ gì vào ô chat khi cần tính năng này?*

## Cấu trúc thân bài

Theo đúng thứ tự (bỏ mục không áp dụng, không thêm mục lạ):

```
## Mục đích
## Cách dùng
## Lưu ý
## Sự cố thường gặp
```

- Xưng hô: gọi người đọc là **"bạn"**, giọng hướng dẫn thân thiện, câu ngắn.
- "Cách dùng" viết theo bước đánh số `1.` `2.`, mỗi bước một hành động,
  nêu đúng nhãn nút/ô trên giao diện (đặt trong **đậm**, ví dụ: bấm **Áp dụng**).
- "Sự cố thường gặp" viết dạng danh sách `-`: *hiện tượng → cách xử lý*.

## Markdown cho phép (sai cú pháp là `make docs` báo lỗi)

- `##` và `###` (không dùng `#` — tiêu đề bài lấy từ frontmatter)
- Đoạn văn, danh sách `- ` và `1. `, trích dẫn `> ` (dùng cho cảnh báo:
  `> **Lưu ý:** …`), bảng `|…|`, code fence ``` ```
- `**đậm**`, `` `mã` ``, link `[chữ](đích)` — đích chỉ được là `#help/<id>`
  (bài khác), `#cameras`/`#scan`… (mở tab), hoặc `http(s)://`
- **Cấm HTML thô.**

## Chất lượng

- Mô tả đúng những gì giao diện đang có — khi nghi ngờ, đọc `web/static/index.html`
  và handler tương ứng trong `internal/server/api.go` trước khi viết.
- Tính năng chỉ dành cho Dahua/KBVision phải ghi rõ ngay đầu bài.
- Thao tác rủi ro (đổi mật khẩu, đổi IP, áp dụng encode làm rớt stream) phải có
  `> **Lưu ý:**` cảnh báo hậu quả và cách khôi phục.
