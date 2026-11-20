package bidahub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteWALStore_MigrationsLedgerAndAutoMigrateFromLegacyJSON(t *testing.T) {
	tmpDir := t.TempDir()
	legacyJSON := filepath.Join(tmpDir, "hub-state.json")
	sqlitePath := filepath.Join(tmpDir, "hub.sqlite")

	// 1. Seed a legacy JSON file first
	seedClubs, seedBoxes := LoadOrSeedWatchlist("")
	if len(seedClubs) == 0 || len(seedBoxes) == 0 {
		t.Fatalf("expected non-empty seed clubs and boxes")
	}
	seedBoxes[0].CPUPct = 77.5
	seedBoxes[0].RTTMs = 42
	seedBoxes[0].Notes = "migrated-from-legacy-json"

	st := persistedState{
		Version:   1,
		UpdatedAt: time.Now().UTC(),
		Clubs:     seedClubs,
		Boxes:     seedBoxes,
	}
	raw, err := json.MarshalIndent(&st, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyJSON, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	// 2. Open Store pointing at hub.sqlite in the same directory; verify it auto-migrates hub-state.json
	srv1, err := NewServer(Config{
		DataPath:     sqlitePath,
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer(sqlitePath) failed: %v", err)
	}

	db := srv1.Store().SQLiteDB()
	if db == nil {
		t.Fatalf("expected non-nil SQLiteDB()")
	}

	// Verify WAL mode and schema_migrations ledger
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode failed: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected journal_mode=wal, got %q", journalMode)
	}

	var migCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations;").Scan(&migCount); err != nil {
		t.Fatalf("query schema_migrations failed: %v", err)
	}
	if migCount < 2 {
		t.Errorf("expected >= 2 migrations in schema_migrations, got %d", migCount)
	}

	// Verify addColumnIfMissing is idempotent
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := addColumnIfMissing(tx, "boxes", "telemetry_source", "ALTER TABLE boxes ADD COLUMN telemetry_source TEXT NOT NULL DEFAULT ''"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("addColumnIfMissing idempotent check failed: %v", err)
	}
	_ = tx.Commit()

	// Verify migrated box data from legacy JSON
	b0, ok := srv1.Store().GetBox(seedBoxes[0].ID)
	if !ok || b0.CPUPct != 77.5 || b0.RTTMs != 42 || b0.Notes != "migrated-from-legacy-json" {
		t.Fatalf("expected legacy JSON box state migrated to SQLite, got %+v", b0)
	}

	// 3. Remove legacy JSON file, mutate a box via UpdateBox, flush dirty telemetry to SQLite, and restart from SQLite alone
	_ = os.Remove(legacyJSON)
	if err := srv1.Store().UpdateBox(seedBoxes[0].ID, func(b *Box) error {
		b.CPUPct = 12.3
		b.RTTMs = 9
		b.TelemetrySource = "live"
		return nil
	}); err != nil {
		t.Fatalf("UpdateBox failed: %v", err)
	}
	srv1.Store().flushDirty()

	// Verify legacy JSON was NOT recreated when running in pure SQLite mode
	if _, err := os.Stat(legacyJSON); !os.IsNotExist(err) {
		t.Errorf("expected no JSON file written in pure SQLite mode, stat err=%v", err)
	}
	_ = srv1.Close()

	// 4. Reopen server from hub.sqlite and verify incremental row-level update persisted
	srv2, err := NewServer(Config{
		DataPath:     sqlitePath,
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer #2(sqlitePath) failed: %v", err)
	}
	defer srv2.Close()

	bAfter, ok := srv2.Store().GetBox(seedBoxes[0].ID)
	if !ok || bAfter.CPUPct != 12.3 || bAfter.RTTMs != 9 || bAfter.TelemetrySource != "live" {
		t.Fatalf("expected incremental SQLite WAL update to survive restart, got %+v", bAfter)
	}
}
