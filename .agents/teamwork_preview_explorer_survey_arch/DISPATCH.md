## 2026-08-23T15:57:00Z
You are an Explorer investigating the `ksp-camera-auto` codebase.
Your working directory is `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_arch/`.
Scope Document / Original Request: `/home/ksp/ksp-camera-auto/ORIGINAL_REQUEST.md`

Your task is a comprehensive investigation of the Architecture, Core Packages, Server, Web UI, Bulk Coordination, Camera Abstraction, and Configuration:
1. Read `/home/ksp/ksp-camera-auto/ORIGINAL_REQUEST.md`.
2. Inspect `main.go`, `internal/server/`, `internal/bulk/`, `internal/camera/`, `internal/config/`, `web/`, `cameras.yaml` and any related files.
3. Analyze and document in detail:
   - Full package map and dependencies.
   - Web UI & REST API: Endpoints, payload structures, SSE/WebSocket or polling, embedded frontend assets (Vue/React/Vanilla/Templates).
   - Sequential execution engine in `internal/bulk`: Job queue, mutexes/workers, error isolation, progress tracking, rate limiting.
   - Camera abstraction layer in `internal/camera`: Core interfaces, structs, lifecycle, Probe -> Apply -> Read-back state machine.
   - Configuration management in `internal/config`: YAML structure, schema, defaults, storage, validation.
   - Data flow diagrams (Mermaid format ready).

Write a comprehensive report to `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_arch/handoff.md` and send a message back with your findings.
