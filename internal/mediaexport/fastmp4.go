// Package mediaexport builds a fast MP4 export of a recording by fetching
// several RTSP playback chunks CONCURRENTLY and concatenating them. NVRs that
// pace RTSP playback at ~1x realtime (Hikvision, Tiandy — verified: neither
// honors "Rate-Control: no", and neither exposes a pure-Go byte-download-by-time
// API) can't stream a clip faster than realtime on a single session, but they
// DO allow many concurrent playback sessions on one channel (verified: 5
// parallel sessions download 5 minutes of video in ~61s). So splitting a clip
// into K sub-ranges fetched in parallel yields a ~K× wall-clock speedup while
// keeping an exact-cut, browser-playable MP4.
package mediaexport

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FastMP4Range splits [start,end] into chunkSec-long sub-ranges, builds each
// one's RTSP playback URL via urlFor, and exports them in parallel (see
// FastMP4). This is the entry point vendor adapters call.
func FastMP4Range(ctx context.Context, w io.Writer, start, end time.Time, chunkSec int, urlFor func(cs, ce time.Time) string, transcodeAudio bool, maxParallel int) error {
	if chunkSec < 1 {
		chunkSec = 60
	}
	step := time.Duration(chunkSec) * time.Second
	var chunks []Chunk
	for t := start; t.Before(end); t = t.Add(step) {
		ce := t.Add(step)
		if ce.After(end) {
			ce = end
		}
		chunks = append(chunks, Chunk{URL: urlFor(t, ce), Seconds: int(ce.Sub(t).Seconds() + 0.5)})
	}
	return FastMP4(ctx, w, chunks, transcodeAudio, maxParallel)
}

// Chunk is one sub-range of the export: an RTSP playback URL and how many
// seconds of video it should yield (used as ffmpeg's -t bound, since some NVRs
// don't stop the RTSP stream at the URL's endtime).
type Chunk struct {
	URL     string
	Seconds int
}

// FastMP4 fetches chunks concurrently (bounded by maxParallel), remuxing each
// RTSP playback to MPEG-TS, then concatenates them into one plain MP4 written to
// w. transcodeAudio re-encodes audio to AAC per chunk (Tiandy streams G.711
// a-law, which MP4 can't carry with -c copy); Hikvision passes it through.
//
// The MP4 is built to a temp file (a plain, faststart moov tolerates the
// per-chunk timestamp resets that a fragmented moov rejects) then copied to w,
// so this is a download path, not a live stream.
func FastMP4(ctx context.Context, w io.Writer, chunks []Chunk, transcodeAudio bool, maxParallel int) error {
	if len(chunks) == 0 {
		return fmt.Errorf("mediaexport: no chunks")
	}
	if maxParallel < 1 {
		maxParallel = 1
	}
	dir, err := os.MkdirTemp("", "fastmp4-")
	if err != nil {
		return fmt.Errorf("mediaexport: temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	paths := make([]string, len(chunks))
	errs := make([]error, len(chunks))
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i, c := range chunks {
		paths[i] = filepath.Join(dir, fmt.Sprintf("c_%04d.ts", i))
		wg.Add(1)
		go func(i int, c Chunk) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errs[i] = ctx.Err()
				return
			}
			errs[i] = fetchChunkTS(ctx, c, paths[i], transcodeAudio)
		}(i, c)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			return fmt.Errorf("mediaexport: chunk %d (%s): %w", i, chunkStderr(paths[i]), e)
		}
	}

	// concat list (all chunks are the same codecs, so -c copy).
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "file '%s'\n", p)
	}
	listPath := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(listPath, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("mediaexport: write list: %w", err)
	}

	outPath := filepath.Join(dir, "out.mp4")
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-c", "copy",
		"-tag:v", "hvc1", // HEVC-in-MP4 for Safari/iOS; harmless for H.264
		"-avoid_negative_ts", "make_zero",
		"-movflags", "+faststart",
		"-y", outPath,
	)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mediaexport: concat: %w: %s", err, tail(errBuf.String(), 300))
	}

	f, err := os.Open(outPath)
	if err != nil {
		return fmt.Errorf("mediaexport: open result: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("mediaexport: stream result: %w", err)
	}
	return nil
}

// fetchChunkTS pulls one RTSP playback chunk and remuxes it to MPEG-TS (the
// only container that concatenates cleanly). Video is copied; audio is copied
// or transcoded to AAC.
func fetchChunkTS(ctx context.Context, c Chunk, outPath string, transcodeAudio bool) error {
	args := []string{"-nostdin", "-rtsp_transport", "tcp", "-i", c.URL}
	if c.Seconds > 0 {
		args = append(args, "-t", strconv.Itoa(c.Seconds))
	}
	args = append(args, "-c:v", "copy")
	if transcodeAudio {
		args = append(args, "-c:a", "aac", "-b:a", "64k")
	} else {
		args = append(args, "-c:a", "copy")
	}
	args = append(args, "-f", "mpegts", "-y", outPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		_ = os.WriteFile(outPath+".err", []byte(tail(errBuf.String(), 400)), 0o600)
		return err
	}
	if fi, err := os.Stat(outPath); err != nil || fi.Size() == 0 {
		return fmt.Errorf("empty chunk")
	}
	return nil
}

// chunkStderr returns a short tail of a failed chunk's captured ffmpeg stderr.
func chunkStderr(tsPath string) string {
	b, err := os.ReadFile(tsPath + ".err")
	if err != nil {
		return ""
	}
	return string(b)
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
