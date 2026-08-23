## 2026-08-23T16:29:54Z

Investigate the design and implementation of the Embedded MCP Server in `kspcam` (Requirement R3):
1. MCP Protocol Specification (Model Context Protocol / JSON-RPC 2.0):
   - Message structures: requests, responses, notifications (`initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`).
   - Transport mechanisms:
     a) Stdio mode via `kspcam --mcp`: reads JSON lines from stdin, writes JSON lines to stdout, redirects application logging to stderr or file.
     b) HTTP / SSE endpoint `/mcp` on web server `:2028`: SSE stream for server->client messages, POST `/mcp/messages` or query param session for client->server messages, with API Key authentication (e.g. `X-MCP-Key` header or `?key=` query param, configured in `config.yaml`).
2. Tools Definition & Schema:
   Enumerate the exact tool names, JSON input schemas, and execution logic for all 4 tool groups:
   - Camera Inventory Tools
   - Camera Config & Bulk Tools
   - Discovery & Diagnosis Tools
   - Shinobi Management Tools
3. Package Architecture:
   - `internal/mcp/`: Server, protocol handler, tool registry, stdio transport, SSE transport.
   - Integration with `cmd/kspcam/main.go` and `internal/server/`.
