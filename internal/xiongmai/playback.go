package xiongmai

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

const xmTimeLayout = "2006-01-02 15:04:05"

// FindRecordings queries OPFileQuery in 64-entry pages and returns normalized
// dahua.Recording segments for channel (0-based).
func (c *Client) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	files, err := c.QueryRecordFiles(ctx, channel, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]dahua.Recording, 0, len(files))
	for _, f := range files {
		durSec := int(f.EndTime.Sub(f.BeginTime).Seconds())
		if durSec < 0 {
			durSec = 0
		}
		out = append(out, dahua.Recording{
			Channel:   channel,
			StartTime: f.BeginTime.Format(xmTimeLayout),
			EndTime:   f.EndTime.Format(xmTimeLayout),
			Duration:  durSec,
			FilePath:  f.FileName,
			Type:      "dav",
			Length:    f.SizeBytes,
		})
	}
	return out, nil
}

// QueryRecordFiles returns detailed Xiongmai RecordFile entries overlapping [start, end],
// automatically paging when OPFileQuery returns full 64-segment batches.
// On short sub-hour queries (e.g. 5-minute virtual NVR segment inside a 1-hour .h264 file),
// if exact [start, end] returns empty, it widens the OPFileQuery BeginTime lookback by 65m
// and filters for segments genuinely overlapping [start, end].
func (c *Client) QueryRecordFiles(ctx context.Context, channel int, start, end time.Time) ([]RecordFile, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return nil, err
	}
	files, err := c.queryRecordFilesRaw(ctx, tr, channel, start, end)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 && end.After(start) && end.Sub(start) <= 2*time.Hour {
		files, err = c.queryRecordFilesRaw(ctx, tr, channel, start.Add(-65*time.Minute), end)
		if err != nil {
			return nil, err
		}
	}
	if start.IsZero() || end.IsZero() {
		return files, nil
	}
	filtered := make([]RecordFile, 0, len(files))
	for _, f := range files {
		if f.EndTime.After(start) && f.BeginTime.Before(end) {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}

func (c *Client) queryRecordFilesRaw(ctx context.Context, tr Transport, channel int, queryStart, queryEnd time.Time) ([]RecordFile, error) {
	cursor := queryStart
	seen := make(map[string]struct{})
	var all []RecordFile

	for range 32 {
		if !cursor.Before(queryEnd) {
			break
		}
		param := map[string]any{
			"Channel":        channel,
			"BeginTime":      cursor.Format(xmTimeLayout),
			"EndTime":        queryEnd.Format(xmTimeLayout),
			"Type":           "h264",
			"Event":          "*",
			"DriverTypeMask": "0x0000FFFF",
			"StreamType":     "0x00000000",
		}
		resp, err := tr.OpDev(ctx, "OPFileQuery", param)
		if err != nil {
			if errors.Is(err, ErrNoRecordings) || isUnsupportedErr(err) {
				break
			}
			return nil, err
		}
		rawList, ok := resp["OPFileQuery"].([]any)
		if !ok || len(rawList) == 0 {
			break
		}
		var lastEnd time.Time
		addedThisPage := 0
		for _, raw := range rawList {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			fn := strings.TrimSpace(stringFieldAny(m, "FileName"))
			bStr := strings.TrimSpace(stringFieldAny(m, "BeginTime"))
			eStr := strings.TrimSpace(stringFieldAny(m, "EndTime"))
			bt, err1 := time.ParseInLocation(xmTimeLayout, bStr, queryStart.Location())
			et, err2 := time.ParseInLocation(xmTimeLayout, eStr, queryStart.Location())
			if err1 != nil || err2 != nil || !et.After(bt) {
				continue
			}
			key := fn + "|" + bStr + "|" + eStr
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			sizeKB := parseHexKB(stringFieldAny(m, "FileLength"))
			all = append(all, RecordFile{
				Channel:   channel,
				FileName:  fn,
				BeginTime: bt,
				EndTime:   et,
				SizeBytes: sizeKB * 1024,
				DiskNo:    intFieldAny(m, "DiskNo"),
				SerialNo:  intFieldAny(m, "SerialNo"),
				RawBegin:  bStr,
				RawEnd:    eStr,
			})
			lastEnd = et
			addedThisPage++
		}
		if len(rawList) < 64 || addedThisPage == 0 || !lastEnd.After(cursor) {
			break
		}
		cursor = lastEnd
	}
	return all, nil
}

func parseHexKB(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if s == "" {
		return 0
	}
	n, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0
	}
	return int64(n)
}

