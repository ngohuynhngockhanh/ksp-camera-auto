package dahua

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
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
func GetSnapshotDVRIP(ctx context.Context, host, user, pass string, channel int, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	c, err := Dial(net.JoinHostPort(host, "37777"), user, pass, timeout)
	if err != nil {
		return nil, fmt.Errorf("dahua: snapshot dvrip %s: login: %w", host, err)
	}
	defer c.Close()
	go func() { <-ctx.Done(); c.Close() }()

	if err := c.writeRaw(snapCommand(channel)); err != nil {
		return nil, fmt.Errorf("dahua: snapshot dvrip %s: send: %w", host, err)
	}

	// The JPEG arrives as a run of 0xbc frames, ending with a zero-length one.
	// Skip any other frame types the device interleaves.
	var jpeg []byte
	for {
		hdr, payload, err := c.readFrame()
		if err != nil {
			return nil, fmt.Errorf("dahua: snapshot dvrip %s: read: %w (after %d bytes)", host, err, len(jpeg))
		}
		if hdr[0] != 0xbc {
			continue
		}
		if len(payload) == 0 {
			break
		}
		jpeg = append(jpeg, payload...)
		if len(jpeg) > 16<<20 {
			return nil, fmt.Errorf("dahua: snapshot dvrip %s: oversized", host)
		}
	}
	if len(jpeg) < 2 || jpeg[0] != 0xff || jpeg[1] != 0xd8 {
		return nil, fmt.Errorf("dahua: snapshot dvrip %s: not a JPEG (%d bytes)", host, len(jpeg))
	}
	return jpeg, nil
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
