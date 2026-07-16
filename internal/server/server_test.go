package server

import (
	"testing"
	"time"
)

func TestLoginLimiterBlocksAfterMaxAttempts(t *testing.T) {
	l := newLoginLimiter(3, time.Hour)
	ip := "1.2.3.4"
	for i := 0; i < 2; i++ {
		l.fail1(ip)
		if l.blocked(ip) {
			t.Fatalf("blocked after %d failures, want not blocked yet (threshold 3)", i+1)
		}
	}
	l.fail1(ip)
	if !l.blocked(ip) {
		t.Fatal("expected blocked after reaching maxAttempts")
	}
}

func TestLoginLimiterUnblocksAfterLockoutExpires(t *testing.T) {
	l := newLoginLimiter(1, 10*time.Millisecond)
	ip := "5.6.7.8"
	l.fail1(ip)
	if !l.blocked(ip) {
		t.Fatal("expected blocked immediately after reaching maxAttempts=1")
	}
	time.Sleep(20 * time.Millisecond)
	if l.blocked(ip) {
		t.Fatal("expected unblocked after lockout window elapsed")
	}
	// blocked() should have opportunistically pruned the expired entry.
	l.mu.Lock()
	_, still := l.fail[ip]
	l.mu.Unlock()
	if still {
		t.Error("expected expired entry to be pruned from the map")
	}
}

func TestLoginLimiterResetClearsFailures(t *testing.T) {
	l := newLoginLimiter(1, time.Hour)
	ip := "9.9.9.9"
	l.fail1(ip)
	if !l.blocked(ip) {
		t.Fatal("expected blocked")
	}
	l.reset(ip)
	if l.blocked(ip) {
		t.Fatal("expected unblocked after reset")
	}
}

func TestNewLoginLimiterZeroValueFallsBackToDefaults(t *testing.T) {
	l := newLoginLimiter(0, 0)
	if l.maxAttempts != 5 {
		t.Errorf("maxAttempts = %d, want 5", l.maxAttempts)
	}
	if l.lockout != 30*time.Minute {
		t.Errorf("lockout = %v, want 30m", l.lockout)
	}
}

func TestLoginLimiterIndependentIPs(t *testing.T) {
	l := newLoginLimiter(1, time.Hour)
	l.fail1("1.1.1.1")
	if l.blocked("2.2.2.2") {
		t.Error("failure on one IP must not block a different IP")
	}
}
