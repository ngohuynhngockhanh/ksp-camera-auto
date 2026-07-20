// Package tiandy is a pure-Go transport for Tiandy NVRs/cameras (e.g. the
// TC-R3440), for the review ("Xem lại") + IP-config surface. It is deliberately
// NOT a full control client like internal/dahua or internal/hik: the vendor's
// only documented route for recording search and device config is a binary C
// NetSDK on port 3000/3001, which would break this project's CGO_ENABLED=0
// static ARM deploy. So this package uses only what a Tiandy device serves over
// standard, pure-Go-reachable protocols:
//
//   - RTSP (:554) — Tiandy is Dahua-lineage here: live cam/realmonitor and,
//     crucially, playback-by-time cam/playback?...starttime/endtime WORK with
//     the ordinary web-admin credentials (verified live on a TC-R3440). This is
//     the whole video path for playback; codec is HEVC, so the MP4 remux retags
//     hev1->hvc1 exactly like internal/hik.
//   - ONVIF (:8082) — device/media services (no Profile G, so no recording
//     index) and GetNetworkInterfaces for IP-config view. Note authenticated
//     ONVIF needs an ONVIF *user*, which on many NVRs is a separate account
//     from web-admin; GetNetworkConfig surfaces a clear hint when the creds
//     aren't accepted (see ErrONVIFUnauthorized).
//
// Because there is no accessible recording index, FindRecordings degrades to a
// synthetic window (see mediafind.go) — the client-side 5-minute quick-view and
// timeline still work, which is sufficient on a continuously-recording NVR.
//
// All results map onto the shared internal/dahua types (Recording,
// NetworkConfig) so the camera adapter and web UI need no vendor-specific
// branch, exactly the seam internal/hik follows.
package tiandy

import (
	"errors"
	"time"
)

// Fixed Tiandy ports. Device.Port (the configured "primary" port) is not used
// for media/config here — RTSP and ONVIF live on their own well-known ports.
const (
	rtspPort  = 554
	onvifPort = 8082
)

// ErrUnsupported is returned by control operations Tiandy does not expose over
// the pure-Go transports (config apply, password change, OSD, native download).
var ErrUnsupported = errors.New("tiandy: thao tác này chưa hỗ trợ (chỉ xem lại + xem cấu hình mạng qua RTSP/ONVIF)")

// ErrONVIFUnauthorized is returned by GetNetworkConfig when the device rejects
// the web-admin credentials for ONVIF — typically because ONVIF has a separate
// user list. The message tells the operator the one device-side step needed.
var ErrONVIFUnauthorized = errors.New("tiandy: ONVIF từ chối tài khoản này — hãy bật ONVIF và tạo user ONVIF trùng tài khoản trên đầu ghi để xem cấu hình IP")

// Client holds connection parameters for one Tiandy device. There is no
// persistent session: RTSP and ONVIF calls each authenticate per-request, so
// the zero cost of "opening" a client means camera.Open always succeeds and any
// connectivity/credential error surfaces from the first real call (same
// property as hik.Dial).
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
