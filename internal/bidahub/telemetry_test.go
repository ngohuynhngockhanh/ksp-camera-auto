package bidahub

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Tier 1 — Feature Coverage (20 Tests: #125–#144, Features F11–F14)
// ============================================================================

// Test 125: Box struct extended telemetry fields & JSON serialization contract (F11)
func TestTier1_F11_BoxTelemetryStructAndJSONSerializationContract(t *testing.T) {
	b := Box{
		ID:              "inut_204_141",
		Name:            "789 Billiards — MIAPool Box 1",
		Online:          true,
		CPUPct:          18.5,
		LoadAvg1m:       1.42,
		CPUCores:        4,
		TempC:           52.3,
		RAMTotalMB:      1945,
		RAMUsedMB:       820,
		RAMAvailableMB:  1125,
		RAMUsedPct:      42,
		RAMPct:          42,
		HeapAllocMB:     11.2,
		HeapSysMB:       22.4,
		KspcamRSSMB:     29.8,
		CameraRSSMB:     144.5,
		Goroutines:      38,
		SwapTotalMB:     1945,
		SwapUsedMB:      28,
		SwapFreeMB:      1917,
		ZRAMTotalMB:     1945,
		ZRAMUsedMB:      28,
		ZRAMPct:         1,
		FFmpegCount:     14,
		UptimeSec:       172800,
		UptimeHuman:     "2d 0h",
		TelemetrySource: "live",
	}

	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("json.Marshal(Box) failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal(Box) failed: %v", err)
	}

	expectedKeys := []string{
		"online", "cpu_pct", "load_avg_1m", "cpu_cores",
		"ram_total_mb", "ram_used_mb", "ram_available_mb", "ram_used_pct", "ram_pct",
		"heap_alloc_mb", "heap_sys_mb", "kspcam_rss_mb", "camera_rss_mb", "goroutines",
		"swap_total_mb", "swap_used_mb", "swap_free_mb",
		"zram_total_mb", "zram_used_mb", "zram_pct",
		"uptime_sec", "uptime_human", "telemetry_source",
	}
	for _, k := range expectedKeys {
		if _, ok := m[k]; !ok {
			t.Errorf("serialized Box JSON missing required key %q", k)
		}
	}
	if m["cpu_pct"] != 18.5 {
		t.Errorf("expected cpu_pct=18.5, got %v", m["cpu_pct"])
	}
	if m["ram_available_mb"] != float64(1125) {
		t.Errorf("expected ram_available_mb=1125, got %v", m["ram_available_mb"])
	}
	if m["heap_alloc_mb"] != 11.2 {
		t.Errorf("expected heap_alloc_mb=11.2, got %v", m["heap_alloc_mb"])
	}
	if m["swap_free_mb"] != float64(1917) {
		t.Errorf("expected swap_free_mb=1917, got %v", m["swap_free_mb"])
	}
	if m["telemetry_source"] != "live" {
		t.Errorf("expected telemetry_source='live', got %v", m["telemetry_source"])
	}
}

// Test 126: Single heartbeat (POST /fleet/api/heartbeat) updates all live telemetry fields (F12)
func TestTier1_F12_SingleHeartbeatUpdatesAllLiveTelemetryFields(t *testing.T) {
	srv, _ := newTestHubServer(t)

	payload := map[string]any{
		"box_id":           "inut_204_141",
		"online":           true,
		"rtt_ms":           19,
		"temp_c":           54.5,
		"cpu_pct":          24.8,
		"load_avg_1m":      1.65,
		"cpu_cores":        4,
		"ram_total_mb":     1945,
		"ram_used_mb":      860,
		"ram_available_mb": 1085,
		"ram_used_pct":     44,
		"ram_pct":          44,
		"heap_alloc_mb":    13.4,
		"heap_sys_mb":      26.1,
		"kspcam_rss_mb":    34.2,
		"camera_rss_mb":    152.8,
		"goroutines":       41,
		"swap_total_mb":    1945,
		"swap_used_mb":     58,
		"swap_free_mb":     1887,
		"zram_total_mb":    1945,
		"zram_used_mb":     58,
		"zram_pct":         3,
		"sd_total_gb":      64,
		"sd_used_gb":       32,
		"sd_used_pct":      50,
		"sd_fs_type":       "f2fs",
		"sd_mount_opts":    "rw,noatime,errors=continue",
		"ffmpeg_count":     14,
		"rx_mbps":          11.6,
		"uptime_sec":       259200,
		"uptime_human":     "3d 0h",
	}

	rec, body := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", payload, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if body["ok"] != true {
		t.Fatalf("expected ok=true, got %v", body)
	}

	b := storeGetBox(srv, "inut_204_141")
	if b == nil {
		t.Fatalf("box inut_204_141 not found")
	}
	if !b.Online {
		t.Errorf("expected Online=true")
	}
	if b.CPUPct != 24.8 || b.LoadAvg1m != 1.65 || b.CPUCores != 4 {
		t.Errorf("CPU fields mismatch: cpu_pct=%v load_avg_1m=%v cpu_cores=%v", b.CPUPct, b.LoadAvg1m, b.CPUCores)
	}
	if b.RAMTotalMB != 1945 || b.RAMUsedMB != 860 || b.RAMAvailableMB != 1085 || b.RAMUsedPct != 44 || b.RAMPct != 44 {
		t.Errorf("RAM fields mismatch: total=%d used=%d avail=%d used_pct=%d pct=%d",
			b.RAMTotalMB, b.RAMUsedMB, b.RAMAvailableMB, b.RAMUsedPct, b.RAMPct)
	}
	if b.HeapAllocMB != 13.4 || b.HeapSysMB != 26.1 || b.KspcamRSSMB != 34.2 || b.CameraRSSMB != 152.8 || b.Goroutines != 41 {
		t.Errorf("Heap/RSS fields mismatch: heap_alloc=%v heap_sys=%v kspcam_rss=%v camera_rss=%v goroutines=%d",
			b.HeapAllocMB, b.HeapSysMB, b.KspcamRSSMB, b.CameraRSSMB, b.Goroutines)
	}
	if b.SwapTotalMB != 1945 || b.SwapUsedMB != 58 || b.SwapFreeMB != 1887 || b.ZRAMPct != 3 {
		t.Errorf("Swap/ZRAM fields mismatch: total=%d used=%d free=%d zram_pct=%d",
			b.SwapTotalMB, b.SwapUsedMB, b.SwapFreeMB, b.ZRAMPct)
	}
	if b.UptimeSec != 259200 || b.UptimeHuman != "3d 0h" {
		t.Errorf("Uptime fields mismatch: sec=%d human=%q", b.UptimeSec, b.UptimeHuman)
	}
	if b.TelemetrySource != "live" {
		t.Errorf("expected TelemetrySource='live', got %q", b.TelemetrySource)
	}
}

// Test 127: Single heartbeat via /api/heartbeat alias works unauthenticated (F12)
func TestTier1_F12_SingleHeartbeatAPIHeartbeatAliasWorks(t *testing.T) {
	srv, _ := newTestHubServer(t)

	payload := map[string]any{
		"box_id":           "inut_205_72",
		"cpu_pct":          16.2,
		"ram_total_mb":     1945,
		"ram_used_mb":      745,
		"ram_available_mb": 1200,
		"heap_alloc_mb":    10.5,
		"swap_total_mb":    1945,
		"swap_used_mb":     12,
		"swap_free_mb":     1933,
	}

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/heartbeat", payload, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/heartbeat status = %d, body = %s", rec.Code, rec.Body.String())
	}

	b := storeGetBox(srv, "inut_205_72")
	if b.CPUPct != 16.2 || b.RAMAvailableMB != 1200 || b.HeapAllocMB != 10.5 || b.SwapFreeMB != 1933 {
		t.Errorf("unexpected telemetry on inut_205_72: %+v", b)
	}
}

