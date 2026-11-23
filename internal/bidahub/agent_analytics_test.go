package bidahub

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
			// Peek batch_id order while forwarding to real hub handler (transparently gunzip!)
			var peek struct {
				BatchID string `json:"batch_id"`
			}
			var bodyReader io.Reader = r.Body
			if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
				gz, gzErr := gzip.NewReader(r.Body)
				if gzErr != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				defer gz.Close()
				bodyReader = gz
			}
			var raw json.RawMessage
			if err := json.NewDecoder(bodyReader).Decode(&raw); err == nil {
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

func TestEdgeAgentAnalytics_ZeroChange204SkipAndGzipDeltaCompression(t *testing.T) {
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

	var mu sync.Mutex
	var ingestBytes []int
	var ingestEncodings []string
	hubHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fleet/api/analytics/ingest" {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			ingestBytes = append(ingestBytes, len(body))
			ingestEncodings = append(ingestEncodings, r.Header.Get("Content-Encoding"))
			mu.Unlock()
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		hubSrv.ServeHTTP(w, r)
	}))
	defer hubHTTP.Close()

	box := hubSrv.Store().Boxes()[0]
	nowLocal := time.Now().In(analyticsTZ)
	today := nowLocal.Format("2006-01-02")
	curHour := nowLocal.Format("2006-01-02 15:00")

	var currentRev atomic.Uint64
	currentRev.Store(10)
	var seenSinceRevs []string

	// Build a realistic 48h + 14d x 16-cam full snapshot to measure size vs dirty-only gzip delta.
	buildFullSnapshot := func(rev uint64) AnalyticsIngestPayload {
		hourly := make([]AnalyticsBoxHourlyIn, 48)
		for i := 0; i < 48; i++ {
			hr := nowLocal.Add(-time.Duration(47-i) * time.Hour).Format("2006-01-02 15:00")
			hourly[i] = AnalyticsBoxHourlyIn{
				BucketHour: hr,
				AnalyticsBoxMetrics: AnalyticsBoxMetrics{
					QRViews: 10 + int64(i), PageViews: 15 + int64(i), UniqueVisitors: 5,
					Sessions: 6, EngagedSessions: 4, ReturningVisitors: 2,
					AcqQR: 4, AcqShare: 1, AcqApp: 1, AcqDirect: 0,
					LiveHLS: 3, VODPlays: 2, ClipDownloads: 2, CutClips: 1,
					AIAnalyzes: 1, AIPickleballRuns: 1, BytesOut: 1_000_000,
				},
			}
		}
		daily := make([]AnalyticsBoxDailyIn, 14)
		cams := make([]AnalyticsCamDailyIn, 0, 14*16)
		for d := 0; d < 14; d++ {
			ds := nowLocal.AddDate(0, 0, -(13 - d)).Format("2006-01-02")
			daily[d] = AnalyticsBoxDailyIn{BucketDate: ds, UniqueVisitors: 20, Sessions: 25, EngagedSessions: 18, ReturningVisitors: 7}
			for c := 1; c <= 16; c++ {
				cams = append(cams, AnalyticsCamDailyIn{
					BucketDate: ds, CameraID: "camera" + strconv.Itoa(c), TableName: "Bàn " + strconv.Itoa(c),
					AnalyticsCamMetrics: AnalyticsCamMetrics{QRViews: 12, UniqueVisitors: 6, ClipDownloads: 2, CutClips: 1, BytesOut: 2_500_000},
				})
			}
		}
		return AnalyticsIngestPayload{
			Source:      "redbida-go",
			SourceEpoch: "ep-delta-1",
			GeneratedAt: nowLocal.Format(time.RFC3339),
			AIEnabled:   true,
			Rev:         rev,
			IsDelta:     false,
			BoxHourly:   hourly,
			BoxDaily:    daily,
			CameraDaily: cams,
		}
	}

	fullRaw, _ := json.Marshal(buildFullSnapshot(10))
	if len(fullRaw) < 40_000 {
		t.Fatalf("expected realistic full snapshot >40KB, got %d bytes", len(fullRaw))
	}

	redbidaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/analytics/delta" {
			http.NotFound(w, r)
			return
		}
		sr := r.URL.Query().Get("since_rev")
		mu.Lock()
		seenSinceRevs = append(seenSinceRevs, sr)
		mu.Unlock()
		since, _ := strconv.ParseUint(sr, 10, 64)
		cur := currentRev.Load()
		if since > 0 && since == cur {
			w.WriteHeader(http.StatusNoContent) // 204 zero-change skip!
			return
		}
		if since == 0 || since > cur {
			writeJSON(w, http.StatusOK, buildFullSnapshot(cur))
			return
		}
		// Dirty-only delta (1 hour + 1 day + 1 camera)
		writeJSON(w, http.StatusOK, AnalyticsIngestPayload{
			Source:      "redbida-go",
			SourceEpoch: "ep-delta-1",
			GeneratedAt: nowLocal.Format(time.RFC3339),
			AIEnabled:   true,
			Rev:         cur,
			IsDelta:     true,
			BoxHourly: []AnalyticsBoxHourlyIn{
				{
					BucketHour: curHour,
					AnalyticsBoxMetrics: AnalyticsBoxMetrics{
						QRViews: 65, PageViews: 75, UniqueVisitors: 12,
						Sessions: 14, EngagedSessions: 10, ReturningVisitors: 5,
						AcqQR: 9, AcqShare: 3, AcqApp: 1, AcqDirect: 1,
						ClipDownloads: 5, CutClips: 3, AIAnalyzes: 4, AIPickleballRuns: 2,
						BytesOut: 4_000_000,
					},
				},
			},
			BoxDaily: []AnalyticsBoxDailyIn{
				{BucketDate: today, UniqueVisitors: 25, Sessions: 30, EngagedSessions: 22, ReturningVisitors: 9},
			},
			CameraDaily: []AnalyticsCamDailyIn{
				{
					BucketDate: today, CameraID: "camera1", TableName: "Bàn 1",
					AnalyticsCamMetrics: AnalyticsCamMetrics{QRViews: 20, UniqueVisitors: 10, ClipDownloads: 4, CutClips: 2},
				},
			},
		})
	}))
	defer redbidaMock.Close()

	stateDir := t.TempDir()
	agent := NewEdgeAgent(AgentConfig{
		HubURL:            hubHTTP.URL,
		BoxID:             box.ID,
		Secret:            testAPIKey,
		RedBidaPort:       serverPortFromURL(t, redbidaMock.URL),
		RedBidaKeyDir:     t.TempDir(),
		AnalyticsStateDir: stateDir,
	})

	ctx := context.Background()

	// 1. Initial sync (since_rev=0) -> sends gzipped full snapshot and saves last_acked_rev=10
	if err := agent.PushAnalyticsOnce(ctx); err != nil {
		t.Fatalf("initial push failed: %v", err)
	}
	st := agent.loadAnalyticsState()
	if st.LastAckedRev != 10 || st.SourceEpoch != "ep-delta-1" {
		t.Fatalf("expected persisted last_acked_rev=10 ep-delta-1, got %+v", st)
	}

	// 2. Idle cycle (rev still 10 -> 204 No Content) -> MUST send 0 HTTP requests / 0 bytes!
	if err := agent.PushAnalyticsOnce(ctx); err != nil {
		t.Fatalf("idle push failed: %v", err)
	}
	mu.Lock()
	hitsAfterIdle := len(ingestBytes)
	mu.Unlock()
	if hitsAfterIdle != 1 {
		t.Fatalf("expected 0 extra Hub requests on 204 No Content, total hits=%d", hitsAfterIdle)
	}

	// 3. New activity (rev=11) -> sends ONLY dirty-bucket delta compressed with gzip (<1.5KB vs >40KB full!)
	currentRev.Store(11)
	if err := agent.PushAnalyticsOnce(ctx); err != nil {
		t.Fatalf("delta push failed: %v", err)
	}

	mu.Lock()
	if len(ingestBytes) != 2 {
		t.Fatalf("expected 2 total ingest hits, got %d", len(ingestBytes))
	}
	if ingestEncodings[0] != "gzip" || ingestEncodings[1] != "gzip" {
		t.Fatalf("expected Content-Encoding: gzip on all ingests, got %v", ingestEncodings)
	}
	deltaWireBytes := ingestBytes[1]
	mu.Unlock()

	if deltaWireBytes >= 1500 {
		t.Fatalf("expected gzipped dirty-bucket delta < 1500 bytes (vs %d full raw), got %d bytes", len(fullRaw), deltaWireBytes)
	}
	if st2 := agent.loadAnalyticsState(); st2.LastAckedRev != 11 {
		t.Fatalf("expected persisted last_acked_rev=11, got %+v", st2)
	}

	ov, err := hubSrv.queryAnalyticsOverview(analyticsQueryParams{Range: "today", BoxID: box.ID}, time.Now())
	if err != nil {
		t.Fatalf("queryAnalyticsOverview failed: %v", err)
	}
	if ov.KPI.UniqueVisitors != 25 || ov.KPI.Sessions != 30 || ov.KPI.EngagedSessions != 22 || ov.KPI.ReturningVisitors != 9 {
		t.Fatalf("unexpected Hub GA4 KPI after dirty delta: %+v", ov.KPI)
	}
	if ov.Acquisition.QR < 9 || ov.Acquisition.Share < 3 || ov.KPI.AIPickleballRuns < 2 {
		t.Fatalf("expected acquisition & pickleball counters persisted, got acq=%+v pb=%d", ov.Acquisition, ov.KPI.AIPickleballRuns)
	}
}

