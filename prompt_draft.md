# Teamwork Autonomous Execution Draft

## Description
Điều tra nguyên nhân khách vào Wi-Fi không quét/truy cập được trên node `inut_204_141` (`77.88.204.141`), kiểm tra router gateway (pass: HWTCD2DC31B6, read-only), và tối ưu hóa hệ thống toàn diện (ZRAM 100%, PM2 Memory Guard, Smart Watchdog, Storage Guard, sysrq cleanup).

## Working Directory
`/home/ksp/ksp-camera-auto`

## Status
In Progress — Phase 1: Deep Network & Diagnostic Survey on inut_204_141; Phase 2: Root Cause Identification (Wi-Fi access & routing); Phase 3: System Hardening & Optimization; Phase 4: Final Verification.

## Requirements Summary
- R1: Khảo sát hiện trạng mạng, router gateway, IP alias, firewall, listening ports, và tìm lý do khách không quét được trên `204_141`.
- R2: Tối ưu hóa hệ thống toàn diện (ZRAM, PM2 Memory Guard, sysrq cleanup, Smart Watchdog, Storage Guard cron, kspcam update).
- R3: Khắc phục sự cố trên iNut và nghiệm thu hoạt động.