// Test 128: Single heartbeat defaults online=true when online field is omitted (F12)
func TestTier1_F12_SingleHeartbeatDefaultOnlineTrueWhenOmitted(t *testing.T) {
	srv, _ := newTestHubServer(t)

	storeMutateBox(t, srv, "inut_205_69", func(b *Box) {
		b.Online = false
	})
	if storeGetBox(srv, "inut_205_69").Online {
		t.Fatalf("precondition failed: expected inut_205_69 offline")
	}

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":        "inut_205_69",
		"cpu_pct":       14.0,
		"heap_alloc_mb": 9.6,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_205_69")
	if !b.Online {
		t.Errorf("expected box to default to Online=true when online field is omitted in single heartbeat")
	}
}

// Test 129: Explicit online:false in single heartbeat marks box OFFLINE and updates KPI & Club (F12)
func TestTier1_F12_SingleHeartbeatExplicitOnlineFalseMarksBoxOfflineAndUpdatesKPIAndClub(t *testing.T) {
	srv, _ := newTestHubServer(t)

	kpiBefore := srv.Store().ComputeKPI()
	totalBoxes := kpiBefore.BoxesTotal

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":  "inut_205_69",
		"online":  false,
		"cpu_pct": 0,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d, body = %s", rec.Code, rec.Body.String())
	}

	b := storeGetBox(srv, "inut_205_69")
	if b.Online {
		t.Errorf("expected inut_205_69 Online=false after explicit online:false heartbeat")
	}
	if b.TelemetrySource != "live" {
		t.Errorf("expected TelemetrySource='live', got %q", b.TelemetrySource)
	}

	kpiAfter := srv.Store().ComputeKPI()
	if kpiAfter.BoxesOnline != totalBoxes-1 || kpiAfter.BoxesOffline != 1 {
		t.Errorf("expected boxes_online=%d boxes_offline=1, got online=%d offline=%d",
			totalBoxes-1, kpiAfter.BoxesOnline, kpiAfter.BoxesOffline)
	}

	club := storeGetClub(srv, b.ClubID)
	if club == nil {
		t.Fatalf("club %s not found", b.ClubID)
	}
	if club.OnlineBoxesCount != 0 {
		t.Errorf("expected club %s OnlineBoxesCount=0, got %d", club.ID, club.OnlineBoxesCount)
	}
}

// Test 130: Batch heartbeat (POST /fleet/api/heartbeat/batch) with {"boxes": [...]} updates multiple boxes & endpoints (F12)
func TestTier1_F12_BatchHeartbeatObjectWrapperUpdatesMultipleBoxesAndKPI(t *testing.T) {
	srv, _ := newTestHubServer(t)
	totalBoxes := len(storeListBoxes(srv))

	batchPayload := map[string]any{
		"boxes": []map[string]any{
			{
				"box_id":           "inut_204_141",
				"online":           true,
				"rtt_ms":           15,
				"cpu_pct":          21.5,
				"load_avg_1m":      1.18,
				"cpu_cores":        4,
				"ram_total_mb":     1945,
				"ram_used_mb":      815,
				"ram_available_mb": 1130,
				"heap_alloc_mb":    11.8,
				"kspcam_rss_mb":    30.5,
				"camera_rss_mb":    142.0,
				"swap_total_mb":    1945,
				"swap_used_mb":     35,
				"swap_free_mb":     1910,
				"ffmpeg_count":     14,
				"uptime_sec":       90000,
				"uptime_human":     "1d 1h",
			},
			{
				"box_id": "inut_205_69",
				"online": false,
			},
			{
				"box_id": "inut_205_85",
				"online": false,
			},
		},
	}

	rec, resp := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", batchPayload, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat/batch status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp)
	}
	if resp["updated"] != float64(3) {
		t.Errorf("expected updated=3, got %v", resp["updated"])
	}
	if resp["boxes_online"] != float64(totalBoxes-2) || resp["boxes_offline"] != float64(2) {
		t.Errorf("expected boxes_online=%d boxes_offline=2, got online=%v offline=%v",
			totalBoxes-2, resp["boxes_online"], resp["boxes_offline"])
	}

	// Verify GET /fleet/api/summary, GET /fleet/api/boxes, and GET /fleet/api/clubs
	for _, ep := range []string{"/fleet/api/summary", "/fleet/api/boxes", "/fleet/api/clubs"} {
		sRec, sBody := doHubRequest(t, srv, http.MethodGet, ep, nil, nil)
		if sRec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", ep, sRec.Code)
		}
		kpiMap, _ := sBody["kpi"].(map[string]any)
		if kpiMap == nil {
			t.Fatalf("GET %s missing kpi object", ep)
		}
		if kpiMap["boxes_online"] != float64(totalBoxes-2) || kpiMap["boxes_offline"] != float64(2) {
			t.Errorf("GET %s kpi mismatch: online=%v offline=%v", ep, kpiMap["boxes_online"], kpiMap["boxes_offline"])
		}
	}
}

// Test 131: Batch heartbeat accepts raw JSON array payload directly (F12)
func TestTier1_F12_BatchHeartbeatRawJSONArrayPayloadSupported(t *testing.T) {
	srv, _ := newTestHubServer(t)
	totalBoxes := len(storeListBoxes(srv))

	rawArray := `[
		{"box_id":"inut_204_150","online":true,"cpu_pct":19.4,"ram_total_mb":1843,"ram_used_mb":763,"ram_available_mb":1080,"heap_alloc_mb":12.1,"swap_total_mb":2662,"swap_used_mb":40,"swap_free_mb":2622},
		{"box_id":"inut_205_87","online":false}
	]`

	rec, resp := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", rawArray, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat/batch (array) status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if resp["updated"] != float64(2) {
		t.Errorf("expected updated=2, got %v", resp["updated"])
	}
	if resp["boxes_online"] != float64(totalBoxes-1) || resp["boxes_offline"] != float64(1) {
		t.Errorf("unexpected online/offline counts: %v", resp)
	}
	if storeGetBox(srv, "inut_205_87").Online {
		t.Errorf("expected inut_205_87 to be offline")
	}
	if storeGetBox(srv, "inut_204_150").CPUPct != 19.4 {
		t.Errorf("expected inut_204_150 cpu_pct=19.4, got %v", storeGetBox(srv, "inut_204_150").CPUPct)
	}
}

// Test 132: Batch heartbeat via /api/heartbeat/batch alias works (F12)
func TestTier1_F12_BatchHeartbeatAPIHeartbeatBatchAliasWorks(t *testing.T) {
	srv, _ := newTestHubServer(t)

	payload := map[string]any{
		"boxes": []map[string]any{
			{"box_id": "inut_205_56", "online": true, "cpu_pct": 17.7, "heap_alloc_mb": 10.9},
			{"box_id": "inut_205_53", "online": false},
		},
	}

	rec, resp := doHubRequest(t, srv, http.MethodPost, "/api/heartbeat/batch", payload, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/heartbeat/batch status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if resp["updated"] != float64(2) {
		t.Errorf("expected updated=2, got %v", resp["updated"])
	}
	if storeGetBox(srv, "inut_205_53").Online {
		t.Errorf("expected inut_205_53 Online=false")
	}
}

// Test 133: Multi-box club partial offline updates Club.OnlineBoxesCount accurately (F12)
func TestTier1_F12_MultiBoxClubPartialOfflineUpdatesClubOnlineBoxesCount(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// 99 Billiards CS11 has 3 boxes: inut_204_150, inut_204_152, inut_206_60
	batch := map[string]any{
		"boxes": []map[string]any{
			{"box_id": "inut_204_150", "online": true, "cpu_pct": 20.1},
			{"box_id": "inut_204_152", "online": false},
			{"box_id": "inut_206_60", "online": true, "cpu_pct": 15.3},
		},
	}
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", batch, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("batch heartbeat failed: %d", rec.Code)
	}

	club := storeGetClub(srv, "club_99_billiards_cs11")
	if club == nil {
		t.Fatalf("club_99_billiards_cs11 not found")
	}
	if club.BoxesCount != 3 || club.OnlineBoxesCount != 2 {
		t.Errorf("expected BoxesCount=3 OnlineBoxesCount=2, got BoxesCount=%d OnlineBoxesCount=%d",
			club.BoxesCount, club.OnlineBoxesCount)
	}
}

