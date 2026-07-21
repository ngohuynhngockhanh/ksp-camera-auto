package hik

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestPlaybackRTSPURLShapeAndTimes checks the RTSP playback-by-time URL this
// package builds: host:554, credentials embedded, and start/end formatted as
// Hikvision's compact "20060102T150405Z" (confirmed live in a real
// playbackURI, e.g. "starttime=20260719T020941Z") using the SAME wall-clock
// digits as the input — the device treats "Z" as decorative and reads the
// value as its own local time (see isapi.hikTimeLayout), so no UTC/offset
// conversion happens here.
func TestPlaybackRTSPURLShapeAndTimes(t *testing.T) {
	start := time.Date(2026, 7, 19, 2, 9, 41, 0, time.UTC)
	end := time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC)
	url := playbackRTSPURL("192.168.1.215", "admin", "duyanh68A", 1, start, end)

	if !strings.HasPrefix(url, "rtsp://admin:duyanh68A@192.168.1.215:554/Streaming/tracks/101/") {
		t.Fatalf("unexpected URL prefix: %s", url)
	}
	if !strings.Contains(url, "starttime=20260719T020941Z") {
		t.Fatalf("starttime not Hik's compact wall-clock format: %s", url)
	}
	if !strings.Contains(url, "endtime=20260719T030000Z") {
		t.Fatalf("endtime not Hik's compact wall-clock format: %s", url)
	}
}

// TestPlaybackRTSPURLNoOffsetApplied is the regression test for the
// timezone-conversion bug: a device-local wall-clock start/end (here tagged
// with an arbitrary +07:00 zone, exactly as a caller might hand in a
// zone-aware time.Time) must appear in the URL with its OWN wall-clock
// digits, not shifted by that zone's offset toward UTC.
func TestPlaybackRTSPURLNoOffsetApplied(t *testing.T) {
	plus7 := time.FixedZone("device", 7*3600)
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, plus7)
	end := time.Date(2026, 7, 20, 11, 0, 0, 0, plus7)
	url := playbackRTSPURL("192.168.1.215", "admin", "duyanh68A", 1, start, end)

	if !strings.Contains(url, "starttime=20260720T100000Z") {
		t.Fatalf("starttime was shifted by the input's offset (want verbatim 10:00:00): %s", url)
	}
	if !strings.Contains(url, "endtime=20260720T110000Z") {
		t.Fatalf("endtime was shifted by the input's offset (want verbatim 11:00:00): %s", url)
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
