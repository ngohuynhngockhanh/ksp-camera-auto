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

// QueryRecordFiles returns detailed Xiongmai RecordFile entries for [start, end],
// automatically paging when OPFileQuery returns full 64-segment batches.
func (c *Client) QueryRecordFiles(ctx context.Context, channel int, start, end time.Time) ([]RecordFile, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return nil, err
	}
	cursor := start
	seen := make(map[string]struct{})
	var all []RecordFile

	for page := 0; page < 32; page++ {
		if !cursor.Before(end) {
			break
		}
		param := map[string]any{
			"Channel":        channel,
			"BeginTime":      cursor.Format(xmTimeLayout),
			"EndTime":        end.Format(xmTimeLayout),
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
			bt, err1 := time.ParseInLocation(xmTimeLayout, bStr, start.Location())
			et, err2 := time.ParseInLocation(xmTimeLayout, eStr, start.Location())
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

// StripXMMediaFrames extracts elementary H.264/H.265 Annex-B video NAL units
// from Xiongmai's proprietary media packet wrapper (XMHead / 1424 frames):
//   - I-Frame header (16 bytes): `00 00 01 FC` (H.264) or `00 00 01 FE` (H.265 implicit)
//     followed by [type, fps, width/8, height/8, timestamp(4B), payloadLen(4B LE)]
//   - P/B-Frame header (8 bytes): `00 00 01 FD` + [reserved(2B), payloadLen(2B LE)]
//   - Audio frame header (8 bytes): `00 00 01 FA` + [type, rate, payloadLen(2B LE)] (skipped for pure video ES)
//
// If the buffer already starts with standard Annex-B `00 00 00 01` without XM
// frame markers, it is returned verbatim.
func StripXMMediaFrames(buf []byte) []byte {
	if len(buf) < 8 {
		return buf
	}
	// Quick check: if no 00 00 01 F[A-F] marker exists at offset 0, check if raw Annex-B.
	if !(buf[0] == 0x00 && buf[1] == 0x00 && buf[2] == 0x01 && buf[3] >= 0xF9) {
		idx := findXMFrameMarker(buf)
		if idx < 0 {
			return buf
		}
		buf = buf[idx:]
	}

	var out bytes.Buffer
	out.Grow(len(buf))
	pos := 0
	for pos+8 <= len(buf) {
		if !(buf[pos] == 0x00 && buf[pos+1] == 0x00 && buf[pos+2] == 0x01 && buf[pos+3] >= 0xF9) {
			next := findXMFrameMarker(buf[pos+1:])
			if next < 0 {
				out.Write(buf[pos:])
				break
			}
			pos += 1 + next
			continue
		}
		marker := buf[pos+3]
		switch marker {
		case 0xFC, 0xFE: // I-frame (16-byte header)
			if pos+16 > len(buf) {
				return out.Bytes()
			}
			frameLen := int(binary.LittleEndian.Uint32(buf[pos+12 : pos+16]))
			end := pos + 16 + frameLen
			if frameLen < 0 || end > len(buf) {
				out.Write(buf[pos+16:])
				return out.Bytes()
			}
			out.Write(buf[pos+16 : end])
			pos = end
		case 0xFD: // P-frame (8-byte header)
			frameLen := int(binary.LittleEndian.Uint16(buf[pos+6 : pos+8]))
			end := pos + 8 + frameLen
			if end > len(buf) {
				out.Write(buf[pos+8:])
				return out.Bytes()
			}
			out.Write(buf[pos+8 : end])
			pos = end
		case 0xFA, 0xF9: // Audio / Info frame (8-byte header) — skip audio to keep clean Annex-B video ES
			frameLen := int(binary.LittleEndian.Uint16(buf[pos+6 : pos+8]))
			end := pos + 8 + frameLen
			if end > len(buf) {
				return out.Bytes()
			}
			pos = end
		default:
			pos += 4
		}
	}
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
	dlErr := c.DownloadNative(ctx, rawFile, channel, start, end)
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

	fps := 25
	if si, serr := c.GetStreamInfo(ctx, channel, 0); serr == nil && si.FPS > 0 {
		fps = si.FPS
	}
	durSec := int(end.Sub(start).Seconds())
	if durSec <= 0 {
		durSec = 300
	}

	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-fflags", "+genpts",
		"-r", strconv.Itoa(fps),
		"-f", demuxFmt,
		"-i", rawPath,
		"-t", strconv.Itoa(durSec + 2),
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