func TestEdgeAgentAnalytics_NeedFullSyncResetsLastAckedRev(t *testing.T) {
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
	curHour := nowLocal.Format("2006-01-02 15:00")

	var mu sync.Mutex
	var requestedRevs []uint64
	redbidaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseUint(r.URL.Query().Get("since_rev"), 10, 64)
		mu.Lock()
		requestedRevs = append(requestedRevs, since)
		mu.Unlock()
		writeJSON(w, http.StatusOK, AnalyticsIngestPayload{
			Source:      "redbida-go",
			SourceEpoch: "ep-unknown-to-hub",
			Rev:         21,
			IsDelta:     since > 0,
			BoxHourly: []AnalyticsBoxHourlyIn{
				{BucketHour: curHour, AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: 15}},
			},
		})
	}))
	defer redbidaMock.Close()

	stateDir := t.TempDir()
	agent := NewEdgeAgent(AgentConfig{
		HubURL:            hubHTTP.URL,
		BoxID:             box.ID,
		Secret:            testAPIKey,
		RedBidaPort:       serverPortFromURL(t, redbidaMock.URL),
		RedBidaKeyDir:     t.TempDir(),
		AnalyticsStateDir: stateDir,
	})
	// Pre-seed state as if agent previously synced to another Hub
	_ = agent.saveAnalyticsState(analyticsAgentState{LastAckedRev: 20, SourceEpoch: "ep-unknown-to-hub"})

	ctx := context.Background()
	// Push 1: sends delta (since_rev=20); Hub sees unknown epoch on delta -> replies need_full_sync: true -> agent resets lastAckedRev=0!
	if err := agent.PushAnalyticsOnce(ctx); err != nil {
		t.Fatalf("first push failed: %v", err)
	}
	if st := agent.loadAnalyticsState(); st.LastAckedRev != 0 {
		t.Fatalf("expected need_full_sync to reset last_acked_rev=0, got %+v", st)
	}

	// Push 2: agent requests since_rev=0 (full snapshot), Hub marks epoch known -> lastAckedRev advances to 21!
	if err := agent.PushAnalyticsOnce(ctx); err != nil {
		t.Fatalf("second push failed: %v", err)
	}
	if st := agent.loadAnalyticsState(); st.LastAckedRev != 21 {
		t.Fatalf("expected last_acked_rev=21 after full sync, got %+v", st)
	}
	if len(requestedRevs) != 2 || requestedRevs[0] != 20 || requestedRevs[1] != 0 {
		t.Fatalf("expected delta request sequence [20, 0], got %v", requestedRevs)
	}
}