// ErrXMStopTimeReached signals that XMStreamExtractor has reached an I-Frame
// beyond the requested end timestamp so the caller can close the stream early.
var ErrXMStopTimeReached = errors.New("xiongmai: playback stop timestamp reached")

type xmStatsCtxKey struct{}

type xmStreamStats struct {
	fps         int
	fpsRational string
	preRollSec  float64
}

func (s *xmStreamStats) recordExtractor(ext *XMStreamExtractor) {
	if ext == nil {
		return
	}
	if r := ext.ObservedFPSRational(); r != "" && s.fpsRational == "" {
		s.fpsRational = r
	}
	if fps := ext.ObservedFPS(); fps >= 1 && fps <= 60 && s.fps == 0 {
		s.fps = fps
	}
	if s.preRollSec == 0 && ext.PreRollSeconds() > 0 {
		s.preRollSec = ext.PreRollSeconds()
	}
}

func withXMStreamStats(ctx context.Context, st *xmStreamStats) context.Context {
	return context.WithValue(ctx, xmStatsCtxKey{}, st)
}

func xmStatsFromContext(ctx context.Context) *xmStreamStats {
	st, _ := ctx.Value(xmStatsCtxKey{}).(*xmStreamStats)
	return st
}

// DecodeXMTimestamp decodes Xiongmai's 32-bit packed DATETIME from bytes [8:12]
// of an XMHead I-Frame (`00 00 01 FC` or `00 00 01 FE`).
func DecodeXMTimestamp(ts uint32, loc *time.Location) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	if loc == nil {
		loc = time.Local
	}
	sec := int(ts & 0x3F)
	min := int((ts >> 6) & 0x3F)
	hour := int((ts >> 12) & 0x1F)
	day := int((ts >> 17) & 0x1F)
	month := int((ts >> 22) & 0x0F)
	year := 2000 + int((ts>>26)&0x3F)
	if year < 2010 || month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || min > 59 || sec > 59 {
		return time.Time{}
	}
	return time.Date(year, time.Month(month), day, hour, min, sec, 0, loc)
}

// EncodeXMTimestamp packs a wall-clock time into Xiongmai's 32-bit XMHead timestamp.
func EncodeXMTimestamp(t time.Time) uint32 {
	if t.IsZero() || t.Year() < 2000 {
		return 0
	}
	yr := uint32((t.Year() - 2000) & 0x3F)
	mo := uint32(int(t.Month()) & 0x0F)
	dy := uint32(t.Day() & 0x1F)
	hr := uint32(t.Hour() & 0x1F)
	mn := uint32(t.Minute() & 0x3F)
	sc := uint32(t.Second() & 0x3F)
	return (yr << 26) | (mo << 22) | (dy << 17) | (hr << 12) | (mn << 6) | sc
}

// XMStreamExtractor stateful demuxer extracts clean H.264/H.265 Annex-B video
// across arbitrary WebSocket / Sofia TCP packet boundaries, trims pre-start GOPs
// in memory, stops immediately past end, and records the stream's true FPS.
type XMStreamExtractor struct {
	dst              io.Writer
	start            time.Time
	end              time.Time
	loc              *time.Location
	decided          bool
	rawPassthrough   bool
	buf              bytes.Buffer
	remKeep          int
	remSkip          int
	startedWriting   bool
	preGOP           bytes.Buffer
	preGOPTime       time.Time
	preGOPFrames     int
	wrote            int64
	videoFrames      int
	headerFPS        int
	firstTS          time.Time
	lastTS           time.Time
	lastSpanTS       time.Time
	framesAtLastSpan int
}

