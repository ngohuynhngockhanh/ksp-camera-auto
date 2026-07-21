package dahua

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// GetSnapshotDVRIP fetches one JPEG frame over the DVRIP config protocol (port
// 37777) — no ffmpeg, no RTSP. It logs in, sends the 0x11 snapshot command, and
// reassembles the JPEG the device streams back in 0xbc data frames (terminated
// by a zero-length 0xbc frame). Reverse-engineered from a NetSDK SnapPictureEx
// capture: the single 0x11 frame is the whole request; the rest of what the SDK
// sends is unrelated boilerplate.
//
// This is the smooth/low-latency path (a single round trip on the already-open
// config protocol, decoded by the device, delivered as a ready JPEG) versus
// spawning ffmpeg to decode an RTSP keyframe. It needs the DVRIP port + a login,
// so callers keep the RTSP/CGI routes as fallbacks (see GetSnapshot).
func GetSnapshotDVRIP(ctx context.Context, host string, port int, user, pass string, channel int, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	c, err := Dial(net.JoinHostPort(host, dvripPort(port)), user, pass, timeout)
	if err != nil {
		return nil, fmt.Errorf("dahua: snapshot dvrip %s: login: %w", host, err)
	}
	defer c.Close()
	go func() { <-ctx.Done(); c.Close() }()

	jpeg, err := c.snapOnce(channel)
	if err != nil {
		return nil, fmt.Errorf("dahua: snapshot dvrip %s: %w", host, err)
	}
	return jpeg, nil
}

// snapOnce sends the 0x11 snapshot command and reassembles one JPEG from the
// 0xbc data frames the device returns (terminated by a zero-length 0xbc frame).
// It reuses the connection, so a caller can loop it for an MJPEG live stream.
func (c *Client) snapOnce(channel int) ([]byte, error) {
	if err := c.writeRaw(snapCommand(channel)); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	var jpeg []byte
	for {
		hdr, payload, err := c.readFrame()
		if err != nil {
			return nil, fmt.Errorf("read: %w (after %d bytes)", err, len(jpeg))
		}
		if hdr[0] != 0xbc {
			continue
		}
		if len(payload) == 0 {
			break
		}
		jpeg = append(jpeg, payload...)
		if len(jpeg) > 16<<20 {
			return nil, fmt.Errorf("oversized")
		}
	}
	if len(jpeg) < 2 || jpeg[0] != 0xff || jpeg[1] != 0xd8 {
		return nil, fmt.Errorf("not a JPEG (%d bytes)", len(jpeg))
	}
	return jpeg, nil
}

// liveSem caps concurrent MJPEG live streams across the process — each holds a
// DVRIP connection open and grabs frames continuously, so bound it like the
// ffmpeg/playback pools to protect a small box.
var liveSem = make(chan struct{}, 3)

// StreamMJPEG streams a channel's live view as multipart/x-mixed-replace MJPEG
// (an <img> shows it natively — no ffmpeg, no HEVC/browser-codec problem). It
// opens ONE DVRIP session and loops the 0x11 snapshot command at ~fps, writing
// each JPEG as a multipart part. The caller sets the Content-Type header with
// the returned boundary before the first byte. It ends when ctx is cancelled
// (the handler's 5-minute cap) or when a write fails (the client navigated away
// — so leaving the page auto-stops it), and writes nothing to the box's disk.
func StreamMJPEG(ctx context.Context, w io.Writer, flush func(), host string, port int, user, pass string, channel, fps int, boundary string) error {
	select {
	case liveSem <- struct{}{}:
		defer func() { <-liveSem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if fps < 1 {
		fps = 1
	}
	if fps > 15 {
		fps = 15
	}
	c, err := Dial(net.JoinHostPort(host, dvripPort(port)), user, pass, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dahua: mjpeg %s: login: %w", host, err)
	}
	defer c.Close()
	go func() { <-ctx.Done(); c.Close() }()

	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		jpeg, err := c.snapOnce(channel)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("dahua: mjpeg %s: %w", host, err)
		}
		if _, err := fmt.Fprintf(w, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(jpeg)); err != nil {
			return nil // client gone
		}
		if _, err := w.Write(jpeg); err != nil {
			return nil
		}
		if _, err := w.Write([]byte("\r\n")); err != nil {
			return nil
		}
		if flush != nil {
			flush()
		}
	}
}

// dvripPort renders a configured DVRIP port for dialling, defaulting to the
// stock 37777 when the caller has no port on hand (0 or negative).
func dvripPort(port int) string {
	if port <= 0 {
		port = 37777
	}
	return strconv.Itoa(port)
}

// snapCommand builds the 0x11 "snapshot" request exactly as captured from the
// NetSDK: a 32-byte frame header (magic 0x11, payload length 40 at [4:8] LE,
// and a fixed 0x0a at header offset 28) plus a 40-byte payload whose only set
// byte is 0x01 at offset 24. Channel is written at payload offset 0 (0-based) as
// a best-effort — verified byte-identical for channel 0 on single-channel IPCs;
// multi-channel NVRs are untested, so callers fall back to RTSP for those.
func snapCommand(channel int) []byte {
	const payLen = 40
	b := make([]byte, headerLen+payLen)
	b[0] = 0x11
	binary.LittleEndian.PutUint32(b[4:8], payLen)
	b[28] = 0x0a
	p := b[headerLen:]
	p[0] = byte(channel)
	p[24] = 0x01
	return b
}
