package bidahub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func serverPortFromURL(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url %q: %v", rawURL, err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil || p <= 0 {
		t.Fatalf("invalid port in %q: %v", rawURL, err)
	}
	return p
}

func TestEdgeAgentAnalytics_RedBidaSnapshotPushEndToEnd(t *testing.T) {
	hubSrv, err := NewServer(Config{
		DataPath:     ":memory:",
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer hubSrv.Close()

	hubHTTP := httptest.NewServer(hubSrv.Handler())
	defer hubHTTP.Close()

	box := hubSrv.Store().Boxes()[0]
	nowLocal := time.Now().In(analyticsTZ)
	today := nowLocal.Format("2006-01-02")
	curHour := nowLocal.Format("2006-01-02 15:00")

	// Mock local redbida-go /internal/analytics/snapshot endpoint
	redbidaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/analytics/snapshot" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, AnalyticsIngestPayload{
			Source:      "redbida-go",
			SourceEpoch: "mock-ep-1",
			GeneratedAt: nowLocal.Format(time.RFC3339),
			AIEnabled:   true,
			BoxHourly: []AnalyticsBoxHourlyIn{
				{
					BucketHour: curHour,
					AnalyticsBoxMetrics: AnalyticsBoxMetrics{
						QRViews: 33, PageViews: 41, UniqueVisitors: 19,
						ClipDownloads: 7, CutClips: 4, CutSeconds: 240,
						AIStudioOpens: 5, AIAnalyzes: 3, AIShots: 21, AIPots: 11,
						AIExportsIOS: 2, AIExportRelay: 2, BytesOut: 18_500_000, OSIOS: 12, OSAndroid: 7,
					},
				},
			},
			BoxDaily: []AnalyticsBoxDailyIn{{BucketDate: today, UniqueVisitors: 17}},
			CameraDaily: []AnalyticsCamDailyIn{
				{
					BucketDate: today, CameraID: "camera02", TableName: "Bàn 02 Carom",
					AnalyticsCamMetrics: AnalyticsCamMetrics{QRViews: 20, UniqueVisitors: 10, AIAnalyzes: 2, AIShots: 14},
				},
			},
		})
	}))
	defer redbidaMock.Close()

	agent := NewEdgeAgent(AgentConfig{
		HubURL:            hubHTTP.URL,
		BoxID:             box.ID,
		Secret:            testAPIKey,
		RedBidaPort:       serverPortFromURL(t, redbidaMock.URL),
		RedBidaKeyDir:     t.TempDir(),
		AnalyticsStateDir: t.TempDir(),
	})

	ctx := context.Background()
	// Push twice to verify idempotency end-to-end
	if err := agent.PushAnalyticsOnce(ctx); err != nil {
		t.Fatalf("first PushAnalyticsOnce failed: %v", err)
	}
	if err := agent.PushAnalyticsOnce(ctx); err != nil {
		t.Fatalf("second PushAnalyticsOnce failed: %v", err)
	}

	ov, err := hubSrv.queryAnalyticsOverview(analyticsQueryParams{Range: "7d", BoxID: box.ID}, time.Now())
	if err != nil {
		t.Fatalf("queryAnalyticsOverview failed: %v", err)
	}
	if ov.KPI.QRViews != 33 || ov.KPI.UniqueVisitors != 17 || ov.KPI.AIAnalyzes != 3 || ov.KPI.AIShots != 21 {
		t.Fatalf("unexpected Hub KPI after redbida-go snapshot push: %+v", ov.KPI)
	}
	cams, err := hubSrv.queryAnalyticsCameras(analyticsCameraQueryParams{Range: "7d", BoxID: box.ID}, time.Now())
	if err != nil || len(cams) != 1 || cams[0].CameraID != "camera02" || cams[0].TableName != "Bàn 02 Carom" {
		t.Fatalf("unexpected Hub cameras: %+v err=%v", cams, err)
	}
}

