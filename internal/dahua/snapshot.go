package dahua

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// ffmpegSem caps how many ffmpeg snapshot processes may run at once across
// the whole process. Each ffmpeg RTSP grab costs ~30-40 MB RSS plus decode
// CPU, and the gallery ("Tất cả kênh" / "Xem hình hàng loạt") fires a
// snapshot per channel — unbounded, that OOMs a small box. This bound makes
// snapshot load flat regardless of how many the UI requests: excess calls
// block on the channel until a slot frees. Default 2; override with
// KSPCAM_FFMPEG_CONCURRENCY (e.g. 1 on the tightest boxes).
var ffmpegSem = make(chan struct{}, ffmpegConcurrency())

func ffmpegConcurrency() int {
	if v := os.Getenv("KSPCAM_FFMPEG_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			return n
		}
	}
	return 2
}

// ffmpegSnapTimeout caps a single ffmpeg grab regardless of the (larger)
// per-request timeout, so a hung/unreachable camera doesn't hold an ffmpeg
// process — and its RAM, and a concurrency slot — for the full request
// deadline. The overall request timeout still applies on top (whichever is
// shorter wins).
const ffmpegSnapTimeout = 12 * time.Second

// GetSnapshot fetches a single JPEG frame for a channel, trying the RTSP
// route first (GetSnapshotRTSP) and falling back to the HTTP CGI route
// (GetSnapshotCGI) if that fails — e.g. ffmpeg isn't installed, or RTSP:554
// isn't reachable while CGI:80 is. RTSP is tried first because it's the more
// broadly-supported/enabled surface in practice: CLIENT_SnapPicture-family
// NetSDK calls turned out to be vehicle-DVR-only (see
// docs/PROTOCOL-DAHUA.md), and snapshot.cgi has been observed returning a
// bare "Bad Request" on modern firmware even with correct credentials (see
// docs/GOTCHAS.md) — whereas RTSP is the same stream path live viewing
// already depends on, so if that works, snapshot works.
func GetSnapshot(ctx context.Context, host, user, pass string, channel int, timeout time.Duration) ([]byte, error) {
	// DVRIP first: a single round trip on the config protocol returns a ready
	// JPEG with no ffmpeg/decode — the smooth, low-latency path. It needs the
	// config port + a login. Restricted to channel 0: the snapshot command's
	// channel byte couldn't be verified on a multi-channel NVR (a single-channel
	// IPC returns channel 0's frame regardless), so for channel>0 we take the
	// RTSP path, which selects the channel correctly, rather than risk silently
	// returning channel 0's image. Every single-channel IPC (channel 0) gets the
	// fast DVRIP path — which is the whole gallery on these fleets.
	if channel == 0 {
		if data, dvripErr := GetSnapshotDVRIP(ctx, host, user, pass, channel, timeout); dvripErr == nil {
			return data, nil
		} else {
			return snapshotFallback(ctx, host, user, pass, channel, timeout, dvripErr)
		}
	}
	return snapshotFallback(ctx, host, user, pass, channel, timeout, nil)
}

// snapshotFallback tries RTSP (ffmpeg) then CGI, threading through any prior
// DVRIP error for the combined message.
func snapshotFallback(ctx context.Context, host, user, pass string, channel int, timeout time.Duration, dvripErr error) ([]byte, error) {
	data, rtspErr := GetSnapshotRTSP(ctx, host, user, pass, channel, timeout)
	if rtspErr == nil {
		return data, nil
	}
	data, cgiErr := GetSnapshotCGI(ctx, host, user, pass, channel, timeout)
	if cgiErr == nil {
		return data, nil
	}
	if dvripErr != nil {
		return nil, fmt.Errorf("dahua: snapshot %s: dvrip: %v; rtsp: %v; cgi: %v", host, dvripErr, rtspErr, cgiErr)
	}
	return nil, fmt.Errorf("dahua: snapshot %s: rtsp: %v; cgi: %v", host, rtspErr, cgiErr)
}

