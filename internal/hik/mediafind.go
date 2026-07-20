package hik

import (
	"context"
	"fmt"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

// deviceTimeLayout mirrors dahua's own deviceTimeLayout ("2026-07-18
// 21:31:07"): the device-local timestamp format the review UI's shared
// timeline rendering expects in dahua.Recording.StartTime/EndTime, so it
// works unchanged across vendors.
const deviceTimeLayout = "2006-01-02 15:04:05"

// FindRecordings lists recorded segments on a channel between start and end
// (device-local wall-clock times, the review UI's convention) via ISAPI
// content search. channel is the native (1-based) Hikvision channel number
// (matching isapi's channelID convention — internal/camera's hikCamera
// converts the 0-based Profile.Channel before calling in, same as its other
// methods); trackID = channel*100+1 selects the channel's recording track
// (verified live: trackID 101 = channel 1 / NVR D1 — Hikvision does not
// expose recordings per sub-stream the way it does live streams).
//
// DeviceLocation resolves the device's own UTC offset so the device-local
// start/end can be converted for ISAPI's UTC-only search, and so each
// returned segment's times can be converted back to device-local for display
// (matching how mediaFileFind's Dahua times are already device-local, no
// conversion needed there).
func (c *Client) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	loc, err := c.DeviceLocation(ctx)
	if err != nil {
		return nil, fmt.Errorf("hik: find recordings: device time: %w", err)
	}
	startUTC := inLocation(start, loc).UTC()
	endUTC := inLocation(end, loc).UTC()

	trackID := channel*100 + 1
	segs, err := c.isapi.SearchTrack(ctx, trackID, startUTC, endUTC, 40)
	if err != nil {
		return nil, fmt.Errorf("hik: find recordings: %w", err)
	}

	out := make([]dahua.Recording, 0, len(segs))
	for _, s := range segs {
		segStart := s.Start.In(loc)
		segEnd := s.End.In(loc)
		out = append(out, dahua.Recording{
			Channel:   channel,
			StartTime: segStart.Format(deviceTimeLayout),
			EndTime:   segEnd.Format(deviceTimeLayout),
			Duration:  int(segEnd.Sub(segStart).Seconds()),
			Type:      "mp4",
			Length:    s.Size,
			// FilePath has no on-device path analog over ISAPI; the segment's
			// own playbackURI is kept here for reference/debugging only (as
			// dahua.Recording.FilePath's doc already says of its own field —
			// playback/download here always go by time range, not this URI).
			FilePath: s.PlaybackURI,
		})
	}
	return out, nil
}

// inLocation reinterprets t's wall-clock fields (year/month/day/hour/min/sec)
// as being in loc, discarding whatever zone t already carries. Needed
// because the web API hands start/end as zone-less local times (parsed with
// time.Parse, which defaults to UTC) that actually mean "device-local wall
// clock" — the same treatment dahua's davTimeStr gives its own device-local
// strings.
func inLocation(t time.Time, loc *time.Location) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
}
