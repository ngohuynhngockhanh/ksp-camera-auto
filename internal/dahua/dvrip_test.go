package dahua

import (
	"strings"
	"testing"
)

func TestParseRealm(t *testing.T) {
	// Real capture from a live device.
	body := "Realm:Login to 18038F6DBFE666A3\r\nRandom:166042717d\r\n\r\n"
	realm, random, err := parseRealm(body)
	if err != nil {
		t.Fatalf("parseRealm: %v", err)
	}
	if realm != "Login to 18038F6DBFE666A3" {
		t.Errorf("realm = %q", realm)
	}
	if random != "166042717d" {
		t.Errorf("random = %q", random)
	}
}

func TestParseRealmErrors(t *testing.T) {
	if _, _, err := parseRealm("garbage"); err == nil {
		t.Error("expected error on garbage input")
	}
}

func TestGen1HashShape(t *testing.T) {
	h := gen1Hash("testpass9")
	if len(h) != 8 {
		t.Fatalf("gen1 hash must be 8 chars, got %d (%q)", len(h), h)
	}
	const allowed = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	for _, r := range h {
		if !strings.ContainsRune(allowed, r) {
			t.Errorf("gen1 hash char %q outside [0-9A-Za-z]", r)
		}
	}
	// Deterministic.
	if gen1Hash("testpass9") != h {
		t.Error("gen1 hash not deterministic")
	}
}

func TestDvripLoginHashFormat(t *testing.T) {
	h := dvripLoginHash("admin", "testpass9", "Login to 18038F6DBFE666A3", "166042717d")
	parts := strings.SplitN(h, "&&", 2)
	if len(parts) != 2 || parts[0] != "admin" {
		t.Fatalf("login hash must start with 'admin&&', got %q", h)
	}
	// tail = 32-char gen2 hash + 32-char md5(gen1) = 64 uppercase hex chars.
	tail := parts[1]
	if len(tail) != 64 {
		t.Errorf("expected 64 hex chars after &&, got %d (%q)", len(tail), tail)
	}
	if tail != strings.ToUpper(tail) {
		t.Error("hash tail should be uppercase")
	}
}

func TestMd5Upper(t *testing.T) {
	// echo -n "abc" | md5sum -> 900150983cd24fb0d6963f7d28e17f72
	if got := md5Upper("abc"); got != "900150983CD24FB0D6963F7D28E17F72" {
		t.Errorf("md5Upper(abc) = %s", got)
	}
}

func TestFormatOfNavigation(t *testing.T) {
	table := []any{
		map[string]any{
			"MainFormat":  []any{map[string]any{"Video": map[string]any{"Width": float64(1920)}}},
			"ExtraFormat": []any{map[string]any{"Video": map[string]any{"Width": float64(640)}}},
		},
	}
	main, err := formatOf(table, 0, StreamMain)
	if err != nil {
		t.Fatalf("main: %v", err)
	}
	if toInt(main["Video"].(map[string]any)["Width"]) != 1920 {
		t.Error("wrong main format")
	}
	sub, err := formatOf(table, 0, StreamSub1)
	if err != nil {
		t.Fatalf("sub1: %v", err)
	}
	if toInt(sub["Video"].(map[string]any)["Width"]) != 640 {
		t.Error("wrong sub format")
	}
	if _, err := formatOf(table, 5, StreamMain); err == nil {
		t.Error("expected out-of-range channel error")
	}
}
