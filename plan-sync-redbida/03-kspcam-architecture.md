# KSPCam Architecture

`internal/redbida` contains four seams:

- `types.go`: public metadata, value and broker contracts;
- `catalog.go`: key-directory discovery and group/risk classification;
- `mqtt.go`: Paho MQTT transport for local `ota-mqtt` topics;
- `service.go`: refresh/apply validation, redaction and acknowledgement mapping.

Server routes:

- `GET /api/redbida/catalog`
- `POST /api/redbida/refresh`
- `POST /api/redbida/apply`
- `GET /api/redbida/time-status`

All routes use the existing session auth and request-body cap. Viewer accounts
can inspect catalog/time status only; apply remains admin-only.
