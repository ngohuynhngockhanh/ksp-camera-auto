package camera

import (
	"errors"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

func TestShouldTryDahuaFallbackOnlyWhenConfiguredPortIsUnreachable(t *testing.T) {
	if shouldTryDahuaFallback(37777, errors.New("login failed: authentication failed")) {
		t.Fatal("authentication failure on 37777 must not rewrite the camera to port 8888")
	}
	if !shouldTryDahuaFallback(37777, dahua.ErrDialUnreachable) {
		t.Fatal("unreachable 37777 should allow probing the known KBVision port 8888")
	}
	if shouldTryDahuaFallback(8888, dahua.ErrDialUnreachable) {
		t.Fatal("a camera already configured on 8888 must not fallback again")
	}
}
