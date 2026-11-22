package bidahub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAnalyticsStateDir stores Node-RED offset baselines and the bounded offline spool.
	DefaultAnalyticsStateDir = "/var/lib/inut-bida-agent"

	analyticsOffsetFilename  = "analytics_offset.json"
	analyticsSpoolFilename   = "analytics_spool.json"
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

func (a *EdgeAgent) runAnalyticsLoop(ctx context.Context) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	initialDelay := 20*time.Second + time.Duration(rng.Intn(15001))*time.Millisecond
	if a.cfg.AnalyticsInterval > 0 && a.cfg.AnalyticsInterval < time.Minute {
		// Short test intervals should not wait 20-35s before initial push.
		initialDelay = a.cfg.AnalyticsInterval
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
		interval := a.cfg.AnalyticsInterval
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		jitter := time.Duration(0)
		if interval >= time.Minute {
			jitter = time.Duration(rng.Intn(45001)) * time.Millisecond
		}
		timer := time.NewTimer(interval + jitter)
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

// PushAnalyticsOnce collects local usage analytics (preferring redbida-go snapshot on loopback,
// falling back to legacy Node-RED counter files) and drains the bounded offline spool to the Hub.
func (a *EdgeAgent) PushAnalyticsOnce(ctx context.Context) error {
	return a.pushAnalyticsAt(ctx, time.Now())
}

func (a *EdgeAgent) pushAnalyticsAt(ctx context.Context, now time.Time) error {
	a.analyticsMu.Lock()
	defer a.analyticsMu.Unlock()

	var current *AnalyticsIngestPayload

	// Step 1: Try redbida-go loopback snapshot endpoint (2s timeout).
	if snap, ok := a.fetchRedBidaSnapshot(ctx); ok {
		snap.BoxID = a.cfg.BoxID
		if strings.TrimSpace(snap.Source) == "" {
			snap.Source = "redbida-go"
		}
		current = &snap
	} else {
		// Step 2: Fallback to legacy Node-RED monotonic counter files.
		if nrBatch, ok, err := a.collectNodeRedDelta(now); err == nil && ok {
			current = &nrBatch
		}
	}

	// Load any previously spooled batches and append current batch (oldest-first draining).
	spool := a.loadAnalyticsSpool()
	if current != nil {
		spool = append(spool, *current)
	}
	if len(spool) == 0 {
		return nil
	}
	spool = boundAnalyticsSpool(spool)

	for i := 0; i < len(spool); i++ {
		if err := a.postAnalyticsBatch(ctx, spool[i]); err != nil {
			// Persist remaining unsent items (bounded) for retry on next tick.
			remaining := boundAnalyticsSpool(spool[i:])
			_ = a.saveAnalyticsSpool(remaining)
			return err
		}
	}

	// All batches drained cleanly — remove spool file if present.
	a.clearAnalyticsSpool()
	return nil
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

func (a *EdgeAgent) postAnalyticsBatch(ctx context.Context, payload AnalyticsIngestPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ingestURL := strings.TrimRight(a.cfg.HubURL, "/") + "/fleet/api/analytics/ingest"
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, ingestURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Box-ID", a.cfg.BoxID)
	req.Header.Set("X-Box-Secret", a.cfg.Secret)
	req.Header.Set("X-Hub-Key", a.cfg.Secret)

	client := a.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("analytics ingest status %d", resp.StatusCode)
	}
	return nil
}

func (a *EdgeAgent) analyticsStateDir() string {
	dir := strings.TrimSpace(a.cfg.AnalyticsStateDir)
	if dir == "" {
		return DefaultAnalyticsStateDir
	}
	return dir
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
	items = boundAnalyticsSpool(items)
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