// NewXMStreamExtractor creates a stateful XMHead -> Annex-B extractor for [start, end].
func NewXMStreamExtractor(dst io.Writer, start, end time.Time) *XMStreamExtractor {
	loc := time.Local
	if !start.IsZero() {
		loc = start.Location()
	} else if !end.IsZero() {
		loc = end.Location()
	}
	return &XMStreamExtractor{
		dst:            dst,
		start:          start,
		end:            end,
		loc:            loc,
		startedWriting: start.IsZero(),
	}
}

func (e *XMStreamExtractor) WroteBytes() int64  { return e.wrote }
func (e *XMStreamExtractor) VideoFrames() int   { return e.videoFrames }
func (e *XMStreamExtractor) InFlightBytes() int { return e.remKeep }

// PreRollSeconds returns how many seconds before start the retained keyframe began.
func (e *XMStreamExtractor) PreRollSeconds() float64 {
	if !e.start.IsZero() && !e.firstTS.IsZero() && e.firstTS.Before(e.start) {
		return e.start.Sub(e.firstTS).Seconds()
	}
	return 0
}

// ObservedFPSRational returns the exact wall-clock rational frame rate "<frames>/<seconds>"
// measured across complete GOP spans [firstTS .. lastSpanTS], so cameras running at 6fps,
// 12fps, 20fps, 25fps, or night-vision VFR remux at 1.000x real-world wall-clock speed.
func (e *XMStreamExtractor) ObservedFPSRational() string {
	if !e.firstTS.IsZero() && e.lastSpanTS.After(e.firstTS) && e.framesAtLastSpan >= 4 {
		spanSec := int(e.lastSpanTS.Sub(e.firstTS).Seconds())
		if spanSec >= 4 && e.framesAtLastSpan >= spanSec && e.framesAtLastSpan <= spanSec*65 {
			return fmt.Sprintf("%d/%d", e.framesAtLastSpan, spanSec)
		}
	}
	if fps := e.ObservedFPS(); fps > 0 {
		return strconv.Itoa(fps)
	}
	return ""
}

func (e *XMStreamExtractor) ObservedFPS() int {
	if !e.firstTS.IsZero() && e.lastSpanTS.After(e.firstTS) && e.framesAtLastSpan >= 4 {
		sec := e.lastSpanTS.Sub(e.firstTS).Seconds()
		if sec >= 4 {
			est := int(float64(e.framesAtLastSpan)/sec + 0.5)
			if est >= 1 && est <= 65 {
				return est
			}
		}
	}
	if e.headerFPS >= 1 && e.headerFPS <= 65 {
		return e.headerFPS
	}
	return 0
}

func (e *XMStreamExtractor) emitVideo(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if e.startedWriting {
		n, err := e.dst.Write(b)
		e.wrote += int64(n)
		return err
	}
	const maxPreGOPBytes = 4 << 20
	if e.preGOP.Len()+len(b) <= maxPreGOPBytes {
		_, _ = e.preGOP.Write(b)
	}
	return nil
}

func (e *XMStreamExtractor) flushPreGOP() error {
	if e.preGOP.Len() == 0 {
		return nil
	}
	n, err := e.dst.Write(e.preGOP.Bytes())
	e.wrote += int64(n)
	e.videoFrames += e.preGOPFrames
	if e.firstTS.IsZero() && !e.preGOPTime.IsZero() {
		e.firstTS = e.preGOPTime
	}
	e.preGOP.Reset()
	e.preGOPFrames = 0
	return err
}

