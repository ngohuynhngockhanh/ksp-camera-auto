# Teamwork Autonomous Execution Draft

## Description
Điều tra và khắc phục triệt để tình trạng cạn RAM (OOM / Out of Memory) trên node `inut_204_157` (`77.88.204.157` — Kely Pickleball).

## Working Directory
`/home/ksp/ksp-camera-auto`

## Status
Completed — inut_204_157 memory root cause identified (MEM_LIMIT_PERCENTAGE=50 clamping ZRAM swap writes + no heap caps on Node.js with 12 camera threads + missing watchdog/storage guard) and fully resolved: ZRAM Swap expanded to 3.5GB without mem_limit, PM2 max-memory-restart limits configured, Smart Watchdog and Storage Guard cron active, available RAM increased to 950MB, and all 12 cameras running 24 healthy FFmpeg processes.

## Requirements Summary
- R1: Khảo sát hiện trạng RAM, dmesg OOM killer, và các tiến trình trên `204_157` -> [DONE]
- R2: Khắc phục điểm nghẽn ZRAM mem_limit và giới hạn heap V8 -> [DONE]
- R3: Cài đặt Smart Watchdog, Storage Guard cron, và chuẩn hóa Shinobi -> [DONE]
- R4: Nghiệm thu toàn diện hoạt động của 12 camera và dung lượng RAM khả dụng -> [DONE]
