package hik

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"time"
)

// playbackSem bounds simultaneous Hikvision playback remux processes across
// the whole tool, mirroring dahua.playbackSem — a SEPARATE pool from Dahua's,
// so a burst of Hik downloads can't starve Dahua ones or vice versa. Each
// ffmpeg -c copy remux is light (no transcode) but can run for a while.
var playbackSem = make(chan struct{}, 4)

// tailWriter keeps only the last max bytes written to it, so ffmpeg's
// continuous progress output on a long (hours) playback can be captured for
// an error message without growing unbounded. Copied from dahua's tailWriter
// rather than imported — internal/hik deliberately doesn't depend on
// internal/dahua for anything but the shared dahua.Recording type.
type tailWriter struct {
	max int
	buf []byte
}

func (t *tailWriter) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

// playbackRTSPURL builds Hikvision's RTSP playback-by-time URL for a channel.
// channel is the native (1-based) Hikvision channel number; trackID =
// channel*100+1 (same convention as SearchTrack — verified live: 101 =
// channel 1 main stream). startUTC/endUTC MUST already be UTC (the device
// rejects/mis-handles a numeric-offset timestamp here, same as SearchTrack),
// formatted compact per Hikvision's playbackURI convention
// ("20260719T020941Z", confirmed live in a real playbackURI).
func playbackRTSPURL(host, user, pass string, channel int, startUTC, endUTC time.Time) string {
	const f = "20060102T150405Z"
	u := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:554", host),
		Path:   fmt.Sprintf("/Streaming/tracks/%d/", channel*100+1),
	}
	u.RawQuery = fmt.Sprintf("starttime=%s&endtime=%s", startUTC.UTC().Format(f), endUTC.UTC().Format(f))
	return u.String()
}

// StreamPlayback streams the recording for channel over [start,end] to w as a
// fragmented MP4. It pulls Hikvision's RTSP playback-by-time and remuxes with
// ffmpeg (-c copy, no transcode) — IDENTICAL args to dahua.StreamPlayback —
// piping ffmpeg's stdout straight to w with nothing written to the box's own
// disk. RTSP honors the exact start/end (unlike StreamNative's native
// download, which is segment-coarse — see download.go), so this is the
// accurate, browser-playable path.
//
// start/end are device-local wall-clock times (the review UI's convention);
// loc (from Client.DeviceLocation, cached by the caller) converts them to the
// UTC the RTSP URL requires. port is accepted for signature parity with
// StreamNative (which needs it to reach ISAPI over HTTP) but unused here —
// Hikvision RTSP playback is always on the fixed port 554.
//
// Acquiring a concurrency slot can block; it respects ctx so a client that
// disconnects while queued doesn't hold the request.
func StreamPlayback(ctx context.Context, w io.Writer, host string, port int, user, pass string, channel int, start, end time.Time, loc *time.Location) error {
	select {
	case playbackSem <- struct{}{}:
		defer func() { <-playbackSem }()
	case <-ctx.Done():
		return fmt.Errorf("hik: playback %s: %w (waiting for a slot)", host, ctx.Err())
	}

	startUTC := inLocation(start, loc).UTC()
	endUTC := inLocation(end, loc).UTC()
	rtsp := playbackRTSPURL(host, user, pass, channel, startUTC, endUTC)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-rtsp_transport", "tcp",
		"-i", rtsp,
		"-c", "copy",
		"-fflags", "+genpts",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"-y", "pipe:1",
	)
	cmd.Stdout = w
	stderr := &tailWriter{max: 4096}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hik: ffmpeg playback %s: %w: %s", host, err, tail(stderr.buf, 300))
	}
	return nil
}

// tail returns the last n bytes of buf as a string (for a short error-message
// snippet of ffmpeg's captured stderr tail).
func tail(buf []byte, n int) string {
	if len(buf) > n {
		buf = buf[len(buf)-n:]
	}
	return string(buf)
}
