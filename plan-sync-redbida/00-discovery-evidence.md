# Discovery Evidence

Verified on `inut_204_63` before implementation:

- Node-RED listens on `0.0.0.0:2023`; its dashboard root returns HTTP 200.
- Active Node-RED project is `ok2`.
- Node-RED flow nodes publish `/private/i_gets` and `/private/i_sets`, and
  listen for `/private/+/ack`.
- `ota-mqtt/index.js` owns `/root/ota-mqtt/change_ok/<key>` and implements
  `i_gets`/`i_sets` handlers.
- Local broker used by the Node-RED flow is `127.0.0.1:12369`.
- `i_gets` payload: `{ "info": ["key"] }`.
- `i_sets` payload: `{ "info": { "key": "value" } }`.

The implementation never edits Node-RED flows, calls an editor write API, or
restarts Node-RED. A later deploy verification must re-check these facts after
the target reconnects.
