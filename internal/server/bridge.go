package server

import (
	"context"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/mediaexport"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/xiongmai"
)

var (
	virtualNVRFileRe = regexp.MustCompile(`^v_nvr_(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2})_(\d+)\.mp4$`)
	cameraMidNumRe   = regexp.MustCompile(`(?i)^camera0*(\d+)$`)
)

type recCacheEntry struct {
	fetchedAt time.Time
	list      []dahua.Recording
}

type flightCall struct {
	wg  sync.WaitGroup
	val any
	err error
}

type bridgeSingleflight struct {
	mu sync.Mutex
	m  map[string]*flightCall
}

func (g *bridgeSingleflight) Do(key string, fn func() (any, error)) (any, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*flightCall)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(flightCall)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}

// nvrBridge implements the Zero-CPU-Encode NVR / Cam-SD Playback Bridge (:2028)
// for go-shinobi-ksp (:8080) and redbida-go (:2023):
//   - Protected by X-Kspcam-Key (or loopback 127.0.0.1 / ::1 when key unset).
//   - Per-host concurrency semaphores (max 2 concurrent pulls per NVR IP;
//     strictly 1 sequential pull on standalone SD-card cameras).
//   - 100% pure packet stream-copy (-c:v copy, -c:a copy / -an, -tag:v hvc1,
//     -movflags +faststart) — NEVER runs CPU video/audio transcoding.
//   - Bounded LRU + TTL RAM Spool (/dev/shm/kspcam-clips) serving HTTP 206 Range.
type nvrBridge struct {
	s          *Server
	spoolDir   string
	maxSpoolB  int64
	clipTTL    time.Duration
	maxPerNVR  int
	lowRAMBox  bool
	stopCh     chan struct{}
	recGroup   bridgeSingleflight
	clipGroup  bridgeSingleflight
	recMu      sync.RWMutex
	recCache   map[string]recCacheEntry
	hostSemMu  sync.Mutex
	hostSems   map[string]chan struct{}
	activeMu   sync.Mutex
	activePins map[string]int
	rtspPortMu sync.Mutex
	rtspPorts  map[string]int
}

func detectSystemRAMMB() int64 {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 2048
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil && kb > 0 {
					return kb / 1024
				}
			}
		}
	}
	return 2048
}

func resolveExecutable(absPath, fallback string) string {
	if st, err := os.Stat(absPath); err == nil && !st.IsDir() {
		return absPath
	}
	return fallback
}

func newNVRBridge(s *Server) *nvrBridge {
	ramMB := detectSystemRAMMB()
	lowRAM := ramMB <= 1350

	spoolDir := strings.TrimSpace(s.cfg.Bridge.SpoolDir)
	if spoolDir == "" {
		spoolDir = "/dev/shm/kspcam-clips"
	}
	if err := os.MkdirAll(spoolDir, 0o750); err != nil {
		spoolDir = filepath.Join(os.TempDir(), "kspcam-clips")
		_ = os.MkdirAll(spoolDir, 0o750)
	}

	maxMB := s.cfg.Bridge.MaxSpoolMB
	if maxMB <= 0 {
		if lowRAM {
			maxMB = 120
		} else {
			maxMB = 240
		}
	}
	ttlSec := s.cfg.Bridge.ClipTTLSeconds
	if ttlSec <= 0 {
		ttlSec = 600
	}
	maxPerNVR := s.cfg.Bridge.MaxPerNVR
	if maxPerNVR <= 0 {
		maxPerNVR = 2
	}

	b := &nvrBridge{
		s:          s,
		spoolDir:   spoolDir,
		maxSpoolB:  int64(maxMB) << 20,
		clipTTL:    time.Duration(ttlSec) * time.Second,
		maxPerNVR:  maxPerNVR,
		lowRAMBox:  lowRAM,
		stopCh:     make(chan struct{}),
		recCache:   make(map[string]recCacheEntry),
		hostSems:   make(map[string]chan struct{}),
		activePins: make(map[string]int),
		rtspPorts:  make(map[string]int),
	}
	b.cleanStaleWorkDirs()
	b.evictSpool(0)
	go b.janitorLoop()
	return b
}

func (b *nvrBridge) Close() {
	select {
	case <-b.stopCh:
	default:
		close(b.stopCh)
	}
}

func (b *nvrBridge) janitorLoop() {
	tk := time.NewTicker(45 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-b.stopCh:
			return
		case <-tk.C:
			b.evictSpool(0)
		}
	}
}

func (b *nvrBridge) cleanStaleWorkDirs() {
	entries, err := os.ReadDir(b.spoolDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".work-") || strings.HasSuffix(e.Name(), ".tmp") {
			_ = os.RemoveAll(filepath.Join(b.spoolDir, e.Name()))
		}
	}
}

func (b *nvrBridge) pinFile(path string) func() {
	b.activeMu.Lock()
	b.activePins[path]++
	b.activeMu.Unlock()
	return func() {
		b.activeMu.Lock()
		b.activePins[path]--
		if b.activePins[path] <= 0 {
			delete(b.activePins, path)
		}
		b.activeMu.Unlock()
	}
}

func (b *nvrBridge) isPinned(path string) bool {
	b.activeMu.Lock()
	defer b.activeMu.Unlock()
	return b.activePins[path] > 0
}

