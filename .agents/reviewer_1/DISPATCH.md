# Reviewer 1 Dispatch

## 2026-08-24T09:46:03Z

Objective:
Independently audit and verify the Deployment, Redbida, Node-RED, and Platform requirements on `inut_204_163`:
1. Check `kspcam.service` status on `inut_204_163:2028` (active running, no restart loops).
2. Check `/healthz` endpoint (200 OK `ok`).
3. Check `/api/redbida/catalog` and `/api/redbida/refresh` endpoints (return 200 OK with valid catalog and zero 500 errors).
4. Verify Venue Name is set to "CX King Luxury" in `/root/ota-mqtt/change_ok/` files (`company_name`, `logo_header_text`, `ui_title`) and synchronized via MQTT `/private/i_sets`.
5. Verify Virtual IP `192.168.1.254/24` is bound to `eth0` and responds to ping.
6. Verify Shinobi API Key and `shinobi_monitor_token` (with `0.0.0.0` IP restriction) are saved in `/root/ota-mqtt/change_ok/shinobi_monitor_token` and `ccio.API`.
