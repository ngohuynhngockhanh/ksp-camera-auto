package hik

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// StreamNative downloads a channel's [start,end] recording as Hikvision's
// proprietary native container (magic bytes "IMKH" — the Hik analog of
// Dahua's DHAV .dav) and writes it to w: search the range, then download and
// concatenate each overlapping segment's playbackURI in order via ISAPI's
// /ISAPI/ContentMgmt/download. It is the device's byte-exact stored file — no
// ffmpeg/remux — so it's the FAST option when precise cut boundaries don't
// matter.
//
// Unlike StreamPlayback's RTSP remux, ISAPI's download interface is
// SEGMENT-COARSE, not range-exact: a request effectively returns each whole
// overlapping segment from its own start to its own end (observed live: a
// 15s requested window returned a 613 MB segment), NOT trimmed to [start,end]
// — so a caller wanting an accurate cut should use StreamPlayback instead.
// The output plays back in VLC / the Hikvision player, NOT a browser <video>
// tag (see web/static/review.js, which labels this option accordingly).
//
// host/port address the device's ISAPI HTTP endpoint (port is the ISAPI
// port, NOT the RTSP 554 StreamPlayback uses). channel is the native
// (1-based) Hikvision channel number, matching FindRecordings/StreamPlayback.
// start/end are device-local wall-clock times, converted to UTC via loc
// (from Client.DeviceLocation, cached by the caller) for ISAPI's search.
func StreamNative(ctx context.Context, w io.Writer, host string, port int, user, pass string, channel int, start, end time.Time, loc *time.Location) error {
	select {
	case playbackSem <- struct{}{}:
		defer func() { <-playbackSem }()
	case <-ctx.Done():
		return fmt.Errorf("hik: native download %s: %w (waiting for a slot)", host, ctx.Err())
	}

	// A short-lived client for this one download; DownloadStream internally
	// uses its own timeout-unbound HTTP client for the actual byte stream
	// (see isapi.httpTransport.streamClient), so the timeout here only bounds
	// SearchTrack and connection setup, not the download itself.
	cl := isapi.New(host, port, false, user, pass, 30*time.Second)

	startUTC := inLocation(start, loc).UTC()
	endUTC := inLocation(end, loc).UTC()
	trackID := channel*100 + 1
	segs, err := cl.SearchTrack(ctx, trackID, startUTC, endUTC, 40)
	if err != nil {
		return fmt.Errorf("hik: native download %s: search: %w", host, err)
	}
	if len(segs) == 0 {
		return fmt.Errorf("hik: native download %s: no recording in range", host)
	}

	for i, seg := range segs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := cl.DownloadStream(ctx, w, seg.PlaybackURI); err != nil {
			return fmt.Errorf("hik: native download %s: segment %d/%d: %w", host, i+1, len(segs), err)
		}
	}
	return nil
}
