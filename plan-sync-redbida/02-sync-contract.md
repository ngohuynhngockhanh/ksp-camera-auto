# Sync Contract

The Go service serializes one MQTT request at a time because the legacy ack
topics do not contain a correlation id.

Read:

```json
{ "info": ["logo_header", "show_toolbar"] }
```

Ack:

```json
{ "info": { "logo_header": "...", "show_toolbar": true } }
```

Write:

```json
{ "info": { "logo_header": "new-value" } }
```

Ack:

```json
{ "info": { "logo_header": { "oldValue": "...", "newValue": "..." } } }
```

The service validates risk, batch size, schema type, value size and logo data
URLs before publishing. Retained/unrelated acknowledgements are ignored. A
write is successful only after a second `i_gets` read-back matches the requested
value; an acknowledgement timeout triggers read-back instead of a blind retry.
