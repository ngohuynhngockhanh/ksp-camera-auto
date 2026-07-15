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

// Client is a Hikvision device handle backed by ISAPI over HTTP(S).
type Client struct {
	isapi *isapi.Client
}

// Dial builds a Client talking ISAPI to host:port. https selects the scheme;
// pass false for plain HTTP (ISAPI's default, and what Hikvision devices
// serve out of the box). timeout bounds every request.
func Dial(host string, port int, https bool, user, pass string, timeout time.Duration) *Client {
	return &Client{isapi: isapi.New(host, port, https, user, pass, timeout)}
}

// Close releases resources held by the client. ISAPI is plain HTTP request/
// response with no persistent session, so there is nothing to release; this
// exists to satisfy callers that treat every vendor client uniformly.
func (c *Client) Close() error { return nil }

// StreamInfo is a read-back summary of one stream's encode settings.
type StreamInfo = isapi.StreamInfo

// GetStreamInfo reads back the current encode settings for a channel/stream.
// ch is the native (1-based) Hikvision channel number; stream is 0-based
// (0=main, 1=sub1, 2=sub2).
func (c *Client) GetStreamInfo(ctx context.Context, ch, stream int) (StreamInfo, error) {
	return c.isapi.GetStreamInfo(ctx, ch, stream)
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