func TestEdgeAgentAnalytics_HeartbeatRealtimePiggybackAndIdleZeroBytes(t *testing.T) {
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

	var mu sync.Mutex
	var capturedBodies [][]byte
	hubHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fleet/api/heartbeat" {
			raw, _ := io.ReadAll(r.Body)
			mu.Lock()
			capturedBodies = append(capturedBodies, raw)
			mu.Unlock()
			r.Body = io.NopCloser(bytes.NewReader(raw))
		}
		hubSrv.ServeHTTP(w, r)
	}))
	defer hubHTTP.Close()

	var activeMode atomic.Bool
	redbidaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/analytics/realtime" {
			http.NotFound(w, r)
			return
		}
		if !activeMode.Load() {
			writeJSON(w, http.StatusOK, RealtimeBoxMetrics{A5: 0, A30: 0})
			return
		}
		m30 := make([]int, 30)
		m30[28] = 3
		m30[29] = 4
		writeJSON(w, http.StatusOK, RealtimeBoxMetrics{
			A5: 4, A30: 11, Live: 3, VOD: 1, Cut: 1, AI: 1,
			M30: m30,
			Hot: []RealtimeHotTable{{Cam: "camera02", Table: "Bàn 02 VIP", N: 2}},
		})
	}))
	defer redbidaMock.Close()

	box := hubSrv.Store().Boxes()[0]
	agent := NewEdgeAgent(AgentConfig{
		HubURL:            hubHTTP.URL,
		BoxID:             box.ID,
		Secret:            testAPIKey,
		RedBidaPort:       serverPortFromURL(t, redbidaMock.URL),
		RedBidaKeyDir:     t.TempDir(),
		AnalyticsStateDir: t.TempDir(),
	})

	ctx := context.Background()

	// 1. Idle (a30 == 0): heartbeat JSON MUST omit "rt" completely (0 extra bytes!)
	if err := agent.PushHeartbeatOnce(ctx); err != nil {
		t.Fatalf("idle PushHeartbeatOnce failed: %v", err)
	}
	mu.Lock()
	idleBody := string(capturedBodies[0])
	mu.Unlock()
	if strings.Contains(idleBody, `"rt"`) {
		t.Fatalf("expected idle heartbeat to omit \"rt\" (0 extra bytes), got: %s", idleBody)
	}

	// 2. Active (a30 == 11): heartbeat piggybacks compact "rt" (~120-250B) with 0 extra HTTP requests!
	activeMode.Store(true)
	if err := agent.PushHeartbeatOnce(ctx); err != nil {
		t.Fatalf("active PushHeartbeatOnce failed: %v", err)
	}
	mu.Lock()
	activeBody := string(capturedBodies[1])
	mu.Unlock()
	if !strings.Contains(activeBody, `"rt":{`) {
		t.Fatalf("expected active heartbeat to piggyback \"rt\", got: %s", activeBody)
	}
	rtRaw, _ := json.Marshal(RealtimeBoxMetrics{
		A5: 4, A30: 11, Live: 3, VOD: 1, Cut: 1, AI: 1,
		M30: append(make([]int, 28), 3, 4),
		Hot: []RealtimeHotTable{{Cam: "camera02", Table: "Bàn 02 VIP", N: 2}},
	})
	if len(rtRaw) > 260 {
		t.Fatalf("expected compact piggybacked rt <= 260 bytes, got %d bytes: %s", len(rtRaw), string(rtRaw))
	}

	// Verify Hub RAM realtime endpoint immediately reflects piggybacked metrics!
	rtResp := hubSrv.QueryAnalyticsRealtime(analyticsRealtimeFilter{BoxID: box.ID}, time.Now())
	if rtResp.Active5m != 4 || rtResp.Active30m != 11 || rtResp.Live != 3 || rtResp.VOD != 1 || rtResp.Cut != 1 || rtResp.AI != 1 {
		t.Fatalf("unexpected Hub realtime totals: %+v", rtResp)
	}
	if len(rtResp.MinuteBars30) != 30 || rtResp.MinuteBars30[28] != 3 || rtResp.MinuteBars30[29] != 4 {
		t.Fatalf("unexpected minute_bars_30: %v", rtResp.MinuteBars30)
	}
	if len(rtResp.HotTables) != 1 || rtResp.HotTables[0].TableName != "Bàn 02 VIP" || rtResp.HotTables[0].Active != 2 {
		t.Fatalf("unexpected hot_tables: %+v", rtResp.HotTables)
	}

	// 3. Back to idle -> next heartbeat clears box from Hub RAM realtime store immediately.
	activeMode.Store(false)
	if err := agent.PushHeartbeatOnce(ctx); err != nil {
		t.Fatalf("second idle PushHeartbeatOnce failed: %v", err)
	}
	rtAfter := hubSrv.QueryAnalyticsRealtime(analyticsRealtimeFilter{BoxID: box.ID}, time.Now())
	if rtAfter.Active5m != 0 || rtAfter.Active30m != 0 || rtAfter.ActiveBoxes != 0 {
		t.Fatalf("expected idle heartbeat to clear box realtime state, got %+v", rtAfter)
	}
}

