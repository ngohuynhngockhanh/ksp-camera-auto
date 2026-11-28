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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FastMP4Range splits [start,end] into chunkSec-long sub-ranges, builds each
// one's RTSP playback URL via urlFor, and exports them in parallel (see
// FastMP4). This is the entry point vendor adapters call.
func FastMP4Range(ctx context.Context, w io.Writer, start, end time.Time, chunkSec int, urlFor func(cs, ce time.Time) string, transcodeAudio bool, maxParallel int) error {
	chunks := splitRange(start, end, chunkSec, urlFor)
	return fastRemux(ctx, w, chunks, remuxOpts{transcodeAudio: transcodeAudio, maxParallel: maxParallel})
}

// FastMP4RangeToFile fetches RTSP playback chunks in parallel and writes the
// concatenated +faststart MP4 directly to outPath in a single ffmpeg pass
// (including inline codec tag selection and VUI range normalization).
func FastMP4RangeToFile(ctx context.Context, outPath string, start, end time.Time, chunkSec int, urlFor func(cs, ce time.Time) string, transcodeAudio bool, maxParallel int) error {
	chunks := splitRange(start, end, chunkSec, urlFor)
	return fastRemux(ctx, nil, chunks, remuxOpts{
		transcodeAudio: transcodeAudio,
		maxParallel:    maxParallel,
		outFilePath:    outPath,
	})
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
	outFilePath string
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
// CountingWriter tracks how many bytes have been written to W.
type CountingWriter struct {
	W io.Writer
	N int64
}

func (cw *CountingWriter) Write(p []byte) (int, error) {
	n, err := cw.W.Write(p)
	cw.N += int64(n)
	return n, err
}

// IsHVC1TagIncompatible reports whether ffmpeg rejected -tag:v hvc1 because the
// video stream is H.264 (avc1) or another non-HEVC codec.
func IsHVC1TagIncompatible(errText string) bool {
	low := strings.ToLower(errText)
	return strings.Contains(low, "hvc1") || strings.Contains(low, "incompatible")
}

// BuildFragmentedMP4Args builds zero-CPU stream-copy fragmented MP4 ffmpeg args
// for RTSP playback streaming.
func BuildFragmentedMP4Args(rtsp string, durSec int, mapVideo0, useHVC1Tag bool) []string {
	args := []string{"-nostdin", "-rtsp_transport", "tcp", "-i", rtsp}
	if durSec > 0 {
		args = append(args, "-t", strconv.Itoa(durSec))
	}
	if mapVideo0 {
		args = append(args, "-map", "0:v:0")
	}
	args = append(args, "-c:v", "copy", "-an")
	if useHVC1Tag {
		args = append(args, "-tag:v", "hvc1")
	}
	return append(args,
		"-fflags", "+genpts",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"-y", "pipe:1",
	)
}

type tailBuf struct {
	max int
	buf []byte
}

func (t *tailBuf) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

// StreamFragmentedMP4 pipes an RTSP playback stream through ffmpeg using
// buildArgs(true) (-tag:v hvc1 for Safari/iOS HEVC compatibility), and retries
// once with buildArgs(false) if zero bytes were emitted and ffmpeg rejects
// -tag:v hvc1 on an H.264 stream.
func StreamFragmentedMP4(ctx context.Context, w io.Writer, buildArgs func(useHVC1Tag bool) []string) error {
	cw := &CountingWriter{W: w}
	runOnce := func(useHVC1Tag bool) (string, error) {
		cmd := exec.CommandContext(ctx, "ffmpeg", buildArgs(useHVC1Tag)...)
		cmd.Stdout = cw
		stderr := &tailBuf{max: 4096}
		cmd.Stderr = stderr
		err := cmd.Run()
		return string(stderr.buf), err
	}
	errText, err := runOnce(true)
	if err != nil && cw.N == 0 && ctx.Err() == nil && IsHVC1TagIncompatible(errText) {
		retryText, retryErr := runOnce(false)
		if retryErr == nil {
			return nil
		}
		err, errText = retryErr, retryText
	}
	if err != nil {
		return fmt.Errorf("%w: %s", err, tail(errText, 300))
	}
	return nil
}

func buildConcatArgs(listPath, outPath string, matroska, dropAudio, useHVC1Tag bool) []string {
	return buildConcatArgsWithVUI(listPath, outPath, matroska, dropAudio, useHVC1Tag, "")
}

func buildConcatArgsWithVUI(listPath, outPath string, matroska, dropAudio, useHVC1Tag bool, vuiBSF string) []string {
	args := []string{
		"-nostdin",
		"-probesize", "500000", "-analyzeduration", "500000",
		"-f", "concat", "-safe", "0", "-i", listPath,
	}
	if !matroska && dropAudio {
		args = append(args, "-map", "0:v:0", "-c:v", "copy", "-an")
	} else {
		args = append(args, "-c", "copy")
	}
	if !matroska && vuiBSF != "" {
		args = append(args, "-bsf:v", vuiBSF)
	}
	args = append(args, "-avoid_negative_ts", "make_zero")
	if matroska {
		return append(args, "-f", "matroska", "-y", outPath)
	}
	if useHVC1Tag {
		// HEVC-in-MP4 for Safari/iOS; omitted on H.264.
		args = append(args, "-tag:v", "hvc1")
	}
	return append(args, "-movflags", "+faststart", "-y", outPath)
}

func probeChunkVideo(ctx context.Context, chunkPath string) (codecName, vuiBSF string) {
	pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(pctx, "ffprobe",
		"-v", "error",
		"-probesize", "500000", "-analyzeduration", "500000",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,color_range,pix_fmt",
		"-of", "json",
		chunkPath,
	).Output()
	if err != nil || len(out) == 0 {
		return "", ""
	}
	var parsed struct {
		Streams []struct {
			CodecName  string `json:"codec_name"`
			ColorRange string `json:"color_range"`
			PixFmt     string `json:"pix_fmt"`
		} `json:"streams"`
	}
	if json.Unmarshal(out, &parsed) != nil || len(parsed.Streams) == 0 {
		return "", ""
	}
	st := parsed.Streams[0]
	codecName = st.CodecName
	if st.ColorRange == "pc" || strings.Contains(st.PixFmt, "yuvj") {
		switch st.CodecName {
		case "hevc":
			vuiBSF = "hevc_metadata=video_full_range_flag=0:colour_primaries=1:transfer_characteristics=1:matrix_coefficients=1"
		case "h264":
			vuiBSF = "h264_metadata=video_full_range_flag=0:colour_primaries=1:transfer_characteristics=1:matrix_coefficients=1"
		}
	}
	return codecName, vuiBSF
}

func runConcatFallbacks(ctx context.Context, matroska bool, firstErrText string, firstErr error, attempt func(dropAudio, useHVC1Tag bool) (string, error)) (string, error) {
	if firstErr == nil || matroska || ctx.Err() != nil {
		return firstErrText, firstErr
	}
	var seq [][2]bool
	if IsHVC1TagIncompatible(firstErrText) {
		seq = [][2]bool{{false, false}, {true, false}}
	} else {
		seq = [][2]bool{{true, true}, {true, false}}
	}
	for _, cfg := range seq {
		if ctx.Err() != nil {
			break
		}
		if _, err := attempt(cfg[0], cfg[1]); err == nil {
			return "", nil
		}
	}
	return firstErrText, firstErr
}

// buildSem serializes whole-file builds process-wide: one build already runs
// up to 10 ffmpeg processes and holds hundreds of MB of /tmp (a ~1 GB tmpfs on
// the deploy boxes), so concurrent builds — typically a user re-clicking the
// download button — would OOM the box instead of just taking turns.
var buildSem = make(chan struct{}, 2)
var (
	rtspTimeoutFlagOnce sync.Once
	rtspTimeoutFlagName string
)

// detectRTSPSocketTimeoutFlag probes `ffmpeg -hide_banner -h demuxer=rtsp` once
// so RTSP chunk fetches always pass a bounded 8-second TCP read timeout (-stimeout
// on FFmpeg 4/5, -rw_timeout if present) without accidentally triggering FFmpeg 5's
// deprecated `-timeout` listen mode.
func detectRTSPSocketTimeoutFlag() string {
	rtspTimeoutFlagOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-h", "demuxer=rtsp").CombinedOutput()
		if err != nil {
			return
		}
		txt := string(out)
		if strings.Contains(txt, "-stimeout") {
			rtspTimeoutFlagName = "-stimeout"
		} else if strings.Contains(txt, "-rw_timeout") {
			rtspTimeoutFlagName = "-rw_timeout"
		}
	})
	return rtspTimeoutFlagName
}

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

	report := progressFrom(ctx)
	report(0, len(chunks), "fetch")

	chunkExt := "ts"
	if opts.matroska {
		chunkExt = "mkv"
	}
	paths := make([]string, len(chunks))
	errs := make([]error, len(chunks))
	sem := make(chan struct{}, opts.maxParallel)
	var fetched int32
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
			if errs[i] == nil {
				report(int(atomic.AddInt32(&fetched, 1)), len(chunks), "fetch")
			}
		}(i, c)
	}
	wg.Wait()
	// Sequential retry pass: if an NVR briefly rejected one of the concurrent RTSP
	// sessions (e.g. 453 Not Enough Bandwidth), retry only the failed chunks one
	// by one before aborting the export.
	for i, e := range errs {
		if e == nil || ctx.Err() != nil {
			continue
		}
		select {
		case <-time.After(400 * time.Millisecond):
		case <-ctx.Done():
			break
		}
		if retryErr := fetchChunk(ctx, chunks[i], paths[i], opts); retryErr == nil {
			errs[i] = nil
			report(int(atomic.AddInt32(&fetched, 1)), len(chunks), "fetch")
		} else {
			errs[i] = retryErr
		}
	}
	for i, e := range errs {
		if e != nil {
			return fmt.Errorf("mediaexport: chunk %d (%s): %w", i, chunkStderr(paths[i]), e)
		}
	}

	report(len(chunks), len(chunks), "concat")

	// concat list (all chunks are the same codecs, so -c copy).
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "file '%s'\n", p)
	}
	listPath := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(listPath, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("mediaexport: write list: %w", err)
	}

	outPath := opts.outFilePath
	if outPath == "" {
		outPath = filepath.Join(dir, "out.mp4")
		if opts.matroska {
			outPath = filepath.Join(dir, "out.mkv")
		}
	}
	runConcat := func(dropAudio, useHVC1Tag bool, vuiBSF string) (string, error) {
		var errBuf strings.Builder
		cmd := exec.CommandContext(ctx, "ffmpeg", buildConcatArgsWithVUI(listPath, outPath, opts.matroska, dropAudio, useHVC1Tag, vuiBSF)...)
		cmd.Stderr = &errBuf
		err := cmd.Run()
		return errBuf.String(), err
	}

	wantHVC1 := !opts.matroska
	vuiBSF := ""
	if !opts.matroska && len(paths) > 0 {
		var codecName string
		codecName, vuiBSF = probeChunkVideo(ctx, paths[0])
		if codecName != "" && codecName != "hevc" {
			wantHVC1 = false
		}
	}

	errText, err := runConcat(false, wantHVC1, vuiBSF)
	if err != nil && ctx.Err() == nil && vuiBSF != "" {
		errText, err = runConcat(false, wantHVC1, "")
	}
	errText, err = runConcatFallbacks(ctx, opts.matroska, errText, err, func(dropAudio, useHVC1Tag bool) (string, error) {
		return runConcat(dropAudio, useHVC1Tag, "")
	})
	if err != nil {
		if opts.outFilePath != "" {
			_ = os.Remove(outPath)
		}
		return fmt.Errorf("mediaexport: concat: %w: %s", err, tail(errText, 300))
	}

	if w == nil {
		report(len(chunks), len(chunks), "send")
		return nil
	}

	f, err := os.Open(outPath)
	if err != nil {
		return fmt.Errorf("mediaexport: open result: %w", err)
	}
	defer f.Close()
	report(len(chunks), len(chunks), "send")
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
// container with STRICT ZERO-CPU ENCODE (-c:v copy, never libx264/aac encode).
// Audio is copied untouched (-c:a copy); if the container muxer rejects a
// non-AAC audio stream (e.g. G.711 pcm_alaw/pcm_mulaw in MPEG-TS), fetchChunk
// automatically retries once with -an (0% CPU) so video export succeeds cleanly.
func fetchChunk(ctx context.Context, c Chunk, outPath string, opts remuxOpts) error {
	runOnce := func(dropAudio bool, useFastProxy bool) error {
		inputURL := c.URL
		var cleanup func()
		if useFastProxy {
			if pURL, cleanFn, ok := startHikFastRTSPProxy(ctx, c.URL); ok {
				inputURL = pURL
				cleanup = cleanFn
			}
		}
		if cleanup != nil {
			defer cleanup()
		}
		args := []string{"-nostdin"}
		if tf := detectRTSPSocketTimeoutFlag(); tf != "" {
			args = append(args, tf, "8000000")
		}
		args = append(args, "-rtsp_transport", "tcp", "-i", inputURL)
		if c.Seconds > 0 {
			args = append(args, "-t", strconv.Itoa(c.Seconds))
		}
		args = append(args, "-c:v", "copy")
		if dropAudio || (opts.transcodeAudio && !opts.matroska) {
			args = append(args, "-an")
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
	if err := runOnce(false, false); err != nil {
		if !opts.matroska && !opts.transcodeAudio && ctx.Err() == nil {
			if errNoAudio := runOnce(true, false); errNoAudio == nil {
				return nil
			}
		}
		return err
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