// Test 134: Shop Owner Portal (/portal/{token}/api/status) reflects live online/offline and telemetry (F12/F13)
func TestTier1_F12_PortalStatusReflectsLiveOnlineOfflineAndTelemetry(t *testing.T) {
	srv, _ := newTestHubServer(t)

	club := storeGetClub(srv, "club_99_billiards_cs11")
	if club == nil {
		t.Fatalf("club_99_billiards_cs11 not found")
	}

	batch := map[string]any{
		"boxes": []map[string]any{
			{
				"box_id":           "inut_204_150",
				"online":           true,
				"cpu_pct":          22.6,
				"ram_total_mb":     1843,
				"ram_used_mb":      800,
				"ram_available_mb": 1043,
				"heap_alloc_mb":    11.5,
				"swap_total_mb":    2662,
				"swap_used_mb":     50,
				"swap_free_mb":     2612,
			},
			{
				"box_id": "inut_204_152",
				"online": false,
			},
			{
				"box_id": "inut_206_60",
				"online": true,
			},
		},
	}
	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", batch, map[string]string{"NO_AUTH": "1"})

	rec, body := doHubRequest(t, srv, http.MethodGet, "/portal/"+club.PortalToken+"/api/status", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /portal/%s/api/status status = %d", club.PortalToken, rec.Code)
	}
	clubObj, _ := body["club"].(map[string]any)
	if clubObj != nil && clubObj["boxes_online"] != nil && clubObj["boxes_online"] != float64(2) {
		t.Errorf("expected portal club.boxes_online=2, got %v", clubObj["boxes_online"])
	}
	boxesArr, _ := body["boxes"].([]any)
	if len(boxesArr) != 3 {
		t.Fatalf("expected 3 boxes in portal response, got %d", len(boxesArr))
	}
	onlineFound := map[string]bool{}
	for _, rawBox := range boxesArr {
		bm, _ := rawBox.(map[string]any)
		id, _ := bm["id"].(string)
		on, _ := bm["online"].(bool)
		onlineFound[id] = on
	}
	if !onlineFound["inut_204_150"] || onlineFound["inut_204_152"] || !onlineFound["inut_206_60"] {
		t.Errorf("portal box online states mismatch: %+v", onlineFound)
	}
}

// Test 135: probeSingleBox queries GET /api/system/metrics on reachable box and populates telemetry (F11/F12)
func TestTier1_F11_ProbeSingleBoxQueriesMockKspcamSystemMetricsEndpoint(t *testing.T) {
	srv, _ := newTestHubServer(t)

	mockKspcam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/system/metrics" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"online":           true,
				"cpu_pct":          23.4,
				"load_avg_1m":      1.35,
				"cpu_cores":        4,
				"temp_c":           51.2,
				"ram_total_mb":     1945,
				"ram_used_mb":      845,
				"ram_available_mb": 1100,
				"ram_used_pct":     43,
				"heap_alloc_mb":    12.8,
				"heap_sys_mb":      24.5,
				"kspcam_rss_mb":    31.2,
				"camera_rss_mb":    148.6,
				"goroutines":       42,
				"swap_total_mb":    1945,
				"swap_used_mb":     64,
				"swap_free_mb":     1881,
				"zram_total_mb":    1945,
				"zram_used_mb":     64,
				"sd_total_gb":      58,
				"sd_used_gb":       28,
				"sd_used_pct":      48,
				"ffmpeg_count":     14,
				"uptime_sec":       86400,
				"uptime_human":     "1d 0h",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockKspcam.Close()

	srv.RegisterTunnelTarget("inut_204_141", "2028", mockKspcam.Listener.Addr().String())

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]any{"box_id": "inut_204_141"}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/poll status = %d, body = %s", rec.Code, rec.Body.String())
	}

	b := storeGetBox(srv, "inut_204_141")
	if !b.Online {
		t.Errorf("expected Online=true")
	}
	if b.TelemetrySource != "probe" {
		t.Errorf("expected TelemetrySource='probe', got %q", b.TelemetrySource)
	}
	if b.CPUPct != 23.4 || b.LoadAvg1m != 1.35 || b.CPUCores != 4 {
		t.Errorf("expected CPU 23.4 / Load 1.35 / Cores 4, got %v / %v / %d", b.CPUPct, b.LoadAvg1m, b.CPUCores)
	}
	if b.RAMTotalMB != 1945 || b.RAMUsedMB != 845 || b.RAMAvailableMB != 1100 || b.RAMUsedPct != 43 || b.RAMPct != 43 {
		t.Errorf("unexpected RAM metrics from probe: total=%d used=%d avail=%d used_pct=%d pct=%d",
			b.RAMTotalMB, b.RAMUsedMB, b.RAMAvailableMB, b.RAMUsedPct, b.RAMPct)
	}
	if b.HeapAllocMB != 12.8 || b.HeapSysMB != 24.5 || b.KspcamRSSMB != 31.2 || b.CameraRSSMB != 148.6 || b.Goroutines != 42 {
		t.Errorf("unexpected Heap/RSS metrics from probe: heap=%v sys=%v ksp_rss=%v cam_rss=%v goroutines=%d",
			b.HeapAllocMB, b.HeapSysMB, b.KspcamRSSMB, b.CameraRSSMB, b.Goroutines)
	}
	if b.SwapTotalMB != 1945 || b.SwapUsedMB != 64 || b.SwapFreeMB != 1881 {
		t.Errorf("unexpected Swap metrics from probe: total=%d used=%d free=%d",
			b.SwapTotalMB, b.SwapUsedMB, b.SwapFreeMB)
	}
	if b.UptimeSec != 86400 || b.UptimeHuman != "1d 0h" {
		t.Errorf("unexpected Uptime metrics from probe: sec=%d human=%q", b.UptimeSec, b.UptimeHuman)
	}
}

// Test 136: Heartbeat updates Shinobi monitors and hardening fields when provided (F12)
func TestTier1_F12_HeartbeatUpdatesShinobiMonitorsAndHardeningScore(t *testing.T) {
	srv, _ := newTestHubServer(t)

	payload := map[string]any{
		"box_id":        "inut_204_141",
		"online":        true,
		"cpu_pct":       19.0,
		"heap_alloc_mb": 11.1,
		"monitors": []map[string]any{
			{
				"mid":                  "cam_table_1",
				"box_id":               "inut_204_141",
				"name":                 "Table 1",
				"host":                 "192.168.1.101",
				"mode":                 "record",
				"status":               "Watching",
				"codec":                "hevc",
				"acodec":               "aac",
				"width":                1920,
				"height":               1080,
				"input_map_choices_ok": true,
				"do_snapshot_disabled": true,
				"frag_mp4_ok":          true,
			},
		},
	}

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", payload, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d", rec.Code)
	}
	b := storeGetBox(srv, "inut_204_141")
	if len(b.Monitors) != 1 || b.Monitors[0].MID != "cam_table_1" {
		t.Errorf("expected 1 updated monitor 'cam_table_1', got %+v", b.Monitors)
	}
}

// Test 137: Live telemetry and offline status persist across Server restarts (F12)
func TestTier1_F12_HeartbeatPersistsLiveTelemetryAcrossServerRestart(t *testing.T) {
	srv, dataPath := newTestHubServer(t)

	batch := map[string]any{
		"boxes": []map[string]any{
			{
				"box_id":           "inut_204_141",
				"online":           true,
				"cpu_pct":          29.5,
				"load_avg_1m":      1.85,
				"ram_total_mb":     1945,
				"ram_used_mb":      945,
				"ram_available_mb": 1000,
				"heap_alloc_mb":    14.7,
				"swap_total_mb":    1945,
				"swap_used_mb":     95,
				"swap_free_mb":     1850,
				"uptime_sec":       36000,
				"uptime_human":     "10h 0m",
			},
			{
				"box_id": "inut_205_69",
				"online": false,
			},
		},
	}
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", batch, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("batch heartbeat failed: %d", rec.Code)
	}
	_ = srv.Close()

	srv2, err := NewServer(Config{
		BindAddr:     "127.0.0.1",
		Port:         2090,
		DataPath:     dataPath,
		APIKey:       testAPIKey,
		StaffSecret:  testStaffSecret,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer restart failed: %v", err)
	}
	defer srv2.Close()

	b1 := storeGetBox(srv2, "inut_204_141")
	if b1.CPUPct != 29.5 || b1.RAMAvailableMB != 1000 || b1.HeapAllocMB != 14.7 || b1.SwapFreeMB != 1850 {
		t.Errorf("persisted telemetry lost across restart: %+v", b1)
	}
	b2 := storeGetBox(srv2, "inut_205_69")
	if b2.Online {
		t.Errorf("expected offline box inut_205_69 to remain Online=false across restart")
	}
}

