package shinobi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
)

type MockShinobiServer struct {
	Server   *httptest.Server
	mu       sync.Mutex
	APIKey   string
	GroupKey string
	Monitors map[string]shinobi.MonitorConfig
	States   map[string]string
	Videos   map[string][]shinobi.Video
}

func NewMockShinobiServer(apiKey, groupKey string) *MockShinobiServer {
	m := &MockShinobiServer{
		APIKey:   apiKey,
		GroupKey: groupKey,
		Monitors: make(map[string]shinobi.MonitorConfig),
		States:   make(map[string]string),
		Videos:   make(map[string][]shinobi.Video),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 2 || parts[0] != m.APIKey {
			http.Error(w, `{"ok":false,"msg":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// Route: GET /:apiKey/monitor/:groupKey
		if len(parts) == 3 && parts[1] == "monitor" && parts[2] == m.GroupKey && r.Method == http.MethodGet {
			list := make([]map[string]any, 0)
			for _, mon := range m.Monitors {
				detJSON, _ := json.Marshal(mon.Details)
				list = append(list, map[string]any{
					"mid":      mon.Mid,
					"ke":       m.GroupKey,
					"name":     mon.Name,
					"type":     mon.Type,
					"mode":     m.States[mon.Mid],
					"host":     mon.Host,
					"port":     mon.Port,
					"protocol": mon.Protocol,
					"path":     mon.Path,
					"ext":      mon.Ext,
					"details":  string(detJSON), // stringified JSON testing
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
			return
		}

		// Route: GET /:apiKey/monitor/:groupKey/:mid
		if len(parts) == 4 && parts[1] == "monitor" && parts[2] == m.GroupKey && r.Method == http.MethodGet {
			mid := parts[3]
			mon, ok := m.Monitors[mid]
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]any{})
				return
			}
			detJSON, _ := json.Marshal(mon.Details)
			res := map[string]any{
				"mid":      mon.Mid,
				"ke":       m.GroupKey,
				"name":     mon.Name,
				"type":     mon.Type,
				"mode":     m.States[mon.Mid],
				"host":     mon.Host,
				"port":     mon.Port,
				"protocol": mon.Protocol,
				"path":     mon.Path,
				"ext":      mon.Ext,
				"details":  string(detJSON),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{res})
			return
		}

		// Route: POST /:apiKey/configureMonitor/:groupKey/:mid
		if len(parts) == 4 && parts[1] == "configureMonitor" && parts[2] == m.GroupKey && r.Method == http.MethodPost {
			mid := parts[3]
			_ = r.ParseForm()
			dataStr := r.FormValue("data")
			var cfg shinobi.MonitorConfig
			if err := json.Unmarshal([]byte(dataStr), &cfg); err != nil {
				http.Error(w, `{"ok":false,"msg":"bad json payload"}`, http.StatusBadRequest)
				return
			}
			cfg.Mid = mid
			m.Monitors[mid] = cfg
			if m.States[mid] == "" {
				m.States[mid] = cfg.Mode
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"msg":"Monitor Saved"}`))
			return
		}

		// Route: GET /:apiKey/configureMonitor/:groupKey/:mid/delete
		if len(parts) == 5 && (parts[1] == "configureMonitor" || parts[1] == "monitor") && parts[2] == m.GroupKey && parts[4] == "delete" {
			mid := parts[3]
			delete(m.Monitors, mid)
			delete(m.States, mid)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"msg":"Monitor Deleted"}`))
			return
		}

		// Route: GET /:apiKey/monitor/:groupKey/:mid/:state
		if len(parts) == 5 && parts[1] == "monitor" && parts[2] == m.GroupKey {
			mid := parts[3]
			state := parts[4]
			m.States[mid] = state
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"msg":"State Changed"}`))
			return
		}

		// Route: GET /:apiKey/videos/:groupKey/:mid
		if len(parts) == 4 && parts[1] == "videos" && parts[2] == m.GroupKey {
			mid := parts[3]
			w.Header().Set("Content-Type", "application/json")
			vids := m.Videos[mid]
			if vids == nil {
				vids = []shinobi.Video{}
			}
			_ = json.NewEncoder(w).Encode(vids)
			return
		}

		http.NotFound(w, r)
	})

	m.Server = httptest.NewServer(mux)
	return m
}

func (m *MockShinobiServer) Close() {
	m.Server.Close()
}

