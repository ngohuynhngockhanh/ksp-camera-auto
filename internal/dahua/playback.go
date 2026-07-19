package dahua

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"time"
)

// playbackConcurrency bounds simultaneous playback remux processes across the
// whole tool. Each ffmpeg -c copy remux is light (~30 MB RSS, no transcode),
// but a playback stream can run for a while, so cap it so a burst of download
// requests can't exhaust a small box.
var playbackSem = make(chan struct{}, 4)

// playbackRTSPURL builds Dahua's RTSP playback-by-time URL for a channel.
// channel is 0-based (converted to Dahua's 1-based RTSP channel). start/end are
// device-local times formatted as YYYY_MM_DD_HH_MM_SS, confirmed live against a
// DH-C5A (rtsp://.../cam/playback?channel=1&starttime=...&endtime=...).
func playbackRTSPURL(host, user, pass string, channel int, start, end time.Time) string {
	const f = "2006_01_02_15_04_05"
	u := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:554", host),
		Path:   "/cam/playback",
	}
	// Build the query in a FIXED order (channel, subtype, starttime, endtime).
	// url.Values.Encode() sorts keys alphabetically, which puts endtime BEFORE
	// starttime — IP cameras tolerate that, but Dahua NVRs are order-sensitive and
	// return RTSP 404 for it. subtype 0 = main stream (required by some models).
	u.RawQuery = fmt.Sprintf("channel=%d&subtype=0&starttime=%s&endtime=%s", channel+1, start.Format(f), end.Format(f))
	return u.String()
}

// tailWriter keeps only the last max bytes written to it, so ffmpeg's
// continuous progress output on a long (hours) playback can be captured for an
// error message without growing unbounded.
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

// StreamPlayback streams the recording for channel over [start,end] to w as a
// fragmented MP4. It pulls Dahua's RTSP playback and remuxes with ffmpeg
// (-c copy, no transcode), piping ffmpeg's stdout straight to w — NOTHING is
// written to the box's local disk, protecting the eMMC/SD from wear on large
// downloads. Dahua serves playback far faster than realtime (~400x observed),
// so even a multi-hour range completes in seconds-to-minutes and the RTSP
// session transparently spans all the underlying .dav segments in the range.
//
// The output is fragmented MP4 (frag_keyframe+empty_moov+default_base_moof) so
// it is a valid, seekable-enough stream without ffmpeg needing to seek back to
// rewrite a moov atom — which a pipe can't do. +genpts repairs the unset/
// non-monotonic DTS Dahua playback emits.
//
// Acquiring a concurrency slot can block; it respects ctx so a client that
// disconnects while queued doesn't hold the request.
func StreamPlayback(ctx context.Context, w io.Writer, host, user, pass string, channel int, start, end time.Time) error {
	select {
	case playbackSem <- struct{}{}:
		defer func() { <-playbackSem }()
	case <-ctx.Done():
		return fmt.Errorf("dahua: playback %s: %w (waiting for a slot)", host, ctx.Err())
	}

	rtsp := playbackRTSPURL(host, user, pass, channel, start, end)
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
		return fmt.Errorf("dahua: ffmpeg playback %s: %w: %s", host, err, snapshotTail(stderr.buf, 300))
	}
	return nil
}
