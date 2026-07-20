package tiandy

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/mediaexport"
	"time"
)

// playbackSem bounds simultaneous Tiandy playback remux processes across the
// whole tool — a SEPARATE pool from Dahua's and Hik's, so a burst on one vendor
// can't starve the others. Each ffmpeg -c copy remux is light but long-lived.
var playbackSem = make(chan struct{}, 4)

// tailBuf keeps only the last max bytes written, to capture ffmpeg's stderr
// tail for an error message without growing unbounded on a multi-hour stream.
type tailBuf struct {
	max int
	buf []byte
}

func (t *tailBuf) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func tail(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return string(b)
}

// tiandyChannel converts a vendor-neutral (0-based) channel to Tiandy's native
// (1-based) RTSP channel number (cam/realmonitor|playback ?channel=N).
func tiandyChannel(neutral int) int { return neutral + 1 }

// playbackRTSPURL builds Tiandy's Dahua-format RTSP playback-by-time URL.
// channel is native (1-based). start/end are device-LOCAL wall-clock times sent
// verbatim in Tiandy's "YYYY_MM_DD_HH_MM_SS" convention — no UTC conversion
// (the device reads its own recordings in local time; verified live on a
// TC-R3440 where local = UTC+7).
func playbackRTSPURL(host, user, pass string, channel int, start, end time.Time) string {
	const f = "2006_01_02_15_04_05"
	u := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:%d", host, rtspPort),
		Path:   "/cam/playback",
	}
	u.RawQuery = fmt.Sprintf("channel=%d&starttime=%s&endtime=%s", channel, start.Format(f), end.Format(f))
	return u.String()
}

// StreamPlayback streams a channel's [start,end] recording to w as a fragmented
// MP4, remuxed from Tiandy RTSP playback-by-time with ffmpeg (-c copy, no
// transcode) — IDENTICAL args to hik.StreamPlayback, including the hev1->hvc1
// retag that makes the HEVC-in-MP4 playable on Safari/iOS. channel is
// vendor-neutral (0-based); start/end are device-local wall-clock times.
//
// Acquiring a concurrency slot can block; it respects ctx so a client that
// disconnects while queued doesn't hold the request.
func (c *Client) StreamPlayback(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	select {
	case playbackSem <- struct{}{}:
		defer func() { <-playbackSem }()
	case <-ctx.Done():
		return fmt.Errorf("tiandy: playback %s: %w (waiting for a slot)", c.host, ctx.Err())
	}

	rtsp := playbackRTSPURL(c.host, c.user, c.pass, tiandyChannel(channel), start, end)
	// Tiandy's RTSP playback streams from starttime at ~realtime and does NOT
	// stop at endtime (verified live — it runs past the window into live). Bound
	// the output to the requested window length with -t so a download terminates
	// with exactly the asked-for span; without it the stream never ends.
	dur := int(end.Sub(start).Seconds())
	if dur < 1 {
		dur = 1
	}
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-rtsp_transport", "tcp",
		"-i", rtsp,
		"-t", fmt.Sprintf("%d", dur),
		// Video is copied (no HEVC transcode — the box can't afford it). Audio
		// is the catch: Tiandy streams G.711 a-law (pcm_alaw), which MP4 cannot
		// carry, so a plain "-c copy" fails with "codec not currently supported
		// in container". Transcode just the audio to AAC (ffmpeg's built-in
		// encoder, ~8 kHz mono — negligible CPU); harmless when there's no audio
		// track. This is the one place Tiandy's remux differs from Hik's.
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "64k",
		// Tiandy records HEVC/H.265. ffmpeg tags an HEVC MP4 track "hev1" by
		// default, but Safari/iOS only play HEVC-in-MP4 tagged "hvc1" (Chrome/
		// Firefox can't decode HEVC either way). Retagging is free — still a
		// video copy, just rewriting the sample-entry fourcc.
		"-tag:v", "hvc1",
		"-fflags", "+genpts",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"-y", "pipe:1",
	)
	cmd.Stdout = w
	stderr := &tailBuf{max: 4096}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tiandy: ffmpeg playback %s: %w: %s", c.host, err, tail(stderr.buf, 300))
	}
	return nil
}

// StreamPlaybackFast exports [start,end] as MP4 much faster than realtime by
// fetching 1-minute chunks over parallel RTSP playback sessions (Tiandy paces a
// single session at ~1x and exposes no byte-download API, but allows several
// concurrent playback sessions per channel). Audio is transcoded to AAC per
// chunk (Tiandy streams G.711 a-law). Same exact-cut, browser-playable MP4 as
// StreamPlayback, ~5× faster for a 5-minute clip.
func (c *Client) StreamPlaybackFast(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	ch := tiandyChannel(channel)
	return mediaexport.FastMP4Range(ctx, w, start, end, 60, func(cs, ce time.Time) string {
		return playbackRTSPURL(c.host, c.user, c.pass, ch, cs, ce)
	}, true, 5)
}

// StreamNative exports [start,end] as an MKV with BOTH streams copied
// untouched (HEVC + G.711 a-law as recorded) — the closest thing to Tiandy's
// "original file" that exists pure-Go: this firmware supports neither ISAPI
// ContentMgmt/search nor ContentMgmt/download (both probed live → notSupport;
// port 3002 is the closed binary NetSDK), so byte-exact container download is
// impossible and RTSP stream-copy is the source of truth. Fetches parallel
// chunks like StreamPlaybackFast. Plays in VLC/desktop players, not a browser.
func (c *Client) StreamNative(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	ch := tiandyChannel(channel)
	return mediaexport.FastNativeRange(ctx, w, start, end, 60, func(cs, ce time.Time) string {
		return playbackRTSPURL(c.host, c.user, c.pass, ch, cs, ce)
	}, 5)
}

// liveRTSPURL builds Tiandy's Dahua-format live RTSP URL (sub stream), used for
// snapshot frame-grabs. channel is native (1-based).
func liveRTSPURL(host, user, pass string, channel, subtype int) string {
	u := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:%d", host, rtspPort),
		Path:   "/cam/realmonitor",
	}
	u.RawQuery = fmt.Sprintf("channel=%d&subtype=%d", channel, subtype)
	return u.String()
}
