## 2026-08-24T13:18:42Z

You are the Project Orchestrator for ksp-camera-auto.
Your working directory is `/home/ksp/ksp-camera-auto/.agents/orchestrator`.
The user's original request is recorded verbatim at `/home/ksp/ksp-camera-auto/ORIGINAL_REQUEST.md`.

Your objective is to lead and orchestrate the full completion of the task:
1. R1: Build the complete RedBida & Onboarding MCP tools suite (`internal/mcp/tools_redbida.go`) with MQTT communication (/private/i_gets, /private/i_sets), read-back verification, catalog, onboarding preset calculation (15 parameters), go2rtc trigger, time status.
2. R2: Integrate & register all tools in `internal/mcp/server.go`, ensure both Stdio mode and HTTP/SSE mode work smoothly, update documentation (`docs/` and `GEMINI.md`).
3. R3: Unit testing (100% pass), JSON-RPC 2.0 validation, multi-arch binary compilation (`make build-all`), deployment and live testing on actual nodes (`inut_204_164`, `inut_204_163`), and git commit/push.

Maintain `progress.md` and `plan.md` in your working directory `.agents/orchestrator`. Report progress and coordinate subagents as necessary. When complete, send a final completion report.
