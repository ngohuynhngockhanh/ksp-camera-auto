package bidahub

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAnalyticsMigration202611220001_InMemoryAndTempSQLite(t *testing.T) {
	paths := []struct {
		name string
		path string
	}{
		{name: "memory", path: ":memory:"},
		{name: "file", path: filepath.Join(t.TempDir(), "hub.sqlite")},
	}

	for _, tc := range paths {
		t.Run(tc.name, func(t *testing.T) {
			srv, err := NewServer(Config{
				DataPath:     tc.path,
				AdminUser:    testAdminUser,
				AdminPass:    testAdminPass,
				ViewerUser:   testViewerUser,
				ViewerPass:   testViewerPass,
				APIKey:       testAPIKey,
				PollInterval: 0,
			})
			if err != nil {
				t.Fatalf("NewServer(%q) failed: %v", tc.path, err)
			}
			defer srv.Close()

			db := srv.Store().SQLiteDB()
			if db == nil {
				t.Fatalf("expected non-nil SQLiteDB for %q", tc.path)
			}

			var migName string
			if err := db.QueryRow("SELECT name FROM schema_migrations WHERE version = 202611220001").Scan(&migName); err != nil {
				t.Fatalf("expected migration 202611220001 in schema_migrations: %v", err)
			}
			if migName != "add_usage_analytics_tables" {
				t.Errorf("unexpected migration name: %q", migName)
			}

			for _, tbl := range []string{"usage_box_hourly", "usage_box_daily", "usage_camera_daily", "analytics_ingest_batches"} {
				var count int
				if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&count); err != nil || count != 1 {
					t.Errorf("expected table %q to exist (count=%d, err=%v)", tbl, count, err)
				}
			}
			for _, idx := range []string{
				"idx_usage_box_hourly_hour",
				"idx_usage_box_hourly_club_hour",
				"idx_usage_box_daily_date",
				"idx_usage_box_daily_club_date",
				"idx_usage_cam_daily_date",
				"idx_usage_cam_daily_club_date",
			} {
				var count int
				if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&count); err != nil || count != 1 {
					t.Errorf("expected index %q to exist (count=%d, err=%v)", idx, count, err)
				}
			}
		})
	}
}

func TestAnalyticsIngest_AuthAndValidation(t *testing.T) {
	srv, _ := newTestHubServer(t)
	boxes := srv.Store().Boxes()
	if len(boxes) == 0 {
		t.Fatalf("expected seeded boxes")
	}
	boxID := boxes[0].ID

	validPayload := AnalyticsIngestPayload{
		BoxID:       boxID,
		Source:      "redbida-go",
		SourceEpoch: "ep-auth-1",
		AIEnabled:   true,
		BoxHourly: []AnalyticsBoxHourlyIn{
			{BucketHour: "2026-11-22 10:00", AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: 5}},
		},
	}

	// 1. Missing secret -> 401
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/analytics/ingest", validPayload, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without secret, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Wrong X-Box-Secret -> 403
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/analytics/ingest", validPayload, map[string]string{
		"NO_AUTH":    "1",
		"X-Box-ID":     boxID,
		"X-Box-Secret": "wrong-secret-xyz",
	})
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401/403 with wrong secret, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Viewer session -> 403
	viewerSess := srv.Store().CreateSession(testViewerUser, "viewer", "127.0.0.1", time.Hour)
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/analytics/ingest", validPayload, map[string]string{
		"NO_AUTH": "1",
		"Cookie":    SessionCookieName + "=" + viewerSess.Token,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer session, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Valid X-Box-Secret -> 200 OK
	rec, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/analytics/ingest", validPayload, map[string]string{
		"NO_AUTH":    "1",
		"X-Box-ID":     boxID,
		"X-Box-Secret": testAPIKey,
	})
	if rec.Code != http.StatusOK || data["ok"] != true {
		t.Fatalf("expected 200 OK with valid X-Box-Secret, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Alias /api/analytics/ingest with X-Hub-Key -> 200 OK
	rec, data = doHubRequest(t, srv, http.MethodPost, "/api/analytics/ingest", validPayload, nil)
	if rec.Code != http.StatusOK || data["ok"] != true {
		t.Fatalf("expected 200 OK on /api/analytics/ingest, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Missing box_id -> 400
	badBox := validPayload
	badBox.BoxID = ""
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/analytics/ingest", badBox, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing box_id, got %d", rec.Code)
	}

	// 7. Node-RED without batch_id -> 400
	badNR := AnalyticsIngestPayload{BoxID: boxID, Source: "nodered"}
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/analytics/ingest", badNR, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for nodered missing batch_id, got %d", rec.Code)
	}

	// 8. Non-POST -> 405
	rec, _ = doHubRequest(t, srv, http.MethodGet, "/fleet/api/analytics/ingest", nil, nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET ingest, got %d", rec.Code)
	}
}

