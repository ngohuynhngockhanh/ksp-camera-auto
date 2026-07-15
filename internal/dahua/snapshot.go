package dahua

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// GetSnapshot fetches a single JPEG frame for a channel via Dahua's HTTP CGI
// (GET /cgi-bin/snapshot.cgi?channel=<channelNo>), confirmed against
// docs-sdk/dahua/dahua_http_api_for_ipcsd-v1.40.pdf section 4.1.3. channel is
// 0-based (the spec: "channel number is default 0 if the request does not
// carry the param").
//
// Unlike this package's usual transport — the DVRIP config protocol on port
// 37777 — snapshot.cgi is plain HTTP+Digest on port 80, the device's
// conventional web/CGI port. host must be bare (no port); the inventory's
// Device.Port is the DVRIP port and is not used here. This is an assumption,
// not something verified against a live device in this codebase: the
// project's live Dahua test camera is only reachable over a NAT'd DVRIP
// port, with HTTP:80 not forwarded (see docs/GOTCHAS.md) — verify on-LAN
// before relying on this in production.
//
// The endpoint has no stream/sub-stream selector — it always returns
// whichever encoder the device's snapshot pipeline uses, so the stream
// parameter callers pass at the camera.Camera interface level is ignored for
// Dahua.
func GetSnapshot(ctx context.Context, host, user, pass string, channel int, timeout time.Duration) ([]byte, error) {
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
