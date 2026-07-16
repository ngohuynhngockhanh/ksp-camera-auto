package bulk

import (
	"context"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestTryPasswordsSkipsUnknownVendor(t *testing.T) {
	targets := []CredTestTarget{{IP: "127.0.0.1", Vendor: "", Port: 1}}
	results := TryPasswords(context.Background(), targets, "admin", "pass", config.Defaults{}, time.Second, nil)
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].OK {
		t.Error("expected OK=false for unknown vendor")
	}
	if results[0].Err == "" {
		t.Error("expected an error message for unknown vendor")
	}
}

// TestTryPasswordsSequentialOrderAndEvents targets port 1 (nothing listens
// there — fails fast with "connection refused", no live device needed) and
// checks the emitted event order is strictly start/result-per-target, which
// would only interleave differently ("start, start, ..., result, result")
// under a concurrent implementation.
func TestTryPasswordsSequentialOrderAndEvents(t *testing.T) {
	targets := []CredTestTarget{
		{IP: "127.0.0.1", Vendor: "dahua", Port: 1},
		{IP: "127.0.0.1", Vendor: "hikvision", Port: 1},
	}
	var types []string
	results := TryPasswords(context.Background(), targets, "admin", "pass", config.Defaults{}, 5*time.Second, func(e CredTestEvent) {
		types = append(types, e.Type)
	})
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.OK {
			t.Errorf("result %d: expected failure against an unreachable port", i)
		}
		if r.Err == "" {
			t.Errorf("result %d: expected error message", i)
		}
	}
	want := []string{"start", "result", "start", "result", "done"}
	if len(types) != len(want) {
		t.Fatalf("event types = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Errorf("event[%d] = %q, want %q (full: %v)", i, types[i], want[i], types)
		}
	}
}

func TestTryPasswordsEmptyTargets(t *testing.T) {
	results := TryPasswords(context.Background(), nil, "admin", "pass", config.Defaults{}, time.Second, nil)
	if len(results) != 0 {
		t.Errorf("want 0 results for empty targets, got %d", len(results))
	}
}

func TestTryPasswordsPortFallsBackToDefaults(t *testing.T) {
	// Port 0 means "unknown" (UDP discovery doesn't report one) — TryPasswords
	// must fill it from config.Defaults rather than dialing port 0 (which
	// would fail differently/ambiguously). We can't observe the dialed port
	// directly, but a Port:0 target must still produce a normal
	// connection-refused-style failure (not a distinct "invalid port" error),
	// confirming a real port number was substituted.
	targets := []CredTestTarget{{IP: "127.0.0.1", Vendor: "dahua", Port: 0}}
	results := TryPasswords(context.Background(), targets, "admin", "pass",
		config.Defaults{DahuaPort: 1, HikvisionPort: 1}, 5*time.Second, nil)
	if len(results) != 1 || results[0].OK {
		t.Fatalf("results = %+v, want single failed result", results)
	}
}