func TestEdgeAgentAnalytics_NodeRedFallbackBaselineDeltaAndReset(t *testing.T) {
	hubSrv, err := NewServer(Config{
		DataPath:     ":memory:",
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer hubSrv.Close()

	var ingestHits atomic.Int32
	hubHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fleet/api/analytics/ingest" {
			ingestHits.Add(1)
		}
		hubSrv.ServeHTTP(w, r)
	}))
	defer hubHTTP.Close()

	// Closed local port so Step 1 (redbida-go) fails immediately and falls back to Node-RED files
	closedSrv := httptest.NewServer(http.NotFoundHandler())
	closedPort := serverPortFromURL(t, closedSrv.URL)
	closedSrv.Close()

	keyDir := t.TempDir()
	stateDir := t.TempDir()
	writeCounter := func(name string, v int64) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(keyDir, name), []byte(strconv.FormatInt(v, 10)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Giant pre-existing historical baselines on legacy box
	writeCounter("view_count", 50000)
	writeCounter("api_count", 20000)
	writeCounter("download_count", 3000)
	writeCounter("packed_count", 1200)
	writeCounter("api_model_count", 450)

	box := hubSrv.Store().Boxes()[0]
	agent := NewEdgeAgent(AgentConfig{
		HubURL:            hubHTTP.URL,
		BoxID:             box.ID,
		Secret:            testAPIKey,
		RedBidaPort:       closedPort,
		RedBidaKeyDir:     keyDir,
		AnalyticsStateDir: stateDir,
	})

	ctx := context.Background()
	now := time.Now()

	// 1. First run: MUST save baseline and emit ZERO requests (prevent historical spike!)
	if err := agent.pushAnalyticsAt(ctx, now); err != nil {
		t.Fatalf("baseline run failed: %v", err)
	}
	if ingestHits.Load() != 0 {
		t.Fatalf("expected 0 ingest requests on first baseline run, got %d", ingestHits.Load())
	}
	if _, err := os.Stat(filepath.Join(stateDir, analyticsOffsetFilename)); err != nil {
		t.Fatalf("expected analytics_offset.json to be written: %v", err)
	}

	// 2. Increment counters -> emits exact positive deltas
	writeCounter("view_count", 50010)     // +10 -> page_views
	writeCounter("api_count", 20007)      // +7  -> qr_views
	writeCounter("download_count", 3003)  // +3  -> clip_downloads
	writeCounter("packed_count", 1202)    // +2  -> cut_clips
	writeCounter("api_model_count", 451)  // +1  -> os_app

	if err := agent.pushAnalyticsAt(ctx, now); err != nil {
		t.Fatalf("delta run failed: %v", err)
	}
	if ingestHits.Load() != 1 {
		t.Fatalf("expected 1 ingest request after delta, got %d", ingestHits.Load())
	}

	// 3. No counter change -> emits ZERO additional requests
	if err := agent.pushAnalyticsAt(ctx, now); err != nil {
		t.Fatalf("idle run failed: %v", err)
	}
	if ingestHits.Load() != 1 {
		t.Fatalf("expected no extra ingest request when counters unchanged, got %d", ingestHits.Load())
	}

	// 4. Counter reset (api_count drops from 20007 -> 5) -> graceful reset emits delta = 5
	writeCounter("api_count", 5)
	if err := agent.pushAnalyticsAt(ctx, now); err != nil {
		t.Fatalf("counter reset run failed: %v", err)
	}
	if ingestHits.Load() != 2 {
		t.Fatalf("expected 2nd ingest request after counter reset, got %d", ingestHits.Load())
	}

	ov, err := hubSrv.queryAnalyticsOverview(analyticsQueryParams{Range: "7d", BoxID: box.ID}, now)
	if err != nil {
		t.Fatalf("queryAnalyticsOverview failed: %v", err)
	}
	if ov.KPI.PageViews != 10 || ov.KPI.QRViews != 12 || ov.KPI.ClipDownloads != 3 || ov.KPI.CutClips != 2 || ov.OSBreakdown.App != 1 {
		t.Fatalf("unexpected Node-RED totals in Hub: kpi=%+v os=%+v", ov.KPI, ov.OSBreakdown)
	}
}

