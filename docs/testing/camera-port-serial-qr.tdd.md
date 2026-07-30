# Camera port, serial number, and QR TDD evidence

## User journeys

- Camera operators see the verified Dahua port (`37777` or `8888`) without an authentication failure causing an incorrect fallback rewrite.
- Camera operators can probe a Dahua camera on `37777`, retain its serial number, and scan that serial from a QR code in the inventory.

## Evidence

| Guarantee | Test | Type | Result |
|---|---|---|---|
| Authentication failures on `37777` do not trigger or persist the `8888` fallback | `internal/camera/port_fallback_test.go` | Unit | PASS |
| Dahua serial response variants are parsed correctly | `internal/dahua/identity_test.go` | Unit | PASS |
| The inventory shows exact `37777`/`8888` values and renders a QR after probe | `tests/ui/cameras.spec.js` | E2E | PASS |

RED was captured with missing `shouldTryDahuaFallback` and `parseSerialNumber` implementations, plus a Playwright failure for the absent `SN / QR` cell. GREEN was verified with `go test ./...` and `npm run test:ui`.

## Coverage and gaps

- Full Go suite: PASS.
- Playwright: 73 passed, 11 skipped by project conditions.
- Package coverage remains below the ECC 80% target (`camera` 1.3%, `dahua` 19.0%) because these packages contain broad hardware integrations; the changed fallback/parser branches and UI journey are directly covered.
- Serial fetching is best-effort after a successful stream probe; no live camera credential was used in automated tests.