// GetSnapshotRTSP grabs one JPEG frame from the camera's RTSP stream
// (rtsp://user:pass@host:554/cam/realmonitor?channel=<n>&subtype=1) by
// shelling out to ffmpeg, confirmed against
// dahua_http_api_for_ipcsd-v1.40.pdf §4.1.1 for the URL shape (channel is
// 1-based there, unlike this package's usual 0-based convention, so callers'
// 0-based channel is converted here). subtype=1 (the sub/extra stream) is
// used deliberately — smaller frames mean less to decode.
// -skip_frame nokey plus -frames:v 1 stop ffmpeg at the first keyframe
// instead of decoding a running stream, keeping this a bounded, cheap
// single-frame grab rather than a continuous transcode (no re-encode of a
// live feed, no reason to burn CPU beyond the one frame this needs).
// Requires ffmpeg on PATH; returns an error (not a panic/hang) if it's
// missing or the stream is unreachable, so GetSnapshot's CGI fallback can
// take over.
func GetSnapshotRTSP(ctx context.Context, host, user, pass string, channel int, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 || timeout > ffmpegSnapTimeout {
		timeout = ffmpegSnapTimeout
	}

	// Acquire a concurrency slot before spending the timeout budget or
	// spawning anything, so a burst of gallery requests queues instead of
	// launching a process each. Give up if the caller's context ends while
	// we're still queued.
	select {
	case ffmpegSem <- struct{}{}:
		defer func() { <-ffmpegSem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("dahua: snapshot %s: %w (waiting for ffmpeg slot)", host, ctx.Err())
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	u := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:554", host),
		Path:   "/cam/realmonitor",
		RawQuery: url.Values{
			"channel": {fmt.Sprintf("%d", channel+1)},
			"subtype": {"1"},
		}.Encode(),
	}

	// Deliberately minimal flags: -nostdin/-threads 1 are safe caps on
	// input/CPU, but stream-probing limiters (-probesize/-analyzeduration)
	// and socket-timeout flags (-rw_timeout/-stimeout) vary by ffmpeg version
	// and can make older builds fail to detect an H.265 RTSP stream at all —
	// the concurrency semaphore above and the context deadline below already
	// bound resources, so we don't need them. -skip_frame nokey + -frames:v 1
	// still stop at the first keyframe (cheap single-frame grab).
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-threads", "1",
		"-rtsp_transport", "tcp",
		"-skip_frame", "nokey",
		"-i", u.String(),
		"-frames:v", "1",
		"-q:v", "6",
		"-f", "image2",
		"-y", "-",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("dahua: ffmpeg not installed: %w", err)
		}
		// Show the TAIL of stderr — ffmpeg's actual error line comes after
		// its multi-line build banner, so a head-truncation would only ever
		// surface the version string, not the diagnosis.
		return nil, fmt.Errorf("dahua: ffmpeg snapshot %s: %w: %s", host, err, snapshotTail(stderr.Bytes(), 300))
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("dahua: ffmpeg snapshot %s: empty output: %s", host, snapshotTail(stderr.Bytes(), 300))
	}
	return stdout.Bytes(), nil
}

// GetSnapshotCGI fetches a single JPEG frame for a channel via Dahua's HTTP
// CGI (GET /cgi-bin/snapshot.cgi?channel=<channelNo>), confirmed against
// docs-sdk/dahua/dahua_http_api_for_ipcsd-v1.40.pdf section 4.1.3. channel is
// 0-based (the spec: "channel number is default 0 if the request does not
// carry the param").
//
// Unlike this package's usual transport — the DVRIP config protocol on port
// 37777 — snapshot.cgi is plain HTTP+Digest on port 80, the device's
// conventional web/CGI port. host must be bare (no port); the inventory's
// Device.Port is the DVRIP port and is not used here.
//
// The endpoint has no stream/sub-stream selector — it always returns
// whichever encoder the device's snapshot pipeline uses, so the stream
// parameter callers pass at the camera.Camera interface level is ignored for
// Dahua.
func GetSnapshotCGI(ctx context.Context, host, user, pass string, channel int, timeout time.Duration) ([]byte, error) {
	digest := isapi.NewDigestTransport(user, pass, nil)
	client := &http.Client{Transport: digest, Timeout: timeout}
	url := fmt.Sprintf("http://%s:80/cgi-bin/snapshot.cgi?channel=%d", host, channel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("dahua: build snapshot request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dahua: snapshot %s: %w", host, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("dahua: read snapshot %s: %w", host, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dahua: snapshot %s: HTTP %d: %s", host, resp.StatusCode, snapshotTruncate(data, 200))
	}
	return data, nil
}

func snapshotTruncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// snapshotTail returns the last n bytes of b (with a leading ellipsis when
// truncated), used for ffmpeg stderr where the meaningful error is at the end.
func snapshotTail(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return "..." + string(b[len(b)-n:])
}