// evictSpool enforces ClipTTL and LRU byte cap (maxSpoolB - reserveBytes).
func (b *nvrBridge) evictSpool(reserveBytes int64) int64 {
	entries, err := os.ReadDir(b.spoolDir)
	if err != nil {
		return 0
	}
	type item struct {
		path    string
		size    int64
		modTime time.Time
	}
	now := time.Now()
	var items []item
	var total int64

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".mp4") {
			continue
		}
		full := filepath.Join(b.spoolDir, name)
		fi, err := os.Stat(full)
		if err != nil {
			continue
		}
		if now.Sub(fi.ModTime()) > b.clipTTL && !b.isPinned(full) {
			_ = os.Remove(full)
			continue
		}
		items = append(items, item{path: full, size: fi.Size(), modTime: fi.ModTime()})
		total += fi.Size()
	}

	budget := b.maxSpoolB - reserveBytes
	if budget < 24<<20 {
		budget = 24 << 20
	}
	if total <= budget {
		return total
	}

	// Evict oldest modified files first.
	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime.Before(items[j].modTime)
	})
	for _, it := range items {
		if total <= budget {
			break
		}
		if b.isPinned(it.path) {
			continue
		}
		if err := os.Remove(it.path); err == nil {
			total -= it.size
		}
	}
	return total
}

func (b *nvrBridge) acquireHostSlot(ctx context.Context, host string, isCamSD bool) (func(), error) {
	capSlots := b.maxPerNVR
	if isCamSD || capSlots < 1 {
		capSlots = 1 // Standalone camera SD cards MUST run sequentially (1 stream)
	}
	key := fmt.Sprintf("%s#%d", host, capSlots)

	b.hostSemMu.Lock()
	sem, ok := b.hostSems[key]
	if !ok {
		sem = make(chan struct{}, capSlots)
		b.hostSems[key] = sem
	}
	b.hostSemMu.Unlock()

	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("bridge: waiting for NVR/Cam slot %s: %w", host, ctx.Err())
	}
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func extractBridgeKey(r *http.Request) string {
	if k := strings.TrimSpace(r.Header.Get("X-Kspcam-Key")); k != "" {
		return k
	}
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(r.URL.Query().Get("key"))
}

func constantMatch(a, b string) bool {
	return a != "" && b != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// authorizeBridge validates either an admin session, a configured X-Kspcam-Key
// (or shared Shinobi 127.0.0.1 admin key), or unauthenticated loopback when no
// explicit bridge.api_key is set.
func (s *Server) authorizeBridge(r *http.Request) bool {
	if !s.cfg.Bridge.Enabled {
		return false
	}
	if s.authed(r) {
		return true
	}
	provided := extractBridgeKey(r)
	cfgKey := strings.TrimSpace(s.cfg.Bridge.APIKey)
	shinobiKey := strings.TrimSpace(s.cfg.Shinobi.APIKey)

	if cfgKey != "" {
		return constantMatch(provided, cfgKey) || (isLoopbackRequest(r) && constantMatch(provided, shinobiKey))
	}
	if provided != "" && (constantMatch(provided, shinobiKey) || constantMatch(provided, strings.TrimSpace(s.cfg.MCP.APIKey))) {
		return true
	}
	return isLoopbackRequest(r)
}

type resolvedBridgeTarget struct {
	Mid     string
	Cam     config.Device
	Source  config.Device
	Channel int // 0-based
	IsCamSD bool
}

// resolveBridgeTarget maps ?id= or ?mid= (e.g. camera01..camera30) to the
// underlying recording source (linked NVR channel or standalone SD camera).
func (s *Server) resolveBridgeTarget(idParam, midParam string, reqChannel int, hasExplicitCh bool) (resolvedBridgeTarget, bool) {
	all := s.inv.List()
	var cam config.Device
	found := false

	idParam = strings.TrimSpace(idParam)
	midParam = strings.TrimSpace(midParam)

	if idParam != "" {
		if d, ok := s.inv.Get(idParam); ok {
			cam = d
			found = true
		}
	}
	if !found && midParam != "" {
		for _, d := range all {
			if d.IsNVR {
				continue
			}
			firstTok := ""
			if fields := strings.Fields(d.Name); len(fields) > 0 {
				firstTok = fields[0]
			}
			if strings.EqualFold(shinobi.DeviceToMid(d), midParam) ||
				strings.EqualFold(strings.TrimSpace(d.Name), midParam) ||
				(firstTok != "" && strings.EqualFold(firstTok, midParam)) ||
				strings.EqualFold(d.ID, midParam) {
				cam = d
				found = true
				break
			}
		}
		// Fallback: camera0N -> Nth non-NVR camera in inventory order.
		if !found {
			if m := cameraMidNumRe.FindStringSubmatch(midParam); len(m) == 2 {
				if idx, err := strconv.Atoi(m[1]); err == nil && idx >= 1 {
					nonNVR := make([]config.Device, 0, len(all))
					for _, d := range all {
						if !d.IsNVR {
							nonNVR = append(nonNVR, d)
						}
					}
					if idx <= len(nonNVR) {
						cam = nonNVR[idx-1]
						found = true
					}
				}
			}
		}
	}
	if !found {
		return resolvedBridgeTarget{}, false
	}

	mid := midParam
	if mid == "" {
		mid = shinobi.DeviceToMid(cam)
	}

	// Determine recording source:
	// 1. Linked NVR (either NoStorage=true or NVRID explicitly configured)
	if cam.NVRID != "" {
		if nvr, ok := s.inv.Get(cam.NVRID); ok {
			ch := cam.NVRChannel - 1
			if ch < 0 {
				ch = 0
			}
			if hasExplicitCh {
				ch = reqChannel
			}
			return resolvedBridgeTarget{
				Mid:     mid,
				Cam:     cam,
				Source:  nvr,
				Channel: ch,
				IsCamSD: false,
			}, true
		}
	}

	// 2. Direct NVR device entry
	if cam.IsNVR {
		ch := reqChannel
		if !hasExplicitCh && cam.NVRChannel > 0 {
			ch = cam.NVRChannel - 1
		}
		return resolvedBridgeTarget{
			Mid:     mid,
			Cam:     cam,
			Source:  cam,
			Channel: ch,
			IsCamSD: false,
		}, true
	}

	// 3. Auto-match against an NVR in inventory by camera IP if camera has no NVRID yet!
	for _, cand := range all {
		if cand.IsNVR && cand.Host == cam.Host {
			ch := reqChannel
			if !hasExplicitCh && cam.NVRChannel > 0 {
				ch = cam.NVRChannel - 1
			}
			return resolvedBridgeTarget{
				Mid:     mid,
				Cam:     cam,
				Source:  cand,
				Channel: ch,
				IsCamSD: false,
			}, true
		}
	}

	// 4. Multi-channel NVR entry imported per-channel (e.g. 192.168.1.50:82-c31..c42)
	//    vs standalone single-channel IP camera with onboard SD card ([Cam-SD]).
	ch := reqChannel
	if !hasExplicitCh && cam.NVRChannel >= 1 {
		ch = cam.NVRChannel - 1
	}
	sameHostPeers := 0
	for _, cand := range all {
		if cand.Host != "" && cand.Host == cam.Host {
			sameHostPeers++
		}
	}
	isMultiChNVR := cam.NVRChannel > 1 || sameHostPeers > 1
	return resolvedBridgeTarget{
		Mid:     mid,
		Cam:     cam,
		Source:  cam,
		Channel: ch,
		IsCamSD: !isMultiChNVR,
	}, true
}

// parseBridgeTime parses either RFC3339 (converting to local device wall-clock)
// or bare local wall-clock layouts ("2006-01-02T15:04:05", "2006-01-02 15:04:05").
func parseBridgeTime(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, fmt.Errorf("missing timestamp")
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.In(time.Local), nil
		}
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15-04-05",
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.In(time.Local), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", raw)
}

