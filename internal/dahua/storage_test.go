package dahua

import (
	"encoding/json"
	"testing"
)

func TestByteCountAcceptsFirmwareFloat(t *testing.T) {
	var got byteCount
	if err := json.Unmarshal([]byte(`120573657088.0`), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != 120573657088 {
		t.Fatalf("got %d", got)
	}
}

func TestByteCountRejectsFraction(t *testing.T) {
	var got byteCount
	if err := json.Unmarshal([]byte(`12.5`), &got); err == nil {
		t.Fatal("expected fractional byte count to fail")
	}
}