func TestClientCRUD(t *testing.T) {
	apiKey := "test_api_key_12345"
	groupKey := "test_group"
	mock := NewMockShinobiServer(apiKey, groupKey)
	defer mock.Close()

	client := shinobi.NewClient(mock.Server.URL, apiKey, groupKey)
	ctx := context.Background()

	// 1. Status test
	st, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("client.Status failed: %v", err)
	}
	if !st.Configured || !st.Connected || st.MonitorCount != 0 {
		t.Errorf("unexpected status: %+v", st)
	}

	// 2. AddMonitor test
	mon := shinobi.MonitorConfig{
		Mid:      "cam_01",
		Name:     "Test Camera 1",
		Type:     "h264",
		Mode:     "record",
		Host:     "192.168.1.10",
		Port:     "554",
		Protocol: "rtsp",
		Path:     "/cam/realmonitor?channel=1&subtype=0",
		Ext:      "mp4",
		Details: shinobi.MonitorDetails{
			AutoHost: "rtsp://admin:pass@192.168.1.10:554/cam/realmonitor?channel=1&subtype=0",
			Muser:    "admin",
			Mpass:    "pass",
			Vcodec:   "copy",
			Acodec:   "copy",
		},
	}
	if err := client.AddMonitor(ctx, mon); err != nil {
		t.Fatalf("AddMonitor failed: %v", err)
	}

	// 3. ListMonitors test
	mons, err := client.ListMonitors(ctx)
	if err != nil {
		t.Fatalf("ListMonitors failed: %v", err)
	}
	if len(mons) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(mons))
	}
	if mons[0].Mid != "cam_01" || mons[0].Name != "Test Camera 1" {
		t.Errorf("unexpected monitor: %+v", mons[0])
	}
	det := mons[0].ParseDetails()
	if det.Muser != "admin" || det.Vcodec != "copy" {
		t.Errorf("unexpected parsed details: %+v", det)
	}

	// 4. GetMonitor test
	fetched, err := client.GetMonitor(ctx, "cam_01")
	if err != nil {
		t.Fatalf("GetMonitor failed: %v", err)
	}
	if fetched.Mid != "cam_01" {
		t.Errorf("expected mid 'cam_01', got %q", fetched.Mid)
	}

	// 5. EditMonitor test
	mon.Name = "Test Camera 1 Renamed"
	if err := client.EditMonitor(ctx, "cam_01", mon); err != nil {
		t.Fatalf("EditMonitor failed: %v", err)
	}
	fetched, err = client.GetMonitor(ctx, "cam_01")
	if err != nil || fetched.Name != "Test Camera 1 Renamed" {
		t.Fatalf("EditMonitor verification failed: %v, %+v", err, fetched)
	}

	// 6. ChangeMonitorState test
	if err := client.ChangeMonitorState(ctx, "cam_01", "stop"); err != nil {
		t.Fatalf("ChangeMonitorState 'stop' failed: %v", err)
	}
	if mock.States["cam_01"] != "stop" {
		t.Errorf("expected state 'stop', got %q", mock.States["cam_01"])
	}
	if err := client.ChangeMonitorState(ctx, "cam_01", "invalid_state"); err == nil {
		t.Error("expected error for invalid monitor state")
	}

	// 7. GetVideos test
	now := time.Now()
	mock.Videos["cam_01"] = []shinobi.Video{
		{
			Mid:      "cam_01",
			Ke:       groupKey,
			Time:     now,
			End:      now.Add(15 * time.Minute),
			Ext:      "mp4",
			Size:     1024000,
			Href:     "/test_group/videos/cam_01/clip1.mp4",
			Filename: "clip1.mp4",
			Status:   1,
		},
	}
	vids, err := client.GetVideos(ctx, "cam_01", 10)
	if err != nil {
		t.Fatalf("GetVideos failed: %v", err)
	}
	if len(vids) != 1 || vids[0].Filename != "clip1.mp4" {
		t.Errorf("unexpected videos: %+v", vids)
	}

	// 8. DeleteMonitor test
	if err := client.DeleteMonitor(ctx, "cam_01"); err != nil {
		t.Fatalf("DeleteMonitor failed: %v", err)
	}
	mons, err = client.ListMonitors(ctx)
	if err != nil {
		t.Fatalf("ListMonitors after delete failed: %v", err)
	}
	if len(mons) != 0 {
		t.Errorf("expected 0 monitors after delete, got %d", len(mons))
	}
}