func TestAnalyticsIngest_RedBidaIdempotencyEpochRestartAndExactDailyUnique(t *testing.T) {
	srv, err := NewServer(Config{
		DataPath:     ":memory:",
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer(:memory:) failed: %v", err)
	}
	defer srv.Close()

	box := srv.Store().Boxes()[0]
	now := time.Date(2026, 11, 22, 14, 30, 0, 0, analyticsTZ)

	snap1 := AnalyticsIngestPayload{
		BoxID:       box.ID,
		Source:      "redbida-go",
		SourceEpoch: "epoch-A",
		AIEnabled:   true,
		BoxHourly: []AnalyticsBoxHourlyIn{
			{
				BucketHour: "2026-11-22 10:00",
				AnalyticsBoxMetrics: AnalyticsBoxMetrics{
					QRViews: 12, PageViews: 18, UniqueVisitors: 8,
					LiveHLS: 4, VODPlays: 6, ClipDownloads: 3, CutClips: 2, CutSeconds: 180,
					AIStudioOpens: 3, AIAnalyzes: 2, AIShots: 14, AIPots: 9, AIPoolRuns: 2,
					AIExportsIOS: 1, AIExportRelay: 1, BytesOut: 5_000_000, OSIOS: 5, OSAndroid: 3,
				},
			},
			{
				BucketHour: "2026-11-22 11:00",
				AnalyticsBoxMetrics: AnalyticsBoxMetrics{
					QRViews: 10, PageViews: 15, UniqueVisitors: 7,
					ClipDownloads: 4, CutClips: 1, CutSeconds: 90,
					AIStudioOpens: 2, AIAnalyzes: 1, AIShots: 6, AIPots: 4, AICaromRuns: 1,
					AIExportsDesktop: 1, BytesOut: 3_000_000, OSDesktop: 2, OSAndroid: 5,
				},
			},
		},
		BoxDaily: []AnalyticsBoxDailyIn{
			// Exact deduplicated box-level daily unique = 10 (less than hourly sum 8+7=15!)
			{BucketDate: "2026-11-22", UniqueVisitors: 10},
		},
		CameraDaily: []AnalyticsCamDailyIn{
			{
				BucketDate: "2026-11-22",
				CameraID:   "camera01",
				TableName:  "Bàn 01 VIP",
				AnalyticsCamMetrics: AnalyticsCamMetrics{
					QRViews: 15, UniqueVisitors: 6, ClipDownloads: 5, CutClips: 2, CutSeconds: 180,
					AIStudioOpens: 3, AIAnalyzes: 2, AIShots: 14, AIPots: 9, AIExports: 1, BytesOut: 4_000_000,
				},
			},
		},
	}

	// Retry exact same snapshot 3 times -> MUST NOT double-count!
	for i := 0; i < 3; i++ {
		if _, err := srv.IngestAnalytics(snap1, now); err != nil {
			t.Fatalf("ingest #%d failed: %v", i, err)
		}
	}

	ov, err := srv.queryAnalyticsOverview(analyticsQueryParams{Range: "7d", BoxID: box.ID}, now)
	if err != nil {
		t.Fatalf("queryAnalyticsOverview failed: %v", err)
	}
	if ov.KPI.QRViews != 22 {
		t.Fatalf("expected QRViews=22 (no double-count), got %d", ov.KPI.QRViews)
	}
	if ov.KPI.UniqueVisitors != 10 {
		t.Fatalf("expected exact daily UniqueVisitors=10 (not hourly sum 15), got %d", ov.KPI.UniqueVisitors)
	}
	if ov.KPI.ClipDownloads != 7 || ov.KPI.CutClips != 3 || ov.KPI.CutSeconds != 270 {
		t.Fatalf("unexpected download/cut totals: dl=%d cut=%d sec=%d", ov.KPI.ClipDownloads, ov.KPI.CutClips, ov.KPI.CutSeconds)
	}
	if ov.KPI.AIAnalyzes != 3 || ov.KPI.AIShots != 20 || ov.KPI.AIPots != 13 {
		t.Fatalf("unexpected AI totals: %+v", ov.KPI)
	}

	// Send stale/lower numbers under SAME epoch-A -> counters MUST NOT regress!
	stale := snap1
	stale.BoxHourly = []AnalyticsBoxHourlyIn{
		{BucketHour: "2026-11-22 10:00", AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: 3, UniqueVisitors: 2}},
	}
	if _, err := srv.IngestAnalytics(stale, now); err != nil {
		t.Fatalf("stale ingest failed: %v", err)
	}
	ov, _ = srv.queryAnalyticsOverview(analyticsQueryParams{Range: "7d", BoxID: box.ID}, now)
	if ov.KPI.QRViews != 22 || ov.KPI.UniqueVisitors != 10 {
		t.Fatalf("stale ingest regressed counters: qr=%d uniq=%d", ov.KPI.QRViews, ov.KPI.UniqueVisitors)
	}

	// Simulate checkpoint wipe & restart -> NEW epoch-B! Previous totals freeze into base_json, ZERO data loss!
	snapEpochB := AnalyticsIngestPayload{
		BoxID:       box.ID,
		Source:      "redbida-go",
		SourceEpoch: "epoch-B",
		AIEnabled:   true,
		BoxHourly: []AnalyticsBoxHourlyIn{
			{
				BucketHour: "2026-11-22 10:00",
				AnalyticsBoxMetrics: AnalyticsBoxMetrics{
					QRViews: 5, PageViews: 7, UniqueVisitors: 3, ClipDownloads: 2,
				},
			},
		},
		BoxDaily: []AnalyticsBoxDailyIn{
			{BucketDate: "2026-11-22", UniqueVisitors: 3},
		},
		CameraDaily: []AnalyticsCamDailyIn{
			{
				BucketDate: "2026-11-22",
				CameraID:   "camera01",
				AnalyticsCamMetrics: AnalyticsCamMetrics{
					QRViews: 4, UniqueVisitors: 2, ClipDownloads: 1, AIAnalyzes: 1,
				},
			},
		},
	}

	// Push epoch-B TWICE (verify epoch transition preserves prior totals AND retries under epoch-B don't double-count!)
	for i := 0; i < 2; i++ {
		if _, err := srv.IngestAnalytics(snapEpochB, now.Add(time.Minute)); err != nil {
			t.Fatalf("epoch-B ingest #%d failed: %v", i, err)
		}
	}

	ov, _ = srv.queryAnalyticsOverview(analyticsQueryParams{Range: "7d", BoxID: box.ID}, now)
	if ov.KPI.QRViews != 27 { // 22 + 5
		t.Fatalf("expected QRViews=27 after epoch-B restart, got %d", ov.KPI.QRViews)
	}
	if ov.KPI.ClipDownloads != 9 { // 7 + 2
		t.Fatalf("expected ClipDownloads=9 after epoch-B restart, got %d", ov.KPI.ClipDownloads)
	}
	if ov.KPI.UniqueVisitors != 13 { // 10 frozen + 3 new epoch
		t.Fatalf("expected UniqueVisitors=13 after epoch-B restart, got %d", ov.KPI.UniqueVisitors)
	}

	cams, err := srv.queryAnalyticsCameras(analyticsCameraQueryParams{Range: "7d", BoxID: box.ID}, now)
	if err != nil || len(cams) != 1 {
		t.Fatalf("expected 1 camera row, got len=%d err=%v", len(cams), err)
	}
	if cams[0].QRViews != 19 || cams[0].ClipDownloads != 6 || cams[0].AIAnalyzes != 3 || cams[0].TableName != "Bàn 01 VIP" {
		t.Fatalf("unexpected camera metrics after epoch-B: %+v", cams[0])
	}
}