// Feed processes the next WebSocket or Sofia 1424 media chunk.
func (e *XMStreamExtractor) Feed(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	pos := 0
	// 1. Drain any in-flight video or audio/info payload spanning across chunks.
	for pos < len(chunk) && (e.remKeep > 0 || e.remSkip > 0) {
		if e.remKeep > 0 {
			take := e.remKeep
			if take > len(chunk)-pos {
				take = len(chunk) - pos
			}
			if err := e.emitVideo(chunk[pos : pos+take]); err != nil {
				return err
			}
			pos += take
			e.remKeep -= take
		}
		if pos < len(chunk) && e.remSkip > 0 {
			take := e.remSkip
			if take > len(chunk)-pos {
				take = len(chunk) - pos
			}
			pos += take
			e.remSkip -= take
		}
	}
	if pos >= len(chunk) {
		return nil
	}

	if e.rawPassthrough {
		e.startedWriting = true
		return e.emitVideo(chunk[pos:])
	}

	_, _ = e.buf.Write(chunk[pos:])
	data := e.buf.Bytes()

	if !e.decided {
		if len(data) < 8 {
			return nil
		}
		e.decided = true
		if findXMFrameMarker(data) < 0 {
			e.rawPassthrough = true
			e.startedWriting = true
			e.buf.Reset()
			return e.emitVideo(data)
		}
	}

	cur := 0
	for cur < len(data) {
		if e.remKeep > 0 {
			take := e.remKeep
			if take > len(data)-cur {
				take = len(data) - cur
			}
			if err := e.emitVideo(data[cur : cur+take]); err != nil {
				return err
			}
			cur += take
			e.remKeep -= take
			continue
		}
		if e.remSkip > 0 {
			take := e.remSkip
			if take > len(data)-cur {
				take = len(data) - cur
			}
			cur += take
			e.remSkip -= take
			continue
		}

		if len(data)-cur < 8 {
			break
		}
		if !(data[cur] == 0x00 && data[cur+1] == 0x00 && data[cur+2] == 0x01 && data[cur+3] >= 0xF9 && data[cur+3] <= 0xFE) {
			idx := findXMFrameMarker(data[cur+1:])
			if idx < 0 {
				// Keep trailing 3 bytes in case a 00 00 01 F* marker straddles the chunk end.
				keepTail := 3
				if len(data)-cur < keepTail {
					keepTail = len(data) - cur
				}
				cur = len(data) - keepTail
				break
			}
			cur += 1 + idx
			continue
		}

		marker := data[cur+3]
		switch marker {
		case 0xFC, 0xFE: // 16-byte I-frame header
			if len(data)-cur < 16 {
				goto saveRemainder
			}
			fps := int(data[cur+5])
			if fps >= 1 && fps <= 65 {
				e.headerFPS = fps
			}
			ts := DecodeXMTimestamp(binary.LittleEndian.Uint32(data[cur+8:cur+12]), e.loc)
			frameLen := int(binary.LittleEndian.Uint32(data[cur+12 : cur+16]))
			if frameLen < 0 || frameLen > maxSofiaPayloadBytes {
				cur += 4
				continue
			}
			// Check end-of-window stop condition on keyframe boundary.
			if !e.end.IsZero() && !ts.IsZero() && ts.After(e.end) {
				if !e.startedWriting && e.preGOP.Len() > 0 {
					if err := e.flushPreGOP(); err != nil {
						return err
					}
				}
				if !e.firstTS.IsZero() && ts.After(e.firstTS) && e.videoFrames > 0 {
					e.lastSpanTS = ts
					e.framesAtLastSpan = e.videoFrames
				}
				e.buf.Reset()
				return ErrXMStopTimeReached
			}
			if !e.startedWriting {
				if e.start.IsZero() || ts.IsZero() || !ts.Before(e.start) {
					// Include preceding GOP if start fell mid-GOP so frame 0 is an IDR keyframe.
					if !ts.IsZero() && e.preGOP.Len() > 0 && ts.After(e.start) && e.start.Sub(e.preGOPTime) <= 12*time.Second {
						if err := e.flushPreGOP(); err != nil {
							return err
						}
					} else {
						e.preGOP.Reset()
						e.preGOPFrames = 0
					}
					e.startedWriting = true
					if e.firstTS.IsZero() && !ts.IsZero() {
						e.firstTS = ts
					}
				} else {
					e.preGOP.Reset()
					e.preGOPFrames = 1
					e.preGOPTime = ts
				}
			}
			if e.startedWriting {
				if !ts.IsZero() {
					if e.firstTS.IsZero() {
						e.firstTS = ts
					} else if ts.After(e.firstTS) && e.videoFrames > 0 {
						e.lastSpanTS = ts
						e.framesAtLastSpan = e.videoFrames
					}
					e.lastTS = ts
				}
				e.videoFrames++
			}
			cur += 16
			e.remKeep = frameLen

		case 0xFB: // 16-byte extended P-frame header
			if len(data)-cur < 16 {
				goto saveRemainder
			}
			frameLen := int(binary.LittleEndian.Uint32(data[cur+12 : cur+16]))
			if frameLen < 0 || frameLen > maxSofiaPayloadBytes {
				cur += 4
				continue
			}
			if e.startedWriting {
				e.videoFrames++
			} else if e.preGOP.Len() > 0 || !e.preGOPTime.IsZero() {
				e.preGOPFrames++
			}
			cur += 16
			e.remKeep = frameLen

		case 0xFD: // 8-byte P-frame header: [0..3]=00 00 01 FD, [4..7]=uint32 LE payloadLen!
			frameLen := int(binary.LittleEndian.Uint32(data[cur+4 : cur+8]))
			if frameLen < 0 || frameLen > maxSofiaPayloadBytes {
				cur += 4
				continue
			}
			if e.startedWriting {
				e.videoFrames++
			} else if e.preGOP.Len() > 0 || !e.preGOPTime.IsZero() {
				e.preGOPFrames++
			}
			cur += 8
			e.remKeep = frameLen

		case 0xFA, 0xF9: // 8-byte Audio / Info header: [4]=codec/sub, [5]=rate, [6..7]=uint16 LE len
			frameLen := int(binary.LittleEndian.Uint16(data[cur+6 : cur+8]))
			cur += 8
			e.remSkip = frameLen
		default:
			cur += 4
		}
	}

saveRemainder:
	if cur >= len(data) {
		e.buf.Reset()
	} else if cur > 0 {
		rem := append([]byte(nil), data[cur:]...)
		e.buf.Reset()
		_, _ = e.buf.Write(rem)
	}
	return nil
}