func TestSyncEngine(t *testing.T) {
	apiKey := "test_api_key_sync"
	groupKey := "test_group_sync"
	mock := NewMockShinobiServer(apiKey, groupKey)
	defer mock.Close()

	client := shinobi.NewClient(mock.Server.URL, apiKey, groupKey)
	ctx := context.Background()

	tmpDir := t.TempDir()
	invPath := filepath.Join(tmpDir, "cameras.yaml")

	inv, err := config.LoadInventory(invPath)
	if err != nil {
		t.Fatalf("LoadInventory failed: %v", err)
	}

	// Seed inventory with 2 cameras
	_ = inv.Upsert(config.Device{
		ID:       "192.168.1.10:37777",
		Name:     "Bàn 1 (Dahua)",
		Host:     "192.168.1.10",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "password123",
	})
	_ = inv.Upsert(config.Device{
		ID:         "192.168.1.200:8000-c2",
		Name:       "Bàn 2 (Hik NVR c2)",
		Host:       "192.168.1.200",
		Port:       8000,
		Vendor:     config.VendorHikvision,
		Username:   "admin",
		Password:   "password123",
		NVRChannel: 2,
	})

	// Test SyncToShinobi (Push)
	reportTo, err := client.SyncToShinobi(ctx, inv)
	if err != nil {
		t.Fatalf("SyncToShinobi failed: %v", err)
	}
	if reportTo.Created != 2 || reportTo.Updated != 0 || len(reportTo.Errors) > 0 {
		t.Errorf("unexpected SyncToShinobi report: %+v", reportTo)
	}

	// Verify Shinobi received the monitors
	mons, err := client.ListMonitors(ctx)
	if err != nil || len(mons) != 2 {
		t.Fatalf("expected 2 monitors in Shinobi, got %d, err: %v", len(mons), err)
	}

	// SyncToShinobi again (should be unchanged)
	reportTo2, err := client.SyncToShinobi(ctx, inv)
	if err != nil {
		t.Fatalf("SyncToShinobi second pass failed: %v", err)
	}
	if reportTo2.Unchanged != 2 || reportTo2.Created != 0 {
		t.Errorf("expected 2 unchanged on second pass, got %+v", reportTo2)
	}

	// Now add a 3rd monitor on Shinobi directly to test SyncFromShinobi (Pull)
	mon3 := shinobi.MonitorConfig{
		Mid:      "cam_192_168_1_50_554",
		Name:     "Cam Bàn 3 (Tiandy)",
		Type:     "h264",
		Mode:     "record",
		Host:     "192.168.1.50",
		Port:     "554",
		Protocol: "rtsp",
		Path:     "/cam/realmonitor?channel=1&subtype=0",
		Ext:      "mp4",
		Details: shinobi.MonitorDetails{
			AutoHost: "rtsp://admin:tiandy123@192.168.1.50:554/cam/realmonitor?channel=1&subtype=0",
			Muser:    "admin",
			Mpass:    "tiandy123",
			Vcodec:   "copy",
		},
	}
	if err := client.AddMonitor(ctx, mon3); err != nil {
		t.Fatalf("failed adding mon3: %v", err)
	}

	// Test SyncFromShinobi (Pull)
	reportFrom, err := client.SyncFromShinobi(ctx, inv)
	if err != nil {
		t.Fatalf("SyncFromShinobi failed: %v", err)
	}
	if reportFrom.Added != 1 || reportFrom.Skipped != 2 {
		t.Errorf("unexpected SyncFromShinobi report: %+v", reportFrom)
	}

	// Verify inventory now has 3 cameras
	devList := inv.List()
	if len(devList) != 3 {
		t.Errorf("expected 3 devices in inventory, got %d", len(devList))
	}
}

func TestUnconfiguredStatus(t *testing.T) {
	client := shinobi.NewClient("", "", "")
	ctx := context.Background()

	st, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("client.Status failed: %v", err)
	}
	if st.Configured || st.Connected {
		t.Errorf("expected unconfigured, got: %+v", st)
	}
}