// Test 138: Batch heartbeat broadcasts SSE fleet_update event with updated KPI (F12)
func TestTier1_F12_BatchHeartbeatBroadcastsSSEFleetUpdateEvent(t *testing.T) {
	srv, _ := newTestHubServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/fleet/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE connect failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	// Read initial connected event
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')

	// Trigger batch heartbeat marking inut_205_69 offline
	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", map[string]any{
		"boxes": []map[string]any{
			{"box_id": "inut_205_69", "online": false},
		},
	}, map[string]string{"NO_AUTH": "1"})

	foundFleetUpdate := false
	for range 12 {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, "fleet_update") || strings.Contains(line, "boxes_offline") {
			foundFleetUpdate = true
			break
		}
	}
	if !foundFleetUpdate {
		t.Errorf("expected SSE fleet_update broadcast after /fleet/api/heartbeat/batch")
	}
}

// Test 139: Public /api/health reflects accurate boxes_online after offline heartbeat (F12)
func TestTier1_F12_HealthEndpointReflectsAccurateBoxesOnlineAfterOfflineHeartbeat(t *testing.T) {
	srv, _ := newTestHubServer(t)
	total := len(storeListBoxes(srv))

	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", map[string]any{
		"boxes": []map[string]any{
			{"box_id": "inut_205_69", "online": false},
			{"box_id": "inut_205_85", "online": false},
			{"box_id": "inut_205_87", "online": false},
		},
	}, map[string]string{"NO_AUTH": "1"})

	rec, body := doHubRequest(t, srv, http.MethodGet, "/api/health", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health status = %d", rec.Code)
	}
	if body["boxes_online"] != float64(total-3) {
		t.Errorf("expected /api/health boxes_online=%d, got %v", total-3, body["boxes_online"])
	}
}

// Test 140: UI static bundle (GET /static/app.js & GET /app.js) contains telemetry rendering logic (F13)
func TestTier1_F13_StaticAppJSContainsTelemetryRenderingForCPUHeapAvailSwapOffline(t *testing.T) {
	srv, _ := newTestHubServer(t)

	for _, path := range []string{"/static/app.js", "/app.js"} {
		rec, _ := doHubRequest(t, srv, http.MethodGet, path, nil, map[string]string{"NO_AUTH": "1"})
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", path, rec.Code)
		}
		js := rec.Body.String()
		requiredTokens := []string{
			"ram_available_mb",
			"heap_alloc_mb",
			"swap_used_mb",
			"swap_free_mb",
			"cpu_pct",
			"OFFLINE",
		}
		for _, tok := range requiredTokens {
			if !strings.Contains(js, tok) {
				t.Errorf("GET %s missing expected telemetry rendering token %q", path, tok)
			}
		}
	}
}

// Test 141: Root SCADA index.html contains Online/Offline LEDs & telemetry layout (F13)
func TestTier1_F13_StaticIndexHTMLContainsTelemetryLayoutElements(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, _ := doHubRequest(t, srv, http.MethodGet, "/", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d", rec.Code)
	}
	html := rec.Body.String()
	for _, token := range []string{"led-online-count", "led-offline-count", "app.js"} {
		if !strings.Contains(html, token) {
			t.Errorf("index.html missing expected token %q", token)
		}
	}
}

// Test 142: Shop Owner Portal HTML (GET /portal/{token}/) contains Online/Offline & telemetry display (F13)
func TestTier1_F13_ShopOwnerPortalHTMLContainsOnlineOfflineAndTelemetryDisplay(t *testing.T) {
	srv, _ := newTestHubServer(t)
	club := storeGetClub(srv, "club_789_mia")
	if club == nil {
		t.Fatalf("club_789_mia not found")
	}

	rec, _ := doHubRequest(t, srv, http.MethodGet, "/portal/"+club.PortalToken+"/", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /portal/%s/ status = %d", club.PortalToken, rec.Code)
	}
	html := rec.Body.String()
	if !strings.Contains(strings.ToUpper(html), "OFFLINE") || !strings.Contains(strings.ToUpper(html), "ONLINE") {
		t.Errorf("portal.html missing ONLINE/OFFLINE status rendering")
	}
}

// Test 143: Box Detail endpoint (GET /fleet/api/boxes/{id}) returns full extended telemetry JSON (F11/F12)
func TestTier1_F12_BoxDetailEndpointReturnsFullExtendedTelemetry(t *testing.T) {
	srv, _ := newTestHubServer(t)

	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":           "inut_204_141",
		"online":           true,
		"cpu_pct":          26.4,
		"load_avg_1m":      1.52,
		"cpu_cores":        4,
		"ram_total_mb":     1945,
		"ram_used_mb":      895,
		"ram_available_mb": 1050,
		"heap_alloc_mb":    12.3,
		"heap_sys_mb":      25.0,
		"kspcam_rss_mb":    32.1,
		"camera_rss_mb":    146.2,
		"goroutines":       39,
		"swap_total_mb":    1945,
		"swap_used_mb":     45,
		"swap_free_mb":     1900,
		"uptime_sec":       120000,
		"uptime_human":     "1d 9h",
	}, map[string]string{"NO_AUTH": "1"})

	rec, body := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_204_141", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /fleet/api/boxes/inut_204_141 status = %d", rec.Code)
	}
	boxMap, _ := body["box"].(map[string]any)
	if boxMap == nil {
		t.Fatalf("missing box object in response")
	}
	if boxMap["cpu_pct"] != 26.4 || boxMap["ram_available_mb"] != float64(1050) || boxMap["heap_alloc_mb"] != 12.3 || boxMap["swap_free_mb"] != float64(1900) {
		t.Errorf("box detail telemetry mismatch: %+v", boxMap)
	}
}

// Test 144: Store.Clubs() and Store.ComputeKPI() always stay 100% consistent with Box.Online (F12)
func TestTier1_F12_StoreClubsAndSummaryAlwaysConsistentWithBoxOnlineState(t *testing.T) {
	srv, _ := newTestHubServer(t)

	storeMutateBox(t, srv, "inut_205_69", func(b *Box) { b.Online = false })
	storeMutateBox(t, srv, "inut_205_85", func(b *Box) { b.Online = false })

	clubs := srv.Store().Clubs()
	sumClubOnline := 0
	sumClubTotal := 0
	for _, c := range clubs {
		sumClubOnline += c.OnlineBoxesCount
		sumClubTotal += c.BoxesCount
	}

	kpi := srv.Store().ComputeKPI()
	if sumClubOnline != kpi.BoxesOnline {
		t.Errorf("club online sum (%d) != kpi.BoxesOnline (%d)", sumClubOnline, kpi.BoxesOnline)
	}
	if sumClubTotal != kpi.BoxesTotal {
		t.Errorf("club total sum (%d) != kpi.BoxesTotal (%d)", sumClubTotal, kpi.BoxesTotal)
	}
}

// ============================================================================
// Tier 2 — Boundary & Corner Cases (20 Tests: #145–#164, Features F11–F14)
// ============================================================================

// Test 145: Auto-calculate ram_used_pct and ram_pct when only ram_total_mb & ram_used_mb are provided (F12)
func TestTier2_F12_AutoCalculateRAMUsedPctFromRAMTotalAndUsedMB(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":           "inut_204_141",
		"ram_total_mb":     2000,
		"ram_used_mb":      850,
		"ram_available_mb": 1150,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_204_141")
	expectedPct := (850 * 100) / 2000 // 42
	if b.RAMUsedPct != expectedPct || b.RAMPct != expectedPct {
		t.Errorf("expected RAMUsedPct=%d and RAMPct=%d, got RAMUsedPct=%d RAMPct=%d",
			expectedPct, expectedPct, b.RAMUsedPct, b.RAMPct)
	}
}

