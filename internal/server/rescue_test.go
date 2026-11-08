package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestBoxRescueActivatePreservesOutOfOrderMidAndZeroIdleMode(t *testing.T) {
	t.Setenv("KSPCAM_KEY", "rescue-test-crypto-key")

	var mu sync.Mutex
	savedMonitors := make(map[string]map[string]any)
	deletedMonitors := make(map[string]bool)

	mockShinobi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 && parts[1] == "configureMonitor" {
			mid := parts[3]
			if len(parts) == 5 && parts[4] == "delete" {
				mu.Lock()
				deletedMonitors[mid] = true
				delete(savedMonitors, mid)
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
				return
			}
			_ = r.ParseForm()
			var mon map[string]any
			_ = json.Unmarshal([]byte(r.FormValue("data")), &mon)
			mu.Lock()
			savedMonitors[mid] = mon
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer mockShinobi.Close()

	tmp := t.TempDir()
	inv, err := config.LoadInventory(filepath.Join(tmp, "cameras.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Seed 12 primary cameras (Camera01..Camera12) on 192.168.1.50:82 (like 5.123!)
	for ch := 1; ch <= 12; ch++ {
		_ = inv.Upsert(config.Device{
			ID:         fmt.Sprintf("192.168.1.50:82-c%d", 30+ch),
			Name:       fmt.Sprintf("Camera%02d", ch),
			Host:       "192.168.1.50",
			Port:       82,
			Vendor:     config.VendorHikvision,
			Username:   "live",
			Password:   "secret",
			NVRChannel: 30 + ch,
		})
	}

	cfg := config.Default()
	cfg.Shinobi.Enabled = true
	cfg.Shinobi.APIURL = mockShinobi.URL
	cfg.Shinobi.APIKey = "ADMINKEY123"
	cfg.Shinobi.GroupKey = "magic2"
	cfg.Redbida.KeyDir = filepath.Join(tmp, "change_ok")
	cfg.Bridge.SpoolDir = filepath.Join(tmp, "spool")
	cfg.Events.Enabled = false

	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	// Note: camera06 intentionally appears BEFORE camera05 (just like 5.122's real backup)!
	backupMonitors := `[
		{"mid":"camera01","host":"192.168.1.50","port":1554,"path":"/Streaming/Channels/301/","details":{"auto_host":"rtsp://live:pw@192.168.1.50:1554/Streaming/Channels/301/","muser":"live","mpass":"pw"}},
		{"mid":"camera06","host":"192.168.1.50","port":1554,"path":"/Streaming/Channels/2001/","details":{"auto_host":"rtsp://live:pw@192.168.1.50:1554/Streaming/Channels/2001/","muser":"live","mpass":"pw"}},
		{"mid":"camera05","host":"192.168.1.50","port":1554,"path":"/Streaming/Channels/1901/","details":{"auto_host":"rtsp://live:pw@192.168.1.50:1554/Streaming/Channels/1901/","muser":"live","mpass":"pw"}},
		{"mid":"camera16","host":"192.168.1.50","port":1554,"path":"/Streaming/Channels/3001/","details":{"auto_host":"rtsp://live:pw@192.168.1.50:1554/Streaming/Channels/3001/","muser":"live","mpass":"pw"}}
	]`

	actBody, _ := json.Marshal(map[string]any{
		"peerName":      "inut_205_122",
		"iface":         "lo",
		"vip":           "",
		"frpcSubdomain": "store-magic-1",
		"monitorsJson":  json.RawMessage(backupMonitors),
	})
	reqAct := httptest.NewRequest(http.MethodPost, "/api/rescue/activate", bytes.NewReader(actBody))
	recAct := httptest.NewRecorder()
	srv.handleBoxRescueActivate(recAct, reqAct)
	if recAct.Code != http.StatusOK {
		t.Fatalf("activate failed (%d): %s", recAct.Code, recAct.Body.String())
	}

	// Verify exact slot mapping:
	// camera01 -> camera13 (ch 3, 0-based 2)
	// camera05 -> camera17 (ch 19, 0-based 18)
	// camera06 -> camera18 (ch 20, 0-based 19)
	// camera16 -> camera28 (ch 30, 0-based 29)
	for mid, wantCh0 := range map[string]int{
		"camera13": 2,
		"camera17": 18,
		"camera18": 19,
		"camera28": 29,
	} {
		tgt, ok := srv.resolveBridgeTarget("", mid, 0, false)
		if !ok {
			t.Fatalf("expected bridge target for %s", mid)
		}
		if tgt.Channel != wantCh0 || tgt.Source.Port != 82 || tgt.IsCamSD {
			t.Fatalf("%s mismatch: ch=%d (want %d) port=%d isCamSD=%v", mid, tgt.Channel, wantCh0, tgt.Source.Port, tgt.IsCamSD)
		}
		mu.Lock()
		mon := savedMonitors[mid]
		mu.Unlock()
		if mon == nil || mon["mode"] != "idle" {
			t.Fatalf("expected go-shinobi monitor %s saved in zero-idle mode='idle', got %+v", mid, mon)
		}
	}

	// Now deactivate and verify clean teardown without touching the 12 primary cameras!
	deactBody, _ := json.Marshal(map[string]any{"peerName": "inut_205_122"})
	reqDeact := httptest.NewRequest(http.MethodPost, "/api/rescue/deactivate", bytes.NewReader(deactBody))
	recDeact := httptest.NewRecorder()
	srv.handleBoxRescueDeactivate(recDeact, reqDeact)
	if recDeact.Code != http.StatusOK {
		t.Fatalf("deactivate failed (%d): %s", recDeact.Code, recDeact.Body.String())
	}
	if len(inv.List()) != 12 {
		t.Fatalf("expected 12 primary cameras remaining after deactivate, got %d", len(inv.List()))
	}
}