func TestBuildMonitorConfigAndDeviceToMid(t *testing.T) {
	client := shinobi.NewClient("http://127.0.0.1:8080", "testkey", "testgroup")

	// 1. Dahua camera
	dDahua := config.Device{
		ID:       "192.168.1.10:37777",
		Name:     "Dahua Cam",
		Host:     "192.168.1.10",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "mock_dahua_pass",
	}
	midDahua := shinobi.DeviceToMid(dDahua)
	if midDahua != "cam_192_168_1_10_37777" {
		t.Errorf("unexpected midDahua: %s", midDahua)
	}
	cfgDahua := client.BuildMonitorConfig(dDahua)
	if cfgDahua.Path != "/cam/realmonitor?channel=1&subtype=0" {
		t.Errorf("unexpected Dahua path: %s", cfgDahua.Path)
	}
	if !strings.Contains(cfgDahua.Details.AutoHost, "rtsp://admin:mock_dahua_pass@192.168.1.10:554/cam/realmonitor") {
		t.Errorf("unexpected Dahua AutoHost: %s", cfgDahua.Details.AutoHost)
	}

	// 2. Hikvision NVR Channel 5
	dHik := config.Device{
		ID:         "192.168.1.200:8000-c5",
		Name:       "Hik NVR c5",
		Host:       "192.168.1.200",
		Port:       8000,
		Vendor:     config.VendorHikvision,
		Username:   "admin",
		Password:   "hikpass",
		NVRChannel: 5,
	}
	midHik := shinobi.DeviceToMid(dHik)
	if midHik != "cam_192_168_1_200_c5" {
		t.Errorf("unexpected midHik: %s", midHik)
	}
	cfgHik := client.BuildMonitorConfig(dHik)
	if cfgHik.Path != "/Streaming/Channels/501" {
		t.Errorf("unexpected Hik path: %s", cfgHik.Path)
	}

	// 3. Tiandy
	dTiandy := config.Device{
		ID:     "192.168.1.30:554",
		Host:   "192.168.1.30",
		Vendor: config.VendorTiandy,
	}
	cfgTiandy := client.BuildMonitorConfig(dTiandy)
	if cfgTiandy.Details.AutoHost != "rtsp://192.168.1.30:554/cam/realmonitor?channel=1&subtype=0" {
		t.Errorf("unexpected Tiandy AutoHost: %s", cfgTiandy.Details.AutoHost)
	}
}

func TestClientErrorResponses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/badkey/monitor/group1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "msg": "Invalid API Key"})
	})
	mux.HandleFunc("/goodkey/monitor/group1/nonexistent", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]any{})
	})
	mux.HandleFunc("/goodkey/configureMonitor/group1/fail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "msg": "Disk Full"})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()

	badClient := shinobi.NewClient(ts.URL, "badkey", "group1")
	_, err := badClient.ListMonitors(ctx)
	if err == nil || !strings.Contains(err.Error(), "Invalid API Key") {
		t.Errorf("expected Invalid API Key error, got: %v", err)
	}

	goodClient := shinobi.NewClient(ts.URL, "goodkey", "group1")
	_, err = goodClient.GetMonitor(ctx, "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got: %v", err)
	}

	err = goodClient.AddMonitor(ctx, shinobi.MonitorConfig{Mid: "fail"})
	if err == nil || !strings.Contains(err.Error(), "Disk Full") {
		t.Errorf("expected Disk Full error, got: %v", err)
	}
}

func TestFlexibleString_UnmarshalNumericAndStringFields(t *testing.T) {
	jsonBlob := `[
		{
			"mid": "cam1",
			"ke": "grp1",
			"name": "Camera 1",
			"port": 554,
			"fps": 25,
			"width": 1920,
			"height": 1080
		},
		{
			"mid": "cam2",
			"ke": "grp1",
			"name": "Camera 2",
			"port": "8554",
			"fps": "30",
			"width": "1280",
			"height": "720"
		},
		{
			"mid": "cam3",
			"ke": "grp1",
			"name": "Camera 3",
			"port": null,
			"fps": null,
			"width": null,
			"height": null
		}
	]`

	var mons []shinobi.Monitor
	if err := json.Unmarshal([]byte(jsonBlob), &mons); err != nil {
		t.Fatalf("failed to unmarshal monitors with mixed numeric/string fields: %v", err)
	}

	if len(mons) != 3 {
		t.Fatalf("expected 3 monitors, got %d", len(mons))
	}

	// Case 1: Numbers
	if mons[0].Port.String() != "554" || string(mons[0].Port) != "554" {
		t.Errorf("expected port '554', got '%s'", mons[0].Port)
	}
	if mons[0].FPS.String() != "25" || mons[0].Width.String() != "1920" || mons[0].Height.String() != "1080" {
		t.Errorf("unexpected numeric fields in mon[0]: fps=%s, w=%s, h=%s", mons[0].FPS, mons[0].Width, mons[0].Height)
	}

	// Case 2: Strings
	if mons[1].Port.String() != "8554" || mons[1].FPS.String() != "30" || mons[1].Width.String() != "1280" || mons[1].Height.String() != "720" {
		t.Errorf("unexpected string fields in mon[1]: port=%s, fps=%s, w=%s, h=%s", mons[1].Port, mons[1].FPS, mons[1].Width, mons[1].Height)
	}

	// Case 3: Nulls
	if mons[2].Port.String() != "" || mons[2].FPS.String() != "" || mons[2].Width.String() != "" || mons[2].Height.String() != "" {
		t.Errorf("expected empty fields for nulls in mon[2], got: port=%s, fps=%s, w=%s, h=%s", mons[2].Port, mons[2].FPS, mons[2].Width, mons[2].Height)
	}
}
