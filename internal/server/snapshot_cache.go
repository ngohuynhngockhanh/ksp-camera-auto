package server

import (
	"sync"
	"time"
)

// snapCacheTTL is how long a fetched JPEG is reused before a fresh grab. The
// gallery re-opens and reloads hit the same channel repeatedly; serving a
// few-seconds-old frame there avoids re-spawning ffmpeg for a view that's
// visually identical anyway. Short enough that a snapshot still feels live.
const snapCacheTTL = 4 * time.Second

// snapCache is a tiny TTL cache + per-key single-flight for camera snapshots.
// Two things it prevents on a low-RAM box: (1) repeated views of the same
// channel re-decoding a frame every time (TTL reuse), and (2) several
// concurrent requests for the same channel each spawning their own ffmpeg
// (single-flight — the first fetches, the rest wait and share the result).
type snapCache struct {
	mu      sync.Mutex
	entries map[string]*snapEntry
}

type snapEntry struct {
	mu   sync.Mutex // held while fetching, so concurrent callers for this key serialize
	data []byte
	exp  time.Time
}

func newSnapCache() *snapCache {
	return &snapCache{entries: map[string]*snapEntry{}}
}

// get returns a cached JPEG for key if still fresh, else calls fetch (with
// this key's fetches serialized so a burst for one channel runs ffmpeg once)
// and caches the result. A fetch error is returned as-is and not cached.
func (c *snapCache) get(key string, fetch func() ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	e := c.entries[key]
	if e == nil {
		e = &snapEntry{}
		c.entries[key] = e
	}
	c.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.data != nil && time.Now().Before(e.exp) {
		return e.data, nil
	}
	data, err := fetch()
	if err != nil {
		return nil, err
	}
	e.data = data
	e.exp = time.Now().Add(snapCacheTTL)
	return data, nil
}