// Finish flushes any buffered single-GOP or raw tail at end-of-stream.
func (e *XMStreamExtractor) Finish() error {
	if !e.decided && e.buf.Len() > 0 {
		data := append([]byte(nil), e.buf.Bytes()...)
		e.buf.Reset()
		e.decided = true
		if findXMFrameMarker(data) < 0 {
			e.rawPassthrough = true
			e.startedWriting = true
			return e.emitVideo(data)
		}
	}
	if !e.startedWriting && e.preGOP.Len() > 0 {
		e.startedWriting = true
		if err := e.flushPreGOP(); err != nil {
			return err
		}
	}
	return nil
}

// StripXMMediaFrames extracts elementary H.264/H.265 Annex-B video NAL units
// from a single buffer of Xiongmai media frames (stateless convenience wrapper).
func StripXMMediaFrames(buf []byte) []byte {
	if len(buf) == 0 {
		return buf
	}
	var out bytes.Buffer
	out.Grow(len(buf))
	ext := NewXMStreamExtractor(&out, time.Time{}, time.Time{})
	_ = ext.Feed(buf)
	_ = ext.Finish()
	return out.Bytes()
}

// DetectXMVibrantCodec inspects raw Annex-B NAL headers to distinguish H.265 vs H.264.
func DetectAnnexBCodec(es []byte) string {
	for i := 0; i+5 < len(es); i++ {
		var nalByte byte
		if es[i] == 0 && es[i+1] == 0 && es[i+2] == 1 {
			nalByte = es[i+3]
		} else if es[i] == 0 && es[i+1] == 0 && es[i+2] == 0 && es[i+3] == 1 {
			nalByte = es[i+4]
		} else {
			continue
		}
		// H.264 SPS = 7, PPS = 8, IDR = 5 (nal_unit_type = nalByte & 0x1F)
		h264Type := nalByte & 0x1F
		if h264Type == 7 || h264Type == 8 || h264Type == 5 {
			return "h264"
		}
		// H.265 VPS = 32, SPS = 33, PPS = 34, IDR = 19/20 (nal_unit_type = (nalByte >> 1) & 0x3F)
		h265Type := (nalByte >> 1) & 0x3F
		if h265Type == 32 || h265Type == 33 || h265Type == 34 || h265Type == 19 || h265Type == 20 {
			return "hevc"
		}
	}
	return "hevc"
}

