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
// via ISAPI content search. channel is the native (1-based) Hikvision channel
// number (internal/camera's hikCamera converts the 0-based Profile.Channel
// before calling in); trackID = channel*100+1 selects the channel's recording
// track (verified live: 101 = channel 1 / NVR D1).
//
// start/end are device-local wall-clock times (the review UI's convention) and
// are passed to the NVR verbatim: this NVR treats ISAPI search times as
// device-local despite their "Z" suffix (see isapi.hikTimeLayout), and returns
// segment times the same way — so there is NO timezone conversion here in
// either direction. (An earlier version converted local↔UTC via the device's
// offset, which shifted every displayed recording and every playback request
// by the device's whole UTC offset — e.g. 7h for a UTC+7 device.)
func (c *Client) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	trackID := channel*100 + 1
	segs, err := c.isapi.SearchTrack(ctx, trackID, start, end, 40)
	if err != nil {
		return nil, fmt.Errorf("hik: find recordings: %w", err)
	}

	out := make([]dahua.Recording, 0, len(segs))
	for _, s := range segs {
		out = append(out, dahua.Recording{
			Channel:   channel,
			StartTime: s.Start.Format(deviceTimeLayout),
			EndTime:   s.End.Format(deviceTimeLayout),
			Duration:  int(s.End.Sub(s.Start).Seconds()),
			Type:      "mp4",
			// FilePath has no on-device path analog over ISAPI; the segment's
			// own playbackURI is kept here for reference/debugging only —
			// playback/download here always go by time range, not this URI.
			FilePath: s.PlaybackURI,
			Length:   s.Size,
		})
	}
	return out, nil
}
