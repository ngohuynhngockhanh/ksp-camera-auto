package camera

import (
	"errors"
	"testing"
)

func TestSafeFPSCapability(t *testing.T) {
	tests := []struct {
		name                string
		current, advertised int
		err                 error
		wantMax             int
		wantSource          string
	}{
		{"device capability", 20, 25, nil, 25, "capability"},
		{"capability below current", 25, 20, nil, 25, "fallback"},
		{"capability error minimum", 15, 0, errors.New("caps failed"), 20, "fallback"},
		{"capability error keeps current", 30, 0, errors.New("caps failed"), 30, "fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeFPSCapability(tt.current, tt.advertised, tt.err)
			if got.MaxFPS != tt.wantMax || got.Source != tt.wantSource {
				t.Fatalf("got %+v, want max=%d source=%s", got, tt.wantMax, tt.wantSource)
			}
		})
	}
}