func TestEdgeAgentAnalytics_OfflineSpoolMergesDuplicateRedBidaBuckets(t *testing.T) {
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
	var ingestHits atomic.Int32
	hubHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hubDown.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/fleet/api/analytics/ingest" {
			ingestHits.Add(1)
		}
		hubSrv.ServeHTTP(w, r)
	}))
	defer hubHTTP.Close()

	box := hubSrv.Store().Boxes()[0]
	nowLocal := time.Now().In(analyticsTZ)
	today := nowLocal.Format("2006-01-02")
	hr1 := nowLocal.Add(-time.Hour).Format("2006-01-02 15:00")
	hr2 := nowLocal.Format("2006-01-02 15:00")

	var step atomic.Int32
	redbidaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/analytics/delta" {
			http.NotFound(w, r)
			return
		}
		s := step.Load()
		writeJSON(w, http.StatusOK, AnalyticsIngestPayload{
			Source:      "redbida-go",
			SourceEpoch: "ep-merge-1",
			Rev:         uint64(s),
			IsDelta:     s > 1,
			AIEnabled:   true,
			BoxHourly: []AnalyticsBoxHourlyIn{
				{BucketHour: hr1, AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: int64(10 * s), Sessions: int64(5 * s)}},
				{BucketHour: hr2, AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: int64(4 * s), AIAnalyzes: int64(s)}},
			},
			BoxDaily: []AnalyticsBoxDailyIn{
				{BucketDate: today, UniqueVisitors: int64(8 * s), Sessions: int64(7 * s), ReturningVisitors: int64(2 * s)},
			},
			CameraDaily: []AnalyticsCamDailyIn{
				{BucketDate: today, CameraID: "camera01", TableName: "Bàn 1", AnalyticsCamMetrics: AnalyticsCamMetrics{QRViews: int64(6 * s)}},
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
	// 4 pushes while Hub is offline -> MUST merge into 1 single spooled payload (not 4 stacked snapshots!)
	for i := int32(1); i <= 4; i++ {
		step.Store(i)
		if err := agent.PushAnalyticsOnce(ctx); err == nil {
			t.Fatalf("expected error while hub offline on step %d", i)
		}
	}

	spooled := agent.loadAnalyticsSpool()
	if len(spooled) != 1 {
		t.Fatalf("expected offline redbida-go snapshots merged into 1 spool entry, got %d", len(spooled))
	}
	if len(spooled[0].BoxHourly) != 2 || spooled[0].BoxHourly[0].QRViews != 40 || spooled[0].BoxHourly[1].QRViews != 16 {
		t.Fatalf("expected merged max hourly buckets [40, 16], got %+v", spooled[0].BoxHourly)
	}
	if len(spooled[0].BoxDaily) != 1 || spooled[0].BoxDaily[0].UniqueVisitors != 32 || spooled[0].BoxDaily[0].ReturningVisitors != 8 {
		t.Fatalf("expected merged daily max (32, 8), got %+v", spooled[0].BoxDaily)
	}
	if len(spooled[0].CameraDaily) != 1 || spooled[0].CameraDaily[0].QRViews != 24 {
		t.Fatalf("expected merged camera daily max 24, got %+v", spooled[0].CameraDaily)
	}

	// Bring Hub online -> drains in ONE single request!
	hubDown.Store(false)
	if err := agent.PushAnalyticsOnce(ctx); err != nil {
		t.Fatalf("drain merged spool failed: %v", err)
	}
	if ingestHits.Load() != 1 {
		t.Fatalf("expected 1 single compressed HTTP ingest after spool merge, got %d", ingestHits.Load())
	}
	ov, err := hubSrv.queryAnalyticsOverview(analyticsQueryParams{Range: "7d", BoxID: box.ID}, time.Now())
	if err != nil {
		t.Fatalf("overview failed: %v", err)
	}
	if ov.KPI.QRViews != 56 || ov.KPI.UniqueVisitors != 32 || ov.KPI.ReturningVisitors != 8 {
		t.Fatalf("unexpected Hub totals after merged spool drain: %+v", ov.KPI)
	}
}

