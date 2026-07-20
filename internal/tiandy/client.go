// Package tiandy is a pure-Go client for Tiandy NVRs/cameras (e.g. the
// TC-R3440), covering both the review ("Xem lại") surface and full device
// configuration, over only standard, pure-Go-reachable protocols (no cgo C
// NetSDK, so the CGO_ENABLED=0 static ARM deploy is preserved):
//
//   - RTSP (:554) — Tiandy is Dahua-lineage here: live cam/realmonitor and
//     playback-by-time cam/playback?...starttime/endtime work with the web
//     credentials (verified on a TC-R3440). This is the video/playback path;
//     codec is HEVC + G.711, so the MP4 remux retags hev1->hvc1 and transcodes
//     audio to AAC (see playback.go). Recording search has no pure-Go index, so
//     FindRecordings degrades to a synthetic window (see mediafind.go).
//   - ISAPI over :8081 (see isapi_session.go) — Tiandy serves a
//     Hikvision-compatible /ISAPI surface, but authenticates with its own CGI
//     session scheme (iterated SHA-256 -> HttpSession header) instead of HTTP
//     Digest. NewISAPIClient wraps that auth as an isapi.Transport, so the whole
//     internal/isapi + internal/hik config stack (network/IP, encode, password,
//     OSD, storage, remote devices, reboot) is reused verbatim.
//
// Playback/index results map onto the shared internal/dahua types, and config
// runs through internal/hik — so the camera adapter and web UI need no
// Tiandy-specific branch beyond dispatch.
package tiandy

import "time"

// rtspPort is the fixed media port. Device.Port is not used for media/config —
// RTSP is always :554 and config/ISAPI is always :8081 (see configPort).
const rtspPort = 554

// ErrUnsupported is returned by the few operations Tiandy exposes over neither
// transport (native .dav download; Wi-Fi provisioning on wired NVRs).
var ErrUnsupported = errorString("tiandy: thao tác này chưa hỗ trợ")

type errorString string

func (e errorString) Error() string { return string(e) }

// Client holds connection parameters for one Tiandy device's RTSP (media)
// plane. tiandy.New never touches the network, so camera.Open always succeeds
// and any connectivity/credential error surfaces from the first real call
// (same property as hik.Dial). The config plane is a separate isapi.Client
// built by NewISAPIClient.
type Client struct {
	host    string
	user    string
	pass    string
	timeout time.Duration
}

// New builds a Client for host with the given credentials. host is the bare
// hostname/IP (no port) — RTSP and ONVIF ports are fixed constants.
func New(host, user, pass string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{host: host, user: user, pass: pass, timeout: timeout}
}

// Close is a no-op (no persistent connection); present for symmetry.
func (c *Client) Close() error { return nil }
