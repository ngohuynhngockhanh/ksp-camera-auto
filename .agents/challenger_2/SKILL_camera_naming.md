# Camera & Shinobi Monitor Naming & Template Standard (Local Copy)

Source: `/home/ksp/ksp-camera-auto/.agents/skills/camera-naming/SKILL.md`

Summary: Golden Template rules for Shinobi monitors:
- mode: "record"
- stream_type: "hls"
- stream_vcodec: "copy"
- vcodec: "copy"
- record_vcodec: "copy"
- rtsp_transport: "tcp"
- preset_stream: "ultrafast"
- hls_time: "2", hls_list_size: "2"
- cust_input: ""
- cust_stream: ""
- cust_record: "-tag:v hvc1"
- watchdog_reset: "1", signal_check: "10"
