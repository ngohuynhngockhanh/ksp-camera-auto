package config

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Setenv("KSPCAM_KEY", "dGVzdC1rZXktMzItYnl0ZXMtZm9yLWFlcy1nY20xMg==") // 32-byte base64
	enc, err := Encrypt("secret-pass")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == "secret-pass" || len(enc) < 5 || enc[:4] != "enc:" {
		t.Fatalf("not encrypted: %q", enc)
	}
	dec, err := Decrypt(enc)
	if err != nil || dec != "secret-pass" {
		t.Fatalf("roundtrip failed: %q err=%v", dec, err)
	}
	// Legacy plaintext passes through.
	if got, _ := Decrypt("plainpw"); got != "plainpw" {
		t.Fatalf("legacy passthrough failed: %q", got)
	}
	// Empty stays empty.
	if got, _ := Encrypt(""); got != "" {
		t.Fatalf("empty should stay empty: %q", got)
	}
}
