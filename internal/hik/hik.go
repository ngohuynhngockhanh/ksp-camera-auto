// Package hik is a thin adapter over internal/isapi exposing exactly the
// operation set internal/camera needs (SetResolution/SetCodec/SetSmartCodec/
// SetAudioAAC/GetStreamInfo). It exists as a stable seam: today Dial talks
// ISAPI over HTTP(S) because that's the only transport reachable from pure
// Go stdlib without third-party deps, but the user's actual Hikvision
// hardware is only reachable on its proprietary binary port (8000). A later
// milestone (M6) will implement that binary transport and can either satisfy
// this same method set directly or have camera.go pick between the two --
// either way, internal/isapi's payload types (StreamingChannel, SmartCodec,
// Audio) are the ones that get reused, which is the point of building this
// package now.
package hik

import (
	"context"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// Client is a Hikvision device handle backed by an isapi.Client. The isapi
// client may run over HTTP(S) (Dial) or over the proprietary port 8000 via the
// optional cgo SDK backend (NewWithClient) — the operation set is identical.
type Client struct {
	isapi  *isapi.Client
	closer func() error
}

// Dial builds a Client talking ISAPI to host:port over HTTP(S). https selects
// the scheme; pass false for plain HTTP (ISAPI's default, and what Hikvision
// devices serve out of the box). timeout bounds every request.
func Dial(host string, port int, https bool, user, pass string, timeout time.Duration) *Client {
	return &Client{isapi: isapi.New(host, port, https, user, pass, timeout)}
}

// NewWithClient builds a Client over a pre-constructed isapi.Client (e.g. one
// backed by the SDK transport). closer, if non-nil, is invoked by Close to
// release the underlying session (e.g. SDK logout).
func NewWithClient(c *isapi.Client, closer func() error) *Client {
	return &Client{isapi: c, closer: closer}
}

// Close releases resources held by the client. For the HTTP transport there is
// nothing to release; for the SDK transport it logs out the session.
func (c *Client) Close() error {
	if c.closer != nil {
		return c.closer()
	}
	return nil
}

// StreamInfo is a read-back summary of one stream's encode settings.
type StreamInfo = isapi.StreamInfo

// NetworkInterface re-exports isapi.NetworkInterface so callers in
// internal/camera need not import internal/isapi directly.
type NetworkInterface = isapi.NetworkInterface

// ErrOverlayUnsupported re-exports isapi.ErrOverlayUnsupported so callers in
// internal/camera need not import internal/isapi directly.
var ErrOverlayUnsupported = isapi.ErrOverlayUnsupported

// GetStreamInfo reads back the current encode settings for a channel/stream.
// ch is the native (1-based) Hikvision channel number; stream is 0-based
// (0=main, 1=sub1, 2=sub2).
func (c *Client) GetStreamInfo(ctx context.Context, ch, stream int) (StreamInfo, error) {
	return c.isapi.GetStreamInfo(ctx, ch, stream)
}

// ProbeAll lists every channel/stream on the device in one request (an NVR
// returns all its cameras).
func (c *Client) ProbeAll(ctx context.Context) ([]StreamInfo, error) {
	return c.isapi.ProbeAll(ctx)
}

// SetPassword changes the admin account (id 1) to userName/newPass.
func (c *Client) SetPassword(ctx context.Context, userName, newPass string) error {
	return c.isapi.SetUserPassword(ctx, 1, userName, newPass)
}

// SetResolution sets the pixel resolution for a channel/stream. Pass fps<=0
// to leave the frame rate untouched.
func (c *Client) SetResolution(ctx context.Context, ch, stream, w, h, fps int) error {
	return c.isapi.SetResolution(ctx, ch, stream, w, h, fps)
}

// SetCodec sets the video codec (isapi.CodecH264/CodecH265/CodecMJPEG) for a
// channel/stream.
func (c *Client) SetCodec(ctx context.Context, ch, stream int, codec string) error {
	return c.isapi.SetCodec(ctx, ch, stream, codec)
}

// SetSmartCodec toggles Smart Codec (H.264+/H.265+) for a channel/stream.
func (c *Client) SetSmartCodec(ctx context.Context, ch, stream int, on bool) error {
	return c.isapi.SetSmartCodec(ctx, ch, stream, on)
}

// SetAudioAAC forces the stream's audio codec to AAC and enables audio.
func (c *Client) SetAudioAAC(ctx context.Context, ch, stream int) error {
	return c.isapi.SetAudioAAC(ctx, ch, stream)
}

// SetGOP sets the I-frame interval (frames) for a channel/stream.
func (c *Client) SetGOP(ctx context.Context, ch, stream, gop int) error {
	return c.isapi.SetGOP(ctx, ch, stream, gop)
}

// SetBitrate sets the video bitrate (Kbps) and, when mode is non-empty, the
// bitrate control mode ("VBR"/"CBR") for a channel/stream.
func (c *Client) SetBitrate(ctx context.Context, ch, stream, kbps int, mode string) error {
	return c.isapi.SetBitrate(ctx, ch, stream, kbps, mode)
}

// GetSnapshot fetches a single JPEG frame for a channel/stream.
func (c *Client) GetSnapshot(ctx context.Context, ch, stream int) ([]byte, error) {
	return c.isapi.GetSnapshot(ctx, ch, stream)
}

// GetNetworkInterfaces reads every interface's IP configuration (static IP /
// DHCP, per LAN and Wi-Fi interface) in one request.
func (c *Client) GetNetworkInterfaces(ctx context.Context) ([]NetworkInterface, error) {
	return c.isapi.GetNetworkInterfaces(ctx)
}

// Reboot restarts the device (ISAPI PUT /ISAPI/System/reboot, with retry).
func (c *Client) Reboot(ctx context.Context) error {
	return c.isapi.Reboot(ctx)
}

// SetStaticIP writes one interface's IP configuration (addressed by the
// device's own interface id, e.g. "1"). See isapi.Client.SetStaticIP for
// validation and the caveat that applying a new IP moves the device off its
// current address.
func (c *Client) SetStaticIP(ctx context.Context, ifaceID string, dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	return c.isapi.SetStaticIP(ctx, ifaceID, dhcpEnable, ip, mask, gateway, dns)
}

// SetChannelName writes the device's own channel name (distinct from our
// inventory label).
func (c *Client) SetChannelName(ctx context.Context, ch int, name string) error {
	return c.isapi.SetChannelName(ctx, ch, name)
}

// GetChannelName reads back the device's own channel name.
func (c *Client) GetChannelName(ctx context.Context, ch int) (string, error) {
	return c.isapi.GetChannelName(ctx, ch)
}

// GetOverlayText reads back the on-screen text overlay lines for a channel
// plus each slot's enable state. Returns isapi.ErrOverlayUnsupported if the
// device doesn't expose them.
func (c *Client) GetOverlayText(ctx context.Context, ch int) (lines []string, enabled []bool, err error) {
	return c.isapi.GetOverlayText(ctx, ch)
}

// SetOverlayText writes on-screen text overlay lines and enable state for a
// channel, applying as many as the device has slots for (returned as
// applied). Returns isapi.ErrOverlayUnsupported if the device doesn't expose
// them.
func (c *Client) SetOverlayText(ctx context.Context, ch int, lines []string, enabled []bool) (applied int, err error) {
	return c.isapi.SetOverlayText(ctx, ch, lines, enabled)
}
