## 2026-08-23T15:57:00Z
You are an Explorer investigating the `ksp-camera-auto` codebase.
Your working directory is `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_proto/`.
Scope Document / Original Request: `/home/ksp/ksp-camera-auto/ORIGINAL_REQUEST.md`

Your task is a deep technical investigation of all Camera Protocols and Discovery Mechanisms:
1. Read `/home/ksp/ksp-camera-auto/ORIGINAL_REQUEST.md`.
2. Inspect `internal/dahua/`, `internal/isapi/`, `internal/hik/`, `internal/hiksdk/`, `internal/discovery/`, and any related protocol implementations.
3. Analyze and document in detail:
   - Dahua DVRIP (port 37777): Binary packet format (headers, magic bytes, length, sequence IDs), two-step MD5 login hash challenge/response algorithm, JSON-RPC configManager (Encode get/set methods, payload structure, response parsing), keep-alive/heartbeat packets, timeout & reconnection handling.
   - Hikvision ISAPI (port 80): HTTP Digest Auth implementation, exact XML endpoints and payloads for `StreamingChannel` (Video resolution, framerate, bitrate, H.264/H.265/H.265+ Smart Codec, Audio AAC/G.711), response status parsing.
   - Hikvision HCNetSDK (port 8000 Cgo/NAT): Cgo bindings in `internal/hiksdk`, dynamic library loading (`libhcnetsdk.so` / headers), `NET_DVR_Login_V40`, `NET_DVR_STDXMLConfig` XML transmission, fallback logic, thread safety and resource cleanup.
   - Discovery mechanisms in `internal/discovery`: ONVIF WS-Discovery (UDP multicast 239.255.255.250:3702), Dahua UDP broadcast (port 37810), Hikvision SADP (UDP broadcast 239.255.255.250:37020), nmap subnet scanning integration.
   - Concurrency & Safety rules: Why and how sequential execution is enforced per camera.

Write a comprehensive report to `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_proto/handoff.md` and send a message back with your findings.