func TestAnalyticsIngest_NodeRedDeltaIdempotencyAndUpgradeToRedBida(t *testing.T) {
	srv, err := NewServer(Config{
		DataPath:     ":memory:",
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer(:memory:) failed: %v", err)
	}
	defer srv.Close()

	box := srv.Store().Boxes()[0]
	now := time.Date(2026, 11, 22, 15, 0, 0, 0, analyticsTZ)

	batch1 := AnalyticsIngestPayload{
		BoxID:   box.ID,
		Source:  "nodered",
		BatchID: box.ID + ":2026-11-22 15:00:1",
		BoxHourly: []AnalyticsBoxHourlyIn{
			{
				BucketHour: "2026-11-22 15:00",
				AnalyticsBoxMetrics: AnalyticsBoxMetrics{
					QRViews: 10, PageViews: 20, ClipDownloads: 4, CutClips: 2, OSApp: 3,
				},
			},
		},
	}

	// First push -> accepted
	res1, err := srv.IngestAnalytics(batch1, now)
	if err != nil || res1.Deduped {
		t.Fatalf("expected non-deduped first batch, got res=%+v err=%v", res1, err)
	}
	// Duplicate replay -> deduped!
	resDup, err := srv.IngestAnalytics(batch1, now)
	if err != nil || !resDup.Deduped {
		t.Fatalf("expected deduped=true on replay, got res=%+v err=%v", resDup, err)
	}

	// Second distinct delta batch -> adds cleanly
	batch2 := batch1
	batch2.BatchID = box.ID + ":2026-11-22 15:00:2"
	batch2.BoxHourly = []AnalyticsBoxHourlyIn{
		{
			BucketHour: "2026-11-22 15:00",
			AnalyticsBoxMetrics: AnalyticsBoxMetrics{
				QRViews: 5, PageViews: 8, ClipDownloads: 1, CutClips: 1, OSApp: 2,
			},
		},
	}
	res2, err := srv.IngestAnalytics(batch2, now)
	if err != nil || res2.Deduped {
		t.Fatalf("expected second batch accepted, got res=%+v err=%v", res2, err)
	}

	ov, _ := srv.queryAnalyticsOverview(analyticsQueryParams{Range: "24h", BoxID: box.ID}, now)
	if ov.KPI.QRViews != 15 || ov.KPI.PageViews != 28 || ov.KPI.ClipDownloads != 5 || ov.KPI.CutClips != 3 || ov.OSBreakdown.App != 5 {
		t.Fatalf("unexpected nodered totals: kpi=%+v os=%+v", ov.KPI, ov.OSBreakdown)
	}

	// Mid-hour upgrade from Node-RED to redbida-go -> MUST preserve earlier 15 QR views!
	upgradeSnap := AnalyticsIngestPayload{
		BoxID:       box.ID,
		Source:      "redbida-go",
		SourceEpoch: "ep-upgraded-1",
		AIEnabled:   true,
		BoxHourly: []AnalyticsBoxHourlyIn{
			{
				BucketHour: "2026-11-22 15:00",
				AnalyticsBoxMetrics: AnalyticsBoxMetrics{
					QRViews: 6, PageViews: 10, ClipDownloads: 2, AIAnalyzes: 1,
				},
			},
		},
	}
	if _, err := srv.IngestAnalytics(upgradeSnap, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("upgrade snapshot failed: %v", err)
	}
	// Retry upgrade snapshot -> MUST NOT double-count!
	if _, err := srv.IngestAnalytics(upgradeSnap, now.Add(6*time.Minute)); err != nil {
		t.Fatalf("retry upgrade snapshot failed: %v", err)
	}

	ov, _ = srv.queryAnalyticsOverview(analyticsQueryParams{Range: "7d", BoxID: box.ID}, now)
	if ov.KPI.QRViews != 21 || ov.KPI.PageViews != 38 || ov.KPI.ClipDownloads != 7 || ov.KPI.AIAnalyzes != 1 {
		t.Fatalf("expected Node-RED counts preserved after redbida-go upgrade, got kpi=%+v", ov.KPI)
	}
}

func TestAnalyticsOverviewCamerasCSVAndPortalStrictIsolation(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Pick two boxes from two DIFFERENT clubs
	allBoxes := srv.Store().Boxes()
	var boxA, boxB *Box
	for _, b := range allBoxes {
		if boxA == nil {
			boxA = b
		} else if b.ClubID != boxA.ClubID {
			boxB = b
			break
		}
	}
	if boxA == nil || boxB == nil {
		t.Fatalf("expected at least 2 distinct clubs in seeded watchlist")
	}
	clubA, _ := srv.Store().GetClub(boxA.ClubID)
	clubB, _ := srv.Store().GetClub(boxB.ClubID)

	nowLocal := time.Now().In(analyticsTZ)
	today := nowLocal.Format("2006-01-02")
	curHour := nowLocal.Format("2006-01-02 15:00")
	curHrInt := nowLocal.Hour()

	// Seed Club A (redbida-go + AI enabled)
	_, err := srv.IngestAnalytics(AnalyticsIngestPayload{
		BoxID:       boxA.ID,
		Source:      "redbida-go",
		SourceEpoch: "ep-a",
		AIEnabled:   true,
		BoxHourly: []AnalyticsBoxHourlyIn{
			{
				BucketHour: curHour,
				AnalyticsBoxMetrics: AnalyticsBoxMetrics{
					QRViews: 100, PageViews: 140, UniqueVisitors: 45,
					LiveHLS: 20, VODPlays: 35, ClipDownloads: 18, CutClips: 12, CutSeconds: 720,
					AIStudioOpens: 15, AIVideosLoads: 12, AIAnalyzes: 9, AIShots: 64, AIPots: 38,
					AIPoolRuns: 7, AICaromRuns: 2, AIExportsIOS: 4, AIExportsDesktop: 3, AIExportRelay: 4,
					AIErrors: 1, HTTPErrors: 2, BytesOut: 120_000_000,
					OSIOS: 25, OSAndroid: 15, OSDesktop: 5,
				},
			},
		},
		BoxDaily: []AnalyticsBoxDailyIn{{BucketDate: today, UniqueVisitors: 40}},
		CameraDaily: []AnalyticsCamDailyIn{
			{
				BucketDate: today, CameraID: "camera01", TableName: "Bàn 01 Pool",
				AnalyticsCamMetrics: AnalyticsCamMetrics{QRViews: 60, UniqueVisitors: 25, ClipDownloads: 10, CutClips: 7, AIAnalyzes: 6, AIShots: 42, AIExports: 5, BytesOut: 80_000_000},
			},
		},
	}, time.Now())
	if err != nil {
		t.Fatalf("ingest boxA failed: %v", err)
	}

	// Seed Club B (legacy nodered, AI disabled)
	_, err = srv.IngestAnalytics(AnalyticsIngestPayload{
		BoxID:   boxB.ID,
		Source:  "nodered",
		BatchID: boxB.ID + ":" + curHour + ":1",
		BoxHourly: []AnalyticsBoxHourlyIn{
			{
				BucketHour: curHour,
				AnalyticsBoxMetrics: AnalyticsBoxMetrics{
					QRViews: 30, PageViews: 50, ClipDownloads: 8, CutClips: 4, OSApp: 6,
				},
			},
		},
		CameraDaily: []AnalyticsCamDailyIn{
			{
				BucketDate: today, CameraID: "camera09", TableName: "Bàn 09 Secret",
				AnalyticsCamMetrics: AnalyticsCamMetrics{QRViews: 30, ClipDownloads: 8},
			},
		},
	}, time.Now())
	if err != nil {
		t.Fatalf("ingest boxB failed: %v", err)
	}

	// 1. Unauthenticated access to overview -> 401
	recUnauth, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/analytics/overview?range=7d", nil, map[string]string{"NO_AUTH": "1"})
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated overview, got %d", recUnauth.Code)
	}

	// 2. Viewer session CAN read overview (read-only allowed)
	viewerSess := srv.Store().CreateSession(testViewerUser, "viewer", "127.0.0.1", time.Hour)
	recView, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/analytics/overview?range=7d", nil, map[string]string{
		"NO_AUTH": "1",
		"Cookie":    SessionCookieName + "=" + viewerSess.Token,
	})
	if recView.Code != http.StatusOK {
		t.Fatalf("expected viewer to GET analytics overview, got %d: %s", recView.Code, recView.Body.String())
	}

	// 3. Full overview check (7d)
	recOv, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/analytics/overview?range=7d", nil, nil)
	if recOv.Code != http.StatusOK {
		t.Fatalf("overview 7d failed: %d %s", recOv.Code, recOv.Body.String())
	}
	var ov AnalyticsOverviewResponse
	if err := json.Unmarshal(recOv.Body.Bytes(), &ov); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if !ov.OK || ov.Range != "7d" || ov.KPI.QRViews != 130 || ov.KPI.ActiveBoxes != 2 || ov.KPI.ActiveClubs != 2 || ov.KPI.AIActiveBoxes != 1 {
		t.Fatalf("unexpected overview KPI: %+v", ov.KPI)
	}
	if len(ov.PeakHours) != 24 {
		t.Fatalf("expected 24 peak_hours entries, got %d", len(ov.PeakHours))
	}
	if ov.PeakHours[curHrInt].QRViews != 130 || ov.PeakHours[curHrInt].DownloadsCuts != 42 || ov.PeakHours[curHrInt].AIRuns != 9 {
		t.Fatalf("unexpected peak hour[%d]: %+v", curHrInt, ov.PeakHours[curHrInt])
	}
	if len(ov.DailySeries) != 7 {
		t.Fatalf("expected 7 daily_series entries, got %d", len(ov.DailySeries))
	}
	lastDay := ov.DailySeries[len(ov.DailySeries)-1]
	if lastDay.Date != today || lastDay.QRViews != 130 || lastDay.AIExports != 7 { // max(4,4)+3 = 7
		t.Fatalf("unexpected last daily_series point: %+v", lastDay)
	}
	if len(ov.TopClubs) != 2 || ov.TopClubs[0].ClubID != clubA.ID || ov.TopClubs[0].AIExports != 7 {
		t.Fatalf("unexpected top_clubs: %+v", ov.TopClubs)
	}

	// 4. ai_only=1 filter excludes Club B (nodered)
	recAI, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/analytics/overview?range=7d&ai_only=1", nil, nil)
	var ovAI AnalyticsOverviewResponse
	_ = json.Unmarshal(recAI.Body.Bytes(), &ovAI)
	if ovAI.KPI.QRViews != 100 || ovAI.KPI.ActiveBoxes != 1 || len(ovAI.TopBoxes) != 1 || ovAI.TopBoxes[0].BoxID != boxA.ID {
		t.Fatalf("expected ai_only=1 to include only boxA, got kpi=%+v boxes=%+v", ovAI.KPI, ovAI.TopBoxes)
	}

	// 5. 24h range queries hourly table directly
	rec24, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/analytics/overview?range=24h&club_id="+clubA.ID, nil, nil)
	var ov24 AnalyticsOverviewResponse
	_ = json.Unmarshal(rec24.Body.Bytes(), &ov24)
	if ov24.Range != "24h" || ov24.KPI.QRViews != 100 || ov24.KPI.UniqueVisitors != 45 {
		t.Fatalf("unexpected 24h overview: range=%s kpi=%+v", ov24.Range, ov24.KPI)
	}

	// 6. Cameras endpoint
	recCams, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/analytics/cameras?range=7d&limit=10", nil, nil)
	if recCams.Code != http.StatusOK {
		t.Fatalf("cameras endpoint failed: %d %s", recCams.Code, recCams.Body.String())
	}
	var camResp AnalyticsCamerasResponse
	_ = json.Unmarshal(recCams.Body.Bytes(), &camResp)
	if !camResp.OK || len(camResp.Cameras) != 2 || camResp.Cameras[0].CameraID != "camera01" || camResp.Cameras[0].TableName != "Bàn 01 Pool" {
		t.Fatalf("unexpected cameras response: %+v", camResp)
	}

	// 7. CSV Export with UTF-8 BOM
	recCSV, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/analytics/export.csv?range=7d", nil, nil)
	if recCSV.Code != http.StatusOK {
		t.Fatalf("CSV export failed: %d", recCSV.Code)
	}
	rawCSV := recCSV.Body.Bytes()
	if !bytes.HasPrefix(rawCSV, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("expected UTF-8 BOM prefix on CSV export")
	}
	if !strings.Contains(recCSV.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("unexpected CSV content-type: %q", recCSV.Header().Get("Content-Type"))
	}
	rows, err := csv.NewReader(bytes.NewReader(rawCSV[3:])).ReadAll()
	if err != nil || len(rows) != 3 { // header + 2 boxes
		t.Fatalf("expected 3 CSV rows (header + 2 boxes), got %d err=%v", len(rows), err)
	}

	// 8. Shop Owner Portal STRICT CLUB ISOLATION (even when attacker passes club_id=clubB & box_id=boxB!)
	portalPath := "/portal/" + clubA.PortalToken + "/api/analytics?range=7d&club_id=" + clubB.ID + "&box_id=" + boxB.ID
	recPortal, _ := doHubRequest(t, srv, http.MethodGet, portalPath, nil, map[string]string{"NO_AUTH": "1"})
	if recPortal.Code != http.StatusOK {
		t.Fatalf("portal analytics failed: %d %s", recPortal.Code, recPortal.Body.String())
	}
	var portalResp struct {
		OK          bool                  `json:"ok"`
		ClubID      string                `json:"club_id"`
		ClubName    string                `json:"club_name"`
		Range       string                `json:"range"`
		KPI         AnalyticsKPI          `json:"kpi"`
		OSBreakdown AnalyticsOSBreakdown  `json:"os_breakdown"`
		PeakHours   []AnalyticsPeakHour   `json:"peak_hours"`
		DailySeries []AnalyticsDailyPoint `json:"daily_series"`
		TopCameras  []AnalyticsCameraRow  `json:"top_cameras"`
	}
	if err := json.Unmarshal(recPortal.Body.Bytes(), &portalResp); err != nil {
		t.Fatalf("decode portal analytics: %v", err)
	}
	if !portalResp.OK || portalResp.ClubID != clubA.ID || portalResp.KPI.QRViews != 100 {
		t.Fatalf("expected strictly clubA analytics (qr=100), got %+v", portalResp)
	}
	if len(portalResp.PeakHours) != 24 || len(portalResp.TopCameras) != 1 || portalResp.TopCameras[0].CameraID != "camera01" {
		t.Fatalf("unexpected portal cameras / peak_hours: cams=%+v peaks=%d", portalResp.TopCameras, len(portalResp.PeakHours))
	}
	if strings.Contains(recPortal.Body.String(), "Bàn 09 Secret") || strings.Contains(recPortal.Body.String(), boxB.ID) {
		t.Fatalf("CRITICAL: club B data leaked into club A portal response: %s", recPortal.Body.String())
	}

	// Portal rejects non-GET and invalid tokens
	recPortalPost, _ := doHubRequest(t, srv, http.MethodPost, "/portal/"+clubA.PortalToken+"/api/analytics", nil, map[string]string{"NO_AUTH": "1"})
	if recPortalPost.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on POST portal analytics, got %d", recPortalPost.Code)
	}
	recPortalBad, _ := doHubRequest(t, srv, http.MethodGet, "/portal/00000000000000000000000000000000/api/analytics", nil, map[string]string{"NO_AUTH": "1"})
	if recPortalBad.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on invalid portal token, got %d", recPortalBad.Code)
	}
}