// Test 146: Auto-calculate RAMUsedMB and RAMUsedPct when ram_total_mb & ram_used_mb (820/1945) provided (F12)
func TestTier2_F12_AutoCalculateRAMUsedMBFromRAMTotalAndAvailableMB(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":           "inut_205_72",
		"ram_total_mb":     1945,
		"ram_used_mb":      820,
		"ram_available_mb": 1125,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_205_72")
	expectedPct := (820 * 100) / 1945 // 42
	if b.RAMUsedPct != expectedPct || b.RAMPct != expectedPct {
		t.Errorf("expected RAMUsedPct=%d RAMPct=%d, got %d / %d", expectedPct, expectedPct, b.RAMUsedPct, b.RAMPct)
	}
}

// Test 147: Sync ram_used_pct when only ram_pct > 0 is provided and vice-versa (F12)
func TestTier2_F12_SyncRAMUsedPctWhenOnlyRAMPctProvided(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Case A: only ram_pct provided
	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":  "inut_204_141",
		"ram_pct": 67,
	}, map[string]string{"NO_AUTH": "1"})
	b1 := storeGetBox(srv, "inut_204_141")
	if b1.RAMPct != 67 || b1.RAMUsedPct != 67 {
		t.Errorf("Case A: expected RAMPct=67 & RAMUsedPct=67, got %d & %d", b1.RAMPct, b1.RAMUsedPct)
	}

	// Case B: only ram_used_pct provided
	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":       "inut_204_150",
		"ram_used_pct": 53,
	}, map[string]string{"NO_AUTH": "1"})
	b2 := storeGetBox(srv, "inut_204_150")
	if b2.RAMPct != 53 || b2.RAMUsedPct != 53 {
		t.Errorf("Case B: expected RAMPct=53 & RAMUsedPct=53, got %d & %d", b2.RAMPct, b2.RAMUsedPct)
	}
}

// Test 148: Auto-calculate swap_free_mb & zram_pct when swap_total_mb & swap_used_mb are provided (F12)
func TestTier2_F12_AutoCalculateSwapFreeMBAndZRAMPctFromSwapTotalAndUsed(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":        "inut_204_141",
		"swap_total_mb": 1945,
		"swap_used_mb":  195,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_204_141")
	if b.SwapFreeMB != 1750 {
		t.Errorf("expected auto-calculated SwapFreeMB=1750 (1945-195), got %d", b.SwapFreeMB)
	}
	expectedZRAMPct := (195 * 100) / 1945 // 10
	if b.ZRAMPct != expectedZRAMPct {
		t.Errorf("expected auto-calculated ZRAMPct=%d, got %d", expectedZRAMPct, b.ZRAMPct)
	}
}

// Test 149: Zero ram_total_mb and zero swap_total_mb never cause division by zero (F12)
func TestTier2_F12_ZeroRAMTotalAndZeroSwapTotalPreventDivisionByZero(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":        "inut_204_141",
		"ram_total_mb":  0,
		"ram_used_mb":   500,
		"swap_total_mb": 0,
		"swap_used_mb":  100,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK without division-by-zero panic, got %d", rec.Code)
	}
}

// Test 150: swap_used_mb >= swap_total_mb does not produce negative swap_free_mb (F12)
func TestTier2_F12_SwapUsedExceedingSwapTotalClampsSwapFreeWithoutNegative(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":        "inut_204_141",
		"swap_total_mb": 1000,
		"swap_used_mb":  1200,
		"swap_free_mb":  0,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d", rec.Code)
	}
	b := storeGetBox(srv, "inut_204_141")
	if b.SwapFreeMB < 0 {
		t.Errorf("expected non-negative SwapFreeMB, got %d", b.SwapFreeMB)
	}
}

// Test 151: Heartbeat with online:false does not overwrite older LastSeen timestamp with now (F12)
func TestTier2_F12_HeartbeatOfflineDoesNotRefreshStaleLastSeenTimestamp(t *testing.T) {
	srv, _ := newTestHubServer(t)

	pastTime := time.Now().UTC().Add(-15 * time.Minute).Truncate(time.Second)
	storeMutateBox(t, srv, "inut_205_69", func(b *Box) {
		b.LastSeen = pastTime
		b.Online = true
	})

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id": "inut_205_69",
		"online": false,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_205_69")
	if b.Online {
		t.Errorf("expected Online=false")
	}
	if time.Since(b.LastSeen) < 10*time.Minute {
		t.Errorf("expected offline heartbeat NOT to overwrite LastSeen with now; got LastSeen=%v (pastTime=%v)", b.LastSeen, pastTime)
	}
}

// Test 152: Poller transitions box with TelemetrySource=="live" and LastSeen > 90s to Online=false when unreachable (F12)
func TestTier2_F12_PollerExpiresStaleLiveTelemetryOlderThan90Seconds(t *testing.T) {
	srv, _ := newTestHubServer(t, func(c *Config) {
		c.DialTimeout = 25 * time.Millisecond
	})

	storeMutateBox(t, srv, "inut_205_69", func(b *Box) {
		b.VPNIP = "127.0.0.1:65531" // unreachable closed port
		b.Online = true
		b.TelemetrySource = "live"
		b.LastSeen = time.Now().UTC().Add(-95 * time.Second)
	})

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]any{"box_id": "inut_205_69"}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/poll status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_205_69")
	if b.Online {
		t.Errorf("expected stale live box (LastSeen > 90s) to transition to Online=false when unreachable")
	}
}

// Test 153: Poller preserves Online=true for fresh live telemetry (LastSeen <= 90s) when direct dial fails (F12)
func TestTier2_F12_PollerPreservesFreshLiveTelemetryWithin90Seconds(t *testing.T) {
	srv, _ := newTestHubServer(t, func(c *Config) {
		c.DialTimeout = 25 * time.Millisecond
	})

	// Push fresh live telemetry via heartbeat
	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":           "inut_204_141",
		"online":           true,
		"cpu_pct":          22.1,
		"ram_available_mb": 1110,
		"heap_alloc_mb":    11.7,
	}, map[string]string{"NO_AUTH": "1"})

	// Backdate LastSeen to 30 seconds ago (still within 90s TTL)
	storeMutateBox(t, srv, "inut_204_141", func(b *Box) {
		b.LastSeen = time.Now().UTC().Add(-30 * time.Second)
	})

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]any{"box_id": "inut_204_141"}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/poll status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_204_141")
	if !b.Online {
		t.Errorf("expected fresh live box (LastSeen 30s <= 90s) to remain Online=true when direct dial from hub fails")
	}
	if b.CPUPct != 22.1 || b.RAMAvailableMB != 1110 || b.HeapAllocMB != 11.7 {
		t.Errorf("expected live telemetry metrics to be preserved, got cpu=%v avail=%d heap=%v",
			b.CPUPct, b.RAMAvailableMB, b.HeapAllocMB)
	}
}

// Test 154: Poller NEVER flips explicitly offline live box back to Online=true when dial fails (F12)
func TestTier2_F12_PollerNeverFlipsExplicitlyOfflineLiveBoxBackToOnline(t *testing.T) {
	srv, _ := newTestHubServer(t, func(c *Config) {
		c.DialTimeout = 25 * time.Millisecond
	})

	// Collector pushes online: false for inut_205_69
	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id": "inut_205_69",
		"online": false,
	}, map[string]string{"NO_AUTH": "1"})

	if storeGetBox(srv, "inut_205_69").Online {
		t.Fatalf("precondition failed: expected inut_205_69 Online=false")
	}

	// Trigger poller while box is unreachable
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]any{"box_id": "inut_205_69"}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/poll status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_205_69")
	if b.Online {
		t.Errorf("CRITICAL BUG: poller flipped offline box inut_205_69 back to Online=true when dial failed!")
	}
}

// Test 155: Poller preserves Offline state for non-live box when dial fails (F12)
func TestTier2_F12_PollerPreservesOfflineStateForNonLiveBoxWhenDialFails(t *testing.T) {
	srv, _ := newTestHubServer(t, func(c *Config) {
		c.DialTimeout = 25 * time.Millisecond
	})

	storeMutateBox(t, srv, "inut_205_85", func(b *Box) {
		b.Online = false
		b.TelemetrySource = "seed"
	})

	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]any{"box_id": "inut_205_85"}, nil)

	b := storeGetBox(srv, "inut_205_85")
	if b.Online {
		t.Errorf("expected non-live offline box inut_205_85 to stay Online=false when dial fails")
	}
}

