package bidahub

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAnalyticsStateDir stores Node-RED offset baselines, revision state, and the bounded offline spool.
	DefaultAnalyticsStateDir = "/var/lib/inut-bida-agent"

	analyticsOffsetFilename  = "analytics_offset.json"
	analyticsSpoolFilename   = "analytics_spool.json"
	analyticsStateFilename   = "analytics_state.json"
	maxAnalyticsSpoolEntries = 48
	maxAnalyticsSpoolBytes   = 256 * 1024 // 256 KB
)

type nodeRedOffsetState struct {
	Initialized   bool   `json:"initialized"`
	ViewCount     int64  `json:"view_count"`
	DownloadCount int64  `json:"download_count"`
	PackedCount   int64  `json:"packed_count"`
	APICount      int64  `json:"api_count"`
	APIModelCount int64  `json:"api_model_count"`
	Seq           int64  `json:"seq"`
	UpdatedAt     string `json:"updated_at"`
}

type analyticsAgentState struct {
	LastAckedRev uint64 `json:"last_acked_rev"`
	SourceEpoch  string `json:"source_epoch"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type errAnalyticsRateLimited struct {
	RetryAfter time.Duration
}

func (e *errAnalyticsRateLimited) Error() string {
	return fmt.Sprintf("analytics ingest rate limited (429), retry after %v", e.RetryAfter)
}

// analyticsPhaseOffset spreads 5-minute analytics loops deterministically across the interval
// using fnv32a(boxID) % intervalSec to prevent thundering herds at :00 / :05.
func analyticsPhaseOffset(boxID string, interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	sec := int64(interval / time.Second)
	if sec <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimSpace(boxID)))
	return time.Duration(int64(h.Sum32())%sec) * time.Second
}

func (a *EdgeAgent) runAnalyticsLoop(ctx context.Context) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	interval := a.cfg.AnalyticsInterval
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	var initialDelay time.Duration
	if interval < time.Minute {
		// Short test intervals should not wait minutes before initial push.
		initialDelay = interval
	} else {
		phase := analyticsPhaseOffset(a.cfg.BoxID, interval)
		jitter := time.Duration(rng.Intn(15001)) * time.Millisecond
		initialDelay = phase + jitter
		if initialDelay < 5*time.Second {
			initialDelay = 5 * time.Second
		}
	}

	initTimer := time.NewTimer(initialDelay)
	select {
	case <-ctx.Done():
		initTimer.Stop()
		return
	case <-initTimer.C:
	}

	if err := a.PushAnalyticsOnce(ctx); err != nil && ctx.Err() == nil {
		log.Printf("[inut-bida-agent] initial analytics push deferred/spooled: %v", err)
	}

	for {
		interval = a.cfg.AnalyticsInterval
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		wait := interval
		if interval >= time.Minute {
			// ±15s jitter around the base interval
			jitter := time.Duration(rng.Intn(30001)-15000) * time.Millisecond
			wait = interval + jitter
			if wait < 30*time.Second {
				wait = 30 * time.Second
			}
		}
		if backoff := a.analyticsBackoffRemaining(time.Now()); backoff > wait {
			wait = backoff
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			if err := a.PushAnalyticsOnce(ctx); err != nil && ctx.Err() == nil {
				log.Printf("[inut-bida-agent] analytics push deferred/spooled: %v", err)
			}
		}
	}
}

func (a *EdgeAgent) analyticsBackoffRemaining(now time.Time) time.Duration {
	a.analyticsMu.Lock()
	until := a.analyticsBackoffUntil
	a.analyticsMu.Unlock()
	if until.IsZero() || !now.Before(until) {
		return 0
	}
	return until.Sub(now)
}

// fetchLocalRealtime queries 127.0.0.1:<RedBidaPort>/internal/analytics/realtime with a fast
// 400ms timeout. Returns nil when idle (A30 <= 0) or unreachable so heartbeat omitempty sends 0 extra bytes.
func (a *EdgeAgent) fetchLocalRealtime(ctx context.Context) *RealtimeBoxMetrics {
	port := a.cfg.RedBidaPort
	if port <= 0 {
		port = 2023
	}
	rtURL := fmt.Sprintf("http://127.0.0.1:%d/internal/analytics/realtime", port)
	reqCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rtURL, nil)
	if err != nil {
		return nil
	}

	client := a.realtimeHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 400 * time.Millisecond}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil || len(raw) == 0 {
		return nil
	}
	var rt RealtimeBoxMetrics
	if err := json.Unmarshal(raw, &rt); err != nil {
		return nil
	}
	rt = sanitizeRealtimeBoxMetrics(rt)
	if rt.A30 <= 0 {
		return nil
	}
	return &rt
}

// PushAnalyticsOnce collects local usage analytics (preferring redbida-go dirty-bucket delta
// with revision gating, falling back cleanly to /snapshot or legacy Node-RED counter files)
// and drains the merged offline spool to the Hub using gzip compression.
func (a *EdgeAgent) PushAnalyticsOnce(ctx context.Context) error {
	return a.pushAnalyticsAt(ctx, time.Now())
}

func (a *EdgeAgent) pushAnalyticsAt(ctx context.Context, now time.Time) error {
	a.analyticsMu.Lock()
	defer a.analyticsMu.Unlock()

	if !a.analyticsBackoffUntil.IsZero() && now.Before(a.analyticsBackoffUntil) {
		return &errAnalyticsRateLimited{RetryAfter: a.analyticsBackoffUntil.Sub(now)}
	}
	a.analyticsBackoffUntil = time.Time{}

	state := a.loadAnalyticsState()
	var current *AnalyticsIngestPayload

	// Step 1: Prefer redbida-go Tier-1 revision-gated delta endpoint.
	delta, status, deltaErr := a.fetchRedBidaDelta(ctx, state.LastAckedRev)
	switch {
	case deltaErr == nil && status == http.StatusNoContent:
		// 204 No Content: zero changes since lastAckedRev -> skip sending anything unless spool is non-empty!
		current = nil
	case deltaErr == nil && status == http.StatusOK:
		// If epoch changed mid-flight on a delta response, immediately re-fetch full snapshot (since_rev=0).
		if delta.IsDelta && state.SourceEpoch != "" && delta.SourceEpoch != "" && delta.SourceEpoch != state.SourceEpoch {
			if full, fStatus, fErr := a.fetchRedBidaDelta(ctx, 0); fErr == nil && fStatus == http.StatusOK {
				delta = full
			}
		}
		delta.BoxID = a.cfg.BoxID
		if strings.TrimSpace(delta.Source) == "" {
			delta.Source = "redbida-go"
		}
		// Realtime is piggybacked on heartbeats; omit from ingest body to minimize bytes.
		delta.Realtime = nil
		if len(delta.BoxHourly) > 0 || len(delta.BoxDaily) > 0 || len(delta.CameraDaily) > 0 {
			current = &delta
		}
	case deltaErr == nil && (status == http.StatusNotFound || status == http.StatusMethodNotAllowed):
		// Step 1b: Older redbida-go without /delta -> cleanly fall back to /internal/analytics/snapshot.
		if snap, ok := a.fetchRedBidaSnapshot(ctx); ok {
			snap.BoxID = a.cfg.BoxID
			if strings.TrimSpace(snap.Source) == "" {
				snap.Source = "redbida-go"
			}
			snap.Realtime = nil
			current = &snap
		}
	default:
		// Step 2: redbida-go unreachable -> fall back to legacy Node-RED monotonic counter files.
		if nrBatch, ok, err := a.collectNodeRedDelta(now); err == nil && ok {
			current = &nrBatch
		}
	}

	// Load any previously spooled batches, append current batch, and coalesce/merge duplicate buckets.
	spool := a.loadAnalyticsSpool()
	if current != nil {
		spool = append(spool, *current)
	}
	if len(spool) == 0 {
		return nil
	}
	spool = boundAnalyticsSpool(mergeAnalyticsSpool(spool))

	for i := 0; i < len(spool); i++ {
		res, retryAfter, err := a.postAnalyticsBatch(ctx, spool[i], now)
		if err != nil {
			if retryAfter > 0 {
				a.analyticsBackoffUntil = now.Add(retryAfter)
			}
			// Persist remaining unsent items (merged & bounded) for retry on next tick.
			remaining := boundAnalyticsSpool(mergeAnalyticsSpool(spool[i:]))
			_ = a.saveAnalyticsSpool(remaining)
			return err
		}

		// Update persisted revision/epoch state on Hub ACK for redbida-go batches.
		if spool[i].Source == "redbida-go" {
			if res.NeedFullSync {
				state.LastAckedRev = 0
				state.SourceEpoch = spool[i].SourceEpoch
				state.UpdatedAt = now.UTC().Format(time.RFC3339)
				_ = a.saveAnalyticsState(state)
			} else if spool[i].Rev > 0 {
				if spool[i].Rev > state.LastAckedRev || spool[i].SourceEpoch != state.SourceEpoch {
					state.LastAckedRev = spool[i].Rev
					state.SourceEpoch = spool[i].SourceEpoch
					state.UpdatedAt = now.UTC().Format(time.RFC3339)
					_ = a.saveAnalyticsState(state)
				}
			} else if spool[i].SourceEpoch != "" && spool[i].SourceEpoch != state.SourceEpoch {
				state.SourceEpoch = spool[i].SourceEpoch
				state.UpdatedAt = now.UTC().Format(time.RFC3339)
				_ = a.saveAnalyticsState(state)
			}
		}
	}

	// All batches drained cleanly — remove spool file if present.
	a.clearAnalyticsSpool()
	return nil
}

func (a *EdgeAgent) fetchRedBidaDelta(ctx context.Context, sinceRev uint64) (AnalyticsIngestPayload, int, error) {
	port := a.cfg.RedBidaPort
	if port <= 0 {
		port = 2023
	}
	deltaURL := fmt.Sprintf("http://127.0.0.1:%d/internal/analytics/delta?since_rev=%d", port, sinceRev)
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, deltaURL, nil)
	if err != nil {
		return AnalyticsIngestPayload{}, 0, err
	}

	client := a.analyticsHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return AnalyticsIngestPayload{}, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return AnalyticsIngestPayload{}, http.StatusNoContent, nil
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return AnalyticsIngestPayload{}, resp.StatusCode, nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, analyticsMaxIngestBytes))
	if err != nil || len(raw) == 0 {
		return AnalyticsIngestPayload{}, resp.StatusCode, fmt.Errorf("empty delta body")
	}
	var snap AnalyticsIngestPayload
	if err := json.Unmarshal(raw, &snap); err != nil {
		return AnalyticsIngestPayload{}, resp.StatusCode, err
	}
	return snap, http.StatusOK, nil
}

func (a *EdgeAgent) fetchRedBidaSnapshot(ctx context.Context) (AnalyticsIngestPayload, bool) {
	port := a.cfg.RedBidaPort
	if port <= 0 {
		port = 2023
	}
	snapURL := fmt.Sprintf("http://127.0.0.1:%d/internal/analytics/snapshot", port)
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, snapURL, nil)
	if err != nil {
		return AnalyticsIngestPayload{}, false
	}

	client := a.analyticsHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return AnalyticsIngestPayload{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return AnalyticsIngestPayload{}, false
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, analyticsMaxIngestBytes))
	if err != nil || len(raw) == 0 {
		return AnalyticsIngestPayload{}, false
	}
	var snap AnalyticsIngestPayload
	if err := json.Unmarshal(raw, &snap); err != nil {
		return AnalyticsIngestPayload{}, false
	}
	return snap, true
}

func (a *EdgeAgent) collectNodeRedDelta(now time.Time) (AnalyticsIngestPayload, bool, error) {
	dir := strings.TrimSpace(a.cfg.RedBidaKeyDir)
	if dir == "" {
		dir = DefaultRedBidaKeyDir
	}

	viewVal, viewOK := readMonotonicIntFile(filepath.Join(dir, "view_count"))
	dlVal, dlOK := readMonotonicIntFile(filepath.Join(dir, "download_count"))
	packedVal, packedOK := readMonotonicIntFile(filepath.Join(dir, "packed_count"))
	apiVal, apiOK := readMonotonicIntFile(filepath.Join(dir, "api_count"))
	modelVal, modelOK := readMonotonicIntFile(filepath.Join(dir, "api_model_count"))

	if !viewOK && !dlOK && !packedOK && !apiOK && !modelOK {
		return AnalyticsIngestPayload{}, false, nil
	}

	offsetPath := filepath.Join(a.analyticsStateDir(), analyticsOffsetFilename)
	prev, exists := loadNodeRedOffset(offsetPath)
	nowLocal := now.In(analyticsTZ)

	if !exists || !prev.Initialized {
		// First run: save current baselines and skip emitting a giant historical spike!
		baseline := nodeRedOffsetState{
			Initialized:   true,
			ViewCount:     viewVal,
			DownloadCount: dlVal,
			PackedCount:   packedVal,
			APICount:      apiVal,
			APIModelCount: modelVal,
			Seq:           0,
			UpdatedAt:     nowLocal.Format(time.RFC3339),
		}
		if err := saveNodeRedOffset(offsetPath, baseline); err != nil {
			return AnalyticsIngestPayload{}, false, err
		}
		return AnalyticsIngestPayload{}, false, nil
	}

	dView := positiveCounterDelta(viewVal, prev.ViewCount)
	dDL := positiveCounterDelta(dlVal, prev.DownloadCount)
	dPacked := positiveCounterDelta(packedVal, prev.PackedCount)
	dAPI := positiveCounterDelta(apiVal, prev.APICount)
	dModel := positiveCounterDelta(modelVal, prev.APIModelCount)

	if dView == 0 && dDL == 0 && dPacked == 0 && dAPI == 0 && dModel == 0 {
		// Still persist if any counter dropped (reset to 0) so future increments are captured.
		if viewVal < prev.ViewCount || dlVal < prev.DownloadCount || packedVal < prev.PackedCount ||
			apiVal < prev.APICount || modelVal < prev.APIModelCount {
			prev.ViewCount = viewVal
			prev.DownloadCount = dlVal
			prev.PackedCount = packedVal
			prev.APICount = apiVal
			prev.APIModelCount = modelVal
			prev.UpdatedAt = nowLocal.Format(time.RFC3339)
			_ = saveNodeRedOffset(offsetPath, prev)
		}
		return AnalyticsIngestPayload{}, false, nil
	}

	nextSeq := prev.Seq + 1
	hourBucket := nowLocal.Format("2006-01-02 15:00")
	batchID := fmt.Sprintf("%s:%s:%d", a.cfg.BoxID, hourBucket, nextSeq)

	nextState := nodeRedOffsetState{
		Initialized:   true,
		ViewCount:     viewVal,
		DownloadCount: dlVal,
		PackedCount:   packedVal,
		APICount:      apiVal,
		APIModelCount: modelVal,
		Seq:           nextSeq,
		UpdatedAt:     nowLocal.Format(time.RFC3339),
	}
	if err := saveNodeRedOffset(offsetPath, nextState); err != nil {
		return AnalyticsIngestPayload{}, false, err
	}

	payload := AnalyticsIngestPayload{
		BoxID:       a.cfg.BoxID,
		Source:      "nodered",
		BatchID:     batchID,
		GeneratedAt: nowLocal.Format(time.RFC3339),
		AIEnabled:   false,
		BoxHourly: []AnalyticsBoxHourlyIn{
			{
				BucketHour: hourBucket,
				AnalyticsBoxMetrics: AnalyticsBoxMetrics{
					PageViews:     dView,
					QRViews:       dAPI,
					ClipDownloads: dDL,
					CutClips:      dPacked,
					OSApp:         dModel,
				},
			},
		},
	}
	return payload, true, nil
}

func (a *EdgeAgent) postAnalyticsBatch(ctx context.Context, payload AnalyticsIngestPayload, now time.Time) (AnalyticsIngestResult, time.Duration, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return AnalyticsIngestResult{}, 0, err
	}

	var gzBuf bytes.Buffer
	gzw := gzip.NewWriter(&gzBuf)
	if _, err := gzw.Write(raw); err != nil {
		_ = gzw.Close()
		return AnalyticsIngestResult{}, 0, err
	}
	if err := gzw.Close(); err != nil {
		return AnalyticsIngestResult{}, 0, err
	}

	ingestURL := strings.TrimRight(a.cfg.HubURL, "/") + "/fleet/api/analytics/ingest"
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, ingestURL, bytes.NewReader(gzBuf.Bytes()))
	if err != nil {
		return AnalyticsIngestResult{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("X-Box-ID", a.cfg.BoxID)
	req.Header.Set("X-Box-Secret", a.cfg.Secret)
	req.Header.Set("X-Hub-Key", a.cfg.Secret)

	client := a.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return AnalyticsIngestResult{}, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"), now)
		return AnalyticsIngestResult{}, retryAfter, &errAnalyticsRateLimited{RetryAfter: retryAfter}
	}
	if resp.StatusCode >= 300 {
		return AnalyticsIngestResult{}, 0, fmt.Errorf("analytics ingest status %d", resp.StatusCode)
	}

	var res AnalyticsIngestResult
	if len(bytes.TrimSpace(body)) > 0 {
		_ = json.Unmarshal(body, &res)
	}
	return res, 0, nil
}

func parseRetryAfterHeader(raw string, now time.Time) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 60 * time.Second
	}
	if sec, err := strconv.Atoi(raw); err == nil {
		if sec < 1 {
			sec = 1
		}
		if sec > 1800 {
			sec = 1800
		}
		return time.Duration(sec) * time.Second
	}
	if t, err := http.ParseTime(raw); err == nil {
		d := t.Sub(now)
		if d < time.Second {
			return time.Second
		}
		if d > 30*time.Minute {
			return 30 * time.Minute
		}
		return d
	}
	return 60 * time.Second
}

func (a *EdgeAgent) analyticsStateDir() string {
	dir := strings.TrimSpace(a.cfg.AnalyticsStateDir)
	if dir == "" {
		return DefaultAnalyticsStateDir
	}
	return dir
}

func (a *EdgeAgent) loadAnalyticsState() analyticsAgentState {
	path := filepath.Join(a.analyticsStateDir(), analyticsStateFilename)
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return analyticsAgentState{}
	}
	var st analyticsAgentState
	if err := json.Unmarshal(raw, &st); err != nil {
		return analyticsAgentState{}
	}
	return st
}

func (a *EdgeAgent) saveAnalyticsState(st analyticsAgentState) error {
	path := filepath.Join(a.analyticsStateDir(), analyticsStateFilename)
	raw, err := json.MarshalIndent(&st, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, raw, 0o600)
}

func (a *EdgeAgent) loadAnalyticsSpool() []AnalyticsIngestPayload {
	path := filepath.Join(a.analyticsStateDir(), analyticsSpoolFilename)
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var items []AnalyticsIngestPayload
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	return items
}

func (a *EdgeAgent) saveAnalyticsSpool(items []AnalyticsIngestPayload) error {
	items = boundAnalyticsSpool(mergeAnalyticsSpool(items))
	path := filepath.Join(a.analyticsStateDir(), analyticsSpoolFilename)
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, raw, 0o600)
}

func (a *EdgeAgent) clearAnalyticsSpool() {
	path := filepath.Join(a.analyticsStateDir(), analyticsSpoolFilename)
	_ = os.Remove(path)
}

// mergeAnalyticsSpool coalesces offline spooled redbida-go snapshots/deltas sharing the same
// (box_id, source_epoch) by merging buckets per bucket_hour, bucket_date, and (bucket_date, camera_id)
// instead of stacking duplicate full snapshots, while preserving distinct Node-RED delta batches oldest-first.
func mergeAnalyticsSpool(items []AnalyticsIngestPayload) []AnalyticsIngestPayload {
	if len(items) == 0 {
		return nil
	}

	type redBidaKey struct {
		boxID string
		epoch string
	}
	out := make([]AnalyticsIngestPayload, 0, len(items))
	rbIndex := make(map[redBidaKey]int)
	seenNRBatch := make(map[string]struct{})

	for _, item := range items {
		src := strings.ToLower(strings.TrimSpace(item.Source))
		if src == "" {
			if strings.TrimSpace(item.BatchID) != "" && strings.TrimSpace(item.SourceEpoch) == "" {
				src = "nodered"
			} else {
				src = "redbida-go"
			}
		}
		item.Source = src
		item.Realtime = nil

		if src == "nodered" {
			bid := strings.TrimSpace(item.BatchID)
			if bid != "" {
				if _, dup := seenNRBatch[bid]; dup {
					continue
				}
				seenNRBatch[bid] = struct{}{}
			}
			out = append(out, item)
			continue
		}

		// Merge cumulative redbida-go payloads by (box_id, source_epoch).
		key := redBidaKey{
			boxID: strings.TrimSpace(item.BoxID),
			epoch: strings.TrimSpace(item.SourceEpoch),
		}
		if idx, exists := rbIndex[key]; exists {
			out[idx] = mergeRedBidaPayloads(out[idx], item)
		} else {
			rbIndex[key] = len(out)
			out = append(out, mergeRedBidaPayloads(AnalyticsIngestPayload{
				BoxID:       item.BoxID,
				Secret:      item.Secret,
				Source:      "redbida-go",
				SourceEpoch: item.SourceEpoch,
				IsDelta:     item.IsDelta,
			}, item))
		}
	}
	return out
}

func mergeRedBidaPayloads(dst, src AnalyticsIngestPayload) AnalyticsIngestPayload {
	res := dst
	if res.BoxID == "" {
		res.BoxID = src.BoxID
	}
	if res.Secret == "" {
		res.Secret = src.Secret
	}
	res.Source = "redbida-go"
	if res.SourceEpoch == "" {
		res.SourceEpoch = src.SourceEpoch
	}
	if src.GeneratedAt > res.GeneratedAt {
		res.GeneratedAt = src.GeneratedAt
	}
	res.AIEnabled = res.AIEnabled || src.AIEnabled
	if src.Rev > res.Rev {
		res.Rev = src.Rev
	}
	// If either payload is a full snapshot, the merged payload covers full snapshot semantics.
	res.IsDelta = res.IsDelta && src.IsDelta

	// 1. Merge BoxHourly by bucket_hour (cumulative max per metric).
	hourlyMap := make(map[string]AnalyticsBoxMetrics, len(dst.BoxHourly)+len(src.BoxHourly))
	for _, list := range [][]AnalyticsBoxHourlyIn{dst.BoxHourly, src.BoxHourly} {
		for _, h := range list {
			hk := strings.TrimSpace(h.BucketHour)
			if hk == "" {
				continue
			}
			m := h.AnalyticsBoxMetrics
			m.sanitize()
			if cur, ok := hourlyMap[hk]; ok {
				hourlyMap[hk] = cur.maxWith(m)
			} else {
				hourlyMap[hk] = m
			}
		}
	}
	if len(hourlyMap) > 0 {
		keys := make([]string, 0, len(hourlyMap))
		for k := range hourlyMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		res.BoxHourly = make([]AnalyticsBoxHourlyIn, 0, len(keys))
		for _, k := range keys {
			res.BoxHourly = append(res.BoxHourly, AnalyticsBoxHourlyIn{
				BucketHour:          k,
				AnalyticsBoxMetrics: hourlyMap[k],
			})
		}
	}

	// 2. Merge BoxDaily by bucket_date (cumulative max per field).
	dailyMap := make(map[string]AnalyticsBoxDailyIn, len(dst.BoxDaily)+len(src.BoxDaily))
	for _, list := range [][]AnalyticsBoxDailyIn{dst.BoxDaily, src.BoxDaily} {
		for _, d := range list {
			dk := strings.TrimSpace(d.BucketDate)
			if dk == "" {
				continue
			}
			d.BucketDate = dk
			d.UniqueVisitors = clampNonNeg64(d.UniqueVisitors)
			d.Sessions = clampNonNeg64(d.Sessions)
			d.EngagedSessions = clampNonNeg64(d.EngagedSessions)
			d.ReturningVisitors = clampNonNeg64(d.ReturningVisitors)
			if cur, ok := dailyMap[dk]; ok {
				cur.UniqueVisitors = max64(cur.UniqueVisitors, d.UniqueVisitors)
				cur.Sessions = max64(cur.Sessions, d.Sessions)
				cur.EngagedSessions = max64(cur.EngagedSessions, d.EngagedSessions)
				cur.ReturningVisitors = max64(cur.ReturningVisitors, d.ReturningVisitors)
				dailyMap[dk] = cur
			} else {
				dailyMap[dk] = d
			}
		}
	}
	if len(dailyMap) > 0 {
		keys := make([]string, 0, len(dailyMap))
		for k := range dailyMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		res.BoxDaily = make([]AnalyticsBoxDailyIn, 0, len(keys))
		for _, k := range keys {
			res.BoxDaily = append(res.BoxDaily, dailyMap[k])
		}
	}

	// 3. Merge CameraDaily by (bucket_date, camera_id) (cumulative max per metric).
	type camKey struct {
		date string
		cam  string
	}
	camMap := make(map[camKey]AnalyticsCamDailyIn, len(dst.CameraDaily)+len(src.CameraDaily))
	for _, list := range [][]AnalyticsCamDailyIn{dst.CameraDaily, src.CameraDaily} {
		for _, c := range list {
			dk := strings.TrimSpace(c.BucketDate)
			cid := strings.TrimSpace(c.CameraID)
			if dk == "" || cid == "" {
				continue
			}
			c.BucketDate = dk
			c.CameraID = cid
			c.TableName = strings.TrimSpace(c.TableName)
			c.AnalyticsCamMetrics.sanitize()
			k := camKey{date: dk, cam: cid}
			if cur, ok := camMap[k]; ok {
				if c.TableName != "" {
					cur.TableName = c.TableName
				}
				cur.AnalyticsCamMetrics = cur.AnalyticsCamMetrics.maxWith(c.AnalyticsCamMetrics)
				camMap[k] = cur
			} else {
				camMap[k] = c
			}
		}
	}
	if len(camMap) > 0 {
		keys := make([]camKey, 0, len(camMap))
		for k := range camMap {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].date != keys[j].date {
				return keys[i].date < keys[j].date
			}
			return keys[i].cam < keys[j].cam
		})
		res.CameraDaily = make([]AnalyticsCamDailyIn, 0, len(keys))
		for _, k := range keys {
			res.CameraDaily = append(res.CameraDaily, camMap[k])
		}
	}

	return res
}

func boundAnalyticsSpool(items []AnalyticsIngestPayload) []AnalyticsIngestPayload {
	if len(items) > maxAnalyticsSpoolEntries {
		items = items[len(items)-maxAnalyticsSpoolEntries:]
	}
	for len(items) > 1 {
		raw, err := json.Marshal(items)
		if err != nil || len(raw) <= maxAnalyticsSpoolBytes {
			break
		}
		items = items[1:]
	}
	return items
}

func readMonotonicIntFile(path string) (int64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil || v < 0 {
		return 0, true
	}
	return v, true
}

func positiveCounterDelta(cur, prev int64) int64 {
	if cur < 0 {
		return 0
	}
	if cur >= prev {
		return cur - prev
	}
	// Counter file was wiped/reset — treat current value as fresh positive delta.
	return cur
}

func loadNodeRedOffset(path string) (nodeRedOffsetState, bool) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return nodeRedOffsetState{}, false
	}
	var st nodeRedOffsetState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nodeRedOffsetState{}, false
	}
	return st, true
}

func saveNodeRedOffset(path string, st nodeRedOffsetState) error {
	raw, err := json.MarshalIndent(&st, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, raw, 0o600)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpPath := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// isAnalyticsRateLimited returns true and the suggested retry duration if err is a 429 rate-limit error.
func isAnalyticsRateLimited(err error) (time.Duration, bool) {
	var rl *errAnalyticsRateLimited
	if errors.As(err, &rl) && rl != nil {
		return rl.RetryAfter, true
	}
	return 0, false
}
