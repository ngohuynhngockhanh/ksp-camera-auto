# Scan credential discovery and inventory upsert TDD evidence

## User journeys

- A Dahua result discovered on `37777` keeps `37777`; a result discovered on
  `8888` keeps `8888` instead of being rewritten by a fallback.
- The scan action can try the supplied credentials and show the actual probe
  error when authentication fails.
- `Thêm vào kho` saves the discovered host, port, vendor, credentials, serial
  number, and QR metadata. If the host already exists, it updates that record
  (including a port change) instead of creating a second row.

## Evidence

| Guarantee | Test | Type | Result |
|---|---|---|---|
| A scanned camera is matched by host/IP when the port changes and its stable ID and non-scan metadata are preserved | `internal/server/cameras_upsert_test.go` | Integration | PASS |
| Blank credentials do not erase credentials already stored for the host | `internal/server/cameras_upsert_test.go` | Integration | PASS |
| Dahua OEM aliases, ONVIF identities, missing ports, and port `8888` normalize to the correct credential-test target | `internal/bulk/credtest_test.go`, `internal/discovery/discovery_test.go` | Unit | PASS |
| The scan page displays the normalized port, shows probe errors, stays on `#scan`, and POSTs the tested credentials to the inventory endpoint | `tests/ui/scan.spec.js` | E2E | PASS |

The implementation was developed red-first with the host-upsert regression in
`95472a1`, then fixed in `368d203`. The focused Go tests, full Go suite, and
Playwright suite pass. `node --check web/static/app.js` and the generated help
bundle check also pass.

## Coverage and gaps

- Full Go suite: PASS.
- Playwright: 81 passed, 11 skipped by project conditions.
- `docs-check` still reports the pre-existing missing help coverage for
  `/api/nvr/health`, `/api/nvr/health/check`, and `/api/nvr/watchdog`.
- Overall Go coverage remains below the repository's 80% target because the
  project contains broad hardware integrations; the changed scan/upsert paths
  are directly covered.
- Serial-number fetching is best-effort after a successful Dahua probe and was
  not exercised against a live camera in automated tests.
