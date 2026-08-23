## 2026-08-23T17:01:00Z

<USER_REQUEST>
You are teamwork_preview_worker implementing Milestone M4: Tests, Documentation, Multi-Arch Build & Remote Validation (Requirement R4).
Your working directory is: /home/ksp/ksp-camera-auto/.agents/worker_m4/
Read /home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md and /home/ksp/ksp-camera-auto/PROJECT.md before doing anything.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A reviewer/auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Scope & Tasks:
1. Documentation Updates:
   - Update `GEMINI.md` and `AGENTS.md`:
     - Document `internal/shinobi/` module (CRUD monitors, streaming states, video queries, manual trigger 2-way sync engine).
     - Document `internal/mcp/` embedded MCP server (JSON-RPC 2.0, Stdio `--mcp`, SSE `/mcp` on `:2028`, API key auth, 24 tool definitions).
     - Document Ansible automated provisioning in `playbook/roles/app_ksp_bida` on `172.16.5.180`.
     - Update REST route matrix and architecture diagrams if needed.
   - Run `make docs` (`go run ./tools/docgen -write`) and verify `make docs-check` passes 100%.

2. Multi-Arch Static Build & Full Test Suite:
   - Run `go test -count=1 -v ./...` and ensure 100% tests pass.
   - Run `make vet` / `go vet ./...`.
   - Run `make build-all` to build static binaries for `amd64`, `armv7`, `arm64` into `dist/`.

3. Live Remote Deployment & Validation on Target `inut_204_63`:
   - Copy or build arm/amd64 binary as needed or run Ansible deployment playbook from `172.16.5.180`:
     `ssh root@172.16.5.180 "ansible-playbook -i /build/armbian-build/ansible/inventories/linux /build/armbian-build/ansible/playbook/ksp-bida.yml -e 'target=inut_204_63'"`
   - Validate live on `inut_204_63`:
     a) Verify `/opt/ksp-cam/config.yaml` has `shinobi` and `mcp` sections.
     b) Query Shinobi monitors via `kspcam` or Shinobi API: `GET http://127.0.0.1:8080/<api_key>/monitor/<group_key>`.
     c) Execute live Stdio MCP request on `inut_204_63`:
        `echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | /opt/ksp-cam/kspcam --config /opt/ksp-cam/config.yaml --mcp`
     d) Test SSE MCP endpoint on `inut_204_63`:
        `curl -i -N -H "Accept: text/event-stream" http://127.0.0.1:2028/mcp`
     e) Verify `systemctl status kspcam` is active and healthy.

Write your handoff report to `/home/ksp/ksp-camera-auto/.agents/worker_m4/handoff.md` and notify parent when complete via send_message.
</USER_REQUEST>
