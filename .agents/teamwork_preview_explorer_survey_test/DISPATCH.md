## 2026-08-23T15:57:00Z

You are an Explorer investigating the `ksp-camera-auto` codebase.
Your working directory is `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_test/`.
Scope Document / Original Request: `/home/ksp/ksp-camera-auto/ORIGINAL_REQUEST.md`

Your task is a comprehensive investigation of Build Systems, Test Suites, Tooling, Sample Scripts, and Test Harness/Simulator Strategy:
1. Read `/home/ksp/ksp-camera-auto/ORIGINAL_REQUEST.md`.
2. Inspect `Makefile`, `package.json`, `go.mod`, `go.sum`, all test files (`*_test.go`), Playwright E2E UI tests (`tests/` or `e2e/`), sample scripts (`chk_samples.js`, `chk_vnmap.js`, shell scripts), sample configs.
3. Analyze and document in detail:
   - Build system: Targets in `Makefile`, cross-compilation (`CGO_ENABLED=0` for linux/amd64, linux/arm, linux/arm64), Cgo build with HCNetSDK (`make build-hiksdk`, linking flags, shared library dependencies).
   - Test suites: Go unit/integration test commands, coverage commands, Playwright test setup/commands, sample scripts usage and test assertions.
   - Test Harness / Mock Simulator design: How to create mock Dahua DVRIP TCP server, Hikvision ISAPI HTTP server, and mock SDK stubs so developers and AI agents can test probe/apply/read-back and discovery logic safely without physical cameras.
   - Go coding conventions in this repo: formatting, error handling, logging, package structure, safety rules.
   - AI Agent Quickstart guidelines: Step-by-step workflow for new agents working on this repo.

Write a comprehensive report to `/home/ksp/ksp-camera-auto/.agents/teamwork_preview_explorer_survey_test/handoff.md` and send a message back with your findings.
