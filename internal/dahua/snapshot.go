package dahua

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

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
	data, rtspErr := GetSnapshotRTSP(ctx, host, user, pass, channel, timeout)
	if rtspErr == nil {
		return data, nil
	}
	data, cgiErr := GetSnapshotCGI(ctx, host, user, pass, channel, timeout)
	if cgiErr == nil {
		return data, nil
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

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-rtsp_transport", "tcp",
		"-skip_frame", "nokey",
		"-i", u.String(),
		"-frames:v", "1",
		"-q:v", "4",
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
		return nil, fmt.Errorf("dahua: ffmpeg snapshot %s: %w: %s", host, err, snapshotTruncate(stderr.Bytes(), 300))
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("dahua: ffmpeg snapshot %s: empty output: %s", host, snapshotTruncate(stderr.Bytes(), 300))
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