// Test 156: Batch heartbeat rejects malformed JSON with 400 Bad Request (F12)
func TestTier2_F12_BatchHeartbeatInvalidJSONReturns400(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", "{broken_json:", map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for malformed JSON on /fleet/api/heartbeat/batch, got %d", rec.Code)
	}
}

// Test 157: Batch heartbeat rejects GET/PUT/DELETE with 405 Method Not Allowed (F12)
func TestTier2_F12_BatchHeartbeatMethodNotAllowedOnGETPUTDELETE(t *testing.T) {
	srv, _ := newTestHubServer(t)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		rec, _ := doHubRequest(t, srv, method, "/fleet/api/heartbeat/batch", nil, map[string]string{"NO_AUTH": "1"})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 Method Not Allowed for %s /fleet/api/heartbeat/batch, got %d", method, rec.Code)
		}
	}
}

// Test 158: Batch heartbeat with empty boxes array returns 200 OK and updated: 0 (F12)
func TestTier2_F12_BatchHeartbeatEmptyBoxesListReturnsZeroUpdated(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, resp := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", map[string]any{
		"boxes": []any{},
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for empty batch, got %d: %s", rec.Code, rec.Body.String())
	}
	if resp["updated"] != float64(0) {
		t.Errorf("expected updated=0 for empty batch, got %v", resp["updated"])
	}
}

// Test 159: Batch heartbeat with unknown box_id still updates valid boxes in the same batch (F12)
func TestTier2_F12_BatchHeartbeatWithUnknownBoxIDSkipsOrEnrollsWithoutFailingValidBoxes(t *testing.T) {
	srv, _ := newTestHubServer(t)

	batch := map[string]any{
		"boxes": []map[string]any{
			{"box_id": "inut_204_141", "online": true, "cpu_pct": 31.2, "heap_alloc_mb": 13.9},
			{"box_id": "inut_unknown_999_999", "online": true, "cpu_pct": 10.0},
			{"box_id": "inut_205_69", "online": false},
		},
	}

	rec, resp := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", batch, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when batch includes unknown box_id, got %d: %s", rec.Code, rec.Body.String())
	}
	if resp["updated"] != float64(2) {
		t.Errorf("expected updated=2 known boxes, got %v", resp["updated"])
	}
	if storeGetBox(srv, "inut_204_141").CPUPct != 31.2 {
		t.Errorf("expected inut_204_141 cpu_pct=31.2, got %v", storeGetBox(srv, "inut_204_141").CPUPct)
	}
	if storeGetBox(srv, "inut_205_69").Online {
		t.Errorf("expected inut_205_69 Online=false")
	}
}

// Test 160: Batch heartbeat entry with empty box_id is ignored gracefully (F12)
func TestTier2_F12_BatchHeartbeatMissingBoxIDEntryIgnoredGracefully(t *testing.T) {
	srv, _ := newTestHubServer(t)

	batch := map[string]any{
		"boxes": []map[string]any{
			{"box_id": "", "online": true, "cpu_pct": 50.0},
			{"box_id": "inut_205_72", "online": true, "cpu_pct": 14.4},
		},
	}

	rec, resp := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", batch, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if resp["updated"] != float64(1) {
		t.Errorf("expected updated=1, got %v", resp["updated"])
	}
}

// Test 161: probeSingleBox handles 500 or non-JSON /api/system/metrics gracefully while still marking TCP-reachable box Online (F11/F12)
func TestTier2_F11_ProbeSingleBoxHandlesNon200OrMalformedMetricsResponseGracefully(t *testing.T) {
	srv, _ := newTestHubServer(t)

	mockBrokenMetrics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer mockBrokenMetrics.Close()

	storeMutateBox(t, srv, "inut_204_141", func(b *Box) {
		b.Online = false
	})
	srv.RegisterTunnelTarget("inut_204_141", "2028", mockBrokenMetrics.Listener.Addr().String())

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]any{"box_id": "inut_204_141"}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/poll status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_204_141")
	if !b.Online {
		t.Errorf("expected TCP-reachable box to be Online=true even if /api/system/metrics returns 500")
	}
}

// Test 162: Explicit ffmpeg_count: 0 in heartbeat updates FFmpegCount to 0 (F12)
func TestTier2_F12_FFmpegCountZeroExplicitlyUpdatesToZero(t *testing.T) {
	srv, _ := newTestHubServer(t)

	if storeGetBox(srv, "inut_204_141").FFmpegCount == 0 {
		storeMutateBox(t, srv, "inut_204_141", func(b *Box) { b.FFmpegCount = 14 })
	}

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":       "inut_204_141",
		"online":       true,
		"ffmpeg_count": 0,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d", rec.Code)
	}

	b := storeGetBox(srv, "inut_204_141")
	if b.FFmpegCount != 0 {
		t.Errorf("expected FFmpegCount=0 when explicitly sent as 0, got %d", b.FFmpegCount)
	}
}

// Test 163: ComputeKPI AvgRTTMs averages only Online boxes and excludes Offline boxes (F12)
func TestTier2_F12_AvgRTTExcludesOfflineBoxes(t *testing.T) {
	srv, _ := newTestHubServer(t)

	for _, b := range storeListBoxes(srv) {
		storeMutateBox(t, srv, b.ID, func(box *Box) {
			if box.ID == "inut_204_141" {
				box.Online = true
				box.RTTMs = 20
			} else if box.ID == "inut_204_150" {
				box.Online = true
				box.RTTMs = 40
			} else {
				box.Online = false
				box.RTTMs = 999
			}
		})
	}

	kpi := srv.Store().ComputeKPI()
	if kpi.BoxesOnline != 2 {
		t.Fatalf("expected BoxesOnline=2, got %d", kpi.BoxesOnline)
	}
	if kpi.AvgRTTMs != 30 {
		t.Errorf("expected AvgRTTMs=30 ((20+40)/2 excluding offline 999ms boxes), got %d", kpi.AvgRTTMs)
	}
}

// Test 164: Adversarial Unicode and special characters in uptime_human & sd_mount_opts round-trip safely (F12)
func TestTier2_F12_AdversarialUptimeHumanAndMountOptsStringsInHeartbeat(t *testing.T) {
	srv, _ := newTestHubServer(t)

	advUptime := `12d 4h <script>alert("xss")</script> · Ổn định`
	advOpts := `rw,noatime,errors=continue; rm -rf /`

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":        "inut_204_141",
		"online":        true,
		"uptime_sec":    1051200,
		"uptime_human":  advUptime,
		"sd_mount_opts": advOpts,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat status = %d", rec.Code)
	}

	dRec, dBody := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_204_141", nil, nil)
	if dRec.Code != http.StatusOK {
		t.Fatalf("GET /fleet/api/boxes/inut_204_141 status = %d", dRec.Code)
	}
	boxMap, _ := dBody["box"].(map[string]any)
	if boxMap["uptime_human"] != advUptime || boxMap["sd_mount_opts"] != advOpts {
		t.Errorf("adversarial string round-trip mismatch: uptime=%v opts=%v", boxMap["uptime_human"], boxMap["sd_mount_opts"])
	}
}

// ============================================================================
// Tier 3 — Cross-Feature Pairwise Combinations (4 Tests: #165–#168)
// ============================================================================

