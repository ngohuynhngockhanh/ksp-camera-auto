package server

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSnapCacheReusesFreshEntry(t *testing.T) {
	c := newSnapCache()
	var calls int32
	fetch := func() ([]byte, error) { atomic.AddInt32(&calls, 1); return []byte("img"), nil }

	for i := 0; i < 3; i++ {
		data, err := c.get("k", fetch)
		if err != nil || string(data) != "img" {
			t.Fatalf("get: %q, %v", data, err)
		}
	}
	if calls != 1 {
		t.Errorf("fetch called %d times, want 1 (cache should reuse)", calls)
	}
}

func TestSnapCacheDoesNotCacheErrors(t *testing.T) {
	c := newSnapCache()
	var calls int32
	fetch := func() ([]byte, error) { atomic.AddInt32(&calls, 1); return nil, errors.New("boom") }
	for i := 0; i < 2; i++ {
		if _, err := c.get("k", fetch); err == nil {
			t.Fatal("expected error")
		}
	}
	if calls != 2 {
		t.Errorf("fetch called %d times, want 2 (errors must not be cached)", calls)
	}
}

// TestSnapCacheSingleFlight confirms a burst of concurrent requests for one
// key runs fetch exactly once (the per-key mutex serializes them and the
// first result is reused) — the behavior that stops a gallery from spawning
// N ffmpeg for the same channel.
func TestSnapCacheSingleFlight(t *testing.T) {
	c := newSnapCache()
	var calls int32
	fetch := func() ([]byte, error) { atomic.AddInt32(&calls, 1); return []byte("x"), nil }

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.get("same", fetch) }()
	}
	wg.Wait()
	if calls != 1 {
		t.Errorf("fetch called %d times for one key, want 1", calls)
	}
}

func TestSnapCacheDistinctKeys(t *testing.T) {
	c := newSnapCache()
	var calls int32
	fetch := func() ([]byte, error) { atomic.AddInt32(&calls, 1); return []byte("x"), nil }
	c.get("a", fetch)
	c.get("b", fetch)
	if calls != 2 {
		t.Errorf("fetch called %d times for 2 keys, want 2", calls)
	}
}