func TestAnalyticsRetentionPruner(t *testing.T) {
	srv, err := NewServer(Config{
		DataPath:     ":memory:",
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer srv.Close()

	box := srv.Store().Boxes()[0]
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, analyticsTZ)

	oldHourly := now.AddDate(0, 0, -95).Format("2006-01-02 15:00")
	keepHourly := now.AddDate(0, 0, -85).Format("2006-01-02 15:00")
	oldDaily := now.AddDate(0, 0, -740).Format("2006-01-02")
	keepDaily := now.AddDate(0, 0, -100).Format("2006-01-02")

	// Ingest expired and retained hourly/daily/camera rows
	_, err = srv.IngestAnalytics(AnalyticsIngestPayload{
		BoxID:       box.ID,
		Source:      "redbida-go",
		SourceEpoch: "ep-prune",
		BoxHourly: []AnalyticsBoxHourlyIn{
			{BucketHour: oldHourly, AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: 11}},
			{BucketHour: keepHourly, AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: 22}},
		},
		CameraDaily: []AnalyticsCamDailyIn{
			{BucketDate: oldDaily, CameraID: "camera01", AnalyticsCamMetrics: AnalyticsCamMetrics{QRViews: 5}},
			{BucketDate: keepDaily, CameraID: "camera01", AnalyticsCamMetrics: AnalyticsCamMetrics{QRViews: 9}},
		},
	}, now)
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	// Also insert an expired >730d daily row + a 100d-old daily row + old & fresh batches
	_, err = srv.IngestAnalytics(AnalyticsIngestPayload{
		BoxID:   box.ID,
		Source:  "nodered",
		BatchID: "old-batch",
		BoxHourly: []AnalyticsBoxHourlyIn{
			{BucketHour: oldDaily + " 10:00", AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: 3}},
		},
	}, now.AddDate(0, 0, -20))
	if err != nil {
		t.Fatalf("ingest old batch failed: %v", err)
	}
	_, err = srv.IngestAnalytics(AnalyticsIngestPayload{
		BoxID:   box.ID,
		Source:  "nodered",
		BatchID: "fresh-batch",
		BoxHourly: []AnalyticsBoxHourlyIn{
			{BucketHour: keepDaily + " 10:00", AnalyticsBoxMetrics: AnalyticsBoxMetrics{QRViews: 7}},
		},
	}, now.AddDate(0, 0, -5))
	if err != nil {
		t.Fatalf("ingest fresh batch failed: %v", err)
	}

	stats, err := srv.Store().PruneAnalytics(now)
	if err != nil {
		t.Fatalf("PruneAnalytics failed: %v", err)
	}
	if stats.HourlyDeleted < 2 || stats.DailyDeleted != 1 || stats.CameraDeleted != 1 || stats.BatchesDeleted != 1 {
		t.Fatalf("unexpected prune stats: %+v", stats)
	}

	// Verify 100-day-old usage_box_daily survived even though its hourly row (>90d) was pruned!
	db := srv.Store().SQLiteDB()
	var keptDailyQR int64
	if err := db.QueryRow("SELECT qr_views FROM usage_box_daily WHERE box_id = ? AND bucket_date = ?", box.ID, keepDaily).Scan(&keptDailyQR); err != nil {
		t.Fatalf("expected 100d-old usage_box_daily row to survive hourly prune: %v", err)
	}
	if keptDailyQR != 7 {
		t.Fatalf("expected keptDailyQR=7, got %d", keptDailyQR)
	}
}
