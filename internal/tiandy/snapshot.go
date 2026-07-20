package tiandy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Snapshot grabs a single JPEG frame for a channel by pulling one frame off the
// Tiandy sub-stream over RTSP with ffmpeg. Tiandy has no pure-Go still-image
// CGI reachable with the web-admin creds, so this decodes one live frame
// instead. channel is vendor-neutral (0-based); stream is ignored (always the
// low-res sub-stream, which is cheapest to open for a thumbnail).
func (c *Client) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	rtsp := liveRTSPURL(c.host, c.user, c.pass, tiandyChannel(channel), 1)
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-rtsp_transport", "tcp",
		"-i", rtsp,
		"-frames:v", "1",
		"-f", "image2",
		"-c:v", "mjpeg",
		"-y", "pipe:1",
	)
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tiandy: snapshot %s: %w: %s", c.host, err, tail(errb.Bytes(), 300))
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("tiandy: snapshot %s: empty frame", c.host)
	}
	return out.Bytes(), nil
}
