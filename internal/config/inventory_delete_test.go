package config

import (
	"path/filepath"
	"testing"
)

func TestInventoryDeleteManyDeduplicatesAndPersists(t *testing.T) {
	t.Setenv("KSPCAM_KEY", "bulk-delete-test-key")
	path := filepath.Join(t.TempDir(), "cameras.yaml")
	inv, err := LoadInventory(path)
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}
	for _, d := range []Device{
		{ID: "cam-1", Host: "192.168.1.1", Port: 37777, Vendor: VendorDahua, Password: "one"},
		{ID: "cam-2", Host: "192.168.1.2", Port: 37777, Vendor: VendorDahua, Password: "two"},
		{ID: "cam-3", Host: "192.168.1.3", Port: 80, Vendor: VendorHikvision, Password: "three"},
	} {
		if err := inv.Upsert(d); err != nil {
			t.Fatalf("seed %s: %v", d.ID, err)
		}
	}

	deleted, skipped, err := inv.DeleteMany([]string{"cam-1", "cam-1", "missing", " cam-2 "})
	if err != nil {
		t.Fatalf("delete many: %v", err)
	}
	if deleted != 2 || skipped != 1 {
		t.Fatalf("counts = deleted %d skipped %d, want 2/1", deleted, skipped)
	}
	if _, ok := inv.Get("cam-1"); ok {
		t.Error("cam-1 still exists")
	}
	if _, ok := inv.Get("cam-2"); ok {
		t.Error("cam-2 still exists")
	}
	if _, ok := inv.Get("cam-3"); !ok {
		t.Error("cam-3 was deleted unexpectedly")
	}

	reloaded, err := LoadInventory(path)
	if err != nil {
		t.Fatalf("reload inventory: %v", err)
	}
	if got := reloaded.List(); len(got) != 1 || got[0].ID != "cam-3" || got[0].Password != "three" {
		t.Fatalf("persisted inventory = %+v, want cam-3 with plaintext password after decrypt", got)
	}
}

func TestInventoryDeleteManyRejectsEmptyIDs(t *testing.T) {
	inv, err := LoadInventory(filepath.Join(t.TempDir(), "cameras.yaml"))
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}
	if _, _, err := inv.DeleteMany([]string{"", "  "}); err == nil {
		t.Fatal("expected empty IDs error")
	}
}
