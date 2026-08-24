# Camera & Shinobi Monitor Naming & Template Standard
(Local dump for worker_camera_m1)
Source: /home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md

1. Naming: Name `CameraXX` (e.g. `Camera01`), Shinobi Monitor ID `cameraXX` (e.g. `camera01`), Inventory ID `<ip>:<port>`.
2. Golden Template standard from Camera01:
   - Stream & Codec: mode `record`, stream_type `hls`, stream_vcodec `copy`, vcodec `copy`, record_vcodec `copy`, cutoff `5` (5-min segment), rtsp_transport `tcp`, preset_stream `ultrafast`, hls_time `2`, hls_list_size `2`.
   - Audio Codec probe & conversion: Probe audio, convert to AAC if non-AAC. If AAC supported: acodec `copy`, stream_acodec `copy`, record_acodec `aac`. If not: acodec `no`, stream_acodec `no`, record_acodec `no`.
   - FFmpeg flags: cust_input `""`, cust_stream `""`, cust_record `"-tag:v hvc1"` for H.265.
   - Watchdog: watchdog_reset `"1"`, signal_check `"10"`.
