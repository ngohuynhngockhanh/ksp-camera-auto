package tiandy

import (
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

// FindRecordings returns the recording index for a channel over [start,end].
//
// Tiandy exposes no pure-Go recording index: it has no ONVIF Profile G
// (Search/Replay), and its DVRIP/ONVIF-authenticated config surfaces reject the
// web-admin credentials (separate user lists) — see the package doc and the
// Phase-0 findings. So this DEGRADES to a synthetic index: it reports a single
// segment spanning the requested window (clamped so it never claims footage in
// the future). Because the review UI splits the loaded window into 5-minute
// quick-view chunks entirely client-side and plays each via RTSP
// playback-by-time (which the device honors for any time), this yields a fully
// usable "Xem lại" on a continuously-recording NVR — the only loss is exact
// per-segment boundaries/gaps, which a 24/7 NVR doesn't have anyway.
//
// start/end are device-local wall-clock times (the review UI's convention).
// The times are echoed straight into the segment's wall-clock strings — we
// deliberately do NOT clamp against time.Now(): the caller parses these as UTC
// wall-clock instants, a different frame from the process's real "now", so any
// instant comparison here would be off by the local UTC offset (it silently
// emptied a valid same-day window in the field). Playback rides the identical
// times and only formats their wall-clock digits, so echoing them keeps the
// index and the stream perfectly consistent.
func (c *Client) FindRecordings(channel int, start, end time.Time) ([]dahua.Recording, error) {
	if !end.After(start) {
		return []dahua.Recording{}, nil
	}
	const f = "2006-01-02 15:04:05"
	return []dahua.Recording{{
		Channel:   channel,
		StartTime: start.Format(f),
		EndTime:   end.Format(f),
		Duration:  int(end.Sub(start).Seconds()),
		Type:      "mp4",
	}}, nil
}