func TestEdgeAgentAnalytics_OfflineSpoolAndOldestFirstDrain(t *testing.T) {
	hubSrv, err := NewServer(Config{
		DataPath:     ":memory:",
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer hubSrv.Close()

	var hubDown atomic.Bool
	hubDown.Store(true)

	var mu sync.Mutex
	var receivedBatchIDs []string

	hubHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hubDown.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/fleet/api/analytics/ingest" {
			// Peek batch_id order while forwarding to real hub handler
			var peek struct {
				BatchID string `json:"batch_id"`
			}
			// Re-buffer body
			var raw json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&raw); err == nil {
				_ = json.Unmarshal(raw, &peek)
				mu.Lock()
				receivedBatchIDs = append(receivedBatchIDs, peek.BatchID)
				mu.Unlock()
				// Forward directly via IngestAnalytics
				var p AnalyticsIngestPayload
				_ = json.Unmarshal(raw, &p)
				res, err := hubSrv.IngestAnalytics(p, time.Now())
				if err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, res)
				return
			}
		}
		hubSrv.ServeHTTP(w, r)
	}))
	defer hubHTTP.Close()

	closedSrv := httptest.NewServer(http.NotFoundHandler())
	closedPort := serverPortFromURL(t, closedSrv.URL)
	closedSrv.Close()

	keyDir := t.TempDir()
	stateDir := t.TempDir()
	writeAPI := func(v int64) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(keyDir, "api_count"), []byte(strconv.FormatInt(v, 10)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	box := hubSrv.Store().Boxes()[0]
	agent := NewEdgeAgent(AgentConfig{
		HubURL:            hubHTTP.URL,
		BoxID:             box.ID,
		Secret:            testAPIKey,
		RedBidaPort:       closedPort,
		RedBidaKeyDir:     keyDir,
		AnalyticsStateDir: stateDir,
	})

	ctx := context.Background()
	now := time.Now()

	// Baseline
	writeAPI(100)
	if err := agent.pushAnalyticsAt(ctx, now); err != nil {
		t.Fatalf("baseline failed: %v", err)
	}

	// 3 pushes while Hub is offline -> spooled to disk
	for i, v := range []int64{105, 112, 120} {
		writeAPI(v)
		if err := agent.pushAnalyticsAt(ctx, now); err == nil {
			t.Fatalf("expected error while hub offline on push #%d", i+1)
		}
	}

	spooled := agent.loadAnalyticsSpool()
	if len(spooled) != 3 {
		t.Fatalf("expected 3 spooled batches on disk, got %d", len(spooled))
	}

	// Bring Hub back online -> next push drains oldest-first and clears spool!
	hubDown.Store(false)
	if err := agent.pushAnalyticsAt(ctx, now); err != nil {
		t.Fatalf("spool drain failed: %v", err)
	}

	mu.Lock()
	gotIDs := append([]string(nil), receivedBatchIDs...)
	mu.Unlock()

	if len(gotIDs) != 3 {
		t.Fatalf("expected 3 drained batches, got %v", gotIDs)
	}
	for i := 0; i < 3; i++ {
		if gotIDs[i] != spooled[i].BatchID {
			t.Fatalf("expected oldest-first drain order: idx %d got %q want %q", i, gotIDs[i], spooled[i].BatchID)
		}
	}
	if _, err := os.Stat(filepath.Join(stateDir, analyticsSpoolFilename)); !os.IsNotExist(err) {
		t.Fatalf("expected spool file removed after full drain, stat err=%v", err)
	}

	ov, err := hubSrv.queryAnalyticsOverview(analyticsQueryParams{Range: "7d", BoxID: box.ID}, now)
	if err != nil {
		t.Fatalf("queryAnalyticsOverview failed: %v", err)
	}
	if ov.KPI.QRViews != 20 { // +5 +7 +8 = 20
		t.Fatalf("expected summed spooled QRViews=20, got %d", ov.KPI.QRViews)
	}
}

func TestEdgeAgentAnalytics_SpoolBounds(t *testing.T) {
	items := make([]AnalyticsIngestPayload, 60)
	for i := range items {
		items[i] = AnalyticsIngestPayload{
			BoxID:   "inut_test",
			Source:  "nodered",
			BatchID: "b:" + strconv.Itoa(i),
		}
	}
	bounded := boundAnalyticsSpool(items)
	if len(bounded) != maxAnalyticsSpoolEntries {
		t.Fatalf("expected %d entries, got %d", maxAnalyticsSpoolEntries, len(bounded))
	}
	if bounded[0].BatchID != "b:12" || bounded[len(bounded)-1].BatchID != "b:59" {
		t.Fatalf("expected newest 48 entries kept (b:12..b:59), got %q..%q", bounded[0].BatchID, bounded[len(bounded)-1].BatchID)
	}

	// Verify byte cap (256 KB) drops oldest entries
	bigItems := make([]AnalyticsIngestPayload, 20)
	for i := range bigItems {
		hourly := make([]AnalyticsBoxHourlyIn, 60)
		for j := range hourly {
			hourly[j] = AnalyticsBoxHourlyIn{BucketHour: "2026-11-22 10:00", AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: int64(i*100 + j)}}
		}
		bigItems[i] = AnalyticsIngestPayload{
			BoxID:       "inut_big",
			Source:      "redbida-go",
			SourceEpoch: "ep-" + strconv.Itoa(i),
			BoxHourly:   hourly,
		}
	}
	boundedBig := boundAnalyticsSpool(bigItems)
	raw, _ := json.Marshal(boundedBig)
	if len(raw) > maxAnalyticsSpoolBytes {
		t.Fatalf("expected spool bytes <= %d, got %d", maxAnalyticsSpoolBytes, len(raw))
	}
	if boundedBig[len(boundedBig)-1].SourceEpoch != "ep-19" {
		t.Fatalf("expected newest entry preserved under byte cap, got %q", boundedBig[len(boundedBig)-1].SourceEpoch)
	}
}