// Test 165: Pairwise (F12 x F1 x F5): Batch heartbeat updates Summary, Clubs, Box Detail, and Shop Owner Portal simultaneously
func TestTier3_Pairwise_BatchHeartbeatUpdatesSummaryClubsBoxDetailAndPortalSimultaneously(t *testing.T) {
	srv, _ := newTestHubServer(t)

	masterClub := storeGetClub(srv, "club_master_billiards")
	pheClub := storeGetClub(srv, "club_phe_vietnam")
	if masterClub == nil || pheClub == nil {
		t.Fatalf("expected seeded clubs club_master_billiards and club_phe_vietnam")
	}

	batch := map[string]any{
		"boxes": []map[string]any{
			{
				"box_id":           "inut_205_72",
				"online":           true,
				"rtt_ms":           18,
				"cpu_pct":          20.5,
				"load_avg_1m":      1.25,
				"cpu_cores":        4,
				"ram_total_mb":     1945,
				"ram_used_mb":      845,
				"ram_available_mb": 1100,
				"heap_alloc_mb":    11.6,
				"kspcam_rss_mb":    30.2,
				"camera_rss_mb":    140.0,
				"swap_total_mb":    1945,
				"swap_used_mb":     45,
				"swap_free_mb":     1900,
				"uptime_sec":       604800,
				"uptime_human":     "7d 0h",
			},
			{
				"box_id": "inut_205_69",
				"online": false,
			},
		},
	}

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", batch, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("batch heartbeat failed: %d", rec.Code)
	}

	// 1. Check /fleet/api/boxes/inut_205_72
	_, boxDetail := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_205_72", nil, nil)
	bm, _ := boxDetail["box"].(map[string]any)
	if bm["cpu_pct"] != 20.5 || bm["ram_available_mb"] != float64(1100) || bm["swap_free_mb"] != float64(1900) {
		t.Errorf("box detail mismatch: %+v", bm)
	}

	// 2. Check /portal/{token}/api/status for PHE Vietnam (offline)
	_, phePortal := doHubRequest(t, srv, http.MethodGet, "/portal/"+pheClub.PortalToken+"/api/status", nil, map[string]string{"NO_AUTH": "1"})
	pheBoxes, _ := phePortal["boxes"].([]any)
	if len(pheBoxes) != 1 {
		t.Fatalf("expected 1 box in PHE portal, got %d", len(pheBoxes))
	}
	if pheBoxMap, _ := pheBoxes[0].(map[string]any); pheBoxMap["online"] != false {
		t.Errorf("expected PHE portal box online=false, got %v", pheBoxMap["online"])
	}
}

// Test 166: Pairwise (F11 x F2): Live metrics probe via tunnel & reverse proxy /fleet/proxy/{id}/kspcam/api/system/metrics coexist
func TestTier3_Pairwise_LiveMetricsProbeViaTunnelAndReverseProxyCoexistence(t *testing.T) {
	srv, _ := newTestHubServer(t)

	mockEdge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/system/metrics" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"online":           true,
				"cpu_pct":          17.8,
				"load_avg_1m":      1.12,
				"cpu_cores":        4,
				"ram_total_mb":     1945,
				"ram_used_mb":      795,
				"ram_available_mb": 1150,
				"ram_used_pct":     41,
				"heap_alloc_mb":    10.4,
				"heap_sys_mb":      21.0,
				"kspcam_rss_mb":    28.5,
				"camera_rss_mb":    135.0,
				"goroutines":       36,
				"swap_total_mb":    1945,
				"swap_used_mb":     25,
				"swap_free_mb":     1920,
				"ffmpeg_count":     14,
				"uptime_sec":       43200,
				"uptime_human":     "12h 0m",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockEdge.Close()

	srv.RegisterTunnelTarget("inut_204_141", "kspcam", mockEdge.Listener.Addr().String())

	// 1. Trigger poll to scrape /api/system/metrics
	pRec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]any{"box_id": "inut_204_141"}, nil)
	if pRec.Code != http.StatusOK {
		t.Fatalf("poll failed: %d", pRec.Code)
	}
	b := storeGetBox(srv, "inut_204_141")
	if b.CPUPct != 17.8 || b.HeapAllocMB != 10.4 || b.SwapFreeMB != 1920 {
		t.Errorf("expected polled metrics on box, got %+v", b)
	}

	// 2. Access /fleet/proxy/inut_204_141/kspcam/api/system/metrics through reverse proxy
	pxRec, pxBody := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_204_141/kspcam/api/system/metrics", nil, nil)
	if pxRec.Code != http.StatusOK {
		t.Fatalf("reverse proxy GET /api/system/metrics failed: %d", pxRec.Code)
	}
	if pxBody["heap_alloc_mb"] != 10.4 || pxBody["ram_available_mb"] != float64(1150) {
		t.Errorf("reverse proxy metrics mismatch: %+v", pxBody)
	}
}

// Test 167: Pairwise (F12 x F3): Heartbeat telemetry updates interact cleanly with 8-Step Golden Hardening audit
func TestTier3_Pairwise_HeartbeatSDMountOptsAndCorruptedMP4InteractWithHardeningAudit(t *testing.T) {
	srv, _ := newTestHubServer(t)

	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":           "inut_204_141",
		"online":           true,
		"cpu_pct":          15.5,
		"ram_total_mb":     1945,
		"ram_used_mb":      800,
		"ram_available_mb": 1145,
		"heap_alloc_mb":    11.0,
		"swap_total_mb":    1945,
		"swap_used_mb":     20,
		"swap_free_mb":     1925,
		"sd_fs_type":       "f2fs",
		"sd_mount_opts":    "rw,noatime,errors=continue",
	}, map[string]string{"NO_AUTH": "1"})

	hRec, hBody := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_204_141/hardening", nil, nil)
	if hRec.Code != http.StatusOK {
		t.Fatalf("GET hardening report failed: %d", hRec.Code)
	}
	if hBody["score"] != float64(8) || hBody["max_score"] != float64(8) {
		t.Errorf("expected hardening score 8/8, got %v/%v", hBody["score"], hBody["max_score"])
	}
}

// Test 168: Pairwise (F12 x F1 RBAC): Viewer role can read live telemetry endpoints but cannot trigger manual poll
func TestTier3_Pairwise_ViewerRoleCanReadLiveTelemetryButCannotTriggerPoll(t *testing.T) {
	srv, _ := newTestHubServer(t)

	viewerCookie := loginAndGetSessionCookie(t, srv, testViewerUser, testViewerPass, "198.51.100.77")
	if viewerCookie == nil {
		t.Fatalf("viewer login failed")
	}
	headers := map[string]string{"Cookie": viewerCookie.Name + "=" + viewerCookie.Value}

	for _, ep := range []string{"/fleet/api/summary", "/fleet/api/boxes", "/fleet/api/clubs"} {
		rec, _ := doHubRequest(t, srv, http.MethodGet, ep, nil, headers)
		if rec.Code != http.StatusOK {
			t.Errorf("viewer GET %s expected 200 OK, got %d", ep, rec.Code)
		}
	}

	pollRec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", nil, headers)
	if pollRec.Code != http.StatusForbidden {
		t.Errorf("viewer POST /fleet/api/poll expected 403 Forbidden, got %d", pollRec.Code)
	}
}

// ============================================================================
// Tier 4 — Real-World Application Scenarios (3 Tests: #169–#171)
// ============================================================================

