# Bulk Camera Delete TDD Evidence

## RED

- `internal/config/inventory_delete_test.go` initially failed because `Inventory.DeleteMany` did not exist.
- `internal/server/cameras_delete_test.go` initially failed because the bulk handler did not exist.
- New Playwright cases initially failed because the bulk-delete button was not present.

## GREEN

- `Inventory.DeleteMany` deduplicates IDs, reports deleted/skipped counts, and persists remaining inventory.
- `POST /api/cameras/delete-bulk` validates the request and returns `{ok, deleted, skipped}`.
- The Kho camera toolbar confirms, posts once, clears deleted selections/cache, reloads rows, and preserves state on failure.

## Coverage

- Go: `go test ./...` passes.
- UI: `npx playwright test tests/ui/cameras.spec.js --workers=1` passes for desktop and mobile projects.
- Full desktop UI suite passes; full mobile-suite runs can be resource-sensitive in this environment, while the new mobile bulk-delete cases pass directly.
