package tiandy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
)

// StreamMJPEG decodes the Tiandy RTSP sub-stream to a multipart/x-mixed-replace
// MJPEG feed (same contract as dahua.StreamMJPEG, so handleLive and the web
// <img> live view are unchanged). Tiandy exposes no still-JPEG HTTP endpoint, so
// unlike Dahua's native snapshot loop this transcodes H.265 → MJPEG with ffmpeg
// — heavier on CPU, hence the sub-stream and a capped fps.
//
// channel is vendor-neutral (0-based); it maps to Tiandy's 1-based RTSP channel.
func StreamMJPEG(ctx context.Context, w io.Writer, flush func(), host, user, pass string, channel, fps int, boundary string) error {
	if fps < 1 {
		fps = 6
	}
	if fps > 12 {
		fps = 12 // cap: H.265→MJPEG transcode is expensive on the ARM box
	}
	rtsp := liveRTSPURL(host, user, pass, tiandyChannel(channel), 1) // subtype=1 (sub stream)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-rtsp_transport", "tcp",
		"-i", rtsp,
		"-an",
		"-vf", fmt.Sprintf("fps=%d", fps),
		"-q:v", "6",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr := &tailBuf{max: 4096}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("tiandy: live %s: %w", host, err)
	}
	defer func() { _ = cmd.Wait() }()

	br := bufio.NewReaderSize(stdout, 1<<20)
	for {
		frame, err := readJPEGFrame(br)
		if err != nil {
			if ctx.Err() != nil {
				return nil // client disconnected / session cap reached
			}
			return fmt.Errorf("tiandy: live %s: %w: %s", host, err, tail(stderr.buf, 200))
		}
		if _, err := fmt.Fprintf(w, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(frame)); err != nil {
			return nil
		}
		if _, err := w.Write(frame); err != nil {
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

// readJPEGFrame reads one complete JPEG (SOI 0xFFD8 … EOI 0xFFD9) from the
// concatenated image2pipe stream.
func readJPEGFrame(br *bufio.Reader) ([]byte, error) {
	out := make([]byte, 0, 64*1024)
	// Sync to the start-of-image marker.
	for {
		b, err := br.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != 0xFF {
			continue
		}
		b2, err := br.ReadByte()
		if err != nil {
			return nil, err
		}
		if b2 == 0xD8 {
			out = append(out, 0xFF, 0xD8)
			break
		}
	}
	// Read until the end-of-image marker.
	for {
		b, err := br.ReadByte()
		if err != nil {
			return nil, err
		}
		out = append(out, b)
		if b != 0xFF {
			continue
		}
		b2, err := br.ReadByte()
		if err != nil {
			return nil, err
		}
		out = append(out, b2)
		if b2 == 0xD9 {
			return out, nil
		}
	}
}