// ParseVirtualNVRFilename parses "v_nvr_2026-11-08T17-35-00_300.mp4" into local [start, end].
func ParseVirtualNVRFilename(filename string) (time.Time, time.Time, bool) {
	m := virtualNVRFileRe.FindStringSubmatch(strings.TrimSpace(filepath.Base(filename)))
	if len(m) != 3 {
		return time.Time{}, time.Time{}, false
	}
	st, err := time.ParseInLocation("2006-01-02T15-04-05", m[1], time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	durSec, err := strconv.Atoi(m[2])
	if err != nil || durSec <= 0 || durSec > 7200 {
		return time.Time{}, time.Time{}, false
	}
	return st, st.Add(time.Duration(durSec) * time.Second), true
}

func (b *nvrBridge) FlushRecordingsCache() {
	b.recMu.Lock()
	b.recCache = make(map[string]recCacheEntry)
	b.recMu.Unlock()
}

func (b *nvrBridge) getRecordingsCached(ctx context.Context, tgt resolvedBridgeTarget, start, end time.Time, forceFresh bool) ([]dahua.Recording, error) {
	// Historical Hybrid tail queries (ending >2m in the past) change at most once
	// per 5-minute segment rollover: quantize to 300s buckets with a 3-minute RAM TTL
	// so 600..1200-segment Imou [Cam-SD] cards load in <1ms on subsequent tab opens!
	// Live Diskless queries (ending near now) use 15s buckets & 15s TTL, and CheckVAR
	// flushes recCache immediately via FlushRecordingsCache().
	isHistoricalTail := time.Since(end) > 2*time.Minute
	bucketSec := int64(15)
	ttl := 15 * time.Second
	if isHistoricalTail {
		bucketSec = 300
		ttl = 3 * time.Minute
	}
	cacheKey := fmt.Sprintf("%s|%d|%d|%d", tgt.Source.ID, tgt.Channel, start.Unix()/bucketSec, end.Unix()/bucketSec)
	if !forceFresh {
		b.recMu.RLock()
		if ent, ok := b.recCache[cacheKey]; ok && time.Since(ent.fetchedAt) < ttl {
			b.recMu.RUnlock()
			return ent.list, nil
		}
		b.recMu.RUnlock()
	}

	val, err := b.recGroup.Do(cacheKey, func() (any, error) {
		to := b.s.reqTimeout(30)
		if to < 30*time.Second {
			to = 30 * time.Second
		}
		cctx, cancel := context.WithTimeout(context.Background(), to)
		defer cancel()
		release, slotErr := b.acquireHostSlot(cctx, tgt.Source.Host, tgt.IsCamSD)
		if slotErr != nil {
			return nil, slotErr
		}
		defer release()

		if tgt.Source.Vendor == config.VendorDahua && !tgt.IsCamSD {
			cgiCtx, cgiCancel := context.WithTimeout(cctx, 12*time.Second)
			cgiList, cgiErr := dahua.FindRecordingsCGI(cgiCtx, tgt.Source.Host, tgt.Source.Port, tgt.Source.Username, tgt.Source.Password, tgt.Channel, start, end)
			cgiCancel()
			if cgiErr == nil {
				b.recMu.Lock()
				b.recCache[cacheKey] = recCacheEntry{fetchedAt: time.Now(), list: cgiList}
				b.recMu.Unlock()
				return cgiList, nil
			}
		}

		cam, err := camera.Open(cctx, tgt.Source, to)
		if err != nil {
			return nil, err
		}
		defer cam.Close()
		rec, ok := cam.(camera.Recorder)
		if !ok {
			return nil, fmt.Errorf("device %s (%s) does not support recordings", tgt.Source.ID, tgt.Source.Vendor)
		}
		list, err := rec.FindRecordings(cctx, tgt.Channel, start, end)
		if err != nil {
			return nil, err
		}
		b.recMu.Lock()
		b.recCache[cacheKey] = recCacheEntry{fetchedAt: time.Now(), list: list}
		b.recMu.Unlock()
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	list, _ := val.([]dahua.Recording)
	return list, nil
}

func (b *nvrBridge) maxClipDuration() time.Duration {
	if b.lowRAMBox {
		return 10 * time.Minute
	}
	return 20 * time.Minute
}

func clipSpoolKey(srcID string, ch int, start, end time.Time) string {
	raw := fmt.Sprintf("%s|%d|%d|%d", srcID, ch, start.Unix(), end.Unix())
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:12]) + ".mp4"
}

// remuxCopyFileToFaststartMP4 remuxes an input media file (e.g. .dav DHAV or
// Hikvision IMKH MPEG-PS) into a browser/iOS-ready MP4 (+faststart, hvc1)
// using STRICT 100% STREAM COPY (-c:v copy, -c:a copy with -an fallback).
// Zero video/audio CPU encoding is ever performed.
func remuxCopyFileToFaststartMP4(ctx context.Context, inputFmt, inPath, outPath string, ssOffset, durSec float64) error {
	ffmpegBin := resolveExecutable("/usr/bin/ffmpeg", "ffmpeg")
	runRemux := func(dropAudio, useHVC1Tag bool) error {
		var args []string
		args = append(args, "-nostdin", "-hide_banner", "-loglevel", "error", "-fflags", "+genpts+igndts")
		if inputFmt != "" {
			args = append(args, "-f", inputFmt)
		}
		if ssOffset > 0.25 {
			args = append(args, "-ss", fmt.Sprintf("%.3f", ssOffset))
		}
		args = append(args, "-i", inPath)
		if durSec > 0 {
			args = append(args, "-t", fmt.Sprintf("%.3f", durSec+0.8))
		}
		args = append(args, "-map", "0:v:0", "-c:v", "copy")
		if useHVC1Tag {
			args = append(args, "-tag:v", "hvc1")
		}
		if dropAudio {
			args = append(args, "-an")
		} else {
			args = append(args, "-map", "0:a:0?", "-c:a", "copy")
		}
		args = append(args,
			"-avoid_negative_ts", "make_zero",
			"-movflags", "+faststart",
			"-y", outPath,
		)
		cmd := exec.CommandContext(ctx, ffmpegBin, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
		}
		fi, statErr := os.Stat(outPath)
		if statErr != nil || fi.Size() < 1024 {
			return fmt.Errorf("remux output empty")
		}
		return nil
	}

	err := runRemux(false, true)
	if err != nil {
		if ctx.Err() != nil {
			return err
		}
		// If source video is H.264 (avc1), FFmpeg rejects -tag:v hvc1; retry without hvc1 tag.
		if strings.Contains(err.Error(), "hvc1") || strings.Contains(err.Error(), "incompatible") {
			if errH264 := runRemux(false, false); errH264 == nil {
				return normalizeFullRangeMetadataZeroCPU(ctx, outPath)
			}
			if errH264NoAud := runRemux(true, false); errH264NoAud == nil {
				return normalizeFullRangeMetadataZeroCPU(ctx, outPath)
			}
		}
		// Incompatible recorded audio (e.g. G.711 pcm_alaw/pcm_mulaw in MP4):
		// retry with -an (0% CPU) instead of running CPU audio transcoding.
		if errNoAudio := runRemux(true, true); errNoAudio == nil {
			return normalizeFullRangeMetadataZeroCPU(ctx, outPath)
		}
		if errNoAudioH264 := runRemux(true, false); errNoAudioH264 == nil {
			return normalizeFullRangeMetadataZeroCPU(ctx, outPath)
		}
		return err
	}
	return normalizeFullRangeMetadataZeroCPU(ctx, outPath)
}

// normalizeFullRangeMetadataZeroCPU inspects VUI color_range via ffprobe and,
// ONLY if full-range ("pc" / yuvj420p) is detected, rewrites the SPS VUI header
// flags via bitstream filter (-c copy, 0% decode/encode CPU) so iPhone OLED
// displays accurate contrast without washed-out highlights.
func normalizeFullRangeMetadataZeroCPU(ctx context.Context, mp4Path string) error {
	ffprobeBin := resolveExecutable("/usr/bin/ffprobe", "ffprobe")
	pctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	out, err := exec.CommandContext(pctx, ffprobeBin,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,color_range,pix_fmt",
		"-of", "json",
		mp4Path,
	).Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	var parsed struct {
		Streams []struct {
			CodecName  string `json:"codec_name"`
			ColorRange string `json:"color_range"`
			PixFmt     string `json:"pix_fmt"`
		} `json:"streams"`
	}
	if json.Unmarshal(out, &parsed) != nil || len(parsed.Streams) == 0 {
		return nil
	}
	st := parsed.Streams[0]
	if st.ColorRange != "pc" && !strings.Contains(st.PixFmt, "yuvj") {
		return nil // Already standard TV range — nothing to touch.
	}

	var bsf string
	switch st.CodecName {
	case "hevc":
		bsf = "hevc_metadata=video_full_range_flag=0:colour_primaries=1:transfer_characteristics=1:matrix_coefficients=1"
	case "h264":
		bsf = "h264_metadata=video_full_range_flag=0:colour_primaries=1:transfer_characteristics=1:matrix_coefficients=1"
	default:
		return nil
	}

	ffmpegBin := resolveExecutable("/usr/bin/ffmpeg", "ffmpeg")
	fixedPath := mp4Path + ".vui.mp4"
	cmd := exec.CommandContext(ctx, ffmpegBin,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", mp4Path,
		"-map", "0:v:0", "-map", "0:a:0?",
		"-c", "copy",
		"-bsf:v", bsf,
		"-tag:v", "hvc1",
		"-movflags", "+faststart",
		"-y", fixedPath,
	)
	if err := cmd.Run(); err != nil {
		_ = os.Remove(fixedPath)
		return nil // Keep original valid MP4 if older ffmpeg lacks bsf
	}
	if fi, err := os.Stat(fixedPath); err == nil && fi.Size() > 1024 {
		_ = os.Rename(fixedPath, mp4Path)
	} else {
		_ = os.Remove(fixedPath)
	}
	return nil
}

func buildDahuaRTSPURL(host, user, pass string, channel0Based int, start, end time.Time) string {
	const f = "2006_01_02_15_04_05"
	u := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:554", host),
		Path:   "/cam/playback",
	}
	u.RawQuery = fmt.Sprintf("channel=%d&subtype=0&starttime=%s&endtime=%s",
		channel0Based+1, start.Format(f), end.Format(f))
	return u.String()
}

