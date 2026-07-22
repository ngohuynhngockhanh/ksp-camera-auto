package dahua

import "testing"

func TestMaxFPSValue(t *testing.T) {
	capability := map[string]any{
		"Video": map[string]any{
			"FPS":     []any{float64(5), float64(20), "25,30"},
			"BitRate": []any{float64(1024), float64(4096)},
		},
	}
	if got := maxFPSValue(capability, false); got != 30 {
		t.Fatalf("maxFPSValue = %d, want 30", got)
	}
}

func TestMaxFPSValueIgnoresUnrelatedNumbers(t *testing.T) {
	capability := map[string]any{"Width": float64(3840), "Height": float64(2160)}
	if got := maxFPSValue(capability, false); got != 0 {
		t.Fatalf("maxFPSValue = %d, want 0", got)
	}
}
