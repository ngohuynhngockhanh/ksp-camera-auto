# Teamwork Autonomous Execution Draft

## Description
Viết tài liệu đánh giá chuyên sâu NVR Advantech / Raysharp / TVT cho dự án `ksp-camera-auto` và triển khai `kspcam` đồng thời chẩn đoán, xử lý triệt để lỗi không ghi hình trên 5 node: `inut_205_74`, `inut_205_97`, `inut_205_21`, `inut_205_22`, và `inut_205_76`.

## Working Directory
`/home/ksp/ksp-camera-auto`

## Status
In Progress — Phase 1: Drafting Technical Evaluation Document; Phase 2: Parallel 5-Node Health Audit & Root-Cause Diagnosis; Phase 3: Deployment & Recovery.

## Requirements Summary
- R1: Soạn tài liệu kỹ thuật NVR Advantech/Raysharp/TVT (`docs/nvr_evaluation_advantech_raysharp.md`).
- R2: Khảo sát & chẩn đoán nguyên nhân camera không ghi hình trên 5 node (`205_74`, `205_97`, `205_21`, `205_22`, `205_76`).
- R3: Deploy static binary `kspcam` và service systemd `:2028` lên cả 5 node.
- R4: Xử lý lỗi, kích hoạt ZRAM, Storage Guard và khôi phục ghi hình hoạt động 100%.