// Test 169: End-to-End 42-Box Fleet Scenario: 39 Online + 3 Offline (inut_205_69, inut_205_85, inut_205_87) + Poller Preservation
func TestTier4_Scenario_Full42BoxFleetBatchTelemetryWith3OfflineBoxesAndPollerPreservation(t *testing.T) {
	srv, _ := newTestHubServer(t, func(c *Config) {
		c.DialTimeout = 15 * time.Millisecond
	})

	allBoxes := storeListBoxes(srv)
	totalBoxes := len(allBoxes)
	if totalBoxes < 30 {
		t.Fatalf("expected at least 30 seeded boxes, got %d", totalBoxes)
	}

	offlineSet := map[string]bool{
		"inut_205_69": true,
		"inut_205_85": true,
		"inut_205_87": true,
	}

	batchEntries := make([]map[string]any, 0, totalBoxes)
	for i, b := range allBoxes {
		if offlineSet[b.ID] {
			batchEntries = append(batchEntries, map[string]any{
				"box_id": b.ID,
				"online": false,
			})
			continue
		}
		batchEntries = append(batchEntries, map[string]any{
			"box_id":           b.ID,
			"online":           true,
			"rtt_ms":           14 + (i % 10),
			"temp_c":           48.0 + float64(i%8),
			"cpu_pct":          15.0 + float64(i%20),
			"load_avg_1m":      1.10 + float64(i%5)*0.1,
			"cpu_cores":        4,
			"ram_total_mb":     1945,
			"ram_used_mb":      820 + (i % 100),
			"ram_available_mb": 1945 - (820 + (i % 100)),
			"heap_alloc_mb":    10.5 + float64(i%5)*0.4,
			"heap_sys_mb":      22.0,
			"kspcam_rss_mb":    29.0 + float64(i%6),
			"camera_rss_mb":    140.0 + float64(i%15),
			"goroutines":       35 + (i % 10),
			"swap_total_mb":    1945,
			"swap_used_mb":     30 + (i % 40),
			"swap_free_mb":     1945 - (30 + (i % 40)),
			"ffmpeg_count":     b.CamerasCount * 2,
			"uptime_sec":       int64(86400 + i*600),
			"uptime_human":     fmt.Sprintf("1d %dh", (i*10)/60),
		})
	}

	// Step 1: Push full fleet batch heartbeat
	bRec, bResp := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", map[string]any{
		"boxes": batchEntries,
	}, map[string]string{"NO_AUTH": "1"})
	if bRec.Code != http.StatusOK {
		t.Fatalf("POST /fleet/api/heartbeat/batch status = %d, body = %s", bRec.Code, bRec.Body.String())
	}
	expectedOnline := totalBoxes - 3
	if bResp["boxes_online"] != float64(expectedOnline) || bResp["boxes_offline"] != float64(3) {
		t.Fatalf("after batch push: expected online=%d offline=3, got online=%v offline=%v",
			expectedOnline, bResp["boxes_online"], bResp["boxes_offline"])
	}

	// Step 2: Verify GET /fleet/api/summary KPI
	sRec, sBody := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, nil)
	if sRec.Code != http.StatusOK {
		t.Fatalf("GET /fleet/api/summary status = %d", sRec.Code)
	}
	kpiMap, _ := sBody["kpi"].(map[string]any)
	if kpiMap["boxes_online"] != float64(expectedOnline) || kpiMap["boxes_offline"] != float64(3) {
		t.Fatalf("summary KPI mismatch: online=%v offline=%v", kpiMap["boxes_online"], kpiMap["boxes_offline"])
	}

	// Step 3: Run background poller for the offline boxes (and sample online boxes) while unreachable directly
	for _, id := range []string{"inut_205_69", "inut_205_85", "inut_205_87", "inut_204_141", "inut_204_150"} {
		pRec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]any{"box_id": id}, nil)
		if pRec.Code != http.StatusOK {
			t.Fatalf("poll(%s) failed: %d", id, pRec.Code)
		}
	}

	// Step 4: Verify offline boxes remain strictly OFFLINE and fresh online boxes remain ONLINE
	for id := range offlineSet {
		box := storeGetBox(srv, id)
		if box == nil {
			t.Fatalf("box %s not found", id)
		}
		if box.Online {
			t.Errorf("box %s was falsely flipped back to Online=true after poll!", id)
		}
	}
	for _, id := range []string{"inut_204_141", "inut_204_150"} {
		box := storeGetBox(srv, id)
		if !box.Online {
			t.Errorf("fresh online box %s was unexpectedly marked offline!", id)
		}
	}

	kpiFinal := srv.Store().ComputeKPI()
	if kpiFinal.BoxesOnline != expectedOnline || kpiFinal.BoxesOffline != 3 {
		t.Errorf("post-poll KPI mismatch: expected online=%d offline=3, got online=%d offline=%d",
			expectedOnline, kpiFinal.BoxesOnline, kpiFinal.BoxesOffline)
	}
}

// Test 170: Real-World Scenario: Edge Box Power Outage -> Offline Detection -> Power Restore & Live Telemetry Recovery
func TestTier4_Scenario_EdgeBoxOutageAndLiveRecoveryLifecycle(t *testing.T) {
	srv, _ := newTestHubServer(t)
	totalBoxes := len(storeListBoxes(srv))
	pheClub := storeGetClub(srv, "club_phe_vietnam")
	if pheClub == nil {
		t.Fatalf("club_phe_vietnam not found")
	}

	// Phase 1: Box inut_205_69 is online with steady telemetry
	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":           "inut_205_69",
		"online":           true,
		"rtt_ms":           21,
		"cpu_pct":          19.5,
		"load_avg_1m":      1.15,
		"cpu_cores":        4,
		"ram_total_mb":     1945,
		"ram_used_mb":      855,
		"ram_available_mb": 1090,
		"heap_alloc_mb":    11.4,
		"swap_total_mb":    1945,
		"swap_used_mb":     25,
		"swap_free_mb":     1920,
		"uptime_sec":       500000,
		"uptime_human":     "5d 18h",
	}, map[string]string{"NO_AUTH": "1"})

	if srv.Store().ComputeKPI().BoxesOnline != totalBoxes {
		t.Fatalf("Phase 1: expected all %d boxes online", totalBoxes)
	}

	// Phase 2: Power outage at venue -> collector reports online: false
	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat/batch", map[string]any{
		"boxes": []map[string]any{
			{"box_id": "inut_205_69", "online": false},
		},
	}, map[string]string{"NO_AUTH": "1"})

	kpiOutage := srv.Store().ComputeKPI()
	if kpiOutage.BoxesOnline != totalBoxes-1 || kpiOutage.BoxesOffline != 1 {
		t.Fatalf("Phase 2: expected online=%d offline=1, got %d / %d", totalBoxes-1, kpiOutage.BoxesOnline, kpiOutage.BoxesOffline)
	}
	if storeGetClub(srv, "club_phe_vietnam").OnlineBoxesCount != 0 {
		t.Errorf("Phase 2: expected club_phe_vietnam OnlineBoxesCount=0")
	}

	// Phase 3: Box reboots and sends fresh heartbeat
	_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":           "inut_205_69",
		"online":           true,
		"rtt_ms":           17,
		"cpu_pct":          34.2,
		"load_avg_1m":      2.10,
		"ram_total_mb":     1945,
		"ram_used_mb":      645,
		"ram_available_mb": 1300,
		"heap_alloc_mb":    9.8,
		"swap_total_mb":    1945,
		"swap_used_mb":     0,
		"swap_free_mb":     1945,
		"uptime_sec":       45,
		"uptime_human":     "45s",
	}, map[string]string{"NO_AUTH": "1"})

	kpiRecovered := srv.Store().ComputeKPI()
	if kpiRecovered.BoxesOnline != totalBoxes || kpiRecovered.BoxesOffline != 0 {
		t.Errorf("Phase 3: expected full recovery online=%d offline=0, got %d / %d",
			totalBoxes, kpiRecovered.BoxesOnline, kpiRecovered.BoxesOffline)
	}
	bRecovered := storeGetBox(srv, "inut_205_69")
	if !bRecovered.Online || bRecovered.UptimeSec != 45 || bRecovered.RAMAvailableMB != 1300 {
		t.Errorf("Phase 3: unexpected box state after recovery: %+v", bRecovered)
	}
}

// Test 171: Real-World Scenario: Collector Bridge Outage Causes 90s TTL Expiration to Mark Unreachable Live Boxes Offline
func TestTier4_Scenario_CollectorBridgeStopCauses90sTTLGracefulOfflineTransition(t *testing.T) {
	srv, _ := newTestHubServer(t, func(c *Config) {
		c.DialTimeout = 15 * time.Millisecond
	})

	targetIDs := []string{"inut_204_141", "inut_205_72"}
	for _, id := range targetIDs {
		_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
			"box_id":        id,
			"online":        true,
			"cpu_pct":       18.0,
			"heap_alloc_mb": 11.0,
		}, map[string]string{"NO_AUTH": "1"})
	}

	// Simulate 2 minutes passing without collector heartbeats and unreachable direct dial
	staleTime := time.Now().UTC().Add(-2 * time.Minute)
	for _, id := range targetIDs {
		storeMutateBox(t, srv, id, func(b *Box) {
			b.VPNIP = "127.0.0.1:65532"
			b.LastSeen = staleTime
			b.TelemetrySource = "live"
		})
		_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]any{"box_id": id}, nil)
	}

	for _, id := range targetIDs {
		b := storeGetBox(srv, id)
		if b.Online {
			t.Errorf("expected box %s with expired 90s live TTL to be marked Online=false", id)
		}
	}
}
