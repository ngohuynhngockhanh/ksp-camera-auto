package hik

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestPlaybackRTSPURLShapeAndUTCTimes checks the RTSP playback-by-time URL
// this package builds: host:554, credentials embedded, and start/end
// formatted as Hikvision's compact UTC "20060102T150405Z" (confirmed live in
// a real playbackURI, e.g. "starttime=20260719T020941Z").
func TestPlaybackRTSPURLShapeAndUTCTimes(t *testing.T) {
	start := time.Date(2026, 7, 19, 2, 9, 41, 0, time.UTC)
	end := time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC)
	url := playbackRTSPURL("192.168.1.215", "admin", "duyanh68A", 1, start, end)

	if !strings.HasPrefix(url, "rtsp://admin:duyanh68A@192.168.1.215:554/Streaming/tracks/101/") {
		t.Fatalf("unexpected URL prefix: %s", url)
	}
	if !strings.Contains(url, "starttime=20260719T020941Z") {
		t.Fatalf("starttime not Hik's compact UTC format: %s", url)
	}
	if !strings.Contains(url, "endtime=20260719T030000Z") {
		t.Fatalf("endtime not Hik's compact UTC format: %s", url)
	}
}

// TestPlaybackRTSPURLTrackIDFormula verifies trackID = channel*100+1, the
// same convention SearchTrack/StreamNative use (verified live: 101 =
// channel 1's recording track).
func TestPlaybackRTSPURLTrackIDFormula(t *testing.T) {
	cases := []struct{ channel, want int }{
		{1, 101},
		{2, 201},
		{32, 3201},
	}
	for _, tc := range cases {
		url := playbackRTSPURL("h", "u", "p", tc.channel, time.Now(), time.Now())
		want := fmt.Sprintf("/Streaming/tracks/%d/", tc.want)
		if !strings.Contains(url, want) {
			t.Errorf("channel %d: url %s missing %s", tc.channel, url, want)
		}
	}
}

// TestInLocationReinterpretsWallClock verifies inLocation keeps t's own
// wall-clock fields but re-tags them with loc, rather than sliding the
// instant — the "parse as device-local wall clock" treatment FindRecordings/
// StreamPlayback/StreamNative all need for the zone-less times the web API
// hands them.
func TestInLocationReinterpretsWallClock(t *testing.T) {
	utcNoon := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	plus7 := time.FixedZone("plus7", 7*3600)
	got := inLocation(utcNoon, plus7)
	if got.Hour() != 12 || got.Location() != plus7 {
		t.Fatalf("inLocation changed the wall-clock hour or didn't tag loc: %v", got)
	}
	if gotUTC := got.UTC(); gotUTC.Hour() != 5 {
		t.Fatalf("12:00 wall clock at +07:00 should be 05:00 UTC, got %v", gotUTC)
	}
}