func findXMFrameMarker(b []byte) int {
	for i := 0; i+4 <= len(b); i++ {
		if b[i] == 0x00 && b[i+1] == 0x00 && b[i+2] == 0x01 && b[i+3] >= 0xF9 {
			return i
		}
	}
	return -1
}

// DownloadNative writes raw Xiongmai elementary stream bytes for [start, end] to w.
func (c *Client) DownloadNative(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	files, err := c.QueryRecordFiles(ctx, channel, start, end)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return ErrNoRecordings
	}
	for _, f := range files {
		if err := tr.DownloadRecordFile(ctx, w, channel, f, start, end); err != nil {
			return err
		}
	}
	return nil
}

// ExportMP4Faststart downloads recorded video for [start, end] and remuxes into
// outMP4 with 0% CPU video transcoding (-c:v copy -an -tag:v hvc1 -movflags +faststart).
func (c *Client) ExportMP4Faststart(ctx context.Context, channel int, start, end time.Time, workDir, outMP4 string) error {
	ffPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("xiongmai export: ffmpeg not found: %w", err)
	}
	if workDir == "" {
		workDir = os.TempDir()
	}
	rawPath := filepath.Join(workDir, fmt.Sprintf("xm_raw_%d_%d.es", channel, time.Now().UnixNano()))
	defer os.Remove(rawPath)

	rawFile, err := os.Create(rawPath)
	if err != nil {
		return err
	}
	var st xmStreamStats
	dlErr := c.DownloadNative(withXMStreamStats(ctx, &st), rawFile, channel, start, end)
	_ = rawFile.Close()
	if dlErr != nil {
		return dlErr
	}

	// Sniff first 64KB to pick exact Annex-B demuxer ("hevc" vs "h264") and FPS.
	head := make([]byte, 64<<10)
	f, err := os.Open(rawPath)
	if err != nil {
		return err
	}
	n, _ := f.Read(head)
	_ = f.Close()
	if n == 0 {
		return fmt.Errorf("xiongmai export: empty elementary stream")
	}
	demuxFmt := DetectAnnexBCodec(head[:n])

	fpsArg := st.fpsRational
	if fpsArg == "" {
		fps := st.fps
		if fps <= 0 {
			if si, serr := c.GetStreamInfo(ctx, channel, 0); serr == nil && si.FPS > 0 {
				fps = si.FPS
			} else {
				fps = 25
			}
		}
		fpsArg = strconv.Itoa(fps)
	}
	durSec := int(end.Sub(start).Seconds())
	if durSec <= 0 {
		durSec = 300
	}
	extraPreRoll := int(st.preRollSec + 0.99)

	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-fflags", "+genpts",
		"-r", fpsArg,
		"-f", demuxFmt,
		"-i", rawPath,
		"-t", strconv.Itoa(durSec + extraPreRoll + 2),
		"-c:v", "copy", "-an",
	}
	if demuxFmt == "hevc" {
		args = append(args, "-tag:v", "hvc1")
	}
	args = append(args, "-movflags", "+faststart", "-f", "mp4", outMP4)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, ffPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xiongmai ffmpeg remux (%s): %w (%s)", demuxFmt, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// StreamPlaybackMP4 writes a fragmented/faststart MP4 for [start, end] to w
// (used by Camera.StreamPlayback / StreamPlaybackFast in /api/playback).
func (c *Client) StreamPlaybackMP4(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	tmpDir, err := os.MkdirTemp("", "kspcam-xm-pb-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	outMP4 := filepath.Join(tmpDir, "clip.mp4")
	if err := c.ExportMP4Faststart(ctx, channel, start, end, tmpDir, outMP4); err != nil {
		return err
	}
	f, err := os.Open(outMP4)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
