package dahua

import "testing"

func TestPasswordHash(t *testing.T) {
	// UPPER(MD5("admin:Login to X:pass")) — the Dahua gen2 stored-password
	// hash form modifyPassword expects. Locks the exact string so a refactor
	// can't silently change the wire format.
	got := passwordHash("admin", "Login to X", "pass")
	want := md5Upper("admin:Login to X:pass")
	if got != want {
		t.Fatalf("passwordHash = %s, want %s", got, want)
	}
	if len(got) != 32 {
		t.Errorf("hash length = %d, want 32 hex chars", len(got))
	}
}
