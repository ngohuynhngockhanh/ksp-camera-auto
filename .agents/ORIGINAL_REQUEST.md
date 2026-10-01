# Original User Request

## Request — 2026-10-01
User request:
"bạn xem thử khách bảo camera ở 5_122 5_123 5_124 nó bị mơ cách nào tăng lên 4mp giúp minh rôi để 1536 vbr h265 được ko nó đỡ hơn hiện tại bị mơ á hihi smart code cho nó nén"

Working directory: `/home/ksp/ksp-camera-auto`

Target Nodes:
- `inut_205_122` (`77.88.205.122`) — Magic Arena Đà Nẵng (Ingests NVR Ch 301..3001)
- `inut_205_123` (`77.88.205.123`) — Magic Arena Đà Nẵng (Ingests NVR Ch 3101..4601)
- `inut_205_124` (`77.88.205.124`) — Magic Arena Đà Nẵng (Ingests NVR Ch 4701..6201)
- Central NVR: Hikvision `DS-7764NI-M4` at `192.168.1.50:82` (RTSP: `1554`)
- Physical Cameras: 64x Hikvision `DS-2CD1043G0-IUF` 4MP (2560x1440)

## Requirements

### R1. Khảo sát hiện trạng độ phân giải và thông số luồng trên 3 node 122, 123, 124 [HOÀN THÀNH]
- Kiểm tra toàn bộ 64 kênh trên NVR Hikvision `192.168.1.50:82`:
  - Độ phân giải: Toàn bộ 64 kênh vật lý đã đạt phần cứng 4MP (`2560x1440`) và chuẩn nén `H.265`.
  - Bitrate & SmartCodec:
    - Kênh 301..1801 (trên node 122): 1536 Kbps VBR, SmartCodec = true.
    - Kênh 1901..5801 (trên node 122, 123, 124): 2048 Kbps VBR, SmartCodec = true.
    - Kênh 5901..6401 (trên node 124): 3072 Kbps VBR, SmartCodec = false.

### R2. Phân tích nguyên nhân video bị mờ theo phản ánh của khách [HOÀN THÀNH]
1. Camera đã ở độ phân giải cảm biến tối đa 4MP (2560x1440), không thể tăng pixel phần cứng cao hơn nữa.
2. SmartCodec (H.265+) nén quá mức khi cảnh tĩnh:
   - Với SmartCodec bật và VBR Upper Cap 1536 Kbps, camera hạ bitrate trung bình xuống chỉ còn ~400–800 Kbps. Đối với khung hình 2560x1440 (3.68 triệu pixel), bitrate này chỉ đạt ~0.008 bit/pixel khiến các chuyển động bi bida, cơ bida hoặc mặt bàn nỉ bị bệt hình, mờ nhòe (motion blur / macroblock compression artifacts).
3. Người xem có thể đang xem luồng phụ (Sub-stream 640x360) qua WebRTC / HLS trên điện thoại nếu đường truyền yếu.
4. Mức độ sắc nét (Sharpness) trên NVR đang để mặc định 50/100.

### R3. Chuẩn hóa đồng bộ toàn bộ các kênh camera về Profile theo yêu cầu [HOÀN THÀNH]
- Áp dụng chuẩn hóa cho toàn bộ các kênh thuộc 3 node 122, 123, 124:
  - Resolution: `2560x1440` (4MP)
  - Video Codec: `H.265`
  - Bitrate: `1536 Kbps VBR` (`vbrUpperCap = 1536`, `vbrLowerCap = 32`)
  - SmartCodec: `true` (H.265+)
  - Đọc lại xác nhận (Read-back verification) 100% các kênh sau khi ghi.

### R4. Nghiệm thu & Kiểm tra ghi hình trên cả 3 node [HOÀN THÀNH]
- Kiểm tra các file video ghi hình mới nhất trên cả 3 node `205_122`, `205_123`, `205_124`.
- Báo cáo kết quả chi tiết, rõ ràng kèm khuyến nghị kỹ thuật về SmartCodec để quán có chất lượng hình ảnh tốt nhất.

## Acceptance Criteria
- [x] C1: Khảo sát chi tiết hiện trạng 64 kênh NVR trên `192.168.1.50:82`.
- [x] C2: Phân tích kỹ thuật chính xác nguyên nhân gây mờ hình cho khách.
- [x] C3: Cấu hình chuẩn hóa toàn bộ các kênh theo yêu cầu (4MP, 1536 VBR, H.265, SmartCodec true).
- [x] C4: Đọc lại đối chiếu (read-back) thành công và xác nhận các file ghi hình mới sinh đều đặn trên cả 3 node.