func buildHikRTSPURL(host string, rtspPort int, user, pass string, channel1Based int, start, end time.Time) string {
	if rtspPort <= 0 {
		rtspPort = 554
	}
	const f = "20060102T150405Z"
	u := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:%d", host, rtspPort),
		Path:   fmt.Sprintf("/Streaming/tracks/%d/", channel1Based*100+1),
	}
	u.RawQuery = fmt.Sprintf("starttime=%s&endtime=%s", start.Format(f), end.Format(f))
	return u.String()
}

func (b *nvrBridge) resolveHikRTSPPort(ctx context.Context, d config.Device) int {
	b.rtspPortMu.Lock()
	if p, ok := b.rtspPorts[d.Host]; ok && p > 0 {
		b.rtspPortMu.Unlock()
		return p
	}
	b.rtspPortMu.Unlock()

	if d.Port > 0 && d.Port != 8000 {
		pctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		cl := isapi.New(d.Host, d.Port, false, d.Username, d.Password, 4*time.Second)
		if p, err := cl.GetRTSPPort(pctx); err == nil && p > 0 {
			b.rtspPortMu.Lock()
			b.rtspPorts[d.Host] = p
			b.rtspPortMu.Unlock()
			return p
		}
	}
	return 554
}

func buildRaysharpRTSPPlaybackURL(host, user, pass string, channel0Based int, start, end time.Time) string {
	const f = "2006_01_02_15_04_05"
	u := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:554", host),
		Path:   "/rtsp/playback",
	}
	u.RawQuery = fmt.Sprintf("channel=%d&subtype=0&starttime=%s&endtime=%s",
		channel0Based+1, start.Format(f), end.Format(f))
	return u.String()
}

