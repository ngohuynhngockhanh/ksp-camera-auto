package server

import (
	"bytes"
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

type mockShinobiBackend struct {
	Server   *httptest.Server
	mu       sync.Mutex
	APIKey   string
	GroupKey string
	Monitors map[string]shinobi.MonitorConfig
	States   map[string]string
	Videos   map[string][]shinobi.Video
}

func newMockShinobiBackend(apiKey, groupKey string) *mockShinobiBackend {
	m := &mockShinobiBackend{
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
					"details":  string(detJSON),
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
			return
		}

		// Route: POST /:apiKey/configureMonitor/:groupKey/:mid
		if len(parts) == 4 && parts[1] == "configureMonitor" && parts[2] == m.GroupKey && r.Method == http.MethodPost {
			mid := parts[3]
			_ = r.ParseForm()
			dataStr := r.FormValue("data")
			var cfg shinobi.MonitorConfig
			_ = json.Unmarshal([]byte(dataStr), &cfg)
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

func (m *mockShinobiBackend) Close() {
	m.Server.Close()
}

func authCookie(srv *Server) *http.Cookie {
	tok := srv.session.create("admin")
	return &http.Cookie{
		Name:  sessionCookie,
		Value: tok,
		Path:  "/",
	}
}

func TestServerShinobiRoutes(t *testing.T) {
	apiKey := "srv_api_key"
	groupKey := "srv_group"
	mockBackend := newMockShinobiBackend(apiKey, groupKey)
	defer mockBackend.Close()

	invPath := filepath.Join(t.TempDir(), "cameras.yaml")
	inv, err := config.LoadInventory(invPath)
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}

	_ = inv.Upsert(config.Device{
		ID:       "192.168.1.100:37777",
		Name:     "Test Cam 1",
		Host:     "192.168.1.100",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "pass",
	})

	cfg := config.Default()
	cfg.CamerasFile = invPath
	cfg.Shinobi = config.ShinobiConfig{
		APIURL:   mockBackend.Server.URL,
		APIKey:   apiKey,
		GroupKey: groupKey,
	}

	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer srv.Close()

	cookie := authCookie(srv)

	// 1. GET /api/shinobi/status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/shinobi/status", nil)
	reqStatus.AddCookie(cookie)
	recStatus := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recStatus, reqStatus)
	if recStatus.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recStatus.Code, http.StatusOK)
	}
	var status shinobi.ShinobiStatus
	if err := json.Unmarshal(recStatus.Body.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if !status.Configured || !status.Connected {
		t.Errorf("unexpected status: %+v", status)
	}

	// 2. POST /api/shinobi/sync-to-shinobi (Push)
	reqSyncTo := httptest.NewRequest(http.MethodPost, "/api/shinobi/sync-to-shinobi", nil)
	reqSyncTo.AddCookie(cookie)
	recSyncTo := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recSyncTo, reqSyncTo)
	if recSyncTo.Code != http.StatusOK {
		t.Fatalf("sync-to code = %d, want %d", recSyncTo.Code, http.StatusOK)
	}
	var reportTo shinobi.SyncReport
	if err := json.Unmarshal(recSyncTo.Body.Bytes(), &reportTo); err != nil {
		t.Fatalf("unmarshal reportTo: %v", err)
	}
	if reportTo.Created != 1 {
		t.Errorf("expected 1 created monitor, got %+v", reportTo)
	}

	// 3. GET /api/shinobi/monitors
	reqMons := httptest.NewRequest(http.MethodGet, "/api/shinobi/monitors", nil)
	reqMons.AddCookie(cookie)
	recMons := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recMons, reqMons)
	if recMons.Code != http.StatusOK {
		t.Fatalf("monitors code = %d, want %d", recMons.Code, http.StatusOK)
	}
	var monViews []shinobiMonitorView
	if err := json.Unmarshal(recMons.Body.Bytes(), &monViews); err != nil {
		t.Fatalf("unmarshal monViews: %v", err)
	}
	if len(monViews) != 1 || monViews[0].Name != "Test Cam 1" {
		t.Errorf("unexpected monViews: %+v", monViews)
	}

	// 4. POST /api/shinobi/monitors (State Change)
	stateBody, _ := json.Marshal(map[string]string{
		"action":    "state",
		"monitorId": monViews[0].Mid,
		"state":     "stop",
	})
	reqState := httptest.NewRequest(http.MethodPost, "/api/shinobi/monitors", bytes.NewReader(stateBody))
	reqState.AddCookie(cookie)
	recState := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recState, reqState)
	if recState.Code != http.StatusOK {
		t.Fatalf("state change code = %d, want %d", recState.Code, http.StatusOK)
	}

	// 5. GET /api/shinobi/videos
	mockBackend.Videos[monViews[0].Mid] = []shinobi.Video{
		{
			Mid:      monViews[0].Mid,
			Ke:       groupKey,
			Time:     time.Now(),
			End:      time.Now().Add(10 * time.Minute),
			Ext:      "mp4",
			Size:     500000,
			Href:     "/video1.mp4",
			Filename: "video1.mp4",
			Status:   1,
		},
	}
	reqVideos := httptest.NewRequest(http.MethodGet, "/api/shinobi/videos?mid="+monViews[0].Mid, nil)
	reqVideos.AddCookie(cookie)
	recVideos := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recVideos, reqVideos)
	if recVideos.Code != http.StatusOK {
		t.Fatalf("videos code = %d, want %d", recVideos.Code, http.StatusOK)
	}
	var videos []shinobi.Video
	if err := json.Unmarshal(recVideos.Body.Bytes(), &videos); err != nil {
		t.Fatalf("unmarshal videos: %v", err)
	}
	if len(videos) != 1 || videos[0].Filename != "video1.mp4" {
		t.Errorf("unexpected videos: %+v", videos)
	}

	// 6. POST /api/shinobi/sync-from-shinobi (Pull)
	reqSyncFrom := httptest.NewRequest(http.MethodPost, "/api/shinobi/sync-from-shinobi", nil)
	reqSyncFrom.AddCookie(cookie)
	recSyncFrom := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recSyncFrom, reqSyncFrom)
	if recSyncFrom.Code != http.StatusOK {
		t.Fatalf("sync-from code = %d, want %d", recSyncFrom.Code, http.StatusOK)
	}
	var reportFrom shinobi.SyncReport
	if err := json.Unmarshal(recSyncFrom.Body.Bytes(), &reportFrom); err != nil {
		t.Fatalf("unmarshal reportFrom: %v", err)
	}
	if reportFrom.Skipped != 1 {
		t.Errorf("expected 1 skipped device, got %+v", reportFrom)
	}

	// 7. POST /api/shinobi/monitors (Delete)
	delBody, _ := json.Marshal(map[string]string{
		"action":    "delete",
		"monitorId": monViews[0].Mid,
	})
	reqDel := httptest.NewRequest(http.MethodPost, "/api/shinobi/monitors", bytes.NewReader(delBody))
	reqDel.AddCookie(cookie)
	recDel := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("delete code = %d, want %d", recDel.Code, http.StatusOK)
	}
}
