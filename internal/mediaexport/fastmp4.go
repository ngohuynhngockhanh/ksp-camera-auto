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
	chunks := splitRange(start, end, chunkSec, urlFor)
	return fastRemux(ctx, w, chunks, remuxOpts{transcodeAudio: transcodeAudio, maxParallel: maxParallel})
}

// FastNativeRange is FastMP4Range's original-bitstream sibling: same parallel
// chunk fetch, but BOTH video and audio are stream-copied untouched into an
// MKV. Matroska is the one mainstream container that carries G.711 a-law
// (Tiandy's recorded audio) without transcoding, so the result is the closest
// thing to the device's "original file" that a pure-Go/ffmpeg path can
// produce: byte-exact HEVC + G.711 bitstreams, exact [start,end] cut, plays
// in VLC and desktop players (not in a browser <video> tag).
func FastNativeRange(ctx context.Context, w io.Writer, start, end time.Time, chunkSec int, urlFor func(cs, ce time.Time) string, maxParallel int) error {
	chunks := splitRange(start, end, chunkSec, urlFor)
	return fastRemux(ctx, w, chunks, remuxOpts{matroska: true, maxParallel: maxParallel})
}

// splitRange cuts [start,end] into chunkSec-long Chunks with URLs from urlFor.
func splitRange(start, end time.Time, chunkSec int, urlFor func(cs, ce time.Time) string) []Chunk {
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
	return chunks
}

// Chunk is one sub-range of the export: an RTSP playback URL and how many
// seconds of video it should yield (used as ffmpeg's -t bound, since some NVRs
// don't stop the RTSP stream at the URL's endtime).
type Chunk struct {
	URL     string
	Seconds int
}

// remuxOpts selects the output flavor of fastRemux.
type remuxOpts struct {
	// transcodeAudio re-encodes audio to AAC per chunk (Tiandy streams G.711
	// a-law, which MP4 can't carry with -c copy); Hikvision passes it through.
	// Ignored in matroska mode, which always copies audio untouched.
	transcodeAudio bool
	// matroska emits an MKV with both streams copied (see FastNativeRange)
	// instead of the browser-oriented MP4.
	matroska    bool
	maxParallel int
}

// fastRemux fetches chunks concurrently (bounded by opts.maxParallel),
// remuxing each RTSP playback to an intermediate container, then concatenates
// them into one file written to w: a plain MP4 (default) or an MKV
// (opts.matroska). Intermediate chunks are MPEG-TS for MP4 output and MKV for
// MKV output (TS can't carry G.711, which matroska mode must preserve).
//
// The result is built to a temp file (a plain, faststart moov tolerates the
// per-chunk timestamp resets that a fragmented moov rejects) then copied to w,
// so this is a download path, not a live stream.
// buildSem serializes whole-file builds process-wide: one build already runs
// up to 10 ffmpeg processes and holds hundreds of MB of /tmp (a ~1 GB tmpfs on
// the deploy boxes), so concurrent builds — typically a user re-clicking the
// download button — would OOM the box instead of just taking turns.
var buildSem = make(chan struct{}, 1)

func fastRemux(ctx context.Context, w io.Writer, chunks []Chunk, opts remuxOpts) error {
	if len(chunks) == 0 {
		return fmt.Errorf("mediaexport: no chunks")
	}
	select {
	case buildSem <- struct{}{}:
		defer func() { <-buildSem }()
	case <-ctx.Done():
		return fmt.Errorf("mediaexport: %w (waiting for an export slot)", ctx.Err())
	}
	if opts.maxParallel < 1 {
		opts.maxParallel = 1
	}
	dir, err := os.MkdirTemp("", "fastmp4-")
	if err != nil {
		return fmt.Errorf("mediaexport: temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	chunkExt := "ts"
	if opts.matroska {
		chunkExt = "mkv"
	}
	paths := make([]string, len(chunks))
	errs := make([]error, len(chunks))
	sem := make(chan struct{}, opts.maxParallel)
	var wg sync.WaitGroup
	for i, c := range chunks {
		paths[i] = filepath.Join(dir, fmt.Sprintf("c_%04d.%s", i, chunkExt))
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
			errs[i] = fetchChunk(ctx, c, paths[i], opts)
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
	outArgs := []string{
		"-nostdin",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-c", "copy",
		"-avoid_negative_ts", "make_zero",
	}
	if opts.matroska {
		outPath = filepath.Join(dir, "out.mkv")
		outArgs = append(outArgs, "-f", "matroska")
	} else {
		outArgs = append(outArgs,
			"-tag:v", "hvc1", // HEVC-in-MP4 for Safari/iOS; harmless for H.264
			"-movflags", "+faststart",
		)
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", append(outArgs, "-y", outPath)...)
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
	// The whole file exists before the first byte goes out, so tell a writer
	// that can forward it (the HTTP layer) the exact size — that's what makes
	// the browser's download UI show a percentage and time-remaining.
	if s, ok := w.(interface{ SetContentLength(int64) }); ok {
		if fi, err := f.Stat(); err == nil {
			s.SetContentLength(fi.Size())
		}
	}
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("mediaexport: stream result: %w", err)
	}
	return nil
}

// fetchChunk pulls one RTSP playback chunk and remuxes it to the intermediate
// container: MPEG-TS for MP4 output (the only container that concatenates
// cleanly there), MKV for matroska output (preserves G.711 audio). Video is
// always copied; audio is copied or transcoded to AAC per opts.
func fetchChunk(ctx context.Context, c Chunk, outPath string, opts remuxOpts) error {
	args := []string{"-nostdin", "-rtsp_transport", "tcp", "-i", c.URL}
	if c.Seconds > 0 {
		args = append(args, "-t", strconv.Itoa(c.Seconds))
	}
	args = append(args, "-c:v", "copy")
	if opts.transcodeAudio && !opts.matroska {
		args = append(args, "-c:a", "aac", "-b:a", "64k")
	} else {
		args = append(args, "-c:a", "copy")
	}
	if opts.matroska {
		args = append(args, "-f", "matroska")
	} else {
		args = append(args, "-f", "mpegts")
	}
	args = append(args, "-y", outPath)

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