// getOrBuildClipMP4 returns the path to a ready +faststart MP4 in /dev/shm/kspcam-clips,
// coalescing concurrent requests for the same [device, channel, start, end] via singleflight.
func (b *nvrBridge) getOrBuildClipMP4(ctx context.Context, tgt resolvedBridgeTarget, start, end time.Time, jobID string) (string, error) {
	dur := end.Sub(start)
	if dur <= 0 {
		return "", fmt.Errorf("end time must be after start time")
	}
	if maxDur := b.maxClipDuration(); dur > maxDur {
		return "", fmt.Errorf("clip duration %s exceeds RAM spool safety limit (%s)", dur.Round(time.Second), maxDur)
	}

	spoolFile := filepath.Join(b.spoolDir, clipSpoolKey(tgt.Source.ID, tgt.Channel, start, end))
	if fi, err := os.Stat(spoolFile); err == nil && fi.Size() > 1024 {
		now := time.Now()
		_ = os.Chtimes(spoolFile, now, now)
		if jobID != "" {
			setExportJob(jobID, func(j *exportJob) { j.Done, j.Total, j.Phase = 1, 1, "done" })
		}
		return spoolFile, nil
	}

	// Detach build context from an individual short HTTP Range probe (e.g. Safari
	// opening a 2-byte Range probe then cancelling to re-open!) so the in-flight
	// 4-second LAN pull completes cleanly into spoolFile for the follow-up Range!
	val, err := b.clipGroup.Do(spoolFile, func() (any, error) {
		if fi, err := os.Stat(spoolFile); err == nil && fi.Size() > 1024 {
			return spoolFile, nil
		}

		buildCtx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		if jobID != "" {
			setExportJob(jobID, func(j *exportJob) { j.Done, j.Total, j.Phase = 0, 2, "fetch" })
			buildCtx = mediaexport.WithProgress(buildCtx, func(done, total int, phase string) {
				setExportJob(jobID, func(j *exportJob) { j.Done, j.Total, j.Phase = done, total, phase })
			})
		}

		releaseSlot, err := b.acquireHostSlot(buildCtx, tgt.Source.Host, tgt.IsCamSD)
		if err != nil {
			return "", err
		}
		defer releaseSlot()

		// Reserve ~55 MB in /dev/shm before pulling new clip.
		b.evictSpool(55 << 20)

		workDir, err := os.MkdirTemp(b.spoolDir, ".work-")
		if err != nil {
			return "", fmt.Errorf("bridge: create workdir: %w", err)
		}
		defer os.RemoveAll(workDir)

		tmpMP4 := filepath.Join(workDir, "out.mp4")
		if err := b.pullAndRemuxZeroCPU(buildCtx, tgt, start, end, workDir, tmpMP4); err != nil {
			if jobID != "" {
				setExportJob(jobID, func(j *exportJob) { j.Phase, j.Error = "error", err.Error() })
			}
			return "", err
		}

		fi, err := os.Stat(tmpMP4)
		if err != nil || fi.Size() < 1024 {
			return "", fmt.Errorf("bridge: exported MP4 is empty")
		}
		b.evictSpool(fi.Size())
		if err := os.Rename(tmpMP4, spoolFile); err != nil {
			return "", fmt.Errorf("bridge: commit spool file: %w", err)
		}
		if jobID != "" {
			setExportJob(jobID, func(j *exportJob) { j.Done, j.Total, j.Phase = 2, 2, "done" })
		}
		return spoolFile, nil
	})
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

func (b *nvrBridge) pullAndRemuxZeroCPU(ctx context.Context, tgt resolvedBridgeTarget, start, end time.Time, workDir, outMP4 string) error {
	d := tgt.Source
	durSec := end.Sub(start).Seconds()

	switch d.Vendor {
	case config.VendorDahua:
		// 1a. Primary NVR Ultra-Fast Path: Binary DVRIP 0xbb .dav download (~5-8 MB/s on LAN).
		// Skipped on standalone [Cam-SD] IPCs (which silently drop 0xf4 frames until read timeout)
		// so [Cam-SD] jumps straight to 1b (Rate-Control:no ~7x RTSP)!
		if !tgt.IsCamSD {
			// 1a-CGI. Stateless HTTP Digest loadfile.cgi (~5+ MB/s on LAN, immune to ST7108/root DVRIP RPC stalls).
			cgiDavPath := filepath.Join(workDir, "raw_cgi.dav")
			if cgiFile, err := os.Create(cgiDavPath); err == nil {
				cgiErr := dahua.StreamDavCGI(ctx, cgiFile, d.Host, d.Port, d.Username, d.Password, tgt.Channel, start, end)
				_ = cgiFile.Close()
				if cgiErr == nil {
					if fi, stErr := os.Stat(cgiDavPath); stErr == nil && fi.Size() > 1024 {
						remuxErr := remuxCopyFileToFaststartMP4(ctx, "dhav", cgiDavPath, outMP4, 0, durSec)
						_ = os.Remove(cgiDavPath)
						if remuxErr == nil {
							return nil
						}
					}
				}
				_ = os.Remove(cgiDavPath)
			}

			// 1a-DVRIP. Binary DVRIP 0xbb .dav download fallback (for NATed NVRs with only TCP 37777/8888 exposed).
			davPath := filepath.Join(workDir, "raw.dav")
			if davFile, err := os.Create(davPath); err == nil {
				davErr := dahua.StreamDav(ctx, davFile, d.Host, d.Port, d.Username, d.Password, tgt.Channel, start, end)
				_ = davFile.Close()
				if davErr == nil {
					if fi, stErr := os.Stat(davPath); stErr == nil && fi.Size() > 1024 {
						remuxErr := remuxCopyFileToFaststartMP4(ctx, "dhav", davPath, outMP4, 0, durSec)
						_ = os.Remove(davPath)
						if remuxErr == nil {
							return nil
						}
					}
				}
				_ = os.Remove(davPath)
			}
		}
		// 1b. Fast RTSP "Rate-Control: no" path (~7x realtime on 1 sequential TCP session)
		// for modern Dahua/Imou [Cam-SD] IPCs that reject 0xf4 PlayBack.download.
		tsPath := filepath.Join(workDir, "fast.ts")
		if tsFile, err := os.Create(tsPath); err == nil {
			fastErr := dahua.StreamTSFast(ctx, tsFile, d.Host, d.Username, d.Password, tgt.Channel, start, end)
			_ = tsFile.Close()
			if fastErr == nil {
				if fi, stErr := os.Stat(tsPath); stErr == nil && fi.Size() > 1024 {
					remuxErr := remuxCopyFileToFaststartMP4(ctx, "mpegts", tsPath, outMP4, 0, durSec)
					_ = os.Remove(tsPath)
					if remuxErr == nil {
						return nil
					}
				}
			}
			_ = os.Remove(tsPath)
		}

		// 2. Fallback Path: RTSP Playback stream-copy (parallel=4 on NVR, parallel=1 on Cam-SD).
		parallel := 4
		chunkSec := 50
		if tgt.IsCamSD || durSec <= 55 {
			parallel = 1
			chunkSec = int(durSec) + 1
		}
		outFile, err := os.Create(outMP4)
		if err != nil {
			return err
		}
		rtspErr := mediaexport.FastMP4Range(ctx, outFile, start, end, chunkSec, func(cs, ce time.Time) string {
			return buildDahuaRTSPURL(d.Host, d.Username, d.Password, tgt.Channel, cs, ce)
		}, false, parallel)
		_ = outFile.Close()
		if rtspErr != nil {
			return rtspErr
		}
		return normalizeFullRangeMetadataZeroCPU(ctx, outMP4)

	case config.VendorHikvision:
		hikCh := tgt.Channel + 1 // Hikvision channels are 1-based (101, 201...)
		// 1. Primary Ultra-Fast Path for Hikvision NVR (!IsCamSD) over HTTP ISAPI:
		//    Time-bounded /ISAPI/ContentMgmt/download exports exact [start, end] at
		//    ~10-20 MB/s (~4.4s for 5 min 2K HEVC) with 0% CPU and 1 HTTP socket!
		if !tgt.IsCamSD && d.Port != 8000 {
			httpPort := d.Port
			if httpPort <= 0 {
				httpPort = 80
			}
			maxIMKH := int64(110 << 20)
			if b.lowRAMBox {
				maxIMKH = 52 << 20
			}
			cl := isapi.New(d.Host, httpPort, false, d.Username, d.Password, 20*time.Second)

			// 1a. Time-only playbackURI (trims server-side on NVR even when HDD segment is 1 GB).
			imkhPath := filepath.Join(workDir, "range.imkh")
			if f, cErr := os.Create(imkhPath); cErr == nil {
				_, rErr := cl.DownloadTimeRange(ctx, f, d.Host, httpPort, hikCh*100+1, start, end, maxIMKH)
				_ = f.Close()
				if rErr == nil {
					remuxErr := remuxCopyFileToFaststartMP4(ctx, "", imkhPath, outMP4, 0, durSec)
					_ = os.Remove(imkhPath)
					if remuxErr == nil {
						return nil
					}
				}
				_ = os.Remove(imkhPath)
			}

			// 1b. Legacy fallback: full segment playbackURI when overlapping segments <= maxIMKH.
			if segs, sErr := cl.SearchTrack(ctx, hikCh*100+1, start, end, 30); sErr == nil && len(segs) > 0 {
				var totalBytes int64
				for _, sg := range segs {
					totalBytes += sg.Size
				}
				if totalBytes > 0 && totalBytes <= maxIMKH {
					segPath := filepath.Join(workDir, "raw.imkh")
					if f, cErr := os.Create(segPath); cErr == nil {
						dlOK := true
						for _, sg := range segs {
							if _, dErr := cl.DownloadStream(ctx, f, sg.PlaybackURI); dErr != nil {
								dlOK = false
								break
							}
						}
						_ = f.Close()
						if dlOK {
							ssOff := 0.0
							if start.After(segs[0].Start) {
								ssOff = start.Sub(segs[0].Start).Seconds()
							}
							remuxErr := remuxCopyFileToFaststartMP4(ctx, "", segPath, outMP4, ssOff, durSec)
							_ = os.Remove(segPath)
							if remuxErr == nil {
								return nil
							}
						}
						_ = os.Remove(segPath)
					}
				}
			}
		}

		// 2. RTSP Playback stream-copy (parallel=4 on NVR, sequential parallel=1 on [Hik-SD]),
		//    auto-discovering custom RTSP ports (e.g. 1554) via ISAPI adminAccesses.
		rtspPort := b.resolveHikRTSPPort(ctx, d)
		parallel := 4
		chunkSec := 30
		switch {
		case tgt.IsCamSD:
			parallel = 1
			chunkSec = int(durSec) + 1
		case durSec <= 20:
			parallel = 1
			chunkSec = int(durSec) + 1
		case durSec <= 95:
			parallel = 5
			chunkSec = 15
		case durSec <= 185:
			parallel = 4
			chunkSec = 35
		default:
			parallel = 5
			chunkSec = 60
		}
		outFile, err := os.Create(outMP4)
		if err != nil {
			return err
		}
		// Pass true (-an) on first pass so G.711 pcm_mulaw/pcm_alaw tracks on Hikvision NVRs
		// never fail/retry chunk fetches or waste RTSP sessions.
		rtspErr := mediaexport.FastMP4Range(ctx, outFile, start, end, chunkSec, func(cs, ce time.Time) string {
			return buildHikRTSPURL(d.Host, rtspPort, d.Username, d.Password, hikCh, cs, ce)
		}, true, parallel)
		_ = outFile.Close()
		if rtspErr != nil {
			return rtspErr
		}
		return normalizeFullRangeMetadataZeroCPU(ctx, outMP4)

	case config.VendorTiandy:
		cam, err := camera.Open(ctx, d, 30*time.Second)
		if err != nil {
			return err
		}
		defer cam.Close()
		rec, ok := cam.(camera.Recorder)
		if !ok {
			return fmt.Errorf("tiandy recorder unavailable")
		}
		outFile, err := os.Create(outMP4)
		if err != nil {
			return err
		}
		err = rec.StreamPlaybackFast(ctx, outFile, tgt.Channel, start, end)
		_ = outFile.Close()
		return err

	case config.VendorRaysharp:
		parallel := 2
		chunkSec := 60
		if tgt.IsCamSD || durSec <= 65 {
			parallel = 1
			chunkSec = int(durSec) + 1
		}
		outFile, err := os.Create(outMP4)
		if err != nil {
			return err
		}
		err = mediaexport.FastMP4Range(ctx, outFile, start, end, chunkSec, func(cs, ce time.Time) string {
			return buildRaysharpRTSPPlaybackURL(d.Host, d.Username, d.Password, tgt.Channel, cs, ce)
		}, false, parallel)
		_ = outFile.Close()
		return err

	case config.VendorXiongmai:
		cl := xiongmai.New(d.Host, d.Port, d.Username, d.Password, 30*time.Second)
		defer cl.Close()
		return cl.ExportMP4Faststart(ctx, tgt.Channel, start, end, workDir, outMP4)
	default:
		return fmt.Errorf("unsupported vendor %q for NVR bridge", d.Vendor)
	}
}

// handleBridgeStatus serves GET /api/v1/bridge/status.
func (s *Server) handleBridgeStatus(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeBridge(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized bridge key")
		return
	}
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	used := s.bridge.evictSpool(0)
	type camMap struct {
		Mid        string `json:"mid"`
		CameraID   string `json:"cameraId"`
		CameraName string `json:"cameraName"`
		SourceID   string `json:"sourceId"`
		SourceHost string `json:"sourceHost"`
		Vendor     string `json:"vendor"`
		Channel    int    `json:"channel"`
		IsCamSD    bool   `json:"isCamSd"`
	}
	var mappings []camMap
	for _, d := range s.inv.List() {
		if d.IsNVR {
			continue
		}
		if tgt, ok := s.resolveBridgeTarget(d.ID, "", 0, false); ok {
			mappings = append(mappings, camMap{
				Mid:        tgt.Mid,
				CameraID:   d.ID,
				CameraName: d.Name,
				SourceID:   tgt.Source.ID,
				SourceHost: tgt.Source.Host,
				Vendor:     string(tgt.Source.Vendor),
				Channel:    tgt.Channel,
				IsCamSD:    tgt.IsCamSD,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"enabled":        s.cfg.Bridge.Enabled,
		"spoolDir":       s.bridge.spoolDir,
		"spoolUsedBytes": used,
		"spoolMaxBytes":  s.bridge.maxSpoolB,
		"lowRamMode":     s.bridge.lowRAMBox,
		"maxPerNvr":      s.bridge.maxPerNVR,
		"mappings":       mappings,
	})
}

// handleBridgeFlushCache serves POST /api/v1/bridge/flush-cache (used by CheckVAR).
func (s *Server) handleBridgeFlushCache(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeBridge(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized bridge key")
		return
	}
	s.bridge.FlushRecordingsCache()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleBridgeRecordings serves GET /api/v1/bridge/recordings?mid=camera01&start=...&end=...
func (s *Server) handleBridgeRecordings(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeBridge(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized bridge key")
		return
	}
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	chStr := strings.TrimSpace(q.Get("channel"))
	reqCh := 0
	hasExplicitCh := false
	if chStr != "" {
		if v, err := strconv.Atoi(chStr); err == nil && v >= 0 {
			reqCh = v
			hasExplicitCh = true
		}
	}
	tgt, ok := s.resolveBridgeTarget(q.Get("id"), q.Get("mid"), reqCh, hasExplicitCh)
	if !ok {
		writeErr(w, http.StatusNotFound, "camera/monitor not found in kspcam inventory")
		return
	}

	now := time.Now().In(time.Local)
	start := now.Add(-4 * time.Hour)
	end := now
	if stRaw := q.Get("start"); stRaw != "" {
		t, err := parseBridgeTime(stRaw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		start = t
	}
	if enRaw := q.Get("end"); enRaw != "" {
		t, err := parseBridgeTime(enRaw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		end = t
	}
	if !end.After(start) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mid": tgt.Mid, "recordings": []any{}})
		return
	}

	forceFresh := q.Get("fresh") == "1" || q.Get("nocache") == "1"
	list, err := s.bridge.getRecordingsCached(r.Context(), tgt, start, end, forceFresh)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if list == nil {
		list = []dahua.Recording{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"mid":        tgt.Mid,
		"sourceId":   tgt.Source.ID,
		"sourceHost": tgt.Source.Host,
		"vendor":     tgt.Source.Vendor,
		"channel":    tgt.Channel,
		"isCamSd":    tgt.IsCamSD,
		"recordings": list,
	})
}

// handleBridgeClipMP4 serves GET /api/v1/bridge/clip.mp4?mid=camera01&file=v_nvr_...
// (or ?mid=camera01&start=...&end=...) via HTTP 206 Partial Content from /dev/shm.
func (s *Server) handleBridgeClipMP4(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeBridge(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized bridge key")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	chStr := strings.TrimSpace(q.Get("channel"))
	reqCh := 0
	hasExplicitCh := false
	if chStr != "" {
		if v, err := strconv.Atoi(chStr); err == nil && v >= 0 {
			reqCh = v
			hasExplicitCh = true
		}
	}
	tgt, ok := s.resolveBridgeTarget(q.Get("id"), q.Get("mid"), reqCh, hasExplicitCh)
	if !ok {
		writeErr(w, http.StatusNotFound, "camera/monitor not found in kspcam inventory")
		return
	}

	var start, end time.Time
	dlFilename := strings.TrimSpace(q.Get("file"))
	if dlFilename != "" {
		st, en, valid := ParseVirtualNVRFilename(dlFilename)
		if !valid {
			writeErr(w, http.StatusBadRequest, "invalid virtual NVR filename")
			return
		}
		start, end = st, en
	} else {
		var err error
		start, err = parseBridgeTime(q.Get("start"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		end, err = parseBridgeTime(q.Get("end"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		dlFilename = fmt.Sprintf("%s_%s.mp4", tgt.Mid, start.Format("20060102_150405"))
	}

	spoolFile, err := s.bridge.getOrBuildClipMP4(r.Context(), tgt, start, end, strings.TrimSpace(q.Get("job")))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	unpin := s.bridge.pinFile(spoolFile)
	defer unpin()

	f, err := os.Open(spoolFile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to open cached clip")
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		writeErr(w, http.StatusInternalServerError, "invalid cached clip")
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")
	if q.Get("download") == "1" || q.Get("dl") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filepath.Base(dlFilename)))
	} else {
		w.Header().Set("Cache-Control", "public, max-age=600")
	}
	http.ServeContent(w, r, filepath.Base(dlFilename), fi.ModTime(), f)
}
