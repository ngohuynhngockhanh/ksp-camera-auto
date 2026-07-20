package tiandy

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPlaybackRTSPURL(t *testing.T) {
	start := time.Date(2026, 7, 20, 16, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 20, 16, 5, 30, 0, time.UTC)
	// tiandyChannel(0) == 1 (neutral 0-based -> native 1-based).
	got := playbackRTSPURL("nvr.example", "admin", "p@ss/w", tiandyChannel(0), start, end)
	for _, want := range []string{
		"rtsp://",
		"@nvr.example:554/cam/playback",
		"channel=1",
		"starttime=2026_07_20_16_00_00",
		"endtime=2026_07_20_16_05_30",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("url %q missing %q", got, want)
		}
	}
	// Password special chars must be percent-encoded in the userinfo.
	if strings.Contains(got, "p@ss/w@") {
		t.Errorf("password not encoded in %q", got)
	}
}

func TestPrefixToMask(t *testing.T) {
	cases := map[int]string{0: "0.0.0.0", 8: "255.0.0.0", 24: "255.255.255.0", 32: "255.255.255.255", -1: "", 33: ""}
	for prefix, want := range cases {
		if got := prefixToMask(prefix); got != want {
			t.Errorf("prefixToMask(%d) = %q, want %q", prefix, got, want)
		}
	}
}

func TestFindRecordingsSynthetic(t *testing.T) {
	c := New("h", "u", "p", time.Second)

	// A fully-past window yields exactly one segment spanning it.
	start := time.Now().Add(-2 * time.Hour)
	end := time.Now().Add(-1 * time.Hour)
	recs, err := c.FindRecordings(3, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 recording, got %d", len(recs))
	}
	if recs[0].Channel != 3 || recs[0].Type != "mp4" {
		t.Errorf("unexpected recording: %+v", recs[0])
	}
	if recs[0].Duration != 3600 {
		t.Errorf("duration = %d, want 3600", recs[0].Duration)
	}

	// The window is echoed verbatim (no now-clamp — see FindRecordings): a
	// 70-minute span yields a single 70-minute segment.
	recs, err = c.FindRecordings(0, time.Now().Add(-10*time.Minute), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Duration != 70*60 {
		t.Errorf("window not echoed verbatim: %+v", recs)
	}

	// An empty/inverted window yields no segments.
	recs, _ = c.FindRecordings(0, time.Now(), time.Now().Add(-time.Hour))
	if len(recs) != 0 {
		t.Errorf("want 0 recordings for inverted window, got %d", len(recs))
	}
}

// TestLiveStreamPlayback exercises the real RTSP->MP4 remux against a physical
// Tiandy NVR. It is skipped unless KSPCAM_TIANDY_HOST is set (host without
// port), with KSPCAM_TIANDY_USER/PASS creds; it plays back a recent 8-second
// window and asserts a non-empty, MP4-signatured stream.
func TestLiveStreamPlayback(t *testing.T) {
	host := os.Getenv("KSPCAM_TIANDY_HOST")
	if host == "" {
		t.Skip("set KSPCAM_TIANDY_HOST to run the live Tiandy playback test")
	}
	user := envOr("KSPCAM_TIANDY_USER", "admin")
	pass := os.Getenv("KSPCAM_TIANDY_PASS")
	c := New(host, user, pass, 30*time.Second)

	end := time.Now().Add(-2 * time.Minute)
	start := end.Add(-8 * time.Second)
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := c.StreamPlayback(ctx, &buf, 0, start, end); err != nil {
		t.Fatalf("StreamPlayback: %v", err)
	}
	if buf.Len() < 1024 {
		t.Fatalf("playback too small: %d bytes", buf.Len())
	}
	// Fragmented MP4 begins with an ftyp box: bytes 4..8 == "ftyp".
	if b := buf.Bytes(); len(b) < 8 || string(b[4:8]) != "ftyp" {
		t.Errorf("output is not MP4 (no ftyp box); first bytes: % x", b[:min(16, len(b))])
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