func TestEdgeAgentAnalytics_DeterministicPhaseOffsetAnd429Backoff(t *testing.T) {
	off1a := analyticsPhaseOffset("inut_204_141", 5*time.Minute)
	off1b := analyticsPhaseOffset("inut_204_141", 5*time.Minute)
	off2 := analyticsPhaseOffset("inut_205_72", 5*time.Minute)
	if off1a != off1b {
		t.Fatalf("expected deterministic phase offset, got %v vs %v", off1a, off1b)
	}
	if off1a < 0 || off1a >= 5*time.Minute || off2 < 0 || off2 >= 5*time.Minute {
		t.Fatalf("phase offsets out of [0, 5m): %v, %v", off1a, off2)
	}
	if off1a == off2 {
		t.Fatalf("expected distinct boxes to spread phase offsets, both got %v", off1a)
	}

	var rateLimit atomic.Bool
	rateLimit.Store(true)
	var hubCalls atomic.Int32
	hubMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hubCalls.Add(1)
		if rateLimit.Load() {
			w.Header().Set("Retry-After", "45")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(w, http.StatusOK, AnalyticsIngestResult{OK: true})
	}))
	defer hubMock.Close()

	now := time.Now()
	curHour := now.In(analyticsTZ).Format("2006-01-02 15:00")
	redbidaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, AnalyticsIngestPayload{
			Source:      "redbida-go",
			SourceEpoch: "ep-429",
			Rev:         1,
			BoxHourly:   []AnalyticsBoxHourlyIn{{BucketHour: curHour, AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: 3}}},
		})
	}))
	defer redbidaMock.Close()

	agent := NewEdgeAgent(AgentConfig{
		HubURL:            hubMock.URL,
		BoxID:             "inut_test_429",
		Secret:            testAPIKey,
		RedBidaPort:       serverPortFromURL(t, redbidaMock.URL),
		RedBidaKeyDir:     t.TempDir(),
		AnalyticsStateDir: t.TempDir(),
	})

	ctx := context.Background()
	err := agent.pushAnalyticsAt(ctx, now)
	if d, ok := isAnalyticsRateLimited(err); !ok || d != 45*time.Second {
		t.Fatalf("expected 429 rate-limited error with 45s Retry-After, got err=%v d=%v ok=%v", err, d, ok)
	}

	// Within 45s backoff window -> MUST NOT hit Hub again!
	err2 := agent.pushAnalyticsAt(ctx, now.Add(10*time.Second))
	if _, ok := isAnalyticsRateLimited(err2); !ok {
		t.Fatalf("expected backoff to suppress Hub request at +10s, got err=%v", err2)
	}
	if hubCalls.Load() != 1 {
		t.Fatalf("expected Hub called once before backoff expired, got %d", hubCalls.Load())
	}

	// After 45s backoff expires -> drains spooled batch cleanly!
	rateLimit.Store(false)
	if err := agent.pushAnalyticsAt(ctx, now.Add(50*time.Second)); err != nil {
		t.Fatalf("expected push to succeed after backoff expiry: %v", err)
	}
	if hubCalls.Load() != 2 {
		t.Fatalf("expected 2nd Hub call after backoff expiry, got %d", hubCalls.Load())
	}
}
