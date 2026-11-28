package hik

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/mediaexport"
)

// StreamPlaybackFast exports [start,end] as MP4 faster than realtime by fetching
// 1-minute chunks over parallel RTSP playback sessions (Hik paces one session at
// ~1x and its fast ISAPI download returns non-MP4 "IMKH"; but it allows several
// concurrent playback sessions). Same exact-cut, browser-playable MP4 as
// StreamPlayback. port is unused (RTSP is always 554), kept for signature parity.
func StreamPlaybackFast(ctx context.Context, w io.Writer, host string, port int, user, pass string, channel int, start, end time.Time) error {
	return mediaexport.FastMP4Range(ctx, w, start, end, 60, func(cs, ce time.Time) string {
		return playbackRTSPURL(host, user, pass, channel, cs, ce)
	}, false, 5)
}

// playbackSem bounds simultaneous Hikvision playback remux processes across
// the whole tool, mirroring dahua.playbackSem — a SEPARATE pool from Dahua's,
// so a burst of Hik downloads can't starve Dahua ones or vice versa. Each
// ffmpeg -c copy remux is light (no transcode) but can run for a while.
var playbackSem = make(chan struct{}, 4)

// playbackRTSPURL builds Hikvision's RTSP playback-by-time URL for a channel.
// channel is the native (1-based) Hikvision channel number; trackID =
// channel*100+1 (same convention as SearchTrack — verified live: 101 =
// channel 1 main stream). start/end are device-LOCAL wall-clock times sent
// verbatim (the device interprets the "Z"-suffixed value as local, same as
// ISAPI search — see isapi.hikTimeLayout), formatted compact per Hikvision's
// playbackURI convention ("20260719T020941Z", confirmed live).
func playbackRTSPURL(host, user, pass string, channel int, start, end time.Time) string {
	const f = "20060102T150405Z"
	u := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:554", host),
		Path:   fmt.Sprintf("/Streaming/tracks/%d/", channel*100+1),
	}
	u.RawQuery = fmt.Sprintf("starttime=%s&endtime=%s", start.Format(f), end.Format(f))
	return u.String()
}

func buildPlaybackFFmpegArgs(rtsp string, useHVC1Tag bool) []string {
	return mediaexport.BuildFragmentedMP4Args(rtsp, 0, true, useHVC1Tag)
}

func isHVC1TagIncompatible(errText string) bool {
	return mediaexport.IsHVC1TagIncompatible(errText)
}

// StreamPlayback streams the recording for channel over [start,end] to w as a
// fragmented MP4. It pulls Hikvision's RTSP playback-by-time and remuxes with
// ffmpeg (-c copy, no transcode), piping ffmpeg's stdout straight to w with
// nothing written to the box's own disk. Starts with -tag:v hvc1 for Safari/iOS
// HEVC playback and retries without the tag if ffmpeg rejects it for H.264.
//
// start/end are device-local wall-clock times (the review UI's convention),
// sent to the NVR verbatim — no UTC conversion. port is unused here (RTSP
// playback is always on fixed port 554).
func StreamPlayback(ctx context.Context, w io.Writer, host string, port int, user, pass string, channel int, start, end time.Time) error {
	select {
	case playbackSem <- struct{}{}:
		defer func() { <-playbackSem }()
	case <-ctx.Done():
		return fmt.Errorf("hik: playback %s: %w (waiting for a slot)", host, ctx.Err())
	}

	rtsp := playbackRTSPURL(host, user, pass, channel, start, end)
	if err := mediaexport.StreamFragmentedMP4(ctx, w, func(useHVC1 bool) []string {
		return buildPlaybackFFmpegArgs(rtsp, useHVC1)
	}); err != nil {
		return fmt.Errorf("hik: ffmpeg playback %s: %w", host, err)
	}
	return nil
}
